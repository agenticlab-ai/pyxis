package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func sourceDecisionCapture(t *testing.T, h *harness) []string {
	t.Helper()
	const source = "docs/tech-design/proposal-demo-v1.md"
	body := mustRead(t, filepath.Join(h.dir, source))
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	blocks, err := reconcile.ExtractLiveDesignDecisions(reconcile.Design{Slug: "proposal-demo-v1", Source: source, SHA256: digest, Status: "live"}, reconcile.File{Path: source, SHA256: digest, Body: body})
	if err != nil || len(blocks) != 1 {
		t.Fatal("invalid native Decision fixture", err)
	}
	document := h.add("--kind", "Document", "--subject", "design/proposal-demo-v1", "--family", "design", "--statement", "Captured design", "--literal-body", "--body-file", source, "--source", source, "--tag", "reconcile:source")
	h.commit("native design source proposal")
	block := blocks[0]
	companion := h.add("--kind", "Decision", "--subject", block.Subject, "--family", block.Family, "--statement", block.Statement, "--source", source,
		"--note", fmt.Sprintf("exact Decision evidence at %s lines %d-%d in section %s; sha256:%s", source, block.StartLine, block.EndLine, block.Section, block.EvidenceSHA256),
		"--tag", "reconcile:design-decision", "--tag", "source-block:"+block.EvidenceSHA256, "--tag", "source-design:"+digest, "--tag", "source:tech-design", "--depends-on", document)
	h.commit("native source-only Decision proposal")
	return []string{document, companion}
}

func sourceDecisionFixture(t *testing.T) (*harness, []string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	putFile(t, h.dir, ".gitignore", ".pyxis/logs/\n")
	h.commit("base")
	h.git("checkout", "-qb", "topic")
	putFile(t, h.dir, "docs/tech-design/proposal-demo-v1.md", "# Proposal\n\n## 2. Decision\n\n**Decision:** Preserve the original source.\n")
	h.commit("original editable design")
	ids := sourceDecisionCapture(t, h)
	putFile(t, h.dir, "docs/tech-design/proposal-demo-v1.md", "# Proposal\n\n## 2. Decision\n\n**Decision:** Preserve the final source.\n")
	h.commit("final editable design")
	return h, ids
}

func TestDiscardProposalRecoversExactSourceDecisionCompanions(t *testing.T) {
	t.Parallel()
	h, ids := sourceDecisionFixture(t)
	if code, _, _ := h.run("evidence", "--check"); code != exitRefused {
		t.Fatal("stale design captures did not fail evidence")
	}
	for _, incomplete := range [][]string{ids[:1], ids[1:]} {
		if code, _, _ := h.run(discardArgs(incomplete, "--write")...); code != exitRefused || h.git("status", "--porcelain") != "" {
			t.Fatal("incomplete source/companion batch changed files")
		}
	}
	var preview, applied proposalDiscardOutput
	decodeJSON(t, h.ok(discardArgs([]string{ids[1], ids[0]}, "--check")...), &preview)
	if preview.Applied || len(preview.Files) != 3 {
		t.Fatal("companion preview invented a literal body")
	}
	head, refs := h.git("rev-parse", "HEAD"), h.git("for-each-ref", "--format=%(refname) %(objectname)")
	index := mustRead(t, filepath.Join(h.dir, ".git/index"))
	decodeJSON(t, h.ok(discardArgs(ids, "--write")...), &applied)
	if !applied.Applied || h.git("rev-parse", "HEAD") != head || h.git("for-each-ref", "--format=%(refname) %(objectname)") != refs || !bytes.Equal(index, mustRead(t, filepath.Join(h.dir, ".git/index"))) {
		t.Fatal("companion withdrawal changed Git state")
	}
	repo, err := gitsrc.Open(t.Context(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range applied.Files {
		committed, err := repo.Files(t.Context(), head, []string{file.Path})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(h.dir, file.Path)); !os.IsNotExist(err) || !bytes.Equal(committed[file.Path], mustRead(t, filepath.Join(h.dir, applied.Archive, file.Path))) {
			t.Fatal("companion archive did not retain committed bytes", file.Path)
		}
	}
	h.commit("withdraw stale design capture graph")
	h.git("merge-base", "--is-ancestor", head, "HEAD")
	freshIDs := sourceDecisionCapture(t, h)
	if freshIDs[0] == ids[0] || freshIDs[1] == ids[1] {
		t.Fatal("withdrawn identity reused")
	}
	h.ok("evidence", "--check")
	h.git("checkout", "-q", "main")
	h.git("merge", "--squash", "topic")
	h.commit("accept final design captures")
	h.git("branch", "-D", "topic")
	clone := filepath.Join(t.TempDir(), "fresh")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--no-local", "--single-branch", "--no-tags", "--branch", "main", "file://"+h.dir, clone)
	newHarness(t, clone).ok("evidence", "--check")
	cmd := exec.Command("git", "cat-file", "-e", head+"^{commit}")
	cmd.Dir = clone
	if cmd.Run() == nil {
		t.Fatal("fresh evidence retained the withdrawn topic commit")
	}
	if _, err := os.Stat(filepath.Join(clone, applied.Archive)); !os.IsNotExist(err) {
		t.Fatal("fresh evidence depends on the proposal archive")
	}
	if code, _, _ := h.run(discardArgs(freshIDs, "--write")...); code != exitRefused {
		t.Fatal("accepted design graph became disposable")
	}
}

func TestDiscardProposalRefusesAuthoredOrAlteredDecisionCompanions(t *testing.T) {
	t.Parallel()
	for _, mutate := range []func(*record.Record){
		func(r *record.Record) { r.Rationale = "An authored semantic claim." },
		func(r *record.Record) { r.Tags = append(r.Tags, "code:internal/demo") },
		func(r *record.Record) { r.Statement = "Not the captured Decision." },
		func(r *record.Record) { r.Relations = nil },
		func(r *record.Record) { r.Relations = append(r.Relations, r.Relations[0]) },
	} {
		h, ids := sourceDecisionFixture(t)
		r, _ := readEvidenceRecord(t, h, ids[1])
		mutate(r)
		body, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		putFile(t, h.dir, store.Dir+"/"+r.Path(), string(body))
		h.commit("adversarial companion")
		for _, mode := range []string{"--check", "--write"} {
			if code, _, _ := h.run(discardArgs(ids, mode)...); code != exitRefused || h.git("status", "--porcelain") != "" {
				t.Fatal("authored or altered companion admitted")
			}
		}
	}
}
