package gate_test

// The Argus boundary-enforcement corpus's byte-identical-non-authority-verdict
// proof (plan docs/plans/argus-boundary-enforcement T5, acceptance criterion 3;
// prd:argus-boundary-enforcement/NR-1, M2 guardrail). The two merge-boundary
// checks this feature added — the authority-contract structure arm and the
// attribution gate — are additive: on any tree with no authority contract, the
// gate's verdict is byte-identical with and without them. Neither check fires,
// so every pre-existing docgate check's verdict on every non-authority-contract
// tree is untouched (the argus-ledger-lint additive-composition precedent).
//
// This complements TestAuthorityArmAdditiveOnNonAuthorityTree (one tree, one
// check) with a corpus of non-authority trees covering BOTH new checks on both
// the base-free and base-present gate paths.

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// newBoundaryCheckVerbs are the two verbs this feature added at the merge
// boundary — the set the byte-identical proof strips to recover the pre-existing
// verdict.
var newBoundaryCheckVerbs = map[string]bool{
	"check-authority-contract": true, // the structure arm
	attribution.Verb:           true, // the attribution gate
}

// TestBoundaryChecksByteIdenticalOnNonAuthorityTrees proves T5 criterion 3: on a
// corpus of trees with no authority contract, the full gate verdict is
// byte-identical with and without the two new merge-boundary checks — they
// contribute nothing, so no pre-existing verdict is perturbed.
func TestBoundaryChecksByteIdenticalOnNonAuthorityTrees(t *testing.T) {
	trees := map[string]map[string]string{
		"a plain wiki note": {
			"docs/wiki/note.md": "An ordinary note. No authority contract here.\n",
		},
		"a code-only tree": {
			"internal/engine/core.go": "package engine\n\nfunc Run() {}\n",
			"cmd/tool/main.go":        "package main\n\nfunc main() {}\n",
		},
		"a docs tree with a governance doc and a foreign ledger": {
			"docs/product-vision.md":          "# Vision\n",
			"docs/problems/widget/problem.md": "# Widget — problem\n",
			"docs/wiki/glossary.md":           "# Glossary\n",
		},
	}

	for name, files := range trees {
		files := files
		t.Run(name, func(t *testing.T) {
			root := authTempRepo(t, files)
			// Both the base-free (desk) and base-present (CI) gate paths.
			for _, base := range []string{"", "HEAD"} {
				with := gate.Gate(root, base, nil, config.Defaults())

				// Neither new check should fire on a non-authority tree.
				for _, d := range with {
					if newBoundaryCheckVerbs[d.Verb] {
						t.Errorf("base=%q: new boundary check %q fired on a non-authority tree: %+v", base, d.Verb, d)
					}
				}

				// Stripping the two new verbs must leave the verdict unchanged —
				// they added nothing, so every pre-existing check's verdict is
				// byte-identical.
				without := make([]schema.Defect, 0, len(with))
				for _, d := range with {
					if !newBoundaryCheckVerbs[d.Verb] {
						without = append(without, d)
					}
				}
				if authSerialize(with) != authSerialize(without) {
					t.Errorf("base=%q: non-authority verdict differs with vs without the new boundary checks", base)
				}
			}
		})
	}
}
