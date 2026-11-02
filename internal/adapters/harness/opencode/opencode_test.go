package opencode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// --- fake exec ----------------------------------------------------------------

// fakeProcess is one recorded child: wait replays fixed output, kill records.
type fakeProcess struct {
	stdout  []byte
	code    int
	waitErr error
	killErr error
	waited  bool
	killed  bool
}

func (p *fakeProcess) wait() ([]byte, int, error) {
	p.waited = true
	return p.stdout, p.code, p.waitErr
}

func (p *fakeProcess) kill() error {
	p.killed = true
	return p.killErr
}

// fakeCommander records every start/run and replays configured results, so the
// adapter runs with no real `opencode` binary (docs/tech-design/implement-now.md §3.10).
type fakeCommander struct {
	// recorded calls
	starts []commandSpec
	runs   []commandSpec

	// start() behaviour
	startErr    error
	procStdout  []byte
	procCode    int
	procWaitErr error
	procs       []*fakeProcess

	// run() behaviour (AuthCheck install probe)
	runStdout []byte
	runCode   int
	runErr    error
	// Optional result for the local-policy model-availability preflight.
	modelsStdout []byte
	modelsCode   int
	modelsErr    error
}

func (c *fakeCommander) start(_ context.Context, spec commandSpec) (process, error) {
	c.starts = append(c.starts, spec)
	if c.startErr != nil {
		return nil, c.startErr
	}
	p := &fakeProcess{stdout: c.procStdout, code: c.procCode, waitErr: c.procWaitErr}
	c.procs = append(c.procs, p)
	return p, nil
}

func (c *fakeCommander) run(_ context.Context, spec commandSpec) ([]byte, int, error) {
	c.runs = append(c.runs, spec)
	if len(spec.Args) == 1 && spec.Args[0] == "models" {
		return c.modelsStdout, c.modelsCode, c.modelsErr
	}
	return c.runStdout, c.runCode, c.runErr
}

// resultJSON is a representative `opencode run --format json` result object.
// PROVISIONAL — opencode is not installed, so this golden shape is the
// re-confirmation target at deployment (§7 Q4); if the live shape differs, this
// fixture and parseResult's struct change together. Tokens = input+output+
// reasoning+cache(read+write) = 1000+200+0+4+30 = 1234, steps (turns) = 7.
const resultJSON = `{"tokens":{"input":1000,"output":200,"reasoning":0,"cache":{"read":4,"write":30}},` +
	`"cost":0.12,"steps":7}`

var wantResult = port.RunResult{Tokens: 1234, Turns: 7, TokensObserved: true, TurnsObserved: true}

func TestParseResult_CurrentJSONLEventsCaptureFinalTextAndUsage(t *testing.T) {
	stdout := []byte(strings.Join([]string{
		`{"type":"step_start","part":{"type":"step-start"}}`,
		`{"type":"step_finish","part":{"type":"step-finish","reason":"tool-calls","tokens":{"input":10,"output":2,"reasoning":1,"cache":{"read":3,"write":4}}}}`,
		`{"type":"text","part":{"type":"text","text":"complete artifact"}}`,
		`{"type":"step_finish","part":{"type":"step-finish","reason":"stop","tokens":{"input":20,"output":5,"reasoning":2,"cache":{"read":6,"write":7}}}}`,
	}, "\n"))
	got, err := parseResult(stdout)
	if err != nil {
		t.Fatal(err)
	}
	want := port.RunResult{Tokens: 60, Turns: 2, Output: "complete artifact", TokensObserved: true, TurnsObserved: true, ToolCallsObserved: true}
	if got != want {
		t.Fatalf("parseResult = %+v, want %+v", got, want)
	}
}

func mustAdapter(t *testing.T, opts ...Option) *Adapter {
	t.Helper()
	cfg, err := harnessconfigtest.Config()
	if err != nil {
		t.Fatal(err)
	}
	opts = append([]Option{WithProfiles(cfg.Profiles(harnessID))}, opts...)
	a, err := New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestReviewLaunchKeepsJSONStreamAndHostConfinedTools(t *testing.T) {
	spec := port.LaunchSpec{
		Purpose: port.LaunchPurposeReview, Model: "anthropic/claude-opus-4-8", Effort: "xhigh",
		Brief: "review", Workdir: t.TempDir(), RuntimeStateDir: t.TempDir(),
	}
	fc := &fakeCommander{}
	a := mustAdapter(t, withCommander(fc))
	args := strings.Join(a.launchArgs(spec, "review"), " ")
	for _, want := range []string{"--format json", "--agent build"} {
		if !strings.Contains(args, want) {
			t.Fatalf("review argv %q omitted %q", args, want)
		}
	}
	if strings.Contains(args, "--pure") {
		t.Fatalf("review argv %q combines --pure with the required JSON stream", args)
	}
	if strings.Contains(args, "--auto") {
		t.Fatalf("review argv retained auto-approval: %q", args)
	}
	if _, err := a.Launch(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	got := fc.starts[0]
	if got.Access == nil || got.Access.Purpose != port.LaunchPurposeReview || got.Access.Workdir != spec.Workdir || got.Access.RuntimeStateDir != spec.RuntimeStateDir {
		t.Fatalf("review launch lost the shared access policy: %+v", got.Access)
	}
	env := "\n" + strings.Join(fc.starts[0].Env, "\n") + "\n"
	for _, want := range []string{"\nOPENCODE_PERMISSION={\"*\":\"allow\"}\n", "\nOPENCODE_DISABLE_DEFAULT_PLUGINS=true\n", "\nOPENCODE_AUTO_SHARE=false\n"} {
		if !strings.Contains(env, want) {
			t.Fatalf("review environment omitted %q: %s", want, env)
		}
	}
}

func fixtureSpec() port.LaunchSpec {
	return port.LaunchSpec{
		Brief:   "implement the widget per the charter",
		Workdir: "/tmp/wt-42",
		Model:   "anthropic/claude-opus-4-8",
		Effort:  "high",
	}
}

// --- command construction -----------------------------------------------------

func TestLaunch_CommandConstruction(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))

	if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if len(fc.starts) != 1 {
		t.Fatalf("start called %d times, want 1", len(fc.starts))
	}
	got := fc.starts[0]
	if got.Access == nil || got.Access.Workdir != fixtureSpec().Workdir || got.Access.Purpose != fixtureSpec().Purpose {
		t.Fatalf("implementation launch missing OS access policy: %+v", got.Access)
	}
	if got.Name != "opencode" {
		t.Errorf("binary = %q, want opencode", got.Name)
	}
	if got.Dir != "/tmp/wt-42" {
		t.Errorf("workdir = %q, want /tmp/wt-42", got.Dir)
	}
	// The brief is delivered as the trailing `run` message (positional), not stdin.
	if got.Stdin != "" {
		t.Errorf("stdin = %q, want empty (opencode takes the brief as the run message)", got.Stdin)
	}
	if len(got.Args) == 0 {
		t.Fatal("no args recorded")
	}
	last := got.Args[len(got.Args)-1]
	if !strings.Contains(last, "implement the widget") {
		t.Errorf("last arg (the run message) missing the charter: %q", last)
	}

	args := strings.Join(got.Args, " ")
	for _, want := range []string{
		"run",
		"--format json",
		"--model anthropic/claude-opus-4-8",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	// opencode-specific: NOT the claude/cursor flags. It has no native turn cap
	// and no CLI effort flag, and its format flag is --format (not --output-format).
	for _, unwanted := range []string{"--output-format", "--max-turns", "--effort", "--permission-mode"} {
		if strings.Contains(args, unwanted) {
			t.Errorf("args %q must not contain %q (opencode has no such flag)", args, unwanted)
		}
	}
}

// DINV-11: every task gets a fresh process and no session is ever resumed.
func TestLaunch_FreshProcessNoSessionReuse(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))

	for i := range 3 {
		if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
			t.Fatalf("Launch %d: %v", i, err)
		}
	}
	if len(fc.starts) != 3 {
		t.Fatalf("start called %d times, want 3 (a fresh process per task)", len(fc.starts))
	}
	// opencode's session-resume surface is `--session <id>` / `-c` (continue) /
	// `resume`. None may ever appear: a fresh process per task is the whole point.
	banned := map[string]bool{
		"--session": true, "-c": true, "--continue": true, "resume": true, "--resume": true,
	}
	for _, s := range fc.starts {
		for _, arg := range s.Args {
			if banned[arg] {
				t.Errorf("launch args must never reuse a session, found %q in %v", arg, s.Args)
			}
		}
	}
}

func TestLaunch_RejectsUnresolvedTier(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))
	for _, spec := range []port.LaunchSpec{
		{Brief: "b", Workdir: "/w", Model: "", Effort: "high"},
		{Brief: "b", Workdir: "/w", Model: "anthropic/m", Effort: ""},
	} {
		if _, err := a.Launch(context.Background(), spec); err == nil {
			t.Errorf("Launch with model=%q effort=%q must fail closed", spec.Model, spec.Effort)
		}
	}
	if len(fc.starts) != 0 {
		t.Errorf("no process must be spawned for an unresolved model, got %d", len(fc.starts))
	}
}

func TestLaunch_EmptyBriefFailsClosed(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))
	spec := fixtureSpec()
	spec.Brief = "   "
	if _, err := a.Launch(context.Background(), spec); err == nil {
		t.Fatal("Launch with an empty charter must fail closed")
	}
	if len(fc.starts) != 0 {
		t.Errorf("no process must be spawned without a brief, got %d", len(fc.starts))
	}
}

func TestLaunch_StartError(t *testing.T) {
	fc := &fakeCommander{startErr: errors.New("exec: not found")}
	a := mustAdapter(t, withCommander(fc))
	if _, err := a.Launch(context.Background(), fixtureSpec()); err == nil {
		t.Fatal("Launch must surface a start failure")
	}
}

// --- Await / JSON parsing -----------------------------------------------------

func TestAwait_ParsesUsageJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		code    int
		want    port.RunResult
		wantErr bool
	}{
		{name: "full usage, exit 0", json: resultJSON, code: 0, want: wantResult},
		{
			name:    "non-zero exit remains an operational failure",
			json:    `{"tokens":{"input":5,"output":5},"steps":40}`,
			code:    1,
			wantErr: true,
		},
		{
			name: "non-zero native error remains recoverable incomplete",
			json: `{"type":"error","error":"stream disconnected"}`,
			code: 1,
			want: port.RunResult{Incomplete: true, Reason: "opencode_result_error"},
		},
		{
			name: "missing usage fields default to zero",
			json: `{"steps":2}`,
			code: 0,
			want: port.RunResult{Tokens: 0, Turns: 2, TurnsObserved: true},
		},
		{
			name: "unknown fields are ignored (open parse)",
			json: `{"steps":3,"brand_new_field":true,"tokens":{"input":7,"output":0}}`,
			code: 0,
			want: port.RunResult{Tokens: 7, Turns: 3, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "leading/trailing whitespace tolerated",
			json: "\n  " + resultJSON + "\n",
			code: 0,
			want: wantResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &fakeCommander{procStdout: []byte(tt.json), procCode: tt.code}
			a := mustAdapter(t, withCommander(fc))
			h, err := a.Launch(context.Background(), fixtureSpec())
			if err != nil {
				t.Fatalf("Launch: %v", err)
			}
			got, err := a.Await(context.Background(), h)
			if len(fc.procs) != 1 || !fc.procs[0].killed {
				t.Fatal("Await did not quiesce the completed agent process group")
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("Await returned nil error for failed CLI run")
				}
				return
			}
			if err != nil {
				t.Fatalf("Await: %v", err)
			}
			if got != tt.want {
				t.Fatalf("RunResult = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAwait_OperationalErrorsAndBadJSON(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		waitErr error
	}{
		{"process could not be awaited", "", errors.New("killed by signal")},
		{"empty output", "", nil},
		{"unparseable output", "this is not json", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &fakeCommander{procStdout: []byte(tt.stdout), procWaitErr: tt.waitErr}
			a := mustAdapter(t, withCommander(fc))
			h, err := a.Launch(context.Background(), fixtureSpec())
			if err != nil {
				t.Fatalf("Launch: %v", err)
			}
			if _, err := a.Await(context.Background(), h); err == nil {
				t.Fatalf("Await must return an error for %q", tt.name)
			}
		})
	}
}

func TestAwait_ContextCancelKillsAndReturns(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))
	h, err := a.Launch(context.Background(), fixtureSpec())
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	if _, err := a.Await(ctx, h); !errors.Is(err, context.Canceled) {
		t.Fatalf("Await err = %v, want context.Canceled", err)
	}
	if len(fc.procs) != 1 || !fc.procs[0].killed {
		t.Fatalf("a cancelled Await must kill the process")
	}
}

func TestAwait_ForeignHandle(t *testing.T) {
	a := mustAdapter(t, withCommander(&fakeCommander{}))
	if _, err := a.Await(context.Background(), foreignHandle{}); err == nil {
		t.Fatal("Await must reject a foreign handle")
	}
}

// --- Kill ---------------------------------------------------------------------

func TestKill(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))
	h, err := a.Launch(context.Background(), fixtureSpec())
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := a.Kill(context.Background(), h); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if len(fc.procs) != 1 || !fc.procs[0].killed {
		t.Fatal("Kill must terminate the launched process")
	}
}

func TestKill_ForeignHandle(t *testing.T) {
	a := mustAdapter(t, withCommander(&fakeCommander{}))
	if err := a.Kill(context.Background(), foreignHandle{}); err == nil {
		t.Fatal("Kill must reject a foreign handle")
	}
}

type foreignHandle struct{}

func (foreignHandle) RuntimeHandle() {}

// --- AuthCheck ----------------------------------------------------------------

func TestConfiguredModelsRejectMalformedProviderIDs(t *testing.T) {
	for _, model := range []string{"unqualified", "/model", "provider/", "-provider/model", "provider/model\n--flag"} {
		profiles := map[nativerouting.Role][]nativerouting.ProfileSpec{
			nativerouting.RoleImplementation: {{ID: "implementation-economy", Model: model, Effort: "none", CostRank: 1, Strongest: true}},
		}
		if _, err := New(WithProfiles(profiles)); err == nil {
			t.Errorf("accepted invalid model %q", model)
		}
	}
}

func TestAuthCheck_MissingBinary(t *testing.T) {
	fc := &fakeCommander{runErr: errors.New("exec: \"opencode\": not found")}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	err := a.AuthCheck(context.Background())
	if err == nil {
		t.Fatal("AuthCheck must fail when the binary is missing")
	}
	if !strings.Contains(err.Error(), "install") {
		t.Errorf("missing-binary error should state the remedy, got %q", err)
	}
	// The auth probe is never reached, and no attempt is consumed.
}

func TestAuthCheck_VersionNonZero(t *testing.T) {
	fc := &fakeCommander{runCode: 1}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	if err := a.AuthCheck(context.Background()); err == nil {
		t.Fatal("AuthCheck must fail when `opencode --version` exits non-zero")
	}
}

func TestAuthCheck_Unauthenticated(t *testing.T) {
	fc := &fakeCommander{runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error {
		return errors.New("no credential found")
	}))
	err := a.AuthCheck(context.Background())
	if err == nil {
		t.Fatal("AuthCheck must fail when unauthenticated")
	}
	for _, want := range []string{"unauthenticated", "ANTHROPIC_API_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("auth error should mention %q, got %q", want, err)
		}
	}
}

func TestAuthCheck_OK(t *testing.T) {
	fc := &fakeCommander{runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	if err := a.AuthCheck(context.Background()); err != nil {
		t.Fatalf("AuthCheck must pass when installed and authenticated: %v", err)
	}
	// The install probe runs `opencode --version`.
	if len(fc.runs) != 1 || strings.Join(fc.runs[0].Args, " ") != "--version" {
		t.Fatalf("expected one `opencode --version` probe, got %+v", fc.runs)
	}
}

func TestDefaultAuthProbe(t *testing.T) {
	// Isolate: clear every credential env var and point the store at an empty dir.
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		t.Setenv(k, "")
	}
	empty := t.TempDir()
	t.Setenv("XDG_DATA_HOME", empty)

	if err := defaultAuthProbe(context.Background()); err == nil {
		t.Fatal("no env token and no auth store: probe must report unauthenticated")
	}

	// An env token satisfies the probe.
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("env token present, probe should pass: %v", err)
	}

	// The on-disk opencode auth store satisfies the probe too.
	t.Setenv("ANTHROPIC_API_KEY", "")
	storeDir := filepath.Join(empty, "opencode")
	if err := os.MkdirAll(storeDir, 0o700); err != nil {
		t.Fatalf("seed auth store dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed auth store: %v", err)
	}
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("auth store present, probe should pass: %v", err)
	}
}

// --- self-registration + conformance ------------------------------------------

func TestSelfRegistration(t *testing.T) {
	// Importing this package ran the adapter's init, which registered "opencode".
	rt, err := runtime.Resolve("opencode")
	if err != nil {
		t.Fatalf("Resolve(opencode): %v", err)
	}
	if _, ok := rt.(*Adapter); !ok {
		t.Fatalf("Resolve(opencode) = %T, want *opencode.Adapter", rt)
	}
	found := false
	for _, b := range runtime.Harnesses() {
		if b == "opencode" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Harnesses() %v does not include opencode", runtime.Harnesses())
	}
}

// The OpenCode adapter passes the shared conformance suite (§3.7), driven
// hermetically through a fake exec wired to report a known usage result.
func TestConformance(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON), runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	err := runtime.CheckConformance(context.Background(), a, runtime.ConformanceCase{
		Spec:       fixtureSpec(),
		WantResult: wantResult,
	})
	if err != nil {
		t.Fatalf("opencode adapter failed conformance: %v", err)
	}
	reviewSpec := fixtureSpec()
	reviewSpec.Purpose = port.LaunchPurposeReview
	if err := runtime.CheckFencingConformance(context.Background(), a, runtime.FencingCase{Spec: reviewSpec, Harness: harnessID}); err != nil {
		t.Fatalf("opencode adapter failed fencing conformance: %v", err)
	}
}

// --- live availability gate ---------------------------------------------------

// TestLive_OpenCodeAvailable is the only test that touches a real `opencode`
// binary. It gates on availability and SKIPS cleanly where the CLI is absent
// (the authoring machine, per §7 Q4) — the hermetic tests + conformance above
// are the in-session coverage. Where opencode IS present it drives the real,
// zero-cost install-probe path (`opencode --version`, via AuthCheck): a genuine
// install failure fails the test, while a merely-unauthenticated environment
// (no provider credential in CI) is tolerated, since the point here is that the
// install probe works end-to-end against the live binary, not that CI holds a
// credential. A full task-drive e2e (a real `opencode run` to a green proposal)
// belongs to the dogfood drill (A-12), not this unit gate.
func TestLive_OpenCodeAvailable(t *testing.T) {
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("opencode not installed (%v) — live e2e skipped; hermetic tests + conformance cover the adapter (design-brainstorm:implement-now-tech-design#F-9, §7 Q4)", err)
	}
	a := mustAdapter(t) // real osCommander + real defaultAuthProbe
	if err := a.AuthCheck(context.Background()); err != nil && !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("live opencode install probe failed: %v", err)
	}
}
