package argusledger

import (
	"strings"
	"testing"
)

// A delivery ledger whose journal stops on external completions and registers,
// refreshes, and clears awaits. The derived block deliberately carries a STALE
// await surfacing (a completion long gone) to prove re-derivation reads the
// journal alone and never trusts the projection (DINV-6).
const awaitDelivery = `# Foo — delivery ledger

| Field | Value |
|---|---|
| **Slug** | foo |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | stage 7 |
| **Open items** | none <!--awaits-->await(pr:999; merged; since 2026-01-01)<!--/awaits--> |
| **Park state** | — |

## Journal

### E-1 — stage-entered — began

- **Date**: 2026-07-10
- **Body**: entered the stage

---

### E-2 — proposal-disposed — stop on the introducing PR

- **Date**: 2026-07-11
- **Body**: opened the introducing PR; the drive stops awaiting its merge
- **Await**: pr:214 · merged and its content an ancestor of main · since 2026-07-11

---

### E-3 — item-materialized — stop on a fleet item

- **Date**: 2026-07-12
- **Body**: projected a plan; awaiting the fleet item landing
- **Await**: item:argus/epic#7 · landed with the automated-implementation tag · since 2026-07-12

---

### E-4 — reconciliation — the PR merged

- **Date**: 2026-07-13
- **Body**: verified pr:214 merged at the goal object; the await clears
- **Await-cleared**: pr:214

---
`

func TestDeriveAwaitsFromJournalAlone(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/delivery.md", awaitDelivery)

	awaits, defects := DeriveAwaits(verb, l)
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	// pr:214 was registered (E-2) then cleared (E-4); item#7 remains.
	if len(awaits) != 1 {
		t.Fatalf("outstanding awaits = %d, want 1: %+v", len(awaits), awaits)
	}
	got := awaits[0]
	if got.GoalObject != "item:argus/epic#7" {
		t.Errorf("goal object = %q, want item:argus/epic#7", got.GoalObject)
	}
	if got.Condition != "landed with the automated-implementation tag" {
		t.Errorf("condition = %q", got.Condition)
	}
	if got.Since != "2026-07-12" {
		t.Errorf("since = %q, want 2026-07-12", got.Since)
	}
	// The stale derived-block surfacing (pr:999) must never leak into the set:
	// re-derivation is journal-authoritative, the projection is never trusted.
	for _, a := range awaits {
		if strings.Contains(a.GoalObject, "999") {
			t.Errorf("derived-block await pr:999 leaked into the re-derived set: %+v", a)
		}
	}
}

// A re-registration on the same goal object refreshes its condition and since
// in place — the latest journal statement wins, still one await.
func TestDeriveAwaitsReRegistrationRefreshes(t *testing.T) {
	body := strings.Replace(awaitDelivery,
		"- **Await-cleared**: pr:214",
		"- **Await**: pr:214 · merged, re-checked after a force-push · since 2026-07-13", 1)
	l := mustParse(t, "docs/problems/foo/delivery.md", body)

	awaits, defects := DeriveAwaits(verb, l)
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if len(awaits) != 2 {
		t.Fatalf("awaits = %d, want 2: %+v", len(awaits), awaits)
	}
	// Registration order is preserved: pr:214 first (E-2), item#7 second (E-3).
	if awaits[0].GoalObject != "pr:214" || awaits[1].GoalObject != "item:argus/epic#7" {
		t.Fatalf("order = [%s %s], want [pr:214 item:argus/epic#7]", awaits[0].GoalObject, awaits[1].GoalObject)
	}
	if awaits[0].Since != "2026-07-13" || !strings.Contains(awaits[0].Condition, "force-push") {
		t.Errorf("pr:214 was not refreshed by re-registration: %+v", awaits[0])
	}
}

// The closed grammar refuses a malformed await line with a named defect — never
// a silent skip that would let the tick miss or misread an await (R1).
func TestAwaitGrammarClosed(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"missing since", "- **Await**: pr:214 · merged"},
		{"missing condition", "- **Await**: pr:214 · since 2026-07-11"},
		{"bad date", "- **Await**: pr:214 · merged · since 2026-7-1"},
		{"empty goal object", "- **Await**:  · merged · since 2026-07-11"},
		{"clear with no goal", "- **Await-cleared**:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(awaitDelivery,
				"- **Await**: pr:214 · merged and its content an ancestor of main · since 2026-07-11",
				tc.line, 1)
			l := mustParse(t, "docs/problems/foo/delivery.md", body)
			awaits, defects := DeriveAwaits(verb, l)
			if len(defects) == 0 {
				t.Fatalf("expected a named grammar defect for %q, got none (awaits=%+v)", tc.line, awaits)
			}
			d := defects[0]
			if d.Op != "await-grammar" {
				t.Errorf("defect op = %q, want await-grammar", d.Op)
			}
			if d.Verb != verb || d.File != "docs/problems/foo/delivery.md" {
				t.Errorf("defect verb/file = %q/%q", d.Verb, d.File)
			}
			if !strings.Contains(d.Reason, "E-2") {
				t.Errorf("defect reason %q does not name the entry E-2", d.Reason)
			}
		})
	}
}

// A well-formed await line is never flagged — closedness must not false-red the
// legal grammar.
func TestAwaitGrammarAcceptsWellFormed(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/delivery.md", awaitDelivery)
	if _, defects := DeriveAwaits(verb, l); len(defects) != 0 {
		t.Fatalf("well-formed awaits flagged: %v", defects)
	}
}

// A line that is not an await line (a foreign bold label) is left entirely
// alone — the grammar hooks only on the Await/Await-cleared labels.
func TestNonAwaitLinesIgnored(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/delivery.md", awaitDelivery)
	awaits, defects := DeriveAwaits(verb, l)
	if len(defects) != 0 || len(awaits) != 1 {
		t.Fatalf("foreign labels disturbed the parse: awaits=%+v defects=%v", awaits, defects)
	}
}

// The surfacing puts one entry per outstanding await inside the Open-items cell
// and clearing an await leaves the block with every other byte unchanged.
func TestRegenerateOpenItemsByteStable(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/delivery.md", awaitDelivery)

	twoAwaits := []Await{
		{GoalObject: "pr:214", Condition: "merged", Since: "2026-07-11"},
		{GoalObject: "item:argus/epic#7", Condition: "landed", Since: "2026-07-12"},
	}
	full, defects := RegenerateOpenItems(verb, l, twoAwaits)
	if len(defects) != 0 {
		t.Fatalf("unexpected defects: %v", defects)
	}
	if !strings.Contains(string(full), "await(pr:214; merged; since 2026-07-11)") ||
		!strings.Contains(string(full), "await(item:argus/epic#7; landed; since 2026-07-12)") {
		t.Fatalf("both awaits not surfaced:\n%s", full)
	}
	// Idempotent: regenerating the same set reproduces the bytes exactly.
	l.Derived.Bytes = full
	again, _ := RegenerateOpenItems(verb, l, twoAwaits)
	if string(again) != string(full) {
		t.Fatalf("regeneration not idempotent:\n--- first ---\n%s\n--- again ---\n%s", full, again)
	}

	// Clear pr:214: the block must change ONLY by removing that await's entry;
	// every other line — Status, Stage position, Park state, and the Open-items
	// cell's own "none" — is byte-for-byte unchanged.
	oneAwait := []Await{twoAwaits[1]}
	cleared, _ := RegenerateOpenItems(verb, l, oneAwait)
	assertOnlyAwaitLineDiffers(t, string(full), string(cleared))
	if strings.Contains(string(cleared), "pr:214") {
		t.Errorf("cleared await pr:214 still surfaced:\n%s", cleared)
	}

	// Clear the last await too: the cell is byte-restored to its pre-await form
	// (no sentinels, the "none" content intact).
	l.Derived.Bytes = cleared
	empty, _ := RegenerateOpenItems(verb, l, nil)
	if strings.Contains(string(empty), "awaits") || strings.Contains(string(empty), "await(") {
		t.Errorf("fully-cleared block still carries an await region:\n%s", empty)
	}
	for _, want := range []string{"| **Open items** | none |", "| **Status** | open |", "| **Park state** | — |"} {
		if !strings.Contains(string(empty), want) {
			t.Errorf("byte-restored block missing %q:\n%s", want, empty)
		}
	}
}

// assertOnlyAwaitLineDiffers fails unless the two blocks differ on exactly the
// Open-items row and nowhere else.
func assertOnlyAwaitLineDiffers(t *testing.T, a, b string) {
	t.Helper()
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	if len(la) != len(lb) {
		t.Fatalf("line count changed %d → %d:\n--- a ---\n%s\n--- b ---\n%s", len(la), len(lb), a, b)
	}
	for i := range la {
		if la[i] == lb[i] {
			continue
		}
		if !strings.Contains(la[i], "**Open items**") {
			t.Errorf("line %d changed outside the Open-items row:\n  a: %q\n  b: %q", i, la[i], lb[i])
		}
	}
}

// A delivery ledger with no Open-items row cannot surface awaits: a named
// defect, never a silent no-op.
func TestRegenerateOpenItemsMissingRow(t *testing.T) {
	body := strings.Replace(awaitDelivery,
		"| **Open items** | none <!--awaits-->await(pr:999; merged; since 2026-01-01)<!--/awaits--> |\n", "", 1)
	l := mustParse(t, "docs/problems/foo/delivery.md", body)
	_, defects := RegenerateOpenItems(verb, l, []Await{{GoalObject: "pr:1", Condition: "merged", Since: "2026-07-11"}})
	if len(defects) != 1 || defects[0].Op != "derived-awaits" {
		t.Fatalf("expected one derived-awaits defect, got %v", defects)
	}
}

// Awaits are a delivery-ledger concept; a problem ledger (no derived block)
// yields no awaits and no defects.
func TestDeriveAwaitsProblemLedgerNoop(t *testing.T) {
	l := mustParse(t, "docs/problems/foo/problem.md", goodProblem)
	if awaits, defects := DeriveAwaits(verb, l); awaits != nil || defects != nil {
		t.Errorf("problem ledger produced awaits/defects: %+v %v", awaits, defects)
	}
}
