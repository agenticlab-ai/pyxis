package release_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
)

const (
	boundMirrorSource = "example-org/pyxis"
	boundMirrorDist   = "example-org/pyxis-dist"
)

// fixtureBoundInstallSH writes a pins-bearing install.sh into the checkout root
// and returns its content.
func fixtureBoundInstallSH(t *testing.T, repo, version string) string {
	t.Helper()
	content := "#!/bin/sh\n" +
		`RELEASE_PROVIDER="${PYXIS_RELEASE_PROVIDER:-github}"` + "\n" +
		`RELEASE_BASE_URL="${PYXIS_RELEASE_BASE_URL:-https://github.com}"` + "\n" +
		`RELEASE_TOKEN_ENV="${PYXIS_RELEASE_TOKEN_ENV:-}"` + "\n" +
		"# install pyxis — served from " + boundMirrorSource + "\n" +
		"REPO=\"" + boundMirrorSource + "\"\n" +
		"# BEGIN PINS " + version + "\n" +
		"PINNED_VERSION=\"" + version + "\"\n" +
		"MANIFEST_SHA256=\"abc\"\n" +
		"# END PINS\n"
	writeFile(t, repo, "install.sh", 0o755, content)
	return content
}

func boundContentsGetStep(stdout, stderr string, exit int) ghStep {
	return ghStep{check: wantExact("api", "repos/"+boundMirrorDist+"/contents/install.sh"), stdout: stdout, stderr: stderr, exit: exit}
}

func boundContentsJSON(t *testing.T, sha, raw string) string {
	t.Helper()
	return fmt.Sprintf(`{"sha":%q,"content":%q,"encoding":"base64"}`, sha, base64.StdEncoding.EncodeToString([]byte(raw)))
}

// boundMirrorPublishSteps is the transcript of a first publication to the bound
// repository: the release is created, and its install.sh is created too.
func boundMirrorPublishSteps() []ghStep {
	return []ghStep{
		{check: wantExact("release", "view", fixtureVersion, "--json", "assets", "--repo", boundMirrorDist), stderr: "release not found", exit: 1},
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if args[0] != "release" || args[1] != "create" {
				t.Fatalf("gh args = %v, want a release create", args)
			}
		}},
		boundContentsGetStep("", "gh: Not Found (HTTP 404)", 1),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if len(args) < 4 || args[0] != "api" || args[2] != "PUT" {
				t.Fatalf("gh args = %v, want the contents PUT", args)
			}
		}},
	}
}

// The tag precondition: an install.sh pinning a different version refuses
// before any remote operation — the stranding hazard made mechanical.
func TestBoundMirrorRefusesStalePins(t *testing.T) {
	repo := fixtureRepo(t)
	fixtureBoundInstallSH(t, repo, "v0.0.1")
	gh := &fakeGH{t: t}
	_, err := release.Mirror(context.Background(), release.MirrorOptions{
		DistTarget: &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist},
		Tag:        fixtureVersion, RepoRoot: repo, SourceRepo: boundMirrorSource, DistRepo: boundMirrorDist, Runner: gh,
	})
	if err == nil || !strings.Contains(err.Error(), "v0.0.1") || !strings.Contains(err.Error(), fixtureVersion) {
		t.Fatalf("err = %v, want the stale-pins refusal naming both versions", err)
	}
	gh.done() // zero gh calls happened
}

// A checkout without install.sh refuses naming the file (a --repo-root
// mistake, not a silent skip of the front-door refresh).
func TestBoundMirrorRefusesMissingInstallSH(t *testing.T) {
	gh := &fakeGH{t: t}
	_, err := release.Mirror(context.Background(), release.MirrorOptions{
		DistTarget: &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist},
		Tag:        fixtureVersion, RepoRoot: t.TempDir(), SourceRepo: boundMirrorSource, DistRepo: boundMirrorDist, Runner: gh,
	})
	if err == nil || !strings.Contains(err.Error(), "install.sh") {
		t.Fatalf("err = %v, want the missing-install.sh refusal", err)
	}
	gh.done()
}

// The prefetched happy path: --out carries the set, so the transcript is
// publish (view + create, both --repo dist) then the install.sh refresh
// (GET divergent content → PUT with the transformed bytes and the sha).
func TestBoundMirrorPrefetchedSet(t *testing.T) {
	repo := fixtureRepo(t)
	src := fixtureBoundInstallSH(t, repo, fixtureVersion)
	out, res := packFixture(t, repo)
	want := strings.ReplaceAll(src, boundMirrorSource, boundMirrorDist)
	gh := &fakeGH{t: t, steps: []ghStep{
		{check: wantExact("release", "view", fixtureVersion, "--json", "assets", "--repo", boundMirrorDist), stderr: "release not found", exit: 1},
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if args[0] != "release" || args[1] != "create" {
				t.Fatalf("gh args = %v, want a release create", args)
			}
		}},
		boundContentsGetStep(boundContentsJSON(t, "oldsha", "stale copy"), "", 0),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if len(args) < 4 || args[0] != "api" || args[1] != "-X" || args[2] != "PUT" || args[3] != "repos/"+boundMirrorDist+"/contents/install.sh" {
				t.Fatalf("gh args = %v, want the contents PUT", args)
			}
			joined := strings.Join(args, "\x00")
			if !strings.Contains(joined, "content="+base64.StdEncoding.EncodeToString([]byte(want))) {
				t.Fatalf("PUT lacks the transformed install.sh content: %v", args)
			}
			if !strings.Contains(joined, "sha=oldsha") {
				t.Fatalf("PUT lacks the current sha (update leg): %v", args)
			}
			if !strings.Contains(joined, fixtureVersion) {
				t.Fatalf("PUT message does not name the tag: %v", args)
			}
		}},
	}}
	mres, err := release.Mirror(context.Background(), release.MirrorOptions{
		DistTarget: &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist},
		Tag:        fixtureVersion, OutDir: out, RepoRoot: repo, SourceRepo: boundMirrorSource, DistRepo: boundMirrorDist, Runner: gh,
	})
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	gh.done()
	if mres.Downloaded || !mres.Publish.Created || mres.InstallSH != "written" || len(mres.Publish.Actions) != len(res.Artifacts) {
		t.Fatalf("result = %+v, want prefetched + created + install.sh written", mres)
	}
}

// A dist install.sh already carrying the transformed bytes is verified,
// never rewritten — the idempotent re-run leg.
func TestBoundMirrorInstallSHAlreadyCurrent(t *testing.T) {
	repo := fixtureRepo(t)
	src := fixtureBoundInstallSH(t, repo, fixtureVersion)
	out, _ := packFixture(t, repo)
	want := strings.ReplaceAll(src, boundMirrorSource, boundMirrorDist)
	gh := &fakeGH{t: t, steps: []ghStep{
		{check: wantExact("release", "view", fixtureVersion, "--json", "assets", "--repo", boundMirrorDist), stderr: "release not found", exit: 1},
		{check: func(t *testing.T, args []string) { t.Helper() }},
		boundContentsGetStep(boundContentsJSON(t, "cursha", want), "", 0),
	}}
	mres, err := release.Mirror(context.Background(), release.MirrorOptions{
		DistTarget: &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist},
		Tag:        fixtureVersion, OutDir: out, RepoRoot: repo, SourceRepo: boundMirrorSource, DistRepo: boundMirrorDist, Runner: gh,
	})
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	gh.done()
	if mres.InstallSH != "verified" {
		t.Fatalf("install.sh action = %q, want verified", mres.InstallSH)
	}
}

// The download leg: no --out means the canonical set is fetched from the
// source repo into a temp dir (the fake materializes it there), and an
// absent dist install.sh is created — PUT without a sha.
func TestBoundMirrorDownloadsAndCreatesInstallSH(t *testing.T) {
	repo := fixtureRepo(t)
	fixtureBoundInstallSH(t, repo, fixtureVersion)
	packed, res := packFixture(t, repo)
	gh := &fakeGH{t: t, steps: []ghStep{
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if len(args) < 3 || args[0] != "release" || args[1] != "download" || args[2] != fixtureVersion {
				t.Fatalf("gh args = %v, want a release download", args)
			}
			joined := strings.Join(args, "\x00")
			if !strings.Contains(joined, "--repo\x00"+boundMirrorSource) {
				t.Fatalf("download does not target the source repo: %v", args)
			}
			dir := ""
			for i, a := range args {
				if a == "--dir" && i+1 < len(args) {
					dir = args[i+1]
				}
			}
			if dir == "" {
				t.Fatalf("download without --dir: %v", args)
			}
			entries, err := os.ReadDir(packed)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				b, err := os.ReadFile(filepath.Join(packed, e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}},
		{check: wantExact("release", "view", fixtureVersion, "--json", "assets", "--repo", boundMirrorDist), stderr: "release not found", exit: 1},
		{check: func(t *testing.T, args []string) { t.Helper() }},
		boundContentsGetStep("", "gh: Not Found (HTTP 404)", 1),
		{check: func(t *testing.T, args []string) {
			t.Helper()
			if strings.Contains(strings.Join(args, "\x00"), "\x00sha=") {
				t.Fatalf("create-leg PUT carries a sha: %v", args)
			}
		}},
	}}
	mres, err := release.Mirror(context.Background(), release.MirrorOptions{
		DistTarget: &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist},
		Tag:        fixtureVersion, RepoRoot: repo, SourceRepo: boundMirrorSource, DistRepo: boundMirrorDist, Runner: gh,
	})
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	gh.done()
	if !mres.Downloaded || !mres.Publish.Created || mres.InstallSH != "written" || len(mres.Publish.Actions) != len(res.Artifacts) {
		t.Fatalf("result = %+v, want downloaded + created + install.sh written", mres)
	}
}
