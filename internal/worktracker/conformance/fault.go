package conformance

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The T9 fault suites — the store's integrity claims made executable
// (docs/tech-design/local-work-tracker.md §3.4; DINV-2/3/4/5; prd
// FR-14/UB-4/UB-5). They run through the port and the medium-neutral Fault seam
// only, importing no adapter symbol (conformance A-1), so the same rows prove
// any medium; the localfs harness runs them in the race-enabled build. Four
// rows, one per integrity claim: read-your-writes under interleave (DINV-4),
// crash safety across the atomic-write cut points (DINV-2), parallel-create with
// no id collision or reuse (DINV-3), and concurrent-append with zero loss
// (DINV-5).

// readYourWritesUnderInterleave: on a decision path, a read after a write always
// reflects that write, in 100% of cases, even while a crowd of other writers
// hammers the store (T9 acceptance 1; DINV-4 — the local filesystem gives
// read-your-writes, so no index or cache sits on a decision path). Each worker
// owns one item so its expectation is unambiguous; all workers run concurrently
// against the one store, so every read-back races the interleave.
func readYourWritesUnderInterleave(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	const workers, cycles = 8, 12

	ids := make([]int, workers)
	for i := range ids {
		w, defects := store.Create(ctx, model.CreateSpec{Title: fmt.Sprintf("worker %d", i)})
		mustClean(t, "create", defects)
		ids[i] = w.ID
	}

	var wg sync.WaitGroup
	fail := make(chan string, workers*cycles)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for c := 0; c < cycles; c++ {
				// A decision-record write, then the latest-wins read must return it.
				payload := fmt.Sprintf("plan-%d", c)
				if _, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "w", Payload: payload}); len(defects) > 0 {
					fail <- fmt.Sprintf("item %d record %d: %v", id, c, defects)
					return
				}
				rec, ok, defects := store.LatestRecord(ctx, id, model.NamespaceTriageDisposition)
				if len(defects) > 0 || !ok || rec.Payload != payload {
					fail <- fmt.Sprintf("item %d: latest-record read %q (ok=%v), want %q — stale read", id, rec.Payload, ok, payload)
					return
				}
				// A state transition, then a direct read must reflect it.
				want, reason := model.StateClosed, model.CloseCompleted
				if c%2 == 1 {
					want, reason = model.StateOpen, model.CloseReasonNone
				}
				if _, defects := store.Transition(ctx, id, want, reason, "w"); len(defects) > 0 {
					fail <- fmt.Sprintf("item %d transition %d: %v", id, c, defects)
					return
				}
				got, defects := store.Get(ctx, id)
				if len(defects) > 0 || got.State != want {
					fail <- fmt.Sprintf("item %d: state read %q, want %q — stale read", id, got.State, want)
					return
				}
			}
		}(ids[i])
	}
	wg.Wait()
	close(fail)
	for msg := range fail {
		t.Fatalf("read-your-writes violated: %s", msg)
	}
}

// crashAtEachCutPointAtomic: injecting a crash at every cut point of the
// atomic-write sequence leaves the item wholly old or wholly new — never torn —
// and loses no prior log entry (T9 acceptance 2; DINV-2, prd UB-4). The item
// carries a body of comments the mid-write crash must preserve whole.
func crashAtEachCutPointAtomic(t *testing.T, h Harness) {
	const priorComments = 5
	for _, cut := range AllCutPoints {
		cut := cut
		t.Run(cut.String(), func(t *testing.T) {
			store, fault := h.NewStore(t)
			ctx := context.Background()
			w, defects := store.Create(ctx, model.CreateSpec{Title: "before"})
			mustClean(t, "create", defects)
			for i := 0; i < priorComments; i++ {
				_, defects := store.Comment(ctx, w.ID, "author", fmt.Sprintf("note %d", i))
				mustCondition(t, len(defects) == 0, "seed comment %d: %v", i, defects)
			}

			committed := fault.CrashAt(t, w.ID, "after", cut)

			got, defects := store.Get(ctx, w.ID)
			mustClean(t, "get-after-crash", defects)
			// Wholly old or wholly new, per whether the crash fell after the commit.
			switch {
			case committed && got.Title != "after":
				t.Fatalf("crash at %v committed but title=%q, want %q (torn write)", cut, got.Title, "after")
			case !committed && got.Title != "before":
				t.Fatalf("crash at %v aborted but title=%q, want %q (torn write)", cut, got.Title, "before")
			}
			// Zero losses: every prior log entry survived, whole and parseable.
			mustCondition(t, countComments(got) == priorComments, "crash at %v lost log entries: %d comments survived, want %d", cut, countComments(got), priorComments)
		})
	}
}

// parallelCreateNoCollisionNoReuse: a multi-process create stress against one
// store completes with every identifier distinct — zero collision and zero reuse
// (T9 acceptance 3; DINV-3, prd UB-5) — and every allocated item resolves whole.
func parallelCreateNoCollisionNoReuse(t *testing.T, h Harness) {
	store, fault := h.NewStore(t)
	const procs, perProc = 8, 20

	ids := fault.ParallelCreate(t, procs, perProc)
	mustCondition(t, len(ids) == procs*perProc, "parallel create yielded %d ids, want %d (a create was lost)", len(ids), procs*perProc)
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		mustCondition(t, !seen[id], "id %d allocated twice — collision or reuse", id)
		seen[id] = true
	}
	// Every allocated id resolves to a real, whole item — no torn placeholder.
	ctx := context.Background()
	for _, id := range ids {
		_, defects := store.Get(ctx, id)
		mustCondition(t, len(defects) == 0, "id %d not resolvable after parallel create: %v", id, defects)
	}
}

// concurrentAppendStressLosesNothing: a burst of appends from many concurrent
// writers to one item lands every entry, in per-writer order, with zero loss
// (T9 acceptance 4; DINV-5 — the per-item lock serializes the read-modify-write).
// This is the contention stress beyond T3's two-writer smoke row.
func concurrentAppendStressLosesNothing(t *testing.T, h Harness) {
	store, _ := h.NewStore(t)
	ctx := context.Background()
	created, defects := store.Create(ctx, model.CreateSpec{Title: "contended log"})
	mustClean(t, "create", defects)
	id := created.ID

	const writers, perWriter = 4, 50
	var wg sync.WaitGroup
	fail := make(chan string, writers*perWriter)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(author string) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if _, defects := store.Comment(ctx, id, author, fmt.Sprintf("%s #%d", author, i)); len(defects) > 0 {
					fail <- fmt.Sprintf("%s comment %d: %v", author, i, defects)
					return
				}
			}
		}(fmt.Sprintf("w%d", w))
	}
	wg.Wait()
	close(fail)
	for msg := range fail {
		t.Fatalf("concurrent append failed: %s", msg)
	}

	got, defects := store.Get(ctx, id)
	mustClean(t, "get", defects)

	perAuthor := map[string]int{}
	nextExpected := map[string]int{}
	total := 0
	for _, e := range got.Log {
		if e.Kind != model.LogComment {
			continue
		}
		total++
		var author string
		var n int
		if _, err := fmt.Sscanf(e.Payload, "%s #%d", &author, &n); err != nil {
			continue
		}
		perAuthor[author]++
		mustCondition(t, n == nextExpected[author], "writer %s stream out of order: saw #%d, want #%d (a reorder or loss)", author, n, nextExpected[author])
		nextExpected[author] = n + 1
	}
	mustCondition(t, total == writers*perWriter, "log carries %d comments, want %d — an append was lost", total, writers*perWriter)
	for w := 0; w < writers; w++ {
		author := fmt.Sprintf("w%d", w)
		mustCondition(t, perAuthor[author] == perWriter, "writer %s landed %d comments, want %d", author, perAuthor[author], perWriter)
	}
}
