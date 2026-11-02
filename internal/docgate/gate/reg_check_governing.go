package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-governing",
		Help:           "governing purity always; trailer/append-only with --merge-base",
		NeedsMergeBase: false,
		Run: func(c Ctx) Defects {
			defects := LintGoverning(c.Root, c.Cfg)
			if c.MergeBase != "" {
				defects = append(defects, CheckGoverning(c.Root, c.MergeBase, c.Head, c.Cfg)...)
			}
			return defects
		},
	})
}

// LintGoverning is FR-18's tree-local half: the abstract layer contains no
// relative markdown link and no baseline requirement-ID citation (rule 1
// as a check — fence-completion DINV-1). Runs with or without a merge base.
func LintGoverning(root string, cfg config.Config) []schema.Defect {
	var defects []schema.Defect
	for _, rel := range governingPaths(cfg) {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue // a governing doc may not exist in a fresh operand
		}
		lines := strings.Split(string(schema.Normalize(b)), "\n")
		fenced := false
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			clean := stripCodeSpans(l)
			for _, target := range linkTargets(clean) {
				if isExternal(target) || strings.HasPrefix(target, "#") {
					continue
				}
				defects = append(defects, vdef("check-governing", rel, "purity",
					fmt.Sprintf("line %d: relative link %q — the abstract layer links to no concrete document", i+1, target),
					"rule 1: concrete documents cite the vision/principles, never the reverse"))
			}
			if m := schema.CorpusIDRe.FindString(clean); m != "" {
				defects = append(defects, vdef("check-governing", rel, "purity",
					fmt.Sprintf("line %d: requirement-ID citation %q in a governing document", i+1, m),
					"rule 1: the vision and principles cite no requirement IDs"))
			}
		}
	}
	return defects
}

// CheckGoverning is FR-14: attestation, not authorization (DINV-4).
func CheckGoverning(root, mergeBase, head string, cfg config.Config) []schema.Defect {
	out, err := exec.Command("git", append([]string{"-C", root, "diff", "--name-only"}, diffSpan(mergeBase, head)...)...).Output()
	if err != nil {
		return []schema.Defect{vdef("check-governing", "docs", "git",
			fmt.Sprintf("git diff against %q failed: %v", mergeBase, err), "pass a resolvable --merge-base ref")}
	}
	touched := []string{}
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, g := range governingPaths(cfg) {
			if p == g {
				touched = append(touched, p)
			}
		}
	}
	if len(touched) == 0 {
		return nil
	}
	if !hasOperatorTrailer(root, mergeBase) {
		var defects []schema.Defect
		for _, p := range touched {
			defects = append(defects, vdef("check-governing", p, "history",
				"governing document changed without the `Governing-Change: operator` trailer",
				"only the operator changes the trust root; add the trailer to attest (review verifies the claim)"))
		}
		return defects
	}
	// BOTH governing documents are append-only even with the trailer
	// (FR-18: AP-7 says append-only from the beginning — the vision was
	// the unguarded half until review #54 A2).
	var defects []schema.Defect
	for _, p := range touched {
		diff, err := exec.Command("git", append(append([]string{"-C", root, "diff", "-U0"}, diffSpan(mergeBase, head)...), "--", p)...).Output()
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(diff), "\n") {
			if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") && strings.TrimSpace(l[1:]) != "" {
				defects = append(defects, vdef("check-governing", p, "history",
					"governing document line modified or deleted — the trust root is append-only even with the trailer",
					"append new entries; supersede content by new entries, never edits"))
				break
			}
		}
	}
	return defects
}
