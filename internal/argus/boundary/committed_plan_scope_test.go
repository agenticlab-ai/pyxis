package boundary

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCommittedProblemCanAuthorItsPlanAndNativeProblemCaptures(t *testing.T) {
	for _, root := range []string{"docs/plans", "roadmaps", "working plans", "roadmaps[2026]", "plans?", "plan*tree", ".", "./", "roadmaps/.."} {
		t.Run(root, func(t *testing.T) {
			cfg := config.Defaults()
			cfg.Plans = root
			scope, ok := ResolveCommittedLedger(cfg, "widget", []byte(committedLedger))
			if !ok {
				t.Fatal("committed problem did not resolve")
			}
			drive := Drive{Slug: "widget", Scope: scope}
			for _, target := range []string{
				root + "/widget/001-plan.md", root + "/widget/001-plan.json",
				root + "/widget/002-plan.md",
				"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md",
			} {
				if got := Classify(cfg, target, drive); got != InScope {
					t.Errorf("same-problem stage artifact %s: got %v", target, got)
				}
			}
			for _, target := range []string{
				root + "/other/001-plan.md", root + "/widget-other/001-plan.md",
				root + "/widget/code.py", root + "/widget/001-plan.md/child",
				root + "/widget/../other/001-plan.md", root + "/widget/nested/001-plan.md",
				"memory/problem/other/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/problem/widget-other/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"memory/problem/widget/../other/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/source/pyxis-config/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"aec_memory/store.py", ".pyxis/config.yaml",
				"memory/problem/widget/code.py", "memory/problem/widget/notes.md",
				"memory/problem/widget/nested/Document-01M4767N12EEK6YZWRM29DF0RP.json",
				"memory/problem/widget/Decision-01M4767N12EEK6YZWRM29DF0RP.json",
				"memory/problem/widget/Document-short.json",
				"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.go",
				"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json/child",
				"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RPI.json",
			} {
				if got := Classify(cfg, target, drive); got != OutOfScope {
					t.Errorf("unowned target %s: got %v", target, got)
				}
			}
			for _, target := range []string{".pyxis/skills/argus/authority-contract.md", cfg.ProductVision} {
				if got := Classify(cfg, target, drive); !got.Protected() {
					t.Errorf("protected target %s: got %v", target, got)
				}
			}
			if root != "docs/plans" && scope.Owns("docs/plans/widget/001-plan.md") {
				t.Fatal("relocated plans still admitted the default root")
			}
			for _, otherRoot := range []string{"roadmaps2", "plansx", "plan-any-tree"} {
				if scope.Owns(otherRoot + "/widget/001-plan.md") {
					t.Errorf("literal root %s admitted a glob match under %s", root, otherRoot)
				}
			}
		})
	}
}

func TestMalformedCommittedDeliveryDoesNotGrantNewScope(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement string }{
		{"missing slug", "| **Slug** | widget |\n", ""},
		{"mismatched slug", "| **Slug** | widget |", "| **Slug** | other |"},
		{"duplicate slug", "| **Slug** | widget |", "| **Slug** | widget |\n| **Slug** | other |"},
		{"unknown header field", "| **Slug** | widget |", "| **Slug** | widget |\n| **Status** | open |"},
		{"malformed header row", "| **Slug** | widget |", "| Slug | widget |"},
		{"wrong version substring", "delivery-ledger/v1", "delivery-ledger/v10"},
		{"duplicate version", "| **Template version** | delivery-ledger/v1 |", "| **Template version** | delivery-ledger/v1 |\n| **Template version** | delivery-ledger/v2 |"},
		{"empty derived state", "| **Status** | open |\n| **Stage position** | prd-new |\n| **Open items** | — |\n| **Park state** | — |\n| **Cumulative spend** | unavailable |", ""},
		{"missing derived field", "| **Open items** | — |\n", ""},
		{"unknown derived field", "| **Open items** | — |", "| **Extra field** | — |"},
		{"extra derived column", "| **Open items** | — |", "| **Open items** | — | extra |"},
		{"empty derived field", "| **Stage position** | prd-new |", "| **Stage position** | |"},
		{"invalid status", "| **Status** | open |", "| **Status** | approved |"},
		{"duplicate status", "| **Status** | open |", "| **Status** | open |\n| **Status** | parked |"},
		{"duplicate derived region", "## Journal", "## Derived state\n\n## Journal"},
		{"duplicate journal", "## Journal", "## Journal\n\n## Journal"},
		{"unknown region", "## Journal", "## Extra region\n\n## Journal"},
		{"slug after header", "| **Slug** | widget |\n", ""},
		{"fenced slug", "| **Slug** | widget |", "~~~markdown\n| **Slug** | widget |\n~~~"},
		{"fenced derived field", "| **Open items** | — |", "~~~markdown\n| **Open items** | — |\n~~~"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := strings.Replace(committedLedger, tc.old, tc.replacement, 1)
			if tc.name == "slug after header" {
				ledger += "\n| **Slug** | widget |\n"
			}
			scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
			assertNoNewDeliveryScope(t, scope)
		})
	}
}

func assertNoNewDeliveryScope(t *testing.T, scope *Scope) {
	t.Helper()
	if scope == nil {
		return
	}
	for _, target := range []string{
		"docs/plans/widget/999-plan.md", "docs/plans/widget/999-plan.json",
		"memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.json", "memory/problem/widget/Document-01M4767N12EEK6YZWRM29DF0RP.md",
	} {
		if scope.Owns(target) {
			t.Errorf("invalid committed delivery granted new scope for %s", target)
		}
	}
}

func TestCommittedDeliveryShapeRequiresTableFrames(t *testing.T) {
	const heading = "| Field | Value |\n"
	const separator = "|---|---|\n"
	for _, tc := range []struct{ name, ledger string }{
		{"missing headings", strings.ReplaceAll(committedLedger, heading, "")},
		{"missing separators", strings.ReplaceAll(committedLedger, separator, "")},
		{"missing both", strings.ReplaceAll(strings.ReplaceAll(committedLedger, heading, ""), separator, "")},
		{"duplicate heading", strings.Replace(committedLedger, heading, heading+heading, 1)},
		{"duplicate separator", strings.Replace(committedLedger, separator, separator+separator, 1)},
		{"reversed frames", strings.ReplaceAll(committedLedger, heading+separator, separator+heading)},
		{"missing derived heading", strings.Replace(committedLedger, "## Derived state\n\n"+heading, "## Derived state\n\n", 1)},
		{"field before separator", strings.Replace(committedLedger, separator+"| **Slug** | widget |\n", "| **Slug** | widget |\n"+separator, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(tc.ledger))
			assertNoNewDeliveryScope(t, scope)
		})
	}
}

func TestCommittedDeliveryShapeRequiresRegionOrder(t *testing.T) {
	derivedAt := strings.Index(committedLedger, "## Derived state")
	journalAt := strings.Index(committedLedger, "## Journal")
	ledger := committedLedger[:derivedAt] + committedLedger[journalAt:] + "\n" + committedLedger[derivedAt:journalAt]
	scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
	if !ok || !scope.Owns("docs/prds/widget/000-base.md") {
		t.Fatal("reordered legacy ledger lost historical ownership")
	}
	assertNoNewDeliveryScope(t, scope)
}

func TestCommittedDeliveryShapeIgnoresFencedExamples(t *testing.T) {
	ledger := strings.Replace(committedLedger, "## Derived state", "~~~markdown\n| **Slug** | other |\n| **Template version** | delivery-ledger/v2 |\n~~~\n\n## Derived state", 1)
	ledger = strings.Replace(ledger, "## Journal", "~~~markdown\n| **Status** | invalid |\n## Derived state\n~~~\n\n## Journal", 1)
	if scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger)); !ok || !scope.Owns("docs/plans/widget/001-plan.md") {
		t.Fatal("fenced examples invalidated the live delivery shape")
	}
	// The derived block remains descriptive: its values do not grant or deny
	// permissions by claiming a particular journal state.
	for _, status := range []string{"parked", "delivered"} {
		ledger := strings.Replace(committedLedger, "| **Status** | open |", "| **Status** | "+status+" |", 1)
		if _, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger)); !ok {
			t.Fatalf("closed status %s was rejected", status)
		}
	}
}

func TestFirstDocumentationProposalDoesNotGrantPlanScope(t *testing.T) {
	cfg := config.Defaults()
	if ResolveFirstPR(cfg, "widget").Owns("docs/plans/widget/001-plan.md") {
		t.Fatal("first proposal bypassed the committed-problem stage transition")
	}
	if scope, ok := ResolveCommittedLedger(cfg, "widget", []byte("not a delivery ledger")); ok || scope != nil {
		t.Fatal("unparseable committed input granted stage scope")
	}
}
