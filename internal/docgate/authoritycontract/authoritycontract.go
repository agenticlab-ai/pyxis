// Package authoritycontract is the sibling parse of record for the Argus
// authority contract (docs/tech-design/argus-authority-contract-structure-arm.md
// §3.1). The authority contract's shape is unlike the A-n/E-n ledgers — it has
// no dense entry anchors and no derived-state region — so it gets its OWN parse
// keyed on its two `## `-delimited tables (the ladder and the change history)
// and its `Current tier` header pointer, never an extension of the ledger
// parse (design §3.1, §4). One parse turns the file into the parsed contract
// the DINV-3/4/5 evaluation reads (evaluate.go).
//
// The parse is closed against the version-pinned registry in
// internal/docgate/schema: a contract declaring an unknown template version is
// rejected rather than parsed leniently, and a contract whose ladder or
// change-history region cannot be delimited yields a named parse defect
// identifying the region — never a crash, never a false pass (UB-3, DINV-2).
// A change-history region whose heading is present but carries zero data rows
// is NOT a parse defect: it parses to an empty history-row list the DINV-4
// evaluation then reds under a non-initial current tier (§3.1, §3.2).
package authoritycontract

import (
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// LadderRow is one ladder tier and the action-class tokens its bundle grants.
// Tokens is empty for the `revoked` kill-switch tier (the ∅ bundle).
type LadderRow struct {
	Tier   string
	Tokens []string
}

// HistoryRow is one change-history row's four fields, positionally parsed.
// A field is "" when its cell is empty or absent — the DINV-3 evaluation names
// the missing field rather than the parse failing the whole file.
type HistoryRow struct {
	Line     int // 1-based source line, for naming the row in a defect
	Date     string
	Move     string
	Tier     string
	Evidence string
}

// Contract is one parsed authority contract: the current-tier pointer, the
// ladder rows, and the change-history rows, against which the DINV-3/4/5
// evaluation runs.
type Contract struct {
	Path        string
	Class       schema.AuthorityContractClass
	CurrentTier string
	Ladder      []LadderRow
	History     []HistoryRow
}

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Parse reads the authority contract against the pinned authority-contract/v1
// registry, emitting named parse defects (never a panic) when the version is
// wrong/absent or a required region cannot be delimited. It returns
// (contract, nil) on a structurally-parseable file — legality is then the
// evaluation's job (Evaluate).
func Parse(verb, filePath string, b []byte) (*Contract, []schema.Defect) {
	class := schema.AuthorityContractV1
	lines := strings.Split(strings.TrimSuffix(string(schema.Normalize(b)), "\n"), "\n")

	// Version pin: reject a contract whose declared template version is not the
	// one this closed parse is pinned to, rather than guessing its shape.
	declared := ""
	for _, l := range lines {
		if m := tmplVersionRe.FindStringSubmatch(l); m != nil {
			declared = strings.TrimSpace(m[1])
			break
		}
	}
	if !strings.Contains(declared, class.TemplateVersion) {
		got := declared
		if got == "" {
			got = "(absent)"
		}
		return nil, []schema.Defect{def(verb, filePath, "version",
			"declared template version "+got+" is not the pinned "+class.TemplateVersion,
			"instantiate from .pyxis/templates/authority-contract-template.md; the structure arm parses only the pinned version")}
	}

	// Current-tier pointer (a header-table field).
	currentTier := ""
	for _, l := range lines {
		if m := schema.AuthorityCurrentTierRe.FindStringSubmatch(l); m != nil {
			currentTier = stripToken(m[1])
			break
		}
	}

	// Regions, delimited by `## ` heading — the same section delimiter the
	// ledger parse uses (LedgerRegionRe is class-independent). Each region body
	// runs from the line after its heading to the next `## ` heading or EOF.
	regions := delimitRegions(lines)

	var defects []schema.Defect

	ladderBody, ok := regions[schema.AuthorityLadderRegion]
	if !ok {
		defects = append(defects, def(verb, filePath, "region",
			"the \""+schema.AuthorityLadderRegion+"\" region could not be delimited (no \"## "+schema.AuthorityLadderRegion+"\" heading)",
			"an authority contract must carry its \"## "+schema.AuthorityLadderRegion+"\" table"))
	}
	historyBody, ok := regions[schema.AuthorityHistoryRegion]
	if !ok {
		defects = append(defects, def(verb, filePath, "region",
			"the \""+schema.AuthorityHistoryRegion+"\" region could not be delimited (no \"## "+schema.AuthorityHistoryRegion+"\" heading)",
			"an authority contract must carry its \"## "+schema.AuthorityHistoryRegion+"\" table"))
	}
	if len(defects) > 0 {
		return nil, defects
	}

	ladder, lerr := parseLadder(verb, filePath, ladderBody)
	if lerr != nil {
		return nil, []schema.Defect{*lerr}
	}
	history := parseHistory(historyBody)

	return &Contract{
		Path:        filePath,
		Class:       class,
		CurrentTier: currentTier,
		Ladder:      ladder,
		History:     history,
	}, nil
}

// parseLadder extracts each ladder tier and its action-class token bundle. A
// ladder region carrying no data rows at all cannot be delimited into a table
// — a named parse defect, never a silent empty ladder that would false-pass
// token-membership (§3.1).
func parseLadder(verb, filePath string, body region) ([]LadderRow, *schema.Defect) {
	var rows []LadderRow
	for _, cells := range dataRows(body) {
		tier := stripToken(cell(cells, 0))
		if tier == "" {
			continue
		}
		rows = append(rows, LadderRow{Tier: tier, Tokens: bundleTokens(cell(cells, 1))})
	}
	if len(rows) == 0 {
		d := def(verb, filePath, "region",
			"the \""+schema.AuthorityLadderRegion+"\" table carries no tier rows",
			"an authority contract's ladder must list its ordered tiers and their action-class bundles")
		return nil, &d
	}
	return rows, nil
}

// parseHistory extracts the change-history rows positionally. Zero data rows is
// a legal parse (an empty history-row list); the DINV-4 evaluation reds it
// under a non-initial current tier (§3.2). A short row's missing cells are ""
// so the DINV-3 evaluation can name the absent field (§3.1).
func parseHistory(body region) []HistoryRow {
	var rows []HistoryRow
	for _, dr := range dataRowsWithLine(body) {
		rows = append(rows, HistoryRow{
			Line:     dr.line,
			Date:     cell(dr.cells, 0),
			Move:     stripToken(cell(dr.cells, 1)),
			Tier:     stripToken(cell(dr.cells, 2)),
			Evidence: cell(dr.cells, 3),
		})
	}
	return rows
}
