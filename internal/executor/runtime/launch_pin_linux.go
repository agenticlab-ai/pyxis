package runtime

import "os/exec"

// Linux retains directory handles for Bubblewrap's descriptor-backed mounts.
func openLaunchScope(paths accessPaths) (accessPaths, func(), error) {
	return openLaunchMounts(paths)
}

func pinLaunchPaths(cmd *exec.Cmd, paths accessPaths) (accessPaths, func(), error) {
	return pinLaunchMounts(cmd, paths)
}
