package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
// adapter runs with no real `codex` binary (docs/tech-design/implement-now.md §3.10).
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

// streamJSONL is a representative `codex exec --json` thread-event stream:
// thread.started → turn.started → two items → turn.completed with usage. The
// completed turn reports total_tokens = input+output = 1234, and there is one
// turn.completed, so the parsed result is {Tokens:1234, Turns:1}.
const streamJSONL = `{"type":"thread.started","thread_id":"t_abc"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking"}}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":900,"output_tokens":234,"reasoning_output_tokens":34,"total_tokens":1234}}`

var wantResult = port.RunResult{Tokens: 1234, Turns: 1, Output: "done", SessionID: "t_abc", TokensObserved: true, TurnsObserved: true}

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

func TestReviewLaunchUsesHostGuardAndFreshProcess(t *testing.T) {
	spec := port.LaunchSpec{
		Purpose: port.LaunchPurposeReview, Model: "gpt-5.6-sol", Effort: "high",
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
	for _, want := range []string{"-a never", "--no-daemon", "--sandbox danger-full-access", "--ignore-user-config"} {
		if !strings.Contains(args, want) {
			t.Fatalf("review argv %q omitted %q", args, want)
		}
	}
	if strings.Contains(args, "--sandbox read-only") || strings.Contains(args, "exec resume") {
		t.Fatalf("review argv requested a nested sandbox or reused a session: %q", args)
	}
	// The lens children are proven from the session records of this launch, so
	// the CLI must be allowed to write them.
	if strings.Contains(args, "--ephemeral") {
		t.Fatalf("review argv discards the session records the host verifies: %q", args)
	}
}

func fixtureSpec() port.LaunchSpec {
	return port.LaunchSpec{
		Brief:   "implement the widget per the charter",
		Workdir: "/tmp/wt-42",
		Model:   "gpt-5-codex",
		Effort:  "high",
	}
}

// --- command construction -----------------------------------------------------

func TestLaunch_CommandConstruction(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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
	if got.Name != "codex" {
		t.Errorf("binary = %q, want codex", got.Name)
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
		"--no-daemon",
		"exec",
		"--json",
		"-m gpt-5-codex",
		`model_reasoning_effort="high"`,
		"--dangerously-bypass-approvals-and-sandbox",
		"--skip-git-repo-check",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	// The prompt is read from stdin: the trailing positional must be `-`.
	if last := got.Args[len(got.Args)-1]; last != "-" {
		t.Errorf("last arg = %q, want `-` (read the brief from stdin)", last)
	}
	// Never delegate tool processes to the ambient shared daemon.
	if got.Args[0] != "--no-daemon" || got.Args[1] != "exec" {
		t.Errorf("invocation must own the non-interactive session: %q", got.Args)
	}
}

// DINV-11: every task gets a fresh process and no session is ever resumed.
func TestLaunch_FreshProcessNoSessionReuse(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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
		for _, banned := range []string{"resume", "--last", "--continue"} {
			if strings.Contains(args, banned) {
				t.Errorf("launch args must never reuse a session, found %q in %q", banned, args)
			}
		}
	}
}

func TestLaunch_RejectsUnresolvedTier(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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

// --- Await / JSONL parsing ----------------------------------------------------

func TestAwait_ParsesUsageStream(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		code    int
		want    port.RunResult
		wantErr bool
	}{
		{"full stream, exit 0", streamJSONL, 0, wantResult, false},
		{
			name:    "non-zero exit remains an operational failure",
			json:    `{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":5,"total_tokens":10}}`,
			code:    1,
			wantErr: true,
		},
		{
			name: "total_tokens absent falls back to input+output",
			json: `{"type":"turn.completed","usage":{"input_tokens":7,"output_tokens":3}}`,
			code: 0,
			want: port.RunResult{Tokens: 10, Turns: 1, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "usage summed and turns counted across multiple turns",
			json: `{"type":"turn.completed","usage":{"total_tokens":100}}` + "\n" +
				`{"type":"turn.started"}` + "\n" +
				`{"type":"turn.completed","usage":{"total_tokens":250}}`,
			code: 0,
			want: port.RunResult{Tokens: 350, Turns: 2, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "unknown event types and fields are ignored (open parse)",
			json: `{"type":"item.completed","item":{"type":"file_change"},"brand_new_field":true}` + "\n" +
				`{"type":"turn.completed","usage":{"input_tokens":7,"output_tokens":0,"total_tokens":7}}`,
			code: 0,
			want: port.RunResult{Tokens: 7, Turns: 1, TokensObserved: true, TurnsObserved: true},
		},
		{
			name: "a completed turn without usage counts a turn but no tokens",
			json: `{"type":"turn.completed"}`,
			code: 0,
			want: port.RunResult{Tokens: 0, Turns: 1, TurnsObserved: true},
		},
		{
			name: "an error-only non-zero JSON stream is recoverable incomplete",
			json: `{"type":"thread.started","thread_id":"t"}` + "\n" +
				`{"type":"turn.started"}` + "\n" +
				`{"type":"turn.failed","error":{"message":"stream disconnected"}}`,
			code: 1,
			want: port.RunResult{Incomplete: true, Reason: "turn.failed", SessionID: "t", TurnsObserved: true},
		},
		{
			name: "blank lines and a stray non-JSON diagnostic line are tolerated",
			json: "\n  warning: something on stdout\n" + streamJSONL + "\n",
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

func TestAwait_OperationalErrorsAndNoEvents(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		waitErr error
	}{
		{"process could not be awaited", "", errors.New("killed by signal")},
		{"empty output", "", nil},
		{"no JSON object on stdout at all", "not json at all\njust text\n", nil},
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
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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
	fc := &fakeCommander{procStdout: []byte(streamJSONL)}
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
	fc := &fakeCommander{runErr: errors.New("exec: \"codex\": not found")}
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
		t.Fatal("AuthCheck must fail when `codex --version` exits non-zero")
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
	for _, want := range []string{"unauthenticated", "OPENAI_API_KEY"} {
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
	// The install probe runs `codex --version`.
	if len(fc.runs) != 1 || strings.Join(fc.runs[0].Args, " ") != "--version" {
		t.Fatalf("expected one `codex --version` probe, got %+v", fc.runs)
	}
}

func TestDefaultAuthProbe(t *testing.T) {
	// Isolate: clear every credential env var and point the store at an empty dir.
	for _, k := range []string{"OPENAI_API_KEY", "CODEX_API_KEY"} {
		t.Setenv(k, "")
	}
	empty := t.TempDir()
	t.Setenv("CODEX_HOME", empty)

	if err := defaultAuthProbe(context.Background()); err == nil {
		t.Fatal("no env token and no auth store: probe must report unauthenticated")
	}

	// An env token satisfies the probe.
	t.Setenv("OPENAI_API_KEY", "sk-test")
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("env token present, probe should pass: %v", err)
	}

	// The on-disk auth store satisfies the probe too.
	t.Setenv("OPENAI_API_KEY", "")
	if err := os.WriteFile(filepath.Join(empty, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed auth store: %v", err)
	}
	if err := defaultAuthProbe(context.Background()); err != nil {
		t.Fatalf("auth store present, probe should pass: %v", err)
	}
}

// --- self-registration + conformance ------------------------------------------

func TestSelfRegistration(t *testing.T) {
	// Importing this package ran the adapter's init, which registered "codex".
	rt, err := runtime.Resolve("codex")
	if err != nil {
		t.Fatalf("Resolve(codex): %v", err)
	}
	if _, ok := rt.(*Adapter); !ok {
		t.Fatalf("Resolve(codex) = %T, want *codex.Adapter", rt)
	}
	found := false
	for _, b := range runtime.Harnesses() {
		if b == "codex" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Harnesses() %v does not include codex", runtime.Harnesses())
	}
}

// The Codex adapter passes the shared conformance suite T7–T9 reuse, driven
// hermetically through a fake exec wired to report a known usage result.
func TestConformance(t *testing.T) {
	fc := &fakeCommander{procStdout: []byte(streamJSONL), runCode: 0}
	a := mustAdapter(t, withCommander(fc), withAuthProbe(func(context.Context) error { return nil }))
	err := runtime.CheckConformance(context.Background(), a, runtime.ConformanceCase{
		Spec:       fixtureSpec(),
		WantResult: wantResult,
	})
	if err != nil {
		t.Fatalf("codex adapter failed conformance: %v", err)
	}
	reviewSpec := fixtureSpec()
	reviewSpec.Purpose = port.LaunchPurposeReview
	if err := runtime.CheckFencingConformance(context.Background(), a, runtime.FencingCase{Spec: reviewSpec, Harness: harnessID}); err != nil {
		t.Fatalf("codex adapter failed fencing conformance: %v", err)
	}
}
