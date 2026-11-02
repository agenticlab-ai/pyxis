package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/trackeridentity"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

type issueRun struct {
	contract  issueentry.Contract
	task      port.Task
	store     *issueentry.Store
	collector issueentry.Collector
	publisher issueentry.SummaryPublisher
	reporter  *report.Reporter
	started   time.Time
	now       func() time.Time
	mu        sync.Mutex
	work      port.Workdir
	// repo is the operated checkout; described is the revision whose
	// implementer description this command retained for recovery; phase is
	// the last command phase entered; closing is the reported terminal.
	repo      string
	described string
	phase     string
	active    bool
	closing   *issueTerminal
}

func issuePreflight(cfg config.Config, repo string) error {
	if !cfg.ReviewConfigured() {
		return errors.New("issue entry requires configured independent executor.review")
	}
	if _, activated := cfg.RoutingEvidenceActivation(); activated {
		return errors.New("unsupported_issue_origin: activated routing-evidence policy does not admit direct issue sources")
	}
	gate, err := codegatemirror.Load(repo)
	if err != nil {
		return err
	}
	for _, check := range gate.Checks {
		if check.Class == codegatemirror.ClassRequired {
			return nil
		}
	}
	return errors.New("issue entry requires at least one required code-gate check")
}

func inspectIssue(ctx context.Context, tracker port.WorkTracker, binding integrations.ResolvedRepository, number int, repo, remote string, out io.Writer) error {
	reader, ok := tracker.(port.IssueReader)
	if !ok {
		return errors.New("configured tracker has no exact issue reader")
	}
	snapshot, err := reader.ReadIssue(ctx, number)
	if err != nil {
		return err
	}
	inspection, err := issueentry.Inspect(string(binding.Provider), binding.Key(), snapshot)
	if err != nil {
		return err
	}
	store, err := issueentry.NewStore(repo, remote)
	if err != nil {
		return err
	}
	if c, ok, err := store.Load(ctx, string(binding.Provider), binding.Key(), number); err != nil {
		return err
	} else if ok {
		inspection.Admitted = &c
	}
	return json.NewEncoder(out).Encode(inspection)
}

func admitIssue(ctx context.Context, tracker port.WorkTracker, binding integrations.ResolvedRepository, number int, inputFile, repo, remote string, started time.Time, now func() time.Time) (*issueRun, port.WorkTracker, error) {
	return admitSelectedIssue(ctx, tracker, binding, number, inputFile, repo, remote, started, now, false, config.Defaults().CISelfHealAttempts())
}

// admitSelectedIssue admits the selected issue or resumes its admitted
// contract. With resumeOnly it never admits: checkpoint recovery acts only on
// a contract an earlier invocation admitted, from the unchanged input.
func admitSelectedIssue(ctx context.Context, tracker port.WorkTracker, binding integrations.ResolvedRepository, number int, inputFile, repo, remote string, started time.Time, now func() time.Time, resumeOnly bool, remoteRepairCap int) (*issueRun, port.WorkTracker, error) {
	reader, ok := tracker.(port.IssueReader)
	if !ok {
		return nil, nil, errors.New("configured tracker has no exact issue reader")
	}
	f, err := os.Open(inputFile)
	if err != nil {
		return nil, nil, err
	}
	// Bound the input before allocating it, including an extra byte for refusal.
	b, readErr := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	closeErr := f.Close()
	if readErr != nil {
		return nil, nil, readErr
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	input, err := issueentry.Decode(b)
	if err != nil {
		return nil, nil, err
	}
	store, err := issueentry.NewStore(repo, remote)
	if err != nil {
		return nil, nil, err
	}
	c, admitted, err := store.Load(ctx, string(binding.Provider), binding.Key(), number)
	if err != nil {
		return nil, nil, err
	}
	if resumeOnly && !admitted {
		return nil, nil, fmt.Errorf("checkpoint recovery requires an admitted contract for issue #%d, and none exists", number)
	}
	associated := false
	if admitted {
		_, associated, err = store.Proposal(ctx, c)
		if err != nil {
			return nil, nil, err
		}
	}
	var task port.Task
	if admitted {
		// Reconcile an already-created native proposal before deciding whether
		// fresh source validation is needed. ScopedTracker still validates the
		// live issue immediately before any unpublished implementation claim.
		c, task, err = issueentry.Resume(input, c, binding.Key())
	} else {
		var snapshot port.IssueSnapshot
		snapshot, err = reader.ReadIssue(ctx, number)
		if err == nil {
			c, task, err = issueentry.Prepare(input, string(binding.Provider), binding.Key(), number, snapshot)
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if binding.Provider == integrations.Carina {
		task.SourceRef = "carina:" + strings.TrimPrefix(input.Source.BindingDigest, "sha256:") + fmt.Sprintf("#%d", number)
	} else {
		task.SourceRef = trackeridentity.SourceRef(binding, number)
	}
	if !admitted {
		err = store.AdmitWithRemoteRepairs(ctx, c, remoteRepairCap)
	} else {
		err = store.Admit(ctx, c)
	}
	if err != nil {
		return nil, nil, err
	}
	session := &issueRun{contract: c, task: task, store: store, started: started, now: now, repo: repo}
	return session, &issueentry.ScopedTracker{Reader: reader, Tracker: tracker, Contract: c, Task: task, Provider: string(binding.Provider), Key: binding.Key(), Associated: associated}, nil
}

func (r *issueRun) scope() string { return "issue/" + strings.TrimPrefix(r.contract.ID, "sha256:") }
func (r *issueRun) summary(p port.Proposal, revision, terminal string) issueentry.Summary {
	return r.collector.Summary(r.contract, r.task.SourceRef, p.Number, revision, terminal, r.now().Sub(r.started).Milliseconds())
}
func (r *issueRun) publish(ctx context.Context, p port.Proposal, revision, terminal string) error {
	retained, err := r.store.RetainSummary(ctx, r.contract, r.summary(p, revision, terminal))
	if err != nil {
		return err
	}
	return r.publisher.PublishIssueSummary(ctx, p, retained)
}

// issueHost retains only the selected worktree. The worker, drive-to-green,
// native publisher, and authenticated review broker retain their ownership.
type issueHost struct {
	executionHost
	session *issueRun
}

func (h *issueHost) OpenProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (port.Proposal, error) {
	if task.ID != h.session.task.ID {
		return port.Proposal{}, errors.New("unselected issue publication")
	}
	gate := &localVerifiedHost{run: codeGateRunner, git: localGit}
	revision, err := gate.verifyLocal(ctx, work)
	if err != nil {
		return port.Proposal{}, err
	}
	h.session.mu.Lock()
	h.session.work = work
	h.session.mu.Unlock()
	p, err := h.executionHost.OpenProposal(ctx, task, work, description)
	if err != nil {
		return p, err
	}
	// One immutable record retains the association and the already verified
	// revision, so a later head read or gate failure cannot strand recovery.
	if err := h.session.store.AssociateVerified(ctx, h.session.contract, p, revision); err != nil {
		return p, err
	}
	// Retain implementation observations before review so a restarted command
	// keeps them. A provisional public write must not interrupt authenticated
	// review handoff; finish enforces successful final summary publication.
	provisional, err := h.session.store.RetainSummary(ctx, h.session.contract, h.session.summary(p, "", "running"))
	if err != nil {
		return p, fmt.Errorf("retain issue observations for PR #%d: %w", p.Number, err)
	}
	if err := h.session.publisher.PublishIssueSummary(ctx, p, provisional); err != nil {
		slog.WarnContext(ctx, "provisional issue summary unavailable; final publication will retry", "proposal", p.Number, "error", err)
	}
	return p, nil
}

func (r *issueRun) recordLocal(ctx context.Context, host executionHost, p port.Proposal) error {
	r.mu.Lock()
	work := r.work
	r.mu.Unlock()
	if work == nil {
		return nil
	} // A restart exact-reads the previously retained proof.
	gate := &localVerifiedHost{run: codeGateRunner, git: localGit}
	sha, err := gate.verifyLocal(ctx, work)
	if err != nil {
		return err
	}
	published, err := host.ProposalHead(ctx, p)
	if err != nil {
		return err
	}
	if published != sha {
		return errors.New("issue local verification does not cover the published revision")
	}
	return r.store.RecordLocal(ctx, r.contract, p, sha)
}

func (r *issueRun) states(ctx context.Context, store reviewobligation.Store, repository string) ([]reviewobligation.State, error) {
	states, err := store.Open(ctx, r.scope())
	if err != nil {
		return nil, err
	}
	p, ok, err := r.store.Proposal(ctx, r.contract)
	if err != nil {
		return nil, err
	}
	if !ok {
		if len(states) != 0 {
			return nil, errors.New("unassociated issue scope contains review obligations")
		}
		return nil, nil
	}
	ref, err := persist.ProposalRef(repository, p.Number)
	if err != nil {
		return nil, err
	}
	for _, state := range states {
		o := state.Obligation
		if o.Repository != repository || o.TaskID != r.task.ID || o.TaskNumber != r.task.Number || o.DemandID != r.task.DemandID || o.ProposalRef != ref {
			return nil, errors.New("issue review scope contains a conflicting obligation")
		}
	}
	return states, nil
}

func reviewStatesAtRevision(states []reviewobligation.State, revision string) []reviewobligation.State {
	current := make([]reviewobligation.State, 0, len(states))
	for _, state := range states {
		if state.Obligation.Revision == revision {
			current = append(current, state)
		}
	}
	return current
}

func (r *issueRun) currentReviewStates(ctx context.Context, host executionHost, store reviewobligation.Store, repository string, p port.Proposal) ([]reviewobligation.State, error) {
	revision, err := host.ProposalHead(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("read current issue review head: %w", err)
	}
	if !issueentry.ValidRevision(revision) {
		return nil, errors.New("current issue review head is not a full commit identity")
	}
	states, err := r.states(ctx, store, repository)
	if err != nil {
		return nil, err
	}
	return reviewStatesAtRevision(states, revision), nil
}

// Filtering the shared metadata label alone is insufficient: another entry
// may have used that label. Validate the exact admitted task/proposal before
// the existing repairer reads artifacts or constructs a pending frontier.
type issueReviewStore struct {
	reviewobligation.Store
	session    *issueRun
	repository string
}

func (s issueReviewStore) Open(ctx context.Context, scope string) ([]reviewobligation.State, error) {
	if scope != s.session.scope() {
		return nil, errors.New("unselected issue review scope")
	}
	return s.session.states(ctx, s.Store, s.repository)
}

// completion checks the same exact revision after reading all three facts.
// The canonical review reducer accepts only the authenticated publication pair.
func (r *issueRun) completion(ctx context.Context, host executionHost, store reviewobligation.Store, repository string, p port.Proposal) (string, string, error) {
	landing, sha, checks, err := host.InspectProposal(ctx, p)
	if err != nil {
		return "", "verification_pending", err
	}
	if landing != port.LandingOpen || checks != port.CheckGreen {
		return sha, "verification_pending", fmt.Errorf("proposal landing=%s checks=%s", landing, checks)
	}
	local, err := r.store.LocalVerified(ctx, r.contract, p, sha)
	if err != nil {
		return sha, "verification_pending", err
	}
	if !local {
		return sha, "verification_pending", errors.New("no retained local verification for current proposal revision")
	}
	states, err := r.states(ctx, store, repository)
	if err != nil {
		return sha, "review_pending", err
	}
	states = reviewStatesAtRevision(states, sha)
	matches := 0
	for _, state := range states {
		if state.Status == "verified" && state.Pair != nil && state.Pair.Revision == sha {
			matches++
		}
	}
	if matches != 1 || len(states) != 1 {
		return sha, "review_pending", errors.New("one authenticated independent review is required for current revision")
	}
	afterLanding, after, afterChecks, err := host.InspectProposal(ctx, p)
	if err != nil {
		return sha, "verification_pending", err
	}
	if afterLanding != port.LandingOpen || after != sha || afterChecks != port.CheckGreen {
		return after, "verification_pending", errors.New("proposal or checks changed during terminal verification")
	}
	return sha, "verified_pr", nil
}

func (r *issueRun) finish(ctx context.Context, host executionHost, reviews reviewobligation.Store, repository string, stdout, stderr io.Writer) int {
	verificationFinished := r.logPhase("final_verification")
	p, ok, err := r.store.Proposal(ctx, r.contract)
	if err != nil || !ok {
		if err == nil {
			err = errors.New("no associated proposal")
		}
		verificationFinished(err)
		_, _ = fmt.Fprintf(stderr, "implement-now: selected issue has no retained proposal: %v\n", err)
		r.closeUnpublished(ctx, stderr)
		return 1
	}
	localErr := r.recordLocal(ctx, host, p)
	revision, terminal, verifyErr := r.completion(ctx, host, reviews, repository, p)
	if localErr != nil {
		terminal = "verification_pending"
		verifyErr = localErr
	}
	verificationFinished(verifyErr)
	publicationFinished := r.logPhase("summary_publication")
	publicationErr := r.publish(ctx, p, revision, terminal)
	publicationFinished(publicationErr)
	if err := publicationErr; err != nil {
		_, _ = fmt.Fprintf(stderr, "implement-now: summary_publication_failed for PR #%d contract %s: %v\n", p.Number, r.contract.ID, err)
		r.close(stderr, issueTerminal{result: "summary_publication_failed", phase: "summary_publication", checkpoint: revision, proposal: p.Number,
			active: r.active, next: fmt.Sprintf("rerun the issue command without --recover-checkpoint to republish PR #%d's execution summary", p.Number)})
		return 1
	}
	if verifyErr != nil {
		_, _ = fmt.Fprintf(stderr, "implement-now: %s PR #%d revision %s: %v\n", terminal, p.Number, revision, verifyErr)
		closing := issueTerminal{result: terminal, phase: "verification", checkpoint: revision, proposal: p.Number, active: r.active,
			next: fmt.Sprintf("rerun the issue command without --recover-checkpoint; it will settle PR #%d's pending CI, use any retained repair authority for red CI, then run independent review", p.Number)}
		if terminal == "review_pending" {
			closing.phase, closing.next = "review", fmt.Sprintf("rerun the issue command without --recover-checkpoint to run PR #%d's pending independent review", p.Number)
		}
		r.close(stderr, closing)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "implement-now: verified_pr #%d revision %s contract %s\n", p.Number, revision, r.contract.ID)
	r.close(stdout, issueTerminal{result: "verified_pr", phase: "complete", checkpoint: revision, proposal: p.Number,
		next: fmt.Sprintf("human review and merge of PR #%d; the executor never merges", p.Number)})
	return 0
}
