package main

import (
	"strconv"
	"strings"
	"testing"
)

// createItem creates an item under root and returns its id.
func createItem(t *testing.T, root, title, body string) string {
	t.Helper()
	args := []string{"create", "--root", root, "--title", title, "--json"}
	if body != "" {
		args = append(args, "--body", body)
	}
	out, errOut, code := exec(args...)
	if code != 0 {
		t.Fatalf("create failed: code=%d stderr=%s", code, errOut)
	}
	return strconv.Itoa(int(decode(t, out)["id"].(float64)))
}

// TestEditVerbTouchesOnlyNamedFields: `carina edit --title` changes the title
// and leaves the body — and every other field — as it was (acceptance 1).
func TestEditVerbTouchesOnlyNamedFields(t *testing.T) {
	root := t.TempDir()
	id := createItem(t, root, "before", "body stays")

	out, errOut, code := exec("edit", "--root", root, id, "--title", "after", "--json")
	if code != 0 {
		t.Fatalf("edit failed: code=%d stderr=%s", code, errOut)
	}
	edited := decode(t, out)
	if edited["title"] != "after" {
		t.Fatalf("title not updated: %v", edited["title"])
	}
	if edited["body"] != "body stays" {
		t.Fatalf("edit disturbed the body: %v", edited["body"])
	}
	if edited["state"] != "open" || edited["type"] != "task" {
		t.Fatalf("edit disturbed a structural field: %v", edited)
	}
}

// TestEditBodyOnly: editing the body leaves the title untouched.
func TestEditBodyOnly(t *testing.T) {
	root := t.TempDir()
	id := createItem(t, root, "keep title", "old body")
	out, _, code := exec("edit", "--root", root, id, "--body", "new body", "--json")
	if code != 0 {
		t.Fatalf("edit body failed: %d", code)
	}
	m := decode(t, out)
	if m["title"] != "keep title" || m["body"] != "new body" {
		t.Fatalf("body-only edit wrong: %v", m)
	}
}

// TestCloseReopenVerbs: close records the reason and closed state; reopen clears
// the reason and returns to open (acceptance 2, the applied-change half).
func TestCloseReopenVerbs(t *testing.T) {
	root := t.TempDir()
	id := createItem(t, root, "lifecycle", "")

	out, errOut, code := exec("close", "--root", root, id, "--json")
	if code != 0 {
		t.Fatalf("close failed: code=%d stderr=%s", code, errOut)
	}
	closed := decode(t, out)
	if closed["state"] != "closed" || closed["close_reason"] != "completed" {
		t.Fatalf("close result wrong: %v", closed)
	}

	out, _, code = exec("reopen", "--root", root, id, "--json")
	if code != 0 {
		t.Fatalf("reopen failed: %d", code)
	}
	reopened := decode(t, out)
	if reopened["state"] != "open" || reopened["close_reason"] != "" {
		t.Fatalf("reopen result wrong: %v", reopened)
	}
}

// TestCloseIdempotentNoOp: closing an already-closed item succeeds (exit 0) and
// leaves it closed with exactly one transition entry (acceptance 2).
func TestCloseIdempotentNoOp(t *testing.T) {
	root := t.TempDir()
	id := createItem(t, root, "idem", "")
	if _, errOut, code := exec("close", "--root", root, id); code != 0 {
		t.Fatalf("first close failed: %s", errOut)
	}
	out, _, code := exec("close", "--root", root, id, "--json")
	if code != 0 {
		t.Fatalf("re-close should succeed as a no-op, got exit %d", code)
	}
	m := decode(t, out)
	if m["state"] != "closed" {
		t.Fatalf("re-close changed state: %v", m["state"])
	}
	log, _ := m["log"].([]any)
	if len(log) != 1 {
		t.Fatalf("re-close appended an audit entry: log has %d entries", len(log))
	}
}

// TestCloseRejectsOutOfSetReason: `--reason bogus` is refused with a defect that
// names the value and the allowed set, exit 1, and no mutation (acceptance 3).
func TestCloseRejectsOutOfSetReason(t *testing.T) {
	root := t.TempDir()
	id := createItem(t, root, "guarded", "")

	out, errOut, code := exec("close", "--root", root, id, "--reason", "bogus", "--json")
	if code != 1 {
		t.Fatalf("out-of-set reason exit = %d, want 1 (stdout=%q)", code, out)
	}
	d := decode(t, errOut)
	if r, _ := d["reason"].(string); !strings.Contains(r, "bogus") {
		t.Fatalf("defect does not name the value: %q", r)
	}
	if rem, _ := d["remedy"].(string); !strings.Contains(rem, "completed") || !strings.Contains(rem, "rejected") {
		t.Fatalf("defect does not name the allowed set: %q", rem)
	}
	// Fail closed: the item is still open.
	show, _, _ := exec("show", "--root", root, id, "--json")
	if decode(t, show)["state"] != "open" {
		t.Fatalf("refused close still mutated the item")
	}
}

// TestEditAndTransitionMissingItem: a lifecycle verb on a missing id fails
// closed (exit 1) naming its own verb.
func TestEditAndTransitionMissingItem(t *testing.T) {
	root := t.TempDir()
	// The store must exist (config load) — create then reference a gap id.
	createItem(t, root, "seed", "")
	for _, verb := range []string{"edit", "close", "reopen"} {
		_, errOut, code := exec(verb, "--root", root, "99", "--json")
		if code != 1 {
			t.Fatalf("%s missing item exit = %d, want 1", verb, code)
		}
		if !strings.Contains(errOut, "no work item") {
			t.Fatalf("%s missing item defect wrong: %s", verb, errOut)
		}
	}
}
