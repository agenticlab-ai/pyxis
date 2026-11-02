package writehook

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// dcfg is the default configuration the write-hook tests decide under (today's
// doc-path roots); the config-driven root resolution is proven at the boundary
// (TestClassify_RelocatedDocRoots).
var dcfg = config.Defaults()

// widgetFirstPR is a resolvable first-PR drive for problem "widget": its own
// doc subtrees are in scope, everything else denies. Used as the corpus's
// legitimate drive so an in-scope write is genuinely allowed.
func widgetFirstPR() boundary.Drive {
	return boundary.Drive{Slug: "widget", Scope: boundary.ResolveFirstPR(dcfg, "widget")}
}

// assertThreePartDeny fails unless d is a deny whose reason names the path, the
// boundary, and a non-empty remedy — the {path, boundary, remedy} shape NFR-4
// mandates and A-1/A-4 assert.
func assertThreePartDeny(t *testing.T, d Decision, wantPath, wantBoundarySub string) {
	t.Helper()
	if d.Allow {
		t.Fatalf("target %q was allowed, want deny", wantPath)
	}
	if d.Path != wantPath {
		t.Errorf("deny path = %q, want %q", d.Path, wantPath)
	}
	if !strings.Contains(d.Boundary, wantBoundarySub) {
		t.Errorf("deny boundary = %q, want it to contain %q", d.Boundary, wantBoundarySub)
	}
	if strings.TrimSpace(d.Remedy) == "" {
		t.Errorf("deny for %q names no remedy — the third part of the reason is missing", wantPath)
	}
	r := d.Reason()
	for _, part := range []string{d.Path, d.Boundary, d.Remedy} {
		if !strings.Contains(r, part) {
			t.Errorf("Reason()=%q omits a required part %q", r, part)
		}
	}
}

func TestDecide_ProtectedCorpusDeniesInScopeAllows(t *testing.T) {
	// A-1: on the seeded write-attempt corpus, every protected-path class is
	// denied with a three-part reason, and the in-scope write is allowed.
	d := widgetFirstPR()

	denies := []struct {
		name, target, boundarySub string
	}{
		{"authority contract", ".pyxis/skills/argus/authority-contract.md", "authority contract"},
		{"authority contract via CLI symlink", ".claude/skills/argus/authority-contract.md", "authority contract"},
		{"frozen governance doc", "docs/product-vision.md", "governance"},
		{"governance incidents", "docs/observability/incidents.md", "governance"},
		{"another problem's ledger", "docs/problems/other/delivery.md", "another problem"},
		{"out of declared scope", "internal/engine/core.go", "out of declared scope"},
	}
	for _, c := range denies {
		t.Run("deny/"+c.name, func(t *testing.T) {
			assertThreePartDeny(t, Decide(dcfg, c.target, d), c.target, c.boundarySub)
		})
	}

	allows := []string{
		"docs/prds/widget/000-base.md",
		"docs/problems/widget/delivery.md",
		"docs/brainstorms/widget-tech-design.md",
	}
	for _, target := range allows {
		t.Run("allow/"+target, func(t *testing.T) {
			got := Decide(dcfg, target, d)
			if !got.Allow {
				t.Fatalf("in-scope target %q was denied (%s) — a false deny (R3)", target, got.Reason())
			}
			if got.Reason() != "" {
				t.Errorf("allow carries a reason %q, want empty", got.Reason())
			}
		})
	}
}

func TestDecide_UnresolvableScopeDeniesNeverAllows(t *testing.T) {
	// A-4 / DINV-2: an unresolvable declared scope denies with the three-part
	// reason, never an in-scope allow.
	nilScope := boundary.Drive{Slug: "widget", Scope: nil}
	got := Decide(dcfg, "internal/engine/core.go", nilScope)
	assertThreePartDeny(t, got, "internal/engine/core.go", "unresolvable")
	if !strings.Contains(got.Remedy, "scope") {
		t.Errorf("unresolvable remedy = %q, want it to point at resolving the declared scope", got.Remedy)
	}

	// No drive at all (empty slug + nil scope) also denies a scope-dependent
	// target, never allows.
	if d := Decide(dcfg, "cmd/docgate/main.go", boundary.Drive{}); d.Allow {
		t.Fatal("a write with no drive was allowed — must fail closed")
	}
}

func TestDecide_InjectionIsStructuralNotContentSteered(t *testing.T) {
	// A-5: a planted-directive-induced protected-path write is still denied —
	// the decision is target-path classification, unsteered by any directive.
	// Even under the drive's own (widget) first-PR scope, the authority
	// contract path denies: the verdict never consults the drive's intent.
	for _, target := range []string{
		".pyxis/skills/argus/authority-contract.md",
		".claude/skills/argus/authority-contract.md",
	} {
		got := Decide(dcfg, target, widgetFirstPR())
		assertThreePartDeny(t, got, target, "authority contract")
	}
}

func TestDecide_PolicyVersionRecorded(t *testing.T) {
	// AP-6: the verdict records the version-pinned protected-path data it was
	// made against, on both an allow and a deny.
	if got := Decide(dcfg, "docs/prds/widget/000-base.md", widgetFirstPR()); got.Policy != boundary.DataVersion {
		t.Errorf("allow policy = %q, want %q", got.Policy, boundary.DataVersion)
	}
	if got := Decide(dcfg, "docs/product-vision.md", widgetFirstPR()); got.Policy != boundary.DataVersion {
		t.Errorf("deny policy = %q, want %q", got.Policy, boundary.DataVersion)
	}
}

func TestDenyForHookError_FailsClosedWithThreePartReason(t *testing.T) {
	// DINV-3: a hook process error/crash/timeout denies with the same
	// three-part reason shape, never allows.
	got := DenyForHookError("docs/prds/widget/000-base.md", errString("hook crashed"))
	assertThreePartDeny(t, got, "docs/prds/widget/000-base.md", HookErrorBoundary)
	if !strings.Contains(got.Remedy, "restart") && !strings.Contains(got.Remedy, "fix") {
		t.Errorf("hook-error remedy = %q, want it to point at fixing/restarting the hook", got.Remedy)
	}

	// Even with no readable target the deny still carries a path placeholder and
	// the three-part reason — never a silent allow.
	blind := DenyForHookError("", nil)
	if blind.Allow {
		t.Fatal("a hook error with no readable target was allowed — must fail closed")
	}
	if strings.TrimSpace(blind.Path) == "" {
		t.Error("hook-error deny with no target names no path placeholder")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
