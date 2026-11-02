package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capture runs run() with piped stdout and returns (exit, stdout).
func capture(t *testing.T, args []string) (int, string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	code := run(args, outW, devnull)
	outW.Close()
	var sb strings.Builder
	buf := make([]byte, 1<<16)
	for {
		n, err := outR.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	return code, sb.String()
}

func TestUsageExits2(t *testing.T) {
	if code, _ := capture(t, []string{"--bogus"}); code != 2 {
		t.Fatalf("unknown flag exit = %d, want 2", code)
	}
	if code, _ := capture(t, []string{"--repo"}); code != 2 {
		t.Fatalf("missing value exit = %d, want 2", code)
	}
	if code, _ := capture(t, []string{"--gh-repo"}); code != 2 {
		t.Fatalf("missing gh repo exit = %d, want 2", code)
	}
	for _, flag := range []string{"-h", "--help"} {
		if code, _ := capture(t, []string{flag}); code != 2 {
			t.Fatalf("%s exit = %d, want 2", flag, code)
		}
	}
}

// TestIdleTickOnBareRepo pins the zero-cost idle contract (DINV-4/AP-19): a
// repo with no problems performs zero probes, invokes nothing, and reports an
// idle dry run.
func TestIdleTickOnBareRepo(t *testing.T) {
	root := t.TempDir()
	code, out := capture(t, []string{"--repo", root})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s", code, out)
	}
	var rep struct {
		Idle    bool `json:"Idle"`
		Probes  int  `json:"Probes"`
		DryRun  bool `json:"dry_run"`
		Invoked []string
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rep); err != nil {
		t.Fatalf("report parse: %v from %q", err, out)
	}
	if !rep.Idle || rep.Probes != 0 || !rep.DryRun || len(rep.Invoked) != 0 {
		t.Fatalf("want idle zero-probe dry report, got %+v", rep)
	}
}

// TestClosedLedgerIsSkippedWithoutProbes pins DINV-3 at the binary level: a
// delivered (closed) ledger draws zero probes and the tick stays idle.
func TestClosedLedgerIsSkippedWithoutProbes(t *testing.T) {
	root := t.TempDir()
	ledger := `# closed — delivery ledger

| Field | Value |
|---|---|
| **Slug** | closed |
| **Template version** | delivery-ledger/v1 |

## Derived state

| Field | Value |
|---|---|
| **Status** | closed |
| **Open items** | — |

## Journal

### E-1 — delivered — done

- **Date**: 2026-07-01
- **Principal**: Argus
- **External effect**: —
- **Body**: closed.
`
	p := filepath.Join(root, "docs", "problems", "closed")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "delivery.md"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := capture(t, []string{"--repo", root})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out=%s", code, out)
	}
	var rep struct {
		Probes int `json:"Probes"`
		Skips  []struct {
			Slug   string
			Reason string
		}
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rep); err != nil {
		t.Fatalf("report parse: %v from %q", err, out)
	}
	if rep.Probes != 0 {
		t.Fatalf("closed ledger drew %d probes, want 0", rep.Probes)
	}
	if len(rep.Skips) != 1 || rep.Skips[0].Reason != "closed" {
		t.Fatalf("want one closed skip, got %+v", rep.Skips)
	}
}

func TestExplicitDoorChangesPostureWithoutInventingWork(t *testing.T) {
	root := t.TempDir()
	code, out := capture(t, []string{"--repo", root, "--gh-repo", "owner/repo", "--", "front-door", "resume"})
	if code != 0 {
		t.Fatalf("exit = %d; out=%s", code, out)
	}
	var rep struct {
		Idle   bool `json:"Idle"`
		DryRun bool `json:"dry_run"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rep); err != nil {
		t.Fatal(err)
	}
	if !rep.Idle || rep.DryRun {
		t.Fatalf("explicit-door idle report = %+v", rep)
	}
}

func TestUnreadableLedgerRootIsAHardFault(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "docs", "problems")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _ := capture(t, []string{"--repo", root, "--dry-run"}); code != 1 {
		t.Fatalf("unreadable ledger root exit = %d, want 1", code)
	}
}
