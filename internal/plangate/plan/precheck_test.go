package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotTree records a directory tree's file paths + contents so a test can
// prove Precheck mutated NOTHING — the read-only guarantee is verified, not
// assumed (contract-gate DINV-1, the gate plangate delegates to is read-only).
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot walk: %v", err)
	}
	return out
}

// runPrecheck runs the precondition against a testdata fixture root and asserts
// the tree is byte-unchanged before/after (the precheck writes nothing).
func runPrecheck(t *testing.T, fixture, feature string) []string {
	t.Helper()
	root := filepath.Join("testdata", fixture)
	before := snapshotTree(t, root)
	defects := Precheck(root, feature)
	after := snapshotTree(t, root)
	if len(before) != len(after) {
		t.Fatalf("precheck changed the file count %d → %d — it must be read-only (DINV-1)", len(before), len(after))
	}
	for p, b := range before {
		if after[p] != b {
			t.Fatalf("precheck mutated %q — it must be read-only (DINV-1)", p)
		}
	}
	msgs := make([]string, len(defects))
	for i, d := range defects {
		msgs[i] = d.String()
	}
	return msgs
}

// A clean contract lets planning proceed: the precondition returns zero
// defects, so the caller does not bounce (FR-27; the clean half of A-8).
func TestPrecheckCleanProceeds(t *testing.T) {
	msgs := runPrecheck(t, "precheck-clean", "demo")
	if len(msgs) != 0 {
		t.Fatalf("clean contract produced %d precheck defect(s), want 0 (planning must proceed):\n%s",
			len(msgs), strings.Join(msgs, "\n"))
	}
}

// A bouncing contract refuses planning: the precondition returns defects — a
// framing bounce naming the feature and verdict class, plus the gate's own
// named offender (the Must with no fit criterion). This is the "zero plan
// output on a bouncing contract" acceptance realized mechanically (FR-27; A-8).
func TestPrecheckBouncingRefuses(t *testing.T) {
	msgs := runPrecheck(t, "precheck-bouncing", "demo")
	if len(msgs) == 0 {
		t.Fatal("bouncing contract produced zero precheck defects — the planning-entry gate did not refuse (FR-27)")
	}
	joined := strings.Join(msgs, "\n")

	// The framing bounce names the feature, the verb, and the non-clean class,
	// and cites FR-27 as the refusing rule.
	if !strings.Contains(joined, "contract-gate") || !strings.Contains(joined, "incomplete") {
		t.Errorf("bounce does not name the contract-gate verdict class:\n%s", joined)
	}
	if !strings.Contains(joined, "FR-27") {
		t.Errorf("bounce does not cite FR-27 (the refusing precondition):\n%s", joined)
	}
	// The gate's named offender is surfaced through the precheck — the Must
	// (FR-1) whose fit criterion is missing, attributed to the structural check.
	if !strings.Contains(joined, "FR-1") {
		t.Errorf("bounce does not surface the offending requirement FR-1:\n%s", joined)
	}
	if !strings.Contains(joined, "precheck/structural-completeness") {
		t.Errorf("bounce does not attribute the offender to the structural-completeness check:\n%s", joined)
	}
}

// A fail-closed edge (a missing / unmerged contract) is itself blocking and
// surfaces as a precheck bounce — a broken or absent contract never slips the
// gate into "proceed" (FR-27; contract-gate §3.11 fail-closed).
func TestPrecheckMissingContractRefuses(t *testing.T) {
	msgs := runPrecheck(t, "precheck-clean", "no-such-feature")
	if len(msgs) == 0 {
		t.Fatal("a missing contract produced zero precheck defects — a fail-closed edge must refuse, never proceed (FR-27)")
	}
	if !strings.Contains(strings.Join(msgs, "\n"), "configuration-error") {
		t.Errorf("a missing contract did not surface a configuration-error bounce:\n%s", strings.Join(msgs, "\n"))
	}
}

// Determinism: repeated prechecks over the same fixture yield the identical
// defect set (the gate is deterministic, DINV-5; the precheck adds no ordering
// of its own).
func TestPrecheckDeterministic(t *testing.T) {
	a := Precheck(filepath.Join("testdata", "precheck-bouncing"), "demo")
	b := Precheck(filepath.Join("testdata", "precheck-bouncing"), "demo")
	if len(a) != len(b) {
		t.Fatalf("precheck defect count not stable: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			t.Fatalf("precheck defect[%d] not stable:\n%s\nvs\n%s", i, a[i].String(), b[i].String())
		}
	}
}
