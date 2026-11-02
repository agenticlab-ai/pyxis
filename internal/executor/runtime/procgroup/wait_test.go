package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

type waitOwner struct {
	exit func(context.Context) error
	stop func() error
}

func (s waitOwner) WaitExit(ctx context.Context) error { return s.exit(ctx) }
func (s waitOwner) Kill() error                        { return s.stop() }

type legacyWaitOwner struct{ stop func() error }

func (s legacyWaitOwner) Kill() error { return s.stop() }

func TestWaitOwnedHelper(t *testing.T) {
	switch os.Args[len(os.Args)-1] {
	case "wait-exit-zero":
		os.Exit(0)
	case "wait-exit-nonzero":
		os.Exit(23)
	case "wait-stay-live":
		for {
			time.Sleep(time.Hour)
		}
	}
}

func ownedWaitFixture(t *testing.T, mode string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestWaitOwnedHelper$", "--", mode)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill() // terminate only this direct fixture process
			_ = cmd.Wait()         // reap the terminated fixture
		}
	})
	return cmd
}

func TestWaitOwnedStopsBeforeReapAndPreservesBothErrors(t *testing.T) {
	exitErr, stopErr := errors.New("exit inspection failed"), errors.New("stop denied")
	for _, tc := range []struct {
		name, mode string
		exit, stop error
		unreaped   bool
	}{
		{"normal exit", "wait-exit-zero", nil, nil, false},
		{"nonzero exit", "wait-exit-nonzero", nil, nil, false},
		{"cleanup failure after confirmed exit", "wait-exit-zero", nil, stopErr, false},
		{"exit inspection failed but cleanup confirmed", "wait-stay-live", exitErr, nil, false},
		{"neither exit nor stop confirmed", "wait-stay-live", exitErr, stopErr, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := ownedWaitFixture(t, tc.mode)
			stopped := false
			owner := waitOwner{exit: func(context.Context) error { return tc.exit }, stop: func() error {
				if cmd.ProcessState != nil {
					t.Fatal("root reaped before cleanup acquired session members")
				}
				stopped = true
				if tc.exit != nil && tc.stop == nil {
					return cmd.Process.Kill()
				}
				return tc.stop
			}}
			waitErr, killErr := WaitOwned(t.Context(), cmd, owner)
			if !stopped || !errors.Is(killErr, tc.stop) || (cmd.ProcessState == nil) != tc.unreaped {
				t.Fatalf("wait=%v stop=%v state=%v", waitErr, killErr, cmd.ProcessState)
			}
			if tc.exit != nil && !errors.Is(waitErr, tc.exit) {
				t.Fatalf("lost exit inspection error: %v", waitErr)
			}
			if tc.mode == "wait-exit-nonzero" {
				var status *exec.ExitError
				if !errors.As(waitErr, &status) || status.ExitCode() != 23 {
					t.Fatalf("lost command exit status: %v", waitErr)
				}
			} else if tc.exit == nil && waitErr != nil {
				t.Fatalf("normal command failed: %v", waitErr)
			}
		})
	}
}

func TestWaitOwnedLegacyAndNilOwner(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		cmd := ownedWaitFixture(t, "wait-exit-zero")
		var owner interface{ Kill() error }
		stopped := false
		if legacy {
			owner = legacyWaitOwner{stop: func() error {
				stopped = true
				if cmd.ProcessState == nil {
					t.Fatal("legacy owner changed ordinary waiting")
				}
				return nil
			}}
		}
		waitErr, stopErr := WaitOwned(t.Context(), cmd, owner)
		if waitErr != nil || stopErr != nil || stopped != legacy {
			t.Fatalf("legacy=%v wait=%v stop=%v stopped=%v", legacy, waitErr, stopErr, stopped)
		}
	}
}
