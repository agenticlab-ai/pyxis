// Package credentialstore keeps routing-evidence signing seeds outside the
// repository and worker filesystem. The operating-system credential store is
// the only production implementation; tests use the small Store interface.
package credentialstore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

const (
	Service = "dev.pyxis.routing-evidence"
	seedLen = ed25519.SeedSize
)

var ErrCredentialUnavailable = errors.New("routing credential is unavailable")

// Store is a narrow secret boundary. Implementations must never persist the
// seed in repository files, command arguments, logs, or child environments.
type Store interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
}

// Key identifies one Ed25519 signing key while keeping its private seed in the
// Store. Account is repository-scoped and purpose-scoped by the caller.
type Key struct {
	Account   string `json:"account"`
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

func Generate(ctx context.Context, store Store, account, purpose string) (Key, error) {
	account, purpose = strings.TrimSpace(account), strings.TrimSpace(purpose)
	if store == nil || account == "" || purpose == "" || strings.ContainsAny(account+purpose, "\x00\r\n") {
		return Key{}, errors.New("routing credential: store, account, and purpose are required")
	}
	seed := make([]byte, seedLen)
	if _, err := rand.Read(seed); err != nil {
		return Key{}, fmt.Errorf("routing credential: generate seed: %w", err)
	}
	defer clear(seed)
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(append([]byte(purpose+"\x00"), public...))
	key := Key{Account: account, KeyID: purpose + ":" + hex.EncodeToString(digest[:8]), PublicKey: hex.EncodeToString(public)}
	if err := store.Put(ctx, account, seed); err != nil {
		return Key{}, fmt.Errorf("routing credential: store seed: %w", err)
	}
	return key, nil
}

func Load(ctx context.Context, store Store, key Key) (ed25519.PrivateKey, error) {
	if store == nil || strings.TrimSpace(key.Account) == "" || strings.TrimSpace(key.KeyID) == "" || strings.TrimSpace(key.PublicKey) == "" {
		return nil, errors.New("routing credential: key identity is incomplete")
	}
	seed, err := store.Get(ctx, key.Account)
	if err != nil {
		return nil, fmt.Errorf("routing credential: load seed: %w", err)
	}
	defer clear(seed)
	if len(seed) != seedLen {
		return nil, fmt.Errorf("%w: seed has %d bytes, want %d", ErrCredentialUnavailable, len(seed), seedLen)
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := private.Public().(ed25519.PublicKey)
	if hex.EncodeToString(public) != key.PublicKey {
		clear(private)
		return nil, fmt.Errorf("%w: stored seed does not match authorized public key", ErrCredentialUnavailable)
	}
	return private, nil
}

func Account(repository, purpose string) (string, error) {
	repository, purpose = strings.TrimSpace(repository), strings.TrimSpace(purpose)
	if repository == "" || purpose == "" || strings.ContainsAny(repository+purpose, "\x00\r\n") || strings.Contains(repository, "#") {
		return "", errors.New("routing credential: repository and purpose are required")
	}
	return repository + "#" + purpose, nil
}

// LoadProducerCapability resolves exactly one active policy key for class and
// proves the local credential matches its reviewed public key. Event kinds and
// roles remain caller-minimized; this helper never broadens them from policy.
func LoadProducerCapability(ctx context.Context, store Store, policy routingevidence.ProducerPolicy, class routingevidence.ProducerClass, kinds []routingevidence.EventKind, roles []string) (routingevidence.SigningCapability, error) {
	var keys []routingevidence.KeyGeneration
	for _, key := range policy.Keys {
		if key.ProducerClass == class && key.RevocationReference == "" && contains(key.PolicyActivationPoints, policy.ActivationPoint) {
			keys = append(keys, key)
		}
	}
	if len(keys) != 1 {
		return routingevidence.SigningCapability{}, fmt.Errorf("routing credential: policy has %d active %s keys, want exactly one", len(keys), class)
	}
	account, err := Account(policy.Repository, "producer:"+string(class))
	if err != nil {
		return routingevidence.SigningCapability{}, err
	}
	private, err := Load(ctx, store, Key{Account: account, KeyID: keys[0].KeyID, PublicKey: keys[0].PublicKey})
	if err != nil {
		return routingevidence.SigningCapability{}, err
	}
	return routingevidence.SigningCapability{Repository: policy.Repository, EvidenceRef: policy.EvidenceRef, ProducerClass: class,
		ImplementationGeneration: keys[0].ImplementationGeneration, KeyID: keys[0].KeyID, PrivateKey: private,
		EventKinds: append([]routingevidence.EventKind(nil), kinds...), Roles: append([]string(nil), roles...)}, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
