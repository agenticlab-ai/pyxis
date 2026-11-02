package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// seed builds a small, fully-known backlog through the CLI and returns its root.
// The corpus mirrors the conformance fixture: #1/#2 open + agent (authorization,
// title "substrate"), #3 open + needs-plan (routing), #4 closed + hold, #5 open
// unrelated.
func seed(t *testing.T, root string) {
	t.Helper()
	mk := func(args ...string) {
		// The verb must lead; --root and the rest follow it.
		full := append([]string{args[0], "--root", root}, args[1:]...)
		if _, errOut, code := exec(full...); code != 0 {
			t.Fatalf("seed %v failed: %s", args, errOut)
		}
	}
	mk("create", "--title", "the substrate", "--body", "one file per item behind the hexagon boundary")
	mk("signal", "add", "1", "agent")
	mk("create", "--title", "substrate proof")
	mk("signal", "add", "2", "agent")
	mk("create", "--title", "draft the plan")
	mk("signal", "add", "3", "needs-plan")
	mk("create", "--title", "retired approach", "--body", "the old hexagon sketch")
	mk("signal", "add", "4", "hold")
	mk("close", "4", "--reason", "completed")
	mk("create", "--title", "unrelated chore", "--body", "tidy the desk")
}

// TestListFilters: the fixture corpus returns the exact expected set and order
// for each filter, including the authorization-role selection (acceptance
// criterion 1).
func TestListFilters(t *testing.T) {
	root := t.TempDir()
	seed(t, root)

	cases := []struct {
		name string
		args []string
		want []int
	}{
		{"all", nil, []int{1, 2, 3, 4, 5}},
		{"open", []string{"--state", "open"}, []int{1, 2, 3, 5}},
		{"closed", []string{"--state", "closed"}, []int{4}},
		{"signal-agent", []string{"--signal", "agent"}, []int{1, 2}},
		{"role-authorization", []string{"--role", "authorization"}, []int{1, 2}},
		{"role-routing", []string{"--role", "routing"}, []int{3}},
		{"open-and-agent", []string{"--state", "open", "--signal", "agent"}, []int{1, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"list", "--root", root, "--json"}, tc.args...)
			out, errOut, code := exec(args...)
			if code != 0 {
				t.Fatalf("list failed: code=%d stderr=%s", code, errOut)
			}
			view := decode(t, out)
			validateResultGolden(t, "testdata/work-item-list.schema.json", view)
			if got := viewIDs(t, view); !equalInts(got, tc.want) {
				t.Fatalf("list %s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}

	// An out-of-set filter is refused with a nonzero exit and a defect.
	if _, _, code := exec("list", "--root", root, "--state", "bogus"); code != 1 {
		t.Fatalf("out-of-set state exit = %d, want 1", code)
	}
	if _, _, code := exec("list", "--root", root, "--role", "bogus"); code != 1 {
		t.Fatalf("out-of-set role exit = %d, want 1", code)
	}
}

// TestSearchExact: search returns every exact match and only exact matches; a
// near-miss token does not match, for zero false positives (acceptance criteria
// 1 & 2). Both verbs' machine output validates against the golden schemas
// (acceptance criterion 3).
func TestSearchExact(t *testing.T) {
	root := t.TempDir()
	seed(t, root)

	// "substrate" is in #1 and #2's titles; #5's "substraight" near-miss is
	// absent from the corpus, so it is a clean fuzzy-vs-exact probe.
	out, errOut, code := exec("search", "--root", root, "--json", "substrate")
	if code != 0 {
		t.Fatalf("search failed: code=%d stderr=%s", code, errOut)
	}
	view := decode(t, out)
	validateResultGolden(t, "testdata/work-item-search.schema.json", view)
	if got := viewIDs(t, view); !equalInts(got, []int{1, 2}) {
		t.Fatalf("search substrate = %v, want [1 2]", got)
	}
	if q, _ := view["query"].(string); q != "substrate" {
		t.Fatalf("search view echoed query %q, want %q", q, "substrate")
	}

	// Body text is searched too.
	body := decode(t, mustRun(t, "search", root, "hexagon"))
	if got := viewIDs(t, body); !equalInts(got, []int{1, 4}) {
		t.Fatalf("search hexagon = %v, want [1 4]", got)
	}

	// A near-miss spelling variant matches nothing — zero false positives.
	miss := decode(t, mustRun(t, "search", root, "substraight"))
	if got := viewIDs(t, miss); len(got) != 0 {
		t.Fatalf("near-miss search matched %v, want none", got)
	}

	// Case-insensitive, but still exact (no fuzzy): uppercase query finds it.
	up := decode(t, mustRun(t, "search", root, "SUBSTRATE"))
	if got := viewIDs(t, up); !equalInts(got, []int{1, 2}) {
		t.Fatalf("case-insensitive search = %v, want [1 2]", got)
	}

	// An empty query is refused with a nonzero exit.
	if _, _, code := exec("search", "--root", root); code != 1 {
		t.Fatalf("empty search exit = %d, want 1", code)
	}
}

// TestListSearchHuman: the human forms name the items and, when empty, say so.
func TestListSearchHuman(t *testing.T) {
	root := t.TempDir()
	seed(t, root)

	out, _, code := exec("list", "--root", root, "--signal", "agent")
	if code != 0 || !strings.Contains(out, "the substrate") || !strings.Contains(out, "substrate proof") {
		t.Fatalf("human list missing items: code=%d out=%q", code, out)
	}
	empty, _, code := exec("search", "--root", root, "no-such-token-anywhere")
	if code != 0 || !strings.Contains(empty, "no items") {
		t.Fatalf("empty search human output = %q (code %d), want a no-items line", empty, code)
	}
}

func mustRun(t *testing.T, verb, root, arg string) string {
	t.Helper()
	out, errOut, code := exec(verb, "--root", root, "--json", arg)
	if code != 0 {
		t.Fatalf("%s %q failed: code=%d stderr=%s", verb, arg, code, errOut)
	}
	return out
}

// viewIDs extracts the item ids from a list/search view, in the order returned.
func viewIDs(t *testing.T, view map[string]any) []int {
	t.Helper()
	raw, ok := view["items"].([]any)
	if !ok {
		t.Fatalf("view has no items array: %v", view)
	}
	out := make([]int, 0, len(raw))
	for _, it := range raw {
		m, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("item is not an object: %v", it)
		}
		out = append(out, int(m["id"].(float64)))
	}
	if c, ok := view["count"].(float64); ok && int(c) != len(out) {
		t.Fatalf("count %d disagrees with items length %d", int(c), len(out))
	}
	return out
}

// validateResultGolden enforces a result view's golden schema — the schema
// const and the required top-level keys — plus that count equals the items
// length.
func validateResultGolden(t *testing.T, schemaPath string, view map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var sch struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Const any `json:"const"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &sch); err != nil {
		t.Fatal(err)
	}
	for _, k := range sch.Required {
		if _, ok := view[k]; !ok {
			t.Errorf("golden %s: missing required key %q", schemaPath, k)
		}
	}
	if c := sch.Properties["schema"].Const; c != nil && view["schema"] != c {
		t.Errorf("golden %s: schema = %v, want const %v", schemaPath, view["schema"], c)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
