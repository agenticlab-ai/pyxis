package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func frozenWitnessFixture(t *testing.T, kind string) (*harness, string, string, []byte) {
	t.Helper()
	h := initRepo(t)
	h.ok("init")
	writeFixtureFile(t, h.dir, "internal/demo/demo.go", "package demo\nconst Value = 1\n", 0o644)
	h.commit("original source")
	before := h.git("rev-parse", "HEAD")
	subject := "learning/example"
	if kind == record.Decision {
		subject = "design/example"
	}
	args := []string{"--kind", kind, "--subject", subject, "--statement", "A source-dependent constraint.", "--source", "README.md", "--tag", "code:internal/demo"}
	if kind == record.Decision {
		args = append(args, "--family", "constraint")
	}
	id := h.add(args...)
	_, raw := readEvidenceRecord(t, h, id)
	writeFixtureFile(t, h.dir, "internal/demo/demo.go", "package demo\nconst Value = 2\n", 0o644)
	h.commit("source changed before record introduction")
	if code, _, _ := h.run("evidence", "--check", "--base", h.git("rev-parse", "HEAD")); code != exitRefused {
		t.Fatal("fixture did not reproduce a failed frozen witness")
	}
	return h, before, id, raw
}

func TestEvidenceFrozenLearningVerifiedSuccessorPreservesHistory(t *testing.T) {
	h, _, id, raw := frozenWitnessFixture(t, record.Learning)
	base := h.git("rev-parse", "HEAD")
	r, _ := readEvidenceRecord(t, h, id)
	successor := h.add("--kind", record.Learning, "--subject", r.Subject, "--statement", r.Statement, "--source", "README.md", "--tag", "code:internal/demo", "--supersedes", id)
	h.commit("replace failed learning with verified successor")
	var report evidenceReport
	decodeJSON(t, h.ok("evidence", "--check", "--base", base, "--json"), &report)
	assertRecoveredWitness(t, report, id, successor, base)
	_, after := readEvidenceRecord(t, h, id)
	if !bytes.Equal(raw, after) {
		t.Fatal("successor repair changed frozen record bytes")
	}
	env, _, _ := h.env("")
	m, err := open(t.Context(), env)
	if err != nil {
		t.Fatal(err)
	}
	p := resolve.New(m.set)
	if p.Status(id) != resolve.Superseded || p.Status(successor) != resolve.Active {
		t.Fatal("failed predecessor remains active")
	}
	clone := filepath.Join(t.TempDir(), "fresh")
	gitIn(t, t.TempDir(), "clone", "--quiet", "--no-local", "--single-branch", "--no-tags", "--branch", "main", "file://"+h.dir, clone)
	fresh := newHarness(t, clone)
	decodeJSON(t, fresh.ok("evidence", "--check", "--base", fresh.git("rev-parse", "HEAD"), "--json"), &report)
	assertRecoveredWitness(t, report, id, successor, base)
	_, retained := readEvidenceRecord(t, fresh, id)
	if !bytes.Equal(raw, retained) {
		t.Fatal("fresh clone lost the original failed witness")
	}
	if out := fresh.ok("evidence", "--check", "--base", fresh.git("rev-parse", "HEAD")); !strings.Contains(out, "2 historical-unresolved") || !strings.Contains(out, "historical witness remains unresolved") {
		t.Fatal("human report hid unresolved evidence", out)
	}
	h.add("--kind", record.Learning, "--subject", r.Subject, "--statement", "Later verified guidance.", "--source", "README.md", "--tag", "code:internal/demo", "--supersedes", successor)
	h.commit("ordinary later successor")
	decodeJSON(t, h.ok("evidence", "--check", "--base", base, "--json"), &report)
	assertRecoveredWitness(t, report, id, successor, base)
}

func assertRecoveredWitness(t *testing.T, report evidenceReport, id, successor, intro string) {
	t.Helper()
	unresolved, verified := 0, 0
	for _, e := range report.Entries {
		switch e.RecordID {
		case id:
			if e.Classification != "historical-unresolved" || e.Snapshot != intro || !strings.Contains(e.Recovery, successor) || (e.Expected == nil && e.ExpectedUnit == nil) || (e.Observed == nil && e.ObservedUnit == nil) {
				t.Fatalf("original defect hidden or certified: %+v", e)
			}
			unresolved++
		case successor:
			if e.Classification != "verified" {
				t.Fatalf("unverified successor accepted: %+v", e)
			}
			verified++
		}
	}
	if unresolved != 2 || verified != 1 || report.blocking() {
		t.Fatalf("bad recovery report: unresolved=%d verified=%d blocking=%v", unresolved, verified, report.blocking())
	}
}

func TestEvidenceSuccessorCannotHideOtherBlockingDefects(t *testing.T) {
	for _, mode := range []string{"new bad record", "unbound successor", "bad successor", "rewritten predecessor", "malformed predecessor", "invalid relation", "duplicate successor", "revoked successor", "decision"} {
		t.Run(mode, func(t *testing.T) {
			kind := record.Learning
			if mode == "decision" {
				kind = record.Decision
			}
			h, before, id, raw := frozenWitnessFixture(t, kind)
			base := h.git("rev-parse", "HEAD")
			if mode == "new bad record" {
				base = before
			}
			r, _ := readEvidenceRecord(t, h, id)
			args := []string{"--kind", kind, "--subject", r.Subject, "--statement", r.Statement, "--supersedes", id}
			if r.Family != "" {
				args = append(args, "--family", r.Family)
			}
			if mode != "unbound successor" {
				args = append(args, "--source", "README.md", "--tag", "code:internal/demo")
			}
			successor := h.add(args...)
			switch mode {
			case "bad successor":
				writeFixtureFile(t, h.dir, "internal/demo/demo.go", "package demo\nconst Value = 3\n", 0o644)
			case "rewritten predecessor":
				r.Statement += " rewritten"
				writeEvidenceRecord(t, h, r)
			case "malformed predecessor":
				bad := bytes.ReplaceAll(raw, []byte(record.EvidenceSchema), []byte("unknown-evidence-schema"))
				writeFixtureFile(t, h.dir, store.Dir+"/"+r.Path(), string(bad), 0o644)
			case "invalid relation", "duplicate successor":
				next, _ := readEvidenceRecord(t, h, successor)
				priorPath := filepath.Join(h.dir, store.Dir, next.Path())
				next.Subject = "learning/unrelated"
				writeEvidenceRecord(t, h, next)
				if mode == "invalid relation" {
					if err := os.Remove(priorPath); err != nil {
						t.Fatal(err)
					}
				}
			case "revoked successor":
				h.ok("revoke", "--reason", "Withdraw replacement.", successor)
			}
			h.commit("candidate with an insufficient replacement")
			code, out, _ := h.run("evidence", "--check", "--base", base, "--json")
			if code != exitRefused || strings.Contains(out, `"classification": "historical-unresolved"`) {
				t.Fatalf("%s hid a blocking defect: %d %s", mode, code, out)
			}
			if mode != "rewritten predecessor" && mode != "malformed predecessor" {
				retained, err := os.ReadFile(filepath.Join(h.dir, store.Dir, r.Path()))
				if err != nil || !bytes.Equal(raw, retained) {
					t.Fatal("failed check changed predecessor", err)
				}
			}
		})
	}
}
