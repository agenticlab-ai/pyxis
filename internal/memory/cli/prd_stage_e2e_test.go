package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
)

func TestE2ERequirementsFirstGateThenDesignCoverage(t *testing.T) {
	t.Parallel()
	fixture := initRepo(t)
	seedReconciliationFixture(t, fixture)
	h, base := memoryGateFixture(t)
	prd := strings.Replace(string(mustRead(t, filepath.Join(fixture.dir, "docs/prds/widget/000-base.md"))),
		"| **Template version**", "| **Design coverage** | requirements-first/v1 |\n| **Template version**", 1)
	result := fold.Fold([]ledger.Entry{{Name: "000-base.md", Bytes: []byte(prd)}})
	if len(result.Defects) != 0 {
		t.Fatal(result.Defects)
	}
	for name, body := range map[string]string{"000-base.md": prd, "effective.md": string(result.EffectiveMD), "effective.source.json": string(result.SourceJSON)} {
		writeFixtureFile(t, h.dir, "docs/prds/widget/"+name, body, 0o644)
	}
	h.commit("introduce requirements without designs")
	out := h.ok("reconcile", "--write", "--actor", "agent:staging", "--json")
	report := decodeReconciliationRun(t, out).Report
	if len(report.DeferredDesignFeatures) != 1 || report.DeferredDesignFeatures[0] != "widget" || report.Blocking() {
		t.Fatalf("requirements stage = %+v", report)
	}
	h.commit("capture requirements")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("refresh exact input identities without new allowances")
	h.gateOK("--base", base)
	cfg, defects := config.Load(h.dir)
	if len(defects) != 0 {
		t.Fatal(defects)
	}
	if defects := gate.CheckStandaloneDocs(h.dir, base, "HEAD", cfg); len(defects) != 0 {
		t.Fatal(defects)
	}
	if code, _, _ := h.run("closeout", "--feature", "widget", "--check"); code == exitOK {
		t.Fatal("requirements-only feature was accepted for closeout")
	}
	requirementsHead := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "code-without-memory")
	writeFixtureFile(t, h.dir, "internal/widget/widget.go", "package widget\nconst Ready = true\n", 0o644)
	h.commit("unreconciled implementation")
	if code, _, _ := h.gate("--base", requirementsHead); code == exitOK {
		t.Fatal("staging waived new code obligations")
	}
	h.git("checkout", "topic")
	for _, name := range []string{"widget-v1.md", "index.md", "index.json"} {
		body := mustRead(t, filepath.Join(fixture.dir, "docs/tech-design", name))
		writeFixtureFile(t, h.dir, "docs/tech-design/"+name, string(body), 0o644)
	}
	h.commit("introduce design in its own PR")
	if defects := gate.CheckStandaloneDocs(h.dir, requirementsHead, "HEAD", cfg); len(defects) != 0 {
		t.Fatal(defects)
	}
	_, out, _ = h.run("reconcile", "--check", "--json")
	report = decodeReconciliationRun(t, out).Report
	if len(report.DeferredDesignFeatures) != 0 || report.Completion == nil || len(report.Completion.Findings) == 0 || !hasFinding(report.Findings, "missing-design-memory") {
		t.Fatalf("design did not retain unfinished coverage and native integrity checks: %+v", report)
	}
	for range 2 {
		_, out, _ = h.run("reconcile", "--write", "--actor", "agent:staging", "--json")
	}
	if report := decodeReconciliationRun(t, out).Report; report.Blocking() {
		t.Fatalf("design capture: %+v", report)
	}
	h.commit("capture design decisions and exact coverage")
	writeFixtureFile(t, h.dir, reconciliationBaselinePath, h.ok("reconcile", "--baseline-generate"), 0o644)
	h.commit("refresh design input identities")
	h.gateOK("--base", requirementsHead)
}
