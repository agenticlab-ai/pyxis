//go:build !windows

package procgroup

import (
	"os"
	"os/exec"
	"testing"
)

func TestKillCheckedRefusesInvalidGroup(t *testing.T) {
	for _, pid := range []int{0, -1} {
		if err := KillChecked(&exec.Cmd{Process: &os.Process{Pid: pid}}, true); err == nil {
			t.Fatalf("invalid pid %d could signal the supervisor's process group", pid)
		}
	}
	if err := KillChecked(&exec.Cmd{}, true); err != nil {
		t.Fatalf("unstarted process: %v", err)
	}
}
