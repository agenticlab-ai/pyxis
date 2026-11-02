package runtime

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchProxyPrivateRequestAndDispatch(t *testing.T) {
	_, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
	cmd := exec.Command(os.Args[0], "-test.run=^TestLaunchEchoHelper$")
	cmd.Env = []string{"PYXIS_LAUNCH_ECHO=1", "GOCOVERDIR=" + paths.state, "SECRET=private-value"}
	if err := confineProcess(cmd, paths); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(cmd.Args, " "), "private-value") {
		t.Fatal("environment leaked into proxy argv")
	}
	info, err := os.Stat(cmd.Args[2])
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("request permissions: %v %v", info, err)
	}
	var stdout, stderr bytes.Buffer
	code, handled := launchProxyMain(cmd.Args, io.NopCloser(strings.NewReader("round trip")), &stdout, &stderr)
	if !handled || code != 7 || stdout.String() != "round trip" || stderr.String() != "separate stderr" {
		t.Fatalf("%d %v %q %q", code, handled, stdout.String(), stderr.String())
	}
	if _, handled := launchProxyMain([]string{"ordinary"}, nil, nil, nil); handled {
		t.Fatal("intercepted ordinary process")
	}
	if code, handled := launchProxyMain([]string{"proxy", launchProxyArg, "missing"}, nil, io.Discard, io.Discard); !handled || code != 125 {
		t.Fatal("missing request did not fail closed")
	}
	if _, err := readLaunchRequest(paths.work); err == nil {
		t.Fatal("accepted directory request")
	}
	large := filepath.Join(paths.state, "oversize")
	if err := os.WriteFile(large, bytes.Repeat([]byte{'x'}, maxLaunchFrame+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readLaunchRequest(large); err == nil {
		t.Fatal("read unbounded request")
	}
	cmd = exec.Command("unused")
	if err := confineProcess(cmd, accessPaths{state: filepath.Join(paths.state, "missing")}); err == nil {
		t.Fatal("ignored request creation failure")
	}
	t.Setenv(launchCapabilityEnv, "{}")
	if _, err := readLaunchRequest(cmd.Args[0]); err == nil {
		t.Fatal("read file with malformed capability")
	}
	if err := confineProcess(cmd, paths); err == nil {
		t.Fatal("accepted malformed capability")
	}
	if err := os.Unsetenv(launchCapabilityEnv); err != nil {
		t.Fatal(err)
	}
	if _, err := readLaunchRequest(large); err == nil {
		t.Fatal("read without a capability")
	}
	if code, err := runLaunchProxy(t.Context(), nil, nil, nil, nil); code != 125 || err == nil {
		t.Fatal("dispatched without capability")
	}
}

func TestLaunchProxyRejectsMalformedReplies(t *testing.T) {
	for _, frame := range []launchFrame{
		{Schema: "unknown", Kind: "exit"},
		{Schema: launchSchema, Kind: "unknown"},
		{Schema: launchSchema, Kind: "exit", ExitCode: 256},
		{Schema: launchSchema, Kind: "stdout", Data: []byte("output")},
		{Schema: launchSchema, Kind: "stderr", Data: []byte("error")},
	} {
		t.Run(frame.Schema+frame.Kind, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = listener.Close() }() // test owns the listener
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }() // server owns the connection
				if err := readLaunchJSON(conn, new(launchRequest)); err != nil {
					return
				}
				_ = writeLaunchJSON(conn, frame) // the client may deliberately reject the reply
			}()
			defer func() {
				_ = listener.Close() // unblock the owned server before joining
				<-done
			}()
			ref := launchCapability{Schema: launchSchema, Address: listener.Addr().String(), Token: strings.Repeat("a", 64), Proxy: os.Args[0], State: t.TempDir()}
			t.Setenv(launchCapabilityEnv, strings.TrimPrefix(withLaunchCapability(nil, ref)[0], launchCapabilityEnv+"="))
			data, err := json.Marshal(launchRequest{Schema: launchSchema})
			if err != nil {
				t.Fatal(err)
			}
			code, err := runLaunchProxy(t.Context(), data, io.NopCloser(strings.NewReader("")), failedLaunchIO{}, failedLaunchIO{})
			if code != 125 || err == nil {
				t.Fatalf("accepted reply: %d %v", code, err)
			}
		})
	}
}
