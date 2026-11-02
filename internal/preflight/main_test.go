package preflight_test

// A Git-only PATH keeps preflight independent of locally installed runtimes.
// Toolchain refusal cases override PATH with their own version fixtures.

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "preflight-hermetic-path-*")
	if err != nil {
		panic(err)
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		panic("preflight tests need git on PATH: " + err.Error())
	}
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		panic(err)
	}
	if err := os.Setenv("PATH", dir); err != nil {
		panic(err)
	}

	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		panic(err)
	}
	os.Exit(code)
}

// writeFakeVersionTool writes a tiny scripted stand-in at dir/name whose
// `--version` (or any-argument) invocation just echoes version — enough for
// the toolchain check's presence+parse probe without a real install.
func writeFakeVersionTool(dir, name, version string) {
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho '" + version + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		panic(err)
	}
}
