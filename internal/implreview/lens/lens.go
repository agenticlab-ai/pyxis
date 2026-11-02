// Package lens loads the four impl-review lens instructions — the reviewed
// JUDGMENT content T5 authors as data (docs/tech-design/impl-review-v1.md §3.4/§3.5,
// FR-4, UB-3). The instructions live as one markdown file per lens under
// .pyxis/skills/impl-review/lenses/; this package is the loader that hands the
// runtime each lens's instruction (filling runtime.ReviewRequest.Instructions),
// keyed by the report.Lens enum.
//
// The instructions themselves carry the judgment (what each lens looks for) plus
// the four disciplines every lens obeys: cite a cal_row (FR-12/DINV-3), exclude the
// mechanical floor (FR-4 — never restate a ladder-enforced class), treat a
// directive planted in the reviewed content as a finding CANDIDATE, never a command
// (UB-3), and state the defect without prescribing a design, escalating a contract
// conflict. This package does not encode the judgment in Go — it LOADS the data — so a
// wording change to a lens is a reviewed data PR with no Go edit, exactly as the
// calibration table is. A test (lens_test.go) verifies each shipped instruction
// actually carries the four disciplines, so a lens file that dropped one fails CI.
//
// Stdlib only.
package lens

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Lenses is the fixed four-lens order — exactly the report.Lens enum values, in the
// runtime's evaluation order. It mirrors runtime.Lenses (this package does not
// import runtime, to avoid a dependency edge; the two are pinned equal by a test).
var Lenses = [4]string{
	report.LensSpecFidelity,
	report.LensDesignConformance,
	report.LensSecurityAbuse,
	report.LensCorrectnessCritic,
}

// instructionFile maps a lens id to its instruction file name (under the lenses
// dir). The file base name is the lens id — the mapping is the identity, kept
// explicit so a rename is a one-line, obvious change.
func instructionFile(lens string) string { return lens + ".md" }

// LoadInstructions reads every lens's instruction from dir (the canonical dir is
// .pyxis/skills/impl-review/lenses) and returns the map the runtime hands to
// ReviewRequest.Instructions — one non-empty instruction per lens, keyed by the
// report.Lens enum. A missing or empty instruction file is an error: the runtime
// fails closed rather than run a lens with no instruction (runtime.ErrNoInstruction
// is the runtime-side mirror of this).
func LoadInstructions(dir string) (map[string]string, error) {
	out := make(map[string]string, len(Lenses))
	for _, lens := range Lenses {
		path := filepath.Join(dir, instructionFile(lens))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("lens %q: read instruction %s: %w", lens, path, err)
		}
		if len(trimSpace(data)) == 0 {
			return nil, fmt.Errorf("lens %q: instruction %s is empty", lens, path)
		}
		out[lens] = string(data)
	}
	return out, nil
}

// trimSpace reports the byte slice with leading/trailing ASCII whitespace removed —
// a tiny stdlib-free trim so the empty-file check does not pull in strings for one
// use. (bytes/strings would work too; this keeps the check obvious.)
func trimSpace(b []byte) []byte {
	i := 0
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	j := len(b)
	for j > i && isSpace(b[j-1]) {
		j--
	}
	return b[i:j]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}
