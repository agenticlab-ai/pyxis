package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// exec runs the CLI with args, returning stdout, stderr, and the exit code.
func exec(args ...string) (string, string, int) {
	var out, err bytes.Buffer
	code := run(args, &out, &err)
	return out.String(), err.String(), code
}

// TestCreateShowRoundTrip: create then show round-trips every header field and
// the body, in both human and machine output, and the machine output validates
// against the golden schema (acceptance criterion 1).
func TestCreateShowRoundTrip(t *testing.T) {
	root := t.TempDir()
	out, errOut, code := exec("create", "--root", root, "--type", "epic",
		"--title", "Ship the substrate", "--body", "First slice.\n\nWith prose.",
		"--priority", "high", "--json")
	if code != 0 {
		t.Fatalf("create failed: code=%d stderr=%s", code, errOut)
	}
	created := decode(t, out)

	id := strconv.Itoa(int(created["id"].(float64)))
	showOut, _, code := exec("show", "--root", root, "--json", id)
	if code != 0 {
		t.Fatalf("show failed: code=%d", code)
	}
	shown := decode(t, showOut)

	// Every field round-trips: the shown machine object equals the created one.
	if !bytes.Equal(canon(t, created), canon(t, shown)) {
		t.Fatalf("machine output not identical across create/show:\ncreate=%s\nshow=%s", out, showOut)
	}
	for k, want := range map[string]any{
		"schema": "worktracker.work-item/v1", "type": "epic", "state": "open",
		"title": "Ship the substrate", "priority": "high", "body": "First slice.\n\nWith prose.",
	} {
		if shown[k] != want {
			t.Errorf("field %q = %v, want %v", k, shown[k], want)
		}
	}
	validateAgainstGolden(t, shown)

	// Human output names the item and body.
	humanShow, _, code := exec("show", "--root", root, id)
	if code != 0 || !strings.Contains(humanShow, "Ship the substrate") || !strings.Contains(humanShow, "With prose.") {
		t.Fatalf("human show missing fields: code=%d out=%q", code, humanShow)
	}
}

// TestSequentialIDs: 200 sequential creates yield 200 distinct, never-reused
// ids, each resolvable by show (acceptance criterion 2).
func TestSequentialIDs(t *testing.T) {
	root := t.TempDir()
	seen := map[int]bool{}
	for i := 0; i < 200; i++ {
		out, errOut, code := exec("create", "--root", root, "--title", "x", "--json")
		if code != 0 {
			t.Fatalf("create %d failed: %s", i, errOut)
		}
		id := int(decode(t, out)["id"].(float64))
		if seen[id] {
			t.Fatalf("id %d reused at create %d", id, i)
		}
		seen[id] = true
		if _, _, code := exec("show", "--root", root, strconv.Itoa(id)); code != 0 {
			t.Fatalf("show #%d failed", id)
		}
	}
	if len(seen) != 200 {
		t.Fatalf("expected 200 distinct ids, got %d", len(seen))
	}
}

// TestMalformedYieldsDefect: a hand-corrupted item shows a defect naming file,
// operation, reason, and remedy, while every other item stays readable
// (acceptance criterion 3).
func TestMalformedYieldsDefect(t *testing.T) {
	root := t.TempDir()
	exec("create", "--root", root, "--title", "corrupt me", "--json")
	exec("create", "--root", root, "--title", "leave me be", "--json")

	// Hand-corrupt item #1 on disk.
	itemsDir := root + "/.workitems/items"
	if err := os.WriteFile(itemsDir+"/1.md", []byte("garbage, not a work item\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := exec("show", "--root", root, "--json", "1")
	if code != 1 {
		t.Fatalf("corrupt show exit = %d, want 1 (stdout=%q)", code, out)
	}
	d := decode(t, errOut)
	for _, k := range []string{"file", "op", "reason", "remedy"} {
		if s, _ := d[k].(string); s == "" {
			t.Errorf("defect missing %q: %s", k, errOut)
		}
	}
	// The sibling item is untouched.
	if _, _, code := exec("show", "--root", root, "2"); code != 0 {
		t.Fatalf("sibling item #2 unreadable after corrupting #1")
	}
}

// A flag may trail the positional id: `carina show 1 --json` must still emit
// machine output (stdlib flag alone stops at the first non-flag).
func TestFlagAfterPositional(t *testing.T) {
	root := t.TempDir()
	exec("create", "--root", root, "--title", "x", "--json")
	out, _, code := exec("show", "--root", root, "1", "--json")
	if code != 0 {
		t.Fatalf("show exit = %d", code)
	}
	if m := decode(t, out); m["schema"] != "worktracker.work-item/v1" {
		t.Fatalf("trailing --json not honored: %s", out)
	}
}

func TestUsageAndErrors(t *testing.T) {
	if _, _, code := exec(); code != 2 {
		t.Fatalf("no-args exit = %d, want 2", code)
	}
	if _, _, code := exec("bogus"); code != 2 {
		t.Fatalf("unknown verb exit = %d, want 2", code)
	}
	if _, _, code := exec("create", "--root", t.TempDir()); code != 1 {
		t.Fatalf("create without title exit = %d, want 1", code)
	}
	if _, _, code := exec("show", "--root", t.TempDir(), "nope"); code != 1 {
		t.Fatalf("non-numeric show id exit = %d, want 1", code)
	}
	for _, verb := range []string{"edit", "close", "reopen", "comment", "record"} {
		if _, _, code := exec(verb, "--root", t.TempDir()); code != 1 {
			t.Errorf("%s without id exit = %d, want 1", verb, code)
		}
	}
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &m); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, s)
	}
	return m
}

func canon(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// validateAgainstGolden enforces the golden JSON Schema's structural clauses —
// required keys, the schema const, and closed-enum fields — over an item view.
func validateAgainstGolden(t *testing.T, item map[string]any) {
	t.Helper()
	raw, err := os.ReadFile("testdata/work-item.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var sch struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Const any   `json:"const"`
			Enum  []any `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &sch); err != nil {
		t.Fatal(err)
	}
	for _, k := range sch.Required {
		if _, ok := item[k]; !ok {
			t.Errorf("golden schema: missing required key %q", k)
		}
	}
	for k, p := range sch.Properties {
		v, ok := item[k]
		if !ok {
			continue
		}
		if p.Const != nil && v != p.Const {
			t.Errorf("golden schema: %q = %v, want const %v", k, v, p.Const)
		}
		if p.Enum != nil && !contains(p.Enum, v) {
			t.Errorf("golden schema: %q = %v not in enum %v", k, v, p.Enum)
		}
	}
}

func contains(set []any, v any) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
