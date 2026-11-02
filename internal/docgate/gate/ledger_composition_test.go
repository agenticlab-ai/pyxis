package gate_test

// The Argus ledger-lint T4 acceptance corpus and composition proofs
// (docs/tech-design/argus-ledger-lint.md §3.3, §3.10; plan T4). The two arms
// (the history arm — reg_check_ledger_history.go — and the cross-document arm
// — reg_check_ledger_citations.go) each pass their own per-arm fixtures; this
// file proves the *cross-arm* properties nothing else asserts:
//
//   - A-2: every per-invariant defect fixture is red at EXACTLY its arm, and
//     every legal-append and legal-citation fixture is green — on one corpus
//     that carries all defect classes at once (TestLedgerCorpusRedAtExactlyItsArm).
//   - A-5 / NR-1 / NR-2: the gate's verdict on a non-ledger tree is byte-identical
//     with and without the two arms, and check-immutable's governed set is
//     unchanged with zero of its defects on a legal ledger append
//     (TestLedgerArmsAdditiveOnNonLedgerTree, TestLedgerImmutableUntouchedByAppend).
//   - A-8 / FR-6 / NR-3: the umbrella's shared enumeration, dispatch, and help
//     carry no hand-edited line for either arm and list both from their
//     descriptors (TestLedgerArmsRegisteredFromDescriptors, TestUmbrellaNoHandEditedArmLine).
//   - A-7 / FR-8 / UB-1: both arms run under the identical umbrella verb on desk
//     and CI, differing only in merge-base source, with the history arm skipped
//     when no merge base is present (TestLedgerDeskCIUmbrellaParity,
//     TestLedgerHistorySkippedBaseFreeUnderUmbrella).

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const (
	histArmVerb = "check-ledger-history"
	citeArmVerb = "check-ledger-citations"
)

// corpusRepo stands up one temp git repo whose BASE commit holds the pristine
// ledgers, the seeded citation target, and every citing document (so the
// base-free cross-document arm — which reads `git ls-files` — sees them as
// tracked), then applies the history-defect mutations to the WORKING TREE only
// (so the base-requiring history arm, comparing HEAD against the working tree,
// sees them as a diff). Both arms then run over this one repo state — the
// combined per-invariant corpus (A-2).
type corpusRepo struct {
	t    *testing.T
	root string
}

func newCorpusRepo(t *testing.T) *corpusRepo {
	t.Helper()
	r := &corpusRepo{t: t, root: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	// Base: pristine ledgers + the citation target + every citing document.
	r.write("docs/problems/foo/delivery.md", histDelivery)           // E-1, E-2
	r.write("docs/problems/foo/problem.md", histProblem)             // A-1, A-2
	r.write("docs/problems/seeded-demo/delivery.md", seededDelivery) // E-1, E-2 + derived
	r.write("docs/wiki/derived-cite.md", "The status is delivery:seeded-demo#Status per the run.\n")
	r.write("docs/wiki/dangling-cite.md", "See delivery:seeded-demo#E-999 for the ruling.\n")
	r.write("docs/wiki/legal-cite.md", "The ruling is recorded at delivery:seeded-demo#E-2.\n")
	r.git("add", ".")
	r.git("commit", "-qm", "base")
	return r
}

func (r *corpusRepo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r *corpusRepo) write(rel, content string) {
	r.t.Helper()
	p := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// TestLedgerCorpusRedAtExactlyItsArm is the A-2 spine: one corpus bearing an
// edited entry, a removed entry, a cited derived block, a dangling in-scope
// anchor, a legal append, and a legal journal citation — each defect class is
// red at EXACTLY its arm, and both legal cases are green. The two arms are
// disjoint: no history defect touches a citing document and no citation defect
// touches a ledger file (the "red at exactly its arm" partition, A-1/A-2).
func TestLedgerCorpusRedAtExactlyItsArm(t *testing.T) {
	r := newCorpusRepo(t)

	// Working-tree mutations. foo/delivery.md: edit the merged E-1 body, change
	// the derived-state region, and legally append E-3. foo/problem.md: remove
	// the merged A-2. The citing documents are already committed and unchanged.
	mutated := strings.Replace(histDelivery, "the first entry", "a QUIETLY REWRITTEN body", 1)
	mutated = strings.Replace(mutated, "| **Stage position** | stage 1 |", "| **Stage position** | stage 2 |", 1)
	mutated += "\n### E-3 — ruling-recorded — third\n\n- **Date**: 2026-07-13\n- **Body**: a brand-new legal append\n\n---\n"
	r.write("docs/problems/foo/delivery.md", mutated)
	r.write("docs/problems/foo/problem.md",
		histProblem[:strings.Index(histProblem, "### A-2 —")]+"---\n")

	hist := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults())
	cite := gate.CheckLedgerCitations(r.root, config.Defaults())

	// Each declared fixture: its expected arm-verb, defect Op, the file it names,
	// and a Reason needle. A green fixture (verdict == "") must draw no defect.
	type fixture struct {
		name     string
		verb, op string
		file     string
		needle   string
		green    bool
	}
	fixtures := []fixture{
		{"edited merged E-1", histArmVerb, "history", "docs/problems/foo/delivery.md", "#E-1", false},
		{"removed merged A-2", histArmVerb, "history", "docs/problems/foo/problem.md", "#A-2", false},
		{"cited derived block", citeArmVerb, "derived-citation", "docs/wiki/derived-cite.md", "seeded-demo", false},
		{"dangling anchor", citeArmVerb, "unresolvable-anchor", "docs/wiki/dangling-cite.md", "seeded-demo", false},
		{"legal append E-3", histArmVerb, "", "docs/problems/foo/delivery.md", "#E-3", true},
		{"legal journal citation", citeArmVerb, "", "docs/wiki/legal-cite.md", "seeded-demo", true},
	}

	all := append(append([]schema.Defect{}, hist...), cite...)
	for _, f := range fixtures {
		matched := matchingDefects(all, f.verb, f.op, f.file, f.needle)
		if f.green {
			// A green fixture draws no defect from ANY arm on its file/needle.
			for _, d := range all {
				if d.File == f.file && strings.Contains(d.Reason, f.needle) {
					t.Errorf("%s: expected green but got defect %+v", f.name, d)
				}
			}
			continue
		}
		if len(matched) != 1 {
			t.Errorf("%s: want exactly one %s/%s defect on %s (needle %q), got %d: %v",
				f.name, f.verb, f.op, f.file, f.needle, len(matched), all)
		}
	}

	// The partition: the history arm names only ledger files; the cross-document
	// arm names only citing documents — neither ever reds the other's fixture.
	for _, d := range hist {
		if d.Verb != histArmVerb {
			t.Errorf("history-arm run produced a foreign verb: %+v", d)
		}
		if !strings.HasPrefix(d.File, "docs/problems/") {
			t.Errorf("history arm named a non-ledger file: %+v", d)
		}
	}
	for _, d := range cite {
		if d.Verb != citeArmVerb {
			t.Errorf("cross-document run produced a foreign verb: %+v", d)
		}
		if strings.HasPrefix(d.File, "docs/problems/") && strings.HasSuffix(d.File, ".md") {
			// The arm may name a *malformed* ledger as a parse defect, but here
			// every ledger is well-formed, so it must name only citing documents.
			t.Errorf("cross-document arm named a ledger file on a well-formed corpus: %+v", d)
		}
	}

	// Red at EXACTLY its arm: exactly the two history defects and the two
	// citation defects, no more (the two legal cases add nothing).
	if len(hist) != 2 {
		t.Errorf("history arm: want exactly 2 defects (edit + removal), got %d: %v", len(hist), hist)
	}
	if len(cite) != 2 {
		t.Errorf("cross-document arm: want exactly 2 defects (derived + unresolvable), got %d: %v", len(cite), cite)
	}
}

// matchingDefects returns the defects matching an arm-verb, an Op, a file, and a
// Reason needle — the per-fixture selector for the A-2 corpus table.
func matchingDefects(ds []schema.Defect, verb, op, file, needle string) []schema.Defect {
	var out []schema.Defect
	for _, d := range ds {
		if d.Verb == verb && d.Op == op && d.File == file && strings.Contains(d.Reason, needle) {
			out = append(out, d)
		}
	}
	return out
}

// serializeDefects renders a defect set into a stable, order-independent string
// so two verdicts can be compared byte-for-byte regardless of accumulation order.
func serializeDefects(ds []schema.Defect) string {
	lines := make([]string, 0, len(ds))
	for _, d := range ds {
		lines = append(lines, strings.Join([]string{d.Verb, d.File, d.Op, d.Reason, d.Remedy}, "\x1f"))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// withoutArms drops every defect contributed by the two ledger-lint arms — the
// "without the two arms" verdict, given the arms are purely additive (they only
// ADD defects, never remove or alter another check's; proven by the per-arm
// partition test). On a non-ledger tree they contribute nothing, so the two
// verdicts must be byte-identical.
func withoutArms(ds []schema.Defect) []schema.Defect {
	var out []schema.Defect
	for _, d := range ds {
		if d.Verb == histArmVerb || d.Verb == citeArmVerb {
			continue
		}
		out = append(out, d)
	}
	return out
}

// TestLedgerArmsAdditiveOnNonLedgerTree proves A-5 / NR-1: on a tree with no
// Argus ledger and no resolvable delivery citation, the gate's verdict is
// byte-identical with and without the two arms — the arms only add defects on
// the ledger classes they newly govern, perturbing no non-ledger verdict.
func TestLedgerArmsAdditiveOnNonLedgerTree(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A realistic non-ledger tree: a PRD ledger directory, a wiki note, a
	// design — but NO docs/problems ledger and NO resolvable `delivery:` citation.
	write("docs/prds/foo/000-base.md", miniBase)
	write("docs/wiki/note.md", "A note that mentions the delivery:<slug>#E-n grammar only.\n")
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-qm", "base")

	cfg := config.Defaults()
	dirs, err := gate.LedgerDirs(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	with := gate.Gate(root, "HEAD", dirs, cfg)

	// No defect on a non-ledger tree may carry an arm verb.
	for _, d := range with {
		if d.Verb == histArmVerb || d.Verb == citeArmVerb {
			t.Errorf("an arm fired on a non-ledger tree: %+v", d)
		}
	}
	// Byte-identical: dropping the (empty) arm contribution changes nothing.
	if serializeDefects(with) != serializeDefects(withoutArms(with)) {
		t.Errorf("non-ledger verdict differs with vs without the two arms")
	}
}

// TestLedgerImmutableUntouchedByAppend proves NR-2 / A-5: a legal ledger append
// trips ZERO check-immutable defects — the check-immutable governed set
// (docs/prds + docs/tech-design) is disjoint from the Argus ledger classes
// (docs/problems), so a docs/problems change is not even in its diff scope. The
// history arm is the separate guard; check-immutable is unchanged.
func TestLedgerImmutableUntouchedByAppend(t *testing.T) {
	r := newCorpusRepo(t)
	// A pure legal append to a delivery ledger, plus its derived-state change.
	appended := strings.Replace(histDelivery, "| **Stage position** | stage 1 |", "| **Stage position** | stage 2 |", 1) +
		"\n### E-3 — ruling-recorded — third\n\n- **Date**: 2026-07-13\n- **Body**: a legal append\n\n---\n"
	r.write("docs/problems/foo/delivery.md", appended)

	if d := gate.CheckImmutable(r.root, "HEAD", "", config.Defaults()); len(d) != 0 {
		t.Fatalf("check-immutable fired on a docs/problems append (its governed set must exclude the ledgers): %v", d)
	}
	if d := gate.CheckLedgerHistory(r.root, "HEAD", config.Defaults()); len(d) != 0 {
		t.Fatalf("the history arm flagged a legal append: %v", d)
	}
}

// TestLedgerArmsRegisteredFromDescriptors proves A-8 / FR-6: both arms are in
// the check registry, contributed from their own descriptor files, with the
// declared merge-base disposition and a non-empty help line the umbrella's
// help walk renders — nothing hand-listed.
func TestLedgerArmsRegisteredFromDescriptors(t *testing.T) {
	want := map[string]bool{histArmVerb: true, citeArmVerb: false} // verb -> NeedsMergeBase
	for verb, needsBase := range want {
		c, ok := gate.Lookup(verb)
		if !ok {
			t.Errorf("%s is not registered", verb)
			continue
		}
		if c.NeedsMergeBase != needsBase {
			t.Errorf("%s: NeedsMergeBase = %v, want %v", verb, c.NeedsMergeBase, needsBase)
		}
		if strings.TrimSpace(c.Help) == "" {
			t.Errorf("%s: empty help line — the umbrella help walk renders the descriptor's Help", verb)
		}
	}
	// Both appear in the verb-sorted enumeration the umbrella and CLI both walk.
	seen := map[string]bool{}
	for _, c := range gate.Checks() {
		seen[c.Verb] = true
	}
	if !seen[histArmVerb] || !seen[citeArmVerb] {
		t.Errorf("both arms must appear in gate.Checks() (the derived enumeration): %v", seen)
	}
}

// TestUmbrellaNoHandEditedArmLine proves A-8 / NR-3: the umbrella's shared
// enumeration (gate.go), the registry (registry.go), and the CLI
// dispatch/help (cmd/docgate/main.go) carry NO hand-edited line naming either
// arm — the arm verbs appear only in their own descriptor files. Adding an arm
// is a single-file addition touching zero shared lines (AP-27).
func TestUmbrellaNoHandEditedArmLine(t *testing.T) {
	root := repoRootForTest(t)
	shared := []string{
		"internal/docgate/gate/gate.go",
		"internal/docgate/gate/registry.go",
		"cmd/docgate/main.go",
	}
	for _, rel := range shared {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		body := string(b)
		for _, verb := range []string{histArmVerb, citeArmVerb} {
			if strings.Contains(body, verb) {
				t.Errorf("%s hand-lists the arm verb %q — the umbrella must assemble arms from descriptors only", rel, verb)
			}
		}
	}
	// Sanity: each arm verb DOES live in its own descriptor file — the single
	// place the addition lands.
	descriptors := map[string]string{
		histArmVerb: "internal/docgate/gate/reg_check_ledger_history.go",
		citeArmVerb: "internal/docgate/gate/reg_check_ledger_citations.go",
	}
	for verb, rel := range descriptors {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		if !strings.Contains(string(b), verb) {
			t.Errorf("%s does not define its own verb %q", rel, verb)
		}
	}
}

// TestLedgerHistorySkippedBaseFreeUnderUmbrella proves A-7 / UB-1: under the
// umbrella with no merge base, the history arm is skipped (it neither runs nor
// errors) while the base-free cross-document arm still runs — exactly
// check-immutable's degradation. With a base present, the history arm runs.
func TestLedgerHistorySkippedBaseFreeUnderUmbrella(t *testing.T) {
	r := newCorpusRepo(t)
	// Corrupt a merged entry: were the history arm to run base-free, it would red.
	r.write("docs/problems/foo/delivery.md",
		strings.Replace(histDelivery, "the first entry", "corrupted", 1))

	// Base-free umbrella: no history-arm defect (skipped), and the run does not error.
	baseFree := gate.Gate(r.root, "", nil, config.Defaults())
	for _, d := range baseFree {
		if d.Verb == histArmVerb {
			t.Fatalf("history arm ran base-free under the umbrella: %+v", d)
		}
	}
	// The cross-document arm is base-free and still governs — it ran (a dangling
	// citation in the seeded corpus proves the arm executed on the base-free path).
	sawCiteArm := false
	for _, d := range baseFree {
		if d.Verb == citeArmVerb {
			sawCiteArm = true
		}
	}
	if !sawCiteArm {
		t.Fatal("the base-free cross-document arm did not run under the umbrella (the dangling citation should have redded)")
	}

	// With a base, the history arm runs and catches the corruption.
	withBase := gate.Gate(r.root, "HEAD", nil, config.Defaults())
	sawHistArm := false
	for _, d := range withBase {
		if d.Verb == histArmVerb {
			sawHistArm = true
		}
	}
	if !sawHistArm {
		t.Fatal("history arm did not run under the umbrella with a merge base present")
	}
}

// TestLedgerDeskCIUmbrellaParity proves A-7 / FR-8 under config-driven
// portability (config-driven-portability-v1 §3.1): both call sites — the desk
// pre-push hook and the CI workflow — invoke the same `docgate` umbrella and
// resolve a trusted merge-base source. The desk hook bakes NO merge-base
// literal: it runs `pre-push` and lets pyxis resolve the ref from the configured
// branch+remote at runtime (DINV-1/DINV-4). CI pins the comparison to the base
// commit supplied by the triggering PR or push event; a moving mainline ref
// must not change what that event verifies. Both use the same gate umbrella.
func TestLedgerDeskCIUmbrellaParity(t *testing.T) {
	root := repoRootForTest(t)
	hook, err := os.ReadFile(filepath.Join(root, ".githooks", "pre-push"))
	if err != nil {
		t.Fatalf("reading the desk hook: %v", err)
	}
	wf, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "docs-gate.yml"))
	if err != nil {
		t.Fatalf("reading the CI workflow: %v", err)
	}
	// The desk hook's EXECUTABLE BODY (comments stripped) is branch/remote-
	// literal-free: it delegates `pre-push` with no baked --merge-base, resolving
	// live from config (A-7, DINV-1). Explanatory comments may name the flag.
	hookBody := stripShellComments(string(hook))
	if strings.Contains(hookBody, "--merge-base") {
		t.Errorf("desk hook body must NOT bake a --merge-base literal — pyxis resolves it from config at runtime (A-7, DINV-1)")
	}
	if strings.Contains(hookBody, "origin/main") || strings.Contains(hookBody, "origin/") {
		t.Errorf("desk hook body must carry no branch/remote literal (DINV-1)")
	}
	if !strings.Contains(hookBody, "docgate-resolve.sh\" pre-push") {
		t.Errorf("desk hook must run the `pre-push` umbrella entry (resolving the merge-base from config)")
	}
	// CI passes the trusted event snapshot to the same umbrella. The complete
	// workflow structure and effective environment are enforced by ciparity.
	wfBody := stripShellComments(string(wf))
	if !strings.Contains(wfBody, `PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"`) {
		t.Errorf("CI workflow must take its merge base from the triggering event")
	}
	if !strings.Contains(wfBody, `run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -`) {
		t.Errorf("CI workflow must run the `gate` umbrella entry with the trusted event base")
	}
}

// repoRootForTest walks up from the test's working directory to the repository
// root (the directory holding go.mod).
func repoRootForTest(t *testing.T) string {
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
			t.Fatal("no go.mod found above the test working directory")
		}
		dir = parent
	}
}

// stripShellComments drops whole-line `#` comments (and the shebang) from a
// shell script so an assertion targets the EXECUTABLE body, not explanatory
// comments — a comment naming a flag the body deliberately no longer bakes
// must not trip a literal-free check.
func stripShellComments(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
