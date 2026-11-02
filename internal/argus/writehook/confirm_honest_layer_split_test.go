package writehook

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
)

// TestConfirmHonestLayerSplit_WriteHookStandsAsFastLayer is the plan-002/T3
// confirmation: the write hook shipped by plan 001 still stands as the fast
// at-write-time layer of the honest-layer split the hardened attribution gate
// completes (design:argus-attribution-hardening#3.8 — "the write hook (part 1)
// … its own design; the honest-layer split"). The hardening changed the
// merge-boundary attribution *derivation*, not this hook; this test pins the
// three T3 acceptance criteria against the shipped decision, adapters, and
// hook-error path in one named place, so a later change to the backstop layer
// cannot silently erode the fast layer.
//
// It asserts nothing new — the behavior is covered by the shipped fixtures
// (TestDecide_*, TestClaudeAdapter_*, TestCodexAdapter_*, TestPortability_*).
// Its job is to bind that behavior to T3's contract as a single regression
// guard for the split.
func TestConfirmHonestLayerSplit_WriteHookStandsAsFastLayer(t *testing.T) {
	// Criterion 1: a protected-path or out-of-scope write is denied fail-closed
	// and an in-scope write is allowed — the canonical deny decision (§3.1).
	t.Run("criterion1/protected-or-out-of-scope-deny-in-scope-allow", func(t *testing.T) {
		drive := widgetFirstPR()

		// A protected path (governance doc) and an out-of-scope path both deny
		// fail-closed with the three-part {path, boundary, remedy} reason.
		assertThreePartDeny(t, Decide(dcfg, "docs/product-vision.md", drive), "docs/product-vision.md", "governance")
		assertThreePartDeny(t, Decide(dcfg, "internal/engine/core.go", drive), "internal/engine/core.go", "out of declared scope")

		// The in-scope write is allowed — the fast layer does not over-deny (R3).
		if got := Decide(dcfg, "docs/prds/widget/000-base.md", drive); !got.Allow {
			t.Fatalf("in-scope write was denied (%s) — the fast layer must not over-deny", got.Reason())
		}
	})

	// Criterion 2: each supported CLI's wiring is a pointer with no protected-path
	// content, and a non-honoring CLI is listed with the merge-boundary backstop
	// as its guarantee (§3.2, DINV-4/DINV-5, UB-2). Asserted at the registry
	// level: every adapter is either a honoring pointer (a named hook, no I/O
	// content authored beyond translation) or a non-honoring CLI whose stated
	// guarantee is the merge-boundary backstop.
	t.Run("criterion2/cli-wiring-is-pointer-non-honoring-lists-backstop", func(t *testing.T) {
		sawNonHonoringBackstop := false
		for _, a := range Adapters() {
			if a.Honoring {
				if strings.TrimSpace(a.Hook) == "" {
					t.Errorf("honoring adapter %q names no pre-write hook surface", a.Name)
				}
				if a.Backstop != "" {
					t.Errorf("honoring adapter %q carries a backstop %q — a honoring CLI's guarantee is its own fail-closed deny, not a backstop", a.Name, a.Backstop)
				}
				continue
			}
			// A non-honoring CLI must be listed with the merge-boundary backstop as
			// its stated guarantee — never a silent fail-open.
			if !strings.Contains(a.Backstop, "merge-boundary") {
				t.Errorf("non-honoring adapter %q backstop = %q, want it to name the merge-boundary layers", a.Name, a.Backstop)
			}
			// And any evaluation attempt on it fails closed (deny), never allows.
			var buf bytes.Buffer
			if exit := a.Evaluate(dcfg, []byte(`{"tool_name":"Write","tool_input":{"file_path":"x"}}`),
				widgetFirstPR(), &buf); exit == 0 {
				t.Errorf("non-honoring adapter %q Evaluate exit = 0, want a hard-deny nonzero (fail-open hole)", a.Name)
			}
			sawNonHonoringBackstop = true
		}
		if !sawNonHonoringBackstop {
			t.Error("no non-honoring CLI is listed with the merge-boundary backstop — the split's honesty rests on naming it")
		}
	})

	// Criterion 3: a forced hook-runtime error denies fail-closed, never allows
	// (§3.3, DINV-3). Both the canonical decision (DenyForHookError) and a
	// honoring adapter fed an unparseable request must deny.
	t.Run("criterion3/hook-runtime-error-denies-never-allows", func(t *testing.T) {
		if d := DenyForHookError("docs/prds/widget/000-base.md", errString("hook crashed")); d.Allow {
			t.Fatal("a hook-runtime error was allowed — the fast layer must fail closed")
		}
		a, ok := Lookup("claude")
		if !ok {
			t.Fatal("claude adapter not registered")
		}
		var buf bytes.Buffer
		a.Evaluate(dcfg, []byte("{ not json"), widgetFirstPR(), &buf)
		dec := decodeClaudeDecision(t, buf.Bytes())
		if dec.HookSpecificOutput.PermissionDecision != "deny" {
			t.Fatalf("a forced hook-runtime error on a honoring CLI did not deny: %q", buf.String())
		}
		if !strings.Contains(dec.HookSpecificOutput.PermissionDecisionReason, HookErrorBoundary) {
			t.Errorf("hook-error deny reason = %q, want it to name %q", dec.HookSpecificOutput.PermissionDecisionReason, HookErrorBoundary)
		}
	})

	// The split, stated: this layer is fast, not unbypassable. An out-of-band
	// write (a raw shell redirect, a non-honoring CLI) skips it, which is exactly
	// why the hardened attribution gate is the unbypassable merge-boundary
	// backstop. Pin that the policy version the fast layer reports matches the
	// shared boundary data every layer keys off — one predicate, one place.
	if got := Decide(dcfg, "docs/product-vision.md", widgetFirstPR()).Policy; got != boundary.DataVersion {
		t.Errorf("fast-layer policy = %q, want the shared %q", got, boundary.DataVersion)
	}
}
