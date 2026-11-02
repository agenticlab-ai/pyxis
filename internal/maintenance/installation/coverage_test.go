package installation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

func TestLoadRefusesInvalidSuffixAndEncoding(t *testing.T) {
	store := newTestStore(t)
	if err := store.prepare(); err != nil {
		t.Fatal(err)
	}
	manifest := readyManifest(t, store, strings.Repeat("e", 32))
	manifest.Host.ExecutablePath = "/tmp/utf8-marker"
	data, err := encode(manifest)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{"trailing value beyond read bound", []byte(string(data) + strings.Repeat(" ", (1<<20)-len(data)) + "{}")},
		{"invalid UTF-8 string", bytes.Replace(data, []byte("utf8-marker"), []byte{0xff}, 1)},
		{"capitalized state override", bytes.Replace(data, []byte(`"state":"ready"`), []byte(`"state":"recovering","State":"ready"`), 1)},
		{"Unicode state override", bytes.Replace(data, []byte(`"state":"ready"`), []byte(`"state":"recovering","ſtate":"ready"`), 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(store.ManifestPath(), test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(context.Background()); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("invalid complete manifest accepted: %v", err)
			}
		})
	}
}

func TestEmptyScheduleCopiesRemainValid(t *testing.T) {
	store := newTestStore(t)
	manifest := readyManifest(t, store, strings.Repeat("f", 32))
	manifest.Schedules = []ScheduleFingerprint{}
	cloned := cloneManifest(manifest)
	if err := cloned.Validate(); err != nil {
		t.Fatalf("copy changed a valid empty schedule binding: %v", err)
	}
	binding, err := proof(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Schedules == nil {
		t.Fatal("ready proof changed an empty schedule binding to nil")
	}
}

func TestRelativeRepositoryUnderSymlinkedWorkingDirectory(t *testing.T) {
	repository, aliases, state := t.TempDir(), t.TempDir(), t.TempDir()
	alias := filepath.Join(aliases, "repo-alias")
	if err := os.Symlink(repository, alias); err != nil {
		t.Fatal(err)
	}
	expected, err := open(repository, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(alias)
	t.Setenv("PWD", alias)
	actual, err := open(".", state)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Repository() != expected.Repository() || actual.ManifestPath() != expected.ManifestPath() {
		t.Fatalf("relative alias split installation identity: got %v, want %v", actual.Repository(), expected.Repository())
	}
}

func TestOversizedPublicationPreservesReadyGeneration(t *testing.T) {
	store := newTestStore(t)
	original := readyManifest(t, store, strings.Repeat("a", 32))
	publish(t, store, original)
	before, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	oversized := cloneManifest(original)
	oversized.Generation = InstallationGeneration(strings.Repeat("b", 32))
	oversized.Schedules = make([]ScheduleFingerprint, 12000)
	for i := range oversized.Schedules {
		oversized.Schedules[i] = ScheduleFingerprint{JobID: fmt.Sprintf("daily-%05d", i), SHA256: strings.Repeat("c", 64)}
	}
	if err := oversized.Validate(); err != nil {
		t.Fatalf("oversized fixture is otherwise invalid: %v", err)
	}
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		return transaction.Publish(oversized)
	}); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("oversized ready publication = %v", err)
	}
	after, err := os.ReadFile(store.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("rejected oversized publication replaced the ready manifest")
	}
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		binding, err := transaction.Ready()
		if err == nil && binding.Generation != original.Generation {
			return errors.New("rejected publication changed the ready generation")
		}
		return err
	}); err != nil {
		t.Fatalf("original ready generation is not usable: %v", err)
	}
}

func TestManifestValidationRefusals(t *testing.T) {
	store := newTestStore(t)
	valid := readyManifest(t, store, strings.Repeat("a", 32))
	tests := map[string]func(*InstallationManifest){
		"generation": func(value *InstallationManifest) { value.Generation = "not-hex" },
		"state":      func(value *InstallationManifest) { value.State = "recovering" },
		"repository": func(value *InstallationManifest) { value.Repository.Root = "relative" },
		"source":     func(value *InstallationManifest) { value.Source.Remote = "bad remote" },
		"schedules":  func(value *InstallationManifest) { value.Schedules = nil },
		"job id":     func(value *InstallationManifest) { value.Schedules[0].JobID = "Bad" },
		"hash length": func(value *InstallationManifest) {
			value.Schedules[0].SHA256 = "ab"
		},
		"hash alphabet": func(value *InstallationManifest) {
			value.Schedules[0].SHA256 = strings.Repeat("z", 64)
		},
		"duplicate": func(value *InstallationManifest) {
			value.Schedules = append(value.Schedules, value.Schedules[0])
		},
		"executable": func(value *InstallationManifest) { value.Host.ExecutablePath = "pyxis" },
		"empty path": func(value *InstallationManifest) { value.Host.ToolSearchPath = "" },
		"relative path": func(value *InstallationManifest) {
			value.Host.ToolSearchPath = "/usr/bin:relative"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			manifest := cloneManifest(valid)
			mutate(&manifest)
			if err := manifest.Validate(); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("validate = %v", err)
			}
		})
	}
	if _, err := ScheduleFingerprints(jobsDocumentInvalid()); err == nil {
		t.Fatal("invalid policy produced schedule fingerprints")
	}
}

func TestStoreRefusalAndProofBranches(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		if _, err := Open(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		t.Setenv("XDG_STATE_HOME", "relative")
		if _, err := Open(t.TempDir()); err == nil {
			t.Fatal("relative state root accepted")
		}
		if _, err := open(filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
			t.Fatal("missing repository accepted")
		}
		if _, err := open(t.TempDir(), "relative"); err == nil {
			t.Fatal("relative explicit state root accepted")
		}
	})

	store := newTestStore(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Load(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load = %v", err)
	}
	if err := store.WithTransaction(context.Background(), nil); err == nil {
		t.Fatal("nil transaction callback accepted")
	}
	if err := store.WithTransaction(canceled, func(*Transaction) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled transaction = %v", err)
	}
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		if err := transaction.Publish(InstallationManifest{}); !errors.Is(err, ErrInvalidManifest) {
			t.Fatalf("invalid publish = %v", err)
		}
		wrong := readyManifest(t, store, strings.Repeat("b", 32))
		wrong.Repository = Repository(t.TempDir())
		if err := transaction.Publish(wrong); !errors.Is(err, ErrInvalidManifest) {
			t.Fatalf("wrong-repository publish = %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	manifest := readyManifest(t, store, strings.Repeat("c", 32))
	publish(t, store, manifest)
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		binding, err := transaction.Ready()
		if err != nil {
			return err
		}
		return transaction.ValidateReady(binding)
	}); err != nil {
		t.Fatalf("current proof refused: %v", err)
	}
	if err := os.Chmod(store.ManifestPath(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background()); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("permissive manifest = %v", err)
	}
	if err := os.Chmod(store.ManifestPath(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ManifestPath(), []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.WithTransaction(context.Background(), func(transaction *Transaction) error {
		return transaction.ValidateReady(ReadyBinding{})
	}); !errors.Is(err, ErrInstallationRequired) || !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("malformed readiness = %v", err)
	}
}

func TestStorageFailureBranches(t *testing.T) {
	t.Run("private directory", func(t *testing.T) {
		store := newTestStore(t)
		if err := os.MkdirAll(filepath.Join(store.stateRoot, "maintenance"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(store.stateRoot, "maintenance"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := store.prepare(); err == nil {
			t.Fatal("permissive maintenance directory accepted")
		}
	})
	t.Run("state root file", func(t *testing.T) {
		repository, parent := t.TempDir(), t.TempDir()
		stateRoot := filepath.Join(parent, "state-file")
		if err := os.WriteFile(stateRoot, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		store, err := open(repository, stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.prepare(); err == nil {
			t.Fatal("state root file accepted")
		}
		if err := store.WithTransaction(context.Background(), func(*Transaction) error { return nil }); err == nil {
			t.Fatal("transaction accepted a state root file")
		}
	})
	t.Run("lock mode", func(t *testing.T) {
		store := newTestStore(t)
		if err := store.prepare(); err != nil {
			t.Fatal(err)
		}
		lock := filepath.Join(store.stateRoot, "maintenance", "transaction.lock")
		if err := os.WriteFile(lock, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(lock, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := store.WithTransaction(context.Background(), func(*Transaction) error { return nil }); err == nil {
			t.Fatal("permissive transaction lock accepted")
		}
	})
	t.Run("publish cancellation", func(t *testing.T) {
		store := newTestStore(t)
		if err := store.prepare(); err != nil {
			t.Fatal(err)
		}
		manifest := readyManifest(t, store, strings.Repeat("d", 32))
		data, err := encode(manifest)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := store.publish(ctx, manifest, data); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled publication = %v", err)
		}
		if _, err := store.Load(context.Background()); !errors.Is(err, ErrInstallationRequired) {
			t.Fatalf("canceled publication exposed authority: %v", err)
		}
	})
	t.Run("missing publication directory", func(t *testing.T) {
		store := newTestStore(t)
		manifest := readyManifest(t, store, strings.Repeat("e", 32))
		data, err := encode(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.publish(context.Background(), manifest, data); err == nil {
			t.Fatal("publication without prepared state succeeded")
		}
	})
}

func TestCodecAndUncertainErrorBranches(t *testing.T) {
	if _, err := decode([]byte(`{} {}`)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("multiple values = %v", err)
	}
	if _, err := decode([]byte(`true`)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("scalar manifest = %v", err)
	}
	cause := errors.New("sync failed")
	err := &UncertainError{Cause: cause}
	if !strings.Contains(err.Error(), cause.Error()) || !errors.Is(err, cause) {
		t.Fatalf("uncertain error chain = %v", err)
	}
	root, openErr := os.OpenRoot(t.TempDir())
	if openErr != nil {
		t.Fatal(openErr)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	if err := syncDirectory(root); err == nil {
		t.Fatal("closed directory root synced")
	}
}

func jobsDocumentInvalid() jobs.Document { return jobs.Document{} }
