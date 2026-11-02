package argusledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const verb = "test-arm"

// testProblems is the default problems root the class-recognition tests key on
// (the relocation case is TestClassForPathRelocatedRoot below).
const testProblems = "docs/problems"

// A minimal but well-formed delivery ledger: header, the two regions, two
// journal entries separated by the `---` rule.
const goodDelivery = `# Foo — delivery ledger

| Field | Value |
|---|---|
| **Slug** | foo |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | stage 1 |

## Journal

### E-1 — stage-entered — first

- **Date**: 2026-07-11
- **Body**: the first entry

---

### E-2 — ruling-recorded — second

- **Date**: 2026-07-12
- **Body**: the second entry

---
`

const goodProblem = `# Foo — problem ledger

| Field | Value |
|---|---|
| **Slug** | foo |
| **Template version** | problem-ledger/v1 |

## Idea

do the thing

## Authority entries

### A-1 — a pre-ruling

- **Date**: 2026-07-11
- **Ruling**: the first ruling

---

### A-2 — an answer

- **Date**: 2026-07-12
- **Ruling**: the second ruling

---
`

func mustParse(t *testing.T, path, body string) *Ledger {
	t.Helper()
	l, d := Parse(testProblems, verb, path, []byte(body))
	if len(d) > 0 {
		t.Fatalf("unexpected defects parsing %s: %v", path, d)
	}
	if l == nil {
		t.Fatalf("nil ledger and no defects for %s", path)
	}
	return l
}

// A well-formed delivery ledger parses to anchor-keyed entry blocks with both
// regions delimited.
func TestDeliveryParses(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/delivery.md", goodDelivery)

	if got := l.Order; len(got) != 2 || got[0] != "E-1" || got[1] != "E-2" {
		t.Fatalf("entry order = %v, want [E-1 E-2]", got)
	}
	if _, ok := l.Entries["E-1"]; !ok {
		t.Fatal("E-1 block missing")
	}
	if !strings.Contains(string(l.Entries["E-1"]), "the first entry") {
		t.Errorf("E-1 block = %q, want the entry body", l.Entries["E-1"])
	}
	// The block is the entry content — the trailing `---` and blank lines are
	// dropped so identity is content, not layout.
	if strings.Contains(string(l.Entries["E-1"]), "---") {
		t.Errorf("E-1 block should not carry the trailing rule: %q", l.Entries["E-1"])
	}
	if strings.Contains(string(l.Entries["E-1"]), "E-2") {
		t.Errorf("E-1 block bled into E-2: %q", l.Entries["E-1"])
	}

	if l.Derived == nil || l.Derived.Name != "Derived state" {
		t.Fatalf("derived region not delimited: %+v", l.Derived)
	}
	if !strings.Contains(string(l.Derived.Bytes), "**Status**") {
		t.Errorf("derived region body = %q, want the Status field", l.Derived.Bytes)
	}
	if l.EntryRegion == nil || l.EntryRegion.Name != "Journal" {
		t.Fatalf("journal region not delimited: %+v", l.EntryRegion)
	}
	// The journal region carries the entries; the derived region does not.
	if !strings.Contains(string(l.EntryRegion.Bytes), "E-1") {
		t.Errorf("journal region should contain E-1: %q", l.EntryRegion.Bytes)
	}
	if strings.Contains(string(l.Derived.Bytes), "E-1") {
		t.Errorf("derived region must not contain journal entries: %q", l.Derived.Bytes)
	}
}

// A problem ledger parses to its A-n entry blocks; it has no derived region.
func TestProblemParses(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/problem.md", goodProblem)

	if got := l.Order; len(got) != 2 || got[0] != "A-1" || got[1] != "A-2" {
		t.Fatalf("entry order = %v, want [A-1 A-2]", got)
	}
	if !strings.Contains(string(l.Entries["A-2"]), "the second ruling") {
		t.Errorf("A-2 block = %q", l.Entries["A-2"])
	}
	if l.Derived != nil {
		t.Errorf("problem ledger has no derived region, got %+v", l.Derived)
	}
	if l.EntryRegion == nil || l.EntryRegion.Name != "Authority entries" {
		t.Fatalf("authority-entries region not delimited: %+v", l.EntryRegion)
	}
}

// A ledger whose required region cannot be delimited yields a named parse
// defect identifying the region, and no crash.
func TestUndelimitableRegionNamedDefect(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		body       string
		wantRegion string
	}{
		{
			name:       "delivery missing derived state",
			path:       "docs/problems/foo/delivery.md",
			body:       strings.Replace(goodDelivery, "## Derived state\n", "## Not derived\n", 1),
			wantRegion: "Derived state",
		},
		{
			name:       "delivery missing journal",
			path:       "docs/problems/foo/delivery.md",
			body:       strings.Replace(goodDelivery, "## Journal\n", "## Log\n", 1),
			wantRegion: "Journal",
		},
		{
			name:       "problem missing authority entries",
			path:       "docs/problems/foo/problem.md",
			body:       strings.Replace(goodProblem, "## Authority entries\n", "## Rulings\n", 1),
			wantRegion: "Authority entries",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, d := Parse(testProblems, verb, tc.path, []byte(tc.body))
			if l != nil {
				t.Fatalf("expected nil ledger on undelimitable region, got %+v", l)
			}
			if len(d) == 0 {
				t.Fatal("expected a named parse defect, got none")
			}
			found := false
			for _, x := range d {
				if x.Op != "region" {
					continue
				}
				if !strings.Contains(x.Reason, tc.wantRegion) {
					t.Errorf("defect reason %q does not name region %q", x.Reason, tc.wantRegion)
				}
				if x.Verb != verb {
					t.Errorf("defect verb = %q, want %q", x.Verb, verb)
				}
				if x.File != tc.path {
					t.Errorf("defect file = %q, want %q", x.File, tc.path)
				}
				found = true
			}
			if !found {
				t.Errorf("no region defect among %v", d)
			}
		})
	}
}

// The version-pinned registry rejects an unknown shape rather than parsing it
// leniently.
func TestVersionPinRejectsUnknownShape(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "wrong delivery version",
			path: "docs/problems/foo/delivery.md",
			body: strings.Replace(goodDelivery, "delivery-ledger/v1", "delivery-ledger/v2", 1),
		},
		{
			name: "absent version",
			path: "docs/problems/foo/delivery.md",
			body: strings.Replace(goodDelivery, "| **Template version** | delivery-ledger/v1 |\n", "", 1),
		},
		{
			name: "problem version on a delivery path",
			path: "docs/problems/foo/delivery.md",
			body: strings.Replace(goodDelivery, "delivery-ledger/v1", "problem-ledger/v1", 1),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, d := Parse(testProblems, verb, tc.path, []byte(tc.body))
			if l != nil {
				t.Fatalf("expected rejection, got a parsed ledger: %+v", l)
			}
			if len(d) != 1 || d[0].Op != "ledger-version" {
				t.Fatalf("expected one ledger-version defect, got %v", d)
			}
			if d[0].Verb != verb {
				t.Errorf("defect verb = %q, want %q", d[0].Verb, verb)
			}
		})
	}
}

// A path that is not an in-scope Argus ledger is not this parse's concern:
// (nil, nil), never a defect.
func TestNonLedgerPathIgnored(t *testing.T) {
	for _, p := range []string{
		"docs/tech-design/argus-ledger-lint.md",
		"README.md",
		"docs/problems/foo/notes.md",
		".pyxis/templates/delivery-ledger-template.md",
	} {
		l, d := Parse(testProblems, verb, p, []byte(goodDelivery))
		if l != nil || d != nil {
			t.Errorf("%s: expected (nil, nil), got (%v, %v)", p, l, d)
		}
	}
}

func TestClassForPath(t *testing.T) {
	cases := []struct {
		path string
		want string // class name, or "" for none
	}{
		{"docs/problems/foo/problem.md", "problem-ledger"},
		{"docs/problems/foo/delivery.md", "delivery-ledger"},
		{"/abs/repo/docs/problems/bar-baz/delivery.md", "delivery-ledger"},
		{"docs/problems/foo/other.md", ""},
		{"docs/problems/problem.md", ""},
		{"problem.md", ""},
	}
	for _, tc := range cases {
		c, ok := ClassForPath(testProblems, tc.path)
		if tc.want == "" {
			if ok {
				t.Errorf("%s: expected no class, got %s", tc.path, c.Name)
			}
			continue
		}
		if !ok || c.Name != tc.want {
			t.Errorf("%s: got (%s, %v), want %s", tc.path, c.Name, ok, tc.want)
		}
	}
}

// TestClassForPathRelocatedRoot proves the ledger-class recognition honors a
// non-default problems root (config-driven-portability FR-1): a ledger under
// the configured root classifies, and the default-root path no longer does —
// so a relocated corpus is read and a stale default location is not.
func TestClassForPathRelocatedRoot(t *testing.T) {
	const relocated = "ops/tickets"
	cases := []struct {
		root string
		path string
		want string
	}{
		{relocated, "ops/tickets/foo/problem.md", "problem-ledger"},
		{relocated, "ops/tickets/foo/delivery.md", "delivery-ledger"},
		{relocated, "/abs/repo/ops/tickets/bar-baz/delivery.md", "delivery-ledger"},
		// The old default location is NOT recognized under the relocated root.
		{relocated, "docs/problems/foo/delivery.md", ""},
		// And, symmetrically, the default root does not recognize the new one.
		{testProblems, "ops/tickets/foo/delivery.md", ""},
	}
	for _, tc := range cases {
		c, ok := ClassForPath(tc.root, tc.path)
		if tc.want == "" {
			if ok {
				t.Errorf("root %q, %s: expected no class, got %s", tc.root, tc.path, c.Name)
			}
			continue
		}
		if !ok || c.Name != tc.want {
			t.Errorf("root %q, %s: got (%s, %v), want %s", tc.root, tc.path, c.Name, ok, tc.want)
		}
	}
}

// A duplicate entry anchor is a named parse defect, never a silent last-wins
// overwrite of the anchor-keyed map.
func TestDuplicateAnchorDefect(t *testing.T) {
	body := strings.Replace(goodDelivery, "### E-2 —", "### E-1 —", 1)
	l, d := Parse(testProblems, verb, "docs/problems/foo/delivery.md", []byte(body))
	if l != nil {
		t.Fatalf("expected rejection on duplicate anchor, got %+v", l)
	}
	found := false
	for _, x := range d {
		if x.Op == "entry" && strings.Contains(x.Reason, "E-1") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a duplicate-anchor defect, got %v", d)
	}
}

// The feature's own committed ledgers parse green — a non-regression on the
// real corpus this parse governs.
func TestRealLedgersParseGreen(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"docs/problems/argus-ledger-lint/problem.md",
		"docs/problems/argus-ledger-lint/delivery.md",
	} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Skipf("real ledger %s unavailable: %v", rel, err)
		}
		if _, d := Parse(testProblems, verb, rel, b); len(d) > 0 {
			t.Errorf("%s parsed with defects: %v", rel, d)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found above the test dir")
	return ""
}
