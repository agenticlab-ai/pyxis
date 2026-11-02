package plan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func lockedGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func lockedWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lockedRecord(t *testing.T, kind, subject, family, statement string, n int) *record.Record {
	t.Helper()
	at := time.Date(2026, 9, 25, 10, 0, n, 0, time.UTC)
	var entropy [10]byte
	entropy[9] = byte(n + 1)
	r := &record.Record{Schema: record.Schema, ID: record.NewID(kind, at, entropy), Kind: kind,
		Subject: subject, Family: family, Statement: statement, Issuer: "fixture@example.test", CreatedAt: at.Format(time.RFC3339)}
	return r
}

func lockedPlanSource(version, basis string) string {
	s := strings.Replace(goldenPlan, "| **PRD** | docs/prds/widget |", "| **PRD** | prd/widget |", 1)
	s = strings.Replace(s, "| **Design** | docs/tech-design/widget-design.md |", "| **Design** | design/widget |\n| **Basis** | "+basis+" |", 1)
	s = strings.Replace(s, "design:widget-design#3.1", "design:widget#D-1", 1)
	if version != "001" {
		s = strings.Replace(s, "plan 001", "plan "+version, 1)
		s = strings.Replace(s, "| **Plan version** | 001 |", "| **Plan version** | "+version+" |", 1)
	}
	return s
}

func lockedPRD() string {
	return strings.Replace(governingEffectiveMD, "# Widget — PRD (projection)\n", "# Widget — PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v2 |\n", 1)
}

func lockedPlanFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	lockedGit(t, root, "init", "-b", "main")
	lockedGit(t, root, "config", "user.name", "Fixture Author")
	lockedGit(t, root, "config", "user.email", "fixture@example.test")
	lockedWrite(t, root, "README.md", "base\n")
	lockedWrite(t, root, "docs/prds/scaffolding-and-onboarding.md", corpusDoc)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", lockedPlanSource("001", "lock/widget/v1"))
	for n, id := range []string{"FR-1", "FR-2", "FR-3", "UB-1"} {
		r := lockedRecord(t, record.Promise, "prd/widget", id, "Locked "+id, n)
		if _, err := store.Write(filepath.Join(root, store.Dir), r); err != nil {
			t.Fatal(err)
		}
	}
	decision := lockedRecord(t, record.Decision, "design/widget", "D-1", "Widget model", 4)
	if _, err := store.Write(filepath.Join(root, store.Dir), decision); err != nil {
		t.Fatal(err)
	}
	for n, doc := range []struct{ subject, family, body string }{
		{"prd/widget", "prd", lockedPRD()},
		{"design/widget", "design", "# Widget tech spec\n\n### D-1 — Widget model\n\n**Decision:** Build a widget model.\n\n- **DINV-1** — Model is closed.\n"},
	} {
		r := lockedRecord(t, record.Document, doc.subject, doc.family, "Locked "+doc.family, n+5)
		r.LiteralBody = true
		if _, err := store.WriteDocument(filepath.Join(root, store.Dir), r, []byte(doc.body)); err != nil {
			t.Fatal(err)
		}
	}
	lockedGit(t, root, "add", "--", "README.md", "docs", "memory")
	lockedGit(t, root, "commit", "-m", "lock widget v1")
	lockedGit(t, root, "tag", "-a", "lock/widget/v1", "-m", "binding lock v1")
	return root
}

func lockedFold(t *testing.T, root, rel string) Result {
	t.Helper()
	return FoldFile(root, config.Defaults(), rel)
}

func lockedDefectContains(defects []string, part string) bool {
	for _, defect := range defects {
		if strings.Contains(defect, part) {
			return true
		}
	}
	return false
}

func lockedReasons(res Result) []string {
	var out []string
	for _, defect := range res.Defects {
		out = append(out, defect.Reason)
	}
	return out
}

func TestDraftBornPlanFoldsFromPinnedMemory(t *testing.T) {
	root := lockedPlanFixture(t)
	res := lockedFold(t, root, "docs/plans/widget/001-plan.md")
	if len(res.Defects) != 0 || len(res.JSON) == 0 {
		t.Fatalf("locked plan did not fold: %v", res.Defects)
	}
	if defects := FeatureCoverage(root, config.Defaults(), "widget"); len(defects) != 0 {
		t.Fatalf("locked feature coverage did not resolve: %v", defects)
	}
	for _, rel := range []string{"docs/prds/widget/effective.source.json", "docs/tech-design/widget.md"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("draft-born plan unexpectedly needed %s: %v", rel, err)
		}
	}
	badTrace := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "prd:widget/FR-1 · design:widget#D-1", "prd:widget/FR-404 · design:widget#D-1", 1)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", badTrace)
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "FR-404") {
		t.Fatalf("unknown requirement trace accepted: %v", got)
	}
	badDecision := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "design:widget#D-1", "design:widget#D-404", 1)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", badDecision)
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "D-404") {
		t.Fatalf("unknown decision trace accepted: %v", got)
	}
	anchor := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "design:widget#D-1", "design:widget#DINV-1", 1)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", anchor)
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); len(got) != 0 {
		t.Fatalf("locked tech-spec anchor did not resolve: %v", got)
	}
	missing := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "| prd:widget/FR-3 | summaries have no consumer yet; next plan |", "", 1)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", missing)
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "FR-3 is neither covered nor deferred") {
		t.Fatalf("unmapped locked requirement accepted: %v", got)
	}
}

func TestDraftBornPlanRefusesBrokenBasis(t *testing.T) {
	root := t.TempDir()
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", lockedPlanSource("001", "lock/widget/v1"))
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "open repository") {
		t.Fatalf("non-Git Basis accepted: %v", got)
	}
	root = lockedPlanFixture(t)
	lockedGit(t, root, "tag", "-d", "lock/widget/v1")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "cannot resolve Basis") {
		t.Fatalf("missing Basis accepted: %v", got)
	}
	root = lockedPlanFixture(t)
	lockedWrite(t, root, "memory/prd/widget/Bogus.json", "{broken")
	lockedGit(t, root, "add", "--", "memory/prd/widget/Bogus.json")
	lockedGit(t, root, "commit", "-m", "broken memory")
	lockedGit(t, root, "tag", "-f", "-a", "lock/widget/v1", "-m", "broken binding lock")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "invalid memory") {
		t.Fatalf("invalid locked memory accepted: %v", got)
	}
	root = lockedPlanFixture(t)
	duplicate := lockedRecord(t, record.Promise, "prd/widget", "FR-1", "Conflicting FR-1", 20)
	if _, err := store.Write(filepath.Join(root, store.Dir), duplicate); err != nil {
		t.Fatal(err)
	}
	lockedGit(t, root, "add", "--", "memory")
	lockedGit(t, root, "commit", "-m", "conflicting memory")
	lockedGit(t, root, "tag", "-f", "-a", "lock/widget/v1", "-m", "conflicting binding lock")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "projection rules") {
		t.Fatalf("conflicted locked memory accepted: %v", got)
	}
	root = lockedPlanFixture(t)
	lockedGit(t, root, "rm", "-r", "--", "memory/design/widget")
	lockedGit(t, root, "commit", "-m", "missing tech spec")
	lockedGit(t, root, "tag", "-f", "-a", "lock/widget/v1", "-m", "incomplete binding lock")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "no active PRD and tech-spec") {
		t.Fatalf("incomplete binding lock accepted: %v", got)
	}
	root = lockedPlanFixture(t)
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	var oldPRD *record.Record
	for _, r := range set.BySubject["prd/widget"] {
		if r.Family == "prd" {
			oldPRD = r
		}
	}
	if oldPRD == nil {
		t.Fatal("fixture PRD missing")
	}
	newPRD := lockedRecord(t, record.Document, "prd/widget", "prd", "Broken PRD", 20)
	newPRD.LiteralBody = true
	newPRD.Relations = []record.Relation{{Type: record.Supersedes, Target: oldPRD.ID}}
	if _, err := store.WriteDocument(filepath.Join(root, store.Dir), newPRD, []byte("not a PRD\n")); err != nil {
		t.Fatal(err)
	}
	lockedGit(t, root, "add", "--", "memory")
	lockedGit(t, root, "commit", "-m", "malformed PRD")
	lockedGit(t, root, "tag", "-f", "-a", "lock/widget/v1", "-m", "malformed binding lock")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "PRD cannot be projected") {
		t.Fatalf("malformed locked PRD accepted: %v", got)
	}
}

func TestDraftBornPlanPinsRetiredRequirementAndKeepsLockOnRefusal(t *testing.T) {
	root := lockedPlanFixture(t)
	oldLock := lockedGit(t, root, "rev-parse", "refs/tags/lock/widget/v1")
	// A newer lock retires FR-2. The earlier plan still reads v1's tree.
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	var oldPRD, oldFR2 *record.Record
	for _, r := range set.BySubject["prd/widget"] {
		switch r.Family {
		case "prd":
			oldPRD = r
		case "FR-2":
			oldFR2 = r
		}
	}
	if oldPRD == nil || oldFR2 == nil {
		t.Fatal("missing fixture records")
	}
	newPRD := lockedRecord(t, record.Document, "prd/widget", "prd", "Locked prd v2", 10)
	newPRD.LiteralBody = true
	newPRD.Relations = []record.Relation{{Type: record.Supersedes, Target: oldPRD.ID}}
	prd := strings.Replace(lockedPRD(), "| FR-2 | The widget shall serve lookups. | why | Must | med | fit | Test | J1 |\n", "", 1)
	if _, err := store.WriteDocument(filepath.Join(root, store.Dir), newPRD, []byte(prd)); err != nil {
		t.Fatal(err)
	}
	revocation := lockedRecord(t, record.Revocation, "prd/widget", "", "Withdraw FR-2", 11)
	revocation.Relations = []record.Relation{{Type: record.Revokes, Target: oldFR2.ID}}
	if _, err := store.Write(filepath.Join(root, store.Dir), revocation); err != nil {
		t.Fatal(err)
	}
	v2 := lockedPlanSource("002", "lock/widget/v2")
	v2 = strings.Replace(v2, "| prd:widget/FR-2 | T2, T3 |\n", "", 1)
	lockedWrite(t, root, "docs/plans/widget/002-plan.md", v2)
	lockedGit(t, root, "add", "--", "memory", "docs/plans/widget/002-plan.md")
	lockedGit(t, root, "commit", "-m", "lock widget v2")
	lockedGit(t, root, "tag", "-a", "lock/widget/v2", "-m", "binding lock v2")
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); len(got) != 0 {
		t.Fatalf("v1 plan lost its locked trace: %v", got)
	}
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/002-plan.md")); !lockedDefectContains(got, "FR-2") {
		t.Fatalf("v2 plan accepted retired trace: %v", got)
	}
	cycle := strings.Replace(lockedPlanSource("001", "lock/widget/v1"), "| **Blocked by** | — |", "| **Blocked by** | T2 |", 1)
	lockedWrite(t, root, "docs/plans/widget/001-plan.md", cycle)
	if got := lockedReasons(lockedFold(t, root, "docs/plans/widget/001-plan.md")); !lockedDefectContains(got, "dependency cycle") {
		t.Fatalf("projection rule did not refuse cycle: %v", got)
	}
	if now := lockedGit(t, root, "rev-parse", "refs/tags/lock/widget/v1"); now != oldLock {
		t.Fatalf("projection refusal changed binding lock: %s -> %s", oldLock, now)
	}
}
