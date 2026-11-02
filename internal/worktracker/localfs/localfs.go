// Package localfs is the file-based WorkTracker adapter — the first (and, in
// the first task, only) implementation of the worktracker port
// (docs/tech-design/local-work-tracker.md §3.7). One work item is one
// human-readable, git-diffable file under the configured bucket-1 store root;
// the file is the sole source of truth (DINV-1), writes are atomic (DINV-2),
// and identifiers are allocated by atomic exclusive-create (DINV-3). No
// database, daemon, index, or network is on any path (DINV-7).
//
// The store is constructed with an injected clock so tests are hermetic; the
// CLI passes time.Now. The adapter satisfies worktracker.WorkTracker — the
// engine imports the port, never this package (DINV-9).
package localfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// Store is the localfs adapter over one work-item store root.
type Store struct {
	root   string
	now    func() time.Time
	giOnce sync.Once // guards the one-per-process rev/ .gitignore write (DINV-3)
	// Read operations are per-store seams so access-path tests can count real
	// filesystem work without using host latency as a proxy for complexity.
	readFile func(string) ([]byte, error)
	readDir  func(string) ([]os.DirEntry, error)
}

// compile-time assertion that the adapter satisfies the port (DINV-9).
var _ worktracker.WorkTracker = (*Store)(nil)

// New returns a Store rooted at root, stamping times from now. A nil clock
// falls back to time.Now (the live path); tests inject a fixed clock.
func New(root string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{root: root, now: now, readFile: os.ReadFile, readDir: os.ReadDir}
}

func (s *Store) itemsDir() string { return filepath.Join(s.root, "items") }

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Create allocates the next identifier by atomic exclusive-create, then writes
// the rendered item over the reserved placeholder atomically (§3.3, §3.4). The
// returned item carries its assigned id, open state, and creation timestamps.
func (s *Store) Create(ctx context.Context, spec model.CreateSpec) (model.WorkItem, []schema.Defect) {
	const verb = "create"
	if err := os.MkdirAll(s.itemsDir(), 0o755); err != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, s.itemsDir(), "mkdir", err.Error(), "ensure the store root is writable")}
	}
	s.sweepTemps()

	w := model.WorkItem{
		Type:     spec.Type,
		State:    model.StateOpen,
		Title:    spec.Title,
		Priority: spec.Priority,
		Body:     spec.Body,
	}
	if w.Type == "" {
		w.Type = model.TypeTask
	}
	if defects := validateCreate(verb, w); len(defects) > 0 {
		return model.WorkItem{}, defects
	}
	now := s.now().UTC()
	w.Created, w.Updated = now, now

	max, err := s.maxID()
	if err != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, s.itemsDir(), "scan", err.Error(), "ensure the store directory is readable")}
	}
	id, err := s.reserveID(max + 1)
	if err != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, s.itemsDir(), "allocate", err.Error(), "ensure the store directory is writable")}
	}
	w.ID = id

	target := itemPath(s.itemsDir(), id)
	if err := s.atomicReplace(target, Render(w), nil); err != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, target, "write", err.Error(), "ensure the store has free space and is writable")}
	}
	return w, nil
}

// Get resolves the item at id from its authoritative file directly — no index
// or cache is consulted (DINV-4). A missing, empty (crashed-create gap), or
// malformed file yields a defect naming file, operation, reason, and remedy,
// quarantining exactly that item while every other item stays readable (§3.13).
func (s *Store) Get(ctx context.Context, id int) (model.WorkItem, []schema.Defect) {
	return s.read("show", id)
}

// read is the single authoritative-file read the whole adapter shares: Get,
// and every read-modify-write (edit, transition, …), resolve an item through
// it so a mutation's defect names *its* verb, not "show". It reads the file
// directly — no index or cache (DINV-4) — and fails closed on a missing, empty,
// or malformed file, quarantining exactly that item (§3.13).
func (s *Store) read(verb string, id int) (model.WorkItem, []schema.Defect) {
	if id <= 0 {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "resolve", fmt.Sprintf("id %d is not a positive integer", id), "reference an item by its positive id (e.g. 1)")}
	}
	path := itemPath(s.itemsDir(), id)
	raw, err := s.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.WorkItem{}, []schema.Defect{def(verb, path, "resolve", fmt.Sprintf("no work item #%d", id), "create the item or reference an existing id")}
		}
		return model.WorkItem{}, []schema.Defect{def(verb, path, "read", err.Error(), "ensure the item file is readable")}
	}
	if strings.TrimSpace(string(raw)) == "" {
		return model.WorkItem{}, []schema.Defect{def(verb, path, "resolve", fmt.Sprintf("no work item #%d (empty placeholder)", id), "the id is a permanent gap from an interrupted create; use a different id")}
	}
	w, defects := Parse(verb, path, raw)
	if len(defects) > 0 {
		return model.WorkItem{}, defects
	}
	if w.ID != id {
		return model.WorkItem{}, []schema.Defect{def(verb, path, "parse-header", fmt.Sprintf("header id %d does not match filename #%d", w.ID, id), "restore the header id to match the file name")}
	}
	return w, nil
}

// validateCreate rejects an out-of-set create input before any file is touched
// (fail closed, AP-17). Title is required; the enum fields must be in set.
func validateCreate(verb string, w model.WorkItem) []schema.Defect {
	var defects []schema.Defect
	add := func(op, reason, remedy string) { defects = append(defects, def(verb, "", op, reason, remedy)) }
	if strings.TrimSpace(w.Title) == "" {
		add("validate", "title is empty", "supply a non-empty title")
	}
	if !model.ValidType(w.Type) {
		add("validate", fmt.Sprintf("type %q is out of set", w.Type), "type is one of "+join(model.Types))
	}
	if !model.ValidPriority(w.Priority) {
		add("validate", fmt.Sprintf("priority %q is out of set", w.Priority), "priority is empty or one of "+join(model.Priorities))
	}
	return defects
}
