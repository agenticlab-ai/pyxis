package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/cli"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// TestMainDispatchesToTheCLI drives main through its process seams: the exit
// code and the streams are captured instead of ending the test process.
func TestMainDispatchesToTheCLI(t *testing.T) {
	if got := processArgs(); len(got) != len(os.Args)-1 {
		t.Fatalf("the default args are the process args without the binary: %v", got)
	}
	var out, errb bytes.Buffer
	code := -1
	processExit = func(c int) { code = c }
	processArgs = func() []string { return []string{"help"} }
	processEnv = func() cli.Env {
		return cli.Env{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Dir: t.TempDir(), Now: time.Now, Entropy: record.Entropy, Getenv: os.Getenv}
	}
	t.Cleanup(func() {
		processExit, processArgs, processEnv = os.Exit, func() []string { return os.Args[1:] }, cli.OSEnv
	})
	main()
	if code != 2 || !strings.Contains(errb.String(), "usage: memory <verb>") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	// the real environment resolves the working directory and the process streams
	env := cli.OSEnv()
	if env.Dir == "" || env.Stdout != os.Stdout || env.Stderr != os.Stderr || env.Stdin != os.Stdin || env.Now == nil || env.Entropy == nil || env.Getenv == nil {
		t.Fatalf("OSEnv: %+v", env)
	}
}
