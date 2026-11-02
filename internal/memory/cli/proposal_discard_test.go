package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func proposalDiscardFixture(t *testing.T) (*harness, []string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	putFile(t, h.dir, ".gitignore", ".pyxis/logs/\n")
	putFile(t, h.dir, "docs/problems/demo/delivery.md", "accepted journal\n")
	h.commit("base")
	previous := h.add("--kind", "Document", "--subject", "problem/demo", "--family", "delivery", "--statement", "Accepted journal", "--literal-body", "--body-file", "docs/problems/demo/delivery.md", "--source", "docs/problems/demo/delivery.md", "--tag", "reconcile:source")
	h.commit("accepted literal capture")
	h.git("checkout", "-qb", "topic")
	var ids []string
	for _, body := range []string{"first journal\n", "second journal\n"} {
		putFile(t, h.dir, "docs/problems/demo/delivery.md", body)
		h.commit("journal source")
		args := []string{"add", "--kind", "Document", "--subject", "problem/demo", "--family", "delivery", "--statement", "Captured journal", "--literal-body", "--body-file", "docs/problems/demo/delivery.md", "--source", "docs/problems/demo/delivery.md", "--tag", "reconcile:source"}
		args = append(args, "--supersedes", previous)
		previous = h.add(args[1:]...)
		ids = append(ids, previous)
		h.commit("literal proposal capture")
	}
	return h, ids
}

func discardArgs(ids []string, mode string) []string {
	args := []string{"discard-proposal", "--reason", "Withdraw superseded intermediate captures; recapture final journal.", mode, "--json"}
	for _, id := range ids {
		args = append(args, "--record", id)
	}
	return args
}

func TestDiscardProposalPreservesHistoryAndRecoversFreshEvidence(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	first, _ := readEvidenceRecord(t, h, ids[0])
	if len(first.Relations) != 1 || first.Relations[0].Type != record.Supersedes {
		t.Fatal("fixture lacks its accepted predecessor")
	}
	frozen, _ := readEvidenceRecord(t, h, first.Relations[0].Target)
	frozenPaths := []string{store.Dir + "/" + frozen.Path(), store.Dir + "/" + frozen.BodyPath()}
	frozenBytes := make(map[string][]byte)
	for _, path := range frozenPaths {
		frozenBytes[path] = mustRead(t, filepath.Join(h.dir, path))
	}
	if code, _, _ := h.run("evidence", "--check"); code != exitRefused {
		t.Fatal("intermediate capture should fail before withdrawal")
	}
	head := h.git("rev-parse", "HEAD")
	index := mustRead(t, filepath.Join(h.dir, ".git/index"))
	refs := h.git("for-each-ref", "--format=%(refname) %(objectname)")
	var preview proposalDiscardOutput
	decodeJSON(t, h.ok(discardArgs(ids, "--check")...), &preview)
	if preview.Applied || len(preview.Files) != 4 || preview.Candidate != head {
		t.Fatalf("invalid preview: %+v", preview)
	}
	if first, second := h.ok(discardArgs(ids, "--check")...), h.ok(discardArgs([]string{ids[1], ids[0], ids[0]}, "--check")...); first != second {
		t.Fatal("preview changed with order, duplicates, clock, or entropy")
	}
	if h.git("status", "--porcelain") != "" {
		t.Fatal("preview mutated the checkout")
	}
	var applied proposalDiscardOutput
	decodeJSON(t, h.ok(discardArgs(ids, "--write")...), &applied)
	if !applied.Applied || h.git("rev-parse", "HEAD") != head {
		t.Fatal("discard did not preserve the original commit")
	}
	if !bytes.Equal(index, mustRead(t, filepath.Join(h.dir, ".git/index"))) || h.git("for-each-ref", "--format=%(refname) %(objectname)") != refs {
		t.Fatal("withdrawal changed Git index or refs")
	}
	repo, err := gitsrc.Open(t.Context(), h.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range applied.Files {
		if _, err := os.Stat(filepath.Join(h.dir, file.Path)); !os.IsNotExist(err) {
			t.Fatal("active proposal file remains", file.Path, err)
		}
		committed, err := repo.Files(t.Context(), head, []string{file.Path})
		if err != nil {
			t.Fatal(err)
		}
		archive := mustRead(t, filepath.Join(h.dir, applied.Archive, file.Path))
		if !bytes.Equal(archive, committed[file.Path]) {
			t.Fatal("archive changed exact committed bytes", file.Path)
		}
	}
	for path, before := range frozenBytes {
		if !bytes.Equal(before, mustRead(t, filepath.Join(h.dir, path))) {
			t.Fatal("withdrawal changed accepted memory", path)
		}
	}
	h.commit("withdraw proposals without rewriting history")
	h.git("merge-base", "--is-ancestor", head, "HEAD")
	if code, _, _ := h.run("evidence", "--check"); code != exitOK {
		t.Fatal("withdrawn candidates still poison evidence")
	}
	// A fresh final capture acquires a new identity and durable landing-tree
	// evidence, while every prior Git object remains unchanged/addressable.
	final := h.add("--kind", "Document", "--subject", "problem/demo", "--family", "delivery", "--statement", "Final journal", "--literal-body", "--body-file", "docs/problems/demo/delivery.md", "--source", "docs/problems/demo/delivery.md", "--tag", "reconcile:source", "--supersedes", frozen.ID)
	if final == ids[0] || final == ids[1] {
		t.Fatal("withdrawn identity reused")
	}
	h.commit("final capture")
	h.ok("evidence", "--check")
	topic := h.git("rev-parse", "HEAD")
	h.git("checkout", "-q", "main")
	h.git("merge", "--squash", "topic")
	h.commit("squash final capture")
	h.git("branch", "-D", "topic")
	clone := filepath.Join(t.TempDir(), "fresh")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--no-local", "--single-branch", "--no-tags", "--branch", "main", "file://"+h.dir, clone)
	fresh := newHarness(t, clone)
	fresh.ok("evidence", "--check")
	for _, oid := range []string{head, topic} {
		cmd := exec.Command("git", "cat-file", "-e", oid+"^{commit}")
		cmd.Dir = clone
		if err := cmd.Run(); err == nil {
			t.Fatal("fresh clone retained a topic commit", oid)
		}
	}
	if _, err := os.Stat(filepath.Join(clone, applied.Archive)); !os.IsNotExist(err) {
		t.Fatal("fresh evidence depends on archive", err)
	}
	if code, _, errout := fresh.run(discardArgs([]string{final}, "--write")...); code != exitRefused || !strings.Contains(errout, "frozen") {
		t.Fatalf("frozen final capture admitted: %d %s", code, errout)
	}
}

func TestDiscardProposalRefusesUnsafeBatchesWithoutWrites(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	for _, args := range [][]string{
		{"discard-proposal"}, {"discard-proposal", "--unknown"},
		discardArgs(ids[:1], "--write"),
		discardArgs([]string{ids[0], "Document-absent"}, "--write"),
		append(discardArgs(ids, "--write"), "--check"),
		append(discardArgs(ids, "--check"), "extra"),
	} {
		if code, _, _ := h.run(args...); code == exitOK {
			t.Fatal("unsafe proposal selection accepted", args)
		}
		if h.git("status", "--porcelain") != "" {
			t.Fatal("refused selection wrote files", args)
		}
	}
	// Native semantic claims are not disposable lossless captures.
	semantic := h.add("--kind", "Learning", "--subject", "learning/demo", "--statement", "A real lesson")
	h.commit("semantic proposal")
	if code, _, errout := h.run(discardArgs([]string{semantic}, "--write")...); code != exitRefused || !strings.Contains(errout, "lossless") {
		t.Fatalf("semantic proposal accepted: %d %s", code, errout)
	}
	// Dirty selected JSON or literal body cannot be silently deleted.
	r, _ := readEvidenceRecord(t, h, ids[1])
	for _, file := range []string{store.Dir + "/" + r.Path(), store.Dir + "/" + r.BodyPath()} {
		original := mustRead(t, filepath.Join(h.dir, file))
		putFile(t, h.dir, file, string(original)+"\n")
		if code, _, _ := h.run(discardArgs(ids, "--write")...); code == exitOK {
			t.Fatal("dirty selected file discarded", file)
		}
		putFile(t, h.dir, file, string(original))
	}
	if r.Kind != record.Document {
		t.Fatal("fixture is not a Document")
	}
}

func TestDiscardProposalRefusesArchiveRedirectsAndConflicts(t *testing.T) {
	t.Parallel()
	for _, redirect := range []bool{false, true} {
		t.Run(map[bool]string{false: "conflicting bytes", true: "symlink directory"}[redirect], func(t *testing.T) {
			h, ids := proposalDiscardFixture(t)
			var out proposalDiscardOutput
			decodeJSON(t, h.ok(discardArgs(ids, "--check")...), &out)
			if redirect {
				if err := os.MkdirAll(filepath.Join(h.dir, ".pyxis"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(h.dir, ".pyxis/logs")); err != nil {
					t.Fatal(err)
				}
			} else {
				putFile(t, h.dir, out.Archive+"/"+out.Files[0].Path, "conflicting archive\n")
			}
			if code, _, _ := h.run(discardArgs(ids, "--write")...); code != exitRefused {
				t.Fatal("unsafe archive destination accepted")
			}
			for _, f := range out.Files {
				if _, err := os.Stat(filepath.Join(h.dir, f.Path)); err != nil {
					t.Fatal("proposal removed despite archive refusal", err)
				}
			}
		})
	}
}
