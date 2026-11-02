package boundary

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func correctedFixture(t *testing.T, base string, next int, target string) string {
	t.Helper()
	l, defects := argusledger.ParseAs("test", "docs/problems/widget/delivery.md", schema.LedgerClasses["delivery-ledger"], []byte(base))
	if len(defects) > 0 {
		t.Fatal(defects)
	}
	digest, err := argusledger.EntryDigest(l, target)
	if err != nil {
		t.Fatal(err)
	}
	gist := argusledger.TypeCorrection{Target: "delivery:widget#" + target, SHA256: digest, From: argusledger.EntryType(l.Entries[target]), To: argusledger.TypeReconciliation}
	return base + fmt.Sprintf("\n### E-%d — reconciliation — correct prior classification\n\n- **Date**: 2026-10-08\n- **Principal**: Argus\n- **Authority-contract version**: authority-contract/v1\n- **Body**: %s\n\nThe original event recorded a review disposition.\n", next, gist.Render())
}

func TestHistoricalTypeCorrectionRestoresOnlyCommittedScope(t *testing.T) {
	base := strings.Replace(committedLedger, "stage-entered", "finding-disposition", 1)
	fixed := correctedFixture(t, base, 3, "E-1")
	if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(base), []byte(fixed)); len(defects) != 0 {
		t.Fatal(defects)
	}
	if !bytes.HasPrefix([]byte(fixed), []byte(base)) {
		t.Fatal("historical file was rewritten")
	}
	scope, ok := ResolveCommittedLedger(dcfg, "widget", []byte(base))
	if !ok {
		t.Fatal("legacy problem scope should remain resolvable")
	}
	assertNoNewDeliveryScope(t, scope)
	// A candidate correction itself grants no new scope. Only a trusted-base
	// change to the corrected ledger enables the existing bounded grammars.
	scope, ok = ResolveCommittedLedger(dcfg, "widget", []byte(fixed))
	if !ok || !scope.Owns("docs/plans/widget/001-plan.md") || !scope.Owns("memory/problem/widget/Document-01M4ER8VM1JYXTMA2FSBDQQE3X.json") {
		t.Fatal("valid corrected committed ledger did not restore normal scope")
	}
	for _, file := range []string{"memory/problem/other/Document-01M4ER8VM1JYXTMA2FSBDQQE3X.json", "internal/engine/core.go", "docs/plans/widget/1000-plan.md", ".pyxis/config.yaml"} {
		if scope.Owns(file) {
			t.Fatalf("corrected ledger widened unrelated scope: %s", file)
		}
	}
}

func TestCorrectionsRefuseMalformedBindingsAndOriginalObligations(t *testing.T) {
	base := strings.Replace(committedLedger, "stage-entered", "finding-disposition", 1)
	valid := correctedFixture(t, base, 3, "E-1")
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong digest", "sha256=", "sha256=0"},
		{"wrong from", "from=finding-disposition", "from=other-unknown"},
		{"unknown to", "to=reconciliation", "to=other-unknown"},
		{"unknown key", "; to=reconciliation", "; other=reconciliation"},
		{"duplicate key", "; to=reconciliation", "; from=reconciliation"},
		{"future version", "journal-type-correction/v1", "journal-type-correction/v2"},
		{"cross ledger", "target=delivery:widget#E-1", "target=delivery:other#E-1"},
		{"forward target", "target=delivery:widget#E-1", "target=delivery:widget#E-4"},
		{"self target", "target=delivery:widget#E-1", "target=delivery:widget#E-3"},
		{"canonical target", "target=delivery:widget#E-1", "target=delivery:widget#E-2"},
		{"wrong carrier", "E-3 — reconciliation", "E-3 — ruling-recorded"},
		{"brief lacks class", "to=reconciliation", "to=brief-emitted"},
		{"answer lacks citation", "to=reconciliation", "to=answer-received"},
		{"early delivery", "to=reconciliation", "to=delivered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := strings.Replace(valid, tc.old, tc.replacement, 1)
			if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(base), []byte(bad)); len(defects) == 0 {
				t.Fatal("invalid correction admitted")
			}
			scope, _ := ResolveCommittedLedger(dcfg, "widget", []byte(bad))
			assertNoNewDeliveryScope(t, scope)
		})
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"missing principal", "Argus · **Authority-contract version**:", "· **Authority-contract version**:"},
		{"missing date", "- **Date**: 2026-10-06\n", ""},
		{"invalid authority", "authority-contract/v1 (tier-2)", "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			badBase := strings.Replace(base, tc.old, tc.replacement, 1)
			bad := correctedFixture(t, badBase, 3, "E-1")
			if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(badBase), []byte(bad)); len(defects) == 0 {
				t.Fatal("correction laundered invalid historical metadata")
			}
		})
	}
	duplicate := correctedFixture(t, valid, 4, "E-1")
	if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(base), []byte(duplicate)); len(defects) == 0 {
		t.Fatal("duplicate correction admitted")
	}
}

func TestNewUnknownCannotBeLaunderedBySameProposalCorrection(t *testing.T) {
	fresh := committedLedger + deliveryTestEntry("finding-disposition", "Recorded a disposition.", "")
	fixed := correctedFixture(t, fresh, 4, "E-3")
	if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(committedLedger), []byte(fixed)); len(defects) != 1 || !strings.Contains(defects[0].Reason, "new E-3") {
		t.Fatalf("new unsupported event was laundered: %v", defects)
	}
	base := strings.Replace(committedLedger, "stage-entered", "finding-disposition", 1)
	edited := strings.Replace(base, "- **Date**: 2026-10-06", "- **Date**: 2026-10-06  ", 1)
	fixed = correctedFixture(t, edited, 3, "E-1")
	if defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(base), []byte(fixed)); len(defects) != 1 || !strings.Contains(defects[0].Reason, "exact bytes") {
		t.Fatalf("normalization masked a historical edit: %v", defects)
	}
}

func TestDeliveryChangeRefusesDamagedBaseAndRemovedHistory(t *testing.T) {
	for _, tc := range []struct {
		name, base, candidate, reason string
	}{
		{name: "damaged trusted base", base: "not a delivery ledger", candidate: committedLedger, reason: "ledger"},
		{name: "removed historical event", base: committedLedger, candidate: strings.Split(committedLedger, "### E-2")[0], reason: "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defects := ValidateDeliveryChange("test", dcfg, "widget", []byte(tc.base), []byte(tc.candidate))
			if len(defects) == 0 {
				t.Fatal("unavailable history was admitted as a new journal")
			}
			if tc.name == "removed historical event" && !strings.Contains(defects[0].Reason, tc.reason) {
				t.Fatalf("wrong refusal: %v", defects)
			}
		})
	}
}
