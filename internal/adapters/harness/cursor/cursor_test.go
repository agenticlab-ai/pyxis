package cursor

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
// adapter runs with no real `cursor-agent` binary and no real pty
// (docs/tech-design/implement-now.md §3.10).
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

	// run() behaviour (AuthCheck probes)
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

// resultJSON is a representative `cursor-agent --output-format json` result
// object. Its usage block is claude-compatible (verified against the installed
// CLI): input+output+cache tokens = 1000+200+30+4 = 1234. Cursor emits no
// num_turns today, so Turns is 0.
const resultJSON = `{"type":"result","subtype":"success","is_error":false,"duration_ms":1234,` +
	`"duration_api_ms":1000,"result":"done","session_id":"sess-1","request_id":"req-1",` +
	`"usage":{"input_tokens":1000,"output_tokens":200,"cache_creation_input_tokens":30,"cache_read_input_tokens":4}}`

var wantResult = port.RunResult{Tokens: 1234, Turns: 0, Output: "done", TokensObserved: true}

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

func TestReviewLaunchUsesHostGuardForToolsAndSandbox(t *testing.T) {
	spec := port.LaunchSpec{
		Purpose: port.LaunchPurposeReview, Model: "cursor-grok-4.5-high", Effort: "high",
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
	args := strings.Join(a.launchArgs(spec, "review"), " ")
	if strings.Contains(args, "--mode plan") || strings.Contains(args, "--sandbox enabled") {
		t.Fatalf("review argv retained conflicting native restrictions: %q", args)
	}
	for _, want := range []string{"--force", "--sandbox disabled", "--trust"} {
		if !strings.Contains(args, want) {
			t.Fatalf("review argv %q omitted %q", args, want)
		}
	}
}

// fixtureSpec uses one of the adapter's qualified model-parameter forms.
func fixtureSpec() port.LaunchSpec {
	return port.LaunchSpec{
		Brief:   "implement the widget per the charter",
		Workdir: "/tmp/wt-42",
		Model:   "cursor-grok-4.5-high",
		Effort:  "high",
	}
}

// --- command construction + pty wrap ------------------------------------------

func TestLaunch_CommandConstruction(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	// Pin the BSD/macOS `script` grammar so these assertions are host-independent:
	// the identical adapter emits the util-linux grammar on a Linux runner, where
	// the inner argv is folded into one `-c` string and the BSD-shaped checks below
	// would spuriously fail. The util-linux grammar is asserted by TestPtyCommand
	// (pure) and TestLaunch_LinuxPtyGrammar (Launch path).
	a := mustAdapter(t, withCommander(fc), withGOOS("darwin"))

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

	// The launch runs under the `script` pty shim, not cursor-agent directly.
	if got.Name != "script" {
		t.Errorf("binary = %q, want script (the pty shim)", got.Name)
	}
	if got.Dir != "/tmp/wt-42" {
		t.Errorf("workdir = %q, want /tmp/wt-42", got.Dir)
	}
	// The brief is delivered as a positional argument (never stdin — a pty echoes
	// stdin into the captured output), so stdin stays empty.
	if got.Stdin != "" {
		t.Errorf("stdin = %q, want empty (brief is positional)", got.Stdin)
	}

	args := strings.Join(got.Args, " ")
	for _, want := range []string{
		"-q /dev/null cursor-agent", // BSD/macOS pty wrap prefix
		"-p",
		"--output-format json",
		"--model cursor-grok-4.5-high",
		"--force",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	// The brief is the final positional argument and carries the charter.
	if last := got.Args[len(got.Args)-1]; !strings.Contains(last, "implement the widget") {
		t.Errorf("final arg %q must be the brief carrying the charter", last)
	}
	// No --effort flag is ever emitted: Cursor has none (effort is folded into
	// the model string).
	if strings.Contains(args, "--effort") {
		t.Errorf("args %q must not contain --effort (Cursor has no such flag)", args)
	}
}

// The pty wrap is the one real Cursor quirk (R6); assert the launch is wrapped in
// `script`. Pinned to the BSD grammar (host-independent) so the inner binary
// appears as its own argv element; TestLaunch_LinuxPtyGrammar covers the
// util-linux shape where it is folded into the `-c` string.
func TestLaunch_WrapsInPty(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc), withGOOS("darwin"))
	if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fc.starts[0]
	if got.Name != scriptBinary {
		t.Fatalf("launch must be wrapped in %q, got %q", scriptBinary, got.Name)
	}
	// The inner executable appears after the wrapper's flags, never as the
	// process image itself.
	if idx := indexOf(got.Args, binary); idx < 0 {
		t.Fatalf("wrapped args %v must contain the inner binary %q", got.Args, binary)
	}
}

// TestPtyCommand pins both platform grammars of the `script` wrap as a pure
// function, so the util-linux form is covered from any host.
func TestPtyCommand(t *testing.T) {
	inner := []string{"-p", "--model", "m m", "the brief"}

	// BSD/macOS: command + args follow the typescript file directly.
	name, args := ptyCommand("darwin", "cursor-agent", inner)
	if name != "script" {
		t.Fatalf("darwin wrapper = %q, want script", name)
	}
	wantBSD := []string{"-q", "/dev/null", "cursor-agent", "-p", "--model", "m m", "the brief"}
	if strings.Join(args, "\x00") != strings.Join(wantBSD, "\x00") {
		t.Fatalf("darwin args = %v, want %v", args, wantBSD)
	}

	// util-linux: single shell-quoted -c string, plus -e for exit propagation.
	name, args = ptyCommand("linux", "cursor-agent", inner)
	if name != "script" {
		t.Fatalf("linux wrapper = %q, want script", name)
	}
	wantLinux := []string{"-q", "-e", "-c", `'cursor-agent' '-p' '--model' 'm m' 'the brief'`, "/dev/null"}
	if strings.Join(args, "\x00") != strings.Join(wantLinux, "\x00") {
		t.Fatalf("linux args = %v, want %v", args, wantLinux)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "'plain'"},
		{"claude-opus-4-8[effort=high]", "'claude-opus-4-8[effort=high]'"},
		{"a'b", `'a'\''b'`},
		{"multi\nline", "'multi\nline'"},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The util-linux grammar is also reachable through the full Launch path via the
// injected GOOS, so the pty-wrap decision is verified end-to-end for Linux.
func TestLaunch_LinuxPtyGrammar(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON)}
	a := mustAdapter(t, withCommander(fc), withGOOS("linux"))
	if _, err := a.Launch(context.Background(), fixtureSpec()); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fc.starts[0]
	if got.Name != "script" {
		t.Fatalf("binary = %q, want script", got.Name)
	}
	if len(got.Args) != 5 || got.Args[0] != "-q" || got.Args[1] != "-e" || got.Args[2] != "-c" || got.Args[4] != "/dev/null" {
		t.Fatalf("linux wrap args = %v, want [-q -e -c <cmd> /dev/null]", got.Args)
	}
	cmd := got.Args[3]
	for _, want := range []string{"'cursor-agent'", "'-p'", "'--model'", "'cursor-grok-4.5-high'", "'--force'"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("linux -c command %q missing %q", cmd, want)
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
	for _, s := range fc.starts {
		args := strings.Join(s.Args, " ")
		for _, banned := range []string{"--resume", "--continue", "--ls", "create-chat"} {
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
			name: "pty artifacts around the JSON are tolerated",
			// A leading EOT (^D), CR line endings, and a trailing prompt — exactly
			// the noise the `script` pty adds around cursor-agent's output.
			json: "\x04" + resultJSON + "\r\n\x04$ ",
			code: 0,
			want: wantResult,
		},
		{
			name: "non-zero native error remains recoverable incomplete",
			json: `{"type":"result","is_error":true,"usage":{"input_tokens":5,"output_tokens":5}}`,
			code: 1,
			want: port.RunResult{Tokens: 10, Incomplete: true, Reason: "cursor_result_error", TokensObserved: true},
		},
		{
			name:    "non-zero complete result remains operational failure",
			json:    `{"type":"result","is_error":false,"result":"done","usage":{"input_tokens":5,"output_tokens":5}}`,
			code:    1,
			wantErr: true,
		},
		{
			name: "missing usage defaults to zero (Cursor without token accounting)",
			json: `{"type":"result","subtype":"success","is_error":false,"result":"done"}`,
			code: 0,
			want: port.RunResult{Tokens: 0, Turns: 0, Output: "done"},
		},
		{
			name: "forward-compat num_turns is read when present",
			json: `{"type":"result","num_turns":9,"usage":{"input_tokens":7,"output_tokens":0}}`,
			code: 0,
			want: port.RunResult{Tokens: 7, Turns: 9, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "unknown fields are ignored (open parse)",
			json: `{"type":"result","brand_new_field":true,"usage":{"input_tokens":7,"output_tokens":0}}`,
			code: 0,
			want: port.RunResult{Tokens: 7, Turns: 0, TokensObserved: true},
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
		{"no JSON object", "just terminal noise, no braces", nil},
		{"unparseable JSON", "\x04{not valid json", nil},
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
	fc := &fakeCommander{runErr: errors.New("exec: \"cursor-agent\": not found")}
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
		t.Fatal("AuthCheck must fail when `cursor-agent --version` exits non-zero")
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
	for _, want := range []string{"unauthenticated", "CURSOR_API_KEY"} {
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
	// The install probe runs `cursor-agent --version`.
	if len(fc.runs) != 1 || strings.Join(fc.runs[0].Args, " ") != "--version" {
		t.Fatalf("expected one `cursor-agent --version` probe, got %+v", fc.runs)
	}
}

func TestDefaultAuthProbe(t *testing.T) {
	// Isolate: clear every credential env var so the probe falls through to the
	// CLI's status subcommand.
	for _, k := range []string{"CURSOR_API_KEY", "CURSOR_AUTH_TOKEN"} {
		t.Setenv(k, "")
	}

	// `status` reports unauthenticated → probe fails.
	fcNo := &fakeCommander{runStdout: []byte(`{"status":"unauthenticated","isAuthenticated":false,"message":"Not logged in"}`)}
	aNo := mustAdapter(t, withCommander(fcNo))
	if err := aNo.defaultAuthProbe(context.Background()); err == nil {
		t.Fatal("status reports unauthenticated: probe must fail")
	}
	if len(fcNo.runs) != 1 || strings.Join(fcNo.runs[0].Args, " ") != "status --format json" {
		t.Fatalf("probe must consult `cursor-agent status --format json`, got %+v", fcNo.runs)
	}

	// `status` reports authenticated → probe passes.
	fcYes := &fakeCommander{runStdout: []byte(`{"status":"authenticated","isAuthenticated":true}`)}
	aYes := mustAdapter(t, withCommander(fcYes))
	if err := aYes.defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("status reports authenticated: probe should pass: %v", err)
	}

	// An env token short-circuits before any subprocess (zero-cost).
	t.Setenv("CURSOR_API_KEY", "key-123")
	fcEnv := &fakeCommander{runStdout: []byte(`{"isAuthenticated":false}`)}
	aEnv := mustAdapter(t, withCommander(fcEnv))
	if err := aEnv.defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("env token present, probe should pass: %v", err)
	}
	if len(fcEnv.runs) != 0 {
		t.Fatalf("an env credential must short-circuit the status subprocess, got %d runs", len(fcEnv.runs))
	}
}

// --- self-registration + conformance ------------------------------------------

func TestSelfRegistration(t *testing.T) {
	// Importing this package ran the adapter's init, which registered "cursor".
	rt, err := runtime.Resolve("cursor")
	if err != nil {
		t.Fatalf("Resolve(cursor): %v", err)
	}
	if _, ok := rt.(*Adapter); !ok {
		t.Fatalf("Resolve(cursor) = %T, want *cursor.Adapter", rt)
	}
	found := false
	for _, b := range runtime.Harnesses() {
		if b == "cursor" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Harnesses() %v does not include cursor", runtime.Harnesses())
	}
}

// The Cursor adapter passes the shared conformance suite T7–T9 reuse, driven
// hermetically through a fake exec wired to report a known usage result.
func TestConformance(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(resultJSON), runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	err := runtime.CheckConformance(context.Background(), a, runtime.ConformanceCase{
		Spec:       fixtureSpec(),
		WantResult: wantResult,
	})
	if err != nil {
		t.Fatalf("cursor adapter failed conformance: %v", err)
	}
	reviewSpec := fixtureSpec()
	reviewSpec.Purpose = port.LaunchPurposeReview
	if err := runtime.CheckFencingConformance(context.Background(), a, runtime.FencingCase{Spec: reviewSpec, Harness: harnessID}); err != nil {
		t.Fatalf("cursor adapter failed fencing conformance: %v", err)
	}
}

// --- real-binary e2e (gated, skips cleanly, no model call) --------------------

// TestE2E_RealCursorAgent exercises the production osCommander against the real
// `cursor-agent` binary — but only the zero-cost, no-model-call surface: the
// AuthCheck preflight, and a genuine `script` pty round-trip running
// `cursor-agent --version`. It is gated on PYXIS_CURSOR_E2E and on the binary
// being installed, so it skips cleanly everywhere else.
func TestE2E_RealCursorAgent(t *testing.T) {
	if os.Getenv("PYXIS_CURSOR_E2E") == "" {
		t.Skip("set PYXIS_CURSOR_E2E=1 to run the real cursor-agent e2e")
	}
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s not installed: %v", binary, err)
	}

	a := mustAdapter(t) // production osCommander + real defaultAuthProbe

	// AuthCheck must return without a model call and without hanging: either it
	// passes (credential present) or it degrades loudly with a Cursor-specific
	// remedy — never a silent or unrelated failure (FR-16).
	if err := a.AuthCheck(context.Background()); err != nil && !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("AuthCheck degraded without naming the backend/remedy: %v", err)
	}

	// A genuine pty round-trip through the real `script` shim: run
	// `cursor-agent --version` (no model call) wrapped exactly as Launch wraps a
	// task, and confirm the child ran and its output was captured.
	name, args := ptyCommand(goruntime.GOOS, binary, []string{"--version"})
	proc, err := osCommander{}.start(context.Background(), commandSpec{Name: name, Args: args})
	if err != nil {
		t.Fatalf("start real pty-wrapped %s --version: %v", binary, err)
	}
	out, code, err := proc.wait()
	if err != nil {
		t.Fatalf("wait pty-wrapped %s --version: %v", binary, err)
	}
	if code != 0 {
		t.Fatalf("%s --version under pty exited %d", binary, code)
	}
	if len(strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\x04' || r == '\r' {
			return -1
		}
		return r
	}, string(out)))) == 0 {
		t.Fatalf("%s --version under pty produced no output", binary)
	}
}

// indexOf returns the first index of s in xs, or -1.
func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return -1
}
