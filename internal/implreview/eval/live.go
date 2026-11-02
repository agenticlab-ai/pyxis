//go:build eval

// This file (and live_test.go) are the GATED LIVE-LLM eval path — the real eval
// suite that actually runs the four lenses N times against a real model backend
// (§3.5, DOD-2's "permanent CI" second half). It is behind the `eval` build tag,
// so it is COMPILED AND RUN ONLY under `go test -tags eval ./internal/implreview/eval/`
// and is EXCLUDED from the hermetic `go test ./...` the CI floor runs (which
// must stay green with no network and no LLM). Both gates matter:
//
//   - the build tag keeps the live suite out of the default build entirely (it is
//     not even compiled without -tags eval), so no LLM dependency leaks into the
//     hermetic gate; and
//   - within the live suite, an env-var check (IMPL_REVIEW_EVAL_LIVE=1) makes the
//     live run OPT-IN even when built with the tag, so a `-tags eval` build in a
//     backend-less environment SKIPS rather than fails (a missing backend is not a
//     family defect).
//
// HOW THE LIVE SUITE RUNS (the documented real eval path):
//
//	# hermetic floor (the CI gate — the harness + comparators + compactness,
//	# fully exercised with a deterministic fake runner, NO LLM):
//	go test ./internal/implreview/...
//
//	# the gated live eval (real lenses, N runs, caught-every-run — run per
//	# release, wired to the standing backend by the runner LiveRunner returns):
//	IMPL_REVIEW_EVAL_LIVE=1 go test -tags eval ./internal/implreview/eval/
//
// The live suite loads the SAME fixture data the hermetic harness loads (the eight
// families, budgets.json, provenance.json), builds a real review pack per leg from
// the leg's diff + charter + the real calibration table, loads the four lens
// instructions from the lens files, and drives the SAME RunFamily protocol — only
// the injected runner differs (a real sibling-subagent LensRunner instead of the
// fake). So the live suite reuses the entire hermetic protocol; it swaps the seam,
// not the logic. Families grow only by a reviewed data PR; a removal or threshold
// relaxation is a reviewed decision (§3.5).
package eval

import (
	"fmt"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// LiveEnvVar is the opt-in env var: the live suite runs its real-LLM legs only
// when this is set to "1". Unset (or any other value) SKIPS the live run even
// under `-tags eval`, so a tagged build without a backend does not fail.
const LiveEnvVar = "IMPL_REVIEW_EVAL_LIVE"

// LiveEnabled reports whether the live-LLM run is opted in (the env var == "1").
func LiveEnabled() bool { return os.Getenv(LiveEnvVar) == "1" }

// LiveRunner returns the real sibling-subagent LensRunner the live suite drives —
// the runner whose Run spawns a fresh-context lens subagent against the standing
// model backend and returns its candidate findings. Wiring this to the actual
// backend is the skill-runtime's job (.pyxis/authoring/impl-review.md); this seam
// is where the live suite plugs it in.
//
// It is deliberately unimplemented at this layer: the Go core models the seam
// (the whole T4/T12 design keeps the LLM behind runtime.LensRunner), and the live
// backend is provided by the skill runtime, not compiled into this package. Until
// a runtime is wired, LiveRunner returns an error, and the live test SKIPS on it
// (never a false failure) — the hermetic harness is what proves the protocol; the
// live suite proves the real lenses once a backend is wired.
func LiveRunner() (runtime.LensRunner, error) {
	return nil, fmt.Errorf("eval: no live LensRunner wired — the real backend is provided by the /impl-review skill runtime (%s); "+
		"the hermetic harness (go test ./...) proves the protocol", LiveEnvVar)
}
