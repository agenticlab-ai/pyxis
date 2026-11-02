package codegatemirror

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

func TestFailureDiagnosticSurvivesCancellationAndLogFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	dir := t.TempDir()
	path := retainCheckOutput(ctx, dir, "gate failed before cancellation")
	if got, err := os.ReadFile(path); err != nil || string(got) != "gate failed before cancellation" {
		t.Fatalf("cancellation lost diagnostic: %q %v", got, err)
	}
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if path := retainCheckOutput(t.Context(), blocked, "still useful inline"); path != "" {
		t.Fatalf("failed log creation fabricated reference: %s", path)
	}
	if got := failureExcerpt("--- FAIL: TestEarly\ndiagnostic\n"); got != "--- FAIL: TestEarly\ndiagnostic" {
		t.Fatalf("short failure evidence: %q", got)
	}
}

func TestFailureExcerptRetainsEarlyFailureAndFullDiagnostic(t *testing.T) {
	output := "ok\tfirst\n--- FAIL: TestAuthority (0.01s)\n    fixture_test.go:23: .git/info/exclude: no such file\nFAIL\tinstaller\n" + strings.Repeat("ok\tlater\n", 100) + "FAIL\n"
	dir := t.TempDir()
	cfg := Config{Checks: []Check{{Name: "tests", Class: ClassRequired}}}
	rep := cfg.run(t.Context(), dir, func(_ context.Context, _ string, check Check) Outcome {
		return Outcome{Name: check.Name, Class: check.Class, Output: output, Detail: "exited 1"}
	})
	summary := rep.FailureSummary()
	if !strings.Contains(summary, "fixture_test.go:23") || !strings.Contains(summary, "full diagnostic:") || !strings.Contains(summary, "intervening output omitted") {
		t.Fatalf("lost primary failure: %s", summary)
	}
	full, err := os.ReadFile(rep.Outcomes[0].LogPath)
	if err != nil || string(full) != output {
		t.Fatalf("full evidence = %q, %v", full, err)
	}
	if info, err := os.Stat(rep.Outcomes[0].LogPath); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("diagnostic is not private")
	}
}

func TestVerificationBaseIsScopedAndInvalidatesCache(t *testing.T) {
	dir := gitFixture(t)
	t.Setenv("PYXIS_CI_BASE", "ambient")
	cfg := Config{Checks: []Check{{Name: "base", Command: "printenv PYXIS_CI_BASE", FailOn: FailOnOutput, Class: ClassRequired, Env: []string{"PYXIS_CI_BASE"}}}}
	for _, base := range []string{"first", "second"} {
		ctx := runtime.WithVerificationBase(t.Context(), base)
		out := runCheck(ctx, dir, cfg.Checks[0])
		if strings.TrimSpace(out.Output) != base {
			t.Fatalf("base leaked across tasks: %q, want %q", out.Output, base)
		}
	}
	first, err := cfg.fingerprint(runtime.WithVerificationBase(t.Context(), "first"), dir, mustGitDir(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	second, err := cfg.fingerprint(runtime.WithVerificationBase(t.Context(), "second"), dir, mustGitDir(t, dir))
	if err != nil || first == second {
		t.Fatalf("changed integrated base reused cache: %v", err)
	}
	if os.Getenv("PYXIS_CI_BASE") != "ambient" {
		t.Fatal("verification mutated process-global env")
	}
}
