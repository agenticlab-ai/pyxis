package claude

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// TestEndToEnd_LiveClaude drives a fixture task through the real `claude` CLI to
// prove the adapter's actual invocation and JSON parsing against the live
// binary. It is gated: it runs only when PYXIS_E2E_CLAUDE is set AND the CLI is
// installed AND authentication preflights clean. Absent any of those it skips
// cleanly — it never fails the suite when claude, credentials, or network are
// missing (the hermetic tests above are the real coverage; this is the
// occasional live re-verification of the CLI contract, §3.7).
func TestEndToEnd_LiveClaude(t *testing.T) {
	if os.Getenv("PYXIS_E2E_CLAUDE") == "" {
		t.Skip("set PYXIS_E2E_CLAUDE=1 to run the live claude conformance (needs the claude CLI, auth, and network)")
	}
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("claude CLI not on PATH: %v", err)
	}

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := a.AuthCheck(ctx); err != nil {
		t.Skipf("claude backend not ready: %v", err)
	}

	h, err := a.Launch(ctx, port.LaunchSpec{
		Brief:   "Create a file named DONE.txt whose contents are exactly `ok`, then stop.",
		Workdir: t.TempDir(),
		Model:   "claude-haiku-4-5",
		Effort:  "low",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	res, err := a.Await(ctx, h)
	if err != nil {
		t.Fatalf("Await: %v", err)
	}
	// The artifact check is the Worker's job (DINV-3); here we only confirm the
	// contract round-trips with parsed telemetry from the real CLI JSON.
	if res.Turns <= 0 {
		t.Errorf("expected a positive turn count from a live run, got %+v", res)
	}
	t.Logf("live claude run reported %+v", res)
}
