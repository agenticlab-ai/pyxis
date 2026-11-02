package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
)

var serveCredential = gitcredential.Serve

// credentialVerb has no invocation log, telemetry, or diagnostic tee: stdout
// is a private Git protocol channel containing an ephemeral credential.
func credentialVerb(args []string, input io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("credential", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", "", "repository whose code-host binding owns the credential")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *root == "" || fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: pyxis credential --repo <path> get|store|erase|configure") // Best-effort diagnostic; usage still refuses if its sink fails.
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if fs.Arg(0) == "configure" {
		changes, err := gitcredential.Configure(ctx, *root, "")
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "pyxis:", err) // Best-effort diagnostic for an already-failed command.
			return 1
		}
		if _, err := fmt.Fprintf(stdout, "configured %d repository-scoped Git credential settings\n", len(changes)); err != nil {
			return 1
		}
		return 0
	}
	if err := serveCredential(ctx, *root, fs.Arg(0), input, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, "pyxis:", err) // Best-effort diagnostic for an already-failed credential request.
		return 1
	}
	return 0
}
