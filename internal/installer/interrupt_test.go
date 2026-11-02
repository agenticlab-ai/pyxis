package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An interrupt lands as context cancellation (cmd/pyxis turns SIGINT into
// exactly that). The next question must refuse by name instead of carrying
// the interview on against a dead run — the #954 defect, where every
// remaining knob was still asked after Ctrl-C.
func TestAskRefusesOnceInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out strings.Builder
	pr := newPrompter(ctx, strings.NewReader("github\ncarina\n"), &out)

	if got, err := pr.ask("first question", "github"); err != nil || got != "github" {
		t.Fatalf("pre-interrupt ask = (%q, %v), want the typed answer", got, err)
	}

	cancel()

	got, err := pr.ask("second question", "carina")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("post-interrupt ask = (%q, %v), want a cancellation refusal", got, err)
	}
	if got != "" {
		t.Errorf("an interrupted ask returned the answer %q — it must resolve nothing", got)
	}
	if strings.Contains(out.String(), "second question") {
		t.Errorf("the interrupted question was still put to the user:\n%s", out.String())
	}
}

// The whole interview stops at the first question after the interrupt: no
// knob silently takes its default, and the error is the named cancellation.
func TestResolveKnobsStopsAtTheInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out strings.Builder
	pr := newPrompter(ctx, strings.NewReader("github\nboth\ncreate\n"), &out)

	opts := &Options{Stdin: strings.NewReader(""), Stdout: &out}
	_, err := resolveKnobs(ctx, opts, pr)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveKnobs on a cancelled run = %v, want a cancellation refusal", err)
	}
	if strings.Contains(out.String(), "issue tracker") {
		t.Errorf("a cancelled run still asked its first question:\n%s", out.String())
	}
}

// The three identity outcomes stay distinct (#954): a real value, a
// git-confirmed absence, and a probe that could not run. Conflating the
// third with the second is what made init ask for a committer name that
// `git config` answers fine.
func TestAmbientIdentitySeparatesAbsenceFromProbeFailure(t *testing.T) {
	isolateGitEnv(t)
	dir := t.TempDir()

	// A git-confirmed absence: the key is unset, git exits 1, no error.
	v, found, err := ambientIdentity(context.Background(), dir, "user.name")
	if err != nil || found || v != "" {
		t.Fatalf("unset key = (%q, %v, %v), want a clean absence", v, found, err)
	}

	// A real value.
	gitCfg(t, dir, "user.name", "Ada Lovelace")
	v, found, err = ambientIdentity(context.Background(), dir, "user.name")
	if err != nil || !found || v != "Ada Lovelace" {
		t.Fatalf("set key = (%q, %v, %v), want the configured value", v, found, err)
	}

	// A probe that could not run: the cancelled context of an interrupted
	// run. This must NOT read as "unset" — that is the false trigger.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	v, found, err = ambientIdentity(cancelled, dir, "user.name")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled probe = (%q, %v, %v), want the cancellation surfaced", v, found, err)
	}
	if found {
		t.Error("a failed probe reported a value")
	}
}

// The consequence, end to end: an interrupted run must never prompt for an
// identity git already holds — it stops with the cancellation instead.
func TestResolveIdentityDoesNotPromptOnAnInterruptedRun(t *testing.T) {
	isolateGitEnv(t)
	dir := t.TempDir()
	gitCfg(t, dir, "user.name", "Ada Lovelace")
	gitCfg(t, dir, "user.email", "ada@example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out strings.Builder
	pr := newPrompter(ctx, strings.NewReader("Someone Else\nelse@example.com\n"), &out)

	_, err := resolveIdentity(ctx, dir, Answers{}, pr)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolveIdentity on a cancelled run = %v, want the cancellation", err)
	}
	if strings.Contains(out.String(), "committer") {
		t.Errorf("an interrupted run asked for committer identity it already had:\n%s", out.String())
	}
}

// isolateGitEnv pins git to the test's own scopes so a developer's real
// ~/.gitconfig can neither satisfy nor break these probes.
func isolateGitEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("HOME", home)
}

// gitCfg writes one repo-local config value, initializing the repository on
// first use.
func gitCfg(t *testing.T, dir, key, value string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if out, gerr := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); gerr != nil {
			t.Fatalf("git init: %v\n%s", gerr, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "config", key, value).CombinedOutput(); err != nil {
		t.Fatalf("git config %s: %v\n%s", key, err, out)
	}
}
