//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"
	"testing/fstest"
)

func TestLinuxIdentityWithParenthesesAndUnavailableIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, state, parent, born, session string
		data                               string
		readErr                            error
		bad                                bool
	}{
		{name: "live command", state: "S", parent: "43", born: "123"},
		{name: "stopped command", state: "T", parent: "43", born: "123"},
		{name: "traced command", state: "t", parent: "43", born: "123"},
		{name: "zombie command", state: "Z", parent: "43", born: "123"},
		{name: "dead command", state: "X", parent: "43", born: "123"},
		{name: "invalid parent", state: "S", parent: "no", born: "123", bad: true},
		{name: "invalid birth", state: "S", parent: "43", born: "no", bad: true},
		{name: "invalid session", state: "S", parent: "43", born: "123", session: "no", bad: true},
		{name: "missing command delimiter", data: "invalid", bad: true},
		{name: "truncated stat", data: "44 (command) S", bad: true},
		{name: "exited while reading", readErr: fs.ErrNotExist, bad: true},
		{name: "kernel reports reaped process", readErr: &os.PathError{Op: "read", Path: "/proc/44/stat", Err: syscall.ESRCH}, bad: true},
		{name: "denied identity", readErr: fs.ErrPermission, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.data
			if data == "" {
				session := tc.session
				if session == "" {
					session = "44"
				}
				data = "44 (command with ) parentheses) " + tc.state + " " + tc.parent + " 45 " + session + " " + strings.Repeat("0 ", 15) + tc.born
			}
			p, err := linuxIdentity(44, func(path string) ([]byte, error) {
				if path != "/proc/44/stat" {
					t.Fatalf("wrong process path: %s", path)
				}
				return []byte(data), tc.readErr
			})
			if (err != nil) != tc.bad {
				t.Fatalf("identity=%+v error=%v", p, err)
			}
			if !tc.bad && (p.pid != 44 || p.parent != 43 || p.session != 44 || p.born != 123 || p.stopped != (tc.state == "T" || tc.state == "t") || p.exited != (tc.state == "Z" || tc.state == "X")) {
				t.Fatalf("wrong kernel identity: %+v", p)
			}
			if (errors.Is(tc.readErr, fs.ErrNotExist) || errors.Is(tc.readErr, syscall.ESRCH)) && err != syscall.ESRCH {
				t.Fatalf("missing PID: %v", err)
			}
		})
	}
}

func TestLinuxSnapshotRacesAndFailures(t *testing.T) {
	entries, err := fs.ReadDir(fstest.MapFS{"44": &fstest.MapFile{}, "self": &fstest.MapFile{}, "0": &fstest.MapFile{}}, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		listErr, readErr error
		canceled, bad    bool
	}{
		{name: "normal"}, {name: "denied foreign identity", readErr: fs.ErrPermission},
		{name: "exited process", readErr: syscall.ESRCH},
		{name: "wrapped exited process", readErr: &os.PathError{Op: "read", Path: "/proc/44/stat", Err: syscall.ESRCH}},
		{name: "unexpected identity failure", readErr: syscall.EIO, bad: true},
		{name: "proc unavailable", listErr: os.ErrNotExist, bad: true},
		{name: "canceled", canceled: true, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			all, err := linuxSnapshot(ctx, func(path string) ([]fs.DirEntry, error) {
				if path != "/proc" {
					t.Fatal("wrong process table")
				}
				return entries, tc.listErr
			}, func(pid int) (processIdentity, error) { return processIdentity{pid: pid, born: 1}, tc.readErr })
			if (err != nil) != tc.bad {
				t.Fatalf("snapshot=%v error=%v", all, err)
			}
			if !tc.bad {
				want := 1
				if tc.readErr != nil {
					want = 0
				}
				if len(all) != want {
					t.Fatalf("wrong table: %v", all)
				}
			}
		})
	}
}
