package release_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
)

const mirrorSource = "example-org/pyxis"

func mirrorDistTarget() *integrations.ResolvedRepository {
	return &integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: boundMirrorDist}
}

func TestMirrorRefusesBeforePublication(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*testing.T, *release.MirrorOptions)
		want string
	}{
		{"missing distribution binding", func(_ *testing.T, o *release.MirrorOptions) { o.DistTarget = nil }, "releases.distribution"},
		{"invalid tag", func(_ *testing.T, o *release.MirrorOptions) { o.Tag = "../bad" }, "not a system tag"},
		{"missing runner", func(_ *testing.T, o *release.MirrorOptions) { o.OutDir, o.Runner = "", nil }, "gh runner"},
		{"missing source repo", func(_ *testing.T, o *release.MirrorOptions) { o.OutDir, o.SourceRepo = "", "" }, "source repo"},
		{"missing installer", func(t *testing.T, o *release.MirrorOptions) { o.RepoRoot = t.TempDir() }, "install.sh"},
		{"missing pin", func(t *testing.T, o *release.MirrorOptions) {
			writeFile(t, o.RepoRoot, "install.sh", 0o755, "#!/bin/sh\n")
		}, "no PINNED_VERSION"},
		{"stale pin", func(t *testing.T, o *release.MirrorOptions) {
			fixtureBoundInstallSH(t, o.RepoRoot, "v0.0.1")
		}, "install.sh pins v0.0.1"},
		{"tampered set", func(t *testing.T, o *release.MirrorOptions) {
			writeFile(t, o.OutDir, "payload-"+fixtureVersion+".tar.gz", 0o644, "tampered")
		}, "failed verification"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := fixtureRepo(t)
			fixtureBoundInstallSH(t, repo, fixtureVersion)
			out, _ := packFixture(t, repo)
			gh := &fakeGH{t: t} // every GitHub call is unexpected
			opts := release.MirrorOptions{Tag: fixtureVersion, OutDir: out, RepoRoot: repo, SourceRepo: mirrorSource, Runner: gh, DistTarget: mirrorDistTarget()}
			tc.edit(t, &opts)
			if _, err := release.Mirror(t.Context(), opts); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Mirror error = %v, want %q", err, tc.want)
			}
			gh.done()
		})
	}
}

// A corrupted download is rejected before the first publication request.
func TestMirrorRefusesATamperedDownload(t *testing.T) {
	repo := fixtureRepo(t)
	fixtureBoundInstallSH(t, repo, fixtureVersion)
	packed, res := packFixture(t, repo)
	var downloadDir string
	gh := &fakeGH{t: t, steps: []ghStep{{check: func(t *testing.T, args []string) {
		t.Helper()
		if len(args) != 8 || !slices.Equal(args[:6], []string{"release", "download", fixtureVersion, "--repo", mirrorSource, "--dir"}) || args[7] != "--clobber" {
			t.Fatalf("unexpected canonical download: %v", args)
		}
		downloadDir = args[6]
		for _, name := range res.Artifacts {
			b, err := os.ReadFile(filepath.Join(packed, name))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, downloadDir, name, 0o644, string(b))
		}
		writeFile(t, downloadDir, release.SumsName, 0o644, "broken checksums")
	}}}}
	_, err := release.Mirror(context.Background(), release.MirrorOptions{
		Tag: fixtureVersion, RepoRoot: repo, SourceRepo: mirrorSource, Runner: gh, DistTarget: mirrorDistTarget(),
	})
	gh.done() // no publication call followed the download
	if err == nil || !strings.Contains(err.Error(), "failed verification") {
		t.Fatalf("tampered download: err=%v", err)
	}
	if _, err := os.Stat(downloadDir); !os.IsNotExist(err) {
		t.Fatalf("temporary download directory was not removed: %v", err)
	}
}

func TestMirrorRefusesUnavailableTemporaryDirectory(t *testing.T) {
	repo := fixtureRepo(t)
	fixtureBoundInstallSH(t, repo, fixtureVersion)
	gh := &fakeGH{t: t}
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	_, err := release.Mirror(t.Context(), release.MirrorOptions{
		Tag: fixtureVersion, RepoRoot: repo, SourceRepo: mirrorSource, Runner: gh, DistTarget: mirrorDistTarget(),
	})
	if err == nil || !strings.Contains(err.Error(), "mirror") {
		t.Fatalf("unavailable temporary directory: %v", err)
	}
	gh.done()
}
