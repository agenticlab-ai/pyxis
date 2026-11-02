package e2e

// Leg 1 — the offline combo (carina + local CI), network-denied (A-5/FR-13;
// design §3.3/§3.11; DINV-7). It proves the zero-egress invariant two ways:
//
//   - The EGRESS SCAN is in-process: converge/doctor/resolve driven directly
//     with a denyFetcher that FAILS on any http(s) scheme and COUNTS attempts.
//     Every enumerated FR-13 behavior — converge (from a prewarmed cache),
//     doctor, pinned-binary resolution — completes with the deny-fetcher
//     serving only file:// and warm-cache hits, and the http attempt counter
//     is ZERO afterward. The counter IS the scan (the charter's contract).
//   - The PACKAGED EXEC PATH is a subprocess: `pyxis exec pyxis -- version`
//     resolves → re-verifies (per-exec re-hash) → execve's a REAL cached binary
//     and runs it, against the prewarmed file:// cache, with HTTP(S)_PROXY
//     pointed at a dead address — an inert tripwire that turns any stray http
//     into a loud failure. `version` is used (not `docgate gate`) so the proof
//     rests on the installer's own acquisition+execve contract and does NOT
//     depend on the tool's own repo-root discovery: the operand `docgate gate`
//     hook/CI path is broken by a separately-reported docgate defect
//     (TestOperandDocgateGateNeedsGoMod) this leg must not mask.
//
// The whole leg runs after a promptless carina+local install (init itself is
// part of FR-13's "with zero network after the artifact fetch" set — the
// install already ran offline-safe from the file:// source and pre-warmed the
// cache).

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestOfflineComboNetworkDenied(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r) // a converged carina+local scratch repo, cache warm

	// ---- the in-process egress scan --------------------------------------
	//
	// Build the exact resolve.Config a subprocess would, but inject the
	// counting deny-fetcher. The cache is already warm (init pre-warmed it),
	// so every acquisition step is a warm-cache hit or a file:// read; a http
	// attempt would both fail the step AND increment the counter.
	cfg := loadConfig(t, repo)
	deny := &denyFetcher{}
	rcfg := resolve.Config{
		Root:    repo,
		Repo:    cfg,
		Fetcher: deny,
		Logger:  slog.New(slog.NewTextHandler(discardWriter{}, nil)),
	}

	// converge (idempotent no-op on the converged tree) — reconcile + pre-warm
	// + reap all run through the deny-fetcher.
	crep, err := converge.Run(context.Background(), rcfg, converge.Options{})
	if err != nil {
		t.Fatalf("offline converge (deny-fetcher): %v", err)
	}
	if crep.Mode != converge.ModeNoop {
		t.Errorf("offline converge mode = %q, want noop (the tree is already converged)", crep.Mode)
	}

	// doctor — the read-only health face, network-free by contract.
	h, err := converge.Doctor(context.Background(), rcfg)
	if err != nil {
		t.Fatalf("offline doctor (deny-fetcher): %v", err)
	}
	if !h.Healthy {
		t.Errorf("offline doctor UNHEALTHY on a clean converged install: %+v", h.Findings)
	}

	// pinned-binary resolution — resolve docgate to a verified cache path
	// (re-hashed on this call against the manifest), offline.
	path, err := resolve.Tool(context.Background(), rcfg, "docgate", "")
	if err != nil {
		t.Fatalf("offline resolve docgate (deny-fetcher): %v", err)
	}
	if _, serr := os.Stat(path); serr != nil {
		t.Fatalf("resolved docgate path does not exist: %v", serr)
	}

	// THE egress scan assertion: zero http(s) attempts through the one
	// acquisition seam across the whole offline sequence (DINV-7).
	if deny.HTTPAttempts != 0 {
		t.Fatalf("egress scan: %d http(s) fetch attempt(s) during the offline sequence — the zero-egress invariant is broken (A-5/DINV-7)", deny.HTTPAttempts)
	}

	// ---- the packaged exec path: subprocess execve, network-tripwired ----
	//
	// A dead HTTP(S)_PROXY makes any stray http from the child a loud failure;
	// the file:// source needs no network, so it is inert on the happy path.
	offlineEnv := append([]string{
		"HTTP_PROXY=http://127.0.0.1:1",
		"HTTPS_PROXY=http://127.0.0.1:1",
		"NO_PROXY=",
	}, env...)

	// `pyxis exec pyxis -- version`: resolve → re-verify (per-exec re-hash
	// against the pinned manifest) → execve the cached pyxis binary → run
	// `version`, exit 0 — the shipped exec grammar exercised on a REAL cached
	// binary, offline. `version` needs no repo root, so this proves exactly
	// the installer's acquisition+execve contract (DINV-1) without depending on
	// the tool's own root discovery (see TestOperandDocgateGateNeedsGoMod for
	// the docgate-gate path, a separately-reported defect it must NOT mask).
	res := pyxis(t, repo, offlineEnv, "exec", "pyxis", "--", "version")
	if res.code != 0 {
		t.Fatalf("offline `pyxis exec pyxis -- version` = %d, want 0 (the packaged execve path, offline)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "pyxis ") {
		t.Fatalf("offline exec did not run the cached pyxis binary (no version output):\n%s", res.stdout)
	}

	t.Logf("offline combo: init→converge→doctor→resolve→exec(pyxis) all offline; %d http attempts", deny.HTTPAttempts)
}
