//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestKernelProcessIdentity(t *testing.T) {
	self, err := readProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if self.pid != os.Getpid() || self.parent != os.Getppid() || self.born == 0 || self.exited || self.stopped {
		t.Fatalf("wrong live identity: %+v", self)
	}
	all, err := listProcesses(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !self.same(all[self.pid]) {
		t.Fatalf("snapshot lost child identity: %+v", all[self.pid])
	}
	if _, err := readProcess(1 << 30); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("absent PID: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := listProcesses(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled inspection: %v", err)
	}
}
