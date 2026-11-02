package localidentity

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistrationRefusesAnnouncementUntilDirectorySyncSucceeds(t *testing.T) {
	home := filepath.Join(t.TempDir(), "new", "nested", "config")
	failure := errors.New("directory sync unavailable")
	calls := 0
	identity, err := openWithSync(t.Context(), home, bytes.NewReader(bytes.Repeat([]byte{1}, 64)), func(ctx context.Context, root *os.Root, directory string) error {
		calls++
		if directory != filepath.Join(home, "feature-drive") {
			t.Fatal("sync targeted an unrelated directory")
		}
		if _, err := readKey(root); err != nil {
			t.Fatalf("sync ran before complete key publication: %v", err)
		}
		if err := syncRegistration(ctx, root, directory); err != nil {
			t.Fatal(err)
		}
		return failure
	})
	if identity != nil || !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("registration announced without durable publication: identity=%v error=%v syncs=%d", identity != nil, err, calls)
	}
	retained, err := os.ReadFile(filepath.Join(home, "feature-drive", keyFile))
	if err != nil {
		t.Fatal(err)
	}
	// A retry must sync the already-published key and preserve its identity,
	// rather than generating a replacement or skipping the durability barrier.
	calls = 0
	recovered, err := openWithSync(t.Context(), home, bytes.NewReader(nil), func(ctx context.Context, root *os.Root, directory string) error {
		calls++
		return syncRegistration(ctx, root, directory)
	})
	if err != nil || recovered == nil || calls != 1 {
		t.Fatalf("durable retry failed: %v, syncs=%d", err, calls)
	}
	if got, err := os.ReadFile(filepath.Join(home, "feature-drive", keyFile)); err != nil || !bytes.Equal(got, retained) {
		t.Fatal("directory-sync retry changed the registration")
	}
}

func TestRegistrationRefusesUnavailableDirectoryAndSyncCancellation(t *testing.T) {
	for _, failure := range []string{"closed root", "missing parent", "cancelled sync"} {
		t.Run(failure, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "config")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			identity, err := openWithSync(ctx, home, bytes.NewReader(bytes.Repeat([]byte{1}, 64)), func(ctx context.Context, root *os.Root, directory string) error {
				switch failure {
				case "closed root":
					if err := root.Close(); err != nil {
						t.Fatal(err)
					}
				case "missing parent":
					if err := os.Rename(home, home+"-retained"); err != nil {
						t.Fatal(err)
					}
				case "cancelled sync":
					cancel()
				}
				return syncRegistration(ctx, root, directory)
			})
			if err == nil || identity != nil {
				t.Fatalf("unavailable durability barrier announced an owner: %v", err)
			}
			if failure == "cancelled sync" && !errors.Is(err, context.Canceled) {
				t.Fatalf("sync cancellation lost its error: %v", err)
			}
		})
	}
}

func TestRegistrationRefusesDirectorySyncIOFailure(t *testing.T) {
	for _, failure := range []string{"key directory", "parent directory", "cancel after key directory", "cancel after durable sync"} {
		t.Run(failure, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "new", "nested", "config")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			identity, err := openWithSync(ctx, home, bytes.NewReader(bytes.Repeat([]byte{1}, 64)), func(ctx context.Context, root *os.Root, directory string) error {
				err := syncRegistrationWith(ctx, root, directory, func(file *os.File) error {
					path := filepath.Clean(file.Name())
					if (failure == "key directory" && path == directory) || (failure == "parent directory" && path == home) {
						// Induce a real fsync error, rather than replacing the
						// complete durability operation with a sentinel.
						if err := file.Close(); err != nil {
							t.Fatal(err)
						}
						return file.Sync()
					}
					err := file.Sync()
					if failure == "cancel after key directory" && path == directory {
						cancel()
					}
					return err
				})
				if failure == "cancel after durable sync" {
					cancel()
				}
				return err
			})
			want := os.ErrClosed
			if failure == "cancel after key directory" || failure == "cancel after durable sync" {
				want = context.Canceled
			}
			if identity != nil || !errors.Is(err, want) {
				t.Fatalf("failed durability operation announced an owner or lost its cause: identity=%v error=%v", identity != nil, err)
			}
			retained, err := os.ReadFile(filepath.Join(home, "feature-drive", keyFile))
			if err != nil {
				t.Fatal(err)
			}
			if recovered, err := open(t.Context(), home, bytes.NewReader(nil)); err != nil || recovered == nil {
				t.Fatalf("durability failure prevented a retry with the retained credential: %v", err)
			}
			if got, err := os.ReadFile(filepath.Join(home, "feature-drive", keyFile)); err != nil || !bytes.Equal(got, retained) {
				t.Fatal("durability failure regenerated the owner credential")
			}
		})
	}
}
