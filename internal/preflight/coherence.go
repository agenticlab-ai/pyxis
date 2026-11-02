// Cross-knob coherence (design init-hardening-v1 §3.1 check 6; plan
// init-hardening/001 T3): the closed per-knob enums already gate each
// VALUE; this check gates COMBINATIONS the enums cannot see individually.
// Its first refusing pair is live from birth (design §0 row 6): ci_mode
// excluding hosted CI while wiring.protection requires the hosted check
// context — a repository born wedged, every PR blocked on a required check
// that can never report. Structural and IO-free, so — unlike the
// gh-facing checks above — it runs unconditionally, the same posture the
// enum gate it complements already has.
package preflight

import (
	"context"
	"fmt"
)

// CrossKnobCoherenceID is check 6's registry id.
const CrossKnobCoherenceID = "cross-knob-coherence"

var crossKnobCoherenceCheck = Check{
	ID:            CrossKnobCoherenceID,
	Applicability: nil, // gates the resolved knob set itself — always applicable
	Probe:         probeCrossKnobCoherence,
}

// probeCrossKnobCoherence refuses the one wedged pair this design names:
// ci_mode=local (hosted CI excluded) together with a wiring.protection
// level that REQUIRES the hosted check context (required-checks or
// checks+review both name it — RequiredCheckContext). The refusal names
// both remedies (design §3.1: "include hosted CI, or set protection to
// none"), never picking one for the caller.
func probeCrossKnobCoherence(ctx context.Context, in Input) (Outcome, error) {
	k := in.Knobs
	hostedExcluded := k.CIMode == "local"
	hostedRequired := k.Wiring.Protection == "required-checks" || k.Wiring.Protection == "checks+review"
	if hostedExcluded && hostedRequired {
		return Outcome{
			Status: StatusRefuse,
			Reason: fmt.Sprintf(
				"ci_mode=%s excludes hosted CI while wiring.protection=%s requires its check context — the repository would be born wedged, every PR blocked on a required check that never reports",
				k.CIMode, k.Wiring.Protection),
			Remedy: "include hosted CI (set ci_mode to actions or both), or set wiring.protection to none",
		}, nil
	}
	return Outcome{Status: StatusPass}, nil
}
