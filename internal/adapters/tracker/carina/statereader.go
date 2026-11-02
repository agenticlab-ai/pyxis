package carinatracker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// AdviceCadence is the poll interval this adapter declares safe
// (docs/tech-design/state-reader-v2.md §3.7): a scan is a handful of local
// stat calls over dozens of files (~0 cost), so a consumer may call States
// once a second with no meaningful load on the store.
const AdviceCadence = time.Second

// StateReader answers the reader port (port.ProposalStateReader) directly
// over a Carina work-item store, one work item standing in for the
// "proposal" the port asks about (staterecord.go resolves the exact
// mapping). It reads the store's files directly — no watch dependency,
// stdlib only — never through the `carina` binary Tracker shells to: a
// per-ref subprocess spawn on a 1-second cadence would cost far more than
// the direct reads this design measures at ~0.
//
// Each ref's last-seen (mtime, size) is cached so an unchanged file is never
// re-parsed — the "zero scanning while unconsulted, no parsing on no
// change" requirement. The store's reads take no lock (rename() already
// makes them safe without one, local-work-tracker-v2 §3.4), so this cache is
// a pure in-process optimization: it is never consulted for correctness, so
// it is safe to lose (a restart just re-parses once).
type StateReader struct {
	root string

	// getItem is the item-fetch seam: NewStateReader wires it to a real
	// localfs.Store's Get; tests override it to count calls and prove a
	// no-change scan never invokes it.
	getItem func(ctx context.Context, id int) (model.WorkItem, []schema.Defect)

	mu   sync.Mutex
	seen map[int]fileStamp          // last-observed (mtime, size) per item id
	last map[int]port.ProposalState // last-parsed, unfiltered state per item id
}

var _ port.ProposalStateReader = (*StateReader)(nil)

// fileStamp is the cheap, stdlib-only change fingerprint a stat yields:
// modification time plus size. Either changing means the file's content may
// have changed; both matching is treated as unchanged.
type fileStamp struct {
	modTime time.Time
	size    int64
}

// NewStateReader binds a StateReader to the work-item store rooted at
// root — the same root a Tracker over the same store is constructed with.
func NewStateReader(root string) *StateReader {
	store := localfs.New(root, time.Now)
	return &StateReader{
		root:    root,
		getItem: store.Get,
		seen:    make(map[int]fileStamp),
		last:    make(map[int]port.ProposalState),
	}
}

// States implements port.ProposalStateReader: every requested ref is
// answered exactly once, in order (DINV-1); a ref whose item cannot be
// stat'd, read, or parsed resolves to a typed UnknownParseDefect, never an
// omission (DINV-2) — Carina's only failure surface is the local
// filesystem, and local-work-tracker-v2 itself unifies "missing" and
// "malformed" under one Defect, so this reader does too. The top-level
// error is reserved for a failure that prevents any per-ref answer at all
// (context cancellation) — never a per-item condition.
func (r *StateReader) States(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]port.ProposalState, len(refs))
	for i, ref := range refs {
		out[i] = r.stateFor(ctx, ref.Number, since)
	}
	return out, nil
}

// stateFor answers one ref by id, consulting the fingerprint cache before
// any parse. Callers must hold mu.
func (r *StateReader) stateFor(ctx context.Context, id int, since port.Cursor) port.ProposalState {
	info, err := os.Stat(itemFilePath(r.root, id))
	if err != nil {
		// Missing, or some other stat failure: the medium cannot answer for
		// this ref this batch. Never cached — a transient condition must not
		// wedge a ref unknown forever once the file reappears.
		delete(r.seen, id)
		delete(r.last, id)
		return unknownState(port.UnknownParseDefect)
	}

	stamp := fileStamp{modTime: info.ModTime(), size: info.Size()}
	if cached, ok := r.last[id]; ok && r.seen[id] == stamp {
		return withComments(cached, filterSince(cached.Comments, since))
	}

	w, defects := r.getItem(ctx, id)
	if len(defects) > 0 {
		delete(r.seen, id)
		delete(r.last, id)
		return unknownState(port.UnknownParseDefect)
	}
	st := deriveState(w)
	r.seen[id] = stamp
	r.last[id] = st
	return withComments(st, filterSince(st.Comments, since))
}

// itemFilePath is local-work-tracker-v2's own documented on-disk convention
// (§3.1: "<work_items_root>/items/<id>.md"), stated here only for the cheap
// stat fast-path — every actual read goes through the store's own Get, so a
// path guess that ever drifted from the real convention would only cost the
// caching optimization, never correctness.
func itemFilePath(root string, id int) string {
	return filepath.Join(root, "items", strconv.Itoa(id)+".md")
}

// unknownState builds the fail-closed typed-unknown result (DINV-1,
// DINV-2): LandingUnknown, no checks, no comments, no revision — nothing
// beside the typed reason.
func unknownState(reason port.UnknownReason) port.ProposalState {
	r := reason
	return port.ProposalState{Landing: port.LandingUnknown, Unknown: &r}
}

// withComments returns a copy of st with Comments replaced — st is a value
// type, so this never mutates a cached entry.
func withComments(st port.ProposalState, comments []port.Comment) port.ProposalState {
	st.Comments = comments
	return st
}

// filterSince returns only the comments strictly newer than since's
// per-surface position, mirroring the port's own FakeStateReader semantics
// (statereaderfake.go's newerComments, unexported there so this is its own
// copy): a surface absent from since.Positions has no lower bound.
func filterSince(comments []port.Comment, since port.Cursor) []port.Comment {
	if len(comments) == 0 {
		return nil
	}
	out := make([]port.Comment, 0, len(comments))
	for _, c := range comments {
		if pos, ok := since.Positions[c.Surface]; ok && !c.CreatedAt.After(pos) {
			continue
		}
		out = append(out, c)
	}
	return out
}
