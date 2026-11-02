package localfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The append-log mutations — comments and latest-wins decision-records
// (docs/tech-design/local-work-tracker.md §3.1, §3.14, DINV-5; T3). Each is a
// serialized read-modify-write (mutate.go): read the authoritative file under
// the per-item lock, append the entry, write atomically. Serialization is what
// makes an interleaved burst of appends lose nothing (PRD UB-4).
//
// A comment always lands (dialogue accumulates forever). A decision-record
// resolves latest-wins per namespace and an identical repeat is a recorded no-op
// — the append is idempotent (AP-22). Both fail closed: an empty comment, an
// out-of-set namespace, or a missing item is refused before any write (AP-17).

// Comment appends author's body to the item as a comment entry (FR-5). The body
// is required — an empty comment is refused naming the offense. Every accepted
// comment appends one entry in time order; a comment is never a no-op.
func (s *Store) Comment(ctx context.Context, id int, author, body string) (model.WorkItem, []schema.Defect) {
	const verb = "comment"
	if strings.TrimSpace(body) == "" {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "validate", "comment body is empty", "supply a non-empty comment body (carina comment <id> --body <text>)")}
	}
	return s.mutate(verb, id, func(w *model.WorkItem) (bool, []schema.Defect) {
		w.Log = append(w.Log, model.LogEntry{
			Kind:    model.LogComment,
			Author:  author,
			At:      s.now().UTC(),
			Payload: body,
		})
		return true, nil
	})
}

// Record appends rec as a namespaced decision-record (FR-9). The namespace must
// be in the closed set (DINV-11) — an out-of-set value is refused naming it. A
// record whose payload equals the current latest record in the same namespace is
// a recorded no-op: latest-wins already resolves to that payload, so nothing is
// written (AP-22). Any other payload appends a fresh record that becomes the new
// authoritative one for the namespace.
func (s *Store) Record(ctx context.Context, id int, rec model.DecisionRecord) (model.WorkItem, []schema.Defect) {
	const verb = "record"
	if !model.ValidNamespace(rec.Namespace) {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("namespace %q is out of set", rec.Namespace), "namespace is one of "+join(model.Namespaces)+" (§3.14)")}
	}
	return s.mutate(verb, id, func(w *model.WorkItem) (bool, []schema.Defect) {
		if latest, ok := w.LatestRecord(rec.Namespace); ok && latest.Payload == rec.Payload {
			return false, nil // identical repeat: latest-wins already holds this — a recorded no-op
		}
		w.Log = append(w.Log, model.LogEntry{
			Kind:      model.LogDecisionRecord,
			Author:    rec.Author,
			At:        s.now().UTC(),
			Namespace: string(rec.Namespace),
			Payload:   rec.Payload,
		})
		return true, nil
	})
}

// LatestRecord returns the authoritative decision-record for ns — the latest one
// appended (DINV-5) — reading the item's file directly (DINV-4). An out-of-set
// namespace is refused naming it; a missing or malformed item surfaces its own
// defect. The resolution itself is model.WorkItem.LatestRecord, the single shared
// implementation (DINV-6).
func (s *Store) LatestRecord(ctx context.Context, id int, ns model.Namespace) (model.DecisionRecord, bool, []schema.Defect) {
	const verb = "record"
	if !model.ValidNamespace(ns) {
		return model.DecisionRecord{}, false, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("namespace %q is out of set", ns), "namespace is one of "+join(model.Namespaces)+" (§3.14)")}
	}
	w, defects := s.read(verb, id)
	if len(defects) > 0 {
		return model.DecisionRecord{}, false, defects
	}
	rec, ok := w.LatestRecord(ns)
	return rec, ok, nil
}
