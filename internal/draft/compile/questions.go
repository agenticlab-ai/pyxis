package compile

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// compileTechQuestions reads the template's §7 table. A proposed resolution
// on an open row is not an answer; the row must be marked resolved.
func compileTechQuestions(feature string, body []byte) ([]Record, error) {
	var out []Record
	inSection := false
	header := false
	seen := map[string]bool{}
	for line := range strings.SplitSeq(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.HasPrefix(strings.ToLower(trimmed), "## 7. open questions")
			header = false
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		parts := strings.Split(trimmed[1:len(trimmed)-1], "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) != 6 {
			return nil, fmt.Errorf("compile draft %s tech spec open questions: expected 6 columns", feature)
		}
		if parts[0] == "ID" {
			header = true
			continue
		}
		if strings.HasPrefix(parts[0], "---") {
			continue
		}
		if !header {
			return nil, fmt.Errorf("compile draft %s tech spec open questions: missing header", feature)
		}
		id, prefix, ok := schema.ParseID(parts[0])
		if !ok || prefix != "Q" || id != parts[0] || seen[id] || parts[1] == "" {
			return nil, fmt.Errorf("compile draft %s tech spec open questions: invalid or repeated question %q", feature, parts[0])
		}
		seen[id] = true
		subject := "design/" + feature
		out = append(out, Record{Draft: reconcile.Draft{
			Kind: record.Question, Subject: subject, Family: id, Statement: parts[1], Intent: parts[2],
		}})
		if resolvedStatus(parts[4]) && parts[5] != "" {
			out = append(out, Record{Draft: reconcile.Draft{
				Kind: record.Answer, Subject: subject, Statement: parts[5],
			}, Links: []Link{{Type: record.Answers, Target: subject + "#" + id}}})
		}
	}
	return out, nil
}
