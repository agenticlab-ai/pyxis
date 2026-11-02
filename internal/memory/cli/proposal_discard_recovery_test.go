package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func proposalDiscardSession(t *testing.T, h *harness, ids []string) (*evidenceSession, *os.Root, proposalDiscardOutput) {
	t.Helper()
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newEvidenceSession(t.Context(), m, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := prepareProposalDiscard(t.Context(), s, ids, "Withdraw superseded intermediate captures; recapture final journal.")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(m.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closePolicyRoot(root) })
	return s, root, out
}

func retainProposalArchive(t *testing.T, root *os.Root, out *proposalDiscardOutput) {
	t.Helper()
	for _, file := range out.Files {
		if err := saveProposalFile(t.Context(), root, out.Archive+"/"+file.Path, file.Bytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveProposalFile(t.Context(), root, out.Archive+"/intent.json", proposalIntent(out), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertProposalsUnchanged(t *testing.T, h *harness, out *proposalDiscardOutput) {
	t.Helper()
	for _, file := range out.Files {
		if !bytes.Equal(file.Bytes, mustRead(t, filepath.Join(h.dir, file.Path))) {
			t.Fatal("proposal bytes lost", file.Path)
		}
	}
}

func TestDiscardProposalRetriesInterruptedWithdrawal(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	s, root, out := proposalDiscardSession(t, h, ids)
	if err := removeProposalFile(t.Context(), root, out.Files[0], out.Archive, s.m.repo.RequireIgnored); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := h.run(discardArgs(ids, "--write")...); code != exitRefused {
		t.Fatal("missing file without complete intent admitted")
	}
	if err := restoreProposalFile(t.Context(), root, out.Files[0], out.Archive); err != nil {
		t.Fatal(err)
	}
	retainProposalArchive(t, root, &out)
	if err := removeProposalFile(t.Context(), root, out.Files[0], out.Archive, s.m.repo.RequireIgnored); err != nil {
		t.Fatal(err)
	}
	wrongReason := append(discardArgs(ids, "--write"), "--reason", "Different withdrawal")
	if code, _, _ := h.run(wrongReason...); code != exitRefused {
		t.Fatal("retry with changed intent admitted")
	}
	h.ok(discardArgs(ids, "--check")...)
	for range 2 {
		var result proposalDiscardOutput
		decodeJSON(t, h.ok(discardArgs(ids, "--write")...), &result)
		if !result.Applied || result.Candidate != out.Candidate {
			t.Fatal("identical retry did not converge", result)
		}
	}
}

func TestDiscardProposalRollsBackWriteFailuresAndPinRaces(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"archive", "intent", "archive-corruption", "cancel-during-archive", "remove-before", "remove-after", "head-after-first", "base-after-last", "cancel-after-first", "concurrent-unselected", "no-removal"} {
		t.Run(scenario, func(t *testing.T) {
			h, ids := proposalDiscardFixture(t)
			s, root, out := proposalDiscardSession(t, h, ids)
			ops := nativeProposalOps(out.Archive, s.m.repo.RequireIgnored)
			calls := 0
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			injected := errors.New("injected proposal write failure")
			switch scenario {
			case "archive", "intent", "archive-corruption", "cancel-during-archive":
				ops.archive = func(ctx context.Context, root *os.Root, name string, body []byte, mode fs.FileMode) error {
					calls++
					if scenario == "archive" && calls == 2 || scenario == "intent" && strings.HasSuffix(name, "/intent.json") {
						return injected
					}
					if scenario == "archive-corruption" && calls == 2 {
						body = []byte("corrupted archival write")
					}
					err := saveProposalFile(ctx, root, name, body, mode)
					if scenario == "cancel-during-archive" && calls == 1 {
						cancel()
					}
					return err
				}
			default:
				ops.remove = func(ctx context.Context, root *os.Root, file proposalDiscardFile) error {
					calls++
					if scenario == "no-removal" || scenario == "remove-before" && calls == 2 {
						if scenario == "no-removal" {
							return nil
						}
						return injected
					}
					if err := removeProposalFile(ctx, root, file, out.Archive, s.m.repo.RequireIgnored); err != nil {
						return err
					}
					switch {
					case scenario == "remove-after" && calls == 2:
						return injected
					case scenario == "head-after-first" && calls == 1:
						h.git("commit", "--allow-empty", "-qm", "concurrent head")
					case scenario == "base-after-last" && calls == len(out.Files):
						h.git("update-ref", "refs/heads/main", out.Candidate)
					case scenario == "cancel-after-first" && calls == 1:
						cancel()
					case scenario == "concurrent-unselected" && calls == 1:
						putFile(t, h.dir, "memory/concurrent.json", "a concurrent writer's bytes")
					}
					return nil
				}
			}
			if err := applyProposalDiscard(ctx, s, root, &out, ops); err == nil || out.Applied {
				t.Fatal("failed or raced mutation reported success", err)
			}
			assertProposalsUnchanged(t, h, &out)
			if scenario == "archive" || scenario == "intent" {
				if err := applyProposalDiscard(t.Context(), s, root, &out, nativeProposalOps(out.Archive, s.m.repo.RequireIgnored)); err != nil || !out.Applied {
					t.Fatal("retry after partial archive failed", err)
				}
			}
		})
	}
}

func TestDiscardProposalRollbackNeverOverwritesReplacement(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	s, root, out := proposalDiscardSession(t, h, ids)
	ops := nativeProposalOps(out.Archive, s.m.repo.RequireIgnored)
	ops.remove = func(ctx context.Context, root *os.Root, file proposalDiscardFile) error {
		if err := removeProposalFile(ctx, root, file, out.Archive, s.m.repo.RequireIgnored); err != nil {
			return err
		}
		putFile(t, h.dir, file.Path, "concurrent replacement")
		return errors.New("removal outcome interrupted")
	}
	err := applyProposalDiscard(t.Context(), s, root, &out, ops)
	if err == nil || !strings.Contains(err.Error(), "restore "+out.Files[0].Path) || out.Applied {
		t.Fatal("restoration conflict not reported", err)
	}
	if string(mustRead(t, filepath.Join(h.dir, out.Files[0].Path))) != "concurrent replacement" {
		t.Fatal("rollback overwrote concurrent replacement")
	}
	if err := checkProposalArchive(t.Context(), root, &out, true); err != nil {
		t.Fatal("original bytes lost despite conflict", err)
	}
}

func TestDiscardProposalRefusesDirtyIndexModesAndUncommittedRecords(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"staged-different-bytes", "mode", "unselected", "uncommitted", "not-ignored", "invalid-committed-memory"} {
		t.Run(scenario, func(t *testing.T) {
			h, ids := proposalDiscardFixture(t)
			_, root, out := proposalDiscardSession(t, h, ids)
			file := out.Files[0]
			switch scenario {
			case "staged-different-bytes":
				putFile(t, h.dir, file.Path, string(file.Bytes)+"\n")
				h.git("add", "--", file.Path)
				putFile(t, h.dir, file.Path, string(file.Bytes))
			case "mode":
				if err := os.Chmod(filepath.Join(h.dir, file.Path), 0755); err != nil {
					t.Fatal(err)
				}
			case "unselected":
				putFile(t, h.dir, "memory/untracked.json", "unselected proposal")
			case "uncommitted":
				ids = []string{h.add("--kind", "Document", "--subject", "problem/demo", "--family", "other", "--statement", "Uncommitted proposal", "--literal-body", "--body-file", "docs/problems/demo/delivery.md", "--source", "docs/problems/demo/delivery.md", "--tag", "reconcile:source")}
			case "not-ignored":
				putFile(t, h.dir, ".gitignore", "")
			case "invalid-committed-memory":
				putFile(t, h.dir, "memory/bad.json", "invalid committed record")
				h.commit("invalid committed memory")
			}
			if code, _, _ := h.run(discardArgs(ids, "--write")...); code != exitRefused {
				t.Fatal("unsafe withdrawal admitted")
			}
			assertProposalsUnchanged(t, h, &out)
			if err := checkProposalArchive(t.Context(), root, &out, true); err == nil {
				t.Fatal("unsafe preflight wrote a complete archive")
			}
		})
	}
}

func TestDiscardProposalRefusesHistoricallyFrozenIdentity(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	_, root, out := proposalDiscardSession(t, h, ids)
	h.git("update-ref", "refs/heads/main", out.Candidate)
	paths := []string{"rm", "--"}
	for _, file := range out.Files {
		paths = append(paths, file.Path)
	}
	h.git(paths...)
	h.commit("historical removal fixture")
	h.git("update-ref", "refs/heads/main", h.git("rev-parse", "HEAD"))
	for _, file := range out.Files {
		if err := restoreProposalFile(t.Context(), root, file, out.Archive); err != nil {
			t.Fatal(err)
		}
	}
	h.commit("reintroduced frozen identity fixture")
	if code, _, errout := h.run(discardArgs(ids, "--write")...); code != exitRefused || !strings.Contains(errout, "previously present on mainline") {
		t.Fatal("historically frozen identity admitted", code, errout)
	}
	assertProposalsUnchanged(t, h, &out)
}

func TestDiscardProposalCommandFailuresAndTextOutput(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	args := discardArgs(ids, "--check")
	textArgs := []string{"discard-proposal", "--reason", "Preview journal", "--record", ids[0], "--record", ids[1], "--check"}
	if text := h.ok(textArgs...); !strings.Contains(text, "applied=false") {
		t.Fatal("text preview missing state", text)
	}
	env, _, _ := h.env("")
	env.Stdout = gateBrokenWriter{}
	if code, err := runDiscardProposal(t.Context(), args[1:], env); code != exitOK || err == nil {
		t.Fatal("output failure hidden", code, err)
	}
	_, root, out := proposalDiscardSession(t, h, ids)
	archiveParent := filepath.Dir(filepath.Join(h.dir, out.Archive, out.Files[0].Path))
	for i := range 100 {
		putFile(t, archiveParent, fmt.Sprintf(".%s.pending-%d", filepath.Base(out.Files[0].Path), i), "interrupted copy")
	}
	if code, _, _ := h.run(discardArgs(ids, "--write")...); code != exitRefused {
		t.Fatal("archive publication failure reported success")
	}
	assertProposalsUnchanged(t, h, &out)
	if err := checkProposalArchive(t.Context(), root, &out, true); err == nil {
		t.Fatal("incomplete archival treated as complete")
	}
	h.git("branch", "-D", "main")
	if code, _, _ := h.run(discardArgs(ids, "--check")...); code != exitRefused {
		t.Fatal("missing configured mainline admitted")
	}
	env.Dir = t.TempDir()
	if _, err := runDiscardProposal(t.Context(), args[1:], env); err == nil {
		t.Fatal("nonrepository proposal command admitted")
	}
}
