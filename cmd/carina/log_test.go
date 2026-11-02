package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// logEntries decodes the schema-versioned `log` array from an item's machine view.
func logEntries(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	raw, ok := m["log"]
	if !ok {
		t.Fatalf("machine view has no log key: %v", m)
	}
	b, _ := json.Marshal(raw)
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("log is not an array of objects: %v", err)
	}
	return out
}

// TestCommentsAppendInOrder: comments accumulate on a work item forever, in the
// order written, each landing one entry (FR-5, T3 acceptance 1 over the CLI).
func TestCommentsAppendInOrder(t *testing.T) {
	root := t.TempDir()
	exec("create", "--root", root, "--title", "dialogue", "--json")

	for _, c := range []struct{ author, body string }{
		{"alice", "one"}, {"bob", "two"}, {"alice", "three"},
	} {
		if _, errOut, code := exec("comment", "--root", root, "1", "--author", c.author, "--body", c.body); code != 0 {
			t.Fatalf("comment %q failed: %s", c.body, errOut)
		}
	}

	out, _, code := exec("show", "--root", root, "1", "--json")
	if code != 0 {
		t.Fatalf("show: code=%d", code)
	}
	log := logEntries(t, decode(t, out))
	if len(log) != 3 {
		t.Fatalf("want 3 comments, got %d", len(log))
	}
	wantBodies := []string{"one", "two", "three"}
	for i, e := range log {
		if e["kind"] != "comment" {
			t.Errorf("entry %d kind = %v, want comment", i, e["kind"])
		}
		if e["payload"] != wantBodies[i] {
			t.Errorf("entry %d payload = %v, want %q", i, e["payload"], wantBodies[i])
		}
	}
}

// TestRecordLatestWinsAndIdempotentRepeat: the latest record per namespace is
// authoritative and an identical repeat is a recorded no-op (FR-9, T3 acceptance
// 2 over the CLI).
func TestRecordLatestWinsAndIdempotentRepeat(t *testing.T) {
	root := t.TempDir()
	exec("create", "--root", root, "--title", "decide", "--json")

	rec := func(ns, body string) (map[string]any, int) {
		out, _, code := exec("record", "--root", root, "1", ns, "--body", body, "--author", "t", "--json")
		if code != 0 {
			return nil, code
		}
		return decode(t, out), code
	}

	rec("triage-disposition", "plan")
	m, code := rec("triage-disposition", "operator")
	if code != 0 {
		t.Fatalf("record operator: code=%d", code)
	}
	// Two distinct records so far; latest is operator.
	if n := countKind(logEntries(t, m), "decision-record"); n != 2 {
		t.Fatalf("want 2 decision-records, got %d", n)
	}

	// An identical repeat of the latest writes nothing — still two records.
	m, _ = rec("triage-disposition", "operator")
	if n := countKind(logEntries(t, m), "decision-record"); n != 2 {
		t.Fatalf("identical repeat appended a record: now %d", n)
	}
	if got := latest(logEntries(t, m), "triage-disposition"); got != "operator" {
		t.Fatalf("latest triage-disposition = %q, want operator", got)
	}

	// A second namespace is independent.
	m, _ = rec("charter", "ship the slice")
	if got := latest(logEntries(t, m), "charter"); got != "ship the slice" {
		t.Fatalf("charter latest = %q", got)
	}
	if got := latest(logEntries(t, m), "triage-disposition"); got != "operator" {
		t.Fatalf("charter write disturbed triage-disposition latest: %q", got)
	}
}

// TestRecordOutOfSetNamespaceRefused: an out-of-set namespace is refused with a
// defect naming the value, and nothing lands (FR-9, T3 acceptance 3 over the CLI).
func TestRecordOutOfSetNamespaceRefused(t *testing.T) {
	root := t.TempDir()
	exec("create", "--root", root, "--title", "guarded", "--json")

	_, errOut, code := exec("record", "--root", root, "1", "bogus-ns", "--body", "x", "--json")
	if code != 1 {
		t.Fatalf("out-of-set namespace exit = %d, want 1", code)
	}
	d := decode(t, errOut)
	if r, _ := d["reason"].(string); !strings.Contains(r, "bogus-ns") {
		t.Fatalf("defect does not name the value: %s", errOut)
	}
	for _, k := range []string{"op", "reason", "remedy"} {
		if s, _ := d[k].(string); s == "" {
			t.Errorf("defect missing %q: %s", k, errOut)
		}
	}

	// Nothing landed on the item.
	out, _, _ := exec("show", "--root", root, "1", "--json")
	if n := countKind(logEntries(t, decode(t, out)), "decision-record"); n != 0 {
		t.Fatalf("a refused record still wrote to the log: %d", n)
	}
}

func countKind(log []map[string]any, kind string) int {
	n := 0
	for _, e := range log {
		if e["kind"] == kind {
			n++
		}
	}
	return n
}

func latest(log []map[string]any, namespace string) string {
	payload := ""
	for _, e := range log {
		if e["kind"] == "decision-record" && e["namespace"] == namespace {
			payload, _ = e["payload"].(string)
		}
	}
	return payload
}
