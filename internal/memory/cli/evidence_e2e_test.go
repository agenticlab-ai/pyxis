package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func readEvidenceRecord(t *testing.T, h *harness, id string) (*record.Record, []byte) {
	t.Helper()
	s, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(h.dir, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	r := s.Get(id)
	if r == nil {
		t.Fatalf("record %s absent: %v", id, s.Problems)
	}
	return r, rawRecord(s, id)
}

func writeEvidenceRecord(t *testing.T, h *harness, r *record.Record) {
	t.Helper()
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, h.dir, store.Dir+"/"+r.Path(), string(b), 0o644)
}

func TestEvidenceSixRecordsSurviveSquashAndFreshInstalledClone(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("memory base")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "topic")
	for _, name := range []string{"admit", "runner", "localfs"} {
		writeFixtureFile(t, h.dir, "internal/"+name+"/a.go", "package "+name+"\nfunc A() bool { return true }\n", 0o644)
		writeFixtureFile(t, h.dir, "internal/"+name+"/b.go", "package "+name+"\n", 0o644)
		writeFixtureFile(t, h.dir, "internal/"+name+"/a_test.go", "package "+name+"\n// Exact cited test bytes.\n", 0o644)
	}
	h.commit("source revision")
	original := h.git("rev-parse", "HEAD")
	var ids []string
	for _, name := range []string{"admit", "runner", "localfs"} {
		ids = append(ids, h.add("--kind", "Learning", "--subject", "learning/"+name, "--statement", "Retain reproducible source evidence.", "--source", "internal/"+name+"/a.go", "--learned-from", "internal/"+name+"/a_test.go", "--tag", "code:internal/"+name))
		ids = append(ids, h.add("--kind", "Need", "--subject", "code/"+name, "--family", "anchor", "--statement", "Production unit exists.", "--source", "code:internal/"+name))
	}
	h.commit("six records")
	for _, id := range ids {
		r, _ := readEvidenceRecord(t, h, id)
		r.Provenance.Commit = original
		r.Provenance.Evidence = nil
		writeEvidenceRecord(t, h, r)
	}
	h.commit("reproduce old branch-only provenance")
	if code, out, _ := h.run("evidence", "--check", "--base", base, "--json"); code != exitRefused || strings.Count(out, `"code": "missing-durable-evidence"`) != 6 {
		t.Fatalf("old references passed: %d %s", code, out)
	}
	args := []string{"evidence", "--base", base, "--json"}
	before := map[string]*record.Record{}
	for _, id := range ids {
		args = append(args, "--record", id)
		before[id], _ = readEvidenceRecord(t, h, id)
	}
	first := h.ok(args...)
	second := h.ok(args...)
	if first != second {
		t.Fatal("repair preview used clock or entropy")
	}
	var repaired evidenceRepairOutput
	decodeJSON(t, h.ok(append(args, "--write")...), &repaired)
	if len(repaired.Applied) != 6 || len(repaired.Pending) != 0 {
		t.Fatalf("repair: %+v", repaired)
	}
	for _, id := range ids {
		r, _ := readEvidenceRecord(t, h, id)
		if r.Provenance.Commit != "" || r.Provenance.Evidence.Locator != record.EvidenceIntroduction {
			t.Fatalf("branch hash retained: %+v", r.Provenance)
		}
		r.Provenance = before[id].Provenance
		if !reflect.DeepEqual(r, before[id]) {
			t.Fatal("repair changed metadata or prose")
		}
	}
	repaired = evidenceRepairOutput{}
	decodeJSON(t, h.ok(append(args, "--write")...), &repaired)
	if len(repaired.Applied) != 0 || len(repaired.Proposed) != 0 {
		t.Fatal("second repair wrote files")
	}
	if _, err := os.Stat(filepath.Join(h.dir, ".pyxis/evidence")); !os.IsNotExist(err) {
		t.Fatalf("repair created a report home: %v", err)
	}
	h.commit("repair record bindings")
	h.ok("evidence", "--check", "--base", base)
	topic := h.git("rev-parse", "HEAD")
	h.git("checkout", "-q", "main")
	h.git("merge", "--squash", "topic")
	h.commit("squash topic")
	h.git("branch", "-D", "topic")
	clone := filepath.Join(t.TempDir(), "fresh")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--no-local", "--single-branch", "--no-tags", "--branch", "main", "file://"+h.dir, clone)
	fresh := newHarness(t, clone)
	for _, oid := range []string{original, topic} {
		cmd := exec.Command("git", "cat-file", "-e", oid+"^{commit}")
		cmd.Dir = clone
		if err := cmd.Run(); err == nil {
			t.Fatal("fresh transport clone retained a topic commit", oid)
		}
	}
	if _, err := os.Stat(filepath.Join(clone, ".git/objects/info/alternates")); !os.IsNotExist(err) {
		t.Fatal("transport clone used alternates")
	}
	// Install the public CLI and expose only Git on PATH: no agent CLI,
	// model credentials, shell fallback, or dependency download is available.
	bin := filepath.Join(t.TempDir(), "memory")
	build := exec.Command("go", "build", "-o", bin, "../../../cmd/memory")
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("install CLI: %v %s", err, out)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	pathDir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(pathDir, "git")); err != nil {
		t.Fatal(err)
	}
	installed := func() []byte {
		t.Helper()
		cmd := exec.Command(bin, "evidence", "--check", "--base", "main", "--json")
		cmd.Dir = clone
		cmd.Env = []string{"PATH=" + pathDir, "HOME=" + t.TempDir(), "GIT_CONFIG_NOSYSTEM=1"}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("installed replay: %v %s", err, out)
		}
		return out
	}
	output := installed()
	if !bytes.Equal(output, installed()) {
		t.Fatal("installed replay is not deterministic")
	}
	var report evidenceReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if report.blocking() || len(report.Entries) != 6 {
		t.Fatalf("fresh clone report: %+v", report)
	}
	if _, err := os.Stat(filepath.Join(clone, ".pyxis/evidence")); !os.IsNotExist(err) {
		t.Fatalf("fresh clone unexpectedly has reports: %v", err)
	}
	fresh.git("rm", "-r", "internal")
	fresh.commit("later source deletion")
	installed() // Historical introductions retain all file and unit evidence.
	if _, err := os.Stat(filepath.Join(clone, ".pyxis/evidence")); !os.IsNotExist(err) {
		t.Fatalf("historical replay created a report home: %v", err)
	}
	// Immutability is part of locating the witness, not a loose content search.
	r, _ := readEvidenceRecord(t, fresh, ids[0])
	r.Statement = "Rewritten after landing."
	writeEvidenceRecord(t, fresh, r)
	fresh.commit("illegal record rewrite")
	if code, out, _ := fresh.run("evidence", "--check", "--json"); code != exitRefused || !strings.Contains(out, "evidence-record-rewritten") {
		t.Fatalf("historical rewrite passed: %d %s", code, out)
	}
	if status := fresh.git("status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("failed historical replay wrote files: %s", status)
	}
}

func TestEvidenceNewInvalidRecordsCannotBecomeDebt(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("base")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "topic")
	id := h.add("--kind", "Learning", "--subject", "learning/invalid", "--statement", "Unverified lesson.")
	r, _ := readEvidenceRecord(t, h, id)
	r.Provenance.Source = "README.md"
	r.Provenance.Commit = base
	r.Tags = []string{"legacy-unverified"}
	writeEvidenceRecord(t, h, r)
	h.add("--kind", "Learning", "--subject", r.Subject, "--statement", "A newer lesson.", "--supersedes", id)
	h.commit("supersede invalid evidence immediately")
	for _, args := range [][]string{{"evidence", "--check", "--base", base}, {"check", "--base", base}, {"reconcile", "--baseline-generate"}, {"reconcile", "--check"}, {"verify", "--offline"}} {
		if code, out, errout := h.run(args...); code != exitRefused || !strings.Contains(out+errout, "missing-durable-evidence") {
			t.Fatalf("%v laundered evidence: %d %s %s", args, code, out, errout)
		}
	}
	env, out, errb := h.env("")
	if code := RunGate(context.Background(), []string{"--base", base}, env); code != exitRefused || !strings.Contains(out.String()+errb.String(), "missing-durable-evidence") {
		t.Fatalf("shared gate: %d %s %s", code, out, errb)
	}
}

func TestEvidencePolicySyncRefusesUnboundDraft(t *testing.T) {
	t.Parallel()
	h, base := partitionedPolicyFixture(t)
	h.git("checkout", "-qb", "topic")
	id := h.add("--kind", "Learning", "--subject", "learning/unbound-policy", "--statement", "Retain exact source evidence.")
	r, _ := readEvidenceRecord(t, h, id)
	r.Provenance.Source = "README.md"
	r.Provenance.Commit = base
	writeEvidenceRecord(t, h, r)
	h.commit("unbound source evidence")
	before := policyFilesOnDisk(t, h)
	if code, out, errout := h.run("reconcile", "--baseline-sync", "--baseline-ref", base); code != exitRefused || !strings.Contains(out+errout, "missing-durable-evidence") {
		t.Fatalf("policy synchronization accepted unbound evidence: %d %s %s", code, out, errout)
	}
	if !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
		t.Fatal("refused evidence changed policy")
	}
	h.ok("evidence", "--base", base, "--source", base, "--record", id, "--write")
	h.commit("bind the exact retained evidence")
	h.ok("reconcile", "--baseline-sync", "--baseline-ref", base)
	if !reflect.DeepEqual(before, policyFilesOnDisk(t, h)) {
		t.Fatal("valid evidence changed unchanged debt policy")
	}
}
