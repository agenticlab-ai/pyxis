package localidentity

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Cancellation is delivered when the selected filesystem publication becomes
// visible, allowing both sides of the create-only boundary to be exercised.
type cancelWhenVisible struct {
	context.Context
	path   string
	cancel context.CancelFunc
}

func (c cancelWhenVisible) Err() error {
	if _, err := os.Stat(c.path); err == nil {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCancellationAroundCredentialPublication(t *testing.T) {
	for _, visible := range []string{".owner-01010101010101010101010101010101", keyFile} {
		t.Run(visible, func(t *testing.T) {
			home := t.TempDir()
			directory := filepath.Join(home, "feature-drive")
			base, cancel := context.WithCancel(t.Context())
			defer cancel()
			ctx := cancelWhenVisible{Context: base, path: filepath.Join(directory, visible), cancel: cancel}
			if identity, err := open(ctx, home, bytes.NewReader(bytes.Repeat([]byte{1}, 64))); !errors.Is(err, context.Canceled) || identity != nil {
				t.Fatalf("cancelled registration announced an owner: %v", err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if visible == keyFile {
				// Cancellation after publication retains the complete key for
				// the next invocation rather than regenerating its identity.
				if len(entries) != 1 || entries[0].Name() != keyFile {
					t.Fatal("completed publication lost its private registration")
				}
				if _, err := open(t.Context(), home, bytes.NewReader(nil)); err != nil {
					t.Fatalf("next invocation could not read the published key: %v", err)
				}
			} else if len(entries) != 0 {
				t.Fatal("cancellation before publication left credential files")
			}
		})
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if identity, err := Open(cancelled, ""); !errors.Is(err, context.Canceled) || identity != nil {
		t.Fatalf("default configuration ignored cancellation: %v", err)
	}
}

func TestRegistrationDoesNotOverwriteExistingTemporaryFile(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, "feature-drive")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ".owner-01010101010101010101010101010101")
	retained := []byte("another registration's temporary file")
	if err := os.WriteFile(path, retained, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := open(t.Context(), home, bytes.NewReader(bytes.Repeat([]byte{1}, 64))); !errors.Is(err, os.ErrExist) {
		t.Fatalf("occupied temporary path was reused: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, retained) {
		t.Fatalf("existing temporary file was altered: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, keyFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed registration published a key")
	}
}
