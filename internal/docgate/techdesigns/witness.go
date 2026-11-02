package techdesigns

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/shapes"
)

// solution-shape T4 (DINV-4, §3.8): the witness / trace / completeness
// cross-check between a design's binding markup and its paired shape. Three
// defects, keyed on the binding markup the design class already carries — no
// new tagging grammar (§3.8):
//
//   - unwitnessed-invariant  — a design DINV-n with no witnessing shape element
//   - untraced-shape-element — a shape element citing no decision in its design
//   - shape-incomplete       — a stateful design (one that references its shape)
//     whose paired shape is absent
//
// Pairing is by slug (§3.1): a shape names its design in the header; a design
// references its shape in the body (§2 — "the frozen design references its
// shape and never contains it"). Pure: the parsed designs and shapes are
// passed in; the gate package does the IO.

var (
	// declaredInvariantRe matches a design's DECLARED invariant — the bold
	// anchor form `**DINV-n**` that opens its definition (§3.3). A plain
	// `DINV-n` mention elsewhere is a citation, not a declaration.
	declaredInvariantRe = regexp.MustCompile(`\*\*DINV-([0-9]+)\*\*`)
	// decisionAnchorRe matches any decision anchor a shape element may cite —
	// a design invariant (`DINV-n`) or an acceptance decision (`A-n`).
	decisionAnchorRe = regexp.MustCompile(`\b(?:DINV-[0-9]+|A-[0-9]+)\b`)
	// shapeRefRe matches a design's typed reference to its shape, `shape:<slug>`.
	shapeRefRe = regexp.MustCompile(`\bshape:([a-z0-9-]+)\b`)
)

// ShapeConsistency runs T4's three checks over the designs and the shapes
// loaded from the corpus (DINV-4). shapesRoot is the configured
// solution-shape corpus root — its basename is how a design's body reference
// to a paired shape is recognized. Deterministic in input order.
func ShapeConsistency(designs []*Design, shps []*shapes.Shape, shapesRoot string) []schema.Defect {
	byPaired := map[string]*shapes.Shape{} // paired-design slug → shape (witness/trace pairing)
	slugSet := map[string]bool{}           // shape slug → exists (completeness resolution)
	for _, s := range shps {
		if s == nil {
			continue
		}
		byPaired[s.Design] = s
		slugSet[s.Slug] = true
	}
	var defects []schema.Defect
	for _, d := range designs {
		if d == nil {
			continue
		}
		// Completeness: a stateful design that references an absent shape.
		for _, slug := range ReferencedShapes(d, shapesRoot) {
			if !slugSet[slug] {
				defects = append(defects, incompleteDef(d.Slug+".md", slug, shapesRoot))
			}
		}
		// Witness + trace: only when a shape is actually paired to the design.
		if s := byPaired[d.Slug]; s != nil {
			defects = append(defects, witnessTrace(d, s)...)
		}
	}
	return defects
}

// witnessTrace runs the witness + trace halves for one paired design+shape.
func witnessTrace(d *Design, s *shapes.Shape) []schema.Defect {
	var defects []schema.Defect
	valid := decisionAnchors(d)
	els := shapeElements(s.Body)

	// Witness: every declared DINV-n is cited by some shape element.
	cited := map[string]bool{}
	for _, e := range els {
		for _, c := range e.citations {
			cited[c] = true
		}
	}
	for _, inv := range DeclaredInvariants(d) {
		if !cited[inv] {
			defects = append(defects, witnessDef(d.Slug+".md", inv, s.Slug))
		}
	}
	// Trace: every element cites at least one decision that exists in the design.
	for _, e := range els {
		traced := false
		for _, c := range e.citations {
			if valid[c] {
				traced = true
				break
			}
		}
		if !traced {
			defects = append(defects, traceDef(s.Slug+".md", e))
		}
	}
	return defects
}

// DeclaredInvariants returns the DINV-n a design DECLARES (bold-anchor form),
// in document order, deduped. Fenced blocks are skipped — a DINV-n inside a
// code block is not a declaration.
func DeclaredInvariants(d *Design) []string {
	if d == nil || d.Doc == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	fenced := false
	for _, l := range d.Doc.Lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, m := range declaredInvariantRe.FindAllStringSubmatch(l, -1) {
			id := "DINV-" + m[1]
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// decisionAnchors returns the set of decision anchors (DINV-n / A-n) present
// in a design — the valid citation targets a shape element must trace to.
func decisionAnchors(d *Design) map[string]bool {
	set := map[string]bool{}
	if d == nil || d.Doc == nil {
		return set
	}
	fenced := false
	for _, l := range d.Doc.Lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, m := range decisionAnchorRe.FindAllString(l, -1) {
			set[m] = true
		}
	}
	return set
}

// ReferencedShapes returns the shape slugs a design references — the signal
// that the design is stateful, carrying concrete state/interfaces realized in
// a shape (§2). A reference is a `shape:<slug>` typed ref or a link/path into
// the shapes corpus (matched on the corpus basename, so both
// `docs/solution-shapes/<slug>.md` and a `../solution-shapes/<slug>.md`
// relative link resolve). Fenced blocks are skipped.
func ReferencedShapes(d *Design, shapesRoot string) []string {
	if d == nil || d.Doc == nil {
		return nil
	}
	base := path.Base(strings.TrimSuffix(shapesRoot, "/"))
	pathRe := regexp.MustCompile(regexp.QuoteMeta(base) + `/([a-z0-9-]+)\.md`)
	seen := map[string]bool{}
	var out []string
	add := func(slug string) {
		if slug != "" && !seen[slug] {
			seen[slug] = true
			out = append(out, slug)
		}
	}
	fenced := false
	for _, l := range d.Doc.Lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, m := range shapeRefRe.FindAllStringSubmatch(l, -1) {
			add(m[1])
		}
		if base != "" {
			for _, m := range pathRe.FindAllStringSubmatch(l, -1) {
				add(m[1])
			}
		}
	}
	return out
}

// shapeElement is one reviewable unit of a shape body: a fenced code block
// (schema or interface, §3.1) plus the decision anchors it cites — read from
// the fence info string and the block's caption (the nearest non-blank,
// non-fence line above it).
type shapeElement struct {
	line      int      // 1-based line within the shape body, for the message
	desc      string   // the fence info string, for the message
	citations []string // decision anchors cited (DINV-n / A-n), deduped
}

// shapeElements splits a shape body into its fenced elements and reads each
// element's citations from its fence info line and its caption.
func shapeElements(body string) []shapeElement {
	lines := strings.Split(body, "\n")
	var els []shapeElement
	for i := 0; i < len(lines); i++ {
		info, ok := fenceInfo(lines[i])
		if !ok {
			continue
		}
		// Caption: the nearest non-blank line above the fence that is not
		// itself a fence delimiter (which would belong to a prior block).
		caption := ""
		for k := i - 1; k >= 0; k-- {
			t := strings.TrimSpace(lines[k])
			if t == "" {
				continue
			}
			if !isFence(lines[k]) {
				caption = lines[k]
			}
			break
		}
		els = append(els, shapeElement{
			line:      i + 1,
			desc:      strings.TrimSpace(info),
			citations: dedupe(decisionAnchorRe.FindAllString(info+"\n"+caption, -1)),
		})
		// Skip to the closing fence so the block body is not rescanned.
		j := i + 1
		for j < len(lines) && !isFence(lines[j]) {
			j++
		}
		i = j
	}
	return els
}

func witnessDef(designFile, inv, shapeSlug string) schema.Defect {
	return schema.Defect{
		Verb:   "unwitnessed-invariant",
		File:   designFile,
		Op:     "witness",
		Reason: fmt.Sprintf("%s has no witnessing element in the paired shape %q", inv, shapeSlug),
		Remedy: "caption a shape element with " + inv + ", or drop the invariant — every DINV-n is witnessed by concrete shape (DINV-4, §3.8)",
	}
}

func traceDef(shapeFile string, e shapeElement) schema.Defect {
	where := fmt.Sprintf("line %d", e.line)
	if e.desc != "" {
		where = fmt.Sprintf("line %d (```%s)", e.line, e.desc)
	}
	return schema.Defect{
		Verb:   "untraced-shape-element",
		File:   shapeFile,
		Op:     "trace",
		Reason: fmt.Sprintf("element at %s cites no decision in the paired design", where),
		Remedy: "caption the element with the DINV-n or A-n it realizes — every shape element traces to a decision (DINV-4, §3.8)",
	}
}

func incompleteDef(designFile, shapeSlug, shapesRoot string) schema.Defect {
	return schema.Defect{
		Verb:   "shape-incomplete",
		File:   designFile,
		Op:     "completeness",
		Reason: fmt.Sprintf("references solution-shape %q but no such shape exists — a stateful design's paired shape is missing", shapeSlug),
		Remedy: fmt.Sprintf("author the paired shape at %s/%s.md, or drop the reference if the design is pure-decision (DINV-4, §3.8)", strings.TrimSuffix(shapesRoot, "/"), shapeSlug),
	}
}

// dedupe returns the input with duplicates removed, order preserved.
func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
