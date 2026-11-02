// Package acting is the merge lane's thin proposal-event adapter and its
// fail-closed activation gate (merge-lane-v1 §3.7, DINV-3/DINV-5; plan T4). It
// is the layer between the hermetic classifier verb (internal/lane/classifier)
// and the platform: it runs the ONE classifier verb per proposal event, records
// the verdict (shadow's whole point), resolves the lane's operating state from
// the repo alone, and — only in the enforcing state, only for an AUTO verdict
// with every required check green, its reviewer precondition met, and the head
// unmoved — approves and merges as the dedicated machine identity, bound to the
// exact evaluated head, then verifies the act reached the goal object.
//
// The lane is born in shadow and the acting layer is disabled BY CONSTRUCTION,
// not by flag-discipline: the activation gate refuses any act while the
// operator's recorded activation acts are absent and the refusal NAMES the
// missing act; a half-present activation (enforce set, but the org-backstop ref
// or the lane-state ref-protection absent) resolves shadow PLUS a named
// activation-incomplete defect at every state read, never a silent enforce
// (FR-9, A-3). The machine identity's credential lives in the executing medium's
// secret store, behind the Machine port; this package holds no credential and
// exposes no accessor to one, so a drive session — which never receives a
// Machine — cannot reach it (DINV-5, A-8).
//
// This file owns the state resolution and the activation-incomplete defects;
// acting.go owns the ports and the per-event flow; identity.go owns the machine
// identity and the durable act self-description. Stdlib only, per the repo Go
// rules; every uncertain input fails closed to shadow/HUMAN, never a silent act.
package acting

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// verb names this adapter in every defect it emits (schema.Defect.Verb).
const verb = "lane-acting"

// State is the lane's resolved operating state, resolvable from the repo alone —
// the policy file plus the lane-state record (NFR-5). There is no hidden state:
// the same policy and observed preconditions resolve the same state every read
// (DINV-4's determinism carried to the acting layer).
type State string

const (
	// Shadow — the classifier runs and records, the acting layer acts on
	// nothing. The birth state and the fail-closed fallback for any
	// not-yet-activated or half-activated policy.
	Shadow State = "shadow"
	// Enforcing — activation is fully present; DINV-3's act may proceed for an
	// AUTO verdict, followed by goal verification.
	Enforcing State = "enforcing"
	// Halted — a goal-verify failure recorded an unresolved halt (DINV-7). T5
	// owns the lane-state record; T4 accepts the halt as an observed signal and
	// stops acts on it.
	Halted State = "halted"
	// Killed — the kill switch flipped the lane HUMAN-only in one data change
	// (A-4).
	Killed State = "killed"
)

// Preconditions are the activation preconditions this feature READS as data —
// never performs or schedules (FR-9's pattern). RefProtectionPresent is the
// medium-observed lane-state ref-protection that restricts the halt namespace to
// the machine identity (DINV-7's write-authority enforcement, owned at T5);
// under enforce it must be present to reach Enforcing. Halted is T5's observed
// lane-state halt signal, false in the shadow/enforce flows this task ships; it
// is accepted here so state resolution stays the single, forward-composable
// definition.
type Preconditions struct {
	RefProtectionPresent bool
	Halted               bool
}

// Resolve computes the lane's operating state from the loaded policy and the
// observed activation preconditions, together with any activation-incomplete
// defects that must surface at EVERY state read (A-3). It is pure and
// fail-closed: an absent policy, an unset kill switch's opposite, a halt, an
// unactivated lane, or a half-present activation all resolve to a non-acting
// state; only a fully-present activation resolves Enforcing.
//
// The order is deliberate and terminal-first: kill before halt before
// activation, so the strongest stop wins and no downstream signal can override
// it (the safety spine of merge-lane-v1 §2).
func Resolve(p *policy.Policy, pc Preconditions) (State, []schema.Defect) {
	// No loaded policy → the lane cannot enforce. It stays shadow; the loader
	// already surfaced the load defect and the classifier routes all-HUMAN
	// (DINV-8). Acting on a policy we could not load would be exactly the
	// fail-open the closed load forbids.
	if p == nil {
		return Shadow, nil
	}
	// The kill switch is the single data change that flips the lane HUMAN-only,
	// evaluated before any enforcement decision (A-4).
	if p.KillSwitch {
		return Killed, nil
	}
	// A recorded, unresolved halt stops acts before any enforcement decision
	// (DINV-7); its lane-state record and clear authority are T5's.
	if pc.Halted {
		return Halted, nil
	}
	// Shadow by default: the acting layer is disabled by construction until the
	// operator's recorded activation is present (FR-9).
	if !p.Activation.Enforce {
		return Shadow, nil
	}
	// enforce:true — a half-present activation resolves shadow PLUS a named
	// activation-incomplete defect at every state read (A-3), never a silent
	// enforce. Both preconditions must be present to reach Enforcing: the
	// operator's org-backstop change (recorded in the policy) and the lane-state
	// ref-protection (observed on the medium).
	var defects []schema.Defect
	if p.Activation.BackstopChangeRef == "" {
		defects = append(defects, def("activation",
			"enforce is set but the operator's org-backstop change ref is absent (activation.backstop_change_ref is empty)",
			"the operator records the org-backstop act before the lane enforces; until then it stays shadow (FR-9, A-3)"))
	}
	if !pc.RefProtectionPresent {
		defects = append(defects, def("activation",
			"enforce is set but the lane-state ref-protection restricting the halt namespace to the machine identity is absent",
			"the operator sets the lane-state ref-protection at activation before the lane enforces; until then it stays shadow (DINV-7, A-3)"))
	}
	if len(defects) > 0 {
		return Shadow, defects
	}
	return Enforcing, nil
}

// def builds one named activation/act defect. File is the generic surface name;
// callers that know the proposal stamp it (see acting.go's refuse).
func def(op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: "lane-acting", Op: op, Reason: reason, Remedy: remedy}
}
