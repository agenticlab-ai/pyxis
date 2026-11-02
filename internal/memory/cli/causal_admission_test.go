package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestEveryRecordWriterRefusesHistoricalActivation(t *testing.T) {
	t.Parallel()
	for _, kind := range record.Kinds() {
		t.Run(kind, func(t *testing.T) {
			h := initRepo(t)
			h.ok("init")
			q := memoryTestRecord(record.Question, "prd/x", "", "existing prerequisite", 1)
			putMemoryRecord(t, h.dir, q)
			family := ""
			if record.IsClaim(kind) {
				family = "F-1"
			}
			r := memoryTestRecord(kind, "prd/x", family, "backdated target", 10)
			var body []byte
			switch kind {
			case record.Answer:
				r.Relations = []record.Relation{relInput(record.Answers, q.ID)}
			case record.Revocation:
				r.Relations = []record.Relation{relInput(record.Revokes, q.ID)}
			case record.Assumption:
				r.Expires = t0.Add(time.Hour).Format(time.RFC3339)
			case record.Document:
				body = []byte("# Valid body\n")
				r.BodySHA256 = record.HashBody(body)
			}
			carrier := memoryTestRecord(record.Learning, "prd/x", "", "historical dangling reference", 20, relInput(record.About, r.ID))
			putMemoryRecord(t, h.dir, carrier)
			env, _, _ := h.env("")
			m, err := open(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.store(t.Context(), r, body, env)
			if err == nil || !strings.Contains(err.Error(), "historical-act-reinterpreted") {
				t.Fatalf("%s admitted a historical activation: %v", kind, err)
			}
			assertAbsentRecordFiles(t, h, r)
		})
	}
}

func TestDocumentTrialIncludesBodyAndRefusesInvalidBodies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		body []byte
	}{{"missing", nil}, {"mismatched", []byte("wrong body\n")}} {
		t.Run(tc.name, func(t *testing.T) {
			h := initRepo(t)
			h.ok("init")
			r := memoryTestRecord(record.Document, "prd/x", "prd", "candidate", 10)
			r.BodySHA256 = record.HashBody([]byte("right body\n"))
			env, _, _ := h.env("")
			m, err := open(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.store(t.Context(), r, tc.body, env); err == nil || !strings.Contains(err.Error(), "record-invalid") {
				t.Fatalf("invalid trial body accepted: %v", err)
			}
			assertAbsentRecordFiles(t, h, r)
		})
	}
}

func TestDocumentWriterCannotReactivateRejectedRevocation(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	claim := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "kept")
	bodyDir := t.TempDir()
	writeFixtureFile(t, bodyDir, "old.md", "# Old\n\n{{FR-1}}\n", 0644)
	writeFixtureFile(t, bodyDir, "new.md", "# New\n", 0644)
	h.add("--kind", "Document", "--subject", "prd/x", "--family", "prd", "--statement", "old", "--body-file", filepath.Join(bodyDir, "old.md"))
	putMemoryRecord(t, h.dir, memoryTestRecord(record.Revocation, "prd/x", "", "withdraw", 100, relInput(record.Revokes, claim)))
	h.clock = t0.Add(50 * time.Second)
	before := memoryRecordCount(t, h.dir)
	code, _, errout := h.run("revise", "prd/x#prd", "--statement", "new", "--body-file", filepath.Join(bodyDir, "new.md"))
	if code != exitRefused || !strings.Contains(errout, "historical-act-reinterpreted") || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("bodyless trial bypass: %d %s", code, errout)
	}
	entries, err := os.ReadDir(filepath.Join(h.dir, store.Dir, "prd/x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("refused revision left files behind: %v", entries)
	} // claim, old JSON/body, rejected revocation.
}

func TestNonRevocationWriterCannotBreakExistingEmbed(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	claim := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "kept")
	bodyDir := t.TempDir()
	writeFixtureFile(t, bodyDir, "body.md", "{{FR-1}}\n", 0644)
	h.add("--kind", "Document", "--subject", "prd/x", "--family", "prd", "--statement", "consumer", "--body-file", filepath.Join(bodyDir, "body.md"))
	before := memoryRecordCount(t, h.dir)
	code, _, errout := h.run("add", "--kind", "Promise", "--subject", "prd/x", "--family", "FR-2", "--statement", "moves family", "--supersedes", claim)
	if code != exitRefused || !strings.Contains(errout, "embed-unresolved") || memoryRecordCount(t, h.dir) != before {
		t.Fatalf("non-revocation projection bypass: %d %s", code, errout)
	}
}

func assertAbsentRecordFiles(t *testing.T, h *harness, r *record.Record) {
	t.Helper()
	for _, name := range []string{r.Path(), r.BodyPath()} {
		if _, err := os.Lstat(filepath.Join(h.dir, store.Dir, name)); !os.IsNotExist(err) {
			t.Fatalf("refused record left %s: %v", name, err)
		}
	}
}
