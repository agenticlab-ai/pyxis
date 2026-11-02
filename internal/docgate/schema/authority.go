package schema

import "regexp"

// The version-pinned closed vocabularies the Argus authority-contract structure
// arm parses against (docs/tech-design/argus-authority-contract-structure-arm.md
// §3.1). The authority contract carries no A-n/E-n anchors, so it is NOT a
// LedgerClass — its shape is two `## `-delimited tables (the ladder and the
// change history) plus a `Current tier` header pointer, and its class rules
// live here as their own closed registry, distinct from the ledger classes'
// anchor-family registry (design §3.4; docgate-v2 "class rules as registries in
// schema"). Unknown anything is a defect, never a lenient parse (DINV-2).

// AuthorityContractVersion is the pinned template version the closed parse
// accepts; any other declared version is a named parse defect.
const AuthorityContractVersion = "authority-contract/v1"

// AuthorityContractPath is the fixed repo-relative location of the Argus
// authority contract (the agent home) — the single file the structure arm
// governs. A tree without this file has nothing for the arm to govern.
const AuthorityContractPath = ".pyxis/skills/argus/authority-contract.md"

// EmptyBundleMarker denotes an empty action-class bundle in a ladder row (the
// `revoked` kill-switch tier): a bundle cell beginning with it grants no
// tokens, so its prose ("every act escalates …") is never token-scanned.
const EmptyBundleMarker = "∅"

// AuthorityContractClass is the closed, version-pinned parse shape of the Argus
// authority contract: the tier vocabulary and its born initial tier, the
// action-class token vocabulary a grant check reads, and the change-history
// move enum — the exact vocabularies argus-artifacts §3.3 fixes. Adding a tier,
// a token, or a move is a successor-design change to this registry, never a
// silent parse-time addition.
type AuthorityContractClass struct {
	TemplateVersion string          // the pinned version the contract declares
	InitialTier     string          // the born tier — the one legitimate zero-move state
	TierOrder       []string        // the ordered tier names, for reference
	ActionClasses   map[string]bool // the closed action-class token vocabulary (DINV-5)
	Moves           map[string]bool // the closed change-history move enum (DINV-3)
}

// AuthorityContractV1 is the pinned authority-contract/v1 vocabulary
// (argus-artifacts §3.3; .pyxis/templates/authority-contract-template.md).
var AuthorityContractV1 = AuthorityContractClass{
	TemplateVersion: AuthorityContractVersion,
	InitialTier:     "floor",
	TierOrder:       []string{"revoked", "floor", "tier-2", "tier-3", "tier-4"},
	ActionClasses: boolSet(
		"dispose-advisory-finding",
		"dispose-blocking-finding",
		"sign-gate-verdict",
		"trigger-materialization",
		"rule-acceptance",
		"approve-merge",
	),
	Moves: boolSet("promotion", "demotion", "revocation", "none"),
}

// AuthorityCurrentTierRe matches the `| **Current tier** | <value> |` header
// row, capturing the pointer's raw value (backtick/angle-bracket wrapping is
// stripped by the parse, not here).
var AuthorityCurrentTierRe = regexp.MustCompile(`^\|\s*\*\*Current tier\*\*\s*\|\s*(.+?)\s*\|`)

// Authority-contract region headings (delimited, like every `## ` region, by
// LedgerRegionRe — reused: the section-heading delimiter is class-independent).
const (
	AuthorityLadderRegion  = "The ladder"
	AuthorityHistoryRegion = "Change history"
)

func boolSet(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it] = true
	}
	return m
}
