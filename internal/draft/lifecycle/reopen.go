package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

// LockedContract is the byte-exact seed read from one verified lock tag.
type LockedContract struct {
	Version   int
	PRD       []byte
	TechSpec  []byte
	Digest    string
	SourceRef string // Verified origin basis; local locks use their immutable landing OID.
}

// LockedReader loads a binding lock from the configured remote.
type LockedReader func(context.Context, string, string, string, int, RunCommand) (LockedContract, error)

type ReopenResult struct {
	Branch      string
	PullRequest string // empty: a reopened draft is published by its preparation
	Version     int
	Digest      string
	Unfinished  []string // the live plan's unfinished tasks, which keep running
}

// Reopen starts version N+1 from the exact text protected by lock N, on a
// local branch. Open tasks of the current plan keep running: the draft may
// change their charters in the plan, and its one pull request — opened when
// the draft is prepared for lock — carries contract and plan together.
func Reopen(ctx context.Context, root string, cfg config.Config, feature string, locked LockedReader, run RunCommand) (ReopenResult, error) {
	return ReopenWithCause(ctx, root, cfg, feature, "other", "", locked, run)
}

// ReopenWithCause records the classified reason and its evidence reference.
func ReopenWithCause(ctx context.Context, root string, cfg config.Config, feature, cause, causeRef string, locked LockedReader, run RunCommand) (ReopenResult, error) {
	return ReopenTracked(ctx, root, cfg, feature, cause, causeRef, locked, run, nil)
}

// ReopenTracked is ReopenWithCause listing the live plan's unfinished tasks
// from the configured task tracker; nil tasks reads the code host's GitHub
// issues.
func ReopenTracked(ctx context.Context, root string, cfg config.Config, feature, cause, causeRef string, locked LockedReader, run RunCommand, tasks worktracker.Lister) (out ReopenResult, err error) {
	if !validFeature(feature) || locked == nil || run == nil {
		return ReopenResult{}, fmt.Errorf("reopen: feature, locked reader, and process adapter are required")
	}
	if cause != "new-need" && cause != "dependency-change" && cause != "gap-lock-should-have-caught" && cause != "other" {
		return ReopenResult{}, fmt.Errorf("reopen cause %q is invalid", cause)
	}
	if cause != "other" && strings.TrimSpace(causeRef) == "" {
		return ReopenResult{}, fmt.Errorf("reopen cause %s needs an evidence reference", cause)
	}
	versionForFailure := 0
	defer func() {
		if err != nil && versionForFailure > 0 {
			err = recordStepFailure(ctx, root, cfg, feature, versionForFailure, "draft-reopen", err)
		}
	}()
	if cfg.Draft.AgentLogin == "" || cfg.Draft.AgentTokenEnv == "" {
		return ReopenResult{}, fmt.Errorf("agent identity is not configured; set draft.agent_login and draft.agent_token_env in .pyxis/config.local.yaml before reopening a draft")
	}
	token := os.Getenv(cfg.Draft.AgentTokenEnv)
	if token == "" {
		return ReopenResult{}, fmt.Errorf("agent identity is not available; set %s with the configured agent account token before reopening a draft", cfg.Draft.AgentTokenEnv)
	}
	agentEnv := []string{"GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
	identity, err := run(ctx, root, agentEnv, "gh", "api", "user", "--jq", ".login")
	if err != nil {
		return ReopenResult{}, fmt.Errorf("verify configured agent identity: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(string(identity)), cfg.Draft.AgentLogin) {
		return ReopenResult{}, fmt.Errorf("configured agent identity %q differs from authenticated account %q", cfg.Draft.AgentLogin, strings.TrimSpace(string(identity)))
	}
	withAgent := func(ctx context.Context, dir string, _ []string, name string, args ...string) ([]byte, error) {
		return run(ctx, dir, agentEnv, name, args...)
	}
	status, err := run(ctx, root, nil, "git", "status", "--porcelain")
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect working tree: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return ReopenResult{}, fmt.Errorf("working tree is not clean; commit or move changes before reopening a draft")
	}
	if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, cfg.Branch); err != nil {
		return ReopenResult{}, fmt.Errorf("fetch current mainline: %w", err)
	}
	snapshot, err := Inspect(ctx, root, cfg, withAgent)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect binding lock: %w", err)
	}
	version := snapshot.LockedVersions[feature]
	if snapshot.PRDLedgers[feature] {
		return ReopenResult{}, fmt.Errorf("%s is in the document pipeline and has no feature-draft reopen path", feature)
	}
	if version < 1 {
		return ReopenResult{}, fmt.Errorf("%s has no binding lock; open its first draft instead", feature)
	}
	if snapshot.OpenDrafts[feature] {
		return ReopenResult{}, fmt.Errorf("%s already has an open draft pull request", feature)
	}
	var unfinished []string
	if tasks != nil {
		unfinished, err = UnfinishedTasksWithTracker(ctx, root, cfg, feature, cfg.MainlineRef(), tasks, run)
	} else {
		unfinished, err = UnfinishedTasks(ctx, root, cfg, feature, withAgent)
	}
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect unfinished plan tasks: %w", err)
	}
	seed, err := locked(ctx, root, cfg.Remote, feature, version, run)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("read binding lock: %w", err)
	}
	if seed.Version != version || len(seed.PRD) == 0 || len(seed.TechSpec) == 0 || seed.Digest == "" {
		return ReopenResult{}, fmt.Errorf("binding lock %s/v%d has incomplete stored text", feature, version)
	}
	if compile.ContractDigest(seed.PRD, seed.TechSpec) != seed.Digest {
		return ReopenResult{}, fmt.Errorf("binding lock text does not reproduce its digest")
	}
	next := version + 1
	versionForFailure = next
	if causeRef == "" {
		causeRef = fmt.Sprintf("lock/%s/v%d", feature, version)
	}
	queue, err := newPendingEvents(root, cfg.Remote, feature, next)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("open pending draft events: %w", err)
	}
	pending, err := queue.Read(ctx)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("read pending draft events: %w", err)
	}
	opened, err := events.New(events.DraftOpened, feature, seed.Digest, time.Now())
	if err != nil {
		return ReopenResult{}, err
	}
	opened.Origin, opened.BasisVersion = "reopen", &version
	reopened, err := events.New(events.DraftReopened, feature, seed.Digest, time.Now())
	if err != nil {
		return ReopenResult{}, err
	}
	reopened.FromVersion, reopened.Cause, reopened.CauseRef = version, cause, causeRef
	eventBody, _, err := events.Append(nil, append(pending, opened, reopened)...)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("append reopening events: %w", err)
	}
	branch := cfg.Draft.BranchPrefix + feature + "-v" + strconv.Itoa(next)
	// A reopened draft is published by its preparation, onto a branch the
	// code host must not have yet; one left by an earlier closed draft would
	// strand it.
	remote, err := run(ctx, root, nil, "git", "ls-remote", cfg.Remote, "refs/heads/"+branch)
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect remote draft branch: %w", err)
	}
	if strings.TrimSpace(string(remote)) != "" {
		return ReopenResult{}, fmt.Errorf("remote branch %s is left from an earlier draft that has no open pull request; delete it on the code host before reopening", branch)
	}
	priorBranch, err := run(ctx, root, nil, "git", "branch", "--show-current")
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect current branch before reopening: %w", err)
	}
	priorHead, err := run(ctx, root, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return ReopenResult{}, fmt.Errorf("inspect current head before reopening: %w", err)
	}
	created, committed := false, false
	var written []string
	defer func() {
		if err != nil && created {
			err = errors.Join(err, rollbackDraftStart(ctx, root, branch, strings.TrimSpace(string(priorBranch)), strings.TrimSpace(string(priorHead)), written, committed, run))
		}
	}()
	if _, err := run(ctx, root, nil, "git", "switch", "-c", branch, cfg.MainlineRef()); err != nil {
		return ReopenResult{}, fmt.Errorf("create reopen branch %s: %w", branch, err)
	}
	created = true
	paths := []string{"prd.md", "tech-spec.md", "problems.md", "events.md"}
	bodies := [][]byte{seed.PRD, seed.TechSpec, {}, eventBody}
	staged := make([]string, 0, len(paths))
	for i, name := range paths {
		rel := filepath.Join(cfg.Drafts, feature, name)
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return ReopenResult{}, fmt.Errorf("create reopen directory: %w", err)
		}
		if _, statErr := os.Lstat(full); statErr == nil {
			return ReopenResult{}, fmt.Errorf("reopen seed path %s already exists", rel)
		} else if !os.IsNotExist(statErr) {
			return ReopenResult{}, fmt.Errorf("inspect reopen seed %s: %w", rel, statErr)
		}
		written = append(written, rel)
		if err := os.WriteFile(full, bodies[i], 0o644); err != nil {
			return ReopenResult{}, fmt.Errorf("write reopen seed %s: %w", rel, err)
		}
		staged = append(staged, filepath.ToSlash(rel))
	}
	if _, err := run(ctx, root, nil, "git", append([]string{"add", "--"}, staged...)...); err != nil {
		return ReopenResult{}, fmt.Errorf("stage reopen seed: %w", err)
	}
	if _, err := run(ctx, root, nil, "git", "commit", "-m", fmt.Sprintf("docs(draft): reopen %s v%d", feature, next)); err != nil {
		return ReopenResult{}, fmt.Errorf("commit reopen seed: %w", err)
	}
	committed = true
	return ReopenResult{Branch: branch, Version: next, Digest: seed.Digest, Unfinished: unfinished}, nil
}
