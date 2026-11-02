package cli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestEvidenceRootMergeIntroductionAndReintroduction(t *testing.T) {
	t.Parallel()
	hermeticGit(t)
	h := newHarness(t, t.TempDir())
	h.git("init", "-q", "-b", "main")
	body := "retained root bytes\r\n"
	writeFixtureFile(t, h.dir, "README.md", body, 0o644)
	r := memoryTestRecord(record.Learning, "learning/root", "", "Root evidence.", 1)
	r.Provenance.Source = "README.md"
	r.Provenance.Evidence = &record.Evidence{Schema: record.EvidenceSchema, Locator: record.EvidenceIntroduction, Files: []record.EvidenceFile{{Path: "README.md", Mode: "100644", SHA256: fmtDigest([]byte(body))}}}
	writeEvidenceRecord(t, h, r)
	h.commit("root with record and evidence")
	h.ok("evidence", "--check")
	h.git("checkout", "-qb", "side")
	r2 := *r
	r2.ID = memoryTestRecord(record.Learning, r.Subject, "", "Merged evidence.", 2).ID
	r2.Statement = "Merged evidence."
	writeEvidenceRecord(t, h, &r2)
	h.commit("side addition")
	h.git("checkout", "main")
	h.git("merge", "--no-ff", "-qm", "merge addition", "side")
	merged := h.git("rev-parse", "HEAD")
	var report evidenceReport
	decodeJSON(t, h.ok("evidence", "--check", "--json"), &report)
	if report.Entries[1].Snapshot != merged {
		t.Fatalf("merge introduction used side commit: %+v", report)
	}
	rPath := store.Dir + "/" + r.Path()
	h.git("rm", rPath)
	h.commit("remove immutable record")
	writeEvidenceRecord(t, h, r)
	h.commit("reintroduce same bytes")
	if code, out, _ := h.run("evidence", "--check", "--json"); code != exitRefused || !strings.Contains(out, "evidence-record-rewritten") {
		t.Fatalf("remove/reintroduce passed: %d %s", code, out)
	}
}

func fmtDigest(body []byte) string {
	sum := sha256.Sum256(body)
	const digits = "0123456789abcdef"
	out := make([]byte, 64)
	for i, b := range sum {
		out[i*2] = digits[b>>4]
		out[i*2+1] = digits[b&15]
	}
	return string(out)
}

func draftRepairFixture(t *testing.T) (*harness, string, []string) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	h.commit("memory base")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "topic")
	writeFixtureFile(t, h.dir, "README.md", "new evidence\n", 0o644)
	h.commit("topic evidence")
	original := h.git("rev-parse", "HEAD")
	var ids []string
	for _, name := range []string{"a", "b"} {
		id := h.add("--kind", "Learning", "--subject", "learning/"+name, "--statement", "Exact evidence.", "--source", "README.md")
		r, _ := readEvidenceRecord(t, h, id)
		r.Provenance.Evidence = nil
		r.Provenance.Commit = original
		writeEvidenceRecord(t, h, r)
		ids = append(ids, id)
	}
	h.commit("legacy draft references")
	return h, base, ids
}

func TestEvidenceRepairGuardsAndPartialRetry(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"changed file", "changed head", "changed base", "original missing", "original changed", "partial write", "partial head"} {
		t.Run(mode, func(t *testing.T) {
			h, base, ids := draftRepairFixture(t)
			env, stdout, _ := h.env("")
			m, err := open(t.Context(), env)
			if err != nil {
				t.Fatal(err)
			}
			s, err := newEvidenceSession(t.Context(), m, "main")
			if err != nil {
				t.Fatal(err)
			}
			_, before := readEvidenceRecord(t, h, ids[0])
			switch mode {
			case "changed file":
				r, _ := readEvidenceRecord(t, h, ids[1])
				r.Statement = "Changed by another writer."
				writeEvidenceRecord(t, h, r)
			case "changed head":
				h.git("commit", "--allow-empty", "-qm", "race")
			case "changed base":
				h.git("update-ref", "refs/heads/main", s.head)
			case "original missing":
				m.set.Get(ids[1]).Provenance.Commit = strings.Repeat("e", 40)
			case "original changed":
				m.set.Get(ids[1]).Provenance.Commit = base
			case "partial write":
				s.replaceDraft = func(root string, r *record.Record, expected []byte) error {
					if r.ID == ids[1] {
						return errors.New("injected disk failure")
					}
					return store.ReplaceDraftEvidence(root, r, expected)
				}
			case "partial head":
				s.replaceDraft = func(root string, r *record.Record, expected []byte) error {
					if err := store.ReplaceDraftEvidence(root, r, expected); err != nil {
						return err
					}
					h.git("commit", "--allow-empty", "-qm", "candidate moved during repair")
					return nil
				}
			}
			if mode == "changed head" || mode == "changed base" {
				found := false
				for _, entry := range s.verify(t.Context()).Entries {
					found = found || entry.Classification == "history-unavailable"
				}
				if !found {
					t.Fatal("verification accepted moved input pins")
				}
			}
			code, err := repairEvidence(t.Context(), s, ids, "", true, true, env)
			if code != exitRefused || err == nil {
				t.Fatalf("%s passed: %d %v", mode, code, err)
			}
			_, after := readEvidenceRecord(t, h, ids[0])
			if !strings.HasPrefix(mode, "partial ") {
				if !bytes.Equal(before, after) {
					t.Fatal("failed preflight wrote a draft")
				}
				return
			}
			var out evidenceRepairOutput
			decodeJSON(t, stdout.String(), &out)
			if len(out.Applied) != 1 || len(out.Pending) != 1 || out.Applied[0] != ids[0] || out.Pending[0] != ids[1] {
				t.Fatalf("ambiguous partial result: %+v", out)
			}
			h.ok("evidence", "--base", base, "--record", ids[0], "--record", ids[1], "--write")
			h.ok("evidence", "--check", "--base", base)
		})
	}
}

func TestEvidenceLegacyClassificationAndHistoryFailures(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	base := h.git("rev-parse", "HEAD")
	for i, pin := range []string{base, strings.Repeat("e", 40)} {
		r := memoryTestRecord(record.Learning, "learning/legacy", "", "Legacy evidence.", i+1)
		r.Provenance.Source = "README.md"
		r.Provenance.Commit = pin
		writeEvidenceRecord(t, h, r)
	}
	h.commit("inherited legacy corpus")
	var report evidenceReport
	decodeJSON(t, h.ok("evidence", "--check", "--json"), &report)
	if report.Entries[0].Classification != "verified" || report.Entries[1].Classification != "legacy-unverified" {
		t.Fatalf("legacy coverage: %+v", report)
	}
	if code, _, errout := h.run("evidence", "--record", report.Entries[0].RecordID, "--write"); code != exitRefused || !strings.Contains(errout, "frozen") {
		t.Fatalf("base record repair: %d %s", code, errout)
	}
	clone := filepath.Join(t.TempDir(), "shallow")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--depth=1", "file://"+h.dir, clone)
	if code, out, errout := newHarness(t, clone).run("evidence", "--check"); code != exitRefused || !strings.Contains(out, "evidence-history-unavailable") {
		t.Fatalf("shallow history passed: %d %s %s", code, out, errout)
	}
	// A missing blob cannot be supplied by a promisor fetch. The Git adapter
	// disables lazy fetching and all transport protocols for evidence reads.
	h.git("checkout", "-qb", "topic")
	h.clock = t0.AddDate(0, 0, 1)
	id := h.add("--kind", "Learning", "--subject", "learning/new", "--statement", "New evidence.", "--source", "README.md")
	h.commit("typed evidence")
	blob := h.git("rev-parse", "HEAD:README.md")
	if err := os.Remove(filepath.Join(h.dir, ".git/objects", blob[:2], blob[2:])); err != nil {
		t.Fatal(err)
	}
	if code, out, errout := h.run("evidence", "--check", "--json"); code != exitRefused || !strings.Contains(out+errout, "evidence-history-unavailable") || !strings.Contains(out, id) {
		t.Fatalf("missing object passed: %d %s %s", code, out, errout)
	}
}

func TestEvidenceSiblingCanOnlyBeReboundByExactBytes(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.commit("base")
	base := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "sibling")
	writeFixtureFile(t, h.dir, "README.md", "same source bytes\n", 0o644)
	h.commit("sibling source")
	sibling := h.git("rev-parse", "HEAD")
	h.git("checkout", "-qb", "reviewed", base)
	writeFixtureFile(t, h.dir, "README.md", "same source bytes\n", 0o644)
	h.commit("reviewed source")
	id := h.add("--kind", "Learning", "--subject", "learning/sibling", "--statement", "Exact repair.", "--source", "README.md")
	r, _ := readEvidenceRecord(t, h, id)
	r.Provenance.Commit = sibling
	r.Provenance.Evidence.Locator = record.EvidenceMainline
	writeEvidenceRecord(t, h, r)
	h.commit("invalid sibling locator")
	if code, out, _ := h.run("evidence", "--check", "--base", base, "--json"); code != exitRefused || !strings.Contains(out, "non-durable-evidence-commit") {
		t.Fatalf("sibling passed: %d %s", code, out)
	}
	h.ok("evidence", "--base", base, "--record", id, "--write")
	h.ok("evidence", "--check", "--base", base)
	h.commit("exact repaired binding")
	writeFixtureFile(t, h.dir, "README.md", "changed after capture\n", 0o644)
	h.commit("post-capture source change")
	if code, out, _ := h.run("evidence", "--check", "--base", base); code != exitRefused || !strings.Contains(out, "evidence-content-mismatch") {
		t.Fatalf("source drift passed: %d %s", code, out)
	}
	_, before := readEvidenceRecord(t, h, id)
	if code, _, errout := h.run("evidence", "--base", base, "--record", id, "--write"); code != exitRefused || !strings.Contains(errout, "existing witness differs") {
		t.Fatalf("stale evidence rebound: %d %s", code, errout)
	}
	_, after := readEvidenceRecord(t, h, id)
	if !bytes.Equal(before, after) {
		t.Fatal("failed repair changed record")
	}
}

func TestEvidenceCommitIdentityAndContradictoryWitnessCannotBeRebound(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	base := h.git("rev-parse", "HEAD")
	h.ok("init")
	h.commit("retain base")
	h.git("checkout", "-qb", "topic")
	id := h.add("--kind", "Learning", "--subject", "learning/identity", "--statement", "Historical commit identity.", "--commit", base)
	h.commit("identity reference")
	if code, _, errout := h.run("evidence", "--record", id, "--source", "main", "--write"); code != exitRefused || !strings.Contains(errout, "different commit identity") {
		t.Fatalf("identity changed: %d %s", code, errout)
	}
	id = h.add("--kind", "Learning", "--subject", "learning/contradiction", "--statement", "Exact historical source.", "--source", "README.md", "--commit", base)
	r, _ := readEvidenceRecord(t, h, id)
	r.Provenance.Evidence.Files[0].SHA256 = strings.Repeat("d", 64)
	writeEvidenceRecord(t, h, r)
	if code, _, errout := h.run("evidence", "--record", id, "--source", base, "--write"); code != exitRefused || !strings.Contains(errout, "original declared commit") {
		t.Fatalf("contradictory witness guessed: %d %s", code, errout)
	}
}
