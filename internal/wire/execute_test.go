package wire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
)

// --- the transcript fake -----------------------------------------------

// rule is one scripted gh response: first matching rule with remaining
// budget wins; times < 0 means unlimited.
type rule struct {
	match          func([]string) bool
	stdout, stderr string
	exit           int
	err            error
	remaining      int
}

// fakeRunner is the hermetic gh transcript: scripted responses in, every
// call recorded out. An UNSCRIPTED call is a test failure — the fake is
// how the tests hold "the executed set never exceeds the declared set".
type fakeRunner struct {
	t     *testing.T
	rules []*rule
	calls [][]string
}

func newFake(t *testing.T) *fakeRunner { return &fakeRunner{t: t} }

func matchSub(parts ...string) func([]string) bool {
	return func(args []string) bool {
		j := strings.Join(args, " ")
		for _, p := range parts {
			if !strings.Contains(j, p) {
				return false
			}
		}
		return true
	}
}

// on scripts a response for calls whose joined argv contains every part.
// Rules match in insertion order, so overrides are added BEFORE defaults.
func (f *fakeRunner) on(times int, stdout, stderr string, exit int, parts ...string) *fakeRunner {
	f.rules = append(f.rules, &rule{match: matchSub(parts...), stdout: stdout, stderr: stderr, exit: exit, remaining: times})
	return f
}

// onErr scripts a transport-level failure (gh absent, context death).
func (f *fakeRunner) onErr(times int, err error, parts ...string) *fakeRunner {
	f.rules = append(f.rules, &rule{match: matchSub(parts...), err: err, remaining: times})
	return f
}

func (f *fakeRunner) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	for _, r := range f.rules {
		if r.remaining == 0 || !r.match(args) {
			continue
		}
		if r.remaining > 0 {
			r.remaining--
		}
		if r.err != nil {
			return "", "", 0, r.err
		}
		return r.stdout, r.stderr, r.exit, nil
	}
	f.t.Errorf("fake gh: unscripted call: gh %s", strings.Join(args, " "))
	return "", "fake: unscripted call", 1, nil
}

// countCalls returns how many recorded calls contain every part.
func (f *fakeRunner) countCalls(parts ...string) int {
	m, n := matchSub(parts...), 0
	for _, c := range f.calls {
		if m(c) {
			n++
		}
	}
	return n
}

func labelsJSON(t *testing.T, labels []dialect.Label) string {
	t.Helper()
	type row struct {
		Name string `json:"name"`
	}
	rows := make([]row, 0, len(labels))
	for _, l := range labels {
		rows = append(rows, row{l.Name})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal labels: %v", err)
	}
	return string(b)
}

// withDefaults appends the permissive fresh-empty-repo transcript: probes
// miss, mutations succeed. Overrides scripted earlier still win.
func withDefaults(f *fakeRunner) *fakeRunner {
	f.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	f.on(-1, "", "GraphQL: Could not resolve to a Repository with the name '"+testSlug+"'.", 1, "repo view")
	f.on(-1, "https://github.com/"+testSlug, "", 0, "repo create")
	f.on(-1, "[]", "", 0, "label list")
	f.on(-1, "", "", 0, "label create")
	f.on(-1, "{}", "", 0, "branches/main/protection")
	f.on(-1, "[]", "", 0, "issue list")
	f.on(-1, "https://github.com/"+testSlug+"/issues/1", "", 0, "issue create")
	return f
}

// --- exec harness helpers ------------------------------------------------

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// sleepRecorder is the injected Sleeper: no real time passes, every delay
// is recorded (the hermetic-clock rule).
type sleepRecorder struct {
	delays []time.Duration
	err    error
}

func (s *sleepRecorder) sleep(_ context.Context, d time.Duration) error {
	s.delays = append(s.delays, d)
	return s.err
}

// tick is the injected clock: strictly monotonic, no real time read.
func tick() func() time.Time {
	t0 := time.Unix(1700000000, 0)
	return func() time.Time {
		t0 = t0.Add(time.Millisecond)
		return t0
	}
}

func execOpts(sl *sleepRecorder) []ExecOption {
	return []ExecOption{
		WithLogger(discardLogger()),
		WithSleeper(sl.sleep),
		WithNow(tick()),
		WithRunID("test-run"),
	}
}

func findResult(t *testing.T, a Audit, name string) OpResult {
	t.Helper()
	for _, r := range a.Operations {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("audit carries no row for %s (rows: %v)", name, a.OperationNames())
	return OpResult{}
}

// attributable reports whether one recorded gh call belongs to the
// declared table: the preflight probe, or a command surface of a declared
// op. Anything else is an over-execution.
func attributable(call []string, ops []Op) bool {
	j := strings.Join(call, " ")
	if j == "--version" {
		return true
	}
	declared := func(name string) bool {
		for _, o := range ops {
			if o.Name == name {
				return true
			}
		}
		return false
	}
	switch {
	case strings.HasPrefix(j, "repo view "), strings.HasPrefix(j, "repo create "):
		return declared(OpCreateRepo)
	case strings.HasPrefix(j, "label list "), strings.HasPrefix(j, "label create "):
		return declared(OpEnsureLabels)
	case strings.HasPrefix(j, "api --method PUT ") && strings.Contains(j, "/protection"):
		return declared(OpSetProtection)
	case strings.HasPrefix(j, "issue list "), strings.HasPrefix(j, "issue create "):
		return declared(OpWelcomeItem)
	}
	return false
}

// --- the tests -----------------------------------------------------------

// TestExecuteWalksExactlyTheDeclaredTable sweeps every knob combination:
// the audit rows equal the declared table one-to-one and in order, and
// every gh call the transcript saw is attributable to a declared op — the
// executed set never exceeds the declared set (ABUSE-3).
func TestExecuteWalksExactlyTheDeclaredTable(t *testing.T) {
	trackers, ciModes, repos, protections := knobSpace()
	for _, tr := range trackers {
		for _, ci := range ciModes {
			for _, rp := range repos {
				for _, pr := range protections {
					name := fmt.Sprintf("%s_%s_%s_%s", tr, ci, rp, pr)
					t.Run(name, func(t *testing.T) {
						ops := Table(knobsConfig(tr, ci, rp, pr))
						fake := withDefaults(newFake(t))
						sl := &sleepRecorder{}
						audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
						if err != nil {
							t.Fatalf("Execute: %v", err)
						}
						if got, want := audit.OperationNames(), opNames(ops); !equalStrings(got, want) {
							t.Fatalf("audit rows %v != declared table %v", got, want)
						}
						if audit.Partial {
							t.Errorf("permissive transcript reported partial")
						}
						for _, r := range audit.Operations {
							if r.Outcome != OutcomeApplied && r.Outcome != OutcomeSkipped {
								t.Errorf("%s: outcome %s (%s), want applied/skipped", r.Name, r.Outcome, r.Reason)
							}
						}
						for _, c := range fake.calls {
							if !attributable(c, ops) {
								t.Errorf("gh call outside the declared table: gh %s", strings.Join(c, " "))
							}
						}
						hasGH := false
						for _, o := range ops {
							if o.Transport == TransportGH {
								hasGH = true
							}
						}
						if !hasGH && len(fake.calls) != 0 {
							t.Errorf("wiring-free table touched gh: %v", fake.calls)
						}
					})
				}
			}
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestExecuteAppliesFreshRepo(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "checks+review"))
	fake := withDefaults(newFake(t))
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if audit.Partial || audit.HandOff != "" {
		t.Fatalf("fresh-repo run: partial=%v handoff=%q", audit.Partial, audit.HandOff)
	}
	if r := findResult(t, audit, OpCreateRepo); r.Outcome != OutcomeApplied {
		t.Errorf("create-repo: %s (%s)", r.Outcome, r.Reason)
	}
	labels := findResult(t, audit, OpEnsureLabels)
	if labels.Outcome != OutcomeApplied {
		t.Errorf("ensure-labels: %s (%s)", labels.Outcome, labels.Reason)
	}
	if want := 1 + len(RoutingLabels()); len(labels.Commands) != want {
		t.Errorf("ensure-labels ran %d gh calls, want %d (list + one create per label)", len(labels.Commands), want)
	}
	if r := findResult(t, audit, OpPushWorkflow); r.Outcome != OutcomeSkipped || !strings.Contains(r.Reason, "converge write set") {
		t.Errorf("push-workflow: %s (%s)", r.Outcome, r.Reason)
	}
	if len(findResult(t, audit, OpPushWorkflow).Commands) != 0 {
		t.Errorf("push-workflow invoked gh")
	}
	if r := findResult(t, audit, OpSetProtection); r.Outcome != OutcomeApplied || !strings.Contains(r.Reason, "checks+review") {
		t.Errorf("set-protection: %s (%s)", r.Outcome, r.Reason)
	}
	if r := findResult(t, audit, OpWelcomeItem); r.Outcome != OutcomeApplied {
		t.Errorf("welcome-item: %s (%s)", r.Outcome, r.Reason)
	}
	if sl.delays != nil {
		t.Errorf("clean run slept: %v", sl.delays)
	}
}

// TestExecuteIdempotentSecondRun replays Execute against a transcript in
// which everything already exists: the probed ops all skip-with-reason
// (AP-22); the full-state protection PUT re-applies by nature.
func TestExecuteIdempotentSecondRun(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, `{"name":"newrepo"}`, "", 0, "repo view")
	fake.on(-1, labelsJSON(t, RoutingLabels()), "", 0, "label list")
	fake.on(-1, "{}", "", 0, "branches/main/protection")
	fake.on(-1, `[{"number":7,"body":"hello\n\n`+WelcomeMarker+`"}]`, "", 0, "issue list")
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if audit.Partial {
		t.Fatalf("idempotent run reported partial")
	}
	for name, wantReason := range map[string]string{
		OpCreateRepo:   "already exists",
		OpEnsureLabels: "already exist",
		OpWelcomeItem:  "#7",
	} {
		r := findResult(t, audit, name)
		if r.Outcome != OutcomeSkipped {
			t.Errorf("%s: outcome %s (%s), want skipped-with-reason", name, r.Outcome, r.Reason)
		}
		if !strings.Contains(r.Reason, wantReason) {
			t.Errorf("%s: reason %q misses %q", name, r.Reason, wantReason)
		}
	}
	if r := findResult(t, audit, OpSetProtection); r.Outcome != OutcomeApplied {
		t.Errorf("set-protection: %s — the full PUT is the idempotency", r.Outcome)
	}
	if n := fake.countCalls("repo create"); n != 0 {
		t.Errorf("create-repo mutated on the second run (%d creates)", n)
	}
	if n := fake.countCalls("label create"); n != 0 {
		t.Errorf("labels mutated on the second run (%d creates)", n)
	}
	if n := fake.countCalls("issue create"); n != 0 {
		t.Errorf("welcome mutated on the second run (%d creates)", n)
	}
}

// TestExecutePermissionHandOff: the protection PUT is denied admin; every
// other permitted op still applies, the hand-off block carries the exact
// remaining command, and Execute reports partial WITHOUT an error (UB-4).
func TestExecutePermissionHandOff(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "", "gh: Must have admin rights to Repository. (HTTP 403)", 1, "branches/main/protection")
	withDefaults(fake)
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("a permission gap is UB-4's partial result, not an error: %v", err)
	}
	if !audit.Partial {
		t.Fatalf("partial not reported")
	}
	prot := findResult(t, audit, OpSetProtection)
	if prot.Outcome != OutcomeHandedOff || !strings.Contains(prot.Reason, "permission denied") {
		t.Errorf("set-protection: %s (%s)", prot.Outcome, prot.Reason)
	}
	for _, name := range []string{OpEnsureLabels, OpWelcomeItem} {
		if r := findResult(t, audit, name); r.Outcome != OutcomeApplied {
			t.Errorf("%s: %s — permitted ops must still complete", name, r.Outcome)
		}
	}
	putLine := shellJoin(findOp(t, ops, OpSetProtection).protectionArgv())
	if !strings.Contains(audit.HandOff, putLine) {
		t.Errorf("hand-off misses the exact remaining command %q:\n%s", putLine, audit.HandOff)
	}
	if !strings.Contains(audit.HandOff, "Must have admin rights") {
		t.Errorf("hand-off does not carry the unredacted evidence:\n%s", audit.HandOff)
	}
}

func findOp(t *testing.T, ops []Op, name string) Op {
	t.Helper()
	for _, o := range ops {
		if o.Name == name {
			return o
		}
	}
	t.Fatalf("table carries no %s", name)
	return Op{}
}

// TestExecutePreviewAndHandOffShareOneRenderer: the hand-off command line
// is byte-identical to the preview's line for the same op — one source
// (AP-11).
func TestExecutePreviewAndHandOffShareOneRenderer(t *testing.T) {
	ops := Table(knobsConfig("carina", "local", "use", "checks+review"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "gh: Must have admin rights to Repository. (HTTP 403)", 1, "protection")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	putLine := shellJoin(ops[0].protectionArgv())
	preview := Preview(ops)[0]
	if !strings.Contains(preview, putLine) {
		t.Fatalf("preview misses the command line:\n%s", preview)
	}
	if !strings.Contains(audit.HandOff, putLine) {
		t.Fatalf("hand-off misses the same command line:\n%s", audit.HandOff)
	}
}

// TestExecutePermissionOnCreateRepoDefersDependents: when create-repo
// cannot complete, every repo-dependent op goes straight to the hand-off
// (attempting it would only 404) and the block completes the whole wiring
// in table order.
func TestExecutePermissionOnCreateRepoDefersDependents(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "GraphQL: Could not resolve to a Repository with the name '"+testSlug+"'.", 1, "repo view")
	fake.on(-1, "", "gh: Resource not accessible by integration (HTTP 403)", 1, "repo create")
	// Deliberately NO rules for labels/protection/issues: any call to them
	// is an unscripted-call test failure.
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !audit.Partial {
		t.Fatalf("partial not reported")
	}
	if r := findResult(t, audit, OpCreateRepo); r.Outcome != OutcomeHandedOff {
		t.Fatalf("create-repo: %s (%s)", r.Outcome, r.Reason)
	}
	for _, name := range []string{OpEnsureLabels, OpSetProtection, OpWelcomeItem} {
		r := findResult(t, audit, name)
		if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "deferred") {
			t.Errorf("%s: outcome %s (%s), want deferred hand-off", name, r.Outcome, r.Reason)
		}
		if len(r.Commands) != 0 {
			t.Errorf("%s attempted gh calls while deferred: %v", name, r.Commands)
		}
	}
	if r := findResult(t, audit, OpPushWorkflow); r.Outcome != OutcomeSkipped {
		t.Errorf("push-workflow: %s", r.Outcome)
	}
	// The block is complete and ordered: create → labels → protection → welcome.
	idx := func(s string) int { return strings.Index(audit.HandOff, s) }
	createIdx := idx(shellJoin(findOp(t, ops, OpCreateRepo).repoCreateArgv()))
	labelIdx := idx("gh label create")
	putIdx := idx(shellJoin(findOp(t, ops, OpSetProtection).protectionArgv()))
	welcomeIdx := idx("gh issue create")
	for what, i := range map[string]int{"repo create": createIdx, "label create": labelIdx, "protection PUT": putIdx, "issue create": welcomeIdx} {
		if i < 0 {
			t.Fatalf("hand-off misses the %s command:\n%s", what, audit.HandOff)
		}
	}
	if !(createIdx < labelIdx && labelIdx < putIdx && putIdx < welcomeIdx) {
		t.Errorf("hand-off commands out of table order:\n%s", audit.HandOff)
	}
	if n := strings.Count(audit.HandOff, "gh label create"); n != len(RoutingLabels()) {
		t.Errorf("hand-off carries %d label creates, want %d", n, len(RoutingLabels()))
	}
}

// TestExecuteTransient429RetriesThenHandsOff: the stated bound — 3
// attempts with capped exponential backoff — then UB-4's hand-off, not an
// error.
func TestExecuteTransient429RetriesThenHandsOff(t *testing.T) {
	ops := Table(knobsConfig("carina", "local", "use", "required-checks")) // exactly set-protection
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "gh: API rate limit exceeded. (HTTP 429)", 1, "protection")
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("transient exhaustion hands off, never errors: %v", err)
	}
	prot := findResult(t, audit, OpSetProtection)
	if prot.Outcome != OutcomeHandedOff || !strings.Contains(prot.Reason, "3 attempts") {
		t.Fatalf("set-protection: %s (%s)", prot.Outcome, prot.Reason)
	}
	if n := fake.countCalls("protection"); n != maxAttempts {
		t.Fatalf("PUT attempted %d times, want %d", n, maxAttempts)
	}
	if len(prot.Commands) != maxAttempts {
		t.Fatalf("audit recorded %d attempts, want %d", len(prot.Commands), maxAttempts)
	}
	wantDelays := []time.Duration{retryBaseDelay, retryBaseDelay * 2}
	if len(sl.delays) != len(wantDelays) || sl.delays[0] != wantDelays[0] || sl.delays[1] != wantDelays[1] {
		t.Fatalf("backoff delays %v, want %v", sl.delays, wantDelays)
	}
	if !strings.Contains(audit.HandOff, shellJoin(ops[0].protectionArgv())) {
		t.Fatalf("hand-off misses the PUT:\n%s", audit.HandOff)
	}
}

func TestExecuteTransientRecovers(t *testing.T) {
	ops := Table(knobsConfig("carina", "local", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(2, "", "gh: Service Unavailable (HTTP 503)", 1, "protection")
	fake.on(-1, "{}", "", 0, "protection")
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	prot := findResult(t, audit, OpSetProtection)
	if prot.Outcome != OutcomeApplied {
		t.Fatalf("set-protection: %s (%s)", prot.Outcome, prot.Reason)
	}
	if len(prot.Commands) != 3 || len(sl.delays) != 2 {
		t.Fatalf("attempts %d, sleeps %d — want 3 and 2", len(prot.Commands), len(sl.delays))
	}
	if audit.Partial {
		t.Fatalf("recovered run reported partial")
	}
}

// TestExecuteGHAbsentRefusesBeforeAnyOp: the UB-7-class named prerequisite
// refusal — zero operations walked, zero mutations, the remedy named.
func TestExecuteGHAbsentRefusesBeforeAnyOp(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "checks+review"))
	fake := newFake(t)
	fake.onErr(-1, fmt.Errorf("wire: locate gh: %w", exec.ErrNotFound), "--version")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err == nil {
		t.Fatalf("gh-absent must refuse")
	}
	var abs *GHAbsentError
	if !errors.As(err, &abs) {
		t.Fatalf("error %v is not the named GHAbsentError", err)
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("refusal does not wrap exec.ErrNotFound: %v", err)
	}
	for _, want := range []string{"UB-7", "cli.github.com", "gh auth login"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal misses %q: %v", want, err)
		}
	}
	if len(audit.Operations) != 0 {
		t.Fatalf("operations walked despite the refusal: %v", audit.OperationNames())
	}
	if len(fake.calls) != 1 {
		t.Fatalf("gh touched beyond the preflight probe: %v", fake.calls)
	}
}

// TestExecuteNoGHOpsNeverTouchesGH: a wiring-free table (the offline
// floor) runs with no runner at all — gh is a GitHub-mode-only
// prerequisite (installer-v1 §3.6).
func TestExecuteNoGHOpsNeverTouchesGH(t *testing.T) {
	ops := Table(knobsConfig("carina", "actions", "use", "none")) // exactly push-workflow
	audit, err := Execute(context.Background(), ops, nil, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := audit.OperationNames(); len(got) != 1 || got[0] != OpPushWorkflow {
		t.Fatalf("audit rows %v, want exactly push-workflow", got)
	}
	r := audit.Operations[0]
	if r.Outcome != OutcomeSkipped || len(r.Commands) != 0 {
		t.Fatalf("push-workflow: outcome %s, %d gh calls", r.Outcome, len(r.Commands))
	}
	if audit.Partial {
		t.Fatalf("converge-carried row must not read as partial")
	}
}

func TestExecuteEmptySlugRefusesPreAction(t *testing.T) {
	cfg := knobsConfig("github", "local", "use", "none")
	cfg.Slug = ""
	ops := Table(cfg)
	fake := newFake(t)
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err == nil || !strings.Contains(err.Error(), "no repository slug") {
		t.Fatalf("want the named pre-action slug refusal, got %v", err)
	}
	if len(audit.Operations) != 0 || len(fake.calls) != 0 {
		t.Fatalf("operations ran despite the refusal: rows=%v calls=%v", audit.OperationNames(), fake.calls)
	}
}

func TestExecuteLabelsPartialCreate(t *testing.T) {
	ops := Table(knobsConfig("github", "local", "use", "none"))
	all := RoutingLabels()
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, labelsJSON(t, all[:2]), "", 0, "label list") // first two exist
	fake.on(-1, "", "", 0, "label create")
	fake.on(-1, "[]", "", 0, "issue list")
	fake.on(-1, "https://github.com/"+testSlug+"/issues/1", "", 0, "issue create")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	labels := findResult(t, audit, OpEnsureLabels)
	if labels.Outcome != OutcomeApplied {
		t.Fatalf("ensure-labels: %s (%s)", labels.Outcome, labels.Reason)
	}
	if n := fake.countCalls("label create"); n != len(all)-2 {
		t.Fatalf("created %d labels, want %d (only the missing)", n, len(all)-2)
	}
	for _, l := range all[2:] {
		if !strings.Contains(labels.Reason, l.Name) {
			t.Errorf("applied reason misses created label %q: %s", l.Name, labels.Reason)
		}
	}
	if !strings.Contains(labels.Reason, "2 already existed") {
		t.Errorf("applied reason misses the existing count: %s", labels.Reason)
	}
}

func TestExecuteLabelCreateAlreadyExistsIsBenign(t *testing.T) {
	ops := Table(knobsConfig("github", "local", "use", "none"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "[]", "", 0, "label list") // stale read: nothing listed…
	fake.on(-1, "", "label with name local-agent already exists; use `--force` to update its color and description", 1, "label create")
	fake.on(-1, "[]", "", 0, "issue list")
	fake.on(-1, "ok", "", 0, "issue create")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("already-exists on create is the ensured end state, not a failure: %v", err)
	}
	if r := findResult(t, audit, OpEnsureLabels); r.Outcome != OutcomeApplied {
		t.Fatalf("ensure-labels: %s (%s)", r.Outcome, r.Reason)
	}
}

// TestExecuteWelcomeAmbiguousTransientReprobes: the welcome create is the
// table's one non-idempotent mutation — after an ambiguous transient
// failure the op re-probes for the marker and, finding the item landed,
// declares applied WITHOUT a second create (no duplicate).
func TestExecuteWelcomeAmbiguousTransientReprobes(t *testing.T) {
	ops := []Op{findOp(t, Table(knobsConfig("github", "local", "use", "none")), OpWelcomeItem)}
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(1, "[]", "", 0, "issue list")
	fake.on(1, "", "Post \"https://api.github.com/graphql\": read tcp: connection reset by peer", 1, "issue create")
	fake.on(1, `[{"number":9,"body":"welcome\n`+WelcomeMarker+`"}]`, "", 0, "issue list")
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpWelcomeItem)
	if r.Outcome != OutcomeApplied || !strings.Contains(r.Reason, "#9") {
		t.Fatalf("welcome-item: %s (%s)", r.Outcome, r.Reason)
	}
	if n := fake.countCalls("issue create"); n != 1 {
		t.Fatalf("welcome created %d times — the re-probe must prevent the duplicate", n)
	}
	if len(sl.delays) != 0 {
		t.Fatalf("slept %v — the successful re-probe resolves without backoff", sl.delays)
	}
}

func TestExecuteUnknownOpNameFailsWithoutRunning(t *testing.T) {
	ops := []Op{{Name: "nuke-org", Transport: TransportGH, Slug: testSlug}}
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err == nil || !strings.Contains(err.Error(), "nuke-org") {
		t.Fatalf("undeclared op must fail by name, got %v", err)
	}
	r := findResult(t, audit, "nuke-org")
	if r.Outcome != OutcomeFailed || !strings.Contains(r.Reason, "undeclared operation") {
		t.Fatalf("row: %s (%s)", r.Outcome, r.Reason)
	}
	if len(fake.calls) != 1 { // the preflight only
		t.Fatalf("an undeclared op reached gh: %v", fake.calls)
	}
}

func TestExecuteContextCancelledDuringBackoffAborts(t *testing.T) {
	ops := Table(knobsConfig("carina", "local", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "gh: Service Unavailable (HTTP 503)", 1, "protection")
	sl := &sleepRecorder{err: context.Canceled}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want the cancellation abort, got %v", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeFailed || !strings.HasPrefix(r.Reason, "aborted:") {
		t.Fatalf("row: %s (%s)", r.Outcome, r.Reason)
	}
	if !audit.Partial {
		t.Fatalf("aborted walk must read partial")
	}
}

// TestExecuteProtection404AdminHandsOff: GitHub hides admin-only endpoints
// behind 404 for non-admin credentials — the charter's 404-admin case is a
// permission hand-off, never a hard failure.
func TestExecuteProtection404AdminHandsOff(t *testing.T) {
	ops := Table(knobsConfig("carina", "local", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "gh: Not Found (HTTP 404)", 1, "protection")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("the 404-admin case is UB-4, not an error: %v", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "404") {
		t.Fatalf("set-protection: %s (%s)", r.Outcome, r.Reason)
	}
	if !strings.Contains(audit.HandOff, shellJoin(ops[0].protectionArgv())) {
		t.Fatalf("hand-off misses the PUT:\n%s", audit.HandOff)
	}
}

// TestExecuteRepoMissingIsAFailureNotAHandOff: with wiring.repo=use and a
// slug that does not exist, the miss is a config defect — named failure,
// error returned — never silently handed off as a permission gap.
// The #897 field failure: gh's read paths lag a same-walk repo creation,
// returning a blank body (exit 0) for the first probe. The walk re-probes
// instead of believing it, and the run stays whole.
func TestExecuteFreshRepoBlankProbeReprobes(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "none"))
	fake := newFake(t)
	fake.on(1, "", "", 0, "label list") // the replication window: blank, exit 0
	fake.on(1, "", "", 0, "issue list")
	withDefaults(fake)
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if audit.Partial || audit.HandOff != "" {
		t.Fatalf("lag within grace must not degrade the run: partial=%v handoff=%q", audit.Partial, audit.HandOff)
	}
	for _, name := range []string{OpEnsureLabels, OpWelcomeItem} {
		if r := findResult(t, audit, name); r.Outcome != OutcomeApplied {
			t.Errorf("%s: %s (%s)", name, r.Outcome, r.Reason)
		}
	}
	if len(sl.delays) != 2 {
		t.Errorf("re-probe backoffs = %v, want one per lagging probe", sl.delays)
	}
	if got := fake.countCalls("label", "list"); got != 2 {
		t.Errorf("label list ran %d time(s), want 2 (blank, then settled)", got)
	}
}

// Lag that outlives the grace hands off green (UB-4's lane) — a read-side
// hiccup on a repository this walk just created must never fail the run.
func TestExecuteFreshRepoNotFoundPersistsHandsOff(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "none"))
	fake := newFake(t)
	ghost := "GraphQL: Could not resolve to a Repository with the name '" + testSlug + "'."
	fake.on(-1, "", ghost, 1, "label list")
	fake.on(-1, "", ghost, 1, "issue list")
	withDefaults(fake)
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake, execOpts(sl)...)
	if err != nil {
		t.Fatalf("hand-offs are not errors (UB-4): %v", err)
	}
	for _, name := range []string{OpEnsureLabels, OpWelcomeItem} {
		r := findResult(t, audit, name)
		if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "read paths had not caught up") {
			t.Errorf("%s: %s (%s)", name, r.Outcome, r.Reason)
		}
	}
	if audit.HandOff == "" {
		t.Error("persistent lag must leave the remedial commands in the hand-off block")
	}
	if fake.countCalls("label", "create") != 0 {
		t.Error("labels were created against a repository the reads never confirmed")
	}
}

// A blank that persists through the grace still converges: no labels
// visible means every routing label is created, and REST creates work
// inside the window ("already exists" stays benign on the other side).
func TestExecuteFreshRepoBlankPersistsCreatesAll(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "none"))
	fake := newFake(t)
	fake.on(-1, "", "", 0, "label list")
	fake.on(-1, "", "", 0, "issue list")
	withDefaults(fake)
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	labels := findResult(t, audit, OpEnsureLabels)
	if labels.Outcome != OutcomeApplied {
		t.Fatalf("ensure-labels: %s (%s)", labels.Outcome, labels.Reason)
	}
	if got, want := fake.countCalls("label", "create"), len(RoutingLabels()); got != want {
		t.Errorf("created %d label(s), want all %d (a blank list means none are visible)", got, want)
	}
	if r := findResult(t, audit, OpWelcomeItem); r.Outcome != OutcomeApplied {
		t.Errorf("welcome-item: %s (%s)", r.Outcome, r.Reason)
	}
}

func TestExecuteRepoMissingIsAFailureNotAHandOff(t *testing.T) {
	ops := Table(knobsConfig("github", "local", "use", "none"))
	fake := newFake(t)
	fake.on(-1, "gh version 2.62.0 (fake)", "", 0, "--version")
	fake.on(-1, "", "GraphQL: Could not resolve to a Repository with the name '"+testSlug+"'.", 1, "label list")
	fake.on(-1, "", "GraphQL: Could not resolve to a Repository with the name '"+testSlug+"'.", 1, "issue list")
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err == nil {
		t.Fatalf("a missing target repository must surface as an error")
	}
	for _, name := range []string{OpEnsureLabels, OpWelcomeItem} {
		if r := findResult(t, audit, name); r.Outcome != OutcomeFailed || !strings.Contains(r.Reason, "not found") {
			t.Errorf("%s: %s (%s)", name, r.Outcome, r.Reason)
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not name %s: %v", name, err)
		}
	}
	if audit.HandOff != "" {
		t.Errorf("failures are not handed off:\n%s", audit.HandOff)
	}
}

// TestAuditShapeEVT4 asserts the EVT-4 record end to end: event name, run
// id, the operations list, the partial flag, per-op timing, and the
// unredacted argv — plus the structured wiring_applied slog emission.
func TestAuditShapeEVT4(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "checks+review"))
	fake := withDefaults(newFake(t))
	var logBuf bytes.Buffer
	sl := &sleepRecorder{}
	audit, err := Execute(context.Background(), ops, fake,
		WithLogger(slog.New(slog.NewJSONHandler(&logBuf, nil))),
		WithSleeper(sl.sleep),
		WithNow(tick()),
		WithRunID("run-42"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if audit.Event != "wiring_applied" { // EVT-4's literal event name
		t.Errorf("event %q, want wiring_applied", audit.Event)
	}
	if audit.RunID != "run-42" {
		t.Errorf("run id %q", audit.RunID)
	}
	if got, want := audit.OperationNames(), opNames(ops); !equalStrings(got, want) {
		t.Errorf("operations %v != declared %v", got, want)
	}
	if audit.Partial {
		t.Errorf("partial true on a complete run")
	}
	for _, r := range audit.Operations {
		if r.Duration <= 0 {
			t.Errorf("%s: duration %v not recorded", r.Name, r.Duration)
		}
		if r.Transport == TransportGH && len(r.Commands) == 0 {
			t.Errorf("%s: no gh argv recorded", r.Name)
		}
	}
	// Redact nothing: the welcome body — marker included — is in the argv.
	welcome := findResult(t, audit, OpWelcomeItem)
	var sawBody bool
	for _, c := range welcome.Commands {
		for _, a := range c {
			if strings.Contains(a, WelcomeMarker) && strings.Contains(a, "First steps") {
				sawBody = true
			}
		}
	}
	if !sawBody {
		t.Errorf("welcome argv does not carry the full body verbatim: %v", welcome.Commands)
	}
	logs := logBuf.String()
	for _, want := range []string{
		`"msg":"wiring_applied"`,
		`"run_id":"run-42"`,
		`"partial":false`,
		`"msg":"wire: operation start"`,
		`"msg":"wire: operation done"`,
	} {
		if !strings.Contains(logs, want) {
			t.Errorf("structured log misses %s:\n%s", want, logs)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		stderr string
		want   failureClass
	}{
		{"gh: Must have admin rights to Repository. (HTTP 403)", classPermission},
		{"gh: Resource not accessible by integration (HTTP 403)", classPermission},
		{"gh: Forbidden (HTTP 403)", classPermission},
		{"gh: API rate limit exceeded for installation ID 123. (HTTP 403)", classTransient}, // rate limit beats the 403
		{"gh: You have exceeded a secondary rate limit. (HTTP 429)", classTransient},
		{"gh: Internal Server Error (HTTP 500)", classTransient},
		{"gh: Bad Gateway (HTTP 502)", classTransient},
		{"gh: Service Unavailable (HTTP 503)", classTransient},
		{"Post \"https://api.github.com/graphql\": dial tcp: lookup api.github.com: no such host", classTransient},
		{"read tcp 10.0.0.1:443: connection reset by peer", classTransient},
		{"net/http: request canceled (Client.Timeout exceeded)", classTransient},
		{"gh: Not Found (HTTP 404)", classNotFound},
		{"GraphQL: Could not resolve to a Repository with the name 'o/r'. (resolveRepository)", classNotFound},
		{"gh: Validation Failed (HTTP 422)", classOther},
		{"something entirely unexpected", classOther},
	}
	for _, tc := range tests {
		if got := classify(tc.stderr); got != tc.want {
			t.Errorf("classify(%q) = %s, want %s", tc.stderr, got, tc.want)
		}
	}
}

// A protection PUT answering "Branch not found" on a repo this run created
// is the structural create-flow case (#936): init commits locally and never
// pushes, so the branch cannot exist yet. The hand-off must say push-first —
// re-authenticating (the old diagnosis) changes nothing.
func TestExecuteProtectionOnUnpushedCreatedRepoDefersToFirstPush(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "", "gh: Branch not found (HTTP 404)", 1, "branches/main/protection")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v (a hand-off is partial, never a failure)", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeHandedOff {
		t.Fatalf("set-protection: %s (%s), want handed-off", r.Outcome, r.Reason)
	}
	if !strings.Contains(r.Reason, "first push") {
		t.Errorf("reason does not name the push-first moment: %s", r.Reason)
	}
	if strings.Contains(r.Reason, "lacks admin") {
		t.Errorf("branch-absent 404 still misdiagnosed as a permission gap: %s", r.Reason)
	}
	if !audit.Partial || !strings.Contains(audit.HandOff, "branches/main/protection") {
		t.Errorf("hand-off block must carry the verbatim command:\n%s", audit.HandOff)
	}
}

// The same "Branch not found" answer on a pre-existing (empty) repository —
// repoCreated is false, so only the message identifies the case.
func TestExecuteProtectionOnEmptyExistingRepoDefersToFirstPush(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "", "gh: Branch not found (HTTP 404)", 1, "branches/main/protection")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "first push") {
		t.Fatalf("set-protection: %s (%s), want the push-first hand-off", r.Outcome, r.Reason)
	}
}

// A bare "Not Found" 404 (no branch-absent message, repo not created this
// run) keeps the charter's admin-hidden diagnosis.
func TestExecuteProtectionAdminHidden404KeepsPermissionDiagnosis(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "use", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "", "gh: Not Found (HTTP 404)", 1, "branches/main/protection")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "lacks admin") {
		t.Fatalf("set-protection: %s (%s), want the admin-hidden diagnosis", r.Outcome, r.Reason)
	}
	if strings.Contains(r.Reason, "first push") {
		t.Errorf("admin-hidden 404 wrongly deferred to first push: %s", r.Reason)
	}
}

// The repoCreated trigger stands alone (#936): a repository created THIS
// run cannot have its branch yet (init never pushes), so even a bare
// "Not Found" 404 — the replication window's shape, or GitHub message
// drift — defers to first push rather than claiming a permission gap.
func TestExecuteProtectionBare404OnCreatedRepoStillDefersToFirstPush(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "create", "required-checks"))
	fake := newFake(t)
	fake.on(-1, "", "gh: Not Found (HTTP 404)", 1, "branches/main/protection")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpSetProtection)
	if r.Outcome != OutcomeHandedOff || !strings.Contains(r.Reason, "first push") {
		t.Fatalf("set-protection: %s (%s), want the push-first hand-off via repoCreated alone", r.Outcome, r.Reason)
	}
}

// onboardingConfig builds a Config whose table declares the onboarding op
// with two standing items.
func onboardingConfig(protection string) Config {
	cfg := knobsConfig("github", "both", "create", protection)
	cfg.OnboardingItems = []WorkItem{
		{Slug: "author-product-vision", Title: "Author your product vision", Body: "the north star"},
		{Slug: "seed-the-code-gate", Title: "Seed the code gate", Body: "run the seed verb"},
	}
	return cfg
}

// The onboarding op files every missing standing item and — because the
// protection PUT handed off — the derived finish-the-wiring item, whose
// body carries the exact handed-off command (#947). Created refs are
// collected in order.
func TestExecuteOnboardingItemsFilesStandingAndDerivedItems(t *testing.T) {
	ops := Table(onboardingConfig("required-checks"))
	fake := newFake(t)
	// Fires ONCE (the protection PUT itself): the derived item's create
	// carries the protection command inside its --body text, so an
	// unbounded substring rule would wrongly fail that create too.
	fake.on(1, "", "gh: Branch not found (HTTP 404)", 1, "branches/main/protection")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpOnboardingItems)
	if r.Outcome != OutcomeApplied || !strings.Contains(r.Reason, "filed 3 onboarding item(s)") {
		t.Fatalf("onboarding-items: %s (%s), want 3 filed", r.Outcome, r.Reason)
	}
	if len(r.Refs) != 3 {
		t.Fatalf("refs = %v, want 3 created references", r.Refs)
	}
	// The derived item's create carries the protection command inside its
	// fenced hand-off block.
	if n := fake.countCalls("issue", "create", "branches/main/protection"); n != 1 {
		t.Errorf("finish-the-wiring item creates carrying the protection command = %d, want 1", n)
	}
	// Standing items carry their markers in the body argv.
	if n := fake.countCalls("issue", "create", OnboardingMarker("author-product-vision")); n != 1 {
		t.Errorf("vision item marker missing from the create argv")
	}
}

// Items whose markers the probe already sees are skipped byte-untouched
// (AP-22); only the missing ones are created.
func TestExecuteOnboardingItemsSkipsExistingMarkers(t *testing.T) {
	ops := Table(onboardingConfig("none"))
	fake := newFake(t)
	existing := `[{"number":7,"body":"seeded earlier ` + OnboardingMarker("author-product-vision") + `"}]`
	fake.on(-1, existing, "", 0, "issue list")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpOnboardingItems)
	if r.Outcome != OutcomeApplied || !strings.Contains(r.Reason, "filed 1 onboarding item(s); 1 already existed") {
		t.Fatalf("onboarding-items: %s (%s), want 1 filed / 1 existed", r.Outcome, r.Reason)
	}
	if n := fake.countCalls("issue", "create", OnboardingMarker("author-product-vision")); n != 0 {
		t.Errorf("an existing item was re-created (%d creates)", n)
	}
}

// A permission wall mid-set hands off every REMAINING create verbatim —
// the walked prefix stays applied, the block completes the rest (UB-4).
func TestExecuteOnboardingItemsPermissionHandsOffRemainder(t *testing.T) {
	ops := Table(onboardingConfig("none"))
	fake := newFake(t)
	fake.on(-1, "", "HTTP 403: Resource not accessible by integration", 1, "issue", "create")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpOnboardingItems)
	if r.Outcome != OutcomeHandedOff {
		t.Fatalf("onboarding-items: %s (%s), want handed-off", r.Outcome, r.Reason)
	}
	for _, want := range []string{"Author your product vision", "Seed the code gate"} {
		if !strings.Contains(audit.HandOff, want) {
			t.Errorf("hand-off block lacks the remaining create for %q:\n%s", want, audit.HandOff)
		}
	}
}

// An empty OnboardingItems set declares NO onboarding row at all — the
// table stays the legacy shape (backwards compatibility for every caller
// that never sets the field).
func TestTableDeclaresOnboardingOnlyWhenItemsExist(t *testing.T) {
	bare := Table(knobsConfig("github", "both", "create", "none"))
	for _, op := range bare {
		if op.Name == OpOnboardingItems {
			t.Fatalf("empty item set still declared %s", OpOnboardingItems)
		}
	}
	withItems := Table(onboardingConfig("none"))
	last := withItems[len(withItems)-1]
	if last.Name != OpOnboardingItems {
		t.Fatalf("onboarding row must be declared LAST (it derives from the walk's hand-offs); table tail = %s", last.Name)
	}
	for _, it := range last.Items {
		if !strings.Contains(it.Body, OnboardingMarker(it.Slug)) {
			t.Errorf("item %q body lacks its marker after Table", it.Slug)
		}
	}
}

// A marker inside a fenced code block is quoted text, never presence: the
// finish-the-wiring item embeds handed-off `issue create` commands (whose
// --body strings carry other items' markers) inside a ```sh fence, and
// those must not read as the items they would create (the marker-poisoning
// class the #947 review caught — a welcome probe once matched the wiring
// item and linked the guide to the wrong issue).
func TestFindMarkerIgnoresFencedQuotes(t *testing.T) {
	body := "run this later:\n\n```sh\ngh issue create --body 'x " + WelcomeMarker + "'\n```\n"
	list := `[{"number":3,"body":` + strconv.Quote(body) + `}]`
	if _, found, err := findMarker(list, WelcomeMarker); err != nil || found {
		t.Fatalf("fenced marker matched (found=%v err=%v) — quoted commands must not poison probes", found, err)
	}
	real := `[{"number":4,"body":` + strconv.Quote("intro\n"+WelcomeMarker+"\n") + `}]`
	if num, found, err := findMarker(real, WelcomeMarker); err != nil || !found || num != 4 {
		t.Fatalf("prose marker missed (num=%d found=%v err=%v)", num, found, err)
	}
}

// All standing items already existing is idempotence, not application: the
// audit row reads skipped (AP-22), matching ensure-labels' taxonomy.
func TestExecuteOnboardingItemsAllExistingSkips(t *testing.T) {
	ops := Table(onboardingConfig("none"))
	fake := newFake(t)
	existing := `[{"number":7,"body":"a ` + OnboardingMarker("author-product-vision") + `"},{"number":8,"body":"b ` + OnboardingMarker("seed-the-code-gate") + `"}]`
	fake.on(-1, existing, "", 0, "issue list")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpOnboardingItems)
	if r.Outcome != OutcomeSkipped || !strings.Contains(r.Reason, "already exist") {
		t.Fatalf("all-existing: %s (%s), want skipped (AP-22)", r.Outcome, r.Reason)
	}
	// The welcome op legitimately creates its own item; no ONBOARDING
	// create may run (every item body carries the onboarding marker).
	if n := fake.countCalls("issue", "create", "pyxis:onboarding"); n != 0 {
		t.Errorf("all-existing walk still ran %d onboarding create(s)", n)
	}
}

// An ambiguous create failure that actually landed converges by re-probe —
// the item counts as filed with no second POST (the welcome op's
// discipline, held for every onboarding item).
func TestExecuteOnboardingItemsAmbiguousLandingConverges(t *testing.T) {
	ops := Table(onboardingConfig("none"))
	fake := newFake(t)
	// The 502 fires only on the vision item's own create; the first two
	// issue lists (the welcome probe, the onboarding probe) see an empty
	// repo, and the third — the vision item's ambiguity re-probe — shows
	// its marker landed.
	fake.on(1, "", "HTTP 502 server error", 1, "issue", "create", "author-product-vision")
	fake.on(2, "[]", "", 0, "issue list")
	fake.on(-1, `[{"number":5,"body":"landed `+OnboardingMarker("author-product-vision")+`"}]`, "", 0, "issue list")
	withDefaults(fake)

	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpOnboardingItems)
	if r.Outcome != OutcomeApplied || !strings.Contains(r.Reason, "filed 2 onboarding item(s)") {
		t.Fatalf("ambiguous landing: %s (%s), want both items filed", r.Outcome, r.Reason)
	}
	if n := fake.countCalls("issue", "create", "author-product-vision"); n != 1 {
		t.Errorf("the landed item was re-POSTed (%d creates)", n)
	}
}

// The welcome-item op collects the guide's reference in every landing shape
// (#948): a fresh create takes gh's stdout URL; an already-existing marker
// derives the URL from the probe's issue number.
func TestExecuteWelcomeItemCollectsRef(t *testing.T) {
	ops := Table(knobsConfig("github", "both", "use", "none"))
	fake := withDefaults(newFake(t))
	audit, err := Execute(context.Background(), ops, fake, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r := findResult(t, audit, OpWelcomeItem)
	if len(r.Refs) != 1 || r.Refs[0] != "https://github.com/"+testSlug+"/issues/1" {
		t.Fatalf("created welcome refs = %v, want the create's stdout URL", r.Refs)
	}

	existing := newFake(t)
	existing.on(-1, `[{"number":9,"body":"carried `+WelcomeMarker+`"}]`, "", 0, "issue list")
	withDefaults(existing)
	audit2, err := Execute(context.Background(), ops, existing, execOpts(&sleepRecorder{})...)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	r2 := findResult(t, audit2, OpWelcomeItem)
	if r2.Outcome != OutcomeSkipped || len(r2.Refs) != 1 || r2.Refs[0] != "https://github.com/"+testSlug+"/issues/9" {
		t.Fatalf("existing welcome: %s refs %v, want the derived issue URL", r2.Outcome, r2.Refs)
	}
}
