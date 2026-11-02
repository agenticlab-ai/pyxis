package attribution

import "testing"

func TestGit_CommittedDraftScopeUsesOnlyMergeBaseEffects(t *testing.T) {
	r := newRepo(t)
	ledger := committedWidget + `
### E-3 — reconciliation — retained reopening

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: drafts/widget/prd.md (+ tech-spec.md, events.md, problems.md)
- **Body**: Recorded the reopening effects.
`
	r.write("docs/problems/widget/delivery.md", ledger)
	base := r.commit("docs: seed committed reopening effects")
	for _, target := range []string{
		"drafts/widget/prd.md", "drafts/widget/tech-spec.md",
		"drafts/widget/events.md", "drafts/widget/problems.md",
		"drafts/widget/reviews/001.md", "drafts/other/prd.md",
		"internal/engine/core.go",
	} {
		r.write(target, "new working file\n")
	}
	r.write("docs/problems/widget/delivery.md", ledger+`
### E-4 — reconciliation — same-proposal ownership claim

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: drafts/widget/reviews/001.md · drafts/other/prd.md · internal/engine/core.go
- **Body**: A candidate-only ownership claim.
`)
	r.commitArgus("docs: reopen with stamped working files", "widget")
	got := r.run(base)
	for _, target := range []string{
		"drafts/widget/prd.md", "drafts/widget/tech-spec.md",
		"drafts/widget/events.md", "drafts/widget/problems.md",
		"docs/problems/widget/delivery.md",
	} {
		if _, refused := got[target]; refused {
			t.Errorf("merge-base recorded effect %q refused: %v", target, got)
		}
	}
	for _, target := range []string{
		"drafts/widget/reviews/001.md", "drafts/other/prd.md", "internal/engine/core.go",
	} {
		if got[target] != "out-of-scope" {
			t.Errorf("same-proposal claim for %q widened scope: %v", target, got)
		}
	}
}
