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

// seededDelivery is a well-formed delivery-ledger/v1 with a two-entry journal
// (E-1, E-2) and the five derived-state fields — the ledger the cross-document
// fixtures cite into. The slug (`seeded-demo`) deliberately does not name any
// real docs/problems/* ledger in this repo, so committing nothing and building
// the corpus in a temp repo keeps the arm's own testdata out of the live scan.
const seededDelivery = `# Seeded demo — delivery ledger

| Field | Value |
|---|---|
| **Slug** | seeded-demo |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | open |
| **Stage position** | Stage 1 |
| **Open items** | none |
| **Park state** | — |
| **Cumulative spend** | unavailable |

## Journal

### E-1 — stage-entered — the drive began

- **Body**: first entry

### E-2 — ruling-recorded — a call was made

- **Body**: second entry
`

// ledgerCorpus builds a temp git repo seeded with the delivery ledger and the
// given citing documents, then runs the cross-document arm over it via the real
// `git ls-files` corpus read.
func ledgerCorpus(t *testing.T, docs map[string]string) []schema.Defect {
	t.Helper()
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
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("docs/problems/seeded-demo/delivery.md", seededDelivery)
	for rel, content := range docs {
		write(rel, content)
	}
	run("add", ".")
	run("commit", "-qm", "corpus")
	return gate.CheckLedgerCitations(root, config.Defaults())
}

func opsFor(defects []schema.Defect, file string) []string {
	var ops []string
	for _, d := range defects {
		if file == "" || d.File == file {
			ops = append(ops, d.Op)
		}
	}
	return ops
}

func hasOp(defects []schema.Defect, op string) bool {
	for _, d := range defects {
		if d.Op == op {
			return true
		}
	}
	return false
}

// A bare-prose citation into the derived-state region is the DINV-6 defect,
// naming the citing document and the derived-region target (acceptance: the
// cited-derived-block fixture, red at this arm only).
func TestLedgerCitationsDerivedBlockDefect(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/summary.md": "A drive summary points at delivery:seeded-demo#Status for its status.\n",
	})
	if !hasOp(defects, "derived-citation") {
		t.Fatalf("want a derived-citation defect, got %v", defects)
	}
	var d schema.Defect
	for _, x := range defects {
		if x.Op == "derived-citation" {
			d = x
		}
	}
	if d.File != "docs/wiki/summary.md" {
		t.Errorf("defect should name the citing document, got file %q", d.File)
	}
	if !strings.Contains(d.Reason, "derived-state") ||
		!strings.Contains(d.Reason, "docs/problems/seeded-demo/delivery.md") {
		t.Errorf("defect should name the derived-region target, got reason %q", d.Reason)
	}
	if d.Verb != "check-ledger-citations" {
		t.Errorf("wrong verb %q", d.Verb)
	}
}

// A bare-prose citation into a real journal E-n entry is legal — silent.
func TestLedgerCitationsJournalEntryLegal(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/summary.md": "The ruling is recorded at delivery:seeded-demo#E-2 as the basis.\n",
	})
	if len(defects) != 0 {
		t.Fatalf("a journal E-n citation must be silent, got %v", defects)
	}
}

// A resolvable-slug reference shown inside a code span or a fenced block is a
// displayed example, not a citation — dropped at recognition stage 1 despite
// resolving (acceptance: raises nothing).
func TestLedgerCitationsCodeSpanAndFenceSilent(t *testing.T) {
	span := "An example citation is `delivery:seeded-demo#Status` into a derived field.\n"
	fence := "```\ndelivery:seeded-demo#Status\ndelivery:seeded-demo#E-999\n```\n"
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/span.md":  span,
		"docs/wiki/fence.md": fence,
	})
	if len(defects) != 0 {
		t.Fatalf("code-span and fenced citations must be silent, got %v", defects)
	}
}

// A recognized citation whose anchor resolves to neither region is the distinct
// unresolvable-anchor defect (acceptance: distinct from the derived-block
// finding).
func TestLedgerCitationsUnresolvableAnchorDefect(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/summary.md": "See delivery:seeded-demo#E-999 for the ruling.\n",
	})
	if !hasOp(defects, "unresolvable-anchor") {
		t.Fatalf("want an unresolvable-anchor defect, got %v", defects)
	}
	if hasOp(defects, "derived-citation") {
		t.Fatalf("an unresolvable anchor must not be conflated with a derived-block finding: %v", defects)
	}
}

// The two defect discriminators are distinct on a single corpus (DINV-5).
func TestLedgerCitationsDistinctDiscriminators(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/a.md": "into the derived block: delivery:seeded-demo#Status\n",
		"docs/wiki/b.md": "into nowhere: delivery:seeded-demo#E-999\n",
	})
	if !hasOp(defects, "derived-citation") || !hasOp(defects, "unresolvable-anchor") {
		t.Fatalf("both discriminators must appear distinctly, got %v", defects)
	}
}

// A reference whose slug names no ledger — a `<slug>` metavariable or an
// absent-ledger mention — is not a citation (recognition stage 2), even in bare
// prose.
func TestLedgerCitationsUnresolvableSlugSilent(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/grammar.md": "Cite entries as delivery:<slug>#E-n in prose.\n" +
			"An absent ledger delivery:no-such-slug#E-1 is not a citation.\n",
	})
	if len(defects) != 0 {
		t.Fatalf("a non-resolving slug must be silent, got %v", defects)
	}
}

// The recognition combined fixture: only the bare-prose resolvable citation is
// classified; every other form is silent (acceptance A-6 shape).
func TestLedgerCitationsRecognitionBoundary(t *testing.T) {
	defects := ledgerCorpus(t, map[string]string{
		"docs/wiki/combined.md": strings.Join([]string{
			"i.   bare-prose derived citation delivery:seeded-demo#Status", // classified (DINV-6)
			"ii.  metavariable delivery:<slug>#E-n",                        // stage 2 drop
			"iii. absent ledger delivery:no-such-slug#E-1",                 // stage 2 drop
			"iv.  code span `delivery:seeded-demo#Status`",                 // stage 1 drop
			"v.   journal legal delivery:seeded-demo#E-1",                  // legal
			"```",
			"vi.  fenced delivery:seeded-demo#E-2", // stage 1 drop
			"```",
			"",
		}, "\n"),
	})
	ops := opsFor(defects, "docs/wiki/combined.md")
	if len(ops) != 1 || ops[0] != "derived-citation" {
		t.Fatalf("only the bare-prose derived citation should classify, got ops %v (defects %v)", ops, defects)
	}
}

// A malformed cited ledger yields a named parse defect, never a false pass
// (UB-3).
func TestLedgerCitationsMalformedLedgerParseDefect(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A delivery ledger declaring the right version but missing the journal
	// region — undelimitable, a named parse defect.
	malformed := "# broken — delivery ledger\n\n| Field | Value |\n|---|---|\n| **Template version** | delivery-ledger/v1 |\n\n## Derived state\n\n| Field | Value |\n|---|---|\n| **Status** | open |\n"
	run("init", "-q", "-b", "main")
	write("docs/problems/seeded-demo/delivery.md", malformed)
	write("docs/wiki/cite.md", "See delivery:seeded-demo#E-1 here.\n")
	run("add", ".")
	run("commit", "-qm", "corpus")
	defects := gate.CheckLedgerCitations(root, config.Defaults())
	if len(defects) == 0 {
		t.Fatalf("a malformed cited ledger must yield a parse defect, got none")
	}
	if hasOp(defects, "derived-citation") {
		t.Fatalf("a malformed ledger must not produce a false classification: %v", defects)
	}
}

// The arm is report-only: a run mutates no file in the corpus (DINV-4).
func TestLedgerCitationsReportOnly(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	run("init", "-q", "-b", "main")
	write("docs/problems/seeded-demo/delivery.md", seededDelivery)
	write("docs/wiki/cite.md", "delivery:seeded-demo#Status\n")
	run("add", ".")
	run("commit", "-qm", "corpus")
	_ = gate.CheckLedgerCitations(root, config.Defaults())
	status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(status)) != "" {
		t.Fatalf("the arm must not mutate the working tree, got: %s", status)
	}
}

// The arm registers as a base-free repo-wide check discoverable in the registry
// (composition: FR-6/A-8).
func TestLedgerCitationsRegistered(t *testing.T) {
	c, ok := gate.Lookup("check-ledger-citations")
	if !ok {
		t.Fatal("check-ledger-citations is not registered")
	}
	if c.NeedsMergeBase {
		t.Error("the cross-document arm must be base-free (NeedsMergeBase=false)")
	}
}
