package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/cli"
)

func TestMainDispatchesGate(t *testing.T) {
	if len(processArgs()) != len(os.Args)-1 {
		t.Fatal("process arguments were not forwarded")
	}
	var stderr bytes.Buffer
	code := -1
	oldExit, oldArgs, oldEnv := processExit, processArgs, processEnv
	t.Cleanup(func() { processExit, processArgs, processEnv = oldExit, oldArgs, oldEnv })
	processExit = func(c int) { code = c }
	processArgs = func() []string { return []string{"--unknown"} }
	processEnv = func() cli.Env { return cli.Env{Stderr: &stderr} }
	main()
	if code != 2 || !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("main = %d: %s", code, stderr.String())
	}
}
