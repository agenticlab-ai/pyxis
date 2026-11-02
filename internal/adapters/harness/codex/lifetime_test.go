package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime/procgroup"
)

type failedLifetime struct {
	*procgroup.Lifetime
	observeErr, stopErr, waitErr error
}

func (s failedLifetime) ObserveStart() error {
	if s.observeErr != nil {
		return s.observeErr
	}
	return s.Lifetime.ObserveStart()
}
func (s failedLifetime) Kill() error {
	return errors.Join(s.Lifetime.Kill(), s.stopErr)
}

func (s failedLifetime) WaitExit(ctx context.Context) error {
	return errors.Join(s.Lifetime.WaitExit(ctx), s.waitErr)
}

func TestOSOwnershipErrorsCannotBecomeSuccessfulExit(t *testing.T) {
	for _, tc := range []struct {
		name                string
		observeErr, stopErr error
	}{
		{"identity failure", syscall.EIO, nil},
		{"identity and cleanup failure", syscall.EIO, syscall.EPERM},
		{"cleanup failure after normal exit", nil, syscall.EPERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cmd *exec.Cmd
			c := osCommander{lifetime: func(child *exec.Cmd) processLifetime {
				cmd = child
				return failedLifetime{Lifetime: procgroup.NewLifetime(child), observeErr: tc.observeErr, stopErr: tc.stopErr}
			}}
			p, err := c.start(t.Context(), commandSpec{Name: "go", Args: []string{"version"}})
			if tc.observeErr != nil {
				if !errors.Is(err, port.ErrNotQuiescent) || !errors.Is(err, tc.observeErr) {
					t.Fatalf("failed ownership launched successfully: %v", err)
				}
				if tc.stopErr != nil {
					if !errors.Is(err, tc.stopErr) {
						t.Fatalf("lost cleanup error: %v", err)
					}
					_ = cmd.Wait() // the real lifetime stopped the child; reap the fixture
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, code, err := p.wait()
			if code != -1 || !errors.Is(err, port.ErrNotQuiescent) || !errors.Is(err, tc.stopErr) {
				t.Fatalf("normal exit hid failed cleanup: code=%d err=%v", code, err)
			}
			if err := p.kill(); !errors.Is(err, tc.stopErr) {
				t.Fatalf("kill hid failed ownership: %v", err)
			}
		})
	}
	t.Run("delegated cleanup failure", func(t *testing.T) {
		p, err := osCommander{}.start(context.Background(), commandSpec{Name: "go", Args: []string{"version"}})
		if err != nil {
			t.Fatal(err)
		}
		native := p.(*osProcess)
		stop := native.stopInvocation
		native.stopInvocation = func() error { return errors.Join(stop(), syscall.EPERM) }
		_, code, err := p.wait()
		if code != -1 || !errors.Is(err, port.ErrNotQuiescent) {
			t.Fatalf("delegate cleanup error became successful exit: %d %v", code, err)
		}
	})
}

func TestOSProcessExitInspectionFailureSurvivesNormalStatus(t *testing.T) {
	for _, nonzero := range []bool{false, true} {
		c := osCommander{lifetime: func(cmd *exec.Cmd) processLifetime {
			return failedLifetime{Lifetime: procgroup.NewLifetime(cmd), waitErr: syscall.EIO}
		}}
		p, err := c.start(t.Context(), commandSpec{Name: os.Args[0], Args: []string{"-test.run=^TestExitInspectionHelper$", "--", fmt.Sprint(nonzero)}})
		if err != nil {
			t.Fatal(err)
		}
		out, code, err := p.wait()
		if code != -1 || !errors.Is(err, syscall.EIO) || len(out) == 0 || errors.Is(err, port.ErrNotQuiescent) {
			t.Fatalf("nonzero=%v out=%q code=%d err=%v", nonzero, out, code, err)
		}
	}
}

func TestExitInspectionHelper(t *testing.T) {
	switch os.Args[len(os.Args)-1] {
	case "true":
		fmt.Println("exit inspection fixture")
		os.Exit(17)
	case "false":
		fmt.Println("exit inspection fixture")
		os.Exit(0)
	}
}
