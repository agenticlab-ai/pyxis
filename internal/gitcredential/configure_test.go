package gitcredential

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func TestConfigureScopesGitHelperAndRestoresPriorValues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git credential executable fixture")
	}
	root := credentialRoot(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	gitTest(t, root, "init", "--quiet")
	gitTest(t, root, "config", "credential.helper", "!printf 'username=ambient\\npassword=ambient\\n'")
	priorKey := "credential.https://forge.example:8443/base/owner/code.git.helper"
	gitTest(t, root, "config", "--add", priorKey, "old-helper-one")
	gitTest(t, root, "config", "--add", priorKey, "old-helper-two")
	exe := filepath.Join(t.TempDir(), "pyxis' literal $(touch should-not-exist)")
	stub := "#!/bin/sh\n[ \"$1\" = credential ] || exit 1\n[ \"$2\" = --repo ] || exit 1\n[ \"$4\" = get ] || exit 0\nprintf 'username=bound\\npassword=bound-test-token\\n\\n'\n"
	if err := os.WriteFile(exe, []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	changes, err := Plan(context.Background(), root, exe)
	if err != nil || len(changes) != 4 {
		t.Fatalf("plan count %d err %v", len(changes), err)
	}
	if got := gitTest(t, root, "config", "--get-all", priorKey); got != "old-helper-one\nold-helper-two\n" {
		t.Fatal("plan mutated configuration")
	}
	changes, err = Configure(context.Background(), root, exe)
	if err != nil {
		t.Fatal(err)
	}
	if later, err := Plan(context.Background(), root, exe); err != nil || len(later) != 0 {
		t.Fatalf("non-idempotent plan %#v %v", later, err)
	}
	fill := exec.Command("git", "-C", root, "credential", "fill")
	fill.Stdin = strings.NewReader("protocol=https\nhost=forge.example:8443\npath=base/owner/code.git\n\n")
	out, err := fill.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("password=bound-test-token")) || bytes.Contains(out, []byte("password=ambient")) {
		t.Fatalf("scoped helper did not own credential request: err %v", err)
	}
	config, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(config, []byte("bound-test-token")) {
		t.Fatal("token persisted in git config")
	}
	if _, err := os.Stat(filepath.Join(root, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("helper path was interpreted as shell code")
	}
	if err := Restore(context.Background(), root, changes); err != nil {
		t.Fatal(err)
	}
	if got := gitTest(t, root, "config", "--get-all", priorKey); got != "old-helper-one\nold-helper-two\n" {
		t.Fatalf("prior helper values not restored: %q", got)
	}
	if got := gitTest(t, root, "config", "credential.helper"); !strings.Contains(got, "ambient") {
		t.Fatal("unrelated credential configuration changed")
	}
}

func TestCredentialPlanBeforeGitInitAndRefusesParentRepository(t *testing.T) {
	root := credentialRoot(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	changes, err := Plan(context.Background(), root, executable)
	if err != nil || len(changes) != 4 {
		t.Fatalf("pre-init plan %d %v", len(changes), err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatal("credential preview initialized Git")
	}
	if _, err := Configure(context.Background(), root, executable); err == nil {
		t.Fatal("configured non-repository")
	}
	gitTest(t, root, "init", "--quiet")
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	profile, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ConfigureForProfile(context.Background(), nested, executable, profile); err == nil {
		t.Fatal("nested operand altered parent Git configuration")
	}
}
