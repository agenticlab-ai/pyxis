package localfs

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
)

// The reverse-parent index (docs/tech-design/worktracker-hierarchy-index.md
// §3.6). rev/ holds one zero-byte marker per parent→child edge, sharded by
// parent id — rev/<shard>/<parent>/<child>. It is DERIVED, non-authoritative,
// gitignored (DINV-3), and rebuildable in full from the item `parent` fields
// (DINV-6). The child's `parent` field stays the sole truth (DINV-1); a marker's
// presence is membership only — it carries no bytes, so the index can only lag,
// never hold a wrong value, which is why decision-path re-verification (DINV-2)
// fully closes the correctness gap. One file per edge keeps concurrent
// parent-writes to different children on disjoint paths (DINV-4, AP-27).

func (s *Store) revDir() string { return filepath.Join(s.root, "rev") }

// shard buckets a parent so the top-level rev/ directory never grows one entry
// per parent — a filesystem-scaling guard only. The read path lists a single
// parent's directory, never rev/ itself, so sharding never costs a query
// (design Q2). Parents are numeric ids set by the API; a non-numeric hand-edit
// buckets deterministically at 0 and is still found.
func shard(parent string) string {
	n, _ := strconv.Atoi(parent)
	return strconv.FormatInt(int64(n&0xff), 16)
}

// parentDir is the directory holding a parent's child markers.
func (s *Store) parentDir(parent string) string {
	return filepath.Join(s.revDir(), shard(parent), parent)
}

// markerPath is the zero-byte marker for the edge parent→childID.
func (s *Store) markerPath(parent string, childID int) string {
	return filepath.Join(s.parentDir(parent), strconv.Itoa(childID))
}

// reindexEdge updates rev/ for one edge change. It is called ONLY after the
// authoritative item write has committed (field-first, DINV-5): a crash between
// the two leaves the truth correct and rev/ lagging by one edge, never the
// reverse. It removes the vacated marker and creates the new one; both ops are
// idempotent and empty (DINV-4). No error is fatal — a lost or torn marker
// degrades to staleness, healed by a Reindex (DINV-6); the truth is already
// durable, so there is nothing to fail closed on here.
func (s *Store) reindexEdge(childID int, oldParent, newParent string) {
	s.ensureRevGitignored()
	if oldParent != "" {
		_ = os.Remove(s.markerPath(oldParent, childID)) // absent = no-op
	}
	if newParent != "" {
		_ = os.MkdirAll(s.parentDir(newParent), 0o755)
		_ = os.WriteFile(s.markerPath(newParent, childID), nil, 0o644) // ~O(1), empty
	}
	s.touchStamp() // the index is now in step with this edge (dirty check baseline)
}

// buildIndex rebuilds rev/ wholly from the item `parent` fields — a pure
// derivation and the sole drift recovery (DINV-6). It discards any existing
// rev/ and re-creates exactly one marker per current edge, so a crash-desynced,
// hand-edited, or absent index converges to the truth. Returns the readable-item
// and edge-marker counts for the ReindexReport (T3). O(N) in the store and off
// the hot path (§3.7): writes stay incremental (reindexEdge), this is the
// occasional heal.
func (s *Store) buildIndex() (items, markers int, err error) {
	if err := os.RemoveAll(s.revDir()); err != nil {
		return 0, 0, err
	}
	// Materialize rev/ even when the store has no edges yet, so its presence marks
	// the index as built and a later read does not rebuild on every call (the
	// lazy-build guard in childMarkers keys on the directory's existence).
	s.ensureRevGitignored()
	if err := os.MkdirAll(s.revDir(), 0o755); err != nil {
		return 0, 0, err
	}
	entries, err := s.readDir(s.itemsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil // an absent store is an empty index, not an error
		}
		return 0, 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := idFromName(e.Name())
		if !ok {
			continue
		}
		w, defects := s.read("reindex", id)
		if len(defects) > 0 {
			continue // a malformed item is quarantined, not fatal to the rebuild
		}
		items++
		if w.Parent != "" {
			s.reindexEdge(w.ID, "", w.Parent)
			markers++
		}
	}
	s.touchStamp() // fresh baseline even for a zero-edge store (RemoveAll dropped it)
	return items, markers, nil
}

// revExists reports whether the derived index directory is present. Its absence
// is normal — a fresh clone, or a store after `pyxis remove` (DINV-3) — and is
// what triggers a lazy build on the first hierarchy read (T2/T3).
func (s *Store) revExists() bool {
	info, err := os.Stat(s.revDir())
	return err == nil && info.IsDir()
}

// ensureRevGitignored writes a .gitignore at the store root ignoring rev/, so the
// derived index is never committed to git (DINV-3) and never reaches the backup
// remote. It is idempotent and additive: an existing .gitignore keeps its rules,
// and the rev/ rule is appended only if absent. It runs at most once per Store
// (giOnce), so concurrent marker writes never race on the file; the read-check
// keeps it idempotent across separate processes too. A failure is non-fatal —
// the index still works locally; only the ignore guard is best-effort.
func (s *Store) ensureRevGitignored() { s.giOnce.Do(s.writeRevGitignore) }

func (s *Store) writeRevGitignore() {
	gi := filepath.Join(s.root, ".gitignore")
	existing, _ := os.ReadFile(gi)
	if gitignoreHasRev(existing) {
		return
	}
	f, err := os.OpenFile(gi, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }() // Best-effort ignore guard; failure never blocks the index.
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString("# derived reverse-parent index — never committed (worktracker-hierarchy-index DINV-3)\nrev/\n")
}

// gitignoreHasRev reports whether the .gitignore body already ignores rev/, so
// the append stays idempotent. It matches the exact `rev/` rule on its own line,
// tolerating surrounding whitespace.
func gitignoreHasRev(body []byte) bool {
	for line := range bytes.SplitSeq(body, []byte("\n")) {
		if bytes.Equal(bytes.TrimSpace(line), []byte("rev/")) {
			return true
		}
	}
	return false
}
