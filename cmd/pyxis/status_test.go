package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/watch"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func TestStatusVerbRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing required", []string{}, "--plan and --repo-slug are required"},
		{"unknown flag", []string{"--unknown"}, "flag provided but not defined"},
		{"flag after positional", []string{"--plan", "docs/plans/x", "--repo-slug", "o/r", "stray", "--late"}, "flags precede positionals"},
		{"unexpected positional", []string{"--plan", "docs/plans/x", "--repo-slug", "o/r", "stray"}, "unexpected positional"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errB bytes.Buffer
			if got := statusVerb(tc.args, &out, &errB); got != statusUsage {
				t.Fatalf("exit = %d, want %d (stderr: %s)", got, statusUsage, errB.String())
			}
			if !strings.Contains(errB.String(), tc.want) {
				t.Fatalf("stderr %q missing %q", errB.String(), tc.want)
			}
		})
	}
}

func TestStatusVerbRendersCleanClosedCarinaPlan(t *testing.T) {
	repo := t.TempDir()
	writePlanJSON(t, repo)
	store := localfs.New(repo, func() time.Time { return time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC) })
	item, defects := store.Create(context.Background(), model.CreateSpec{Title: "[T1] done"})
	if len(defects) != 0 {
		t.Fatalf("create item: %v", defects)
	}
	if _, defects := store.Transition(context.Background(), item.ID, model.StateClosed, model.CloseCompleted, "test"); len(defects) != 0 {
		t.Fatalf("close item: %v", defects)
	}
	report := fakeReport(t, repo, fmt.Sprintf(`{"task_id":"T1","issue":%d,"status":"existing"}`, item.ID))
	restore := listClaims
	defer func() { listClaims = restore }()
	listClaims = func(context.Context, string, string) (map[int]bool, error) { return map[int]bool{}, nil }
	// Repository-wide atomic coverage instrumentation makes this integration
	// test slower than the production one-second advice cadence. Keep the
	// fixture's check deterministically live instead of racing wall time.
	watch.NewSink(filepath.Join(repo, watch.DefaultEventsDir), 0, nil).Append(watch.Event{Time: time.Now().Add(time.Minute), Kind: watch.EventCheckRan})

	var stdout, stderr bytes.Buffer
	code := statusVerb([]string{
		"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake",
		"--tracker", "carina", "--carina-root", repo, "--report-cmd", report,
	}, &stdout, &stderr)
	if code != statusClean {
		t.Fatalf("exit=%d, want clean; stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
}

func TestStatusVerbConfiguredReviewFailsClosedOnUnreadableEvidence(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	writePlanJSON(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte("executor:\n  review:\n    backend: codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := localfs.New(repo, time.Now)
	item, defects := store.Create(context.Background(), model.CreateSpec{Title: "[T1] pending review"})
	if len(defects) != 0 {
		t.Fatalf("create item: %v", defects)
	}
	report := fakeReport(t, repo, fmt.Sprintf(`{"task_id":"T1","issue":%d,"status":"existing"}`, item.ID))
	restore := listClaims
	defer func() { listClaims = restore }()
	listClaims = func(context.Context, string, string) (map[int]bool, error) { return map[int]bool{}, nil }

	var stdout, stderr bytes.Buffer
	code := statusVerb([]string{
		"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake",
		"--tracker", "carina", "--carina-root", repo, "--report-cmd", report,
	}, &stdout, &stderr)
	if code != statusDegraded {
		t.Fatalf("exit=%d, want rendered-degraded; stderr=%s output=%s", code, stderr.String(), stdout.String())
	}
}

// TestStatusVerbCannotBuild proves the verb is REACHABLE (the T9 gap this PR
// closes: before it, `pyxis status` was an unknown verb) and that a failing
// plan-report is a cannot-render fault (exit 1), distinct from a usage
// refusal (exit 2). It goes through the real verb wiring — the shipped
// surface, not the render function in isolation (the whole point of T9's
// FR-7 "when the operator runs the status verb").
func TestStatusVerbCannotBuild(t *testing.T) {
	var out, errB bytes.Buffer
	code := statusVerb([]string{
		"--repo", t.TempDir(), "--repo-slug", "agenticlab-ai/pyxis",
		"--plan", "docs/plans/self-drive-wake",
		"--report-cmd", "/bin/sh -c exit\\ 7", // a report command that fails
	}, &out, &errB)
	if code != statusCannotBuild {
		t.Fatalf("exit = %d, want %d (cannot-render); stderr: %s", code, statusCannotBuild, errB.String())
	}
	if !strings.Contains(errB.String(), "status:") {
		t.Fatalf("cannot-build must name the fault: %s", errB.String())
	}
}

func TestStatusVerbRejectsMalformedPoliciesBeforeReadingThePlan(t *testing.T) {
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{name: "wake thresholds", config: "wake:\n  bogus: yes\n", want: "unknown key under wake"},
		{name: "executor", config: "executor:\n  bogus: yes\n", want: "load executor config"},
		{name: "tracker", config: "system:\n  mode: source\n", args: []string{"--tracker", "future"}, want: "unknown tracker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.MkdirAll(filepath.Join(repo, ".pyxis"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, ".pyxis", "config.yaml"), []byte(tt.config), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/feature"}
			args = append(args, tt.args...)
			var stdout, stderr bytes.Buffer
			if got := statusVerb(args, &stdout, &stderr); got != statusUsage || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("exit=%d stderr=%q, want usage containing %q", got, stderr.String(), tt.want)
			}
		})
	}
}

func TestStatusVerbRendersCarinaPlanState(t *testing.T) {
	repo := t.TempDir()
	writePlanJSON(t, repo)
	store := localfs.New(repo, func() time.Time { return time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC) })
	item, defects := store.Create(context.Background(), model.CreateSpec{Title: "[T1] ready"})
	if len(defects) != 0 {
		t.Fatalf("create item: %v", defects)
	}
	report := fakeReport(t, repo, fmt.Sprintf(`{"task_id":"T1","issue":%d,"status":"existing"}`, item.ID))
	restore := listClaims
	defer func() { listClaims = restore }()
	listClaims = func(context.Context, string, string) (map[int]bool, error) { return map[int]bool{}, nil }

	var stdout, stderr bytes.Buffer
	code := statusVerb([]string{
		"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake",
		"--tracker", "carina", "--carina-root", repo, "--report-cmd", report,
	}, &stdout, &stderr)
	if code != statusDegraded {
		t.Fatalf("exit=%d, want rendered-degraded %d; stderr=%s output=%s", code, statusDegraded, stderr.String(), stdout.String())
	}
	for _, want := range []string{"T1", "ready-unclaimed", "checker"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("status output missing %q: %s", want, stdout.String())
		}
	}
}
