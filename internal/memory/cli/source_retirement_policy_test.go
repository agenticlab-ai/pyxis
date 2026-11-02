package cli

import (
	"bytes"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
)

func TestStandaloneRetirementPolicyUpgradePreservesIntegrityInventory(t *testing.T) {
	t.Parallel()
	h := standaloneRetirementFixture(t)
	h.git("checkout", "main")
	files := policyFilesOnDisk(t, h)
	legacy := bytes.Replace(files[reconcile.PolicyPath], []byte(reconcile.Generator), []byte("memory-reconcile/v4"), 1)
	writeFixtureFile(t, h.dir, reconcile.PolicyPath, string(legacy), 0644)
	h.commit("retain exact pre-retirement evaluator policy")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "policy-upgrade")
	h.git("commit", "--allow-empty", "-m", "candidate requires current evaluator")
	if code, _, stderr := h.run("reconcile", "--check", "--baseline", reconcile.PolicyPath, "--baseline-ref", base); code == exitOK || !bytes.Contains([]byte(stderr), []byte("candidate enforcement requires")) {
		t.Fatalf("candidate selected historical enforcement: %d %s", code, stderr)
	}
	var out policySyncOutput
	decodeJSON(t, h.ok("reconcile", "--baseline-sync", "--baseline-ref", base, "--json"), &out)
	if out.Status != "complete" || len(out.Planned) != 1 || out.Planned[0].Path != reconcile.PolicyPath {
		t.Fatalf("upgrade changed allowance members: %+v", out.Planned)
	}
	for name, body := range policyFilesOnDisk(t, h) {
		if !bytes.Equal(body, files[name]) {
			t.Fatalf("upgrade did not restore the exact current policy at %s", name)
		}
	}
	h.commit("adopt retirement evaluator without changing debt")
	h.gateOK("--base", base)
}
