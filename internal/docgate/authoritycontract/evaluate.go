package authoritycontract

import (
	"fmt"
	"strconv"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Op discriminators (schema.Defect.Op) — the invariant class a defect belongs
// to, so a version/region parse defect, a DINV-3, a DINV-4, and a DINV-5
// finding are never conflated (design §3.6).
const (
	OpTierInLadder    = "tier-in-ladder"    // DINV-4 — current tier absent from the ladder
	OpBarePointerFlip = "bare-pointer-flip" // DINV-4 — empty/mismatched change history
	OpBornSeed        = "born-seed"         // DINV-4 — history not anchored to a born seed at the initial tier
	OpAttribution     = "attribution"       // DINV-3 — a change-history row missing a field / bad move
	OpTokenMembership = "token-membership"  // DINV-5 — an undefined action-class token
)

// Evaluate runs the DINV-3/4/5 structural evaluation over a parsed contract and
// returns one named defect per violation (never first-fail, so a contract with
// several defects names them all — design §3.2). A legal contract, and the
// born contract (a single `move: none` seed row at the initial tier), raise
// zero defects. A non-empty history must originate at that born seed: a
// fabricated birth minted at a non-initial tier, or a history whose oldest row
// is a tier move with no birth beneath it, satisfies the latest-row==pointer
// rule yet fakes a legitimate origin — the DINV-4 born-seed rule reds it
// (design §3.8's born-contract definition). The evaluation is pure structure —
// table position and closed-vocabulary membership — and never interprets prose
// (DINV-1).
func Evaluate(verb, filePath string, c *Contract) []schema.Defect {
	var defects []schema.Defect

	// DINV-4 — tier-in-ladder: the current-tier pointer must name a tier the
	// ladder declares.
	ladderTiers := map[string]bool{}
	for _, r := range c.Ladder {
		ladderTiers[r.Tier] = true
	}
	if !ladderTiers[c.CurrentTier] {
		cur := c.CurrentTier
		if cur == "" {
			cur = "(absent)"
		}
		defects = append(defects, def(verb, filePath, OpTierInLadder,
			"Current tier "+cur+" is not a tier declared in the ladder",
			"set Current tier to a tier the ladder lists, or add the tier to the ladder by a successor-design change"))
	}

	// DINV-4 — no bare pointer flip (history mandatory). An empty/heading-only
	// change history under a NON-initial current tier is itself a bare pointer
	// flip (the canonical "pointer flip with no history entry"); the one exempt
	// zero-move state is the born contract at the initial tier. Otherwise the
	// latest history row's resulting tier must equal the current tier.
	if len(c.History) == 0 {
		if c.CurrentTier != c.Class.InitialTier {
			defects = append(defects, def(verb, filePath, OpBarePointerFlip,
				"the change history is empty or heading-only while Current tier "+c.CurrentTier+" is not the born initial tier "+c.Class.InitialTier+" — a bare pointer flip with no history entry",
				"land the tier move with its change-history row, or restore the wiped history; only the born contract's seed row at "+c.Class.InitialTier+" is exempt"))
		}
	} else {
		latest := c.History[len(c.History)-1]
		if latest.Tier != c.CurrentTier {
			lt := latest.Tier
			if lt == "" {
				lt = "(absent)"
			}
			defects = append(defects, def(verb, filePath, OpBarePointerFlip,
				fmt.Sprintf("the latest change-history row (line %d) resolves to tier %s but Current tier is %s — a bare pointer flip", latest.Line, lt, c.CurrentTier),
				"append the change-history row for the current tier; a pointer move is never bare of its history entry"))
		}
	}

	// DINV-4 — born-seed genesis. A non-empty change history must originate at
	// the born contract's seed: its OLDEST row is a birth (`move: none`)
	// resolving to the initial tier (design §3.8 — "the born contract, whose
	// single seed row carries `move: none` and the initial tier"). This closes
	// two forgeries the latest-row==pointer rule above admits — both fake a
	// legitimate origin: a `move: none` seed minted at a non-initial tier, and a
	// history whose oldest row is a tier move with no birth beneath it. A second
	// `move: none` anywhere but the genesis is a re-birth. Each branch is gated
	// on the field it reads being present (and, for the non-birth case, a
	// defined move) so an empty/out-of-enum move or an absent tier stays DINV-3's
	// to report — the born-seed rule never double-names a field DINV-3 owns.
	if len(c.History) > 0 {
		genesis := c.History[0]
		switch {
		case genesis.Move == "none" && genesis.Tier != "" && genesis.Tier != c.Class.InitialTier:
			defects = append(defects, def(verb, filePath, OpBornSeed,
				fmt.Sprintf("the oldest change-history row (line %d) is a born seed (move: none) resolving to tier %s, but a birth can only resolve to the initial tier %s — a fabricated birth dressed as a born contract", genesis.Line, genesis.Tier, c.Class.InitialTier),
				"a `move: none` seed is legal only at the initial tier "+c.Class.InitialTier+"; reach a higher tier by landing a promotion row over a real born seed, never by minting the birth at that tier"))
		case genesis.Move != "" && genesis.Move != "none" && c.Class.Moves[genesis.Move]:
			defects = append(defects, def(verb, filePath, OpBornSeed,
				fmt.Sprintf("the oldest change-history row (line %d) records move %q, but the change history must begin with the born seed (move: none at the initial tier %s) — a tier move cannot be the first recorded state", genesis.Line, genesis.Move, c.Class.InitialTier),
				"begin the change history with the born seed row (move: none, tier "+c.Class.InitialTier+") beneath the tier moves"))
		}
		for _, r := range c.History[1:] {
			if r.Move == "none" {
				defects = append(defects, def(verb, filePath, OpBornSeed,
					fmt.Sprintf("change-history row %s (line %d) records move: none, but only the genesis row is a birth — a second born seed is a re-birth", rowLabel(r), r.Line),
					"a tier returning to the initial tier is a demotion or revocation move, never another `move: none` birth"))
			}
		}
	}

	// DINV-3 — attribution present: every change-history row carries all four
	// fields, and its move is in the closed enum.
	for _, r := range c.History {
		for _, f := range missingHistoryFields(r) {
			defects = append(defects, def(verb, filePath, OpAttribution,
				fmt.Sprintf("change-history row %s (line %d) is missing its %s field", rowLabel(r), r.Line, f),
				"every change-history row carries date · move · resulting tier · evidence refs"))
		}
		if r.Move != "" && !c.Class.Moves[r.Move] {
			defects = append(defects, def(verb, filePath, OpAttribution,
				fmt.Sprintf("change-history row %s (line %d) has move %q, which is not in the closed move enum", rowLabel(r), r.Line, r.Move),
				"move must be one of: promotion, demotion, revocation, none"))
		}
	}

	// DINV-5 — token-membership: every ladder bundle token is a defined
	// action-class token. This IS the structural "no never-class act can be
	// granted" — a never-class act is absent from the vocabulary by
	// construction; there is no separate blocklist scan (design §3.2).
	for _, r := range c.Ladder {
		for _, tok := range r.Tokens {
			if !c.Class.ActionClasses[tok] {
				defects = append(defects, def(verb, filePath, OpTokenMembership,
					fmt.Sprintf("ladder tier %s grants token %q, which is not a defined action-class token", r.Tier, tok),
					"grant only defined action-class tokens; a new token is a successor-design change to the vocabulary, never a quiet bundle edit"))
			}
		}
	}

	return defects
}

// missingHistoryFields returns the names of the required fields a change-history
// row leaves empty — one DINV-3 defect per missing field, so a row missing
// several names each.
func missingHistoryFields(r HistoryRow) []string {
	var missing []string
	if r.Date == "" {
		missing = append(missing, "date")
	}
	if r.Move == "" {
		missing = append(missing, "move")
	}
	if r.Tier == "" {
		missing = append(missing, "resulting tier")
	}
	if r.Evidence == "" {
		missing = append(missing, "evidence refs")
	}
	return missing
}

// rowLabel names a change-history row for a defect: its date when present, else
// its ordinal source line — enough to point a reviewer at the exact row.
func rowLabel(r HistoryRow) string {
	if r.Date != "" {
		return r.Date
	}
	return "at line " + strconv.Itoa(r.Line)
}
