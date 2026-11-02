// Package wake is the wake channel's Go home (docs/tech-design/wake-channel-v1.md):
// the bounded backstop tick that guarantees a stopped drive resumes, the
// normalized instruction-free signal that optimizes it, and the front-door
// delivery that converges the two. The await grammar and the staleness
// computation the tick reads live in internal/docgate/argusledger (T6, T7); this
// package consumes them and adds the delivery machinery outside the ledger's
// parse (wake-channel-v1 §3.9's seam split).
//
// Everything here is hermetic and port-driven: the platform reads (goal-object
// probes) and the CLI invocation (the front door) are consumer-side interfaces
// the host adapter fills and the tests substitute, so the tick's logic — read
// awaits, exclude parks and closed ledgers, probe, converge, invoke, exit — is
// proven without a repo, a network, or a clock (the repo Go rules; §3.6's
// "no new credential, egress destination, or data class").
package wake

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Reason is the closed touch-reason vocabulary a wake signal carries — WHAT
// completed, never a free-form field (DINV-1; AP-2/AP-6). A signal is
// instruction-free by construction: it names a problem slug and one of these
// tokens, nothing else, so no byte of the wake surface can carry a directive.
// The resumed drive re-verifies the completion at the goal object regardless of
// the reason, so the reason is a hint and an audit key, never an authority
// (§3.6 — the channel grants nothing the resume protocol does not independently
// verify).
type Reason string

const (
	// ReasonMerge — an awaited proposal (goal object `pr:<ref>`) merged; the
	// completion is verified by mainline ancestry (AP-26).
	ReasonMerge Reason = "merge"
	// ReasonItem — an awaited tracker item (goal object `item:<id>`) landed; the
	// completion is verified by record existence (AP-18).
	ReasonItem Reason = "item"
	// ReasonAnswer — a park's answer appended; delivered by a push adapter that
	// observes the append, never by the backstop (DINV-2). Verified by record
	// existence of the answer entry.
	ReasonAnswer Reason = "answer"
)

// knownReason is the closed membership test the argus wake stack's signal
// validates against — argusGrammar's own vocabulary (below), kept under its
// existing name since call sites already spell it this way.
func knownReason(r Reason) bool {
	return argusGrammar.Known(r)
}

// Source is the delivery path an effective touch arrived by — the EVT-3
// `delivered-by` value (§3.5). It is delivery metadata recorded on the resumed
// drive's journal, NOT part of the injectable signal surface: it is set by the
// path that delivered the touch, never read off the wire, so it can carry no
// injected content.
type Source string

const (
	// SourceSignal — a push adapter observed the completion and emitted the
	// normalized signal (the latency optimization, never the guarantee).
	SourceSignal Source = "signal"
	// SourceBackstop — the bounded backstop tick found the completion by an exact
	// probe (the guarantee).
	SourceBackstop Source = "backstop"
	// SourceManual — a human front-door touch (the stated sequential fallback on
	// a bare host — DINV-7).
	SourceManual Source = "manual"
)

// Signal is the whole normalized wake payload: a problem slug and a touch
// reason, nothing else (DINV-1). Every field is a closed, validated token, so
// the struct is the injection boundary — a poisoned wake surface reduces to a
// Signal or to nothing, and there is no field for a directive to ride.
type Signal struct {
	Slug   string
	Reason Reason
}

// slugRe is the closed slug charset: a problem slug is kebab-case (the
// docs/problems/<slug>/ directory name). It admits no space, slash, newline, or
// markup, so a poisoned "slug" carrying a directive fails the parse rather than
// reaching the front door — the structural half of DINV-1's instruction-free
// contract. Every Grammar shares this one charset — only the reason vocabulary
// varies by consumer (below).
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Grammar is the closed-schema wake-signal grammar, parametrized by its closed
// reason vocabulary — the one implementation every wake consumer's signal is
// built from, so a second consumer with its own reasons (a different wake
// stack, a different transition vocabulary) shares this package's parsing and
// validation instead of hand-rolling a twin (self-drive-wake/001 T6; design-
// brainstorm:self-drive-wake-tech-design#D-1). argusGrammar below is this type
// pinned to the argus wake stack's own three reasons — the package-level
// Signal/NewSignal/ParseWire/Wire surface is argusGrammar's behavior by
// construction, so parametrizing this type changes zero argus-observed byte.
type Grammar struct {
	reasons map[Reason]bool
	wire    *regexp.Regexp
}

// NewGrammar builds a closed-schema grammar over the given reason vocabulary.
// A caller-supplied empty set is legal but validates nothing — never a silent
// wildcard that would accept any reason.
func NewGrammar(reasons ...Reason) Grammar {
	known := make(map[Reason]bool, len(reasons))
	alts := make([]string, 0, len(reasons))
	for _, r := range reasons {
		known[r] = true
		alts = append(alts, regexp.QuoteMeta(string(r)))
	}
	sort.Strings(alts) // a deterministic pattern regardless of argument order
	g := Grammar{reasons: known}
	if len(alts) > 0 {
		g.wire = regexp.MustCompile(`^wake:([a-z0-9][a-z0-9-]*):(` + strings.Join(alts, "|") + `)(\b|$)`)
	}
	return g
}

// Known reports whether reason belongs to this grammar's closed vocabulary.
func (g Grammar) Known(r Reason) bool { return g.reasons[r] }

// New builds a validated signal against this grammar, fail-closed: an
// ill-formed identifier or a reason outside the closed vocabulary yields
// ok=false and the zero Signal, never a coerced value. It is the one
// constructor every delivery path funnels through, so no path can mint a
// signal the wire grammar would refuse.
func (g Grammar) New(id string, reason Reason) (Signal, bool) {
	if !slugRe.MatchString(id) || !g.Known(reason) {
		return Signal{}, false
	}
	return Signal{Slug: id, Reason: reason}, true
}

// ParseWire extracts a signal from ANY raw wake-surface bytes under this
// grammar's reason vocabulary, reducing it to identifier + reason and
// discarding everything else (DINV-1). A surface that does not begin with a
// well-formed `wake:<id>:<reason>` token yields ok=false — no touch, never a
// guess. This is the injection boundary in force: whatever a push adapter, a
// poisoned copy, or a forged sender writes after the token is data the parser
// never reads, so the worst a poisoned signal costs is one bounded touch on
// the clean identifier it names (or nothing). An empty-vocabulary grammar
// parses nothing (no wire pattern was ever compiled for it).
func (g Grammar) ParseWire(raw string) (Signal, bool) {
	if g.wire == nil {
		return Signal{}, false
	}
	m := g.wire.FindStringSubmatch(raw)
	if m == nil {
		return Signal{}, false
	}
	return Signal{Slug: m[1], Reason: Reason(m[2])}, true
}

// ParseClosed parses a signal from a closed JSON object — exactly the two keys
// "id" and "reason", nothing else (FR-10's structured counterpart to ParseWire:
// "the wake channel shall accept no inbound content ... extra fields are
// rejected at parse, never forwarded"). ParseWire's CLI-argv surface must
// tolerate and discard trailing bytes because an attacker can only ever append
// to a single string; a keyed representation has no such harmless place for an
// extra key to hide, so here an unrecognized key is itself the defect, named
// and returned rather than silently ignored. An ill-formed identifier or a
// reason outside this grammar's vocabulary is a second, distinct fail-closed
// case — either way, ok signal or a named defect, never a partial one.
func (g Grammar) ParseClosed(raw string) (Signal, *schema.Defect) {
	var fields struct {
		ID     string `json:"id"`
		Reason Reason `json:"reason"`
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fields); err != nil {
		d := closedParseDefect(raw, "closed parse: "+err.Error())
		return Signal{}, &d
	}
	if dec.More() {
		d := closedParseDefect(raw, "closed parse: trailing data after the signal object")
		return Signal{}, &d
	}
	sig, ok := g.New(fields.ID, fields.Reason)
	if !ok {
		d := closedParseDefect(raw, fmt.Sprintf("identifier %q / reason %q did not validate against the grammar", fields.ID, fields.Reason))
		return Signal{}, &d
	}
	return sig, nil
}

func closedParseDefect(raw, why string) schema.Defect {
	return schema.Defect{
		Verb:   verb,
		File:   raw,
		Op:     "parse",
		Reason: "wake signal failed closed parse: " + why,
		Remedy: `a wake signal is exactly {"id": "<identifier>", "reason": "<reason>"} — no other key, and both values must validate against the grammar`,
	}
}

// argusGrammar is the argus wake stack's own closed reason vocabulary, pinned
// so the package-level Signal/NewSignal/ParseWire/Wire surface — the argus
// tick's existing contract — is this Grammar's behavior by construction: T6's
// parametrization changes no argus-observed byte.
var argusGrammar = NewGrammar(ReasonMerge, ReasonItem, ReasonAnswer)

// NewSignal builds a validated signal under the argus wake stack's grammar,
// fail-closed: an ill-formed slug or an unknown reason yields ok=false and the
// zero Signal, never a coerced value. It is the one constructor every argus
// delivery path funnels through, so no path can mint a signal the wire grammar
// would refuse.
func NewSignal(slug string, reason Reason) (Signal, bool) {
	return argusGrammar.New(slug, reason)
}

// Wire renders the signal to its normalized on-the-wire form — the exact,
// minimal surface a push adapter emits into the front door. It is the inverse of
// ParseWire on a well-formed signal and carries not one byte beyond slug and
// reason.
func (s Signal) Wire() string {
	return "wake:" + s.Slug + ":" + string(s.Reason)
}

// ParseWire extracts a signal from ANY raw wake-surface bytes under the argus
// wake stack's grammar, reducing it to slug + reason and discarding everything
// else (DINV-1). A surface that does not begin with a well-formed
// `wake:<slug>:<reason>` token yields ok=false — no touch, never a guess.
func ParseWire(raw string) (Signal, bool) {
	return argusGrammar.ParseWire(raw)
}

// reasonForAwait maps a completed await to its touch reason by the goal object's
// closed scheme (the same scheme probeAwait dispatches on): a `pr:` merge, an
// `item:` land. An await whose scheme has no reason mapping is not a
// backstop-deliverable completion and yields ok=false.
func reasonForAwait(a argusledger.Await) (Reason, bool) {
	scheme, _, ok := splitScheme(a.GoalObject)
	if !ok {
		return "", false
	}
	switch scheme {
	case schemePR:
		return ReasonMerge, true
	case schemeItem:
		return ReasonItem, true
	}
	return "", false
}
