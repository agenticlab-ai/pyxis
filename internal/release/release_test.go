package release_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/release"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

const fixtureVersion = "v9.9.9"

// fixtureTools is the shipped set the fixture repo's cmd/ tree scans to
// (cmd/pyxis-release is present there and excluded by the rule). "pyxis" is
// the front door install.sh pins, so the fixture corpus carries it — the
// pins tests derive their anchor from exactly this set.
var fixtureTools = []string{"pyxis", "toola", "toolb"}

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// gitRun executes git in dir with a pinned, hermetic identity/config.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE=2026-07-19T00:00:00Z", "GIT_COMMITTER_DATE=2026-07-19T00:00:00Z",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, rel string, mode os.FileMode, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	// Permission-preservation tests need the requested mode under any umask.
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

const fixturePolicy = `schema_version: class-policy/v1
classes:
  - { glob: ".pyxis/config.yaml", class: repo-owned }
  - { glob: ".pyxis/**", class: engine-verbatim }
  - { glob: ".github/workflows/docs-gate.yml", class: generated }
  - { glob: "docs/tech-design/index.*", class: generated }
`

// fixtureOverlay is the overlay-SOURCE workflow template: a GENERATED surface
// carrying {{branch}}/{{remote}} tokens (config-driven-portability-v1 §3.1,
// FR-4). StagePayload renders it with config.Defaults() into the staged
// operand workflow, so the staged file is fixtureOverlayRendered (token-free,
// origin/main) — never the token-bearing source (which would be a broken
// workflow in an operand's .github/workflows/).
const fixtureOverlay = `name: docs-gate
on:
  push:
    branches: [{{branch}}]
jobs:
  docs-gate:
    runs-on: ubuntu-latest
    steps:
      - name: bootstrap pyxis
        run: bash .pyxis/scripts/pyxis-bootstrap.sh
      - name: gate
        run: |
          git fetch {{remote}} {{branch}}
          pyxis exec docgate -- gate --merge-base {{remote}}/{{branch}}
`

// fixtureOverlayRendered is fixtureOverlay under the default config (branch
// main, remote origin) — what StagePayload stages into .github/workflows/.
const fixtureOverlayRendered = `name: docs-gate
on:
  push:
    branches: [main]
jobs:
  docs-gate:
    runs-on: ubuntu-latest
    steps:
      - name: bootstrap pyxis
        run: bash .pyxis/scripts/pyxis-bootstrap.sh
      - name: gate
        run: |
          git fetch origin main
          pyxis exec docgate -- gate --merge-base origin/main
`

// fixtureRepo builds a minimal, committed source tree Pack can stage: the
// payload set, the overlay pair, the design
// indexes, and a cmd/ tree for the scan.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	needGit(t)
	dir := t.TempDir()
	writeFile(t, dir, ".pyxis/class-policy.yaml", 0o644, fixturePolicy)
	writeFile(t, dir, ".pyxis/config.yaml", 0o644, "schema_version: pyxis-config/v2\n")
	writeFile(t, dir, ".pyxis/templates/prd-template.md", 0o644, "# template\n")
	writeFile(t, dir, ".pyxis/scripts/clean.sh", 0o755, "#!/bin/sh\necho clean\n")
	writeFile(t, dir, ".pyxis/dist/github-workflows/docs-gate.yml", 0o644, fixtureOverlay)
	writeFile(t, dir, ".pyxis/dist/github-workflows/README.md", 0o644, "packer documentation, never mapped\n")
	writeFile(t, dir, "docs/tech-design/index.json", 0o644, "{}\n")
	writeFile(t, dir, "docs/tech-design/index.md", 0o644, "# index\n")
	for _, tool := range append(slices.Clone(fixtureTools), release.BuildOnlyTool) {
		writeFile(t, dir, "cmd/"+tool+"/main.go", 0o644, "package main\n\nfunc main() {}\n")
	}
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "seed", "--no-gpg-sign")
	return dir
}

// fixtureBinaries writes the full shipped-set × platform binary matrix.
func fixtureBinaries(t *testing.T) string {
	t.Helper()
	return fixtureBinariesFor(t, fixtureVersion)
}

// fixtureBinariesFor is fixtureBinaries for an arbitrary version — what the
// second-version packs need.
func fixtureBinariesFor(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	for _, tool := range fixtureTools {
		for _, plat := range release.Platforms {
			writeFile(t, dir, tool+"-"+version+"-"+plat, 0o755, "binary "+tool+" "+plat+"\n")
		}
	}
	return dir
}

func packFixture(t *testing.T, repo string) (outDir string, res release.PackResult) {
	t.Helper()
	outDir = t.TempDir()
	res, err := release.Pack(context.Background(), release.PackOptions{
		Version:     fixtureVersion,
		RepoRoot:    repo,
		OutDir:      outDir,
		BinariesDir: fixtureBinaries(t),
	})
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	return outDir, res
}

// readTarGz returns member name → content for assertions.
func readTarGz(t *testing.T, path string) (map[string][]byte, map[string]int64) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close archive: %v", err)
		}
	}()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	content, modes := map[string][]byte{}, map[string]int64{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		content[hdr.Name] = b
		modes[hdr.Name] = hdr.Mode
		if !hdr.ModTime.Equal(time.Unix(0, 0)) {
			t.Errorf("member %s carries a non-zeroed mtime %v", hdr.Name, hdr.ModTime)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Errorf("member %s carries uid/gid %d/%d", hdr.Name, hdr.Uid, hdr.Gid)
		}
	}
	return content, modes
}

// The complete pack: artifact set, manifest content, overlay mapping,
// deterministic archives, and a green Verify (A-1's dry-run shape).
func TestPackCompleteAndDeterministic(t *testing.T) {
	repo := fixtureRepo(t)
	out1, res := packFixture(t, repo)
	out2, _ := packFixture(t, repo)

	wantArtifacts := []string{
		"SHA256SUMS",
		"payload-" + fixtureVersion + ".tar.gz",
		"release-manifest-" + fixtureVersion + ".json",
	}
	for _, tool := range fixtureTools {
		for _, plat := range release.Platforms {
			wantArtifacts = append(wantArtifacts, tool+"-"+fixtureVersion+"-"+plat)
		}
	}
	sort.Strings(wantArtifacts)
	if !slices.Equal(res.Artifacts, wantArtifacts) {
		t.Fatalf("artifact set = %v, want %v", res.Artifacts, wantArtifacts)
	}
	if res.Manifest.DepsBundleSHA256 != "" {
		t.Fatal("new release unexpectedly pins a dependency bundle")
	}

	// Two packs of the same tree are byte-identical (the deterministic
	// archive contract publish's upload-or-verify relies on).
	for _, name := range wantArtifacts {
		b1, err := os.ReadFile(filepath.Join(out1, name))
		if err != nil {
			t.Fatal(err)
		}
		b2, err := os.ReadFile(filepath.Join(out2, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b1, b2) {
			t.Errorf("artifact %s differs across two packs of the same tree", name)
		}
	}

	// The manifest: derived, classed, complete.
	m := res.Manifest
	if m.SystemVersion != fixtureVersion || len(m.SourceCommit) != 40 {
		t.Errorf("manifest version/commit = %q/%q", m.SystemVersion, m.SourceCommit)
	}
	for _, tool := range fixtureTools {
		for _, plat := range release.Platforms {
			if len(m.Binaries[tool][plat]) != 64 {
				t.Errorf("manifest binaries[%s][%s] missing", tool, plat)
			}
		}
	}
	if _, ok := m.Binaries[release.BuildOnlyTool]; ok {
		t.Errorf("the build-only tool leaked into the shipped manifest")
	}
	classOf := map[string]relmanifest.Class{}
	hashed := map[string]bool{}
	for _, f := range m.Payload.Files {
		classOf[f.Path] = f.Class
		hashed[f.Path] = f.SHA256 != nil
	}
	for path, want := range map[string]relmanifest.Class{
		// The overlay TARGET is generated (converge renders it per-config —
		// config-driven-portability-v1 §3.1, FR-4); the overlay SOURCE stays
		// engine-verbatim (ships byte-exact under .pyxis/**).
		".github/workflows/docs-gate.yml":            relmanifest.Generated,
		".pyxis/config.yaml":                         relmanifest.RepoOwned,
		".pyxis/dist/github-workflows/docs-gate.yml": relmanifest.EngineVerbatim,
		"docs/tech-design/index.json":                relmanifest.Generated,
	} {
		if classOf[path] != want {
			t.Errorf("manifest classes %q as %q, want %q", path, classOf[path], want)
		}
	}
	// sha nullability: repo-owned and the generated overlay target carry null;
	// the engine-verbatim overlay source carries a hash.
	if hashed[".pyxis/config.yaml"] || hashed[".github/workflows/docs-gate.yml"] {
		t.Errorf("sha256 must be null for the repo-owned config and the generated overlay target")
	}
	if !hashed[".pyxis/dist/github-workflows/docs-gate.yml"] {
		t.Errorf("sha256 must be present for the engine-verbatim overlay source")
	}

	// The payload archive: operand-side layout — the overlay TARGET carries
	// the DEFAULT RENDER of the template (token-free, origin/main), the SOURCE
	// keeps its token-bearing bytes, the README maps nowhere, exec bits hold.
	content, modes := readTarGz(t, filepath.Join(out1, "payload-"+fixtureVersion+".tar.gz"))
	if got := string(content[".github/workflows/docs-gate.yml"]); got != fixtureOverlayRendered {
		t.Errorf("staged workflow is not the default render of the overlay template:\n%s", got)
	}
	if strings.Contains(string(content[".github/workflows/docs-gate.yml"]), "{{") {
		t.Errorf("staged operand workflow still carries a substitution token — a broken workflow would ship")
	}
	if _, ok := content[".github/workflows/README.md"]; ok {
		t.Errorf("the overlay README leaked into the operand workflow dir")
	}
	if got := string(content[".pyxis/dist/github-workflows/docs-gate.yml"]); got != fixtureOverlay {
		t.Errorf("the overlay source must ship its token-bearing template bytes, got:\n%s", got)
	}
	if modes[".pyxis/scripts/clean.sh"]&0o111 == 0 {
		t.Errorf("executable payload script lost its exec bit")
	}

	// The local verify half is green on a completed pack.
	if err := release.Verify(out1, fixtureVersion); err != nil {
		t.Fatalf("verify on a clean pack: %v", err)
	}
}

// The artifact set under a version is CLOSED, and SHA256SUMS is written by
// sweeping the whole out dir — so packing a second version over a leftover
// first one would otherwise produce a set covering BOTH, internally
// consistent and publishable, silently republishing the old release's
// binaries as assets of the new tag. Pack refuses first; Verify refuses any
// such set it is handed, whoever built it.
func TestPackAndVerifyRefuseForeignArtifacts(t *testing.T) {
	repo := fixtureRepo(t)
	out, _ := packFixture(t, repo)
	const other = "v9.9.8"

	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: other, RepoRoot: repo, OutDir: out,
		BinariesDir: fixtureBinariesFor(t, other),
	})
	if err == nil {
		t.Fatal("pack over another release's set succeeded, want a refusal")
	}
	for _, want := range []string{"another release", "payload-" + fixtureVersion + ".tar.gz"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not mention %q:\n%v", want, err)
		}
	}
	// Refused BEFORE writing: the operator's existing set is intact.
	if err := release.Verify(out, fixtureVersion); err != nil {
		t.Errorf("the refused pack disturbed the existing set: %v", err)
	}

	// And a set contaminated some other way (hand-built, an older pack)
	// fails verification rather than publishing.
	writeFile(t, out, "pyxis-"+other+"-linux-amd64", 0o755, "a foreign binary\n")
	if _, err := release.ExportedWriteSums(out); err != nil {
		t.Fatal(err)
	}
	err = release.Verify(out, fixtureVersion)
	if err == nil || !strings.Contains(err.Error(), "pyxis-"+other+"-linux-amd64") {
		t.Fatalf("verify = %v, want a refusal naming the foreign artifact", err)
	}
}

// Verify catches a flipped byte anywhere in the set, naming the artifact.
func TestVerifyCatchesFlippedByte(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	for _, name := range []string{
		"payload-" + fixtureVersion + ".tar.gz",
		"toola-" + fixtureVersion + "-linux-amd64",
	} {
		p := filepath.Join(out, name)
		orig, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		flipped := slices.Clone(orig)
		flipped[len(flipped)/2] ^= 0xff
		if err := os.WriteFile(p, flipped, 0o644); err != nil {
			t.Fatal(err)
		}
		err = release.Verify(out, fixtureVersion)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("verify after flipping %s = %v, want a refusal naming it", name, err)
		}
		if err := os.WriteFile(p, orig, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A vanished artifact is caught from both directions.
	p := filepath.Join(out, "toolb-"+fixtureVersion+"-darwin-arm64")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := release.Verify(out, fixtureVersion); err == nil || !strings.Contains(err.Error(), "toolb-"+fixtureVersion+"-darwin-arm64") {
		t.Errorf("verify with a missing binary = %v, want a refusal naming it", err)
	}
}

// Precondition (a): a staged file referencing a retired resolver refuses the
// pack by name — while the retired FILES may live on as exec-grammar
// delegates without tripping it (the post-cutover end state). The referencing
// file here is a shipped skill (the operand-breaking case the cutover fixed in
// a shipped authoring skill); the source-mode desk hooks are handled by
// TestPackExcludesSourceDeskHooks below.
func TestPackRefusesRetiredResolverReference(t *testing.T) {
	repo := fixtureRepo(t)
	writeFile(t, repo, ".pyxis/skills/reviewer/SKILL.md", 0o644, "# reviewer\n\nOn confirm run `.pyxis/scripts/docgate-resolve.sh` gate.\n")
	writeFile(t, repo, ".pyxis/scripts/docgate-resolve.sh", 0o755, "#!/bin/sh\n# one-line delegate (the cutover end state)\npyxis exec docgate -- \"$@\"\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "skill", "--no-gpg-sign")

	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: fixtureBinaries(t),
	})
	var pre *release.PreconditionError
	if !errors.As(err, &pre) || pre.Name != "retired-resolver-references" {
		t.Fatalf("pack = %v, want the retired-resolver-references precondition", err)
	}
	if len(pre.Findings) != 1 || !strings.Contains(pre.Findings[0], ".pyxis/skills/reviewer/SKILL.md") || !strings.Contains(pre.Findings[0], "docgate-resolve.sh") {
		t.Fatalf("findings = %v, want exactly the skill naming the resolver (the delegate file itself is exempt)", pre.Findings)
	}

	// Remove the referencing skill; the bare delegate alone packs green.
	gitRun(t, repo, "rm", "-q", ".pyxis/skills/reviewer/SKILL.md")
	gitRun(t, repo, "commit", "-q", "-m", "sweep", "--no-gpg-sign")
	if _, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: fixtureBinaries(t),
	}); err != nil {
		t.Fatalf("pack with only the delegate present = %v, want green", err)
	}
}

func TestCheckRetiredResolversReportsMissingStage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if err := release.CheckRetiredResolvers(missing); err == nil {
		t.Fatal("missing staged root was accepted")
	}
}

// The source-mode desk hooks (.pyxis/hooks/**) are source-repo surface excluded
// from the operand payload, so a resolver reference THERE never reaches the
// precondition — an operand runs the .pyxis/dist/operand-hooks/ variant, never
// these. A repo carrying a resolver-naming desk hook still packs green, and no
// .pyxis/hooks/ path lands in the manifest.
func TestPackExcludesSourceDeskHooks(t *testing.T) {
	repo := fixtureRepo(t)
	writeFile(t, repo, ".pyxis/hooks/pre-push", 0o755, "#!/bin/sh\nexec \"$(dirname \"$0\")/../scripts/docgate-resolve.sh\" pre-push\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "desk hook", "--no-gpg-sign")

	res, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: fixtureBinaries(t),
	})
	if err != nil {
		t.Fatalf("pack with a source desk hook present = %v, want green (desk hooks excluded from payload)", err)
	}
	for _, f := range res.Manifest.Payload.Files {
		if strings.HasPrefix(f.Path, ".pyxis/hooks/") {
			t.Errorf("source desk hook %s leaked into the payload manifest", f.Path)
		}
	}
}

// Precondition (b): a source-toolchain step in the staged operand workflow
// refuses the pack by name.
func TestPackRefusesToolchainWorkflowStep(t *testing.T) {
	repo := fixtureRepo(t)
	writeFile(t, repo, ".pyxis/dist/github-workflows/docs-gate.yml", 0o644,
		"name: docs-gate\njobs:\n  g:\n    steps:\n      - uses: actions/setup-go@v5\n      - run: go vet ./...\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "bad workflow", "--no-gpg-sign")

	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: fixtureBinaries(t),
	})
	var pre *release.PreconditionError
	if !errors.As(err, &pre) || pre.Name != "workflow-toolchain-steps" {
		t.Fatalf("pack = %v, want the workflow-toolchain-steps precondition", err)
	}
	joined := strings.Join(pre.Findings, "\n")
	for _, want := range []string{".github/workflows/docs-gate.yml", "setup-go", "go vet"} {
		if !strings.Contains(joined, want) {
			t.Errorf("findings %q missing %q", joined, want)
		}
	}
}

// Precondition (d): an unclassed staged file fails the pack via the walk
// (DINV-6, A-11) naming the file.
func TestPackRefusesUnclassedFile(t *testing.T) {
	repo := fixtureRepo(t)
	// Drop the workflow row: the overlay-mapped file becomes unclassed.
	writeFile(t, repo, ".pyxis/class-policy.yaml", 0o644, strings.Replace(fixturePolicy,
		"  - { glob: \".github/workflows/docs-gate.yml\", class: generated }\n", "", 1))
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "narrow policy", "--no-gpg-sign")

	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(),
		BinariesDir: fixtureBinaries(t),
	})
	var unclassed *relmanifest.UnclassedError
	if !errors.As(err, &unclassed) || !slices.Contains(unclassed.Paths, ".github/workflows/docs-gate.yml") {
		t.Fatalf("pack = %v, want the DINV-6 unclassed refusal naming the workflow", err)
	}
}

// A partial or padded binary matrix refuses by name (A-1: every scan-derived
// tool × platform, exactly).
func TestPackRefusesIncompleteOrForeignBinaries(t *testing.T) {
	repo := fixtureRepo(t)

	missing := fixtureBinaries(t)
	if err := os.Remove(filepath.Join(missing, "toolb-"+fixtureVersion+"-linux-arm64")); err != nil {
		t.Fatal(err)
	}
	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(), BinariesDir: missing,
	})
	if err == nil || !strings.Contains(err.Error(), "missing binary toolb-"+fixtureVersion+"-linux-arm64") {
		t.Fatalf("pack with a missing platform binary = %v, want it named", err)
	}

	foreign := fixtureBinaries(t)
	writeFile(t, foreign, release.BuildOnlyTool+"-"+fixtureVersion+"-linux-amd64", 0o755, "the packer itself\n")
	_, err = release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(), BinariesDir: foreign,
	})
	if err == nil || !strings.Contains(err.Error(), "outside the scan-derived shipped set") {
		t.Fatalf("pack with the build-only tool in the matrix = %v, want the shipped-set refusal", err)
	}

	stray := fixtureBinaries(t)
	writeFile(t, stray, "notes.txt", 0o644, "stray\n")
	_, err = release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(), BinariesDir: stray,
	})
	if err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Fatalf("pack with a stray file in the matrix = %v, want it named", err)
	}
}

// The version grammar is fail-closed before any filesystem work.
func TestPackRefusesBadVersion(t *testing.T) {
	_, err := release.Pack(context.Background(), release.PackOptions{
		Version: "1.2.3", OutDir: t.TempDir(), BinariesDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "not a system tag") {
		t.Fatalf("pack with a tagless version = %v", err)
	}
}

// Packaging needs only Git; no JavaScript runtime or dependency installer runs.
func TestPackWithoutNodeOrNpm(t *testing.T) {
	repo := fixtureRepo(t)
	bins := fixtureBinaries(t)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	pathDir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(pathDir, "git")); err != nil {
		t.Skipf("cannot symlink git: %v", err)
	}
	t.Setenv("PATH", pathDir)

	_, err = release.Pack(context.Background(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: t.TempDir(), BinariesDir: bins,
	})
	if err != nil {
		t.Fatalf("pack without Node or npm: %v", err)
	}
}

// The LIVE source tree now PASSES the retired-resolver precondition — the
// resolver cutover has landed: the source-mode desk hooks (.pyxis/hooks/**)
// are excluded from the operand payload (the operand runs the
// .pyxis/dist/operand-hooks/ variant, which calls the pinned pyxis binary), and
// shipped skills locate the front door without naming resolver scripts.
// No staged file references a retired per-tool resolver.
// The staged overlay workflow is toolchain-free. This is the green gate the
// first real release requires (installer-v1 §3.10).
func TestLiveTreePayloadPreconditions(t *testing.T) {
	needGit(t)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis", "class-policy.yaml")); err != nil {
		t.Fatalf("repo root not found: %v", err)
	}
	staged := t.TempDir()
	if _, err := release.StagePayload(context.Background(), root, staged); err != nil {
		t.Fatalf("stage the live payload: %v", err)
	}

	// Retired-resolver precondition: GREEN — no staged file names a retired
	// per-tool resolver script (the cutover retired every payload call site).
	if err := release.CheckRetiredResolvers(staged); err != nil {
		t.Errorf("live tree fails the retired-resolver precondition — a staged file still names a resolver:\n%v", err)
	}
	// The source-mode desk hooks are source-repo surface, never staged into the
	// operand payload (an operand has no engine source to `go run`).
	if fileExists(filepath.Join(staged, ".pyxis", "hooks", "pre-push")) {
		t.Errorf("the source-mode desk hooks (.pyxis/hooks/) leaked into the operand payload")
	}
	// Interactive shell setup belongs to the machine installation, never to
	// the managed repository payload. Check its former and current homes.
	for _, rel := range []string{".pyxis/scripts/startup", "script/startup", "scripts/startup"} {
		if fileExists(filepath.Join(staged, filepath.FromSlash(rel))) {
			t.Errorf("machine shell setup %s leaked into the operand payload", rel)
		}
	}

	// The staged operand workflow set is clean of toolchain steps (the T9
	// overlay variant), and the staged tree walks fully classed.
	if err := release.CheckWorkflowToolchainFree(staged); err != nil {
		t.Errorf("live staged workflow set = %v, want toolchain-free", err)
	}
	if got := filepath.Join(staged, ".github", "workflows", "docs-gate.yml"); !fileExists(got) {
		t.Errorf("overlay mapping did not stage the operand workflow")
	}
	if fileExists(filepath.Join(staged, ".github", "workflows", "README.md")) {
		t.Errorf("the overlay README leaked into the operand workflow dir")
	}
}

func TestLiveTreeStagesAuthoringContractsForInstalledRepos(t *testing.T) {
	needGit(t)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourceFiles := make(map[string][]byte)
	for _, rel := range []string{
		".pyxis/authoring/plan-new.md",
		".pyxis/authoring/prd-new.md",
		".pyxis/authoring/incident.md",
		".pyxis/model-routing.md",
		".pyxis/templates/prd-amendment-template.md",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		sourceFiles[rel] = content
	}
	if !bytes.Contains(sourceFiles[".pyxis/authoring/plan-new.md"], []byte("go run ./cmd/plangate precheck")) ||
		!bytes.Contains(sourceFiles[".pyxis/authoring/plan-new.md"], []byte("`docs/tech-design/plan-new-v1.md`")) {
		t.Fatal("source plan guide must keep runnable source commands and source contract paths")
	}
	if !bytes.Contains(sourceFiles[".pyxis/model-routing.md"], []byte("go run ./cmd/plangate routes")) ||
		!bytes.Contains(sourceFiles[".pyxis/templates/prd-amendment-template.md"], []byte("go run ./cmd/docgate fold")) {
		t.Fatal("transitive source guides must keep runnable source commands")
	}
	if !bytes.Contains(sourceFiles[".pyxis/authoring/prd-new.md"], []byte("docs/templates/prd-template.md")) ||
		!bytes.Contains(sourceFiles[".pyxis/authoring/incident.md"], []byte("docgate graduate")) {
		t.Fatal("source guides must keep their source template path and bare source commands")
	}
	staged := t.TempDir()
	if _, err := release.StagePayload(t.Context(), root, staged); err != nil {
		t.Fatal(err)
	}
	for rel, before := range sourceFiles {
		after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("release staging changed source file %s", rel)
		}
	}
	for _, ref := range []string{
		"docs/tech-design/argus-artifacts.md",
		"docs/tech-design/argus-agent-v1.md",
		"docs/tech-design/argus-draft-attribution.md",
		"docs/tech-design/brainstorm-record-model.md",
		"docs/tech-design/prd-amendment-model.md",
		"docs/tech-design/routing-envelope-v1.md",
		"docs/prds/argus/effective.md",
		"docs/prds/plan-new/effective.md",
	} {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref)))
		if err != nil {
			t.Fatal(err)
		}
		installed, err := os.ReadFile(filepath.Join(staged, ".pyxis", "reference", filepath.FromSlash(ref)))
		if err != nil {
			t.Fatalf("installed authoring contract %s is missing: %v", ref, err)
		}
		if !bytes.Equal(installed, source) {
			t.Errorf("installed authoring contract %s differs from its source", ref)
		}
	}
	guide, err := os.ReadFile(filepath.Join(staged, ".pyxis", "authoring", "argus.md"))
	if err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string][]string{
		".pyxis/authoring/argus.md": {
			"pyxis draft status <feature>",
			"pyxis draft prepare-lock <feature>",
			".pyxis/reference/docs/tech-design/argus-artifacts.md",
			".pyxis/reference/docs/prds/argus/effective.md",
			"ARGUS_DRIVE_SLUG=<this problem's slug>",
			"Argus-Drive",
			"Argus-Bind",
			"audit attribution",
			"Do not export `ARGUS_DRIVE_SLUG` for the whole session",
			"contract-record commit in a temporary worktree",
			"Every pushed round and the exact prepared head must pass attribution",
			"verifies the source bindings before refreshing them",
			"git reset --soft HEAD^",
			"git commit -C ORIG_HEAD",
			"use `git commit --amend` for a drive commit",
			"Do not create drive-authored merge commits",
			".pyxis/bin:$PATH",
			".pyxis/reference/docs/tech-design/argus-draft-attribution.md",
		},
		".pyxis/authoring/plan-new.md": {
			"pyxis exec plangate -- precheck",
			".pyxis/reference/docs/tech-design/plan-new-v1.md",
			".pyxis/reference/docs/prds/plan-new/effective.md",
			"docs/prds/<slug>/effective.md",
		},
		".pyxis/authoring/implement-now.md": {
			"pyxis watch --repo <checkout>",
			"pyxis exec implement-now -- --repo <checkout>",
			"pyxis exec plangate -- report docs/plans/<feature>",
			"pyxis exec feature-drive -- dogfood",
		},
		".pyxis/authoring/prd-new.md": {
			".pyxis/templates/prd-template.md",
			"pyxis exec docgate -- gate",
		},
		".pyxis/authoring/tech-design-new.md": {
			".pyxis/templates/tech-design-template.md",
		},
		".pyxis/authoring/prd-amend.md": {
			".pyxis/templates/prd-amendment-template.md",
			"pyxis exec docgate -- lint-structure",
		},
		".pyxis/authoring/incident.md": {
			"pyxis exec docgate -- add-incident",
			"pyxis exec docgate -- graduate --id <n>",
		},
		".pyxis/authoring/prd-review.md": {
			"pyxis exec docgate -- context-pack --stage",
		},
		".pyxis/authoring/tech-design-review.md": {
			"pyxis exec docgate -- context-pack --stage",
		},
		".pyxis/skills/impl-review/SKILL.md": {
			".pyxis/reference/docs/tech-design/impl-review-v1.md",
			".pyxis/reference/docs/prds/impl-review/effective.md",
		},
		".pyxis/skills/argus/SKILL.md": {
			".pyxis/reference/docs/tech-design/argus-artifacts.md",
		},
		".pyxis/model-routing.md": {
			"pyxis exec plangate -- routes",
			"pyxis exec carina -- create --native-task",
			"pyxis exec carina -- signal add <id> agent",
			"../.pyxis/reference/docs/tech-design/routing-envelope-v1.md",
		},
		".pyxis/templates/prd-amendment-template.md": {
			"pyxis exec docgate -- fold docs/prds/<feature>",
			"pyxis exec docgate -- verify-generated",
			".pyxis/reference/docs/tech-design/prd-amendment-model.md",
		},
		".pyxis/templates/plan-template.md": {
			"pyxis exec plangate -- fold",
			"pyxis exec plangate -- dimensions",
		},
		".pyxis/pipeline.md": {
			"pyxis exec docgate -- contract-gate",
		},
	} {
		content, err := os.ReadFile(filepath.Join(staged, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range want {
			if !bytes.Contains(content, []byte(value)) {
				t.Errorf("installed %s lacks %q", rel, value)
			}
		}
		if bytes.Contains(content, []byte("go run ./cmd/")) {
			t.Errorf("installed %s still carries a source-only command", rel)
		}
	}
	if bytes.Contains(guide, []byte("`docs/tech-design/argus-artifacts.md`")) {
		t.Error("installed Argus guide still points at the absent source design")
	}
}

func TestStagePayloadRefusesMissingAuthoringContract(t *testing.T) {
	dir := fixtureRepo(t)
	writeFile(t, dir, ".pyxis/authoring/example.md", 0o644, "Read `docs/tech-design/missing.md` before acting.\n")
	gitRun(t, dir, "add", ".pyxis/authoring/example.md")
	if _, err := release.StagePayload(t.Context(), dir, t.TempDir()); err == nil || !strings.Contains(err.Error(), "authoring reference docs/tech-design/missing.md") {
		t.Fatalf("missing source contract should refuse the release, got %v", err)
	}
}

func TestStagePayloadRefusesMissingAuthoringTemplate(t *testing.T) {
	dir := fixtureRepo(t)
	writeFile(t, dir, ".pyxis/authoring/example.md", 0o644, "Read `docs/templates/missing.md` before acting.\n")
	gitRun(t, dir, "add", ".pyxis/authoring/example.md")
	if _, err := release.StagePayload(t.Context(), dir, t.TempDir()); err == nil || !strings.Contains(err.Error(), "authoring template .pyxis/templates/missing.md is missing") {
		t.Fatalf("missing shipped template should refuse the release, got %v", err)
	}
}

// StagePayload excludes engine-internal test material — the bash shim tests,
// the render/skill fixtures, and any Go testdata — while keeping the runtime
// inputs that live beside them (a skill's SKILL.md, its calibration.md). An
// operand runs the skills, never the engine's tests, and no skill reads its
// fixtures/ at runtime.
func TestStagePayloadExcludesDevOnlyMaterial(t *testing.T) {
	needGit(t)
	dir := t.TempDir()
	// Shipped runtime inputs, and dev-only material beside them.
	writeFile(t, dir, ".pyxis/dist/github-workflows/docs-gate.yml", 0o644, fixtureOverlay)
	writeFile(t, dir, ".pyxis/skills/argus/SKILL.md", 0o644, "# skill\n")
	writeFile(t, dir, ".pyxis/skills/argus/calibration.md", 0o644, "# calibration\n")
	writeFile(t, dir, ".pyxis/skills/argus/authority-contract.md", 0o644, "Source operator promotion — never distribute\n")
	writeFile(t, dir, ".pyxis/templates/authority-contract-template.md", 0o644, "# authoring template\n")
	writeFile(t, dir, ".pyxis/skills/argus/fixtures/eval-1.md", 0o644, "dev-only fixture\n")
	writeFile(t, dir, ".pyxis/ui/fixtures/sample.json", 0o644, "{}\n")
	writeFile(t, dir, ".pyxis/scripts/example.mjs", 0o644, "// renderer\n")
	writeFile(t, dir, ".pyxis/scripts/tests/test-example.mjs", 0o644, "// shim test\n")
	writeFile(t, dir, ".pyxis/testdata/golden.txt", 0o644, "golden\n")
	writeFile(t, dir, ".pyxis/logs/executor/events.jsonl", 0o600, "private execution output\n")
	writeFile(t, dir, ".pyxis/evidence/memory/failure.json", 0o644, "{\"historical_path\":\"docgate-resolve.sh\"}\n")
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "seed", "--no-gpg-sign")

	staged, err := release.StagePayload(context.Background(), dir, t.TempDir())
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	got := map[string]bool{}
	for _, p := range staged {
		got[p] = true
	}
	for _, keep := range []string{
		".pyxis/skills/argus/SKILL.md",
		".pyxis/skills/argus/calibration.md",
		".pyxis/templates/authority-contract-template.md",
		".pyxis/scripts/example.mjs",
	} {
		if !got[keep] {
			t.Errorf("staged set dropped a runtime input %q\nstaged: %v", keep, staged)
		}
	}
	for _, drop := range []string{
		".pyxis/skills/argus/fixtures/eval-1.md",
		".pyxis/skills/argus/authority-contract.md",
		".pyxis/ui/fixtures/sample.json",
		".pyxis/scripts/tests/test-example.mjs",
		".pyxis/testdata/golden.txt",
		".pyxis/logs/executor/events.jsonl",
		".pyxis/evidence/memory/failure.json",
	} {
		if got[drop] {
			t.Errorf("staged set shipped dev-only material %q\nstaged: %v", drop, staged)
		}
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Legacy auxiliary artifacts remain hash-verified, but cannot leak into a new pack.
func TestLegacyDependencyBundleVerification(t *testing.T) {
	repo := fixtureRepo(t)
	out, res := packFixture(t, repo)
	name := "deps-" + fixtureVersion + ".tar.gz"
	bundle := filepath.Join(out, name)
	writeEmptyTarGz(t, bundle)
	if _, err := release.ExportedWriteSums(out); err != nil {
		t.Fatal(err)
	}
	if err := release.Verify(out, fixtureVersion); err == nil || !strings.Contains(err.Error(), "unexpected dependency bundle") {
		t.Fatalf("unpinned bundle accepted: %v", err)
	}
	if _, err := release.Pack(t.Context(), release.PackOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: out, BinariesDir: fixtureBinaries(t),
	}); err == nil || !strings.Contains(err.Error(), name) {
		t.Fatalf("stale bundle accepted by new pack: %v", err)
	}
	res.Manifest.SchemaVersion = "release-manifest/v1"
	res.Manifest.DepsBundleSHA256 = sumOf(t, out, name)
	manifest, err := res.Manifest.Encode()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, out, "release-manifest-"+fixtureVersion+".json", 0o644, string(manifest))
	if _, err := release.ExportedWriteSums(out); err != nil {
		t.Fatal(err)
	}
	if err := release.Verify(out, fixtureVersion); err != nil {
		t.Fatalf("legacy release refused: %v", err)
	}
	writeFile(t, out, name, 0o644, "tampered archive")
	if _, err := release.ExportedWriteSums(out); err != nil {
		t.Fatal(err)
	}
	if err := release.Verify(out, fixtureVersion); err == nil || !strings.Contains(err.Error(), "deps_bundle_sha256") {
		t.Fatalf("tampered legacy bundle accepted despite manifest pin: %v", err)
	}
}

func writeEmptyTarGz(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("close empty archive: %v", err)
		}
	}()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}
