// Command ci-parity rejects hosted verification steps outside the shared gate.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/ciparity"
)

func main() {
	if run(context.Background(), ".", os.Args[1:], os.Stderr) != 0 {
		os.Exit(1)
	}
}

func run(ctx context.Context, root string, args []string, output io.Writer) int {
	if len(args) > 0 {
		if _, err := fmt.Fprintln(output, "usage: ci-parity (run at the repository root)"); err != nil {
			return 1
		}
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			return 0
		}
		return 1
	}
	if err := ciparity.Check(ctx, root); err != nil {
		_, _ = fmt.Fprintf(output, "ci-parity: %v\n", err) // Best-effort diagnostic; the operation already failed.
		return 1
	}
	if _, err := fmt.Fprintln(output, "ci-parity: all blocking verification steps use the shared gate"); err != nil {
		return 1
	}
	return 0
}
