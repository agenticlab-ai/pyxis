package wake

import (
	"context"
	"fmt"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Unattended materialization (docs/tech-design/fleet-bridge-v1.md §3.1, §3.2;
// prd:argus-unattended-drive FR-11, UB-6). A plan freezing at its merge is an
// ordinary await; the resumed touch re-verifies the freeze at the goal object,
// then triggers projection **through the planning stage's own contract** — with
// no human prompt — and verifies each created item at the goal object. The
// bridge changes WHEN, never HOW: it introduces no second projection path and
// relaxes no precondition (DINV-1); re-triggers and racing wakes converge to the
// projection contract's own guarantee (DINV-2); every created item is verified
// (exists + the automated-implementation tag) and journaled before the bridge
// advances, and a discrepancy routes back through the stage, never a drive item
// edit (DINV-3).

// PlanFreeze is the plan-freeze await's parameters: the problem slug, the plan's
// introducing proposal as a `pr:` goal object, the plan id (for the EVT-4 gist),
// and the registration date.
type PlanFreeze struct {
	Slug    string
	PlanRef string // the introducing proposal, e.g. "pr:292"
	Plan    string // the plan id, e.g. "argus-unattended-drive/001"
	Since   string // YYYY-MM-DD
}

// PlanFreezeAwait builds the canonical plan-freeze await a drive registers when
// it stops at a frozen plan's introducing proposal (§3.1): the goal object is
// the proposal landed on the mainline, the condition is merged content an
// ancestor. It rides the ordinary await grammar (T6) — the bridge adds a trigger
// moment, not a twin await machinery.
func PlanFreezeAwait(pf PlanFreeze) argusledger.Await {
	return argusledger.Await{
		GoalObject: pf.PlanRef,
		Condition:  "the plan is merged and its content an ancestor of the mainline",
		Since:      pf.Since,
	}
}

// ProjectedItem is one task's projection outcome as the bridge consumes it —
// normalized from the planning stage's own report so this package imports no
// projection internals (the port's contract, not its mechanics). Ref is the
// created item's goal-object identity; Created distinguishes a fresh creation
// from an already-live one (the no-op signal); Duplicate marks a task id the
// stage found on more than one carrier (a component of the named-both halt).
type ProjectedItem struct {
	PlanTaskID string
	Ref        string
	Created    bool
	Duplicate  bool
}

// ProjectionResult is the planning stage's own projection outcome, normalized.
// Halted is the stage's detected named-both halt — a true concurrent
// both-probe-miss race the stage caught, routing withheld for the pair; the
// bridge surfaces it loudly and never repairs it (DINV-2). Defects are the
// stage's own, carried through unchanged.
type ProjectionResult struct {
	Items   []ProjectedItem
	Halted  bool
	Defects []schema.Defect
}

// Projector triggers projection through the planning stage's OWN contract — its
// consent, staged modes, idempotency, and wiring order unchanged (DINV-1). The
// real adapter calls internal/plangate/project.Project with the repo's tracker
// and dialect; the tests substitute a stub. The bridge holds only this trigger
// seam, so it adds no serialization, lock, or second projection path around the
// stage (DINV-2).
type Projector interface {
	Project(ctx context.Context) (ProjectionResult, error)
}

// ItemChecker verifies one created item at the goal object by an exact tracker
// read (AP-26 — never the projection's own self-report): whether the item exists
// and whether it carries the repo-configured automated-implementation tag (the
// dialect's routing label, e.g. "local-agent"). The real adapter reads the
// tracker; the tests substitute a stub. It is READ-ONLY: the bridge has no
// item-edit port, so "no item is edited by the drive" holds by construction
// (DINV-3).
type ItemChecker interface {
	Verify(ctx context.Context, ref string) (exists bool, hasTag bool, err error)
}

// Outcome is a materialization touch's result.
type Outcome string

const (
	// OutcomeMaterialized — the trigger created at least one item; every created
	// item verified at the goal object and was journaled.
	OutcomeMaterialized Outcome = "materialized"
	// OutcomeNoOp — a re-trigger with the items already live: nothing created,
	// a journaled no-op (UB-6).
	OutcomeNoOp Outcome = "no-op"
	// OutcomeHalt — the stage's detected named-both halt: routing withheld for
	// the racing pair, surfaced loudly, never repaired (DINV-2).
	OutcomeHalt Outcome = "halt"
	// OutcomeDiscrepancy — a created item's tag verification failed; a
	// reconciliation is journaled and the discrepancy is routed back through the
	// stage, no item edited (DINV-3).
	OutcomeDiscrepancy Outcome = "discrepancy"
	// OutcomeRefused — the plan merge did not re-verify at the goal object; no
	// projection triggered (the wake was premature or forged — the resume
	// protocol's own rule: never trust the wake's reason).
	OutcomeRefused Outcome = "refused"
)

// MaterialEntry is one journal entry the materialization touch produces for the
// resumed drive to append to its delivery ledger — a declared entry type and its
// EVT-4 gist body (§3.6). The bridge produces the entries; the drive owns the
// append (the journal is the drive's, never written by this package).
type MaterialEntry struct {
	Type string
	Gist string
}

// MaterializeResult is the touch's cold-readable outcome. Entries are the
// journal rows to append in order (the trigger, the confirmations, the
// reconciliations); Alarm asks the caller to surface loudly (the named-both halt
// or a tag discrepancy); RoutedBack records that a discrepancy re-entered the
// stage; Defects carry the stage's own defects and any refusal reason,
// surfaced, never repaired.
type MaterializeResult struct {
	Outcome    Outcome
	Entries    []MaterialEntry
	Alarm      bool
	RoutedBack bool
	Defects    []schema.Defect
}

// Materialize runs one plan-freeze materialization touch. It re-verifies the
// plan merge at the goal object, triggers projection through the stage's own
// contract with no prompt, and verifies each created item at the goal object,
// producing the journal entries the drive appends. It returns a hard error only
// for an infrastructure fault (a probe read, a projection call, or an item read
// that errored); the stage's detected halt, a tag discrepancy, and a premature
// wake are named outcomes in the result, not errors.
func Materialize(ctx context.Context, pf PlanFreeze, prober Prober, projector Projector, checker ItemChecker) (MaterializeResult, error) {
	// 1. Re-verify the plan merge at the goal object — never trust the wake's
	//    reason (the resume protocol's own rule). A premature or forged wake finds
	//    the freeze unverified and triggers nothing (§3.1).
	scheme, ref, ok := splitScheme(pf.PlanRef)
	if !ok || scheme != schemePR {
		return MaterializeResult{}, fmt.Errorf("wake: a plan-freeze goal object must be a pr: proposal, got %q", pf.PlanRef)
	}
	merged, err := prober.AncestorOfMainline(ctx, ref)
	if err != nil {
		return MaterializeResult{}, fmt.Errorf("wake: re-verifying the plan merge %q at the goal object: %w", pf.PlanRef, err)
	}
	if !merged {
		return MaterializeResult{Outcome: OutcomeRefused, Defects: []schema.Defect{refuseUnmerged(pf)}}, nil
	}

	// 2. Trigger projection through the planning stage's OWN contract. No prompt,
	//    no second path, no serialization added around the stage (DINV-1/DINV-2).
	res, err := projector.Project(ctx)
	if err != nil {
		return MaterializeResult{}, fmt.Errorf("wake: triggering projection for %q: %w", pf.Plan, err)
	}

	// The trigger is the act's own ruling-recorded entry, naming what the stage
	// projected (§3.6).
	trigger := argusledger.MaterializationGist{Plan: pf.Plan, Items: itemRefs(res.Items), Trigger: argusledger.TriggerWake}
	out := MaterializeResult{Entries: []MaterialEntry{{Type: argusledger.TypeRulingRecorded, Gist: trigger.Render()}}}

	// 3. The stage's detected named-both halt: routing withheld for the pair,
	//    surfaced loudly, never repaired, never serialized around (DINV-2).
	if res.Halted {
		out.Outcome = OutcomeHalt
		out.Alarm = true
		out.Defects = res.Defects
		return out, nil
	}

	// 4. Verify each CREATED item at the goal object — exists + the
	//    automated-implementation tag (DINV-3). An already-live item was verified
	//    at its own creation touch; a re-trigger re-verifies nothing (AP-19).
	createdAny := false
	var failed []ProjectedItem
	for _, it := range res.Items {
		if !it.Created {
			continue
		}
		createdAny = true
		exists, hasTag, err := checker.Verify(ctx, it.Ref)
		if err != nil {
			return MaterializeResult{}, fmt.Errorf("wake: verifying item %q at the goal object: %w", it.Ref, err)
		}
		if exists && hasTag {
			g := argusledger.MaterializationGist{Plan: pf.Plan, Items: []string{it.Ref}, Tag: argusledger.TagVerified, Trigger: argusledger.TriggerWake}
			out.Entries = append(out.Entries, MaterialEntry{Type: argusledger.TypeItemMaterialized, Gist: g.Render()})
			continue
		}
		failed = append(failed, it)
		g := argusledger.MaterializationGist{Plan: pf.Plan, Items: []string{it.Ref}, Tag: argusledger.TagFailed, Trigger: argusledger.TriggerWake}
		out.Entries = append(out.Entries, MaterialEntry{Type: argusledger.TypeReconciliation, Gist: g.Render()})
		out.Defects = append(out.Defects, tagDiscrepancyDefect(pf, it, exists))
	}

	// 5. A tag discrepancy routes back through the stage's own contract — never a
	//    drive item edit (DINV-3). Routing back is re-triggering the idempotent
	//    projection (it re-applies the routing tag); the bridge holds no item-edit
	//    port, so "no item edited by the drive" is structural.
	if len(failed) > 0 {
		if _, err := projector.Project(ctx); err != nil {
			return MaterializeResult{}, fmt.Errorf("wake: routing a tag discrepancy for %q back through the stage: %w", pf.Plan, err)
		}
		out.RoutedBack = true
		out.Alarm = true
		out.Outcome = OutcomeDiscrepancy
		return out, nil
	}

	// 6. No discrepancy: materialized if anything was created, else a journaled
	//    no-op (the re-trigger with items already live — UB-6).
	if createdAny {
		out.Outcome = OutcomeMaterialized
	} else {
		out.Outcome = OutcomeNoOp
	}
	return out, nil
}

// itemRefs collects the projected items' goal-object refs in sorted order for
// the trigger gist — deterministic regardless of the report's item order.
func itemRefs(items []ProjectedItem) []string {
	refs := make([]string, 0, len(items))
	for _, it := range items {
		refs = append(refs, it.Ref)
	}
	sort.Strings(refs)
	return refs
}

func refuseUnmerged(pf PlanFreeze) schema.Defect {
	return schema.Defect{Verb: verb, File: pf.Slug, Op: "materialize",
		Reason: "the plan-freeze wake for " + quote(pf.PlanRef) + " did not re-verify at the goal object: the plan's content is not an ancestor of the mainline",
		Remedy: "the await stands and the next touch re-verifies; a wake never grants what the goal object does not confirm (fleet-bridge-v1 §3.1)"}
}

func tagDiscrepancyDefect(pf PlanFreeze, it ProjectedItem, exists bool) schema.Defect {
	why := "the automated-implementation tag is absent or misapplied"
	if !exists {
		why = "the item does not exist at the goal object"
	}
	return schema.Defect{Verb: verb, File: it.Ref, Op: "materialize",
		Reason: "materialized item " + quote(it.Ref) + " (plan " + pf.Plan + ") failed goal-object verification: " + why,
		Remedy: "the discrepancy is journaled and routed back through the planning stage's own contract; the drive never edits the item (fleet-bridge-v1 DINV-3)"}
}
