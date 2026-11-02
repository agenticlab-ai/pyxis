package jobs

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeRequiresExplicitJobsAndEnabled(t *testing.T) {
	valid := string(rawEncode(t, Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}}))
	var withoutEnabled []string
	for line := range strings.SplitSeq(valid, "\n") {
		if strings.TrimSpace(line) != "enabled: true" {
			withoutEnabled = append(withoutEnabled, line)
		}
	}
	for name, data := range map[string]string{
		"missing jobs":    "schema_version: pyxis-maintenance-jobs/v1\nsource: {remote: origin, branch: main}\n",
		"null jobs":       "schema_version: pyxis-maintenance-jobs/v1\nsource: {remote: origin, branch: main}\njobs: null\n",
		"missing enabled": strings.Join(withoutEnabled, "\n"),
		"null enabled":    strings.Replace(valid, "enabled: true", "enabled: null", 1),
		"string enabled":  strings.Replace(valid, "enabled: true", "enabled: 'true'", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(data)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Decode = %v", err)
			}
		})
	}
	if _, err := Decode([]byte("schema_version: pyxis-maintenance-jobs/v1\nsource: {remote: origin, branch: main}\njobs: []\n")); err != nil {
		t.Fatal(err)
	}
	disabled := strings.Replace(valid, "enabled: true", "enabled: false", 1)
	if document, err := Decode([]byte(disabled)); err != nil || document.Jobs[0].Enabled {
		t.Fatalf("explicit false = %#v, %v", document, err)
	}
}

func TestInvalidUpdateIdentitiesReturnInvalidWithoutChanges(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	before := fileBytes(t, store)
	for _, test := range []struct{ id, replacement string }{{"INVALID", "INVALID"}, {"first", "INVALID"}} {
		if err := store.Update(t.Context(), test.id, fixtureJob(test.replacement)); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Update(%q, %q) = %v", test.id, test.replacement, err)
		}
	}
	if !reflect.DeepEqual(before, fileBytes(t, store)) {
		t.Fatal("invalid update changed bytes")
	}
}

func TestOptionShapedBranchesAreInvalid(t *testing.T) {
	for _, branch := range []string{"-topic", "--force", "--upload-pack=command"} {
		t.Run(branch, func(t *testing.T) {
			document := Document{SchemaVersion: SchemaVersion, Source: fixtureSource, Jobs: []Job{fixtureJob("first")}}
			document.Source.Branch = branch
			if _, err := Encode(document); !errors.Is(err, ErrInvalid) {
				t.Fatalf("source branch = %v", err)
			}
			document.Source = fixtureSource
			document.Jobs[0].Delivery.Branch = branch
			if _, err := Encode(document); !errors.Is(err, ErrInvalid) {
				t.Fatalf("delivery branch = %v", err)
			}
		})
	}
}

func TestDirectorySwapCannotRedirectJobsWrite(t *testing.T) {
	for _, phase := range []string{"stage", "replace"} {
		t.Run(phase, func(t *testing.T) {
			store := fixtureStore(t, fixtureJob("first"))
			before := fileBytes(t, store)
			directory, backup := filepath.Dir(store.path), filepath.Join(store.root, ".pyxis/saved-maintenance")
			outside := t.TempDir()
			referent := filepath.Join(outside, "jobs.yaml")
			if err := os.WriteFile(referent, before, 0o600); err != nil {
				t.Fatal(err)
			}
			swap := func() error {
				if err := os.Rename(directory, backup); err != nil {
					return err
				}
				return os.Symlink(outside, directory)
			}
			if phase == "stage" {
				store.beforeWrite = swap
			} else {
				store.rename = func(root *os.Root, from, to string) error {
					if err := swap(); err != nil {
						return err
					}
					return root.Rename(from, to)
				}
			}
			err := store.Update(t.Context(), "first", changedJob())
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("directory swap = %v", err)
			}
			if errors.Is(err, ErrUnconfirmed) != (phase == "replace") {
				t.Fatalf("committed state misreported: %v", err)
			}
			got, readErr := os.ReadFile(referent)
			if readErr != nil || !reflect.DeepEqual(got, before) {
				t.Fatalf("outside file changed: %v", readErr)
			}
			if err := os.Remove(directory); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backup, directory); err != nil {
				t.Fatal(err)
			}
			if phase == "stage" && !reflect.DeepEqual(before, fileBytes(t, store)) {
				t.Fatal("pre-commit swap changed original bytes")
			}
			if phase == "replace" {
				var uncertain *UnconfirmedError
				if !errors.As(err, &uncertain) || !reflect.DeepEqual(uncertain.Bytes, fileBytes(t, store)) {
					t.Fatal("lost committed bytes after pinned replacement")
				}
			}
		})
	}
}

func TestMissingRepositoryAndBlockedStateReturnFilesystemErrors(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	if err := os.Rename(store.root, filepath.Join(t.TempDir(), "saved")); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{func() error { _, err := store.Read(t.Context(), "first"); return err }(), store.Delete(t.Context(), "first"), store.commit(t.Context(), []byte("unused"))} {
		if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrNotFound) {
			t.Fatalf("removed repository = %v", err)
		}
	}
	store = fixtureStore(t)
	if err := os.MkdirAll(filepath.Join(store.root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.root, ".pyxis/state"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(t.Context(), &fixtureSource, fixtureJob("first")); err == nil {
		t.Fatal("blocked state accepted")
	}
}

func TestLockSwapCannotCreateOutsideState(t *testing.T) {
	for _, target := range []string{"directory", "leaf"} {
		t.Run(target, func(t *testing.T) {
			store := fixtureStore(t, fixtureJob("first"))
			before := fileBytes(t, store)
			outside := t.TempDir()
			store.beforeLock = func() error {
				if target == "directory" {
					directory := filepath.Dir(store.lockPath)
					if err := os.Rename(directory, filepath.Join(store.root, ".pyxis/state/saved-maintenance")); err != nil {
						return err
					}
					return os.Symlink(outside, directory)
				}
				if err := os.Remove(store.lockPath); err != nil {
					return err
				}
				return os.Symlink(filepath.Join(outside, "jobs.lock"), store.lockPath)
			}
			if err := store.Update(t.Context(), "first", changedJob()); !errors.Is(err, ErrInvalid) {
				t.Fatalf("lock swap = %v", err)
			}
			if _, err := os.Stat(filepath.Join(outside, "jobs.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("outside lock created: %v", err)
			}
			if !reflect.DeepEqual(before, fileBytes(t, store)) {
				t.Fatal("lock swap changed policy")
			}
		})
	}
}

func TestLockSetupFailureDoesNotRetainOwnership(t *testing.T) {
	store := fixtureStore(t, fixtureJob("first"))
	failure := errors.New("lock setup interrupted")
	before := fileBytes(t, store)
	store.beforeLock = func() error { return failure }
	if err := store.Update(t.Context(), "first", changedJob()); !errors.Is(err, failure) {
		t.Fatalf("lock setup = %v", err)
	}
	if !reflect.DeepEqual(before, fileBytes(t, store)) {
		t.Fatal("lock setup failure changed bytes")
	}
	store.beforeLock = nil
	if err := store.Update(t.Context(), "first", changedJob()); err != nil {
		t.Fatal(err)
	}
}
