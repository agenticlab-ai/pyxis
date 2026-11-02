package installer_test

// The conditional GitHub-coordinate exchange and the apply-time remote
// materialization (design init-hardening-v1 §3.7; plan init-hardening/001
// T5; prd:init-hardening FR-6/FR-8, ABUSE-1's re-ask, prd:installer FR-23's
// conditional-exchange boundary) — driven black-box through installer.Init
// with a live prompter and the hermetic gh transport, the same harness the
// FR-12/FR-13 interactive tests use.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// gitIn runs one git subcommand in dir, returning combined output.
func gitIn(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	return string(out), err
}

// mustGit is gitIn or a fatal.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := gitIn(t, dir, args...); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// coordinateOptions is interactiveOptions WITHOUT a promptless coordinate:
// the exchange's own elicitation path is the surface under test, so the
// coordinate must arrive from stdin (or not at all), never from Answers.
func coordinateOptions(r release, target, stdin string, stdout *bytes.Buffer) installer.Options {
	return installer.Options{
		Target:         target,
		Pin:            installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource: "file://" + r.src,
		WireRunner:     hermeticGH{},
		Stdin:          strings.NewReader(stdin),
		Stdout:         stdout,
		Logger:         quiet(),
	}
}

// The interactive script, in the order the interview now asks (#1043): the
// repository question comes FIRST, and its "create a new GitHub repository"
// branch is what elicits the coordinate. These fixtures see no empty
// repository (hermeticGH's listing is empty), so the menu is create/local
// and a bare Enter takes create.
const (
	// chooseCreate picks the menu's create entry. The menu carries NO
	// default (an answer naming an external destination must be chosen, not
	// inherited from a closed stdin), so this is an explicit "1".
	chooseCreate = "1\n"
	// remainingDefaults answers the five questions that follow the
	// repository decision: tracker · ci_mode · protection · argus ·
	// surfaces, each a bare Enter.
	remainingDefaults = "\n\n\n\n\n"
)

func TestCoordinateElicitedFoldedEchoedAndMaterialized(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	rep, err := installer.Init(t.Context(), coordinateOptions(r, target, chooseCreate+"octo/created\n"+remainingDefaults, &out))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if len(rep.Commits) == 0 {
		t.Fatal("Init reported no commits — not a green install")
	}
	// The user just typed the coordinate — no echo line repeats it (#898);
	// the wiring checklist header is where the resolved slug stays visible.
	if strings.Contains(out.String(), "GitHub coordinate: octo/created") {
		t.Errorf("stdout re-echoes the coordinate the user just typed:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "wiring GitHub (octo/created):") {
		t.Errorf("the wiring checklist does not name the resolved coordinate:\n%s", out.String())
	}
	url, gerr := gitIn(t, target, "remote", "get-url", "origin")
	if gerr != nil {
		t.Fatalf("remote get-url: %v", gerr)
	}
	if strings.TrimSpace(url) != wire.RemoteURL("octo/created") {
		t.Errorf("remote url = %q, want %q (stageRemote materializes at apply)", strings.TrimSpace(url), wire.RemoteURL("octo/created"))
	}
}

func TestCoordinateMalformedReasksThenAccepts(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	stdin := chooseCreate + "not a coordinate!!\n" + "octo/created\n" + remainingDefaults
	if _, err := installer.Init(t.Context(), coordinateOptions(r, target, stdin, &out)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "is not a valid owner/name coordinate") {
		t.Errorf("stdout carries no re-ask notice (ABUSE-1):\n%s", out.String())
	}
	if !strings.Contains(out.String(), "wiring GitHub (octo/created):") {
		t.Errorf("the corrected coordinate did not reach the wiring surface:\n%s", out.String())
	}
}

func TestCoordinateThreeMalformedAbortsPreWrite(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	stdin := chooseCreate + "bad one!\n" + "bad two!\n" + "bad three!\n"
	_, err := installer.Init(t.Context(), coordinateOptions(r, target, stdin, &out))
	if err == nil {
		t.Fatal("Init succeeded on three malformed coordinates, want the bounded abort")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("err = %v, want the bounded-abort refusal naming the collected answers", err)
	}
	entries, derr := os.ReadDir(target)
	if derr != nil {
		t.Fatalf("read target: %v", derr)
	}
	if len(entries) != 0 {
		t.Errorf("target holds %d entries after an elicitation abort, want 0 (DINV-1: zero writes)", len(entries))
	}
}

func TestCoordinateRemotePresentAsksNothing(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	mustGit(t, target, "init", "-b", "main")
	mustGit(t, target, "remote", "add", "origin", wire.RemoteURL("octo/widget"))

	var out bytes.Buffer
	rep, err := installer.Init(t.Context(), coordinateOptions(r, target, remainingDefaults, &out))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if len(rep.Commits) == 0 {
		t.Fatal("not a green install")
	}
	if strings.Contains(out.String(), "GitHub repository (owner/name)") {
		t.Errorf("the exchange fired despite a configured remote (FR-6: origin-present runs ask nothing):\n%s", out.String())
	}
	if !strings.Contains(out.String(), "GitHub coordinate: octo/widget") {
		t.Errorf("the remote-resolved coordinate was not echoed (FR-8):\n%s", out.String())
	}
	url, _ := gitIn(t, target, "remote", "get-url", "origin")
	if strings.TrimSpace(url) != wire.RemoteURL("octo/widget") {
		t.Errorf("pre-configured remote changed: %q (AP-22: materialization is idempotent)", strings.TrimSpace(url))
	}
}

func TestCoordinateBlankFallsThroughToTheNamedRefusal(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var out bytes.Buffer
	_, err := installer.Init(t.Context(), coordinateOptions(r, target, chooseCreate+"\n"+remainingDefaults, &out))
	if err == nil {
		t.Fatal("Init succeeded with no coordinate anywhere, want the GitHub-mode check's refusal")
	}
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) || refusal.Outcome.Check != preflight.GitHubModeID {
		t.Fatalf("err = %v, want the preflight GitHub-mode refusal (FR-7 stays T3's, never duplicated)", err)
	}
	entries, derr := os.ReadDir(target)
	if derr != nil {
		t.Fatalf("read target: %v", derr)
	}
	if len(entries) != 0 {
		t.Errorf("target holds %d entries after a pre-write refusal, want 0 (DINV-1)", len(entries))
	}
}
