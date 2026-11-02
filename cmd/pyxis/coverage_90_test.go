package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/surfaces"
	"github.com/agenticlab-ai/pyxis/internal/upgrade"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

func TestCoverage90PrintInitPreviewDetails(t *testing.T) {
	report := installer.Report{
		RunID:        "run-1",
		Preview:      true,
		WouldGitInit: true,
		Branch:       "main",
		ConfigPath:   ".pyxis/config.yaml",
		Converge: converge.Report{
			WouldWrite: []converge.Action{{Path: "generated.txt"}},
			WouldFetch: []converge.Fetch{{Artifact: "payload", URL: "https://example.invalid/payload"}},
			Incomplete: []string{"manifest unavailable"},
		},
		Stages: []string{"admission", "converge"},
	}
	var out bytes.Buffer
	printInitReport(&out, report)
	for _, want := range []string{"git init", "write .pyxis/config.yaml", "converge generated.txt", "fetch payload", "incomplete: manifest unavailable", "nothing was performed"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("preview missing %q:\n%s", want, out.String())
		}
	}

	report.WouldGitInit = false
	out.Reset()
	printInitReport(&out, report)
	if !strings.Contains(out.String(), "adopt the empty repository") {
		t.Fatalf("adopt preview missing: %s", out.String())
	}
}

func TestCoverage90SmallRenderBranches(t *testing.T) {
	if got := upgradeArrow(upgrade.Report{Downgrade: true}); !strings.Contains(got, "downgrade") {
		t.Fatalf("upgradeArrow = %q", got)
	}
	if got := short("tiny"); got != "tiny" {
		t.Fatalf("short = %q", got)
	}
	var out bytes.Buffer
	printRollbackReport(&out, installer.RollbackReport{FailedStage: "converge", Restored: false, Errors: []string{"restore failed"}})
	if !strings.Contains(out.String(), "restore failed") {
		t.Fatalf("rollback output = %q", out.String())
	}
}

func TestCoverage90PlanMappingFailures(t *testing.T) {
	ctx := context.Background()
	planDir := "docs/plans/self-drive-wake"

	t.Run("report command", func(t *testing.T) {
		repo := t.TempDir()
		script := coverage90Script(t, repo, "", 1)
		if _, err := planMapping(ctx, repo, planDir, "gh", "", script); err == nil || !strings.Contains(err.Error(), "plan report") {
			t.Fatalf("planMapping error = %v", err)
		}
	})

	t.Run("report JSON", func(t *testing.T) {
		repo := t.TempDir()
		script := coverage90Script(t, repo, "not json", 0)
		if _, err := planMapping(ctx, repo, planDir, "gh", "", script); err == nil || !strings.Contains(err.Error(), "parse plan report") {
			t.Fatalf("planMapping error = %v", err)
		}
	})

	t.Run("missing plan document", func(t *testing.T) {
		repo := t.TempDir()
		script := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
		if _, err := planMapping(ctx, repo, planDir, "carina", repo, script); err == nil || !strings.Contains(err.Error(), "read plan json") {
			t.Fatalf("planMapping error = %v", err)
		}
	})

	t.Run("malformed plan document", func(t *testing.T) {
		repo := t.TempDir()
		dir := filepath.Join(repo, planDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "001-plan.json"), []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		script := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
		if _, err := planMapping(ctx, repo, planDir, "gh", "", script); err == nil || !strings.Contains(err.Error(), "parse plan json") {
			t.Fatalf("planMapping error = %v", err)
		}
	})

	t.Run("empty report", func(t *testing.T) {
		repo := t.TempDir()
		writePlanJSON(t, repo)
		script := fakeReport(t, repo, "")
		if _, err := planMapping(ctx, repo, planDir, "gh", "", script); err == nil || !strings.Contains(err.Error(), "mapping empty") {
			t.Fatalf("planMapping error = %v", err)
		}
	})
}

func TestCoverage90WatchConfigurationFailures(t *testing.T) {
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{name: "executor config", config: "executor:\n  unknown: true\n", want: "load executor config"},
		{name: "wake thresholds", config: "wake:\n  unknown: true\n", want: "unknown key"},
		{name: "tracker", args: []string{"--tracker", "unknown"}, want: "unknown tracker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := reviewRearmRepo(t, tt.config)
			args := []string{"--repo", root, "--repo-slug", "owner/repo", "--plan", "docs/plans/demo"}
			args = append(args, tt.args...)
			var stdout, stderr bytes.Buffer
			if code := watchVerb(args, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("watchVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCoverage90WatchSelectsCarinaRuntime(t *testing.T) {
	repo := reviewRearmRepo(t, "")
	writePlanJSON(t, repo)
	report := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
	oldRun := watchRunLive
	t.Cleanup(func() { watchRunLive = oldRun })
	watchRunLive = func(context.Context, watch.PlanSource, watch.Reader, watch.Sweep, watch.Clock, <-chan struct{}) (watch.Summary, error) {
		return watch.Summary{Completed: true}, nil
	}
	var stdout, stderr bytes.Buffer
	code := watchVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--tracker", "carina", "--carina-root", repo, "--report-cmd", report, "--sweep-cmd", "true"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "backend carina") {
		t.Fatalf("watchVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCoverage90StatusSelectsGitHubAndReportsReadFailure(t *testing.T) {
	repo := reviewRearmRepo(t, "")
	writePlanJSON(t, repo)
	report := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := statusVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--tracker", "gh", "--report-cmd", report}, &stdout, &stderr)
	if code != statusCannotBuild || !strings.Contains(stderr.String(), "status:") {
		t.Fatalf("statusVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCoverage90StatusWiringFailures(t *testing.T) {
	oldBuild := statusBuildReport
	oldStore, oldLedger := statusNewReviewStore, statusNewRecoveryLedger
	t.Cleanup(func() {
		statusBuildReport = oldBuild
		statusNewReviewStore, statusNewRecoveryLedger = oldStore, oldLedger
	})

	t.Run("github report build", func(t *testing.T) {
		repo := reviewRearmRepo(t, "")
		writePlanJSON(t, repo)
		report := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
		statusBuildReport = func(context.Context, watch.PlanSource, port.ProposalStateReader, *watch.Sink, watch.Thresholds, time.Duration, time.Time) (watch.Report, error) {
			return watch.Report{}, errors.New("build failed")
		}
		var stdout, stderr bytes.Buffer
		code := statusVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--tracker", "gh", "--report-cmd", report}, &stdout, &stderr)
		if code != statusCannotBuild || !strings.Contains(stderr.String(), "build failed") {
			t.Fatalf("statusVerb = %d stderr=%q", code, stderr.String())
		}
	})

	for _, tc := range []struct {
		name       string
		storeError bool
		want       string
	}{
		{name: "review store", storeError: true, want: "review obligation store"},
		{name: "recovery ledger", want: "review recovery ledger"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := reviewRearmRepo(t, "executor:\n  review:\n    backend: codex\n")
			writePlanJSON(t, repo)
			report := fakeReport(t, repo, `{"task_id":"T1","issue":1}`)
			statusNewReviewStore = func(string, string, string) (*reviewobligation.GitStore, error) {
				if tc.storeError {
					return nil, errors.New("store failed")
				}
				return new(reviewobligation.GitStore), nil
			}
			statusNewRecoveryLedger = func(string, string, string) (*watch.GitRecoveryLedger, error) {
				return nil, errors.New("ledger failed")
			}
			var stdout, stderr bytes.Buffer
			code := statusVerb([]string{"--repo", repo, "--repo-slug", "owner/repo", "--plan", "docs/plans/self-drive-wake", "--tracker", "carina", "--carina-root", repo, "--report-cmd", report}, &stdout, &stderr)
			if code != statusCannotBuild || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("statusVerb = %d stderr=%q", code, stderr.String())
			}
		})
	}
}

func TestCoverage90SurfacesWiringFailuresAndInteractiveOutcomes(t *testing.T) {
	oldRoot, oldInteractive, oldAsk := surfacesRepoRoot, surfacesInteractive, surfacesAsk
	t.Cleanup(func() {
		surfacesRepoRoot, surfacesInteractive, surfacesAsk = oldRoot, oldInteractive, oldAsk
	})

	t.Run("repository root", func(t *testing.T) {
		surfacesRepoRoot = func() (string, error) { return "", errors.New("root failed") }
		var stdout, stderr bytes.Buffer
		if code := surfacesVerb(nil, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "root failed") {
			t.Fatalf("surfacesVerb = %d stderr=%q", code, stderr.String())
		}
	})

	for _, tc := range []struct {
		name     string
		want     []string
		answered bool
		err      error
		code     int
		text     string
	}{
		{name: "ask failure", err: errors.New("ask failed"), code: 1, text: "ask failed"},
		{name: "closed input", answered: false, code: 0, text: "nothing was performed"},
		{name: "declined", answered: true, want: nil, code: 0, text: "nothing selected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := surfacesRepo(t, "")
			surfacesRepoRoot = func() (string, error) { return root, nil }
			surfacesInteractive = func() bool { return true }
			surfacesAsk = func(io.Writer, io.Reader, surfaces.Status) ([]string, bool, error) {
				return tc.want, tc.answered, tc.err
			}
			var stdout, stderr bytes.Buffer
			code := surfacesVerb(nil, &stdout, &stderr)
			if code != tc.code || !strings.Contains(stdout.String()+stderr.String(), tc.text) {
				t.Fatalf("surfacesVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCoverage90ReviewRearmRecordsKnownSnapshot(t *testing.T) {
	root := reviewRearmRepo(t, "executor:\n  review:\n    backend: codex\n")
	oldStore, oldLedger := newReviewRearmStore, newReviewRearmLedger
	oldRound, oldRearm, oldSnapshot := newReviewRearmRound, performReviewRearm, snapshotReviewRearm
	t.Cleanup(func() {
		newReviewRearmStore, newReviewRearmLedger = oldStore, oldLedger
		newReviewRearmRound, performReviewRearm, snapshotReviewRearm = oldRound, oldRearm, oldSnapshot
	})
	newReviewRearmStore = func(string, string, string) (*reviewobligation.GitStore, error) {
		return new(reviewobligation.GitStore), nil
	}
	newReviewRearmLedger = func(string, string, string) (*watch.GitRecoveryLedger, error) {
		return new(watch.GitRecoveryLedger), nil
	}
	newReviewRearmRound = func() (string, error) { return "round-1", nil }
	performReviewRearm = func(watch.ReviewStateReader, context.Context, string, string, string, string, watch.RecoveryPolicy) (string, error) {
		return "evidence-1", nil
	}
	snapshotReviewRearm = func(watch.ReviewStateReader, context.Context, string) (watch.ReviewWakeState, error) {
		return watch.ReviewWakeState{Status: watch.ReviewStateKnown, Rows: []watch.ReviewWakeRow{
			{ReviewObligationState: watch.ReviewObligationState{ID: "other"}},
			{ReviewObligationState: watch.ReviewObligationState{ID: "o-1", TaskID: "T1", ProposalRef: "1", Revision: "abc", AttemptCount: 2}, RecoveryEpoch: "epoch-1", RecoveryCount: 1, RecoveryBound: 3},
		}}, nil
	}
	var stdout, stderr bytes.Buffer
	code := reviewRearmVerb([]string{"--repo", root, "--scope", "demo/001", "--obligation", "o-1", "--principal", "operator", "--reason", "safe"}, &stdout, &stderr)
	if code != statusClean {
		t.Fatalf("reviewRearmVerb = %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(root, watch.DefaultEventsDir, "current.jsonl"))
	if err != nil || !strings.Contains(string(data), "review_recovery_decision") {
		t.Fatalf("recovery event: %q, %v", data, err)
	}
}

func coverage90Script(t *testing.T, dir, output string, exit int) string {
	t.Helper()
	path := filepath.Join(dir, "coverage-90-command.sh")
	body := "#!/bin/sh\n"
	if output != "" {
		body += "printf '%s\\n' '" + output + "'\n"
	}
	body += "exit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
