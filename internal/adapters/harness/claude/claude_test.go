package claude

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
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
// adapter runs with no real `claude` binary (docs/tech-design/implement-now.md §3.10).
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
	return c.runStdout, c.runCode, c.runErr
}

// resultJSON is a representative `claude --output-format json` result object:
// input+output+cache tokens = 1000+200+30+4 = 1234, num_turns = 7.
const resultJSON = `{"type":"result","subtype":"success","is_error":false,"num_turns":7,` +
	`"session_id":"abc","total_cost_usd":0.12,` +
	`"usage":{"input_tokens":1000,"output_tokens":200,"cache_creation_input_tokens":30,"cache_read_input_tokens":4}}`

var wantResult = port.RunResult{Tokens: 1234, Turns: 7, SessionID: "abc", TokensObserved: true, TurnsObserved: true}

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

func TestReviewLaunchUsesHostGuardForToolPermissions(t *testing.T) {
	spec := port.LaunchSpec{
		Purpose: port.LaunchPurposeReview, Model: "claude-opus-5", Effort: "max",
		Brief: "review", Workdir: "/review", RuntimeStateDir: "/runtime",
	}
	fc := &fakeCommander{}
	a := mustAdapter(t, withCommander(fc))
	if _, err := a.Launch(t.Context(), spec); err != nil {
		t.Fatal(err)
	}
	got := fc.starts[0]
	if got.Access == nil || got.Access.Purpose != port.LaunchPurposeReview || got.Access.Workdir != spec.Workdir || got.Access.RuntimeStateDir != spec.RuntimeStateDir {
		t.Fatalf("review launch lost the shared access policy: %+v", got.Access)
	}
	args := strings.Join(got.Args, " ")
	if !strings.Contains(args, "--permission-mode bypassPermissions") || strings.Contains(args, "--permission-mode plan") {
		t.Fatalf("review argv does not allow host-confined inspection commands: %q", args)
	}
	if !strings.Contains(args, "--settings "+reviewSettings) {
		t.Fatalf("review argv did not disable the nested command sandbox: %q", args)
	}
}

func fixtureSpec() port.LaunchSpec {
	return port.LaunchSpec{
		Brief:   "implement the widget per the charter",
		Workdir: "/tmp/wt-42",
		Model:   "claude-opus-5",
		Effort:  "high",
	}
}

// --- command construction -----------------------------------------------------

func TestLaunch_CommandConstruction(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc), WithMaxTurns(33))

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
	if got.Name != "claude" {
		t.Errorf("binary = %q, want claude", got.Name)
	}
	if got.Dir != "/tmp/wt-42" {
		t.Errorf("workdir = %q, want /tmp/wt-42", got.Dir)
	}
	// The brief is delivered on stdin (never a positional arg) and carries the charter.
	if !strings.Contains(got.Stdin, "implement the widget") {
		t.Errorf("stdin missing the charter: %q", got.Stdin)
	}

	args := strings.Join(got.Args, " ")
	for _, want := range []string{
		"-p",
		"--permission-mode bypassPermissions",
		"--model claude-opus-5",
		"--effort high",
		"--output-format json",
		"--max-turns 33",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
}

// A non-positive turn cap means NO turn cap: `--max-turns` is omitted entirely
// rather than passed as a sentinel the CLI would read as a real bound. The
// launch stays bounded — by the Worker's wall-clock budget, not by turns.
func TestLaunch_NoTurnCapOmitsFlag(t *testing.T) {
	for _, turns := range []int{0, -1} {
		fc := &fakeCommander{procStdout: []byte(resultJSON)}
		a := mustAdapter(t, withCommander(fc), WithMaxTurns(turns))

		if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
			t.Fatalf("WithMaxTurns(%d) Launch: %v", turns, err)
		}
		args := strings.Join(fc.starts[0].Args, " ")
		if strings.Contains(args, "--max-turns") {
			t.Errorf("WithMaxTurns(%d): args %q carry --max-turns, want it omitted", turns, args)
		}
		// The rest of the invocation is unchanged.
		if !strings.Contains(args, "--output-format json") {
			t.Errorf("WithMaxTurns(%d): args %q lost --output-format", turns, args)
		}
	}
}

// The built-in default is no turn cap, so an adapter with no WithMaxTurns option
// omits the flag too — the config seam is not required to opt out.
func TestLaunch_DefaultHasNoTurnCap(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))

	if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if args := strings.Join(fc.starts[0].Args, " "); strings.Contains(args, "--max-turns") {
		t.Errorf("default args %q carry --max-turns, want no turn cap", args)
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
	for _, s := range fc.starts {
		args := strings.Join(s.Args, " ")
		for _, banned := range []string{"--resume", "--continue", "--fork-session", "--session-id"} {
			if strings.Contains(args, banned) {
				t.Errorf("launch args must never reuse a session, found %q in %q", banned, args)
			}
		}
	}
}

func TestLaunch_RejectsUnresolvedTier(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc))
	for _, spec := range []port.LaunchSpec{
		{Brief: "b", Workdir: "/w", Model: "", Effort: "high"},
		{Brief: "b", Workdir: "/w", Model: "m", Effort: ""},
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
			name: "non-zero incomplete result remains recoverable (max-turns)",
			json: `{"num_turns":40,"subtype":"error_max_turns","is_error":true,"usage":{"input_tokens":5,"output_tokens":5}}`,
			code: 1, // agent hit --max-turns; the process still finished
			want: port.RunResult{Tokens: 10, Turns: 40, Incomplete: true, Reason: "error_max_turns", TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "missing usage fields default to zero",
			json: `{"num_turns":2}`,
			code: 0,
			want: port.RunResult{Tokens: 0, Turns: 2, TurnsObserved: true},
		},
		{
			name: "unknown fields are ignored (open parse)",
			json: `{"num_turns":3,"brand_new_field":true,"usage":{"input_tokens":7,"output_tokens":0}}`,
			code: 0,
			want: port.RunResult{Tokens: 7, Turns: 3, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "leading/trailing whitespace tolerated",
			json: "\n  " + resultJSON + "\n",
			code: 0,
			want: wantResult,
		},
		{
			name: "max-turns subtype marks the result incomplete",
			json: `{"num_turns":150,"subtype":"error_max_turns","is_error":true,"usage":{"input_tokens":5,"output_tokens":5}}`,
			code: 0,
			want: port.RunResult{Tokens: 10, Turns: 150, Incomplete: true, Reason: "error_max_turns", TokensObserved: true, TurnsObserved: true},
		},
		{
			name:    "non-zero complete result remains operational failure",
			json:    resultJSON,
			code:    9,
			wantErr: true,
		},
		{
			name: "is_error without a subtype is incomplete with a generic reason",
			json: `{"num_turns":3,"is_error":true,"usage":{"input_tokens":1,"output_tokens":1}}`,
			code: 0,
			want: port.RunResult{Tokens: 2, Turns: 3, Incomplete: true, Reason: "error", TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "success subtype is complete with no reason",
			json: `{"num_turns":7,"subtype":"success","is_error":false,"usage":{"input_tokens":1,"output_tokens":1}}`,
			code: 0,
			want: port.RunResult{Tokens: 2, Turns: 7, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "array transcript form — the result element carries the telemetry (CLI 2.x)",
			json: `[{"type":"system","subtype":"init"},{"type":"assistant"},` + resultJSON + `]`,
			code: 0,
			want: wantResult,
		},
		{
			name: "array with the result element not last is still found by its type marker",
			json: `[` + resultJSON + `,{"type":"assistant"}]`,
			code: 0,
			want: wantResult,
		},
		{
			name: "array with no result marker falls back to the last element",
			json: `[{"type":"assistant"},{"num_turns":9,"usage":{"input_tokens":3,"output_tokens":1}}]`,
			code: 0,
			want: port.RunResult{Tokens: 4, Turns: 9, TokensObserved: true, TurnsObserved: true},
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
					t.Fatal("Await succeeded, want process-exit error")
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
		{"empty result array", "[]", nil},
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

func TestAuthCheck_MissingBinary(t *testing.T) {
	fc := &fakeCommander{runErr: errors.New("exec: \"claude\": not found")}
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
		t.Fatal("AuthCheck must fail when `claude --version` exits non-zero")
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
	// The install probe runs `claude --version`.
	if len(fc.runs) != 1 || strings.Join(fc.runs[0].Args, " ") != "--version" {
		t.Fatalf("expected one `claude --version` probe, got %+v", fc.runs)
	}
}

func TestDefaultAuthProbe(t *testing.T) {
	// Keep the probe hermetic: on a darwin host a real login-Keychain credential
	// would otherwise satisfy the "unauthenticated" case. Stub the Keychain probe
	// absent, restoring it afterwards.
	origKeychain := keychainCredentialExists
	keychainCredentialExists = func() bool { return false }
	defer func() { keychainCredentialExists = origKeychain }()

	// Isolate: clear every credential env var and point the store at an empty dir.
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"} {
		t.Setenv(k, "")
	}
	empty := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", empty)

	if err := defaultAuthProbe(context.Background()); err == nil {
		t.Fatal("no env token and no credential store: probe must report unauthenticated")
	}

	// An env token satisfies the probe.
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("env token present, probe should pass: %v", err)
	}

	// The on-disk credential store satisfies the probe too.
	t.Setenv("ANTHROPIC_API_KEY", "")
	if err := os.WriteFile(filepath.Join(empty, ".credentials.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed credential store: %v", err)
	}
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("credential store present, probe should pass: %v", err)
	}
}

// TestDefaultAuthProbe_MacKeychain covers the darwin fallback: with no env token
// and no credential file, a present login-Keychain item must satisfy the probe
// (the A-12 dogfood false-negative — a logged-in macOS CLI reported
// "unauthenticated" because only the Linux credential file was checked).
func TestDefaultAuthProbe_MacKeychain(t *testing.T) {
	if goruntime.GOOS != "darwin" {
		t.Skip("Keychain fallback is darwin-only")
	}
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"} {
		t.Setenv(k, "")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir()) // no .credentials.json present

	origKeychain := keychainCredentialExists
	defer func() { keychainCredentialExists = origKeychain }()

	keychainCredentialExists = func() bool { return false }
	if err := defaultAuthProbe(context.Background()); err == nil {
		t.Fatal("no Keychain item and no other credential: probe must report unauthenticated")
	}

	keychainCredentialExists = func() bool { return true }
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("Keychain item present, probe should pass on darwin: %v", err)
	}
}

// --- self-registration + conformance ------------------------------------------

func TestSelfRegistration(t *testing.T) {
	// Importing this package ran the adapter's init, which registered "claude".
	rt, err := runtime.Resolve("claude")
	if err != nil {
		t.Fatalf("Resolve(claude): %v", err)
	}
	if _, ok := rt.(*Adapter); !ok {
		t.Fatalf("Resolve(claude) = %T, want *claude.Adapter", rt)
	}
	found := false
	for _, b := range runtime.Harnesses() {
		if b == "claude" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Harnesses() %v does not include claude", runtime.Harnesses())
	}
}

// The Claude adapter passes the shared conformance suite T7–T9 reuse, driven
// hermetically through a fake exec wired to report a known usage result.
func TestConformance(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON), runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	err := runtime.CheckConformance(context.Background(), a, runtime.ConformanceCase{
		Spec:       fixtureSpec(),
		WantResult: wantResult,
	})
	if err != nil {
		t.Fatalf("claude adapter failed conformance: %v", err)
	}
	reviewSpec := fixtureSpec()
	reviewSpec.Purpose = port.LaunchPurposeReview
	if err := runtime.CheckFencingConformance(context.Background(), a, runtime.FencingCase{Spec: reviewSpec, Harness: harnessID}); err != nil {
		t.Fatalf("claude adapter failed fencing conformance: %v", err)
	}
}
