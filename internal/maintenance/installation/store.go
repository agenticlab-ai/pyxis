package installation

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/statedir"
)

const manifestName = "manifest.json"
const maxManifestBytes = 1 << 20

type Store struct {
	repository, repositoryID, stateRoot string
	rename                              func(*os.Root, string, string) error
	syncDir                             func(*os.Root) error
}

type Transaction struct {
	store  *Store
	ctx    context.Context
	active bool
}

type priorState struct {
	SchemaVersion string                `json:"schema_version"`
	TablePresent  bool                  `json:"table_present"`
	Table         []byte                `json:"table"`
	Manifest      *InstallationManifest `json:"manifest,omitempty"`
}

type UncertainError struct {
	Manifest InstallationManifest
	Cause    error
}

func (e *UncertainError) Error() string {
	return fmt.Sprintf("%v: %v", ErrPublicationUncertain, e.Cause)
}
func (e *UncertainError) Unwrap() error        { return e.Cause }
func (e *UncertainError) Is(target error) bool { return target == ErrPublicationUncertain }

func Open(repository string) (*Store, error) {
	stateRoot, err := statedir.Root()
	if err != nil {
		return nil, err
	}
	return open(repository, stateRoot)
}

func open(repository, stateRoot string) (*Store, error) {
	canonical, err := filepath.Abs(repository)
	if err != nil {
		return nil, fmt.Errorf("resolve maintenance repository: %w", err)
	}
	canonical, err = filepath.EvalSymlinks(canonical)
	if err != nil {
		return nil, fmt.Errorf("canonicalize maintenance repository: %w", err)
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return nil, errors.New("maintenance repository must be an existing directory")
	}
	if !filepath.IsAbs(stateRoot) {
		return nil, errors.New("maintenance state root must be absolute")
	}
	identity := Repository(filepath.Clean(canonical))
	return &Store{repository: identity.Root, repositoryID: identity.SHA256, stateRoot: filepath.Clean(stateRoot), rename: (*os.Root).Rename, syncDir: syncDirectory}, nil
}

func (store *Store) Repository() RepositoryIdentity { return Repository(store.repository) }

func (store *Store) ManifestPath() string {
	return filepath.Join(store.stateRoot, "maintenance", "installations", store.repositoryID, manifestName)
}

// Load observes the atomically published manifest without creating local state.
func (store *Store) Load(ctx context.Context) (InstallationManifest, error) {
	if err := ctx.Err(); err != nil {
		return InstallationManifest{}, err
	}
	manifest, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		return InstallationManifest{}, store.installRequired("absent", nil)
	}
	return manifest, err
}

// WithTransaction serializes every per-user scheduler/authority transition.
// The transaction must not be retained after run returns.
func (store *Store) WithTransaction(ctx context.Context, run func(*Transaction) error) error {
	if run == nil {
		return errors.New("maintenance installation transaction callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return store.withLock(ctx, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		transaction := &Transaction{store: store, ctx: ctx, active: true}
		defer func() { transaction.active = false }()
		return run(transaction)
	})
}

func (transaction *Transaction) Load() (InstallationManifest, error) {
	if err := transaction.admit(); err != nil {
		return InstallationManifest{}, err
	}
	return transaction.store.Load(transaction.ctx)
}

func (transaction *Transaction) Publish(manifest InstallationManifest) error {
	if err := transaction.admit(); err != nil {
		return err
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	if manifest.Repository != transaction.store.Repository() {
		return invalid("repository identity does not match this store")
	}
	data, err := encode(manifest)
	if err != nil {
		return err
	}
	return transaction.store.publish(transaction.ctx, manifest, data)
}

func (transaction *Transaction) Ready() (ReadyBinding, error) {
	if err := transaction.admit(); err != nil {
		return ReadyBinding{}, err
	}
	manifest, err := transaction.store.load()
	if err != nil {
		return ReadyBinding{}, transaction.store.installRequired("unreadable", err)
	}
	if manifest.State != StateReady {
		return ReadyBinding{}, transaction.store.installRequired(string(manifest.State), nil)
	}
	return proof(manifest)
}

func (transaction *Transaction) ValidateReady(saved ReadyBinding) error {
	current, err := transaction.Ready()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, saved) {
		return fmt.Errorf("%w: saved %q, current %q", ErrGenerationReplaced, saved.Generation, current.Generation)
	}
	return nil
}

// Backup retains the exact prior cron/manifest pair inside the restricted
// installation directory while the caller owns the installation lock.
func (transaction *Transaction) Backup(table []byte, tablePresent bool, manifest *InstallationManifest) (string, error) {
	if err := transaction.admit(); err != nil {
		return "", err
	}
	snapshot := priorState{SchemaVersion: "pyxis-maintenance-prior/v1", TablePresent: tablePresent, Table: bytes.Clone(table), Manifest: manifest}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("encode maintenance prior state: %w", err)
	}
	directory, err := os.OpenRoot(filepath.Dir(transaction.store.ManifestPath()))
	if err != nil {
		return "", fmt.Errorf("open maintenance backup directory: %w", err)
	}
	defer func() { _ = directory.Close() }() // Backup writes and directory sync finish before close.
	name := "prior-" + rand.Text() + ".json"
	file, err := directory.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return "", fmt.Errorf("create maintenance prior-state backup: %w", err)
	}
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		_ = directory.Remove(name) // Best-effort cleanup cannot make a failed backup authoritative.
		return "", fmt.Errorf("persist maintenance prior-state backup: %w", err)
	}
	if err := transaction.store.syncDir(directory); err != nil {
		return filepath.Join(filepath.Dir(transaction.store.ManifestPath()), name), fmt.Errorf("sync maintenance prior-state backup: %w", err)
	}
	return filepath.Join(filepath.Dir(transaction.store.ManifestPath()), name), nil
}

// Restore publishes a prior manifest (or its verified absence) with a bounded
// recovery context even when the transaction's operation context was canceled.
func (transaction *Transaction) Restore(ctx context.Context, manifest *InstallationManifest) error {
	if transaction == nil || !transaction.active {
		return errors.New("maintenance installation transaction is not active")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if manifest != nil {
		if err := manifest.Validate(); err != nil || manifest.Repository != transaction.store.Repository() {
			return errors.Join(invalid("invalid prior manifest"), err)
		}
		data, err := encode(*manifest)
		if err != nil {
			return err
		}
		if err := transaction.store.publish(ctx, *manifest, data); err != nil {
			return err
		}
		restored, err := transaction.store.load()
		if err != nil || !reflect.DeepEqual(restored, *manifest) {
			return errors.Join(errors.New("restored maintenance manifest did not verify"), err)
		}
		return nil
	}
	directory, err := os.OpenRoot(filepath.Dir(transaction.store.ManifestPath()))
	if err != nil {
		return fmt.Errorf("open maintenance installation directory: %w", err)
	}
	defer func() { _ = directory.Close() }() // Restore verifies the manifest before this handle closes.
	if err := directory.Remove(manifestName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove maintenance manifest: %w", err)
	}
	if err := transaction.store.syncDir(directory); err != nil {
		return err
	}
	if _, err := transaction.store.load(); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(errors.New("maintenance manifest absence did not verify"), err)
	}
	return nil
}

func (transaction *Transaction) admit() error {
	if transaction == nil || !transaction.active {
		return errors.New("maintenance installation transaction is not active")
	}
	return transaction.ctx.Err()
}

func (store *Store) load() (InstallationManifest, error) {
	root, err := os.OpenRoot(filepath.Dir(store.ManifestPath()))
	if err != nil {
		return InstallationManifest{}, err
	}
	defer func() { _ = root.Close() }() // A read-only directory handle has no pending writes.
	file, err := root.OpenFile(manifestName, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return InstallationManifest{}, err
	}
	defer func() { _ = file.Close() }() // A read-only descriptor has no pending writes.
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return InstallationManifest{}, invalid("manifest must be a 0600 regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil || len(data) > maxManifestBytes {
		return InstallationManifest{}, fmt.Errorf("read maintenance manifest within 1 MiB: %w", errors.Join(ErrInvalidManifest, err))
	}
	manifest, err := decode(data)
	if err != nil {
		return InstallationManifest{}, err
	}
	if manifest.Repository != store.Repository() {
		return InstallationManifest{}, invalid("manifest repository identity does not match this store")
	}
	return manifest, nil
}

func (store *Store) publish(ctx context.Context, manifest InstallationManifest, data []byte) error {
	renamed, err := store.publishFile(ctx, manifestName, data)
	if renamed && err != nil {
		return &UncertainError{Manifest: cloneManifest(manifest), Cause: err}
	}
	return err
}

func (store *Store) publishFile(ctx context.Context, target string, data []byte) (bool, error) {
	directory, err := os.OpenRoot(filepath.Dir(store.ManifestPath()))
	if err != nil {
		return false, fmt.Errorf("open maintenance installation directory: %w", err)
	}
	defer func() { _ = directory.Close() }() // The directory handle carries no buffered file writes.
	name := "." + target + "-" + rand.Text() + ".tmp"
	temporary, err := directory.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return false, fmt.Errorf("create maintenance manifest temporary file: %w", err)
	}
	remove := true
	defer func() {
		_ = temporary.Close() // Publication checks close explicitly; this also cleans up earlier failures.
		if remove {
			_ = directory.Remove(name) // An unpublished private temporary file grants no execution authority.
		}
	}()
	if _, err := io.Copy(temporary, bytes.NewReader(data)); err != nil {
		return false, fmt.Errorf("write maintenance manifest: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return false, fmt.Errorf("sync maintenance manifest: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return false, fmt.Errorf("close maintenance manifest: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := store.rename(directory, name, target); err != nil {
		return false, fmt.Errorf("replace maintenance state: %w", err)
	}
	remove = false
	return true, store.syncDir(directory)
}

func (store *Store) withLock(ctx context.Context, run func() error) error {
	if err := store.prepare(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Join(store.stateRoot, "maintenance"))
	if err != nil {
		return fmt.Errorf("open maintenance state: %w", err)
	}
	defer func() { _ = root.Close() }() // The directory handle carries no buffered writes.
	file, err := root.OpenFile("transaction.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if errors.Is(err, os.ErrExist) {
		file, err = root.OpenFile("transaction.lock", os.O_RDWR|syscall.O_NOFOLLOW, 0)
	}
	if err != nil {
		return fmt.Errorf("open maintenance transaction lock: %w", err)
	}
	defer func() { _ = file.Close() }() // No file data is written; closing also releases the owned lock.
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return errors.New("maintenance transaction lock must be a 0600 regular file")
	}
	for {
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return fmt.Errorf("lock maintenance installation transaction: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }() // The following close releases the lock if unlock fails.
	return run()
}

func (store *Store) prepare() error {
	if err := os.MkdirAll(store.stateRoot, 0o700); err != nil {
		return fmt.Errorf("create pyxis state root: %w", err)
	}
	root, err := os.OpenRoot(store.stateRoot)
	if err != nil {
		return fmt.Errorf("open pyxis state root: %w", err)
	}
	defer func() { _ = root.Close() }() // The directory handle carries no buffered file writes.
	relative := filepath.Join("maintenance", "installations", store.repositoryID)
	if err := root.MkdirAll(relative, 0o700); err != nil {
		return fmt.Errorf("create maintenance installation state: %w", err)
	}
	for _, name := range []string{"maintenance", filepath.Join("maintenance", "installations"), relative} {
		info, err := root.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("maintenance state directory %s must be private", name)
		}
	}
	return nil
}

func (store *Store) installRequired(state string, cause error) error {
	message := fmt.Errorf("%w: installation is %s for %q; run pyxis maintenance install --repo %q", ErrInstallationRequired, state, store.repository, store.repository)
	return errors.Join(message, cause)
}

func encode(manifest InstallationManifest) ([]byte, error) {
	data, err := json.Marshal(manifest)
	if err != nil || len(data)+1 > maxManifestBytes {
		return nil, errors.Join(invalid("encode JSON within 1 MiB"), err)
	}
	return append(data, '\n'), nil
}

func decode(data []byte) (InstallationManifest, error) {
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil || !utf8.Valid(data) {
		return InstallationManifest{}, errors.Join(invalid("decode JSON requires valid UTF-8 and unique object keys"), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest InstallationManifest
	if err := decoder.Decode(&manifest); err != nil {
		return InstallationManifest{}, invalid("decode closed JSON: %v", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return InstallationManifest{}, invalid("expected exactly one JSON value")
	}
	if err := manifest.Validate(); err != nil {
		return InstallationManifest{}, err
	}
	return manifest, nil
}

func uniqueJSON(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			// Tags use lowercase ASCII. Reject aliases before encoding/json's
			// case folding can map an undeclared spelling onto an authority field.
			if !ok || name != strings.ToLower(name) || len(name) != utf8.RuneCountInString(name) || seen[name] {
				return fmt.Errorf("duplicate or invalid object key %q", name)
			}
			seen[name] = true
		}
		if err := uniqueJSON(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func proof(manifest InstallationManifest) (ReadyBinding, error) {
	data, err := encode(manifest)
	if err != nil {
		return ReadyBinding{}, err
	}
	return ReadyBinding{Generation: manifest.Generation, Repository: manifest.Repository, Source: manifest.Source, Host: manifest.Host, Schedules: slices.Clone(manifest.Schedules), ManifestSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}, nil
}

func cloneManifest(manifest InstallationManifest) InstallationManifest {
	manifest.Schedules = slices.Clone(manifest.Schedules)
	return manifest
}

func syncDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }() // Directory.Sync reports durability failures; closing has no buffered writes.
	return directory.Sync()
}
