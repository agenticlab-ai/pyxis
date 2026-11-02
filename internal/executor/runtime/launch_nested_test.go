package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Run the complete real boundary conformance test inside the production
// verification boundary. The former single-level fixture did not cover this.
func TestVerificationRunsRealNestedBoundaryConformance(t *testing.T) {
	if err := ProbeProcessAccess(t.Context()); err != nil {
		t.Fatal(err)
	}
	policy, parent := accessFixture(t, false)
	head := accessGit(t, parent, "rev-parse", "HEAD")
	ctx, cleanup, err := VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.v", "-test.run=^TestProcessAccessEnforcesGitOwnership$")
	cmd.Dir, cmd.Env = policy.Workdir, append(os.Environ(), "PYXIS_REQUIRE_PROCESS_CONFINEMENT=1")
	if err := ConfineVerificationCommand(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestProcessAccessEnforcesGitOwnership") || strings.Contains(string(out), "--- SKIP") {
		t.Fatalf("nested conformance: %v\n%s", err, out)
	}
	if got := accessGit(t, parent, "rev-parse", "HEAD"); got != head {
		t.Fatal("nested launch changed outer checkpoint")
	}
}
