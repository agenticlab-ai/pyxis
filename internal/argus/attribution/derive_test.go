package attribution

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repo is a throwaway git repo the integration tests build commit-by-commit to
// exercise the real trailer derivation — the local-substitutable seam of §3.10:
// a real `git` read of a real commit graph, never a mock of it.
type repo struct {
	t   *testing.T
	dir string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.email", "t@example.com")
	r.git("config", "user.name", "Test")
	r.git("config", "commit.gpgsign", "false")
	return r
}

// git runs a git command in the repo, failing the test on error. The command
// runs with a scrubbed environment carrying no credential — the derivation must
// need none (A-3, DINV-3).
func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE=2026-07-12T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-12T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *repo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commit stages every change and commits with the given message (which may end
// in an Argus-Drive trailer block). Returns the new HEAD sha.
func (r *repo) commit(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// commitArgus authors a genuine drive commit exactly as the producer does: it
// stages the change, computes the in-tree content binding over the commit's own
// tree+parents via the single-sourced attribution.ComputeBinding — proving the
// producer and the gate stay in lockstep — and commits with both the
// `Argus-Drive` stamp and its `Argus-Bind` binding in the final trailer block.
// preBody is the message content before the trailer block (a subject, and any
// planted body line such as an attestation). Returns the new HEAD sha.
func (r *repo) commitArgus(preBody, slug string) string {
	r.t.Helper()
	r.git("add", "-A")
	tree := r.git("write-tree")
	var parents []string
	if head, ok := r.gitTry("rev-parse", "HEAD"); ok && head != "" {
		parents = []string{head}
	}
	bind := ComputeBinding(tree, parents)
	msg := preBody + "\n\n" + TrailerKey + ": " + slug + "\n" + BindTrailerKey + ": " + bind + "\n"
	r.git("commit", "-q", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// gitTry runs a git command, returning its output and whether it succeeded
// (rev-parse HEAD fails on an unborn branch — a parentless commit, not an error).
func (r *repo) gitTry(args ...string) (string, bool) {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", r.dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err == nil
}

// bindTrailer reads the `Argus-Bind` value a commit's message carries.
func (r *repo) bindTrailer(sha string) string {
	r.t.Helper()
	return parseTrailer(r.git("log", "-1", "--format=%B", sha)).bind
}

func TestVerifyRebaseSource(t *testing.T) {
	tree, parents := strings.Repeat("a", 40), []string{strings.Repeat("b", 40)}
	valid := ComputeBinding(tree, parents)
	for _, tc := range []struct {
		name, message, want string
	}{
		{"unstamped", "plain commit\n", ""},
		{"valid", "drive commit\n\nArgus-Drive: widget\nArgus-Bind: " + valid + "\n", ""},
		{"stale", "drive commit\n\nArgus-Drive: widget\nArgus-Bind: " + strings.Repeat("0", 64) + "\n", "stale or replayed"},
		{"partial", "drive commit\n\nArgus-Drive: widget\n", "incomplete or malformed"},
		{"binding only", "drive commit\n\nArgus-Bind: " + valid + "\n", "incomplete or malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyRebaseSource(tc.message, tree, parents)
			if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("verify error = %v, want %q", err, tc.want)
			}
		})
	}
}

// run evaluates the gate over base..HEAD and returns the refused paths keyed by
// their op token. base is the resolved fork-point commit (the tests pass the
// base commit's sha directly, exactly what the docgate umbrella hands a
// base-requiring check).
func (r *repo) run(base string) map[string]string {
	r.t.Helper()
	got := map[string]string{}
	for _, d := range Evaluate(r.dir, base, dcfg) {
		got[d.File] = d.Op
	}
	return got
}

// TestGit_ArgusTrailerRefused_HumanNot proves the real trailer read: an
// Argus-trailered commit touching the authority contract is refused; a plain
// (untrailered, determinable-human) commit touching a governance document is
// not. Two separate PRs off a shared empty base.
func TestGit_ArgusTrailerRefused_HumanNot(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	// PR 1 — an Argus commit adds the authority contract (a produced, bound stamp).
	r.write(authorityContract, "policy\n")
	r.commitArgus("feat: touch policy", "widget")
	got := r.run(base)
	if got[authorityContract] != "authority-contract" {
		t.Fatalf("Argus authority-contract diff not refused: %v", got)
	}

	// PR 2 — a fresh clone-like base; a POSITIVELY-HUMAN commit adds a governance
	// doc, carrying the positive attestation the governance class already
	// requires (no Argus stamp → positively-human, §3.3).
	r2 := newRepo(t)
	r2.write("README.md", "seed\n")
	base2 := r2.commit("chore: seed")
	r2.write(governanceDocument, "vision\n")
	r2.commit("docs: human edits the vision\n\nGoverning-Change: operator")
	if got := r2.run(base2); len(got) != 0 {
		t.Fatalf("positively-human governance diff was refused: %v", got)
	}
}

// TestGit_MalformedTrailerUnderivableFailsClosed proves the underivable branch
// against a real commit: a commit whose Argus-Drive trailer is present but
// carries no derivable slug (an unverifiable trailer) touching the authority
// contract fails closed; the same malformed-trailer commit touching a
// non-protected code path is not refused.
func TestGit_MalformedTrailerUnderivableFailsClosed(t *testing.T) {
	// Underivable ∧ protected → refuse.
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")
	r.write(authorityContract, "policy\n")
	r.commit("feat: touch policy\n\nArgus-Drive:\n") // present but empty value
	got := r.run(base)
	if got[authorityContract] != "authority-contract" {
		t.Fatalf("underivable (malformed-trailer) authority-contract diff not failed closed: %v", got)
	}

	// Underivable ∧ non-protected → not refused (fail-closed scoped to protected).
	r2 := newRepo(t)
	r2.write("README.md", "seed\n")
	base2 := r2.commit("chore: seed")
	r2.write("internal/tool/x.go", "package tool\n")
	r2.commit("feat: code\n\nArgus-Drive:\n")
	if got := r2.run(base2); len(got) != 0 {
		t.Fatalf("underivable non-protected diff was refused: %v", got)
	}
}

// TestGit_WidenSlugAgainstCommittedLedger proves DINV-6 end-to-end: with a
// broader problem's committed delivery ledger present at the merge base, an
// Argus commit that names that broader slug and writes an out-of-scope code
// path reds as out-of-scope — the width comes from the committed ledger, not
// the trailer — while a path that ledger genuinely owns passes.
func TestGit_WidenSlugAgainstCommittedLedger(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.write("docs/problems/broad/delivery.md", committedLedger) // committed at base
	base := r.commit("chore: seed with broad's merged ledger")

	// The drive names "broad" and writes a code path the merged ledger does not
	// own, plus a path it does.
	r.write("internal/engine/core.go", "package engine\n")
	r.write("docs/tech-design/broad.md", "design\n")
	r.commitArgus("feat: widen attempt", "broad")

	got := r.run(base)
	if got["internal/engine/core.go"] != "out-of-scope" {
		t.Fatalf("widen-your-own-scope code path not refused as out-of-scope: %v", got)
	}
	if _, refused := got["docs/tech-design/broad.md"]; refused {
		t.Fatalf("a committed-ledger-owned path was refused: %v", got)
	}
}

// TestGit_FirstPRScope proves the first-PR grammar end-to-end (no committed
// ledger at the base): an Argus commit for a brand-new problem writing its own
// doc subtrees passes, while a code path (even under a broader slug) reds.
func TestGit_FirstPRScope(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	r.write("docs/problems/newthing/problem.md", "p\n")
	r.write("docs/prds/newthing/000-base.md", "prd\n")
	r.write("internal/engine/core.go", "package engine\n") // out-of-scope
	r.commitArgus("feat: first PR", "newthing")

	got := r.run(base)
	if got["internal/engine/core.go"] != "out-of-scope" {
		t.Fatalf("first-PR out-of-scope code path not refused: %v", got)
	}
	for _, p := range []string{"docs/problems/newthing/problem.md", "docs/prds/newthing/000-base.md"} {
		if _, refused := got[p]; refused {
			t.Fatalf("first-PR own-subtree write %q was refused: %v", p, got)
		}
	}
}

// TestGit_UnreadableBaseFailsClosed proves a git failure (an unreadable base
// commit) is a fail-closed defect (the gate reds), never a silent pass. Both an
// empty base and a nonexistent base commit red.
func TestGit_UnreadableBaseFailsClosed(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.commit("chore: seed")

	if d := Evaluate(r.dir, "", dcfg); len(d) != 1 || d[0].Op != "git" {
		t.Fatalf("empty base did not fail closed with a single git defect: %v", d)
	}
	if d := Evaluate(r.dir, "0000000000000000000000000000000000000000", dcfg); len(d) != 1 || d[0].Op != "git" {
		t.Fatalf("nonexistent base commit did not fail closed with a single git defect: %v", d)
	}
}

// TestGit_MixedPRRefusesOnlyOffending proves selectivity across a single PR:
// an Argus commit touching a governance doc AND an in-scope-looking (its own
// slug subtree) doc refuses only the governance doc, and a later human commit's
// unrelated change is untouched.
func TestGit_MixedPRRefusesOnlyOffending(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	r.write(governanceDocument, "vision\n")           // protected — refuse
	r.write("docs/problems/newthing/problem.md", "p") // first-PR own subtree — ok
	r.commitArgus("feat: mixed", "newthing")

	r.write("CONTRIBUTING.md", "human note\n") // human commit, non-protected
	r.commit("docs: human note")

	got := r.run(base)
	if got[governanceDocument] != "governance-document" {
		t.Fatalf("governance doc not refused in a mixed PR: %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("mixed PR refused more than the one offending diff: %v", got)
	}
}

// TestGit_ReplayedTrailerUnderivableFailsClosed is the A-3 replay half: a
// trailer lifted verbatim from a real Argus commit onto a DIFFERENT commit fails
// the in-tree content binding (it binds the original commit's tree+parents, not
// this one's), so it derives underivable — and on a protected-path diff, fails
// closed (never a silent pass, never re-classified as Argus).
func TestGit_ReplayedTrailerUnderivableFailsClosed(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	// A genuine Argus commit on an in-scope path — its Argus-Bind is derived over
	// ITS OWN tree+parents.
	r.write("docs/problems/widget/problem.md", "p\n")
	legit := r.commitArgus("feat: legit in-scope", "widget")
	lifted := r.bindTrailer(legit)
	if lifted == "" {
		t.Fatal("the genuine Argus commit carried no content binding")
	}

	// Lift that exact drive+bind trailer onto a different commit (off the same
	// base) touching a protected path. The binding does not re-derive here.
	r.git("checkout", "-q", base)
	r.write(authorityContract, "policy\n")
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "feat: replay\n\n"+TrailerKey+": widget\n"+BindTrailerKey+": "+lifted+"\n")

	got := r.run(base)
	if got[authorityContract] != "authority-contract" {
		t.Fatalf("replayed-trailer protected-path diff not failed closed: %v", got)
	}
	// It went the UNDERIVABLE path (replay detected), not classified as Argus.
	for _, d := range Evaluate(r.dir, base, dcfg) {
		if d.File == authorityContract && !strings.Contains(d.Reason, "underivable") {
			t.Fatalf("replayed trailer was not derived underivable: %q", d.Reason)
		}
	}
}

func TestGit_DraftRoundAndPreparedRecordsAreFirstPRScope(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")
	r.write("drafts/widget/prd.md", "# Widget draft\n")
	round := r.commitArgus("docs(draft): author widget", "widget")
	if got := r.run(base); len(got) != 0 {
		t.Fatalf("slug-scoped draft round was refused: %v", got)
	}

	prepare := func() {
		r.git("rm", "-r", "--", "drafts/widget")
		r.write("memory/prd/widget/Document-prd.json", "{}\n")
		r.write("memory/design/widget/Document-design.json", "{}\n")
	}
	prepare()
	r.commit("docs(draft): prepare lock widget") // Pyxis-generated, no drive slug
	if got := r.run(base); len(got) != 0 {
		t.Fatalf("prepared head should pass after draft working files are removed: %v", got)
	}

	// Even if a generated record is stamped, its own slug's compiled-record
	// subtree is in scope; the command still leaves its own commit unstamped.
	r.git("checkout", "-q", round)
	prepare()
	r.commitArgus("docs(draft): prepare lock widget", "widget")
	if got := r.run(base); len(got) != 0 {
		t.Fatalf("slug-scoped compiled records were refused: %v", got)
	}
}

func TestGit_CanonicalProblemCapturesStayInFirstPRSlug(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")
	for _, path := range []string{
		"docs/problems/widget/problem.md",
		"docs/problems/widget/delivery.md",
		"memory/problem/widget/Document-problem.json",
		"memory/problem/widget/Document-problem.md",
		"memory/problem/widget/Document-delivery.json",
		"memory/problem/widget/Document-delivery.md",
	} {
		r.write(path, "native source capture fixture\n")
	}
	r.commitArgus("docs(argus): capture widget front door", "widget")
	if got := r.run(base); len(got) != 0 {
		t.Fatalf("own canonical ledgers and native captures refused: %v", got)
	}
	for _, path := range []string{
		"memory/problem/other/Document-problem.json",
		"memory/problem/other/Document-problem.md",
		authorityContract,
		governanceDocument,
		"internal/engine/core.go",
	} {
		r.write(path, "foreign or protected fixture\n")
	}
	r.commitArgus("docs(argus): attempt unrelated captures", "widget")
	got := r.run(base)
	for path, want := range map[string]string{
		"memory/problem/other/Document-problem.json": "out-of-scope",
		"memory/problem/other/Document-problem.md":   "out-of-scope",
		authorityContract:         "authority-contract",
		governanceDocument:        "governance-document",
		"internal/engine/core.go": "out-of-scope",
	} {
		if got[path] != want {
			t.Errorf("unrelated path %s classified %q, want %q", path, got[path], want)
		}
	}
	if len(got) != 5 {
		t.Fatalf("expected only five unrelated paths to refuse: %v", got)
	}
}

// TestGit_BothSignalsCollisionRefuses is the A-3 collision half (FND-1): a
// protected-path commit carrying BOTH a produced, content-bound Argus stamp AND
// a planted `Governing-Change: operator` attestation is REFUSED — positively-
// Argus wins the precedence, so a drive-plantable human attestation can never
// launder a positively-Argus protected-path diff (DINV-5).
func TestGit_BothSignalsCollisionRefuses(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	// The drive stamps a real Argus commit AND plants the attestation in the body.
	r.write(governanceDocument, "vision\n")
	r.commitArgus("feat: touch vision\n\nGoverning-Change: operator", "widget")

	got := r.run(base)
	if got[governanceDocument] != "governance-document" {
		t.Fatalf("both-signals collision not refused (positively-Argus must win): %v", got)
	}
	// The refusal names the positively-Argus case, not the human one.
	for _, d := range Evaluate(r.dir, base, dcfg) {
		if d.File == governanceDocument && !strings.Contains(d.Reason, "Argus-authored") {
			t.Fatalf("collision not refused as Argus-authored: %q", d.Reason)
		}
	}
}

// TestGit_PositivelyHumanAttestedPasses is the A-4/NR-4 pass: a protected-path
// diff with NO Argus stamp but carrying the positive human attestation derives
// positively-human and passes — the human-author path stays open, with no step
// beyond the ordinary attestation the class already requires.
func TestGit_PositivelyHumanAttestedPasses(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")

	r.write(authorityContract, "policy\n")
	r.write(governanceDocument, "vision\n")
	r.commit("docs: operator lands a governing change\n\nGoverning-Change: operator")

	if got := r.run(base); len(got) != 0 {
		t.Fatalf("positively-human (attested) protected-path diff was refused: %v", got)
	}
}

// TestGit_AbsentSignalUnderivableFailsClosed is the A-2 absence case: a
// protected-path diff with NEITHER a produced Argus stamp NOR a positive human
// attestation is underivable and fails closed — absence is no longer a
// masquerading "human" (§3.3, DINV-2). The rule stays scoped to the protected
// subset: a non-protected unattested diff is not refused.
func TestGit_AbsentSignalUnderivableFailsClosed(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	base := r.commit("chore: seed")
	r.write(governanceDocument, "vision\n")
	r.commit("docs: unattested vision edit") // no Argus stamp, no attestation
	got := r.run(base)
	if got[governanceDocument] != "governance-document" {
		t.Fatalf("unattested protected-path diff not failed closed as underivable: %v", got)
	}
	for _, d := range Evaluate(r.dir, base, dcfg) {
		if d.File == governanceDocument && !strings.Contains(d.Reason, "underivable") {
			t.Fatalf("unattested protected-path diff not derived underivable: %q", d.Reason)
		}
	}

	// A non-protected, unattested diff is NOT refused (fail-closed scoped to the
	// protected subset — a normal human/code PR needs no attestation).
	r2 := newRepo(t)
	r2.write("README.md", "seed\n")
	base2 := r2.commit("chore: seed")
	r2.write("internal/tool/x.go", "package tool\n")
	r2.commit("feat: ordinary code") // unattested, non-protected
	if got := r2.run(base2); len(got) != 0 {
		t.Fatalf("unattested non-protected diff was refused: %v", got)
	}
}

// TestGit_FromHereForward_PreBaseHistoryNotRefused proves the from-here-forward
// transition (§3.7, A-6): the gate evaluates only the PR's own base..HEAD range,
// so a positively-Argus protected-path commit that lands BEFORE the merge base —
// pre-mechanism history — is never in range and is never retroactively refused,
// while the byte-identical change committed IN range on the same repo IS refused.
// The range is the from-here-forward guarantee: nothing before the fork point can
// be false-refused, so no back-fill of history is needed (and none is done).
func TestGit_FromHereForward_PreBaseHistoryNotRefused(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "seed\n")
	r.commit("chore: seed")

	// Pre-mechanism HISTORY: a positively-Argus protected-path commit landed
	// before the PR's fork point. (A real pre-mechanism commit would carry no
	// produced signal at all; a produced one before the base is likewise out of
	// range — the point is the range, not the signal.)
	r.write(authorityContract, "policy v1\n")
	r.commitArgus("feat: historical policy touch", "widget")

	// The PR forks HERE — everything above is history, outside base..HEAD.
	base := r.git("rev-parse", "HEAD")

	// The PR's own commit touches only a non-protected path (an ordinary edit
	// that trips no rule on its own), so any refusal could only come from the
	// out-of-range historical commit.
	r.write("docs/wiki/note.md", "an ordinary in-range note\n")
	r.commit("docs: in-range note")

	if got := r.run(base); len(got) != 0 {
		t.Fatalf("pre-mechanism history (an out-of-range Argus protected-path commit) was retroactively refused: %v", got)
	}

	// Control: the SAME protected-path change committed IN range IS refused —
	// proving the pass above is the range at work, not a derivation that stopped
	// biting.
	r2 := newRepo(t)
	r2.write("README.md", "seed\n")
	base2 := r2.commit("chore: seed")
	r2.write(authorityContract, "policy v1\n")
	r2.commitArgus("feat: in-range policy touch", "widget")
	if got := r2.run(base2); got[authorityContract] != "authority-contract" {
		t.Fatalf("the in-range Argus authority-contract diff was not refused (the control): %v", got)
	}
}
