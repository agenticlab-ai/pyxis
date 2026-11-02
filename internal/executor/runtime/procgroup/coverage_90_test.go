//go:build !windows

package procgroup_test

import (
	"os/exec"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

func TestKillBeforeProcessStartIsANoop(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	procgroup.Kill(cmd, false)
	if cmd.Process != nil {
		t.Fatal("an unstarted command unexpectedly acquired a process")
	}
}
