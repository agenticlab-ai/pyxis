package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The T12 resolver-delegate shim suite runs under the Go harness so `go test
// ./...` (CI and desk) covers the four exec-grammar delegates without a
// workflow step (mirroring T11's shim_test.go wiring). The script asserts each
// resolver is a bash-3.2-clean locate-and-delegate shim carrying no
// fetch/parse logic, and proves the docgate delegate reaches its tool green on
// the clean source tree through the shim itself — the NR-1 parity proof for
// this call site (plan installer/001 T12; installer-v1 §3.10 "Shims").
//
// The script shells `go run ./cmd/pyxis` from the repo root, so the Go
// toolchain and a clean checkout are its only preconditions; it is hermetic
// otherwise (no network — the source repo runs in source mode).
func TestResolverDelegateShimSuite(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go not available: %v", err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, ".pyxis", "scripts", "tests", "test-resolver-delegates.sh")
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the resolver-delegate shim suite failed: %v\n%s", err, out)
	}
}

// An explicit unavailable base must reach the real docgate invocation. Silently
// substituting origin/main makes a stacked design look bundled with parent code.
func TestDelegateShimSuitesHonorTrustedBase(t *testing.T) {
	for _, tool := range []string{"bash", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const absent = "0000000000000000000000000000000000000000"
	t.Setenv("PYXIS_CI_BASE", absent)
	for _, name := range []string{"test-docgate-hook.sh", "test-resolver-delegates.sh"} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", filepath.Join(root, ".pyxis", "scripts", "tests", name))
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), absent) {
				t.Fatalf("shim ignored the unavailable trusted base: %v\n%s", err, out)
			}
		})
	}
}
