package payload_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestHelpPuritySuite runs the /help skill's derivation-purity conformance
// test under the Go harness so `go test ./...` (code-gate, the pre-push hook,
// and CI) enforces it on every change — the design sites the check on the
// scripts-test harness (ecosystem-help-v1 §3.6, DQ-4), and this wrapper is
// how a scripts-test-harness check reaches CI without the docs-gate workflow
// growing a step (the same choice internal/release makes for the bootstrap
// shim). The bash test is hermetic: it reads the committed help files and the
// live skills tree, and sandboxes its add-a-skill fixture under a tempdir —
// it never mutates the real tree. It proves ecosystem-help-v1 #DINV-1 (the
// catalog is never a kept list) and #DINV-3 (statelessness), i.e.
// prd:ecosystem-help/FR-4.
func TestHelpPuritySuite(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, ".pyxis", "scripts", "tests", "test-help-purity.sh")
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the /help purity conformance test failed: %v\n%s", err, out)
	}
}
