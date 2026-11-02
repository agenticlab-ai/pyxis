package cli

// End-to-end: every verb driven in-process the way the binary drives it,
// against real git repositories in temp directories — a bare origin shared by
// two clones where the flows need sharing.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/search"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var t0 = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

// TestMain lets this test binary double as a judge command for collide.
func TestMain(m *testing.M) {
	// Set the Git isolation once for the test process and all of its children.
	// Per-test environment mutations prevented independent repository fixtures
	// from running in parallel.
	if err := os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull); err != nil {
		panic(err)
	}
	if err := os.Setenv("GIT_CONFIG_NOSYSTEM", "1"); err != nil {
		panic(err)
	}
	if os.Getenv("PYXIS_TEST_GIT_PROXY") == "1" && filepath.Base(os.Args[0]) == "git" {
		runEvidenceGitProxy()
		return
	}
	if os.Getenv("MEMORY_TEST_JUDGE") == "" {
		os.Exit(m.Run())
	}
	var in search.JudgeInput
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		os.Exit(3)
	}
	out := search.JudgeOutput{}
	for i, c := range in.Candidates {
		rel := search.RelUnrelated
		if i == 0 {
			rel = search.RelSupersedes
		}
		out.Verdicts = append(out.Verdicts, search.Verdict{ID: c.ID, Relation: rel, Why: "judged"})
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

type harness struct {
	t       *testing.T
	dir     string
	clock   time.Time
	entropy uint16
	envs    map[string]string
}

func newHarness(t *testing.T, dir string) *harness {
	return &harness{t: t, dir: dir, clock: t0, envs: map[string]string{}}
}

func (h *harness) env(stdin string) (Env, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return Env{
		Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errb, Dir: h.dir,
		Now: func() time.Time { h.clock = h.clock.Add(time.Second); return h.clock },
		Entropy: func() ([10]byte, error) {
			h.entropy++
			var e [10]byte
			e[8], e[9] = byte(h.entropy>>8), byte(h.entropy)
			return e, nil
		},
		Getenv: func(k string) string { return h.envs[k] },
	}, &out, &errb
}

func (h *harness) run(args ...string) (int, string, string) {
	env, out, errb := h.env("")
	code := Run(context.Background(), args, env)
	return code, out.String(), errb.String()
}

func (h *harness) runStdin(stdin string, args ...string) (int, string, string) {
	env, out, errb := h.env(stdin)
	code := Run(context.Background(), args, env)
	return code, out.String(), errb.String()
}

// ok runs a verb that must succeed and returns its stdout.
func (h *harness) ok(args ...string) string {
	h.t.Helper()
	code, out, errb := h.run(args...)
	if code != 0 {
		h.t.Fatalf("memory %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errb)
	}
	return out
}

// add runs add and returns the new id.
func (h *harness) add(args ...string) string {
	h.t.Helper()
	out := h.ok(append([]string{"add"}, args...)...)
	return strings.SplitN(out, "\n", 2)[0]
}

func (h *harness) git(args ...string) string {
	h.t.Helper()
	return gitIn(h.t, h.dir, args...)
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=m@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=m@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func hermeticGit(t *testing.T) {
	t.Helper()
	if os.Getenv("GIT_CONFIG_GLOBAL") != os.DevNull || os.Getenv("GIT_CONFIG_NOSYSTEM") != "1" {
		t.Fatal("test Git configuration is not isolated")
	}
}

func initRepo(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "config", "user.email", "m@example.com")
	gitIn(t, dir, "config", "user.name", "m")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return newHarness(t, dir)
}

func (h *harness) commit(msg string) {
	h.t.Helper()
	h.git("add", "-A")
	h.git("commit", "-q", "-m", msg)
}

func want(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Fatalf("output lacks %q:\n%s", w, got)
		}
	}
}

// contract seeds a small product contract on main: a goal, two promises, an
// expectation, and a design decision serving one promise and the expectation.
func contract(h *harness) map[string]string {
	h.t.Helper()
	ids := map[string]string{}
	writeFixtureFile(h.t, h.dir, "internal/executor/executor.go", "package executor\n", 0o644)
	h.commit("seed executor source evidence")
	h.ok("init")
	ids["G-1"] = h.add("--kind", "Need", "--subject", "prd/stacked-prs", "--family", "G-1", "--statement", "Reviewers see one reviewable slice per PR.",
		"--intent", "make review cheap", "--rationale", "large PRs were skimmed, not read")
	ids["FR-1"] = h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-1",
		"--statement", "When a task produces more than one reviewable slice, the executor shall open one PR per slice, each based on the previous.",
		"--intent", "one reviewable unit per PR", "--rationale", "stacking keeps each review small", "--fit", "a three-slice task yields three PRs in a chain",
		"--tag", "review", "--tag", "code:internal/executor", "--learned-from", "incident:I2")
	ids["FR-2"] = h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-2",
		"--statement", "When the top PR of a stack merges, the executor shall retarget the remaining PRs to main.", "--intent", "no orphaned branches", "--tag", "code:internal/executor")
	ids["DOD-1"] = h.add("--kind", "Expectation", "--subject", "prd/stacked-prs", "--family", "DOD-1", "--statement", "Every slice PR carries its own tests.")
	ids["anchor"] = h.add("--kind", "Need", "--subject", "code/executor", "--family", "anchor", "--statement", "package internal/executor exists", "--tag", "code:internal/executor")
	ids["D-1"] = h.add("--kind", "Decision", "--subject", "design/stacked-prs", "--family", "D-1",
		"--statement", "The executor keeps a stack manifest per task; each slice branch is based on the previous slice's head.",
		"--intent", "serve one PR per slice", "--serves", "prd/stacked-prs#FR-1", "--serves", ids["DOD-1"], "--about", ids["anchor"])
	h.commit("contract")
	return ids
}

func TestE2EContractViewsAndLateDiscovery(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	prd := h.ok("view", "prd/stacked-prs")
	want(t, prd, "# prd/stacked-prs", "## Needs", "### G-1", "## Promises", "### FR-1", "- intent: one reviewable unit per PR", "- fit criterion: a three-slice task",
		"- learned from: incident:I2", "- tags: review, code:internal/executor", " · served by design/stacked-prs#D-1", "### FR-2", " · **undesigned**", "## Expectations", "### DOD-1")
	design := h.ok("view", "design/stacked-prs")
	want(t, design, "# design/stacked-prs", "### D-1", "- serves prd/stacked-prs#FR-1: When a task produces", "- about code/executor")
	want(t, h.ok("delivery", "prd/stacked-prs"), "| FR-1 | design/stacked-prs#D-1 | designed |", "| FR-2 |  | **undesigned** |", "2 of 3 claims designed; 1 undesigned")
	want(t, h.ok("status"), "| prd/stacked-prs | 4 | 0 | 0 |", "- nothing")
	want(t, h.ok("why", "prd/stacked-prs#FR-1"), "# why prd/stacked-prs#FR-1 says what it says", "## now · active", "## Rests on it now", "- serves design/stacked-prs#D-1 (Decision)")

	// planning finds FR-1 unverifiable: a branch carries the learning and the corrected promise
	h.git("checkout", "-q", "-b", "feature/late-discovery")
	learning := h.add("--kind", "Learning", "--subject", "prd/stacked-prs", "--statement", "Planning found FR-1's fit criterion cannot be verified as written: slices are not known before the task runs.",
		"--learned-from", "plan:stacked-prs/001")
	fr1b := h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-1",
		"--statement", "When a task produces more than one reviewable slice, the executor shall open one PR per slice as each slice reaches green, each based on the previous.",
		"--intent", "one reviewable unit per PR, verifiable as slices appear", "--rationale", "slices are discovered while the task runs, so the criterion must be evaluated incrementally",
		"--supersedes", "prd/stacked-prs#FR-1", "--learned-from", "plan:stacked-prs/001")
	code, out, _ := h.run("packet", "--base", "main")
	if code != 0 {
		t.Fatalf("packet must not block: %s", out)
	}
	want(t, out, "proposal on feature/late-discovery against main@", "2 records added, consequence medium", "+ Learning", "+ Promise", "(supersedes 1)",
		"! reopens: "+ids["D-1"]+" (design/stacked-prs#D-1) serves "+ids["FR-1"], "check: ok")
	body := h.ok("packet", "--base", "main", "--body", "--description", "late discovery: FR-1 verifiable as slices appear")
	want(t, body, "# late discovery: FR-1 verifiable as slices appear", "## Product sign-off — the promise delta", "### Promise FR-1 on prd/stacked-prs — replaces the current promise",
		"- **before** (since 2026-09-03): When a task produces more than one reviewable slice, the executor shall open one PR per slice, each based on the previous.",
		"- **after**: When a task produces more than one reviewable slice, the executor shall open one PR per slice as each slice reaches green",
		"- **learned from**: plan:stacked-prs/001", "## Engineering sign-off — the design delta", "- no decisions or plans change", "## What it reopens",
		"design/stacked-prs#D-1 serves "+ids["FR-1"], "**Learning** on prd/stacked-prs: Planning found")
	// the author carries the design successor the packet asked for
	d1b := h.add("--kind", "Decision", "--subject", "design/stacked-prs", "--family", "D-1",
		"--statement", "The executor keeps a stack manifest per task; a slice branch is opened when the slice reaches green, based on the previous slice's head.",
		"--intent", "keep the design pointing at FR-1 as corrected", "--supersedes", "design/stacked-prs#D-1", "--serves", fr1b, "--serves", "prd/stacked-prs#DOD-1", "--about", ids["anchor"])
	out = h.ok("check", "--base", "main")
	want(t, out, "3 records added", "check: ok")
	if strings.Contains(out, "! reopens") {
		t.Fatalf("carrying the successor closes the reopen:\n%s", out)
	}
	since := h.ok("view", "prd/stacked-prs", "--since", "main")
	want(t, since, "**Changed since the reference**: 2 records new here", "### FR-1 **(replaced)**", "· 1 earlier version(s)", "## Learnings", "Planning found FR-1's fit criterion")
	want(t, h.ok("view", "design/stacked-prs"), "### D-1", "a slice branch is opened when the slice reaches green")
	why := h.ok("why", "prd/stacked-prs#FR-1")
	want(t, why, "## now · active · 2026-09-03 · "+fr1b, "- why: slices are discovered while the task runs", "## v-1 · superseded · 2026-09-03 · "+ids["FR-1"], "- serves design/stacked-prs#D-1")
	h.commit("late discovery")
	h.git("checkout", "-q", "main")
	h.git("merge", "-q", "--no-ff", "-m", "merge late discovery", "feature/late-discovery")
	want(t, h.ok("status"), "| prd/stacked-prs | 5 | 1 | 0 |", "| design/stacked-prs | 1 | 1 | 0 |", "- nothing")
	want(t, h.ok("delivery", "prd/stacked-prs"), "| FR-1 | design/stacked-prs#D-1 | designed |")
	_ = learning
	_ = d1b
}

func TestE2EAdmissionRefusalsAndMergeCheck(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	// a claim in a family that already has a head must name it
	code, _, errb := h.run("add", "--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-1", "--statement", "a second FR-1")
	if code != 1 || !strings.Contains(errb, "conflict-at-admission") || !strings.Contains(errb, ids["FR-1"]) {
		t.Fatalf("exit %d: %s", code, errb)
	}
	// two branches change the same family; the second to merge is blocked until it rebases
	h.git("checkout", "-q", "-b", "feature/a")
	a := h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-2", "--statement", "retarget within one minute of the merge", "--supersedes", ids["FR-2"])
	h.commit("a")
	h.git("checkout", "-q", "main")
	h.git("checkout", "-q", "-b", "feature/b")
	b := h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-2", "--statement", "retarget and re-run checks", "--supersedes", ids["FR-2"])
	h.commit("b")
	out := h.ok("packet", "--base", "main")
	want(t, out, "! concurrent: feature/a also changes prd/stacked-prs#FR-2 ("+a+")", "check: ok")
	h.git("checkout", "-q", "main")
	h.git("merge", "-q", "--no-ff", "-m", "merge a", "feature/a")
	h.git("checkout", "-q", "feature/b")
	code, out, errb = h.run("check", "--base", "main")
	if code != 1 {
		t.Fatalf("stale branch must be blocked: exit %d\n%s%s", code, out, errb)
	}
	want(t, out, "✗ effective-set-moved: "+b+" replaces "+ids["FR-2"]+", which is superseded on main@", "; the head is now "+a, "✗ conflict-at-admission: prd/stacked-prs#FR-2 would have 2 active claims", "check: blocked")
	want(t, errb, "2 blocker(s)")
	// rebasing the claim: at write time the tool refuses a stale target too
	h.git("merge", "-q", "main")
	code, _, errb = h.run("add", "--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-2", "--statement", "rebased", "--supersedes", ids["FR-2"])
	if code != 1 || !strings.Contains(errb, "effective-set-moved") || !strings.Contains(errb, "the head is now ") {
		t.Fatalf("exit %d: %s", code, errb)
	}
	want(t, h.ok("status"), "- conflict: prd/stacked-prs#FR-2 has 2 active claims")
	// a Revocation of the stale record plus a claim on the current head lands cleanly
	h.ok("revoke", b, "--reason", "written against a head that moved")
	h.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-2", "--statement", "retarget within one minute and re-run checks", "--supersedes", a)
	h.commit("rebased")
	want(t, h.ok("check", "--base", "main"), "check: ok")

	// immutability: editing or deleting a stored record is refused at the merge check
	h.git("checkout", "-q", "main")
	h.git("checkout", "-q", "-b", "feature/tamper")
	path := filepath.Join(h.dir, "memory", "prd", "stacked-prs", ids["G-1"]+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte("Reviewers see"), []byte("Reviewers get"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.dir, "memory", "prd", "stacked-prs", ids["DOD-1"]+".json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "memory", "prd", "stacked-prs", "notes.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ = h.run("check", "--base", "main")
	if code != 1 {
		t.Fatalf("tampering must be blocked:\n%s", out)
	}
	want(t, out, "✗ record-modified: prd/stacked-prs/"+ids["G-1"]+".json changed after it was stored", "✗ record-deleted: prd/stacked-prs/"+ids["DOD-1"]+".json was deleted",
		"✗ record-invalid: prd/stacked-prs/notes.json")
	body := h.ok("packet", "--base", "main", "--body")
	want(t, body, "## Blockers — this cannot merge as it stands", "**record-modified**")
	h.git("checkout", "-q", "--", ".")
	h.git("clean", "-fdq")
	// revoking is the append-only way to withdraw; dependents are listed for reconsideration
	out = h.ok("revoke", ids["DOD-1"], "--reason", "tests ride the top PR instead")
	want(t, out, "reconsider — these rest on "+ids["DOD-1"], "serves design/stacked-prs#D-1 ("+ids["D-1"]+")")
	code, _, errb = h.run("revoke", ids["DOD-1"], "--reason", "again")
	if code != 1 || !strings.Contains(errb, "already revoked") {
		t.Fatalf("double revoke: %d %s", code, errb)
	}
	want(t, h.ok("check", "--base", "main"), "+ Revocation", "check: ok")
	want(t, h.ok("status"), "- reopened: design/stacked-prs#D-1 serves "+ids["DOD-1"]+", which is revoked")
	_ = b
}

func TestE2EAskOnceAndPack(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	out := h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Should the reviewer of a slice PR see the whole stack or only its own diff?")
	want(t, out, "new question Question-")
	qid := strings.Fields(out)[2]
	out = h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Does the reviewer of a slice PR see the whole stack or only its own diff?")
	want(t, out, "asked already and still open ("+qid, "wait for its answer, or pass --anyway")
	h.ok("answer", qid, "--text", "Only its own diff; the stack is linked from the PR body.")
	out = h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Should a slice reviewer see the whole stack or only its own diff?")
	want(t, out, "answered already ("+qid, "answer (m@example.com, 2026-09-03): Only its own diff")
	// one active answer per question: a second is refused, a correction replaces, a revoked correction restores the first
	if code, _, errb := h.run("answer", qid, "--text", "The whole stack."); code != 1 || !strings.Contains(errb, "already has an answer") {
		t.Fatalf("second answer: %d %s", code, errb)
	}
	replacement := strings.Split(h.ok("answer", qid, "--text", "The whole stack, collapsed by default.", "--replace"), "\n")[0]
	want(t, h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Should a slice reviewer see the whole stack or only its own diff?"), "The whole stack, collapsed by default.")
	h.ok("revoke", replacement, "--reason", "the correction was wrong")
	want(t, h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Should a slice reviewer see the whole stack or only its own diff?"), "Only its own diff")
	if code, _, errb := h.run("answer", strings.Fields(h.ok("ask", "--subject", "prd/stacked-prs", "--text", "Is there a maximum stack depth?"))[2], "--text", "x", "--replace"); code != 1 || !strings.Contains(errb, "no active answer to replace") {
		t.Fatalf("replace with nothing to replace: %d %s", code, errb)
	}
	if code, _, _ := h.run("ask", "--subject", "prd/stacked-prs", "--text", "Should a slice reviewer see the whole stack?", "--anyway"); code != 0 {
		t.Fatal("--anyway records a new question")
	}
	h.ok("ask", "--subject", "prd/stacked-prs", "--text", "How many slices may a stack have?")
	code, _, errb := h.run("answer", ids["FR-1"], "--text", "not a question")
	if code != 2 || !strings.Contains(errb, "is not a question") {
		t.Fatalf("answering a promise: %d %s", code, errb)
	}
	want(t, h.ok("view", "prd/stacked-prs"), "## Questions", "- **open** (", "How many slices may a stack have?", "→ **Only its own diff; the stack is linked from the PR body.**")

	pk := h.ok("pack", "--subject", "prd/stacked-prs", "--budget", "4000")
	want(t, pk, "# Task pack — prd/stacked-prs", "budget 4000 tokens", "## Requirements (the task)", "- **Promise FR-1** (prd/stacked-prs, "+ids["FR-1"]+")",
		"## Decisions that serve them", "- **Decision D-1** (design/stacked-prs, "+ids["D-1"]+")", "- serves: prd/stacked-prs#FR-1, prd/stacked-prs#DOD-1",
		"## Open questions — ask before assuming", "How many slices may a stack have?", "## Already answered — do not ask again", "Only its own diff", "- nothing: the pack is complete for its scope")
	pk = h.ok("pack", "--subject", "prd/stacked-prs", "--family", "FR-2")
	want(t, pk, "- **Promise FR-2**", "- no decision serves these claims: they are undesigned", "- adjacent, not requested: prd/stacked-prs#DOD-1", "- adjacent, not requested: prd/stacked-prs#FR-1")
	if strings.Contains(pk, "Promise FR-1**") {
		t.Fatalf("FR-1 is out of scope:\n%s", pk)
	}
	var parsed struct {
		Requirements []map[string]any `json:"requirements"`
		Omissions    []string         `json:"omissions"`
		Used         int              `json:"used_tokens"`
	}
	out = h.ok("pack", "--subject", "prd/stacked-prs", "--budget", "700", "--json")
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("pack --json: %v\n%s", err, out)
	}
	// the budget bounds the JSON actually delivered, newline included: the reported figure is the delivered size
	if parsed.Used > 700 || record.Tokens(out) != parsed.Used || len(parsed.Omissions) == 0 {
		t.Fatalf("budget: used %d, delivered %d tokens, %+v", parsed.Used, record.Tokens(out), parsed.Omissions)
	}
	for _, budget := range []string{"400", "1200"} {
		var again struct {
			Used int `json:"used_tokens"`
		}
		raw := h.ok("pack", "--subject", "prd/stacked-prs", "--budget", budget, "--json")
		if err := json.Unmarshal([]byte(raw), &again); err != nil || again.Used != record.Tokens(raw) {
			t.Fatalf("budget %s: reported %d, delivered %d (%v)", budget, again.Used, record.Tokens(raw), err)
		}
	}
	if code, _, errb := h.run("pack", "--subject", "prd/stacked-prs", "--budget", "-1"); code != 2 || !strings.Contains(errb, "positive") {
		t.Fatalf("negative budget: %d %s", code, errb)
	}
	if code, _, errb := h.run("pack", "--subject", "prd/stacked-prs", "--budget", "1"); code != 1 || !strings.Contains(errb, "too small") {
		t.Fatalf("infeasible budget: %d %s", code, errb)
	}
	md := h.ok("pack", "--subject", "prd/stacked-prs", "--budget", "500")
	if record.Tokens(md) > 500 {
		t.Fatalf("the markdown delivered is within budget: %d tokens", record.Tokens(md))
	}
}

func TestPackAcceptsCodeOnlyScope(t *testing.T) {
	h := initRepo(t)
	contract(h)
	// The existing fixture decision has a direct code association. No duplicate
	// learning or subject-specific copy is needed to reach it from a task path.
	out := h.ok("pack", "--code", "internal/executor", "--json")
	want(t, out, `"code"`, "internal/executor", "The executor keeps a stack manifest per task")
	if code, _, errb := h.run("pack", "--code", "../outside"); code != 1 || !strings.Contains(errb, "repository-relative") {
		t.Fatalf("bad scope: %d %s", code, errb)
	}
}

func TestE2ESearchAndCollide(t *testing.T) {
	h := initRepo(t)
	ids := contract(h)
	out := h.ok("search", "retarget remaining PRs after the top merges")
	want(t, out, " 1 ", ids["FR-2"], "prd/stacked-prs#FR-2 [active]")
	out = h.ok("search", "slice", "--tag", "review", "--code", "internal/executor/stack.go", "-k", "2")
	want(t, out, ids["FR-1"], "boosts: tag:review, code:internal/executor")
	out = h.ok("search", "zzz", "--code", "internal/executor/stack.go")
	want(t, out, ids["D-1"], "boosts: code:internal/executor")
	want(t, h.ok("search", "zzz"), "nothing matches")
	out = h.ok("collide", "--statement", "When a task produces several reviewable slices, open one PR per slice, each based on the previous one.",
		"--subject", "prd/stacked-prs", "--family", "FR-1", "--kind", "Promise")
	want(t, out, "candidates (judge: heuristic)", " 1 ", "supersedes "+ids["FR-1"], "the active head of the draft's family", "suggested: --supersedes "+ids["FR-1"])
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEMORY_TEST_JUDGE", "1")
	out = h.ok("collide", "--statement", "retarget the remaining PRs to main", "--subject", "prd/stacked-prs", "--judge-cmd", exe+" -test.run=NONE", "--json")
	var parsed struct {
		Judge      string            `json:"judge"`
		Candidates []json.RawMessage `json:"candidates"`
		Verdicts   []search.Verdict  `json:"verdicts"`
		Supersedes []string          `json:"supersedes"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("collide --json: %v\n%s", err, out)
	}
	if !strings.HasPrefix(parsed.Judge, exe) || len(parsed.Candidates) == 0 || len(parsed.Verdicts) != len(parsed.Candidates) || parsed.Verdicts[0].Why != "judged" || len(parsed.Supersedes) != 1 {
		t.Fatalf("judge output: %+v", parsed)
	}
	out = h.ok("collide", "--statement", "retarget the remaining PRs to main", "--subject", "prd/stacked-prs", "--judge-cmd", exe+" -test.run=NONE")
	want(t, out, "candidates (judge: "+exe, "judged")
	code, _, errb := h.run("collide", "--statement", "retarget the remaining PRs to main", "--judge-cmd", "/no/such/judge")
	if code != 1 || !strings.Contains(errb, "judge") {
		t.Fatalf("missing judge: %d %s", code, errb)
	}
	want(t, h.ok("collide", "--statement", "zzz qqq", "--no-inflight"), "no candidates")
}

func TestE2ESharingThroughOrigin(t *testing.T) {
	t.Parallel()
	hermeticGit(t)
	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "--bare", "-b", "main")
	cloneA := filepath.Join(t.TempDir(), "a")
	cloneB := filepath.Join(t.TempDir(), "b")
	gitIn(t, filepath.Dir(cloneA), "clone", "-q", origin, cloneA)
	gitIn(t, filepath.Dir(cloneB), "clone", "-q", origin, cloneB)
	for _, dir := range []string{cloneA, cloneB} {
		gitIn(t, dir, "config", "user.email", filepath.Base(dir)+"@example.com")
		gitIn(t, dir, "config", "user.name", filepath.Base(dir))
	}
	a := newHarness(t, cloneA)
	b := newHarness(t, cloneB)
	b.clock = t0.Add(time.Hour) // distinct ids on each side
	b.entropy = 1000
	if err := os.WriteFile(filepath.Join(cloneA, "README.md"), []byte("repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.git("add", "-A")
	a.git("commit", "-q", "-m", "init")
	a.git("push", "-q", "-u", "origin", "main")
	ids := contract(a)
	a.git("push", "-q", "origin", "main")
	b.git("pull", "-q", "origin", "main")
	// A opens a branch that changes FR-1 and pushes it; B works on FR-1 on its own branch
	a.git("checkout", "-q", "-b", "feature/from-a")
	fromA := a.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-1", "--statement", "one PR per slice, opened at green", "--supersedes", ids["FR-1"])
	a.commit("from a")
	a.git("push", "-q", "-u", "origin", "feature/from-a")
	b.git("fetch", "-q", "origin")
	b.git("checkout", "-q", "-b", "feature/from-b")
	fromB := b.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-1", "--statement", "one PR per slice, opened at green, retargeted on merge", "--supersedes", ids["FR-1"])
	b.commit("from b")
	out := b.ok("packet")
	want(t, out, "against origin/main", "! concurrent: origin/feature/from-a also changes prd/stacked-prs#FR-1 ("+fromA+")", "check: ok")
	want(t, b.ok("pack", "--subject", "prd/stacked-prs", "--family", "FR-1"), "## Concurrent — other branches on these families", fromA, "in flight on origin/feature/from-a")
	want(t, b.ok("search", "one PR per slice opened at green"), fromA, "[in flight on origin/feature/from-a]")
	want(t, b.ok("collide", "--statement", "one PR per slice opened at green", "--subject", "prd/stacked-prs", "--family", "FR-1"), "conflicts", "another branch (origin/feature/from-a) is changing the same family")
	// only the active version of a document in flight is searchable by default
	a.git("checkout", "-q", "-b", "feature/doc-from-a")
	aDraft := writeFile(t, t.TempDir(), "prd.md", "# Stacked PRs\n\nThe quokka paragraph explains {{FR-2}} to newcomers.\n")
	docA := a.add("--kind", "Document", "--subject", "prd/stacked-prs", "--family", "prd", "--statement", "Stacked PRs", "--body-file", aDraft)
	aDraft2 := writeFile(t, t.TempDir(), "prd-v2.md", "# Stacked PRs\n\nThe wombat paragraph is the current explanation of {{FR-2}}.\n")
	docA2 := strings.SplitN(a.ok("revise", "prd/stacked-prs#prd", "--body-file", aDraft2), "\n", 2)[0]
	a.commit("document versions from a")
	a.git("push", "-q", "-u", "origin", "feature/doc-from-a")
	b.git("fetch", "-q", "origin")
	if out := b.ok("search", "quokka paragraph newcomers"); strings.Contains(out, docA) {
		t.Fatalf("superseded in-flight document leaked into search:\n%s", out)
	}
	want(t, b.ok("search", "wombat current explanation"), docA2, "[in flight on origin/feature/doc-from-a]")
	a.git("checkout", "-q", "feature/from-a")
	// A merges first; B is now stale and the check says so after a fetch
	a.git("checkout", "-q", "main")
	a.git("merge", "-q", "--no-ff", "-m", "merge from a", "feature/from-a")
	a.git("push", "-q", "origin", "main")
	b.git("fetch", "-q", "origin")
	code, out, _ := b.run("check", "--base", "origin/main")
	if code != 1 {
		t.Fatalf("stale branch must be blocked:\n%s", out)
	}
	want(t, out, "✗ effective-set-moved: "+fromB+" replaces "+ids["FR-1"]+", which is superseded on origin/main@", "; the head is now "+fromA)
	// a local branch and its remote-tracking twin count once
	b.git("push", "-q", "-u", "origin", "feature/from-b")
	b.git("checkout", "-q", "-b", "feature/third")
	third := b.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-3", "--statement", "a third promise")
	out = b.ok("packet")
	if strings.Count(out, "! concurrent") != 0 {
		t.Fatalf("FR-3 has no concurrent work:\n%s", out)
	}
	b.commit("third")
	// a stale local twin never hides what the remote branch has since gained: B tracks A's branch at its old
	// commit, A adds FR-3 there and pushes, B fetches; the packet on feature/third must see the remote's FR-3
	b.git("branch", "feature/from-a", "origin/feature/from-a")
	a.git("checkout", "-q", "feature/from-a")
	fromA3 := a.add("--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-3", "--statement", "a third promise from a")
	a.commit("from a, third")
	a.git("push", "-q", "origin", "feature/from-a")
	b.git("fetch", "-q", "origin")
	want(t, b.ok("packet"), "! concurrent: origin/feature/from-a also changes prd/stacked-prs#FR-3 ("+fromA3+")")
	b.git("checkout", "-q", "feature/from-b")
	_ = third
	want(t, b.ok("view", "prd/stacked-prs", "--since", "origin/main"), "**Changed since the reference**: 1 records new here")
}

func TestE2EUsageErrorsAndJSONAdd(t *testing.T) {
	h := initRepo(t)
	for _, args := range [][]string{{}, {"help"}, {"bogus"}, {"add", "--kind", "Thing", "--subject", "prd/x", "--statement", "s"}, {"add", "--nope"}, {"view"}, {"why"}, {"delivery"},
		{"status", "extra"}, {"search"}, {"collide"}, {"pack"}, {"pack", "--subject", "bad subject"}, {"revoke"}, {"ask"}, {"answer"}, {"add", "--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "s", "--serves", "nope#X"}} {
		if code, _, errb := h.run(args...); code != 2 || errb == "" {
			t.Fatalf("memory %v: exit %d, stderr %q", args, code, errb)
		}
	}
	if code, _, errb := h.run("view", "prd/none"); code != 1 || !strings.Contains(errb, "no records on prd/none") {
		t.Fatalf("view of nothing: %d %s", code, errb)
	}
	if code, _, errb := h.run("why", "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV"); code != 1 || !strings.Contains(errb, "no record") {
		t.Fatalf("why of nothing: %d %s", code, errb)
	}
	if code, _, errb := h.run("packet", "--base", "nope"); code != 2 || !strings.Contains(errb, "does not exist") {
		t.Fatalf("bad base: %d %s", code, errb)
	}
	if code, _, errb := h.run("check"); code != 2 || !strings.Contains(errb, "check needs an explicit --base") {
		t.Fatalf("check without a base: %d %s", code, errb)
	}
	if code, _, errb := h.run("view", "prd/x", "--since", "nope"); code != 1 {
		t.Fatalf("bad since: %d %s", code, errb)
	}
	// no actor anywhere
	h.git("config", "--unset", "user.email")
	if code, _, errb := h.run("add", "--kind", "Need", "--subject", "prd/x", "--family", "G-1", "--statement", "s"); code != 2 || !strings.Contains(errb, "no actor") {
		t.Fatalf("no actor: %d %s", code, errb)
	}
	h.envs["MEMORY_ACTOR"] = "agent:worker-1"
	writeFixtureFile(t, h.dir, "internal/x/x.go", "package x\n", 0o644)
	writeFixtureFile(t, h.dir, "docs/prds/x/000-base.md", "# X\n", 0o644)
	h.commit("pin JSON import evidence")
	code, out, _ := h.runStdin(`{"kind":"Promise","subject":"prd/x","family":"FR-1","statement":"from json","tags":["code:internal/x"],"learned_from":["log:P-1"],"source":"docs/prds/x/000-base.md","note":"pinned import"}`, "add", "--json")
	if code != 0 {
		t.Fatalf("json add: %s", out)
	}
	want(t, h.ok("view", "prd/x"), "by agent:worker-1", "from json", "- learned from: log:P-1")
	want(t, h.ok("why", "prd/x#FR-1"), "- from: docs/prds/x/000-base.md", "pinned import")
	if code, _, errb := h.runStdin(`{"kind":"Promise","colour":"red"}`, "add", "--json"); code != 2 || !strings.Contains(errb, "colour") {
		t.Fatalf("closed json: %d %s", code, errb)
	}
	want(t, h.ok("init"), "initialised")
	want(t, h.ok("init"), "already initialised")
	// outside git there is no memory: every verb refuses, and nothing is written into a standalone tree
	plain := newHarness(t, t.TempDir())
	plain.envs["MEMORY_ACTOR"] = "m"
	for _, args := range [][]string{{"init"}, {"add", "--kind", "Need", "--subject", "prd/p", "--family", "G-1", "--statement", "plain goal"}, {"view", "prd/p"}, {"status"},
		{"check", "--base", "x"}, {"ask", "--subject", "prd/p", "--text", "q?"}} {
		if code, _, errb := plain.run(args...); code != 2 || !strings.Contains(errb, "not inside a git repository") {
			t.Fatalf("memory %v outside git: %d %s", args, code, errb)
		}
	}
	if _, err := os.Stat(filepath.Join(plain.dir, "memory")); !os.IsNotExist(err) {
		t.Fatal("nothing may be written outside a repository")
	}
	// a repository with no base branch yet
	fresh := newHarness(t, t.TempDir())
	gitIn(t, fresh.dir, "init", "-q", "-b", "trunk")
	gitIn(t, fresh.dir, "config", "user.email", "m@example.com")
	fresh.ok("init")
	if code, _, errb := fresh.run("packet"); code != 2 || !strings.Contains(errb, "no base ref") {
		t.Fatalf("no base: %d %s", code, errb)
	}
	// a broken policy stops every verb with the reason
	if err := os.WriteFile(filepath.Join(h.dir, "memory", "policy.json"), []byte(`{"schema":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errb := h.run("status"); code != 1 || !strings.Contains(errb, "policy.json") {
		t.Fatalf("broken policy: %d %s", code, errb)
	}
	// with git unavailable the tool fails closed instead of treating the tree as a standalone memory
	t.Setenv("PATH", t.TempDir())
	if code, _, errb := h.run("add", "--kind", "Need", "--subject", "prd/x", "--family", "G-9", "--statement", "s"); code != 2 || !strings.Contains(errb, "git is unavailable") {
		t.Fatalf("git unavailable: %d %s", code, errb)
	}
	if _, err := os.Stat(filepath.Join(h.dir, "memory", "prd", "x", "Need-")); !os.IsNotExist(err) {
		t.Fatal("nothing may be written when git is unavailable")
	}
}

func TestE2EEntropyAndActorFailures(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	q := strings.Fields(h.ok("ask", "--subject", "prd/stacked-prs", "--text", "a question?"))[2]
	env, _, errb := h.env("")
	env.Entropy = func() ([10]byte, error) { return [10]byte{}, errors.New("no entropy") }
	for _, args := range [][]string{
		{"add", "--kind", "Need", "--subject", "prd/x", "--family", "G-1", "--statement", "s"},
		{"revoke", ids["G-1"], "--reason", "r"},
		{"ask", "--subject", "prd/x", "--text", "brand new question?"},
		{"answer", q, "--text", "a"},
	} {
		errb.Reset()
		if code := Run(context.Background(), args, env); code != 1 || !strings.Contains(errb.String(), "no entropy") {
			t.Fatalf("memory %v without entropy: exit %d, %s", args, code, errb.String())
		}
	}
	h.git("config", "--unset", "user.email")
	for _, args := range [][]string{
		{"revoke", ids["G-1"], "--reason", "r"},
		{"ask", "--subject", "prd/x", "--text", "brand new question?"},
		{"answer", q, "--text", "a"},
	} {
		if code, _, errb := h.run(args...); code != 2 || !strings.Contains(errb, "no actor") {
			t.Fatalf("memory %v without an actor: exit %d, %s", args, code, errb)
		}
	}
	if code, _, errb := h.run("revoke", "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV", "--reason", "r"); code != 2 || !strings.Contains(errb, "no record") {
		t.Fatalf("revoke of nothing: %d %s", code, errb)
	}
	// a memory directory that cannot be written
	if err := os.Chmod(filepath.Join(h.dir, "memory", "prd", "stacked-prs"), 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(filepath.Join(h.dir, "memory", "prd", "stacked-prs"), 0o755) }()
	h.envs["MEMORY_ACTOR"] = "m"
	if code, _, errb := h.run("revoke", ids["G-1"], "--reason", "r"); code != 1 || !strings.Contains(errb, "create") {
		t.Fatalf("unwritable dir: %d %s", code, errb)
	}
	if code, _, errb := h.run("answer", q, "--text", "a"); code != 1 || !strings.Contains(errb, "create") {
		t.Fatalf("unwritable dir: %d %s", code, errb)
	}
	if code, _, errb := h.run("ask", "--subject", "prd/stacked-prs", "--text", "a completely different question about budgets?"); code != 1 || !strings.Contains(errb, "create") {
		t.Fatalf("unwritable dir: %d %s", code, errb)
	}
	if code, _, errb := h.run("add", "--kind", "Need", "--subject", "prd/stacked-prs", "--family", "G-2", "--statement", "s"); code != 1 || !strings.Contains(errb, "create") {
		t.Fatalf("unwritable dir: %d %s", code, errb)
	}
}

// TestE2EConcurrentWritersHoldTheLock: several writers adding a claim to the
// same empty family at once. Admission is check-and-write under the memory's
// lock, so exactly one wins each family and the rest are refused as a
// conflict at admission, never two active heads.
func TestE2EConcurrentWritersHoldTheLock(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	contract(h)
	const writers = 6
	for round, family := range []string{"FR-10", "FR-11", "FR-12"} {
		codes := make(chan int, writers)
		errs := make(chan string, writers)
		for i := range writers {
			w := newHarness(t, h.dir)
			w.clock = t0.Add(time.Duration(round*writers+i+1) * time.Hour)
			w.entropy = uint16(1000 * (round*writers + i + 1))
			go func() {
				code, _, errb := w.run("add", "--kind", "Promise", "--subject", "prd/stacked-prs", "--family", family, "--statement", "raced")
				codes <- code
				errs <- errb
			}()
		}
		won, refused := 0, 0
		for range writers {
			switch code := <-codes; code {
			case 0:
				won++
			case 1:
				refused++
			default:
				t.Fatalf("unexpected exit %d: %s", code, <-errs)
			}
		}
		if won != 1 || refused != writers-1 {
			t.Fatalf("family %s: %d winners, %d refused", family, won, refused)
		}
		for range writers {
			if e := <-errs; e != "" && !strings.Contains(e, "conflict-at-admission") {
				t.Fatalf("a loser is refused as a conflict, got: %s", e)
			}
		}
		if s := h.ok("status"); strings.Contains(s, "- conflict:") {
			t.Fatalf("no family ever has two heads:\n%s", s)
		}
	}
	// the lock lives in the repository's git directory, shared by every writer of the repository whatever
	// their temp dir. The persistent inode is retained; kernel ownership is released.
	lockPath, err := LockPath(context.Background(), h.dir)
	if err != nil || !strings.HasSuffix(lockPath, filepath.Join(".git", "memory-write.lock")) {
		t.Fatalf("lock path %q %v", lockPath, err)
	}
	release, err := store.Lock(lockPath, 0)
	if err != nil {
		t.Fatalf("writer left kernel ownership held: %v", err)
	}
	release()
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	h.clock = t0.Add(100 * time.Hour)
	h.add("--kind", "Need", "--subject", "prd/stacked-prs", "--family", "G-9", "--statement", "after an old unlocked sidecar")
}

func TestJSONAddRefusesDanglingRawRelation(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	existing := h.add("--kind", "Need", "--subject", "prd/x", "--family", "G-1", "--statement", "existing target")
	missing := "Need-01ARZ3NDEKTSV4RRFFQ69G5FAV"
	payload := `{"kind":"Learning","subject":"prd/x","statement":"raw relation","relations":[{"type":"depends_on","target":"` + existing + `"},{"type":"about","target":"` + missing + `"}]}`
	before, err := filepath.Glob(filepath.Join(h.dir, "memory", "prd", "x", "*.json"))
	if err != nil {
		t.Fatal(err)
	}

	code, out, errb := h.runStdin(payload, "add", "--json")
	if code != exitUsage || out != "" || !strings.Contains(errb, "relation about target "+missing+": no record "+missing) {
		t.Fatalf("dangling raw relation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	after, err := filepath.Glob(filepath.Join(h.dir, "memory", "prd", "x", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("a refused raw relation changed the record count from %d to %d", len(before), len(after))
	}
}

func TestClaimAdmissionRequiresEveryActiveClaim(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	first := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "first")
	second := memoryTestRecord(record.Promise, "prd/x", "FR-1", "second", 20)
	putMemoryRecord(t, h.dir, second)
	h.clock = t0.Add(30 * time.Second)
	before, err := filepath.Glob(filepath.Join(h.dir, "memory", "prd", "x", "*.json"))
	if err != nil {
		t.Fatal(err)
	}

	code, out, errout := h.run("add", "--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "partial resolution", "--supersedes", second.ID)
	if code != exitRefused || out != "" || !strings.Contains(errout, "conflict-at-admission") || !strings.Contains(errout, first) {
		t.Fatalf("partial family resolution = exit %d\nstdout: %s\nstderr: %s", code, out, errout)
	}
	after, err := filepath.Glob(filepath.Join(h.dir, "memory", "prd", "x", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("refused partial resolution changed record count from %d to %d", len(before), len(after))
	}
}

func TestJSONAddEnforcesAnswerAndRevocationAdmission(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	question := strings.Fields(h.ok("ask", "--subject", "prd/x", "--text", "which?"))[2]
	answer := strings.Split(h.ok("answer", question, "--text", "first"), "\n")[0]

	code, out, errb := h.runStdin(`{"kind":"Answer","subject":"prd/x","statement":"conflict","relations":[{"type":"answers","target":"`+question+`"}]}`, "add", "--json")
	if code != exitRefused || out != "" || !strings.Contains(errb, "answer-conflict-at-admission") {
		t.Fatalf("duplicate raw Answer: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	code, out, errb = h.runStdin(`{"kind":"Answer","subject":"prd/x","statement":"replacement","relations":[{"type":"answers","target":"`+question+`"},{"type":"supersedes","target":"`+answer+`"}]}`, "add", "--json")
	if code != exitOK || out == "" || errb != "" {
		t.Fatalf("valid raw Answer replacement: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	code, out, errb = h.run("revoke", question, "--reason", "withdraw answered question")
	if code != exitRefused || out != "" || !strings.Contains(errb, "revocation-breaks-projection") {
		t.Fatalf("supported answered-Question Revocation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	code, out, errb = h.runStdin(`{"kind":"Revocation","subject":"prd/x","statement":"withdraw answered question","relations":[{"type":"revokes","target":"`+question+`"}]}`, "add", "--json")
	if code != exitRefused || out != "" || !strings.Contains(errb, "revocation-breaks-projection") {
		t.Fatalf("raw answered-Question Revocation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	code, _, errb = h.run("answer", question, "--text", "must still be answered")
	if code != exitRefused || !strings.Contains(errb, "already has an answer") {
		t.Fatalf("refused Revocation changed the question projection: exit %d stderr %s", code, errb)
	}

	otherQuestion := strings.Fields(h.ok("ask", "--subject", "prd/x", "--text", "where?"))[2]
	code, out, errb = h.runStdin(`{"kind":"Answer","subject":"prd/y","statement":"wrong subject","relations":[{"type":"answers","target":"`+otherQuestion+`"}]}`, "add", "--json")
	if code != exitRefused || out != "" || !strings.Contains(errb, "answers-across-subjects") {
		t.Fatalf("cross-subject raw Answer: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}

	target := h.add("--kind", "Need", "--subject", "prd/x", "--family", "G-2", "--statement", "revocation target")
	code, out, errb = h.runStdin(`{"kind":"Revocation","subject":"prd/y","statement":"wrong subject","relations":[{"type":"revokes","target":"`+target+`"}]}`, "add", "--json")
	if code != exitRefused || out != "" || !strings.Contains(errb, "revokes-across-subjects") {
		t.Fatalf("cross-subject raw Revocation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
	h.ok("revoke", target, "--reason", "done")
	code, out, errb = h.runStdin(`{"kind":"Revocation","subject":"prd/x","statement":"again","relations":[{"type":"revokes","target":"`+target+`"}]}`, "add", "--json")
	if code != exitRefused || out != "" || !strings.Contains(errb, "revokes-inactive-record") {
		t.Fatalf("inactive-target raw Revocation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
	}
}

func TestRevocationAdmissionRejectsReactivationAndAllowsRepair(t *testing.T) {
	t.Parallel()
	t.Run("reactivated answer and family conflicts are refused", func(t *testing.T) {
		h := initRepo(t)
		q := strings.Fields(h.ok("ask", "--subject", "prd/x", "--text", "which?"))[2]
		a1 := strings.Split(h.ok("answer", q, "--text", "one"), "\n")[0]
		a2 := memoryTestRecord(record.Answer, "prd/x", "", "two", 20, relInput(record.Answers, q))
		putMemoryRecord(t, h.dir, a2)
		h.clock = t0.Add(30 * time.Second)
		code, out, errb := h.runStdin(`{"kind":"Answer","subject":"prd/x","statement":"resolution","relations":[{"type":"answers","target":"`+q+`"},{"type":"supersedes","target":"`+a1+`"},{"type":"supersedes","target":"`+a2.ID+`"}]}`, "add", "--json")
		if code != exitOK || errb != "" {
			t.Fatalf("resolve answer conflict: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
		}
		a3 := strings.Split(out, "\n")[0]
		code, out, errb = h.run("revoke", a3, "--reason", "reactivate both answers")
		if code != exitRefused || out != "" || !strings.Contains(errb, "revocation-breaks-projection") || !strings.Contains(errb, "answer conflict") {
			t.Fatalf("answer-conflict reactivation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
		}

		c1 := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "left")
		c2 := memoryTestRecord(record.Promise, "prd/x", "FR-1", "right", 50)
		putMemoryRecord(t, h.dir, c2)
		h.clock = t0.Add(60 * time.Second)
		c3 := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "resolved", "--supersedes", c1, "--supersedes", c2.ID)
		code, out, errb = h.runStdin(`{"kind":"Revocation","subject":"prd/x","statement":"reactivate both claims","relations":[{"type":"revokes","target":"`+c3+`"}]}`, "add", "--json")
		if code != exitRefused || out != "" || !strings.Contains(errb, "revocation-breaks-projection") || !strings.Contains(errb, "family conflict") {
			t.Fatalf("family-conflict reactivation: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
		}
		status := h.ok("status")
		if strings.Contains(status, "answer-conflict") || strings.Contains(status, "conflict: prd/x#FR-1") {
			t.Fatalf("refused revocations changed the live projection:\n%s", status)
		}
	})

	t.Run("revoking an invalid revocation repairs it through both entry points", func(t *testing.T) {
		h := initRepo(t)
		claim := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "kept")
		bad := memoryTestRecord(record.Revocation, "prd/y", "", "wrong subject", 20, relInput(record.Revokes, claim))
		putMemoryRecord(t, h.dir, bad)
		h.clock = t0.Add(30 * time.Second)
		if out := h.ok("revoke", bad.ID, "--reason", "remove invalid act"); !strings.Contains(out, "Revocation-") {
			t.Fatalf("supported repair output: %s", out)
		}
		if status := h.ok("status"); strings.Contains(status, "revokes-across-subjects") {
			t.Fatalf("supported repair left the invalid act live:\n%s", status)
		}

		claim2 := h.add("--kind", "Promise", "--subject", "prd/z", "--family", "FR-2", "--statement", "also kept")
		bad2 := memoryTestRecord(record.Revocation, "prd/y", "", "wrong subject again", 50, relInput(record.Revokes, claim2))
		putMemoryRecord(t, h.dir, bad2)
		h.clock = t0.Add(60 * time.Second)
		code, out, errb := h.runStdin(`{"kind":"Revocation","subject":"prd/y","statement":"remove second invalid act","relations":[{"type":"revokes","target":"`+bad2.ID+`"}]}`, "add", "--json")
		if code != exitOK || out == "" || errb != "" {
			t.Fatalf("raw repair: exit %d\nstdout: %s\nstderr: %s", code, out, errb)
		}
		if status := h.ok("status"); strings.Contains(status, "revokes-across-subjects") {
			t.Fatalf("raw repair left the invalid act live:\n%s", status)
		}
	})

	t.Run("revocation reduces existing conflicts one member at a time", func(t *testing.T) {
		t.Run("family", func(t *testing.T) {
			h := initRepo(t)
			first := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "one")
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Promise, "prd/x", "FR-1", "two", 20))
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Promise, "prd/x", "FR-1", "three", 30))
			h.clock = t0.Add(40 * time.Second)
			if code, out, errb := h.run("revoke", first, "--reason", "remove one"); code != exitOK || out == "" || errb != "" {
				t.Fatalf("partial family repair: exit %d, stdout %q, stderr %q", code, out, errb)
			}
		})

		t.Run("answers", func(t *testing.T) {
			h := initRepo(t)
			question := strings.Fields(h.ok("ask", "--subject", "prd/x", "--text", "which?"))[2]
			first := strings.Split(h.ok("answer", question, "--text", "one"), "\n")[0]
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Answer, "prd/x", "", "two", 20, relInput(record.Answers, question)))
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Answer, "prd/x", "", "three", 30, relInput(record.Answers, question)))
			h.clock = t0.Add(40 * time.Second)
			if code, out, errb := h.run("revoke", first, "--reason", "remove one"); code != exitOK || out == "" || errb != "" {
				t.Fatalf("partial answer repair: exit %d, stdout %q, stderr %q", code, out, errb)
			}
		})

		t.Run("revocations", func(t *testing.T) {
			h := initRepo(t)
			target := h.add("--kind", "Promise", "--subject", "prd/x", "--family", "FR-1", "--statement", "target")
			first := strings.Split(h.ok("revoke", target, "--reason", "one"), "\n")[0]
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Revocation, "prd/x", "", "two", 20, relInput(record.Revokes, target)))
			putMemoryRecord(t, h.dir, memoryTestRecord(record.Revocation, "prd/x", "", "three", 30, relInput(record.Revokes, target)))
			h.clock = t0.Add(40 * time.Second)
			if code, out, errb := h.run("revoke", first, "--reason", "remove one"); code != exitOK || out == "" || errb != "" {
				t.Fatalf("partial revocation repair: exit %d, stdout %q, stderr %q", code, out, errb)
			}
		})
	})
}

func relInput(typ, target string) record.Relation {
	return record.Relation{Type: typ, Target: target}
}

func memoryTestRecord(kind, subject, family, statement string, second int, relations ...record.Relation) *record.Record {
	var entropy [10]byte
	entropy[8], entropy[9] = byte(second>>8), byte(second)
	at := t0.Add(time.Duration(second) * time.Second)
	return &record.Record{Schema: record.Schema, ID: record.NewID(kind, at, entropy), Kind: kind, Subject: subject, Family: family,
		Statement: statement, Issuer: "m@example.com", CreatedAt: at.Format(time.RFC3339), Relations: relations}
}

func putMemoryRecord(t *testing.T, root string, r *record.Record) {
	t.Helper()
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, root, filepath.ToSlash(filepath.Join(store.Dir, r.Path())), string(b))
}

// TestE2EAnswersFollowTheirQuestion: an Answer is bound to an open question;
// it cannot answer a revoked one nor replace another question's answer.
func TestE2EAnswersFollowTheirQuestion(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	contract(h)
	q1 := strings.Fields(h.ok("ask", "--subject", "prd/stacked-prs", "--text", "one?"))[2]
	q2 := strings.Fields(h.ok("ask", "--subject", "prd/stacked-prs", "--text", "two?"))[2]
	a1 := strings.Split(h.ok("answer", q1, "--text", "one!"), "\n")[0]
	h.commit("questions")
	// raw relations through --json cannot make an answer to q2 supersede q1's answer
	code, _, errb := h.runStdin(`{"kind":"Answer","subject":"prd/stacked-prs","statement":"hijack","relations":[{"type":"answers","target":"`+q2+`"},{"type":"supersedes","target":"`+a1+`"}]}`, "add", "--json")
	if code != 1 || !strings.Contains(errb, "supersedes-across-questions") {
		t.Fatalf("cross-question supersession at write time: %d %s", code, errb)
	}
	// a stale branch answers q2 while main revokes it: refused at the gate
	h.git("checkout", "-q", "-b", "feature/late-answer")
	h.ok("answer", q2, "--text", "two!")
	h.commit("late answer")
	h.git("checkout", "-q", "main")
	h.ok("revoke", q2, "--reason", "no longer relevant")
	h.commit("q2 withdrawn")
	h.git("checkout", "-q", "feature/late-answer")
	code, out, _ := h.run("check", "--base", "main")
	if code != 1 || !strings.Contains(out, "✗ revocation-breaks-projection") {
		t.Fatalf("answering a revoked question must block: %d\n%s", code, out)
	}
	// and at write time on a tree where the question is already gone
	h.git("checkout", "-q", "main")
	if code, _, errb := h.run("answer", q2, "--text", "too late"); code != 1 || !strings.Contains(errb, "an answer needs an open question") {
		t.Fatalf("answering a revoked question at write time: %d %s", code, errb)
	}
}

func TestAnswerReplacementRefusesClockRegression(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	q := strings.Fields(h.ok("ask", "--subject", "prd/widget", "--text", "Which answer?"))[2]
	h.ok("answer", q, "--text", "the first")
	before := memoryRecordCount(t, h.dir)
	h.clock = t0.Add(-time.Hour)
	code, _, errout := h.run("answer", q, "--text", "an older correction", "--replace")
	if code != exitRefused || !strings.Contains(errout, "supersedes-out-of-order") {
		t.Fatalf("clock-regressed answer replacement = exit %d, stderr %q", code, errout)
	}
	if after := memoryRecordCount(t, h.dir); after != before {
		t.Fatalf("refused answer replacement wrote a record: before %d, after %d", before, after)
	}
}

func TestPlainQuestionAcceptsIndependentOlderRecord(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")
	h.ok("ask", "--subject", "prd/widget", "--text", "First question?")
	before := memoryRecordCount(t, h.dir)
	h.clock = t0.Add(-time.Hour)
	code, _, errout := h.run("ask", "--subject", "prd/widget", "--text", "Unrelated second question?", "--anyway")
	if code != exitOK {
		t.Fatalf("clock-regressed question = exit %d, stderr %q", code, errout)
	}
	if after := memoryRecordCount(t, h.dir); after != before+1 {
		t.Fatalf("independent question not written: before %d, after %d", before, after)
	}
}

func putFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestE2ETrustedBase: the candidate cannot choose the base it is judged
// against. A config in the candidate's tree that points the mainline at the
// candidate itself buys nothing: the packet refuses a self-base, the gate
// takes its base only from --base, and a defective config stops every verb.
func TestE2ETrustedBase(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	h.git("checkout", "-q", "-b", "gate-bypass")
	putFile(t, h.dir, ".pyxis/config.yaml", "schema_version: "+config.SchemaVersionV2+"\nbranch: gate-bypass\n")
	path := filepath.Join(h.dir, "memory", "prd", "stacked-prs", ids["G-1"]+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Replace(raw, []byte("Reviewers see"), []byte("Reviewers get"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	h.commit("bypass attempt")
	if code, _, errb := h.run("packet"); code != 1 || !strings.Contains(errb, "own commit") {
		t.Fatalf("the configured mainline is the candidate itself; the packet must refuse: %d %s", code, errb)
	}
	if code, _, errb := h.run("check"); code != 2 || !strings.Contains(errb, "explicit --base") {
		t.Fatalf("a gate without a trusted base: %d %s", code, errb)
	}
	if code, _, errb := h.run("check", "--base", "gate-bypass"); code != 1 || !strings.Contains(errb, "own commit") {
		t.Fatalf("self-base: %d %s", code, errb)
	}
	code, out, _ := h.run("check", "--base", "main")
	if code != 1 || !strings.Contains(out, "✗ record-modified: prd/stacked-prs/"+ids["G-1"]+".json") || !strings.Contains(out, "against main@") {
		t.Fatalf("the trusted base catches the edit: %d\n%s", code, out)
	}
	putFile(t, h.dir, ".pyxis/config.yaml", "schema_version: "+config.SchemaVersionV2+"\nbogus: 1\n")
	if code, _, errb := h.run("status"); code != 1 || !strings.Contains(errb, "config has 1 defect") {
		t.Fatalf("a defective config stops the tool: %d %s", code, errb)
	}
}

// TestE2ESymlinkedRecord: a record that is a symlink reads as valid JSON from
// the working tree but git stores only the link target path. Both the working
// tree and the base tree refuse it.
func TestE2ESymlinkedRecord(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	h.git("checkout", "-q", "-b", "feature/link")
	putFile(t, h.dir, "payload.json", string(mustRead(t, filepath.Join(h.dir, "memory", "prd", "stacked-prs", ids["FR-1"]+".json"))))
	link := filepath.Join(h.dir, "memory", "prd", "stacked-prs", "Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV.json")
	if err := os.Symlink("../../../payload.json", link); err != nil {
		t.Fatal(err)
	}
	code, out, _ := h.run("check", "--base", "main")
	if code != 1 || !strings.Contains(out, "✗ record-invalid: prd/stacked-prs/Promise-01ARZ3NDEKTSV4RRFFQ69G5FAV.json: invalid record: a symlink") {
		t.Fatalf("a linked record must be refused: %d\n%s", code, out)
	}
	h.commit("link")
	h.git("checkout", "-q", "main")
	h.git("merge", "-q", "--no-ff", "-m", "merged the link", "feature/link")
	h.git("checkout", "-q", "-b", "feature/after-link")
	h.add("--kind", "Need", "--subject", "prd/stacked-prs", "--family", "G-2", "--statement", "another goal")
	code, out, _ = h.run("check", "--base", "main")
	if code != 1 || !strings.Contains(out, "a symlink") {
		t.Fatalf("the link on the base is still refused when read from git: %d\n%s", code, out)
	}
	if code, _, errb := h.run("add", "--kind", "Need", "--subject", "prd/stacked-prs", "--family", "G-3", "--statement", "s"); code != 0 {
		t.Fatalf("writes beside a link still work: %s", errb)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestE2EPolicyChangeIsHigh: a branch that loosens policy.json cannot use the
// loosened policy to judge itself; the change is surfaced and rated high.
func TestE2EPolicyChangeIsHigh(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	contract(h)
	h.git("checkout", "-q", "-b", "feature/loosen")
	putFile(t, h.dir, "memory/policy.json", `{"schema":1,"consequence":{"by_kind":{"Policy":"low"},"high_subject_prefixes":[]}}`)
	h.add("--kind", "Policy", "--subject", "governance/principles", "--family", "AP-1", "--statement", "agents may merge their own proposals")
	out := h.ok("packet", "--base", "main")
	want(t, out, "1 records added, consequence high", "! policy-changed: policy.json changes in this proposal")
	want(t, h.ok("packet", "--base", "main", "--body"), "## Policy change — operator sign-off")
	h.git("checkout", "-q", "--", ".")
	h.git("clean", "-fdq")
	// a broken policy on the base stops the check with the reason
	h.git("checkout", "-q", "main")
	putFile(t, h.dir, "memory/policy.json", `{"schema":7}`)
	h.commit("broken policy on main")
	h.git("checkout", "-q", "-b", "feature/after-broken")
	if code, _, errb := h.run("check", "--base", "main"); code != 1 || !strings.Contains(errb, "policy.json") {
		t.Fatalf("broken base policy: %d %s", code, errb)
	}
}

// TestE2ESupersessionStaysOnSubject: a claim on one subject cannot retire a
// requirement on another, at write time or at the gate.
func TestE2ESupersessionStaysOnSubject(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	code, _, errb := h.run("add", "--kind", "Promise", "--subject", "prd/other", "--family", "FR-99", "--statement", "retires a stranger", "--supersedes", ids["FR-1"])
	if code != 1 || !strings.Contains(errb, "supersedes-across-subjects") {
		t.Fatalf("write-time refusal: %d %s", code, errb)
	}
	// a hand-written file with the same edge is refused by the gate, and FR-1 stays in its PRD
	h.git("checkout", "-q", "-b", "feature/stranger")
	raw := mustRead(t, filepath.Join(h.dir, "memory", "prd", "stacked-prs", ids["FR-1"]+".json"))
	forged := strings.Replace(string(raw), `"subject": "prd/stacked-prs"`, `"subject": "prd/other"`, 1)
	forged = strings.Replace(forged, `"id": "`+ids["FR-1"]+`"`, `"id": "Promise-01ZZZZZZZZZZZZZZZZZZZZZZZZ"`, 1)
	forged = strings.Replace(forged, `"family": "FR-1"`, `"family": "FR-99"`, 1)
	forged = strings.Replace(forged, `"statement": `, `"relations": [{"type": "supersedes", "target": "`+ids["FR-1"]+`"}], "statement": `, 1)
	putFile(t, h.dir, "memory/prd/other/Promise-01ZZZZZZZZZZZZZZZZZZZZZZZZ.json", forged)
	code, out, _ := h.run("check", "--base", "main")
	if code != 1 || !strings.Contains(out, "✗ supersedes-across-subjects") {
		t.Fatalf("gate: %d\n%s", code, out)
	}
	want(t, h.ok("view", "prd/stacked-prs"), "### FR-1")
	want(t, h.ok("status"), "- supersedes-across-subjects: Promise-01ZZZZZZZZZZZZZZZZZZZZZZZZ on prd/other supersedes "+ids["FR-1"])
}
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestE2EDocumentsAndVersions(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	ids := contract(h)
	// a document embeds claims; it never restates them
	v1 := writeFile(t, h.dir, "prd.md", "# Stacked PRs\n\n## Why\n\nReviewers skim large PRs, so {{G-1}}.\n\n## Requirements\n\n{{FR-1}}\n\n{{FR-2}}\n\nDone when {{prd/stacked-prs#DOD-1}}; the design is {{design/stacked-prs#D-1}}.\n")
	code, _, errb := h.run("add", "--kind", "Document", "--subject", "prd/stacked-prs", "--family", "prd", "--statement", "Stacked PRs")
	if code != 2 || !strings.Contains(errb, "needs its narrative") {
		t.Fatalf("document without body: %d %s", code, errb)
	}
	code, _, errb = h.run("add", "--kind", "Promise", "--subject", "prd/stacked-prs", "--family", "FR-7", "--statement", "s", "--body-file", v1)
	if code != 2 || !strings.Contains(errb, "only a Document takes a body") {
		t.Fatalf("body on a promise: %d %s", code, errb)
	}
	drafts := t.TempDir() // drafts live outside the repository: the memory holds the stored bodies
	bad := writeFile(t, drafts, "bad.md", "# Stacked PRs\n\n{{FR-99}}\n")
	code, _, errb = h.run("add", "--kind", "Document", "--subject", "prd/stacked-prs", "--family", "prd", "--statement", "Stacked PRs", "--body-file", bad)
	if code != 1 || !strings.Contains(errb, "embed-unresolved: {{prd/stacked-prs#FR-99}}") {
		t.Fatalf("unresolved embed: %d %s", code, errb)
	}
	out := h.ok("add", "--kind", "Document", "--subject", "prd/stacked-prs", "--family", "prd", "--statement", "Stacked PRs", "--body-file", "prd.md",
		"--intent", "give the PRD its narrative")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	docID := lines[0]
	if len(lines) != 3 || !strings.HasPrefix(docID, "Document-") || !strings.HasSuffix(lines[2], docID+".md") {
		t.Fatalf("add output:\n%s", out)
	}
	rendered := h.ok("render", "prd/stacked-prs")
	for _, want := range []string{"> **prd/stacked-prs · prd** · version 1 · since 2026-09-03 by m@example.com · rendered from the working tree",
		"Reviewers skim large PRs, so **G-1** (Reviewers see one reviewable slice per PR.).", "> **FR-1** · When a task produces more than one reviewable slice",
		"> _intent: one reviewable unit per PR · why: stacking keeps each review small · fit criterion: a three-slice task yields three PRs in a chain · since 2026-09-03 · served by design/stacked-prs#D-1_",
		"> **FR-2** · When the top PR of a stack merges", "· **undesigned**_", "Done when **DOD-1** (Every slice PR carries its own tests.); the design is **D-1** (The executor keeps a stack manifest",
		"## Document history", "- v1 · 2026-09-03 · m@example.com · " + docID + " · give the PRD its narrative"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("render lacks %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "## Claims this document does not cover") {
		t.Fatalf("every claim is embedded:\n%s", rendered)
	}
	want(t, h.ok("view", "prd/stacked-prs"), "Narrative: `prd` document \"Stacked PRs\" (version 1) — render it with `memory render prd/stacked-prs --slot prd`")
	want(t, h.ok("search", "reviewers skim large"), docID, "prd/stacked-prs#prd [active] Stacked PRs")
	want(t, h.ok("pack", "--subject", "prd/stacked-prs", "--family", "FR-2"), "## Narrative around the task", "Stacked PRs › Requirements")
	h.commit("document v1")

	// a revision on a branch: the previous version stays as history
	h.git("checkout", "-q", "-b", "feature/prd-v2")
	v2 := writeFile(t, drafts, "prd-v2.md", "# Stacked PRs\n\n## Why\n\nReviewers skim large PRs, so {{G-1}}. Stacks keep every review under one screen.\n\n## Requirements\n\n{{FR-1}}\n\n{{FR-2}}\n\nDone when {{DOD-1}}; the design is {{design/stacked-prs#D-1}}.\n")
	code, _, errb = h.run("revise", "prd/stacked-prs#FR-1", "--body-file", v2)
	if code != 2 || !strings.Contains(errb, "not a Document") {
		t.Fatalf("revise a promise: %d %s", code, errb)
	}
	code, _, errb = h.run("revise", "prd/stacked-prs#prd", "--body-file", "missing.md")
	if code != 2 || !strings.Contains(errb, "--body-file") {
		t.Fatalf("revise with a missing file: %d %s", code, errb)
	}
	out = h.ok("revise", "prd/stacked-prs#prd", "--body-file", v2, "--intent", "say why stacks help")
	v2ID := strings.Split(out, "\n")[0]
	want(t, h.ok("render", "prd/stacked-prs"), "version 2", "Stacks keep every review under one screen.", "- v2 · 2026-09-03 · m@example.com · "+v2ID+" · say why stacks help", "- v1 · ")
	want(t, h.ok("render", "prd/stacked-prs", "--at", "main"), "version 1", "rendered from main")
	if strings.Contains(h.ok("render", "prd/stacked-prs", "--at", "main"), "under one screen") {
		t.Fatal("the render at main is v1")
	}
	want(t, h.ok("why", "prd/stacked-prs#prd"), "## now · active", v2ID, "## v-1 · superseded", docID)
	body := h.ok("packet", "--base", "main", "--body")
	want(t, body, "## Narrative delta — the documents", "### prd/stacked-prs · prd \"Stacked PRs\" — version 2", "- **intent**: say why stacks help",
		"- **embeds**: design/stacked-prs#D-1, prd/stacked-prs#DOD-1, prd/stacked-prs#FR-1, prd/stacked-prs#FR-2, prd/stacked-prs#G-1", "```diff", "-Reviewers skim large PRs, so {{G-1}}.",
		"+Reviewers skim large PRs, so {{G-1}}. Stacks keep every review under one screen.")
	want(t, h.ok("check", "--base", "main"), "+ Document", "check: ok")
	// concurrent narrative edits on two branches are flagged like any family
	h.commit("document v2")
	h.git("checkout", "-q", "main")
	h.git("checkout", "-q", "-b", "feature/prd-other")
	h.ok("revise", "prd/stacked-prs#prd", "--body-file", v2, "--intent", "another editor")
	want(t, h.ok("packet", "--base", "main"), "! concurrent: feature/prd-v2 also changes prd/stacked-prs#prd")
	h.git("checkout", "-q", "--", ".")
	h.git("clean", "-fdq")
	// tampering with a stored body is caught, as is a body without an envelope
	h.git("checkout", "-q", "feature/prd-v2")
	h.git("checkout", "-q", "-b", "feature/tamper-doc")
	mdPath := filepath.Join(h.dir, "memory", "prd", "stacked-prs", v2ID+".md")
	if err := os.WriteFile(mdPath, []byte("# rewritten in place\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(h.dir, "memory", "prd", "stacked-prs"), "Document-01ARZ3NDEKTSV4RRFFQ69G5FAV.md", "orphan")
	code, out, _ = h.run("check", "--base", "main")
	if code != 1 {
		t.Fatalf("tampered body must be blocked:\n%s", out)
	}
	want(t, out, "✗ record-invalid: prd/stacked-prs/"+v2ID+".md: invalid record: the body of "+v2ID+" does not match body_sha256",
		"✗ record-invalid: prd/stacked-prs/Document-01ARZ3NDEKTSV4RRFFQ69G5FAV.md: invalid record: a body with no Document envelope")
	code, rendered, errb = h.run("render", "prd/stacked-prs")
	if code != 1 || rendered != "" || !strings.Contains(errb, "cannot render prd/stacked-prs#prd") || !strings.Contains(errb, v2ID+".md") {
		t.Fatalf("a corrupt v2 must fail closed instead of rendering v1: exit %d\nstdout: %s\nstderr: %s", code, rendered, errb)
	}
	code, rendered, errb = h.run("view", "prd/stacked-prs")
	if code != 1 || rendered != "" || !strings.Contains(errb, "cannot view prd/stacked-prs") || !strings.Contains(errb, v2ID+".md") {
		t.Fatalf("a corrupt v2 must fail closed instead of advertising v1: exit %d\nstdout: %s\nstderr: %s", code, rendered, errb)
	}
	h.git("checkout", "-q", "--", ".")
	h.git("clean", "-fdq")
	// JSON authoring with an inline body, and a design document with its own slot
	code, out, _ = h.runStdin(`{"kind":"Document","subject":"design/stacked-prs","family":"design","statement":"Stacked PRs design","body":"# Design\n\n{{D-1}} serves {{prd/stacked-prs#FR-1}}.\n"}`, "add", "--json")
	if code != 0 {
		t.Fatalf("json document: %s", out)
	}
	want(t, h.ok("render", "design/stacked-prs"), "> **design/stacked-prs · design** · version 1", "**D-1** (The executor keeps a stack manifest per task; each slice branch is based on the previous slice's head.) serves **FR-1** (When a task produces")
	if code, _, errb := h.run("render", "prd/stacked-prs", "--slot", "design"); code != 1 || !strings.Contains(errb, "no active design document") {
		t.Fatalf("render of a missing slot: %d %s", code, errb)
	}
	for _, args := range [][]string{{"render"}, {"render", "bad subject"}, {"revise"}, {"revise", "prd/stacked-prs", "--body-file", "x"}, {"revise", "prd/stacked-prs#nope", "--body-file", "prd.md"}} {
		if code, _, _ := h.run(args...); code != 2 {
			t.Fatalf("memory %v: exit %d", args, code)
		}
	}
	if code, _, errb := h.run("render", "prd/stacked-prs", "--at", "nope"); code != 1 || !strings.Contains(errb, "does not exist") {
		t.Fatalf("render at a bad ref: %d %s", code, errb)
	}
	_ = ids
}

func TestE2ELiteralDocumentImportAndRevision(t *testing.T) {
	t.Parallel()
	h := initRepo(t)
	h.ok("init")

	native := writeFile(t, t.TempDir(), "native.md", "# Native document\n")
	nativeID := strings.SplitN(h.ok("add", "--kind", "Document", "--subject", "reference/native", "--family", "source", "--statement", "Native document", "--body-file", native), "\n", 2)[0]
	payload, err := json.Marshal(addInput{Kind: record.Document, Subject: "reference/native", Family: "source", Statement: "Native made literal",
		Supersedes: []string{nativeID}, Body: "# Native document\n\n{{missing_claim}}\n", LiteralBody: true})
	if err != nil {
		t.Fatal(err)
	}
	code, _, errb := h.runStdin(string(payload), "add", "--json", "--actor", "import-test")
	if code != exitRefused || !strings.Contains(errb, "document-mode-changed") {
		t.Fatalf("native-to-literal mode switch: exit %d, stderr %s", code, errb)
	}
	h.commit("memory policy")

	drafts := t.TempDir()
	v1Body := "# Imported installer PRD\n\n## Contract\n\nThe heliotrope migration guardrail keeps {{installer_channel}} byte-for-byte, including `{{inline_example}}`.\n"
	v1 := writeFile(t, drafts, "installer-v1.md", v1Body)
	writeFixtureFile(t, h.dir, "docs/prds/installer/000-base.md", v1Body, 0o644)
	h.commit("pin literal import source")
	importCommit := h.git("rev-parse", "HEAD")
	h.git("commit", "--allow-empty", "-qm", "retain historical import")
	out := h.ok("add", "--kind", "Document", "--subject", "reference/installer-prd", "--family", "source", "--statement", "Imported installer PRD",
		"--body-file", v1, "--literal-body", "--source", "docs/prds/installer/000-base.md", "--pr", "42", "--commit", importCommit, "--note", "pinned import")
	v1ID := strings.SplitN(out, "\n", 2)[0]

	assertLiteralBody := func(rendered, body string) {
		t.Helper()
		want(t, rendered, "literal source body; brace-shaped text is not interpreted")
		bodyStart := strings.Index(rendered, "\n\n")
		historyStart := strings.Index(rendered, "\n## Document history\n")
		if bodyStart < 0 || historyStart < bodyStart {
			t.Fatalf("render does not have the expected document envelope:\n%s", rendered)
		}
		if got := rendered[bodyStart+2 : historyStart]; got != body {
			t.Fatalf("literal body changed during render:\n--- got ---\n%s\n--- want ---\n%s", got, body)
		}
	}

	assertLiteralBody(h.ok("render", "reference/installer-prd", "--slot", "source"), v1Body)
	want(t, h.ok("search", "heliotrope migration guardrail"), v1ID, "reference/installer-prd#source [active] Imported installer PRD")
	pack := h.ok("pack", "--subject", "reference/installer-prd", "--budget", "4000")
	want(t, pack, "## Narrative around the task", "Imported installer PRD › Contract", "heliotrope migration guardrail keeps {{installer_channel}} byte-for-byte")
	if strings.Contains(pack, "## Narrative around the task\n\n- no document section embeds these claims") {
		t.Fatalf("literal narrative was reported as empty:\n%s", pack)
	}
	h.commit("literal document v1")

	h.git("checkout", "-q", "-b", "feature/literal-document-v2")
	v2Body := "# Imported installer PRD\n\n## Contract\n\nThe heliotrope migration guardrail now keeps {{installer_channel_v2}} byte-for-byte, including `{{inline_example_v2}}`.\n"
	v2 := writeFile(t, drafts, "installer-v2.md", v2Body)
	out = h.ok("revise", "reference/installer-prd#source", "--body-file", v2, "--intent", "refresh the imported source")
	v2ID := strings.SplitN(out, "\n", 2)[0]
	assertLiteralBody(h.ok("render", "reference/installer-prd", "--slot", "source"), v2Body)

	why := h.ok("why", "reference/installer-prd#source")
	want(t, why, v2ID, v1ID)
	if got := strings.Count(why, "- from: docs/prds/installer/000-base.md (PR #42)"); got != 1 {
		t.Fatalf("the original import should carry its PR exactly once (got %d):\n%s", got, why)
	}
	want(t, why, "(commit "+importCommit+") — pinned import")
	if got := strings.Count(why, "- from: docs/prds/installer/000-base.md"); got != 2 {
		t.Fatalf("the source path should identify both imported versions (got %d):\n%s", got, why)
	}
	want(t, h.ok("check", "--base", "main"), "+ Document", "check: ok")
}
