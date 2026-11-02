package compile

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// Requirement retains the PRD columns that the memory claim mapping does not
// carry but readiness must judge.
type Requirement struct {
	ID, Priority, FitCriterion, VerifyMethod string
}

func compilePRD(feature string, body []byte) ([]Record, []Requirement, error) {
	doc, defects := ledger.ParseProjection("prd.md", schema.Normalize(body))
	if len(defects) > 0 {
		return nil, nil, fmt.Errorf("compile draft %s PRD: %s", feature, defects[0].String())
	}
	if doc == nil {
		return nil, nil, fmt.Errorf("compile draft %s PRD: parser returned no document", feature)
	}
	out := make([]Record, 0, len(doc.UnitOrder))
	var requirements []Requirement
	for _, id := range doc.UnitOrder {
		unit := doc.Units[id]
		table := doc.Tables[unit.Table]
		if len(table.Cols) != len(unit.Cells) {
			return nil, nil, fmt.Errorf("compile draft %s PRD row %s: %d cells for %d columns", feature, id, len(unit.Cells), len(table.Cols))
		}
		switch unit.Prefix {
		case "FR", "NR", "NFR", "UB":
			requirements = append(requirements, Requirement{ID: id, Priority: cell(table.Cols, unit.Cells, "Priority"), FitCriterion: cell(table.Cols, unit.Cells, "Fit criterion"), VerifyMethod: cell(table.Cols, unit.Cells, "Verify")})
		}
		mapped := reconcile.ProjectDraftRow(id, unit.Prefix, table.Cols, unit.Cells)
		subject := "prd/" + feature
		switch unit.Prefix {
		case "Q":
			if strings.TrimSpace(mapped.Statement) == "" {
				return nil, nil, fmt.Errorf("compile draft %s PRD row %s: question is empty", feature, id)
			}
			out = append(out, Record{Draft: reconcile.Draft{
				Kind: record.Question, Subject: subject, Family: id,
				Statement: mapped.Statement, Intent: mapped.Intent,
				Rationale: mapped.Rationale, FitCriterion: mapped.FitCriterion,
			}})
			status := cell(table.Cols, unit.Cells, "Status")
			resolution := ""
			if resolvedStatus(status) {
				resolution = cell(table.Cols, unit.Cells, "Resolution")
			}
			if resolution == "" {
				lower := strings.ToLower(status)
				for _, prefix := range []string{"answered — ", "resolved — ", "withdrawn — "} {
					if strings.HasPrefix(lower, prefix) {
						resolution = strings.TrimSpace(status[len(prefix):])
						break
					}
				}
			}
			if resolution != "" {
				out = append(out, Record{Draft: reconcile.Draft{
					Kind: record.Answer, Subject: subject, Statement: resolution,
				}, Links: []Link{{Type: record.Answers, Target: subject + "#" + id}}})
			}
		case "R":
			statement := cell(table.Cols, unit.Cells, "Risk / assumption / rabbit hole")
			if statement == "" {
				return nil, nil, fmt.Errorf("compile draft %s PRD row %s: risk or assumption is empty", feature, id)
			}
			out = append(out, Record{Draft: reconcile.Draft{
				Kind: record.Decision, Subject: subject, Family: id,
				Statement: statement, Rationale: cell(table.Cols, unit.Cells, "Mitigation or test"),
			}})
		case "NG", "WR":
			// Context rows remain in the literal PRD Document. The ledger
			// reconciler deliberately has no durable claim mapping for them.
		default:
			if !mapped.Mapped {
				return nil, nil, fmt.Errorf("compile draft %s PRD row %s: no complete claim mapping", feature, id)
			}
			out = append(out, Record{Draft: reconcile.Draft{
				Kind: mapped.Kind, Subject: subject, Family: id,
				Statement: mapped.Statement, Intent: mapped.Intent,
				Rationale: mapped.Rationale, FitCriterion: mapped.FitCriterion,
				Consequence: mapped.Consequence,
			}})
		}
	}
	return out, requirements, nil
}

func resolvedStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	for _, prefix := range []string{"answered", "resolved", "closed", "withdrawn"} {
		if status == prefix || strings.HasPrefix(status, prefix+" — ") {
			return true
		}
	}
	return false
}

func cell(columns, cells []string, name string) string {
	for i, column := range columns {
		if strings.TrimSpace(strings.Trim(column, "* ")) == name {
			return strings.TrimSpace(cells[i])
		}
	}
	return ""
}
