package localfs

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The lifecycle mutations — edit and the open/closed transitions
// (docs/tech-design/local-work-tracker.md §3.10, DINV-11; T2). Each is a
// serialized read-modify-write of the one authoritative file (mutate.go): read
// it directly under the per-item lock (DINV-4), apply the change, and write it
// back atomically (temp + fsync + rename, DINV-2). All validation runs before
// any byte is written, so a rejected mutation leaves the item wholly untouched
// (fail closed, AP-17). A change that is already applied is a recorded no-op: no
// write, no stamp bump, no log entry (AP-22) — which keeps "the updated stamp
// advances only on a real change" and "one transition entry per applied change"
// both literally true.

// Edit changes exactly the fields the patch names (title, body) and advances
// the updated stamp when a value actually differs, leaving every other field
// byte-identical. An empty or effectless patch is a no-op.
func (s *Store) Edit(ctx context.Context, id int, patch model.EditPatch) (model.WorkItem, []schema.Defect) {
	return s.mutate("edit", id, func(w *model.WorkItem) (bool, []schema.Defect) {
		changed := false
		if patch.Title != nil && *patch.Title != w.Title {
			w.Title = *patch.Title
			changed = true
		}
		if patch.Body != nil && *patch.Body != w.Body {
			w.Body = *patch.Body
			changed = true
		}
		return changed, nil
	})
}

// Transition moves the item to state `to` idempotently. It validates the target
// state and (when closing) the close reason against their closed sets first
// (DINV-11) — an out-of-set value is refused naming the value and the allowed
// set, even when the item already holds the target state, so validation is
// never skipped by the no-op short-circuit. A transition into the state the
// item already holds changes nothing and appends nothing. An applied transition
// advances the updated stamp and appends one audit entry to the log.
func (s *Store) Transition(ctx context.Context, id int, to model.State, reason model.CloseReason, author string) (model.WorkItem, []schema.Defect) {
	verb := transitionVerb(to)
	if !model.ValidState(to) {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("state %q is out of set", to), "state is one of "+join(model.States)+" (§3.14)")}
	}
	if to == model.StateClosed && !model.ValidCloseReason(reason) {
		return model.WorkItem{}, []schema.Defect{def(verb, "", "validate", fmt.Sprintf("close_reason %q is out of set", reason), "close_reason is empty or one of "+join(model.CloseReasons)+" (§3.14)")}
	}
	return s.mutate(verb, id, func(w *model.WorkItem) (bool, []schema.Defect) {
		if w.State == to {
			return false, nil // already in the target state — a recorded no-op (AP-22)
		}
		from := w.State
		w.State = to
		if to == model.StateClosed {
			w.CloseReason = reason
		} else {
			w.CloseReason = model.CloseReasonNone
		}
		w.Log = append(w.Log, model.LogEntry{
			Kind:    model.LogTransition,
			Author:  author,
			At:      s.now().UTC(),
			Payload: transitionPayload(from, to, w.CloseReason),
		})
		return true, nil
	})
}

// transitionVerb names the CLI verb a target state corresponds to, so a
// transition's defect reads as "close"/"reopen" rather than an opaque token.
func transitionVerb(to model.State) string {
	switch to {
	case model.StateClosed:
		return "close"
	case model.StateOpen:
		return "reopen"
	default:
		return "transition"
	}
}

// transitionPayload renders the human-legible audit line an applied transition
// records: the state change, plus the reason when closing.
func transitionPayload(from, to model.State, reason model.CloseReason) string {
	if to == model.StateClosed && reason != model.CloseReasonNone {
		return fmt.Sprintf("%s → %s (%s)", from, to, reason)
	}
	return fmt.Sprintf("%s → %s", from, to)
}
