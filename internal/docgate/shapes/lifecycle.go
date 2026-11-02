// Lifecycle over the closed status enum (solution-shape-v1 §3.6, DINV-3/DINV-5):
// the authoritative → superseded-by-code flip, the stale-shape defect for an
// authoritative shape whose implementing code has landed, and the binding-markup
// discrimination that tells a shape-confined change apart from a Decision change.
// Pure: every function takes its inputs and returns typed defects — the git/IO
// wiring lives in the gate package, per the docgate defects-as-data pattern.
package shapes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Transition validates one status flip recorded in a shape header (§3.6). The
// enum is closed and the flip is one-way: authoritative → superseded-by-code is
// the retirement, re-stating the same status is idempotent, and the reverse
// (superseded-by-code → authoritative) is illegal — once code is the source of
// truth a shape does not un-supersede (DINV-3). An unknown endpoint is the named
// status defect. Q2 defers the automatic code signal; until then the flip is an
// explicit authoring act, and this is the check that keeps it legal.
func Transition(file, from, to string) []schema.Defect {
	var defects []schema.Defect
	if !statuses[from] {
		defects = append(defects, def(file, "status", fmt.Sprintf("unknown prior status %q", from),
			"status is one of: authoritative, superseded-by-code"))
	}
	if !statuses[to] {
		defects = append(defects, def(file, "status", fmt.Sprintf("unknown status %q", to),
			"status is one of: authoritative, superseded-by-code"))
	}
	if len(defects) > 0 {
		return defects
	}
	if from == StatusSupersededByCode && to == StatusAuthoritative {
		return []schema.Defect{def(file, "lifecycle",
			"status regressed superseded-by-code → authoritative",
			"the flip is one-way: once code supersedes a shape it stays superseded (§3.6, DINV-3)")}
	}
	return nil
}

// StaleShape is the stale-shape defect (§3.4, DINV-3, R4): a shape still marked
// authoritative once its implementing code exists is a second source of truth —
// the code is now the truth and the shape must flip to superseded-by-code.
// codeExists is injected: the automatic signal is Q2-deferred, so the gate
// passes the signal it has (none yet), while callers that DO know pass true.
func StaleShape(s *Shape, codeExists bool) []schema.Defect {
	if s == nil || s.Status != StatusAuthoritative || !codeExists {
		return nil
	}
	return []schema.Defect{{
		Verb:   "stale-shape",
		File:   s.Slug + ".md",
		Op:     "lifecycle",
		Reason: "authoritative shape whose implementing code exists — the code is now the source of truth",
		Remedy: "flip the shape's status to superseded-by-code (§3.6); the code answers what the shape is now",
	}}
}

// bindingTokenRe matches the binding markup that names a design's durable core
// (§3.3, §3.8, DINV-5): the Decision statement and the DINV-n / A-n anchors.
var bindingTokenRe = regexp.MustCompile(`\*\*Decision:\*\*|\bDINV-[0-9]+\b|\bA-[0-9]+\b`)

// BindingMarkup returns the design's binding-markup lines in order — the lines
// carrying a **Decision:**, a DINV-n, or an A-n token, trimmed, outside code
// fences (a token inside a fenced block is illustrative, not binding). This is
// the durable core that only supersession may change; everything else in the
// design body is non-binding and freely editable.
func BindingMarkup(b []byte) []string {
	var out []string
	fenced := false
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if bindingTokenRe.MatchString(l) {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// BindingMarkupChanged discriminates a shape-confined change from a Decision
// change (DINV-5, §3.6): it reports whether a design's binding markup differs
// between two versions. False means the edit left the durable core intact — a
// non-binding change; true means the durable core moved, which is a Decision
// change and proceeds only by supersession, never as a shape re-approval (FR-6).
func BindingMarkupChanged(oldB, newB []byte) bool {
	return !equalLines(BindingMarkup(oldB), BindingMarkup(newB))
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
