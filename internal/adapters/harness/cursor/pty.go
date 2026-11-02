package cursor

import "strings"

// scriptBinary is the pty-providing shim. cursor-agent refuses to run headless
// without a controlling TTY (design:implement-now#3.7 R6 — "cursor requires a
// pty"), so every launch is wrapped in the POSIX `script` utility: it allocates
// a pseudo-terminal, execs the child against the slave side (so the child sees
// stdout as a tty), and copies the child's output onto our pipe.
//
// Using `script` keeps the production exec identical to the other adapters' — a
// plain os/exec child, no bespoke pty syscalls and no new dependency (the one
// real quirk is absorbed as argv). Because the wrap is just argv, the same
// injected commander the non-pty paths use records it verbatim, so the wrap
// decision is unit-tested with no real pty.
const scriptBinary = "script"

// ptyCommand rewrites an intended (name, args) invocation into the `script`
// wrapper for the host OS. `script`'s argument grammar is not portable:
//
//   - BSD/macOS: `script [-q] file command [args...]` — the child command and
//     its args follow the typescript file directly and are passed straight to
//     execvp (no shell re-parsing); the child's exit status is script's exit
//     status by default.
//   - util-linux: `script [-q] [-e] -c "<command>" file` — the child is a single
//     string run through `sh -c`, so every token is shell-quoted here, and `-e`
//     is required for the child's exit code to surface as script's exit code.
//
// /dev/null is the typescript sink — only the child's stdout is wanted on the
// pipe, not a recorded file. The returned (name, args) feed the same commander
// the direct paths use, so a test observes the wrap as the recorded argv.
func ptyCommand(goos, name string, args []string) (string, []string) {
	if goos == "linux" {
		var b strings.Builder
		b.WriteString(shellQuote(name))
		for _, a := range args {
			b.WriteByte(' ')
			b.WriteString(shellQuote(a))
		}
		return scriptBinary, []string{"-q", "-e", "-c", b.String(), "/dev/null"}
	}
	// BSD/macOS grammar (darwin and the *BSDs).
	wrapped := make([]string, 0, len(args)+3)
	wrapped = append(wrapped, "-q", "/dev/null", name)
	wrapped = append(wrapped, args...)
	return scriptBinary, wrapped
}

// shellQuote single-quotes s for POSIX `sh -c`, so an arbitrary argument — a
// bracketed model string like claude-opus-4-8[effort=high], or a multi-line
// brief — survives util-linux `script`'s single-string command form with no
// reinterpretation. Each embedded single quote is rewritten to the canonical
// close-escape-reopen sequence produced by the replacement below.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
