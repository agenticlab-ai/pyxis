package jobs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidRepositoryRoots(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file, filepath.Join(root, "missing")} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if _, err := New(path); !errors.Is(err, ErrInvalid) {
				t.Fatalf("New(%q) = %v", path, err)
			}
		})
	}
}

func TestReadAndMutationFailuresPreserveStoredBytes(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	before := fileBytes(t, store)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	tests := []struct {
		name string
		run  func() error
		want error
	}{
		{"canceled read", func() error { _, err := store.Read(canceled, "first"); return err }, context.Canceled},
		{"canceled list", func() error { _, err := store.List(canceled); return err }, context.Canceled},
		{"canceled create", func() error { return store.Create(canceled, nil, fixtureJob("other")) }, context.Canceled},
		{"invalid read id", func() error { _, err := store.Read(t.Context(), "INVALID"); return err }, ErrInvalid},
		{"invalid delete id", func() error { return store.Delete(t.Context(), "INVALID") }, ErrInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if got := fileBytes(t, store); string(got) != string(before) {
				t.Fatal("failed operation changed stored bytes")
			}
		})
	}
	missing := fixtureStore(t)
	if _, err := missing.Read(t.Context(), "first"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing document read = %v", err)
	}
}

func TestStoredFailuresAreReturnedByEveryReaderAndMutation(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "filesystem", true: "invalid YAML"}[corrupt], func(t *testing.T) {
			store := fixtureStore(t, fixtureJob("first"))
			if err := os.Remove(store.path); err != nil {
				t.Fatal(err)
			}
			var want error
			if corrupt {
				if err := os.WriteFile(store.path, []byte("schema_version: ["), 0o600); err != nil {
					t.Fatal(err)
				}
				want = ErrInvalid
			} else if err := os.Mkdir(store.path, 0o700); err != nil {
				t.Fatal(err)
			}
			_, readErr := store.Read(t.Context(), "first")
			_, listErr := store.List(t.Context())
			createErr := store.Create(t.Context(), nil, fixtureJob("other"))
			for _, err := range []error{readErr, listErr, createErr} {
				if err == nil || want != nil && !errors.Is(err, want) {
					t.Fatalf("stored failure = %v, want %v", err, want)
				}
			}
		})
	}
}

func TestStorageLockUsesStateAndRejectsLaterSymlinks(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	if want := filepath.Join(store.root, ".pyxis/state/maintenance/jobs.lock"); store.lockPath != want {
		t.Fatalf("lock path = %q, want %q", store.lockPath, want)
	}
	before := fileBytes(t, store)
	if err := os.Remove(store.lockPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside.lock"), store.lockPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(t.Context(), "first", changedJob()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("late symlink = %v", err)
	}
	if got := fileBytes(t, store); string(got) != string(before) {
		t.Fatal("late symlink mutation changed stored bytes")
	}
}

func TestUnconfirmedCommitRetainsCauseAndCommittedJob(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	failure := errors.New("directory durability unavailable")
	store.syncDir = func(*os.Root) error { return failure }
	err := store.Update(t.Context(), "first", changedJob())
	var unconfirmed *UnconfirmedError
	if !errors.As(err, &unconfirmed) || !errors.Is(err, failure) || !strings.Contains(err.Error(), failure.Error()) {
		t.Fatalf("unconfirmed result lost its cause: %v", err)
	}
	job, readErr := store.Read(t.Context(), "first")
	if readErr != nil || job.Prompt != changedJob().Prompt {
		t.Fatalf("committed job = %#v, %v", job, readErr)
	}
}

func TestTypedValidationAndOptionalDelivery(t *testing.T) {
	for _, timezone := range []string{"", "Local", "No/SuchTimezone"} {
		document := Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}}
		document.Jobs[0].Schedule.Timezone = timezone
		if _, err := Encode(document); !errors.Is(err, ErrInvalid) {
			t.Fatalf("timezone %q = %v", timezone, err)
		}
	}
	document := Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}}
	document.Jobs[0].Delivery = nil
	if _, err := Encode(document); err != nil {
		t.Fatal(err)
	}
	store := fixtureStore(t, document.Jobs[0])
	job, err := store.Read(t.Context(), "first")
	if err != nil || job.Delivery != nil {
		t.Fatalf("optional delivery = %#v, %v", job, err)
	}
	root, openErr := os.OpenRoot(t.TempDir())
	if openErr != nil {
		t.Fatal(openErr)
	}
	if closeErr := root.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err := syncDirectory(root); err == nil {
		t.Fatalf("missing sync directory = %v", err)
	}
}
