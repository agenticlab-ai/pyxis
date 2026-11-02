package codegatemirror

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestRepositoryCoverageGateWiring pins the source repository's two CI
// surfaces to one required test and coverage predicate. The command belongs in the
// shared code-gate list: the local pre-push hook and GitHub Actions must both
// delegate to that list, rather than copying the coverage command and drifting.
func TestRepositoryCoverageGateWiring(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("load repository code-gate: %v", err)
	}

	const (
		coverageName    = "test-coverage"
		coverageCommand = "go run ./cmd/coverage-gate -min 90"
	)
	requireCheck := func(name string) Check {
		t.Helper()
		for _, check := range cfg.Checks {
			if check.Name == name {
				return check
			}
		}
		t.Fatalf("repository code-gate has no %q check", name)
		return Check{}
	}
	coverage := requireCheck(coverageName)
	if coverage.Command != coverageCommand {
		t.Errorf("coverage command = %q, want %q", coverage.Command, coverageCommand)
	}
	if coverage.Class != ClassRequired {
		t.Errorf("coverage class = %q, want %q", coverage.Class, ClassRequired)
	}
	if coverage.FailOn != FailOnExit {
		t.Errorf("coverage fail-on = %q, want %q", coverage.FailOn, FailOnExit)
	}
	if coverage.Group != "tests" || coverage.Phase != 2 {
		t.Errorf("coverage must run in the tests group after cheap checks: %+v", coverage)
	}

	// Lint must remain a required exit-code check in the same list. Pinning the
	// invocation prevents PATH differences between an editor, the desk and CI.
	lint := requireCheck("lint")
	if lint.Class != ClassRequired || lint.FailOn != FailOnExit {
		t.Fatalf("lint may report findings without failing CI: %+v", lint)
	}
	if !strings.Contains(lint.Command, "golangci-lint@v2.12.2 run ./...") {
		t.Fatalf("lint does not use the shared version pin: %s", lint.Command)
	}
	for name, command := range map[string]string{
		"ci-parity": "go run ./cmd/ci-parity",
		"memory":    "go run ./cmd/memory-gate",
	} {
		check := requireCheck(name)
		if check.Command != command || check.Class != ClassRequired || check.FailOn != FailOnExit {
			t.Fatalf("%s is not a required shared predicate: %+v", name, check)
		}
	}
	memory := requireCheck("memory")
	if strings.Join(memory.Env, " ") != "PYXIS_CI_BASE PYXIS_CI_EVENT GITHUB_ACTIONS GOMEMLIMIT" {
		t.Fatalf("memory gate must declare its trusted CI inputs and optional heap hint: %+v", memory)
	}
	if memory.Group != "memory" || memory.Phase != 2 {
		t.Errorf("memory must overlap the full test suite after cheap checks: %+v", memory)
	}
	if hook := requireCheck("hook-shim"); hook.Phase != 2 {
		t.Errorf("hook shim must occupy the hosted default group in phase 2: %+v", hook)
	}
	for _, name := range []string{"hook-shim", "test-coverage"} {
		check := requireCheck(name)
		if !slices.Contains(check.Env, "PYXIS_CI_BASE") {
			t.Errorf("%s does not forward the trusted PR base to its shim tests", name)
		}
	}

	surfaces := []struct {
		name string
		path string
		call string
	}{
		{
			name: "local CI pre-push",
			path: ".githooks/pre-push",
			call: `code-gate-resolve.sh" run`,
		},
		{
			name: "GitHub CI",
			path: ".github/workflows/docs-gate.yml",
			call: ".pyxis/scripts/code-gate-resolve.sh run",
		},
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join(root, surface.path))
			if err != nil {
				t.Fatalf("read %s: %v", surface.path, err)
			}
			if !strings.Contains(string(data), surface.call) {
				t.Errorf("%s does not invoke the shared code-gate with %q", surface.path, surface.call)
			}
		})
	}
}
