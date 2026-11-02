package main

import (
	"strconv"
	"strings"
	"testing"
)

// createID creates an item and returns its assigned id as text.
func createID(t *testing.T, root, typ, title string) string {
	t.Helper()
	args := []string{"create", "--root", root, "--title", title, "--json"}
	if typ != "" {
		args = append(args, "--type", typ)
	}
	out, errOut, code := exec(args...)
	if code != 0 {
		t.Fatalf("create %q failed: code=%d stderr=%s", title, code, errOut)
	}
	return strconv.Itoa(int(decode(t, out)["id"].(float64)))
}

// TestParentSetClearAndRollup: parent set/clear resolves both directions and the
// epic reports exactly its open children — the CLI half of acceptance 1 and 2.
func TestParentSetClearAndRollup(t *testing.T) {
	root := t.TempDir()
	epic := createID(t, root, "epic", "the epic")
	c1 := createID(t, root, "task", "child one")
	c2 := createID(t, root, "task", "child two")

	if _, errOut, code := exec("parent", "set", c1, epic, "--root", root); code != 0 {
		t.Fatalf("parent set c1 failed: %s", errOut)
	}
	if _, errOut, code := exec("parent", "set", c2, epic, "--root", root); code != 0 {
		t.Fatalf("parent set c2 failed: %s", errOut)
	}

	// Forward: the child names its parent.
	childShow, _, code := exec("show", "--root", root, c1)
	if code != 0 || !strings.Contains(childShow, "parent: #"+epic) {
		t.Fatalf("child does not name its parent: %q", childShow)
	}
	// Reverse: the epic reports both open children.
	epicShow, _, code := exec("show", "--root", root, epic)
	if code != 0 {
		t.Fatalf("epic show failed")
	}
	if !strings.Contains(epicShow, "open children: 2") ||
		!strings.Contains(epicShow, "#"+c1) || !strings.Contains(epicShow, "#"+c2) {
		t.Fatalf("epic rollup wrong: %q", epicShow)
	}

	// Clear c1: it leaves both directions.
	if _, errOut, code := exec("parent", "clear", c1, "--root", root); code != 0 {
		t.Fatalf("parent clear failed: %s", errOut)
	}
	childShow, _, _ = exec("show", "--root", root, c1)
	if strings.Contains(childShow, "parent: #") {
		t.Fatalf("cleared child still names a parent: %q", childShow)
	}
	epicShow, _, _ = exec("show", "--root", root, epic)
	if !strings.Contains(epicShow, "open children: 1") || strings.Contains(epicShow, "#"+c1+" ") {
		t.Fatalf("rollup did not drop the cleared child: %q", epicShow)
	}
}

// TestParentMissingRefused: a parent reference to a missing item is refused with
// a defect naming the target, and nothing is written — CLI half of acceptance 3.
func TestParentMissingRefused(t *testing.T) {
	root := t.TempDir()
	child := createID(t, root, "task", "child")

	out, errOut, code := exec("parent", "set", child, "999", "--root", root, "--json")
	if code != 1 {
		t.Fatalf("missing-parent exit = %d, want 1 (stdout=%q)", code, out)
	}
	d := decode(t, errOut)
	for _, k := range []string{"file", "op", "reason", "remedy"} {
		if s, _ := d[k].(string); s == "" {
			t.Errorf("defect missing %q: %s", k, errOut)
		}
	}
	if r, _ := d["reason"].(string); !strings.Contains(r, "999") {
		t.Fatalf("refusal does not name the missing target: %s", errOut)
	}
	// The child was not repointed.
	childShow, _, _ := exec("show", "--root", root, child)
	if strings.Contains(childShow, "parent: #") {
		t.Fatalf("child was repointed despite the refusal: %q", childShow)
	}
}
