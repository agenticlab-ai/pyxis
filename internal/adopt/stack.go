package adopt

import (
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
)

// requiredGateActions is the fixed executor.gate.<command> action set the
// adopter must supply once any stack is detected: build, test, and lint —
// the exact three kinds FR-7 names ("inventing no build, test, or lint
// command"). The set is the same for every stack: naming it is detection's
// whole job; wiring or inventing the underlying command is not (design
// §3.6).
var requiredGateActions = []string{
	"executor.gate.build",
	"executor.gate.test",
	"executor.gate.lint",
}

// DetectStack reads the ecosystem marker registry (internal/verify/ecosystem)
// and names every stack whose marker file is present at the repository
// root, together with the fixed gate-command action set the adopter must
// supply (design:existing-repo-onboarding-v2#3.6;
// prd:existing-repo-onboarding/FR-7). A repository no marker matches
// returns the zero Stack: Render degrades an empty Detected to the single
// unknown-stack line (prd:existing-repo-onboarding/UB-3) — detection itself
// never errors and never fabricates a command.
func DetectStack(root string) Stack {
	defs := ecosystem.New()
	var detected []string
	for _, name := range defs.StackNames() {
		if hasMarker(root, defs.ForStack(name).MarkerFiles) {
			detected = append(detected, name)
		}
	}
	if len(detected) == 0 {
		return Stack{}
	}
	return Stack{
		Detected:    detected,
		GateActions: append([]string(nil), requiredGateActions...),
	}
}

// hasMarker reports whether any named marker file exists at the repository
// root. A stat error of any kind — absent, permission-denied — is treated
// as "not present": detection degrades rather than errors.
func hasMarker(root string, markers []string) bool {
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			return true
		}
	}
	return false
}
