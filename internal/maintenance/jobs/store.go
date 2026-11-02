package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Store struct {
	root, path, lockPath string
	beforeWrite          func() error
	beforeLock           func() error
	rename               func(*os.Root, string, string) error
	syncDir              func(*os.Root) error
}
type UnconfirmedError struct {
	Bytes []byte
	Cause error
}

func (e *UnconfirmedError) Error() string        { return fmt.Sprintf("%v: %v", ErrUnconfirmed, e.Cause) }
func (e *UnconfirmedError) Unwrap() error        { return e.Cause }
func (e *UnconfirmedError) Is(target error) bool { return target == ErrUnconfirmed }

func New(root string) (*Store, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, invalid("canonicalize repository root: %v", err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, invalid("absolute repository root: %v", err)
	}
	if info, statErr := os.Stat(canonical); statErr != nil || !info.IsDir() {
		return nil, invalid("repository root is not a directory")
	}
	store := &Store{root: canonical, path: filepath.Join(canonical, ".pyxis/maintenance/jobs.yaml"), lockPath: filepath.Join(canonical, ".pyxis/state/maintenance/jobs.lock"), rename: (*os.Root).Rename, syncDir: syncDirectory}
	if err := store.rejectSymlinks(); err != nil {
		return nil, err
	}
	return store, nil
}
func (store *Store) Root() string { return store.root }

// Load returns the complete validated desired-policy document without writing.
func (store *Store) Load(ctx context.Context) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	document, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	document.Jobs = append([]Job{}, document.Jobs...)
	for index := range document.Jobs {
		document.Jobs[index] = clone(document.Jobs[index])
	}
	return document, nil
}

func (store *Store) Read(ctx context.Context, id string) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	if !idPattern.MatchString(id) {
		return Job{}, invalid("invalid job id %q", id)
	}
	document, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		return Job{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return Job{}, err
	}
	index := find(document.Jobs, id)
	if index < 0 {
		return Job{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return clone(document.Jobs[index]), nil
}
func (store *Store) List(ctx context.Context) ([]Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	document, err := store.load()
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := make([]Job, len(document.Jobs))
	for index := range document.Jobs {
		result[index] = clone(document.Jobs[index])
	}
	return result, nil
}
func (store *Store) Create(ctx context.Context, source *SourceBinding, job Job) error {
	return store.mutate(ctx, "create", job.ID, job, source)
}
func (store *Store) Update(ctx context.Context, id string, job Job) error {
	if !idPattern.MatchString(id) || !idPattern.MatchString(job.ID) {
		return invalid("invalid job identity")
	}
	if id != job.ID {
		return fmt.Errorf("%w: job identity cannot change", ErrConflict)
	}
	return store.mutate(ctx, "update", id, job, nil)
}
func (store *Store) Delete(ctx context.Context, id string) error {
	if !idPattern.MatchString(id) {
		return invalid("invalid job id %q", id)
	}
	return store.mutate(ctx, "delete", id, Job{}, nil)
}

func (store *Store) mutate(ctx context.Context, operation, id string, job Job, source *SourceBinding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := store.rejectSymlinks(); err != nil {
		return err
	}
	return store.withLock(ctx, func() error {
		document, err := store.load()
		missing := errors.Is(err, os.ErrNotExist)
		if err != nil && !missing {
			return err
		}
		index := find(document.Jobs, id)
		switch operation {
		case "create":
			if missing {
				if source == nil || !validSource(*source) {
					return invalid("valid source binding required for initialization")
				}
				document = Document{SchemaVersion: SchemaVersion, Source: *source, Jobs: []Job{}}
			} else if source != nil && *source != document.Source {
				return fmt.Errorf("%w: source binding differs", ErrConflict)
			}
			if index >= 0 {
				return fmt.Errorf("%w: %s", ErrAlreadyExists, id)
			}
			document.Jobs = append(document.Jobs, clone(job))
		case "update":
			if index < 0 {
				return fmt.Errorf("%w: %s", ErrNotFound, id)
			}
			document.Jobs[index] = clone(job)
		case "delete":
			if index < 0 {
				return fmt.Errorf("%w: %s", ErrNotFound, id)
			}
			document.Jobs = append(document.Jobs[:index], document.Jobs[index+1:]...)
		}
		data, err := Encode(document)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return store.commit(ctx, data)
	})
}
func (store *Store) load() (Document, error) {
	if err := store.rejectSymlinks(); err != nil {
		return Document{}, err
	}
	root, err := os.OpenRoot(store.root)
	if err != nil {
		return Document{}, fmt.Errorf("open jobs repository: %w", err)
	}
	defer func() { _ = root.Close() }() // Closing the read-only directory handle has no durable effect.
	data, err := root.ReadFile(".pyxis/maintenance/jobs.yaml")
	if err != nil {
		return Document{}, err
	}
	return Decode(data)
}

func (store *Store) commit(ctx context.Context, data []byte) error {
	root, err := os.OpenRoot(store.root)
	if err != nil {
		return fmt.Errorf("open jobs repository: %w", err)
	}
	defer func() { _ = root.Close() }() // Operations own this directory handle and finish before it closes.
	if err := root.MkdirAll(".pyxis/maintenance", 0o755); err != nil {
		return fmt.Errorf("create jobs directory: %w", err)
	}
	directory, err := root.OpenRoot(".pyxis/maintenance")
	if err != nil {
		return fmt.Errorf("open jobs directory: %w", err)
	}
	defer func() { _ = directory.Close() }() // Operations own this pinned directory handle.
	if err := store.rejectSymlinks(); err != nil {
		return err
	}
	if store.beforeWrite != nil {
		if err := store.beforeWrite(); err != nil {
			return fmt.Errorf("stage jobs file: %w", err)
		}
	}
	if err := store.rejectSymlinks(); err != nil {
		return err
	}
	name := ".jobs-" + rand.Text() + ".tmp"
	temporary, err := directory.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create jobs temp: %w", err)
	}
	remove := true
	defer func() {
		_ = temporary.Close() // Best effort after every explicit close path.
		if remove {
			_ = directory.Remove(name) // Best-effort cleanup cannot change the target.
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return fmt.Errorf("write jobs temp: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync jobs temp: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close jobs temp: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := store.rename(directory, name, "jobs.yaml"); err != nil {
		return fmt.Errorf("replace jobs file: %w", err)
	}
	remove = false
	if err := store.rejectSymlinks(); err != nil {
		return &UnconfirmedError{Bytes: append([]byte(nil), data...), Cause: err}
	}
	if err := store.syncDir(directory); err != nil {
		return &UnconfirmedError{Bytes: append([]byte(nil), data...), Cause: err}
	}
	return nil
}

func (store *Store) withLock(ctx context.Context, run func() error) error {
	root, err := os.OpenRoot(store.root)
	if err != nil {
		return fmt.Errorf("open jobs repository: %w", err)
	}
	defer func() { _ = root.Close() }() // The operation owns this handle and the lock descriptor below.
	if err := root.MkdirAll(".pyxis/state/maintenance", 0o700); err != nil {
		return fmt.Errorf("create jobs lock directory: %w", err)
	}
	directory, err := root.OpenRoot(".pyxis/state/maintenance")
	if err != nil {
		return fmt.Errorf("open jobs lock directory: %w", err)
	}
	defer func() { _ = directory.Close() }() // The lock descriptor closes before its pinned directory.
	if err := store.rejectSymlinks(); err != nil {
		return err
	}
	if store.beforeLock != nil {
		if err := store.beforeLock(); err != nil {
			return fmt.Errorf("stage jobs lock: %w", err)
		}
	}
	// macOS openat(O_CREATE) can return ENOENT when first creators race
	// (https://go.dev/issue/81246). Create exclusively or open the winner's file.
	file, err := directory.OpenFile("jobs.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if errors.Is(err, os.ErrExist) {
		file, err = directory.OpenFile("jobs.lock", os.O_RDWR|syscall.O_NOFOLLOW, 0)
	}
	if err != nil {
		if refusal := store.rejectSymlinks(); refusal != nil {
			return refusal
		}
		return fmt.Errorf("open jobs lock: %w", err)
	}
	defer func() { _ = file.Close() }() // Closing the lock descriptor cannot change the completed operation.
	if err := store.rejectSymlinks(); err != nil {
		return err
	}
	for {
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			return fmt.Errorf("lock jobs store: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }() // Closing the descriptor also releases a failed unlock.
	return run()
}
func (store *Store) rejectSymlinks() error {
	for _, path := range []string{filepath.Join(store.root, ".pyxis"), filepath.Dir(store.path), store.path, filepath.Join(store.root, ".pyxis/state"), filepath.Dir(store.lockPath), store.lockPath} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return invalid("storage target is unreadable or symlinked: %s", path)
		}
	}
	return nil
}
func find(list []Job, id string) int {
	for index := range list {
		if list[index].ID == id {
			return index
		}
	}
	return -1
}
func clone(job Job) Job {
	if job.Delivery != nil {
		delivery := *job.Delivery
		job.Delivery = &delivery
	}
	return job
}
func syncDirectory(root *os.Root) error {
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }() // Sync is the durability result; close cannot roll it back.
	return directory.Sync()
}
