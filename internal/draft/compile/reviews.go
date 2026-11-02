package compile

import (
	"fmt"
	"regexp"
	"strings"
)

var reviewDigest = regexp.MustCompile(`(?m)^\*\*Draft digest:\*\* (sha256:[0-9a-f]{64})\s*$`)
var findingHeading = regexp.MustCompile(`^### (FND-[1-9][0-9]*) — `)

type Finding struct {
	ID          string
	Severity    string
	Blocking    bool
	Disposition string
}

// Review is the version a review report says it read. Readiness compares this
// digest with the compiled contract digest; compilation does not judge it.
type Review struct {
	Path     string
	Digest   string
	Part     string // prd or tech-spec
	Findings []Finding
}

func compileReview(name string, body []byte) (Review, error) {
	matches := reviewDigest.FindAllSubmatch(body, -1)
	if len(matches) != 1 {
		return Review{}, fmt.Errorf("compile draft review %s: exactly one **Draft digest:** sha256:<hex> line is required", name)
	}
	review := Review{Path: name, Digest: strings.TrimSpace(string(matches[0][1]))}
	var current *Finding
	flush := func() {
		if current != nil {
			review.Findings = append(review.Findings, *current)
		}
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "**Review subject:**"); ok {
			review.Part = strings.TrimSpace(value)
		}
		if strings.HasPrefix(line, "### ") {
			flush()
			current = nil
			if match := findingHeading.FindStringSubmatch(line); match != nil {
				current = &Finding{ID: match[1]}
			}
		}
		if current != nil {
			if value, ok := strings.CutPrefix(line, "- **Severity**:"); ok {
				fields := strings.Fields(value)
				if len(fields) > 0 {
					current.Severity = strings.ToLower(fields[0])
				}
				current.Blocking = current.Severity == "blocking"
			}
			if value, ok := strings.CutPrefix(line, "- **Disposition**:"); ok {
				current.Disposition = strings.TrimSpace(value)
			}
		}
	}
	flush()
	if review.Part == "" {
		first, _, _ := strings.Cut(strings.ToLower(string(body)), "\n")
		switch {
		case strings.Contains(first, "prd review"):
			review.Part = "prd"
		case strings.Contains(first, "tech-spec review"), strings.Contains(first, "tech spec review"):
			review.Part = "tech-spec"
		}
	}
	return review, nil
}
