//go:build linux

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"syscall"
	"testing"
)

// The T10 hermetic proof — the factory-in-a-box lifecycle runs entirely on local
// filesystem syscalls, making zero network calls (docs/tech-design/
// local-work-tracker.md §3.12, DINV-7, A-6; prd:local-work-tracker/FR-17,
// DOD-3). A full lifecycle — create, signal, link, parent, comment, record,
// transition, query, close — is driven through the real Carina verb surface in
// a child process; the parent ptrace-traces every syscall the child (and each of
// its threads) makes and asserts none is a socket-family call. The trace is the
// evidence: a real list of the syscalls the lifecycle used, network count zero.
//
// This is pure stdlib ptrace — no strace, no external tool, no dependency — so
// it is hermetic (AGENTS.md Go rules). It is Linux-only (ptrace) and needs the
// per-GOARCH syscall table in the sibling arch files; on an unsupported arch or
// where the sandbox forbids ptrace it skips with a stated reason rather than
// masquerading a coverage gap as a pass.

// unixExitKill is PTRACE_O_EXITKILL — send SIGKILL to the tracee if the tracer
// dies, so a killed test never orphans the traced child. Defined locally because
// the stdlib syscall package does not export it on every version.
const unixExitKill = 0x00100000

// TestHermeticLifecycle re-execs the child under a syscall tracer and asserts the
// full lifecycle completes (child exit 0) with zero network syscalls in the
// trace (A-6, FR-17).
func TestHermeticLifecycle(t *testing.T) {
	if !hermeticArchSupported {
		t.Skipf("syscall trace unsupported on GOARCH=%s (no syscall table); the hermetic proof runs on amd64/arm64 Linux", runtime.GOARCH)
	}
	root := t.TempDir()
	outFile := filepath.Join(t.TempDir(), "child.out")

	seen, exitCode, err := traceLifecycle(os.Args[0], root, outFile)
	if err != nil {
		// A sandbox that forbids ptrace (yama scope, seccomp) cannot host this
		// proof; skip loudly rather than fail. It runs anywhere ptrace of a child
		// is permitted (developer machines, standard CI).
		t.Skipf("cannot trace child (ptrace unavailable in this sandbox): %v", err)
	}
	childOut, _ := os.ReadFile(outFile)
	if exitCode != 0 {
		t.Fatalf("hermetic lifecycle child failed (exit %d):\n%s", exitCode, childOut)
	}

	net := networkSyscallSet()
	var offending []string
	for nr := range seen {
		if name, ok := net[nr]; ok {
			offending = append(offending, name)
		}
	}
	sort.Strings(offending)
	if len(offending) > 0 {
		t.Fatalf("hermetic lifecycle made network syscalls %v — FR-17/DINV-7 requires zero", offending)
	}
	t.Logf("hermetic lifecycle: %d distinct syscalls traced across all threads, 0 network (GOARCH=%s)", len(seen), runtime.GOARCH)
}

// traceLifecycle forks the test binary in child mode under ptrace and single-
// steps it at syscall granularity, recording every syscall number every thread
// makes. It returns the set of syscall numbers seen, the child's exit code, and
// any setup error (a permission failure the caller turns into a skip).
//
// It handles the whole thread tree: PTRACE_O_TRACECLONE|FORK|VFORK auto-attach
// each new thread/process the Go runtime spawns, so a network syscall on any
// thread is caught, not just the main one. The tracer thread is locked to its
// OS thread for the duration — all ptrace calls for a tracee must issue from the
// one tracer task.
func traceLifecycle(binary, root, outFile string) (seen map[uint64]bool, exitCode int, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	devnull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = devnull.Close() }() // read-only fixture; a close error changes nothing
	out, err := os.Create(outFile)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = out.Close() }() // the child writes this file and the test reads it after exit

	argv := []string{binary, "-test.run=^TestHermeticLifecycleChild$", "-test.count=1"}
	attr := &syscall.ProcAttr{
		Env:   append(os.Environ(), hermeticRootEnv+"="+root),
		Files: []uintptr{devnull.Fd(), out.Fd(), out.Fd()},
		Sys:   &syscall.SysProcAttr{Ptrace: true},
	}
	pid, err := syscall.ForkExec(binary, argv, attr)
	if err != nil {
		return nil, 0, err
	}

	var ws syscall.WaitStatus
	// The initial stop: the child stops on exec (PTRACE_TRACEME + exec).
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return nil, 0, err
	}
	opts := syscall.PTRACE_O_TRACESYSGOOD | syscall.PTRACE_O_TRACECLONE |
		syscall.PTRACE_O_TRACEFORK | syscall.PTRACE_O_TRACEVFORK | unixExitKill
	if err := syscall.PtraceSetOptions(pid, opts); err != nil {
		return nil, 0, err
	}

	seen = map[uint64]bool{}
	live := map[int]bool{pid: true}
	exitCode = -1
	if err := syscall.PtraceSyscall(pid, 0); err != nil {
		return nil, 0, err
	}

	for len(live) > 0 {
		tid, werr := syscall.Wait4(-1, &ws, 0, nil)
		if werr != nil {
			if werr == syscall.ECHILD {
				break
			}
			if werr == syscall.EINTR {
				continue
			}
			return nil, 0, werr
		}
		switch {
		case ws.Exited():
			if tid == pid {
				exitCode = ws.ExitStatus()
			}
			delete(live, tid)
		case ws.Signaled():
			if tid == pid {
				exitCode = 128 + int(ws.Signal())
			}
			delete(live, tid)
		case ws.Stopped():
			live[tid] = true
			if ws.StopSignal() == syscall.SIGTRAP|0x80 {
				// A syscall-entry-or-exit stop: read the syscall number (preserved
				// across the syscall in the nr register on both supported arches).
				var regs syscall.PtraceRegs
				if regErr := syscall.PtraceGetRegs(tid, &regs); regErr == nil {
					seen[syscallNumber(&regs)] = true
				}
				_ = syscall.PtraceSyscall(tid, 0)
			} else {
				// A clone/fork event stop or a new-thread SIGSTOP: resume,
				// suppressing the signal (the short lifecycle relies on none).
				_ = syscall.PtraceSyscall(tid, 0)
			}
		}
	}
	if exitCode == -1 {
		exitCode = 0
	}
	return seen, exitCode, nil
}
