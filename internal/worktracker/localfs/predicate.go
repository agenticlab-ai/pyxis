package localfs

import (
	"context"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The ready-set and triage-set predicates over the localfs store (docs/
// tech-design/local-work-tracker.md §3.6, FR-12, FR-13). Each scans the item
// files directly into a corpus and delegates to the single medium-neutral
// predicate in the model (model.ReadySet / model.TriageSet), so the CLI and the
// engine — both reaching this one adapter through the port — return byte-
// identical sets (DINV-6). The scan reads the authoritative files with no index
// or cache on the path, so the set reflects every prior write (DINV-4,
// read-your-writes). A malformed item is quarantined (skipped) exactly as the
// hierarchy and dependency scans do (§3.13); only a directory-level IO error is a
// defect.

// ReadySet returns the store's ready items in creation order (FR-12).
func (s *Store) ReadySet(ctx context.Context) ([]model.WorkItem, []schema.Defect) {
	items, defects := s.scanAll("ready")
	if len(defects) > 0 {
		return nil, defects
	}
	return model.ReadySet(items), nil
}

// TriageSet returns the store's triage (undecided) items in creation order (FR-13).
func (s *Store) TriageSet(ctx context.Context) ([]model.WorkItem, []schema.Defect) {
	items, defects := s.scanAll("triage")
	if len(defects) > 0 {
		return nil, defects
	}
	return model.TriageSet(items), nil
}

// scanAll reads every well-formed item file directly into a corpus (DINV-4). A
// file that fails to parse is quarantined (skipped) so one bad hand-edit cannot
// break a dispatch decision over its well-formed siblings (§3.13); only a
// directory-level IO error is a defect. An absent items directory is an empty
// corpus, not an error.
func (s *Store) scanAll(verb string) ([]model.WorkItem, []schema.Defect) {
	entries, err := s.readDir(s.itemsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []schema.Defect{def(verb, s.itemsDir(), "scan", err.Error(), "ensure the store directory is readable")}
	}
	var items []model.WorkItem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := idFromName(e.Name())
		if !ok {
			continue
		}
		w, defects := s.read(verb, id)
		if len(defects) > 0 {
			continue // a malformed sibling is quarantined, not a set member
		}
		items = append(items, w)
	}
	return items, nil
}
