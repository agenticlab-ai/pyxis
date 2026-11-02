package ghcodehost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxFailedLogBytes = 8 << 20

var errGHLogLimit = errors.New("GitHub failed logs exceed the bounded diagnostic buffer")

func shellGHLog(ctx context.Context, limit int, args ...string) ([]byte, error) {
	return boundedLogCommand(ctx, limit, "gh", args...)
}

// Bound while reading the command's pipes, before Output can allocate the full
// remote log. Overflow cancels the owned command and returns no partial log.
func boundedLogCommand(ctx context.Context, limit int, name string, args ...string) ([]byte, error) {
	return boundedLogCommandEnv(ctx, limit, nil, name, args...)
}

func boundedLogCommandEnv(ctx context.Context, limit int, env []string, name string, args ...string) ([]byte, error) {
	if limit <= 0 {
		return nil, errGHLogLimit
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Env = env
	cmd.WaitDelay = 2 * time.Second
	stdout := &boundedLogBuffer{remaining: limit, cancel: cancel}
	stderr := &boundedLogBuffer{remaining: 64 << 10, cancel: cancel}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if stdout.exceeded || stderr.exceeded {
		return nil, errGHLogLimit
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("GitHub failed-log command: %w", ctx.Err())
	}
	if err != nil {
		message := strings.TrimSpace(stderr.buffer.String())
		if env == nil {
			env = os.Environ()
		}
		for _, entry := range env {
			key, value, _ := strings.Cut(entry, "=")
			if (key == "GH_TOKEN" || key == "GITHUB_TOKEN") && value != "" {
				message = strings.ReplaceAll(message, value, "[redacted]")
			}
		}
		return nil, fmt.Errorf("GitHub failed-log command: %w: %s", err, message)
	}
	return stdout.buffer.Bytes(), nil
}

type boundedLogBuffer struct {
	buffer    bytes.Buffer
	remaining int
	exceeded  bool
	cancel    context.CancelFunc
}

func (b *boundedLogBuffer) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		b.exceeded = true
		b.cancel()
		return 0, errGHLogLimit
	}
	n, err := b.buffer.Write(p)
	b.remaining -= n
	return n, err
}
