package cli

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestContractHistoryRequiresCausalRetainedBase(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"configured mainline", "missing mainline", "missing ref", "unrelated ref", "malformed base config"} {
		t.Run(mode, func(t *testing.T) {
			h := initRepo(t)
			h.ok("init")
			h.commit("memory")
			ref := ""
			switch mode {
			case "missing ref":
				ref = "nonexistent"
			case "unrelated ref":
				head := h.git("rev-parse", "HEAD")
				h.git("checkout", "--orphan", "unrelated")
				h.git("rm", "-rf", ".")
				writeFixtureFile(t, h.dir, "other", "unrelated", 0644)
				h.commit("unrelated root")
				ref = h.git("rev-parse", "HEAD")
				h.git("checkout", "--detach", head)
			case "malformed base config":
				writeFixtureFile(t, h.dir, ".pyxis/config.yaml", "unknown-key: true\n", 0644)
				h.commit("malformed trusted input")
				ref = h.git("rev-parse", "HEAD")
				h.git("rm", ".pyxis/config.yaml")
				h.commit("restore candidate config")
			}
			env, _, _ := h.env("")
			m, err := open(t.Context(), env)
			if err != nil {
				t.Fatal(err)
			}
			m.reconciliationBase = ref
			if mode == "missing mainline" {
				m.branch = "absent"
			}
			s := reconcile.Snapshot{Commit: h.git("rev-parse", "HEAD"), Designs: []reconcile.Design{{Status: "live", Serves: []string{"prd:widget/FR-1"}}}, PRDs: []reconcile.PRD{{Feature: "widget", Units: []reconcile.PRDUnit{{ID: "FR-1", Mapped: true, Status: "retired"}}}}}
			got, err := contractHistorySnapshot(t.Context(), m, s)
			switch mode {
			case "configured mainline":
				if err != nil || got.ContractHistory == nil {
					t.Fatalf("history missing: %v", err)
				}
			case "missing mainline":
				if err != nil || got.ContractHistory != nil {
					t.Fatalf("missing proof fabricated: %v", err)
				}
			default:
				if err == nil {
					t.Fatal("unverifiable predecessor accepted")
				}
			}
		})
	}
}
