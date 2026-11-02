package boundary

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCommittedDraftScopeRecordsExactConfiguredPaths(t *testing.T) {
	for _, root := range []string{"drafts", "working/drafts", ".local/drafts"} {
		t.Run(root, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.Drafts = "./" + root + "/"
			ledger := committedLedger + strings.ReplaceAll(`
### E-3 — reconciliation — retained reopening

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: ROOT/widget/events.md · ROOT/widget/prd.md (+ tech-spec.md, problems.md)
- **Body**: ROOT/widget/reviews/001.md is not recorded as an effect.
`, "ROOT", root)
			scope, ok := ResolveCommittedLedger(cfg, "widget", []byte(ledger))
			if !ok {
				t.Fatal("valid committed ledger did not resolve")
			}
			drive := Drive{Slug: "widget", Scope: scope}
			for _, target := range []string{
				root + "/widget/events.md", root + "/widget/prd.md",
				root + "/widget/tech-spec.md", root + "/widget/problems.md",
				"docs/prds/widget/000-base.md",
			} {
				if got := Classify(cfg, target, drive); got != InScope {
					t.Errorf("recorded effect %q = %v, want InScope", target, got)
				}
			}
			for _, target := range []string{
				root + "/widget/reviews/001.md", root + "/other/prd.md",
				root + "-other/widget/prd.md", "unconfigured/widget/prd.md",
				"internal/engine/core.go",
			} {
				if got := Classify(cfg, target, drive); got != OutOfScope {
					t.Errorf("unrecorded effect %q = %v, want OutOfScope", target, got)
				}
			}
			if root != "drafts" && scope.Owns("drafts/widget/prd.md") {
				t.Fatal("relocated draft root granted an unrecorded default path")
			}
		})
	}
}

func TestCommittedDraftScopePreservesQuotedWhitespacePaths(t *testing.T) {
	cfg := config.Defaults()
	cfg.Drafts = "working drafts"
	for _, quote := range []string{"`", "\"", "'"} {
		t.Run(quote, func(t *testing.T) {
			ledger := committedLedger + fmt.Sprintf(`
### E-3 — reconciliation — retained reopening

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: (%[1]sworking drafts/widget/events.md%[1]s) · %[1]sworking drafts/widget/prd.md%[1]s (+ %[1]stech-spec.md%[1]s, problems.md)
- **Body**: %[1]sworking drafts/widget/reviews/001.md%[1]s is not recorded as an effect.
`, quote)
			scope, ok := ResolveCommittedLedger(cfg, "widget", []byte(ledger))
			if !ok {
				t.Fatal("valid committed ledger did not resolve")
			}
			drive := Drive{Slug: "widget", Scope: scope}
			for _, target := range []string{
				"working drafts/widget/events.md", "working drafts/widget/prd.md",
				"working drafts/widget/tech-spec.md", "working drafts/widget/problems.md",
				"docs/prds/widget/effective.md",
			} {
				if got := Classify(cfg, target, drive); got != InScope {
					t.Errorf("quoted recorded effect %q = %v, want InScope", target, got)
				}
			}
			for _, target := range []string{
				"working drafts/widget/reviews/001.md", "working drafts/other/prd.md",
				"working drafts-other/widget/prd.md", "internal/engine/core.go",
			} {
				if got := Classify(cfg, target, drive); got != OutOfScope {
					t.Errorf("unrecorded effect %q = %v, want OutOfScope", target, got)
				}
			}
		})
	}
}

func TestCommittedLedgerEffectsKeepUnquotedPossessives(t *testing.T) {
	ledger := committedLedger + `
### E-3 — reconciliation — recorded plan

- **Date**: 2026-10-06
- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)
- **External effect**: operator's commit docs/plans/widget/001-plan.md and reviewer's docs/plans/widget/001-plan.json
- **Body**: Recorded the plan effect.
`
	scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
	if !ok {
		t.Fatal("valid committed ledger did not resolve")
	}
	for _, target := range []string{"docs/plans/widget/001-plan.md", "docs/plans/widget/001-plan.json"} {
		if !scope.Owns(target) {
			t.Errorf("unquoted legacy effect %q was lost between prose apostrophes", target)
		}
	}
}

func TestCommittedLedgerEffectsPreserveLegacyTokenBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, effect   string
		owned, refused []string
	}{
		{"citation suffix", "`internal/engine/core.go`#L42", nil, []string{"internal/engine/core.go"}},
		{"quoted legacy group", "`docs/prds/widget/extra.md (+ folded.md)`", []string{"docs/prds/widget/extra.md", "docs/prds/widget/folded.md"}, nil},
		{"unicode whitespace", "docs/a.md\u00a0docs/b.md\u2003docs/c.md", []string{"docs/a.md", "docs/b.md", "docs/c.md"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := committedLedger + "\n### E-3 — reconciliation — extra effects\n\n- **Date**: 2026-10-06\n- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)\n- **External effect**: " + tc.effect + "\n- **Body**: Recorded the extra effects.\n"
			scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
			if !ok {
				t.Fatal("valid committed ledger did not resolve")
			}
			for _, target := range tc.owned {
				if !scope.Owns(target) {
					t.Errorf("legacy effect %q was lost", target)
				}
			}
			for _, target := range tc.refused {
				if scope.Owns(target) {
					t.Errorf("citation %q widened committed scope", target)
				}
			}
		})
	}
}

func TestCommittedDraftScopeRejectsQuotedSuffixesAndUnclosedQuotes(t *testing.T) {
	cfg := config.Defaults()
	cfg.Drafts = "working drafts"
	for _, effect := range []string{
		"`working drafts/widget/prd.md`#L42", "\"working drafts/widget/prd.md\"suffix",
		"`working drafts/widget/prd.md", "'working drafts/widget/prd.md",
	} {
		ledger := committedLedger + "\n### E-3 — reconciliation — extra effects\n\n- **Date**: 2026-10-06\n- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)\n- **External effect**: " + effect + "\n- **Body**: Recorded the extra effects.\n"
		scope, ok := ResolveCommittedLedger(cfg, "widget", []byte(ledger))
		if !ok || scope.Owns("working drafts/widget/prd.md") {
			t.Errorf("invalid quoted effect %q granted the draft path or invalidated the ledger", effect)
		}
	}
}
