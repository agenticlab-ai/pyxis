package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// A well-formed delivery ledger: header, the two regions, two journal entries.
const histDelivery = `# Foo — delivery ledger

| Field | Value |
|---|---|
| **Slug** | foo |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | stage 1 |

## Journal

### E-1 — stage-entered — first

- **Date**: 2026-07-11
- **Body**: the first entry

---

### E-2 — ruling-recorded — second

- **Date**: 2026-07-12
- **Body**: the second entry

---
`

const histProblem = `# Foo — problem ledger

| Field | Value |
|---|---|
| **Slug** | foo |
| **Template version** | problem-ledger/v1 |

## Idea

do the thing

## Authority entries

### A-1 — a pre-ruling

- **Date**: 2026-07-11
- **Ruling**: the first ruling

---

### A-2 — an answer

- **Date**: 2026-07-12
- **Ruling**: the second ruling

---
`

// histRepo stands up a temp git repo with the two seeded ledgers committed as
// the merge base, returning the root and a mutate() to change the working tree
// (files staged+committed so the base is HEAD~1).
type histRepo struct {
	t    *testing.T
	root string
}

func newHistRepo(t *testing.T) *histRepo {
	t.Helper()
	r := &histRepo{t: t, root: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write("docs/problems/foo/delivery.md", histDelivery)
	r.write("docs/problems/foo/problem.md", histProblem)
	r.git("add", ".")
	r.git("commit", "-qm", "base")
	return r
}

func (r *histRepo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *histRepo) write(rel, content string) {
	r.t.Helper()
	p := filepath.Join(r.root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *histRepo) rm(rel string) {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.root, filepath.FromSlash(rel))); err != nil {
		r.t.Fatal(err)
	}
}

// FR-1: an edited merged entry (body changed) is an edit defect naming the
// ledger and the disturbed anchor.
func TestLedgerHistoryEditDefect(t *testing.T) {
	r := newHistRepo(t)
	r.write("docs/problems/foo/delivery.md",
		strings.Replace(histDelivery, "the first entry", "a QUIETLY REWRITTEN body", 1))
	d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	wantDefect(t, d, "changed vs merge base")
	wantDefect(t, d, "delivery.md#E-1")
	assertArmOnly(t, d)
}

// FR-1: a removed merged entry is a removal defect.
func TestLedgerHistoryRemovalDefect(t *testing.T) {
	r := newHistRepo(t)
	// Drop E-2 entirely — a merged entry deleted.
	trimmed := histDelivery[:strings.Index(histDelivery, "### E-2 —")] + "---\n"
	r.write("docs/problems/foo/delivery.md", trimmed)
	d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	wantDefect(t, d, "removed or renumbered vs merge base")
	wantDefect(t, d, "delivery.md#E-2")
	assertArmOnly(t, d)
}

// FR-1: renumbering a merged entry presents the old anchor as absent → removal.
func TestLedgerHistoryRenumberDefect(t *testing.T) {
	r := newHistRepo(t)
	// A-2 renumbered to A-3 — the merged A-2 anchor vanishes.
	r.write("docs/problems/foo/problem.md",
		strings.Replace(histProblem, "### A-2 —", "### A-3 —", 1))
	d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	wantDefect(t, d, "removed or renumbered vs merge base")
	wantDefect(t, d, "problem.md#A-2")
	assertArmOnly(t, d)
}

// FR-1: deleting the whole ledger file reds every merged anchor.
func TestLedgerHistoryWholeFileRemoval(t *testing.T) {
	r := newHistRepo(t)
	r.rm("docs/problems/foo/delivery.md")
	d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	wantDefect(t, d, "delivery.md#E-1")
	wantDefect(t, d, "delivery.md#E-2")
}

// FR-2: a legal append (a new, strictly-higher-numbered entry) raises zero
// history-arm defects — even though the derived-state region also changed.
func TestLedgerHistoryLegalAppendSilent(t *testing.T) {
	r := newHistRepo(t)
	appended := strings.Replace(histDelivery,
		"| **Stage position** | stage 1 |", "| **Stage position** | stage 2 |", 1) +
		`
### E-3 — ruling-recorded — third

- **Date**: 2026-07-13
- **Body**: a brand-new legal append

---
`
	r.write("docs/problems/foo/delivery.md", appended)
	if d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults()); len(d) != 0 {
		t.Fatalf("legal append flagged: %v", d)
	}
}

// An untouched working tree raises nothing.
func TestLedgerHistoryUntouchedGreen(t *testing.T) {
	r := newHistRepo(t)
	if d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults()); len(d) != 0 {
		t.Fatalf("untouched ledgers flagged: %v", d)
	}
}

// UB-3: a malformed working-tree ledger is a named parse defect, not a crash
// and not a false green.
func TestLedgerHistoryMalformedWorkingTree(t *testing.T) {
	r := newHistRepo(t)
	// Remove the Journal heading — the entry region can no longer be delimited.
	r.write("docs/problems/foo/delivery.md",
		strings.Replace(histDelivery, "## Journal\n", "## Log\n", 1))
	d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	wantDefect(t, d, "Journal region could not be delimited")
}

// FR-5 / UB-1: with no merge base the umbrella skips the arm entirely — it
// neither runs nor errors. (The base-requiring descriptor drives the skip.)
func TestLedgerHistorySkippedWithNoMergeBase(t *testing.T) {
	r := newHistRepo(t)
	// Corrupt a merged entry in the working tree: were the arm to run, it would
	// red. The umbrella must skip it because mergeBase is "".
	r.write("docs/problems/foo/delivery.md",
		strings.Replace(histDelivery, "the first entry", "corrupted", 1))
	for _, chk := range gate.Checks() {
		if chk.Verb == "check-ledger-history" && !chk.NeedsMergeBase {
			t.Fatal("history arm must be base-requiring (NeedsMergeBase true)")
		}
	}
	// Gate with no merge base and no ledger dirs: the arm is skipped, so the
	// corruption is invisible and the run is clean of history-arm defects.
	d := gate.Gate(r.root, "", nil, config.Defaults())
	for _, x := range d {
		if x.Verb == "check-ledger-history" {
			t.Fatalf("history arm ran with no merge base: %v", x)
		}
	}
}

// A-4 / DINV-4: the arm is report-only — the working tree is byte-unchanged
// across a run (including a run that finds defects).
func TestLedgerHistoryReportOnly(t *testing.T) {
	r := newHistRepo(t)
	r.write("docs/problems/foo/delivery.md",
		strings.Replace(histDelivery, "the first entry", "edited", 1))
	before := snapshotTree(t, r.root)
	_ = gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	after := snapshotTree(t, r.root)
	if before != after {
		t.Fatal("history arm mutated the working tree")
	}
}

// TestLedgerHistoryRelocatedProblemsRoot is the config-driven-portability
// relocation proof for this check (FR-1): with the problems root relocated by
// config, the history arm lists and judges ledgers at the CONFIGURED location —
// so an edited merged entry under the relocated root is still caught, and a
// stale default-location tree is not silently judged. It also confirms the
// default-config run over the same relocated tree is a no-op (the check reads
// the configured root, never the literal docs/problems).
func TestLedgerHistoryRelocatedProblemsRoot(t *testing.T) {
	const relocated = "docs/tickets"
	r := &histRepo{t: t, root: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.write(relocated+"/foo/delivery.md", histDelivery)
	r.write(relocated+"/foo/problem.md", histProblem)
	r.git("add", ".")
	r.git("commit", "-qm", "base")

	// Edit a merged entry under the relocated root.
	r.write(relocated+"/foo/delivery.md",
		strings.Replace(histDelivery, "the first entry", "a QUIETLY REWRITTEN body", 1))

	cfg := config.Defaults()
	cfg.Problems = relocated

	// Under the configured root, the edit is caught, naming the relocated file.
	d := gate.CheckLedgerHistory(r.root, "HEAD", cfg)
	wantDefect(t, d, "changed vs merge base")
	wantDefect(t, d, relocated+"/foo/delivery.md#E-1")
	assertArmOnly(t, d)

	// Under a DEFAULT config (problems == docs/problems), the relocated tree is
	// invisible — the check reads the configured root, not a baked literal, so a
	// ledger at docs/tickets is simply not this run's concern (no false green,
	// no cross-root leakage).
	if d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults()); len(d) != 0 {
		t.Fatalf("default-config run judged a ledger outside its configured root: %v", d)
	}
}

// assertArmOnly proves the defects come from the history arm alone — the
// red-at-exactly-its-arm guarantee (A-1).
func assertArmOnly(t *testing.T, d []schema.Defect) {
	t.Helper()
	if len(d) == 0 {
		t.Fatal("expected at least one history-arm defect")
	}
	for _, x := range d {
		if x.Verb != "check-ledger-history" {
			t.Errorf("defect from another arm: %v", x)
		}
	}
}

// snapshotTree captures every file path+content under root (excluding .git) as
// one string, so a byte-level mutation is detectable.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		body, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		b.WriteString(rel)
		b.WriteByte(0)
		b.Write(body)
		b.WriteByte(0)
		return nil
	})
	return b.String()
}
