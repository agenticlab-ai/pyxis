package adopt_test

// Hermetic tests for Apply's orchestration (existing-repo-onboarding/001 T8;
// prd:existing-repo-onboarding/FR-3, FR-8, FR-11, UB-2): a real inspect→apply
// round trip over a fixture release that serves BOTH the manifest and a
// verified payload archive (apply's ClassifyWithArchive fetch-verifies the
// archive, unlike Drift's read-only face), asserting the four T8 acceptance
// criteria — the plan materialises, the epoch records the current commit
// without a new commit, an absent/stale plan refuses and changes nothing, and
// every added path carries a content baseline — plus the never-touch
// byte-identity that is adoption's core trust guarantee (ABUSE-1). It reuses
// inspect_test.go's fixture scaffolding (hexOf, writeFile, git, isolateGit,
// pf, snapshot, nonEmptyAdopterRepo, mapsEqual) and adds only the archive.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// tarGzAdopt packs the fixture payload archive — every file at 0644 (the adopt
// fixture needs no exec bit; RepairEngineVerbatim compares content, not mode).
func tarGzAdopt(t *testing.T, files []pf) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{Name: f.path, Mode: 0o644, Size: int64(len(f.b))}); err != nil {
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

// newReleaseWithArchive builds a fixture release serving the manifest AND a
// real payload archive whose sha256 the manifest pins — the shape apply needs
// (ClassifyWithArchive ensures the payload; inspect's archive-free newRelease
// does not). Drift (inspect's classifier) ignores the archive, so the same
// release drives both verbs.
func newReleaseWithArchive(t *testing.T, files []pf) release {
	t.Helper()
	const ver = "v0.5.0"
	src := t.TempDir()
	archive := tarGzAdopt(t, files)
	man := relmanifest.Manifest{
		SchemaVersion:    relmanifest.ManifestSchemaVersion,
		SystemVersion:    ver,
		SourceCommit:     strings.Repeat("cd", 20),
		Binaries:         map[string]map[string]string{},
		Payload:          relmanifest.Payload{ArchiveSHA256: hexOf(archive)},
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
	writeFile(t, filepath.Join(src, "payload-"+ver+".tar.gz"), archive)
	return release{src: src, ver: ver, pin: hexOf(mb)}
}

// draftPlan runs Inspect to draft the adoption plan against the release, the
// real stage-preceding-apply — so apply is tested against a plan the pipeline
// actually produces, never a hand-built fixture.
func draftPlan(t *testing.T, root string, r release) adopt.Report {
	t.Helper()
	rep, err := adopt.Inspect(context.Background(), adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	})
	if err != nil {
		t.Fatalf("Inspect (draft the plan): %v", err)
	}
	if rep.RoutedToInit {
		t.Fatal("RoutedToInit = true, want a drafted plan for a non-empty repo")
	}
	return rep
}

func applyOpts(root string, r release) adopt.Options {
	return adopt.Options{
		Target:         root,
		Pin:            r.installerPin(),
		ArtifactSource: "file://" + r.src,
		Logger:         quiet(),
	}
}

func commitCount(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(git(t, root, "rev-list", "--count", "HEAD"))
}

func headSHA(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
}

// ---- FR-3 / FR-8 / FR-11: materialise, record the epoch, idempotent ---------

func TestApplyMaterialisesPlanRecordsEpochAndIsIdempotent(t *testing.T) {
	files := adoptFiles()
	r := newReleaseWithArchive(t, files)
	root := nonEmptyAdopterRepo(t)

	adopterBefore := snapshot(t, root) // the adopter's own committed content
	head := headSHA(t, root)
	commitsBefore := commitCount(t, root)

	draftPlan(t, root, r)
	rep, err := adopt.Apply(context.Background(), applyOpts(root, r))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// AC: the plan is stamped applied with the epoch = the current commit.
	if rep.Plan.Status != adopt.StatusApplied {
		t.Errorf("Plan.Status = %q, want applied", rep.Plan.Status)
	}
	if rep.Plan.Epoch != head {
		t.Errorf("Plan.Epoch = %q, want the current commit %q", rep.Plan.Epoch, head)
	}
	if rep.EpochCommit != head {
		t.Errorf("EpochCommit = %q, want %q", rep.EpochCommit, head)
	}

	// AC: apply creates NO commit of its own.
	if got := commitCount(t, root); got != commitsBefore {
		t.Errorf("commit count = %s after apply, want unchanged %s (apply must create no commit)", got, commitsBefore)
	}

	// AC: the intended tree — the engine-verbatim path is materialised
	// byte-exact from the archive, the generated index exists, the config is
	// established carrying the epoch.
	ev := find(t, files, ".pyxis/templates/prd-template.md")
	if got := readTree(t, root, ".pyxis/templates/prd-template.md"); !bytes.Equal(got, ev.b) {
		t.Errorf("engine-verbatim not materialised byte-exact:\n got %q\nwant %q", got, ev.b)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "tech-design", "index.md")); err != nil {
		t.Errorf("generated index.md not materialised: %v", err)
	}
	cfg := readTree(t, root, ".pyxis/config.yaml")
	if !strings.Contains(string(cfg), head) {
		t.Errorf("config does not record the epoch commit %q:\n%s", head, cfg)
	}

	// AC: every added path carries a content baseline (DINV-7). The config is
	// the fixture's sole add entry (repo-owned, missing → add).
	if len(rep.Plan.AddedPaths) != 1 || rep.Plan.AddedPaths[0].Path != ".pyxis/config.yaml" {
		t.Fatalf("AddedPaths = %+v, want exactly the config surface", rep.Plan.AddedPaths)
	}
	if want := hexOf(cfg); rep.Plan.AddedPaths[0].SHA256 != want {
		t.Errorf("config baseline = %q, want the materialised content hash %q", rep.Plan.AddedPaths[0].SHA256, want)
	}

	// The engine's hooks are activated (ordinary activation: no prior path).
	if got := strings.TrimSpace(git(t, root, "config", "--local", "--get", "core.hooksPath")); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks (engine hooks activated)", got)
	}

	// ABUSE-1 / FR-3: every path the adopter owned before apply is byte-identical.
	afterApply := snapshot(t, root)
	for path, sum := range adopterBefore {
		if afterApply[path] != sum {
			t.Errorf("never-touch violation: %q changed (%q → %q)", path, sum, afterApply[path])
		}
	}

	// FR-11: a second apply lands zero further changes.
	firstTree := snapshot(t, root)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	secondTree := snapshot(t, root)
	if !mapsEqual(firstTree, secondTree) {
		t.Errorf("re-apply was not idempotent:\nfirst  %v\nsecond %v", firstTree, secondTree)
	}
	if got := commitCount(t, root); got != commitsBefore {
		t.Errorf("re-apply created a commit: count %s, want %s", got, commitsBefore)
	}
}

// ---- UB-2: an absent plan refuses by name and changes nothing ---------------

func TestApplyRefusesAbsentPlan(t *testing.T) {
	r := newReleaseWithArchive(t, adoptFiles())
	root := nonEmptyAdopterRepo(t) // never inspected → no plan
	before := snapshot(t, root)

	_, err := adopt.Apply(context.Background(), applyOpts(root, r))
	if !errors.Is(err, adopt.ErrNoPlan) {
		t.Fatalf("err = %v, want ErrNoPlan", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatalf("a refused apply mutated the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- UB-2: a plan stale relative to the tree refuses and changes nothing ----

func TestApplyRefusesStalePlan(t *testing.T) {
	files := adoptFiles()
	r := newReleaseWithArchive(t, files)
	root := nonEmptyAdopterRepo(t)

	// Draft the plan, then drop the engine-verbatim entry so a divergent
	// manifest path (the missing template) is no longer named — the tree has
	// diverged from what the plan describes (a stand-in for any post-draft tree
	// change the reviewer never saw).
	rep := draftPlan(t, root, r)
	stale := rep.Plan
	kept := stale.Entries[:0]
	for _, e := range stale.Entries {
		if e.Path != ".pyxis/templates/prd-template.md" {
			kept = append(kept, e)
		}
	}
	stale.Entries = kept
	out, err := stale.Encode()
	if err != nil {
		t.Fatalf("re-encode the stale plan: %v", err)
	}
	writeFile(t, filepath.Join(root, filepath.FromSlash(adopt.PlanPath)), out)

	before := snapshot(t, root)
	_, err = adopt.Apply(context.Background(), applyOpts(root, r))
	if !errors.Is(err, adopt.ErrStalePlan) {
		t.Fatalf("err = %v, want ErrStalePlan", err)
	}
	if !strings.Contains(err.Error(), ".pyxis/templates/prd-template.md") {
		t.Errorf("stale refusal does not name the offending path:\n%v", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Fatalf("a refused (stale) apply mutated the tree:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- FR-4 / FR-5: apply orchestrates the collision resolutions --------------

// adoptFilesWithWorkflow adds the installed-workflow surface to the baseline
// fixture: the engine-verbatim template (payload.InstalledWorkflow — what
// desiredWorkflow renders from the archive) and the generated target
// (.github/workflows/docs-gate.yml). A token-less template renders to itself,
// so branch/remote substitution is a clean no-op here.
func adoptFilesWithWorkflow() []pf {
	return append(adoptFiles(),
		pf{path: ".pyxis/dist/github-workflows/docs-gate.yml", b: []byte("name: docs-gate\non: push\n"), class: relmanifest.EngineVerbatim},
		pf{path: ".github/workflows/docs-gate.yml", b: nil, class: relmanifest.Generated},
	)
}

func TestApplyRedirectsCollidingWorkflowAndChainsExistingHooks(t *testing.T) {
	files := adoptFilesWithWorkflow()
	r := newReleaseWithArchive(t, files)

	// An adopter repo carrying BOTH collisions at once: a pre-existing workflow
	// at the engine's fixed target, and a hook manager owning core.hooksPath.
	isolateGit(t)
	root := t.TempDir()
	git(t, root, "init", "-b", "main")
	writeFile(t, filepath.Join(root, "README.md"), []byte("# my project\n"))
	const adopterWorkflow = "name: my-own-ci\non: pull_request\n"
	writeFile(t, filepath.Join(root, ".github", "workflows", "docs-gate.yml"), []byte(adopterWorkflow))
	const huskyHook = "#!/bin/sh\necho adopter-hook\n"
	writeFile(t, filepath.Join(root, ".husky", "pre-push"), []byte(huskyHook))
	git(t, root, "add", "-A")
	git(t, root, "commit", "-m", "initial")
	git(t, root, "config", "--local", "core.hooksPath", ".husky")

	draftPlan(t, root, r)
	rep, err := adopt.Apply(context.Background(), applyOpts(root, r))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// FR-5: the engine's workflow lands under the distinct name; the adopter's
	// fixed-target workflow is byte-identical (never overwritten).
	distinct := adopt.DistinctWorkflowPath()
	if got := readTree(t, root, distinct); !bytes.Equal(got, []byte("name: docs-gate\non: push\n")) {
		t.Errorf("engine workflow not landed at %s: %q", distinct, got)
	}
	if got := readTree(t, root, ".github/workflows/docs-gate.yml"); string(got) != adopterWorkflow {
		t.Errorf("adopter's workflow was mutated:\n got %q\nwant %q", got, adopterWorkflow)
	}

	// FR-4: the engine chained onto the adopter's hook path; the prior path is
	// recorded for undo; the adopter's hook file is byte-identical.
	if got := strings.TrimSpace(git(t, root, "config", "--local", "--get", "core.hooksPath")); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks (engine chained ahead)", got)
	}
	if got := strings.TrimSpace(git(t, root, "config", "--local", "--get", "pyxis.chainedhookspath")); got != ".husky" {
		t.Errorf("chain marker = %q, want .husky", got)
	}
	if rep.Plan.Hooks.PriorHooksPath != ".husky" {
		t.Errorf("Plan.Hooks.PriorHooksPath = %q, want .husky (recorded for undo)", rep.Plan.Hooks.PriorHooksPath)
	}
	if !rep.Chained {
		t.Error("Report.Chained = false, want true (a prior hook path was chained)")
	}
	if got := readTree(t, root, ".husky/pre-push"); string(got) != huskyHook {
		t.Errorf("adopter's hook was mutated:\n got %q\nwant %q", got, huskyHook)
	}

	// The redirected workflow re-applies idempotently (its distinct path is not
	// a manifest path, so the guard — not the divergence scan — must catch it).
	before := snapshot(t, root)
	if _, err := adopt.Apply(context.Background(), applyOpts(root, r)); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if after := snapshot(t, root); !mapsEqual(before, after) {
		t.Errorf("re-apply with a redirected workflow was not idempotent:\nbefore %v\nafter  %v", before, after)
	}
}

// ---- helpers ----------------------------------------------------------------

func readTree(t *testing.T, root, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return b
}

func find(t *testing.T, files []pf, path string) pf {
	t.Helper()
	for _, f := range files {
		if f.path == path {
			return f
		}
	}
	t.Fatalf("fixture has no file %q", path)
	return pf{}
}
