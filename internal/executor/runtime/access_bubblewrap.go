package runtime

import (
	"fmt"
	"os/exec"
	"strconv"
)

// Bubblewrap supplies read-only mounts which also reject raw writes, chmod,
// hard links, and absolute Git-directory overrides. Unlike a chmod guard, the
// child cannot undo the mount policy. It requires unprivileged namespaces;
// missing or disabled support fails before any model CLI can run.
func confineBubblewrapProcess(cmd *exec.Cmd, paths accessPaths, lookPath func(string) (string, error)) error {
	binary, err := lookPath("bwrap")
	if err != nil {
		return fmt.Errorf("%w: Linux requires bubblewrap (bwrap): %v; %s", ErrAccessUnsupported, err, processAccessRemedy("linux"))
	}
	args := []string{binary, "--die-with-parent", "--unshare-user", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--new-session", "--cap-drop", "ALL", "--ro-bind", "/", "/", "--proc", "/proc", "--dev", "/dev"}
	if !paths.readOnly {
		if paths.workFD != 0 {
			args = append(args, "--bind-fd", strconv.Itoa(paths.workFD), paths.work)
		} else {
			args = append(args, "--bind", paths.work, paths.work)
		}
	}
	if paths.stateFD != 0 {
		args = append(args, "--bind-fd", strconv.Itoa(paths.stateFD), paths.state)
	} else {
		args = append(args, "--bind", paths.state, paths.state)
	}
	for _, path := range paths.protected {
		args = append(args, "--ro-bind", path, path)
	}
	args = append(args, "--chdir", paths.work, "--", cmd.Path)
	cmd.Path = binary
	cmd.Args = append(args, cmd.Args[1:]...)
	return nil
}
