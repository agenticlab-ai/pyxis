package authoritycontract

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const testVerb = "check-authority-contract"
const testFile = ".pyxis/skills/argus/authority-contract.md"

// stdLadder is the pinned legal ladder (argus-artifacts §3.3) — the ∅ revoked
// bundle, the additive `+`/`·` notation, and tier-4's trailing italic note the
// token parse must all read correctly.
const stdLadder = "| Tier (ordered) | Bundle (action classes) |\n" +
	"|---|---|\n" +
	"| `revoked` | ∅ — every act escalates; the kill switch |\n" +
	"| `floor` | dispose-advisory-finding |\n" +
	"| `tier-2` | + dispose-blocking-finding · sign-gate-verdict · trigger-materialization |\n" +
	"| `tier-3` | + rule-acceptance |\n" +
	"| `tier-4` | + approve-merge *(activates only after the operator changes org governance)* |\n"

// stdHistory is a legal two-row history landing at tier-2 (matches a tier-2
// current pointer). bornHistory is the single seed row at the initial tier.
const stdHistory = "| Date | Move | Tier | Evidence |\n" +
	"|---|---|---|---|\n" +
	"| 2026-07-11 | none | floor | born at floor — the introducing PR |\n" +
	"| 2026-07-11 | promotion | tier-2 | operator-directed early promotion |\n"

const bornHistory = "| Date | Move | Tier | Evidence |\n" +
	"|---|---|---|---|\n" +
	"| 2026-07-11 | none | floor | born at floor — the introducing PR |\n"

// emptyHistory is a heading-and-header-only change history (zero data rows).
const emptyHistory = "| Date | Move | Tier | Evidence |\n|---|---|---|---|\n"

// contract assembles a full authority-contract file from a current-tier
// pointer, a ladder table, and a change-history table.
func contract(currentTier, ladder, history string) string {
	return "# Argus authority contract — test\n\n" +
		"| Field | Value |\n|---|---|\n" +
		"| **Current tier** | " + currentTier + " |\n" +
		"| **Template version** | authority-contract/v1 |\n\n" +
		"## The ladder\n\n" + ladder + "\n" +
		"## Change history\n\n" + history
}

// parseAndEval runs the full arm pipeline over a contract body — a parse defect
// short-circuits the evaluation, exactly as the gate descriptor composes them.
func parseAndEval(t *testing.T, body string) []schema.Defect {
	t.Helper()
	c, pd := Parse(testVerb, testFile, []byte(body))
	if c == nil {
		return pd
	}
	if len(pd) != 0 {
		t.Fatalf("Parse returned both a contract and defects: %v", pd)
	}
	return Evaluate(testVerb, testFile, c)
}

// TestLegalAndBornPass is A-1/A-3's green spine: a legal contract, the born
// contract (one `move: none` seed at the initial tier), and the empty-history
// born exemption (zero rows while Current tier is the initial tier) each raise
// zero defects.
func TestLegalAndBornPass(t *testing.T) {
	greens := map[string]string{
		"legal (tier-2, matching latest row)": contract("tier-2", stdLadder, stdHistory),
		"born (single seed row at floor)":     contract("floor", stdLadder, bornHistory),
		"empty history at the initial tier":   contract("floor", stdLadder, emptyHistory),
		"backticked current tier":             contract("`tier-2`", stdLadder, stdHistory),
	}
	for name, body := range greens {
		if d := parseAndEval(t, body); len(d) != 0 {
			t.Errorf("%s: want zero defects, got %d: %v", name, len(d), d)
		}
	}
}

// TestDINV4 is A-1: current-tier-not-in-ladder, latest-history-tier mismatch,
// and empty-history-under-a-non-initial-tier each report exactly one named
// defect on the file, distinct in Op.
func TestDINV4(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		op     string
		needle string
	}{
		{
			// current tier not in ladder; latest row also tier-99 so ONLY
			// tier-in-ladder fires (no incidental bare-pointer-flip).
			"current-tier not in ladder",
			contract("tier-99", stdLadder,
				"| Date | Move | Tier | Evidence |\n|---|---|---|---|\n"+
					"| 2026-07-11 | none | floor | born |\n"+
					"| 2026-07-11 | promotion | tier-99 | ev |\n"),
			OpTierInLadder, "tier-99",
		},
		{
			// current tier-3 (in the ladder) but the latest row lands at tier-2.
			"latest-history-tier ≠ current tier",
			contract("tier-3", stdLadder, stdHistory),
			OpBarePointerFlip, "tier-3",
		},
		{
			// zero history rows while current tier-2 is non-initial.
			"empty history under a non-initial tier",
			contract("tier-2", stdLadder, emptyHistory),
			OpBarePointerFlip, "tier-2",
		},
	}
	for _, tc := range cases {
		d := parseAndEval(t, tc.body)
		assertExactlyOne(t, tc.name, d, tc.op, tc.needle)
	}
}

// TestDINV4BornSeed is the born-seed genesis rule (design §3.8's born-contract
// definition): a non-empty change history must originate at the born seed
// (move: none at the initial tier). Each forgery that satisfies the
// latest-row==pointer rule yet fakes a legitimate origin — a `none` seed minted
// at a non-initial tier, a tier move as the genesis with no birth beneath it,
// and a second `none` re-birth — reports exactly one named born-seed defect; a
// legitimate multi-move history that begins at the born seed passes clean.
func TestDINV4BornSeed(t *testing.T) {
	h := func(rows ...string) string {
		return "| Date | Move | Tier | Evidence |\n|---|---|---|---|\n" + strings.Join(rows, "\n") + "\n"
	}
	born := "| 2026-07-11 | none | floor | born at floor — the introducing PR |"
	cases := []struct {
		name    string
		current string
		history string
		needle  string
	}{
		{
			// none@tier-4 with a matching tier-4 pointer: tier-in-ladder and the
			// bare-pointer-flip rule both pass, so ONLY born-seed fires.
			"fabricated `none` seed at a non-initial tier",
			"tier-4", h("| 2026-07-11 | none | tier-4 | fabricated |"),
			"fabricated birth",
		},
		{
			// a promotion as the sole (genesis) row — no born seed beneath it.
			"genesis is a tier move with no birth beneath it",
			"tier-2", h("| 2026-07-11 | promotion | tier-2 | no seed |"),
			"must begin with the born seed",
		},
		{
			// a real born seed, then a second `move: none` — a re-birth. Current
			// tier floor matches the latest row, so only born-seed fires.
			"a second `none` re-birth",
			"floor", h(born, "| 2026-07-12 | none | floor | re-birth |"),
			"re-birth",
		},
	}
	for _, tc := range cases {
		d := parseAndEval(t, contract(tc.current, stdLadder, tc.history))
		assertExactlyOne(t, tc.name, d, OpBornSeed, tc.needle)
	}

	// A legitimate multi-move history that begins at the born seed passes clean.
	legit := contract("tier-3", stdLadder, h(born,
		"| 2026-07-11 | promotion | tier-2 | operator-directed early promotion |",
		"| 2026-07-12 | promotion | tier-3 | operator promotion |"))
	if d := parseAndEval(t, legit); len(d) != 0 {
		t.Errorf("legit born-anchored multi-move history: want zero defects, got %d: %v", len(d), d)
	}
}

// TestDINV3 is A-2: a change-history row missing each required field, and an
// out-of-enum move, each report a named attribution defect on a non-latest row
// (so the DINV-4 latest-row check is not incidentally tripped); a complete
// contract passes.
func TestDINV3(t *testing.T) {
	// Row 1 is mutated; row 2 remains the valid tier-2 latest row.
	hist := func(row1 string) string {
		return "| Date | Move | Tier | Evidence |\n|---|---|---|---|\n" +
			row1 + "\n" +
			"| 2026-07-11 | promotion | tier-2 | operator promotion |\n"
	}
	cases := []struct {
		name   string
		row1   string
		needle string
	}{
		{"missing date", "|  | none | floor | born |", "date"},
		{"missing move", "| 2026-07-11 |  | floor | born |", "move"},
		{"missing resulting tier", "| 2026-07-11 | none |  | born |", "resulting tier"},
		{"missing evidence refs", "| 2026-07-11 | none | floor |  |", "evidence refs"},
		{"out-of-enum move", "| 2026-07-11 | sideways | floor | born |", "sideways"},
	}
	for _, tc := range cases {
		d := parseAndEval(t, contract("tier-2", stdLadder, hist(tc.row1)))
		assertExactlyOne(t, tc.name, d, OpAttribution, tc.needle)
	}
}

// TestDINV5 is A-3: a ladder bundle carrying a token that is not a defined
// action-class token reports a named token-membership defect; the pinned legal
// vocabulary passes (covered by TestLegalAndBornPass). The check is
// token-membership, not a blocklist — an arbitrary undefined token reds.
func TestDINV5(t *testing.T) {
	for _, tok := range []string{"self-promote", "amend-authority"} {
		ladder := strings.Replace(stdLadder, "sign-gate-verdict", tok, 1)
		d := parseAndEval(t, contract("tier-2", ladder, stdHistory))
		assertExactlyOne(t, "undefined token "+tok, d, OpTokenMembership, tok)
	}
}

// TestParseDefects is A-5: a malformed contract (undelimitable ladder or
// change-history region) and a wrong/absent template version each yield a
// named parse defect (never a crash, never a pass).
func TestParseDefects(t *testing.T) {
	legal := contract("tier-2", stdLadder, stdHistory)
	cases := []struct {
		name   string
		body   string
		op     string
		needle string
	}{
		{"undelimitable ladder region",
			strings.Replace(legal, "## The ladder", "## Something else", 1),
			"region", schema.AuthorityLadderRegion},
		{"undelimitable change-history region",
			strings.Replace(legal, "## Change history", "## Notes", 1),
			"region", schema.AuthorityHistoryRegion},
		{"ladder region with no tier rows",
			// keep the heading but strip its table body → a delimitable region
			// that carries no rows is still a parse defect.
			contract("tier-2", "*(no table here — just guidance prose)*\n", stdHistory),
			"region", schema.AuthorityLadderRegion},
		{"wrong template version",
			strings.Replace(legal, "authority-contract/v1", "authority-contract/v2", 1),
			"version", "authority-contract/v2"},
		{"absent template version",
			strings.Replace(legal, "| **Template version** | authority-contract/v1 |\n", "", 1),
			"version", "(absent)"},
	}
	for _, tc := range cases {
		d := parseAndEval(t, tc.body)
		if len(d) == 0 {
			t.Errorf("%s: want a named parse defect, got none (a false pass)", tc.name)
			continue
		}
		found := false
		for _, x := range d {
			if x.Op == tc.op && x.File == testFile && strings.Contains(x.Reason, tc.needle) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want a %s parse defect naming %q, got %v", tc.name, tc.op, tc.needle, d)
		}
	}
}

// TestEveryViolationReported proves the evaluation is not first-fail: a contract
// with a bad token AND a bad move AND a bare pointer flip names all three.
func TestEveryViolationReported(t *testing.T) {
	ladder := strings.Replace(stdLadder, "rule-acceptance", "self-promote", 1)
	hist := "| Date | Move | Tier | Evidence |\n|---|---|---|---|\n" +
		"| 2026-07-11 | sideways | floor | born |\n" +
		"| 2026-07-11 | promotion | tier-2 | ev |\n"
	// current tier-3 in the ladder, but the latest row lands at tier-2 → flip.
	d := parseAndEval(t, contract("tier-3", ladder, hist))
	ops := map[string]bool{}
	for _, x := range d {
		ops[x.Op] = true
	}
	for _, want := range []string{OpTokenMembership, OpAttribution, OpBarePointerFlip} {
		if !ops[want] {
			t.Errorf("want a %s defect among %v", want, d)
		}
	}
}

// assertExactlyOne asserts d has exactly one defect and it carries the expected
// Op, names the governed file, and mentions the needle — the "one named defect
// per violation" contract, isolated per fixture.
func assertExactlyOne(t *testing.T, name string, d []schema.Defect, op, needle string) {
	t.Helper()
	if len(d) != 1 {
		t.Errorf("%s: want exactly one defect, got %d: %v", name, len(d), d)
		return
	}
	got := d[0]
	if got.Op != op {
		t.Errorf("%s: Op = %q, want %q (%v)", name, got.Op, op, got)
	}
	if got.File != testFile {
		t.Errorf("%s: File = %q, want the governed file %q", name, got.File, testFile)
	}
	if !strings.Contains(got.Reason, needle) {
		t.Errorf("%s: Reason %q does not mention %q", name, got.Reason, needle)
	}
}
