package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

func TestReviewRearmUsageAndConfigurationRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
		text string
	}{
		{name: "missing", want: statusUsage, text: "are required"},
		{name: "bad flag", args: []string{"--unknown"}, want: statusUsage, text: "flag provided"},
		{name: "positional", args: []string{"--scope", "demo/001", "--obligation", "o-1", "--principal", "operator", "--reason", "safe", "extra"}, want: statusUsage, text: "positional arguments"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := reviewRearmVerb(tc.args, &stdout, &stderr); got != tc.want || !strings.Contains(stderr.String(), tc.text) {
				t.Fatalf("reviewRearmVerb = %d stdout=%q stderr=%q", got, stdout.String(), stderr.String())
			}
		})
	}
}

func TestReviewRearmFailsClosedAtEachConfigurationBoundary(t *testing.T) {
	validArgs := func(root string) []string {
		return []string{"--repo", root, "--scope", "demo/001", "--obligation", "o-1", "--principal", "operator:alice", "--reason", "verified recovery"}
	}
	for _, tc := range []struct {
		name string
		body string
		want int
		text string
	}{
		{name: "executor malformed", body: "executor:\n  unknown: true\n", want: statusUsage, text: "load executor config"},
		{name: "review absent", body: "executor:\n  routing:\n    default: codex\n", want: statusCannotBuild, text: "executor.review is not configured"},
		{name: "wake malformed", body: "executor:\n  review:\n    backend: codex\nwake:\n  unknown: true\n", want: statusUsage, text: "load wake config"},
		{name: "repo malformed", body: "executor:\n  review:\n    backend: codex\nunknown_top_level: true\n", want: statusCannotBuild, text: "load repo config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reviewRearmRepo(t, tc.body)
			var stdout, stderr bytes.Buffer
			if got := reviewRearmVerb(validArgs(root), &stdout, &stderr); got != tc.want || !strings.Contains(stderr.String(), tc.text) {
				t.Fatalf("reviewRearmVerb = %d stdout=%q stderr=%q", got, stdout.String(), stderr.String())
			}
		})
	}
}

func TestReviewRearmSurfacesUnavailableEvidenceStore(t *testing.T) {
	root := reviewRearmRepo(t, "executor:\n  review:\n    backend: codex\n")
	var stdout, stderr bytes.Buffer
	code := reviewRearmVerb([]string{"--repo", root, "--scope", "demo/001", "--obligation", "o-1", "--principal", "operator:alice", "--reason", "verified recovery"}, &stdout, &stderr)
	if code != statusCannotBuild || !strings.Contains(stderr.String(), "review-rearm:") {
		t.Fatalf("reviewRearmVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestReviewRearmSurfacesEachOperationalBoundary(t *testing.T) {
	root := reviewRearmRepo(t, "executor:\n  review:\n    backend: codex\n")
	args := []string{"--repo", root, "--scope", "demo/001", "--obligation", "o-1", "--principal", "operator:alice", "--reason", "verified recovery"}
	wantErr := errors.New("synthetic boundary failure")

	for _, failAt := range []string{"store", "ledger", "round", "rearm", "success"} {
		t.Run(failAt, func(t *testing.T) {
			oldStore, oldLedger := newReviewRearmStore, newReviewRearmLedger
			oldRound, oldRearm, oldSnapshot := newReviewRearmRound, performReviewRearm, snapshotReviewRearm
			t.Cleanup(func() {
				newReviewRearmStore, newReviewRearmLedger = oldStore, oldLedger
				newReviewRearmRound, performReviewRearm = oldRound, oldRearm
				snapshotReviewRearm = oldSnapshot
			})
			newReviewRearmStore = func(string, string, string) (*reviewobligation.GitStore, error) {
				if failAt == "store" {
					return nil, wantErr
				}
				return new(reviewobligation.GitStore), nil
			}
			newReviewRearmLedger = func(string, string, string) (*watch.GitRecoveryLedger, error) {
				if failAt == "ledger" {
					return nil, wantErr
				}
				return new(watch.GitRecoveryLedger), nil
			}
			newReviewRearmRound = func() (string, error) {
				if failAt == "round" {
					return "", wantErr
				}
				return "round-1", nil
			}
			performReviewRearm = func(watch.ReviewStateReader, context.Context, string, string, string, string, watch.RecoveryPolicy) (string, error) {
				if failAt == "rearm" {
					return "", wantErr
				}
				return "evidence-1", nil
			}
			snapshotReviewRearm = func(watch.ReviewStateReader, context.Context, string) (watch.ReviewWakeState, error) {
				return watch.ReviewWakeState{Status: watch.ReviewStateUnknown}, nil
			}

			var stdout, stderr bytes.Buffer
			code := reviewRearmVerb(args, &stdout, &stderr)
			if failAt == "success" {
				if code != statusClean || !strings.Contains(stdout.String(), "evidence-1") {
					t.Fatalf("success exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
				return
			}
			if code != statusCannotBuild || !strings.Contains(stderr.String(), wantErr.Error()) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func reviewRearmRepo(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
