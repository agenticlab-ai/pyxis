package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestDiscardProposalRefusesCommittedDanglingRelations(t *testing.T) {
	t.Parallel()
	for _, selected := range []bool{true, false} {
		t.Run(map[bool]string{true: "selected", false: "surviving"}[selected], func(t *testing.T) {
			h, ids := proposalDiscardFixture(t)
			id := ids[0]
			if !selected {
				id = h.add("--kind", "Learning", "--subject", "learning/demo", "--statement", "Surviving claim")
			}
			r, _ := readEvidenceRecord(t, h, id)
			r.Relations = append(r.Relations, record.Relation{Type: record.DependsOn, Target: ids[0][:len(ids[0])-1] + "Z"})
			body, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			putFile(t, h.dir, store.Dir+"/"+r.Path(), string(body))
			h.commit("adversarial committed dangling relation")
			for _, mode := range []string{"--check", "--write"} {
				if code, _, errout := h.run(discardArgs(ids, mode)...); code != exitRefused || !strings.Contains(errout, "invalid committed memory") {
					t.Fatalf("dangling relation admitted: %d %s", code, errout)
				}
			}
			if h.git("status", "--porcelain") != "" {
				t.Fatal("refusal changed committed files")
			}
			if _, err := os.Stat(filepath.Join(h.dir, ".pyxis/logs/memory/proposal-discard")); !os.IsNotExist(err) {
				t.Fatal("dangling graph created a recovery archive", err)
			}
		})
	}
}

func TestDiscardProposalRefusesIgnoredWorkingSurvivors(t *testing.T) {
	t.Parallel()
	h, ids := proposalDiscardFixture(t)
	putFile(t, h.dir, ".gitignore", ".pyxis/logs/\nmemory/problem/ignored/\n")
	h.commit("ignore adversarial working proposal")
	id := h.add("--kind", "Document", "--subject", "problem/ignored", "--family", "journal", "--statement", "Ignored working survivor", "--literal-body", "--body-file", "docs/problems/demo/delivery.md", "--source", "docs/problems/demo/delivery.md", "--depends-on", ids[0])
	r, _ := readEvidenceRecord(t, h, id)
	before := map[string][]byte{}
	for _, name := range []string{store.Dir + "/" + r.Path(), store.Dir + "/" + r.BodyPath()} {
		before[name] = mustRead(t, filepath.Join(h.dir, name))
	}
	if h.git("status", "--porcelain") != "" {
		t.Fatal("fixture is not ignored")
	}
	for _, mode := range []string{"--check", "--write"} {
		if code, _, errout := h.run(discardArgs(ids, mode)...); code != exitRefused || !strings.Contains(errout, "absent from committed HEAD") {
			t.Fatalf("ignored survivor admitted: %d %s", code, errout)
		}
	}
	for name, body := range before {
		if !bytes.Equal(body, mustRead(t, filepath.Join(h.dir, name))) {
			t.Fatal("ignored survivor changed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(h.dir, ".pyxis/logs/memory/proposal-discard")); !os.IsNotExist(err) {
		t.Fatal("ignored survivor created a recovery archive", err)
	}
}

func TestDiscardProposalRetainsReplacementTakenAtRemoval(t *testing.T) {
	t.Parallel()
	for _, occupied := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore replacement", true: "preserve both replacements"}[occupied], func(t *testing.T) {
			h, ids := proposalDiscardFixture(t)
			s, root, out := proposalDiscardSession(t, h, ids)
			ops := nativeProposalOps(out.Archive, s.m.repo.RequireIgnored)
			nativeRemove := ops.remove
			competing := []byte("competing bytes arriving after the last comparison")
			ops.remove = func(ctx context.Context, root *os.Root, file proposalDiscardFile) error {
				putFile(t, h.dir, file.Path, string(competing))
				if !occupied {
					return nativeRemove(ctx, root, file)
				}
				// Introduce a second replacement only after the first object has
				// been taken and before exclusive restoration can publish it.
				taken := false
				atInspection := proposalIOContext{Context: ctx, beforeCheck: func() {
					if !taken {
						if _, err := os.Stat(filepath.Join(h.dir, file.Path)); os.IsNotExist(err) {
							putFile(t, h.dir, file.Path, "second replacement")
							taken = true
						}
					}
				}}
				return nativeRemove(atInspection, root, file)
			}
			err := applyProposalDiscard(t.Context(), s, root, &out, ops)
			if err == nil || out.Applied || !strings.Contains(err.Error(), "actual object retained") {
				t.Fatal("replacement falsely reported as successful withdrawal", err)
			}
			want := string(competing)
			if occupied {
				want = "second replacement"
			}
			if string(mustRead(t, filepath.Join(h.dir, out.Files[0].Path))) != want {
				t.Fatal("working replacement was overwritten")
			}
			matches, err := filepath.Glob(filepath.Join(h.dir, out.Archive, "taken-*/payload"))
			if err != nil || len(matches) != 1 || !bytes.Equal(competing, mustRead(t, matches[0])) {
				t.Fatal("actual competing object was lost", matches, err)
			}
			if err := checkProposalArchive(t.Context(), root, &out, true); err != nil {
				t.Fatal("committed recovery copies lost", err)
			}
		})
	}
}
