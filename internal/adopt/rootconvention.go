package adopt

// The root-convention-file collision resolution (design
// existing-repo-onboarding-v2 §3.3, NG-2; prd:existing-repo-onboarding/FR-6,
// FR-9): AGENTS.md (payload.OperandAgentsManualTarget) content has no
// honest mechanical merge, so the engine never rewrites one that already
// exists. ResolveRootConvention plans a collide-manual entry carrying the
// exact lines for the human to add by hand; Plan.ManualInstructions later
// lets undo name those same entries for the human to reverse (design §3.5)
// — symmetric to how the plan emitted them, since adoption never wrote to
// the file either way.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// manualInstructionDetail is the fixed "exact lines" a root-convention-file
// collide-manual entry carries. It is deliberately NOT the shipped operand
// manual's own prose (payload.OperandAgentsManual): reproducing that
// content here would need the verified release archive, and this
// resolution runs at plan time, before any archive access exists in this
// module (the one converge change this whole design sanctions is T1's
// write-primitive exposure at apply time, design §3.7) — it would also
// create a second, hand-synced copy of content the payload template
// already owns (AP-11). Instead it points the human at that template's own
// operand path — an ordinary engine-verbatim payload file, materialized
// there once the plan is applied regardless of this collision — and names
// the two commands that matter immediately: doctor for status, the gate
// for the check every push already runs.
var manualInstructionDetail = fmt.Sprintf(
	"- This repository is factory-operated by pyxis — run `pyxis doctor` for its health, drift, and adoption status.\n"+
		"- Run `pyxis exec docgate -- gate` before every push — the same check the git hooks and CI run.\n"+
		"- The operating manual pyxis would have authored lives at `%s` once this plan is applied — fold what fits your workflow into this file.",
	payload.OperandAgentsManual,
)

// ResolveRootConvention detects whether the root convention file the engine
// would author (payload.OperandAgentsManualTarget, "AGENTS.md") already
// exists in root. When it does, ok is true and the returned Entry is the
// collide-manual resolution: the engine writes nothing to the file, at plan
// time or ever (design §3.3, "content has no honest mechanical merge"), and
// Detail carries the exact lines for the human to add by hand. ok is false
// when the path is absent — there is no collision to resolve; a non-empty
// tree with no prior root convention file is outside this task's scope
// (existing-repo-onboarding/001 T7 covers only FR-6's collision case).
func ResolveRootConvention(root string) (Entry, bool, error) {
	abs := filepath.Join(root, payload.OperandAgentsManualTarget)
	switch _, err := os.Stat(abs); {
	case err == nil:
		return Entry{Path: payload.OperandAgentsManualTarget, Class: ClassCollideManual, Detail: manualInstructionDetail}, true, nil
	case os.IsNotExist(err):
		return Entry{}, false, nil
	default:
		return Entry{}, false, fmt.Errorf("adopt: probe %s: %w", payload.OperandAgentsManualTarget, err)
	}
}

// ManualInstructions returns every collide-manual entry in the plan, in
// their existing (path-sorted) order — the paths undo names for the human
// to reverse by hand (design §3.5; prd:existing-repo-onboarding/FR-9),
// symmetric to how the plan named them for the human to add: adoption never
// wrote to any of these paths, so undo's whole job here is naming them,
// never deleting them.
func (p Plan) ManualInstructions() []Entry {
	var out []Entry
	for _, e := range p.Entries {
		if e.Class == ClassCollideManual {
			out = append(out, e)
		}
	}
	return out
}
