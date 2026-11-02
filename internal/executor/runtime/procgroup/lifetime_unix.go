//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type processIdentity struct {
	pid, parent     int
	session         int
	born, fraction  uint64
	stopped, exited bool
}

func (p processIdentity) same(q processIdentity) bool {
	return p.pid == q.pid && p.born == q.born && p.fraction == q.fraction
}

// Lifetime owns private-session members and observed descendants independently
// of their process groups. The direct child stays unreaped until cleanup.
// Kernel birth identities prevent a recycled PID from becoming kill authority.
// This is command cleanup, not a sandbox: persistent external daemons must not
// be used as native command executors. Delegated commands have their own owners.
type Lifetime struct {
	cmd     *exec.Cmd
	mu      sync.Mutex
	known   map[int]processIdentity
	root    processIdentity
	session int
	cancel  context.CancelFunc
	done    chan struct{}
	closed  bool
	once    sync.Once
	err     error
	grace   time.Duration
	list    func(context.Context) (map[int]processIdentity, error)
	read    func(int) (processIdentity, error)
	signal  func(int, syscall.Signal) error
}

func NewLifetime(cmd *exec.Cmd) *Lifetime {
	return &Lifetime{cmd: cmd, known: make(map[int]processIdentity), grace: DefaultGrace, list: listProcesses, read: readProcess, signal: syscall.Kill}
}

// ObserveStart precedes handing the process to Await. The direct child is
// unreaped here, so its birth identity cannot refer to a replacement process.
func (s *Lifetime) ObserveStart() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	} // cancellation can win immediately after Start
	if s.cmd.Process == nil {
		return errors.New("process lifetime: command not started")
	}
	if len(s.known) == 0 {
		if err := s.captureStart(); err != nil {
			return err
		}
	}
	if s.cancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel, s.done = cancel, make(chan struct{})
		go s.observe(ctx, s.done)
	}
	return nil
}

func (s *Lifetime) captureStart() error {
	pid := s.cmd.Process.Pid
	root, err := s.read(pid)
	if errors.Is(err, syscall.ESRCH) && s.cmd.ProcessState == nil && s.cmd.SysProcAttr != nil && s.cmd.SysProcAttr.Setsid {
		// Start established this private session before returning. An exited,
		// unreaped direct child still reserves its ID; it can own the session
		// even when the kernel no longer exposes its birth identity. Do not use
		// the missing birth identity to authorize a signal to any process.
		root, err = processIdentity{pid: pid, session: pid, exited: true}, nil
	}
	if err != nil {
		return fmt.Errorf("process lifetime: capture child identity pid %d: %w", pid, err)
	}
	return s.captureRoot(root)
}

func (s *Lifetime) captureRoot(root processIdentity) error {
	s.known[root.pid], s.root = root, root
	if s.cmd.SysProcAttr != nil && s.cmd.SysProcAttr.Setsid {
		if root.session != root.pid && !root.exited {
			return errors.New("process lifetime: private session was not established")
		}
		s.session = root.pid
	}
	return nil
}

func (s *Lifetime) observe(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processes, err := s.list(ctx)
			if err != nil {
				continue
			} // forced cleanup performs a fresh, fail-closed read
			s.mu.Lock()
			s.collect(processes)
			s.mu.Unlock()
		}
	}
}

// collect admits members of the unreaped root's private session, or children of
// the same live, already-owned parent identity. Retained birth identities still
// authorize cleanup after a descendant changes session or is reparented to init.
func (s *Lifetime) collect(processes map[int]processIdentity) {
	for changed := true; changed; {
		changed = false
		for pid, p := range processes {
			if old, known := s.known[pid]; known && old.same(p) {
				continue
			}
			// The direct child must remain unreaped while this session is used as
			// authority. The kernel reserves its ID, even after an intermediate
			// parent disappears. A recycled root never authorizes new members.
			if s.session != 0 && s.cmd.ProcessState == nil && p.session == s.session {
				s.known[pid] = p
				changed = true
				continue
			}
			parent, owned := s.known[p.parent]
			current, present := processes[p.parent]
			if owned && present && parent.same(current) && !current.exited {
				s.known[pid] = p
				changed = true
			}
		}
	}
}

// WaitExit observes termination without reaping the direct child. Cleanup must
// run before exec.Cmd.Wait releases its PID and private-session identity.
func (s *Lifetime) WaitExit(ctx context.Context) error {
	s.mu.Lock()
	root := s.root
	s.mu.Unlock()
	if root.exited {
		return nil
	}
	for {
		p, err := s.read(root.pid)
		if errors.Is(err, syscall.ESRCH) {
			return nil // the owned direct child has exited but has not been reaped
		}
		if err != nil {
			return fmt.Errorf("process lifetime: observe exit of pid %d before reap: %w", root.pid, err)
		}
		if !root.same(p) {
			return errors.New("process lifetime: direct child identity changed before reap")
		}
		if p.exited {
			return nil
		}
		if err := waitSnapshot(ctx); err != nil {
			return err
		}
	}
}

// Kill freezes the owned family before terminating it, preventing a concurrent
// fork from escaping the final ancestry read. It waits for every owned identity
// to be absent or exited before acknowledging quiescence. Permission, inspection
// and timeout failures remain errors; no name-based or cross-task killing occurs.
func (s *Lifetime) Kill() error {
	s.once.Do(func() { s.err = s.stop() })
	return s.err
}

func (s *Lifetime) stop() error {
	s.mu.Lock()
	s.closed = true
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd.Process == nil {
		return nil
	}
	if len(s.known) == 0 {
		err := s.captureStart()
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	ctx, cancelStop := context.WithTimeout(context.Background(), s.grace)
	defer cancelStop()
	// Even failed inspection or freezing must attempt to stop identities already
	// owned. Preserve the failure rather than certifying incomplete cleanup.
	var stopErr error
	if s.session != 0 && s.cmd.ProcessState != nil {
		stopErr = errors.New("process lifetime: private-session root was reaped before cleanup")
	}
	// Stop known producers before the machine-wide discovery pass. Otherwise a
	// busy parent can keep forking while that pass runs, and even a single root
	// requires another complete scan just to observe its stop acknowledgement.
	// Discovery below still captures new session members and proves every owned
	// identity frozen; a failed signal never becomes a quiescence certificate.
	for _, owned := range s.known {
		stopErr = errors.Join(stopErr, s.send(owned, syscall.SIGSTOP))
	}
	for {
		processes, err := s.list(ctx)
		if err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("process lifetime: inspect owned family: %w", err))
			break
		}
		s.collect(processes)
		frozen := true
		for pid, owned := range s.known {
			p, present := processes[pid]
			if !present || !owned.same(p) || p.exited || p.stopped {
				continue
			}
			frozen = false
			stopErr = errors.Join(stopErr, s.send(owned, syscall.SIGSTOP))
		}
		if frozen || stopErr != nil {
			break
		}
		if err := waitSnapshot(ctx); err != nil {
			stopErr = errors.Join(stopErr, err)
			break
		}
	}
	var killErr error
	for _, owned := range s.known {
		killErr = errors.Join(killErr, s.send(owned, syscall.SIGKILL))
	}
	if killErr != nil {
		return errors.Join(stopErr, killErr)
	}
	for {
		live := false
		for _, owned := range s.known {
			p, err := s.read(owned.pid)
			if errors.Is(err, syscall.ESRCH) {
				continue
			}
			if err != nil {
				return errors.Join(stopErr, fmt.Errorf("process lifetime: confirm exit pid %d: %w", owned.pid, err))
			}
			live = live || (owned.same(p) && !p.exited)
		}
		if !live {
			return stopErr
		}
		if err := waitSnapshot(ctx); err != nil {
			return errors.Join(stopErr, err)
		}
	}
}

func (s *Lifetime) send(owned processIdentity, signal syscall.Signal) error {
	p, err := s.read(owned.pid)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("process lifetime: identify signal target pid %d for %s: %w", owned.pid, signal, err)
	}
	if !owned.same(p) || p.exited {
		return nil
	}
	if err := s.signal(owned.pid, signal); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("process lifetime: signal owned pid %d: %w", owned.pid, err)
	}
	return nil
}

func waitSnapshot(ctx context.Context) error {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("process lifetime: stop acknowledgement: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
