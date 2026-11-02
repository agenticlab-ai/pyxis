package main

import (
	"fmt"
	"io"
)

// Terminal diagnostics and summaries are best effort. The operation's result
// determines the exit status; a broken terminal must not prevent interruption,
// rollback, or reporting an operation that has already completed. These helpers
// retain that existing contract while explicitly accounting for write errors.
func terminalPrintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) // The terminal output does not govern the operation's result.
}

func terminalPrintln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...) // The terminal output does not govern the operation's result.
}
