package selfhost

// A-1 — the self-host proof (design config-driven-portability-v1 §3.8; #A-1;
// prd:config-driven-portability/NR-1). The pyxis repository's own real corpus,
// relocated under a NON-DEFAULT config (branch renamed, remote set, doc corpora
// moved, surface pointers moved), runs green through the full doc gate + doctor
// + the branch/remote runtime resolution. A target-facing literal missed
// anywhere fails here, on the real corpus, because a non-default config no
// longer lets a missed value coincidentally match its default (R1, DINV-1).
//
// The full gate here is the base-PRESENT gate umbrella (gate.Gate with a real
// merge base) — every registered check, including the history checks
// (check-immutable, check-governing) that a base-free desk run skips. The
// history checks execute their git diffs against the CONFIGURED branch and the
// CONFIGURED corpus paths (so their config plumbing is exercised); but note the
// relocated corpus lands as ONE all-additions commit whose sha IS the merge
// base, so each history diff spans an EMPTY range and finds nothing to flag —
// they run, they do not detect. The doc-path-literal catch A-1 actually relies
// on is the TREE-LOCAL resolution: gate.LedgerDirs errors (→ this test fatals,
// below) if the configured PRDs root does not resolve to the relocated tree,
// and the per-ledger fold + tree-local checks read content only at the config-
// resolved roots. The branch/remote and governing-doc catches are proven by
// their own dedicated legs (TestSelfHostBranchRemoteResolveFromConfig via
// MainlineRef; TestSelfHostDoctorIsHealthy's skeleton-target assertion) — the
// gate leg is the corpus-wide coherence half, not the branch/remote half.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// TestSelfHostRelocatedCorpusIsGreen is A-1's core: the full doc gate is green
// over the real corpus relocated under a non-default config.
func TestSelfHostRelocatedCorpusIsGreen(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: the self-host proof archives the real corpus and runs the full gate")
	}
	requireGit(t)

	src := repoRoot(t)
	root := t.TempDir()
	materializeCorpus(t, src, root)
	relocateCorpus(t, root)
	cfg := nonDefaultConfig(t, src)
	regenerateGeneratedSurfaces(t, root, cfg)
	mergeBase := initRelocatedRepo(t, root)
	copySourceLockTags(t, src, root)

	// The ledger directories must resolve from the configured (relocated) PRDs
	// root. This is A-1's load-bearing resolution catch: gate.LedgerDirs reads
	// root/cfg.PRDs, and relocateCorpus renamed docs/ away, so a PRDs path that
	// stayed hardcoded to the default (docs/prds) makes this ReadDir ERROR — the
	// fatal below — rather than silently reading the wrong tree. A non-empty
	// result additionally proves the relocated root is populated.
	dirs, err := gate.LedgerDirs(root, cfg)
	if err != nil {
		t.Fatalf("resolving ledger directories from the relocated config (cfg.PRDs=%q): %v — a hardcoded PRDs path would fail here (DINV-1, R1)", cfg.PRDs, err)
	}
	if len(dirs) == 0 {
		t.Fatalf("no ledger directories found under the relocated PRDs root %q — the relocation did not take", cfg.PRDs)
	}
	// The full gate over the relocated corpus, base present so every check runs.
	// The tree-local checks (the per-ledger fold, LintGoverning over the relocated
	// governing paths, check-citations/links, lint-wiki/-tech-designs, verify-*)
	// read content only at config-resolved roots, so a doc-path literal that failed
	// to resolve reads an empty/absent tree and reds here; the history checks run
	// their configured-path diffs but over an empty range (see the package doc).
	defects := gate.Gate(root, mergeBase, dirs, cfg)
	if len(defects) != 0 {
		t.Fatalf("the full gate is NOT green over the relocated corpus (%d defect(s)) — a target-facing literal is likely still hardcoded (DINV-1, R1):\n%s",
			len(defects), renderDefects(defects))
	}
	t.Logf("A-1: full gate GREEN over %d ledger directories, corpus relocated under branch=%q remote=%q prefix=%q",
		len(dirs), cfg.Branch, cfg.Remote, corpusPrefix)
}

// TestSelfHostDoctorIsHealthy is A-1's doctor leg: `pyxis doctor`, run over the
// relocated tree under the non-default config, reports Healthy — and its config-
// driven checks (skeleton targets, surface pointers) resolve to the CONFIGURED
// locations, never the defaults. Doctor runs in source mode (this corpus is the
// engine's own source, DINV-8): the drift check is skipped by name (an advisory,
// not a finding), and every environment/skeleton/surface check runs. A healthy
// verdict here means no config-driven doctor check hardcoded a default path.
func TestSelfHostDoctorIsHealthy(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: the self-host proof archives the real corpus")
	}
	requireGit(t)

	src := repoRoot(t)
	root := t.TempDir()
	materializeCorpus(t, src, root)
	relocateCorpus(t, root)
	cfg := nonDefaultConfig(t, src)
	regenerateGeneratedSurfaces(t, root, cfg)
	_ = initRelocatedRepo(t, root)
	// A pyxis-operated repo activates the hook gate; set it so doctor's hooks
	// check passes (the source repo's one-time `git config core.hooksPath`).
	git(t, root, "config", "core.hooksPath", ".githooks")

	// Source mode: doctor works in the source repo (branches AROUND converge's
	// source-mode refusal), skipping only the drift diff. No CLI-surface pointer
	// files exist in the throwaway, so wiring [codex] would red the surfaces
	// check — the DOCTOR-surface proof is that SurfacePointers derives from the
	// configured knob (asserted directly below), so here we leave the knob unset
	// to keep the environment check set clean and let Healthy mean "environment
	// + config-driven skeleton resolution are green".
	cfg.System.Mode = "source"
	cfg.Knobs.CLISurfaces = nil
	rc := resolve.Config{Root: root, Repo: cfg, Logger: quietLogger(), Fetcher: denyFetcher{t}}

	h, err := converge.Doctor(context.Background(), rc)
	if err != nil {
		t.Fatalf("doctor over the relocated tree: %v", err)
	}
	if h.Mode != "source" {
		t.Errorf("doctor Mode = %q, want source (the corpus is the engine's own source, DINV-8)", h.Mode)
	}
	if !h.Healthy {
		t.Fatalf("doctor is NOT healthy over the relocated corpus; findings:\n%s", renderHealth(h))
	}

	// The config-driven doctor skeleton check resolves the CONFIGURED (relocated)
	// governing-doc targets, not the defaults (T3's skeleton map as doctor consumes
	// it — SkeletonTargets is fed the SAME cfg the Doctor run received, DINV-1).
	skel := payload.SkeletonTargets(cfg)
	for _, want := range []string{cfg.ProductVision, cfg.ArchitecturePrinciples} {
		if !strings.HasPrefix(want, corpusPrefix+"/") {
			t.Fatalf("test bug: governing-doc target %q is not relocated", want)
		}
		found := false
		for _, tgt := range skel {
			if tgt == want {
				found = true
			}
		}
		if !found {
			t.Errorf("doctor's skeleton targets %v do not include the relocated governing doc %q (DINV-1: the skeleton map is a pure function of the governing-doc path config)", skel, want)
		}
	}
	// The surface-pointer derivation honors a configured non-default CLI-surface
	// knob. This is a DIRECT check of SurfacePointers (the map doctor's surfaces
	// check consumes), NOT of this Doctor run — the run above deliberately set
	// CLISurfaces=nil (no pointer files exist in the throwaway), so its surfaces
	// check is a clean no-op. The derivation itself is what must honor config, and
	// [codex] is a non-default choice this repo wires nowhere, so a hardcoded
	// surface map would not return it (DINV-1). The default/[claude] byte-identity
	// of the same derivation is pinned by TestDefaultSurfacePointersByteIdentical.
	if _, wired := converge.SurfacePointers(config.Config{Knobs: config.Knobs{CLISurfaces: []string{"codex"}}})["codex"]; !wired {
		t.Errorf("the surface-pointer derivation does not honor a configured non-default CLI-surface knob [codex] (DINV-1)")
	}
	t.Logf("A-1: doctor HEALTHY over the relocated corpus (source mode; skeleton targets resolve to %v)", skel)
}

// TestSelfHostBranchRemoteResolveFromConfig is A-1's branch/remote leg: the
// runtime resolution the git-hooks delegate to (docgate's pre-push mainline
// resolution) yields the CONFIGURED <remote>/<branch>, not "origin/main", over
// the relocated tree — so a repo whose mainline is not origin/main is honored
// (T2, DINV-1/DINV-4). This is the branch/remote half of the coherence property,
// proven on the same non-default config the gate leg uses.
func TestSelfHostBranchRemoteResolveFromConfig(t *testing.T) {
	src := repoRoot(t)
	cfg := nonDefaultConfig(t, src)

	// The one place the mainline ref is composed (config.MainlineRef, AP-11) —
	// exactly what docgate's pre-push resolves when the hook bakes no
	// --merge-base (cmd/docgate/main.go: `mbRef = cfg.MainlineRef()`).
	if got, want := cfg.MainlineRef(), nonDefaultRemote+"/"+nonDefaultBranch; got != want {
		t.Fatalf("MainlineRef() = %q, want %q — branch/remote did not resolve from config (DINV-1/DINV-4)", got, want)
	}
	// And it differs from the default, so a missed literal would be caught.
	if cfg.MainlineRef() == config.Defaults().MainlineRef() {
		t.Fatalf("the non-default mainline ref equals the default %q — the branch/remote surfaces were not varied", config.Defaults().MainlineRef())
	}
	t.Logf("A-1: branch/remote resolve from config — mainline ref = %q (default would be %q)", cfg.MainlineRef(), config.Defaults().MainlineRef())
}

// regenerateGeneratedSurfaces re-renders the GENERATED-class surfaces that a
// relocation invalidates, exactly as converge would on the operand tree — the
// operand's duty when it relocates its corpus (design §3.4). Two generated
// surfaces depend on the corpus/config:
//
//   - the tech-design index (index.md + index.json under the configured
//     tech_designs root) — regenerated through the SOLE generator
//     (gate.GenIndexBytes, what `docgate gen-index` writes), so verify-index
//     finds the committed index byte-identical to the recomputed one;
//   - the installed CI workflow (.github/workflows/docs-gate.yml) — re-rendered
//     from the shipped template with the configured branch/remote substituted
//     (payload.RenderInstalledWorkflow), so the operand's workflow triggers and
//     diffs against the configured mainline (A-5).
//
// Regenerating through the SAME generators the engine uses (never a hand-write)
// keeps this honest: it is precisely what a `pyxis converge` on the relocated
// operand would do, not a fixture fudge.
func regenerateGeneratedSurfaces(t *testing.T, root string, cfg config.Config) {
	t.Helper()

	md, js, defects := gate.GenIndexBytes(root, cfg)
	if len(defects) > 0 {
		t.Fatalf("regenerating the tech-design index at the relocated root %q: %s", cfg.TechDesigns, renderDefects(defects))
	}
	idxDir := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns))
	writeFile(t, filepath.Join(idxDir, "index.md"), md)
	writeFile(t, filepath.Join(idxDir, "index.json"), js)

	// The installed workflow renders from the shipped template with the
	// configured branch/remote — the generated surface a relocated/renamed repo
	// must carry so its CI triggers on the right branch.
	tmplPath := filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflow))
	tmpl, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("read the shipped workflow template %s: %v", payload.InstalledWorkflow, err)
	}
	pin := strings.Repeat("a", 64)
	rendered, err := payload.RenderInstalledWorkflow(tmpl, cfg.Branch, cfg.Remote, payload.WorkflowPins{
		Version: "v0.0.0", LinuxARM64: pin, LinuxAMD64: pin, ArtifactSource: "https://releases.example/v0.0.0",
	})
	if err != nil {
		t.Fatalf("render the installed workflow (branch=%q remote=%q): %v", cfg.Branch, cfg.Remote, err)
	}
	// The rendered workflow carries the configured branch/remote, not the
	// defaults — a direct A-5 spot check on the self-host config.
	for _, want := range []string{"branches: [" + cfg.Branch + "]", "git fetch " + cfg.Remote + " " + cfg.Branch} {
		if !strings.Contains(string(rendered), want) {
			t.Fatalf("the rendered installed workflow is missing the configured literal %q (A-5, DINV-4)", want)
		}
	}
	if strings.Contains(string(rendered), "{{") {
		t.Fatalf("the rendered installed workflow retains a substitution token")
	}
	writeFile(t, filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflowTarget)), rendered)
}

// renderDefects joins a defect slice into a readable multi-line block for a
// failure message.
func renderDefects(defects []schema.Defect) string {
	var b strings.Builder
	for _, d := range defects {
		b.WriteString("  - ")
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}

// renderHealth joins a doctor report's rows into a readable block, marking
// findings (which red the run) distinctly from advisories.
func renderHealth(h converge.Health) string {
	var b strings.Builder
	for _, f := range h.Findings {
		b.WriteString("  - [")
		b.WriteString(string(f.Severity))
		b.WriteString("] ")
		b.WriteString(string(f.Class))
		b.WriteString(" ")
		b.WriteString(f.Name)
		b.WriteString(": ")
		b.WriteString(f.Detail)
		b.WriteByte('\n')
	}
	return b.String()
}
