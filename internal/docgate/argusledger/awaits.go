package argusledger

// The await grammar — the closed typed vocabulary a stopped drive registers in
// a delivery ledger and a resumed session re-derives from the journal alone
// (docs/tech-design/wake-channel-v1.md §3.1, DINV-1; prd:argus-unattended-drive
// FR-13). When a drive stops on an awaited external completion the journal's
// stop entry names the awaited goal object and the completion condition; the
// journal is the sole authority (the artifacts design's DINV-6 — the derived
// block is a regenerable projection, never trusted). The derived block surfaces
// one line per outstanding await inside its existing Open-items element, riding
// the delivery-ledger class **additively** — no template version bump (the
// first-class Awaits row is deferred to a future bump under argus-artifacts
// §3.8's successor rule; wake-channel-v1 §7 Q2).
//
// The grammar is closed (AP-6): a line whose bold label is `Await` or
// `Await-cleared` but whose shape does not match yields a named grammar defect,
// never a silent skip — the tick that consumes awaits (T8) must never misread
// or miss one (wake-channel-v1 R1).

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Await is one outstanding awaited external completion, re-derived from the
// journal. GoalObject is the identity key (the awaited artifact — a PR to
// merge, an item to land, an answer to append); Condition is the completion
// condition the resumed touch verifies at that object; Since is the
// registration date the staleness computation (T7) reads.
type Await struct {
	GoalObject string
	Condition  string
	Since      string // YYYY-MM-DD
}

// The closed await grammar. A register line names the awaited goal object, its
// completion condition, and the since-date, separated by ` · ` (the house
// middot); a field therefore may not itself contain the separator — the parse
// is closed, not lenient. A clear line names only the goal object it clears.
var (
	// awaitLabelRe recognizes an await line by its bold label alone — the hook a
	// malformed line is caught on rather than skipped. `Await-cleared` is listed
	// first so the alternation never mis-captures it as a bare `Await`.
	awaitLabelRe = regexp.MustCompile(`^-\s+\*\*(Await-cleared|Await)\*\*\s*:`)
	// awaitRegisterRe is the full closed register grammar.
	awaitRegisterRe = regexp.MustCompile(`^-\s+\*\*Await\*\*:\s+(\S.*?)\s+·\s+(\S.*?)\s+·\s+since\s+(\d{4}-\d{2}-\d{2})\s*$`)
	// awaitClearRe is the full closed clear grammar.
	awaitClearRe = regexp.MustCompile(`^-\s+\*\*Await-cleared\*\*:\s+(\S.*?)\s*$`)
)

// openItemsRowRe splits a Derived-state `| **Open items** | <value> |` row into
// its label prefix, the value cell, and the trailing pipe — so the surfacing
// rewrites only the value and leaves every other byte of the row untouched.
var openItemsRowRe = regexp.MustCompile(`^(\|\s*\*\*Open items\*\*\s*\|)(.*)(\|\s*)$`)

// awaitSegmentRe matches the sentinel-delimited await region the surfacing owns
// inside the Open-items cell, including its leading whitespace — so stripping it
// restores the cell's other content exactly.
var awaitSegmentRe = regexp.MustCompile(`\s*<!--awaits-->.*?<!--/awaits-->`)

// DeriveAwaits re-derives the outstanding await set of a delivery ledger from
// its journal alone (never the derived block — the journal is the authority,
// DINV-6). It walks the `E-n` entries in document order: a register line adds
// or refreshes an await keyed by its goal object, a clear line removes that
// object's await. The result is the still-outstanding set in registration
// order, plus a named grammar defect for every malformed await line (the closed
// grammar refuses a silent skip). It is meaningful only on a delivery ledger;
// on any other class it returns (nil, nil).
func DeriveAwaits(verb string, l *Ledger) ([]Await, []schema.Defect) {
	if l == nil || l.Class.DerivedRegion == "" {
		return nil, nil
	}
	reg := map[string]Await{}
	var order []string // goal objects in first-registration order
	seen := map[string]bool{}
	var defects []schema.Defect

	for _, id := range l.Order {
		for _, line := range strings.Split(string(l.Entries[id]), "\n") {
			label := awaitLabelRe.FindStringSubmatch(line)
			if label == nil {
				continue
			}
			switch label[1] {
			case "Await":
				m := awaitRegisterRe.FindStringSubmatch(line)
				if m == nil {
					defects = append(defects, awaitGrammarDefect(verb, l.Path, id, line))
					continue
				}
				goal := strings.TrimSpace(m[1])
				a := Await{GoalObject: goal, Condition: strings.TrimSpace(m[2]), Since: m[3]}
				if !seen[goal] {
					seen[goal] = true
					order = append(order, goal)
				}
				reg[goal] = a
			case "Await-cleared":
				m := awaitClearRe.FindStringSubmatch(line)
				if m == nil {
					defects = append(defects, awaitGrammarDefect(verb, l.Path, id, line))
					continue
				}
				delete(reg, strings.TrimSpace(m[1]))
			}
		}
	}

	var out []Await
	for _, goal := range order {
		if a, ok := reg[goal]; ok {
			out = append(out, a)
		}
	}
	return out, defects
}

// RegenerateOpenItems returns the delivery ledger's `## Derived state` region
// bytes with the Open-items cell's await region regenerated from awaits, every
// other byte of the region byte-identical. It surfaces one typed entry per
// outstanding await inside a sentinel-delimited span the surfacing owns, so a
// cleared await simply leaves the block and the element's other content — and
// every other derived-state field — is untouched (FR-13; the acceptance's
// byte-unchanged bar). The operation is idempotent. A delivery ledger whose
// derived block carries no Open-items row yields a named defect: the element the
// awaits ride must exist.
func RegenerateOpenItems(verb string, l *Ledger, awaits []Await) ([]byte, []schema.Defect) {
	if l == nil || l.Derived == nil {
		return nil, []schema.Defect{def(verb, ledgerPath(l), "derived-awaits",
			"no derived-state region to surface awaits in",
			"parse a delivery ledger with a delimited \"## Derived state\" region first")}
	}
	lines := strings.Split(strings.TrimSuffix(string(l.Derived.Bytes), "\n"), "\n")
	row := -1
	for i, line := range lines {
		if openItemsRowRe.MatchString(line) {
			row = i
			break
		}
	}
	if row == -1 {
		return nil, []schema.Defect{def(verb, l.Path, "derived-awaits",
			"the Derived state region has no \"| **Open items** |\" row to surface awaits in",
			"a delivery-ledger/v1 derived block must carry the Open items field (argus-artifacts §3.2)")}
	}
	// Rebuild the cell canonically: its non-await content (whatever prose the
	// drive wrote) trimmed, then the regenerated await span, wrapped in single
	// spaces — so regeneration is idempotent and a clear touches only the span.
	m := openItemsRowRe.FindStringSubmatch(lines[row])
	content := strings.TrimSpace(awaitSegmentRe.ReplaceAllString(m[2], ""))
	var parts []string
	if content != "" {
		parts = append(parts, content)
	}
	if seg := renderAwaitSegment(awaits); seg != "" {
		parts = append(parts, seg)
	}
	lines[row] = m[1] + " " + strings.Join(parts, " ") + " " + m[3]
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

// renderAwaitSegment renders the outstanding awaits as the sentinel-delimited
// span the Open-items cell carries — one closed, self-delimited entry per await.
// An empty set renders to nothing so a fully-cleared cell is byte-restored.
func renderAwaitSegment(awaits []Await) string {
	if len(awaits) == 0 {
		return ""
	}
	entries := make([]string, len(awaits))
	for i, a := range awaits {
		entries[i] = fmt.Sprintf("await(%s; %s; since %s)", a.GoalObject, a.Condition, a.Since)
	}
	return "<!--awaits-->" + strings.Join(entries, " ") + "<!--/awaits-->"
}

func awaitGrammarDefect(verb, filePath, anchor, line string) schema.Defect {
	return def(verb, filePath, "await-grammar",
		fmt.Sprintf("malformed await line in %s: %q", anchor, strings.TrimSpace(line)),
		"a register line reads `- **Await**: <goal object> · <condition> · since <YYYY-MM-DD>`; "+
			"a clear line reads `- **Await-cleared**: <goal object>` — the grammar is closed")
}

// ledgerPath names the defect's file: the ledger's own resolved path when one
// is present. A nil ledger has no known path (the parse never bound one), so it
// falls back to a generic descriptor — not a target-facing doc-path literal, so
// nothing here presumes the configured problems root (config-driven-portability
// A-2/DINV-1).
func ledgerPath(l *Ledger) string {
	if l == nil {
		return "(unparsed delivery ledger)"
	}
	return l.Path
}
