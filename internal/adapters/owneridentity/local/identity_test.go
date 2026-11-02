package localidentity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/conformancetest"
)

func TestRegistrationPersistsIdentityAndSignsWithItsPublicKey(t *testing.T) {
	home := t.TempDir()
	first, err := open(t.Context(), home, bytes.NewReader(bytes.Repeat([]byte{7}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := open(t.Context(), home, bytes.NewReader(nil))
	if err != nil || second.Registration() != first.Registration() {
		t.Fatalf("restart changed owner or consumed fresh entropy: %v", err)
	}
	registration := first.Registration()
	conformancetest.RunOwnerIdentity(t, second, featuredrive.ExecutionAssignment{Domain: "local", OwnerID: registration.OwnerID}, featuredrive.Owner{Domain: "local", ID: registration.OwnerID})
	public, err := hex.DecodeString(registration.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || registration.SchemaVersion != "feature-drive-local-owner/v1" || registration.Domain != "local" {
		t.Fatalf("invalid public registration: %+v, %v", registration, err)
	}
	body := []byte("canonical transition")
	signature, err := second.Sign(t.Context(), body)
	if err != nil || !ed25519.Verify(public, body, signature) || ed25519.Verify(public, []byte("changed transition"), signature) {
		t.Fatalf("persistent key did not authenticate the transition: %v", err)
	}
	for _, assignment := range []featuredrive.ExecutionAssignment{
		{Domain: "local", OwnerID: registration.OwnerID},
		{Domain: "remote", OwnerID: registration.OwnerID},
		{Domain: "local", OwnerID: "unregistered"},
		{Domain: "local"},
		{},
	} {
		got := second.Authenticate(t.Context(), assignment)
		if assignment.Domain == "local" && assignment.OwnerID == registration.OwnerID {
			if got != (featuredrive.Owner{Domain: "local", ID: registration.OwnerID}) {
				t.Fatalf("registered owner refused: %+v", got)
			}
		} else if got != (featuredrive.Owner{}) {
			t.Fatalf("foreign assignment authenticated: %+v", got)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := second.Sign(cancelled, body); !errors.Is(err, context.Canceled) || second.Authenticate(cancelled, featuredrive.ExecutionAssignment{Domain: "local", OwnerID: registration.OwnerID}) != (featuredrive.Owner{}) {
		t.Fatal("cancelled caller retained signing or authentication")
	}
	for name, want := range map[string]os.FileMode{"feature-drive": 0o700, filepath.Join("feature-drive", keyFile): 0o600} {
		info, err := os.Stat(filepath.Join(home, name))
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("private registration permissions for %s: %v", name, err)
		}
	}
}

func TestConcurrentRegistrationUsesOneCompleteKey(t *testing.T) {
	home := t.TempDir()
	const callers = 8
	identities := make(chan *Identity, callers)
	errorsFound := make(chan error, callers)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for index := range callers {
		workers.Go(func() {
			<-start
			identity, err := open(t.Context(), home, bytes.NewReader(bytes.Repeat([]byte{byte(index + 1)}, 64)))
			identities <- identity
			errorsFound <- err
		})
	}
	close(start)
	workers.Wait()
	close(identities)
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	var winner Registration
	for identity := range identities {
		if winner.OwnerID == "" {
			winner = identity.Registration()
		}
		if identity.Registration() != winner {
			t.Fatal("concurrent processes registered different owners")
		}
	}
	entries, err := os.ReadDir(filepath.Join(home, "feature-drive"))
	if err != nil || len(entries) != 1 || entries[0].Name() != keyFile {
		t.Fatalf("registration retained incomplete or temporary credentials: %v, %v", entries, err)
	}
}

func TestExistingInvalidRegistrationIsRefusedAndPreserved(t *testing.T) {
	for _, defect := range []string{"short", "oversized", "corrupt", "public permissions", "symlink", "directory", "public directory", "directory symlink"} {
		t.Run(defect, func(t *testing.T) {
			home := t.TempDir()
			directory := filepath.Join(home, "feature-drive")
			if err := os.Mkdir(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
			path := filepath.Join(directory, keyFile)
			mode := os.FileMode(0o600)
			switch defect {
			case "short":
				key = key[:32]
			case "oversized":
				key = append(key, 0)
			case "corrupt":
				key[0] ^= 1
			case "public permissions":
				mode = 0o644
			}
			if err := os.WriteFile(path, key, mode); err != nil {
				t.Fatal(err)
			}
			// A restrictive umask must not repair the public-permissions fixture.
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			switch defect {
			case "symlink", "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if defect == "directory" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					target := filepath.Join(home, "external-key")
					if err := os.WriteFile(target, key, 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
			case "public directory":
				if err := os.Chmod(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			case "directory symlink":
				target := filepath.Join(home, "external-directory")
				if err := os.Rename(directory, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, directory); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := open(t.Context(), home, bytes.NewReader(nil)); !errors.Is(err, ErrRegistration) {
				t.Fatalf("%s was accepted or regenerated: %v", defect, err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatalf("refused registration was modified: %v", err)
			}
			if defect != "directory" {
				body, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(body, key) {
					t.Fatalf("refusal changed credential bytes: %v", err)
				}
			}
		})
	}
}

func TestRegistrationFailuresPublishNoKey(t *testing.T) {
	for _, size := range []int{0, 32} {
		home := t.TempDir()
		if _, err := open(t.Context(), home, bytes.NewReader(bytes.Repeat([]byte{1}, size))); !errors.Is(err, io.EOF) {
			t.Fatalf("entropy failure not reported: %v", err)
		}
		entries, err := os.ReadDir(filepath.Join(home, "feature-drive"))
		if err != nil || len(entries) != 0 {
			t.Fatalf("failed generation published credentials: %v, %v", entries, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	home := filepath.Join(t.TempDir(), "uncreated")
	if _, err := Open(cancelled, home); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled registration: %v", err)
	}
	if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled registration touched configuration")
	}
	occupied := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(occupied, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(t.Context(), occupied); err == nil {
		t.Fatal("file used as configuration directory")
	}
}
