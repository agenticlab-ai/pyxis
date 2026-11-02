package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The test binary stands in for an installed pyxis when this is set: one mode
// per release behaviour InstalledShellSetup has to tell apart.
const installedPyxisMode = "PYXIS_UPGRADE_TEST_INSTALLED_PYXIS"

func TestMain(m *testing.M) {
	if mode := os.Getenv(installedPyxisMode); mode != "" {
		os.Exit(installedPyxis(mode, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func installedPyxis(mode string, args []string) int {
	help := strings.Join(args, " ") == "shell install --help"
	if strings.Join(args, " ") != "shell install" && !help {
		fmt.Fprintf(os.Stderr, "unexpected arguments %q\n", args)
		return 64
	}
	switch {
	case mode == "predates-verb" && help:
		fmt.Fprintln(os.Stderr, "pyxis: shell: unknown verb")
		return 2
	case mode == "predates-verb":
		fmt.Println("pyxis: installed without a supporting verb")
		return 0
	case help:
		return 0
	case mode == "fails":
		fmt.Fprintln(os.Stderr, "pyxis: shell: unfinished managed block\nsecond line")
		return 1
	case mode == "fails-silently":
		return 1
	}
	fmt.Println("pyxis: interactive worktrees enabled for zsh in /home/u/.zshrc\nOpen a new zsh terminal to activate them.")
	return 0
}

func TestInstalledShellSetupRunsTheInstalledBinary(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ mode, want, refusal string }{
		{mode: "installs", want: "interactive worktrees enabled for zsh in /home/u/.zshrc"},
		{mode: "predates-verb"},
		{mode: "fails", refusal: "unfinished managed block"},
		{mode: "fails-silently", refusal: "exit status 1"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv(installedPyxisMode, tc.mode)
			got, err := InstalledShellSetup(context.Background(), binary)
			if tc.refusal == "" && (err != nil || got != tc.want) {
				t.Fatalf("account=%q err=%v, want %q", got, err, tc.want)
			}
			if tc.refusal != "" && (err == nil || got != "" || !strings.Contains(err.Error(), tc.refusal) || strings.Contains(err.Error(), "second line")) {
				t.Fatalf("account=%q err=%v, want a refusal naming %q", got, err, tc.refusal)
			}
		})
	}
}

// The launcher ships inside each release's binary. Upgrade reinstalls it from
// the binary it has just put in place, after the pin, and reports the account.
func TestRunReinstallsTheShellFromTheReplacedBinary(t *testing.T) {
	blob := []byte("new pyxis")
	pins := map[string]string{"darwin-arm64": sum(blob)}
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), pins), blob: blob}
	target, cfgDir := installedBinary(t, "old"), t.TempDir()
	calls := 0
	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", TargetPath: target, ConfigDir: cfgDir, HTTP: run,
		ShellSetup: func(_ context.Context, binary string) (string, error) {
			calls++
			installed, readErr := os.ReadFile(binary)
			if readErr != nil || string(installed) != string(blob) {
				t.Errorf("shell setup ran from %s before the new binary was in place (%q, %v)", binary, installed, readErr)
			}
			if _, statErr := os.Stat(filepath.Join(cfgDir, "release.env")); statErr != nil {
				t.Errorf("shell setup ran before the pin was recorded: %v", statErr)
			}
			return "interactive worktrees enabled for zsh in /home/u/.zshrc", nil
		},
	})
	if err != nil || calls != 1 || rep.Shell != "interactive worktrees enabled for zsh in /home/u/.zshrc" {
		t.Fatalf("calls=%d report=%+v err=%v", calls, rep, err)
	}
}

// A machine already on the release still gets its launcher brought in line, so
// re-running after a failed shell step repairs it. Preview performs nothing.
func TestRunReinstallsTheShellWhenAlreadyCurrentButNeverInPreview(t *testing.T) {
	blob := []byte("new pyxis")
	pins := map[string]string{"darwin-arm64": sum(blob)}
	for _, tc := range []struct {
		name, current string
		preview       bool
		want          int
	}{
		{"already current", "v9.9.9", false, 1},
		{"already current preview", "v9.9.9", true, 0},
		{"upgrade preview", "v9.9.8", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), pins), blob: blob}
			target, err := filepath.EvalSymlinks(installedBinary(t, "old"))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			rep, err := Run(context.Background(), Options{
				Current: tc.current, Platform: "darwin-arm64", TargetPath: target, ConfigDir: t.TempDir(), HTTP: run, Preview: tc.preview,
				ShellSetup: func(_ context.Context, binary string) (string, error) {
					calls++
					if binary != target {
						t.Errorf("shell setup ran from %s, want the installed %s", binary, target)
					}
					return "current", nil
				},
			})
			if err != nil || calls != tc.want || (rep.Shell != "") != (tc.want == 1) {
				t.Fatalf("calls=%d report=%+v err=%v", calls, rep, err)
			}
		})
	}
}

// A failed shell step comes after the swap and the pin: the report says both
// happened and the refusal names the command that retries only this step.
func TestRunDisclosesAShellFailureAfterTheSwap(t *testing.T) {
	blob := []byte("new pyxis")
	pins := map[string]string{"darwin-arm64": sum(blob)}
	run := &fakeRelease{tag: "v9.9.9", install: installSH("v9.9.9", strings.Repeat("bb", 32), pins), blob: blob}
	target := installedBinary(t, "old")
	rep, err := Run(context.Background(), Options{
		Current: "v9.9.8", Platform: "darwin-arm64", TargetPath: target, ConfigDir: t.TempDir(), HTTP: run,
		ShellSetup: func(context.Context, string) (string, error) { return "", errors.New("unfinished managed block") },
	})
	if err == nil || !rep.BinaryReplaced || rep.PinRecorded == "" || rep.Shell != "" {
		t.Fatalf("report=%+v err=%v", rep, err)
	}
	for _, want := range []string{"v9.9.9 is installed", "unfinished managed block", target + " shell install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks %q: %v", want, err)
		}
	}
}
