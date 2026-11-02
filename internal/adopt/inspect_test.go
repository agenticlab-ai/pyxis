package adopt_test

// Hermetic tests for Inspect's tree-state dispatch (UB-5, UB-1, UB-7) and
// its classify+assemble pass over a non-empty repository (FR-1, FR-2): a
// complete fixture release served over file://, a target tree built per
// test, and byte-identity assertions over everything the run did not act
// on. The empty-tree route-to-init leg is exercised separately, over the
// unexported installerInitFn seam (internal_test.go, package adopt) — it
// needs no release fixture there, since Inspect never reaches converge for
// that leg.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// ---- fixture scaffolding ----------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// git runs one git subcommand in dir for fixture setup, failing the test on
// error — test scaffolding only, distinct from Inspect's own read-only
// probes.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// isolateGit hermetically severs the test from the real developer identity
// and any real ~/.gitconfig (installer_test.go's own pattern).
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Fixture Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "fixture@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Fixture Author")
	t.Setenv("GIT_COMMITTER_EMAIL", "fixture@example.com")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-02T03:04:05Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T03:04:05Z")
	t.Setenv("HOME", t.TempDir())
}

// pf is one fixture release payload file.
type pf struct {
	path  string
	b     []byte
	class relmanifest.Class
}

// adoptFiles is a minimal manifest exercising every class T3's classify pass
// maps: repo-owned (missing → add), engine-verbatim, and generated via the
// docgate index generator (never the installed-workflow path, which needs
// archive access Drift's read-only face never has — classify.go's own
// documented asymmetry).
func adoptFiles() []pf {
	return []pf{
		{path: ".pyxis/config.yaml", b: []byte("# packed source config; never extracted\n"), class: relmanifest.RepoOwned},
		{path: ".pyxis/templates/prd-template.md", b: []byte("# PRD template\nbody\n"), class: relmanifest.EngineVerbatim},
		{path: "docs/tech-design/index.md", b: []byte("stale packed index\n"), class: relmanifest.Generated},
		{path: "docs/tech-design/index.json", b: []byte("{}\n"), class: relmanifest.Generated},
	}
}

// release is a fixture release directory a file:// source serves — the
// manifest only; Drift never fetches the payload archive.
type release struct {
	src string
	ver string
	pin string
}

func newRelease(t *testing.T, files []pf) release {
	t.Helper()
	const ver = "v0.5.0"
	src := t.TempDir()
	man := relmanifest.Manifest{
		SchemaVersion:    relmanifest.ManifestSchemaVersion,
		SystemVersion:    ver,
		SourceCommit:     strings.Repeat("cd", 20),
		Binaries:         map[string]map[string]string{},
		Payload:          relmanifest.Payload{ArchiveSHA256: hexOf([]byte("never fetched by Drift"))},
		DepsBundleSHA256: strings.Repeat("ab", 32),
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
	writeFile(t, filepath.Join(src, "release-manifest-"+ver+".json"), mb)
	return release{src: src, ver: ver, pin: hexOf(mb)}
}

func (r release) installerPin() installer.Pin {
	return installer.Pin{Version: r.ver, ManifestSHA256: r.pin}
}

// nonEmptyAdopterRepo builds a git repository carrying the adopter's OWN
// pre-existing, committed content — a top-level directory (never-touch
// label "src/**") and a top-level file (label "README.md") — with none of
// adoptFiles()'s manifest paths present, the ordinary first-time-adopt
// shape.
func nonEmptyAdopterRepo(t *testing.T) string {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	writeFile(t, filepath.Join(root, "README.md"), []byte("# my project\n"))
	writeFile(t, filepath.Join(root, "src", "main.go"), []byte("package main\n\nfunc main() {}\n"))
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "initial")
	return root
}

// snapshot hashes every regular file under root, keyed by slash-relative
// path, EXCLUDING .git and .pyxis/bin. .git is excluded because
// `git status`/`ls-files` (Inspect's own dispatch probes) may rewrite
// .git/index's stat-cache bytes without any logical change, which would
// otherwise read as a false-positive mutation. .pyxis/bin is resolve's
// gitignored manifest/binary cache (resolve.Config.cacheDir): classifying a
// repository whose cache is cold fetch-verifies the pinned manifest into it
// exactly as converge.Drift's own doc comment discloses ("a cold cache
// fetch-verifies into the gitignored cache, resolve's own contract") —
// converge's own suite proves the same point by pre-warming the cache before
// asserting Drift is byte-identical (TestDriftClassifiesWithoutMutating).
// That cache directory is never git-tracked and is excluded from FR-1's
// tracked-path byte-identity claim for the same reason .git is: it is the
// engine's own bookkeeping, never the repository's content.
func snapshot(t *testing.T, root string) map[string]string {
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
		if rel == "." {
			return nil
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git/") || rel == ".pyxis/bin" || strings.HasPrefix(rel, ".pyxis/bin/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		out[rel] = hexOf(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// ---- UB-5: non-git target refuses by name, writes nothing ------------------

func TestInspectRefusesNonGitTarget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), []byte("hello\n"))
	before := snapshot(t, root)

	_, err := adopt.Inspect(context.Background(), adopt.Options{
		Target: root,
		Pin:    installer.Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("ab", 32)},
	})
	if !errors.Is(err, adopt.ErrNotGitRepository) {
		t.Fatalf("err = %v, want ErrNotGitRepository", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatalf("a refused adopt mutated the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- UB-7: a malformed prior config refuses by name, writes nothing --------

func TestInspectRefusesMalformedPriorConfig(t *testing.T) {
	root := nonEmptyAdopterRepo(t)
	// A prior .pyxis/config.yaml this closed schema does not parse (an
	// unknown top-level SECTION) — simulating a partial/interrupted prior
	// adoption attempt, or a hand-corrupted config.
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte("bogus_section:\n  key: value\n"))
	before := snapshot(t, root)

	_, err := adopt.Inspect(context.Background(), adopt.Options{
		Target: root,
		Pin:    installer.Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("ab", 32)},
	})
	if !errors.Is(err, adopt.ErrConfigSurfaceInvalid) {
		t.Fatalf("err = %v, want ErrConfigSurfaceInvalid", err)
	}
	if !strings.Contains(err.Error(), "bogus_section") {
		t.Errorf("refusal does not name the offending key:\n%v", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatalf("a refused adopt mutated the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- the pin is required only once dispatch needs it -----------------------

func TestInspectRefusesMissingPinOnNonEmptyRepo(t *testing.T) {
	root := nonEmptyAdopterRepo(t)
	before := snapshot(t, root)

	_, err := adopt.Inspect(context.Background(), adopt.Options{Target: root})
	if !errors.Is(err, adopt.ErrPinRequired) {
		t.Fatalf("err = %v, want ErrPinRequired", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatalf("a refused adopt mutated the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- FR-1/FR-2: classify a non-empty repo and write ONLY the draft plan ----

func TestInspectClassifiesNonEmptyRepoAndWritesOnlyTheDraftPlan(t *testing.T) {
	files := adoptFiles()
	r := newRelease(t, files)
	root := nonEmptyAdopterRepo(t)
	before := snapshot(t, root)

	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if rep.RoutedToInit {
		t.Fatalf("RoutedToInit = true, want false for a non-empty repository")
	}
	if rep.PlanPath != adopt.PlanPath {
		t.Errorf("PlanPath = %q, want %q", rep.PlanPath, adopt.PlanPath)
	}
	if rep.Plan.SchemaVersion != adopt.SchemaVersion {
		t.Errorf("Plan.SchemaVersion = %q, want %q", rep.Plan.SchemaVersion, adopt.SchemaVersion)
	}
	if rep.Plan.Status != adopt.StatusDraft {
		t.Errorf("Plan.Status = %q, want draft", rep.Plan.Status)
	}

	want := map[string]adopt.Class{
		".pyxis/config.yaml":               adopt.ClassAdd,
		".pyxis/templates/prd-template.md": adopt.ClassEngineVerbatim,
		"docs/tech-design/index.md":        adopt.ClassGenerated,
		"docs/tech-design/index.json":      adopt.ClassGenerated,
		"src/**":                           adopt.ClassNeverTouch,
		"README.md":                        adopt.ClassNeverTouch,
	}
	if len(rep.Plan.Entries) != len(want) {
		t.Fatalf("Entries = %+v, want exactly %d entries matching %v", rep.Plan.Entries, len(want), want)
	}
	for _, e := range rep.Plan.Entries {
		wc, ok := want[e.Path]
		if !ok {
			t.Errorf("unexpected entry %q (class %s)", e.Path, e.Class)
			continue
		}
		if e.Class != wc {
			t.Errorf("entry %q: class = %s, want %s", e.Path, e.Class, wc)
		}
	}
	// Entries render in the producer's own deterministic order (path-sorted).
	for i := 1; i < len(rep.Plan.Entries); i++ {
		if rep.Plan.Entries[i-1].Path > rep.Plan.Entries[i].Path {
			t.Errorf("Entries not path-sorted: %q before %q", rep.Plan.Entries[i-1].Path, rep.Plan.Entries[i].Path)
		}
	}

	// The written plan file round-trips to the exact same Plan the Report carries.
	planAbs := filepath.Join(root, filepath.FromSlash(adopt.PlanPath))
	b, err := os.ReadFile(planAbs)
	if err != nil {
		t.Fatalf("read the written plan: %v", err)
	}
	got, err := adopt.Parse(b)
	if err != nil {
		t.Fatalf("parse the written plan: %v", err)
	}
	if got.Status != rep.Plan.Status || len(got.Entries) != len(rep.Plan.Entries) {
		t.Errorf("the written plan diverges from Report.Plan: %+v vs %+v", got, rep.Plan)
	}

	// FR-1: every tracked-namespace path that existed before Inspect ran is
	// byte-identical after; the ONLY new path is the plan itself (the
	// resolve manifest cache is excluded from snapshot — see its doc comment).
	after := snapshot(t, root)
	for path, sum := range before {
		got, ok := after[path]
		if !ok {
			t.Errorf("pre-existing path %q vanished after adopt", path)
			continue
		}
		if got != sum {
			t.Errorf("pre-existing path %q changed after adopt (never-touch violated)", path)
		}
	}
	var newPaths []string
	for path := range after {
		if _, ok := before[path]; !ok {
			newPaths = append(newPaths, path)
		}
	}
	sort.Strings(newPaths)
	if len(newPaths) != 1 || newPaths[0] != adopt.PlanPath {
		t.Errorf("new paths after adopt = %v, want exactly [%s]", newPaths, adopt.PlanPath)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// FR-6 / §3.3: an existing root convention file is DISCLOSED in the draft plan
// as a collide-manual entry carrying the human's instruction lines — not left
// as a silent never-touch entry.
func TestInspectDisclosesRootConventionCollisionAsCollideManual(t *testing.T) {
	r := newRelease(t, adoptFiles())
	root := nonEmptyAdopterRepo(t)
	writeFile(t, filepath.Join(root, "AGENTS.md"), []byte("# their agents manual\n"))
	git(t, root, "add", "AGENTS.md")
	git(t, root, "commit", "-m", "their root convention file")

	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	var agents *adopt.Entry
	n := 0
	for i := range rep.Plan.Entries {
		if rep.Plan.Entries[i].Path == "AGENTS.md" {
			agents = &rep.Plan.Entries[i]
			n++
		}
	}
	if agents == nil {
		t.Fatal("the draft plan carries no entry for the existing AGENTS.md")
	}
	if n != 1 {
		t.Errorf("AGENTS.md appears in %d entries, want exactly 1 (its never-touch entry replaced, not duplicated)", n)
	}
	if agents.Class != adopt.ClassCollideManual {
		t.Errorf("AGENTS.md class = %s, want %s (disclosed, not a silent never-touch)", agents.Class, adopt.ClassCollideManual)
	}
	if agents.Detail == "" {
		t.Error("the collide-manual AGENTS.md entry carries no instruction lines for the human (FR-6)")
	}
}

// FR-4 / §3.3: a repository already setting core.hooksPath has the hook chain
// DISCLOSED in the draft plan as a chain entry naming the prior path — and,
// per the reconciled DINV-7, it carries no added_paths baseline (the git-config
// chain is reversed via hooks.prior_hooks_path, not a file deletion).
func TestInspectDisclosesHookChainWhenCoreHooksPathSet(t *testing.T) {
	r := newRelease(t, adoptFiles())

	// No prior hook path → no chain disclosed.
	plain := nonEmptyAdopterRepo(t)
	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target: plain, Pin: r.installerPin(), ArtifactSource: "file://" + r.src, Logger: quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect (no hook path): %v", err)
	}
	for _, e := range rep.Plan.Entries {
		if e.Class == adopt.ClassChain {
			t.Fatalf("chain entry disclosed for a repo with no core.hooksPath: %+v", e)
		}
	}

	// A hook manager owning core.hooksPath → the chain is disclosed.
	chained := nonEmptyAdopterRepo(t)
	git(t, chained, "config", "--local", "core.hooksPath", ".husky")
	rep, err = adopt.Inspect(context.Background(), adopt.Options{
		Target: chained, Pin: r.installerPin(), ArtifactSource: "file://" + r.src, Logger: quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect (chained): %v", err)
	}
	var chain *adopt.Entry
	for i := range rep.Plan.Entries {
		if rep.Plan.Entries[i].Class == adopt.ClassChain {
			chain = &rep.Plan.Entries[i]
		}
	}
	if chain == nil {
		t.Fatal("no chain entry disclosed for a repo with core.hooksPath set")
	}
	if !strings.Contains(chain.Detail, ".husky") {
		t.Errorf("chain entry does not name the prior path .husky for the reviewer: %q", chain.Detail)
	}
	// The draft carries no added_paths — the chain needs no baseline (reconciled DINV-7).
	if len(rep.Plan.AddedPaths) != 0 {
		t.Errorf("draft plan carries added_paths %v; the chain needs no baseline and a draft stamps none", rep.Plan.AddedPaths)
	}
}

// FR-7 / §3.6: inspect detects the stack from marker files and surfaces it in
// the draft plan (name + gate actions), rather than always rendering "unknown".
func TestInspectDetectsStackIntoThePlan(t *testing.T) {
	r := newRelease(t, adoptFiles())

	// A marker-less repo detects no stack — the degrade-to-unknown case (UB-3).
	plain := nonEmptyAdopterRepo(t)
	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target: plain, Pin: r.installerPin(), ArtifactSource: "file://" + r.src, Logger: quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect (no markers): %v", err)
	}
	if len(rep.Plan.Stack.Detected) != 0 {
		t.Errorf("no marker present, want empty Detected, got %v", rep.Plan.Stack.Detected)
	}

	// A Python marker → the stack is detected and its gate actions surfaced.
	py := nonEmptyAdopterRepo(t)
	writeFile(t, filepath.Join(py, "pyproject.toml"), []byte("[project]\nname = \"svc\"\n"))
	git(t, py, "add", "pyproject.toml")
	git(t, py, "commit", "-m", "python project")
	rep, err = adopt.Inspect(context.Background(), adopt.Options{
		Target: py, Pin: r.installerPin(), ArtifactSource: "file://" + r.src, Logger: quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect (python): %v", err)
	}
	found := false
	for _, s := range rep.Plan.Stack.Detected {
		if s == "python" {
			found = true
		}
	}
	if !found {
		t.Errorf("plan.Stack.Detected = %v, want it to include python", rep.Plan.Stack.Detected)
	}
	if len(rep.Plan.Stack.GateActions) == 0 {
		t.Error("a detected stack must surface the gate-command actions the adopter supplies (FR-7)")
	}
}
