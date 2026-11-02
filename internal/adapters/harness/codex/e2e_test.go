package codex

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// TestEndToEnd_LiveCodex drives a fixture task through the real `codex exec` CLI
// to prove the adapter's actual invocation and JSONL parsing against the live
// binary. It is gated: it runs only when PYXIS_E2E_CODEX is set AND the CLI is
// installed AND authentication preflights clean. Absent any of those it skips
// cleanly — it never fails the suite when codex, credentials, or network are
// missing (the hermetic tests above are the real coverage; this is the
// occasional live re-verification of the CLI contract, §3.7).
func TestEndToEnd_LiveCodex(t *testing.T) {
	if os.Getenv("PYXIS_E2E_CODEX") == "" {
		t.Skip("set PYXIS_E2E_CODEX=1 to run the live codex conformance (needs the codex CLI, auth, and network)")
	}
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("codex CLI not on PATH: %v", err)
	}

	a, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := a.AuthCheck(ctx); err != nil {
		t.Skipf("codex backend not ready: %v", err)
	}

	h, err := a.Launch(ctx, port.LaunchSpec{
		Brief:   "Create a file named DONE.txt whose contents are exactly `ok`, then stop.",
		Workdir: t.TempDir(),
		Model:   "gpt-5.6-luna",
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
	// contract round-trips with parsed telemetry from the real CLI JSON stream.
	if res.Turns <= 0 {
		t.Errorf("expected a positive turn count from a live run, got %+v", res)
	}
	t.Logf("live codex run reported %+v", res)
}
