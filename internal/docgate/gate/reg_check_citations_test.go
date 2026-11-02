package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// universe writes a standard resolution world into a fixture repo: one indexed
// design (slug `real-design`, anchors DINV-3 + A-1), a PRD ledger `feat` with a
// live FR-1 and a superseded FR-2, a flat corpus carrying VNEXT-1, and a
// brainstorm record `rec` with anchors D-1 + D-2. Citing docs resolve against
// exactly these. None of the universe files carry a `grammar:`-prefixed token,
// so scanning them adds no defect of their own.
func universe(write func(string, string)) {
	write("docs/tech-design/real-design.md",
		"# Real\n\n| Field | Value |\n|---|---|\n| **Status** | Approved |\n\n## 3. The design\n\nDINV-3 holds; A-1 verifies it.\n")
	write("docs/prds/feat/effective.source.json",
		`{"units":{"FR-1":{"status":"live"},"FR-2":{"status":"superseded"}}}`+"\n")
	write("docs/prds/corpus.md", "# Corpus\n\n- VNEXT-1 — a placeholder requirement.\n")
	write("docs/brainstorms/rec.md", "# Record\n\n### D-1 — first\n\nbody\n\n### D-2 — second\n\nbody\n")
}

// repoRoot walks up from the test's working directory to the module root
// (the go.mod dir) so A-6 can run over the live tree; if not found (an unusual
// environment) the own-corpus assertion skips rather than fails spuriously.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("module root (go.mod) not found; skipping own-corpus A-6")
		}
		dir = parent
	}
}

// A-1 (dangling reds with a named defect) + A-2 (present-or-superseded passes).
func TestCheckCitations_DanglingAndPresent(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)

	// Every grammar, each dangling → a defect naming file, line, token, universe.
	write("docs/citing.md", ""+
		"prd:feat/FR-99 is not a unit.\n"+ // unit absent from projection
		"design:nonesuch has no slug.\n"+ // slug absent
		"design:real-design#NOPE missing anchor.\n"+ // anchor absent
		"brainstorm:norecord#D-1 has no record.\n"+ // record absent
		"corpus:ZZ-9 is not a corpus ID.\n") // corpus ID absent
	defects := gate.CheckCitations(root, cfg, gate.ScanScope{})
	for _, want := range []string{"prd:feat/FR-99", "design:nonesuch", "design:real-design#NOPE", "brainstorm:norecord#D-1", "corpus:ZZ-9"} {
		wantDefect(t, defects, want)
	}
	for _, d := range defects {
		if d.Verb != "check-citations" {
			t.Fatalf("check emitted a non-own verb %q (DINV-5 parity): %+v", d.Verb, d)
		}
		if d.File == "" || d.Reason == "" || d.Remedy == "" {
			t.Fatalf("defect missing file/reason/remedy (A-1): %+v", d)
		}
	}

	// Present targets — live, superseded, whole-PRD, corpus, design anchor,
	// record anchor — all resolve green (A-2, DINV-1: superseded is present).
	write("docs/citing.md", ""+
		"prd:feat/FR-1 · prd:feat/FR-2 · prd:feat · "+ // live · superseded · whole-PRD
		"corpus:VNEXT-1 · design:real-design · design:real-design#DINV-3 · design:real-design#A-1 · "+
		"brainstorm:rec#D-1 · brainstorm:rec#D-2\n")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("present/superseded citations flagged (A-2): %v", d)
	}
}

// A-3: a citation inside an inline code span or a fenced block never fires.
func TestCheckCitations_CodeSpanAndFenceExempt(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)
	write("docs/citing.md", ""+
		"An example shown as `design:nonesuch#NOPE` never fires.\n\n"+
		"```\ndesign:also-nonesuch and prd:feat/FR-77 in a fence are examples\n```\n")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("code-span/fenced example fired (A-3, DINV-3): %v", d)
	}
}

// Compound anchors — a range (`D-1..#D-4`), a slash-list (`DINV-3/4/5`), and an
// embedded-# list — resolve on their lead component (firstAnchor, §3.3
// generalized); a compound whose LEAD is absent still reds.
func TestCheckCitations_CompoundAnchors(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)
	write("docs/citing.md", ""+
		"brainstorm:rec#D-1..#D-4 resolves on D-1.\n"+ // range, lead present, tail absent → resolves
		"design:real-design#DINV-3/4/5 resolves on DINV-3.\n"+ // slash-list, lead present
		"brainstorm:rec#D-2/#D-1 resolves on D-2.\n") // list, lead present
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("compound anchor with a present lead flagged: %v", d)
	}
	// A compound whose LEAD endpoint is absent reds (the lead is the checked one).
	write("docs/citing.md", "brainstorm:rec#D-9..#D-1 leads with an absent anchor.\n")
	wantDefect(t, gate.CheckCitations(root, cfg, gate.ScanScope{}), "brainstorm:rec#D-9..#D-1")
}

// docs/reviews holds reviewer commentary written in reading-shorthand that the
// §3.1 grammar does not admit; it is out of the scan. The same dangling token
// in an authored artifact is flagged.
func TestCheckCitations_ReviewsExcluded(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)
	write("docs/reviews/r.md", "design:Scope and prd:feat/000 are review shorthand.\n")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("a citation under docs/reviews was scanned: %v", d)
	}
	// Move the same dangling token into an authored PRD artifact → flagged.
	write("docs/prds/feat/000-base.md", "prd:feat/000 dangles here.\n")
	wantDefect(t, gate.CheckCitations(root, cfg, gate.ScanScope{}), "prd:feat/000")
}

// The grandfather list skips a listed (file, token) pre-existing dangler, and
// only that pair — the same token in another file still reds.
func TestCheckCitations_Grandfather(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)
	write("docs/frozen.md", "design:gone#D-5 is a frozen dangler.\n")
	write("docs/live.md", "design:gone#D-5 is a fresh dangler.\n")

	// Ungrandfathered: both red.
	if got := len(gate.CheckCitations(root, cfg, gate.ScanScope{})); got != 2 {
		t.Fatalf("want 2 danglers ungrandfathered, got %d", got)
	}
	// Grandfather only the frozen one → live.md still reds, frozen.md does not.
	cfg.GrandfatheredCitations = []string{"docs/frozen.md | design:gone#D-5"}
	defects := gate.CheckCitations(root, cfg, gate.ScanScope{})
	if len(defects) != 1 || defects[0].File != "docs/live.md" {
		t.Fatalf("grandfather not precise to (file, token): %v", defects)
	}
}

// A-5: the check runs base-free (no merge base in its signature) and mutates no
// scanned file.
func TestCheckCitations_BaseFreeNoWrite(t *testing.T) {
	root, _, write := fixtureRepo(t)
	cfg := config.Defaults()
	universe(write)
	const body = "prd:feat/FR-1 resolves.\n"
	write("docs/citing.md", body)
	_ = gate.CheckCitations(root, cfg, gate.ScanScope{})
	got, err := os.ReadFile(filepath.Join(root, "docs/citing.md"))
	if err != nil || string(got) != body {
		t.Fatalf("check mutated a scanned file (A-5, DINV-4): %q err=%v", string(got), err)
	}
}

// A-6: the live repository tree passes check-citations under its real config
// (the grandfather list included) — no false positive on any real citation.
func TestCheckCitations_OwnCorpusGreen(t *testing.T) {
	root := repoRoot(t)
	cfg, defs := config.Load(root)
	if len(defs) != 0 {
		t.Fatalf("live config did not parse cleanly: %v", defs)
	}
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("own-corpus check-citations is not green (A-6): %v", d)
	}
}

func TestCheckCitations_DraftBornPlanUsesPinnedLock(t *testing.T) {
	root, run, write := fixtureRepo(t)
	addRecord := func(kind, subject, family string, n int, body string) {
		t.Helper()
		at := time.Date(2026, 9, 28, 10, 0, n, 0, time.UTC)
		var entropy [10]byte
		entropy[9] = byte(n)
		r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, at, entropy), Kind: kind,
			Subject: subject, Family: family, Statement: "fixture " + family,
			Issuer: "fixture@example.test", CreatedAt: at.Format(time.RFC3339)}
		var err error
		if body == "" {
			_, err = store.Write(filepath.Join(root, store.Dir), r)
		} else {
			r.LiteralBody = true
			_, err = store.WriteDocument(filepath.Join(root, store.Dir), r, []byte(body))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	addRecord(record.Promise, "prd/widget", "FR-1", 1, "")
	addRecord(record.Decision, "design/widget", "D-1", 2, "")
	addRecord(record.Document, "prd/widget", "prd", 3, `# Widget PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v2 |

### 5.1 Functional requirements

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | The widget shall exist. | why | Must | low | fit | Test | D-1 |
`)
	addRecord(record.Document, "design/widget", "design", 4, "# Widget design\n\n### D-1 — model\n\n- **DINV-1** — closed model.\n")
	write("README.md", "base\n")
	run("add", "--", "README.md", "memory")
	run("commit", "-qm", "binding lock")
	run("tag", "-a", "lock/widget/v1", "-m", "binding lock v1")

	plan := "docs/plans/widget/001-plan.md"
	header := "# Widget plan\n\n| Field | Value |\n|---|---|\n| **Feature** | widget |\n| **PRD** | prd/widget |\n| **Design** | design/widget |\n| **Basis** | lock/widget/v1 |\n\n## Tasks\n\n"
	check := func(body string) []string {
		t.Helper()
		write(plan, header+body)
		var reasons []string
		for _, d := range gate.CheckCitationsInFiles(root, config.Defaults(), []string{filepath.Join(root, plan)}) {
			reasons = append(reasons, d.Reason)
		}
		return reasons
	}
	if got := check("prd:widget/FR-1 · design:widget#D-1 · design:widget#DINV-1 · design:widget\n"); len(got) != 0 {
		t.Fatalf("valid locked citations flagged: %v", got)
	}
	// Another grammar falls through to the ordinary resolvers.
	if got := check("brainstorm:widget#D-1\n"); len(got) != 1 || strings.Contains(got[0], "lock/widget") {
		t.Fatalf("foreign grammar resolved through the plan lock: %v", got)
	}
	got := check("prd:widget/FR-2 · design:widget#D-2 · prd:other/FR-1\n")
	if len(got) != 3 || !strings.Contains(got[0], "FR-2") || !strings.Contains(got[1], "D-2") || !strings.Contains(got[2], "other") {
		t.Fatalf("dangling locked or foreign citations accepted: %v", got)
	}
	got = check("design:widget#DINV-2\n")
	if len(got) != 1 || !strings.Contains(got[0], "DINV-2") {
		t.Fatalf("unknown locked design anchor accepted: %v", got)
	}
	// A newer working-tree document cannot change the meaning of v1.
	write("docs/plans/widget/001-plan.md", header+"prd:widget/FR-2\n")
	addRecord(record.Promise, "prd/widget", "FR-2", 5, "")
	if d := gate.CheckCitationsInFiles(root, config.Defaults(), []string{filepath.Join(root, plan)}); len(d) != 1 || !strings.Contains(d[0].Reason, "FR-2") {
		t.Fatalf("working-tree memory changed pinned citation result: %v", d)
	}
	// The exception is limited to plans with a matching lock header.
	write("docs/citing.md", "prd:widget/FR-1\n")
	if d := gate.CheckCitationsInFiles(root, config.Defaults(), []string{filepath.Join(root, "docs/citing.md")}); len(d) != 1 {
		t.Fatalf("ordinary document used plan lock: %v", d)
	}
	for _, tc := range []struct{ root, plan string }{
		{"docs/custom-plans", "docs/custom-plans/widget/001-plan.md"},
		{"./docs/custom-plans", "docs/custom-plans/widget/001-plan.md"},
		{"docs/../docs/custom-plans", "docs/custom-plans/widget/001-plan.md"},
		{".", "widget/001-plan.md"},
	} {
		cfg := config.Defaults()
		cfg.Plans = tc.root
		write(tc.plan, header+"prd:widget/FR-1\n")
		if d := gate.CheckCitationsInFiles(root, cfg, []string{filepath.Join(root, tc.plan)}); len(d) != 0 {
			t.Fatalf("configured plan root %q did not use lock: %v", tc.root, d)
		}
	}
	for _, dir := range []string{"docs/plans", "docs/custom-plans"} {
		if err := os.RemoveAll(filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/citing.md", "# no citations\n")
	cfg := config.Defaults()
	cfg.Plans = "contracts/plans"
	externalPlan := "contracts/plans/widget/001-plan.md"
	write("contracts/plans/README.md", "# Plans\n")
	write(externalPlan, header+"prd:widget/FR-2\n")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 1 || d[0].File != externalPlan || !strings.Contains(d[0].Reason, "FR-2") {
		t.Fatalf("tree-wide check missed dangling citation outside docs: %v", d)
	}
	write(externalPlan, header+"prd:widget/FR-1\n")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 0 {
		t.Fatalf("tree-wide check flagged valid citation outside docs: %v", d)
	}
	run("tag", "-d", "lock/widget/v1")
	if d := gate.CheckCitations(root, cfg, gate.ScanScope{}); len(d) != 1 || !strings.Contains(d[0].Reason, "resolve lock/widget/v1") {
		t.Fatalf("missing binding lock accepted: %v", d)
	}
	unversioned := t.TempDir()
	if err := os.MkdirAll(filepath.Join(unversioned, filepath.Dir(plan)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unversioned, plan), []byte(header+"prd:widget/FR-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if d := gate.CheckCitationsInFiles(unversioned, config.Defaults(), []string{filepath.Join(unversioned, plan)}); len(d) != 1 || !strings.Contains(d[0].Reason, "open repository") {
		t.Fatalf("lock resolved outside a repository: %v", d)
	}
}

// A plan moved onto the lock its own change prepares resolves through the
// records that change commits, until merging the change tags the lock.
func TestCheckCitations_DraftBornPlanReadsItsPendingLock(t *testing.T) {
	root, run, write := fixtureRepo(t)
	addRecord := func(kind, subject, family string, n int, body string, supersedes ...string) string {
		t.Helper()
		at := time.Date(2026, 9, 28, 10, 0, n, 0, time.UTC)
		var entropy [10]byte
		entropy[9] = byte(n)
		r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, at, entropy), Kind: kind,
			Subject: subject, Family: family, Statement: "fixture " + family,
			Issuer: "fixture@example.test", CreatedAt: at.Format(time.RFC3339)}
		for _, target := range supersedes {
			r.Relations = append(r.Relations, record.Relation{Type: record.Supersedes, Target: target})
		}
		var err error
		if body == "" {
			_, err = store.Write(filepath.Join(root, store.Dir), r)
		} else {
			r.LiteralBody = true
			_, err = store.WriteDocument(filepath.Join(root, store.Dir), r, []byte(body))
		}
		if err != nil {
			t.Fatal(err)
		}
		return r.ID
	}
	prd := func(rows ...string) string {
		var body strings.Builder
		body.WriteString("# Widget PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v2 |\n\n### 5.1 Functional requirements\n\n" +
			"| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |\n|---|---|---|---|---|---|---|---|\n")
		for _, id := range rows {
			body.WriteString("| " + id + " | The widget shall exist. | why | Must | low | fit | Test | D-1 |\n")
		}
		return body.String()
	}
	addRecord(record.Promise, "prd/widget", "FR-1", 1, "")
	addRecord(record.Decision, "design/widget", "D-1", 2, "")
	prdV1 := addRecord(record.Document, "prd/widget", "prd", 3, prd("FR-1"))
	designV1 := addRecord(record.Document, "design/widget", "design", 4, "# Widget design\n\n### D-1 — model\n")
	write("README.md", "base\n")
	run("add", "--", "README.md", "memory")
	run("commit", "-qm", "binding lock")
	run("tag", "-a", "lock/widget/v1", "-m", "binding lock v1")

	plan := "docs/plans/widget/001-plan.md"
	check := func(basis string) []string {
		t.Helper()
		write(plan, "# Widget plan\n\n| Field | Value |\n|---|---|\n| **Feature** | widget |\n| **PRD** | prd/widget |\n| **Design** | design/widget |\n| **Basis** | "+basis+" |\n\n## Tasks\n\nprd:widget/FR-2 · design:widget#D-2\n")
		var reasons []string
		for _, d := range gate.CheckCitationsInFiles(root, config.Defaults(), []string{filepath.Join(root, plan)}) {
			reasons = append(reasons, d.Reason)
		}
		return reasons
	}
	addRecord(record.Promise, "prd/widget", "FR-2", 5, "")
	addRecord(record.Decision, "design/widget", "D-2", 6, "")
	addRecord(record.Document, "prd/widget", "prd", 7, prd("FR-1", "FR-2"), prdV1)
	addRecord(record.Document, "design/widget", "design", 8, "# Widget design\n\n### D-1 — model\n\n### D-2 — scale\n", designV1)
	// Records only in the working tree are no change the commit prepares.
	if got := check("lock/widget/v2"); len(got) != 2 || !strings.Contains(got[0], "resolve lock/widget/v2") {
		t.Fatalf("uncommitted records stood in for the pending lock: %v", got)
	}
	run("add", "--", "memory")
	run("commit", "-qm", "prepare lock v2")
	if got := check("lock/widget/v2"); len(got) != 0 {
		t.Fatalf("plan on its pending lock did not resolve through the prepared records: %v", got)
	}
	// Only the next version after a tagged lock can be pending.
	if got := check("lock/widget/v3"); len(got) != 2 || !strings.Contains(got[0], "resolve lock/widget/v3") {
		t.Fatalf("a lock two versions ahead resolved: %v", got)
	}
	run("tag", "-a", "lock/widget/v2", "-m", "binding lock v2")
	if got := check("lock/widget/v2"); len(got) != 0 {
		t.Fatalf("tagged lock did not resolve: %v", got)
	}
	// A checkout that does not contain the previous lock prepares no next one.
	run("checkout", "-q", "--detach", "lock/widget/v1")
	if got := check("lock/widget/v3"); len(got) != 2 || !strings.Contains(got[0], "resolve lock/widget/v3") {
		t.Fatalf("a commit behind the previous lock resolved as the pending lock: %v", got)
	}
}
