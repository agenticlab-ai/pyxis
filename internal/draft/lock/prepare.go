package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

var featureName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// rebaseLivePlan moves the feature's live plan onto the lock being prepared.
// Tests replace it to exercise preparation without a complete plan fixture.
var rebaseLivePlan = plan.RebaseLive

type PrepareResult struct {
	Branch      string
	Head        string
	PullRequest int
	Verdict     readiness.Verdict
	Proposal    *codehost.ProposalGeneration
	Request     string
}

// LocalPreparation supplies the installed memory gate and verified binding
// reader. It does not give preparation approval or landing authority.
type LocalPreparation struct {
	Proposals codehost.DraftProposals
	Locked    lifecycle.LockedReader
	Memory    func(context.Context, string, string) error
	// Unfinished lists the live plan's unfinished tasks. Nil means the task
	// state cannot be read here, so a live plan counts as in use and moves
	// onto the new version, where it must fit.
	Unfinished func(context.Context) ([]string, error)
	// AppendOnly reports that the tasks' tracker cannot rewrite their
	// comments, so a live plan in use cannot move onto the new version.
	AppendOnly bool
}

// Prepare works in a detached temporary worktree. Until the compiled tree
// passes readiness and the memory gate, a failure cannot change the author's
// branch or its working files.
func Prepare(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand) (out PrepareResult, err error) {
	return prepare(ctx, root, cfg, feature, run, nil, nil)
}

// PrepareTracked is Prepare reading the live plan's task state from the
// configured task tracker; nil tasks reads the code host's GitHub issues.
func PrepareTracked(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand, tasks worktracker.Lister) (PrepareResult, error) {
	return prepare(ctx, root, cfg, feature, run, nil, tasks)
}

func PrepareLocal(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand, local LocalPreparation) (PrepareResult, error) {
	if local.Proposals == nil || local.Locked == nil || local.Memory == nil {
		return PrepareResult{}, fmt.Errorf("local preparation requires proposal, binding, and memory readers")
	}
	return prepare(ctx, root, cfg, feature, run, &local, nil)
}

func prepare(ctx context.Context, root string, cfg config.Config, feature string, run lifecycle.RunCommand, local *LocalPreparation, tasks worktracker.Lister) (out PrepareResult, err error) {
	if !featureName.MatchString(feature) || run == nil {
		return out, fmt.Errorf("prepare lock: a valid feature and process adapter are required")
	}
	refused := false
	version := 1
	defer func() {
		if err != nil && !refused {
			err = recordPrepareFailure(ctx, root, cfg, feature, version, out.Verdict.DraftDigest, err)
		}
	}()
	branch := cfg.Draft.BranchPrefix + feature
	var agentEnv []string
	actor := "agent:" + cfg.Draft.AgentLogin
	if local == nil {
		if cfg.Draft.AgentLogin == "" || cfg.Draft.AgentTokenEnv == "" {
			return out, fmt.Errorf("prepare lock: configure the agent identity before requesting approval")
		}
		token := os.Getenv(cfg.Draft.AgentTokenEnv)
		if token == "" {
			return out, fmt.Errorf("prepare lock: %s is not set", cfg.Draft.AgentTokenEnv)
		}
		agentEnv = []string{"GH_TOKEN=" + token, "GIT_TERMINAL_PROMPT=0"}
		identity, err := run(ctx, root, agentEnv, "gh", "api", "user", "--jq", ".login")
		if err != nil {
			return out, fmt.Errorf("verify agent identity: %w", err)
		}
		if !strings.EqualFold(strings.TrimSpace(string(identity)), cfg.Draft.AgentLogin) {
			return out, fmt.Errorf("authenticated account %q differs from configured draft agent %q", strings.TrimSpace(string(identity)), cfg.Draft.AgentLogin)
		}
	} else {
		actor = "agent:local-draft"
	}
	checkedOut, err := command(ctx, run, root, nil, "git", "branch", "--show-current")
	if err != nil {
		return out, fmt.Errorf("inspect draft branch: %w", err)
	}
	if raw, ok := strings.CutPrefix(checkedOut, branch+"-v"); ok {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr == nil && parsed > 1 && strconv.Itoa(parsed) == raw {
			version, branch = parsed, checkedOut
		}
	}
	out.Branch = branch
	if checkedOut != branch {
		return out, fmt.Errorf("prepare lock %s from branch %s, not %s", feature, branch, checkedOut)
	}
	dirty, err := command(ctx, run, root, nil, "git", "status", "--porcelain")
	if err != nil {
		return out, fmt.Errorf("inspect draft working files: %w", err)
	}
	if dirty != "" {
		return out, fmt.Errorf("prepare lock: commit or move working changes before preparation")
	}
	oldHead, err := command(ctx, run, root, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve draft head: %w", err)
	}
	var base, remoteOID string
	var generation codehost.ProposalGeneration
	if local == nil {
		if _, err := run(ctx, root, nil, "git", "fetch", cfg.Remote, cfg.Branch); err != nil {
			return out, fmt.Errorf("fetch current mainline: %w", err)
		}
		base, err = command(ctx, run, root, nil, "git", "rev-parse", cfg.MainlineRef())
		if err != nil {
			return out, fmt.Errorf("resolve current mainline: %w", err)
		}
		remoteHead, err := command(ctx, run, root, nil, "git", "ls-remote", cfg.Remote, "refs/heads/"+branch)
		if err != nil {
			return out, fmt.Errorf("inspect remote draft branch: %w", err)
		}
		// A reopened draft stays local until it is ready: preparation
		// publishes it and opens its one pull request. A published branch
		// must still be exactly the local head.
		remoteOID, _, _ = strings.Cut(remoteHead, "\t")
		if remoteOID != "" && remoteOID != oldHead {
			return out, fmt.Errorf("remote draft branch %s moved; fetch and reconcile it before preparation", branch)
		}
	} else {
		r, resolveErr := local.Proposals.Mainline(ctx)
		if resolveErr != nil {
			return out, resolveErr
		}
		base, cfg.Remote = r.MainlineOID, r.Origin
		generation, err = local.Proposals.Read(ctx, fmt.Sprintf("draft-%s-v%d", feature, version))
		if err != nil {
			return out, fmt.Errorf("read local draft: %w", err)
		}
		p := generation.Proposal
		if p.Kind != "draft" || p.Feature != feature || (p.State != "open" && p.State != "prepared") || p.Branch != "refs/heads/"+branch || p.TargetRef != r.MainlineRef {
			return out, fmt.Errorf("local draft differs from the open origin proposal; reconcile it or resume a prepared round")
		}
		if p.State == "prepared" {
			if p.TargetOID != base {
				return out, codehost.ErrProposalConflict
			}
			out, err = resumePreparedLocal(ctx, root, cfg, generation, version, oldHead, branch, run, *local)
			if err != nil && out.Verdict.DraftDigest != "" && !out.Verdict.Ready {
				refused = true
				err = recordReadinessRefusal(ctx, root, cfg, feature, version, out.Verdict, err)
			}
			return out, err
		}
		if p.Head != oldHead {
			return out, fmt.Errorf("local draft differs from the open origin proposal; reconcile it or resume a prepared round")
		}
		remoteOID = p.Head
	}
	if err := verifySourceStamps(ctx, root, base, oldHead, run); err != nil {
		return out, fmt.Errorf("verify draft stamps before rebase: %w", err)
	}
	tmp, err := os.MkdirTemp("", "pyxis-draft-lock-*")
	if err != nil {
		return out, fmt.Errorf("create preparation worktree: %w", err)
	}
	defer func() {
		if _, cleanupErr := run(ctx, root, nil, "git", "worktree", "remove", "--force", tmp); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove preparation worktree: %w", cleanupErr))
		}
	}()
	if _, err := run(ctx, root, nil, "git", "worktree", "add", "--detach", tmp, oldHead); err != nil {
		return out, fmt.Errorf("create preparation worktree: %w", err)
	}
	if err := sharePreparationCache(root, tmp); err != nil {
		return out, fmt.Errorf("share installed tool cache with preparation worktree: %w", err)
	}
	if _, err := run(ctx, tmp, []string{"ARGUS_STAMP_REBASE=1", "ARGUS_DRIVE_SLUG="}, "git", "rebase", base); err != nil {
		return out, fmt.Errorf("rebase draft onto current mainline %s: %w", base, err)
	}
	repo, err := gitsrc.Open(ctx, tmp)
	if err != nil {
		return out, fmt.Errorf("open rebased draft: %w", err)
	}
	rebasedHead, err := repo.Head(ctx)
	if err != nil {
		return out, fmt.Errorf("resolve rebased head: %w", err)
	}
	result, err := compile.CompileBranch(ctx, repo, base, rebasedHead, cfg.Drafts, feature)
	if err != nil {
		return out, fmt.Errorf("compile rebased draft: %w", err)
	}
	sources, err := readiness.LoadSources(ctx, repo, base, cfg)
	if err != nil {
		return out, fmt.Errorf("load pinned readiness sources: %w", err)
	}
	latest, err := latestLockedVersion(ctx, tmp, cfg.Remote, feature, run)
	if err != nil {
		return out, err
	}
	var binding *resolve.Projection
	var locked lifecycle.LockedContract
	if version > 1 && latest == version-1 {
		reader := lifecycle.LockedReader(LoadContract)
		if local != nil {
			reader = local.Locked
		}
		binding, locked, err = loadBindingProjectionWithReader(ctx, tmp, cfg.Remote, feature, version-1, run, repo, reader)
		if err != nil {
			return out, fmt.Errorf("load binding version: %w", err)
		}
	}
	out.Verdict = readiness.Judge(result, sources)
	if latest != version-1 {
		remedy := fmt.Sprintf("latest binding lock is v%d; reopen from that version", latest)
		if version == 1 {
			remedy = fmt.Sprintf("feature is already locked at v%d; reopen it", latest)
		} else if latest == 0 {
			remedy = "no binding lock exists; open the first version"
		}
		out.Verdict.Unmet = append(out.Verdict.Unmet, readiness.Unmet{Condition: "FR-20", Item: feature, Remedy: remedy})
		out.Verdict.Sort()
	} else if version > 1 {
		reader := lifecycle.LockedReader(LoadContract)
		if local != nil {
			reader = local.Locked
		}
		guard, guardErr := guardReopenedWithReader(ctx, tmp, cfg.Remote, repo, run, base, rebasedHead, cfg.Drafts, feature, locked, result, reader)
		if guardErr != nil {
			return out, fmt.Errorf("guard reopened lock: %w", guardErr)
		}
		out.Verdict.Unmet = append(out.Verdict.Unmet, guard...)
		out.Verdict.Sort()
	}
	if !out.Verdict.Ready {
		refused = true
		body, marshalErr := json.Marshal(out.Verdict)
		if marshalErr != nil {
			return out, fmt.Errorf("render refused readiness: %w", marshalErr)
		}
		cause := fmt.Errorf("readiness refused at current mainline: %s", body)
		return out, recordReadinessRefusal(ctx, root, cfg, feature, version, out.Verdict, cause)
	}
	queue, err := events.OpenPending(root, cfg.Remote, feature, version)
	if err != nil {
		return out, fmt.Errorf("open pending draft events: %w", err)
	}
	pending, err := queue.Read(ctx)
	if err != nil {
		return out, fmt.Errorf("read pending draft events: %w", err)
	}
	changes, err := compile.ChangeEvents(result, time.Now())
	if err != nil {
		return out, fmt.Errorf("emit requirement-change events: %w", err)
	}
	result, err = compile.WithEvents(result, append(pending, changes...)...)
	if err != nil {
		return out, fmt.Errorf("retain draft events: %w", err)
	}
	documents, recordPaths, err := writeRecordsWithBinding(tmp, actor, result, sources, binding, version)
	if err != nil {
		return out, fmt.Errorf("compile records: %w", err)
	}
	ledgerPaths, err := captureProblemLedgers(ctx, tmp, repo, rebasedHead, feature, actor)
	if err != nil {
		return out, fmt.Errorf("capture problem ledgers: %w", err)
	}
	recordPaths = append(recordPaths, ledgerPaths...)
	draftPath := filepath.ToSlash(filepath.Join(cfg.Drafts, feature))
	if _, err := run(ctx, tmp, nil, "git", "rm", "-r", "--", draftPath); err != nil {
		return out, fmt.Errorf("remove prepared working files: %w", err)
	}
	if _, err := run(ctx, tmp, nil, "git", append([]string{"add", "--"}, recordPaths...)...); err != nil {
		return out, fmt.Errorf("stage compiled records: %w", err)
	}
	preparerEnv := []string{"ARGUS_STAMP_REBASE=", "ARGUS_DRIVE_SLUG="}
	if _, err := run(ctx, tmp, preparerEnv, "git", "commit", "-m", "docs(draft): prepare lock "+feature, "-m", "Draft-Digest: "+result.Digest); err != nil {
		return out, fmt.Errorf("commit prepared lock: %w", err)
	}
	unfinished := func(ctx context.Context) ([]string, error) {
		return lifecycle.UnfinishedTasks(ctx, root, cfg, feature, func(ctx context.Context, dir string, _ []string, name string, args ...string) ([]byte, error) {
			return run(ctx, dir, agentEnv, name, args...)
		})
	}
	appendOnly := false
	if tasks != nil {
		unfinished = func(ctx context.Context) ([]string, error) {
			return lifecycle.UnfinishedTasksWithTracker(ctx, root, cfg, feature, base, tasks, run)
		}
		if a, ok := tasks.(AppendOnlyTasks); ok {
			appendOnly = a.AppendOnly()
		}
	}
	if local != nil {
		unfinished, appendOnly = local.Unfinished, local.AppendOnly
	}
	if err := rebasePreparedPlan(ctx, tmp, cfg, feature, version, base, run, preparerEnv, unfinished, appendOnly); err != nil {
		var unfit *planUnfitError
		if errors.As(err, &unfit) || errors.Is(err, errAppendOnlyTasks) || errors.Is(err, errFinishedPlan) {
			refused = true
			return out, recordStepFailure(ctx, root, cfg, feature, version, result.Digest, "prepare-lock", err)
		}
		return out, err
	}
	out.Head, err = command(ctx, run, tmp, nil, "git", "rev-parse", "HEAD")
	if err != nil {
		return out, fmt.Errorf("resolve prepared head: %w", err)
	}
	other, err := otherChanges(ctx, tmp, base, out.Head, feature, cfg.Plans, run)
	if err != nil {
		return out, err
	}
	if err := keepsLivePlan(other); err != nil {
		refused = true
		return out, recordStepFailure(ctx, root, cfg, feature, version, result.Digest, "prepare-lock", err)
	}
	if local != nil {
		if err := local.Memory(ctx, tmp, base); err != nil {
			return out, fmt.Errorf("memory gate at prepared tree: %w", err)
		}
		out.Request, err = localRequestWith(result, sources, out.Verdict, out.Head, documents, other)
		if err != nil {
			return out, err
		}
		request, requestErr := codehost.NewDraftReviewRequest(result.Digest)
		if requestErr != nil {
			return out, requestErr
		}
		p := generation.Proposal
		published, publishErr := local.Proposals.Publish(ctx, codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch,
			Head: out.Head, State: "prepared", ExpectedProposalOID: generation.OID, ExpectedBranchOID: remoteOID,
			ExpectedTargetOID: base, ReviewRequest: &request})
		if publishErr != nil {
			return out, fmt.Errorf("publish prepared local draft: %w", publishErr)
		}
		out.Proposal = &published
		if err := alignPreparedLocal(ctx, root, published, oldHead, branch, run, *local); err != nil {
			return out, err
		}
		return out, nil
	}
	if _, err := run(ctx, tmp, nil, "go", "run", "./cmd/memory-gate", "--base", base); err != nil {
		return out, fmt.Errorf("memory gate at prepared tree: %w", err)
	}
	repoURL, err := command(ctx, run, tmp, agentEnv, "gh", "repo", "view", "--json", "url", "--jq", ".url")
	if err != nil {
		return out, fmt.Errorf("resolve code-host repository: %w", err)
	}
	pushURL, err := pushURL(repoURL)
	if err != nil {
		return out, err
	}
	body, err := requestWith(result, sources, out.Verdict, repoURL, out.Head, documents, other)
	if err != nil {
		return out, fmt.Errorf("render approval request: %w", err)
	}
	var unreviewed error // the pull request opened, but its reviewer was not requested
	if remoteOID == "" {
		// First publication: the prepared head is the branch's only
		// published state, and its pull request opens with the request.
		out.PullRequest, err = lifecycle.PublishNew(ctx, run, tmp, agentEnv, pushURL, branch, out.Head, cfg.Branch, fmt.Sprintf("%s — contract v%d", feature, version), body, cfg.Approval.OperatorLogin)
		if err != nil && !errors.Is(err, lifecycle.ErrReviewNotRequested) {
			return out, fmt.Errorf("publish prepared lock: %w", err)
		}
		unreviewed = err
	} else {
		number, err := command(ctx, run, tmp, agentEnv, "gh", "pr", "view", branch, "--json", "number", "--jq", ".number")
		if err != nil {
			return out, fmt.Errorf("find draft pull request: %w", err)
		}
		out.PullRequest, err = strconv.Atoi(number)
		if err != nil || out.PullRequest < 1 {
			return out, fmt.Errorf("invalid draft pull request number %q", number)
		}
		if _, err := run(ctx, tmp, agentEnv, "git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "--force-with-lease=refs/heads/"+branch+":"+remoteOID, pushURL, out.Head+":refs/heads/"+branch); err != nil {
			return out, fmt.Errorf("push prepared lock %s: %w", branch, err)
		}
		pr := strconv.Itoa(out.PullRequest)
		if _, err := run(ctx, tmp, agentEnv, "gh", "pr", "edit", pr, "--body", body); err != nil {
			return out, fmt.Errorf("publish approval request for PR #%s: %w", pr, err)
		}
		if _, err := run(ctx, tmp, agentEnv, "gh", "pr", "ready", pr); err != nil {
			return out, fmt.Errorf("mark prepared PR #%s ready for review: %w", pr, err)
		}
		if operator := cfg.Approval.OperatorLogin; operator != "" {
			if _, err := run(ctx, tmp, agentEnv, "gh", "pr", "edit", pr, "--add-reviewer", operator); err != nil {
				return out, fmt.Errorf("request operator review for PR #%s: %w", pr, err)
			}
		}
	}
	if _, err := run(ctx, root, nil, "git", "reset", "--hard", out.Head); err != nil {
		return out, fmt.Errorf("align prepared working tree: %w", err)
	}
	if unreviewed != nil {
		return out, fmt.Errorf("publish prepared lock: %w", unreviewed)
	}
	return out, nil
}

func verifySourceStamps(ctx context.Context, root, base, head string, run lifecycle.RunCommand) error {
	fork, err := command(ctx, run, root, nil, "git", "merge-base", base, head)
	if err != nil {
		return fmt.Errorf("find draft fork: %w", err)
	}
	data, err := run(ctx, root, nil, "git", "log", "--format=%H%x1f%T%x1f%P%x1f%B%x1e", fork+".."+head)
	if err != nil {
		return fmt.Errorf("read source commits: %w", err)
	}
	for rec := range strings.SplitSeq(string(data), "\x1e") {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		fields := strings.SplitN(rec, "\x1f", 4)
		if len(fields) != 4 {
			return fmt.Errorf("malformed source commit metadata")
		}
		parents := strings.Fields(fields[2])
		if len(parents) != 1 {
			return fmt.Errorf("commit %s: draft rebase requires a single-parent source commit (got %d parents)", fields[0], len(parents))
		}
		if err := attribution.VerifyRebaseSource(fields[3], fields[1], parents); err != nil {
			return fmt.Errorf("commit %s: %w", fields[0], err)
		}
	}
	return nil
}

// A linked worktree does not inherit ignored cache files. Share the original
// release cache so its installed hook can execute the pinned stamper offline.
// Keep bin itself a directory: the installed .pyxis/bin/ ignore does not
// match a symlink at that path, and the memory gate would correctly refuse it.
func sharePreparationCache(root, tmp string) error {
	source, err := filepath.Abs(filepath.Join(root, ".pyxis", "bin"))
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", source)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	destination := filepath.Join(tmp, ".pyxis", "bin")
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("preparation cache path %s already exists", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Symlink(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return fmt.Errorf("share preparation cache entry %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func recordPrepareFailure(ctx context.Context, root string, cfg config.Config, feature string, version int, digest string, cause error) error {
	return recordStepFailure(ctx, root, cfg, feature, version, digest, "prepare-lock", cause)
}

func recordStepFailure(ctx context.Context, root string, cfg config.Config, feature string, version int, digest, step string, cause error) error {
	label := step
	if step == "prepare-lock" {
		label = "preparation"
	}
	queue, err := events.OpenPending(root, cfg.Remote, feature, version)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("queue %s failure: %w", label, err))
	}
	event, err := events.New(events.DraftStepFailed, feature, digest, time.Now())
	if err != nil {
		return errors.Join(cause, err)
	}
	event.Step, event.ErrorClass = step, "step-error"
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := queue.Add(writeCtx, event); err != nil {
		return errors.Join(cause, fmt.Errorf("queue %s failure: %w", label, err))
	}
	return cause
}

func recordReadinessRefusal(ctx context.Context, root string, cfg config.Config, feature string, version int, verdict readiness.Verdict, cause error) error {
	queue, err := events.OpenPending(root, cfg.Remote, feature, version)
	if err != nil {
		return errors.Join(cause, err)
	}
	event, err := events.New(events.LockRefused, feature, verdict.DraftDigest, time.Now())
	if err != nil {
		return errors.Join(cause, err)
	}
	for _, unmet := range verdict.Unmet {
		event.UnmetConditions = append(event.UnmetConditions, unmet.Condition+": "+unmet.Item+" — "+unmet.Remedy)
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := queue.Add(writeCtx, event); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func loadBindingProjectionWithReader(ctx context.Context, root, remote, feature string, version int, run lifecycle.RunCommand, repo *gitsrc.Repo, reader lifecycle.LockedReader) (*resolve.Projection, lifecycle.LockedContract, error) {
	locked, err := reader(ctx, root, remote, feature, version, run)
	if err != nil {
		return nil, locked, err
	}
	ref := fmt.Sprintf("refs/tags/lock/%s/v%d", feature, version)
	if locked.SourceRef != "" {
		ref = locked.SourceRef
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: ref, Prefix: store.Dir})
	if err != nil {
		return nil, locked, fmt.Errorf("load memory at %s: %w", ref, err)
	}
	if len(set.Problems) > 0 || len(set.DocumentIssues) > 0 || len(set.Dangling) > 0 {
		return nil, locked, fmt.Errorf("binding lock %s has unresolved memory defects", ref)
	}
	binding := resolve.New(set)
	if len(binding.Violations) > 0 || len(binding.Conflicts()) > 0 {
		return nil, locked, fmt.Errorf("binding lock %s has unresolved memory conflicts", ref)
	}
	return binding, locked, nil
}

func command(ctx context.Context, run lifecycle.RunCommand, dir string, env []string, name string, args ...string) (string, error) {
	out, err := run(ctx, dir, env, name, args...)
	return strings.TrimSpace(string(out)), err
}

func pushURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("code-host repository URL %q must be HTTPS without credentials", raw)
	}
	return strings.TrimSuffix(raw, ".git") + ".git", nil
}
