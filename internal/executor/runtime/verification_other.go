//go:build !darwin

package runtime

import (
	"context"
	"os/exec"
)

func configureVerificationTools(context.Context, *exec.Cmd, bool) error {
	return nil // these platforms do not use Apple's tool launchers
}
