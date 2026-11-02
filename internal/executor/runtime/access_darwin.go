package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Seatbelt applies to the entire process tree, not just the harness shell tool.
// Git's administrative paths are explicit denials inside any writable root.
// Reads and provider network access retain the native harness policy; this
// outer layer enforces write ownership, not a claim of complete OS isolation.
func confineNativeProcess(cmd *exec.Cmd, paths accessPaths) error {
	const sandbox = "/usr/bin/sandbox-exec"
	if _, err := os.Stat(sandbox); err != nil {
		return fmt.Errorf("%w: macOS requires sandbox-exec: %v", ErrAccessUnsupported, err)
	}
	args := []string{sandbox, "-p", seatbeltProfile(paths), cmd.Path}
	// An offline verifier requests this exact network-only policy. Apply its
	// denial in the host's existing write policy, rather than asking a confined
	// child to apply another Seatbelt profile (which macOS can reject).
	// All other profiles retain their normal command semantics.
	if cmd.Path == sandbox && len(cmd.Args) >= 4 && cmd.Args[1] == "-p" && cmd.Args[2] == "(version 1) (allow default) (deny network*)" {
		args[2] += "(deny network*)\n"
		args[3] = cmd.Args[3]
		cmd.Args = cmd.Args[3:]
	}
	cmd.Path = sandbox
	cmd.Args = append(args, cmd.Args[1:]...)
	return nil
}

func seatbeltProfile(paths accessPaths) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n")
	b.WriteString("(allow file-write* (literal \"/dev/null\") (literal \"/dev/tty\"))\n")
	// Cursor's registered adapter uses script(1); permit only PTY devices,
	// without granting write access to disks or other device nodes.
	b.WriteString("(allow file-write* (regex #\"^/dev/(ptmx|ttys[0-9]+)$\"))\n")
	for _, path := range []string{paths.state, paths.work} {
		if path == paths.work && paths.readOnly {
			continue
		}
		fmt.Fprintf(&b, "(allow file-write* (subpath %s))\n", strconv.Quote(path))
	}
	for _, path := range paths.protected {
		fmt.Fprintf(&b, "(deny file-write* (literal %s) (subpath %s))\n", strconv.Quote(path), strconv.Quote(path))
	}
	// Preserve every ancestor's root constraint in the same kernel policy.
	// A mutable nested directory cannot widen access via a symlink swap.
	for _, parent := range paths.ancestors {
		roots := "(subpath " + strconv.Quote(parent.state) + ")"
		if !parent.readOnly {
			roots += " (subpath " + strconv.Quote(parent.work) + ")"
		}
		b.WriteString("(deny file-write* (require-not (require-any " + roots + " (literal \"/dev/null\") (literal \"/dev/tty\") (regex #\"^/dev/(ptmx|ttys[0-9]+)$\"))))\n")
	}
	return b.String()
}
