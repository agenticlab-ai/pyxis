package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

type AbandonResult struct {
	Branch      string
	PullRequest int
	Head        string
	RecordPath  string
}

// Abandon replaces a draft proposal with one reason Learning. A reopened
// draft that was never published opens its one pull request with that record.
// The operator's code-owner review is required on that path; this command
// requests it and never merges the PR. The repository must auto-delete merged
// head branches.
func Abandon(ctx context.Context, root string, cfg config.Config, feature, reason string, run RunCommand) (out AbandonResult, err error) {
	reason = strings.TrimSpace(reason)
	if !validFeature(feature) || reason == "" || len(reason) > 1000 || strings.ContainsAny(reason, "\r\n\x00") || run == nil {
		return out, fmt.Errorf("abandon: valid feature, one-line reason (1–1000 bytes), and process adapter are required")
	}
	if cfg.Draft.AgentLogin == "" || cfg.Draft.AgentTokenEnv == "" {
		return out, fmt.Errorf("agent identity is not configured; set draft.agent_login and draft.agent_token_env in .pyxis/config.local.yaml")
	}
	token := os.Getenv(cfg.Draft.AgentTokenEnv)
	if token == "" {
		return out, fmt.Errorf("agent identity is not available; set %s before abandoning a draft", cfg.Draft.AgentTokenEnv)
	}
	agentEnv := []string{"GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
	identity, err := run(ctx, root, agentEnv, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return out, fmt.Errorf("verify configured agent identity: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(string(identity)), cfg.Draft.AgentLogin) {
		return out, fmt.Errorf("configured agent identity %q differs from authenticated account %q", cfg.Draft.AgentLogin, strings.TrimSpace(string(identity)))
	}
	withAgent := func(ctx context.Context, dir string, _ []string, name string, args ...string) ([]byte, error) {
		return run(ctx, dir, agentEnv, name, args...)
	}
	status, err := run(ctx, root, nil, "git", "status", "--porcelain")
	if err != nil {
		return out, fmt.Errorf("inspect draft working tree: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return out, fmt.Errorf("working tree is not clean; commit or move changes before abandonment")
	}
	if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, cfg.Branch); err != nil {
		return out, fmt.Errorf("fetch mainline before abandonment: %w", err)
	}
	snapshot, err := Inspect(ctx, root, cfg, withAgent)
	if err != nil {
		return out, fmt.Errorf("inspect draft for abandonment: %w", err)
	}
	// A reopened draft stays local until its preparation publishes it, so a
	// locked feature may have a draft without a pull request.
	published := snapshot.OpenDrafts[feature]
	if snapshot.PRDLedgers[feature] || (!published && snapshot.LockedVersions[feature] == 0) {
		return out, fmt.Errorf("%s has no open feature-draft pull request to abandon", feature)
	}
	version := snapshot.LockedVersions[feature] + 1
	defer func() {
		if err != nil {
			err = recordStepFailure(ctx, root, cfg, feature, version, "draft-abandon", err)
		}
	}()
	branch := cfg.Draft.BranchPrefix + feature
	if version > 1 {
		branch += "-v" + strconv.Itoa(version)
	}
	current, err := run(ctx, root, nil, "git", "branch", "--show-current")
	if err != nil {
		return out, fmt.Errorf("inspect draft branch: %w", err)
	}
	if strings.TrimSpace(string(current)) != branch {
		return out, fmt.Errorf("abandon %s from branch %s, not %s", feature, strings.TrimSpace(string(current)), branch)
	}
	oldHead, err := run(ctx, root, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve draft head: %w", err)
	}
	remote, err := run(ctx, root, nil, "git", "ls-remote", cfg.Remote, "refs/heads/"+branch)
	if err != nil {
		return out, fmt.Errorf("inspect remote draft branch: %w", err)
	}
	remoteHead, _, _ := strings.Cut(strings.TrimSpace(string(remote)), "\t")
	switch {
	case !published && remoteHead != "":
		return out, fmt.Errorf("remote draft branch %s has no open pull request; reconcile it before abandonment", branch)
	case published && remoteHead != strings.TrimSpace(string(oldHead)):
		return out, fmt.Errorf("remote draft branch %s moved or is missing; reconcile it before abandonment", branch)
	}
	var pull struct {
		Number      int    `json:"number"`
		IsDraft     bool   `json:"isDraft"`
		HeadRefName string `json:"headRefName"`
	}
	if published {
		prData, err := run(ctx, root, agentEnv, "gh", "pr", "view", branch, "--json", "number,isDraft,headRefName")
		if err != nil {
			return out, fmt.Errorf("find draft pull request: %w", err)
		}
		if err := json.Unmarshal(prData, &pull); err != nil || pull.Number < 1 || pull.HeadRefName != branch {
			return out, fmt.Errorf("invalid draft pull request for %s: %v", branch, err)
		}
		out.PullRequest = pull.Number
	}
	repoSlug, err := run(ctx, root, agentEnv, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return out, fmt.Errorf("resolve code-host repository: %w", err)
	}
	slug := strings.TrimSpace(string(repoSlug))
	if !githubRepo.MatchString(slug) {
		return out, fmt.Errorf("code-host repository %q is invalid", slug)
	}
	autoDelete, err := run(ctx, root, agentEnv, "gh", "api", "repos/"+slug, "--jq", ".delete_branch_on_merge")
	if err != nil || strings.TrimSpace(string(autoDelete)) != "true" {
		return out, fmt.Errorf("repository must enable automatic head-branch deletion before abandonment: %v", err)
	}
	repoURL, err := run(ctx, root, agentEnv, "gh", "repo", "view", "--json", "url", "--jq", ".url")
	if err != nil {
		return out, fmt.Errorf("resolve code-host push URL: %w", err)
	}
	pushURL, err := agentPushURL(strings.TrimSpace(string(repoURL)))
	if err != nil {
		return out, err
	}
	eventLog, err := abandonmentEvents(ctx, root, cfg, feature, version, strings.TrimSpace(string(oldHead)))
	if err != nil {
		return out, err
	}
	rationale := reason
	if len(eventLog) > 0 {
		rationale += "\n\nDraft events:\n" + string(eventLog)
	}
	tmp, err := os.MkdirTemp("", "pyxis-draft-abandon-*")
	if err != nil {
		return out, fmt.Errorf("create abandonment worktree: %w", err)
	}
	defer func() {
		if _, cleanupErr := run(ctx, root, nil, "git", "worktree", "remove", "--force", tmp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove abandonment worktree: %w", cleanupErr))
		}
	}()
	if _, err := run(ctx, root, nil, "git", "worktree", "add", "--detach", tmp, cfg.MainlineRef()); err != nil {
		return out, fmt.Errorf("create abandonment worktree: %w", err)
	}
	var entropy [10]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return out, fmt.Errorf("mint abandonment record: %w", err)
	}
	now := time.Now().UTC()
	learning := &record.Record{
		Schema: record.Schema, ID: record.NewID(record.Learning, now, entropy), Kind: record.Learning,
		Subject:   fmt.Sprintf("learning/draft-abandonment-%s-v%d", feature, version),
		Statement: fmt.Sprintf("Abandoned %s draft version %d: %s", feature, version, reason),
		Intent:    "Preserve the reason for an abandoned contract draft without landing its working content.",
		Rationale: rationale, Issuer: "agent:" + cfg.Draft.AgentLogin, CreatedAt: now.Format(time.RFC3339),
		Provenance: record.Provenance{Source: fmt.Sprintf("draft-abandonment:%s:v%d", feature, version), PR: out.PullRequest},
	}
	if _, err := store.Write(filepath.Join(tmp, store.Dir), learning); err != nil {
		return out, fmt.Errorf("write abandonment Learning: %w", err)
	}
	out.RecordPath = filepath.ToSlash(filepath.Join(store.Dir, learning.Path()))
	if _, err := run(ctx, tmp, nil, "git", "add", "--", out.RecordPath); err != nil {
		return out, fmt.Errorf("stage abandonment Learning: %w", err)
	}
	if _, err := run(ctx, tmp, nil, "git", "commit", "-m", fmt.Sprintf("docs(draft): abandon %s v%d", feature, version)); err != nil {
		return out, fmt.Errorf("commit abandonment Learning: %w", err)
	}
	base, err := run(ctx, tmp, nil, "git", "rev-parse", cfg.MainlineRef())
	if err != nil {
		return out, fmt.Errorf("resolve abandonment policy base: %w", err)
	}
	policy, err := run(ctx, tmp, nil, "go", "run", "./cmd/memory", "reconcile", "--baseline-sync", "--baseline-ref", strings.TrimSpace(string(base)), "--json")
	if err != nil {
		return out, fmt.Errorf("synchronize abandonment memory policy: %w", err)
	}
	var sync struct {
		Schema  string   `json:"schema_version"`
		Status  string   `json:"status"`
		Head    string   `json:"head"`
		Written []string `json:"written"`
	}
	if err := json.Unmarshal(policy, &sync); err != nil {
		return out, fmt.Errorf("read abandonment policy changes: %w", err)
	}
	prePolicy, err := run(ctx, tmp, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve abandonment head before policy sync: %w", err)
	}
	if sync.Schema != "memory-policy-sync/v1" || sync.Status != "complete" || sync.Head != strings.TrimSpace(string(prePolicy)) || sync.Written == nil {
		return out, fmt.Errorf("abandonment policy sync returned an incomplete result")
	}
	if len(sync.Written) != 0 {
		for _, path := range sync.Written {
			if path != reconcile.PolicyPath && !reconcile.PolicyMemberPath(path) {
				return out, fmt.Errorf("abandonment policy sync wrote unexpected path %q", path)
			}
		}
		if _, err := run(ctx, tmp, nil, "git", append([]string{"add", "--"}, sync.Written...)...); err != nil {
			return out, fmt.Errorf("stage abandonment memory policy: %w", err)
		}
		if _, err := run(ctx, tmp, nil, "git", "commit", "-m", fmt.Sprintf("chore(memory): synchronize %s abandonment", feature)); err != nil {
			return out, fmt.Errorf("commit abandonment memory policy: %w", err)
		}
	}
	if _, err := run(ctx, tmp, nil, "go", "run", "./cmd/memory-gate", "--base", strings.TrimSpace(string(base))); err != nil {
		return out, fmt.Errorf("memory gate at abandonment tree: %w", err)
	}
	head, err := run(ctx, tmp, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve abandonment head: %w", err)
	}
	out.Head = strings.TrimSpace(string(head))
	body := fmt.Sprintf("Abandon draft %s v%d.\n\nReason: %s\n\nThis proposal adds one Learning and requires the operator's code-owner review. The repository deletes the branch after merge.", feature, version, reason)
	var unreviewed error // the pull request opened, but its reviewer was not requested
	if !published {
		out.PullRequest, err = PublishNew(ctx, run, tmp, agentEnv, pushURL, branch, out.Head, cfg.Branch, fmt.Sprintf("docs(draft): abandon %s v%d", feature, version), body, cfg.Approval.OperatorLogin)
		if err != nil && !errors.Is(err, ErrReviewNotRequested) {
			return out, fmt.Errorf("publish abandonment Learning: %w", err)
		}
		unreviewed = err
	} else {
		if _, err := run(ctx, tmp, agentEnv, "git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "--force-with-lease=refs/heads/"+branch+":"+remoteHead, pushURL, out.Head+":refs/heads/"+branch); err != nil {
			return out, fmt.Errorf("publish abandonment Learning: %w", err)
		}
		if _, err := run(ctx, root, agentEnv, "gh", "pr", "edit", strconv.Itoa(out.PullRequest), "--body", body); err != nil {
			return out, fmt.Errorf("publish abandonment explanation: %w", err)
		}
		if pull.IsDraft {
			if _, err := run(ctx, root, agentEnv, "gh", "pr", "ready", strconv.Itoa(out.PullRequest)); err != nil {
				return out, fmt.Errorf("mark abandonment PR ready: %w", err)
			}
		}
		if operator := cfg.Approval.OperatorLogin; operator != "" {
			if _, err := run(ctx, root, agentEnv, "gh", "pr", "edit", strconv.Itoa(out.PullRequest), "--add-reviewer", operator); err != nil {
				return out, fmt.Errorf("request operator review: %w", err)
			}
		}
	}
	if _, err := run(ctx, root, nil, "git", "reset", "--hard", out.Head); err != nil {
		return out, fmt.Errorf("align abandoned local branch: %w", err)
	}
	out.Branch = branch
	if unreviewed != nil {
		return out, fmt.Errorf("publish abandonment Learning: %w", unreviewed)
	}
	return out, nil
}
