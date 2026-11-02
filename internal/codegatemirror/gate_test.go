package codegatemirror

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRun_FixtureChecks exercises each fail-on semantics and the infra path
// with hermetic shell-free commands (true/false/echo resolve via PATH).
func TestRun_FixtureChecks(t *testing.T) {
	t.Parallel()
	work := t.TempDir()

	tests := []struct {
		name       string
		check      Check
		wantPassed bool
	}{
		{"exit-zero passes", Check{Name: "ok", Command: "true", FailOn: FailOnExit}, true},
		{"exit-nonzero fails", Check{Name: "bad", Command: "false", FailOn: FailOnExit}, false},
		{"output-clean passes", Check{Name: "quiet", Command: "true", FailOn: FailOnOutput}, true},
		{"output-present fails on zero exit", Check{Name: "loud", Command: "echo offender", FailOn: FailOnOutput}, false},
		{"missing binary is infra failure", Check{Name: "gone", Command: "code-gate-no-such-binary-xyz", FailOn: FailOnExit}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.check.Class = ClassRequired
			rep := Config{Checks: []Check{tt.check}}.Run(context.Background(), work)
			if len(rep.Outcomes) != 1 {
				t.Fatalf("got %d outcomes, want 1", len(rep.Outcomes))
			}
			if rep.Outcomes[0].Passed != tt.wantPassed {
				t.Fatalf("check passed = %v, want %v (detail: %q)", rep.Outcomes[0].Passed, tt.wantPassed, rep.Outcomes[0].Detail)
			}
			if rep.Passed != tt.wantPassed {
				t.Fatalf("gate passed = %v, want %v", rep.Passed, tt.wantPassed)
			}
		})
	}
}

// TestRun_AdvisoryNeverGates proves an advisory red is recorded but does not
// fail the gate, while a required red does (§3.10 / DINV-1).
func TestRun_AdvisoryNeverGates(t *testing.T) {
	t.Parallel()
	work := t.TempDir()

	advisoryRed := Config{Checks: []Check{
		{Name: "ok", Command: "true", FailOn: FailOnExit, Class: ClassRequired},
		{Name: "nit", Command: "false", FailOn: FailOnExit, Class: ClassAdvisory},
	}}
	rep := advisoryRed.Run(context.Background(), work)
	if !rep.Passed {
		t.Fatalf("advisory red should not gate: rep.Passed = false, want true")
	}
	if rep.Outcomes[1].Passed {
		t.Fatalf("advisory check should be recorded as failed")
	}

	requiredRed := Config{Checks: []Check{
		{Name: "must", Command: "false", FailOn: FailOnExit, Class: ClassRequired},
	}}
	if requiredRed.Run(context.Background(), work).Passed {
		t.Fatalf("required red must gate: rep.Passed = true, want false")
	}
}

// TestRun_CancelledFailsClosed proves a cancelled run never reports a pass.
func TestRun_CancelledFailsClosed(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := Config{Checks: []Check{{Name: "ok", Command: "true", FailOn: FailOnExit, Class: ClassRequired}}}.
		Run(ctx, t.TempDir())
	if rep.Passed {
		t.Fatalf("cancelled run reported pass; must fail closed (DINV-4)")
	}
}

// TestRun_RealGoChecksPass is the load-bearing smoke: a real gofmt and a real
// go build, run through code-gate over this repo under the confined runner,
// actually PASS. If they don't, the default env (PATH/HOME) is wrong — the
// whole point of envFor.
func TestRun_RealGoChecksPass(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)

	cfg := Config{Checks: []Check{
		{Name: "gofmt", Command: "gofmt -l internal/codegatemirror", FailOn: FailOnOutput, Class: ClassRequired},
		{Name: "build", Command: "go build ./internal/codegatemirror/...", FailOn: FailOnExit, Class: ClassRequired},
	}}
	rep := cfg.Run(context.Background(), root)
	for _, o := range rep.Outcomes {
		if !o.Passed {
			t.Errorf("real go check %q failed through code-gate — env likely wrong: %s", o.Name, o.Detail)
		}
	}
	if !rep.Passed {
		t.Fatalf("real go checks did not pass through the runner (env/confinement problem)")
	}
}

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
