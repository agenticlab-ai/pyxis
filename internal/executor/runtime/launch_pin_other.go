//go:build !linux

package runtime

import "os/exec"

func openLaunchScope(paths accessPaths) (accessPaths, func(), error) {
	paths.launchMounts = launchMounts{} // this platform carries no mount handles
	return paths, func() {}, nil        // Seatbelt intersects every ancestor's policy
}

func pinLaunchPaths(_ *exec.Cmd, paths accessPaths) (accessPaths, func(), error) {
	return paths, func() {}, nil // Seatbelt intersects ancestor paths in one policy
}
