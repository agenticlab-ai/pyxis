package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const miniBase = `# t — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v1 |

## 5. R

### 5.1 F

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |
`

func writeLedger(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func wantDefect(t *testing.T, defects []schema.Defect, reason string) {
	t.Helper()
	for _, d := range defects {
		if strings.Contains(d.Reason, reason) {
			return
		}
	}
	t.Errorf("no defect containing %q (got %v)", reason, defects)
}

func TestLintStructure(t *testing.T) {
	dir := writeLedger(t, map[string]string{"000-base.md": miniBase, "002-late.md": "x"})
	entries, _ := gate.ReadEntries(dir)
	wantDefect(t, gate.LintStructure(dir, entries), "numbering gap")

	dir2 := writeLedger(t, map[string]string{"001-a.md": "x"})
	entries2, _ := gate.ReadEntries(dir2)
	wantDefect(t, gate.LintStructure(dir2, entries2), "missing 000-base.md")
}

func TestVerifyGeneratedRejectsHandEdits(t *testing.T) {
	dir := writeLedger(t, map[string]string{"000-base.md": miniBase})
	entries, _ := gate.ReadEntries(dir)
	res := fold.Fold(entries)
	if len(res.Defects) != 0 {
		t.Fatalf("unexpected defects: %v", res.Defects)
	}
	// Missing projection → defect.
	wantDefect(t, gate.VerifyGenerated(dir, res), "projection artifact missing")
	// Correct projection → clean.
	if err := os.WriteFile(filepath.Join(dir, "effective.md"), res.EffectiveMD, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "effective.source.json"), res.SourceJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	if d := gate.VerifyGenerated(dir, res); len(d) != 0 {
		t.Fatalf("clean projection flagged: %v", d)
	}
	// Hand-edited projection → defect naming the artifact.
	if err := os.WriteFile(filepath.Join(dir, "effective.md"), append(res.EffectiveMD, []byte("helpful fix\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	wantDefect(t, gate.VerifyGenerated(dir, res), "differs from the recomputed fold")
}

func TestVerifyRebase(t *testing.T) {
	baseEntry := ledger.Entry{Name: "000-base.md", Bytes: []byte(miniBase)}
	faithful := ledger.Entry{Name: "001-rebase.md", Bytes: []byte(miniBase)}
	if d := gate.VerifyRebase([]ledger.Entry{baseEntry, faithful}); len(d) != 0 {
		t.Fatalf("faithful rebase flagged: %v", d)
	}
	tampered := ledger.Entry{Name: "001-rebase.md", Bytes: []byte(strings.Replace(miniBase,
		"| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |",
		"| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |\n| FR-9 | smuggled | r | Must | med | f | Test | G1 |", 1))}
	wantDefect(t, gate.VerifyRebase([]ledger.Entry{baseEntry, tampered}), "not semantically identical")
}

func TestGateUmbrellaOnRealRepo(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	// Load the repo's own config (not Defaults): the real-repo gate mirrors the
	// CLI, so config-driven surfaces — the grandfather/exemption lists a
	// config-consuming check like check-citations reads — are the real ones.
	cfg, cdefs := config.Load(root)
	if len(cdefs) != 0 {
		t.Fatalf("real repo config parse: %v", cdefs)
	}
	dirs, err := gate.LedgerDirs(root, cfg)
	if err != nil {
		t.Fatalf("LedgerDirs: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("expected at least the docgate ledger directory")
	}
	// NR-1: the gate examines ledger directories only — and the real repo,
	// including its own regenerated projection, is clean (DOD-5 evidence).
	if defects := gate.Gate(root, "", dirs, cfg); len(defects) != 0 {
		t.Fatalf("real repo gate red: %v", defects)
	}
}

func TestGoverningAndIncidents(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("docs/architecture-principles.md", "**AP-1 — x.** body\n")
	write("docs/observability/incidents.md", "## Entries\n\n**I1 (2026-07-07) [t]**\n\n- **Decision:** d\n- **Broke:** b\n- **Lesson:** l\n- **Links:** #1\n")
	run("add", ".")
	run("commit", "-qm", "base")
	// Untrailed governing change → red.
	write("docs/architecture-principles.md", "**AP-1 — x.** body\n\n**AP-2 — y.** body\n")
	run("add", ".")
	run("commit", "-qm", "sneaky append")
	wantDefect(t, gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()), "without the `Governing-Change: operator` trailer")
	// Trailed append → green.
	run("commit", "-q", "--amend", "-m", "append principle\n\nGoverning-Change: operator")
	if d := gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()); len(d) != 0 {
		t.Fatalf("trailed append flagged: %v", d)
	}
	// Trailed EDIT of principles → red (append-only even with trailer).
	write("docs/architecture-principles.md", "**AP-1 — REWRITTEN.** body\n\n**AP-2 — y.** body\n")
	run("add", ".")
	run("commit", "-qm", "edit\n\nGoverning-Change: operator")
	wantDefect(t, gate.CheckGoverning(root, "HEAD~1", "", config.Defaults()), "append-only")
	// Incidents: the FIRST entry replacing the seeded placeholder → green (the I1 bug).
	write("docs/observability/incidents.md", "## Entries\n\n*(none yet — I1 is allocated by the first captured incident.)*\n")
	run("add", ".")
	run("commit", "-qm", "seed placeholder")
	write("docs/observability/incidents.md", "## Entries\n\n**I1 (2026-07-07) [t]**\n\n- **Decision:** d\n- **Broke:** b\n- **Lesson:** l\n- **Links:** #1\n")
	run("add", ".")
	run("commit", "-qm", "first incident")
	cfg := config.Defaults()
	if d := gate.CheckIncidentsAppend(root, "HEAD~1", cfg); len(d) != 0 {
		t.Fatalf("placeholder replacement flagged: %v", d)
	}
	// Graduation: the Links line grows append-only → green.
	write("docs/observability/incidents.md", "## Entries\n\n**I1 (2026-07-07) [t]**\n\n- **Decision:** d\n- **Broke:** b\n- **Lesson:** l\n- **Links:** #1 Graduated: AP-99\n")
	run("add", ".")
	run("commit", "-qm", "graduate")
	if d := gate.CheckIncidentsAppend(root, "HEAD~1", cfg); len(d) != 0 {
		t.Fatalf("graduation flagged: %v", d)
	}
	// Links REWRITE (not growth) → red.
	write("docs/observability/incidents.md", "## Entries\n\n**I1 (2026-07-07) [t]**\n\n- **Decision:** d\n- **Broke:** b\n- **Lesson:** l\n- **Links:** #2\n")
	run("add", ".")
	run("commit", "-qm", "links rewrite")
	wantDefect(t, gate.CheckIncidentsAppend(root, "HEAD~1", cfg), "modified")
	run("reset", "-q", "--hard", "HEAD~1")
	// Deleting a committed entry → red.
	write("docs/observability/incidents.md", "## Entries\n")
	run("add", ".")
	run("commit", "-qm", "delete entry")
	wantDefect(t, gate.CheckIncidentsAppend(root, "HEAD~1", cfg), "deleted")
	run("reset", "-q", "--hard", "HEAD~1")
	// Incidents: tamper → red; shape lint catches gaps and missing fields.
	write("docs/observability/incidents.md", "## Entries\n\n**I1 (2026-07-07) [t]**\n\n- **Decision:** REWRITTEN\n- **Broke:** b\n- **Lesson:** l\n- **Links:** #1\n")
	run("add", ".")
	run("commit", "-qm", "tamper")
	wantDefect(t, gate.CheckIncidentsAppend(root, "HEAD~1", cfg), "modified")
	write("docs/observability/incidents.md", "## Entries\n\n**I2 (2026-07-07) [t]**\n\n- **Decision:** d\n- **Broke:** b\n- **Links:** #1\n")
	wantDefect(t, gate.LintIncidents(root, cfg), "missing field Lesson")
	wantDefect(t, gate.LintIncidents(root, cfg), "IDs not dense")
}

func TestCheckImmutableDesigns(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	run("init", "-q", "-b", "main")
	write("docs/tech-design/frozen.md", "# frozen design\n")
	write("docs/tech-design/index.md", "# index v1\n")
	write("docs/prds/f/000-base.md", "# base\n")
	run("add", ".")
	run("commit", "-qm", "base")
	// Modify a merged design → red naming supersession.
	write("docs/tech-design/frozen.md", "# frozen design REWRITTEN\n")
	run("add", ".")
	run("commit", "-qm", "rewrite")
	wantDefect(t, gate.CheckImmutable(root, "HEAD~1", "", cfg), "merged designs are immutable")
	run("reset", "-q", "--hard", "HEAD~1")
	// Delete → red.
	run("rm", "-q", "docs/tech-design/frozen.md")
	run("commit", "-qm", "delete")
	wantDefect(t, gate.CheckImmutable(root, "HEAD~1", "", cfg), "deleted")
	run("reset", "-q", "--hard", "HEAD~1")
	// New design in the PR (absent at base) → green; index.md rewrite → green.
	write("docs/tech-design/new-design.md", "# draft\n")
	write("docs/tech-design/index.md", "# index v2 regenerated\n")
	run("add", ".")
	run("commit", "-qm", "new design + index regen")
	if d := gate.CheckImmutable(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("draft/index changes flagged: %v", d)
	}
	// PRD fence regression guard: entry modify still red.
	write("docs/prds/f/000-base.md", "# base REWRITTEN\n")
	run("add", ".")
	run("commit", "-qm", "tamper ledger")
	wantDefect(t, gate.CheckImmutable(root, "HEAD~1", "", cfg), "merged entries are immutable")
}

func TestCheckSingleAmendment(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	run("init", "-q", "-b", "main")
	// Fixture commits must not leave detached maintenance racing TempDir cleanup.
	run("config", "gc.auto", "0")
	run("config", "maintenance.auto", "false")
	write("docs/prds/docgate/000-base.md", "# base\n")
	write("docs/prds/docgate/effective.md", "# proj\n")
	run("add", ".")
	run("commit", "-qm", "base")

	// Exactly one amendment (a projection regen alongside does not count) → green.
	write("docs/prds/docgate/006-one.md", "# amendment\n")
	write("docs/prds/docgate/effective.md", "# proj v2\n")
	run("add", ".")
	run("commit", "-qm", "one amendment")
	if d := gate.CheckSingleAmendment(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("single amendment flagged: %v", d)
	}

	// A second amendment in the same ledger (two added entries) → red naming both.
	write("docs/prds/docgate/007-two.md", "# amendment\n")
	run("add", ".")
	run("commit", "-qm", "second amendment")
	d := gate.CheckSingleAmendment(root, "HEAD~2", "", cfg)
	wantDefect(t, d, "one amendment per PR")
	wantDefect(t, d, "006-one.md")
	wantDefect(t, d, "007-two.md")
	if len(d) == 0 || !strings.Contains(d[0].Remedy, "each PR adds exactly one ledger entry") {
		t.Fatalf("remedy clause not surfaced: %v", d)
	}
	run("reset", "-q", "--hard", "HEAD~1")

	// Two entries across two different ledgers (the PR #56 shape) → red. The new
	// ledger's generated projection is added alongside and must not be counted.
	write("docs/prds/incident-machinery/000-base.md", "# base\n")
	write("docs/prds/incident-machinery/effective.md", "# proj\n")
	write("docs/prds/incident-machinery/effective.source.json", "{}\n")
	run("add", ".")
	run("commit", "-qm", "new second ledger")
	d = gate.CheckSingleAmendment(root, "HEAD~2", "", cfg)
	wantDefect(t, d, "one amendment per PR")
	wantDefect(t, d, "docgate/006-one.md")
	wantDefect(t, d, "incident-machinery/000-base.md")
	// A new PRD's lone 000-base over the preceding state → green (one entry).
	if d := gate.CheckSingleAmendment(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("lone new-PRD base flagged: %v", d)
	}

	// The flat placeholder corpus is never counted, even when NNN-named (NG-1):
	// one flat file plus one real amendment is still one amendment → green.
	write("docs/prds/000-flat.md", "# placeholder\n")
	write("docs/prds/docgate/007-three.md", "# amendment\n")
	run("add", ".")
	run("commit", "-qm", "flat placeholder + one amendment")
	if d := gate.CheckSingleAmendment(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("flat placeholder counted or single amendment flagged: %v", d)
	}

	// Modifications don't count — only additions do (--diff-filter=A): rewriting
	// a committed entry alongside adding one new entry stays green.
	write("docs/prds/docgate/000-base.md", "# base rewritten\n")
	write("docs/prds/docgate/008-four.md", "# amendment\n")
	run("add", ".")
	run("commit", "-qm", "modify base + one amendment")
	if d := gate.CheckSingleAmendment(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("modified entry counted or single amendment flagged: %v", d)
	}

	// A non-ASCII feature-dir name must still be counted: git octal-escapes and
	// double-quotes such paths under core.quotepath, so a raw `--name-only`
	// parse would miss them and fail open. -z (unquoted) keeps the count honest.
	write("docs/prds/aççent/012-x.md", "# amendment\n")
	write("docs/prds/aççent/013-y.md", "# amendment\n")
	run("add", ".")
	run("commit", "-qm", "two amendments under a non-ascii ledger dir")
	dUnicode := gate.CheckSingleAmendment(root, "HEAD~1", "", cfg)
	wantDefect(t, dUnicode, "one amendment per PR")
	wantDefect(t, dUnicode, "012-x.md")
	wantDefect(t, dUnicode, "013-y.md")
}

func TestCheckStandaloneDocs(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	run("init", "-q", "-b", "main")
	// Fixture commits must not leave detached maintenance racing TempDir cleanup.
	run("config", "gc.auto", "0")
	run("config", "maintenance.auto", "false")
	write("docs/prds/docgate/000-base.md", "# base\n")
	run("add", ".")
	run("commit", "-qm", "base")

	// A lone amendment + its generated projection → green (one class, no code).
	write("docs/prds/docgate/001-a.md", "# amendment\n")
	write("docs/prds/docgate/effective.md", "# proj\n")
	write("docs/prds/docgate/effective.source.json", "{}\n")
	run("add", ".")
	run("commit", "-qm", "amendment")
	if d := gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("lone amendment flagged: %v", d)
	}

	// A lone design + its generated index.* → green.
	write("docs/tech-design/d.md", "# design\n")
	write("docs/tech-design/index.md", "# idx\n")
	write("docs/tech-design/index.json", "{}\n")
	run("add", ".")
	run("commit", "-qm", "design")
	if d := gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("lone design flagged: %v", d)
	}

	// Pure code + rollup → green (no governed class).
	write("internal/x/x.go", "package x")
	write("docs/techspec.md", "spec")
	run("add", ".")
	run("commit", "-qm", "impl")
	if d := gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg); len(d) != 0 {
		t.Fatalf("code+rollup flagged: %v", d)
	}

	// A governed class + code → red (the doc↔code bundle).
	write("docs/prds/docgate/002-b.md", "# amendment\n")
	write("internal/x/y.go", "package x")
	run("add", ".")
	run("commit", "-qm", "amendment + code")
	wantDefect(t, gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg), "implementation code")

	// Two governed classes → red (the PR #82 shape: amendment + design).
	write("docs/prds/docgate/003-c.md", "# amendment\n")
	write("docs/tech-design/e.md", "# design\n")
	run("add", ".")
	run("commit", "-qm", "amendment + design")
	d := gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg)
	wantDefect(t, d, "governed doc classes")
	wantDefect(t, d, "PRD ledger")
	wantDefect(t, d, "technical design")

	// Incident registry + code → red.
	write("docs/observability/incidents.md", "## Entries\n")
	write("internal/x/z.go", "package x")
	run("add", ".")
	run("commit", "-qm", "incident + code")
	wantDefect(t, gate.CheckStandaloneDocs(root, "HEAD~1", "", cfg), "incident registry")

	// Generated memory reports no longer receive a document-only exemption,
	// including the legacy writer's content-addressed names.
	digest := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, path string
		code       bool
	}{
		{"generated report", ".pyxis/evidence/memory/" + digest + ".json", false},
		{"report plus code", ".pyxis/evidence/memory/" + digest + ".json", true},
		{"unaddressed JSON", ".pyxis/evidence/memory/report.json", false},
		{"short digest", ".pyxis/evidence/memory/" + digest[1:] + ".json", false},
		{"uppercase digest", ".pyxis/evidence/memory/" + strings.Repeat("B", 64) + ".json", false},
		{"Go source", ".pyxis/evidence/memory/" + digest + ".go", false},
		{"suffixed source", ".pyxis/evidence/memory/" + digest + ".json.go", false},
		{"nested report", ".pyxis/evidence/memory/nested/" + digest + ".json", false},
		{"neighboring home", ".pyxis/evidence/memory-other/" + digest + ".json", false},
		{"other producer", ".pyxis/evidence/" + digest + ".json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write("docs/tech-design/e.md", "# "+tc.name+"\n")
			write(tc.path, "{\"case\":\""+tc.name+"\"}\n")
			if tc.code {
				write("internal/x/evidence.go", "package x\n")
			}
			run("add", "docs/tech-design/e.md", tc.path)
			if tc.code {
				run("add", "internal/x/evidence.go")
			}
			run("commit", "-qm", tc.name)
			for _, head := range []string{"", "HEAD"} {
				d := gate.CheckStandaloneDocs(root, "HEAD~1", head, cfg)
				wantDefect(t, d, "implementation code")
			}
		})
	}
}

func TestWikiRollupLintOnRealRepo(t *testing.T) {
	root, _ := filepath.Abs("../../..")
	cfg := config.Defaults()
	if d := gate.LintWiki(root, cfg); len(d) != 0 {
		t.Fatalf("real wiki flagged: %v", d)
	}
	if d := gate.LintRollups(root, cfg); len(d) != 0 {
		t.Fatalf("real rollups flagged: %v", d)
	}
	if d := gate.LintTechDesigns(root, "", "", cfg); len(d) != 0 {
		t.Fatalf("real design corpus flagged: %v", d)
	}
	if d := gate.VerifyIndex(root, cfg); len(d) != 0 {
		t.Fatalf("real index flagged: %v", d)
	}
}

func TestLintWikiReportsIncompleteRoutingHeader(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "wiki")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "entry.md"), []byte("# Entry\n\n| Field | Value |\n|---|---|\n| **Last reviewed** | someday |\n| **Authority** | binding |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defects := gate.LintWiki(root, config.Defaults())
	wantDefect(t, defects, "missing header row")
	wantDefect(t, defects, "no YYYY-MM-DD date")
	wantDefect(t, defects, "does not declare the entry descriptive")
}
