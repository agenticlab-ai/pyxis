package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
)

type OpenResult struct {
	Branch      string
	PullRequest string
}

// Open starts an unlocked contract on its own branch and opens its proposal
// under the configured agent account. Identity and day-one checks precede all
// writes. A failed publication names the branch so a later invocation can be
// recovered without silently rewriting the scaffold.
func Open(ctx context.Context, root string, cfg config.Config, feature string, run RunCommand) (out OpenResult, err error) {
	if !validFeature(feature) {
		return OpenResult{}, fmt.Errorf("feature %q must be a lowercase hyphenated slug", feature)
	}
	defer func() {
		if err != nil {
			err = recordStepFailure(ctx, root, cfg, feature, 1, "draft-open", err)
		}
	}()
	if cfg.Draft.AgentLogin == "" || cfg.Draft.AgentTokenEnv == "" {
		return OpenResult{}, fmt.Errorf("agent identity is not configured; set draft.agent_login and draft.agent_token_env in .pyxis/config.local.yaml before opening a draft")
	}
	token := os.Getenv(cfg.Draft.AgentTokenEnv)
	if token == "" {
		return OpenResult{}, fmt.Errorf("agent identity is not available; set %s with the configured agent account token before opening a draft", cfg.Draft.AgentTokenEnv)
	}
	if run == nil {
		return OpenResult{}, fmt.Errorf("draft process adapter is required")
	}
	agentEnv := []string{"GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
	identity, err := run(ctx, root, agentEnv, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return OpenResult{}, fmt.Errorf("verify configured agent identity: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(string(identity)), cfg.Draft.AgentLogin) {
		return OpenResult{}, fmt.Errorf("configured agent identity %q differs from authenticated account %q; set the agent token and login to the same account", cfg.Draft.AgentLogin, strings.TrimSpace(string(identity)))
	}
	status, err := run(ctx, root, nil, "git", "status", "--porcelain")
	if err != nil {
		return OpenResult{}, fmt.Errorf("inspect working tree: %w", err)
	}
	if len(strings.TrimSpace(string(status))) > 0 {
		return OpenResult{}, fmt.Errorf("working tree is not clean; commit or move changes before opening a draft")
	}
	if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, cfg.Branch); err != nil {
		return OpenResult{}, fmt.Errorf("fetch mainline before opening %s: %w", feature, err)
	}
	ledgerPath := filepath.ToSlash(filepath.Join(cfg.PRDs, feature, "000-base.md"))
	ledger, err := run(ctx, root, nil, "git", "ls-tree", "-r", "--name-only", cfg.MainlineRef(), "--", ledgerPath)
	if err != nil {
		return OpenResult{}, fmt.Errorf("inspect mainline PRD ledger for %s: %w", feature, err)
	}
	if strings.TrimSpace(string(ledger)) == ledgerPath {
		return OpenResult{}, fmt.Errorf("%s has a PRD ledger: the day-one rule keeps existing features in the document pipeline", feature)
	}
	withAgent := func(ctx context.Context, dir string, _ []string, name string, args ...string) ([]byte, error) {
		return run(ctx, dir, agentEnv, name, args...)
	}
	snapshot, err := Inspect(ctx, root, cfg, withAgent)
	if err != nil {
		return OpenResult{}, fmt.Errorf("inspect existing contracts: %w", err)
	}
	if snapshot.PRDLedgers[feature] {
		return OpenResult{}, fmt.Errorf("%s has a PRD ledger: the day-one rule keeps existing features in the document pipeline", feature)
	}
	if snapshot.LockedVersions[feature] > 0 {
		return OpenResult{}, fmt.Errorf("%s is already locked at version %d; use pyxis draft reopen %s", feature, snapshot.LockedVersions[feature], feature)
	}
	if snapshot.OpenDrafts[feature] {
		return OpenResult{}, fmt.Errorf("%s already has an open draft pull request", feature)
	}
	branch := cfg.Draft.BranchPrefix + feature
	files, err := scaffold(ctx, root, cfg, feature, run)
	if err != nil {
		return OpenResult{}, err
	}
	var prd, tech []byte
	for _, file := range files {
		switch filepath.Base(file.Path) {
		case "prd.md":
			prd = file.Body
		case "tech-spec.md":
			tech = file.Body
		}
	}
	queue, err := newPendingEvents(root, cfg.Remote, feature, 1)
	if err != nil {
		return OpenResult{}, fmt.Errorf("open pending draft events: %w", err)
	}
	opened, err := events.New(events.DraftOpened, feature, compile.ContractDigest(prd, tech), time.Now())
	if err != nil {
		return OpenResult{}, err
	}
	opened.Origin = "new"
	files, err = scaffoldEvents(ctx, files, queue, opened)
	if err != nil {
		return OpenResult{}, err
	}
	priorBranch, err := run(ctx, root, nil, "git", "branch", "--show-current")
	if err != nil {
		return OpenResult{}, fmt.Errorf("inspect current branch before opening: %w", err)
	}
	priorHead, err := run(ctx, root, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return OpenResult{}, fmt.Errorf("inspect current head before opening: %w", err)
	}
	created, committed := false, false
	var written []string
	defer func() {
		if err != nil && created {
			err = errors.Join(err, rollbackDraftStart(ctx, root, branch, strings.TrimSpace(string(priorBranch)), strings.TrimSpace(string(priorHead)), written, committed, run))
		}
	}()
	if _, err := run(ctx, root, nil, "git", "switch", "-c", branch, cfg.MainlineRef()); err != nil {
		return OpenResult{}, fmt.Errorf("create draft branch %s: %w", branch, err)
	}
	created = true
	paths := make([]string, 0, len(files))
	for _, file := range files {
		path := filepath.Join(root, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return OpenResult{}, fmt.Errorf("create scaffold directory for %s: %w", file.Path, err)
		}
		if _, statErr := os.Lstat(path); statErr == nil {
			return OpenResult{}, fmt.Errorf("draft scaffold path %s already exists", file.Path)
		} else if !os.IsNotExist(statErr) {
			return OpenResult{}, fmt.Errorf("inspect scaffold path %s: %w", file.Path, statErr)
		}
		written = append(written, file.Path)
		if err := os.WriteFile(path, file.Body, 0o644); err != nil {
			return OpenResult{}, fmt.Errorf("write scaffold %s: %w", file.Path, err)
		}
		paths = append(paths, file.Path)
	}
	if _, err := run(ctx, root, nil, "git", append([]string{"add", "--"}, paths...)...); err != nil {
		return OpenResult{}, fmt.Errorf("stage draft scaffold on %s: %w", branch, err)
	}
	if _, err := run(ctx, root, nil, "git", "commit", "-m", "docs(draft): open "+feature+" contract"); err != nil {
		return OpenResult{}, fmt.Errorf("commit draft scaffold on %s: %w", branch, err)
	}
	committed = true
	repoURL, err := run(ctx, root, agentEnv, "gh", "repo", "view", "--json", "url", "--jq", ".url")
	if err != nil {
		return OpenResult{}, fmt.Errorf("resolve code-host repository for %s: %w", branch, err)
	}
	pushURL, err := agentPushURL(strings.TrimSpace(string(repoURL)))
	if err != nil {
		return OpenResult{}, err
	}
	if _, err := run(ctx, root, agentEnv, "git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", pushURL, branch+":refs/heads/"+branch); err != nil {
		return OpenResult{}, fmt.Errorf("push draft branch %s as the agent: %w", branch, err)
	}
	prURL, err := run(ctx, root, agentEnv, "gh", "pr", "create", "--draft", "--head", branch, "--base", cfg.Branch, "--title", feature+" — contract draft", "--body", "Draft PRD and tech spec for "+feature+". Lock preparation will replace the working files with approved records.")
	if err != nil {
		return OpenResult{}, fmt.Errorf("branch %s was pushed, but opening its draft pull request failed: %w; resolve the code-host error and open the PR from this branch using the agent account", branch, err)
	}
	return OpenResult{Branch: branch, PullRequest: strings.TrimSpace(string(prURL))}, nil
}

type scaffoldFile struct {
	Path string
	Body []byte
}

func scaffold(ctx context.Context, root string, cfg config.Config, feature string, run RunCommand) ([]scaffoldFile, error) {
	return scaffoldAt(ctx, root, cfg, feature, cfg.MainlineRef(), run)
}

func scaffoldAt(ctx context.Context, root string, cfg config.Config, feature, mainline string, run RunCommand) ([]scaffoldFile, error) {
	rootPath := filepath.Join(cfg.Drafts, feature)
	templates := []struct{ path, destination string }{
		{"prd-template.md", "prd.md"},
		{"draft-tech-spec-template.md", "tech-spec.md"},
		{"draft-problems-template.md", "problems.md"},
		{"draft-events-template.md", "events.md"},
	}
	files := make([]scaffoldFile, 0, len(templates)+2)
	for _, item := range templates {
		templatePath := filepath.ToSlash(filepath.Join(cfg.TemplatesDir, item.path))
		body, err := run(ctx, root, nil, "git", "show", mainline+":"+templatePath)
		if err != nil {
			return nil, fmt.Errorf("read draft template %s from current mainline: %w", item.path, err)
		}
		body = []byte(strings.ReplaceAll(string(body), "<Feature or capability name>", feature))
		body = []byte(strings.ReplaceAll(string(body), "<Feature>", feature))
		if item.destination == "prd.md" {
			body = emptyDraftPRDRows(body)
		}
		files = append(files, scaffoldFile{Path: filepath.Join(rootPath, item.destination), Body: body})
	}
	for _, dir := range []string{"brainstorm", "reviews"} {
		files = append(files, scaffoldFile{Path: filepath.Join(rootPath, dir, ".keep"), Body: []byte{}})
	}
	return files, nil
}

// Template examples guide authors; they are not an authored requirement
// baseline. Preserve the structure and guidance while letting each row's
// first real appearance establish the compiler's ordinary change history.
func emptyDraftPRDRows(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			cells := strings.SplitN(strings.TrimSpace(line), "|", 3)
			if len(cells) == 3 {
				if _, _, unit := schema.ParseID(cells[1]); unit {
					continue
				}
			}
		}
		kept = append(kept, line)
	}
	return []byte(strings.Join(kept, "\n"))
}

func agentPushURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("code-host repository URL %q must be HTTPS without credentials", raw)
	}
	return strings.TrimSuffix(raw, ".git") + ".git", nil
}
