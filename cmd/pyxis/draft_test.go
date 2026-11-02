package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	draftlock "github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

func TestDraftStatusAndOpenCLI(t *testing.T) {
	root := t.TempDir()
	oldRoot, oldInspect, oldOpen := draftRepoRoot, draftInspect, draftOpen
	t.Cleanup(func() { draftRepoRoot, draftInspect, draftOpen = oldRoot, oldInspect, oldOpen })
	draftRepoRoot = func() (string, error) { return root, nil }
	draftInspect = func(context.Context, string, config.Config, lifecycle.RunCommand) (lifecycle.Snapshot, error) {
		return lifecycle.Snapshot{
			OpenDrafts:     map[string]bool{"new": true, "reopened": true},
			LockedVersions: map[string]int{"locked": 2, "reopened": 1},
			PRDLedgers:     map[string]bool{"existing": true},
		}, nil
	}
	draftOpen = func(_ context.Context, gotRoot string, _ config.Config, feature string, _ lifecycle.RunCommand) (lifecycle.OpenResult, error) {
		if gotRoot != root || feature != "new" {
			t.Fatalf("open inputs %q %q", gotRoot, feature)
		}
		return lifecycle.OpenResult{Branch: "draft/new", PullRequest: "https://example.test/pr/1"}, nil
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"status"}, &out, &errout); code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "new: drafting first version (v1)") || !strings.Contains(out.String(), "reopened: drafting version 2 while version 1 binds") || !strings.Contains(out.String(), "locked: locked at version 2") || !strings.Contains(out.String(), "existing: document pipeline") {
		t.Fatalf("status: exit %d stdout %q stderr %q", code, out.String(), errout.String())
	}
	out.Reset()
	if code := draftVerb([]string{"status", "new"}, &out, &errout); code != 0 || out.String() != "new: drafting first version (v1)\n" {
		t.Fatalf("filtered status: exit %d stdout %q", code, out.String())
	}
	out.Reset()
	if code := draftVerb([]string{"open", "new"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "https://example.test/pr/1") {
		t.Fatalf("open: exit %d stdout %q stderr %q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{}, {"open"}, {"open", "new", "extra"}, {"status", "--bad"}, {"other"}} {
		out.Reset()
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || errout.Len() == 0 {
			t.Errorf("args %v: exit %d stdout %q stderr %q", args, code, out.String(), errout.String())
		}
	}
	other := filepath.Join(root, ".pyxis/config.yaml")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("draft:\n  branch_perfix: draft/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errout.Reset()
	if code := draftVerb([]string{"status"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "branch_perfix") {
		t.Fatalf("closed config: exit %d stderr %q", code, errout.String())
	}
}

func TestDraftPrepareAndRoundCLI(t *testing.T) {
	root := t.TempDir()
	oldRoot, oldPrepare, oldRound := draftRepoRoot, draftPrepare, draftRound
	t.Cleanup(func() { draftRepoRoot, draftPrepare, draftRound = oldRoot, oldPrepare, oldRound })
	draftRepoRoot = func() (string, error) { return root, nil }
	draftPrepare = func(_ context.Context, gotRoot string, _ config.Config, feature string, _ lifecycle.RunCommand, _ worktracker.Lister) (draftlock.PrepareResult, error) {
		if gotRoot != root || feature != "widget" {
			t.Fatalf("prepare inputs %q %q", gotRoot, feature)
		}
		return draftlock.PrepareResult{Head: "prepared", PullRequest: 17, Verdict: readiness.Verdict{DraftDigest: "sha256:abc"}}, nil
	}
	draftRound = func(_ context.Context, gotRoot string, _ config.Config, feature string, _ lifecycle.RunCommand) (draftlock.RoundResult, error) {
		if gotRoot != root || feature != "widget" {
			t.Fatalf("round inputs %q %q", gotRoot, feature)
		}
		return draftlock.RoundResult{Head: "restored", Digest: "sha256:abc"}, nil
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"prepare-lock", "widget"}, &out, &errout); code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "PR #17 at prepared") {
		t.Fatalf("prepare: exit %d stdout %q stderr %q", code, out.String(), errout.String())
	}
	out.Reset()
	if code := draftVerb([]string{"round", "widget"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "restored at restored") {
		t.Fatalf("round: exit %d stdout %q stderr %q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"prepare-lock"}, {"round"}, {"prepare-lock", "--bad"}, {"round", "widget", "extra"}} {
		out.Reset()
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || errout.Len() == 0 {
			t.Errorf("args %v: exit %d stdout %q stderr %q", args, code, out.String(), errout.String())
		}
	}
	draftPrepare = func(context.Context, string, config.Config, string, lifecycle.RunCommand, worktracker.Lister) (draftlock.PrepareResult, error) {
		return draftlock.PrepareResult{}, errors.New("readiness refused")
	}
	errout.Reset()
	if code := draftVerb([]string{"prepare-lock", "widget"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "readiness refused") {
		t.Fatalf("prepare failure: exit %d stderr %q", code, errout.String())
	}
}

func TestDraftReopenAndAbandonCLI(t *testing.T) {
	root := t.TempDir()
	oldRoot, oldReopen, oldAbandon := draftRepoRoot, draftReopen, draftAbandon
	t.Cleanup(func() { draftRepoRoot, draftReopen, draftAbandon = oldRoot, oldReopen, oldAbandon })
	draftRepoRoot = func() (string, error) { return root, nil }
	draftReopen = func(_ context.Context, gotRoot string, _ config.Config, feature, cause, causeRef string, reader lifecycle.LockedReader, run lifecycle.RunCommand, _ worktracker.Lister) (lifecycle.ReopenResult, error) {
		if gotRoot != root || feature != "widget" || cause != "other" || causeRef != "" || reader == nil || run == nil {
			t.Fatalf("reopen inputs root=%q feature=%q", gotRoot, feature)
		}
		return lifecycle.ReopenResult{Branch: "draft/widget-v2", PullRequest: "https://example.test/pr/42", Version: 2}, nil
	}
	draftAbandon = func(_ context.Context, gotRoot string, _ config.Config, feature, reason string, run lifecycle.RunCommand) (lifecycle.AbandonResult, error) {
		if gotRoot != root || feature != "widget" || reason != "withdrawn" || run == nil {
			t.Fatalf("abandon inputs root=%q feature=%q reason=%q", gotRoot, feature, reason)
		}
		return lifecycle.AbandonResult{Branch: "draft/widget", PullRequest: 42, Head: "abandoned"}, nil
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"reopen", "widget"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "drafting v2") || errout.Len() != 0 {
		t.Fatalf("reopen CLI: exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
	draftReopen = func(_ context.Context, _ string, _ config.Config, feature, cause, causeRef string, _ lifecycle.LockedReader, _ lifecycle.RunCommand, _ worktracker.Lister) (lifecycle.ReopenResult, error) {
		if feature != "widget" || cause != "dependency-change" || causeRef != "issue:42" {
			t.Fatalf("classified reopen: %s %s %s", feature, cause, causeRef)
		}
		return lifecycle.ReopenResult{Branch: "draft/widget-v2", Version: 2}, nil
	}
	out.Reset()
	if code := draftVerb([]string{"reopen", "widget", "--cause", "dependency-change", "--cause-ref", "issue:42"}, &out, &errout); code != 0 {
		t.Fatalf("classified reopen CLI: exit=%d stderr=%q", code, errout.String())
	}
	out.Reset()
	if code := draftVerb([]string{"abandon", "widget", "--reason", "withdrawn"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "operator review required") {
		t.Fatalf("abandon CLI: exit=%d stdout=%q stderr=%q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"reopen"}, {"abandon", "widget"}, {"abandon", "widget", "--bad", "text"}} {
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || errout.Len() == 0 {
			t.Fatalf("invalid args %v: exit=%d stderr=%q", args, code, errout.String())
		}
	}
	draftReopen = func(context.Context, string, config.Config, string, string, string, lifecycle.LockedReader, lifecycle.RunCommand, worktracker.Lister) (lifecycle.ReopenResult, error) {
		return lifecycle.ReopenResult{}, errors.New("unfinished tasks")
	}
	errout.Reset()
	if code := draftVerb([]string{"reopen", "widget"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "unfinished tasks") {
		t.Fatalf("reopen failure: exit=%d stderr=%q", code, errout.String())
	}
	draftAbandon = func(context.Context, string, config.Config, string, string, lifecycle.RunCommand) (lifecycle.AbandonResult, error) {
		return lifecycle.AbandonResult{}, errors.New("operator review setting missing")
	}
	errout.Reset()
	if code := draftVerb([]string{"abandon", "widget", "--reason", "withdrawn"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "operator review setting missing") {
		t.Fatalf("abandon failure: exit=%d stderr=%q", code, errout.String())
	}
}

func TestDraftCLINamedFailures(t *testing.T) {
	root := t.TempDir()
	oldRoot, oldInspect, oldOpen := draftRepoRoot, draftInspect, draftOpen
	t.Cleanup(func() { draftRepoRoot, draftInspect, draftOpen = oldRoot, oldInspect, oldOpen })
	draftRepoRoot = func() (string, error) { return root, nil }
	draftInspect = func(context.Context, string, config.Config, lifecycle.RunCommand) (lifecycle.Snapshot, error) {
		return lifecycle.Snapshot{}, errors.New("code host unavailable")
	}
	draftOpen = func(context.Context, string, config.Config, string, lifecycle.RunCommand) (lifecycle.OpenResult, error) {
		return lifecycle.OpenResult{}, errors.New("agent identity is not configured")
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"status"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "code host unavailable") {
		t.Fatalf("status failure: %d %q", code, errout.String())
	}
	errout.Reset()
	if code := draftVerb([]string{"open", "widget"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "agent identity") {
		t.Fatalf("open failure: %d %q", code, errout.String())
	}
	draftRepoRoot = func() (string, error) { return "", errors.New("no repository") }
	errout.Reset()
	if code := draftVerb([]string{"status"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "no repository") {
		t.Fatalf("root failure: %d %q", code, errout.String())
	}
}

func TestDraftCheckpointCLI(t *testing.T) {
	root := t.TempDir()
	oldRoot, oldCheckpoint := draftRepoRoot, draftCheckpoint
	t.Cleanup(func() { draftRepoRoot, draftCheckpoint = oldRoot, oldCheckpoint })
	draftRepoRoot = func() (string, error) { return root, nil }
	want := checkpoint.Result{Missing: []string{"2 of the first 3 draft-born features have locked"}}
	draftCheckpoint = func(_ context.Context, gotRoot string, _ config.Config, run lifecycle.RunCommand) (checkpoint.Result, error) {
		if gotRoot != root || run == nil {
			t.Fatalf("checkpoint inputs: %q %v", gotRoot, run)
		}
		return want, nil
	}
	var out, errout bytes.Buffer
	if code := draftVerb([]string{"checkpoint"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "checkpoint not yet due: 2 of the first 3") || errout.Len() != 0 {
		t.Fatalf("not due: %d %q %q", code, out.String(), errout.String())
	}
	want = checkpoint.Result{Due: true, RecordPath: "docs/memory/checkpoint/feature-draft/record.yaml", Inventory: checkpoint.Inventory{Features: []string{"alpha", "beta", "gamma"}}}
	out.Reset()
	if code := draftVerb([]string{"checkpoint"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "inventory written") || !strings.Contains(out.String(), "alpha, beta, gamma") {
		t.Fatalf("written: %d %q %q", code, out.String(), errout.String())
	}
	want.AlreadyRecorded = true
	out.Reset()
	if code := draftVerb([]string{"checkpoint"}, &out, &errout); code != 0 || !strings.Contains(out.String(), "already recorded") {
		t.Fatalf("repeat: %d %q %q", code, out.String(), errout.String())
	}
	for _, args := range [][]string{{"checkpoint", "alpha"}, {"checkpoint", "--force"}} {
		errout.Reset()
		if code := draftVerb(args, &out, &errout); code != 2 || !strings.Contains(errout.String(), "usage") {
			t.Fatalf("invalid %v: %d %q", args, code, errout.String())
		}
	}
	draftCheckpoint = func(context.Context, string, config.Config, lifecycle.RunCommand) (checkpoint.Result, error) {
		return checkpoint.Result{}, errors.New("mainline unavailable")
	}
	errout.Reset()
	if code := draftVerb([]string{"checkpoint"}, &out, &errout); code != 1 || !strings.Contains(errout.String(), "mainline unavailable") {
		t.Fatalf("failure: %d %q", code, errout.String())
	}
}
