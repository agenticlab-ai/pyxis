//go:build darwin

package procgroup

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// XNU's fixed proc_info ABIs, read without cgo, argv or environment access.
// Short BSD info (flavor 13) and unique identity info (17) do not require the
// target to have our UID. Full BSD info does, so it cannot distinguish an exited
// descendant's reused PID from a still-owned process whose inspection is denied.
type processInfoCall func(action, pid, flavor int, data []byte) (int, error)

func processInfo(action, pid, flavor int, data []byte) (int, error) {
	var address unsafe.Pointer
	if len(data) > 0 {
		address = unsafe.Pointer(&data[0])
	}
	n, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, uintptr(action), uintptr(pid), uintptr(flavor), 0, uintptr(address), uintptr(len(data)))
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}

func readProcess(pid int) (processIdentity, error) {
	return bsdSessionIdentity(pid, processInfo, syscall.Getsid)
}

func bsdSessionIdentity(pid int, call processInfoCall, sid func(int) (int, error)) (processIdentity, error) {
	p, err := bsdIdentity(pid, call)
	if err != nil {
		return p, err
	}
	if p.exited {
		return p, nil // a zombie no longer needs session lookup to establish exit
	}
	p.session, err = sid(pid)
	if err != nil {
		return processIdentity{}, fmt.Errorf("process identity: session for pid %d: %w", pid, err)
	}
	// A PID can be recycled between the metadata read and session lookup.
	unique, err := bsdUniqueIdentity(pid, call)
	if err != nil {
		return processIdentity{}, err
	}
	if unique != p.born {
		return processIdentity{}, syscall.ESRCH
	}
	return p, nil
}

func bsdIdentity(pid int, call processInfoCall) (processIdentity, error) {
	unique, err := bsdUniqueIdentity(pid, call)
	if err != nil {
		return processIdentity{}, err
	}
	var data [64]byte // proc_bsdshortinfo, bsd/sys/proc_info.h
	n, err := call(2, pid, 13, data[:])
	if err != nil {
		return processIdentity{}, fmt.Errorf("process identity: short BSD info for pid %d: %w", pid, err)
	}
	if n != len(data) {
		return processIdentity{}, fmt.Errorf("process identity: short BSD info for %d", pid)
	}
	if got := int(binary.LittleEndian.Uint32(data[:4])); got != pid {
		return processIdentity{}, fmt.Errorf("process identity: BSD info returned pid %d for %d", got, pid)
	}
	confirmed, err := bsdUniqueIdentity(pid, call)
	if err != nil {
		return processIdentity{}, err
	}
	if confirmed != unique {
		return processIdentity{}, syscall.ESRCH
	}
	status := binary.LittleEndian.Uint32(data[12:16])
	return processIdentity{pid: pid, parent: int(binary.LittleEndian.Uint32(data[4:8])), born: unique, stopped: status == 4, exited: status == 5}, nil
}

func bsdUniqueIdentity(pid int, call processInfoCall) (uint64, error) {
	var data [56]byte // proc_uniqidentifierinfo, bsd/sys/proc_info_private.h
	n, err := call(2, pid, 17, data[:])
	if err != nil {
		return 0, fmt.Errorf("process identity: unique ID for pid %d: %w", pid, err)
	}
	if n != len(data) {
		return 0, fmt.Errorf("process identity: short unique ID info for pid %d", pid)
	}
	unique := binary.LittleEndian.Uint64(data[16:24])
	if unique == 0 {
		return 0, fmt.Errorf("process identity: unavailable unique ID for pid %d", pid)
	}
	return unique, nil
}

func listProcesses(ctx context.Context) (map[int]processIdentity, error) {
	return bsdSnapshot(ctx, processInfo, readProcess)
}

func bsdSnapshot(ctx context.Context, call processInfoCall, identify func(int) (processIdentity, error)) (map[int]processIdentity, error) {
	// LISTPIDS=1, PROC_ALL_PIDS=1. Retry if the process table grows while read.
	n, err := call(1, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	for range 3 {
		if n > 4<<20 {
			return nil, fmt.Errorf("process identity: process table exceeds bound")
		}
		pids := make([]byte, n+256)
		read, err := call(1, 1, 0, pids)
		if err != nil {
			return nil, err
		}
		if read >= len(pids) {
			n = read
			continue
		}
		out := make(map[int]processIdentity)
		for offset := 0; offset+4 <= read; offset += 4 {
			pid := int32(binary.LittleEndian.Uint32(pids[offset : offset+4]))
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if pid <= 0 {
				continue
			}
			p, err := identify(int(pid))
			// macOS hides protected system processes from unprivileged callers.
			// Exclude those entries from admission. Lifetime re-reads every owned
			// identity before signaling and acknowledging it, so an inaccessible
			// owned process still makes cleanup fail closed.
			if errors.Is(err, syscall.ESRCH) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
				continue
			}
			if err != nil {
				return nil, err
			}
			out[p.pid] = p
		}
		return out, nil
	}
	return nil, fmt.Errorf("process identity: unstable process table")
}
