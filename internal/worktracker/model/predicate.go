package model

import "sort"

// The ready-set and triage-set predicates — the engine's two dispatch questions,
// each answered by exactly one implementation shared verbatim by the WorkTracker
// port and the Carina CLI (docs/tech-design/local-work-tracker.md §3.6, DINV-6;
// prd:local-work-tracker/FR-12, FR-13, FR-16). They live in the medium-neutral
// model so the two front doors cannot drift: readiness once diverged into a
// token-burning re-fire loop, and one predicate in one place is the structural
// cure (AP-11/INV-30).
//
// Both predicates read only header-sized fields (state, type, signals,
// assignees, blocked_by) and the append log's decision-records — never a body —
// so a decision-path scan stays cheap at capacity (§3.6). The corpus is passed
// in whole because "no open blocker" resolves a local blocked_by id against the
// other items; the adapter supplies the corpus from a direct file scan, keeping
// any index off the decision path (DINV-4).

// TriageDisposition is the value a triage-disposition decision-record carries —
// the triage decision, a closed engine-recognized set (§3.14, DINV-11). It is
// the payload of a record in the NamespaceTriageDisposition namespace; T3 owns
// the record medium and deferred its meaning to this predicate task.
type TriageDisposition string

const (
	// DispositionPlan routes the item to planning.
	DispositionPlan TriageDisposition = "plan"
	// DispositionOperator routes the item to a human operator.
	DispositionOperator TriageDisposition = "operator"
	// DispositionReject closes the item as rejected — the one closing disposition.
	DispositionReject TriageDisposition = "reject"
	// DispositionSkip structurally skips the item (a rollup, not actionable).
	DispositionSkip TriageDisposition = "skip"
)

// TriageDispositions is the closed triage-disposition value set, in canonical
// order (§3.14).
var TriageDispositions = []TriageDisposition{DispositionPlan, DispositionOperator, DispositionReject, DispositionSkip}

// ValidTriageDisposition reports whether d is in the closed set.
func ValidTriageDisposition(d TriageDisposition) bool { return in(TriageDispositions, d) }

// closingDispositions are the triage-disposition values that close an item —
// only reject (⇒ close with rejected, §3.14). An item carrying a closing
// disposition is on its way out and is never ready, even while still open.
var closingDispositions = []TriageDisposition{DispositionReject}

// rollupTypes are the structural (non-leaf) hierarchy roles: feature, milestone,
// and epic roll up children; only a task is a dispatchable leaf. A structural
// rollup is never in the ready or triage set — it is not itself implemented, its
// children are (§3.6, §3.14).
var rollupTypes = []Type{TypeFeature, TypeMilestone, TypeEpic}

// IsStructuralRollup reports whether w is a structural rollup (a non-task
// hierarchy node) — excluded from both dispatch predicates.
func IsStructuralRollup(w WorkItem) bool { return in(rollupTypes, w.Type) }

// authorizationSignals is the set of signal names whose role is authorization,
// drawn from the governed seed vocabulary. The authorization role is not
// operand-extensible (only classification is, §3.14), so the seed's
// authorization signals are the complete set — the "factory may implement and
// merge this" token that gates the ready-set (§3.5, DINV-6).
func authorizationSignals() map[string]bool {
	out := map[string]bool{}
	for _, s := range SeedSignals() {
		if s.Role == RoleAuthorization {
			out[s.Name] = true
		}
	}
	return out
}

// CarriesAuthorization reports whether w carries an authorization-role signal —
// the ready-set gate (§3.6, FR-12).
func CarriesAuthorization(w WorkItem) bool {
	auth := authorizationSignals()
	for _, name := range w.Signals {
		if auth[name] {
			return true
		}
	}
	return false
}

// hasClosingDisposition reports whether w's latest triage-disposition record is a
// closing one (reject). A reject that a later record superseded does not count —
// latest-wins (DINV-5).
func hasClosingDisposition(w WorkItem) bool {
	rec, ok := w.LatestRecord(NamespaceTriageDisposition)
	if !ok {
		return false
	}
	return in(closingDispositions, TriageDisposition(rec.Payload))
}

// hasTriageDisposition reports whether w carries any triage-disposition record —
// i.e. triage has decided it, so it has left the undecided set (§3.6, FR-13).
func hasTriageDisposition(w WorkItem) bool {
	_, ok := w.LatestRecord(NamespaceTriageDisposition)
	return ok
}

// hasOpenBlocker reports whether w has a local blocked_by edge whose target is
// open. Only local ids are resolved: a cross-repo owner/repo#id reference is
// opaque and never resolved in v1 (§3.5, Q5), so the local predicate does not
// gate on it — the engine composes cross-repo readiness across its ports (§3.6
// boundary). A local edge whose target is missing or unresolvable does not
// block (a vanished blocker no longer blocks); add-time refusal keeps a dangling
// edge from arising in practice (UB-2). This mirrors the local-only reverse
// Blocking scan, which also treats cross-repo edges as leaves in the local graph.
func hasOpenBlocker(w WorkItem, index map[int]WorkItem) bool {
	for _, ref := range w.BlockedBy {
		kind, id := ClassifyRef(ref)
		if kind != RefLocal {
			continue
		}
		if b, ok := index[id]; ok && b.State == StateOpen {
			return true
		}
	}
	return false
}

// IsReady reports whether w is in the ready set given the corpus index for
// blocker resolution: open ∧ carries an authorization signal ∧ not a structural
// rollup ∧ no open blocker ∧ no closing disposition (§3.6, FR-12).
func IsReady(w WorkItem, index map[int]WorkItem) bool {
	return w.State == StateOpen &&
		CarriesAuthorization(w) &&
		!IsStructuralRollup(w) &&
		!hasOpenBlocker(w, index) &&
		!hasClosingDisposition(w)
}

// IsTriage reports whether w is in the triage (undecided) set given the corpus
// index for blocker resolution: open ∧ no triage disposition ∧ not a structural
// rollup ∧ no open blocker ∧ not assignee-held (in flight) (§3.6, FR-13).
func IsTriage(w WorkItem, index map[int]WorkItem) bool {
	return w.State == StateOpen &&
		!hasTriageDisposition(w) &&
		!IsStructuralRollup(w) &&
		!hasOpenBlocker(w, index) &&
		len(w.Assignees) == 0
}

// ReadySet returns the ready subset of items in creation (id) order (INV-12,
// fair rotation). It is the single implementation the port and CLI share.
func ReadySet(items []WorkItem) []WorkItem { return selectSet(items, IsReady) }

// TriageSet returns the triage (undecided) subset of items in creation (id)
// order. It is the single implementation the port and CLI share.
func TriageSet(items []WorkItem) []WorkItem { return selectSet(items, IsTriage) }

// selectSet indexes the corpus by id (for blocker resolution) and returns the
// members satisfying pred, stably ordered by id.
func selectSet(items []WorkItem, pred func(WorkItem, map[int]WorkItem) bool) []WorkItem {
	index := make(map[int]WorkItem, len(items))
	for _, w := range items {
		index[w.ID] = w
	}
	out := make([]WorkItem, 0)
	for _, w := range items {
		if pred(w, index) {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// PredicateSetSchema tags the machine (JSON) projection of a predicate set
// (prd:local-work-tracker/FR-15; AP-6). Additive evolution only.
const PredicateSetSchema = "worktracker.predicate-set/v1"

// SetView is the schema-versioned machine projection of a predicate set — the
// named predicate and its member items, each as the same ItemView `show` emits.
// The port and the CLI both build it from ReadySet/TriageSet, so their machine
// output is byte-identical (DINV-6, FR-16).
type SetView struct {
	Schema    string     `json:"schema"`
	Predicate string     `json:"predicate"`
	Items     []ItemView `json:"items"`
}

// Set projects a predicate's member items onto the schema-versioned set view.
// Items is always present (never null) so a consumer reads a stable shape.
func Set(predicate string, items []WorkItem) SetView {
	v := SetView{Schema: PredicateSetSchema, Predicate: predicate, Items: []ItemView{}}
	for _, w := range items {
		v.Items = append(v.Items, w.View())
	}
	return v
}
