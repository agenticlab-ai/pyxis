package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/release"
)

// The I3 argument discipline at the verb edge: unknown verbs, unknown
// flags, and stray positionals all refuse pre-action with exit 2.
func TestRunRefusesMangledInvocations(t *testing.T) {
	cases := [][]string{
		{},                      // no verb
		{"frobnicate"},          // unknown verb
		{"pack", "--bogus"},     // unknown flag
		{"tools", "positional"}, // stray positional (verbs take flags only)
		{"verify", "--out", "x", "extra"},
		{"publish", "-x"},
		{"mirror", "positional"},
		{"pins", "positional"},
		{"pins", "--bogus"},
		{"cut", "positional"},
		{"cut", "--bogus"},
		{"build", "positional"},
		{"mirror", "--dist-repo", "example/retired"},
		{"cut", "--dist-repo", "example/retired"},
		{"publish", "--repo", "example/retired"},
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 {
			t.Errorf("run(%v) = %d, want 2 (refuse pre-action)\n%s", args, code, errb.String())
		}
	}
}

// Semantic failures (a bad version, a missing artifact set) exit 1 with the
// library's named error.
func TestRunSurfacesNamedErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"pack", "--version", "nope", "--out", t.TempDir(), "--binaries", t.TempDir()}, &out, &errb); code != 1 {
		t.Fatalf("pack with a bad version = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "not a system tag") {
		t.Fatalf("stderr = %q, want the version refusal", errb.String())
	}
	errb.Reset()
	if code := run([]string{"verify", "--version", "v1.2.3", "--out", t.TempDir()}, &out, &errb); code != 1 {
		t.Fatalf("verify on an empty dir = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "SHA256SUMS") {
		t.Fatalf("stderr = %q, want the missing-SHA256SUMS refusal", errb.String())
	}
}

// Distribution publishes only to an explicit binding; the refusal comes before
// any stage, and names what is missing and that plain cut is complete.
func TestDistributionRequiresAnExplicitBinding(t *testing.T) {
	for _, args := range [][]string{
		{"mirror", "--tag", "v1.2.3", "--repo-root", t.TempDir()},
		{"cut", "--version", "v1.2.3", "--mirror", "--repo-root", t.TempDir()},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 1 || !strings.Contains(errb.String(), "releases.distribution") || !strings.Contains(errb.String(), "pyxis-release cut") || out.Len() != 0 {
			t.Fatalf("run(%v) = %d, stdout %q, stderr %q; want the binding refusal before any stage", args, code, out.String(), errb.String())
		}
	}
}

// The retired distribution inputs are gone, not ignored: the flag grammar is
// closed, so a stale invocation refuses instead of silently publishing nothing.
func TestRetiredDistributionFlagsRefuse(t *testing.T) {
	for _, args := range [][]string{
		{"mirror", "--r2-endpoint", "https://example.test"},
		{"mirror", "--r2-bucket", "b"},
		{"cut", "--r2-endpoint", "https://example.test"},
		{"mirror", "--require-source-binding"},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 {
			t.Errorf("run(%v) = %d, want 2\n%s", args, code, errb.String())
		}
	}
}

// Plain cut names what remains; a bound target adds its own step, and a public
// release is otherwise done once the pin PR merges.
func TestCutOutcomeNamesWhatRemains(t *testing.T) {
	for _, tc := range []struct {
		name          string
		res           release.CutResult
		bound         bool
		want, without string
	}{
		{"pins written, public", release.CutResult{Pins: release.PinsResult{Action: "written"}}, false, "the pin PR", "--mirror"},
		{"pins written, bound", release.CutResult{Pins: release.PinsResult{Action: "written"}}, true, "--mirror", ""},
		{"pins current, public", release.CutResult{Pins: release.PinsResult{Action: "current"}}, false, "DONE", "--mirror"},
		{"pins current, bound", release.CutResult{Pins: release.PinsResult{Action: "current"}}, true, "--mirror", "DONE"},
		{"mirrored", release.CutResult{Mirror: &release.MirrorResult{}}, true, "DONE", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			printCutOutcome(&out, "v1.2.3", tc.res, tc.bound)
			if got := out.String(); !strings.Contains(got, tc.want) || (tc.without != "" && strings.Contains(got, tc.without)) {
				t.Fatalf("outcome %q, want %q and not %q", got, tc.want, tc.without)
			}
		})
	}
}

// pins refuses when neither source is available, and never invents an
// install.sh — the verb edge passes both legs' inputs through.
func TestRunPinsRefusesWithoutAnInstallSH(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	code := run([]string{"pins", "--version", "v1.2.3", "--repo-root", root, "--out", t.TempDir()}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "install.sh") {
		t.Fatalf("run = %d, stderr %q; want 1 naming install.sh", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "install.sh")); !os.IsNotExist(err) {
		t.Fatal("pins created an install.sh where none existed")
	}
}

// cut refuses on a bad version before touching the toolchain, the network,
// or the filesystem — including the default artifact dir.
func TestRunCutRefusesBadVersion(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	code := run([]string{"cut", "--version", "0.1.2", "--repo-root", root}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "not a system tag") {
		t.Fatalf("run = %d, stderr %q; want 1 with the version refusal", code, errb.String())
	}
	for _, dir := range []string{release.DefaultOutDir, filepath.Join(root, release.DefaultOutDir)} {
		if _, err := os.Stat(dir); err == nil {
			t.Fatalf("the refused run created %s", dir)
		}
	}
}

// The tools verb prints the scan-derived shipped set, one per line — the
// workflow's matrix input.
func TestRunToolsPrintsScan(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"toola", "toolb", "pyxis-release"} {
		if err := os.MkdirAll(filepath.Join(root, "cmd", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"tools", "--repo-root", root}, &out, &errb); code != 0 {
		t.Fatalf("tools = %d: %s", code, errb.String())
	}
	if got := out.String(); got != "toola\ntoolb\n" {
		t.Fatalf("tools output = %q", got)
	}
}

func TestRunHelpAndEveryActionErrorSurface(t *testing.T) {
	for _, verb := range []string{"help", "--help", "-h"} {
		var out, errb bytes.Buffer
		if code := run([]string{verb}, &out, &errb); code != 0 || !strings.Contains(errb.String(), "usage:") {
			t.Fatalf("run(%q) = %d, stderr %q", verb, code, errb.String())
		}
	}

	missing := filepath.Join(t.TempDir(), "missing")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "tools", args: []string{"tools", "--repo-root", missing}, want: "cmd"},
		{name: "build", args: []string{"build", "--version", "bad", "--out", t.TempDir(), "--repo-root", t.TempDir()}, want: "not a system tag"},
		{name: "publish", args: []string{"publish", "--tag", "bad", "--out", t.TempDir()}, want: "not a system tag"},
		{name: "mirror", args: []string{"mirror", "--tag", "bad", "--out", t.TempDir(), "--repo-root", t.TempDir()}, want: "not a system tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(tc.args, &out, &errb); code != 1 || !strings.Contains(errb.String(), tc.want) {
				t.Fatalf("run(%v) = %d, stderr %q; want exit 1 containing %q", tc.args, code, errb.String(), tc.want)
			}
		})
	}
}

func TestMainDelegatesExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"pyxis-release", "help"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 0 {
		t.Fatalf("main exit = %d, want 0", got)
	}
}

func TestRunRendersEverySuccessfulReleaseAction(t *testing.T) {
	oldTools, oldBuild, oldPack := releaseShippedTools, releaseBuild, releasePack
	oldVerify, oldPublish, oldPins := releaseVerify, releasePublish, releasePins
	oldMirror, oldCut := releaseMirror, releaseCut
	t.Cleanup(func() {
		releaseShippedTools, releaseBuild, releasePack = oldTools, oldBuild, oldPack
		releaseVerify, releasePublish, releasePins = oldVerify, oldPublish, oldPins
		releaseMirror, releaseCut = oldMirror, oldCut
	})
	releaseShippedTools = func(string) ([]string, error) { return []string{"pyxis"}, nil }
	releaseBuild = func(context.Context, release.BuildOptions) (release.BuildResult, error) {
		return release.BuildResult{Tools: []string{"pyxis"}, Binaries: []string{"pyxis-v1"}}, nil
	}
	releasePack = func(context.Context, release.PackOptions) (release.PackResult, error) {
		return release.PackResult{Artifacts: []string{"manifest", "sums"}}, nil
	}
	releaseVerify = func(string, string) error { return nil }
	releasePublish = func(context.Context, release.PublishOptions) (release.PublishResult, error) {
		return release.PublishResult{Created: true, Actions: map[string]string{"manifest": "uploaded"}}, nil
	}
	releasePins = func(context.Context, release.PinsOptions) (release.PinsResult, error) {
		return release.PinsResult{Action: "written", Source: "fixture"}, nil
	}
	releaseMirror = func(context.Context, release.MirrorOptions) (release.MirrorResult, error) {
		return release.MirrorResult{Downloaded: true, InstallSH: "written"}, nil
	}
	cutCall := 0
	releaseCut = func(context.Context, release.CutOptions) (release.CutResult, error) {
		cutCall++
		switch cutCall {
		case 1:
			return release.CutResult{Mirror: &release.MirrorResult{}}, nil
		case 2:
			return release.CutResult{Pins: release.PinsResult{Action: "written"}}, nil
		default:
			return release.CutResult{Pins: release.PinsResult{Action: "current"}}, nil
		}
	}

	cases := [][]string{
		{"tools"},
		{"build", "--version", "v1.2.3", "--out", "out"},
		{"pack", "--version", "v1.2.3", "--out", "out", "--binaries", "bin"},
		{"verify", "--version", "v1.2.3", "--out", "out"},
		{"publish", "--tag", "v1.2.3", "--out", "out"},
		{"pins", "--version", "v1.2.3"},
		{"mirror", "--tag", "v1.2.3", "--out", "out"},
		{"cut", "--version", "v1.2.3"},
		{"cut", "--version", "v1.2.3"},
		{"cut", "--version", "v1.2.3"},
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("run(%v) = %d, stderr %q", args, code, errb.String())
		}
		if out.Len() == 0 {
			t.Fatalf("run(%v) emitted no success summary", args)
		}
	}
}
