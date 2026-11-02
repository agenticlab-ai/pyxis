package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

type launchBrokerKey struct{}

// Acquiring a verification context must not start background I/O. Executors
// can exhaust their budget before any verifier runs, including under synctest.
// The owner starts one broker on its first protected launch and joins it at
// cleanup; a closed owner can never recreate the listener.
type launchBrokerOwner struct {
	ctx    context.Context
	mu     sync.Mutex
	broker *launchBroker
	closed bool
}

func (o *launchBrokerOwner) get(ctx context.Context) (*launchBroker, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, errors.New("runtime: verification launch owner is closed")
	}
	if err := o.ctx.Err(); err != nil {
		return nil, err
	}
	if o.broker == nil {
		broker, err := newLaunchBroker(o.ctx)
		if err != nil {
			return nil, fmt.Errorf("runtime: initialize verification launch broker: %w", err)
		}
		o.broker = broker
	}
	return o.broker, nil
}

func (o *launchBrokerOwner) close() error {
	o.mu.Lock()
	o.closed = true
	broker := o.broker
	o.mu.Unlock()
	if broker != nil {
		return broker.close()
	}
	return nil
}

// The verification supervisor owns this listener, all its connections and all
// delegated process groups. Cleanup cancels launches and joins their handlers.
// Children only receive a capability for the write policy already imposed on
// them; knowing the endpoint or inventing an environment marker grants nothing.
type launchBroker struct {
	ctx         context.Context
	cancel      context.CancelFunc
	listener    net.Listener
	proxy       string
	wrap        func(*exec.Cmd, accessPaths) error
	done        chan struct{}
	once        sync.Once
	wg          sync.WaitGroup
	mu          sync.Mutex
	scopes      map[string]launchScope
	connections map[net.Conn]bool
	stopErr     error
}

type launchScope struct {
	paths accessPaths
	close func()
}

type launchRequest struct {
	Schema string      `json:"schema_version"`
	Token  string      `json:"token"`
	Path   string      `json:"path"`
	Args   []string    `json:"args"`
	Env    []string    `json:"env"`
	Paths  launchPaths `json:"paths"`
}

func newLaunchBroker(ctx context.Context) (*launchBroker, error) {
	return startLaunchBroker(ctx, confineNativeProcess)
}

func startLaunchBroker(ctx context.Context, wrap func(*exec.Cmd, accessPaths) error) (*launchBroker, error) {
	proxy, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("runtime: launch proxy executable: %w", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("runtime: launch broker: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	b := &launchBroker{ctx: ctx, cancel: cancel, listener: listener, proxy: proxy, wrap: wrap, done: make(chan struct{}), scopes: make(map[string]launchScope), connections: make(map[net.Conn]bool)}
	go b.accept()
	context.AfterFunc(ctx, func() { _ = b.close() }) // the owner also joins and reads cleanup errors
	return b, nil
}

func (b *launchBroker) grant(paths accessPaths) (launchCapability, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return launchCapability{}, fmt.Errorf("runtime: launch capability: %w", err)
	}
	paths, closeRoots, err := openLaunchScope(paths)
	if err != nil {
		return launchCapability{}, err
	}
	ref := launchCapability{Schema: launchSchema, Address: b.listener.Addr().String(), Token: hex.EncodeToString(token[:]), Proxy: b.proxy, State: paths.state}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		closeRoots()
		return launchCapability{}, err
	}
	b.scopes[ref.Token] = launchScope{paths: paths, close: closeRoots}
	return ref, nil
}

func (b *launchBroker) close() error {
	b.once.Do(func() {
		b.cancel()
		_ = b.listener.Close() // closing releases the owned accept loop
		b.mu.Lock()
		for conn := range b.connections {
			_ = conn.Close() // closing cancels the owned launch and stdin reader
		}
		b.mu.Unlock()
	})
	<-b.done
	// A refused signal can leave a handler waiting in Cmd.Wait. Do not let
	// cleanup itself become unbounded or report that handler as quiescent.
	ctx, cancel := context.WithTimeout(context.Background(), procgroup.DefaultGrace)
	defer cancel()
	for {
		b.mu.Lock()
		active, stopErr := len(b.connections), b.stopErr
		b.mu.Unlock()
		if active == 0 {
			break
		}
		if stopErr != nil {
			return stopErr
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("delegated commands did not acknowledge cleanup: %w", ctx.Err())
		case <-timer.C:
		}
	}
	b.wg.Wait()
	b.mu.Lock()
	defer b.mu.Unlock()
	for token, scope := range b.scopes {
		scope.close()
		delete(b.scopes, token)
	}
	return b.stopErr
}

func (b *launchBroker) kill(lifetime interface{ Kill() error }) error {
	err := lifetime.Kill()
	if err != nil {
		b.mu.Lock()
		b.stopErr = errors.Join(b.stopErr, fmt.Errorf("stop delegated command: %w", err))
		b.mu.Unlock()
	}
	return err
}

func (b *launchBroker) accept() {
	defer close(b.done)
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return // cancellation/cleanup closes the listener
		}
		b.mu.Lock()
		if len(b.connections) >= 64 || b.ctx.Err() != nil {
			b.mu.Unlock()
			_ = conn.Close() // reject excess connections without spawning work
			continue
		}
		b.connections[conn] = true
		b.mu.Unlock()
		b.wg.Go(func() {
			defer func() {
				_ = conn.Close() // the handler owns this connection
				b.mu.Lock()
				delete(b.connections, conn)
				b.mu.Unlock()
			}()
			b.serve(conn)
		})
	}
}

func (b *launchBroker) serve(conn net.Conn) {
	reply := &launchWriter{conn: conn}
	// Bound unauthenticated requests; successful launches inherit the owning
	// gate's deadline instead. Every frame has a separate size bound.
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}
	var request launchRequest
	if err := readLaunchJSON(conn, &request); err != nil {
		reply.finish(125, err)
		return
	}
	b.mu.Lock()
	parent, ok := b.scopes[request.Token]
	b.mu.Unlock()
	if request.Schema != launchSchema || !ok || len(request.Args) == 0 || request.Path == "" {
		reply.finish(125, errors.New("unauthorized launch request"))
		return
	}
	paths, err := narrowLaunchPaths(parent.paths, request.Paths)
	if err != nil {
		reply.finish(125, err)
		return
	}
	ref, err := b.grant(paths)
	if err != nil {
		reply.finish(125, err)
		return
	}
	defer func() {
		b.mu.Lock()
		scope := b.scopes[ref.Token]
		delete(b.scopes, ref.Token)
		b.mu.Unlock()
		scope.close()
	}()
	b.mu.Lock()
	paths = b.scopes[ref.Token].paths
	b.mu.Unlock()
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return
	}
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	stdin, input, err := os.Pipe()
	if err != nil {
		reply.finish(125, err)
		return
	}
	defer func() { _ = stdin.Close() }() // release the owned child input
	defer func() { _ = input.Close() }() // release the owned forwarding pipe
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = input.Close() }() // disconnect releases the child's stdin
		defer cancel()                       // disconnecting the proxy cancels the whole delegated group
		for {
			var frame launchFrame
			if err := readLaunchJSON(conn, &frame); err != nil || frame.Schema != launchSchema {
				return
			}
			switch frame.Kind {
			case "stdin":
				if _, err := input.Write(frame.Data); err != nil {
					return
				}
			case "stdin-eof":
				if err := input.Close(); err != nil {
					return
				}
			default:
				return
			}
		}
	}()
	defer func() {
		_ = conn.Close()  // unblock the owned input reader before joining it
		_ = input.Close() // unblock a reader currently forwarding stdin
		<-done
	}()
	cmd := exec.CommandContext(ctx, request.Path, request.Args[1:]...)
	cmd.Args, cmd.Dir = request.Args, paths.work
	cmd.Env = withLaunchCapability(cleanLaunchEnv(request.Env), ref)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, reply.stream("stdout"), reply.stream("stderr")
	cmd.WaitDelay = 2 * time.Second
	procgroup.SetOwnedSession(cmd)
	lifetime := procgroup.NewLifetime(cmd)
	cmd.Cancel = func() error { return b.kill(lifetime) }
	paths, release, err := pinLaunchPaths(cmd, paths)
	if err != nil {
		reply.finish(125, err)
		return
	}
	defer release()
	if err := b.wrap(cmd, paths); err != nil {
		reply.finish(125, err)
		return
	}
	err = cmd.Start()
	if err == nil {
		err = lifetime.ObserveStart()
		if err == nil {
			err, _ = procgroup.WaitOwned(ctx, cmd, lifetime) // b.kill below records and surfaces the cached stop error
		} else if killErr := b.kill(lifetime); killErr == nil {
			_ = cmd.Wait() // reap the stopped command whose identity read failed
		}
	}
	if killErr := b.kill(lifetime); killErr != nil {
		reply.finish(125, errors.Join(err, killErr))
		return
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() >= 0 {
		reply.finish(exit.ExitCode(), nil)
	} else if err != nil {
		reply.finish(125, err)
	} else {
		reply.finish(0, nil)
	}
}

func cleanLaunchEnv(env []string) []string {
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		// A loader runs before the native wrapper applies its policy. No request
		// may inject code into that unsandboxed supervisor-side startup.
		if key != launchCapabilityEnv && !strings.HasPrefix(key, "LD_") && !strings.HasPrefix(key, "DYLD_") && key != "GCONV_PATH" {
			clean = append(clean, entry)
		}
	}
	return clean
}
