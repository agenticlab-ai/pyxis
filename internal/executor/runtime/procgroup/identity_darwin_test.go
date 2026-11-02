//go:build darwin

package procgroup

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
)

func TestBSDIdentityDecodingAndUnavailableIdentity(t *testing.T) {
	for _, status := range []uint32{2, 4, 5} {
		p, err := bsdIdentity(44, bsdFixtureCall(t, 44, 43, status, 100))
		if err != nil || p.pid != 44 || p.parent != 43 || p.born != 100 || p.fraction != 0 || p.stopped != (status == 4) || p.exited != (status == 5) {
			t.Fatalf("kernel identity: %+v %v", p, err)
		}
	}
	if _, err := bsdIdentity(44, func(int, int, int, []byte) (int, error) { return 0, syscall.EPERM }); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("permission error: %v", err)
	}
}

func bsdFixtureCall(t *testing.T, pid, parent int, status uint32, unique uint64) processInfoCall {
	t.Helper()
	return func(action, requested, flavor int, data []byte) (int, error) {
		if action != 2 || requested != pid {
			t.Fatalf("wrong kernel request: %d %d %d", action, requested, flavor)
		}
		switch flavor {
		case 13:
			if len(data) != 64 {
				t.Fatal("wrong short BSD info ABI")
			}
			binary.LittleEndian.PutUint32(data[:4], uint32(pid))
			binary.LittleEndian.PutUint32(data[4:8], uint32(parent))
			binary.LittleEndian.PutUint32(data[12:16], status)
		case 17:
			if len(data) != 56 {
				t.Fatal("wrong unique identity ABI")
			}
			binary.LittleEndian.PutUint64(data[16:24], unique)
		default:
			// Full BSD info requires the target UID to match the caller. Reusing
			// an owned PID for a foreign-user process must not require that read.
			return 0, syscall.EPERM
		}
		return len(data), nil
	}
}

func TestBSDIdentityRacesAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failAt   int
		failure  error
		short    bool
		zero     bool
		changed  bool
		wrongPID bool
		want     error
	}{
		{name: "initial unique ID denied", failAt: 1, failure: syscall.EPERM, want: syscall.EPERM},
		{name: "short initial unique ID", failAt: 1, short: true},
		{name: "zero unique ID", zero: true},
		{name: "short BSD info denied", failAt: 2, failure: syscall.EACCES, want: syscall.EACCES},
		{name: "short BSD info truncated", failAt: 2, short: true},
		{name: "BSD info for wrong PID", wrongPID: true},
		{name: "final unique ID denied", failAt: 3, failure: syscall.EPERM, want: syscall.EPERM},
		{name: "PID recycled during metadata read", changed: true, want: syscall.ESRCH},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := bsdFixtureCall(t, 44, 43, 2, 100)
			calls := 0
			_, err := bsdIdentity(44, func(action, pid, flavor int, data []byte) (int, error) {
				calls++
				if calls == tc.failAt {
					if tc.short {
						return 1, nil
					}
					return 0, tc.failure
				}
				n, err := base(action, pid, flavor, data)
				if flavor == 17 && tc.zero {
					binary.LittleEndian.PutUint64(data[16:24], 0)
				}
				if calls == 3 && tc.changed {
					binary.LittleEndian.PutUint64(data[16:24], 101)
				}
				if flavor == 13 && tc.wrongPID {
					binary.LittleEndian.PutUint32(data[:4], 45)
				}
				return n, err
			})
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("identity error=%v want %v", err, tc.want)
			}
			if tc.failure != nil && !strings.Contains(err.Error(), "pid 44") {
				t.Fatalf("inspection failure omitted target: %v", err)
			}
		})
	}
}

func TestBSDSessionIdentityAndExitRace(t *testing.T) {
	for _, tc := range []struct {
		name            string
		status          uint32
		infoErr, sidErr error
		changedAfterSID bool
		finalErr        error
	}{
		{name: "live private session", status: 2},
		{name: "zombie requires no session lookup", status: 5, sidErr: syscall.ESRCH},
		{name: "kernel identity denied", infoErr: syscall.EPERM},
		{name: "session lookup denied", status: 2, sidErr: syscall.EACCES},
		{name: "exit during session lookup", status: 2, sidErr: syscall.ESRCH},
		{name: "PID recycled during session lookup", status: 2, changedAfterSID: true},
		{name: "final identity inspection denied", status: 2, finalErr: syscall.EPERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lookedUp := false
			base := bsdFixtureCall(t, 44, 43, tc.status, 100)
			p, err := bsdSessionIdentity(44, func(action, pid, flavor int, data []byte) (int, error) {
				if tc.infoErr != nil {
					return 0, tc.infoErr
				}
				if lookedUp && tc.finalErr != nil {
					return 0, tc.finalErr
				}
				n, err := base(action, pid, flavor, data)
				if lookedUp && tc.changedAfterSID {
					binary.LittleEndian.PutUint64(data[16:24], 101)
				}
				return n, err
			}, func(pid int) (int, error) {
				lookedUp = true
				if pid != 44 {
					t.Fatal("session lookup for wrong PID")
				}
				return 44, tc.sidErr
			})
			wantErr := tc.infoErr
			if tc.status != 5 && wantErr == nil {
				wantErr = tc.sidErr
			}
			if tc.changedAfterSID {
				wantErr = syscall.ESRCH
			}
			if tc.finalErr != nil {
				wantErr = tc.finalErr
			}
			if !errors.Is(err, wantErr) || lookedUp != (tc.infoErr == nil && tc.status != 5) {
				t.Fatalf("identity=%+v error=%v lookup=%v", p, err, lookedUp)
			}
			if wantErr == nil && (p.born != 100 || p.exited != (tc.status == 5) || (tc.status != 5 && p.session != 44)) {
				t.Fatalf("wrong session identity: %+v", p)
			}
		})
	}
}

func TestLifetimeReusedBSDPIDDoesNotRequireForeignUserPermissions(t *testing.T) {
	s, table := simulatedLifetime(t)
	foreign, err := bsdSessionIdentity(44, bsdFixtureCall(t, 44, 1, 2, 20), func(int) (int, error) { return 90, nil })
	if err != nil {
		t.Fatal(err)
	}
	// The scope recorded birth 10; PID 44 now belongs to a different kernel
	// unique ID, and its child belongs to that foreign identity too.
	table.pids[44] = foreign
	table.pids[45] = processIdentity{pid: 45, parent: 44, session: 90, born: 21}
	s.read = func(pid int) (processIdentity, error) {
		if pid != 44 {
			t.Fatalf("read unowned pid %d", pid)
		}
		return bsdSessionIdentity(pid, bsdFixtureCall(t, pid, 1, 2, 20), func(int) (int, error) { return 90, nil })
	}
	if err := s.Kill(); err != nil {
		t.Fatalf("recycled foreign PID prevented cleanup: %v", err)
	}
	if len(table.signals) != 0 || len(table.pids) != 2 {
		t.Fatalf("foreign identities were signalled: %v %v", table.signals, table.pids)
	}
}

func TestBSDSnapshotRacesAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		firstError, readError, identityError error
		growing, oversized                   bool
		wantError                            bool
	}{
		{name: "live and missing processes"},
		{name: "protected process", identityError: syscall.EPERM},
		{name: "wrapped protected process", identityError: fmt.Errorf("unique identity: %w", syscall.EPERM)},
		{name: "process exited during inspection", identityError: syscall.ESRCH},
		{name: "wrapped exit race", identityError: fmt.Errorf("short BSD info: %w", syscall.ESRCH)},
		{name: "unexpected identity failure", identityError: syscall.EIO, wantError: true},
		{name: "list permission denied", firstError: syscall.EACCES, wantError: true},
		{name: "table read failed", readError: syscall.EIO, wantError: true},
		{name: "unstable process table", growing: true, wantError: true},
		{name: "oversized process table", oversized: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all, err := bsdSnapshot(t.Context(), func(action, pid, flavor int, data []byte) (int, error) {
				if action != 1 || pid != 1 || flavor != 0 {
					t.Fatal("wrong table request")
				}
				if data == nil {
					if tc.oversized {
						return 5 << 20, nil
					}
					return 8, tc.firstError
				}
				if tc.growing {
					return len(data), nil
				}
				binary.LittleEndian.PutUint32(data[0:4], 44)
				return 8, tc.readError
			}, func(pid int) (processIdentity, error) { return processIdentity{pid: pid, born: 1}, tc.identityError })
			if (err != nil) != tc.wantError {
				t.Fatalf("snapshot=%v error=%v", all, err)
			}
			if !tc.wantError && len(all) != map[bool]int{true: 0, false: 1}[tc.identityError != nil] {
				t.Fatalf("wrong process table: %v", all)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := bsdSnapshot(ctx, func(int, int, int, []byte) (int, error) { return 4, nil }, readProcess); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot: %v", err)
	}
}
