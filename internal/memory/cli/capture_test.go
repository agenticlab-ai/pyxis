package cli

import (
	"strings"
	"testing"
)

func TestCaptureAdmissionAndHistoricalRetrieval(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	before := memoryRecordCount(t, h.dir)
	code, _, errout := h.run("add", "--kind", "Need", "--subject", "code/widget", "--family", "anchor", "--statement", "Inventory widget exists.", "--tag", "reconcile:anchor")
	if code != exitRefused || !strings.Contains(errout, "derived-memory") || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("generated anchor admitted: %d %s", code, errout)
	}
	code, _, errout = h.run("add", "--kind", "Learning", "--subject", "learning/widget", "--statement", "Inventory widget is a package.",
		"--intent", "Preserve the implemented responsibility of internal/widget in durable software memory.",
		"--rationale", "Source-code audit found production code without a dedicated semantic memory record.", "--tag", "reconcile:code-semantics")
	if code != exitRefused || !strings.Contains(errout, "derived-memory") || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("generated summary admitted: %d %s", code, errout)
	}

	// Seed an inherited v6 record directly. Immutable validation and explicit
	// historical reads must continue to accept the old format unchanged.
	old := h.add("--kind", "Need", "--subject", "code/widget", "--family", "anchor", "--statement", "Inventory widget exists.")
	r, _ := readEvidenceRecord(t, h, old)
	r.Tags = []string{"reconcile:anchor"}
	writeEvidenceRecord(t, h, r)
	lesson := h.add("--kind", "Learning", "--subject", "learning/widget", "--statement", "Inventory tests must observe reads rather than elapsed time.", "--rationale", "Host scheduling masked full scans and failed correct indexed reads.")
	if out := h.ok("search", "Inventory"); strings.Contains(out, old) || !strings.Contains(out, lesson) {
		t.Fatalf("ordinary retrieval mixed derived inventory with useful knowledge: %s", out)
	}
	if out := h.ok("search", "Inventory", "--all"); !strings.Contains(out, old) || !strings.Contains(out, lesson) {
		t.Fatalf("explicit history lost records: %s", out)
	}
	if out := h.ok("view", "code/widget"); !strings.Contains(out, "Inventory widget exists.") {
		t.Fatalf("direct historical access lost anchor: %s", out)
	}
}

func TestRetainedBranchCannotResurrectKnownMemory(t *testing.T) {
	h := initRepo(t)
	h.ok("init")
	h.commit("memory base")
	h.git("checkout", "-q", "-b", "feature/lesson")
	old := h.add("--kind", "Learning", "--subject", "learning/widget", "--statement", "Quokka retained report advice.")
	h.commit("lesson")
	// A squash leaves the old branch outside mainline ancestry, although its
	// immutable identity is already present on main and later withdrawn.
	h.git("checkout", "-q", "main")
	h.git("merge", "--squash", "feature/lesson")
	h.commit("squash lesson")
	h.ok("revoke", old, "--reason", "Reports are now reproducible local diagnostics.")
	h.commit("withdraw obsolete advice")
	h.git("checkout", "-q", "-b", "feature/current")
	h.git("commit", "--allow-empty", "-qm", "current work")
	for _, args := range [][]string{{"search", "Quokka"}, {"pack", "--subject", "learning/widget"}} {
		if out := h.ok(args...); strings.Contains(out, old) {
			t.Fatalf("retained branch resurrected revoked identity in %v:\n%s", args, out)
		}
	}
	want(t, h.ok("search", "Quokka", "--all"), old, "[revoked]")
	h.git("checkout", "-q", "feature/lesson")
	fresh := h.add("--kind", "Learning", "--subject", "learning/widget", "--statement", "Wombat independent new lesson.")
	h.commit("new branch discovery")
	h.git("checkout", "-q", "feature/current")
	want(t, h.ok("search", "Wombat"), fresh, "[in flight on feature/lesson]")
}
