//go:build darwin || linux

package procgroup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

func TestLifetimeStopsSeparateGroupsAndLeavesForeignProcess(t *testing.T) {
	foreign := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLifetimeHelper$", "--", "lifetime-leaf")
	if err := foreign.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = foreign.Process.Kill(); _ = foreign.Wait() }) // reap the unrelated fixture
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLifetimeHelper$", "--", "lifetime-root")
	SetOwnedSession(cmd)
	scope := NewLifetime(cmd)
	cmd.Cancel = scope.Kill
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = scope.Kill(); _ = cmd.Wait() }) // reap even on a failed assertion
	if err := scope.ObserveStart(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(line[:len(line)-1])
	if err != nil {
		t.Fatal(err)
	}
	child, err := readProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p, err := readProcess(pid); err == nil && child.same(p) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}) // no fixture leaks on regression
	if group, err := syscall.Getpgid(pid); err != nil || group == cmd.Process.Pid {
		t.Fatalf("child did not leave parent group: %d %v", group, err)
	}
	if err := scope.Kill(); err != nil {
		t.Fatal(err)
	}
	if p, err := readProcess(pid); err == nil && child.same(p) && !p.exited {
		t.Fatalf("cleanup acknowledged a live descendant: %+v", p)
	}
	if err := syscall.Kill(foreign.Process.Pid, 0); err != nil {
		t.Fatalf("unrelated process was stopped: %v", err)
	}
	if err := scope.Kill(); err != nil {
		t.Fatalf("idempotent cleanup: %v", err)
	}
	if err := scope.ObserveStart(); err != nil {
		t.Fatalf("closed owner restarted observation: %v", err)
	}
}

func TestLifetimeHelper(t *testing.T) {
	mode := os.Args[len(os.Args)-1]
	if mode == "lifetime-leaf" {
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode == "lifetime-root-middle-exit" {
		middle := exec.Command(os.Args[0], "-test.run=^TestLifetimeHelper$", "--", "lifetime-root-exit-separate")
		middle.Stdout = os.Stdout
		if err := middle.Run(); err != nil {
			os.Exit(92)
		}
		fmt.Println("middle-exited")
		for {
			time.Sleep(time.Hour)
		}
	}
	if mode != "lifetime-root" && mode != "lifetime-root-exit" && mode != "lifetime-root-exit-separate" {
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestLifetimeHelper$", "--", "lifetime-leaf")
	if mode != "lifetime-root-exit" {
		Setpgid(cmd)
	}
	if err := cmd.Start(); err != nil {
		os.Exit(90)
	}
	fmt.Println(cmd.Process.Pid)
	if mode != "lifetime-root" {
		os.Exit(0)
	}
	_ = cmd.Wait() // the owner intentionally terminates this process tree
	os.Exit(91)
}

func TestLifetimeDoesNotAcknowledgeUnobservedOrphan(t *testing.T) {
	for _, mode := range []string{"lifetime-root-exit", "lifetime-root-exit-separate", "lifetime-root-middle-exit"} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestLifetimeHelper$", "--", mode)
			SetOwnedSession(cmd)
			scope := NewLifetime(cmd)
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = scope.Kill(); _ = cmd.Wait() }) // always reap this fixture
			if err := scope.ObserveStart(); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(out)
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(line[:len(line)-1])
			if err != nil {
				t.Fatal(err)
			}
			child, err := readProcess(pid)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if p, err := readProcess(pid); err == nil && child.same(p) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}) // stop only this proven fixture identity on a failed assertion
			if mode == "lifetime-root-middle-exit" {
				if line, err := reader.ReadString('\n'); err != nil || line != "middle-exited\n" {
					t.Fatalf("intermediate parent did not exit: %q %v", line, err)
				}
			} else {
				if err := scope.WaitExit(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			stopErr := scope.Kill()
			if err := cmd.Wait(); err != nil && mode != "lifetime-root-middle-exit" {
				t.Fatal(err)
			}
			if stopErr != nil {
				t.Fatalf("private-session cleanup failed: %v", stopErr)
			}
			if p, err := readProcess(pid); err == nil && child.same(p) && !p.exited && stopErr == nil {
				t.Fatalf("cleanup acknowledged unobserved orphan %d", pid)
			}
		})
	}
}

type processTable struct {
	mu               sync.Mutex
	pids             map[int]processIdentity
	signals          []int
	listErr, readErr error
	denied           syscall.Signal
	ignore           bool
}

func simulatedLifetime(t *testing.T) (*Lifetime, *processTable) {
	t.Helper()
	proc, err := os.FindProcess(44)
	if err != nil {
		t.Fatal(err)
	}
	s := NewLifetime(&exec.Cmd{Process: proc})
	root := processIdentity{pid: 44, parent: 43, born: 10}
	table := &processTable{pids: map[int]processIdentity{44: root}}
	s.known[44] = root
	s.root = root
	s.list = func(ctx context.Context) (map[int]processIdentity, error) {
		table.mu.Lock()
		defer table.mu.Unlock()
		if table.listErr != nil {
			return nil, table.listErr
		}
		out := make(map[int]processIdentity)
		maps.Copy(out, table.pids)
		return out, ctx.Err()
	}
	s.read = func(pid int) (processIdentity, error) {
		table.mu.Lock()
		defer table.mu.Unlock()
		if table.readErr != nil {
			return processIdentity{}, table.readErr
		}
		p, ok := table.pids[pid]
		if !ok {
			return processIdentity{}, syscall.ESRCH
		}
		return p, nil
	}
	s.signal = func(pid int, signal syscall.Signal) error {
		table.mu.Lock()
		defer table.mu.Unlock()
		table.signals = append(table.signals, pid)
		if table.denied == signal {
			return syscall.EPERM
		}
		if table.ignore {
			return nil
		}
		p, ok := table.pids[pid]
		if !ok {
			return syscall.ESRCH
		}
		if signal == syscall.SIGSTOP {
			p.stopped = true
			table.pids[pid] = p
		} else {
			delete(table.pids, pid)
		}
		return nil
	}
	return s, table
}

func TestLifetimeWaitExitPreservesIdentityAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Lifetime, *processTable)
		want   error
		bad    bool
	}{
		{"exited", func(_ *Lifetime, table *processTable) { p := table.pids[44]; p.exited = true; table.pids[44] = p }, nil, false},
		{"absent", func(_ *Lifetime, table *processTable) { delete(table.pids, 44) }, nil, false},
		{"inspection denied", func(_ *Lifetime, table *processTable) { table.readErr = syscall.EACCES }, syscall.EACCES, true},
		{"recycled PID", func(_ *Lifetime, table *processTable) { p := table.pids[44]; p.born++; table.pids[44] = p }, nil, true},
		{"live after cancellation", func(*Lifetime, *processTable) {}, context.Canceled, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, table := simulatedLifetime(t)
			tc.mutate(s, table)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			err := s.WaitExit(ctx)
			if (err != nil) != tc.bad || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("exit observation=%v want %v", err, tc.want)
			}
			if len(table.signals) != 0 {
				t.Fatalf("exit observation signalled a process: %v", table.signals)
			}
		})
	}
}

func TestLifetimeFreezesKnownParentsBeforeGlobalInspection(t *testing.T) {
	s, table := simulatedLifetime(t)
	inspect := s.list
	s.list = func(ctx context.Context) (map[int]processIdentity, error) {
		table.mu.Lock()
		stopped := table.pids[44].stopped
		table.mu.Unlock()
		if !stopped {
			t.Fatal("global discovery ran while the known parent could still fork")
		}
		return inspect(ctx)
	}
	if err := s.Kill(); err != nil {
		t.Fatal(err)
	}
	if len(table.pids) != 0 {
		t.Fatalf("cleanup acknowledged a live process: %v", table.pids)
	}
}

func TestLifetimeAcknowledgesExitWithinBudgetWithSlowDiscovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, table := simulatedLifetime(t)
		inspect := s.list
		s.list = func(ctx context.Context) (map[int]processIdentity, error) {
			// One scan fits the unchanged five-second cleanup budget; doing a
			// second whole-table pass solely to observe the root's stop does not.
			time.Sleep(3 * time.Second)
			return inspect(ctx)
		}
		if err := s.Kill(); err != nil {
			t.Fatalf("slow discovery prevented cleanup acknowledgement: %v", err)
		}
		if len(table.pids) != 0 {
			t.Fatalf("cleanup acknowledged a live process: %v", table.pids)
		}
	})
}

func TestLifetimePrivateSessionAuthority(t *testing.T) {
	for _, tc := range []struct {
		name    string
		exited  bool
		readErr error
		session int
		bad     bool
	}{
		{name: "established session", session: 44},
		{name: "missing session", session: 43, bad: true},
		{name: "zombie has no session lookup", exited: true},
		{name: "exited before initial read", readErr: syscall.ESRCH},
		{name: "identity inspection denied", readErr: syscall.EACCES, bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, table := simulatedLifetime(t)
			s.known = make(map[int]processIdentity)
			SetOwnedSession(s.cmd)
			p := table.pids[44]
			p.session, p.exited = tc.session, tc.exited
			table.pids[44], table.readErr = p, tc.readErr
			if err := s.ObserveStart(); (err != nil) != tc.bad {
				t.Fatalf("session capture=%v want failure=%v", err, tc.bad)
			}
			if tc.bad {
				_ = s.Kill() // best-effort fixture cleanup; inspection deliberately fails
				return
			}
			if tc.readErr != nil {
				delete(table.pids, 44)
				table.readErr = nil
				if err := s.WaitExit(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			table.pids[45] = processIdentity{pid: 45, parent: 1, session: 44, born: 11}
			table.pids[46] = processIdentity{pid: 46, parent: 1, session: 90, born: 12}
			if err := s.Kill(); err != nil {
				t.Fatal(err)
			}
			if _, exists := table.pids[45]; exists {
				t.Fatal("reparented private-session child survived")
			}
			if _, exists := table.pids[46]; !exists {
				t.Fatal("foreign session was signalled")
			}
		})
	}
	// Once the trusted direct child is reaped, its numeric session is no longer
	// sufficient authority to admit new identities. Report non-quiescence.
	s, table := simulatedLifetime(t)
	s.session = 44
	s.cmd.ProcessState = &os.ProcessState{}
	table.pids[45] = processIdentity{pid: 45, parent: 1, session: 44, born: 11}
	if err := s.Kill(); err == nil {
		t.Fatal("accepted cleanup after premature reap")
	}
	if _, exists := table.pids[45]; !exists {
		t.Fatal("reaped session admitted an unproven identity")
	}
}

func TestLifetimeIdentityAndStopFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Lifetime, *processTable)
		want   error
	}{
		{"reparented child remains owned", func(s *Lifetime, table *processTable) {
			child := processIdentity{pid: 45, parent: 44, born: 11}
			table.pids[45] = child
			s.collect(table.pids)
			child.parent = 1
			table.pids[45] = child
		}, nil},
		{"reused PID and its children are foreign", func(s *Lifetime, table *processTable) {
			table.pids[44] = processIdentity{pid: 44, parent: 43, born: 20}
			table.pids[45] = processIdentity{pid: 45, parent: 44, born: 21}
		}, nil},
		{"inspection failure retains error after best effort kill", func(s *Lifetime, table *processTable) { table.listErr = syscall.EIO }, syscall.EIO},
		{"identity denied cannot certify cleanup", func(s *Lifetime, table *processTable) { table.readErr = syscall.EACCES }, syscall.EACCES},
		{"freeze denied still attempts termination", func(s *Lifetime, table *processTable) { table.denied = syscall.SIGSTOP }, syscall.EPERM},
		{"kill denied cannot certify cleanup", func(s *Lifetime, table *processTable) { table.denied = syscall.SIGKILL }, syscall.EPERM},
		{"successful signal without exit is not acknowledgement", func(s *Lifetime, table *processTable) { table.ignore = true; s.grace = time.Millisecond }, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, table := simulatedLifetime(t)
			tc.mutate(s, table)
			err := s.Kill()
			if !errors.Is(err, tc.want) {
				t.Fatalf("cleanup=%v want %v", err, tc.want)
			}
			if again := s.Kill(); again != err {
				t.Fatalf("idempotent cleanup changed outcome: %v", again)
			}
			if tc.name == "reused PID and its children are foreign" && len(table.signals) != 0 {
				t.Fatalf("signaled recycled identity: %v", table.signals)
			}
			if tc.name == "reparented child remains owned" && len(table.pids) != 0 {
				t.Fatalf("owned child survived: %v", table.pids)
			}
			if tc.name == "freeze denied still attempts termination" && len(table.pids) != 0 {
				t.Fatalf("failed freeze left process alive: %v", table.pids)
			}
		})
	}
}

func TestLifetimeStartAndObservation(t *testing.T) {
	if err := NewLifetime(&exec.Cmd{}).ObserveStart(); err == nil {
		t.Fatal("accepted unstarted process")
	}
	if err := NewLifetime(&exec.Cmd{}).Kill(); err != nil {
		t.Fatal(err)
	}
	s, table := simulatedLifetime(t)
	s.known = make(map[int]processIdentity)
	table.readErr = syscall.EIO
	if err := s.ObserveStart(); !errors.Is(err, syscall.EIO) {
		t.Fatalf("capture failure: %v", err)
	}
	if err := s.Kill(); !errors.Is(err, syscall.EIO) {
		t.Fatalf("stop capture failure: %v", err)
	}
	s, table = simulatedLifetime(t)
	s.known = make(map[int]processIdentity)
	delete(table.pids, 44)
	if err := s.Kill(); err != nil {
		t.Fatalf("already absent child: %v", err)
	}
	synctest.Test(t, func(t *testing.T) {
		s, table := simulatedLifetime(t)
		s.known = make(map[int]processIdentity)
		if err := s.ObserveStart(); err != nil {
			t.Fatal(err)
		}
		if err := s.ObserveStart(); err != nil {
			t.Fatal(err)
		}
		table.mu.Lock()
		table.listErr = syscall.EIO
		table.mu.Unlock()
		time.Sleep(time.Second)
		synctest.Wait()
		table.mu.Lock()
		table.listErr = nil
		table.pids[45] = processIdentity{pid: 45, parent: 44, born: 11}
		table.mu.Unlock()
		time.Sleep(time.Second)
		synctest.Wait()
		if err := s.Kill(); err != nil {
			t.Fatal(err)
		}
		if len(table.pids) != 0 {
			t.Fatalf("observer lost child: %v", table.pids)
		}
	})
}
