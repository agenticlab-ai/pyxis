package gitrefstore

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckpointCASRejectsMovementWithoutPartialPublication(t *testing.T) {
	for _, change := range []string{"moved", "deleted"} {
		t.Run(change, func(t *testing.T) {
			origin, repo := cloneFixture(t)
			store, err := NewWithCheckpoint(repo, "origin", "refs/pyxis/test-drive", "refs/pyxis/test-checkpoint")
			if err != nil {
				t.Fatal(err)
			}
			genesis, err := store.EstablishGenesis(t.Context(), "fixture")
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.Fetch(t.Context(), "")
			if err != nil || snapshot.Head != genesis.Commit || snapshot.CheckpointHead != genesis.Commit {
				t.Fatalf("genesis was not retained atomically: %+v, %v", snapshot, err)
			}
			candidate, err := store.commitFiles(t.Context(), genesis.Commit, map[string][]byte{"state": []byte("next")}, "candidate")
			if err != nil {
				t.Fatal(err)
			}
			if change == "moved" {
				source := strings.TrimSpace(runGit(t, origin, "rev-parse", "refs/heads/main"))
				runGit(t, origin, "update-ref", store.checkpoint, source)
			} else {
				runGit(t, origin, "update-ref", "-d", store.checkpoint)
			}
			before := runGit(t, origin, "show-ref")
			if err := store.pushCAS(t.Context(), genesis.Commit, candidate); !errors.Is(err, errCASConflict) {
				t.Fatalf("lost checkpoint lease accepted: %v", err)
			}
			if after := runGit(t, origin, "show-ref"); after != before {
				t.Fatalf("atomic refusal changed refs: %s -> %s", before, after)
			}
		})
	}
}

func TestCheckpointStoreRefusesUnsupportedAtomicPush(t *testing.T) {
	origin, repo := cloneFixture(t)
	runGit(t, origin, "config", "receive.advertiseAtomic", "false")
	store, err := NewWithCheckpoint(repo, "origin", "refs/pyxis/test-drive", "refs/pyxis/test-checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	before := runGit(t, origin, "show-ref")
	if _, err := store.EstablishGenesis(t.Context(), "fixture"); err == nil {
		t.Fatal("origin without atomic push support accepted paired publication")
	}
	if after := runGit(t, origin, "show-ref"); after != before {
		t.Fatal("unsupported atomic push changed refs")
	}
}

func TestCheckpointStoreRequiresDistinctSafeEvidenceRefs(t *testing.T) {
	for _, checkpoint := range []string{"", "refs/heads/main", "refs/pyxis/test-drive", "refs/pyxis/a\x00b", "refs/pyxis/a.lock"} {
		if _, err := NewWithCheckpoint("/repo", "origin", "refs/pyxis/test-drive", checkpoint); err == nil {
			t.Errorf("accepted checkpoint %q", checkpoint)
		}
	}
	if _, err := NewWithCheckpoint("", "origin", "refs/pyxis/test-drive", "refs/pyxis/test-checkpoint"); err == nil {
		t.Fatal("checkpoint constructor accepted invalid primary binding")
	}
}
