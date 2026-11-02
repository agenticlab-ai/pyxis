package git

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/admission"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/claim"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

type failedWriteRefs struct{ base refStore }

func (f failedWriteRefs) Update(context.Context, string, string, func([]byte) ([]byte, bool, error)) ([]byte, error) {
	return nil, errors.New("injected remote write failure")
}

func (f failedWriteRefs) Read(ctx context.Context, path string) ([]byte, error) {
	return f.base.Read(ctx, path)
}

func integrationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func integrationSnapshot(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		result = append(result, fmt.Sprintf("%s %x", rel, sha256.Sum256(data)))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(result)
	return result
}

func TestIndependentControlsContendThroughRemoteCASWithoutTouchingEnrolledGit(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	otherRemote := filepath.Join(root, "other-remote.git")
	enrolled := filepath.Join(root, "enrolled")
	integrationGit(t, root, "init", "--bare", "--quiet", remote)
	integrationGit(t, root, "init", "--bare", "--quiet", otherRemote)
	integrationGit(t, root, "init", "--quiet", "-b", "main", enrolled)
	integrationGit(t, enrolled, "remote", "add", "origin", remote)
	document := jobs.Document{SchemaVersion: jobs.SchemaVersion, Source: jobs.SourceBinding{Remote: "origin", Branch: "main"}, Jobs: []jobs.Job{}}
	policy, err := jobs.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{".pyxis/maintenance/jobs.yaml": policy, ".pyxis/config.yaml": []byte(harnessconfigtest.YAML)} {
		path := filepath.Join(enrolled, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	integrationGit(t, enrolled, "add", ".pyxis/config.yaml", ".pyxis/maintenance/jobs.yaml")
	integrationGit(t, enrolled, "commit", "-m", "published")
	integrationGit(t, enrolled, "push", "origin", "main")
	integrationGit(t, enrolled, "remote", "add", "other", otherRemote)
	integrationGit(t, enrolled, "push", "other", "main")
	publishedCommit := integrationGit(t, enrolled, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(enrolled, "dirty.txt"), []byte("preserve\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(enrolled, ".git", "FETCH_HEAD"), []byte("preserve-fetch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(enrolled, ".git", "hooks", "pre-push"), []byte("#!/bin/sh\nexit 91\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	integrationGit(t, enrolled, "config", "maintenance.fixture", "preserve")
	assertHookRejects := func() {
		t.Helper()
		command := exec.Command("git", "-C", enrolled, "push", "origin", "HEAD:refs/heads/hook-probe")
		if output, err := command.CombinedOutput(); err == nil {
			t.Fatalf("enrolled pre-push hook did not reject ordinary push: %s", output)
		}
	}
	assertHookRejects()
	common := integrationGit(t, enrolled, "rev-parse", "--path-format=absolute", "--git-common-dir")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	source := admission.ControlSource{Repository: enrolled, GitCommon: common, RemoteURL: remote}
	firstControl, err := source.Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = firstControl.Control.Close() }()
	secondControl, err := source.Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = secondControl.Control.Close() }()
	firstStore, err := New(firstControl.Control, "daily")
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := New(secondControl.Control, "daily")
	if err != nil {
		t.Fatal(err)
	}
	before := integrationSnapshot(t, enrolled)
	first := adapterRecord("2026-10-20", strings.Repeat("a", 64), publishedCommit)
	second := adapterRecord("2026-10-20", strings.Repeat("b", 64), publishedCommit)
	second.ClaimedAt = first.ClaimedAt.Add(11 * time.Hour)
	var results [2]claim.ClaimResult
	var errs [2]error
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index, input := range []struct {
		store  *Store
		record claim.Record
	}{{firstStore, first}, {secondStore, second}} {
		wait.Add(1)
		go func(index int, input struct {
			store  *Store
			record claim.Record
		}) {
			defer wait.Done()
			<-start
			results[index], errs[index] = input.store.Claim(context.Background(), input.record)
		}(index, input)
	}
	close(start)
	wait.Wait()
	owned, consumed := 0, 0
	for index, result := range results {
		if errs[index] != nil {
			t.Fatalf("contender %d: %v", index, errs[index])
		}
		switch result.Disposition {
		case claim.DispositionOwned:
			owned++
		case claim.DispositionConsumed:
			consumed++
		}
	}
	if owned != 1 || consumed != 1 {
		t.Fatalf("contention dispositions = %+v", results)
	}
	otherJob, err := New(firstControl.Control, "other")
	if err != nil {
		t.Fatal(err)
	}
	independentJob := first
	independentJob.JobID, independentJob.OwnerNonce = "other", strings.Repeat("c", 64)
	if result, err := otherJob.Claim(t.Context(), independentJob); err != nil || result.Disposition != claim.DispositionOwned {
		t.Fatalf("independent job = (%+v, %v)", result, err)
	}
	independentDate := first
	independentDate.JobDate, independentDate.OwnerNonce = "2026-10-21", strings.Repeat("d", 64)
	if result, err := firstStore.Claim(t.Context(), independentDate); err != nil || result.Disposition != claim.DispositionOwned {
		t.Fatalf("independent date = (%+v, %v)", result, err)
	}
	otherPublished, err := (admission.ControlSource{Repository: enrolled, GitCommon: common, RemoteURL: otherRemote}).Capture(t.Context(), document.Source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = otherPublished.Control.Close() }()
	otherStore, err := New(otherPublished.Control, "daily")
	if err != nil {
		t.Fatal(err)
	}
	independentRemote := first
	independentRemote.OwnerNonce = strings.Repeat("e", 64)
	if result, err := otherStore.Claim(t.Context(), independentRemote); err != nil || result.Disposition != claim.DispositionOwned {
		t.Fatalf("independent remote = (%+v, %v)", result, err)
	}
	malformedRefs, err := firstControl.Control.RefStore("refs/pyxis/maintenance/malformed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := malformedRefs.Update(t.Context(), "occurrences/2026-10-20.json", "malformed fixture", func([]byte) ([]byte, bool, error) {
		return []byte(`{"schema_version":"future"}`), true, nil
	}); err != nil {
		t.Fatal(err)
	}
	malformedStore, err := New(firstControl.Control, "malformed")
	if err != nil {
		t.Fatal(err)
	}
	malformedCandidate := first
	malformedCandidate.JobID = "malformed"
	if result, err := malformedStore.Claim(t.Context(), malformedCandidate); err == nil || result.Disposition == claim.DispositionOwned {
		t.Fatalf("malformed remote state authorized owner: result=%+v err=%v", result, err)
	}
	terminalStore, err := New(firstControl.Control, "terminal")
	if err != nil {
		t.Fatal(err)
	}
	terminalClaim := first
	terminalClaim.JobID, terminalClaim.OwnerNonce = "terminal", strings.Repeat("f", 64)
	if result, err := terminalStore.Claim(t.Context(), terminalClaim); err != nil || result.Disposition != claim.DispositionOwned {
		t.Fatalf("terminal fixture claim = (%+v, %v)", result, err)
	}
	failingTerminal := &Store{refs: failedWriteRefs{base: terminalStore.refs}, jobID: "terminal"}
	if _, err := failingTerminal.Transition(t.Context(), terminalClaim, claim.Terminal{Outcome: claim.OutcomeFailed, FinishedAt: terminalClaim.ClaimedAt.Add(time.Hour)}); err == nil {
		t.Fatal("failed terminal write reported success")
	}
	terminalLoser := terminalClaim
	terminalLoser.OwnerNonce = strings.Repeat("1", 64)
	if result, err := terminalStore.Claim(t.Context(), terminalLoser); err != nil || result.Disposition != claim.DispositionConsumed || result.Record.Outcome != claim.OutcomeClaimed {
		t.Fatalf("failed terminal write did not retain consumed claim: result=%+v err=%v", result, err)
	}
	assertHookRejects()
	if after := integrationSnapshot(t, enrolled); !reflect.DeepEqual(before, after) {
		t.Fatalf("claim operations changed enrolled worktree or Git state\nbefore=%v\nafter=%v", before, after)
	}
	if _, err := os.Stat(filepath.Join(enrolled, "hook-ran")); !os.IsNotExist(err) {
		t.Fatalf("enrolled hook side effect: %v", err)
	}
}
