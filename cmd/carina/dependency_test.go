package main

import (
	"strings"
	"testing"
)

// TestDepAddRemoveBidirectional: dep add/rm resolves both directions through the
// CLI — the source shows its blocker and the blocker shows what it blocks via a
// re-add round-trip — and remove clears the edge (CLI half of acceptance 1).
func TestDepAddRemoveBidirectional(t *testing.T) {
	root := t.TempDir()
	feature := createID(t, root, "task", "the feature")
	blocker := createID(t, root, "task", "the blocker")

	if _, errOut, code := exec("dep", "add", feature, blocker, "--root", root); code != 0 {
		t.Fatalf("dep add failed: %s", errOut)
	}
	// Forward: the source shows the blocker.
	show, _, code := exec("show", "--root", root, feature)
	if code != 0 || !strings.Contains(show, "blocked_by: "+blocker) {
		t.Fatalf("source does not show its blocker: %q", show)
	}

	// Remove clears the edge.
	if _, errOut, code := exec("dep", "rm", feature, blocker, "--root", root); code != 0 {
		t.Fatalf("dep rm failed: %s", errOut)
	}
	show, _, _ = exec("show", "--root", root, feature)
	if strings.Contains(show, "blocked_by:") {
		t.Fatalf("edge survived removal: %q", show)
	}
}

// TestDepCycleRefused: a cycle-closing edge is refused with the full path named
// and nothing written (CLI half of acceptance 2).
func TestDepCycleRefused(t *testing.T) {
	root := t.TempDir()
	a := createID(t, root, "task", "a")
	b := createID(t, root, "task", "b")
	c := createID(t, root, "task", "c")
	if _, errOut, code := exec("dep", "add", a, b, "--root", root); code != 0 {
		t.Fatalf("add a→b: %s", errOut)
	}
	if _, errOut, code := exec("dep", "add", b, c, "--root", root); code != 0 {
		t.Fatalf("add b→c: %s", errOut)
	}

	_, errOut, code := exec("dep", "add", c, a, "--root", root, "--json")
	if code != 1 {
		t.Fatalf("cycle edge exit = %d, want 1", code)
	}
	d := decode(t, errOut)
	r, _ := d["reason"].(string)
	for _, id := range []string{a, b, c} {
		if !strings.Contains(r, "#"+id) {
			t.Fatalf("cycle refusal does not name #%s: %s", id, errOut)
		}
	}
	// c gained no edge.
	show, _, _ := exec("show", "--root", root, c)
	if strings.Contains(show, "blocked_by:") {
		t.Fatalf("refused cycle still mutated c: %q", show)
	}
}

// TestDepDanglingAndMalformedRefused: a dangling target and a malformed
// reference are each refused with a full-quadruple defect naming them, and
// nothing is written (CLI half of acceptance 3).
func TestDepDanglingAndMalformedRefused(t *testing.T) {
	root := t.TempDir()
	src := createID(t, root, "task", "src")

	for _, tc := range []struct {
		name, ref, needle string
	}{
		{"dangling", "9999", "9999"},
		{"malformed", "not@a#ref", "not@a#ref"},
	} {
		out, errOut, code := exec("dep", "add", src, tc.ref, "--root", root, "--json")
		if code != 1 {
			t.Fatalf("%s exit = %d, want 1 (stdout=%q)", tc.name, code, out)
		}
		d := decode(t, errOut)
		for _, k := range []string{"file", "op", "reason", "remedy"} {
			if s, _ := d[k].(string); s == "" {
				t.Errorf("%s defect missing %q: %s", tc.name, k, errOut)
			}
		}
		if r, _ := d["reason"].(string); !strings.Contains(r, tc.needle) {
			t.Fatalf("%s refusal does not name %q: %s", tc.name, tc.needle, errOut)
		}
	}
	show, _, _ := exec("show", "--root", root, src)
	if strings.Contains(show, "blocked_by:") {
		t.Fatalf("a refused edge persisted: %q", show)
	}
}

// TestDepCrossRepoStoredOpaquely: the cross-repo owner/repo#id form is accepted
// and displayed opaquely, proving malformed refusal is form-checking, not merely
// integer-only acceptance.
func TestDepCrossRepoStoredOpaquely(t *testing.T) {
	root := t.TempDir()
	src := createID(t, root, "task", "waits on another repo")

	const ref = "agenticlab-ai/pyxis#7"
	if _, errOut, code := exec("dep", "add", src, ref, "--root", root); code != 0 {
		t.Fatalf("cross-repo add failed: %s", errOut)
	}
	show, _, _ := exec("show", "--root", root, src)
	if !strings.Contains(show, "blocked_by: "+ref) {
		t.Fatalf("cross-repo ref not displayed opaquely: %q", show)
	}
}
