package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestEvidenceCommandSelectionAndRefusals(t *testing.T) {
	t.Parallel()
	h, base, ids := draftRepairFixture(t)
	for _, args := range [][]string{
		{"evidence"}, {"evidence", "--unknown"}, {"evidence", "--check", "extra"},
		{"evidence", "--check", "--write"}, {"evidence", "--check", "--source", "HEAD"},
		{"evidence", "--record", "absent"}, {"evidence", "--record", "memory/absent.json"},
		{"evidence", "--record", ids[0], "--source", "missing-source"},
	} {
		if code, _, _ := h.run(args...); code == exitOK {
			t.Fatalf("accepted invalid command: %v", args)
		}
	}
	r, original := readEvidenceRecord(t, h, ids[0])
	path := store.Dir + "/" + r.Path()
	var preview evidenceRepairOutput
	decodeJSON(t, h.ok("evidence", "--source", "HEAD", "--record", path, "--record", ids[0], "--json"), &preview)
	if len(preview.Proposed) != 1 || len(preview.Applied) != 0 {
		t.Fatalf("selection duplicated or preview wrote: %+v", preview)
	}
	_, after := readEvidenceRecord(t, h, ids[0])
	if !bytes.Equal(original, after) {
		t.Fatal("preview changed record")
	}
	for _, asJSON := range []bool{false, true} {
		env, _, _ := h.env("")
		env.Stdout = gateBrokenWriter{}
		args := []string{"evidence", "--record", ids[0]}
		if asJSON {
			args = append(args, "--json")
		}
		if code := Run(t.Context(), args, env); code != exitRefused {
			t.Fatal("preview lost output error")
		}
	}
	// Reproducible reports are output only; even a blocked legacy report home
	// must not prevent a check from explaining its defects or a valid repair.
	writeFixtureFile(t, h.dir, ".pyxis/evidence", "obstruction", 0o600)
	if code, out, errout := h.run("evidence", "--check", "--retain"); code != exitRefused || !strings.Contains(out, "missing-durable-evidence") || strings.Contains(errout, "retain") {
		t.Fatalf("failed check depended on report retention: %d %s %s", code, out, errout)
	}
	_, after = readEvidenceRecord(t, h, ids[0])
	if !bytes.Equal(original, after) {
		t.Fatal("check wrote draft")
	}
	h.ok("evidence", "--record", ids[0], "--record", ids[1], "--write")
	if got := string(mustRead(t, filepath.Join(h.dir, ".pyxis/evidence"))); got != "obstruction" {
		t.Fatal("repair changed the legacy report home")
	}
	if err := os.Remove(filepath.Join(h.dir, ".pyxis/evidence")); err != nil {
		t.Fatal(err)
	}
	h.ok("evidence", "--record", ids[0], "--record", ids[1], "--write")
	h.commit("bound records")
	h.git("update-ref", "refs/heads/main", h.git("rev-parse", "HEAD"))
	if code, _, errout := h.run("evidence", "--record", ids[0], "--write"); code != exitRefused || !strings.Contains(errout, "frozen") {
		t.Fatalf("frozen repair: %d %s", code, errout)
	}
	// Removing HEAD cannot be mistaken for a clean or empty evidence set.
	h.git("update-ref", "-d", "refs/heads/topic")
	if code, _, _ := h.run("evidence", "--check", "--base", base); code != exitRefused {
		t.Fatal("unborn candidate passed")
	}
	for _, write := range []bool{false, true} {
		env, _, _ := h.env("")
		env.Dir = t.TempDir()
		args := []string{"evidence", "--check"}
		if write {
			args = []string{"evidence", "--record", ids[0], "--write"}
		}
		if code := Run(t.Context(), args, env); code == exitOK {
			t.Fatal("non-repository passed")
		}
	}
}

func TestEvidenceChecksEmitFailuresWithoutFiles(t *testing.T) {
	t.Parallel()
	h, _, _ := draftRepairFixture(t)
	for _, args := range [][]string{
		{"evidence", "--check"},
		{"evidence", "--check", "--json"},
		{"evidence", "--check", "--retain", "--json"},
		{"evidence", "--check", "--base", "absent", "--retain", "--json"},
	} {
		code, out, errout := h.run(args...)
		if code != exitRefused || !strings.Contains(out, "evidence") {
			t.Fatalf("%v lost the failing verdict: %d %s %s", args, code, out, errout)
		}
		if status := h.git("status", "--porcelain", "--untracked-files=all"); status != "" {
			t.Fatalf("%v wrote files: %s", args, status)
		}
		if _, err := os.Stat(filepath.Join(h.dir, ".pyxis/evidence")); !os.IsNotExist(err) {
			t.Fatalf("%v created a report home: %v", args, err)
		}
	}
}

func TestEvidenceAdmissionChecksEveryDeclaredSource(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("base")
	h.git("checkout", "-qb", "topic")
	writeFixtureFile(t, h.dir, "a.md", "a\n", 0o644)
	writeFixtureFile(t, h.dir, "b.md", "b\n", 0o644)
	h.commit("two sources")
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range []string{"a.md", "b.md"} {
		r := memoryTestRecord(record.Learning, "learning/batch", "", "Capture each source.", i+1)
		r.Provenance.Source = p
		if err := m.captureEvidence(t.Context(), r); err != nil {
			t.Fatal(err)
		}
		if len(r.Provenance.Evidence.Files) != 1 || r.Provenance.Evidence.Files[0].Path != p {
			t.Fatal("batch reused prior source selection")
		}
	}
	writeFixtureFile(t, h.dir, "b.md", "changed\n", 0o644)
	if code, _, errout := h.run("add", "--kind", "Learning", "--subject", "learning/dirty", "--statement", "No mixed snapshot.", "--source", "a.md", "--learned-from", "b.md"); code != exitRefused || !strings.Contains(errout, "uncommitted") {
		t.Fatalf("dirty second source: %d %s", code, errout)
	}
	for _, source := range []string{"missing.md", "code:internal/absent", "../escape", "a.md\nargument"} {
		if code, _, _ := h.run("add", "--kind", "Learning", "--subject", "learning/invalid", "--statement", "Reject invalid declaration.", "--source", source); code != exitRefused {
			t.Fatal("accepted invalid source", source)
		}
	}
}
