package release_test

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The bootstrap shim's test suite runs under the Go harness so `go test
// ./...` (CI and desk) covers the one shell first-mover without the source
// workflow growing a step (the T11 charter's wiring choice; the T12/T13
// migration may additionally wire it into the workflow file it owns). The
// script is hermetic — file:// fixtures only, no network.
func TestBootstrapShimSuite(t *testing.T) {
	for _, tool := range []string{"bash", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, ".pyxis", "scripts", "tests", "test-pyxis-bootstrap.sh")
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the bootstrap shim test failed: %v\n%s", err, out)
	}
}
