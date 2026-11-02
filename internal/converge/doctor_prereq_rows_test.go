package converge_test

// Doctor reports the shared Git prerequisite and its best-effort version floor.
// The prerequisite report is limited to tools the installed system actually uses.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
)

// stubVersionPATH pins PATH to stubs that PRINT a version line — the floor
// leg's fixture (stubPATH's exit-0 stubs deliberately print nothing, which
// exercises the tolerant present-only path instead).
func stubVersionPATH(t *testing.T, tools map[string]string) {
	t.Helper()
	tools["bwrap"] = ""
	dir := t.TempDir()
	for name, line := range tools {
		body := "#!/bin/sh\necho \"" + line + "\"\n"
		if name == "git" {
			// mirror stubPATH's git quirk for the hooks probe only when the
			// caller wants a functioning-but-versioned git: `git config --get`
			// on an unset key must exit 1, everything else echoes the version.
			body = "#!/bin/sh\ncase \"$1\" in config) exit 1;; esac\necho \"" + line + "\"\n"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("write stub %s: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestDoctorDoesNotRequireNodeOrNpm(t *testing.T) {
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	_, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t}
	stubPATH(t, "git") // no package runtime required
	h := mustDoctor(t, ctx, cfg)
	if rows := doctorRows(h, converge.CheckPrerequisite); len(rows) != 0 {
		t.Errorf("prerequisite rows = %+v, want none with Git present", rows)
	}
}

func TestDoctorBelowFloorRow(t *testing.T) {
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	_, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t}
	stubVersionPATH(t, map[string]string{
		"git": "git version 0.0.1", // below any recorded floor
	})
	h := mustDoctor(t, ctx, cfg)
	wantRow(t, h, converge.CheckPrerequisite, "git", converge.SeverityFinding, "below the release's documented floor", "0.0.1", "FR-3")
	if h.Healthy {
		t.Error("Healthy = true with git below floor")
	}
}
