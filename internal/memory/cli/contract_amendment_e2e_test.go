package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestE2EAmendedContractReopensFrozenDesignUntilSuccessorLands(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	seedReconciliationFixture(t, h)
	alignReconciliationFixture(t, h)
	h.commit("complete original native contract")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0644)
	h.commit("exact baseline")
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", "HEAD", "--migrate-v2")
	h.commit("partitioned current policy")
	base := h.git("rev-parse", "HEAD")
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	old := m.proj.Head("prd/widget", "FR-1").ID
	h.git("checkout", "-qb", "amendment")
	baseBody := mustRead(t, filepath.Join(h.dir, "docs/prds/widget/000-base.md"))
	amendment := []byte(`# Amendment 001 — replace widget contract

| Field | Value |
|---|---|
| **Amends** | widget (chain: 000 only) |
| **Status** | Approved |
| **Owner** | @owner |
| **Date** | 2026-09-18 |
| **Learned-from** | operator-decision:review |
| **Blast radius** | tier: contract-changing; touches: FR-1; reason: retire readiness and introduce status |
| **Template version** | amendment-template/v1 |

## Operations

### op: retire FR-1

Reason: The approved replacement contract reports status.

### op: add FR

Reason: The replacement status requires a serving design in the next stage.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-2 | When enabled, the widget shall report its local status. | expose current status | Must | med | Enabled returns local status | Test | G1 |
`)
	projection := fold.Fold([]ledger.Entry{{Name: "000-base.md", Bytes: baseBody}, {Name: "001-replace-contract.md", Bytes: amendment}})
	if len(projection.Defects) != 0 {
		t.Fatal(projection.Defects)
	}
	for name, body := range map[string][]byte{"001-replace-contract.md": amendment, "effective.md": projection.EffectiveMD, "effective.source.json": projection.SourceJSON} {
		writeFixtureFile(t, h.dir, "docs/prds/widget/"+name, string(body), 0644)
	}
	// Regenerate the index's declared-target status; the frozen design source
	// itself remains byte-identical throughout the contract PR.
	amendedDesign, defects := techdesigns.Parse("widget-v1", mustRead(t, filepath.Join(h.dir, "docs/tech-design/widget-v1.md")))
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	amendedMD, amendedJSON, defects := techdesigns.GenIndex([]*techdesigns.Design{amendedDesign}, techdesigns.Env{Grandfathered: map[string]bool{}, PrecedenceGrandfathered: map[string]bool{}, PRDUnits: map[string]map[string]string{"widget": {"FR-1": "retired", "FR-2": "live"}}, CorpusIDs: map[string]bool{}, NewDesigns: map[string]bool{}, DocExists: func(string) bool { return true }})
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	writeFixtureFile(t, h.dir, "docs/tech-design/index.md", string(amendedMD), 0644)
	writeFixtureFile(t, h.dir, "docs/tech-design/index.json", string(amendedJSON), 0644)
	h.commit("approved contract amendment precedes replacement design")
	h.ok("revoke", old, "--reason", "The approved amendment retires FR-1")
	run := decodeReconciliationRun(t, h.ok("reconcile", "--write", "--json"))
	if run.Report.Blocking() || run.Report.Counts.Completion != 2 {
		t.Fatalf("contract stage: %+v", run.Report)
	}
	h.commit("capture exact amendment and native transitions")
	h.gateOK("--base", base)
	h.git("checkout", "main")
	h.git("merge", "--squash", "amendment")
	h.commit("land amendment alone")
	h.git("branch", "-D", "amendment")
	if code, _, stderr := h.run("closeout", "--feature", "widget", "--check"); code == exitOK || !strings.Contains(stderr, "completion") {
		t.Fatalf("incomplete closeout: %d %s", code, stderr)
	}
	cloneDir := filepath.Join(t.TempDir(), "amended-transport")
	h.git("clone", "--no-local", "file://"+h.dir, cloneDir)
	clone := newHarness(t, cloneDir)
	var verified verificationOutput
	decodeJSON(t, clone.ok("verify", "--offline", "--json"), &verified)
	if verified.Report.Counts.Completion != 2 || verified.Report.Blocking() {
		t.Fatalf("amended clone: %+v", verified.Report)
	}
	base = h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "successor")
	oldDesign := mustRead(t, filepath.Join(h.dir, "docs/tech-design/widget-v1.md"))
	newDesign := strings.ReplaceAll(string(oldDesign), "| **Supersedes** | — |", "| **Supersedes** | widget-v1 |")
	newDesign = strings.ReplaceAll(newDesign, "prd:widget/FR-1", "prd:widget/FR-2")
	newDesign = strings.ReplaceAll(newDesign, "readiness", "local status")
	d1, defects := techdesigns.Parse("widget-v1", oldDesign)
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	d2, defects := techdesigns.Parse("widget-v2", []byte(newDesign))
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	indexMD, indexJSON, defects := techdesigns.GenIndex([]*techdesigns.Design{d1, d2}, techdesigns.Env{Grandfathered: map[string]bool{}, PrecedenceGrandfathered: map[string]bool{}, PRDUnits: map[string]map[string]string{"widget": {"FR-1": "retired", "FR-2": "live"}}, CorpusIDs: map[string]bool{}, NewDesigns: map[string]bool{}, DocExists: func(string) bool { return true }})
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	writeFixtureFile(t, h.dir, "docs/tech-design/widget-v2.md", newDesign, 0644)
	writeFixtureFile(t, h.dir, "docs/tech-design/index.md", string(indexMD), 0644)
	writeFixtureFile(t, h.dir, "docs/tech-design/index.json", string(indexJSON), 0644)
	h.commit("approved successor design")
	env, _, _ = h.env("")
	m, err = open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range m.proj.Active("design/widget-v1") {
		if decision.Kind == record.Decision {
			h.ok("revoke", decision.ID, "--reason", "The approved successor retires this predecessor decision")
		}
	}
	for range 2 {
		h.run("reconcile", "--write")
	}
	run = decodeReconciliationRun(t, h.ok("reconcile", "--check", "--json"))
	if run.Report.Counts.Completion != 0 {
		t.Fatalf("successor retained completion: %+v", run.Report)
	}
	h.commit("capture exact successor decisions")
	h.gateOK("--base", base)
	h.git("checkout", "main")
	h.git("merge", "--squash", "successor")
	h.commit("land successor alone")
	h.git("branch", "-D", "successor")
	finalDir := filepath.Join(t.TempDir(), "designed-transport")
	h.git("clone", "--no-local", "file://"+h.dir, finalDir)
	final := newHarness(t, finalDir)
	final.gateOK()
	decodeJSON(t, final.ok("verify", "--offline", "--json"), &verified)
	if verified.Report.Blocking() || verified.Report.Counts.Completion != 0 {
		t.Fatalf("designed clone: %+v", verified.Report)
	}
}
