// Coverage region tests (plan-viewer-v1 §3.6 region 4, plan T3). The matrix
// must render so a reviewer can navigate it: every coverage row appears, every
// task reference resolves to an in-page card anchor, and every deferred row
// carries its distinct marking and reason (A-4 / PRD FR-4). The assertions run
// over the green fixture's real fold path, so the matrix reflects the machine
// face, not a hand-built doc.
package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// coverageRegion returns the bytes of the coverage section only.
func coverageRegion(page string) string {
	return section(page, `id="coverage"`, "</section>")
}

// Every covered row renders its requirement, and every task reference is an
// in-page link whose target is a real card anchor (A-4's covered half).
func TestCoverageRowsAndLinks(t *testing.T) {
	root, page := renderPage(t)
	doc := plan.FoldFile(root, config.Defaults(), planRel).Doc
	region := coverageRegion(page)
	if region == "" {
		t.Fatal("no coverage region rendered")
	}

	for _, row := range doc.Coverage {
		if !strings.Contains(region, row.Requirement) {
			t.Errorf("coverage region missing requirement %q", row.Requirement)
		}
		for _, ref := range row.Tasks {
			link := `href="#` + ref + `"`
			if !strings.Contains(region, link) {
				t.Errorf("coverage row %q: task ref %q is not an in-page link (%s)", row.Requirement, ref, link)
			}
			// The link resolves: the task-card region emits exactly this anchor.
			if !strings.Contains(page, `id="`+ref+`"`) {
				t.Errorf("coverage task ref %q resolves to no card anchor id=%q", ref, ref)
			}
		}
	}
}

// Every deferred row renders with its reason and a distinct marking that a
// covered row does not carry (A-4's deferred half).
func TestCoverageDeferredDistinct(t *testing.T) {
	root, page := renderPage(t)
	doc := plan.FoldFile(root, config.Defaults(), planRel).Doc
	region := coverageRegion(page)

	if len(doc.Deferred) == 0 {
		t.Fatal("fixture has no deferred rows to assert distinct marking")
	}
	for _, row := range doc.Deferred {
		// The deferred row and its reason both render.
		rowHTML := section(region, row.Requirement, "</tr>")
		if rowHTML == "" || !strings.Contains(rowHTML, row.Reason) {
			t.Errorf("deferred row %q missing its reason %q", row.Requirement, row.Reason)
		}
		// The distinct marking rides on the deferred row, never a covered one.
		if !strings.Contains(rowHTML, "deferred") {
			t.Errorf("deferred row %q carries no distinct marking", row.Requirement)
		}
	}

	// The marking is exclusive to deferred rows: the covered rows carry it on
	// neither their <tr> class nor their cell.
	coveredRe := regexp.MustCompile(`(?s)<tr class="cov-row">.*?</tr>`)
	for _, m := range coveredRe.FindAllString(region, -1) {
		if strings.Contains(m, "deferred") {
			t.Errorf("a covered row carries the deferred marking:\n%s", m)
		}
	}
}

// A plan with no deferred rows still renders every covered row, and the region
// carries no stray deferred marking.
func TestCoverageNoDeferred(t *testing.T) {
	doc := maximalDoc()
	doc.Coverage = []plan.CoverageRow{
		{Requirement: "prd:x/FR-1", Tasks: []string{"T1"}},
	}
	b, err := Page(doc, stubBundle)
	if err != nil {
		t.Fatal(err)
	}
	region := coverageRegion(string(b))
	if !strings.Contains(region, "prd:x/FR-1") || !strings.Contains(region, `href="#T1"`) {
		t.Errorf("covered row not rendered without deferred rows:\n%s", region)
	}
	if strings.Contains(region, "deferred") {
		t.Errorf("region marks a row deferred with no deferred rows present:\n%s", region)
	}
}
