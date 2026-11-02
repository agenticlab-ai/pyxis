// Package localidentity authenticates the local execution owner with a
// persistent private registration in the user's Pyxis configuration home.
package localidentity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

const keyFile = "owner.ed25519"

var ErrRegistration = errors.New("local owner: invalid private registration")

type Registration struct {
	SchemaVersion string `json:"schema_version"`
	Domain        string `json:"domain"`
	OwnerID       string `json:"owner_id"`
	PublicKey     string `json:"public_key"`
}

type Identity struct {
	key          ed25519.PrivateKey
	registration Registration
}

var _ featuredrive.OwnerIdentity = (*Identity)(nil)

// Open creates the registration once. Concurrent callers read the winner;
// an existing invalid credential is refused rather than silently replaced.
func Open(ctx context.Context, configHome string) (*Identity, error) {
	if configHome == "" {
		directory, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("local owner configuration home: %w", err)
		}
		configHome = filepath.Join(directory, "pyxis")
	}
	return open(ctx, configHome, rand.Reader)
}

func open(ctx context.Context, configHome string, entropy io.Reader) (*Identity, error) {
	return openWithSync(ctx, configHome, entropy, syncRegistration)
}

func openWithSync(ctx context.Context, configHome string, entropy io.Reader, syncDirectory func(context.Context, *os.Root, string) error) (identity *Identity, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory := filepath.Join(configHome, "feature-drive")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create local owner directory: %w", err)
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return nil, ErrRegistration
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open local owner directory: %w", err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrRegistration
	}
	key, err := readKey(root)
	if errors.Is(err, os.ErrNotExist) {
		_, generated, generateErr := ed25519.GenerateKey(entropy)
		if generateErr != nil {
			return nil, fmt.Errorf("generate local owner registration: %w", generateErr)
		}
		if err := publishKey(ctx, root, generated, entropy); err != nil {
			return nil, fmt.Errorf("publish local owner registration: %w", err)
		}
		key, err = readKey(root)
	}
	if err != nil {
		return nil, fmt.Errorf("read local owner registration: %w", err)
	}
	if err := syncDirectory(ctx, root, directory); err != nil {
		return nil, fmt.Errorf("sync local owner registration: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	public := key.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(public)
	return &Identity{key: key, registration: Registration{SchemaVersion: "feature-drive-local-owner/v1", Domain: "local", OwnerID: "local-" + hex.EncodeToString(digest[:]), PublicKey: hex.EncodeToString(public)}}, nil
}

func readKey(root *os.Root) (key ed25519.PrivateKey, err error) {
	info, err := root.Lstat(keyFile)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != ed25519.PrivateKeySize {
		return nil, ErrRegistration
	}
	file, err := root.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o600 {
		return nil, ErrRegistration
	}
	body, err := io.ReadAll(io.LimitReader(file, ed25519.PrivateKeySize+1))
	if err != nil {
		return nil, err
	}
	if len(body) != ed25519.PrivateKeySize || !bytes.Equal(body, ed25519.NewKeyFromSeed(body[:ed25519.SeedSize])) {
		return nil, ErrRegistration
	}
	return ed25519.PrivateKey(body), nil
}

func publishKey(ctx context.Context, root *os.Root, key ed25519.PrivateKey, entropy io.Reader) (err error) {
	var nonce [16]byte
	if _, err := io.ReadFull(entropy, nonce[:]); err != nil {
		return err
	}
	name := ".owner-" + hex.EncodeToString(nonce[:])
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Remove(name)) }()
	_, writeErr := file.Write(key)
	if err := errors.Join(writeErr, file.Sync(), file.Close(), ctx.Err()); err != nil {
		return err
	}
	// A create-only link publishes fully written bytes without overwriting the
	// key another process may already have installed.
	if err := root.Link(name, keyFile); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

// Persist the key's directory entry through the anchored root, then the
// ancestor entries that may have been created by MkdirAll.
func syncRegistration(ctx context.Context, root *os.Root, directory string) error {
	return syncRegistrationWith(ctx, root, directory, (*os.File).Sync)
}

func syncRegistrationWith(ctx context.Context, root *os.Root, directory string, syncFile func(*os.File) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	if err := errors.Join(syncFile(file), file.Close()); err != nil {
		return err
	}
	for parent := filepath.Dir(directory); ; parent = filepath.Dir(parent) {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, err := os.Open(parent)
		if err != nil {
			return err
		}
		if err := errors.Join(syncFile(file), file.Close()); err != nil {
			return err
		}
		if filepath.Dir(parent) == parent {
			return nil
		}
	}
}

func (i *Identity) Registration() Registration { return i.registration }

func (i *Identity) Authenticate(ctx context.Context, assignment featuredrive.ExecutionAssignment) featuredrive.Owner {
	if ctx.Err() != nil || assignment.Domain != "local" || assignment.OwnerID != i.registration.OwnerID {
		return featuredrive.Owner{}
	}
	return featuredrive.Owner{Domain: "local", ID: i.registration.OwnerID}
}

func (i *Identity) Sign(ctx context.Context, body []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	signature := ed25519.Sign(i.key, body)
	return signature, ctx.Err()
}
