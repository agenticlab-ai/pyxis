//go:build darwin || linux

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A delegated verifier is parented by the supervisor and has a separate PGID.
// A verifier can delegate another command. Neither is in the model's group;
// killing only the model cannot join them before checkpointing.
func TestInvocationJoinsNestedDelegatedGroupsBeforeReturning(t *testing.T) {
	parent, parentCleanup, err := VerificationContext(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer parentCleanup()
	ctx, stop := InvocationContext(parent)
	defer func() {
		if err := stop(); err != nil {
			t.Error(err)
		}
	}()
	b, ref, paths := testLaunchBroker(t, func(*exec.Cmd, accessPaths) error { return nil })
	// Inject only the OS wrapper seam; exercise the real listener, framing,
	// commands, separate process groups, cancellation and acknowledgement.
	owner := ctx.Value(launchBrokerKey{}).(*launchBrokerOwner)
	owner.broker = b
	request := launchRequestForTest(t, paths)
	request.Args = []string{os.Args[0], "-test.run=^TestInvocationNestedHelper$"}
	request.Env = append(os.Environ(), "PYXIS_INVOCATION_HELPER=nested", "GOCOVERDIR="+paths.state)
	request.Token = ref.Token
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", ref.Address)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }() // test owns the proxy connection
	if err := writeLaunchJSON(conn, request); err != nil {
		t.Fatal(err)
	}
	if err := writeLaunchJSON(conn, launchFrame{Schema: launchSchema, Kind: "stdin-eof"}); err != nil {
		t.Fatal(err)
	}
	var pids []int
	for len(pids) < 2 {
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var frame launchFrame
		if err := readLaunchJSON(conn, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.Kind == "stdout" {
			for s := range strings.FieldsSeq(string(frame.Data)) {
				pid, err := strconv.Atoi(s)
				if err != nil {
					t.Fatalf("unexpected helper output %q", frame.Data)
				}
				pids = append(pids, pid)
			}
		}
	}
	if pids[0] == pids[1] {
		t.Fatal("nested invocation reused the verifier")
	}
	for _, pid := range pids {
		if pgid, err := syscall.Getpgid(pid); err != nil || pgid != pid {
			t.Fatalf("delegated process lacks its separate group: pid=%d pgid=%d err=%v", pid, pgid, err)
		}
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	for _, pid := range pids {
		if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
			t.Fatalf("pid %d survived acknowledged cleanup: %v", pid, err)
		}
	}
	if parent.Err() != nil {
		t.Fatal("stopping the invocation canceled supervisor verification")
	}
	if _, err := owner.get(t.Context()); err == nil {
		t.Fatal("closed invocation accepted another launch")
	}
}

func TestInvocationNestedHelper(t *testing.T) {
	mode := os.Getenv("PYXIS_INVOCATION_HELPER")
	if mode == "" {
		return
	}
	if _, err := fmt.Fprintln(os.Stdout, os.Getpid()); err != nil {
		os.Exit(90)
	}
	if mode == "leaf" {
		for {
			time.Sleep(time.Hour)
		}
	}
	ref, inherited, err := currentLaunchCapability()
	if err != nil || !inherited {
		os.Exit(91)
	}
	request := launchRequest{Schema: launchSchema, Path: os.Args[0], Args: []string{os.Args[0], "-test.run=^TestInvocationNestedHelper$"}, Env: append(os.Environ(), "PYXIS_INVOCATION_HELPER=leaf"), Paths: launchPaths{State: ref.State}}
	// The helper's cwd is the delegated work root, irrespective of ambient PWD.
	request.Paths.Work, err = os.Getwd()
	if err != nil {
		os.Exit(92)
	}
	data, err := json.Marshal(request)
	if err != nil {
		os.Exit(93)
	}
	_, err = runLaunchProxy(context.Background(), data, io.NopCloser(strings.NewReader("")), os.Stdout, os.Stderr)
	if err != nil {
		os.Exit(94)
	}
	os.Exit(95)
}
