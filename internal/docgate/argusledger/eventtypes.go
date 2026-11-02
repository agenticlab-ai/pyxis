package argusledger

// The closed delivery-ledger entry-type vocabulary and the EVT-4 materialization
// gist grammar (docs/tech-design/fleet-bridge-v1.md §3.6). The journal's entry
// types are a fixed set the drive writes and the derived-state derivations read;
// the bridge's materialization rows (the trigger, the per-item confirmations,
// the tag-discrepancy reconciliations) land on the *declared* types, carrying
// EVT-4's typed properties in the entry's **gist body in a typed grammar** —
// never an invented structured field on the frozen entry class (the artifacts
// design's first-class-field deferral). The grammar is closed (AP-6): an unknown
// key is a named defect, never a silent skip.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The closed delivery-ledger entry-type vocabulary (delivery-ledger/v1's
// journal grammar). One place for the set every reader shares (AP-11).
const (
	TypeStageEntered       = "stage-entered"
	TypeRulingRecorded     = "ruling-recorded"
	TypeAssumptionRecorded = "assumption-recorded"
	TypeItemMaterialized   = "item-materialized"
	TypeBriefEmitted       = "brief-emitted"
	TypeAnswerReceived     = "answer-received"
	TypeProposalDisposed   = "proposal-disposed"
	TypeReconciliation     = "reconciliation"
	TypeBumpIssued         = "bump-issued"
	TypeDelivered          = "delivered"
)

// entryTypeSet is the closed membership test for the vocabulary above.
var entryTypeSet = map[string]bool{
	TypeStageEntered: true, TypeRulingRecorded: true, TypeAssumptionRecorded: true,
	TypeItemMaterialized: true, TypeBriefEmitted: true, TypeAnswerReceived: true,
	TypeProposalDisposed: true, TypeReconciliation: true, TypeBumpIssued: true,
	TypeDelivered: true,
}

// KnownEntryType reports whether t is one of the closed delivery-ledger entry
// types — the fail-closed check a writer or lint uses before trusting a type.
func KnownEntryType(t string) bool { return entryTypeSet[t] }

// entryHeadingRe captures the type token of a journal entry from its heading
// `### E-n — <type> — <gist>` (the house `— ` separator). The gist may itself
// contain the separator; only the FIRST field after the anchor is the type.
var entryHeadingRe = regexp.MustCompile(`^###\s+E-\d+\s+—\s+([a-z][a-z-]*)\s+—`)

// EntryType extracts a journal entry block's type token from its heading line,
// or "" when the block does not open with a typed entry heading. It reads only
// the first line, so it is cheap over an entry block's bytes.
func EntryType(block []byte) string {
	line := block
	if i := strings.IndexByte(string(block), '\n'); i >= 0 {
		line = block[:i]
	}
	m := entryHeadingRe.FindStringSubmatch(string(line))
	if m == nil {
		return ""
	}
	return m[1]
}

// TriggerKind is the EVT-4 `trigger` property: an unattended wake-delivered
// materialization, or a directed human prompt — the one distinction between a
// pull and a push run (AP-2; the behavior is identical, only the record differs).
type TriggerKind string

const (
	TriggerWake   TriggerKind = "wake"
	TriggerPrompt TriggerKind = "prompt"
)

func knownTrigger(k TriggerKind) bool { return k == TriggerWake || k == TriggerPrompt }

// TagStatus is the EVT-4 tag-verification property: the automated-implementation
// tag was present at the goal object (verified) or absent/misapplied (failed —
// the reconciliation's cause, routed back through the stage).
type TagStatus string

const (
	TagVerified TagStatus = "verified"
	TagFailed   TagStatus = "failed"
	TagOmitted  TagStatus = "" // the trigger entry, recorded before verification
)

func knownTag(s TagStatus) bool { return s == TagVerified || s == TagFailed || s == TagOmitted }

// MaterializationGist is EVT-4's typed grammar riding the gist body of a
// materialization journal entry (the trigger's `ruling-recorded`, an item
// confirmation's `item-materialized`, a discrepancy's `reconciliation`). It
// carries the plan, the item ids, the tag status, and the trigger — the
// properties the basis's EVT-4 row contracts — as a closed `key=value; …` line,
// so no frozen entry class gains an invented structured field (§3.6). Plan and
// trigger are required; items and tag are present where the entry kind carries
// them.
type MaterializationGist struct {
	Plan    string
	Items   []string
	Tag     TagStatus
	Trigger TriggerKind
}

// Render serializes the gist to its closed `key=value; …` form, fields in a
// fixed order (plan, items, tag, trigger) so the same gist is byte-identical.
// Items are joined with commas (a comma is not a legal id char here); an empty
// items list and an omitted tag are simply absent, so a trigger entry and a
// per-item confirmation render their own shapes without optional-field noise.
func (g MaterializationGist) Render() string {
	parts := []string{"plan=" + g.Plan}
	if len(g.Items) > 0 {
		parts = append(parts, "items="+strings.Join(g.Items, ","))
	}
	if g.Tag != TagOmitted {
		parts = append(parts, "tag="+string(g.Tag))
	}
	parts = append(parts, "trigger="+string(g.Trigger))
	return strings.Join(parts, "; ")
}

// ParseMaterializationGist parses the closed gist grammar, fail-closed: an
// unknown key, a missing required field (plan, trigger), or an invalid tag or
// trigger value is a named defect, never a silently dropped property. It is the
// inverse of Render on a well-formed gist and the lint hook a malformed
// materialization row is caught on rather than skipped.
func ParseMaterializationGist(verb, filePath, anchor, gist string) (MaterializationGist, []schema.Defect) {
	var g MaterializationGist
	seen := map[string]bool{}
	bad := func(reason string) []schema.Defect {
		return []schema.Defect{def(verb, filePath, "materialization-gist",
			fmt.Sprintf("%s in %s: %q", reason, anchor, strings.TrimSpace(gist)),
			"the EVT-4 gist reads `plan=<id>; items=<a,b>; tag=verified|failed; trigger=wake|prompt` — the grammar is closed (fleet-bridge-v1 §3.6)")}
	}
	for _, seg := range strings.Split(gist, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		k, v, ok := strings.Cut(seg, "=")
		if !ok {
			return MaterializationGist{}, bad("malformed segment (want key=value)")
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if seen[k] {
			return MaterializationGist{}, bad("duplicate key " + quote(k))
		}
		seen[k] = true
		switch k {
		case "plan":
			g.Plan = v
		case "items":
			for _, id := range strings.Split(v, ",") {
				if id = strings.TrimSpace(id); id != "" {
					g.Items = append(g.Items, id)
				}
			}
			sort.Strings(g.Items)
		case "tag":
			g.Tag = TagStatus(v)
			if !knownTag(g.Tag) || g.Tag == TagOmitted {
				return MaterializationGist{}, bad("unknown tag status " + quote(v))
			}
		case "trigger":
			g.Trigger = TriggerKind(v)
			if !knownTrigger(g.Trigger) {
				return MaterializationGist{}, bad("unknown trigger " + quote(v))
			}
		default:
			return MaterializationGist{}, bad("unknown key " + quote(k))
		}
	}
	if g.Plan == "" {
		return MaterializationGist{}, bad("missing required key 'plan'")
	}
	if g.Trigger == "" {
		return MaterializationGist{}, bad("missing required key 'trigger'")
	}
	return g, nil
}

// quote wraps a value for a readable, unambiguous reason.
func quote(s string) string { return "'" + s + "'" }
