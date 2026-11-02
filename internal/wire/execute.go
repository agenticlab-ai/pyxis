package wire

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os/exec"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// EventWiringApplied is EVT-4's event name (prd:installer/§5.7): emitted as
// one structured slog record when a walk finishes, carrying the operations
// list and the partial flag — the ABUSE-3 audit trail's log half.
const EventWiringApplied = "wiring_applied"

// Outcome is the audited result of one declared operation.
type Outcome string

const (
	OutcomeApplied   Outcome = "applied"
	OutcomeSkipped   Outcome = "skipped-with-reason"
	OutcomeHandedOff Outcome = "handed-off"
	OutcomeFailed    Outcome = "failed"
)

// Audit is the EVT-4 record of one Execute walk: exactly the declared
// table's rows, each with what ran and how it ended. The executed set
// never exceeds the declared set by construction — Execute walks the table
// it is handed and constructs no other operation (prd:installer/ABUSE-3).
type Audit struct {
	continuation *continuationState
	Event        string     // EventWiringApplied
	RunID        string     // the run correlation id (installer-v1 §3.8)
	Operations   []OpResult // one row per walked op, in table order
	Partial      bool       // EVT-4 partial: any op handed-off or failed
	HandOff      string     // the copy-paste remainder block ("" when none)
}

// OperationNames renders EVT-4's `operations` field — the walked names in
// table order.
func (a Audit) OperationNames() []string {
	names := make([]string, 0, len(a.Operations))
	for _, r := range a.Operations {
		names = append(names, r.Name)
	}
	return names
}

// OpResult is one audit row. Commands records gh argv or native HTTP method
// and endpoint in invocation order. Credentials travel only through the
// selected child's environment or the native client's authorization header.
type OpResult struct {
	Name       string
	Transport  Transport
	Connection integrations.ResolvedRepository
	Outcome    Outcome
	Reason     string        // why skipped / handed off / failed; detail when applied
	Commands   [][]string    // gh argv or native method/URL, in invocation order
	Refs       []string      // created-resource references (issue URLs), in creation order
	Duration   time.Duration // wall time spent on this op
}

// Sleeper waits between retry attempts, honoring cancellation — injectable
// so retry tests run without real time (the hermetic-clock rule).
type Sleeper func(ctx context.Context, d time.Duration) error

// ctxSleep is the production Sleeper.
func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Bounded transient retry (prd:installer/UB-4 "within a stated bound"):
// maxAttempts total invocations per gh step, exponential backoff from
// retryBaseDelay capped at retryMaxDelay, then the step hands off.
const (
	maxAttempts    = 3
	retryBaseDelay = 500 * time.Millisecond
	retryMaxDelay  = 4 * time.Second
)

// ExecOption injects a seam into Execute — logger, sleeper, clock, run id.
// Defaults are production (slog.Default, real sleep/clock, random id);
// tests inject all four.
type ExecOption func(*executor)

func WithLogger(l *slog.Logger) ExecOption { return func(x *executor) { x.log = l } }
func WithSleeper(s Sleeper) ExecOption     { return func(x *executor) { x.sleep = s } }
func WithNow(now func() time.Time) ExecOption {
	return func(x *executor) { x.now = now }
}
func WithRunID(id string) ExecOption { return func(x *executor) { x.runID = id } }

// WithPriorAudit continues a declared wiring table across installation's
// initial push, preserving handoffs and repository creation state.
func WithPriorAudit(prior Audit) ExecOption { return func(x *executor) { x.prior = &prior } }

type repositoryState struct{ deferred, created bool }
type continuationState struct {
	entries      []handoffEntry
	failed       []string
	repositories map[string]repositoryState
}

type executor struct {
	prior         *Audit
	runner        Runner
	log           *slog.Logger
	sleep         Sleeper
	now           func() time.Time
	runID         string
	forgejoClient func(integrations.ResolvedRepository) (*forgejo.Client, error)
}

// walkState carries the walk's cross-op memory: the hand-off entries in
// table order, the failed-op names, and the create-repo deferral bit.
type walkState struct {
	entries []handoffEntry
	failed  []string
	// repoDeferred: create-repo was declared but did not complete
	// (handed off or failed). Every later gh op depends on the repository
	// existing, so attempting it would only 404 — instead each goes
	// straight to the hand-off block, keeping the block a complete,
	// in-order remedy (create first, then the rest) per UB-4's
	// "hand-off completes the remainder verbatim".
	repoDeferred bool
	// repoCreated: create-repo APPLIED in this walk — the repository is
	// seconds old, so a later read probe's not-found (or an ok probe with
	// an empty body) is GitHub's replication window, not a bad slug
	// (issue #897: `gh label list` returned blank/exit-0 368ms after the
	// create applied, and believing it rolled back a correct run).
	repoCreated bool
}

type handoffEntry struct {
	op   string
	why  string
	cmds [][]string
	raw  string
}

// Execute walks exactly ops — the declared table — through runner. Per op
// it logs before/after, records every gh argv, and classifies failures per
// UB-4: a permission gap hands the op off and the walk continues; a
// transient failure retries within the stated bound then hands off; an
// unclassified failure marks the op failed and the walk still continues.
//
// Pre-action refusals (zero operations performed, AP-17): a gh-transport
// op with no repository slug; a nil Runner with gh ops declared; gh absent
// or unrunnable (GHAbsentError — the UB-7-class named prerequisite
// refusal, probed via `gh --version` before the walk). A table with no
// gh-transport rows never touches the runner.
//
// The returned error is nil when every op applied, skipped, or handed off
// — a hand-off is UB-4's partial result, not an error; the Audit carries
// the copy-paste block. The error is non-nil only for pre-action
// refusals, a transport-level abort (gh vanished mid-walk, context
// cancelled), or ops that FAILED outside UB-4's classes.
func Execute(ctx context.Context, ops []Op, runner Runner, opts ...ExecOption) (Audit, error) {
	x := &executor{runner: runner, log: slog.Default(), sleep: ctxSleep, now: time.Now, forgejoClient: defaultForgejoClient}
	for _, o := range opts {
		o(x)
	}
	if x.runID == "" {
		x.runID = newRunID()
	}

	needGH := false
	checkedForgejo := make(map[string]bool)
	for _, op := range ops {
		if op.Transport == TransportForgejo {
			if err := integrations.ValidateRepository(op.Slug); err != nil {
				return Audit{}, err
			}
			identity := op.Connection.Key() + "\n" + op.Connection.TokenEnv
			if !checkedForgejo[identity] {
				client, err := x.forgejoClient(op.Connection)
				if err != nil {
					return Audit{}, err
				}
				if _, err := client.CurrentUser(ctx); err != nil {
					return Audit{}, fmt.Errorf("forgejo wiring preflight: %w", err)
				}
				checkedForgejo[identity] = true
			}
		}
		if op.Transport != TransportGH {
			continue
		}
		needGH = true
		if op.Slug == "" {
			return Audit{}, fmt.Errorf("wire: operation %q carries no repository slug — Config.Slug (owner/name) is required for GitHub wiring; refusing before any operation (fail closed)", op.Name)
		}
	}
	if needGH {
		if runner == nil {
			return Audit{}, errors.New("wire: nil Runner with gh-transport operations declared — refusing before any operation")
		}
		if err := x.preflight(ctx); err != nil {
			return Audit{}, err
		}
	}

	audit := Audit{Event: EventWiringApplied, RunID: x.runID}
	st := &walkState{}
	// Creation state belongs to a repository, not the whole installation:
	// the task tracker and code host may be on independent servers.
	repositories := make(map[string]repositoryState)
	if x.prior != nil {
		audit.Operations = append([]OpResult(nil), x.prior.Operations...)
		if saved := x.prior.continuation; saved != nil {
			st.entries = append([]handoffEntry(nil), saved.entries...)
			st.failed = append([]string(nil), saved.failed...)
			maps.Copy(repositories, saved.repositories)
		}
	}
	finish := func() {
		audit.continuation = &continuationState{entries: st.entries, failed: st.failed, repositories: repositories}
		for _, r := range audit.Operations {
			if r.Outcome == OutcomeHandedOff || r.Outcome == OutcomeFailed {
				audit.Partial = true
			}
		}
		audit.HandOff = renderHandOff(st.entries)
		x.log.LogAttrs(ctx, slog.LevelInfo, EventWiringApplied,
			slog.String("run_id", audit.RunID),
			slog.Any("operations", audit.OperationNames()),
			slog.Bool("partial", audit.Partial),
			slog.Bool("handoff", audit.HandOff != ""))
	}

	for _, op := range ops {
		identity := op.Connection.Key()
		if op.Connection.Provider == "" {
			identity = "github:" + op.Slug
		}
		saved := repositories[identity]
		st.repoDeferred, st.repoCreated = saved.deferred, saved.created
		x.log.LogAttrs(ctx, slog.LevelInfo, "wire: operation start",
			slog.String("run_id", x.runID),
			slog.String("op", op.Name),
			slog.String("transport", string(op.Transport)),
			slog.String("condition", op.Condition))
		res, err := x.runOp(ctx, op, st)
		repositories[identity] = repositoryState{st.repoDeferred, st.repoCreated}
		audit.Operations = append(audit.Operations, res)
		x.log.LogAttrs(ctx, slog.LevelInfo, "wire: operation done",
			slog.String("run_id", x.runID),
			slog.String("op", op.Name),
			slog.String("outcome", string(res.Outcome)),
			slog.String("reason", res.Reason),
			slog.Int64("duration_ms", res.Duration.Milliseconds()),
			slog.Int("gh_invocations", len(res.Commands)))
		if err != nil {
			// Transport-level abort: gh vanished mid-walk or the context
			// died. The remaining declared ops are not walked — the audit
			// stays a prefix of the table (still never exceeding it).
			finish()
			return audit, fmt.Errorf("wire: walk aborted at %s: %w", op.Name, err)
		}
	}
	finish()
	if len(st.failed) > 0 {
		// The message is user-facing (cmd prints it verbatim on a failed
		// init): name the ops and their reasons, and keep the audit/UB-4
		// taxonomy in the audit itself where it lives.
		return audit, fmt.Errorf("wire: %s", failureSummary(audit, st.failed))
	}
	return audit, nil
}

// failureSummary renders "op failed: reason" for each failed op — the
// walk error a human reads first, so it carries the cause, not just the
// count.
func failureSummary(audit Audit, failed []string) string {
	parts := make([]string, 0, len(failed))
	for _, name := range failed {
		reason := ""
		for _, r := range audit.Operations {
			if r.Name == name {
				reason = r.Reason
			}
		}
		if reason == "" {
			parts = append(parts, name+" failed")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s failed: %s", name, reason))
	}
	return strings.Join(parts, "; ")
}

// preflight probes the gh transport once (`gh --version`) before any
// declared operation runs. Any inability to run gh — absent from PATH,
// broken install, dead context — is the named GHAbsentError refusal; the
// walk never starts (UB-7 class, AP-17 fail-closed).
func (x *executor) preflight(ctx context.Context) error {
	stdout, stderr, exit, err := x.runner.RunGH(ctx, "--version")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return &GHAbsentError{Reason: "gh is not on PATH", Err: err}
		}
		return &GHAbsentError{Reason: fmt.Sprintf("gh cannot be run: %v", err), Err: err}
	}
	if exit != 0 {
		return &GHAbsentError{Reason: fmt.Sprintf("`gh --version` exited %d: %s", exit, firstLine(stderr))}
	}
	x.log.LogAttrs(ctx, slog.LevelDebug, "wire: gh preflight ok",
		slog.String("run_id", x.runID),
		slog.String("gh_version", firstLine(stdout)))
	return nil
}

// runOp dispatches one declared row. Anything outside the five declared
// names — or a row whose transport contradicts its name — executes nothing
// and fails by name: even a forged table row cannot reach gh through an
// undeclared shape (ABUSE-3's bound, held by construction).
func (x *executor) runOp(ctx context.Context, op Op, st *walkState) (OpResult, error) {
	if op.Transport == TransportForgejo {
		return x.runForgejoOp(ctx, op, st)
	}
	if op.Connection.Provider == integrations.GitHub {
		if original, ok := x.runner.(GHRunner); ok {
			x.runner = GHRunner{TokenEnv: op.Connection.TokenEnv, Host: "github.com"}
			defer func() { x.runner = original }()
		}
	}
	res := OpResult{Name: op.Name, Transport: op.Transport, Connection: op.Connection}
	start := x.now()
	var err error
	switch op.Name {
	case OpPushWorkflow:
		if op.Transport == TransportConverge {
			res.Outcome, res.Reason = OutcomeSkipped, reasonConvergeCarried
		} else {
			x.fail(&res, st, fmt.Sprintf("push-workflow declares transport %q; its one declared transport is converge", op.Transport))
		}
	case OpCreateRepo, OpEnsureLabels, OpSetProtection, OpWelcomeItem, OpOnboardingItems:
		if op.Transport != TransportGH {
			x.fail(&res, st, fmt.Sprintf("%s declares transport %q; its one declared transport is gh", op.Name, op.Transport))
			break
		}
		if st.repoDeferred {
			x.handoff(op, &res, st, op.ensureCommands(), "deferred: create-repo did not complete — run these after the repository exists")
			break
		}
		switch op.Name {
		case OpCreateRepo:
			err = x.opCreateRepo(ctx, op, &res, st)
		case OpEnsureLabels:
			err = x.opEnsureLabels(ctx, op, &res, st)
		case OpSetProtection:
			err = x.opSetProtection(ctx, op, &res, st)
		case OpWelcomeItem:
			err = x.opWelcomeItem(ctx, op, &res, st)
		case OpOnboardingItems:
			err = x.opOnboardingItems(ctx, op, &res, st)
		}
	default:
		x.fail(&res, st, fmt.Sprintf("undeclared operation %q — Execute runs only the six declared rows (ABUSE-3)", op.Name))
	}
	if err != nil && res.Outcome == "" {
		// Transport-level abort mid-op: the row records the abort; the
		// walk-level error carries it (Execute returns non-nil).
		res.Outcome, res.Reason = OutcomeFailed, "aborted: "+err.Error()
	}
	res.Duration = x.now().Sub(start)
	return res, err
}

// handoff marks res handed-off and appends the exact remaining commands to
// the walk's hand-off block (UB-4).
func (x *executor) handoff(op Op, res *OpResult, st *walkState, cmds [][]string, why string) {
	res.Outcome, res.Reason = OutcomeHandedOff, why
	st.entries = append(st.entries, handoffEntry{op: op.Name, why: why, cmds: cmds})
}

// fail marks res failed — the outside-UB-4 outcome that makes Execute's
// return error non-nil.
func (x *executor) fail(res *OpResult, st *walkState, why string) {
	res.Outcome, res.Reason = OutcomeFailed, why
	st.failed = append(st.failed, res.Name)
}

// stepOutcome is one gh step's terminal state after retries.
type stepOutcome struct {
	ok       bool // exit 0, or a benign already-exists outcome
	benign   bool // ok came from a benign pattern, not exit 0
	class    failureClass
	stdout   string
	stderr   string
	attempts int
}

// runOnce performs exactly one gh invocation, recording the argv in the
// audit row first — the record exists even if the transport dies mid-call.
func (x *executor) runOnce(ctx context.Context, res *OpResult, args []string) (string, string, int, error) {
	res.Commands = append(res.Commands, append([]string(nil), args...))
	stdout, stderr, exit, err := x.runner.RunGH(ctx, args...)
	if err != nil {
		return stdout, stderr, exit, fmt.Errorf("run gh %s: %w", strings.Join(args, " "), err)
	}
	return stdout, stderr, exit, nil
}

// runStep runs one gh step with the bounded transient retry: up to
// maxAttempts invocations, exponential backoff (capped) between them, via
// the injected sleeper. benign patterns turn a non-zero exit into success
// (an "already exists" create IS the ensured end state). The returned
// error is transport-level only and aborts the walk.
func (x *executor) runStep(ctx context.Context, res *OpResult, args []string, benign []string) (stepOutcome, error) {
	delay := retryBaseDelay
	var out stepOutcome
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out.attempts = attempt
		stdout, stderr, exit, err := x.runOnce(ctx, res, args)
		if err != nil {
			return stepOutcome{}, err
		}
		out.stdout, out.stderr = stdout, stderr
		if exit == 0 {
			out.ok = true
			return out, nil
		}
		if matchesAny(stderr, benign) {
			out.ok, out.benign = true, true
			return out, nil
		}
		out.class = classify(stderr)
		if out.class != classTransient || attempt == maxAttempts {
			return out, nil
		}
		x.log.LogAttrs(ctx, slog.LevelWarn, "wire: transient gh failure, retrying",
			slog.String("run_id", x.runID),
			slog.String("op", res.Name),
			slog.Int("attempt", attempt),
			slog.String("stderr", firstLine(stderr)),
			slog.Duration("backoff", delay))
		if serr := x.sleep(ctx, delay); serr != nil {
			return stepOutcome{}, fmt.Errorf("backoff interrupted: %w", serr)
		}
		delay = min(delay*2, retryMaxDelay)
	}
	return out, nil
}

// probeFreshRepo runs a read probe with grace for the replication window
// after a same-walk repository creation: GitHub's read paths lag the
// create by seconds, surfacing as not-found or as an ok probe with an
// empty body, and believing either one fails a correct run (#897). While
// walkState.repoCreated, such results re-probe on the transient backoff
// schedule; without a same-walk create the first result stands — a
// genuinely wrong slug must keep failing by name.
func (x *executor) probeFreshRepo(ctx context.Context, res *OpResult, st *walkState, args []string) (stepOutcome, error) {
	delay := retryBaseDelay
	for attempt := 1; ; attempt++ {
		out, err := x.runStep(ctx, res, args, nil)
		if err != nil {
			return stepOutcome{}, err
		}
		lagging := st.repoCreated &&
			((out.ok && strings.TrimSpace(out.stdout) == "") || (!out.ok && out.class == classNotFound))
		if !lagging || attempt == maxAttempts {
			return out, nil
		}
		x.log.LogAttrs(ctx, slog.LevelWarn, "wire: fresh-repo read lag, re-probing",
			slog.String("run_id", x.runID),
			slog.String("op", res.Name),
			slog.Int("attempt", attempt),
			slog.Duration("backoff", delay))
		if serr := x.sleep(ctx, delay); serr != nil {
			return stepOutcome{}, fmt.Errorf("backoff interrupted: %w", serr)
		}
		delay = min(delay*2, retryMaxDelay)
	}
}

// freshRepoLagWhy renders the hand-off reason for a read that never
// settled inside the replication window's grace.
func freshRepoLagWhy(o stepOutcome) string {
	evidence := firstLine(o.stderr)
	if evidence == "" {
		evidence = "gh returned an empty body"
	}
	return fmt.Sprintf("the repository was created this run but its read paths had not caught up after %d probes (%s) — handing off per UB-4's extension", maxAttempts, evidence)
}

// transientWhy / permissionWhy render the hand-off reasons with the
// classified evidence — the raw gh stderr line, unredacted.
func transientWhy(st stepOutcome) string {
	return fmt.Sprintf("transient failure persisted through %d attempts (%s) — handing off per UB-4's extension", st.attempts, firstLine(st.stderr))
}

func permissionWhy(st stepOutcome) string {
	return fmt.Sprintf("permission denied (%s)", firstLine(st.stderr))
}

// issueURL renders a github.com issue URL from the slug and number — used
// where an item's existence is known from a probe rather than a create's
// own stdout (#948).
func issueURL(slug string, num int) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", slug, num)
}

// branchAbsent404 recognizes GitHub's branch-missing 404 by its message:
// the API answers "Branch not found" when the branch does not exist, versus
// a bare "Not Found" when it hides an admin-only endpoint from a non-admin
// credential — the one textual difference that separates "push first" from
// "re-authenticate" (#936).
func branchAbsent404(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "branch not found")
}

// --- the five op executors -------------------------------------------------

func (x *executor) opCreateRepo(ctx context.Context, op Op, res *OpResult, st *walkState) error {
	probe, err := x.runStep(ctx, res, op.probeArgv(), nil)
	if err != nil {
		return err
	}
	if probe.ok {
		res.Outcome = OutcomeSkipped
		res.Reason = fmt.Sprintf("repository %s already exists (AP-22: nothing to create)", op.Slug)
		return nil
	}
	switch probe.class {
	case classNotFound:
		// The expected miss — create it.
		cr, err := x.runStep(ctx, res, op.repoCreateArgv(), []string{"already exists"})
		if err != nil {
			return err
		}
		switch {
		case cr.ok && cr.benign:
			res.Outcome = OutcomeApplied
			st.repoCreated = true
			res.Reason = fmt.Sprintf("repository %s already existed at create time (converged)", op.Slug)
		case cr.ok:
			res.Outcome = OutcomeApplied
			st.repoCreated = true
			res.Reason = fmt.Sprintf("created private repository %s", op.Slug)
		case cr.class == classPermission:
			st.repoDeferred = true
			x.handoff(op, res, st, [][]string{op.repoCreateArgv()}, permissionWhy(cr))
		case cr.class == classTransient:
			st.repoDeferred = true
			x.handoff(op, res, st, [][]string{op.repoCreateArgv()}, transientWhy(cr))
		default:
			st.repoDeferred = true
			x.fail(res, st, fmt.Sprintf("create repository %s: %s", op.Slug, firstLine(cr.stderr)))
		}
	case classPermission:
		st.repoDeferred = true
		x.handoff(op, res, st, [][]string{op.repoCreateArgv()},
			permissionWhy(probe)+" — existence could not be verified; `gh repo create` fails loudly if it already exists")
	case classTransient:
		st.repoDeferred = true
		x.handoff(op, res, st, [][]string{op.repoCreateArgv()}, transientWhy(probe))
	default:
		st.repoDeferred = true
		x.fail(res, st, fmt.Sprintf("probe repository %s: %s", op.Slug, firstLine(probe.stderr)))
	}
	return nil
}

func (x *executor) opEnsureLabels(ctx context.Context, op Op, res *OpResult, st *walkState) error {
	list, err := x.probeFreshRepo(ctx, res, st, op.probeArgv())
	if err != nil {
		return err
	}
	if !list.ok {
		switch list.class {
		case classPermission:
			x.handoff(op, res, st, op.ensureCommands(), permissionWhy(list))
		case classTransient:
			x.handoff(op, res, st, op.ensureCommands(), transientWhy(list))
		case classNotFound:
			if st.repoCreated {
				x.handoff(op, res, st, op.ensureCommands(), freshRepoLagWhy(list))
			} else {
				x.fail(res, st, fmt.Sprintf("repository %s not found while listing labels (%s) — check the slug and the wiring.repo knob", op.Slug, firstLine(list.stderr)))
			}
		default:
			x.fail(res, st, fmt.Sprintf("list labels on %s: %s", op.Slug, firstLine(list.stderr)))
		}
		return nil
	}
	var have []struct {
		Name string `json:"name"`
	}
	// A blank body from an ok probe is the replication window's other face
	// (gh emits nothing, not "[]") — no labels visible yet. The per-label
	// creates below converge either way: "already exists" is benign.
	if s := strings.TrimSpace(list.stdout); s != "" {
		if err := json.Unmarshal([]byte(s), &have); err != nil {
			x.fail(res, st, fmt.Sprintf("decode `gh label list` output: %v", err))
			return nil
		}
	}
	exists := func(name string) bool {
		for _, h := range have {
			if strings.EqualFold(h.Name, name) {
				return true
			}
		}
		return false
	}
	missing := op.Labels[:0:0]
	for _, l := range op.Labels {
		if !exists(l.Name) {
			missing = append(missing, l)
		}
	}
	if len(missing) == 0 {
		res.Outcome = OutcomeSkipped
		res.Reason = fmt.Sprintf("all %d routing labels already exist (AP-22)", len(op.Labels))
		return nil
	}
	var created []string
	for i, l := range missing {
		cr, err := x.runStep(ctx, res, op.labelCreateArgv(l, false), []string{"already exists"})
		if err != nil {
			return err
		}
		if cr.ok {
			created = append(created, l.Name)
			continue
		}
		// The unresolved remainder, in blind-run-safe --force form.
		remaining := make([][]string, 0, len(missing)-i)
		for _, r := range missing[i:] {
			remaining = append(remaining, op.labelCreateArgv(r, true))
		}
		switch cr.class {
		case classPermission:
			x.handoff(op, res, st, remaining, permissionWhy(cr))
		case classTransient:
			x.handoff(op, res, st, remaining, transientWhy(cr))
		default:
			x.fail(res, st, fmt.Sprintf("create label %q: %s", l.Name, firstLine(cr.stderr)))
		}
		return nil
	}
	res.Outcome = OutcomeApplied
	res.Reason = fmt.Sprintf("created %d of %d routing labels (%s); %d already existed",
		len(created), len(op.Labels), strings.Join(created, ", "), len(op.Labels)-len(missing))
	return nil
}

func (x *executor) opSetProtection(ctx context.Context, op Op, res *OpResult, st *walkState) error {
	put, err := x.runStep(ctx, res, op.protectionArgv(), nil)
	if err != nil {
		return err
	}
	switch {
	case put.ok:
		level := "required-checks"
		if op.RequireReview {
			level = "checks+review"
		}
		res.Outcome = OutcomeApplied
		res.Reason = fmt.Sprintf("branch protection PUT on %s@%s — level %s, contexts %s (full-state PUT: idempotent by nature)",
			op.Slug, op.Branch, level, strings.Join(op.Contexts, ", "))
	case put.class == classPermission:
		x.handoff(op, res, st, [][]string{op.protectionArgv()}, permissionWhy(put))
	case put.class == classNotFound && (st.repoCreated || branchAbsent404(put.stderr)):
		// NOT a permission gap (#936): pyxis init never pushes, so on a
		// repository born this run — or any still-empty one — the branch
		// cannot exist yet, and GitHub answers "Branch not found" (distinct
		// from the admin-hidden endpoint's bare "Not Found"). The handed-off
		// command becomes valid the moment the first push creates the
		// branch; re-authenticating changes nothing here.
		x.handoff(op, res, st, [][]string{op.protectionArgv()},
			fmt.Sprintf("branch %s does not exist on GitHub yet (init commits locally and never pushes) — protection cannot be set before your first push creates the branch; run the command below after that push, it is safe to re-run (%s)", op.Branch, firstLine(put.stderr)))
	case put.class == classNotFound:
		// The charter's 404-admin case: GitHub hides admin-only endpoints
		// from non-admin credentials behind 404, not 403.
		x.handoff(op, res, st, [][]string{op.protectionArgv()},
			fmt.Sprintf("HTTP 404 on the protection endpoint (%s) — GitHub answers 404, not 403, when the credential lacks admin on the repository; treat as a permission gap", firstLine(put.stderr)))
	case put.class == classTransient:
		x.handoff(op, res, st, [][]string{op.protectionArgv()}, transientWhy(put))
	default:
		x.fail(res, st, fmt.Sprintf("set branch protection on %s@%s: %s", op.Slug, op.Branch, firstLine(put.stderr)))
	}
	return nil
}

func (x *executor) opWelcomeItem(ctx context.Context, op Op, res *OpResult, st *walkState) error {
	probe, err := x.probeFreshRepo(ctx, res, st, op.probeArgv())
	if err != nil {
		return err
	}
	if !probe.ok {
		switch probe.class {
		case classPermission:
			x.handoff(op, res, st, [][]string{op.welcomeCreateArgv()},
				permissionWhy(probe)+" — existence could not be verified; check for a duplicate welcome item before running")
		case classTransient:
			x.handoff(op, res, st, [][]string{op.welcomeCreateArgv()}, transientWhy(probe))
		case classNotFound:
			if st.repoCreated {
				x.handoff(op, res, st, [][]string{op.welcomeCreateArgv()},
					freshRepoLagWhy(probe)+" — existence could not be verified; check for a duplicate welcome item before running")
			} else {
				x.fail(res, st, fmt.Sprintf("repository %s not found while probing for the welcome item (%s)", op.Slug, firstLine(probe.stderr)))
			}
		default:
			x.fail(res, st, fmt.Sprintf("probe for the welcome item on %s: %s", op.Slug, firstLine(probe.stderr)))
		}
		return nil
	}
	if num, found, ferr := findWelcomeMarker(probe.stdout); ferr != nil {
		x.fail(res, st, fmt.Sprintf("decode `gh issue list` output: %v", ferr))
		return nil
	} else if found {
		res.Outcome = OutcomeSkipped
		res.Reason = fmt.Sprintf("welcome item already exists (#%d carries %s; AP-22)", num, WelcomeMarker)
		// The existing item's URL still rides the audit (#948): the closing
		// surface links the guide whether this run created it or found it.
		res.Refs = append(res.Refs, issueURL(op.Slug, num))
		return nil
	}

	// The create is the table's ONE non-idempotent mutation: a duplicate
	// welcome item is possible if an ambiguous failure (timeout, 5xx)
	// actually landed. So each retry re-probes for the marker first and
	// declares applied when the earlier attempt is found to have landed —
	// the probe, not blind re-POST, is the convergence check.
	delay := retryBaseDelay
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stdout, stderr, exit, err := x.runOnce(ctx, res, op.welcomeCreateArgv())
		if err != nil {
			return err
		}
		if exit == 0 {
			res.Outcome = OutcomeApplied
			res.Reason = "created the welcome item"
			// gh prints the created issue's URL — the guide link the
			// closing surface renders (#948).
			if ref := firstLine(stdout); ref != "" {
				res.Refs = append(res.Refs, ref)
			}
			return nil
		}
		switch class := classify(stderr); class {
		case classPermission:
			x.handoff(op, res, st, [][]string{op.welcomeCreateArgv()}, permissionWhy(stepOutcome{stderr: stderr}))
			return nil
		case classTransient:
			if attempt == maxAttempts {
				x.handoff(op, res, st, [][]string{op.welcomeCreateArgv()},
					transientWhy(stepOutcome{stderr: stderr, attempts: attempt}))
				return nil
			}
			pstdout, _, pexit, perr := x.runOnce(ctx, res, op.probeArgv())
			if perr != nil {
				return perr
			}
			if pexit == 0 {
				if num, found, ferr := findWelcomeMarker(pstdout); ferr == nil && found {
					res.Outcome = OutcomeApplied
					res.Reason = fmt.Sprintf("welcome item #%d landed on an earlier ambiguous attempt (verified by re-probe)", num)
					res.Refs = append(res.Refs, issueURL(op.Slug, num))
					return nil
				}
			}
			if serr := x.sleep(ctx, delay); serr != nil {
				return fmt.Errorf("backoff interrupted: %w", serr)
			}
			delay = min(delay*2, retryMaxDelay)
		default:
			x.fail(res, st, fmt.Sprintf("create the welcome item on %s: %s", op.Slug, firstLine(stderr)))
			return nil
		}
	}
	return nil
}

// wiringRemainderItem derives the finish-the-wiring onboarding item from
// the walk's accumulated hand-off entries (#947): the same rendered block
// the terminal shows, preserved where it cannot scroll away. Derived at
// execution time — only the walk knows which steps handed off — which is
// why the onboarding-items row is declared last in the table.
func wiringRemainderItem(entries []handoffEntry) WorkItem {
	return WiringRemainderItem(renderHandOff(entries))
}

// WiringRemainderItem builds the finish-the-wiring onboarding item around
// an already-rendered hand-off block — the ONE body shape both tracker
// lanes file (#947): the gh lane derives the block from its own walk;
// the carina lane passes the audit's block.
func WiringRemainderItem(handOffBlock string) WorkItem {
	const slug = "finish-github-wiring"
	body := "pyxis init completed every wiring step it could; the steps below needed you and were handed off.\n\n" +
		"Run each command at the moment its note names. Update the selected connection's credentials or permissions first when access is missing. Creation commands can create duplicates, so inspect the target before re-running one.\n\n" +
		"```sh\n" + strings.TrimRight(handOffBlock, "\n") + "\n```\n\n" +
		"Close this item when every command above has run clean.\n\n" +
		OnboardingMarker(slug)
	return WorkItem{
		Slug:  slug,
		Title: "Finish the integration wiring pyxis init could not complete",
		Body:  body,
	}
}

// opOnboardingItems ensures the onboarding work items (#947): the
// installer-authored standing set on the Op, plus the derived
// finish-the-wiring item when the walk left hand-offs. Idempotent per item
// via body markers (AP-22): the probe's body-set decides create vs skip,
// and each create follows the welcome item's convergence discipline — a
// retried ambiguous failure re-probes for the marker rather than blindly
// re-POSTing, so a duplicate is never minted.
func (x *executor) opOnboardingItems(ctx context.Context, op Op, res *OpResult, st *walkState) error {
	items := append([]WorkItem(nil), op.Items...)
	if len(st.entries) > 0 {
		items = append(items, wiringRemainderItem(st.entries))
	}
	if len(items) == 0 {
		res.Outcome = OutcomeSkipped
		res.Reason = "no onboarding items to file"
		return nil
	}
	remaining := func(from int) [][]string {
		cmds := make([][]string, 0, len(items)-from)
		for _, it := range items[from:] {
			cmds = append(cmds, op.itemCreateArgv(it))
		}
		return cmds
	}

	probe, err := x.probeFreshRepo(ctx, res, st, op.probeArgv())
	if err != nil {
		return err
	}
	if !probe.ok {
		switch probe.class {
		case classPermission:
			x.handoff(op, res, st, remaining(0), permissionWhy(probe)+" — existence could not be verified; check for duplicate onboarding items before running")
		case classTransient:
			x.handoff(op, res, st, remaining(0), transientWhy(probe))
		case classNotFound:
			if st.repoCreated {
				x.handoff(op, res, st, remaining(0), freshRepoLagWhy(probe)+" — existence could not be verified; check for duplicate onboarding items before running")
			} else {
				x.fail(res, st, fmt.Sprintf("repository %s not found while probing for onboarding items (%s)", op.Slug, firstLine(probe.stderr)))
			}
		default:
			x.fail(res, st, fmt.Sprintf("probe for onboarding items on %s: %s", op.Slug, firstLine(probe.stderr)))
		}
		return nil
	}

	created, existed := 0, 0
	for i, it := range items {
		marker := OnboardingMarker(it.Slug)
		if num, found, ferr := findMarker(probe.stdout, marker); ferr != nil {
			x.fail(res, st, fmt.Sprintf("decode `gh issue list` output: %v", ferr))
			return nil
		} else if found {
			existed++
			// The existing item's ref rides the audit too (#948): the
			// epilogue's todo list names every item, found or filed.
			res.Refs = append(res.Refs, issueURL(op.Slug, num))
			continue
		}
		delay := retryBaseDelay
	create:
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			stdout, stderr, exit, rerr := x.runOnce(ctx, res, op.itemCreateArgv(it))
			if rerr != nil {
				return rerr
			}
			if exit == 0 {
				if ref := firstLine(stdout); ref != "" {
					res.Refs = append(res.Refs, ref)
				}
				created++
				break create
			}
			switch classify(stderr) {
			case classPermission:
				x.handoff(op, res, st, remaining(i), permissionWhy(stepOutcome{stderr: stderr}))
				return nil
			case classTransient:
				if attempt == maxAttempts {
					x.handoff(op, res, st, remaining(i), transientWhy(stepOutcome{stderr: stderr, attempts: attempt}))
					return nil
				}
				// The ambiguous-landing re-probe: this item's marker found
				// means the earlier attempt landed despite its failure shape.
				pstdout, _, pexit, perr := x.runOnce(ctx, res, op.probeArgv())
				if perr != nil {
					return perr
				}
				if pexit == 0 {
					if num, found, ferr := findMarker(pstdout, marker); ferr == nil && found {
						created++
						res.Refs = append(res.Refs, issueURL(op.Slug, num))
						break create
					}
				}
				if serr := x.sleep(ctx, delay); serr != nil {
					return fmt.Errorf("backoff interrupted: %w", serr)
				}
				delay = min(delay*2, retryMaxDelay)
			default:
				x.fail(res, st, fmt.Sprintf("create onboarding item %q on %s: %s", it.Title, op.Slug, firstLine(stderr)))
				return nil
			}
		}
	}

	if created == 0 {
		// Nothing was mutated: the audit says so in ensure-labels' own
		// taxonomy (AP-22 idempotence reads as skipped, never applied).
		res.Outcome = OutcomeSkipped
		res.Reason = fmt.Sprintf("all %d onboarding item(s) already exist (AP-22)", existed)
		return nil
	}
	res.Outcome = OutcomeApplied
	res.Reason = fmt.Sprintf("filed %d onboarding item(s); %d already existed", created, existed)
	return nil
}

// findWelcomeMarker scans a `gh issue list --json number,body` payload for
// WelcomeMarker, returning the first carrying item's number.
func findWelcomeMarker(stdout string) (int, bool, error) {
	return findMarker(stdout, WelcomeMarker)
}

// findMarker scans a `gh issue list` JSON body-set for an idempotency
// marker, returning the carrying issue's number on a hit. Markers count
// only OUTSIDE fenced code blocks: the finish-the-wiring item quotes
// handed-off commands verbatim inside a ```sh fence, and a quoted
// `issue create --body '…<marker>…'` command must never read as the item
// it would create (the marker-poisoning class the #947 review caught).
func findMarker(stdout, marker string) (int, bool, error) {
	// Blank is a real gh output (not "[]") inside the post-create
	// replication window — no items visible yet, not a decode defect.
	if strings.TrimSpace(stdout) == "" {
		return 0, false, nil
	}
	var items []struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		return 0, false, fmt.Errorf("unmarshal issue list: %w", err)
	}
	for _, it := range items {
		if markerOutsideFences(it.Body, marker) {
			return it.Number, true, nil
		}
	}
	return 0, false, nil
}

// markerOutsideFences reports whether marker occurs in body outside any
// ``` fenced block.
func markerOutsideFences(body, marker string) bool {
	inFence := false
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

// renderHandOff renders the copy-paste remainder block (UB-4): the exact
// gh commands, in table order, that complete the wiring under a credential
// that has the missing permission. Every command is safe to re-run
// (ensure-form label creates, the idempotent PUT, loud-fail creates).
func renderHandOff(entries []handoffEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# wiring steps that need you\n")
	b.WriteString("# Each entry says why it waited; run its command at the moment it names\n")
	b.WriteString("# (update the selected connection's credentials or permissions when needed).\n")
	b.WriteString("# Creation commands can create duplicates; inspect the target before re-running them.\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "\n# %s — %s\n", e.op, e.why)
		if e.raw != "" {
			b.WriteString(e.raw)
			b.WriteByte('\n')
		}
		for _, c := range e.cmds {
			b.WriteString(shellJoin(c))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// newRunID mints the run correlation id (installer-v1 §3.8); tests inject
// theirs via WithRunID.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
