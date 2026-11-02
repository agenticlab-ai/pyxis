//go:build !windows

package runner

import (
	"os/exec"
	"testing"
)

func TestCoverage90KillGroupBeforeStartIsNoop(t *testing.T) {
	killGroup(exec.Command("unused"), false)
}
