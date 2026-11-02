package compile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

var problemHeading = regexp.MustCompile(`^### (P-[1-9][0-9]*) — ([A-Z]+-[1-9][0-9]*) — (.+)$`)

// Ruling retains declared authorship for readiness and the lock request.
// The operator's exact-version approval, rather than this field, vouches for it.
type Ruling struct {
	Problem string
	Author  string
	Reason  string
	Change  string
}

type Problem struct {
	ID          string
	Requirement string
	Statement   string
	RaisedBy    string
	Ruling      *Ruling
}

func compileProblems(feature string, body []byte) ([]Problem, []Record, error) {
	var problems []Problem
	var records []Record
	seen := map[string]bool{}
	var current *Problem
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Statement == "" {
			return fmt.Errorf("compile draft %s problem %s: Problem line is missing", feature, current.ID)
		}
		subject := "prd/" + feature
		records = append(records, Record{Draft: reconcile.Draft{
			Kind: record.Question, Subject: subject, Family: current.ID,
			Statement: current.Statement, Intent: current.RaisedBy,
		}})
		if current.Ruling != nil {
			records = append(records, Record{Draft: reconcile.Draft{
				Kind: record.Answer, Subject: subject,
				Statement: current.Ruling.Change, Intent: current.Ruling.Author,
				Rationale: current.Ruling.Reason,
			}, Links: []Link{{Type: record.Answers, Target: subject + "#" + current.ID}}})
		}
		problems = append(problems, *current)
		return nil
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, nil, err
			}
			current = nil
			if strings.HasPrefix(line, "### P-") {
				match := problemHeading.FindStringSubmatch(line)
				if match == nil || seen[match[1]] {
					return nil, nil, fmt.Errorf("compile draft %s: invalid or repeated problem heading %q", feature, line)
				}
				if id, _, ok := schema.ParseID(match[2]); !ok || id != match[2] {
					return nil, nil, fmt.Errorf("compile draft %s problem %s: invalid requirement ID %q", feature, match[1], match[2])
				}
				seen[match[1]] = true
				current = &Problem{ID: match[1], Requirement: match[2]}
			}
			continue
		}
		if current == nil {
			continue
		}
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "**Raised by:**"); ok {
			parts := strings.SplitN(value, "**Problem:**", 2)
			current.RaisedBy = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				current.Statement = strings.TrimSpace(parts[1])
			}
		} else if value, ok := strings.CutPrefix(line, "**Problem:**"); ok {
			current.Statement = strings.TrimSpace(value)
		} else if value, ok := strings.CutPrefix(line, "**Ruling:**"); ok {
			parts := strings.SplitN(strings.TrimSpace(value), " · ", 3)
			if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
				return nil, nil, fmt.Errorf("compile draft %s problem %s: ruling needs author, reason, and change", feature, current.ID)
			}
			current.Ruling = &Ruling{Problem: current.ID, Author: strings.TrimSpace(parts[0]), Reason: strings.TrimSpace(parts[1]), Change: strings.TrimSpace(parts[2])}
		}
	}
	if err := flush(); err != nil {
		return nil, nil, err
	}
	return problems, records, nil
}
