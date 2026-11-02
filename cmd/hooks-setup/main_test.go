package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainRelaysStatus(t *testing.T) {
	oldExit, oldArgs := processExit, os.Args
	t.Cleanup(func() { processExit, os.Args = oldExit, oldArgs })
	os.Args = []string{"hooks-setup", "unexpected"}
	code := -1
	processExit = func(got int) { code = got }
	main()
	if code != 2 {
		t.Fatalf("main status = %d", code)
	}
}

func TestSetupCLI(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var output bytes.Buffer
	if code := run(t.Context(), []string{"unexpected"}, &output); code != 2 {
		t.Fatalf("usage status = %d", code)
	}
	if code := run(t.Context(), nil, &output); code != 1 {
		t.Fatalf("non-repository status = %d", code)
	}
	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if code := run(t.Context(), nil, &output); code != 0 {
		t.Fatalf("setup status = %d: %s", code, &output)
	}
	out, err := exec.Command("git", "config", "core.hooksPath").Output()
	if err != nil || !strings.Contains(string(out), "pyxis-guard-hooks") {
		t.Fatalf("configured hooks = %q, %v", out, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code := run(ctx, nil, &output); code != 1 {
		t.Fatalf("cancelled setup status = %d", code)
	}
}
