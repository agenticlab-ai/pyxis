package runtime

import "os/exec"

func confineNativeProcess(cmd *exec.Cmd, paths accessPaths) error {
	return confineBubblewrapProcess(cmd, paths, exec.LookPath)
}
