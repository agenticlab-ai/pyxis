package e2e

// Regression proofs for defects the matrix surfaced and the train then fixed —
// kept as tests so a reopened bug reds here again.

import (
	"os/exec"
	"strings"
	"testing"
)

// TestOperandDocgateResolvesRootWithoutGoMod is the regression proof for the
// defect T13 caught: the shipped operand gate path — the pre-commit/pre-push
// hooks and the installed-repo CI workflow variant, all `pyxis exec docgate --
// gate` — could not run in an installed operand, because docgate's repoRoot
// walked up ONLY for `go.mod` and an installed operand ships NO Go source (a
// founding invariant). `docgate gate` failed with "no go.mod found above /" and
// never ran the gate. Nothing caught it because the only source-less gate tests
// call gate.Gate() in process with an explicit root (T8's green proof); this
// harness was the first to drive the docgate BINARY inside a source-less operand.
//
// The fix: every pyxis binary shares one root-discovery predicate
// (internal/reporoot) — the nearest ancestor with .pyxis/config.yaml, then the
// git working-tree root — so docgate resolves an operand root exactly as pyxis
// does. This test drives the docgate binary in a source-less dir carrying only
// .pyxis/config.yaml and proves it finds the root (never the go.mod refusal) and
// reaches real gate work.
func TestOperandDocgateResolvesRootWithoutGoMod(t *testing.T) {
	requireHarness(t)

	// A source-less operand: only .pyxis/config.yaml — no go.mod, no Go source,
	// exactly what an installed repo is. The config walk finds the root without
	// needing the git fallback.
	operand := t.TempDir()
	writeFile(t, at(operand, ".pyxis/config.yaml"),
		[]byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v9.9.9\"\n  manifest_sha256: \""+strings.Repeat("ab", 32)+"\"\n  mode: release\n"),
		0o644)

	cmd := exec.Command(builtBins["docgate"], "gate")
	cmd.Dir = operand
	cmd.Env = []string{"HOME=" + t.TempDir()}
	out, _ := cmd.CombinedOutput()

	// The fix's core assertion: docgate never dead-ends on go.mod in an operand.
	if strings.Contains(string(out), "no go.mod found") {
		t.Fatalf("docgate still refuses on go.mod in a source-less operand — the reporoot fix regressed:\n%s", out)
	}
	// And it got past root discovery into real gate work (this bare fixture has
	// no ledgers, so the gate reports a missing docs path rather than a root
	// failure — the point is it RESOLVED the root and ran).
	if !strings.Contains(string(out), operand) {
		t.Fatalf("docgate did not resolve the operand root (expected output to reference %s):\n%s", operand, out)
	}
}
