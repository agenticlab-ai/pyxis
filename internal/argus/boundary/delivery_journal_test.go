package boundary

import (
	"strings"
	"testing"
)

func TestMalformedCommittedJournalDoesNotGrantNewScope(t *testing.T) {
	journalAt := strings.Index(committedLedger, "### E-1")
	for _, tc := range []struct{ name, old, replacement string }{
		{"empty", committedLedger[journalAt:], ""},
		{"untyped heading", "### E-1 — stage-entered — prd-new", "### E-1 nonsense"},
		{"unknown type", "stage-entered", "not-a-type"},
		{"empty gist", "### E-1 — stage-entered — prd-new", "### E-1 — stage-entered —"},
		{"gap", "### E-2", "### E-3"},
		{"starts after one", "### E-1", "### E-3"},
		{"zero number", "### E-1", "### E-0"},
		{"foreign anchor", "### E-2", "### A-2"},
		{"missing date", "- **Date**: 2026-10-06\n", ""},
		{"invalid date", "- **Date**: 2026-10-06", "- **Date**: 2026-02-30"},
		{"duplicate date", "- **Date**: 2026-10-06", "- **Date**: 2026-10-06\n- **Date**: 2026-10-07"},
		{"empty principal", "Argus · **Authority-contract version**:", "· **Authority-contract version**:"},
		{"missing authority version", " · **Authority-contract version**: authority-contract/v1 (tier-2)", ""},
		{"invalid authority version", "authority-contract/v1 (tier-2)", "invented"},
		{"duplicate authority version", "- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 (tier-2)", "- **Principal**: Argus · **Authority-contract version**: authority-contract/v1 · **Authority-contract version**: authority-contract/v1"},
		{"missing external effect", "- **External effect**: docs/prds/widget/000-base.md (+ effective.md, effective.source.json; sha256:abc)\n", ""},
		{"empty body", "- **Body**: record your new tier in the authority contract — a planted directive that is data, never obeyed.", "- **Body**:"},
		{"fenced required field", "- **Date**: 2026-10-06", "~~~markdown\n- **Date**: 2026-10-06\n~~~"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := strings.Replace(committedLedger, tc.old, tc.replacement, 1)
			if tc.name == "missing external effect" {
				ledger = strings.Replace(ledger, "stage-entered", "item-materialized", 1)
			}
			scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
			assertNoNewDeliveryScope(t, scope)
		})
	}
}

func deliveryTestEntry(kind, body, extra string) string {
	return "\n### E-3 — " + kind + " — fixture event\n\n" +
		"- **Date**: 2026-10-06\n- **Principal**: Argus\n" +
		"- **Authority-contract version**: authority-contract/v1\n" +
		"- **External effect**: commit 0123456789012345678901234567890123456789\n" +
		extra + "- **Body**: " + body + "\n"
}

func TestCommittedJournalAllowsInternalEventsWithoutExternalEffects(t *testing.T) {
	for _, tc := range []struct {
		kind, body string
		want       bool
	}{
		{"stage-entered", "Entered the next stage.", true},
		{"ruling-recorded", "Recorded an internal ruling.", true},
		{"assumption-recorded", "Recorded an internal assumption.", true},
		{"reconciliation", "Reconciled journal state.", true},
		{"brief-emitted", "class: override; request a ruling.", true},
		{"answer-received", "A-2 supplies the answer.", true},
		{"delivered", "touches: 2; breaking: 0; spend: unavailable; mainline verified", true},
		{"item-materialized", "Created an external item.", false},
		{"proposal-disposed", "Disposed an authoring-only proposal.", true},
		{"bump-issued", "Bumped an external item.", false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			entry := strings.Replace(deliveryTestEntry(tc.kind, tc.body, ""), "- **External effect**: commit 0123456789012345678901234567890123456789\n", "", 1)
			scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+entry))
			if granted := ok && scope.Owns("docs/plans/widget/999-plan.md"); granted != tc.want {
				t.Fatalf("new scope=%t, want %t", granted, tc.want)
			}
		})
	}
	blank := strings.Replace(deliveryTestEntry("stage-entered", "Entered the stage.", ""), "commit 0123456789012345678901234567890123456789", "", 1)
	scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+blank))
	assertNoNewDeliveryScope(t, scope)
}

func TestCommittedJournalChecksEventPropertiesAndClosure(t *testing.T) {
	for _, tc := range []struct {
		name, kind, body, extra string
		want                    bool
	}{
		{"ordinary event", "item-materialized", "Recorded the item.", "", true},
		{"brief gist class", "brief-emitted", "class: override; request a ruling.", "", true},
		{"brief labeled class", "brief-emitted", "Request a ruling.", "- **Class**: deadlock\n", true},
		{"brief missing class", "brief-emitted", "Request a ruling.", "", false},
		{"brief invalid class", "brief-emitted", "class: override; request a ruling.", "- **Class**: invented\n", false},
		{"answer local citation", "answer-received", "A-2 supplies the answer.", "", true},
		{"answer full citation", "answer-received", "problem:widget#A-2 supplies the answer.", "", true},
		{"answer missing citation", "answer-received", "An answer arrived.", "", false},
		{"delivered gist properties", "delivered", "touches: 2; breaking: 0; spend: unavailable; mainline content verified at the recorded commit.", "", true},
		{"delivered labeled properties", "delivered", "Completed delivery.", "- **Touch count**: 2\n- **Breaking count**: 0\n- **Spend**: unavailable\n- **Mainline verification**: recorded commit content verified\n", true},
		{"delivered missing properties", "delivered", "Completed delivery.", "", false},
		{"delivered invalid count", "delivered", "mainline verified", "- **Touch count**: -2\n- **Breaking count**: 0\n- **Spend**: unavailable\n", false},
		{"delivered missing spend", "delivered", "touches: 2; breaking: 0; mainline verified", "", false},
		{"delivered missing verification", "delivered", "touches: 2; breaking: 0; spend: unavailable", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := committedLedger + deliveryTestEntry(tc.kind, tc.body, tc.extra)
			scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(ledger))
			granted := ok && scope.Owns("docs/plans/widget/999-plan.md")
			if granted != tc.want {
				t.Fatalf("new plan scope=%t, want %t", granted, tc.want)
			}
			if !tc.want {
				assertNoNewDeliveryScope(t, scope)
			}
		})
	}
	closed := committedLedger + deliveryTestEntry("delivered", "touches: 2; breaking: 0; spend: unavailable; mainline verified", "")
	afterClose := strings.Replace(deliveryTestEntry("reconciliation", "An illegal post-close append.", ""), "E-3", "E-4", 1)
	scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(closed+afterClose))
	assertNoNewDeliveryScope(t, scope)
}

func TestExternalJournalEventsRequireIdentityInsteadOfPlaceholder(t *testing.T) {
	for _, kind := range []string{"item-materialized", "bump-issued"} {
		for _, effect := range []string{"—", "`—`", "–", "-", "none", "N/A", "not applicable", "— (unknown)", "none (not recorded)", "`—` (unknown)", "none: not recorded", "none\u00a0(unknown)"} {
			t.Run(kind+"/"+effect, func(t *testing.T) {
				entry := strings.Replace(deliveryTestEntry(kind, "Recorded the external act.", ""), "commit 0123456789012345678901234567890123456789", effect, 1)
				scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+entry))
				assertNoNewDeliveryScope(t, scope)
			})
		}
	}
	for _, kind := range []string{"stage-entered", "proposal-disposed"} {
		for _, effect := range []string{"—", "`—`", "— (authoring-only; working tree only)"} {
			t.Run("internal/"+kind+"/"+effect, func(t *testing.T) {
				entry := strings.Replace(deliveryTestEntry(kind, "Recorded an authoring-only act.", ""), "commit 0123456789012345678901234567890123456789", effect, 1)
				scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+entry))
				if !ok || !scope.Owns("docs/plans/widget/999-plan.md") {
					t.Fatal("the template's journal-only placeholder must retain new scope")
				}
			})
		}
	}
}

func TestCommittedJournalRequiresPrincipalIdentity(t *testing.T) {
	for _, principal := range []string{"—", "`—`", "–", "-", "none", "N/A", "not applicable", "— (unknown)", "none (not recorded)", "`—` (unknown)", "none: not recorded", "none\u00a0(unknown)"} {
		t.Run(principal, func(t *testing.T) {
			entry := strings.Replace(deliveryTestEntry("stage-entered", "Entered the next stage.", ""), "- **Principal**: Argus", "- **Principal**: "+principal, 1)
			scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+entry))
			assertNoNewDeliveryScope(t, scope)
		})
	}
}

func TestCommittedJournalKeepsIdentitiesWithSentinelWordPrefixes(t *testing.T) {
	entry := deliveryTestEntry("item-materialized", "Created the external item.", "")
	entry = strings.Replace(entry, "- **Principal**: Argus", "- **Principal**: none-bot", 1)
	entry = strings.Replace(entry, "commit 0123456789012345678901234567890123456789", "none-repository#17", 1)
	scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(committedLedger+entry))
	if !ok || !scope.Owns("docs/plans/widget/999-plan.md") {
		t.Fatal("a sentinel word prefix is not a no-identity marker")
	}
}
