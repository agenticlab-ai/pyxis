package gitdrive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestSignedStoreColdRollbackToGenesisRefusesLoadAndRecreation(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := newFixture(t, format)
			prior := f.assign(t)
			git(t, f.origin, "update-ref", f.store.ref, f.binding.Genesis.Commit)
			clone := filepath.Join(t.TempDir(), "cold")
			git(t, filepath.Dir(clone), "clone", "--quiet", f.origin, clone)
			if err := os.RemoveAll(f.repo); err != nil {
				t.Fatal(err)
			}
			git(t, f.origin, "gc", "--prune=now")
			before := git(t, f.origin, "show-ref")
			cold := f.newStore(t, clone, f.operator)
			if _, err := cold.Load(t.Context(), f.binding.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Errorf("rollback reported as pristine: %v", err)
			}
			if err := cold.Create(t.Context(), f.initial()); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Errorf("rollback permitted a new lineage: %v", err)
			}
			if err := cold.CAS(t.Context(), prior.FeatureID, prior.Version, step(prior, "effect-intent")); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Errorf("rollback permitted new work: %v", err)
			}
			if git(t, f.origin, "show-ref") != before {
				t.Fatal("rollback refusal changed the origin")
			}
		})
	}
}

func TestSignedStoreColdReadRefusesMissingOrDivergentCheckpoint(t *testing.T) {
	for _, change := range []string{"valid older prefix", "missing checkpoint", "rewound checkpoint", "deleted primary"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t, "sha1")
			prior := f.assign(t)
			snapshot, err := f.store.git.Fetch(t.Context(), "")
			if err != nil || snapshot.CheckpointHead != snapshot.Head {
				t.Fatalf("publication lost checkpoint: %+v, %v", snapshot, err)
			}
			switch change {
			case "valid older prefix":
				git(t, f.origin, "update-ref", f.store.ref, snapshot.Commits[1].Object)
			case "missing checkpoint":
				git(t, f.origin, "update-ref", "-d", f.store.checkpoint)
			case "rewound checkpoint":
				git(t, f.origin, "update-ref", f.store.checkpoint, f.binding.Genesis.Commit)
			case "deleted primary":
				git(t, f.origin, "update-ref", "-d", f.store.ref)
			}
			before := git(t, f.origin, "show-ref")
			cold := f.newStore(t, f.repo, f.operator)
			if _, err := cold.Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Errorf("%s accepted on cold read: %v", change, err)
			}
			if err := cold.Create(t.Context(), f.initial()); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Errorf("%s permitted recreation: %v", change, err)
			}
			if git(t, f.origin, "show-ref") != before {
				t.Fatal("checkpoint refusal changed origin refs")
			}
			f.unchangedSource(t)
		})
	}
}
