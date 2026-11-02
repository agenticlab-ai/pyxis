//go:build unix

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// A FIFO pauses initialization after diagnostic creation but before installer
// admission, making the early signal window deterministic without timing the
// tiny interval between .pyxis and logs directory creation.
func TestSignalBeforeInstallerAdmissionIsHandled(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo := t.TempDir()
	env := pyxisEnv(t, repo)
	fifo := filepath.Join(t.TempDir(), "integrations.yaml")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(builtBins["pyxis"], append(initArgs(r), "--integrations", fifo)...)
	cmd.Dir, cmd.Env = repo, env
	var stdout bytes.Buffer
	stderr := &signalNotice{received: make(chan struct{})}
	cmd.Stdout, cmd.Stderr = &stdout, stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		// A failed assertion must not strand a process blocked on the FIFO.
		_ = cmd.Process.Kill()
	}()
	deadline := time.After(5 * time.Second)
	var pipe *os.File
	for {
		// Opening only the writer succeeds once the child's ReadFile has
		// opened the reader. An O_RDWR parent would not establish that
		// handshake and could close before the child starts reading.
		var err error
		pipe, err = os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.ENXIO) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			t.Fatalf("init ended before configuration read: %v: %s", err, stderr.String())
		case <-deadline:
			t.Fatal("init never reached configuration read")
		case <-time.After(time.Millisecond):
		}
	}
	defer func() { _ = pipe.Close() }() // May already be closed to deliver EOF.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stderr.received:
	case err := <-done:
		t.Fatalf("early SIGTERM bypassed graceful interruption: %v: %s", err, stderr.String())
	case <-deadline:
		t.Fatal("early SIGTERM was not acknowledged")
	}
	if _, err := pipe.WriteString("integrations:\n  schema_version: pyxis-integrations/v1\n"); err != nil {
		t.Fatal(err)
	}
	if err := pipe.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(stderr.String(), "pyxis: init: interrupted") {
			t.Fatalf("early signal did not end as interrupted: %v: %s", err, stderr.String())
		}
	case <-deadline:
		t.Fatal("interrupted init did not end after configuration read")
	}
	assertEmptyDir(t, repo, "early signal")
	if result := pyxis(t, repo, env, initArgs(r)...); result.code != 0 {
		t.Fatalf("retry after early signal failed: %d: %s", result.code, result.stderr)
	}
}

// The process copier owns all writes. String is read only after Wait completes;
// the channel lets the test release the FIFO after the real signal handler ran.
type signalNotice struct {
	buffer   bytes.Buffer
	once     sync.Once
	received chan struct{}
}

func (w *signalNotice) String() string { return w.buffer.String() }

func (w *signalNotice) Write(p []byte) (int, error) {
	n, err := w.buffer.Write(p)
	if bytes.Contains(p, []byte("interrupt received")) {
		w.once.Do(func() { close(w.received) })
	}
	return n, err
}
