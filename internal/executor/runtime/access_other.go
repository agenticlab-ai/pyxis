//go:build !darwin && !linux

package runtime

import (
	"fmt"
	"os/exec"
	"runtime"
)

func confineNativeProcess(_ *exec.Cmd, _ accessPaths) error {
	return fmt.Errorf("%w on %s; implementation and review require a supported process boundary", ErrAccessUnsupported, runtime.GOOS)
}
