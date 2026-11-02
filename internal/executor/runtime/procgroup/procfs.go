//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func linuxIdentity(pid int, read func(string) ([]byte, error)) (processIdentity, error) {
	data, err := read("/proc/" + strconv.Itoa(pid) + "/stat")
	if os.IsNotExist(err) || errors.Is(err, syscall.ESRCH) {
		return processIdentity{}, syscall.ESRCH
	}
	if err != nil {
		return processIdentity{}, err
	}
	// comm can contain spaces and parentheses; fields after the final ')' start
	// at field 3. Field 22 is the kernel birth tick, unchanged through exec.
	end := strings.LastIndex(string(data), ") ")
	if end < 0 {
		return processIdentity{}, fmt.Errorf("process identity: malformed stat for %d", pid)
	}
	fields := strings.Fields(string(data)[end+2:])
	if len(fields) < 20 {
		return processIdentity{}, fmt.Errorf("process identity: malformed stat for %d", pid)
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return processIdentity{}, err
	}
	born, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return processIdentity{}, err
	}
	session, err := strconv.Atoi(fields[3])
	if err != nil {
		return processIdentity{}, err
	}
	return processIdentity{pid: pid, parent: parent, session: session, born: born, stopped: fields[0] == "T" || fields[0] == "t", exited: fields[0] == "Z" || fields[0] == "X"}, nil
}

func linuxSnapshot(ctx context.Context, readDir func(string) ([]fs.DirEntry, error), identify func(int) (processIdentity, error)) (map[int]processIdentity, error) {
	entries, err := readDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make(map[int]processIdentity)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		p, err := identify(pid)
		if errors.Is(err, syscall.ESRCH) || os.IsPermission(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[pid] = p
	}
	return out, nil
}
