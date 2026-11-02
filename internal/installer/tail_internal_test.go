package installer

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// TestWelcomeNextStepsRendersConfiguredGoverningPaths proves the terminal
// epilogue names the CONFIGURED governing-doc locations, not baked-in
// literals (config-driven-portability-v1 §3.2, FR-1/FR-3; DINV-1) — so an
// adopter whose vision/principles live elsewhere is told their real
// locations.
func TestWelcomeNextStepsRendersConfiguredGoverningPaths(t *testing.T) {
	t.Run("default config names the default paths", func(t *testing.T) {
		steps := welcomeNextSteps(config.Knobs{Tracker: "carina"}, config.Defaults(), "", nil)
		joined := strings.Join(steps, "\n")
		if !strings.Contains(joined, "yours to author: docs/product-vision.md · docs/architecture-principles.md") {
			t.Errorf("default epilogue does not name both governing docs:\n%s", joined)
		}
	})

	t.Run("relocated config renders the configured locations", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.ProductVision = "governance/vision.md"
		cfg.ArchitecturePrinciples = "governance/principles.md"
		steps := welcomeNextSteps(config.Knobs{Tracker: "carina"}, cfg, "", nil)
		joined := strings.Join(steps, "\n")
		for _, want := range []string{"governance/vision.md", "governance/principles.md"} {
			if !strings.Contains(joined, want) {
				t.Errorf("relocated epilogue missing configured path %q:\n%s", want, joined)
			}
		}
		// The stale defaults must NOT appear anywhere in the epilogue (guard
		// against a residual literal).
		for _, stale := range []string{"docs/product-vision.md", "docs/architecture-principles.md"} {
			if strings.Contains(joined, stale) {
				t.Errorf("relocated epilogue still names the stale governing-doc literal %q:\n%s", stale, joined)
			}
		}
	})
}

// TestGatesActiveLineNamesResolvedCIMode proves the T8 truth-surface
// acceptance criterion (prd:init-hardening/FR-14, design init-hardening-v1
// §3.9) across both registers (#937): the terminal epilogue's terse gates
// row and the welcome item's prose sentence each state which gates are
// ACTIVE worded to the resolved ci_mode knob (local | actions | both), and
// the epilogue states the code gate's PENDING status naming its enabling
// seed command regardless of ci_mode — a fresh install never carries a
// verification manifest, so that line is knob-independent.
func TestGatesActiveLineNamesResolvedCIMode(t *testing.T) {
	cases := []struct {
		ciMode       string
		wantLine     string
		wantSentence string
	}{
		{
			"local",
			"gates: docs gate live at your desk (git hooks) — hosted CI off (ci_mode=local)",
			"Active gate: the docs gate at your desk (git hooks) — hosted CI is off (ci_mode=local).",
		},
		{
			"actions",
			"gates: docs gate live at your desk (git hooks) + hosted CI (ci_mode=actions)",
			"Active gates: the docs gate at your desk (git hooks) and in hosted CI (ci_mode=actions).",
		},
		{
			"both",
			"gates: docs gate live at your desk (git hooks) + hosted CI (ci_mode=both)",
			"Active gates: the docs gate at your desk (git hooks) and in hosted CI (ci_mode=both).",
		},
	}
	for _, c := range cases {
		t.Run(c.ciMode, func(t *testing.T) {
			if got := gatesActiveLine(c.ciMode); got != c.wantLine {
				t.Errorf("gatesActiveLine(%q) = %q, want %q", c.ciMode, got, c.wantLine)
			}
			if got := gatesActiveSentence(c.ciMode); got != c.wantSentence {
				t.Errorf("gatesActiveSentence(%q) = %q, want %q", c.ciMode, got, c.wantSentence)
			}

			steps := welcomeNextSteps(config.Knobs{Tracker: "carina", CIMode: c.ciMode}, config.Defaults(), "", nil)
			joined := strings.Join(steps, "\n")
			if !strings.Contains(joined, c.wantLine) {
				t.Errorf("epilogue (ci_mode=%s) missing the resolved gates row:\n%s", c.ciMode, joined)
			}
			if !strings.Contains(joined, "pending: the code gate") || !strings.Contains(joined, "pyxis exec codegate -- seed") {
				t.Errorf("epilogue (ci_mode=%s) does not state the code gate's pending status naming the seed command:\n%s", c.ciMode, joined)
			}
		})
	}

	// welcomeVars (the tracker work-item body's substitution source) must
	// resolve gates_active to the prose sentence — the item body flows as
	// sentences — while deriving from the same knob, so the two surfaces can
	// never disagree about which gates are active.
	for _, c := range cases {
		vars := welcomeVars(config.Knobs{Tracker: "github", CIMode: c.ciMode}, config.Defaults())
		if vars["gates_active"] != c.wantSentence {
			t.Errorf("welcomeVars(ci_mode=%s)[gates_active] = %q, want %q", c.ciMode, vars["gates_active"], c.wantSentence)
		}
	}
}

// TestOnboardingWorkItemsRenderConfiguredPaths proves the standing set
// (#947) names the CONFIGURED governing-doc locations — an adopter's
// relocated docs appear at their real paths — and that the slugs (the
// idempotency keys) and the seed command are stable.
func TestOnboardingWorkItemsRenderConfiguredPaths(t *testing.T) {
	items := OnboardingWorkItems(config.Defaults())
	if len(items) != 3 {
		t.Fatalf("standing set = %d items, want 3", len(items))
	}
	wantSlugs := []string{"author-product-vision", "author-architecture-principles", "seed-the-code-gate"}
	for i, want := range wantSlugs {
		if items[i].Slug != want {
			t.Errorf("item %d slug = %q, want %q", i, items[i].Slug, want)
		}
	}
	if !strings.Contains(items[0].Body, "docs/product-vision.md") {
		t.Errorf("vision item does not name the default path:\n%s", items[0].Body)
	}
	if !strings.Contains(items[2].Body, "pyxis exec codegate -- seed") {
		t.Errorf("seed item does not carry the seed command:\n%s", items[2].Body)
	}

	cfg := config.Defaults()
	cfg.ProductVision = "governance/vision.md"
	relocated := OnboardingWorkItems(cfg)
	if !strings.Contains(relocated[0].Body, "governance/vision.md") || strings.Contains(relocated[0].Body, "docs/product-vision.md") {
		t.Errorf("relocated vision item still names the default path:\n%s", relocated[0].Body)
	}
}

// TestWelcomeNextStepsLinksAndSendsOff proves the #948 additions: a known
// guide ref renders as the full-guide line verbatim (the user clicks, not
// hunts), the filed onboarding set renders as a short-ref todo line, and
// the epilogue always ends on the send-off.
func TestWelcomeNextStepsLinksAndSendsOff(t *testing.T) {
	onb := &OnboardingResult{Refs: []string{
		"https://github.com/octo/widget/issues/2",
		"https://github.com/octo/widget/issues/3",
		"#4",
	}}
	steps := welcomeNextSteps(config.Knobs{Tracker: "github"}, config.Defaults(), "https://github.com/octo/widget/issues/1", onb)
	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "full guide: https://github.com/octo/widget/issues/1") {
		t.Errorf("guide line does not carry the ref:\n%s", joined)
	}
	if !strings.Contains(joined, "your todo list: 3 onboarding item(s) in the tracker (#2 · #3 · #4)") {
		t.Errorf("todo line missing or refs not shortened:\n%s", joined)
	}
	if last := steps[len(steps)-1]; !strings.Contains(last, "one /feature-brainstorm away") {
		t.Errorf("the epilogue does not end on the send-off: %q", last)
	}

	// No ref, no items: the fallback guide line and no todo line.
	bare := strings.Join(welcomeNextSteps(config.Knobs{Tracker: "github"}, config.Defaults(), "", nil), "\n")
	if !strings.Contains(bare, "full guide: the welcome issue in your GitHub tracker") {
		t.Errorf("bare epilogue lost the fallback guide line:\n%s", bare)
	}
	if strings.Contains(bare, "todo list") {
		t.Errorf("bare epilogue renders a todo line with nothing filed:\n%s", bare)
	}
}
