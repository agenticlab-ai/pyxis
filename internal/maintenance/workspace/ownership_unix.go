//go:build darwin || linux

package workspace

import (
	"os"
	"syscall"
)

func ownedByProcess(info os.FileInfo) bool {
	return ownedBy(info, os.Geteuid())
}

func ownedBy(info os.FileInfo, effectiveUID int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(effectiveUID)
}
