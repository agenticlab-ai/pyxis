package techdesigns

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// CarriesShape is solution-shape T3 (DINV-1, §3.8): a frozen technical
// design carries NO concrete solution-shape in its body — at most a
// reference to its paired shape document. Concrete shape is the schema in
// native form the design §3.1 enumerates: a ```sql / ```dbml block, or a
// ```mermaid erDiagram. A design body that contains one is carrying the
// volatile shape that must live in the non-frozen sibling; the check emits
// a design-carries-shape defect naming the offending block. A design that
// only *mentions* or *links* its shape (prose, an inline code span, a
// flowchart) passes clean — the tag is which fenced form it is, so no new
// grammar is needed (§3.8). Pure: the parsed designs are passed in.
func CarriesShape(designs []*Design) []schema.Defect {
	var defects []schema.Defect
	for _, d := range designs {
		defects = append(defects, carriesShape(d)...)
	}
	return defects
}

func shapeDef(file, reason string) schema.Defect {
	return schema.Defect{
		Verb:   "design-carries-shape",
		File:   file,
		Op:     "shape",
		Reason: reason,
		Remedy: "move the concrete schema/interfaces to the paired solution-shape and reference it from the design — a frozen design carries no shape (DINV-1, §3.1)",
	}
}

func carriesShape(d *Design) []schema.Defect {
	if d == nil || d.Doc == nil {
		return nil
	}
	file := d.Slug + ".md"
	var defects []schema.Defect
	lines := d.Doc.Lines
	for i := 0; i < len(lines); i++ {
		info, ok := fenceInfo(lines[i])
		if !ok {
			continue
		}
		// Scan to the closing fence; the enclosed lines are the block body.
		j := i + 1
		for j < len(lines) && !isFence(lines[j]) {
			j++
		}
		if form := concreteSchema(info, lines[i+1:min(j, len(lines))]); form != "" {
			// Doc.Lines drops the H1, so the file line is the index + 2.
			defects = append(defects, shapeDef(file, fmt.Sprintf(
				"line %d: %s in the design body — concrete solution-shape content, not a reference",
				i+2, form)))
		}
		i = j // resume after the closing fence
	}
	return defects
}

// isFence reports whether a line is a code-fence delimiter (``` at line
// start, ignoring leading indentation).
func isFence(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

// fenceInfo returns the info string of an opening fence (the text after the
// backticks), and whether the line is a fence at all.
func fenceInfo(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "```") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimLeft(t, "`")), true
}

// concreteSchema names the concrete-shape form a fenced block is, or ""
// when the block is not shape content. The forms are exactly those §3.1
// names: a sql or dbml schema block, or a mermaid entity-relationship
// diagram. A mermaid flowchart / sequence / other diagram is not schema.
func concreteSchema(info string, body []string) string {
	lang := strings.ToLower(firstToken(info))
	switch lang {
	case "sql":
		return "a ```sql schema block"
	case "dbml":
		return "a ```dbml schema block"
	case "mermaid":
		// erDiagram may sit on the fence line (```mermaid erDiagram) or open
		// the block body.
		if strings.EqualFold(firstToken(strings.TrimSpace(strings.TrimPrefix(info, firstToken(info)))), "erDiagram") {
			return "a ```mermaid erDiagram"
		}
		for _, l := range body {
			if s := strings.TrimSpace(l); s != "" {
				if strings.EqualFold(firstToken(s), "erDiagram") {
					return "a ```mermaid erDiagram"
				}
				break
			}
		}
	}
	return ""
}

func firstToken(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}
