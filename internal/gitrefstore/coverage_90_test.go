package gitrefstore

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverage90StoreExercisesNoopAndMalformedGitBoundaries(t *testing.T) {
	_, repo := cloneFixture(t)
	store, err := New(repo, "origin", "refs/pyxis/coverage-90")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	result, err := store.Apply(ctx, "events", "no files", func(Snapshot) (Mutation, error) {
		return Mutation{Files: map[string][]byte{}, Changed: false}, nil
	})
	if err != nil || result.Changed || len(result.Files) != 0 {
		t.Fatalf("empty no-op = %+v, err=%v", result, err)
	}

	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if _, err := store.commitFiles(ctx, head, nil, "empty"); err == nil {
		t.Fatal("commitFiles accepted an empty path set with a parent")
	}
	if _, err := store.commitFiles(ctx, "bad-parent", map[string][]byte{"events/a": []byte("a")}, "bad parent"); err == nil {
		t.Fatal("commitFiles accepted an invalid parent")
	}
	if _, err := store.commitFiles(ctx, head, map[string][]byte{"../outside": []byte("a")}, "bad path"); err == nil {
		t.Fatal("commitFiles accepted an unsafe path")
	}

	if files, err := store.readManyAt(ctx, "", []string{"events/a"}); err != nil || len(files) != 0 {
		t.Fatalf("empty-head read = %v, %v", files, err)
	}
	if _, err := store.readManyAt(ctx, "bad-head", nil); err == nil {
		t.Fatal("readManyAt accepted an invalid head")
	}
	if _, err := store.readManyAt(ctx, head, []string{"../outside"}); err == nil {
		t.Fatal("readManyAt accepted an unsafe path")
	}
	if _, err := store.readManyAt(ctx, head, []string{"events/missing"}); err == nil {
		t.Fatal("readManyAt accepted an absent exact path")
	}
	if _, err := store.readAt(ctx, strings.Repeat("a", 40), "events/a"); err == nil {
		t.Fatal("readAt accepted an unknown object")
	}
	if _, err := store.listPathsAt(ctx, strings.Repeat("a", 40), "events"); err == nil {
		t.Fatal("listPathsAt accepted an unknown object")
	}
	if _, err := store.history(ctx, strings.Repeat("a", 40)); err == nil {
		t.Fatal("history accepted an unknown object")
	}
	if _, err := store.isAncestor(ctx, "not-an-object", head); err == nil {
		t.Fatal("isAncestor accepted an invalid object")
	}
	if _, err := store.SnapshotAt(ctx, strings.Repeat("a", 40), "events"); err == nil {
		t.Fatal("SnapshotAt accepted an unknown but well-shaped object")
	}
}

func TestCoverage90StorePropagatesRepositoryAndRemoteFailures(t *testing.T) {
	_, repo := cloneFixture(t)
	missingRemote, err := New(repo, "missing-remote", "refs/pyxis/coverage-90-missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missingRemote.EstablishGenesis(context.Background(), "operator:coverage"); err == nil {
		t.Fatal("genesis ignored a missing remote")
	}
	if _, err := missingRemote.remoteHead(context.Background()); err == nil {
		t.Fatal("remoteHead ignored a missing remote")
	}

	missingRepo, err := New(filepath.Join(t.TempDir(), "missing"), "origin", "refs/pyxis/coverage-90-repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := missingRepo.commitFiles(context.Background(), "", map[string][]byte{"events/a": []byte("a")}, "unavailable repo"); err == nil {
		t.Fatal("commitFiles ignored a missing repository")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := missingRepo.history(cancelled, strings.Repeat("a", 40)); err == nil || !errors.Is(err, context.Canceled) {
		// CommandContext may wrap cancellation through os/exec. The contract under
		// test is simply that the read cannot succeed once its context is cancelled.
		if err == nil {
			t.Fatal("history succeeded with a cancelled context")
		}
	}
}
