package reconcile

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const designDecisionMemoryTag = "reconcile:design-decision"

// A repository path may begin with a dot, but it cannot use sentence
// punctuation as its final path byte. Keeping the final byte alphanumeric (or
// underscore) means prose such as "internal/widget." still associates the
// exact unit instead of swallowing the full stop into the candidate token.
var codeEvidenceTokenRE = regexp.MustCompile(`[a-z0-9._](?:[a-z0-9/._-]*[a-z0-9_])?`)

func analyzeDesignDecisionMemory(out *Report, s Snapshot, d Design, set *store.Set, proj *resolve.Projection) bool {
	file, ok := s.Files[d.Source]
	if !ok || d.Status != "live" {
		return false
	}
	blocks, err := ExtractLiveDesignDecisions(d, file)
	if err != nil {
		out.Findings = append(out.Findings, Finding{
			Code: "invalid-design-decision-source", Scope: "design/" + d.Slug,
			Detail: err.Error(), Remedy: "repair the live design's explicit Decision blocks before distilling them", Blocking: true,
		})
		return true
	}
	expectedFamilies := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		expectedFamilies[block.Family] = true
	}
	for _, head := range proj.Active("design/" + d.Slug) {
		if head.Kind == record.Decision && head.HasTag(designDecisionMemoryTag) && !expectedFamilies[head.Family] {
			out.Findings = append(out.Findings, Finding{
				Code: "stale-design-decision-memory", Scope: head.Subject + "#" + head.Family,
				Detail: "active reconciled Decision " + head.ID + " has no corresponding explicit Decision block in the live design",
				Remedy: "revoke the obsolete source-projected Decision before treating the design as fully reconciled", Blocking: true,
			})
		}
	}
	document := exactDesignDocument(s, d, set, proj)
	missing, actionable := 0, 0
	for _, block := range blocks {
		head := proj.Head(block.Subject, block.Family)
		if head != nil {
			if err := validateDesignDecisionMemory(head, block, document, s.Code); err != nil {
				out.Findings = append(out.Findings, Finding{
					Code: "design-decision-memory-drift", Scope: block.Subject + "#" + block.Family,
					Detail: err.Error(), Remedy: "append an explicit successor that restores exact source evidence and code associations", Blocking: true,
				})
			}
			continue
		}
		missing++
		if draft, ok := designDecisionDraft(s, block, document); ok {
			actionable++
			out.Actions = append(out.Actions, Action{Code: "add-design-decision", Scope: block.Subject + "#" + block.Family, Draft: draft})
		}
	}
	if missing > 0 {
		out.Findings = append(out.Findings, Finding{
			Code: "missing-design-memory", Scope: "design/" + d.Slug,
			Detail:   fmt.Sprintf("live design is missing %d of %d explicit Decision block(s) from native memory", missing, len(blocks)),
			Remedy:   "run memory reconcile --write after its exact source Document exists",
			Writable: actionable == missing, Blocking: true,
		})
	}
	return true
}

func designDecisionDraft(s Snapshot, block DesignDecisionBlock, document *record.Record) (Draft, bool) {
	if document == nil || !validGitObjectID(s.Commit) || !validDigest(block.SourceSHA256) || !validDigest(block.EvidenceSHA256) {
		return Draft{}, false
	}
	tags := []string{
		designDecisionMemoryTag,
		"source-block:" + block.EvidenceSHA256,
		"source-design:" + block.SourceSHA256,
		"source:tech-design",
	}
	for _, unit := range mentionedCodeUnits(block.Evidence, s.Code) {
		tags = append(tags, "code:"+unit.Path)
	}
	sort.Strings(tags)
	return Draft{
		Kind: record.Decision, Subject: block.Subject, Family: block.Family,
		Statement: block.Statement,
		Tags:      tags,
		Relations: []record.Relation{{Type: record.DependsOn, Target: document.ID}},
		Provenance: record.Provenance{
			Source: block.Source, Commit: s.Commit,
			Note: fmt.Sprintf("exact Decision evidence at %s lines %d-%d in section %s; sha256:%s", block.Source, block.StartLine, block.EndLine, block.Section, block.EvidenceSHA256),
		},
	}, true
}

func validateDesignDecisionMemory(head *record.Record, block DesignDecisionBlock, document *record.Record, code []codeinventory.Unit) error {
	if head.Kind != record.Decision || head.Statement != block.Statement {
		return fmt.Errorf("memory head %s does not preserve the exact Decision statement", head.ID)
	}
	if head.Intent != "" || head.Rationale != "" || head.FitCriterion != "" || head.Consequence != "" || head.Expires != "" || len(head.LearnedFrom) != 0 || len(head.Alternatives) != 0 {
		return fmt.Errorf("memory head %s adds semantic fields absent from the explicit Decision block", head.ID)
	}
	wantTags := []string{designDecisionMemoryTag, "source-block:" + block.EvidenceSHA256, "source-design:" + block.SourceSHA256, "source:tech-design"}
	for _, unit := range mentionedCodeUnits(block.Evidence, code) {
		wantTags = append(wantTags, "code:"+unit.Path)
	}
	if !sameStringSet(head.Tags, wantTags) {
		return fmt.Errorf("memory head %s does not carry one exact design, block, and migration tag", head.ID)
	}
	wantNote := fmt.Sprintf("exact Decision evidence at %s lines %d-%d in section %s; sha256:%s", block.Source, block.StartLine, block.EndLine, block.Section, block.EvidenceSHA256)
	if head.Provenance.Source != block.Source || !hasSourceLocator(head) || head.Provenance.Note != wantNote {
		return fmt.Errorf("memory head %s does not carry exact design source provenance", head.ID)
	}
	if document == nil || !sameProjectedRelationSet(head.Relations, []record.Relation{{Type: record.DependsOn, Target: document.ID}}) {
		return fmt.Errorf("memory head %s does not carry the exact source dependency", head.ID)
	}
	return nil
}

func mentionedCodeUnits(evidence string, units []codeinventory.Unit) []codeinventory.Unit {
	tokens := map[string]bool{}
	for _, token := range codeEvidenceTokenRE.FindAllString(strings.ToLower(evidence), -1) {
		tokens[token] = true
	}
	var out []codeinventory.Unit
	for _, unit := range units {
		matched := tokens[strings.ToLower(unit.Path)]
		for _, file := range unit.Files {
			matched = matched || tokens[strings.ToLower(file)]
		}
		if matched {
			out = append(out, unit)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left, right := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
