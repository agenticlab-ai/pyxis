package selfhost

// A-3 — the default-config non-regression proof (design config-driven-
// portability-v1 §3.8; #A-3; #DINV-2; prd:config-driven-portability/NR-1/FR-6).
// Under a DEFAULT config, every derived target-facing surface is byte-identical
// to the pre-change engine's hardcoded value — so a repository that sets no
// non-default value behaves exactly as before the config surface existed. This
// is the strict counterpart to A-1: A-1 proves a NON-default config is honored;
// A-3 proves a DEFAULT config regresses nothing.
//
// The design names three derived surfaces to pin at the default (§3.8, the
// in-process golden bin): the generated-workflow render, the governing/skeleton
// maps, and the resolved paths. The workflow render's default byte-identity is
// already covered by converge's TestWorkflowDefaultRenderByteIdentical (T4); the
// branch/remote/path defaults and the two derived maps (T2/T3) are pinned here,
// so every derived surface has a named default-byte-identity assertion in one
// place or the other. The whole suite staying green under this task's relocation
// harness (go test ./... = 0 fail) is A-3's "existing suite passes
// unchanged" half.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// TestSourceHarnessViewsCurrent keeps the source checkout's committed
// bootstrap views byte-identical to the same template-driven views init and
// converge generate for operands. Source mode cannot run converge, so this is
// the mechanical freshness gate that prevents shared-skill fallback.
func TestSourceHarnessViewsCurrent(t *testing.T) {
	root := repoRoot(t)
	tmpl, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(payload.HarnessImplementNowSkill)))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadDir(filepath.Join(root, ".pyxis", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range canonical {
		if !entry.IsDir() || entry.Name() == "implement-now" {
			continue
		}
		want := "../../.pyxis/skills/" + entry.Name()
		for _, view := range []string{".agents", ".claude", ".codex", ".cursor", ".opencode"} {
			got, err := os.Readlink(filepath.Join(root, view, "skills", entry.Name()))
			if err != nil || filepath.ToSlash(got) != want {
				t.Errorf("%s/skills/%s = %q, %v; want %q", view, entry.Name(), got, err, want)
			}
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".agents", "skills", "implement-now")); !os.IsNotExist(err) {
		t.Fatalf(".agents/skills exposes implement-now: %v", err)
	}
	for harness, vars := range map[string]map[string]string{
		"claude":   {"harness_name": "Claude Code", "backend_id": "claude"},
		"codex":    {"harness_name": "Codex", "backend_id": "codex"},
		"cursor":   {"harness_name": "Cursor CLI", "backend_id": "cursor"},
		"opencode": {"harness_name": "OpenCode", "backend_id": "opencode"},
	} {
		want, err := payload.Substitute(tmpl, vars)
		if err != nil {
			t.Fatal(err)
		}
		generated := filepath.Join(root, ".pyxis", "generated", "harnesses", harness, "skills", "implement-now", "SKILL.md")
		got, err := os.ReadFile(generated)
		if err != nil || string(got) != string(want) {
			t.Errorf("%s generated implement-now is stale: %v", harness, err)
		}
		link := filepath.Join(root, "."+harness, "skills", "implement-now")
		wantLink := "../../.pyxis/generated/harnesses/" + harness + "/skills/implement-now"
		gotLink, err := os.Readlink(link)
		if err != nil || filepath.ToSlash(gotLink) != wantLink {
			t.Errorf("%s = %q, %v; want %q", link, gotLink, err, wantLink)
		}
	}
}

// preChangeDefaults are the exact target-facing values the engine hardcoded
// before config-driven portability — the literals T1/T2 moved into the schema.
// Pinning Defaults() against this independent copy is DINV-2 made a test: if a
// default is ever retuned, this fails loudly, because a changed default is a
// changed behavior for every repo that sets no override.
var preChangeDefaults = struct {
	branch, remote string
	docPaths       map[string]string // config field name -> pre-change literal
}{
	branch: "main",
	remote: "origin",
	docPaths: map[string]string{
		"PRDs":                   "docs/prds",
		"Feedback":               "docs/feedback",
		"TechDesigns":            "docs/tech-design",
		"SolutionShapes":         "docs/solution-shapes",
		"Brainstorms":            "docs/brainstorms",
		"Problems":               "docs/problems",
		"Reviews":                "docs/reviews",
		"Wiki":                   "docs/wiki",
		"RollupPRD":              "docs/prd.md",
		"RollupTechspec":         "docs/techspec.md",
		"Incidents":              "docs/observability/incidents.md",
		"ProductVision":          "docs/product-vision.md",
		"ArchitecturePrinciples": "docs/architecture-principles.md",
		"DesignPrinciples":       "docs/design-principles.md",
		"Plans":                  "docs/plans",
		"WorkItems":              ".workitems",
		"RiskPolicy":             "docs/policy/risk-policy.yaml",
		"MaintenancePolicy":      "docs/policy/maintenance-sweep.yaml",
	},
}

// TestDefaultConfigBranchRemotePathsByteIdentical pins the default branch,
// remote, and every target-facing doc path to their pre-change literals
// (DINV-2). A default config resolves each surface to exactly the value the
// engine baked before the schema carried it.
func TestDefaultConfigBranchRemotePathsByteIdentical(t *testing.T) {
	d := config.Defaults()

	if d.Branch != preChangeDefaults.branch {
		t.Errorf("Defaults().Branch = %q, want the pre-change literal %q (DINV-2)", d.Branch, preChangeDefaults.branch)
	}
	if d.Remote != preChangeDefaults.remote {
		t.Errorf("Defaults().Remote = %q, want the pre-change literal %q (DINV-2)", d.Remote, preChangeDefaults.remote)
	}
	if d.Drafts != "drafts" {
		t.Errorf("Defaults().Drafts = %q, want the feature-draft default %q", d.Drafts, "drafts")
	}

	// Each target-facing doc path equals its pre-change literal. Keyed by the
	// Config field name so the pin (preChangeDefaults.docPaths) and the observed
	// default line up one-to-one and stay legible.
	got := map[string]string{
		"PRDs":                   d.PRDs,
		"Feedback":               d.Feedback,
		"TechDesigns":            d.TechDesigns,
		"SolutionShapes":         d.SolutionShapes,
		"Brainstorms":            d.Brainstorms,
		"Problems":               d.Problems,
		"Reviews":                d.Reviews,
		"Wiki":                   d.Wiki,
		"RollupPRD":              d.RollupPRD,
		"RollupTechspec":         d.RollupTechspec,
		"Incidents":              d.Incidents,
		"ProductVision":          d.ProductVision,
		"ArchitecturePrinciples": d.ArchitecturePrinciples,
		"DesignPrinciples":       d.DesignPrinciples,
		"Plans":                  d.Plans,
		"WorkItems":              d.WorkItems,
		"RiskPolicy":             d.RiskPolicy,
		"MaintenancePolicy":      d.MaintenancePolicy,
	}
	for field, want := range preChangeDefaults.docPaths {
		if got[field] != want {
			t.Errorf("Defaults().%s = %q, want the pre-change literal %q (DINV-2)", field, got[field], want)
		}
	}

	// Completeness: every single path the closed schema carries (config.PathDefaults,
	// the authoritative pathSchema enumeration) must be pinned above, so a future
	// schema addition cannot introduce a new target-facing default that this
	// non-regression proof silently fails to guard (DINV-2). templates_dir is the
	// one older schema path deliberately excluded — it is system-internal, not
	// target-facing (NG-1). Drafts is also excluded because feature-draft
	// introduced it after the pre-change baseline; its default is asserted above.
	pinnedValues := make(map[string]bool, len(preChangeDefaults.docPaths))
	for _, v := range preChangeDefaults.docPaths {
		pinnedValues[v] = true
	}
	for key, def := range config.PathDefaults() {
		if key == "templates_dir" || key == "drafts" {
			continue
		}
		if !pinnedValues[def] {
			t.Errorf("schema path %q (default %q) is not pinned in preChangeDefaults.docPaths — every target-facing schema path must have a byte-identity pin (DINV-2); add it", key, def)
		}
	}
}

// TestDefaultMainlineRefByteIdentical pins the composed mainline ref under the
// default config to the former "origin/main" literal (DINV-2) — the ONE place
// (config.MainlineRef, AP-11) every branch/remote consumer derives it, so this
// guards the composition every hook/lane/plan-tracker read inherits.
func TestDefaultMainlineRefByteIdentical(t *testing.T) {
	if got := config.Defaults().MainlineRef(); got != "origin/main" {
		t.Errorf("Defaults().MainlineRef() = %q, want the pre-change literal %q (DINV-2)", got, "origin/main")
	}
}

// TestDefaultSkeletonTargetsByteIdentical pins T3's derived skeleton-target map
// under the default config to the former literal targets (DINV-2): the two
// governing docs seed at docs/product-vision.md and docs/architecture-
// principles.md, exactly as the pre-change literal map did.
func TestDefaultSkeletonTargetsByteIdentical(t *testing.T) {
	skel := payload.SkeletonTargets(config.Defaults())
	want := map[string]string{
		payload.ProductVisionSkeleton:          "docs/product-vision.md",
		payload.ArchitecturePrinciplesSkeleton: "docs/architecture-principles.md",
	}
	if len(skel) != len(want) {
		t.Fatalf("SkeletonTargets(default) has %d entries, want %d: %v", len(skel), len(want), skel)
	}
	for link, target := range want {
		if skel[link] != target {
			t.Errorf("SkeletonTargets(default)[%q] = %q, want the pre-change literal %q (DINV-2)", link, skel[link], target)
		}
	}
}

// TestDefaultSurfacePointersByteIdentical pins T3's derived surface-pointer map
// under the default config (DINV-2). A repo wiring NO CLI surface (the default
// knob is unset) yields an empty pointer map — the former literal map's behavior
// for a repo wiring nothing; wiring [claude] yields the claude pointer set
// with the harness-specific generated skill view. Both are pinned so the
// derivation can never silently change the installer contract.
func TestDefaultSurfacePointersByteIdentical(t *testing.T) {
	// Default (unset knob): empty map.
	if got := converge.SurfacePointers(config.Defaults()); len(got) != 0 {
		t.Errorf("SurfacePointers(default) = %v, want empty (a repo wiring no surface) (DINV-2)", got)
	}
	// Wiring [claude]: skills select the generated Claude view while the
	// harness-neutral scripts and manual pointers retain their canonical targets.
	cfg := config.Defaults()
	cfg.Knobs.CLISurfaces = []string{"claude"}
	got := converge.SurfacePointers(cfg)
	claude, ok := got["claude"]
	if !ok {
		t.Fatalf("SurfacePointers([claude]) has no claude entry: %v", got)
	}
	want := []converge.SurfacePointer{
		{Link: ".claude/skills", Target: "../.pyxis/generated/harnesses/claude/skills"},
		{Link: ".claude/scripts", Target: "../.pyxis/scripts"},
		{Link: "CLAUDE.md", Target: "AGENTS.md"},
	}
	if len(claude) != len(want) {
		t.Fatalf("SurfacePointers([claude])[claude] has %d pointers, want %d: %v", len(claude), len(want), claude)
	}
	for i, w := range want {
		if claude[i] != w {
			t.Errorf("SurfacePointers([claude])[claude][%d] = %+v, want %+v (DINV-2)", i, claude[i], w)
		}
	}
}
