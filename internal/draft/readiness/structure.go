package readiness

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// checkStructure reads the PRD's numbered headings through the same parser as
// the draft compiler. A skipped section still has its heading and is present.
func checkStructure(result compile.Result) []Unmet {
	var body []byte
	for _, entry := range result.Records {
		if entry.Draft.Kind == record.Document && entry.Draft.Subject == "prd/"+result.Feature && entry.Draft.Family == "prd" {
			body = entry.Draft.Body
			break
		}
	}
	if body == nil {
		return []Unmet{{"FR-41", "prd.md", "include the PRD working file"}}
	}
	doc, defects := ledger.ParseProjection("prd.md", schema.Normalize(body))
	if doc == nil || len(defects) > 0 {
		return []Unmet{{"FR-41", "prd.md", "repair the PRD section structure"}}
	}
	seen := map[string]bool{}
	for _, section := range doc.Sections {
		seen[section.Anchor] = true
	}
	var unmet []Unmet
	for _, anchor := range []string{"0", "1", "2", "3", "4", "5", "5.8", "6", "7", "8"} {
		if !seen[anchor] {
			unmet = append(unmet, Unmet{"FR-41", "prd.md#" + anchor, "add PRD template section §" + anchor})
		}
	}
	if seen["5.8"] {
		done := false
		for _, unit := range doc.Units {
			if unit.Prefix == "DOD" && unit.Section == "5.8" {
				done = true
				break
			}
		}
		if !done {
			unmet = append(unmet, Unmet{"FR-41", "prd.md#5.8/DOD", "add a definition of done criterion under §5.8"})
		}
	}
	return unmet
}
