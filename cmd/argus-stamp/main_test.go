package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/argus/stamp"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// dcfg is the default configuration the stamp/attribution integration runs
// under (today's problems root).
var dcfg = config.Defaults()

// runMain drives run() with args, returning exit code, stdout, stderr.
func runMain(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// writeMsg writes a commit-message file and returns its path.
func writeMsg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// stampWorktree inits a hermetic git repo with one staged file and points the
// stamper at it via ARGUS_STAMP_WORKTREE, so a drive commit's content binding
// (GitBinding → git write-tree) has a real tree to bind to. Returns the repo.
func stampWorktree(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", "-b", "main")
	writeRepo(t, repo, "staged.txt", "content\n")
	run("add", "staged.txt")
	t.Setenv("ARGUS_STAMP_WORKTREE", repo)
	return repo
}

func TestRun_StampsFileKeyedOnSlugWithBinding(t *testing.T) {
	// The hook entry rewrites the commit-message file in place, keyed on the
	// active drive's slug, carrying BOTH the drive trailer and its content
	// binding computed from the worktree's staged tree (§3.1, §3.2).
	stampWorktree(t)
	p := writeMsg(t, "feat: add widget\n")
	code, _, errb := runMain("--slug=widget", p)
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, errb)
	}
	got := readFile(t, p)
	if !strings.Contains(got, stamp.TrailerKey+": widget") {
		t.Errorf("file not stamped with the drive trailer:\n%q", got)
	}
	if !strings.Contains(got, stamp.BindTrailerKey+": ") {
		t.Errorf("file not stamped with the content binding:\n%q", got)
	}
}

func TestRun_NoSlugLeavesFileUntouched(t *testing.T) {
	// A commit outside a drive (no slug) leaves the file byte-for-byte
	// unchanged — no trailer.
	const body = "feat: add widget\n\nbody.\n"
	p := writeMsg(t, body)
	code, _, _ := runMain(p) // no --slug, no $ARGUS_DRIVE_SLUG
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := readFile(t, p); got != body {
		t.Errorf("no-slug run mutated the file:\n got %q\nwant %q", got, body)
	}
}

func TestRun_MalformedSlugAbortsWithoutWriting(t *testing.T) {
	// A malformed slug aborts (nonzero exit) and does not rewrite the file — the
	// producer fails closed on a real misconfiguration, even with a resolvable
	// worktree (the binding computes; the slug is what's rejected).
	stampWorktree(t)
	const body = "feat: x\n"
	p := writeMsg(t, body)
	code, _, errb := runMain("--slug=Bad Slug", p)
	if code == 0 {
		t.Error("malformed slug did not abort the commit")
	}
	if !strings.Contains(errb, "malformed drive slug") {
		t.Errorf("stderr = %q, want it to name the malformed slug", errb)
	}
	if got := readFile(t, p); got != body {
		t.Errorf("aborted run still rewrote the file: %q", got)
	}
}

func TestRun_MissingFileArg(t *testing.T) {
	if code, _, _ := runMain("--slug=widget"); code == 0 {
		t.Error("missing commit-message file arg did not error")
	}
}

func TestRun_RefusesMalformedFlagMissingFileAndUnboundWorktree(t *testing.T) {
	if code, _, _ := runMain("--not-a-flag"); code == 0 {
		t.Error("malformed flag did not error")
	}
	if code, _, errb := runMain(filepath.Join(t.TempDir(), "missing")); code == 0 || !strings.Contains(errb, "reading commit-message file") {
		t.Fatalf("missing file: exit=%d stderr=%q", code, errb)
	}

	message := writeMsg(t, "feat: cannot bind\n")
	t.Setenv("ARGUS_STAMP_WORKTREE", filepath.Join(t.TempDir(), "not-a-repository"))
	if code, _, errb := runMain("--slug=widget", message); code == 0 || !strings.Contains(errb, "content binding") {
		t.Fatalf("unbound worktree: exit=%d stderr=%q", code, errb)
	}
}

func TestGitWorktreeDiscoversRepositoryAndFallsBack(t *testing.T) {
	repo := stampWorktree(t)
	t.Setenv("ARGUS_STAMP_WORKTREE", "")
	t.Chdir(repo)
	got := gitWorktree()
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(repo)
	if gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("gitWorktree() = %q, want repository %q (stat errors %v, %v)", got, repo, gotErr, wantErr)
	}

	t.Chdir(t.TempDir())
	if got := gitWorktree(); got != "." {
		t.Fatalf("gitWorktree() outside repository = %q, want .", got)
	}
}

func TestRun_PathsAndKey(t *testing.T) {
	code, out, _ := runMain("paths")
	if code != 0 || !strings.Contains(out, "claude") {
		t.Errorf("`paths` did not list the claude wiring: exit=%d out=%q", code, out)
	}
	code, out, _ = runMain("key")
	if code != 0 || strings.TrimSpace(out) != stamp.TrailerKey {
		t.Errorf("`key` = %q, want %q", out, stamp.TrailerKey)
	}
}

// TestEndToEnd_ProducerHookStampsAndGateRefuses is the A-1 proof: a commit
// authored THROUGH the producer hook (git's core.hooksPath prepare-commit-msg
// firing the built stamper) carries the Argus-Drive trailer by construction —
// no hand-addition — and the real attribution gate then derives it as Argus and
// refuses it on a protected path. A control commit authored outside a drive
// (no slug) carries no trailer and derives as human (not refused).
func TestEndToEnd_ProducerHookStampsAndGateRefuses(t *testing.T) {
	bin := buildStamp(t)
	repo := t.TempDir()
	git := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(gitEnv(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	git(nil, "init", "-q", "-b", "main")
	// Install the producer hook on git's own core.hooksPath surface — a
	// delegate-only shim execing the built stamper with the commit-message file,
	// exactly as the repo's prepare-commit-msg shim does.
	hooks := filepath.Join(repo, ".githooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\nexec \"" + bin + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	git(nil, "config", "core.hooksPath", ".githooks")

	// Base commit (no drive) — a plain file; this is the merge base the gate
	// evaluates from.
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(nil, "add", "README.md")
	git(nil, "commit", "-q", "-m", "chore: base")
	base := revParse(t, repo, "HEAD")

	// A DRIVE commit through the hook, touching a protected path (the governance
	// vision). ARGUS_DRIVE_SLUG is the drive context the harness sets.
	writeRepo(t, repo, "docs/product-vision.md", "governance edit\n")
	git(nil, "add", "docs/product-vision.md")
	git([]string{"ARGUS_DRIVE_SLUG=widget"}, "commit", "-q", "-m", "feat: touch vision")

	// (1) BOTH trailers are present by construction — read straight from the
	// commit: the authorship stamp AND its in-tree content binding (§3.1, §3.2).
	body := commitBody(t, repo, "HEAD")
	if !strings.Contains(body, stamp.TrailerKey+": widget") {
		t.Fatalf("drive commit was not stamped with the drive trailer by the hook:\n%q", body)
	}
	if !strings.Contains(body, stamp.BindTrailerKey+": ") {
		t.Fatalf("drive commit was not stamped with the content binding by the hook:\n%q", body)
	}

	// (2) The real gate derives it as Argus (its produced binding re-derives) and
	// refuses it on the protected path — the DOD-10 live-produced-signal proof.
	defects := attribution.Evaluate(repo, base, dcfg)
	if !refuses(defects, "docs/product-vision.md") {
		t.Fatalf("the gate did not refuse the produced Argus protected-path diff; defects=%v", defects)
	}

	// (3) Control: a POSITIVELY-HUMAN commit — authored outside a drive (no
	// slug, so no Argus stamp) and carrying the positive attestation the
	// governance class already requires — derives as positively-human and is NOT
	// refused (§3.3, NR-4). Absence of the Argus signal alone would now fail
	// closed; the attestation is what positively establishes human authorship.
	git(nil, "checkout", "-q", base)
	writeRepo(t, repo, "docs/product-vision.md", "human edit\n")
	git(nil, "add", "docs/product-vision.md")
	git(nil, "commit", "-q", "-m", "docs: human vision edit\n\nGoverning-Change: operator") // no ARGUS_DRIVE_SLUG
	humanBody := commitBody(t, repo, "HEAD")
	if strings.Contains(humanBody, stamp.TrailerKey) {
		t.Fatalf("non-drive commit carried an Argus trailer:\n%q", humanBody)
	}
	if refuses(attribution.Evaluate(repo, base, dcfg), "docs/product-vision.md") {
		t.Fatal("the gate refused a positively-human protected-path diff (the anti-segment breach)")
	}

	// (4) Absence control: the SAME human governance edit WITHOUT the positive
	// attestation now fails closed — absence of any positive signal on a
	// protected path is underivable, not a masquerading human (§3.3, DINV-2).
	git(nil, "checkout", "-q", base)
	writeRepo(t, repo, "docs/product-vision.md", "unattested edit\n")
	git(nil, "add", "docs/product-vision.md")
	git(nil, "commit", "-q", "-m", "docs: unattested vision edit") // no slug, no attestation
	if !refuses(attribution.Evaluate(repo, base, dcfg), "docs/product-vision.md") {
		t.Fatal("the gate did not fail closed on an unattested (underivable) protected-path diff")
	}
}

func TestEndToEnd_ResetAndRecommitRepairsMissingDriveStamp(t *testing.T) {
	bin := buildStamp(t)
	repo := t.TempDir()
	git := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(gitEnv(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git(nil, "init", "-q", "-b", "main")
	hooks := filepath.Join(repo, ".githooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte("#!/bin/sh\nexec \""+bin+"\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(nil, "config", "core.hooksPath", ".githooks")
	writeRepo(t, repo, "README.md", "seed\n")
	git(nil, "add", "README.md")
	git(nil, "commit", "-q", "-m", "chore: base")
	base := revParse(t, repo, "HEAD")

	writeRepo(t, repo, "docs/problems/widget/problem.md", "problem\n")
	git(nil, "add", "docs/problems/widget/problem.md")
	git(nil, "commit", "-q", "-m", "docs: record widget") // missing drive context
	old := revParse(t, repo, "HEAD")
	if strings.Contains(commitBody(t, repo, old), stamp.TrailerKey) {
		t.Fatal("control commit unexpectedly carried the drive trailer")
	}

	// The guide's repair puts HEAD back at the original parent before the
	// hook computes the binding. -C reuses the message from ORIG_HEAD.
	git(nil, "reset", "--soft", "HEAD^")
	if got := revParse(t, repo, "ORIG_HEAD"); got != old {
		t.Fatalf("ORIG_HEAD = %s, want replaced commit %s", got, old)
	}
	git([]string{"ARGUS_DRIVE_SLUG=widget"}, "commit", "-q", "-C", "ORIG_HEAD")
	if got := revParse(t, repo, "HEAD^"); got != base {
		t.Fatalf("repaired commit parent = %s, want %s", got, base)
	}
	wantBind := attribution.ComputeBinding(revParse(t, repo, "HEAD^{tree}"), []string{base})
	body := commitBody(t, repo, "HEAD")
	if !strings.Contains(body, "Argus-Drive: widget") || !strings.Contains(body, "Argus-Bind: "+wantBind) {
		t.Fatalf("repaired commit lacks derivable drive trailers: %q", body)
	}
	if got := attribution.Evaluate(repo, base, dcfg); len(got) != 0 {
		t.Fatalf("repaired in-scope drive commit was refused: %v", got)
	}
}

func TestEndToEnd_RebaseRefreshesOnlyStampedCommits(t *testing.T) {
	bin := buildStamp(t)
	repo := t.TempDir()
	git := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(gitEnv(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git(nil, "init", "-q", "-b", "main")
	hooks := filepath.Join(repo, ".githooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte("#!/bin/sh\nexec \""+bin+"\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git(nil, "config", "core.hooksPath", ".githooks")
	writeRepo(t, repo, "README.md", "seed\n")
	git(nil, "add", "README.md")
	git(nil, "commit", "-q", "-m", "chore: base")
	git(nil, "switch", "-q", "-c", "draft/widget")
	writeRepo(t, repo, "drafts/widget/prd.md", "# Widget\n")
	git(nil, "add", "drafts/widget/prd.md")
	git([]string{"ARGUS_DRIVE_SLUG=widget"}, "commit", "-q", "-m", "docs(draft): author widget")
	oldBind := attribution.ComputeBinding(revParse(t, repo, "HEAD^{tree}"), []string{revParse(t, repo, "HEAD^")})
	writeRepo(t, repo, "notes.txt", "human note\n")
	git(nil, "add", "notes.txt")
	git(nil, "commit", "-q", "-m", "docs: add human note")
	git(nil, "switch", "-q", "main")
	writeRepo(t, repo, "base.txt", "mainline advanced\n")
	git(nil, "add", "base.txt")
	git(nil, "commit", "-q", "-m", "chore: advance mainline")
	base := revParse(t, repo, "HEAD")
	git(nil, "switch", "-q", "draft/widget")
	git([]string{"ARGUS_STAMP_REBASE=1", "ARGUS_DRIVE_SLUG="}, "rebase", "main")
	if got := commitBody(t, repo, "HEAD"); strings.Contains(got, stamp.TrailerKey) {
		t.Fatalf("unstamped human commit acquired a drive stamp: %q", got)
	}
	stamped := revParse(t, repo, "HEAD^")
	wantBind := attribution.ComputeBinding(revParse(t, repo, stamped+"^{tree}"), []string{base})
	if wantBind == oldBind {
		t.Fatal("rebase did not change the content binding")
	}
	body := commitBody(t, repo, stamped)
	if !strings.Contains(body, "Argus-Drive: widget") || !strings.Contains(body, "Argus-Bind: "+wantBind) {
		t.Fatalf("rebased drive commit has stale or missing binding: %q", body)
	}
	if got := attribution.Evaluate(repo, base, dcfg); len(got) != 0 {
		t.Fatalf("rebased in-scope draft was refused: %v", got)
	}
}

func TestRun_RebaseRejectsPartialStamp(t *testing.T) {
	t.Setenv("ARGUS_STAMP_REBASE", "1")
	t.Setenv("ARGUS_DRIVE_SLUG", "")
	p := writeMsg(t, "docs: draft\n\nArgus-Drive: widget\n")
	if code, _, errb := runMain(p); code == 0 || !strings.Contains(errb, "incomplete or malformed") {
		t.Fatalf("partial stamp was accepted: exit=%d stderr=%q", code, errb)
	}
}

func TestRun_RebaseKeepsUnstampedMessageAndRefreshesExistingSlug(t *testing.T) {
	repo := stampWorktree(t)
	t.Setenv("ARGUS_STAMP_REBASE", "1")
	t.Setenv("ARGUS_DRIVE_SLUG", "other") // ambient context cannot relabel replayed work

	plain := writeMsg(t, "docs: human draft\n")
	if code, _, errb := runMain(plain); code != 0 || readFile(t, plain) != "docs: human draft\n" {
		t.Fatalf("unstamped replay changed: exit=%d stderr=%q body=%q", code, errb, readFile(t, plain))
	}

	cmd := exec.Command("git", "-C", repo, "write-tree")
	treeBytes, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	oldBind := attribution.ComputeBinding(strings.TrimSpace(string(treeBytes)), []string{strings.Repeat("a", 40)})
	currentBind, err := stamp.GitBinding(repo)
	if err != nil {
		t.Fatal(err)
	}
	if oldBind == currentBind {
		t.Fatal("source and rebased bindings unexpectedly match")
	}
	replayed := writeMsg(t, "docs: drive draft\n\nArgus-Drive: widget\nArgus-Bind: "+oldBind+"\n")
	if code, _, errb := runMain(replayed); code != 0 {
		t.Fatalf("stamped replay refused: exit=%d stderr=%q", code, errb)
	}
	body := readFile(t, replayed)
	if !strings.Contains(body, "Argus-Drive: widget") || !strings.Contains(body, "Argus-Bind: "+currentBind) || strings.Contains(body, "Argus-Drive: other") {
		t.Fatalf("replayed stamp has wrong identity or binding: %q", body)
	}
}

func refuses(defects []schema.Defect, path string) bool {
	for _, d := range defects {
		if d.File == path {
			return true
		}
	}
	return false
}

func buildStamp(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable — skipping the end-to-end hook build")
	}
	bin := filepath.Join(t.TempDir(), "argus-stamp")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build argus-stamp: %v\n%s", err, out)
	}
	return bin
}

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.x",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.x",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_ALLOW_PROTOCOL=false",
	)
}

func writeRepo(t *testing.T, repo, rel, body string) {
	t.Helper()
	p := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func revParse(t *testing.T, repo, ref string) string {
	t.Helper()
	return gitOut(t, repo, "rev-parse", ref)
}

func commitBody(t *testing.T, repo, ref string) string {
	t.Helper()
	return gitOut(t, repo, "log", "-1", "--format=%B", ref)
}

func gitOut(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
