package installer_test

// Hermetic tests for the T7 init engine's front half (installer-v1 §3.5
// steps 1–3 + 8; plan installer/001 T7): a complete fixture release served
// over file://, per-entry-state target fixtures, and the admission /
// knob-resolution / identity / two-commit-epoch / preview / config-round-trip
// legs the charter names (prd:installer FR-3/FR-4/FR-7/FR-19/UB-11/UB-10;
// A-12). Every git invocation runs under an isolated HOME + neutralized
// global/system git config so the developer's real identity never leaks in.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// TestMain installs default T8 seams for the whole test binary so the front-half
// T7 tests (which now run the full pipeline including the tail) complete
// hermetically: github-mode wiring runs a transcript fake (no gh, no network).
// A Git-only PATH proves initialization requires no package runtime.
// Tests for CLI-surface auto-detection override PATH locally.
func TestMain(m *testing.M) {
	var mu sync.Mutex
	publications := make(map[string]string)
	installer.SetPublicationGitForTest(func(ctx context.Context, dir string, args ...string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		mu.Lock()
		defer mu.Unlock()
		key := dir + "\n" + args[len(args)-2]
		if args[0] == "ls-remote" {
			if head := publications[key]; head != "" {
				return head + "\t" + args[len(args)-1] + "\n", nil
			}
			return "", nil
		}
		if slices.Contains(args, "push") {
			head, _, ok := strings.Cut(args[len(args)-1], ":")
			if !ok {
				return "", fmt.Errorf("invalid publication refspec: %v", args)
			}
			publications[key] = head
			return "", nil
		}
		return "", fmt.Errorf("unexpected publication command: %v", args)
	})
	installer.SetWireExecuteForTest(func(_ context.Context, ops []wire.Op) (wire.Audit, error) {
		audit := wire.Audit{Event: wire.EventWiringApplied, RunID: "test-run"}
		for _, op := range ops {
			out := wire.OpResult{Name: op.Name, Transport: op.Transport, Outcome: wire.OutcomeApplied, Reason: "default fake"}
			if op.Transport == wire.TransportConverge {
				out.Outcome, out.Reason = wire.OutcomeSkipped, "converge-carried"
			}
			audit.Operations = append(audit.Operations, out)
		}
		return audit, nil
	})
	restorePATH := setHermeticToolchainPATH()
	defer restorePATH()
	os.Exit(m.Run())
}

// setHermeticToolchainPATH builds one PATH for the whole test binary carrying
// a symlink to the real git (every test needs a genuine git; the package
// shells out to it directly), with no package runtime on PATH.
// Returns a restore function (os.Exit below never actually calls it, but it
// keeps the seam symmetric and testable in isolation).
func setHermeticToolchainPATH() (restore func()) {
	dir, err := os.MkdirTemp("", "installer-hermetic-path-*")
	if err != nil {
		panic("installer: build hermetic PATH: " + err.Error())
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		panic("installer tests need git on PATH: " + err.Error())
	}
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		panic(err)
	}

	old, hadOld := os.LookupEnv("PATH")
	if err := os.Setenv("PATH", dir); err != nil {
		panic(err)
	}
	return func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
		if hadOld {
			if err := os.Setenv("PATH", old); err != nil {
				panic(err)
			}
		} else {
			if err := os.Unsetenv("PATH"); err != nil {
				panic(err)
			}
		}
	}
}

// writeFakeVersionTool writes a tiny scripted stand-in at dir/name whose
// `--version` (or any-argument) invocation just echoes version — enough for
// the toolchain check's presence+parse probe without a real install (the same
// shape internal/preflight's own main_test.go uses).
func writeFakeVersionTool(dir, name, version string) {
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho '" + version + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		panic(err)
	}
}

// ---- git environment isolation --------------------------------------------

// isolateGit neutralizes the ambient git identity for a test: no global/system
// config, and the commit dates pinned so commit shas are reproducible across
// legs (the A-12 byte-identical comparison relies on this). It leaves the
// per-repo identity to each test — an "identity unset everywhere" test sets
// nothing; an "ambient identity" test points GIT_CONFIG_GLOBAL at a real file.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-02T03:04:05Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T03:04:05Z")
	// A HOME with no .gitconfig, belt-and-braces against any tool reading it.
	t.Setenv("HOME", t.TempDir())
}

// ambientIdentityConfig writes a global git config carrying an identity and
// points GIT_CONFIG_GLOBAL at it, so `git config --get user.name/email`
// resolves it exactly as it would on a configured machine.
func ambientIdentityConfig(t *testing.T, name, email string) {
	t.Helper()
	gc := filepath.Join(t.TempDir(), "gitconfig")
	body := "[user]\n\tname = " + name + "\n\temail = " + email + "\n"
	if err := os.WriteFile(gc, []byte(body), 0o644); err != nil {
		t.Fatalf("write ambient gitconfig: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", gc)
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String()
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// ---- fixture release --------------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, path string, b []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// pf is one payload file of the fixture release.
type pf struct {
	path  string
	b     []byte
	class relmanifest.Class
	exec  bool
}

// payloadFiles mirrors the real class-policy shapes minus the operand config
// (init WRITES the config itself before converge, so it is not a packed
// engine-verbatim/repo-owned payload file here): engine-verbatim templates and
// scripts (one executable) and a workflow, plus the generated design indexes,
// AND the T8 instantiable set + installed hook variants the tail seeds from —
// the REAL source-repo files (the fixture "carries the real skeletons", plan
// T8 test rubric), so the full pipeline (T7 front + T8 tail) runs against the
// actual instantiables. The config's repo-owned validation is exercised
// implicitly: init's own config must parse, or converge would refuse it.
func payloadFiles(t *testing.T) []pf {
	t.Helper()
	files := []pf{
		{path: ".pyxis/scripts/hook.sh", b: []byte("#!/bin/sh\nexit 0\n"), class: relmanifest.EngineVerbatim, exec: true},
		{path: ".pyxis/skills/help/SKILL.md", b: []byte("---\nname: help\n---\n"), class: relmanifest.EngineVerbatim},
		{path: ".pyxis/skills/implement-now/SKILL.md", b: []byte("---\nname: implement-now\n---\n"), class: relmanifest.EngineVerbatim},
		{path: ".github/workflows/docs-gate.yml", b: []byte("name: docs-gate\n"), class: relmanifest.EngineVerbatim},
		{path: "docs/tech-design/index.md", b: []byte("stale packed index\n"), class: relmanifest.Generated},
		{path: "docs/tech-design/index.json", b: []byte("{}\n"), class: relmanifest.Generated},
	}
	// The real instantiables + installed hook variants, engine-verbatim (the
	// tail reads them from the converged operand tree). Loaded from the source
	// repo so the seed step exercises the genuine skeletons/manual/hooks.
	for _, rel := range []string{
		".pyxis/templates/product-vision-skeleton.md",
		".pyxis/templates/architecture-principles-skeleton.md",
		".pyxis/templates/operand-agents-manual.md",
		".pyxis/templates/harness-implement-now-skill.md",
		".pyxis/templates/authority-contract-template.md",
		".pyxis/templates/welcome-item.md",
		".pyxis/dist/operand-hooks/pre-commit",
		".pyxis/dist/operand-hooks/pre-push",
		".pyxis/dist/operand-hooks/prepare-commit-msg",
	} {
		files = append(files, pf{path: rel, b: readSourceFile(t, rel), class: relmanifest.EngineVerbatim, exec: strings.HasPrefix(rel, ".pyxis/dist/operand-hooks/")})
	}
	return files
}

// srcRepoRoot resolves the source repo root from this package directory — the
// real instantiables the fixture payload carries are the source repo's own.
func srcRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve source repo root: %v", err)
	}
	return root
}

func readSourceFile(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(srcRepoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read source payload file %s: %v", rel, err)
	}
	return b
}

// fakeWire is a transcript-fake wire dispatch: it records the declared table it
// was handed and returns a fully-applied audit, so github-mode wiring runs
// hermetically (no gh, no network). Installed via SetWireExecuteForTest.
type fakeWire struct {
	ops []wire.Op
}

func (f *fakeWire) execute(_ context.Context, ops []wire.Op) (wire.Audit, error) {
	f.ops = append(f.ops, ops...)
	audit := wire.Audit{Event: wire.EventWiringApplied, RunID: "test-run"}
	for _, op := range ops {
		out := wire.OpResult{Name: op.Name, Transport: op.Transport, Outcome: wire.OutcomeApplied, Reason: "fake: applied"}
		if op.Transport == wire.TransportConverge {
			out.Outcome, out.Reason = wire.OutcomeSkipped, "fake: converge-carried"
		}
		// The welcome-item op collects the created issue's URL (#948); the
		// fake mirrors it so the report's guide-ref plumbing is exercised.
		if op.Name == wire.OpWelcomeItem {
			out.Refs = []string{"https://github.com/" + op.Slug + "/issues/1"}
		}
		audit.Operations = append(audit.Operations, out)
	}
	return audit, nil
}

// installFakeWire wires the github-mode dispatch to a transcript fake and an
// origin remote so the wire Slug guard is satisfied, returning the fake for
// assertions. The remote is added to the target repo after git init, so the
// caller invokes this only for real (non-preview) runs whose repo exists.
func installFakeWire(t *testing.T) *fakeWire {
	t.Helper()
	fw := &fakeWire{}
	restore := installer.SetWireExecuteForTest(fw.execute)
	t.Cleanup(restore)
	return fw
}

func tarGz(t *testing.T, files []pf) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		mode := int64(0o644)
		if f.exec {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: f.path, Mode: mode, Size: int64(len(f.b))}); err != nil {
			t.Fatalf("tar header %s: %v", f.path, err)
		}
		if _, err := tw.Write(f.b); err != nil {
			t.Fatalf("tar write %s: %v", f.path, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release is a complete fixture release directory a file:// source serves.
type release struct {
	src  string
	ver  string
	pin  string
	plat string
	bin  []byte
}

func newRelease(t *testing.T) release {
	t.Helper()
	const ver = "v0.4.0"
	src := t.TempDir()
	plat := resolve.Platform()
	bin := []byte("#!/bin/sh\necho docgate fixture\n")
	files := payloadFiles(t)
	archive := tarGz(t, files)

	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: ver,
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {plat: hexOf(bin)}},
		Payload:       relmanifest.Payload{ArchiveSHA256: hexOf(archive)},
	}
	for _, f := range files {
		entry := relmanifest.PayloadFile{Path: f.path, Class: f.class}
		if f.class == relmanifest.EngineVerbatim {
			sum := hexOf(f.b)
			entry.SHA256 = &sum
		}
		man.Payload.Files = append(man.Payload.Files, entry)
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(src, "release-manifest-"+ver+".json"), mb, 0o644)
	writeFile(t, filepath.Join(src, "payload-"+ver+".tar.gz"), archive, 0o644)
	writeFile(t, filepath.Join(src, "docgate-"+ver+"-"+plat), bin, 0o644)
	return release{src: src, ver: ver, pin: hexOf(mb), plat: plat, bin: bin}
}

// hermeticGH is the shared happy-path wire.Runner the front-half fixtures
// inject for the preflight GitHub-mode/scope checks (design init-hardening-v1
// §3.1 checks 3/4; plan init-hardening/001 T3): gh present, authenticated,
// the fixture coordinate exists, and a classic token carrying "repo" — so a
// github-mode-applicable knob set clears the new pre-write checks without
// touching a real gh. It is unrelated to installFakeWire/fakeWire above,
// which swap the WIRING STAGE's own dispatch (SetWireExecuteForTest) — that
// seam ignores the wire.Runner it is handed, so this fake is the checks' only
// transport. The check's own refusal/advisory behavior is internal/preflight's
// test matrix (github_test.go); these fixtures only need the all-clear path.
type hermeticGH struct{}

func (hermeticGH) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	switch {
	case len(args) >= 1 && args[0] == "--version":
		return "gh version 2.40.0 (test fake)\n", "", 0, nil
	case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
		return "", "Logged in to github.com as test-fake\n", 0, nil
	case len(args) >= 2 && args[0] == "repo" && args[1] == "view":
		return `{"name":"widget"}`, "", 0, nil
	case len(args) >= 2 && args[0] == "api" && args[1] == "-i":
		return "HTTP/2.0 200 OK\nX-Oauth-Scopes: repo\n\n{}", "", 0, nil
	case isRepoListQuery(args):
		// A credential that can see NO empty repository: the first question
		// still renders, offering only "create" and "keep it local", which
		// is the shape the coordinate-exchange fixtures drive (#1043).
		return `{"data":{"viewer":{"repositories":{"nodes":[]}}}}`, "", 0, nil
	case isRepoStateQuery(args):
		// Unknown state — the create branch's normal answer for a name that
		// does not resolve under this credential.
		return `{"data":{"repository":null}}`, "", 0, nil
	case isActiveAccountQuery(args):
		// Whoever asked. A repository miss is ambiguous, and naming the
		// account is what makes it diagnosable on a multi-login machine.
		return "octo-bot\n", "", 0, nil
	}
	return "", "", 0, nil
}

// isRepoListQuery / isRepoStateQuery recognize the two read-only GraphQL
// calls the repository question makes, so fakes can answer them by intent
// rather than by matching whole query text.
func isRepoListQuery(args []string) bool {
	return graphqlQueryContains(args, "repositories(")
}

func isRepoStateQuery(args []string) bool {
	return graphqlQueryContains(args, "repository(owner:")
}

// isActiveAccountQuery recognizes the login probe (`gh api user --jq .login`).
func isActiveAccountQuery(args []string) bool {
	return len(args) >= 2 && args[0] == "api" && args[1] == "user"
}

func graphqlQueryContains(args []string, needle string) bool {
	if len(args) < 2 || args[0] != "api" || args[1] != "graphql" {
		return false
	}
	for _, a := range args {
		if strings.Contains(a, needle) {
			return true
		}
	}
	return false
}

// fixtureCoordinate is the promptless GitHub coordinate (FR-7) the front-half
// fixtures supply: none of these targets carries a configured git remote, so
// the preflight GitHub-mode check would otherwise refuse pre-write with
// neither a remote nor a coordinate to resolve.
const fixtureCoordinate = "octo/widget"

// baseOptions is a promptless (answers-provided) install of this release into
// target, with a working ambient-or-answers identity assumed by the caller.
func (r release) baseOptions(target string) installer.Options {
	tracker := "github"
	ci := "both"
	repo, prot := "use", "required-checks"
	argus := false
	return installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource:  "file://" + r.src,
		AnswersProvided: true,
		Answers: installer.Answers{
			Tracker: tracker, CIMode: ci, WiringRepo: repo, WiringProtection: prot, Argus: &argus,
			CLISurfaces: []string{}, // explicit empty — no CLI wired (deterministic, PATH-independent)
			Coordinate:  fixtureCoordinate,
		},
		WireRunner: hermeticGH{},
		Logger:     quiet(),
	}
}

// carinaOptions is a promptless carina-tracker install: no github wiring runs,
// so the pipeline is fully local (the offline combo's tracker). ci_mode local,
// wiring.protection none, argus off. Identity is the caller's ambient/answers.
func (r release) carinaOptions(target string) installer.Options {
	argus := false
	return installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource:  "file://" + r.src,
		AnswersProvided: true,
		Answers: installer.Answers{
			RemoteURL: filepath.Join(r.src, "local.git"), // transcript-faked publication
			Tracker:   "carina", CIMode: "local",
			WiringRepo: "use", WiringProtection: "none", Argus: &argus,
			CLISurfaces: []string{},
		},
		Logger: quiet(),
	}
}

// opNames renders the fake's recorded declared table for assertions.
func (f *fakeWire) opNames() []string {
	out := make([]string, 0, len(f.ops))
	for _, op := range f.ops {
		out = append(out, op.Name)
	}
	return out
}

// gitCmd builds a git command in dir with the isolated env (identity dates
// pinned), for tests that need to inspect the invocation's output/exit rather
// than fataling on failure (git(...) fatals).
func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = os.Environ()
	return cmd
}

// pathWithGitOnly returns an env whose PATH holds ONLY a temp dir carrying a
// git symlink — so a subprocess resolves git but NOT pyxis (the fail-closed
// operand-hook probe then misses). The returned slice is prepended to a
// command's env so this PATH wins.
func pathWithGitOnly(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	return []string{"PATH=" + dir}
}

// gitRevListCount returns the number of commits on HEAD, or 0 when HEAD is
// unborn (no commit exists — e.g. a refusal before the tree commit).
func gitRevListCount(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return 0 // unborn HEAD or no repo
	}
	n := 0
	for f := range strings.FieldsSeq(string(out)) {
		if v, cerr := strconv.Atoi(f); cerr == nil {
			n = v
		}
	}
	return n
}

// firstLines returns the first n lines of s (for bounded error output).
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// boolToken renders a bool the way the operand manual's argus_enabled
// substitution does (strconv.FormatBool → "true"/"false").
func boolToken(b bool) string { return strconv.FormatBool(b) }

// ---- tree snapshot ----------------------------------------------------------

// snap captures every path under root as rel → sha256+mode (dirs "dir"),
// skipping .git (its objects differ by construction).
func snap(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// CLI-surface pointers (FR-11) are symlinks — record the target,
			// never follow (the target is a directory). Both entry-state legs
			// create identical pointers, so A-12 byte-identity is preserved.
			tgt, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			out[rel] = "symlink " + filepath.ToSlash(tgt)
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out[rel] = hexOf(b) + " " + info.Mode().Perm().String()
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

func sameSnap(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s: %s changed (%s → %s)", what, k, v, after[k])
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s: %s appeared", what, k)
		}
	}
}

func at(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

func commitSubjects(commits []installer.Commit) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

// ---- bare-dir install: git-inited, two commits, epoch (FR-7, A-12) ----------

func TestBareDirInstallTwoCommitsEpoch(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada Lovelace", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// A repository was created (no .git existed).
	if !rep.GitInited {
		t.Error("GitInited = false, want true (a bare directory has no repository)")
	}
	if _, serr := os.Stat(at(target, ".git")); serr != nil {
		t.Fatalf("no .git after a bare-dir install: %v", serr)
	}
	if rep.Branch != "main" {
		t.Errorf("Branch = %q, want main", rep.Branch)
	}

	// EXACTLY two commits, in order, and the epoch points at commit #1.
	if want := []string{"pyxis init — converged tree", "pyxis init — epoch"}; !slices.Equal(commitSubjects(rep.Commits), want) {
		t.Fatalf("commit subjects = %v, want %v", commitSubjects(rep.Commits), want)
	}
	if !rep.EpochWritten {
		t.Error("EpochWritten = false, want true")
	}
	log := strings.Fields(strings.TrimSpace(git(t, target, "rev-list", "HEAD")))
	if len(log) != 2 {
		t.Fatalf("git rev-list HEAD has %d commits, want exactly 2:\n%v", len(log), log)
	}

	// The config closed-parses and carries the epoch pointing at commit #1.
	cfgBytes, rerr := os.ReadFile(at(target, ".pyxis/config.yaml"))
	if rerr != nil {
		t.Fatalf("read config: %v", rerr)
	}
	c, defects := config.Parse(cfgBytes)
	if len(defects) > 0 {
		t.Fatalf("written config does not closed-parse: %v", defects)
	}
	if c.Epoch.Commit != rep.Commits[0].SHA {
		t.Errorf("epoch.commit = %q, want commit #1 sha %q", c.Epoch.Commit, rep.Commits[0].SHA)
	}
	if c.Epoch.SystemVersion != r.ver || c.Epoch.ManifestSHA256 != r.pin {
		t.Errorf("epoch pin = {%q,%q}, want {%q,%q}", c.Epoch.SystemVersion, c.Epoch.ManifestSHA256, r.ver, r.pin)
	}
	if c.System.Mode != "release" || c.System.ArtifactSource != "file://"+r.src {
		t.Errorf("system = %+v, want release mode + the artifact-source override", c.System)
	}

	// The operand .gitignore was written and lists the runtime caches.
	if !rep.GitIgnoreWritten {
		t.Error("GitIgnoreWritten = false, want true")
	}
	gi, rerr := os.ReadFile(at(target, ".gitignore"))
	if rerr != nil {
		t.Fatalf("read .gitignore: %v", rerr)
	}
	for _, dir := range []string{".pyxis/bin/", "/.pyxis/logs/", ".pyxis/feedback-queue/", ".pyxis/contract-gate-reports/", ".pyxis/scratch/"} {
		if !strings.Contains(string(gi), dir) {
			t.Errorf(".gitignore lacks %q:\n%s", dir, gi)
		}
	}

	// The converged tree materialized and is committed (tracked). The config,
	// the .gitignore, and every verbatim/generated file are tracked.
	tracked := map[string]bool{}
	for ln := range strings.SplitSeq(strings.TrimSpace(git(t, target, "ls-files")), "\n") {
		tracked[ln] = true
	}
	// The two init writes, converge's engine-verbatim + generated files, AND the
	// T8 seeded set (skeletons, AGENTS.md, registries, the installed hook
	// variants) are all tracked in the one genesis commit.
	for _, want := range []string{
		".pyxis/config.yaml", ".gitignore",
		".pyxis/templates/product-vision-skeleton.md", ".pyxis/scripts/hook.sh",
		".github/workflows/docs-gate.yml", "docs/tech-design/index.md", "docs/tech-design/index.json",
		// T8 seeded (plan installer/001 T8): the instantiated governance docs +
		// manual + registries + the operand hook variants. Only the two governing
		// docs are seeded; the living maps are opt-in (not seeded).
		"docs/product-vision.md", "docs/architecture-principles.md",
		"AGENTS.md", "docs/observability/incidents.md",
		".githooks/pre-commit", ".githooks/pre-push", ".githooks/prepare-commit-msg",
	} {
		if !tracked[want] {
			t.Errorf("%s is not tracked after the tree commit (tracked: %v)", want, tracked)
		}
	}
	// The gitignored cache never enters the commit (explicit-path staging).
	if tracked[".pyxis/bin/docgate-"+r.ver+"-"+r.plat] {
		t.Error("a gitignored cache entry was committed — staging must be by explicit path")
	}
}

// ---- A-12: the empty-git-repo leg is byte-identical to the bare-dir leg ------

func TestEmptyGitRepoLegByteIdenticalToBareDir(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada Lovelace", "ada@example.com")
	r := newRelease(t)

	// Leg A: a bare directory (init runs git init).
	bareTarget := t.TempDir()
	repA, err := installer.Init(context.Background(), r.baseOptions(bareTarget))
	if err != nil {
		t.Fatalf("bare-dir Init: %v", err)
	}

	// Leg B: a pre-existing empty git repo on main.
	gitTarget := t.TempDir()
	git(t, gitTarget, "init", "-b", "main")
	repB, err := installer.Init(context.Background(), r.baseOptions(gitTarget))
	if err != nil {
		t.Fatalf("empty-git-repo Init: %v", err)
	}
	if repB.GitInited {
		t.Error("empty-git-repo leg reported GitInited = true (the repo already existed)")
	}

	// git ls-files: identical tracked path set.
	lsA := strings.TrimSpace(git(t, bareTarget, "ls-files"))
	lsB := strings.TrimSpace(git(t, gitTarget, "ls-files"))
	if lsA != lsB {
		t.Fatalf("tracked file sets differ:\nbare: %q\ngit:  %q", lsA, lsB)
	}

	// With the commit dates + identity pinned, commit #1 is reproducible, so
	// the epoch commit matches and the whole working tree is byte-identical.
	if repA.Commits[0].SHA != repB.Commits[0].SHA {
		t.Errorf("commit #1 sha differs (%s vs %s) — the epoch, hence the config, would diverge", repA.Commits[0].SHA, repB.Commits[0].SHA)
	}
	sameSnap(t, snap(t, bareTarget), snap(t, gitTarget), "A-12 tree parity")
}

// ---- UB-11: content-holding targets refuse pre-write, routed by name -------

func TestNonEmptyAndDirtyTargetsRefusePreWrite(t *testing.T) {
	r := newRelease(t)

	t.Run("tracked content", func(t *testing.T) {
		isolateGit(t)
		ambientIdentityConfig(t, "Ada", "ada@example.com")
		target := t.TempDir()
		git(t, target, "init", "-b", "main")
		git(t, target, "config", "user.name", "Ada")
		git(t, target, "config", "user.email", "ada@example.com")
		writeFile(t, at(target, "README.md"), []byte("existing project\n"), 0o644)
		git(t, target, "add", "--", "README.md")
		git(t, target, "commit", "-m", "seed")

		before := snap(t, target)
		_, err := installer.Init(context.Background(), r.baseOptions(target))
		if err == nil {
			t.Fatal("Init = nil error, want the UB-11 refusal")
		}
		if !errorsIs(err, preflight.ErrRefused) {
			t.Errorf("err = %v, want a preflight.ErrRefused refusal", err)
		}
		if !strings.Contains(err.Error(), "README.md") {
			t.Errorf("refusal does not name the conflicting path: %v", err)
		}
		if !strings.Contains(err.Error(), "pyxis adopt") {
			t.Errorf("refusal does not name the adopt route (FR-2): %v", err)
		}
		sameSnap(t, before, snap(t, target), "refused non-empty target")
	})

	t.Run("dirty worktree", func(t *testing.T) {
		isolateGit(t)
		ambientIdentityConfig(t, "Ada", "ada@example.com")
		target := t.TempDir()
		git(t, target, "init", "-b", "main")
		// An untracked file makes the worktree dirty (status --porcelain).
		writeFile(t, at(target, "scratch.txt"), []byte("wip\n"), 0o644)

		before := snap(t, target)
		_, err := installer.Init(context.Background(), r.baseOptions(target))
		if err == nil || !errorsIs(err, preflight.ErrRefused) {
			t.Fatalf("err = %v, want a preflight.ErrRefused refusal", err)
		}
		if !strings.Contains(err.Error(), "scratch.txt") {
			t.Errorf("refusal does not name the dirty path: %v", err)
		}
		if !strings.Contains(err.Error(), "pyxis adopt") {
			t.Errorf("refusal does not name the adopt route (FR-2): %v", err)
		}
		if _, serr := os.Stat(at(target, ".pyxis")); serr == nil {
			t.Error("a refused dirty target still had .pyxis written")
		}
		sameSnap(t, before, snap(t, target), "refused dirty target")
	})

	t.Run("non-git content", func(t *testing.T) {
		// No .git at all: FR-2's other refusal class routes through the
		// runnable two-step (git init -b <branch>, then adopt) rather than
		// adopt alone, since there is no repository yet to hand it.
		target := t.TempDir()
		writeFile(t, at(target, "README.md"), []byte("existing project\n"), 0o644)

		before := snap(t, target)
		_, err := installer.Init(context.Background(), r.baseOptions(target))
		if err == nil || !errorsIs(err, preflight.ErrRefused) {
			t.Fatalf("err = %v, want a preflight.ErrRefused refusal", err)
		}
		for _, want := range []string{"README.md", "git init -b", "pyxis adopt"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal missing %q: %v", want, err)
			}
		}
		if _, serr := os.Stat(at(target, ".git")); serr == nil {
			t.Error("a refused non-git target had a repository created")
		}
		sameSnap(t, before, snap(t, target), "refused non-git content target")
	})
}

// ---- FR-3/FR-5: front-loaded toolchain + artifact-source checks (T2) -------

// buildToolchainPATH writes a scripted `--version` stand-in
// (writeFakeVersionTool, TestMain's own helper) for every (tool, output)
// pair into a fresh dir and returns it — a tool omitted from overrides is
// absent from PATH entirely, so exec.LookPath misses it exactly as it would
// on a machine without that tool installed. Mirrors internal/preflight's
// own toolchain_test.go helper of the same shape, reproduced here because
// this suite exercises the check through Init() end to end, not
// preflight.Run() directly.
func buildToolchainPATH(t *testing.T, overrides map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, output := range overrides {
		writeFakeVersionTool(dir, name, output)
	}
	return dir
}

// TestToolchainDefectRefusesPreWrite proves the front-loaded Toolchain check
// (design init-hardening-v1 §3.1 check 2; DINV-1; plan init-hardening/001
// T2's acceptance criteria) refuses Init() before any write, at the same
// pre-write step admission already occupies: an absent or below-floor
// executable names that tool's remedy, fires EVT-2 with check=toolchain
// (never init_checks_passed), and leaves a fresh target exactly as found —
// nothing written, not even .git.
func TestToolchainDefectRefusesPreWrite(t *testing.T) {
	r := newRelease(t)

	cases := []struct {
		name       string
		overrides  map[string]string // tool -> --version output; omitted = absent
		wantSubstr []string
	}{
		{
			name:       "git absent",
			overrides:  map[string]string{},
			wantSubstr: []string{"git: not found on PATH", "install git"},
		},
		{
			name:       "git below floor",
			overrides:  map[string]string{"git": "git version 2.10.0 (fixture)"},
			wantSubstr: []string{"git: 2.10.0 is below the required", "upgrade git"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := buildToolchainPATH(t, tc.overrides)
			t.Setenv("PATH", dir)

			target := t.TempDir()
			var buf bytes.Buffer
			opts := r.baseOptions(target)
			opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

			before := snap(t, target)
			_, err := installer.Init(context.Background(), opts)
			if err == nil || !errorsIs(err, preflight.ErrRefused) {
				t.Fatalf("err = %v, want a preflight.ErrRefused refusal", err)
			}
			for _, want := range tc.wantSubstr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q missing %q", err, want)
				}
			}
			sameSnap(t, before, snap(t, target), "refused toolchain-defect target")

			evt := findEvent(t, buf.String(), "init_refused")
			if evt == nil {
				t.Fatalf("no init_refused event in:\n%s", buf.String())
			}
			if evt["check"] != preflight.ToolchainID {
				t.Errorf("event check = %v, want %q", evt["check"], preflight.ToolchainID)
			}
			if strings.Contains(buf.String(), "init_checks_passed") {
				t.Error("init_checks_passed fired despite a toolchain refusal")
			}
		})
	}
}

// TestArtifactSourceUnreachableRefusesPreWrite proves the front-loaded
// Artifact-source check (design init-hardening-v1 §3.1 check 5; DINV-1; plan
// init-hardening/001 T2's acceptance criteria) refuses Init() before any
// write when the cache is cold and the configured source does not answer:
// the refusal names the host and the credential remedy, fires EVT-2 with
// check=artifact-source, and leaves a fresh target exactly as found.
func TestArtifactSourceUnreachableRefusesPreWrite(t *testing.T) {
	r := newRelease(t) // Pin shape only — ArtifactSource is overridden below
	target := t.TempDir()
	unreachable := filepath.Join(t.TempDir(), "does-not-exist") // deliberately never created

	var buf bytes.Buffer
	opts := r.baseOptions(target)
	opts.ArtifactSource = "file://" + unreachable
	opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	before := snap(t, target)
	_, err := installer.Init(context.Background(), opts)
	if err == nil || !errorsIs(err, preflight.ErrRefused) {
		t.Fatalf("err = %v, want a preflight.ErrRefused refusal", err)
	}
	for _, want := range []string{"release-manifest-" + r.ver, "check network access to"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q missing %q", err, want)
		}
	}
	sameSnap(t, before, snap(t, target), "refused artifact-source-unreachable target")

	evt := findEvent(t, buf.String(), "init_refused")
	if evt == nil {
		t.Fatalf("no init_refused event in:\n%s", buf.String())
	}
	if evt["check"] != preflight.ArtifactSourceID {
		t.Errorf("event check = %v, want %q", evt["check"], preflight.ArtifactSourceID)
	}
	if strings.Contains(buf.String(), "init_checks_passed") {
		t.Error("init_checks_passed fired despite an artifact-source refusal")
	}
}

// ---- UB-10: identity unset everywhere + promptless → named refusal ----------

func TestIdentityUnsetPromptlessRefusesNoHalfState(t *testing.T) {
	isolateGit(t) // GIT_CONFIG_GLOBAL=/dev/null → no ambient identity
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target) // AnswersProvided, no GitName/GitEmail, no Stdin
	_, err := installer.Init(context.Background(), opts)
	if err == nil {
		t.Fatal("Init = nil error, want the UB-10 identity refusal")
	}
	for _, want := range []string{"user.name", "git config --global"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q: %v", want, err)
		}
	}
	// No git init, no half state: the target must have NO .git after.
	if _, serr := os.Stat(at(target, ".git")); serr == nil {
		t.Error("identity refusal still initialized a repository (git init ran before the identity probe)")
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("identity refusal left state in the target: %v", entries)
	}
}

// ---- UB-10: identity from the answers file is written repo-local ------------

func TestIdentityFromAnswersWrittenRepoLocalCommitSucceeds(t *testing.T) {
	isolateGit(t) // no ambient identity — the answers file is the only source
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target)
	opts.Answers.GitName = "Grace Hopper"
	opts.Answers.GitEmail = "grace@example.com"

	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.Identity.NameSource != "answers" || rep.Identity.EmailSource != "answers" {
		t.Errorf("identity sources = (%s,%s), want (answers,answers)", rep.Identity.NameSource, rep.Identity.EmailSource)
	}
	// The identity was written repo-local (so the commit could not fail).
	if got := strings.TrimSpace(git(t, target, "config", "--local", "user.name")); got != "Grace Hopper" {
		t.Errorf("repo-local user.name = %q, want Grace Hopper", got)
	}
	if got := strings.TrimSpace(git(t, target, "config", "--local", "user.email")); got != "grace@example.com" {
		t.Errorf("repo-local user.email = %q, want grace@example.com", got)
	}
	// The commit carries that identity.
	if got := strings.TrimSpace(git(t, target, "log", "-1", "--format=%an <%ae>")); got != "Grace Hopper <grace@example.com>" {
		t.Errorf("commit author = %q, want the answers-file identity", got)
	}
}

// ---- UB-10: an ambient half is NOT rewritten repo-local ---------------------

func TestAmbientIdentityHalvesNotDuplicatedLocally(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ambient Name", "ambient@example.com")
	r := newRelease(t)
	target := t.TempDir()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.Identity.NameSource != "ambient" || rep.Identity.EmailSource != "ambient" {
		t.Fatalf("identity sources = (%s,%s), want (ambient,ambient)", rep.Identity.NameSource, rep.Identity.EmailSource)
	}
	// An ambient half is already in git's config — init must NOT write it
	// repo-local (the UB-10 don't-duplicate rule): --local read returns empty.
	cmd := exec.Command("git", "-C", target, "config", "--local", "--get", "user.name")
	if out, _ := cmd.Output(); strings.TrimSpace(string(out)) != "" {
		t.Errorf("ambient user.name was duplicated into the repo-local config: %q", strings.TrimSpace(string(out)))
	}
}

// ---- FR-4: an answers-file leg resolves knobs with zero prompts -------------

func TestAnswersFileLegZeroPrompts(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	// Parse a real answers-file body (the closed subset) and feed it, with a
	// CLOSED stdin (any read would be an unwanted prompt).
	ans, perr := installer.ParseAnswers([]byte(`# an operator's answers
tracker: github
ci_mode: actions
wiring: { repo: create, protection: checks+review }
cli_surfaces: []
argus: true
coordinate: octo/widget
`))
	if perr != nil {
		t.Fatalf("ParseAnswers: %v", perr)
	}
	opts := installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource:  "file://" + r.src,
		Answers:         ans,
		AnswersProvided: true,
		WireRunner:      hermeticGH{},
		Stdin:           failReader{t}, // a prompt would read this and fail the test
		Logger:          quiet(),
	}
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.Knobs.Tracker != "github" || rep.Knobs.CIMode != "actions" || !rep.Knobs.Argus {
		t.Errorf("knobs from the answers file wrong: %+v", rep.Knobs)
	}
	if rep.Knobs.Wiring.Repo != "create" || rep.Knobs.Wiring.Protection != "checks+review" {
		t.Errorf("wiring from the answers file wrong: %+v", rep.Knobs.Wiring)
	}
}

// failReader fails the test on any read — the zero-prompt probe.
type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Error("stdin was read on a promptless (answers-provided) run — FR-4 requires zero prompts")
	return 0, io.EOF
}

// ---- FR-4: --yes defaults every unset knob, promptless ----------------------

func TestAcceptDefaultsResolvesRecommendedKnobs(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := installer.Options{
		Target:         target,
		Pin:            installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource: "file://" + r.src,
		AcceptDefaults: true, // --yes: promptless, everything defaulted
		Answers:        installer.Answers{Coordinate: fixtureCoordinate},
		WireRunner:     hermeticGH{},
		Stdin:          failReader{t}, // a prompt would fail the test
		Logger:         quiet(),
	}
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// The recommended defaults (FR-4): github + both + use/required-checks +
	// argus off. cli_surfaces is auto-detected (PATH-dependent) — not asserted
	// here beyond "no auto sentinel survives".
	if rep.Knobs.Tracker != "github" || rep.Knobs.CIMode != "both" {
		t.Errorf("defaults wrong: tracker=%q ci_mode=%q", rep.Knobs.Tracker, rep.Knobs.CIMode)
	}
	if rep.Knobs.Wiring.Repo != "use" || rep.Knobs.Wiring.Protection != "required-checks" {
		t.Errorf("wiring defaults wrong: %+v", rep.Knobs.Wiring)
	}
	if rep.Knobs.Argus {
		t.Error("argus defaulted on — it is opt-in, off by default (FR-16)")
	}
	if slices.Contains(rep.Knobs.CLISurfaces, "auto") {
		t.Error("the auto sentinel survived into the resolved knobs")
	}
}

// ---- FR-19: preview performs nothing, populates the would-do set ------------

func TestPreviewZeroWritesReportsWouldDoSet(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target)
	opts.Preview = true
	before := snap(t, target) // an empty dir

	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("preview Init: %v", err)
	}
	if !rep.Preview {
		t.Error("Report.Preview = false on a preview run")
	}
	if !rep.WouldGitInit {
		t.Error("WouldGitInit = false, want true (a bare directory)")
	}
	if len(rep.ConfigBytes) == 0 {
		t.Error("preview ConfigBytes empty — the would-write config is part of the would-do set")
	}
	if _, defects := config.Parse(rep.ConfigBytes); len(defects) > 0 {
		t.Errorf("preview config does not closed-parse: %v", defects)
	}
	if len(rep.Stages) == 0 {
		t.Error("preview Stages empty — the pipeline stage list is part of the would-do set")
	}
	if len(rep.Commits) != 0 || rep.EpochWritten {
		t.Error("preview reported commits/epoch — nothing was installed")
	}
	// ZERO writes: no .git, no config, no .gitignore, no cache — the dir is
	// exactly as before.
	if _, serr := os.Stat(at(target, ".git")); serr == nil {
		t.Error("preview created a repository")
	}
	sameSnap(t, before, snap(t, target), "preview")
}

// ---- config round-trips end to end (parseRendered guards internally too) ----

func TestWrittenConfigReParsesEndToEnd(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
		t.Fatalf("Init: %v", err)
	}
	b, rerr := os.ReadFile(at(target, ".pyxis/config.yaml"))
	if rerr != nil {
		t.Fatalf("read config: %v", rerr)
	}
	c, defects := config.Parse(b)
	if len(defects) > 0 {
		t.Fatalf("the end-to-end written config does not re-parse: %v", defects)
	}
	// Every captured path key holds its compiled default (v2 refuses otherwise).
	for k, want := range config.PathDefaults() {
		got := pathValue(c, k)
		if got != want {
			t.Errorf("paths.%s = %q, want the compiled default %q", k, got, want)
		}
	}
}

// pathValue reads a captured path off the parsed config by its schema key.
func pathValue(c config.Config, key string) string {
	switch key {
	case "prds":
		return c.PRDs
	case "drafts":
		return c.Drafts
	case "feedback":
		return c.Feedback
	case "tech_designs":
		return c.TechDesigns
	case "solution_shapes":
		return c.SolutionShapes
	case "brainstorms":
		return c.Brainstorms
	case "problems":
		return c.Problems
	case "reviews":
		return c.Reviews
	case "wiki":
		return c.Wiki
	case "rollup_prd":
		return c.RollupPRD
	case "rollup_techspec":
		return c.RollupTechspec
	case "incidents":
		return c.Incidents
	case "product_vision":
		return c.ProductVision
	case "architecture_principles":
		return c.ArchitecturePrinciples
	case "design_principles":
		return c.DesignPrinciples
	case "plans":
		return c.Plans
	case "work_items":
		return c.WorkItems
	case "risk_policy":
		return c.RiskPolicy
	case "maintenance_policy":
		return c.MaintenancePolicy
	case "templates_dir":
		return c.TemplatesDir
	}
	return "<unknown key>"
}

// ---- unknown answers-file key refuses ---------------------------------------

func TestUnknownAnswersFileKeyRefuses(t *testing.T) {
	_, err := installer.ParseAnswers([]byte("tracker: github\nmood: cheerful\n"))
	if err == nil {
		t.Fatal("ParseAnswers = nil, want the closed-grammar refusal")
	}
	if !strings.Contains(err.Error(), "unknown key") || !strings.Contains(err.Error(), "mood") {
		t.Errorf("refusal not named: %v", err)
	}
	// A malformed enum surfaces at resolution, named by knob.
	bad := true
	opts := installer.Options{
		Answers:         installer.Answers{Tracker: "gitlab", Argus: &bad},
		AnswersProvided: true,
	}
	// resolveKnobs is unexported; exercise it via the public seam by driving
	// Init far enough to resolve knobs. Target validation must pass first,
	// and — since Pin now activates the front-loaded artifact-source check
	// (plan init-hardening/001 T2) — ArtifactSource must resolve to a
	// reachable fixture too, or the real default (github.com) refuses this
	// hermetic test on a 404/no-network before it ever reaches knobs.
	r := newRelease(t)
	target := t.TempDir()
	opts.Target = target
	opts.Pin = installer.Pin{Version: r.ver, ManifestSHA256: r.pin}
	opts.ArtifactSource = "file://" + r.src
	opts.Logger = quiet()
	_, kerr := installer.Init(context.Background(), opts)
	if kerr == nil || !strings.Contains(kerr.Error(), "knobs.tracker") {
		t.Errorf("out-of-enum tracker not refused by name: %v", kerr)
	}
}

// ---- auto cli-surfaces detection via PATH stubs -----------------------------

func TestAutoCLISurfacesDetectsViaPATH(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)

	// A PATH with fake claude + codex executables (but not cursor/opencode).
	// Git is symlinked into the same dir for the toolchain check. Detection
	// then finds exactly the installed CLI subset regardless of what the
	// developer's machine has.
	binDir := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		writeFile(t, filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755)
	}
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		t.Skipf("git not on PATH: %v", lerr)
	}
	if serr := os.Symlink(gitPath, filepath.Join(binDir, "git")); serr != nil {
		t.Fatalf("symlink git into the stub dir: %v", serr)
	}
	t.Setenv("PATH", binDir) // ONLY the stub dir — deterministic detection

	target := t.TempDir()
	opts := installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource:  "file://" + r.src,
		AnswersProvided: true,
		Answers:         installer.Answers{CLISurfaces: []string{"auto"}}, // detect
		// Unset Tracker/CIMode/WiringRepo/WiringProtection default to
		// github/both/use/required-checks (FR-4) — squarely GitHub-mode
		// applicable, so the preflight needs a coordinate and a transport.
		WireRunner: hermeticGH{},
		// Identity via answers so no ambient git/ PATH lookups matter.
		Logger: quiet(),
	}
	opts.Answers.GitName = "Ada"
	opts.Answers.GitEmail = "ada@example.com"
	opts.Answers.Coordinate = fixtureCoordinate

	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if want := []string{"claude", "codex"}; !slices.Equal(rep.Knobs.CLISurfaces, want) {
		t.Fatalf("detected cli_surfaces = %v, want %v (the installed subset on PATH)", rep.Knobs.CLISurfaces, want)
	}
}

// ---- EVT-1: emitted on success, silent on a failed pipeline stage -----------

func TestEVT1EmittedOnSuccessOnly(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)

	// Success: the install_completed event fires with the contracted fields.
	t.Run("success emits", func(t *testing.T) {
		target := t.TempDir()
		var buf bytes.Buffer
		opts := r.baseOptions(target)
		opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
		if _, err := installer.Init(context.Background(), opts); err != nil {
			t.Fatalf("Init: %v", err)
		}
		evt := findEvent(t, buf.String(), "install_completed")
		if evt == nil {
			t.Fatalf("no install_completed event in:\n%s", buf.String())
		}
		if evt["version"] != r.ver || evt["tracker"] != "github" {
			t.Errorf("event fields wrong: %v", evt)
		}
		if evt["run_id"] == nil || evt["run_id"] == "" {
			t.Error("event missing run_id (the correlation id)")
		}
		if _, ok := evt["duration_ms"]; !ok {
			t.Error("event missing duration_ms")
		}
	})

	// Failure: swap a package-level pipeline slot (t8Slots.Seed) to a failing
	// stage via the export_test seam — EVT-1 must NOT fire (the
	// fire-only-after-the-whole-pipeline contract). The seam sets-and-restores.
	t.Run("failed stage stays silent", func(t *testing.T) {
		target := t.TempDir()
		restore := installer.SetFailingSeedStageForTest()
		defer restore()

		var buf bytes.Buffer
		opts := r.baseOptions(target)
		opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
		_, err := installer.Init(context.Background(), opts)
		if err == nil {
			t.Fatal("Init = nil error, want the injected stage failure")
		}
		if !strings.Contains(err.Error(), "seed") {
			t.Errorf("error does not name the failed stage: %v", err)
		}
		if strings.Contains(buf.String(), "install_completed") {
			t.Errorf("install_completed fired despite a failed pipeline stage:\n%s", buf.String())
		}
	})
}

// ---- init_checks_passed: prd:init-hardening's own EVT-1 (T2) ---------------
//
// Distinct from install_completed above (installer-v1's differently-scoped,
// coincidentally-also-"EVT-1" event, prd:installer's own numbering):
// init_checks_passed fires the instant every front-loaded preflight check
// has passed — step 1, before knob resolution and before any write —
// whereas install_completed fires only after the whole pipeline, T8's tail
// included.

// TestInitChecksPassedEventFiresOnceOnSuccess proves design init-hardening-v1
// §3.4 / plan init-hardening/001 T2's acceptance criterion: on a successful
// pass, init_checks_passed fires exactly once, naming every evaluated check
// in registry evaluation order (admission, toolchain, artifact-source) plus
// its own duration.
func TestInitChecksPassedEventFiresOnceOnSuccess(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var buf bytes.Buffer
	opts := r.baseOptions(target)
	opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	if _, err := installer.Init(context.Background(), opts); err != nil {
		t.Fatalf("Init: %v", err)
	}

	var matches int
	var evt map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == "init_checks_passed" {
			matches++
			evt = m
		}
	}
	if matches != 1 {
		t.Fatalf("init_checks_passed fired %d time(s), want exactly 1:\n%s", matches, buf.String())
	}
	if evt["run_id"] == nil || evt["run_id"] == "" {
		t.Error("event missing run_id (the correlation id)")
	}
	if _, ok := evt["duration_ms"]; !ok {
		t.Error("event missing duration_ms")
	}
	checksField, ok := evt["checks"].([]any)
	if !ok {
		t.Fatalf("event checks = %v (%T), want a JSON array", evt["checks"], evt["checks"])
	}
	got := make([]string, len(checksField))
	for i, c := range checksField {
		got[i], _ = c.(string)
	}
	want := []string{preflight.AdmissionID, preflight.ToolchainID, preflight.ProcessBoundaryID, preflight.ArtifactSourceID, preflight.CrossKnobCoherenceID}
	if !slices.Equal(got, want) {
		t.Errorf("checks = %v, want %v (registry evaluation order)", got, want)
	}
}

// ---- EVT-2: emitted on every preflight refusal, with typed properties ------

func TestEVT2InitRefusedEvent(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)

	target := t.TempDir()
	git(t, target, "init", "-b", "main")
	git(t, target, "config", "user.name", "Ada")
	git(t, target, "config", "user.email", "ada@example.com")
	writeFile(t, at(target, "README.md"), []byte("existing project\n"), 0o644)
	git(t, target, "add", "--", "README.md")
	git(t, target, "commit", "-m", "seed")

	var buf bytes.Buffer
	opts := r.baseOptions(target)
	opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	if _, err := installer.Init(context.Background(), opts); err == nil {
		t.Fatal("Init = nil error, want the admission refusal")
	}

	evt := findEvent(t, buf.String(), "init_refused")
	if evt == nil {
		t.Fatalf("no init_refused event in:\n%s", buf.String())
	}
	if evt["check"] != preflight.AdmissionID {
		t.Errorf("event check = %v, want %q", evt["check"], preflight.AdmissionID)
	}
	if evt["remedy_present"] != true {
		t.Errorf("event remedy_present = %v, want true", evt["remedy_present"])
	}
	if evt["route_present"] != true {
		t.Errorf("event route_present = %v, want true", evt["route_present"])
	}
	if strings.Contains(buf.String(), "install_completed") {
		t.Error("install_completed fired despite an admission refusal")
	}
}

func findEvent(t *testing.T, logs, msg string) map[string]any {
	t.Helper()
	var found map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(logs), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == msg {
			found = m
		}
	}
	return found
}

// errorsIs is a tiny local errors.Is (avoids importing errors just for one call
// alongside the other helpers).
func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// ---- FR-12/FR-13: the answerable question surface --------------------------
//
// interactiveOptions builds a pure-interactive Options (no answers/flags, no
// --yes) over stdin, so Init constructs a real prompter: !AnswersProvided &&
// !AcceptDefaults && Stdin != nil (installer.go's own gate). Ambient identity
// is the caller's job (ambientIdentityConfig) so UB-10's identity halves
// never consume any of the scripted knob answers below.
func interactiveOptions(r release, target, stdin string, stdout *bytes.Buffer) installer.Options {
	return installer.Options{
		Target:         target,
		Pin:            installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource: "file://" + r.src,
		// wiring.protection's recommended default (required-checks) is
		// tracker-independent, so even a scripted "carina" answer can still
		// leave GitHub mode applicable — the coordinate and transport travel
		// promptless here exactly as a --coordinate flag would alongside
		// otherwise-interactive knobs (FR-7 is per-field, not per-run).
		Answers:    installer.Answers{Coordinate: fixtureCoordinate},
		WireRunner: hermeticGH{},
		Stdin:      strings.NewReader(stdin),
		Stdout:     stdout,
		Logger:     quiet(),
	}
}

// TestInteractiveClosedStdinRendersQuestionsAndTakesDefaults drives a real
// prompter (Stdin non-nil, nothing else provided) whose stream is immediately
// closed. Two FR-12/FR-13 facts at once: (1) every one of the five knobs'
// questions renders its gloss, its closed value set, and its default — the
// tracker-derived ci_mode default naming its basis — and (2) a closed input
// stream still resolves every knob to its recommended default without ever
// entering the re-ask loop (today's EOF semantics, now proven through a live
// prompter rather than only through the promptless pr==nil path).
func TestInteractiveClosedStdinRendersQuestionsAndTakesDefaults(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com") // identity asks nothing
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	opts := interactiveOptions(r, target, "", &out) // "" — stdin closes immediately
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// The recommended defaults, exactly TestAcceptDefaultsResolvesRecommendedKnobs'
	// promptless expectations — an EOF'd prompter must resolve identically.
	if rep.Knobs.Tracker != "github" || rep.Knobs.CIMode != "both" {
		t.Errorf("defaults wrong: tracker=%q ci_mode=%q", rep.Knobs.Tracker, rep.Knobs.CIMode)
	}
	if rep.Knobs.Wiring.Repo != "use" || rep.Knobs.Wiring.Protection != "required-checks" {
		t.Errorf("wiring defaults wrong: %+v", rep.Knobs.Wiring)
	}
	if rep.Knobs.Argus {
		t.Error("argus defaulted on — it is opt-in, off by default (FR-16)")
	}
	if slices.Contains(rep.Knobs.CLISurfaces, "auto") {
		t.Error("the auto sentinel survived into the resolved knobs")
	}

	transcript := out.String()
	for _, want := range []string{
		"Where should pyxis track issues and PRs? (carina is the built-in tracker; it works fully offline)",
		"issue tracker (github|carina) [github]:",
		"Where should changes be checked before they land?",
		"recommended: both — your GitHub tracker makes hosted CI available",
		"CI mode (local|actions|both) [both]:",
		"Should pyxis use an existing GitHub repository, or create a new private one for you?",
		"GitHub repo wiring (use|create) [use]:",
		"How strictly should the main branch be protected?",
		"recommended: required-checks — hosted CI is on, so GitHub can require its check to pass",
		"branch protection (none|required-checks|checks+review) [required-checks]:",
		"Enable argus, the AI engineer that can drive work on its own? (off keeps every decision with you)",
		"enable argus (true|false) [false]:",
		"Which coding agents should get pyxis's skills? (auto detects what's installed)",
		"agent CLI surfaces (auto | a comma-separated list (detectable: claude, codex, cursor, opencode)) [auto]:",
	} {
		if !strings.Contains(transcript, want) {
			t.Errorf("transcript lacks %q:\n%s", want, transcript)
		}
	}
}

// TestInteractiveOutOfEnumReAsksAndRecovers proves the bounded re-ask loop
// (FR-13) never kills the run on one bad keystroke: an invalid tracker answer
// re-asks (no process death, no error), and the next line — a valid answer —
// completes resolution.
func TestInteractiveOutOfEnumReAsksAndRecovers(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	// tracker: "bogus" (invalid, re-asks) then "carina" (valid); every
	// question after it closes stdin and takes its default.
	opts := interactiveOptions(r, target, "bogus\ncarina\n", &out)
	opts.Answers.RemoteURL = filepath.Join(r.src, "local.git")
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if rep.Knobs.Tracker != "carina" {
		t.Errorf("Knobs.Tracker = %q, want carina (the recovered second attempt)", rep.Knobs.Tracker)
	}
	if rep.Knobs.CIMode != "local" {
		t.Errorf("Knobs.CIMode = %q, want local (tracker=carina's derived default)", rep.Knobs.CIMode)
	}
	if !strings.Contains(out.String(), `"bogus" is not one of github|carina`) {
		t.Errorf("transcript does not name the rejected answer:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "2 attempt(s) left") {
		t.Errorf("transcript does not report the remaining attempts:\n%s", out.String())
	}
}

// TestInteractiveOutOfEnumAbortsListingCollectedAnswers proves FR-13's bound:
// three consecutive invalid answers to one question abort the run — but
// unlike the pre-T7 behavior (one bad keystroke silently discarded every
// prior answer), the abort names the failing question and reports every knob
// already resolved. It also proves the abort is still pre-write (DINV-1):
// knob resolution runs before any mutation, so the target is untouched.
func TestInteractiveOutOfEnumAbortsListingCollectedAnswers(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	before := snap(t, target)

	var out bytes.Buffer
	// tracker resolves cleanly to "carina"; ci_mode then gets three straight
	// invalid answers and must abort on the third, never reaching wiring.
	opts := interactiveOptions(r, target, "carina\nx\ny\nz\n", &out)
	_, err := installer.Init(context.Background(), opts)
	if err == nil {
		t.Fatal("Init = nil error, want the FR-13 bounded abort")
	}
	if !errorsIs(err, installer.ErrKnobAskAborted) {
		t.Errorf("err = %v, want ErrKnobAskAborted", err)
	}
	for _, want := range []string{"knobs.ci_mode", `"z" is not one of local|actions|both`, "tracker=carina"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("abort error lacks %q: %v", want, err)
		}
	}
	// wiring.repo was never reached — its default never appears as "given".
	if strings.Contains(err.Error(), "wiring.repo=") {
		t.Errorf("abort error names a knob past the failing one: %v", err)
	}
	sameSnap(t, before, snap(t, target), "aborted knob resolution")
}

// TestInteractiveCLISurfacesAutoMixReAsksThenRecovers proves the re-ask bound
// also covers cli_surfaces' one re-askable defect (mixing the auto sentinel
// with explicit names) even though its value set is open beyond the
// detectable few — FR-13 is not limited to resolveEnum's closed enums.
func TestInteractiveCLISurfacesAutoMixReAsksThenRecovers(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	// Five blank lines take tracker..argus's defaults; cli_surfaces then gets
	// one contradictory answer (auto mixed with a name) before a valid one.
	stdin := strings.Repeat("\n", 5) + "auto,claude\nclaude,codex\n"
	opts := interactiveOptions(r, target, stdin, &out)
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if want := []string{"claude", "codex"}; !slices.Equal(rep.Knobs.CLISurfaces, want) {
		t.Errorf("CLISurfaces = %v, want %v (the recovered second attempt)", rep.Knobs.CLISurfaces, want)
	}
	if !strings.Contains(out.String(), `"auto,claude" is not one of`) {
		t.Errorf("transcript does not name the rejected mix:\n%s", out.String())
	}
}

// Preview's declared wiring table equals the real run's declared set —
// including the onboarding-items row (#947 review finding: preview once
// omitted the sixth row, so a real run performed issue creates the preview
// never enumerated, breaking FR-24's every-external-operation contract).
func TestPreviewDeclaresTheOnboardingItemsRow(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	opts := r.baseOptions(target)
	opts.Preview = true
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("preview Init: %v", err)
	}
	var row *wire.Op
	for i := range rep.WiringOps {
		if rep.WiringOps[i].Name == wire.OpOnboardingItems {
			row = &rep.WiringOps[i]
		}
	}
	if row == nil {
		t.Fatalf("preview wiring ops %v lack the onboarding-items row the real run declares", rep.WiringOps)
	}
	if len(row.Items) != len(installer.OnboardingWorkItems(config.Defaults())) {
		t.Fatalf("preview onboarding row declares %d item(s), want the standing set", len(row.Items))
	}
}
