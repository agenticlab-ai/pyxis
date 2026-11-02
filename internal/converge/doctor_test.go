package converge_test

// Hermetic tests for the T6 doctor face (installer-v1 §3.5/§3.8;
// prd:installer FR-14, UB-7, NFR-4): the same file:// fixture release the
// T5 suite converges, tampered per ownership class; PATH-stubbed
// prerequisite legs; real-git hooks legs; tracker/surface/epoch/
// deprecation/cache/skeleton fixtures; the source-mode branch AROUND
// converge's unweakened ErrSourceMode refusal; and the zero-mutation
// snapshot proof (T5's snap/sameSnap helpers, reused in-package).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
)

// ---- doctor fixture scaffolding ---------------------------------------------

// isolateGit pins every non-local git config scope to empty, so the host's
// global/system config can never leak into the hooks probe.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// stubPATH pins PATH to a directory holding exactly the named stub
// executables — the removed-prerequisite fixture. The git stub mimics
// `git config --get` on an unset key (exit 1), so the hooks probe stays
// deterministic when only presence is under test.
func stubPATH(t *testing.T, names ...string) {
	t.Helper()
	// Model a usable boundary in prerequisite unit fixtures. Dedicated process
	// regressions cover missing and unusable hosts.
	names = append(names, "bwrap")
	dir := t.TempDir()
	for _, n := range names {
		body := "#!/bin/sh\nexit 0\n"
		if n == "git" {
			body = "#!/bin/sh\nexit 1\n"
		}
		writeFile(t, filepath.Join(dir, n), []byte(body), 0o755)
	}
	t.Setenv("PATH", dir)
}

// gitInitRepo turns the operand root into a real git repository (the hooks
// probe reads the repo's effective config), skipping when the host has no
// git, and isolating the config scopes.
func gitInitRepo(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	isolateGit(t)
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Skipf("git init: %v (%s)", err, out)
	}
}

func gitHooksPathSet(t *testing.T, root, val string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", root, "config", "core.hooksPath", val).CombinedOutput(); err != nil {
		t.Fatalf("git config core.hooksPath %s: %v (%s)", val, err, out)
	}
}

// cfgWith rewrites the operand's tree config to the release pins plus extra
// blocks and reparses it, keeping the repo-owned tree file and the parsed
// operand coherent.
func cfgWith(t *testing.T, r release, root string, cfg resolve.Config, extra string) resolve.Config {
	t.Helper()
	b := append(slices.Clone(r.operandConfig()), []byte(extra)...)
	writeFile(t, at(root, ".pyxis/config.yaml"), b, 0o644)
	repo, defects := config.Parse(b)
	if len(defects) > 0 {
		t.Fatalf("fixture config defects: %v", defects)
	}
	cfg.Repo = repo
	return cfg
}

// repoTemplate reads a REAL shipped template from this source repo — the
// bytes init instantiates verbatim (payload.SkeletonTargets), so detection
// is tested against exactly what instantiation leaves.
func repoTemplate(t *testing.T, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read shipped template %s: %v", rel, err)
	}
	return b
}

// wireSurface creates the claude-surface pointers: .pyxis/skills (scripts
// already exists in the fixture payload) plus the two vendor symlinks.
func wireClaude(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(at(root, ".pyxis/generated/harnesses/claude/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(at(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		".claude/skills":  "../.pyxis/generated/harnesses/claude/skills",
		".claude/scripts": "../.pyxis/scripts",
		"CLAUDE.md":       "AGENTS.md", // the manual pointer Claude Code reads
	} {
		if err := os.Symlink(target, at(root, link)); err != nil {
			t.Fatal(err)
		}
	}
}

func doctorRows(h converge.Health, cl converge.CheckClass) []converge.HealthFinding {
	var out []converge.HealthFinding
	for _, f := range h.Findings {
		if f.Class == cl {
			out = append(out, f)
		}
	}
	return out
}

// wantRow asserts one row of the class exists under the name with the
// severity, its detail carrying every fragment.
func wantRow(t *testing.T, h converge.Health, cl converge.CheckClass, name string, sev converge.Severity, frags ...string) {
	t.Helper()
	for _, f := range doctorRows(h, cl) {
		if f.Name != name {
			continue
		}
		if f.Severity != sev {
			t.Errorf("%s/%s severity = %s, want %s", cl, name, f.Severity, sev)
		}
		for _, frag := range frags {
			if !strings.Contains(f.Detail, frag) {
				t.Errorf("%s/%s detail %q lacks %q", cl, name, f.Detail, frag)
			}
		}
		return
	}
	t.Errorf("no %s row named %q in %+v", cl, name, h.Findings)
}

func wantNoRows(t *testing.T, h converge.Health, cl converge.CheckClass) {
	t.Helper()
	if rows := doctorRows(h, cl); len(rows) != 0 {
		t.Errorf("%s rows = %+v, want none", cl, rows)
	}
}

func mustDoctor(t *testing.T, ctx context.Context, cfg resolve.Config) converge.Health {
	t.Helper()
	h, err := converge.Doctor(ctx, cfg)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	return h
}

// ---- drift: each injected class detected and named (FR-14, A-4's classes) ----

func TestDoctorDriftFindingsPerClassNamed(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}

	// One injection per drift state, across all three ownership classes.
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered by a malicious PR\n"), 0o644) // verbatim modified
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {                                     // verbatim missing
		t.Fatal(err)
	}
	writeFile(t, at(root, "docs/tech-design/index.json"), []byte("{\"stale\":true}\n"), 0o644) // generated stale
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {                    // repo-owned missing
		t.Fatal(err)
	}
	writeFile(t, at(root, ".pyxis/config.yaml"), []byte("bogus:\n  x: y\n"), 0o644) // repo-owned invalid (tree file; cfg.Repo stays the good parse)

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)

	if h.Mode != "release" || !h.ManifestPinned || h.SystemVersion != r.ver {
		t.Errorf("summary = mode %q pinned %v system %q, want release/true/%s", h.Mode, h.ManifestPinned, h.SystemVersion, r.ver)
	}
	if h.Healthy {
		t.Error("Healthy = true with injected drift")
	}
	wantRow(t, h, converge.CheckDrift, ".pyxis/templates/prd-template.md", converge.SeverityFinding, "engine-verbatim modified", "manifest pins")
	wantRow(t, h, converge.CheckDrift, ".pyxis/scripts/hook.sh", converge.SeverityFinding, "engine-verbatim missing")
	wantRow(t, h, converge.CheckDrift, "docs/tech-design/index.json", converge.SeverityFinding, "generated stale")
	wantRow(t, h, converge.CheckDrift, ".pyxis/stamp/origin.json", converge.SeverityFinding, "repo-owned missing", "DINV-3")
	wantRow(t, h, converge.CheckDrift, ".pyxis/config.yaml", converge.SeverityFinding, "repo-owned invalid", "closed-schema parse failed")
	if rows := doctorRows(h, converge.CheckDrift); len(rows) != 5 {
		t.Errorf("drift rows = %d (%+v), want exactly the 5 injections", len(rows), rows)
	}
}

func TestDoctorDriftUngeneratableNamed(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	writeFile(t, at(root, "docs/tech-design/broken.md"), []byte("not a design\n"), 0o644)

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	for _, p := range []string{"docs/tech-design/index.md", "docs/tech-design/index.json"} {
		wantRow(t, h, converge.CheckDrift, p, converge.SeverityFinding, "generated ungeneratable", "generator refuses the design corpus")
	}
}

// ---- drift: manifest not cached → the named advisory, zero fetches -----------

func TestDoctorManifestUncachedAdvisoryNoFetch(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t} // a cold cache MUST NOT trigger a fetch

	before := snap(t, root)
	h := mustDoctor(t, context.Background(), cfg)
	sameSnap(t, before, snap(t, root), "uncached doctor")

	rows := doctorRows(h, converge.CheckDrift)
	if len(rows) != 1 {
		t.Fatalf("drift rows = %+v, want exactly the uncached advisory", rows)
	}
	wantRow(t, h, converge.CheckDrift, "release-manifest", converge.SeverityAdvisory,
		"drift unknown", "run `pyxis converge`", "no fetch")
	if h.ArtifactSource != "file://"+r.src {
		t.Errorf("ArtifactSource = %q, want the override surfaced (FR-22)", h.ArtifactSource)
	}
}

func TestDoctorReleaseModeMissingPinIsFinding(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	b := []byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v0.4.0\"\n  mode: release\n")
	writeFile(t, at(root, ".pyxis/config.yaml"), b, 0o644)
	repo, defects := config.Parse(b)
	if len(defects) > 0 {
		t.Fatalf("fixture config defects: %v", defects)
	}
	cfg.Repo = repo
	_ = r

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, context.Background(), cfg)
	if h.ManifestPinned {
		t.Error("ManifestPinned = true with no manifest_sha256 in the config")
	}
	wantRow(t, h, converge.CheckDrift, "release-manifest", converge.SeverityFinding, "system.manifest_sha256", "DINV-2")
	if h.Healthy {
		t.Error("Healthy = true with a release-mode config missing its pin")
	}
}

// ---- source mode: drift skipped by name, env still checked, Drift unweakened --

func TestDoctorSourceModeSkipsDriftEnvStillChecked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Repo.System.Mode = "source"
	cfg.Fetcher = denyFetcher{t}
	ctx := context.Background()

	before := snap(t, root)
	h := mustDoctor(t, ctx, cfg)
	sameSnap(t, before, snap(t, root), "source-mode doctor")

	if h.Mode != "source" {
		t.Errorf("Mode = %q, want source (T12: doctor reports source mode)", h.Mode)
	}
	rows := doctorRows(h, converge.CheckDrift)
	if len(rows) != 1 {
		t.Fatalf("drift rows = %+v, want exactly the skipped advisory", rows)
	}
	wantRow(t, h, converge.CheckDrift, "manifest-drift", converge.SeverityAdvisory,
		"skipped — source mode", "source tree is authoritative", "DINV-8")
	// Environment health still runs: the temp root is no git repo and the
	// isolated config sets no hooksPath, so the hooks check must have fired.
	wantRow(t, h, converge.CheckHooks, "core.hooksPath", converge.SeverityFinding, "unset", `".githooks"`)

	// The tension is resolved by branching, never by weakening: converge's
	// own read-only face still refuses source mode outright.
	if _, derr := converge.Drift(ctx, cfg); !errors.Is(derr, converge.ErrSourceMode) {
		t.Errorf("converge.Drift in source mode = %v, want ErrSourceMode (the refusal must stand)", derr)
	}
	if _, rerr := converge.Run(ctx, cfg, converge.Options{}); !errors.Is(rerr, converge.ErrSourceMode) {
		t.Errorf("converge.Run in source mode = %v, want ErrSourceMode", rerr)
	}
}

// ---- prerequisites: each removed binary named; gh only in GitHub mode --------

func TestDoctorPrerequisitesRemovedNamed(t *testing.T) {
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	t.Run("git absent", func(t *testing.T) {
		_, cfg := r.newOperand(t)
		cfg.Fetcher = denyFetcher{t}
		stubPATH(t)
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckPrerequisite, "git", converge.SeverityFinding, "not found on PATH", "install git", "UB-7")
		wantRow(t, h, converge.CheckHooks, "core.hooksPath", converge.SeverityAdvisory, "not probed", "git is unavailable")
		if rows := doctorRows(h, converge.CheckPrerequisite); len(rows) != 1 {
			t.Errorf("prerequisite rows = %+v, want git only", rows)
		}
		if h.Healthy {
			t.Error("Healthy = true with git missing")
		}
	})

	t.Run("gh absent in github tracker mode", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: github\n")
		cfg.Fetcher = denyFetcher{t}
		stubPATH(t, "git")
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckPrerequisite, "gh", converge.SeverityFinding, "GitHub mode", "cli.github.com")
	})

	t.Run("gh absent under active wiring", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: carina\n  wiring: { repo: use, protection: required-checks }\n")
		cfg.Fetcher = denyFetcher{t}
		stubPATH(t, "git")
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckPrerequisite, "gh", converge.SeverityFinding, "GitHub mode")
	})

	t.Run("gh absent in a non-GitHub config is not a finding", func(t *testing.T) {
		_, cfg := r.newOperand(t)
		cfg.Fetcher = denyFetcher{t}
		stubPATH(t, "git") // no gh anywhere on PATH
		h := mustDoctor(t, ctx, cfg)
		wantNoRows(t, h, converge.CheckPrerequisite)
	})
}

// ---- hooks: inactive, active, wrong value ------------------------------------

func TestDoctorHooksInactiveActive(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	gitInitRepo(t, root)
	cfg.Fetcher = denyFetcher{t}
	ctx := context.Background()

	h := mustDoctor(t, ctx, cfg)
	wantRow(t, h, converge.CheckHooks, "core.hooksPath", converge.SeverityFinding,
		"unset", `".githooks"`, "git config core.hooksPath .githooks", "FR-9")

	gitHooksPathSet(t, root, ".githooks")
	h = mustDoctor(t, ctx, cfg)
	wantNoRows(t, h, converge.CheckHooks)

	gitHooksPathSet(t, root, "custom/hooks")
	h = mustDoctor(t, ctx, cfg)
	wantRow(t, h, converge.CheckHooks, "core.hooksPath", converge.SeverityFinding, `"custom/hooks"`, `expected ".githooks"`)
}

// ---- tracker: carina store present/absent; github not probed -----------------

func TestDoctorTrackerCarinaAndGithub(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	t.Run("carina store absent then present", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: carina\n")
		cfg.Fetcher = denyFetcher{t}
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckTracker, "carina", converge.SeverityFinding, `".workitems"`, "carina init")

		if err := os.MkdirAll(at(root, ".workitems"), 0o755); err != nil {
			t.Fatal(err)
		}
		h = mustDoctor(t, ctx, cfg)
		wantNoRows(t, h, converge.CheckTracker)
	})

	t.Run("carina store is a file", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: carina\n")
		cfg.Fetcher = denyFetcher{t}
		writeFile(t, at(root, ".workitems"), []byte("not a dir"), 0o644)
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckTracker, "carina", converge.SeverityFinding, "not a directory")
	})

	t.Run("github reachability not probed", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: github\n")
		cfg.Fetcher = denyFetcher{t}
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckTracker, "github", converge.SeverityAdvisory, "not probed (network-free)")
	})

	t.Run("no tracker captured checks nothing", func(t *testing.T) {
		_, cfg := r.newOperand(t)
		cfg.Fetcher = denyFetcher{t}
		h := mustDoctor(t, ctx, cfg)
		wantNoRows(t, h, converge.CheckTracker)
	})
}

// ---- CLI surfaces: pointer wired / broken / foreign / unknown ----------------

// TestSurfacePointersDerivation proves converge.SurfacePointers is a pure
// function of the configured CLI-surface knob (config-driven-portability-v1
// §3.2, FR-3; DINV-1): the enabled surfaces' pointers are byte-identical to the
// former literal map's values; an enabled-but-unknown surface is absent (a
// caller reads it as unknown → advisory); and the derivation holds no cache —
// successive calls under a changed knob yield changed results.
func TestSurfacePointersDerivation(t *testing.T) {
	surf := func(names ...string) config.Config {
		c := config.Defaults()
		c.Knobs.CLISurfaces = names
		return c
	}

	// (c) byte-identity: the derived pointers for each known surface match the
	// exact Link→Target pairs the engine shipped as literals before this lift.
	claudeWant := []converge.SurfacePointer{
		{Link: ".claude/skills", Target: "../.pyxis/generated/harnesses/claude/skills"},
		{Link: ".claude/scripts", Target: "../.pyxis/scripts"},
		{Link: "CLAUDE.md", Target: "AGENTS.md"},
	}
	codexWant := []converge.SurfacePointer{
		{Link: ".agents/skills", Target: "../.pyxis/generated/harnesses/neutral/skills"},
		{Link: ".codex/skills", Target: "../.pyxis/generated/harnesses/codex/skills"},
	}
	got := converge.SurfacePointers(surf("claude", "codex"))
	if !slices.Equal(got["claude"], claudeWant) {
		t.Errorf("claude pointers = %+v, want %+v", got["claude"], claudeWant)
	}
	if !slices.Equal(got["codex"], codexWant) {
		t.Errorf("codex pointers = %+v, want %+v", got["codex"], codexWant)
	}

	// (a) enabling only one surface derives only that surface's pointers; the
	// other known surface is absent (it was not enabled).
	only := converge.SurfacePointers(surf("claude"))
	if _, ok := only["codex"]; ok {
		t.Error("claude-only config derived codex pointers — not a pure function of the knob")
	}
	if len(only) != 1 || !slices.Equal(only["claude"], claudeWant) {
		t.Errorf("claude-only derivation = %+v, want the claude pointers alone", only)
	}

	// An enabled-but-unknown surface is absent from the map: a caller's
	// `pointers, known := SurfacePointers(cfg)[s]` reads it as unknown, exactly
	// as the former literal map did (doctor then names it an advisory).
	unknown := converge.SurfacePointers(surf("kiro"))
	if _, known := unknown["kiro"]; known {
		t.Error("unknown surface kiro is present in the derived map")
	}

	// A default config (no surface knob set) derives an empty map — matching the
	// former literal map's behavior for a repo wiring no surface.
	if n := len(converge.SurfacePointers(config.Defaults())); n != 0 {
		t.Errorf("default-config surface pointers = %d entries, want 0 (no surface wired)", n)
	}

	// (b) no cache: two calls under DIFFERENT knobs return different maps — the
	// function recomputes from config, never a memoized earlier result.
	first := converge.SurfacePointers(surf("claude"))
	second := converge.SurfacePointers(surf("codex"))
	if _, ok := first["codex"]; ok {
		t.Error("no-cache: the claude call leaked a codex entry")
	}
	if _, ok := second["claude"]; ok {
		t.Error("no-cache: the codex call returned a stale claude entry")
	}
}

func TestDoctorCLISurfacePointers(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	ctx := context.Background()
	claude := "knobs:\n  cli_surfaces: [claude]\n"

	t.Run("pointers absent", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, claude)
		cfg.Fetcher = denyFetcher{t}
		h := mustDoctor(t, ctx, cfg)
		rows := doctorRows(h, converge.CheckSurfaces)
		if len(rows) != 3 {
			t.Fatalf("surface rows = %+v, want the three absent pointers", rows)
		}
		wantRow(t, h, converge.CheckSurfaces, "claude", converge.SeverityFinding, "is absent", "AP-25")
		var details []string
		for _, f := range rows {
			details = append(details, f.Detail)
		}
		joined := strings.Join(details, "\n")
		for _, p := range []string{".claude/skills", ".claude/scripts", "CLAUDE.md"} {
			if !strings.Contains(joined, p) {
				t.Errorf("surface findings %q do not name pointer %s", joined, p)
			}
		}
	})

	t.Run("pointers wired", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, claude)
		cfg.Fetcher = denyFetcher{t}
		wireClaude(t, root)
		h := mustDoctor(t, ctx, cfg)
		wantNoRows(t, h, converge.CheckSurfaces)
	})

	t.Run("dangling pointer", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, claude)
		cfg.Fetcher = denyFetcher{t}
		wireClaude(t, root)
		if err := os.Remove(at(root, ".claude/skills")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../.pyxis/no-such-dir", at(root, ".claude/skills")); err != nil {
			t.Fatal(err)
		}
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckSurfaces, "claude", converge.SeverityFinding, ".claude/skills", "want")
	})

	t.Run("content dir instead of a pointer", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, claude)
		cfg.Fetcher = denyFetcher{t}
		wireClaude(t, root)
		if err := os.Remove(at(root, ".claude/skills")); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(at(root, ".claude/skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckSurfaces, "claude", converge.SeverityFinding, "is not a symlink", "AP-25")
	})

	t.Run("codex pointers wired", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  cli_surfaces: [codex]\n")
		cfg.Fetcher = denyFetcher{t}
		if err := os.MkdirAll(at(root, ".pyxis/generated/harnesses/neutral/skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(at(root, ".pyxis/generated/harnesses/codex/skills"), 0o755); err != nil {
			t.Fatal(err)
		}
		for link, target := range map[string]string{
			".agents/skills": "../.pyxis/generated/harnesses/neutral/skills",
			".codex/skills":  "../.pyxis/generated/harnesses/codex/skills",
		} {
			if err := os.MkdirAll(filepath.Dir(at(root, link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, at(root, link)); err != nil {
				t.Fatal(err)
			}
		}
		h := mustDoctor(t, ctx, cfg)
		wantNoRows(t, h, converge.CheckSurfaces)
	})

	t.Run("unknown surface is a named advisory", func(t *testing.T) {
		root, cfg := r.newOperand(t)
		cfg = cfgWith(t, r, root, cfg, "knobs:\n  cli_surfaces: [kiro]\n")
		cfg.Fetcher = denyFetcher{t}
		h := mustDoctor(t, ctx, cfg)
		wantRow(t, h, converge.CheckSurfaces, "kiro", converge.SeverityAdvisory, "no pointer convention")
	})
}

// ---- epoch: reported present or absent ----------------------------------------

func TestDoctorEpochPresentAbsent(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	ctx := context.Background()

	root, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	if h.EpochPresent {
		t.Error("EpochPresent = true with no epoch block")
	}
	wantRow(t, h, converge.CheckEpoch, "epoch", converge.SeverityAdvisory, "no epoch block", "pyxis init")

	commit := strings.Repeat("e1", 20)
	cfg = cfgWith(t, r, root, cfg, fmt.Sprintf("epoch:\n  system_version: %q\n  commit: %q\n  manifest_sha256: %q\n", r.ver, commit, r.pin))
	h = mustDoctor(t, ctx, cfg)
	if !h.EpochPresent {
		t.Fatal("EpochPresent = false with the epoch block set")
	}
	if h.Epoch.SystemVersion != r.ver || h.Epoch.Commit != commit || h.Epoch.ManifestSHA256 != r.pin {
		t.Errorf("Epoch = %+v, want the block reported verbatim", h.Epoch)
	}
	wantNoRows(t, h, converge.CheckEpoch)
}

// ---- deprecations: the dual-read notices, verbatim ------------------------------

func TestDoctorDeprecationSurfacedVerbatim(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg = cfgWith(t, r, root, cfg, "docgate:\n  version: \"source\"\n")
	cfg.Fetcher = denyFetcher{t}

	deps := cfg.Repo.Deprecations()
	if len(deps) != 1 {
		t.Fatalf("fixture Deprecations = %v, want the one legacy-docgate notice", deps)
	}
	h := mustDoctor(t, context.Background(), cfg)
	rows := doctorRows(h, converge.CheckDeprecation)
	if len(rows) != 1 {
		t.Fatalf("deprecation rows = %+v, want exactly one", rows)
	}
	if rows[0].Detail != deps[0] {
		t.Errorf("deprecation detail = %q, want the notice VERBATIM %q", rows[0].Detail, deps[0])
	}
	if rows[0].Severity != converge.SeverityAdvisory || rows[0].Name != ".pyxis/config.yaml" {
		t.Errorf("deprecation row = %+v, want an advisory on .pyxis/config.yaml", rows[0])
	}
}

// ---- cache hygiene: stale entries named, never removed --------------------------

func TestDoctorStaleCacheAdvisories(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	stale := []string{"docgate-v0.1.0-" + r.plat, "payload-v0.1.0.tar.gz"}
	for _, name := range stale {
		writeFile(t, at(root, ".pyxis/bin/"+name), []byte("old"), 0o644)
	}

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	rows := doctorRows(h, converge.CheckCache)
	if len(rows) != 2 {
		t.Fatalf("cache rows = %+v, want the two stale entries", rows)
	}
	var names []string
	for _, f := range rows {
		names = append(names, f.Name)
		if f.Severity != converge.SeverityAdvisory {
			t.Errorf("cache row %s severity = %s, want advisory", f.Name, f.Severity)
		}
		if !strings.Contains(f.Detail, "off-version") || !strings.Contains(f.Detail, "pyxis converge") {
			t.Errorf("cache detail %q lacks the naming/remedy", f.Detail)
		}
	}
	if !slices.Equal(names, stale) {
		t.Errorf("cache advisories = %v, want %v (sorted, named)", names, stale)
	}
	for _, name := range stale {
		if _, err := os.Stat(at(root, ".pyxis/bin/"+name)); err != nil {
			t.Errorf("stale entry %s was touched — doctor never reaps: %v", name, err)
		}
	}
}

// ---- skeletons: unauthored (the REAL shipped templates) vs authored -------------

func TestDoctorSkeletonAdvisories(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t}
	ctx := context.Background()

	// Phase 1 — instantiate both governing skeletons byte-verbatim (what init
	// does, payload.SkeletonTargets): each is an unauthored advisory.
	for tmpl, target := range payload.SkeletonTargets(cfg.Repo) {
		writeFile(t, at(root, target), repoTemplate(t, tmpl), 0o644)
	}
	h := mustDoctor(t, ctx, cfg)
	rows := doctorRows(h, converge.CheckSkeleton)
	if len(rows) != 2 {
		t.Fatalf("skeleton rows = %+v, want both unauthored advisories", rows)
	}
	wantRow(t, h, converge.CheckSkeleton, "docs/product-vision.md", converge.SeverityAdvisory,
		"unauthored skeleton", "nothing follows the authoring line", "NG-1")
	wantRow(t, h, converge.CheckSkeleton, "docs/architecture-principles.md", converge.SeverityAdvisory,
		"unauthored skeleton", "nothing follows the authoring line")
	// Advisories never red the run on their own: no other findings were
	// injected, but the environment may add some — assert the skeleton rows
	// specifically carry advisory severity (done above via wantRow).

	// Phase 2 — author each governing doc the way its family contracts:
	// content after the authoring line. All advisories clear.
	appendAfterMarker := func(target, content string) {
		b := mustRead(t, at(root, target))
		writeFile(t, at(root, target), append(b, []byte(content)...), 0o644)
	}
	appendAfterMarker("docs/product-vision.md", "\n## 1. North star\n\nShip verified software from ideas.\n")
	appendAfterMarker("docs/architecture-principles.md", "\n**AP-1 — Fail closed.** On ambiguity, refuse and name the remedy.\n")
	h = mustDoctor(t, ctx, cfg)
	wantNoRows(t, h, converge.CheckSkeleton)

	// Phase 3 — a governance doc that never came from a skeleton (this
	// source repo's case) carries neither marker and is never an advisory.
	writeFile(t, at(root, "docs/product-vision.md"), []byte("# Product Vision\n\nHand-authored from scratch.\n"), 0o644)
	h = mustDoctor(t, ctx, cfg)
	wantNoRows(t, h, converge.CheckSkeleton)
}

// ---- verification: the code-without-manifest advisory (prd:init-hardening/FR-15, design init-hardening-v1 §3.9) ----

// TestDoctorVerificationAdvisory proves the T8 acceptance criterion directly:
// a code-free tree stays silent, a recognized stack marker with no manifest
// yields the advisory naming the codegate seed command (and the manifest
// path, read from invoke.ManifestRelPath — the same single source the check
// itself and the seed verb consume, never a second hand-kept copy), and
// seeding the manifest silences the advisory even though the marker file
// persists.
func TestDoctorVerificationAdvisory(t *testing.T) {
	isolateGit(t)
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	cfg.Fetcher = denyFetcher{t}
	ctx := context.Background()

	// Phase 1 — the fixture payload carries no ecosystem marker file: there
	// is no code to gate yet, so the check reports nothing.
	h := mustDoctor(t, ctx, cfg)
	wantNoRows(t, h, converge.CheckVerification)

	// Phase 2 — a recognized stack marker (go.mod) with no manifest is the
	// advisory, naming both the seed command and the manifest path.
	writeFile(t, at(root, "go.mod"), []byte("module fixture\n\ngo 1.23\n"), 0o644)
	h = mustDoctor(t, ctx, cfg)
	wantRow(t, h, converge.CheckVerification, invoke.ManifestRelPath, converge.SeverityAdvisory,
		"recognized code is present with no verification manifest",
		"pyxis exec codegate -- seed",
		invoke.ManifestRelPath)

	// Phase 3 — seeding the manifest silences the advisory even though the
	// marker file is still present: a manifest, once it exists, is
	// operator-owned and never re-advised.
	writeFile(t, at(root, invoke.ManifestRelPath), []byte("{}\n"), 0o644)
	h = mustDoctor(t, ctx, cfg)
	wantNoRows(t, h, converge.CheckVerification)
}

// ---- the zero-mutation proof: whole tree + cache, before/after ------------------

func TestDoctorZeroMutationSnapshot(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	// Everything at once: converged cache, injected drift, a stale cache
	// entry, a deprecation notice, carina + surfaces + epoch knobs, an
	// unauthored skeleton, and an active git repo with hooks configured.
	extra := "knobs:\n  tracker: carina\n  cli_surfaces: [claude]\n" +
		fmt.Sprintf("epoch:\n  system_version: %q\n  commit: %q\n  manifest_sha256: %q\n", r.ver, strings.Repeat("e2", 20), r.pin) +
		"docgate:\n  version: \"source\"\n"
	cfg = cfgWith(t, r, root, cfg, extra)
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	writeFile(t, at(root, ".pyxis/bin/docgate-v0.1.0-"+r.plat), []byte("old"), 0o755)
	writeFile(t, at(root, "docs/product-vision.md"), repoTemplate(t, payload.ProductVisionSkeleton), 0o644)
	if err := os.MkdirAll(at(root, ".workitems"), 0o755); err != nil {
		t.Fatal(err)
	}
	wireClaude(t, root)
	gitInitRepo(t, root)
	gitHooksPathSet(t, root, ".githooks")

	before := snap(t, root)
	cfg.Fetcher = denyFetcher{t} // zero fetches on top of zero writes
	h, err := converge.Doctor(ctx, cfg)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	sameSnap(t, before, snap(t, root), "doctor run") // the ENTIRE tree incl. .pyxis/bin and .git

	// Sanity: the run actually reported across the taxonomy.
	wantRow(t, h, converge.CheckDrift, ".pyxis/templates/prd-template.md", converge.SeverityFinding, "engine-verbatim modified")
	wantRow(t, h, converge.CheckCache, "docgate-v0.1.0-"+r.plat, converge.SeverityAdvisory, "off-version")
	wantRow(t, h, converge.CheckSkeleton, "docs/product-vision.md", converge.SeverityAdvisory, "unauthored skeleton")
	if rows := doctorRows(h, converge.CheckDeprecation); len(rows) != 1 {
		t.Errorf("deprecation rows = %+v, want one", rows)
	}
	if !h.EpochPresent {
		t.Error("EpochPresent = false, want the fixture epoch reported")
	}
	if h.Healthy {
		t.Error("Healthy = true with an injected drift finding")
	}

	// Findings arrive grouped by the taxonomy order.
	rank := map[converge.CheckClass]int{
		converge.CheckDrift: 0, converge.CheckPrerequisite: 1, converge.CheckHooks: 2,
		converge.CheckTracker: 3, converge.CheckSurfaces: 4, converge.CheckEpoch: 5,
		converge.CheckDeprecation: 6, converge.CheckCache: 7, converge.CheckSkeleton: 8,
		converge.CheckVerification: 9,
	}
	last := -1
	for _, f := range h.Findings {
		if rank[f.Class] < last {
			t.Fatalf("Findings not grouped by taxonomy: %+v", h.Findings)
		}
		last = rank[f.Class]
	}
}

// ---- the EVT-3 event shape -------------------------------------------------------

func TestDoctorEventShape(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("warming Run: %v", err)
	}
	// The deterministic drift fixture from the per-class test: 5 drift
	// findings; PATH stubs make the environment rows exact (no prereq rows,
	// one hooks finding via the stub git's unset probe).
	writeFile(t, at(root, ".pyxis/templates/prd-template.md"), []byte("tampered\n"), 0o644)
	if err := os.Remove(at(root, ".pyxis/scripts/hook.sh")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, at(root, "docs/tech-design/index.json"), []byte("stale\n"), 0o644)
	if err := os.Remove(at(root, ".pyxis/stamp/origin.json")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, at(root, ".pyxis/config.yaml"), []byte("bogus:\n  x: y\n"), 0o644)
	stubPATH(t, "git")

	var buf bytes.Buffer
	cfg.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)

	var evt map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == "doctor_run" {
			evt = m
		}
	}
	if evt == nil {
		t.Fatalf("no doctor_run event in:\n%s", buf.String())
	}
	if evt["healthy"] != false {
		t.Errorf("event healthy = %v, want false", evt["healthy"])
	}
	if evt["mode"] != "release" {
		t.Errorf("event mode = %v, want release", evt["mode"])
	}
	if evt["run_id"] != h.RunID || h.RunID == "" {
		t.Errorf("event run_id = %v, report run_id = %q — one id on every event", evt["run_id"], h.RunID)
	}
	if evt["findings"] != float64(6) { // 5 drift + 1 hooks
		t.Errorf("event findings = %v, want 6", evt["findings"])
	}
	if evt["advisories"] != float64(1) { // the absent epoch
		t.Errorf("event advisories = %v, want 1", evt["advisories"])
	}
	byClass, _ := evt["findings_by_class"].(map[string]any)
	if byClass == nil {
		t.Fatalf("event lacks the findings_by_class group: %v", evt)
	}
	want := map[string]float64{
		"drift": 5, "prerequisite": 0, "hooks": 1, "tracker": 0, "surfaces": 0,
		"epoch": 1, "deprecation": 0, "cache": 0, "skeleton": 0, "verification": 0,
	}
	for cl, n := range want {
		if byClass[cl] != n {
			t.Errorf("findings_by_class[%s] = %v, want %v", cl, byClass[cl], n)
		}
	}
	if evt["artifact_source"] != "file://"+r.src {
		t.Errorf("artifact_source = %v, want the override surfaced loudly", evt["artifact_source"])
	}
}

// ---- the healthy leg: a clean converged install reports Healthy -----------------

func TestDoctorHealthyOnCleanInstall(t *testing.T) {
	r := newRelease(t, stdFiles())
	root, cfg := r.newOperand(t)
	ctx := context.Background()
	cfg = cfgWith(t, r, root, cfg, "knobs:\n  tracker: carina\n  cli_surfaces: [claude]\n"+
		fmt.Sprintf("epoch:\n  system_version: %q\n  commit: %q\n  manifest_sha256: %q\n", r.ver, strings.Repeat("aa", 20), r.pin))
	if _, err := converge.Run(ctx, cfg, converge.Options{}); err != nil {
		t.Fatalf("converge: %v", err)
	}
	if err := os.MkdirAll(at(root, ".workitems"), 0o755); err != nil {
		t.Fatal(err)
	}
	wireClaude(t, root)
	gitInitRepo(t, root) // skips without git
	gitHooksPathSet(t, root, ".githooks")

	cfg.Fetcher = denyFetcher{t}
	h := mustDoctor(t, ctx, cfg)
	if !h.Healthy {
		t.Errorf("Healthy = false on a clean install; findings: %+v", h.Findings)
	}
	if len(h.Findings) != 0 {
		t.Errorf("Findings = %+v, want none at all (no findings, no advisories) on a clean install", h.Findings)
	}
	if h.Mode != "release" || !h.ManifestPinned || !h.EpochPresent {
		t.Errorf("summary = mode %q pinned %v epoch %v, want release/true/true", h.Mode, h.ManifestPinned, h.EpochPresent)
	}
}

// ---- guards ----------------------------------------------------------------------

func TestDoctorGuards(t *testing.T) {
	if _, err := converge.Doctor(context.Background(), resolve.Config{Logger: quiet()}); err == nil || !strings.Contains(err.Error(), "Root is empty") {
		t.Errorf("empty root err = %v, want the named guard", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := converge.Doctor(ctx, resolve.Config{Root: "x", Logger: quiet()}); !errors.Is(err, context.Canceled) {
		t.Errorf("dead-context err = %v, want context.Canceled", err)
	}
}
