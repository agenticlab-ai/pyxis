package runtime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLaunchBrokerOwnerSharesOneListenerAndCannotRestartAfterClose(t *testing.T) {
	owner := &launchBrokerOwner{ctx: t.Context()}
	t.Cleanup(func() {
		if err := owner.close(); err != nil {
			t.Error(err)
		}
	})
	var wg sync.WaitGroup
	brokers := make(chan *launchBroker, 8)
	for range cap(brokers) {
		wg.Go(func() {
			broker, err := owner.get(t.Context())
			if err != nil {
				t.Errorf("start broker: %v", err)
				return
			}
			brokers <- broker
		})
	}
	wg.Wait()
	close(brokers)
	var first *launchBroker
	for broker := range brokers {
		if first == nil {
			first = broker
		} else if broker != first {
			t.Fatal("concurrent launches created different listeners")
		}
	}
	if first == nil {
		t.Fatal("no launch broker was created")
	}
	if err := owner.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.get(t.Context()); err == nil || !strings.Contains(err.Error(), "owner is closed") {
		t.Fatalf("closed owner accepted a launch: %v", err)
	}
	select {
	case <-first.done:
	default:
		t.Fatal("cleanup did not join the listener")
	}
}

func TestLaunchBrokerOwnerRejectsCancellationBeforeStarting(t *testing.T) {
	for _, canceled := range []string{"owner", "caller"} {
		t.Run(canceled, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			caller, cancelCaller := context.WithCancel(t.Context())
			defer cancelCaller()
			if canceled == "owner" {
				cancel()
			} else {
				cancelCaller()
			}
			owner := &launchBrokerOwner{ctx: ctx}
			defer func() {
				if err := owner.close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := owner.get(caller); !errors.Is(err, context.Canceled) || owner.broker != nil {
				t.Fatalf("canceled launch started a listener: %v", err)
			}
		})
	}
}

func testLaunchBroker(t *testing.T, wrap func(*exec.Cmd, accessPaths) error) (*launchBroker, launchCapability, accessPaths) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	paths := accessPaths{work: filepath.Join(root, "work"), state: filepath.Join(root, "state")}
	for _, path := range []string{paths.work, paths.state} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	b, err := startLaunchBroker(t.Context(), wrap)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.close(); err != nil {
			t.Error(err)
		}
	})
	ref, err := b.grant(paths)
	if err != nil {
		t.Fatal(err)
	}
	return b, ref, paths
}

func launchRequestForTest(t *testing.T, paths accessPaths) launchRequest {
	t.Helper()
	return launchRequest{Schema: launchSchema, Path: os.Args[0], Args: []string{os.Args[0], "-test.run=^TestLaunchEchoHelper$"}, Env: []string{"PYXIS_LAUNCH_ECHO=1", "GOCOVERDIR=" + paths.state, "LD_PRELOAD=forbidden", "DYLD_INSERT_LIBRARIES=forbidden"}, Paths: wireLaunchPaths(paths)}
}

func TestLaunchEchoHelper(t *testing.T) {
	if os.Getenv("PYXIS_LAUNCH_ECHO") == "" {
		return
	}
	if os.Getenv("LD_PRELOAD") != "" || os.Getenv("DYLD_INSERT_LIBRARIES") != "" || os.Getenv(launchCapabilityEnv) == "" {
		os.Exit(90)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(91)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		os.Exit(92)
	}
	if _, err := os.Stderr.Write([]byte("separate stderr")); err != nil {
		os.Exit(93)
	}
	os.Exit(7)
}

func TestLaunchProxyPreservesStreamsAndExitStatus(t *testing.T) {
	_, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
	request := launchRequestForTest(t, paths)
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	input := strings.Repeat("streamed input\n", 4000)
	var stdout, stderr bytes.Buffer
	code, err := runLaunchProxy(t.Context(), data, io.NopCloser(strings.NewReader(input)), &stdout, &stderr)
	if err != nil || code != 7 || stdout.String() != input || stderr.String() != "separate stderr" {
		t.Fatalf("code=%d err=%v stdout=%d stderr=%q", code, err, stdout.Len(), stderr.String())
	}
}

func TestLaunchBrokerRejectsUntrustedRequests(t *testing.T) {
	_, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error {
		t.Error("unauthorized command reached wrapper")
		return errors.New("forbidden")
	})
	for _, tc := range []struct {
		name  string
		alter func(*launchRequest)
	}{
		{"token", func(r *launchRequest) { r.Token = "forged" }},
		{"schema", func(r *launchRequest) { r.Schema = "unknown" }},
		{"empty argv", func(r *launchRequest) { r.Args = nil }},
		{"empty path", func(r *launchRequest) { r.Path = "" }},
		{"write root", func(r *launchRequest) { r.Paths.State = t.TempDir() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := launchRequestForTest(t, paths)
			request.Token = ref.Token
			tc.alter(&request)
			conn, err := net.Dial("tcp4", ref.Address)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }() // test owns the connection
			if err := writeLaunchJSON(conn, request); err != nil {
				t.Fatal(err)
			}
			var reply launchFrame
			if err := readLaunchJSON(conn, &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Kind != "exit" || reply.ExitCode != 125 || reply.Error == "" {
				t.Fatalf("accepted request: %+v", reply)
			}
		})
	}
	conn, err := net.Dial("tcp4", ref.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }() // test owns the connection
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	var reply launchFrame
	if err := readLaunchJSON(conn, &reply); err != nil || reply.Error == "" {
		t.Fatalf("malformed request: %+v %v", reply, err)
	}
}

func TestLaunchBrokerWrapperFailureAndRevocation(t *testing.T) {
	b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return errors.New("native boundary unavailable") })
	t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
	data, err := json.Marshal(launchRequestForTest(t, paths))
	if err != nil {
		t.Fatal(err)
	}
	code, err := runLaunchProxy(t.Context(), data, io.NopCloser(strings.NewReader("")), io.Discard, io.Discard)
	if code != 125 || err == nil || !strings.Contains(err.Error(), "native boundary unavailable") {
		t.Fatalf("%d %v", code, err)
	}
	if err := b.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.grant(paths); err == nil {
		t.Fatal("closed broker granted a capability")
	}
	if _, err := net.Dial("tcp4", ref.Address); err == nil {
		t.Fatal("listener survived cleanup")
	}
}

func TestLaunchBrokerMissingExecutableFailsClosed(t *testing.T) {
	_, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
	request := launchRequestForTest(t, paths)
	request.Path = filepath.Join(paths.work, "missing")
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	code, err := runLaunchProxy(t.Context(), data, io.NopCloser(strings.NewReader("")), io.Discard, io.Discard)
	if code != 125 || err == nil {
		t.Fatal("missing executable produced a verdict")
	}
}

func TestLaunchBrokerDisconnectCancelsCommand(t *testing.T) {
	started := make(chan struct{})
	b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { close(started); return nil })
	request := launchRequestForTest(t, paths)
	request.Token = ref.Token
	conn, err := net.Dial("tcp4", ref.Address)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLaunchJSON(conn, request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-t.Context().Done():
		t.Fatal("launch did not start")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if err := b.close(); err != nil {
			t.Error(err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not join cancelled launch")
	}
}

func TestLaunchBrokerOwnerCancellationClosesPendingConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	b, err := newLaunchBroker(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp4", b.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }() // test owns the connection
	cancel()
	if err := b.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("pending connection survived owner cancellation")
	}
}

func TestLaunchBrokerSuccessRevokesChildScope(t *testing.T) {
	b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
	request := launchRequestForTest(t, paths)
	request.Path, request.Args = "/usr/bin/true", []string{"/usr/bin/true"}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := runLaunchProxy(t.Context(), data, io.NopCloser(strings.NewReader("")), io.Discard, io.Discard); code != 0 || err != nil {
		t.Fatalf("successful command lost its verdict: %d %v", code, err)
	}
	b.wg.Wait() // this request has completed; no new connection is being created
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.scopes) != 1 {
		t.Fatal("completed child retained launch authority")
	}
}

func TestLaunchBrokerInvalidInputCancelsWaitingChild(t *testing.T) {
	for _, frames := range [][]launchFrame{
		{{Schema: launchSchema, Kind: "unknown"}},
		{{Schema: launchSchema, Kind: "stdin-eof"}, {Schema: launchSchema, Kind: "stdin", Data: []byte("after EOF")}},
		{{Schema: launchSchema, Kind: "stdin-eof"}, {Schema: launchSchema, Kind: "stdin-eof"}},
	} {
		t.Run(frames[len(frames)-1].Kind, func(t *testing.T) {
			b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
			request := launchRequestForTest(t, paths)
			request.Token = ref.Token
			conn, err := net.Dial("tcp4", ref.Address)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }() // test owns the protocol client
			if err := writeLaunchJSON(conn, request); err != nil {
				t.Fatal(err)
			}
			for _, frame := range frames {
				if err := writeLaunchJSON(conn, frame); err != nil {
					t.Fatal(err)
				}
			}
			for {
				var reply launchFrame
				if err := readLaunchJSON(conn, &reply); err != nil {
					t.Fatal(err)
				}
				if reply.Kind == "exit" {
					if reply.ExitCode == 0 {
						t.Fatal("invalid input produced success")
					}
					break
				}
			}
			b.wg.Wait() // all I/O and the cancelled process must be joined
		})
	}
}

type failingLaunchDeadline struct {
	net.Conn
	input  *bytes.Reader
	calls  int
	failAt int
}

func (c *failingLaunchDeadline) Read(data []byte) (int, error) { return c.input.Read(data) }
func (*failingLaunchDeadline) Write(data []byte) (int, error)  { return len(data), nil }
func (c *failingLaunchDeadline) SetReadDeadline(time.Time) error {
	c.calls++
	if c.calls == c.failAt {
		return io.ErrClosedPipe
	}
	return nil
}

func TestLaunchBrokerDeadlineFailureNeverStartsCommand(t *testing.T) {
	b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error {
		t.Error("deadline failure reached native launch")
		return nil
	})
	request := launchRequestForTest(t, paths)
	request.Token = ref.Token
	var wire bytes.Buffer
	if err := writeLaunchJSON(&wire, request); err != nil {
		t.Fatal(err)
	}
	for _, failAt := range []int{1, 2} {
		b.serve(&failingLaunchDeadline{input: bytes.NewReader(wire.Bytes()), failAt: failAt})
		b.mu.Lock()
		remaining := len(b.scopes)
		b.mu.Unlock()
		if remaining != 1 {
			t.Fatal("failed setup retained child authority")
		}
	}
}

type failedLaunchIO struct{}

func (failedLaunchIO) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (failedLaunchIO) Read([]byte) (int, error)  { return 0, io.ErrClosedPipe }
func (failedLaunchIO) Close() error              { return nil }

func TestLaunchFramesAreBoundedAndClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"truncated prefix", []byte{0}},
		{"empty frame", []byte{0, 0, 0, 0}},
		{"oversize", []byte{0, 16, 0, 1}},
		{"truncated data", []byte{0, 0, 0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := readLaunchJSON(bytes.NewReader(tc.data), new(launchFrame)); err == nil {
				t.Fatal("accepted invalid frame")
			}
		})
	}
	for _, data := range []string{`{"schema_version":"confined-launch/v1","unknown":true}`, `{} {}`} {
		var framed bytes.Buffer
		var prefix [4]byte
		binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
		framed.Write(prefix[:])
		framed.WriteString(data)
		if err := readLaunchJSON(&framed, new(launchFrame)); err == nil {
			t.Fatal("accepted non-closed JSON")
		}
	}
	if err := writeLaunchJSON(io.Discard, make(chan int)); err == nil {
		t.Fatal("accepted unserializable frame")
	}
	if err := writeLaunchJSON(io.Discard, strings.Repeat("x", maxLaunchFrame)); err == nil {
		t.Fatal("accepted oversize frame")
	}
	if err := writeLaunchJSON(failedLaunchIO{}, launchFrame{}); err == nil {
		t.Fatal("ignored failed write")
	}
	writer := &launchWriter{conn: failedLaunchIO{}}
	if _, err := writer.stream("stdout").Write([]byte("test")); err == nil {
		t.Fatal("ignored stream error")
	}
}

func TestLaunchProxyFailsClosedOnBadCapabilitiesAndRequests(t *testing.T) {
	_, ref, _ := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	valid := strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"=")
	for _, tc := range []struct{ name, capability, request string }{
		{"capability", "{}", "{}"},
		{"empty JSON", valid, ""},
		{"JSON", valid, "{"},
		{"size", valid, strings.Repeat("x", maxLaunchFrame+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(launchCapabilityEnv, tc.capability)
			code, err := runLaunchProxy(t.Context(), []byte(tc.request), io.NopCloser(strings.NewReader("")), io.Discard, io.Discard)
			if code != 125 || err == nil {
				t.Fatalf("%d %v", code, err)
			}
		})
	}
	t.Setenv(launchCapabilityEnv, valid)
	cmd := exec.Command("unused")
	cmd.ExtraFiles = []*os.File{os.Stdin}
	if err := confineProcess(cmd, accessPaths{}); err == nil {
		t.Fatal("accepted inherited descriptor")
	}
	cmd.ExtraFiles = nil
	cmd.Args = []string{strings.Repeat("x", maxLaunchFrame)}
	if err := confineProcess(cmd, accessPaths{}); err == nil {
		t.Fatal("accepted oversize delegated command")
	}
}
