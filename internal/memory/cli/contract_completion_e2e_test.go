package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/contractgate"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

func TestE2EStagedContractMigratesV3PolicyWithoutGrantingDebt(t *testing.T) {
	t.Parallel()
	fixture := initRepo(t)
	seedReconciliationFixture(t, fixture)
	h, _ := memoryGateFixture(t)
	for _, name := range []string{"000-base.md", "effective.md", "effective.source.json"} {
		writeFixtureFile(t, h.dir, "docs/prds/widget/"+name, string(mustRead(t, filepath.Join(fixture.dir, "docs/prds/widget", name))), 0o644)
	}
	h.commit("unmarked requirements precede design")
	run := decodeReconciliationRun(t, h.ok("reconcile", "--write", "--json"))
	if run.Report.Blocking() || run.Report.Counts.Completion != 1 {
		t.Fatalf("exact unmarked contract not staged: %+v", run.Report)
	}
	h.commit("capture exact requirements")
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := reconciliationSnapshot(t.Context(), m)
	if err != nil {
		t.Fatal(err)
	}
	legacy := reconcile.Analyze(snap, m.set, resolve.New(m.set), reconcile.Options{LegacyContractCoverage: true})
	if legacy.Counts.Findings != 1 || legacy.Findings[0].Code != "unserved-prd-memory" {
		t.Fatalf("bad v3 fixture: %+v", legacy)
	}
	oldFiles, err := reconcile.MarshalPolicySet(reconcile.NewPolicySet(legacy))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range oldFiles {
		writeFixtureFile(t, h.dir, name, string(body), 0o644)
	}
	h.commit("retain an exact historical v3 partitioned policy")
	base := h.git("rev-parse", "HEAD")
	h.git("branch", "-f", "main", base)
	h.git("commit", "--allow-empty", "-m", "admit under current classification")
	if code, _, stderr := h.run("reconcile", "--check", "--baseline", reconciliationBaselinePath, "--baseline-ref", base); code == exitOK || !strings.Contains(stderr, "historical") {
		t.Fatalf("candidate selected v3 enforcement: %d %s", code, stderr)
	}
	out := h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--json")
	var sync policySyncOutput
	decodeJSON(t, out, &sync)
	if sync.Report.Counts.Findings != 0 || sync.Report.Counts.Completion != 1 || sync.Policy.Generator != reconcile.Generator || len(sync.Planned) != 2 {
		t.Fatalf("migration changed integrity grants: %+v", sync)
	}
	h.commit("remove the completion allowance and record evaluator v4")
	h.gateOK("--base", base)
	if code, _, _ := h.run("closeout", "--feature", "widget", "--check"); code == exitOK {
		t.Fatal("undesigned feature closed")
	}
	if verdict := contractgate.Run(h.dir, "widget"); !verdict.Blocked() {
		t.Fatalf("planning accepted absent design: %+v", verdict)
	}
	h.git("checkout", "main")
	h.git("merge", "--squash", "topic")
	h.commit("land classification after squash")
	h.git("branch", "-D", "topic")
	cloneDir := filepath.Join(t.TempDir(), "transport-clone")
	h.git("clone", "--no-local", "file://"+h.dir, cloneDir)
	clone := newHarness(t, cloneDir)
	clone.gateOK()
	var verified verificationOutput
	decodeJSON(t, clone.ok("verify", "--offline", "--json"), &verified)
	if verified.Report.Counts.Completion != 1 || len(verified.Closeouts) != 0 {
		t.Fatalf("clone certified completion: %+v", verified)
	}
}
