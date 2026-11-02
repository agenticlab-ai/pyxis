// Shape-lifecycle wiring (issue #136; solution-shape-v1 §3.6, DINV-3/DINV-5):
// the git/IO half of the lifecycle checks whose logic lives pure in the shapes
// package. Same DINV-3 discipline as every verb — reads bytes and git history,
// reports typed defects, writes nothing.
package gate

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
)

func init() {
	Register(Check{
		Verb:           "check-shape-lifecycle",
		Help:           "shape status flip, stale-shape, binding-markup discrimination (DINV-3/5)",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckShapeLifecycle(c.Root, c.MergeBase, c.Cfg) },
	})
}

// CheckShapeLifecycle is the wired lifecycle check (issue #136): the stale-shape
// defect for an authoritative shape whose code exists, the one-way status flip
// over the closed enum, and the binding-markup discrimination that refuses a
// Decision change smuggled as a shape change. mergeBase may be empty (desk use);
// the history-dependent halves then no-op.
func CheckShapeLifecycle(root, mergeBase string, cfg config.Config) []schema.Defect {
	shps, defects := LoadShapes(root, cfg)

	// stale-shape (§3.4, DINV-3). The automatic "code has landed" signal is
	// Q2-deferred — until it lands, the flip is an explicit authoring act and
	// the gate has no code signal, so the predicate is false here. The check is
	// wired and ready to light up the moment Q2 resolves; shapes.StaleShape's
	// true branch is exercised by its unit tests.
	for _, s := range shps {
		defects = append(defects, shapes.StaleShape(s, false)...)
	}
	if mergeBase == "" {
		return defects
	}

	// The status flip is one-way (§3.6): validate each shape's transition
	// against its status at the merge base.
	for _, s := range shps {
		old, err := gitShow(root, mergeBase, cfg.SolutionShapes+"/"+s.Slug+".md")
		if err != nil {
			continue // a shape new since the base has no prior status
		}
		oldShape, _ := shapes.Parse(s.Slug, old)
		if oldShape == nil {
			continue // an unparseable prior version is LoadShapes/parse's to report
		}
		defects = append(defects, shapes.Transition(s.Slug+".md", oldShape.Status, s.Status)...)
	}

	// Binding-markup discrimination (DINV-5, FR-6): a change that alters the
	// binding markup of a shape's paired design is a Decision change — refused
	// as a shape change, routed to supersession. Keyed on the pairing, so it
	// speaks in the shape domain and complements check-immutable's blanket fence.
	for _, s := range shps {
		if s.Design == "" {
			continue
		}
		rel := cfg.TechDesigns + "/" + s.Design + ".md"
		old, err := gitShow(root, mergeBase, rel)
		if err != nil {
			continue // paired design new since the base — reviewed whole
		}
		newB, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if shapes.BindingMarkupChanged(old, newB) {
			defects = append(defects, vdef("solution-shape", rel, "lifecycle",
				"the paired design's binding markup (**Decision:**/DINV-n/A-n) changed — a Decision change, not a shape change",
				"a Decision change proceeds only by supersession: author a new design that cites the old (DINV-5, FR-6)"))
		}
	}
	return defects
}

// gitShow returns a file's bytes at a git ref, or an error when the path is
// absent there (a file new since the ref).
func gitShow(root, ref, rel string) ([]byte, error) {
	return exec.Command("git", "-C", root, "show", ref+":"+rel).Output()
}
