package acting

import "encoding/json"

var marshalActRecord = json.Marshal

// Identity is the actor an act is attributed to in the medium's durable history.
// The dedicated machine identity carries Machine=true and a name distinguishable
// from any human handle, so an AUTO act is told apart from a human's forever
// (DINV-5, A-8). The credential behind this identity lives in the executing
// medium's secret store, reachable only by the lane's acting step — never from
// this value, which carries no secret, and never from any drive session.
type Identity struct {
	// Name is the login/handle recorded on the approval and merge. The dedicated
	// machine identity's name is distinct from every human's (A-8).
	Name string `json:"name"`
	// Machine is true only for the dedicated machine identity; a human actor
	// carries false. This is the structural distinguisher DINV-5 requires.
	Machine bool `json:"machine"`
}

// IsHuman reports whether this identity is a human actor — the complement of the
// machine identity. The lane's acts are the machine identity's alone.
func (i Identity) IsHuman() bool { return !i.Machine }

// ActRecordVersion is the one act self-description shape this build writes.
// Versioned, closed-schema data — additive evolution only (AP-6). It is the
// durable `auto_act` event row (merge-lane-v1 §3.4) the act carries into the
// medium's history, cold-readable years later (NFR-4).
const ActRecordVersion = "auto-act/v1"

// ActRecord is the machine identity's durable act self-description — the
// approval and merge, self-describing in the medium's history (merge-lane-v1
// §3.6's act block). It pins exactly what an audit needs to tie a verdict to its
// act to its goal verification: who acted (and that it was the machine, not a
// human), the exact evaluated head the act bound to, the policy version, a
// pointer to the evaluation block, the consumed reviewer-verdict ref where the
// class carries one, and the goal-object verification outcome. A GoalVerify of
// "fail" is a lying badge → the lane must halt (DINV-7; T5 persists the halt).
type ActRecord struct {
	RecordVersion   string `json:"record_version"`
	ApprovedBy      string `json:"approved_by"`
	MachineActor    bool   `json:"machine_actor"`
	HeadRevision    string `json:"head_revision"`
	PolicyVersion   string `json:"policy_version"`
	VerdictRecord   string `json:"verdict_record"`
	ReviewerVerdict string `json:"reviewer_verdict,omitempty"`
	GoalVerify      string `json:"goal_verify"` // pass | fail
}

// The two goal-verify outcomes. A pass means the merged content is an ancestor
// of the mainline; a fail means the badge lied (DINV-7, AP-26/I2).
const (
	goalVerifyPass = "pass"
	goalVerifyFail = "fail"
)

// Marshal serializes the act record deterministically — the struct's field
// order is fixed and every field is a scalar — so the same act yields
// byte-identical bytes (AP-6). One line, no trailing newline.
func (a ActRecord) Marshal() []byte {
	b, err := marshalActRecord(a)
	if err != nil {
		// ActRecord carries only scalars, so Marshal cannot fail; this guards a
		// future field that breaks that (programmer error) rather than emitting a
		// partial, unreadable record.
		panic("acting: act record marshal: " + err.Error())
	}
	return b
}
