package release_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// install.sh's own suite ran nowhere: no Go harness referenced it and no
// workflow invoked it, so the only thing exercising the pinned installer was
// a human remembering to run it. That is how a digest-tool bug reached a
// consumer's machine. It joins the bootstrap shim under the same harness, so
// `go test ./...` covers both first-movers. Hermetic — file:// fixtures and a
// PATH-stubbed curl, no network.
func TestInstallSHSuite(t *testing.T) {
	for _, tool := range []string{"bash", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, ".pyxis", "scripts", "tests", "test-install-sh.sh")
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the install.sh test failed: %v\n%s", err, out)
	}
}
