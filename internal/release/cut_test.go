package release_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
)

// fixtureCommit is the packed set's source commit — cut's provenance hop
// asserts the tag points at exactly this.
func fixtureCommit(t *testing.T, res release.PackResult) string {
	t.Helper()
	if len(res.Manifest.SourceCommit) != 40 {
		t.Fatalf("fixture source commit = %q", res.Manifest.SourceCommit)
	}
	return res.Manifest.SourceCommit
}

func tagRefPath(tag string) string {
	return "repos/" + pinsSourceRepo + "/git/ref/tags/" + tag
}

// tagRefStep answers the tag lookup with a lightweight tag at commit.
func tagRefStep(commit string) ghStep {
	return ghStep{
		check:  wantExact("api", tagRefPath(fixtureVersion)),
		stdout: fmt.Sprintf(`{"object":{"sha":%q,"type":"commit"}}`, commit),
	}
}

func tagAbsentStep() ghStep {
	return ghStep{check: wantExact("api", tagRefPath(fixtureVersion)), stderr: "gh: Not Found (HTTP 404)", exit: 1}
}

// canonicalPublishSteps is publish's transcript against the source repo: a
// release that does not exist yet, then one create carrying the whole set.
func canonicalPublishSteps() []ghStep {
	return []ghStep{
		viewStep("", "release not found", 1),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if !slices.Equal(args[:4], []string{"release", "create", fixtureVersion, "--verify-tag"}) {
				t.Fatalf("gh args = %v, want a --verify-tag create on the canonical repo", args)
			}
			if slices.Contains(args, "--repo") {
				t.Fatalf("the canonical publish must not target another repo: %v", args)
			}
		}},
	}
}

func cutOpts(t *testing.T, repo, out string, gh *fakeGH) release.CutOptions {
	t.Helper()
	return release.CutOptions{
		Version: fixtureVersion, RepoRoot: repo, OutDir: out,
		SourceRepo: pinsSourceRepo, Runner: gh,
	}
}

// Plain cut is the complete public release, so --mirror has nowhere to publish
// without an explicit binding, and says so before any stage runs.
func TestCutMirrorRequiresADistributionBindingBeforeAnyStage(t *testing.T) {
	repo := t.TempDir()
	gh := &fakeGH{t: t}
	opts := cutOpts(t, repo, filepath.Join(repo, "out"), gh)
	opts.Mirror = true
	if _, err := release.Cut(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "releases.distribution") {
		t.Fatalf("missing distribution binding: %v", err)
	}
	gh.done()
	entries, err := os.ReadDir(repo)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the refusal wrote release files: %v, %v", entries, err)
	}
}

// The first run of a release: reuse the packed set, verify the tag's
// provenance, publish canonically, and leave install.sh pinning the release
// — stopping short of the mirror, which is the human gate's other side.
func TestCutFirstRunPublishesAndPins(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)

	gh := &fakeGH{t: t, steps: append([]ghStep{tagRefStep(fixtureCommit(t, packed))}, canonicalPublishSteps()...)}
	var lines []string
	opts := cutOpts(t, repo, out, gh)
	opts.Log = func(s string) { lines = append(lines, s) }
	res, err := release.Cut(context.Background(), opts)
	if err != nil {
		t.Fatalf("cut: %v", err)
	}
	gh.done()

	if !res.Reused {
		t.Error("cut repacked a set that already verified")
	}
	if res.Tag != "present" || !res.Publish.Created {
		t.Errorf("tag/publish = %q/%v", res.Tag, res.Publish.Created)
	}
	if res.Pins.Action != "written" {
		t.Errorf("pins action = %q, want written", res.Pins.Action)
	}
	if res.Mirror != nil {
		t.Error("cut mirrored without --mirror")
	}
	if got := readInstallSH(t, repo); !strings.Contains(got, `PINNED_VERSION="`+fixtureVersion+`"`) {
		t.Error("install.sh was not repinned")
	}
	if len(lines) == 0 {
		t.Error("cut logged no progress")
	}
}

// Provenance is one hop: a tag pointing somewhere other than the commit the
// artifacts were packed from refuses before anything is published.
func TestCutRefusesTagProvenanceMismatch(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)
	other := strings.Repeat("a", 40)

	gh := &fakeGH{t: t, steps: []ghStep{tagRefStep(other)}}
	_, err := release.Cut(context.Background(), cutOpts(t, repo, out, gh))
	if err == nil || !strings.Contains(err.Error(), other) || !strings.Contains(err.Error(), fixtureCommit(t, packed)) {
		t.Fatalf("err = %v, want the provenance refusal naming both commits", err)
	}
	gh.done()
}

// An annotated tag's ref points at the tag OBJECT; cut dereferences it
// rather than reading every annotated tag as a mismatch.
func TestCutDereferencesAnnotatedTag(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)
	commit := fixtureCommit(t, packed)
	tagObj := strings.Repeat("b", 40)

	steps := []ghStep{
		{check: wantExact("api", tagRefPath(fixtureVersion)), stdout: fmt.Sprintf(`{"object":{"sha":%q,"type":"tag"}}`, tagObj)},
		{check: wantExact("api", "repos/"+pinsSourceRepo+"/git/tags/"+tagObj), stdout: fmt.Sprintf(`{"object":{"sha":%q,"type":"commit"}}`, commit)},
	}
	gh := &fakeGH{t: t, steps: append(steps, canonicalPublishSteps()...)}
	res, err := release.Cut(context.Background(), cutOpts(t, repo, out, gh))
	if err != nil {
		t.Fatalf("cut: %v", err)
	}
	gh.done()
	if res.Tag != "present" {
		t.Errorf("tag = %q, want present", res.Tag)
	}
}

// A missing tag refuses by default, naming the exact commit to tag — and
// nothing is published.
func TestCutRefusesMissingTag(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)

	gh := &fakeGH{t: t, steps: []ghStep{tagAbsentStep()}}
	_, err := release.Cut(context.Background(), cutOpts(t, repo, out, gh))
	if err == nil || !strings.Contains(err.Error(), fixtureCommit(t, packed)) || !strings.Contains(err.Error(), "--push-tag") {
		t.Fatalf("err = %v, want the missing-tag refusal naming the commit and the flag", err)
	}
	gh.done()
}

// --push-tag creates the tag through the same seam, anchored at the packed
// commit — tag, commit, and artifacts cannot diverge.
func TestCutPushTagAnchorsAtThePackedCommit(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)
	commit := fixtureCommit(t, packed)

	steps := []ghStep{tagAbsentStep(), {check: wantExact("api", "-X", "POST", "repos/"+pinsSourceRepo+"/git/refs",
		"-f", "ref=refs/tags/"+fixtureVersion, "-f", "sha="+commit)}}
	gh := &fakeGH{t: t, steps: append(steps, canonicalPublishSteps()...)}
	opts := cutOpts(t, repo, out, gh)
	opts.PushTag = true
	res, err := release.Cut(context.Background(), opts)
	if err != nil {
		t.Fatalf("cut: %v", err)
	}
	gh.done()
	if res.Tag != "created" {
		t.Errorf("tag = %q, want created", res.Tag)
	}
}

// The gate: --mirror with the regenerated pins still uncommitted refuses.
// Mirroring then would serve dist consumers an anchor the source repo's own
// front door does not carry.
func TestCutRefusesMirrorWithUncommittedPins(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)

	gh := &fakeGH{t: t, steps: append([]ghStep{tagRefStep(fixtureCommit(t, packed))}, canonicalPublishSteps()...)}
	opts := cutOpts(t, repo, out, gh)
	opts.Mirror, opts.DistTarget = true, mirrorDistTarget()
	res, err := release.Cut(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "not committed") {
		t.Fatalf("err = %v, want the uncommitted-pins refusal", err)
	}
	gh.done() // the mirror's gh calls never happened
	// The work already done is reported, not thrown away.
	if res.Pins.Action != "written" || !res.Publish.Created {
		t.Errorf("the refusal discarded the completed stages: %+v", res)
	}
}

// The second run, after the pin PR merges: the set is reused, publish
// verifies rather than re-uploads, pins read current, and the mirror runs.
func TestCutSecondRunMirrors(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)
	commit := fixtureCommit(t, packed)

	// Run 1 — publish + pins.
	gh := &fakeGH{t: t, steps: append([]ghStep{tagRefStep(commit)}, canonicalPublishSteps()...)}
	if _, err := release.Cut(context.Background(), cutOpts(t, repo, out, gh)); err != nil {
		t.Fatalf("cut run 1: %v", err)
	}
	gh.done()

	// The human gate: the pin PR lands.
	gitRun(t, repo, "add", "install.sh")
	gitRun(t, repo, "commit", "-q", "-m", "chore(release): point install.sh at "+fixtureVersion, "--no-gpg-sign")
	pinned := readInstallSH(t, repo)

	// Run 2 — everything verifies; the mirror publishes.
	names, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	steps := []ghStep{tagRefStep(commit), viewStep(`{"assets":[]}`, "", 0)}
	for _, e := range names { // publish uploads each artifact into the existing release
		steps = append(steps, uploadStep(fixtureVersion, out, e.Name()))
	}
	steps = append(steps, boundMirrorPublishSteps()...)
	gh2 := &fakeGH{t: t, steps: steps}
	opts := cutOpts(t, repo, out, gh2)
	opts.Mirror, opts.DistTarget = true, mirrorDistTarget()
	res, err := release.Cut(context.Background(), opts)
	if err != nil {
		t.Fatalf("cut run 2: %v", err)
	}
	gh2.done()

	if !res.Reused || res.Pins.Action != "current" {
		t.Errorf("run 2 reused/pins = %v/%q, want true/current", res.Reused, res.Pins.Action)
	}
	if res.Mirror == nil || res.Mirror.InstallSH != "written" {
		t.Errorf("mirror = %+v, want the pinned distribution install.sh written", res.Mirror)
	}
	if after := readInstallSH(t, repo); after != pinned {
		t.Error("run 2 rewrote the committed install.sh")
	}
}

// The one confusing route to a publish refusal: run 2 from a checkout that
// no longer has the packed set. Cut repacks, the repack is not byte-
// identical after source or toolchain changes, and the published tag
// refuses the divergence. The refusal must say what to do instead of
// leaving the operator to re-run a command that can never succeed.
func TestCutExplainsAPublishRefusalAfterRepacking(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, packed := packFixture(t, repo)
	commit := fixtureCommit(t, packed)

	// The tag exists and the release already carries a diverging asset
	// (SHA256SUMS sorts first, so publish reaches it immediately).
	gh := &fakeGH{t: t, steps: []ghStep{
		tagRefStep(commit),
		viewStep(`{"assets":[{"name":"`+release.SumsName+`"}]}`, "", 0),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if args[0] != "release" || args[1] != "download" {
				t.Fatalf("gh args = %v, want the verify download", args)
			}
		}, deliver: []byte("different bytes entirely\n")},
	}}
	opts := cutOpts(t, repo, out, gh)
	opts.BinariesDir = fixtureBinaries(t)

	opts.Repack = true // stand in for "the set was gone, so cut rebuilt"
	res, err := release.Cut(context.Background(), opts)
	if err == nil {
		t.Fatal("err = nil, want the publish refusal")
	}
	gh.done()
	for _, want := range []string{"different bytes", "pins --version " + fixtureVersion} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not carry %q:\n%v", want, err)
		}
	}
	// The stages that DID complete are still reported, not discarded.
	if res.SourceCommit != commit || res.Tag != "present" {
		t.Errorf("the refusal discarded completed stages: %+v", res)
	}
}

// A packed set for a DIFFERENT version in --out is not "reusable": cut
// refuses rather than publishing the wrong artifacts under this tag, and
// leaves the existing set untouched.
func TestCutDoesNotReuseAnotherVersionsSet(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	out, _ := packFixture(t, repo)

	opts := cutOpts(t, repo, out, &fakeGH{t: t})
	opts.Version = "v9.9.8"
	opts.BinariesDir = fixtureBinariesFor(t, "v9.9.8")

	_, err := release.Cut(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "another release") {
		t.Fatalf("err = %v, want the foreign-artifact refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(out, "release-manifest-"+fixtureVersion+".json")); statErr != nil {
		t.Error("the existing set was disturbed by the refused run")
	}
}

// With no --out, the set lands under the REPO ROOT, not the process's
// working directory — a --repo-root elsewhere must not scatter ~60
// binaries wherever the operator happened to stand (and outside the
// .gitignore entry that hides them).
func TestCutDefaultsOutDirUnderRepoRoot(t *testing.T) {
	repo := fixtureRepo(t)
	stalePinsInstallSH(t, repo)
	// Stand somewhere else entirely.
	elsewhere := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	gh := &fakeGH{t: t, steps: []ghStep{tagAbsentStep()}}
	opts := release.CutOptions{
		Version: fixtureVersion, RepoRoot: repo, SourceRepo: pinsSourceRepo,
		Runner:      gh,
		BinariesDir: fixtureBinaries(t),
	}
	// Stops at the missing-tag refusal — but only AFTER packing, which is
	// what puts the set on disk for the assertions below.
	if _, err := release.Cut(context.Background(), opts); err == nil {
		t.Fatal("err = nil, want the missing-tag refusal")
	}
	gh.done()

	if _, err := os.Stat(filepath.Join(repo, release.DefaultOutDir, release.SumsName)); err != nil {
		t.Errorf("no artifact set under the repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, release.DefaultOutDir)); !os.IsNotExist(err) {
		t.Error("the set landed in the working directory instead of the repo root")
	}
}
