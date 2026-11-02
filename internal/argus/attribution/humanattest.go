package attribution

import "strings"

// humanAttestation is the positive human-authorship attestation the gate reads
// to establish a protected-path diff as positively-human (§3.3, DINV-5). It is
// the shipped, credential-free `Governing-Change: operator` pattern — the
// ordinary artifact a legitimate protected-path PR already carries (the
// governance and authority-contract classes require it at check-governing), so
// establishing human authorship demands NO new step of the human author (FR-13).
//
// This is the gate's OWN single-sourced positive-human reader (§7 Q4 fixes the
// own-reader; it reuses the `Governing-Change: operator` key). Per AP-11 it is
// deliberately NOT a second caller of internal/docgate/gate.hasOperatorTrailer:
// that reader answers a different predicate — "a governing-doc / gate-policy
// change is operator-attested" — while this one answers "this protected-path
// diff is positively human-authored for attribution", and the two will evolve
// independently (this use needs §3.3's positively-Argus precedence and the
// attribution gate's per-file scoping; the governing use is a whole-range
// substring by design). One predicate, one place; twin predicates always
// diverge.
const humanAttestation = "Governing-Change: operator"

// positivelyHumanAttested reports whether the PR's commit range (base..HEAD)
// carries the positive human-authorship attestation — a whole-body substring
// read over the range, mirroring the shape §3.3 fixes. It is the establishment
// half of the three-state resolution: a not-positively-Argus protected-path
// diff is positively-human (pass, NR-4) only when this is true; absent it, the
// diff is underivable and fails closed (DINV-2). It is NOT out of the drive's
// reach — a drive-influenced commit body can plant the string — which is
// exactly why the positively-Argus-wins precedence (Evaluate) evaluates the
// produced, out-of-reach Argus signal FIRST: a planted attestation can never
// launder a positively-Argus diff (§3.3, DINV-5).
func positivelyHumanAttested(root, base string) (bool, error) {
	out, err := gitOutput(root, "log", "--format=%B", base+"..HEAD")
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), humanAttestation), nil
}
