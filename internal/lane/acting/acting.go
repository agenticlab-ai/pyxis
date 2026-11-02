package acting

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// Medium is the lane's READ-ONLY view of the platform — the thin read layer the
// adapter drives (merge-lane-v1 §3.8's local-substitutable seam). It carries no
// credential and performs no mutation: the required-check states come from the
// medium's own set (AP-11, never a private copy), the head is re-read at act
// time so the verdict binds to fresh content (AP-15, DINV-3), the reviewer
// verdict is read as the arm's PUBLISHED check state (never parsed from the
// proposal's tree, DINV-3), and the goal-object verification reads the mainline
// ancestry of the merged content (AP-26/I2, DINV-7). A drive session may hold a
// Medium: reads leak no acting power.
type Medium interface {
	// Head returns the proposal's current head revision. Called at act time so a
	// post-evaluation push voids the verdict before any act (DINV-3).
	Head(ctx context.Context, proposal string) (string, error)
	// AllRequiredGreen reports whether every required mechanical check is green
	// for the proposal at head, read from the medium's own required-check set
	// (AP-11, DINV-3).
	AllRequiredGreen(ctx context.Context, proposal, head string) (bool, error)
	// ReviewVerdict returns the review-verdict arm's PUBLISHED check state
	// (verdict.CheckContext) for the proposal at head — the trusted, produced
	// state, never a verdict parsed from the proposal's own content (DINV-3).
	ReviewVerdict(ctx context.Context, proposal, head string) (verdict.CheckState, error)
	// MergedIntoMainline reports whether the act's merged content is an ancestor
	// of the mainline — the goal-object verification after every act (DINV-7). A
	// false means the badge lied.
	MergedIntoMainline(ctx context.Context, proposal, head string) (bool, error)
}

// Recorder persists the lane-verdict record that rides the proposal. Recording
// is the whole point of shadow and is NOT an act: it runs in every state,
// including shadow, and touches no merge gate. It is a separate port from Medium
// because a drive-side probe may be handed a read-only Medium with no Recorder,
// and from Machine because recording carries no credential.
type Recorder interface {
	// RecordVerdict appends one evaluation's verdict record to the proposal's
	// append-only verdict block (verdict.AppendBlock's carrier).
	RecordVerdict(ctx context.Context, proposal string, rec verdict.Record) error
}

// Machine is the credentialed acting port — the ONLY place approve+merge happen,
// performed as the dedicated machine identity whose credential lives in the
// executing medium's secret store, out of any drive session's reach (DINV-5).
// The adapter calls it ONLY in the enforcing state; a shadow-only or drive-side
// deployment is constructed with a nil Machine and can never act. The interface
// deliberately exposes NO credential accessor: the credential is sealed inside
// the workflow's implementation, never surfaced to this package or its Results.
type Machine interface {
	// Identity returns the machine identity the acts are attributed to —
	// distinguishable from any human in history (A-8).
	Identity() Identity
	// Act approves and merges the proposal bound to the EXACT head revision. An
	// implementation MUST fail rather than act if the medium's head no longer
	// equals head, so the act never lands on unevaluated content (DINV-3); the
	// adapter also re-checks the head before calling Act (belt and suspenders).
	Act(ctx context.Context, proposal, head string) error
}

// Adapter is the thin proposal-event adapter. It holds the read seam, the
// verdict recorder, and — only where the deployment may act — the credentialed
// Machine. It computes nothing itself beyond wiring the pinned inputs into the
// single classifier verb, so no call site can drift from the desks and fixtures
// (R5, AP-11).
type Adapter struct {
	Medium   Medium
	Recorder Recorder
	// Machine is nil for a shadow-only or drive-side deployment; the adapter
	// never dereferences it outside the enforcing state, and a nil Machine in the
	// enforcing state is a fail-closed refusal, never a panic.
	Machine Machine
}

// Event is one proposal event the adapter classifies. It carries the pinned
// inputs the verdict depends on — the proposal id, its diff (consumed as data),
// the merge-base/head pair the diff derives from, and the pinned check-state
// snapshot id — so the adapter passes them straight to the classifier and
// derives nothing (DINV-9, R5).
type Event struct {
	Proposal      string
	Diff          classifier.Diff
	Base          string
	Head          string
	CheckSnapshot string
}

// Result is the adapter's per-event outcome — a cold-readable record of what the
// lane did and, when it did not act, exactly why. Every field is data: the
// resolved state, the classifier verdict and its recorded projection, whether an
// act occurred and its durable self-description, whether a goal-verify failure
// means the lane must halt, and the named refusals (activation-incomplete, an
// unmet precondition, a voided revision, a not-green check, ...). A HUMAN verdict
// carries no refusal: it is a routing, not a refused act.
type Result struct {
	State    State
	Verdict  classifier.Verdict
	Record   verdict.Record
	Acted    bool
	Act      *ActRecord
	Halt     bool
	Refusals []schema.Defect
}

// Handle runs one proposal event end-to-end: classify with the single verb,
// record the verdict (shadow's contract), resolve the lane state, and — only in
// the enforcing state, only for an AUTO verdict clearing DINV-3's act gate — act
// as the machine identity bound to the evaluated head, then verify at the goal
// object. It returns a non-nil error ONLY for an infrastructure failure (a
// medium read or a recorder write that errored); a policy/activation/precondition
// stop is a named refusal in the Result, not an error, so the caller records the
// verdict either way. Acting never happens for a HUMAN verdict, in any
// non-enforcing state, on a moved head, without green checks, or without the
// reviewer precondition — each a fail-closed refusal (DINV-1, DINV-3).
func (a *Adapter) Handle(ctx context.Context, ev Event, p *policy.Policy, pc Preconditions) (Result, error) {
	// 1. Classify — the single verb, pinned inputs in, nothing computed here.
	//    The downgrade stage is deferred by default (FR-4); a deployment that
	//    configures one wires it through the classifier, not the adapter.
	in := classifier.Inputs{Diff: ev.Diff, Base: ev.Base, Head: ev.Head, CheckState: ev.CheckSnapshot}
	v := classifier.Classify(in, p, nil)
	rec := verdict.NewRecord(v)
	res := Result{Verdict: v, Record: rec}

	// 2. Record the verdict — every proposal carries a recorded verdict, in
	//    every state, including shadow (FR-8, A-5). Recording is not an act.
	if err := a.Recorder.RecordVerdict(ctx, ev.Proposal, rec); err != nil {
		return Result{}, fmt.Errorf("recording the lane verdict for %q: %w", ev.Proposal, err)
	}

	// 3. Resolve the lane state; the activation-incomplete defect(s), when a
	//    half-present activation resolves shadow, surface at THIS read (A-3).
	state, defects := Resolve(p, pc)
	res.State = state
	res.Refusals = append(res.Refusals, stamp(ev.Proposal, defects)...)

	// 4. A HUMAN verdict is routed to a human — never an act, and not a refused
	//    act. Nothing more to do.
	if v.Lane != classifier.Auto {
		return res, nil
	}

	// 5. The act gate. An AUTO verdict reaches an act ONLY in the enforcing
	//    state; every other state produces a NAMED refusal so no act is ever
	//    refused silently (FR-9, A-3). (The Killed state cannot co-occur with an
	//    AUTO verdict — the kill switch routes the classifier HUMAN, handled at
	//    step 4 — so it needs no case here.)
	if state != Enforcing {
		switch state {
		case Halted:
			res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
				"the lane is halted pending a human disposition; no further AUTO act occurs (DINV-7)",
				"a delivery-ledger-recorded human disposition clears the halt (T5)"))
		default:
			// Shadow: either enforce:false (no activation defect above — name the
			// missing act explicitly so the refusal is never silent, FR-9) or a
			// half-present activation (its named defect(s) already stamped in step 3).
			if len(defects) == 0 {
				res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
					"the lane is not activated: no act occurs until the operator records activation (activation.enforce is unset)",
					"the operator activates the lane by ratifying enforce plus the org-backstop and lane-state ref-protection (FR-9)"))
			}
		}
		return res, nil
	}

	// 6. Enforcing — DINV-3's act gate, in order, each a fail-closed refusal:
	//    revision binding → required checks → reviewer precondition → act → verify.

	// A shadow-only Adapter reaching Enforcing is a misconfiguration; fail
	// closed rather than dereference a nil Machine (never a panic across the
	// boundary, per the repo Go rules).
	if a.Machine == nil {
		res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
			"the enforcing lane has no machine identity configured; fail-closed, no act",
			"wire the acting Machine only in a deployment that holds the machine-identity credential (DINV-5)"))
		return res, nil
	}

	// Revision binding (AP-15, DINV-3): re-read the CURRENT head; a
	// post-evaluation push moved it, so the verdict evaluated stale content and
	// is void before any act.
	cur, err := a.Medium.Head(ctx, ev.Proposal)
	if err != nil {
		return res, fmt.Errorf("reading the current head for %q: %w", ev.Proposal, err)
	}
	if cur != ev.Head {
		res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
			"a post-evaluation push moved the head from "+ev.Head+" to "+cur+"; the verdict is void (DINV-3)",
			"re-evaluate at the new head; an act never lands on unevaluated content"))
		return res, nil
	}

	// Every required mechanical check green, read from the medium's own set
	// (AP-11, DINV-3).
	green, err := a.Medium.AllRequiredGreen(ctx, ev.Proposal, ev.Head)
	if err != nil {
		return res, fmt.Errorf("reading required-check states for %q: %w", ev.Proposal, err)
	}
	if !green {
		res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
			"a required mechanical check is not green; no act (DINV-3)",
			"the act waits until every required check the medium publishes is green"))
		return res, nil
	}

	// The reviewer precondition where the matched AUTO class carries one — read
	// ONLY from the arm's published check state, never the proposal's tree
	// (DINV-3). An AUTO verdict naming a class the loaded policy no longer
	// carries fails closed.
	class, ok := findClass(p, v.AutoClass)
	if !ok {
		res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
			"the verdict's AUTO class "+quote(v.AutoClass)+" is not in the loaded policy; fail-closed, no act",
			"re-classify under the loaded policy; the act trusts only a class the mainline policy carries"))
		return res, nil
	}
	reviewerRef := ""
	if class.RequiresReviewerVerdict {
		st, err := a.Medium.ReviewVerdict(ctx, ev.Proposal, ev.Head)
		if err != nil {
			return res, fmt.Errorf("reading the review-verdict check state for %q: %w", ev.Proposal, err)
		}
		if !verdict.PreconditionMet(class, st) {
			res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
				"the reviewer-verdict precondition is unmet: "+st.Reason,
				"the act waits for the review-verdict arm to publish an approving state (DINV-3)"))
			return res, nil
		}
		reviewerRef = st.Context + ":" + string(st.Status)
	}

	// Act as the machine identity, bound to the exact evaluated head (DINV-5).
	id := a.Machine.Identity()
	if err := a.Machine.Act(ctx, ev.Proposal, ev.Head); err != nil {
		// A refused/failed act is a named refusal, not an infrastructure error:
		// the medium declined (a late head move, a lost check race). Fail closed.
		res.Refusals = append(res.Refusals, refuse(ev.Proposal, "act",
			"the machine-identity act did not complete: "+err.Error(),
			"the act is fail-closed; nothing merged, re-evaluate the proposal"))
		return res, nil
	}
	res.Acted = true

	// Goal-object verification after every act (AP-26/I2, DINV-7): the merged
	// content must be an ancestor of the mainline. A false is a lying badge → the
	// lane must halt; T5 persists the halt record and stops further acts.
	merged, err := a.Medium.MergedIntoMainline(ctx, ev.Proposal, ev.Head)
	if err != nil {
		return res, fmt.Errorf("verifying mainline ancestry for %q: %w", ev.Proposal, err)
	}
	gv := goalVerifyPass
	if !merged {
		gv = goalVerifyFail
		res.Halt = true
	}
	res.Act = &ActRecord{
		RecordVersion:   ActRecordVersion,
		ApprovedBy:      id.Name,
		MachineActor:    id.Machine,
		HeadRevision:    ev.Head,
		PolicyVersion:   v.Pinned.PolicyVersion,
		VerdictRecord:   ev.Proposal + "@" + ev.Head,
		ReviewerVerdict: reviewerRef,
		GoalVerify:      gv,
	}
	return res, nil
}

// findClass returns the loaded policy's AUTO class of the given name. A verdict
// can only name a class the policy carried at classification, but the act reads
// the class fresh so a policy change between evaluation and act fails closed
// rather than acting on a stale perimeter.
func findClass(p *policy.Policy, name string) (policy.AutoClass, bool) {
	for _, c := range p.AutoClasses {
		if c.Name == name {
			return c, true
		}
	}
	return policy.AutoClass{}, false
}

// stamp fills the proposal into a batch of defects' File slot for traceability.
func stamp(proposal string, ds []schema.Defect) []schema.Defect {
	for i := range ds {
		ds[i].File = proposal
	}
	return ds
}

// refuse builds one named act refusal keyed to the proposal.
func refuse(proposal, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: proposal, Op: op, Reason: reason, Remedy: remedy}
}

// quote wraps a value for a readable, unambiguous reason.
func quote(s string) string { return "'" + s + "'" }
