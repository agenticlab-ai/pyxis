package boundary

import (
	"os"
	"path/filepath"
	"testing"
)

// These accepted source ledgers are historical inputs, not freshly generated
// fixtures. Their prior ownership must survive stricter validation of the new
// plan/capture extension without rewriting the journals.
func TestAcceptedDeliveryLedgersRetainHistoricalScope(t *testing.T) {
	for _, tc := range []struct {
		slug, historicalPath string
		newGrammar           bool
	}{
		{"argus-ledger-lint", "docs/plans/argus-ledger-lint/001-plan.md", true},
		{"maintenance-sweep", "docs/prds/maintenance-sweep/000-base.md", false},
		{"argus-attribution-hardening", "docs/prds/argus-boundary-enforcement/001-attribution-hardening.md", false},
		{"argus-boundary-enforcement", "docs/prds/argus-boundary-enforcement/000-base.md", true},
		{"forgejo-feature-draft", "docs/plans/forgejo-feature-draft/001-plan.md", false},
		{"argus-unattended-drive", "docs/prds/argus-unattended-drive/000-base.md", false},
	} {
		t.Run(tc.slug, func(t *testing.T) {
			ledger, err := os.ReadFile(filepath.Join("../../..", "docs/problems", tc.slug, "delivery.md"))
			if err != nil {
				t.Fatal(err)
			}
			scope, ok := ResolveCommittedLedger(dcfg, tc.slug, ledger)
			if !ok {
				t.Fatal("accepted ledger lost historical scope")
			}
			drive := Drive{Slug: tc.slug, Scope: scope}
			for _, target := range []string{tc.historicalPath, "docs/problems/" + tc.slug + "/delivery.md"} {
				if got := Classify(dcfg, target, drive); got != InScope {
					t.Errorf("historical path %s: got %v", target, got)
				}
			}
			for _, target := range []string{
				"docs/plans/unrelated/999-plan.md", "memory/problem/unrelated/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"internal/unrecorded.go", ".pyxis/config.yaml",
			} {
				if got := Classify(dcfg, target, drive); got != OutOfScope {
					t.Errorf("unowned path %s: got %v", target, got)
				}
			}
			for _, target := range []string{dcfg.ProductVision, ".pyxis/skills/argus/authority-contract.md", "docs/problems/unrelated/delivery.md"} {
				if got := Classify(dcfg, target, drive); !got.Protected() {
					t.Errorf("protected path %s: got %v", target, got)
				}
			}
			for _, target := range []string{
				"docs/plans/" + tc.slug + "/999-plan.md",
				"docs/plans/" + tc.slug + "/999-plan.json",
				"memory/problem/" + tc.slug + "/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"memory/problem/" + tc.slug + "/Document-01M4767N12EEK6YZWRM29DF0RP.md",
			} {
				if granted := scope.Owns(target); granted != tc.newGrammar {
					t.Errorf("new grammar for %s: got %t, want %t", target, granted, tc.newGrammar)
				}
			}
		})
	}
}
