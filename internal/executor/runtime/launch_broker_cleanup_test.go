package runtime

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

func TestBrokerCleanupRequiresExitAcknowledgement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		conn, peer := net.Pipe()
		defer func() {
			if err := peer.Close(); err != nil {
				t.Errorf("close fixture transport: %v", err)
			}
		}()
		done := make(chan struct{})
		close(done)
		b := &launchBroker{ctx: ctx, cancel: cancel, listener: closedLaunchListener{}, done: done, connections: map[net.Conn]bool{conn: true}}
		b.wg.Add(1)
		// The accept loop is already joined, but a delegated command has not
		// acknowledged its stop. Closing transport alone must not certify it.
		if err := b.close(); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unacknowledged cleanup: %v", err)
		}
		delete(b.connections, conn)
		b.wg.Done()
		if err := b.close(); err != nil {
			t.Fatalf("acknowledged cleanup: %v", err)
		}
	})
}

type closedLaunchListener struct{}

func (closedLaunchListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (closedLaunchListener) Close() error              { return nil }
func (closedLaunchListener) Addr() net.Addr            { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }

func TestBrokerCleanupDoesNotWaitForeverAfterRefusedSignal(t *testing.T) {
	b, err := startLaunchBroker(t.Context(), func(*exec.Cmd, accessPaths) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	conn, peer := net.Pipe()
	defer func() {
		if err := peer.Close(); err != nil {
			t.Errorf("close fixture transport: %v", err)
		}
	}()
	b.mu.Lock()
	b.connections[conn] = true
	b.wg.Add(1)
	b.mu.Unlock()
	if err := b.kill(refusedLifetime{}); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("refused signal disappeared: %v", err)
	}
	// Represent the owned handler that cannot acknowledge a denied stop. The
	// cleanup must fail closed without blocking on that handler indefinitely.
	done := make(chan error, 1)
	go func() { done <- b.close() }()
	select {
	case err := <-done:
		if !errors.Is(err, syscall.EPERM) {
			t.Fatalf("cleanup hid refused signal: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("cleanup hung after a refused signal")
	}
	b.mu.Lock()
	delete(b.connections, conn)
	b.mu.Unlock()
	b.wg.Done()
	if err := b.close(); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("cleanup changed its failed outcome: %v", err)
	}
}

type refusedLifetime struct{}

func (refusedLifetime) Kill() error { return syscall.EPERM }
