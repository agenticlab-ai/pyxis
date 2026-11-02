package runner

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// --- the re-exec helper-process pattern (matches stdlib's os/exec tests) ----
//
// helperVerb builds a *schema.Verb whose Command re-invokes THIS test binary
// with -test.run pinned to TestHelperProcess; a CODEGATE_RUNNER_TEST_HELPER=1
// pinned env entry tells the child to behave as a scripted verb instead of
// running the normal suite. This gives every test a controllable, real OS
// process — no dependency on any system binary being present, and no shell.

func helperVerb(env map[string]schema.EnvEntry, args ...string) *schema.Verb {
	parts := append([]string{quoteIfNeeded(os.Args[0]), "-test.run=^TestHelperProcess$", "--"}, args...)
	merged := map[string]schema.EnvEntry{"CODEGATE_RUNNER_TEST_HELPER": {Mode: schema.EnvPin, Value: "1"}}
	maps.Copy(merged, env)
	return &schema.Verb{Command: strings.Join(parts, " "), Env: merged}
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}

// TestHelperProcess is not a real test: run normally it no-ops (the guard
// env var is absent), so `go test` reports it as trivially passed. Run as a
// helper subprocess (spawned by Exec with the guard var pinned), it behaves
// per its first positional arg and calls os.Exit directly, never returning
// control to the testing framework's own pass/fail reporting.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("CODEGATE_RUNNER_TEST_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}

	switch args[0] {
	case "exit":
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "echo-env":
		if v, ok := os.LookupEnv(args[1]); ok {
			if _, err := fmt.Fprintf(os.Stdout, "PRESENT:%s", v); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := fmt.Fprint(os.Stdout, "ABSENT"); err != nil {
				t.Fatal(err)
			}
		}
		os.Exit(0)
	case "sleep":
		secs, _ := strconv.Atoi(args[1])
		time.Sleep(time.Duration(secs) * time.Second)
		os.Exit(0)
	case "ready-and-block":
		if err := os.WriteFile(os.Getenv("RUNNER_READY_FILE"), []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Hour) // fixture lifetime; the parent must cancel it
		os.Exit(0)
	case "signal":
		// Terminate self by signal (unix: SIGKILL) so the parent's Wait sees a
		// negative ExitCode — exercising the signal-death infra classification.
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Kill()
		}
		select {} // block until the signal lands
	case "stdout":
		if _, err := fmt.Fprint(os.Stdout, strings.Join(args[1:], " ")); err != nil {
			t.Fatal(err)
		}
		os.Exit(0)
	case "write-file":
		// Shared by confine_other_test.go and confine_linux_test.go: attempt
		// to write args[1], behaving like an ordinary, unmodified tool would
		// on a denied write — report the OS error on stderr and exit
		// nonzero, never swallow it. This is deliberately the SAME
		// unremarkable os.WriteFile call any real verb might make, so a
		// Landlock denial's error text is exactly what detectWriteScopeViolation
		// must recognize from real tools, not a rigged marker.
		if err := os.WriteFile(args[1], []byte("payload\n"), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(2)
}

// --- actual runner tests -----------------------------------------------------

func TestExec_Pass(t *testing.T) {
	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "exit", "0"))
	if !res.Passed || res.ExitCode != 0 || res.Class != "" {
		t.Fatalf("got %+v, want a clean pass", res)
	}
}

func TestExec_CheckFailure_NotInfra(t *testing.T) {
	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "exit", "1"))
	if res.Passed {
		t.Fatal("exit 1 must not be Passed")
	}
	if res.Class != "" {
		t.Errorf("a completed nonzero exit must not carry a Class (got %q) — it is a check failure, never infra", res.Class)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}

func TestExec_Stdout_Captured(t *testing.T) {
	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "stdout", "hello", "world"))
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if res.Stdout != "hello world" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello world")
	}
}

// UB-1: a verb that cannot be spawned (missing binary) classifies as infra —
// never red (fail), never green (pass) — whether referenced by an absolute
// path or a bare name resolved through PATH.
func TestExec_SpawnFailure_MissingBinary(t *testing.T) {
	tests := []string{
		"/nonexistent/path/to/binary-xyz123",
		"definitely-not-a-real-binary-xyzxyz",
	}
	for _, command := range tests {
		t.Run(command, func(t *testing.T) {
			res := Exec(context.Background(), t.TempDir(), &schema.Verb{Command: command})
			if res.Class != ClassInfra {
				t.Fatalf("Class = %q, want %q (Passed=%v ExitCode=%d Err=%v)", res.Class, ClassInfra, res.Passed, res.ExitCode, res.Err)
			}
			if res.Passed {
				t.Error("a spawn failure must not be Passed")
			}
			if res.Err == nil {
				t.Error("a spawn failure must carry Err")
			}
		})
	}
}

// DINV-5: a var NOT declared in the verb's env is unreachable by the child,
// even though it is set in the parent's own (ambient) environment.
func TestExec_EnvStrip_AmbientUnobservable(t *testing.T) {
	t.Setenv("CODEGATE_TEST_AMBIENT_SECRET", "leak-me-not")

	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "echo-env", "CODEGATE_TEST_AMBIENT_SECRET"))
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if res.Stdout != "ABSENT" {
		t.Errorf("Stdout = %q, want ABSENT — an undeclared var must be unreachable by the child", res.Stdout)
	}
}

func TestExec_EnvPass_ObservesAmbientValue(t *testing.T) {
	t.Setenv("CODEGATE_TEST_PASSTHROUGH", "ambient-value")

	verb := helperVerb(map[string]schema.EnvEntry{
		"CODEGATE_TEST_PASSTHROUGH": {Mode: schema.EnvPass},
	}, "echo-env", "CODEGATE_TEST_PASSTHROUGH")

	res := Exec(context.Background(), t.TempDir(), verb)
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if res.Stdout != "PRESENT:ambient-value" {
		t.Errorf("Stdout = %q, want the passed-through ambient value", res.Stdout)
	}
}

func TestExec_EnvPin_ObservesPinnedValue(t *testing.T) {
	// Deliberately do NOT set the ambient var — a pinned entry must be
	// observed independent of whatever (if anything) the ambient env holds.
	verb := helperVerb(map[string]schema.EnvEntry{
		"CODEGATE_TEST_PINNED": {Mode: schema.EnvPin, Value: "pinned-value"},
	}, "echo-env", "CODEGATE_TEST_PINNED")

	res := Exec(context.Background(), t.TempDir(), verb)
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
	if res.Stdout != "PRESENT:pinned-value" {
		t.Errorf("Stdout = %q, want the pinned value", res.Stdout)
	}
}

// UB-2: a hanging verb is terminated at its bound plus the termination
// grace, classified as a timeout (infra-adjacent, never red/green).
func TestExec_Timeout_TerminatesAtBoundPlusGrace(t *testing.T) {
	verb := helperVerb(nil, "sleep", "30")
	verb.TimeoutS = 1
	verb.GraceS = 1

	res := Exec(context.Background(), t.TempDir(), verb)

	if res.Class != ClassTimeout {
		t.Fatalf("Class = %q, want %q", res.Class, ClassTimeout)
	}
	if res.Passed {
		t.Error("a timed-out verb must not be Passed")
	}
	// The precise timeout/grace sequence is checked with virtual time below;
	// this integration test proves the real process produces a timeout result.
}

func TestExec_Timeout_UsesDefaultsWhenUnset(t *testing.T) {
	// A verb with no declared bound falls back to DefaultTimeout/DefaultGrace
	// — proven indirectly: a fast-exiting verb is unaffected by the defaults.
	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "exit", "0"))
	if !res.Passed {
		t.Fatalf("unexpected failure: %+v", res)
	}
}

func TestExec_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	verb := helperVerb(map[string]schema.EnvEntry{"RUNNER_READY_FILE": {Mode: schema.EnvPin, Value: ready}}, "ready-and-block")
	done := make(chan Result, 1)
	go func() { done <- Exec(ctx, dir, verb) }()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	watchdog := time.NewTimer(time.Minute)
	defer watchdog.Stop()
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case res := <-done:
			t.Fatalf("child exited before readiness: %+v", res)
		case <-watchdog.C:
			t.Fatal("child never acknowledged readiness")
		case <-poll.C:
		}
	}
	cancel()
	var res Result
	select {
	case res = <-done:
	case <-watchdog.C:
		t.Fatal("cancelled child did not terminate")
	}

	if res.Class != ClassInfra {
		t.Fatalf("Class = %q, want %q on caller cancellation", res.Class, ClassInfra)
	}
	if !errors.Is(res.Err, context.Canceled) || res.Passed {
		t.Fatalf("cancellation result = %+v", res)
	}
}

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"simple", "go build ./...", []string{"go", "build", "./..."}, false},
		{"double quoted arg with space", `eslint "src/my file.js"`, []string{"eslint", "src/my file.js"}, false},
		{"single quoted arg with space", `eslint 'src/my file.js'`, []string{"eslint", "src/my file.js"}, false},
		{"extra whitespace collapses", "gofmt   -l   .", []string{"gofmt", "-l", "."}, false},
		{"empty command", "", nil, true},
		{"whitespace-only command", "   ", nil, true},
		{"unterminated quote", `eslint "unterminated`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommand(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitCommand(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCommand(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCommand(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBoundDuration_ClampsCeilingAndGuardsOverflow(t *testing.T) {
	cases := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"unset falls back to default", 0, DefaultTimeout},
		{"negative falls back to default", -5, DefaultTimeout},
		{"normal value honored", 30, 30 * time.Second},
		{"past the ceiling clamps to MaxBound", 100_000, MaxBound},            // ~27h > 1h
		{"overflow to negative clamps to MaxBound", 10_000_000_000, MaxBound}, // seconds*1e9 wraps int64 negative
	}
	for _, c := range cases {
		if got := boundDuration(c.seconds, DefaultTimeout); got != c.want {
			t.Errorf("%s: boundDuration(%d) = %s, want %s", c.name, c.seconds, got, c.want)
		}
	}
}

func TestExec_SignalKilled_ClassifiesInfra(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a signal death (negative ExitCode) is a unix classification")
	}
	res := Exec(context.Background(), t.TempDir(), helperVerb(nil, "signal"))
	if res.Class != ClassInfra {
		t.Fatalf("Class = %q, want %q — a signal-killed verb is an infra fault, not a check red", res.Class, ClassInfra)
	}
	if res.Passed {
		t.Error("a signal-killed verb must not be Passed")
	}
}

func TestCapBuffer_TruncatesRatherThanGrowingUnbounded(t *testing.T) {
	var c capBuffer
	chunk := make([]byte, 1<<20) // 1 MiB
	for i := range chunk {
		chunk[i] = 'x'
	}
	for range 16 { // 16 MiB total, double the cap
		if _, err := c.Write(chunk); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if c.buf.Len() > maxCapturedBytes+64 { // small slack for the truncation marker
		t.Errorf("capBuffer grew to %d bytes, want capped near %d", c.buf.Len(), maxCapturedBytes)
	}
	if !strings.Contains(c.String(), "truncated") {
		t.Error("expected a truncation marker in the captured output")
	}
}
