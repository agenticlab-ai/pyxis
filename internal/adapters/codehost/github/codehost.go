package ghcodehost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
)

// DefaultBaseBranch is the mainline a proposal targets when none is configured.
// It resolves from the repo config schema's single source (config.Defaults —
// "main" on a default config, the ONE place that literal lives, DINV-1) rather
// than carrying an independent literal, so a repo whose default branch is not
// "main" is honored wherever the base is left blank (config-driven-portability-v1
// §3.2, FR-2). The production caller (cmd/implement-now) resolves the configured
// branch and passes it explicitly; this fallback serves a direct-construction
// caller (the hermetic tests) that leaves the base blank. Proposals are opened
// against freshly-fetched mainline; a stacked base (an unmerged blocker's branch)
// is never used (DINV-2, AP-26).
var DefaultBaseBranch = config.Defaults().Branch

// client is the narrow, consumer-side GitHub surface the code-host adapter
// needs — one method per exact `gh api` call. The production implementation
// shells `gh` (client.go); tests inject a fake with recorded responses so the
// adapter logic is hermetic (AP-1, the hexagon; §3.10).
type client interface {
	// findOpenPRByHead returns the number of an open proposal already opened from
	// head, if any — the idempotency read that prevents a duplicate proposal on a
	// re-invocation (prd:implement-now/FR-14, AP-22).
	findOpenPRByHead(ctx context.Context, head string) (number int, found bool, err error)
	// compareAhead returns how many commits head is ahead of base, by an exact,
	// fresh compare — the nothing-to-propose signal (0 ⇒ hollow, DINV-3).
	compareAhead(ctx context.Context, base, head string) (aheadBy int, err error)
	// createPR opens a draft proposal from head into base and returns its number.
	createPR(ctx context.Context, head, base, title, body string) (number int, err error)
	// pullHeadSHA returns the current head commit SHA of a proposal, freshly.
	pullHeadSHA(ctx context.Context, number int) (sha string, err error)
	// pullSnapshot returns the proposal lifecycle and exact current head used by
	// the drive-to-green movement guard.
	pullSnapshot(ctx context.Context, number int) (pullSnapshot, error)
	// checkRuns reads the check runs on a commit through the Checks API — the
	// exact, fresh required-check states, never a status rollup (AP-13, AP-15).
	checkRuns(ctx context.Context, sha string) (runs []checkRun, err error)
	// createComment posts one conversation comment on a proposal (GitHub models
	// a PR's conversation comments as issue comments) and returns its id — the
	// write the pre-publish review artifact lands through.
	createComment(ctx context.Context, number int, body string) (id string, err error)
	// pullBody reads a proposal's body by an exact, fresh read.
	pullBody(ctx context.Context, number int) (string, error)
	// setPullBody replaces a proposal's body.
	setPullBody(ctx context.Context, number int, body string) error
}

// checkRun is the adapter's view of one Checks-API check run.
type checkRun struct {
	Name       string
	Status     string // "queued" | "in_progress" | "completed" | …
	Conclusion string // set when completed: "success" | "failure" | "neutral" | "skipped" | …
}

// Host is the GitHub-backed CodeHost for one repository. It holds no cache:
// every Checks call re-reads the live head SHA and its check runs (AP-15).
type Host struct {
	gh   client
	base string // the mainline branch a proposal targets
	repo string // selected repository; canonical URLs must match before closing
}

// compile-time proof the adapter satisfies the ports.
var (
	_ port.CodeHost      = (*Host)(nil)
	_ port.ProposalProbe = (*Host)(nil)
)

// NewGH builds a production Host over a real gh-shelling client. repo is the
// "owner/name" slug; base is the mainline branch a proposal targets (empty ⇒
// DefaultBaseBranch).
func NewGH(repo, base string) *Host {
	return newHost(newGHClient(repo), base)
}

// NewGHWithBudget is the injection seam for callers that own the state
// directory and event sink. NewGH remains the production convenience form.
func NewGHWithBudget(repo, base string, b *budget.Budget) *Host {
	return newHost(newGHClientWithBudget(repo, b), base)
}

// newHost is the injection seam shared by NewGH and the hermetic tests.
func newHost(gh client, base string) *Host {
	if base == "" {
		base = DefaultBaseBranch
	}
	h := &Host{gh: gh, base: base}
	if native, ok := gh.(*ghClient); ok {
		h.repo = native.repo
	}
	return h
}

// OpenProposal opens a proposal from the task's head branch (the isolated
// worktree branch, off freshly-fetched mainline — DINV-2). It is idempotent and
// verify-by-artifact honest (DINV-3):
//
//   - an existing open proposal for the head is returned as-is (no duplicate —
//     prd:implement-now/FR-14, AP-22);
//   - a head with no commits ahead of mainline produced nothing to propose, so
//     it returns port.ErrNoProposal (the Worker maps it to hollow — DINV-3);
//   - otherwise a fresh draft proposal is opened and its number returned.
//
// The executor never merges (DINV-6): the proposal is opened as a draft and
// left for merge-lane / a human to land.
func (h *Host) OpenProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (port.Proposal, error) {
	head, err := proposalHead(task, work)
	if err != nil {
		return port.Proposal{}, err
	}

	// Idempotency first: a re-invocation over an already-opened head returns the
	// existing proposal, never a duplicate (AP-22).
	if number, found, err := h.gh.findOpenPRByHead(ctx, head); err != nil {
		return port.Proposal{}, fmt.Errorf("ghcodehost: look up existing proposal for %q: %w", head, err)
	} else if found {
		return port.Proposal{Number: number, Branch: head}, nil
	}

	// Nothing-to-propose detection: an exact, fresh compare against mainline. A
	// clean run that changed nothing leaves the head 0 commits ahead — hollow,
	// never a proposal (DINV-3).
	ahead, err := h.gh.compareAhead(ctx, h.base, head)
	if err != nil {
		if isNotFound(err) {
			// The head branch is absent on the remote (nothing was published for it),
			// so there is nothing to propose — hollow, not a hard failure (DINV-3,
			// #397). In the Model-A flow the Worker publishes the branch before this
			// call, so this is a defense-in-depth guard: a raw compare-404 must never
			// reach the report as an operational error.
			return port.Proposal{}, port.ErrNoProposal
		}
		return port.Proposal{}, fmt.Errorf("ghcodehost: compare %s...%s: %w", h.base, head, err)
	}
	if ahead == 0 {
		return port.Proposal{}, port.ErrNoProposal
	}

	number, err := h.gh.createPR(ctx, head, h.base, proposalTitle(task), proposalBody(task, h.repo, description))
	if err != nil {
		return port.Proposal{}, fmt.Errorf("ghcodehost: open proposal from %q: %w", head, err)
	}
	return port.Proposal{Number: number, Branch: head}, nil
}

// DescribeProposal replaces the description of the proposal open from the
// task's head, keeping the rest of its body: the task association, the
// execution summary, and anything another writer appended.
func (h *Host) DescribeProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (bool, error) {
	head, err := proposalHead(task, work)
	if err != nil {
		return false, err
	}
	number, found, err := h.gh.findOpenPRByHead(ctx, head)
	if err != nil {
		return false, fmt.Errorf("ghcodehost: look up the proposal to describe for %q: %w", head, err)
	}
	if !found {
		return false, nil
	}
	rendered := describeTask(task, h.repo, description).Render()
	read := func(ctx context.Context) (string, error) { return h.gh.pullBody(ctx, number) }
	write := func(ctx context.Context, body string) error { return h.gh.setPullBody(ctx, number, body) }
	if err := prdescription.Publish(ctx, rendered, read, write); err != nil {
		return true, fmt.Errorf("ghcodehost: describe proposal #%d: %w", number, err)
	}
	return true, nil
}

// proposalHead is the branch a task's proposal is opened from. The Task's head
// branch and the Workdir's branch denote the same isolated head
// (port.Task.HeadBranch, port.Workdir.Branch); the workdir's is the fallback
// when the charter left the field empty.
func proposalHead(task port.Task, work port.Workdir) (string, error) {
	head := task.HeadBranch
	if head == "" && work != nil {
		head = work.Branch()
	}
	if head == "" {
		return "", fmt.Errorf("ghcodehost: task %q has no head branch to open a proposal from", task.ID)
	}
	return head, nil
}

// ProposalHead reads the proposal's current head commit SHA by an exact, fresh
// read (worker.ProposalHeadReader): the Worker compares it with the revision
// its pre-publish review accepted to decide whether that review covers what
// was actually published (AP-13 — a review binds one exact revision).
func (h *Host) ProposalHead(ctx context.Context, proposal port.Proposal) (string, error) {
	sha, err := h.gh.pullHeadSHA(ctx, proposal.Number)
	if err != nil {
		return "", fmt.Errorf("ghcodehost: read head SHA for proposal #%d: %w", proposal.Number, err)
	}
	if strings.TrimSpace(sha) == "" {
		return "", fmt.Errorf("ghcodehost: proposal #%d reports no head SHA", proposal.Number)
	}
	return sha, nil
}

// PublishReview lands the pre-publish review artifact (the composed human
// rendering plus machine payload) on the proposal as one conversation comment
// (worker.ReviewPublisher) and returns its medium reference. It is a content
// write, metered through the shared budget like a proposal open.
func (h *Host) PublishReview(ctx context.Context, proposal port.Proposal, artifact string) (string, error) {
	if strings.TrimSpace(artifact) == "" {
		return "", fmt.Errorf("ghcodehost: publish review on proposal #%d: empty artifact", proposal.Number)
	}
	id, err := h.gh.createComment(ctx, proposal.Number, artifact)
	if err != nil {
		return "", fmt.Errorf("ghcodehost: publish review on proposal #%d: %w", proposal.Number, err)
	}
	return "github-comment:" + id, nil
}

// Checks reads the aggregate required-check state of a proposal through an
// exact, fresh read: the proposal's current head SHA, then that commit's check
// runs and commit statuses (never a status rollup — AP-13). Each run is
// translated to a per-context state by checkState — the same derivation
// StateReader.States uses (state_reader.go) — then reduced fail-closed by
// reduceChecks: one implementation of "is this check green" for this adapter
// (AP-11, docs/tech-design/state-reader-v2.md §3.1's delegation decision).
func (h *Host) Checks(ctx context.Context, proposal port.Proposal) (port.CheckState, error) {
	sha, err := h.gh.pullHeadSHA(ctx, proposal.Number)
	if err != nil {
		return port.CheckPending, fmt.Errorf("ghcodehost: read head SHA for proposal #%d: %w", proposal.Number, err)
	}
	state, err := h.checksAt(ctx, sha)
	if err != nil {
		return port.CheckPending, fmt.Errorf("ghcodehost: read check runs for %s: %w", sha, err)
	}
	return state, nil
}

// OpenProposals is the pre-claim duplicate-dispatch probe (port.ProposalProbe):
// for each candidate task, is a proposal already open on the branch a Worker
// would open one from?
//
// It reuses findOpenPRByHead — the exact read OpenProposal itself uses for
// idempotency — so the probe and the opener can never disagree about what "a
// proposal already exists for this head" means. It reads nothing else: check
// state is deliberately not consulted, because existence alone decides
// (port.OpenProposal). A task with no head branch is reported as unproposed:
// the probe withholds on positive evidence, and an underived branch is none
// (the Dispatcher fills the branch before probing, so this is defensive).
func (h *Host) OpenProposals(ctx context.Context, tasks []port.Task) (map[string]port.OpenProposal, error) {
	out := make(map[string]port.OpenProposal, len(tasks))
	for _, task := range tasks {
		head := strings.TrimSpace(task.HeadBranch)
		if head == "" {
			continue
		}
		number, found, err := h.gh.findOpenPRByHead(ctx, head)
		if err != nil {
			return nil, fmt.Errorf("ghcodehost: look up open proposal for %q: %w", head, err)
		}
		if !found {
			continue
		}
		out[task.ID] = port.OpenProposal{Number: number}
	}
	return out, nil
}

// InspectProposal joins a proposal's landing state, exact current head, and
// aggregate required-check state for drive-to-green's pre-write guard. The pull
// and check-run reads are fresh and the latter is pinned to the head returned by
// the former. A branch mismatch is a typed adapter error, never a healthy-looking
// observation of a different proposal head.
func (h *Host) InspectProposal(ctx context.Context, proposal port.Proposal) (port.Landing, string, port.CheckState, error) {
	snapshot, err := h.gh.pullSnapshot(ctx, proposal.Number)
	if err != nil {
		return port.LandingUnknown, "", port.CheckPending, fmt.Errorf("ghcodehost: inspect proposal #%d: %w", proposal.Number, err)
	}
	if proposal.Branch != "" && snapshot.HeadRef != "" && proposal.Branch != snapshot.HeadRef {
		return port.LandingUnknown, snapshot.HeadSHA, port.CheckPending,
			fmt.Errorf("ghcodehost: proposal #%d head branch is %q, want %q", proposal.Number, snapshot.HeadRef, proposal.Branch)
	}
	state, err := h.checksAt(ctx, snapshot.HeadSHA)
	if err != nil {
		return port.LandingUnknown, snapshot.HeadSHA, port.CheckPending, fmt.Errorf("ghcodehost: inspect check runs for %s: %w", snapshot.HeadSHA, err)
	}
	landing := port.LandingOpen
	switch {
	case snapshot.Merged:
		landing = port.LandingLanded
	case strings.EqualFold(snapshot.State, "closed"):
		landing = port.LandingClosedUnlanded
	case !strings.EqualFold(snapshot.State, "open"):
		return port.LandingUnknown, snapshot.HeadSHA, port.CheckPending,
			fmt.Errorf("ghcodehost: proposal #%d has unknown state %q", proposal.Number, snapshot.State)
	}
	return landing, snapshot.HeadSHA, state, nil
}

// isNotFound reports whether err is a gh "not found" (HTTP 404) — the signal a
// referenced ref (here, an unpublished head branch) is absent on the remote. gh
// surfaces it in its stderr text (e.g. "gh: Not Found (HTTP 404)"), so this
// classifies by message, the same shape the tracker adapter uses. It lets
// OpenProposal treat an absent head as nothing-to-propose (hollow) rather than a
// hard error (DINV-3).
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "404") || strings.Contains(msg, "not found")
}

// checkContexts translates the Checks API's raw runs into the port's
// per-context shape via checkState (state_reader.go) — the same translator
// the batched read's decodeProposal uses — so exactly one function decides
// "is this one check green" for this adapter (AP-11). checksAt combines this
// result with the exact commit's latest status for each status context.
func checkContexts(runs []checkRun) []port.CheckContext {
	out := make([]port.CheckContext, len(runs))
	for i, r := range runs {
		out[i] = port.CheckContext{Name: r.Name, State: checkState(r.Status, r.Conclusion, "")}
	}
	return out
}

// reduceChecks reduces per-context check states to one fail-closed verdict
// (DINV-3, DINV-4, AP-13): only when every context is GREEN is the result
// GREEN — the sole state that admits a succeeded outcome. A RED context
// dominates and returns immediately (it matches the code host's own merge
// box, and drives the self-heal loop). Anything still PENDING — or no context
// reported at all — is PENDING, the not-green zero value: a proposal with no
// checks is never vacuously green.
func reduceChecks(checks []port.CheckContext) port.CheckState {
	if len(checks) == 0 {
		return port.CheckPending
	}
	anyPending := false
	for _, c := range checks {
		switch c.State {
		case port.CheckRed:
			return port.CheckRed
		case port.CheckPending:
			anyPending = true
		}
	}
	if anyPending {
		return port.CheckPending
	}
	return port.CheckGreen
}

// proposalTitle preserves the authored title. Task identity belongs in the
// machine association and body, where it cannot invalidate a conventional title.
func proposalTitle(t port.Task) string {
	if title := strings.TrimSpace(t.Title); title != "" {
		prefix, _, hasColon := strings.Cut(title, ": ")
		kind, _, _ := strings.Cut(strings.TrimSuffix(prefix, "!"), "(")
		switch kind {
		case "feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert":
			if hasColon {
				return title
			}
		}
		return "chore: " + title
	}
	if t.ID != "" {
		return fmt.Sprintf("chore: implement task %s", t.ID)
	}
	return "chore: implement planned task"
}

// proposalBody renders a new proposal's body: the task's description, then the
// machine task association. The executor opens and verifies proposals; it
// never merges (prd:implement-now/NR-3).
func proposalBody(t port.Task, repository string, d prdescription.Description) string {
	var b strings.Builder
	b.WriteString(describeTask(t, repository, d).Render())
	b.WriteString("\n")
	if t.SourceKey != "" {
		// The fixed string/integer record is always JSON-marshalable.
		association, err := json.Marshal(struct {
			Schema string `json:"schema"`
			Task   string `json:"task"`
			Source string `json:"source"`
			Number int    `json:"number"`
		}{"pyxis-task-proposal/v1", t.ID, t.SourceRef, t.Number})
		if err == nil {
			b.WriteString("<!-- pyxis-task-proposal:" + base64.RawURLEncoding.EncodeToString(association) + " -->\n")
		}
	}
	return b.String()
}

// describeTask ends the description's What section with trustworthy tracker
// provenance. The WorkTracker adapter supplies a medium-native reference to
// the source item (GitHub "#123", "Carina work-item 2", …); it is used
// verbatim, so a non-GitHub item is never rendered as a misleading "#N" GitHub
// auto-link, and a tracker that set no ref falls back to a bare
// "work-item N". An unambiguous same-repository GitHub issue reference also
// gets GitHub's native closing directive; other references remain neutral.
// Closing an issue when a human merges the proposal does not weaken the
// executor's never-merge boundary (DINV-6).
func describeTask(t port.Task, repository string, d prdescription.Description) prdescription.Description {
	ref := strings.TrimSpace(t.SourceRef)
	if ref == "" && t.Number > 0 {
		ref = fmt.Sprintf("work-item %d", t.Number)
	}
	line := "- Opened by the Pyxis executor, which never merges."
	switch {
	case ref != "" && t.ID != "":
		line = fmt.Sprintf("- Implements task %s (%s), opened by the Pyxis executor, which never merges.", t.ID, ref)
	case ref != "":
		line = fmt.Sprintf("- Implements %s, opened by the Pyxis executor, which never merges.", ref)
	case t.ID != "":
		line = fmt.Sprintf("- Implements task %s, opened by the Pyxis executor, which never merges.", t.ID)
	}
	if closing := githubClosingRef(ref, repository); closing != "" {
		line += "\n- Closes " + closing
	}
	d.What = strings.TrimRight(d.What, "\n") + "\n" + line
	return d
}

// sameRepoGitHubIssueRef accepts only GitHub's unqualified "#<positive-int>"
// form. Qualified refs and prose belong to another repository or tracker and
// must never accidentally close an issue in the proposal repository.
func sameRepoGitHubIssueRef(ref string) bool {
	n, err := strconv.Atoi(strings.TrimPrefix(ref, "#"))
	return err == nil && n > 0 && strings.HasPrefix(ref, "#") && ref == fmt.Sprintf("#%d", n)
}

// githubClosingRef accepts a native short ref, or a canonical URL whose host
// and repository match this adapter. Numbers alone never establish identity.
func githubClosingRef(ref, repository string) string {
	if sameRepoGitHubIssueRef(ref) {
		return ref
	}
	if repository == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return ""
	}
	prefix := "/" + repository + "/issues/"
	if !strings.HasPrefix(u.Path, prefix) {
		return ""
	}
	closing := "#" + strings.TrimPrefix(u.Path, prefix)
	if !sameRepoGitHubIssueRef(closing) {
		return ""
	}
	return closing
}
