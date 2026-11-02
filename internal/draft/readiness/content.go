// Package readiness judges a compiled draft without writing memory records.
package readiness

import (
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

type Unmet struct {
	Condition string `json:"condition"`
	Item      string `json:"item"`
	Remedy    string `json:"remedy"`
}

// Verdict is the closed, shared output of local, CI, and lock readiness.
type Verdict struct {
	SchemaVersion string  `json:"schema_version"`
	Feature       string  `json:"feature"`
	DraftDigest   string  `json:"draft_digest"`
	Mainline      string  `json:"mainline"`
	Ready         bool    `json:"ready"`
	Unmet         []Unmet `json:"unmet"`
}

func New(feature, digest, mainline string) Verdict {
	return Verdict{SchemaVersion: "draft-readiness/v1", Feature: feature, DraftDigest: digest, Mainline: mainline, Unmet: []Unmet{}}
}

// Judge computes the shared readiness verdict over one compiled draft and the
// pinned mainline sources it cites.
func Judge(result compile.Result, sources Sources) Verdict {
	v := New(result.Feature, result.Digest, result.Mainline)
	authority := checkRulingAuthority(result, sources)
	v.Unmet = append(v.Unmet, authority.unmet...)
	subject := "prd/" + result.Feature + "#"
	answered := map[string]bool{}
	served := map[string]bool{}
	for _, entry := range result.Records {
		for _, link := range entry.Links {
			if entry.Draft.Kind == record.Answer && link.Type == record.Answers && !authority.refused[strings.TrimPrefix(link.Target, "prd/"+result.Feature+"#")] {
				answered[link.Target] = true
			}
			if entry.Draft.Kind == record.Decision && entry.Draft.Subject == "design/"+result.Feature && link.Type == record.Serves {
				served[link.Target] = true
			}
		}
	}
	for _, entry := range result.Records {
		if entry.Draft.Kind != record.Question || entry.Draft.Subject != "prd/"+result.Feature && entry.Draft.Subject != "design/"+result.Feature {
			continue
		}
		id := entry.Draft.Family
		item := entry.Draft.Subject + "#" + id
		if !answered[item] {
			v.Unmet = append(v.Unmet, Unmet{"FR-12", item, "answer the question or record a product ruling"})
		}
	}
	for _, req := range result.Requirements {
		item := subject + req.ID
		must := mustPriority(req.Priority)
		if must && !served[item] {
			v.Unmet = append(v.Unmet, Unmet{"FR-13", item, "add a tech-spec decision serving " + req.ID})
		}
		condition := "FR-40"
		if must {
			condition = "FR-14"
		}
		if !substantive(req.FitCriterion) {
			v.Unmet = append(v.Unmet, Unmet{condition, item, "add a pass/fail fit criterion"})
		}
		if !substantive(req.VerifyMethod) {
			v.Unmet = append(v.Unmet, Unmet{condition, item, "add a verification method"})
		}
	}
	latest := map[string]compile.Review{}
	for _, review := range result.Reviews {
		if review.Part != "prd" && review.Part != "tech-spec" {
			v.Unmet = append(v.Unmet, Unmet{"UB-7", review.Path, "identify the review subject as prd or tech-spec"})
			continue
		}
		for _, finding := range review.Findings {
			if finding.Severity != "blocking" && finding.Severity != "advisory" {
				v.Unmet = append(v.Unmet, Unmet{"UB-7", review.Path + "#" + finding.ID, "set finding severity to blocking or advisory"})
			}
		}
		if previous, ok := latest[review.Part]; !ok || previous.Path < review.Path {
			latest[review.Part] = review
		}
	}
	for _, part := range []string{"prd", "tech-spec"} {
		review, ok := latest[part]
		if !ok {
			v.Unmet = append(v.Unmet, Unmet{"FR-16", "reviews/" + part, "run the " + part + " review on the current draft"})
			continue
		}
		if review.Digest != result.Digest {
			v.Unmet = append(v.Unmet, Unmet{"FR-16", review.Path, "re-run the " + part + " review on the current draft"})
		}
		for _, finding := range review.Findings {
			if finding.Blocking && !disposed(finding.Disposition, sources.OperatorLogin) {
				v.Unmet = append(v.Unmet, Unmet{"FR-16", review.Path + "#" + finding.ID, "apply the finding or record a declined disposition with the operator's ruling"})
			}
		}
	}
	v.Unmet = append(v.Unmet, checkStructure(result)...)
	v.Unmet = append(v.Unmet, checkLinks(result, sources)...)
	if !sources.LockedVersion {
		v.Unmet = append(v.Unmet, checkNumbering(result, sources.Mainline)...)
	}
	v.Sort()
	return v
}

// The PRD template permits a ranking after the priority class, such as
// "Must (rank 2)". Read the leading class without changing the literal row.
func mustPriority(priority string) bool {
	fields := strings.Fields(priority)
	return len(fields) > 0 && strings.EqualFold(fields[0], "Must")
}

func substantive(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "—", "-", "none", "n/a", "tbd", "todo":
		return false
	}
	return true
}

// Sort preserves the readiness contract's condition order and stable item order.
func (v *Verdict) Sort() {
	order := map[string]int{"FR-10": 0, "FR-12": 1, "FR-13": 2, "FR-14": 3, "FR-16": 4, "FR-20": 5, "FR-22": 6, "FR-38": 7, "FR-39": 8, "FR-40": 9, "FR-41": 10, "FR-42": 11, "UB-7": 12}
	sort.SliceStable(v.Unmet, func(i, j int) bool {
		a, b := v.Unmet[i], v.Unmet[j]
		if order[a.Condition] != order[b.Condition] {
			return order[a.Condition] < order[b.Condition]
		}
		if a.Item != b.Item {
			return a.Item < b.Item
		}
		return a.Remedy < b.Remedy
	})
	v.Ready = len(v.Unmet) == 0
}
