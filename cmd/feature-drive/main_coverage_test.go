package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMainDelegatesFailureExit(t *testing.T) {
	oldArgs, oldExit := os.Args, processExit
	t.Cleanup(func() {
		os.Args = oldArgs
		processExit = oldExit
	})
	os.Args = []string{"feature-drive"}
	got := -1
	processExit = func(code int) { got = code }
	main()
	if got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
}

func TestRunRejectsInvalidArgumentsAndWorkspace(t *testing.T) {
	for _, args := range [][]string{nil, {"dogfood", "--unknown"}, {"dogfood"}} {
		if err := run(context.Background(), args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%v) succeeded", args)
		}
	}
	root := t.TempDir()
	workspace := filepath.Join(root, "file")
	if err := os.WriteFile(workspace, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"dogfood", "--workspace", workspace}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("run accepted a regular file as its workspace")
	}
}
