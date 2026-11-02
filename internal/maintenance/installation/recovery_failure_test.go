package installation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransactionBackupSyncFailureRetainsRestrictedPriorEvidence(t *testing.T) {
	store := newTestStore(t)
	prior := readyManifest(t, store, strings.Repeat("a", 32))
	publish(t, store, prior)
	want := errors.New("injected backup directory sync")
	var path string
	err := store.WithTransaction(t.Context(), func(tx *Transaction) error {
		original := store.syncDir
		store.syncDir = func(*os.Root) error { return want }
		defer func() { store.syncDir = original }()
		var backupErr error
		path, backupErr = tx.Backup([]byte("# original table\n"), true, &prior)
		return backupErr
	})
	if !errors.Is(err, want) || path == "" {
		t.Fatalf("backup=%q err=%v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup stat=%v err=%v", info, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved priorState
	if err := json.Unmarshal(data, &saved); err != nil || !saved.TablePresent || !bytes.Equal(saved.Table, []byte("# original table\n")) || saved.Manifest == nil || saved.Manifest.Generation != prior.Generation {
		t.Fatalf("backup=%+v err=%v", saved, err)
	}
	current, err := store.Load(t.Context())
	if err != nil || current.Generation != prior.Generation || current.State != StateReady {
		t.Fatalf("prior authority changed: %+v %v", current, err)
	}
}

func TestTransactionRestoreRefusalsPreserveReadyAuthority(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*Store, InstallationManifest) (context.Context, InstallationManifest, error)
	}{
		{"canceled cleanup", func(_ *Store, prior InstallationManifest) (context.Context, InstallationManifest, error) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx, prior, context.Canceled
		}},
		{"invalid prior", func(_ *Store, prior InstallationManifest) (context.Context, InstallationManifest, error) {
			prior.State = "invented"
			return t.Context(), prior, ErrInvalidManifest
		}},
		{"foreign repository", func(_ *Store, prior InstallationManifest) (context.Context, InstallationManifest, error) {
			prior.Repository = Repository(t.TempDir())
			return t.Context(), prior, ErrInvalidManifest
		}},
		{"oversized prior", func(_ *Store, prior InstallationManifest) (context.Context, InstallationManifest, error) {
			prior.Host.ToolSearchPath = "/" + strings.Repeat("p", maxManifestBytes)
			return t.Context(), prior, ErrInvalidManifest
		}},
		{"failed atomic restore", func(store *Store, prior InstallationManifest) (context.Context, InstallationManifest, error) {
			want := errors.New("injected restore rename")
			store.rename = func(*os.Root, string, string) error { return want }
			return t.Context(), prior, want
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newTestStore(t)
			prior := readyManifest(t, store, strings.Repeat("b", 32))
			publish(t, store, prior)
			ctx, restore, want := test.setup(store, prior)
			err := store.WithTransaction(t.Context(), func(tx *Transaction) error { return tx.Restore(ctx, &restore) })
			if !errors.Is(err, want) {
				t.Fatalf("restore=%v want=%v", err, want)
			}
			current, err := store.Load(t.Context())
			if err != nil || current.Generation != prior.Generation || current.State != StateReady {
				t.Fatalf("failed restore changed authority: %+v %v", current, err)
			}
		})
	}
}

func TestTransactionBackupAndRestoreRequireLiveOwnership(t *testing.T) {
	store := newTestStore(t)
	var expired *Transaction
	if err := store.WithTransaction(t.Context(), func(tx *Transaction) error { expired = tx; return nil }); err != nil {
		t.Fatal(err)
	}
	if path, err := expired.Backup(nil, false, nil); err == nil || path != "" {
		t.Fatalf("expired backup=%q err=%v", path, err)
	}
	if err := expired.Restore(t.Context(), nil); err == nil {
		t.Fatal("expired transaction removed authority")
	}
	if err := store.WithTransaction(t.Context(), func(tx *Transaction) error { return tx.Restore(t.Context(), nil) }); err != nil {
		t.Fatalf("live owner could not confirm absent manifest: %v", err)
	}
}

func TestTransactionMissingDirectoryNeverConfirmsBackupOrRestore(t *testing.T) {
	for _, operation := range []string{"backup", "restore"} {
		t.Run(operation, func(t *testing.T) {
			store := newTestStore(t)
			prior := readyManifest(t, store, strings.Repeat("c", 32))
			publish(t, store, prior)
			dir := filepath.Dir(store.ManifestPath())
			moved := dir + "-moved"
			err := store.WithTransaction(t.Context(), func(tx *Transaction) error {
				if err := os.Rename(dir, moved); err != nil {
					return err
				}
				if operation == "backup" {
					_, err := tx.Backup(nil, true, &prior)
					return err
				}
				return tx.Restore(t.Context(), nil)
			})
			if err == nil {
				t.Fatal("missing installation directory reported success")
			}
			if _, err := store.Load(t.Context()); !errors.Is(err, ErrInstallationRequired) {
				t.Fatalf("missing state exposed authority: %v", err)
			}
			if err := os.Rename(moved, dir); err != nil {
				t.Fatal(err)
			}
			current, err := store.Load(t.Context())
			if err != nil || current.Generation != prior.Generation {
				t.Fatalf("original manifest damaged: %+v %v", current, err)
			}
		})
	}
}
