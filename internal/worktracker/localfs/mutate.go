package localfs

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// mutate is the single serialized read-modify-write every item mutation shares
// (docs/tech-design/local-work-tracker.md §3.4, DINV-2/DINV-4/DINV-5). It takes
// the per-item advisory lock, reads the authoritative file directly, applies fn,
// and — only when fn reports a real change — writes the result back atomically,
// advancing the updated stamp. Because the whole read-through-write runs under
// the lock, concurrent writers to one item serialize: no append is lost and no
// change clobbers another. A defect from the read or from fn aborts before any
// byte is written (fail closed, AP-17); a no-op writes nothing and leaves the
// file — and its updated stamp — exactly as it was (AP-22).
func (s *Store) mutate(verb string, id int, fn func(w *model.WorkItem) (changed bool, defects []schema.Defect)) (model.WorkItem, []schema.Defect) {
	return s.mutateThen(verb, id, fn, nil)
}

// mutateThen is mutate with an optional post-commit hook. When fn reports a real
// change and the atomic item write succeeds, after runs inside the same per-item
// lock, so any derived side effect it performs is ordered strictly AFTER the
// authoritative write is durable (field-first). The reverse-parent index uses
// this to maintain a marker only once the child's `parent` truth has committed
// (DINV-5): a crash between the two leaves the truth correct and the derived
// index lagging, never the reverse. after never runs on a no-op or a defect.
func (s *Store) mutateThen(verb string, id int, fn func(w *model.WorkItem) (changed bool, defects []schema.Defect), after func(w model.WorkItem)) (model.WorkItem, []schema.Defect) {
	var (
		item    model.WorkItem
		defects []schema.Defect
	)
	lockErr := s.withItemLock(id, func() error {
		w, ds := s.read(verb, id)
		if len(ds) > 0 {
			defects = ds
			return nil
		}
		changed, ds := fn(&w)
		if len(ds) > 0 {
			defects = ds
			return nil
		}
		if !changed {
			item = w // recorded no-op: the file is left exactly as it was
			return nil
		}
		w.Updated = s.now().UTC()
		target := itemPath(s.itemsDir(), w.ID)
		if err := s.atomicReplace(target, Render(w), nil); err != nil {
			return err
		}
		item = w
		if after != nil {
			after(w) // field-first: the item write above is already durable (DINV-5)
		}
		return nil
	})
	if lockErr != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), id), "write", lockErr.Error(), "ensure the store has free space and is writable")}
	}
	return item, defects
}

// rewriteItem is the write half of a mutation whose read and validation already
// happened before the call — the dependency edges, whose cycle check must read
// other items outside the single-item fn mutate offers. It persists the
// already-modified item w atomically under the per-item lock, advancing the
// updated stamp (docs/tech-design/local-work-tracker.md §3.4, DINV-2). A write
// failure is fail-closed: the prior file is left intact and a defect is returned
// (AP-17).
func (s *Store) rewriteItem(verb string, w model.WorkItem) (model.WorkItem, []schema.Defect) {
	lockErr := s.withItemLock(w.ID, func() error {
		w.Updated = s.now().UTC()
		return s.atomicReplace(itemPath(s.itemsDir(), w.ID), Render(w), nil)
	})
	if lockErr != nil {
		return model.WorkItem{}, []schema.Defect{def(verb, itemPath(s.itemsDir(), w.ID), "write", lockErr.Error(), "ensure the store has free space and is writable")}
	}
	return w, nil
}
