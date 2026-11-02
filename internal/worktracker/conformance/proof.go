package conformance

import (
	"context"
	"strconv"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The T10 proof row — the defect-quadruple sweep (docs/tech-design/
// local-work-tracker.md §3.13; prd:local-work-tracker/UB-1). It walks every
// file-implicating failure mode of the medium in one place and asserts each one
// fails closed with a Defect naming the full quadruple: the file at fault, the
// operation, the reason, and the remedy. The earlier tasks each asserted the
// quadruple for their own refusal (undefined signal, cycle, …); this row is the
// single sweep that proves no §3.13 failure mode names less than the whole
// four, over the port alone so it holds for any medium. Pure input-validation
// refusals (an out-of-set enum before any file is touched) carry no file to
// blame and are covered by the per-field rows (A-13); this sweep is the
// file-level contract of UB-1.

// failureCase is one file-implicating failure mode: a name, the operation that
// triggers it against a prepared store, and the resulting defects.
type failureCase struct {
	name    string
	trigger func(t *testing.T, store worktracker.WorkTracker, fault Fault) []schema.Defect
}

// defectQuadrupleSweep drives every §3.13 file-level failure mode and asserts
// each returns at least one defect, every defect names the full quadruple
// (file, op, reason, remedy), and a well-formed sibling created alongside the
// fault stays readable — a failure quarantines only its own subject.
func defectQuadrupleSweep(t *testing.T, h Harness) {
	cases := []failureCase{
		{"MissingItem", func(t *testing.T, store worktracker.WorkTracker, _ Fault) []schema.Defect {
			_, defects := store.Get(context.Background(), 999999)
			return defects
		}},
		{"MalformedItem", func(t *testing.T, store worktracker.WorkTracker, fault Fault) []schema.Defect {
			w := mustCreate(t, store, "will be corrupted")
			fault.Corrupt(t, w.ID, []byte("this is not a work item\n"))
			_, defects := store.Get(context.Background(), w.ID)
			return defects
		}},
		{"UndefinedSignal", func(t *testing.T, store worktracker.WorkTracker, _ Fault) []schema.Defect {
			w := mustCreate(t, store, "carries no undefined signal")
			_, defects := store.AddSignal(context.Background(), w.ID, "no-such-signal")
			return defects
		}},
		{"DependencyCycle", func(t *testing.T, store worktracker.WorkTracker, _ Fault) []schema.Defect {
			ctx := context.Background()
			a := mustCreate(t, store, "a")
			b := mustCreate(t, store, "b")
			if _, defects := store.AddBlockedBy(ctx, b.ID, strconv.Itoa(a.ID)); len(defects) > 0 {
				t.Fatalf("seed edge b→a: %v", defects)
			}
			// a blocked_by b would close the a→b→a cycle.
			_, defects := store.AddBlockedBy(ctx, a.ID, strconv.Itoa(b.ID))
			return defects
		}},
		{"DanglingDependency", func(t *testing.T, store worktracker.WorkTracker, _ Fault) []schema.Defect {
			w := mustCreate(t, store, "blocked on a ghost")
			_, defects := store.AddBlockedBy(context.Background(), w.ID, "999999")
			return defects
		}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store, fault := h.NewStore(t)
			// A well-formed sibling: no failure mode may render it unreadable.
			sibling := mustCreate(t, store, "innocent bystander")

			defects := tc.trigger(t, store, fault)
			if len(defects) == 0 {
				t.Fatalf("%s did not fail closed: no defect returned", tc.name)
			}
			for _, d := range defects {
				if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
					t.Fatalf("%s defect omits part of the quadruple (file/op/reason/remedy): %+v", tc.name, d)
				}
			}
			if _, sd := store.Get(context.Background(), sibling.ID); len(sd) > 0 {
				t.Fatalf("%s left the well-formed sibling #%d unreadable: %v", tc.name, sibling.ID, sd)
			}
		})
	}
}

func mustCreate(t *testing.T, store worktracker.WorkTracker, title string) model.WorkItem {
	t.Helper()
	w, defects := store.Create(context.Background(), model.CreateSpec{Title: title})
	mustClean(t, "create", defects)
	return w
}
