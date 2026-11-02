package localfs

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
)

// Reindex rebuilds the reverse-parent index wholly from the item files — the sole
// drift recovery (worktracker-hierarchy-index §3.7, DINV-6). This is the explicit
// trigger (the `carina reindex` verb); the same rebuild also runs lazily on an
// absent index (childMarkers) and before a decision read when the dirty check
// trips. It never reads the old index — the truth is the only input.
func (s *Store) Reindex(ctx context.Context) (worktracker.ReindexReport, []schema.Defect) {
	return s.reindexWith("explicit")
}

// reindexWith runs the full rebuild and returns a report tagged with its trigger.
// The duration is measured on the injected clock, so it is 0 under a fixed test
// clock and real wall-clock in production — hermetic, never asserted upon.
func (s *Store) reindexWith(trigger string) (worktracker.ReindexReport, []schema.Defect) {
	start := s.now()
	items, markers, err := s.buildIndex()
	if err != nil {
		return worktracker.ReindexReport{}, []schema.Defect{def("reindex", s.revDir(), "rebuild", err.Error(), "ensure the store root is writable so the index can be rebuilt")}
	}
	return worktracker.ReindexReport{
		Schema:  worktracker.ReindexSchemaVersion,
		Trigger: trigger,
		Items:   items,
		Markers: markers,
		Dur:     s.now().Sub(start),
	}, nil
}

// stampPath is the index's freshness stamp: an empty file whose mtime marks the
// last moment the index was maintained in step with the truth (design Q1).
func (s *Store) stampPath() string { return filepath.Join(s.revDir(), ".stamp") }

// touchStamp refreshes the freshness stamp to now. It runs after every index
// maintenance — a full build (buildIndex) and each incremental edge write
// (reindexEdge) — so that any item file newer than the stamp is, by construction,
// a write the index did NOT see: an out-of-band hand-edit (NR-2). Best-effort: a
// missing stamp just reads as dirty and forces one heal.
func (s *Store) touchStamp() {
	_ = os.MkdirAll(s.revDir(), 0o755)
	_ = os.WriteFile(s.stampPath(), nil, 0o644)
}

// dirty reports whether the item tree may hold a write the index has not seen —
// an item file whose mtime is at or after the freshness stamp (design Q1, the
// hand-edit case R1). An absent stamp is dirty (never built or wiped). It only
// stats item files — never reads them — and is consulted ONLY on the decision
// path, where completeness outranks the O(k) display bound; a display read never
// pays it, so the display query path stays O(k) (§3.7, A-2).
//
// The comparison is at-or-after (>=), not strictly after, on purpose: inode mtime
// granularity is coarse on some filesystems — Linux stamps it from a jiffy-based
// coarse clock (~1–4ms) — so a hand-edit in the same tick as the last stamp shares
// its mtime. A strict > would miss that hand-edit and a decision read would
// under-report — a correctness bug that failed exactly this way on Linux CI while
// passing on ns-resolution macOS. >= never misses a hand-edit; the cost is that a
// normal API write landing in the stamp's tick also trips one rebuild before the
// next decision read — an idempotent, off-hot-path O(N) heal (writes stay
// incremental; display reads are untouched). A truly cheap dirty signal that also
// excludes same-tick API writes is deferred (Q1).
func (s *Store) dirty() bool {
	si, err := os.Stat(s.stampPath())
	if err != nil {
		return true // no stamp → never built, or wiped: rebuild to be sure
	}
	stampT := si.ModTime()
	entries, err := s.readDir(s.itemsDir())
	if err != nil {
		return false // no items directory → nothing to be dirty about
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := idFromName(e.Name()); !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if !info.ModTime().Before(stampT) { // mtime >= stampT (coarse-mtime safe)
			return true
		}
	}
	return false
}
