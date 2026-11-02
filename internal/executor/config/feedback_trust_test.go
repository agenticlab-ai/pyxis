package config_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
)

func TestFeedbackTrustIndependentOfReviewInstallation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        []string
		review      bool
	}{
		{name: "independent trust", input: "executor:\n  feedback_trusted: review-app[bot]\n", want: []string{"review-app[bot]"}},
		{name: "trim comma separated logins", input: "executor:\n  feedback_trusted: alice, , Bob-2,\n", want: []string{"alice", "Bob-2"}},
		{name: "empty trust", input: "executor:\n  feedback_trusted:\n"},
		{name: "legacy trust", input: "executor:\n  review:\n    feedback_trusted: local-reviewer\n", want: []string{"local-reviewer"}, review: true},
		{name: "legacy override after independent trust", input: "executor:\n  feedback_trusted: shared-reviewer\n  review:\n    feedback_trusted: local-reviewer\n", want: []string{"local-reviewer"}, review: true},
		{name: "legacy override before independent trust", input: "executor:\n  review:\n    feedback_trusted: local-reviewer\n  feedback_trusted: shared-reviewer\n", want: []string{"local-reviewer"}, review: true},
		{name: "empty legacy clears independent trust", input: "executor:\n  review:\n    feedback_trusted:\n  feedback_trusted: shared-reviewer\n", review: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := config.Parse([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.FeedbackTrusted(), tc.want) {
				t.Fatalf("feedback trust = %v, want %v", cfg.FeedbackTrusted(), tc.want)
			}
			if cfg.ReviewConfigured() != tc.review || cfg.ReviewPrePublish() != tc.review {
				t.Fatalf("review installed = %v, pre-publish = %v, want %v", cfg.ReviewConfigured(), cfg.ReviewPrePublish(), tc.review)
			}
		})
	}
}

func TestFeedbackTrustLocalOverridesShared(t *testing.T) {
	for _, tc := range []struct {
		name, local string
		review      bool
	}{
		{name: "independent local trust", local: "executor:\n  feedback_trusted: local-reviewer\n"},
		{name: "legacy local trust", local: "executor:\n  review:\n    feedback_trusted: local-reviewer\n", review: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, ".pyxis")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("executor:\n  feedback_trusted: shared-reviewer\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte(tc.local), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(cfg.FeedbackTrusted(), []string{"local-reviewer"}) || cfg.ReviewConfigured() != tc.review {
				t.Fatalf("effective trust = %v, review installed = %v", cfg.FeedbackTrusted(), cfg.ReviewConfigured())
			}
		})
	}
}
