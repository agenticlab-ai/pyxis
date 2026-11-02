package pack

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	reviewruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

// RunContextIDCommand exposes the existing provenance derivation to a native
// reviewer without requiring an interpreter or compiling proposal source.
func RunContextIDCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("internal-review-context", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("reviewer-root", "", "actual native reviewer root identity")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return 2
	}
	if err := ctx.Err(); err != nil {
		return 1
	}
	id, err := reviewruntime.ContextID(*root)
	if err != nil {
		// Best-effort diagnostic; the exit status carries the failure.
		_, _ = fmt.Fprintln(stderr, "trusted review context:", err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, id); err != nil {
		return 1
	}
	return 0
}

// RunCommand is the host executable's private pack-assembly entrypoint. Its
// source mapping and assembler are compiled into trusted host code; proposal
// source, declarations, caches and PATH are never used to select executable code.
func RunCommand(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("internal-review-pack", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", "", "private input root")
	subject := flags.String("subject", "", "proposal reference")
	out := flags.String("out", "", "private output file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *root == "" || *subject == "" || *out == "" {
		return 2
	}
	b, err := BuildFromRoot(ctx, *root, *subject)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(*out), 0o700)
	}
	if err == nil {
		err = os.WriteFile(*out, b, 0o600)
	}
	if err != nil {
		// Best-effort diagnostic; the exit status carries the failure.
		_, _ = fmt.Fprintln(stderr, "trusted review pack:", err)
		return 1
	}
	return 0
}

// BuildFromRoot reads precisely the five canonical inputs from a source-less
// private root. OpenRoot confines all paths, including symlinks, to that root.
func BuildFromRoot(ctx context.Context, root, subject string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("review pack: open private inputs: %w", err)
	}
	defer func() { _ = r.Close() }() // read-only directory handle; no durable state to flush
	in := ReviewInputs{Subject: subject}
	for _, input := range []struct {
		path string
		dest *[]byte
	}{
		{proposalPath, &in.Proposal}, {verdictPath, &in.CodegateVerdict},
		{charterPath, &in.Charter}, {designPath, &in.DesignBindings},
		{calibrationPath, &in.Calibration},
	} {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		b, err := r.ReadFile(filepath.FromSlash(input.path))
		if err != nil {
			return nil, fmt.Errorf("review pack: read %s: %w", input.path, err)
		}
		*input.dest = b
	}
	return Build(in)
}
