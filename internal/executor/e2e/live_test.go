package e2e

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLiveDogfood_Gated is the gated LIVE dogfood drill — acceptance A-12 proper.
// It SKIPS CLEANLY in the normal run and executes only behind PYXIS_E2E_LIVE=1
// with a real repo/plan and an installed, authenticated coding-agent CLI. When
// the gate is on but a precondition is missing it SKIPS with a precise remedy
// rather than failing — so the path exists and is documented without ever turning
// the default `go test` red.
//
// Why it is gated, not hermetic: a true dogfood needs the real external media the
// hermetic drill deliberately fakes — a live coding-agent CLI spawning fresh
// headless processes that actually mutate code, a live code host + CI, and a real
// frozen plan. Those cannot run in-session; the hermetic drill above proves the
// engine COMPOSES, and this drill is where the live media are exercised end to end
// (docs/tech-design/implement-now.md §3.10, §7; A-12).
//
// What a real run does: it drives the /implement-now surface — the built
// cmd/implement-now binary runs ONE frontier-sweep of the real frozen plan's ready
// tasks to genuinely-green PRs, with the operator only reviewing and merging; the
// executor NEVER merges (DINV-6). Exit 0 (all green / nothing ready) or 1 (a task
// failed/escalated — reported) both mean a completed sweep; exit 2 is a
// wiring/usage error the drill catches.
//
// To run it:
//
//	PYXIS_E2E_LIVE=1 \
//	PYXIS_E2E_REPO=/abs/path/to/checkout \
//	PYXIS_E2E_REPO_SLUG=owner/name \
//	[PYXIS_E2E_ARGS="--unattended --base main"] \
//	go test -run TestLiveDogfood_Gated -count=1 ./internal/executor/e2e/...
//
// with git + gh on PATH, a backend CLI (claude/codex/cursor-agent/opencode)
// installed and authenticated, and the repo's tracker carrying at least one open
// automated-implementation item.
func TestLiveDogfood_Gated(t *testing.T) {
	if os.Getenv("PYXIS_E2E_LIVE") != "1" {
		t.Skip("live dogfood is gated: set PYXIS_E2E_LIVE=1 (plus PYXIS_E2E_REPO, PYXIS_E2E_REPO_SLUG and an installed+authed backend CLI) to drive the real /implement-now dogfood; the hermetic drill is the default in-session proxy (A-12)")
	}

	repo := strings.TrimSpace(os.Getenv("PYXIS_E2E_REPO"))
	slug := strings.TrimSpace(os.Getenv("PYXIS_E2E_REPO_SLUG"))
	if repo == "" || slug == "" {
		t.Skip("live dogfood needs PYXIS_E2E_REPO (a real checkout path) and PYXIS_E2E_REPO_SLUG (owner/name)")
	}
	if _, err := os.Stat(repo); err != nil {
		t.Skipf("PYXIS_E2E_REPO %q is not accessible: %v", repo, err)
	}
	for _, bin := range []string{"go", "git", "gh"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("live dogfood needs %q on PATH: %v", bin, err)
		}
	}
	// At least one coding-agent CLI must be installed for a real model call.
	if !anyOnPath("claude", "codex", "cursor-agent", "opencode") {
		t.Skip("live dogfood needs a coding-agent CLI (claude/codex/cursor-agent/opencode) installed and authenticated")
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		t.Fatalf("locate module root for the /implement-now build: %v", err)
	}

	// Drive the real /implement-now surface against the real repo. This is the A-12
	// path: the executor opens genuinely-green PRs and reports the blocked set; the
	// operator reviews and merges.
	extra := strings.Fields(os.Getenv("PYXIS_E2E_ARGS"))
	args := append([]string{"run", "./cmd/implement-now", "--repo", repo, "--repo-slug", slug}, extra...)
	cmd := exec.Command("go", args...)
	cmd.Dir = moduleRoot
	out, runErr := cmd.CombinedOutput()
	t.Logf("/implement-now output:\n%s", out)

	if runErr == nil {
		t.Logf("live /implement-now dogfood completed cleanly (exit 0 — all green or nothing ready)")
		return
	}
	var exit *exec.ExitError
	if errors.As(runErr, &exit) && exit.ExitCode() == 1 {
		// A completed sweep that reported at least one failed/escalated task. That is
		// a real, reported result of the dogfood — not a harness failure.
		t.Logf("sweep completed with a failed/escalated task (exit 1) — reported, not a harness error")
		return
	}
	t.Fatalf("live /implement-now dogfood did not complete (exit != 0/1, a wiring/usage error): %v", runErr)
}

// anyOnPath reports whether any of the named binaries resolves on PATH.
func anyOnPath(names ...string) bool {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return true
		}
	}
	return false
}

// findModuleRoot walks up from the working directory to the directory holding
// go.mod — the root the `go run ./cmd/implement-now` invocation is issued from.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}
