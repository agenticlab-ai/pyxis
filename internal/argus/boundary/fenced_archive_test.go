package boundary

import "testing"

func TestCommittedScopeIgnoresFencedArchiveAndReadsFollowingEffects(t *testing.T) {
	for _, fence := range []string{"~~~~", "````"} {
		t.Run(fence, func(t *testing.T) {
			ledger := committedLedger + "\n### E-3 — reconciliation — archived subject\n\n- **Date**: 2026-10-06\n- **Principal**: Argus · **Authority-contract version**: authority-contract/v1\n- **External effect**: —\n- **Body**: The archive is data.\n\n" +
				fence + "markdown\n## Archived section\n" +
				"### E-99 — archived example\n" +
				"- **External effect**: internal/engine/core.go drafts/widget/reviews/012.md\n" +
				fence + "\n\n### E-4 — reconciliation — actual report\n\n" +
				"- **Date**: 2026-10-06\n- **Principal**: Argus · **Authority-contract version**: authority-contract/v1\n- **External effect**: drafts/widget/reviews/011.md\n- **Body**: Recorded the actual report.\n"
			scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
			if !ok {
				t.Fatal("valid ledger did not resolve")
			}
			if !scope.Owns("drafts/widget/reviews/011.md") {
				t.Fatal("actual report following archive was not owned")
			}
			for _, p := range []string{"internal/engine/core.go", "drafts/widget/reviews/012.md", "drafts/other/reviews/011.md"} {
				if scope.Owns(p) {
					t.Errorf("fenced or unrecorded artifact %s was owned", p)
				}
			}
		})
	}
}
