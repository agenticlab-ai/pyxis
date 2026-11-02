package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

type OutputReference struct{ Harness, Stdout, Stderr string }
type outputObserverKey struct{}

// WithOutputObserver makes the actual private log paths visible to the owning
// supervisor without forwarding raw output, prompts, credentials or env.
func WithOutputObserver(ctx context.Context, observe func(OutputReference)) context.Context {
	return context.WithValue(ctx, outputObserverKey{}, observe)
}

// CaptureOutput tees child output to private files in the persistent owning
// checkout. It records only output, never the launch environment or auth state.
// The parent owns the files and closes them after Wait drains the child pipes.
// A sink failure warns once and preserves the child's existing output protocol.
func CaptureOutput(ctx context.Context, workspace, harness string, stdout, stderr io.Writer) (io.Writer, io.Writer, func()) {
	if workspace == "" {
		return stdout, stderr, func() {}
	}
	root := runlogs.Repository(ctx, workspace)
	dir, err := runlogs.Resolve(ctx, root, runlogs.Agents+"/"+harness, "")
	if err == nil {
		err = os.MkdirAll(dir, 0o700)
	}
	if err == nil {
		dir, err = os.MkdirTemp(dir, time.Now().UTC().Format("20060102T150405")+"-*")
	}
	var outFile, errFile *os.File
	if err == nil {
		outFile, err = os.OpenFile(filepath.Join(dir, "stdout.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err == nil {
		errFile, err = os.OpenFile(filepath.Join(dir, "stderr.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	}
	if err != nil {
		if outFile != nil {
			if closeErr := outFile.Close(); closeErr != nil {
				slog.Warn("close agent output", "error", closeErr)
			}
		}
		slog.Warn("cannot retain agent output", "harness", harness, "error", err)
		return stdout, stderr, func() {}
	}
	state := &outputCapture{}
	if observe, ok := ctx.Value(outputObserverKey{}).(func(OutputReference)); ok && observe != nil {
		observe(OutputReference{Harness: harness, Stdout: outFile.Name(), Stderr: errFile.Name()})
	}
	closeLogs := func() {
		for _, f := range []*os.File{outFile, errFile} {
			if err := f.Close(); err != nil {
				slog.Warn("close agent output", "path", f.Name(), "error", err)
			}
		}
	}
	return &outputStream{state, stdout, outFile}, &outputStream{state, stderr, errFile}, closeLogs
}

type outputCapture struct {
	mu     sync.Mutex
	warned bool
}

type outputStream struct {
	state       *outputCapture
	destination io.Writer
	file        *os.File
}

func (s *outputStream) Write(p []byte) (int, error) {
	// Codex combines stderr and stdout in one buffer. Sharing this lock also
	// preserves that adapter's serialization when os/exec drains two pipes.
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	n, err := s.destination.Write(p)
	if _, logErr := s.file.Write(p); logErr != nil && !s.state.warned {
		s.state.warned = true
		slog.Warn("cannot append agent output", "path", s.file.Name(), "error", fmt.Errorf("write execution log: %w", logErr))
	}
	return n, err
}
