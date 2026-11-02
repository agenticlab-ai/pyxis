package preflight_test

// The toolchain fixture matrix (design init-hardening-v1 §3.1 check 2;
// DINV-1; prd:init-hardening FR-3; plan init-hardening/001 T2's acceptance
// criteria): a present, at-or-above-floor toolchain passes; an absent or
// below-floor executable refuses pre-write naming that tool's remedy; every
// miss is enumerated in one pass (AP-16), not just the first. main_test.go's
// TestMain seeds a Git-only PATH for the rest of the package's tests; this file
// overrides PATH locally per case, exactly the shape main_test.go's own
// comment promises.

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

// Version strings comfortably at or above toolchain_floors.json's floor set
// (git 2.28.0) — the shape each tool fixture's
// `--version` output takes.
const goodGitOutput = "git version 2.43.0 (Apple Git-146)"

// toolchainOutcome runs the registry and returns just the toolchain outcome
// (the admission and artifact-source checks are along for the ride, exactly
// as admission_test.go's admission() helper filters for its own check).
func toolchainOutcome(t *testing.T, target string) (preflight.Outcome, error) {
	t.Helper()
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target})
	for _, out := range rep {
		if out.Check == preflight.ToolchainID {
			return out, err
		}
	}
	t.Fatalf("Report carries no toolchain outcome: %+v (err=%v)", rep, err)
	return preflight.Outcome{}, err
}

// buildToolchainPATH writes a scripted `--version` stand-in (main_test.go's
// writeFakeVersionTool) for every (tool, output) pair into a fresh dir and
// returns it — a tool omitted from overrides is absent from PATH entirely,
// so exec.LookPath misses it exactly as it would on a machine without that
// tool installed.
func buildToolchainPATH(t *testing.T, overrides map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, output := range overrides {
		writeFakeVersionTool(dir, name, output)
	}
	return dir
}

func TestToolchainPassWithoutNodeOrNpm(t *testing.T) {
	dir := buildToolchainPATH(t, map[string]string{"git": goodGitOutput})
	t.Setenv("PATH", dir)

	out, err := toolchainOutcome(t, t.TempDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Status != preflight.StatusPass {
		t.Fatalf("outcome = %+v, want pass", out)
	}
}

func TestToolchainAbsentOrBelowFloorRefuses(t *testing.T) {
	cases := []struct {
		name       string
		overrides  map[string]string
		wantSubstr []string // each must appear somewhere in Conflicts+Remedy
	}{
		{
			name:       "git absent",
			overrides:  nil,
			wantSubstr: []string{"git: not found on PATH", "install git"},
		},
		{
			name: "git below floor",
			overrides: map[string]string{
				"git": "git version 2.20.0 (Apple Git-1)",
			},
			wantSubstr: []string{"git: 2.20.0 is below the required 2.28.0", "upgrade git"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := buildToolchainPATH(t, tc.overrides)
			t.Setenv("PATH", dir)

			out, err := toolchainOutcome(t, t.TempDir())
			if err == nil {
				t.Fatal("Run = nil error, want a toolchain refusal")
			}
			if out.Status != preflight.StatusRefuse {
				t.Fatalf("outcome = %+v, want refuse", out)
			}
			blob := out.Reason + " | " + strings.Join(out.Conflicts, "; ") + " | " + out.Remedy
			for _, want := range tc.wantSubstr {
				if !strings.Contains(blob, want) {
					t.Errorf("toolchain refusal %q does not contain %q", blob, want)
				}
			}
		})
	}
}
