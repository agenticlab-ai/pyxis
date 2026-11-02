//go:build linux

package procgroup

import (
	"context"
	"os"
)

func readProcess(pid int) (processIdentity, error) {
	return linuxIdentity(pid, os.ReadFile)
}

func listProcesses(ctx context.Context) (map[int]processIdentity, error) {
	return linuxSnapshot(ctx, os.ReadDir, readProcess)
}
