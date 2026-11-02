package main

import (
	"strings"
	"testing"
)

// TestListParentCLI: `list --parent <id>` returns exactly the item's direct
// children (all states by default), composes with --state, and echoes the parent
// in --json — the CLI half of prd:local-work-tracker/FR-19 (plan-002 T2).
func TestListParentCLI(t *testing.T) {
	root := t.TempDir()
	epic := createID(t, root, "epic", "epic")
	c1 := createID(t, root, "task", "child one")
	c2 := createID(t, root, "task", "child two")
	unrelated := createID(t, root, "task", "unrelated")
	for _, c := range []string{c1, c2} {
		if _, errOut, code := exec("parent", "set", c, epic, "--root", root); code != 0 {
			t.Fatalf("parent set %s: %s", c, errOut)
		}
	}

	// All states by default: exactly the two children, never the unrelated item.
	out, errOut, code := exec("list", "--parent", epic, "--root", root)
	if code != 0 {
		t.Fatalf("list --parent: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "#"+c1) || !strings.Contains(out, "#"+c2) {
		t.Fatalf("list --parent missing a child: %q", out)
	}
	if strings.Contains(out, "#"+unrelated+" ") {
		t.Fatalf("list --parent leaked an unrelated item: %q", out)
	}

	// --json echoes the parent filter and the count.
	jout, _, code := exec("list", "--parent", epic, "--json", "--root", root)
	if code != 0 {
		t.Fatalf("list --parent --json: %d", code)
	}
	view := decode(t, jout)
	q, _ := view["query"].(map[string]any)
	if q["parent"] != epic {
		t.Fatalf("json query.parent = %v, want %q: %s", q["parent"], epic, jout)
	}
	if cnt, _ := view["count"].(float64); int(cnt) != 2 {
		t.Fatalf("json count = %v, want 2", view["count"])
	}

	// Close one child; --state open narrows to the other (the filter composes).
	if _, errOut, code := exec("close", c1, "--reason", "completed", "--author", "t", "--root", root); code != 0 {
		t.Fatalf("close: %s", errOut)
	}
	openOut, _, _ := exec("list", "--parent", epic, "--state", "open", "--root", root)
	if !strings.Contains(openOut, "#"+c2) || strings.Contains(openOut, "#"+c1+" ") {
		t.Fatalf("list --parent --state open wrong: %q", openOut)
	}
}

// TestReindexCLI: `carina reindex` rebuilds the index and reports the marker and
// item counts, in both human and schema-versioned machine form (plan-002 T3).
func TestReindexCLI(t *testing.T) {
	root := t.TempDir()
	epic := createID(t, root, "epic", "epic")
	c1 := createID(t, root, "task", "c1")
	c2 := createID(t, root, "task", "c2")
	for _, c := range []string{c1, c2} {
		if _, errOut, code := exec("parent", "set", c, epic, "--root", root); code != 0 {
			t.Fatalf("parent set: %s", errOut)
		}
	}

	// Human form names the marker and item counts.
	out, errOut, code := exec("reindex", "--root", root)
	if code != 0 {
		t.Fatalf("reindex: code=%d stderr=%s", code, errOut)
	}
	if !strings.Contains(out, "2 markers") || !strings.Contains(out, "3 items") {
		t.Fatalf("reindex human output wrong: %q", out)
	}

	// Machine form is schema-versioned with the counts.
	jout, _, code := exec("reindex", "--json", "--root", root)
	if code != 0 {
		t.Fatalf("reindex --json: %d", code)
	}
	rep := decode(t, jout)
	if rep["schema"] != "worktracker.reindex-report/v1" {
		t.Fatalf("reindex schema = %v", rep["schema"])
	}
	if m, _ := rep["markers"].(float64); int(m) != 2 {
		t.Fatalf("reindex markers = %v, want 2", rep["markers"])
	}
	if it, _ := rep["items"].(float64); int(it) != 3 {
		t.Fatalf("reindex items = %v, want 3", rep["items"])
	}
}

// TestShowGroupedRollup: `show <epic>` reports its children grouped by open and
// closed, so a completed task still surfaces in the closed group rather than
// vanishing from the open-only view (plan-002 T4, FR-7).
func TestShowGroupedRollup(t *testing.T) {
	root := t.TempDir()
	epic := createID(t, root, "epic", "epic")
	oc := createID(t, root, "task", "open child")
	cc := createID(t, root, "task", "closed child")
	for _, c := range []string{oc, cc} {
		if _, errOut, code := exec("parent", "set", c, epic, "--root", root); code != 0 {
			t.Fatalf("parent set %s: %s", c, errOut)
		}
	}
	if _, errOut, code := exec("close", cc, "--reason", "completed", "--author", "t", "--root", root); code != 0 {
		t.Fatalf("close: %s", errOut)
	}

	out, _, code := exec("show", "--root", root, epic)
	if code != 0 {
		t.Fatalf("show failed: %d", code)
	}
	if !strings.Contains(out, "open children: 1") || !strings.Contains(out, "#"+oc) {
		t.Fatalf("show missing the open group: %q", out)
	}
	if !strings.Contains(out, "closed children: 1") || !strings.Contains(out, "#"+cc) {
		t.Fatalf("show missing the closed group (a completed task must still surface): %q", out)
	}
}
