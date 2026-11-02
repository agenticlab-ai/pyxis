package attribution

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/argus/journal"
	"github.com/agenticlab-ai/pyxis/internal/argus/writehook"
)

const correctionLedger = `# Widget — delivery ledger

| Field | Value |
|---|---|
| **Slug** | widget |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | execution |
| **Open items** | T1 |
| **Park state** | none |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — finding-disposition — recorded review feedback

- **Date**: 2026-10-08
- **Principal**: Argus
- **Authority-contract version**: authority-contract/v1
- **Body**: Recorded the review disposition without inventing an authority grant.
`

func TestBothConsumersRequireCorrectedCommittedLedger(t *testing.T) {
	r := newRepo(t)
	ledgerPath := "docs/problems/widget/delivery.md"
	capturePath := "memory/problem/widget/Document-01M4ER8VM1JYXTMA2FSBDQQE3X.json"
	r.write(ledgerPath, correctionLedger)
	base := r.commit("historically accepted ledger")
	meta := journal.Metadata{Date: "2026-10-08", Principal: "Argus", AuthorityVersion: "authority-contract/v1", Reason: "Correct the original type; preserve all review evidence."}
	fixed, err := journal.RenderCorrections(dcfg, "widget", []byte(correctionLedger), []journal.CorrectionRequest{{Target: "E-1", To: "reconciliation"}}, meta)
	if err != nil {
		t.Fatal(err)
	}
	r.write(ledgerPath, string(fixed))
	corrected := r.commitArgus("append historical correction", "widget")
	if refusals := Evaluate(r.dir, base, dcfg); len(refusals) != 0 {
		t.Fatalf("existing same-problem scope cannot land its correction: %v", refusals)
	}
	r.write(capturePath, "{}\n") // Scope test only; record admission is separate.
	r.commitArgus("capture corrected source", "widget")
	if refusals := Evaluate(r.dir, base, dcfg); len(refusals) != 1 || refusals[0].File != capturePath {
		t.Fatalf("candidate correction conferred capture scope: %v", refusals)
	}
	if refusals := Evaluate(r.dir, corrected, dcfg); len(refusals) != 0 {
		t.Fatalf("corrected committed input did not restore ordinary capture scope: %v", refusals)
	}
	for _, tc := range []struct {
		bytes []byte
		allow bool
	}{{[]byte(correctionLedger), false}, {fixed, true}} {
		scope, ok := boundary.ResolveCommittedLedger(dcfg, "widget", tc.bytes)
		if !ok {
			t.Fatal("ledger did not parse")
		}
		drive := boundary.Drive{Slug: "widget", Scope: scope}
		if decision := writehook.Decide(dcfg, capturePath, drive); decision.Allow != tc.allow {
			t.Fatalf("write-hook disagreed with trusted-input attribution: %+v", decision)
		}
		for _, target := range []string{authorityContract, governanceDocument, outOfScopeCode, "memory/problem/other/Document-01M4ER8VM1JYXTMA2FSBDQQE3X.json"} {
			if decision := writehook.Decide(dcfg, target, drive); decision.Allow {
				t.Fatalf("correction bypassed another boundary: %s", target)
			}
		}
	}
}
