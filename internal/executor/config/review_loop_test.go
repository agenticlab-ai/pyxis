package config_test

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
)

// The pre-publish review loop's config half: a configured review block runs
// the loop by default with two remediation rounds; both are data an operator
// overrides; the grammar stays closed.
func TestReviewLoopKeys(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  review:\n    backend: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReviewPrePublish() || cfg.ReviewRemediationRounds() != 3 {
		t.Fatalf("defaults = pre_publish %v rounds %d, want true/3", cfg.ReviewPrePublish(), cfg.ReviewRemediationRounds())
	}

	cfg, err = config.Parse([]byte("executor:\n  review:\n    backend: claude\n    remediation_rounds: 4\n    pre_publish: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewPrePublish() || cfg.ReviewRemediationRounds() != 4 {
		t.Fatalf("explicit = pre_publish %v rounds %d, want false/4", cfg.ReviewPrePublish(), cfg.ReviewRemediationRounds())
	}

	cfg, err = config.Parse([]byte("executor:\n  review:\n    pre_publish: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReviewPrePublish() {
		t.Fatal("pre_publish: true on an inherited-backend block must enable the loop")
	}

	if _, err := config.Parse([]byte("executor:\n  review:\n    pre_publish: yes\n")); !errors.Is(err, config.ErrMalformed) {
		t.Errorf("pre_publish accepts only true/false: got %v", err)
	}
	if _, err := config.Parse([]byte("executor:\n  review:\n    remediation_rounds: 0\n")); !errors.Is(err, config.ErrInvalidCap) {
		t.Errorf("remediation_rounds must be a positive integer: got %v", err)
	}

	cfg, err = config.Parse([]byte("executor:\n  routing:\n    default: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewPrePublish() {
		t.Fatal("an absent review block must leave the loop uninstalled")
	}
}

// review.feedback_trusted is the operator allowlist of code-host logins whose
// proposal comments may wake a feedback round; empty by default.
func TestReviewFeedbackTrustedParses(t *testing.T) {
	cfg, err := config.Parse([]byte("executor:\n  review:\n    feedback_trusted: alice, Bob-2\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.FeedbackTrusted()
	if len(got) != 2 || got[0] != "alice" || got[1] != "Bob-2" {
		t.Fatalf("feedback_trusted = %v", got)
	}
	got[0] = "mutated"
	if cfg.FeedbackTrusted()[0] != "alice" {
		t.Fatal("FeedbackTrusted must return a copy")
	}
	def, err := config.Parse([]byte("executor:\n  review:\n    backend: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(def.FeedbackTrusted()) != 0 {
		t.Fatal("the allowlist must default empty")
	}
}
