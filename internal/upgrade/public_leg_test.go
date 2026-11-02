package upgrade

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// deadRunner fails every call, proving a leg never reaches for gh.
type deadRunner struct{ calls int }

func (d *deadRunner) Run(context.Context, string, ...string) (string, error) {
	d.calls++
	return "", errors.New("gh is not installed")
}

// A new user holds no credential and no gh, and the default path must work for
// them from pins to pin hand-off.
func TestPublicLegNeedsNoGitHubCLI(t *testing.T) {
	blob := []byte("public bytes")
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	gh := &deadRunner{}
	target, cfg := installedBinary(t, "old"), t.TempDir()

	rep, err := Run(t.Context(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: cfg, HTTP: run, Runner: gh,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gh.calls != 0 {
		t.Errorf("the public leg called gh %d time(s)", gh.calls)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the public asset was not installed: %q", got)
	}
	want := []string{publicPinsURL(DefaultRepo), publicAssetURL(DefaultRepo, "v9.9.9", rep.Artifact)}
	if strings.Join(run.calls, " ") != strings.Join(want, " ") {
		t.Errorf("requests = %v, want %v", run.calls, want)
	}
	if body := read(t, filepath.Join(cfg, "release.env")); strings.Contains(body, "PYXIS_CHANNEL") {
		t.Errorf("the public default recorded a channel:\n%s", body)
	}
}

// The public URLs are the contract the install one-liner, the resolver's
// default channel, and this leg all share.
func TestPublicURLsMatchTheDefaultChannel(t *testing.T) {
	if got, want := publicAssetURL(DefaultRepo, "v1.2.3", "asset"), resolve.DefaultChannelRoot+"/v1.2.3/asset"; got != want {
		t.Errorf("asset URL = %s, want %s", got, want)
	}
	if got := publicPinsURL(DefaultRepo); got != "https://raw.githubusercontent.com/agenticlab-ai/pyxis/main/install.sh" {
		t.Errorf("pins URL = %s", got)
	}
	if !strings.Contains(InstallLine, publicPinsURL(DefaultRepo)) {
		t.Errorf("InstallLine %q does not read the pins URL", InstallLine)
	}
}

// GitHub answers a release download with a redirect to its object host, which
// the default client must follow under the policy Run installs.
func TestPublicLegFollowsTheRedirectToGitHubsObjectHost(t *testing.T) {
	blob := []byte("object host bytes")
	install := installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)})
	asset := publicAssetURL(DefaultRepo, "v9.9.9", "pyxis-v9.9.9-darwin-arm64")
	const objectURL = "https://release-assets.githubusercontent.com/signed"
	var seen []string
	client := &http.Client{CheckRedirect: redirectPolicy, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.URL.String())
		resp := &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{}, Request: r, Body: io.NopCloser(strings.NewReader(""))}
		switch r.URL.String() {
		case publicPinsURL(DefaultRepo):
			resp.Body = io.NopCloser(strings.NewReader(install))
		case asset:
			resp.StatusCode, resp.Status = http.StatusFound, "302 Found"
			resp.Header.Set("Location", objectURL)
		case objectURL:
			resp.Body = io.NopCloser(strings.NewReader(string(blob)))
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
		return resp, nil
	})}
	target := installedBinary(t, "old")

	if _, err := Run(t.Context(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: client,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the redirected asset was not installed: %q (requests %v)", got, seen)
	}
}

// A stored or flagged value naming the retired host, or the default root
// itself, reads through the public leg: the retired host no longer answers, and
// the default root serves no <root>/install.sh.
func TestRetiredAndDefaultChannelsSelectThePublicLeg(t *testing.T) {
	for _, channel := range []string{
		"https://dist.pyxis.agenticlab.ai",
		"https://dist.pyxis.agenticlab.ai/",
		resolve.DefaultChannelRoot,
		resolve.DefaultChannelRoot + "/",
	} {
		t.Run(channel, func(t *testing.T) {
			blob := []byte("public bytes")
			run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
			target, cfg := installedBinary(t, "old"), t.TempDir()
			if _, err := Run(t.Context(), Options{
				Current: "v9.9.8", Platform: "darwin-arm64", Channel: channel,
				TargetPath: target, ConfigDir: cfg, HTTP: run,
			}); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if run.calls[0] != publicPinsURL(DefaultRepo) {
				t.Errorf("first read = %s, want the public pins", run.calls[0])
			}
			if body := read(t, filepath.Join(cfg, "release.env")); strings.Contains(body, "PYXIS_CHANNEL") {
				t.Errorf("the record kept a channel:\n%s", body)
			}
		})
	}
}

// An installed machine already on the pinned release still loses the retired
// address: the up-to-date path repairs the record.
func TestUpToDateRunRepairsARetiredChannelRecord(t *testing.T) {
	manifest := strings.Repeat("bb", 32)
	run := &fakeRelease{install: installSH("v9.9.9", manifest, map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	cfg := t.TempDir()
	path := filepath.Join(cfg, "release.env")
	if err := os.WriteFile(path, []byte("PYXIS_SYSTEM_VERSION=v9.9.9\nPYXIS_MANIFEST_SHA256="+manifest+"\nPYXIS_CHANNEL=https://dist.pyxis.agenticlab.ai\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(t.Context(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64", Channel: "https://dist.pyxis.agenticlab.ai",
		TargetPath: installedBinary(t, "current"), ConfigDir: cfg, HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.UpToDate || !rep.PinRepaired {
		t.Fatalf("report = %+v, want an up-to-date run that repaired the record", rep)
	}
	if body := read(t, path); strings.Contains(body, "dist.pyxis") {
		t.Errorf("the retired channel survived the repair:\n%s", body)
	}
}

// A bare 404 on the public pins must not tell a user about --channel, which
// they never passed.
func TestPublicPinsMissNamesTheRepositoryNotTheChannelFlag(t *testing.T) {
	_, err := Run(t.Context(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: &fakeDoer{},
	})
	if err == nil || !strings.Contains(err.Error(), DefaultRepo) || strings.Contains(err.Error(), "--channel") {
		t.Fatalf("err = %v, want a pins refusal naming the repository, not --channel", err)
	}
}

// A GitHub binding that names no token is a public fork: its own coordinates,
// still anonymous, still no gh.
func TestGitHubBindingWithoutATokenReadsAnonymously(t *testing.T) {
	blob := []byte("fork bytes")
	fork := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/fork"}
	doer := &fakeDoer{bodies: map[string]string{
		publicPinsURL("o/fork"): installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}),
		publicAssetURL("o/fork", "v9.9.9", "pyxis-v9.9.9-darwin-arm64"): string(blob),
	}}
	gh := &deadRunner{}
	target := installedBinary(t, "old")
	if _, err := Run(t.Context(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Release: &fork,
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: doer, Runner: gh,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gh.calls != 0 {
		t.Errorf("a tokenless binding called gh %d time(s)", gh.calls)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the fork's asset was not installed: %q", got)
	}
}

// ghTranscript scripts a private fork's gh leg.
type ghTranscript struct {
	install string
	blob    []byte
	calls   []string
}

func (g *ghTranscript) Run(_ context.Context, name string, args ...string) (string, error) {
	g.calls = append(g.calls, name+" "+strings.Join(args, " "))
	if strings.Contains(strings.Join(args, " "), "contents/install.sh") {
		return g.install, nil
	}
	var dir, pattern string
	for i, a := range args {
		if a == "--dir" && i+1 < len(args) {
			dir = args[i+1]
		}
		if a == "--pattern" && i+1 < len(args) {
			pattern = args[i+1]
		}
	}
	return "", os.WriteFile(filepath.Join(dir, pattern), g.blob, 0o644)
}

// A binding that names a token is a private fork, whose plain download URLs 404
// whatever the credential, so it is the one leg that still reads through gh.
func TestPrivateForkWithATokenReadsThroughGh(t *testing.T) {
	blob := []byte("private bytes")
	private := integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/private", TokenEnv: "PRIVATE_TOKEN"}
	gh := &ghTranscript{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	doer := &fakeDoer{}
	target := installedBinary(t, "old")
	if _, err := Run(t.Context(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Release: &private,
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: doer, Runner: gh,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(doer.seen) != 0 {
		t.Errorf("the private leg made anonymous requests: %v", doer.seen)
	}
	if len(gh.calls) != 2 || !strings.Contains(gh.calls[0], "repos/o/private/contents/install.sh") || !strings.Contains(gh.calls[1], "release download v9.9.9 --repo o/private") {
		t.Errorf("gh calls = %v", gh.calls)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the private asset was not installed: %q", got)
	}
}
