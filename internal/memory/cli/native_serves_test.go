package cli

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestNativeServesWaitsForTrustedEvaluator(t *testing.T) {
	for _, baseGenerator := range []string{"memory-reconcile/v5", "memory-reconcile/v6", reconcile.Generator} {
		t.Run(baseGenerator, func(t *testing.T) {
			h := initRepo(t)
			h.ok("init")
			h.commit("initialize memory")
			baseline := h.ok("reconcile", "--baseline-generate")
			writeFixtureFile(t, h.dir, reconciliationBaselinePath, strings.Replace(baseline, reconcile.Generator, baseGenerator, 1), 0o644)
			h.commit("trusted evaluator")
			base := h.git("rev-parse", "HEAD")
			h.git("checkout", "-qb", "topic")
			claim := memoryTestRecord(record.Promise, "prd/widget", "FR-1", "A native requirement.", 60)
			decision := memoryTestRecord(record.Decision, "design/widget", "D-1", "Serve the requirement.", 61, record.Relation{Type: record.Serves, Target: claim.ID})
			putMemoryRecord(t, h.dir, claim)
			putMemoryRecord(t, h.dir, decision)
			writeFixtureFile(t, h.dir, reconciliationBaselinePath, baseline, 0o644)
			h.commit("native requirement and decision")
			env, _, _ := h.env("")
			m, err := open(t.Context(), env)
			if err != nil {
				t.Fatal(err)
			}
			m.reconciliationBase = base
			head := h.git("rev-parse", "HEAD")
			_, report, err := pinnedReconciliation(t.Context(), m, head, reconcile.Generator)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, finding := range report.Findings {
				if finding.Code == "unauthorized-prd-serves" && finding.Scope == decision.ID+" -> "+claim.ID {
					found = true
				}
			}
			if want := baseGenerator == "memory-reconcile/v5"; found != want {
				t.Fatalf("native edge refused = %t, want %t: %#v", found, want, report.Findings)
			}
		})
	}
}
