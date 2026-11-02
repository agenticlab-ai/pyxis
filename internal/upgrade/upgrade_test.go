package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installSH renders a pins-bearing install.sh for the fixture release.
func installSH(version, manifestSHA string, platformSHAs map[string]string) string {
	var b strings.Builder
	b.WriteString("#!/usr/bin/env sh\nset -eu\n\n# BEGIN PINS " + version + "\n")
	fmt.Fprintf(&b, "PINNED_VERSION=%q\n", version)
	fmt.Fprintf(&b, "MANIFEST_SHA256=%q\n", manifestSHA)
	for _, plat := range sortedKeys(platformSHAs) {
		os, arch, _ := strings.Cut(plat, "-")
		fmt.Fprintf(&b, "sha_%s_%s=%q\n", os, arch, platformSHAs[plat])
	}
	b.WriteString("# END PINS\n")
	return b.String()
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// fakeRelease scripts the public GitHub leg by URL: the default branch's
// install.sh and the release asset.
type fakeRelease struct {
	tag     string
	install string
	blob    []byte
	calls   []string
	failOn  string
	fail    error
}

func (f *fakeRelease) Do(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	f.calls = append(f.calls, url)
	if f.failOn != "" && strings.Contains(url, f.failOn) {
		return nil, f.fail
	}
	body, code := "", http.StatusNotFound
	switch {
	case url == publicPinsURL(DefaultRepo):
		body, code = f.install, http.StatusOK
	case strings.HasPrefix(url, "https://github.com/"+DefaultRepo+"/releases/download/"):
		body, code = string(f.blob), http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// fakeDoer scripts the channel leg by URL.
type fakeDoer struct {
	bodies map[string]string
	status map[string]int
	seen   []string
	auth   []string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	f.seen = append(f.seen, url)
	if u, p, ok := req.BasicAuth(); ok {
		f.auth = append(f.auth, u+":"+p)
	}
	code := http.StatusOK
	if c, ok := f.status[url]; ok {
		code = c
	}
	body, ok := f.bodies[url]
	if !ok && code == http.StatusOK {
		code = http.StatusNotFound
	}
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// installedBinary lays down a stand-in for the installed CLI.
func installedBinary(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pyxis")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The public leg end to end: the default branch's install.sh carries the pins,
// the asset is verified against them, and only then does the running binary get
// replaced — plus the release.env pin hand-off that keeps `pyxis init` flagless.
func TestRunGitHubLegVerifiesThenReplaces(t *testing.T) {
	blob := []byte("#!/bin/sh\necho new pyxis\n")
	pins := map[string]string{"darwin-arm64": sum(blob), "linux-amd64": strings.Repeat("aa", 32)}
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), pins), blob: blob}
	target := installedBinary(t, "old")
	cfgDir := t.TempDir()

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: cfgDir, HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Target != "v9.9.9" || rep.UpToDate {
		t.Fatalf("report = %+v, want the v9.9.9 upgrade", rep)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(blob) {
		t.Fatalf("the installed binary was not replaced (err=%v)", err)
	}
	info, err := os.Stat(target)
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("the installed binary is not executable: mode=%v err=%v", info.Mode(), err)
	}
	env, err := os.ReadFile(filepath.Join(cfgDir, "release.env"))
	if err != nil {
		t.Fatalf("release.env not written: %v", err)
	}
	for _, want := range []string{"PYXIS_SYSTEM_VERSION=v9.9.9", "PYXIS_MANIFEST_SHA256=" + strings.Repeat("bb", 32)} {
		if !strings.Contains(string(env), want) {
			t.Errorf("release.env lacks %q:\n%s", want, env)
		}
	}
	// install.sh must be read from the DEFAULT BRANCH, never at a tag: in
	// this repo a tag cannot carry the pins for its own artifacts (the pin
	// commit lands after the tag), so a tag's copy names the PREVIOUS release
	// and would silently downgrade a current machine.
	if len(run.calls) == 0 || run.calls[0] != publicPinsURL(DefaultRepo) {
		t.Errorf("first read = %v, want the default branch's install.sh", run.calls)
	}
	for _, c := range run.calls {
		if strings.Contains(c, "releases/latest") {
			t.Errorf("upgrade consulted `latest`, whose tag may have no published pins yet: %s", c)
		}
	}
}

// The regression the tag-reading design would have caused (#1028 review):
// when the newest TAG is ahead of the pins (the window between cutting a
// release and merging its pin commit), the pinned release is the one whose
// bytes can be verified — and the machine must never be moved backwards to
// a release older than itself by a discovery mistake.
func TestRunTargetsThePinnedReleaseNotTheNewestTag(t *testing.T) {
	blob := []byte("pinned bytes")
	// install.sh (default branch) pins v9.9.9 while tag v10.0.0 exists.
	run := &fakeRelease{tag: "v10.0.0", install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	target := installedBinary(t, "old")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Target != "v9.9.9" {
		t.Fatalf("target = %s, want the PINNED release (v9.9.9), not the newest tag", rep.Target)
	}
	for _, c := range run.calls {
		if strings.Contains(c, "/releases/download/") && !strings.Contains(c, "v9.9.9") {
			t.Errorf("downloaded a release the pins do not describe: %s", c)
		}
	}
}

// A machine already on the pinned release is never moved, and in particular
// never moved BACKWARDS.
func TestRunNeverDowngradesTheCurrentRelease(t *testing.T) {
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	target := installedBinary(t, "current v9.9.9")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.UpToDate {
		t.Fatalf("report = %+v, want UpToDate (no move at all)", rep)
	}
	if got, _ := os.ReadFile(target); string(got) != "current v9.9.9" {
		t.Errorf("the current binary was overwritten: %q", got)
	}
}

// A sha line that is empty or malformed leaves the platform unpinned, and an
// unpinned platform refuses — verification can never be made vacuous by the
// pins block itself.
func TestParsePinsRejectsVacuousDigests(t *testing.T) {
	for _, body := range []string{
		"PINNED_VERSION=\"v1.0.0\"\nsha_darwin_arm64=\"\"\n",
		"PINNED_VERSION=\"v1.0.0\"\nsha_darwin_arm64=\"abc\"\n",
		"PINNED_VERSION=\"v1.0.0\"\n#sha_darwin_arm64=\"" + strings.Repeat("ab", 32) + "\"\n",
	} {
		if _, err := parsePins(body); err == nil {
			t.Errorf("parsePins accepted a pins block with no usable digest:\n%s", body)
		}
	}
}

// Digests compare case-insensitively on both sides, so an upper-case pin
// verifies rather than reading as a mismatch.
func TestRunVerifiesAgainstAnUppercasePin(t *testing.T) {
	blob := []byte("new bytes")
	up := strings.ToUpper(sum(blob))
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": up}), blob: blob}
	target := installedBinary(t, "old")

	if _, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("an upper-case pin failed to verify: %q", got)
	}
}

// The checksum floor (DINV-1): bytes that do not hash to the pinned value
// install nothing, and the running binary is left byte-identical.
func TestRunRefusesOnChecksumMismatch(t *testing.T) {
	pins := map[string]string{"darwin-arm64": strings.Repeat("cc", 32)} // not the blob's
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), pins), blob: []byte("tampered")}
	target := installedBinary(t, "old")

	_, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want a checksum refusal", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "old" {
		t.Errorf("the installed binary changed despite the refusal: %q", got)
	}
}

// Already current: report it and touch nothing.
func TestRunUpToDateDoesNothing(t *testing.T) {
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	target := installedBinary(t, "current")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.UpToDate {
		t.Fatalf("report = %+v, want UpToDate", rep)
	}
	if got, _ := os.ReadFile(target); string(got) != "current" {
		t.Errorf("an up-to-date run rewrote the binary: %q", got)
	}
	for _, c := range run.calls {
		if strings.Contains(c, "/releases/download/") {
			t.Errorf("an up-to-date run still downloaded an artifact")
		}
	}
}

// Preview reports the whole would-do set and performs none of it — no
// download, no write (FR-24's posture, held by every mutating verb here).
func TestRunPreviewPerformsNothing(t *testing.T) {
	blob := []byte("new")
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	target := installedBinary(t, "old")
	cfgDir := t.TempDir()

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: cfgDir, HTTP: run, Preview: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.Preview || rep.Target != "v9.9.9" || rep.SHA256 == "" {
		t.Fatalf("preview report = %+v, want the full would-do set", rep)
	}
	if got, _ := os.ReadFile(target); string(got) != "old" {
		t.Errorf("preview replaced the binary: %q", got)
	}
	if _, err := os.Stat(filepath.Join(cfgDir, "release.env")); !os.IsNotExist(err) {
		t.Errorf("preview wrote release.env")
	}
	for _, c := range run.calls {
		if strings.Contains(c, "/releases/download/") {
			t.Errorf("preview downloaded an artifact: %s", c)
		}
	}
}

// The channel leg reads its pins and bytes from a mirror's root, anonymously,
// and a refusal surfaces the status it got.
func TestRunChannelLegReadsTheMirrorRoot(t *testing.T) {
	blob := []byte("channel bytes")
	root := "https://dist.example.test"
	doer := &fakeDoer{bodies: map[string]string{
		root + "/install.sh":                       installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}),
		root + "/v9.9.9/pyxis-v9.9.9-darwin-arm64": string(blob),
	}}
	target := installedBinary(t, "old")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Channel: root,
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: doer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Target != "v9.9.9" {
		t.Fatalf("report = %+v", rep)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("channel bytes were not installed: %q", got)
	}
	if len(doer.auth) != 0 {
		t.Errorf("a credential was presented: %v", doer.auth)
	}

	locked := &fakeDoer{
		bodies: map[string]string{root + "/install.sh": ""},
		status: map[string]int{root + "/install.sh": http.StatusUnauthorized},
	}
	_, err = Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Channel: root,
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: locked,
	})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want a refusal naming the status", err)
	}
}

// An unwritable install directory refuses BEFORE any download, naming the
// path and the install line that can replace it.
func TestRunRefusesAnUnwritableTargetBeforeFetching(t *testing.T) {
	blob := []byte("new")
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	dir := t.TempDir()
	target := filepath.Join(dir, "pyxis")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory write permission, so this refusal cannot be provoked")
	}
	if err := os.Chmod(dir, 0o500); err != nil { // read+execute, no write
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // restores access so TempDir can clean up

	_, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("err = %v, want an unwritable-target refusal", err)
	}
	if !strings.Contains(err.Error(), InstallLine) {
		t.Errorf("the refusal does not name the install line: %v", err)
	}
	for _, c := range run.calls {
		if strings.Contains(c, "/releases/download/") {
			t.Errorf("an unwritable target still downloaded: %s", c)
		}
	}
}

// Moving BACKWARDS never happens by default (#1031 review): a channel that
// rolls its pin back would otherwise replace a newer installed binary with
// an older artifact, verifying cleanly the whole way.
func TestRunRefusesADowngradeWithoutIntent(t *testing.T) {
	blob := []byte("older release")
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	target := installedBinary(t, "newer v9.9.10")

	_, err := Run(context.Background(), Options{
		Current: "v9.9.10", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "OLDER") {
		t.Fatalf("err = %v, want a refusal to move backwards", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "newer v9.9.10" {
		t.Errorf("the newer binary was replaced anyway: %q", got)
	}

	// With explicit intent it proceeds, and says so.
	rep, err := Run(context.Background(), Options{
		Current: "v9.9.10", Platform: "darwin-arm64", AllowDowngrade: true,
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run with --allow-downgrade: %v", err)
	}
	if !rep.Downgrade {
		t.Errorf("report = %+v, want Downgrade recorded", rep)
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the intended downgrade did not install: %q", got)
	}
}

// Version ordering drives that guard, including multi-digit components —
// v9.9.10 is NEWER than v9.9.9, which string comparison gets backwards.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"v9.9.10", "v9.9.9", 1, true},
		{"v9.9.9", "v9.9.10", -1, true},
		{"v0.1.43", "v0.1.42", 1, true},
		{"v0.1.4", "v0.1.41", -1, true},
		{"v1.2.3", "1.2.3", 0, true},
		{"v2.0", "v2.0.0", 0, true},
		{"source", "v1.0.0", 0, false},
		{"", "v1.0.0", 0, false},
	}
	for _, c := range cases {
		got, ok := compareVersions(c.a, c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("compareVersions(%q,%q) = (%d,%v), want (%d,%v)", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

// An unorderable pair is "direction unknown", which fails closed rather than
// assuming forward.
func TestRunRefusesAnUnorderableTarget(t *testing.T) {
	run := &fakeRelease{install: installSH("nightly", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	_, err := Run(context.Background(), Options{
		Current: "v1.0.0", Platform: "darwin-arm64",
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot tell whether") {
		t.Fatalf("err = %v, want a fail-closed refusal on an unorderable pair", err)
	}
}

// A source build has no installed CLI to replace: under `go run` the
// executable is a throwaway, so "upgrading" it would report success and
// change nothing an operator can run tomorrow.
func TestRunRefusesASourceBuild(t *testing.T) {
	for _, current := range []string{"source", ""} {
		_, err := Run(context.Background(), Options{
			Current: current, Platform: "darwin-arm64",
			TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(),
			HTTP: &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})},
		})
		if err == nil || !strings.Contains(err.Error(), "source build") {
			t.Fatalf("Current=%q: err = %v, want a source-build refusal", current, err)
		}
		if !strings.Contains(err.Error(), InstallLine) {
			t.Errorf("the refusal does not point at installation: %v", err)
		}
	}
}

// A pins block with no MANIFEST_SHA256 is refused before anything is
// fetched: installing the binary while leaving an older pin behind is a
// half-upgrade that `pyxis init` would later resolve against.
func TestParsePinsRequiresTheManifestDigest(t *testing.T) {
	body := "PINNED_VERSION=\"v1.0.0\"\nsha_darwin_arm64=\"" + strings.Repeat("ab", 32) + "\"\n"
	if _, err := parsePins(body); err == nil || !strings.Contains(err.Error(), "MANIFEST_SHA256") {
		t.Fatalf("err = %v, want a refusal naming the missing manifest digest", err)
	}
}

// A release download redirects to GitHub's object host, so a cross-origin hop
// must follow — and nothing may leave TLS, because an intermediary could then
// choose both the bytes and the digest they are checked against.
func TestRedirectPolicyFollowsAcrossHostsButNeverOffTLS(t *testing.T) {
	from, err := http.NewRequest(http.MethodGet, "https://github.com/o/r/releases/download/v1/asset", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := []*http.Request{from}

	downgrade, _ := http.NewRequest(http.MethodGet, "http://github.com/o/r/releases/download/v1/asset", nil)
	if err := redirectPolicy(downgrade, via); err == nil || !strings.Contains(err.Error(), "stay on TLS") {
		t.Fatalf("a scheme-downgrade redirect was allowed: %v", err)
	}
	objectHost, _ := http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/asset", nil)
	if err := redirectPolicy(objectHost, via); err != nil {
		t.Errorf("a cross-origin TLS redirect was refused: %v", err)
	}
	// The hop limit still applies.
	deep := make([]*http.Request, 10)
	for i := range deep {
		deep[i] = from
	}
	if err := redirectPolicy(objectHost, deep); err == nil || !strings.Contains(err.Error(), "10 redirects") {
		t.Errorf("the redirect hop limit is gone: %v", err)
	}
}

// A platform the release does not pin refuses by name, listing what it does.
func TestRunRefusesAnUnpinnedPlatform(t *testing.T) {
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	_, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "plan9-mips",
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "plan9-mips") || !strings.Contains(err.Error(), "darwin-arm64") {
		t.Fatalf("err = %v, want a named refusal listing the pinned platforms", err)
	}
}

func TestParsePinsRefusesACopyWithoutPins(t *testing.T) {
	if _, err := parsePins("#!/bin/sh\necho hello\n"); err == nil || !strings.Contains(err.Error(), "PINNED_VERSION") {
		t.Fatalf("err = %v, want a not-a-pins-bearing-copy refusal", err)
	}
	if _, err := parsePins(`PINNED_VERSION="v1.0.0"` + "\n"); err == nil || !strings.Contains(err.Error(), "per-platform sha") {
		t.Fatalf("err = %v, want a refusal for a pins block with no platform shas", err)
	}
}

// RepoPin reads an installed repo's pin and stays quiet elsewhere — it only
// decides whether to PRINT a pointer, so a miss must never be an error.
func TestRepoPinReadsInstalledReposOnly(t *testing.T) {
	dir := t.TempDir()
	if got := RepoPin(dir); got != "" {
		t.Errorf("RepoPin on a plain directory = %q, want empty", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "schema_version: pyxis-config/v2\nsystem:\n  version: \"v0.1.42\"\n  mode: release\n"
	if err := os.WriteFile(filepath.Join(dir, ".pyxis", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RepoPin(dir); got != "v0.1.42" {
		t.Errorf("RepoPin = %q, want v0.1.42", got)
	}
}

// A real TLS channel that 302s to a real plain-http host must never be followed:
// the plain host could serve forged pins (#1031 review, P1).
func TestChannelRedirectOffTLSIsRefused(t *testing.T) {
	var contacted bool
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contacted = true
		_, _ = w.Write([]byte("PINNED_VERSION=\"v9.9.9\"\n")) // the client asserts what it received
	}))
	defer plain.Close()

	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	defer tlsSrv.Close()

	client := tlsSrv.Client()             // trusts the test CA
	client.CheckRedirect = redirectPolicy // exactly what Run installs

	_, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Channel: tlsSrv.URL,
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: client,
	})
	if err == nil {
		t.Fatal("the scheme-downgrading redirect was followed")
	}
	if contacted {
		t.Fatal("the plain-http host was contacted")
	}
	if !strings.Contains(err.Error(), "stay on TLS") {
		t.Errorf("err = %v, want the TLS-only refusal", err)
	}
}

// A channel supplies BOTH the pins this verb trusts and the binary it
// installs, so plain http would let an intermediary choose the bytes and the
// digest they are checked against (#1031 review, P1).
func TestChannelMustBeTLS(t *testing.T) {
	var served bool
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		_, _ = w.Write([]byte("PINNED_VERSION=\"v9.9.9\"\n")) // the client asserts what it received
	}))
	defer plain.Close()

	_, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", Channel: plain.URL,
		TargetPath: installedBinary(t, "old"), ConfigDir: t.TempDir(), HTTP: plain.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "must be https") {
		t.Fatalf("err = %v, want an https-only channel refusal", err)
	}
	if served {
		t.Error("the plain-http channel was contacted before the refusal")
	}
}

// A prerelease must not read as an older release (#1031 review, P1): treating
// v1.2.4-rc1 as 1.2.0 let the genuinely older v1.2.3 install over it.
func TestPrereleaseVersionsOrderCorrectly(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"v1.2.4-rc1", "v1.2.3", 1, true},  // the defect: this used to read as -1
		{"v1.2.3", "v1.2.4-rc1", -1, true}, // and so this looked like an upgrade
		{"v1.2.4", "v1.2.4-rc1", 1, true},  // a final release outranks its rc
		{"v1.2.4-rc1", "v1.2.4", -1, true},
		{"v1.2.4-rc1", "v1.2.4-rc1", 0, true},
		{"v1.2.4-rc1", "v1.2.4-rc2", 0, false}, // precedence we will not invent
		{"v1.2.3-", "v1.2.3", 0, false},
		{"v1.2.x", "v1.2.3", 0, false},
	}
	for _, c := range cases {
		got, ok := compareVersions(c.a, c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("compareVersions(%q,%q) = (%d,%v), want (%d,%v)", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

// End to end: the channel pins v1.2.3 while the machine runs v1.2.4-rc1 —
// a downgrade that must refuse without intent.
func TestRunRefusesADowngradeFromAPrerelease(t *testing.T) {
	blob := []byte("older")
	run := &fakeRelease{install: installSH("v1.2.3", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	target := installedBinary(t, "rc build")
	_, err := Run(context.Background(), Options{
		Current: "v1.2.4-rc1", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
	})
	if err == nil || !strings.Contains(err.Error(), "OLDER") {
		t.Fatalf("err = %v, want a downgrade refusal against a prerelease", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "rc build" {
		t.Errorf("the prerelease binary was replaced: %q", got)
	}
}

// "Last wins" must resolve the EFFECTIVE assignment, then validate it — a
// later block that blanks a key must not inherit an earlier release's value
// (#1031 review, P1).
func TestParsePinsUsesEffectiveAssignmentsNotLastValidOnes(t *testing.T) {
	oldSHA, newSHA := strings.Repeat("11", 32), strings.Repeat("22", 32)
	body := "PINNED_VERSION=\"v1.0.0\"\nMANIFEST_SHA256=\"" + oldSHA + "\"\nsha_darwin_arm64=\"" + oldSHA + "\"\n" +
		"PINNED_VERSION=\"v2.0.0\"\nMANIFEST_SHA256=\"\"\nsha_darwin_arm64=\"" + newSHA + "\"\n"
	if _, err := parsePins(body); err == nil || !strings.Contains(err.Error(), "MANIFEST_SHA256") {
		t.Fatalf("err = %v, want a refusal — v2.0.0 blanked the digest and must not inherit v1.0.0's", err)
	}

	// The same shape with a real digest resolves entirely to the last block.
	body = strings.Replace(body, "MANIFEST_SHA256=\"\"", "MANIFEST_SHA256=\""+newSHA+"\"", 1)
	p, err := parsePins(body)
	if err != nil {
		t.Fatalf("parsePins: %v", err)
	}
	if p.Version != "v2.0.0" || p.ManifestSHA != newSHA || p.PlatformSHAs["darwin-arm64"] != newSHA {
		t.Fatalf("pins = %+v, want every field from the LAST block", p)
	}
}

// An explicit --to installs any release, either direction, but only with the
// operator's own digest — install.sh's PYXIS_VERSION + PYXIS_SHA256 pairing
// (#1028's contract, restored). It records no release pin, and says so.
func TestRunExplicitTargetNeedsItsDigest(t *testing.T) {
	blob := []byte("explicit release")
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)}), blob: blob}
	base := Options{Current: "v9.9.9", Platform: "darwin-arm64", ConfigDir: t.TempDir(), HTTP: run}

	o := base
	o.To, o.TargetPath = "v9.9.7", installedBinary(t, "old")
	if _, err := Run(context.Background(), o); err == nil || !strings.Contains(err.Error(), "needs its sha256") {
		t.Fatalf("err = %v, want a refusal pairing --to with its digest", err)
	}
	o = base
	o.SHA256, o.TargetPath = sum(blob), installedBinary(t, "old")
	if _, err := Run(context.Background(), o); err == nil || !strings.Contains(err.Error(), "no target") {
		t.Fatalf("err = %v, want a refusal for a digest with no target", err)
	}
	o = base
	o.To, o.SHA256, o.TargetPath = "v9.9.7", "not-hex", installedBinary(t, "old")
	if _, err := Run(context.Background(), o); err == nil || !strings.Contains(err.Error(), "sha256 digest") {
		t.Fatalf("err = %v, want a malformed-digest refusal", err)
	}

	// An off-pin target also needs its MANIFEST digest, or the pin hand-off
	// could not honestly happen and `pyxis init` would resolve the old
	// release (#1031 review, P1).
	o = base
	o.To, o.SHA256, o.TargetPath = "v9.9.7", sum(blob), installedBinary(t, "old")
	if _, err := Run(context.Background(), o); err == nil || !strings.Contains(err.Error(), "--manifest-sha256") {
		t.Fatalf("err = %v, want a refusal naming the missing manifest digest", err)
	}

	// With all three it installs the older release without --allow-downgrade
	// (naming it IS the intent) AND records the pin for it.
	o = base
	o.To, o.SHA256, o.ManifestSHA256 = "v9.9.7", sum(blob), strings.Repeat("77", 32)
	o.TargetPath = installedBinary(t, "old")
	cfg := t.TempDir()
	o.ConfigDir = cfg
	rep, err := Run(context.Background(), o)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Target != "v9.9.7" || !rep.Downgrade || rep.PinRecorded == "" {
		t.Fatalf("report = %+v, want an explicit downgrade that DOES record its pin", rep)
	}
	env, _ := os.ReadFile(filepath.Join(cfg, "release.env"))
	if !strings.Contains(string(env), "PYXIS_SYSTEM_VERSION=v9.9.7") || !strings.Contains(string(env), strings.Repeat("77", 32)) {
		t.Errorf("release.env does not name the release just installed:\n%s", env)
	}
	if got, _ := os.ReadFile(o.TargetPath); string(got) != string(blob) {
		t.Errorf("the explicit release did not install: %q", got)
	}
	// A digest that does not match still refuses, explicit or not.
	o.SHA256 = strings.Repeat("ee", 32)
	o.TargetPath = installedBinary(t, "old")
	o.ConfigDir = t.TempDir()
	if _, err := Run(context.Background(), o); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want the checksum floor to hold for --to too", err)
	}
}

// A pin destination that cannot be written refuses BEFORE the binary is
// swapped, so a failure there cannot strand a new binary beside an old pin
// (#1031 review, P1).
func TestRunRefusesAnUnwritablePinBeforeReplacing(t *testing.T) {
	blob := []byte("new")
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": sum(blob)}), blob: blob}
	target := installedBinary(t, "old")
	// A regular file where the config DIRECTORY belongs.
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: blocked, HTTP: run,
	})
	if err == nil {
		t.Fatal("an unwritable pin destination was accepted")
	}
	if rep.BinaryReplaced {
		t.Error("the binary was replaced before the pin destination was checked")
	}
	if got, _ := os.ReadFile(target); string(got) != "old" {
		t.Errorf("the installed binary changed: %q", got)
	}
	for _, c := range run.calls {
		if strings.Contains(c, "/releases/download/") {
			t.Errorf("an unwritable pin destination still downloaded: %s", c)
		}
	}
}

// A fully specified target never consults the pins: an unreachable or malformed
// install.sh must not block installing a release the operator has already
// pinned by hand (#1031 review, P1).
func TestRunFullySpecifiedTargetSkipsDiscovery(t *testing.T) {
	blob := []byte("hand-pinned release")
	// Reading install.sh fails: discovery would break the run.
	run := &fakeRelease{blob: blob, failOn: "install.sh", fail: errors.New("install.sh unavailable")}
	cfg := t.TempDir()
	target := installedBinary(t, "old")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64",
		To: "v9.9.7", SHA256: sum(blob), ManifestSHA256: strings.Repeat("77", 32),
		TargetPath: target, ConfigDir: cfg, HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Target != "v9.9.7" || rep.PinRecorded == "" {
		t.Fatalf("report = %+v", rep)
	}
	for _, seen := range run.calls {
		if strings.Contains(seen, "install.sh") {
			t.Errorf("a fully specified target still read the pins: %s", seen)
		}
	}
	if got, _ := os.ReadFile(target); string(got) != string(blob) {
		t.Errorf("the hand-pinned release did not install: %q", got)
	}
}

// Re-running upgrade after a swap that failed to record the pin must REPAIR
// the pin — otherwise the advice printed on that failure ("re-run") is a
// dead end: the new binary reports current and returns (#1031 review, P1).
func TestRunRepairsAStalePinWhenAlreadyCurrent(t *testing.T) {
	run := &fakeRelease{install: installSH("v9.9.9", strings.Repeat("bb", 32), map[string]string{"darwin-arm64": strings.Repeat("dd", 32)})}
	cfg := t.TempDir()
	// The state a failed pin-write leaves behind: new binary, old pin.
	if err := os.WriteFile(filepath.Join(cfg, "release.env"),
		[]byte("PYXIS_SYSTEM_VERSION=v9.9.8\nPYXIS_MANIFEST_SHA256="+strings.Repeat("aa", 32)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := installedBinary(t, "already v9.9.9")

	rep, err := Run(context.Background(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: cfg, HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.UpToDate || !rep.PinRepaired || rep.PinRecorded == "" {
		t.Fatalf("report = %+v, want an up-to-date run that repaired the pin", rep)
	}
	env, _ := os.ReadFile(filepath.Join(cfg, "release.env"))
	if !strings.Contains(string(env), "PYXIS_SYSTEM_VERSION=v9.9.9") || !strings.Contains(string(env), strings.Repeat("bb", 32)) {
		t.Errorf("the stale pin was not repaired:\n%s", env)
	}
	if got, _ := os.ReadFile(target); string(got) != "already v9.9.9" {
		t.Errorf("an up-to-date run rewrote the binary: %q", got)
	}

	// Idempotent: a second run finds the pin correct and reports no repair.
	rep2, err := Run(context.Background(), Options{
		Current: "v9.9.9", Platform: "darwin-arm64",
		TargetPath: target, ConfigDir: cfg, HTTP: run,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep2.PinRepaired || rep2.PinRecorded != "" {
		t.Errorf("report = %+v, want no repair on an already-correct pin", rep2)
	}
}

// A manifest may never be BORROWED for bytes it does not describe (#1031
// review, P1). Matching the channel's VERSION is not enough: the discovered
// manifest describes the release whose binary the channel pins, so reusing
// it for a different --sha256 would record a pin describing something other
// than what was installed.
func TestRunRefusesToBorrowAManifestForOtherBytes(t *testing.T) {
	pinnedBlob := []byte("the release the channel pins") // digest S1
	otherBlob := []byte("different bytes, same version") // digest S2
	manifestM := strings.Repeat("1a", 32)
	run := &fakeRelease{
		install: installSH("v1.0.1", manifestM, map[string]string{"darwin-arm64": sum(pinnedBlob)}),
		blob:    otherBlob, // the channel serves the S2 bytes
	}
	base := Options{Current: "v1.0.0", Platform: "darwin-arm64", HTTP: run}

	// --to v1.0.1 --sha256 S2: same version, different bytes → refuse.
	o := base
	o.To, o.SHA256 = "v1.0.1", sum(otherBlob)
	o.TargetPath, o.ConfigDir = installedBinary(t, "old"), t.TempDir()
	_, err := Run(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), "not the digest the channel pins") {
		t.Fatalf("err = %v, want a refusal to borrow the manifest for other bytes", err)
	}
	if got, _ := os.ReadFile(o.TargetPath); string(got) != "old" {
		t.Errorf("bytes were installed despite the refusal: %q", got)
	}
	if _, serr := os.Stat(filepath.Join(o.ConfigDir, "release.env")); !os.IsNotExist(serr) {
		t.Error("a pin was recorded despite the refusal")
	}

	// The same target WITH its own manifest digest is the operator taking
	// responsibility for both halves: it installs and records that manifest.
	manifestM2 := strings.Repeat("2b", 32)
	o = base
	o.To, o.SHA256, o.ManifestSHA256 = "v1.0.1", sum(otherBlob), manifestM2
	o.TargetPath, o.ConfigDir = installedBinary(t, "old"), t.TempDir()
	if _, err := Run(context.Background(), o); err != nil {
		t.Fatalf("Run with the full tuple: %v", err)
	}
	env, _ := os.ReadFile(filepath.Join(o.ConfigDir, "release.env"))
	if strings.Contains(string(env), manifestM) || !strings.Contains(string(env), manifestM2) {
		t.Errorf("release.env recorded the borrowed manifest instead of the supplied one:\n%s", env)
	}

	// And --to naming the pinned release WITH its real digest still borrows
	// legitimately: these are that release's bytes.
	run.blob = pinnedBlob
	o = base
	o.To, o.SHA256 = "v1.0.1", sum(pinnedBlob)
	o.TargetPath, o.ConfigDir = installedBinary(t, "old"), t.TempDir()
	rep, err := Run(context.Background(), o)
	if err != nil {
		t.Fatalf("Run on the pinned release's own digest: %v", err)
	}
	env, _ = os.ReadFile(filepath.Join(o.ConfigDir, "release.env"))
	if !strings.Contains(string(env), manifestM) {
		t.Errorf("the pinned release's manifest was not recorded:\n%s", env)
	}
	if rep.Target != "v1.0.1" {
		t.Errorf("report = %+v", rep)
	}
}
