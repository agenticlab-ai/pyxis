package state

import (
	"context"
	"fmt"
	"strings"
)

// Store is the lane-state record's repo-resident home behind a thin port — the
// dedicated lane-state ref outside the protected branch namespace. The workflow
// adapter fills it (a git-ref read/append as the machine identity); the tests
// substitute an in-memory block. The port carries no credential accessor: the
// machine identity behind AppendState lives in the executing medium's secret
// store, never surfaced here (DINV-5).
type Store interface {
	// ReadState returns the current lane-state block and the platform-attributed
	// writer of the ref tip. A never-written ref is (nil, Writer{}, nil): the lane
	// has never halted. The writer is read from the platform's attribution, not
	// the record's bytes, so Resolve can refuse a non-machine write (DINV-7).
	ReadState(ctx context.Context) ([]byte, Writer, error)
	// AppendState appends the entry to the lane-state ref as the machine identity
	// and returns only after the write is durable — the SYNCHRONOUS "before any
	// further act" ordering DINV-7 requires. It appends to the existing block
	// (append-only); it never rewrites an earlier entry.
	AppendState(ctx context.Context, e Entry) error
}

// Alarm raises the halt/kill alarm on the affected drive's delivery ledger — the
// argus alarm surface (§3.4). It is a distinct port from Store: the halt's
// load-bearing datum is the repo-resident record every act gate reads, and the
// alarm is the notification layered over it, never a substitute for it.
type Alarm interface {
	// Raise records the alarm on the delivery ledger the event names.
	Raise(ctx context.Context, ev AlarmEvent) error
}

// The two alarm kinds the lane raises on the delivery ledger.
const (
	AlarmHalt = "halt"
	AlarmKill = "kill"
)

// AlarmEvent is one halt/kill alarm on a drive's delivery ledger. It names the
// affected ledger and pins the proposal, head, and reason so the alarm is
// cold-readable years later (NFR-4).
type AlarmEvent struct {
	Kind         string // AlarmHalt | AlarmKill
	Ledger       string // the affected drive's delivery ledger (docs/problems/<slug>/delivery.md)
	Proposal     string
	HeadRevision string
	Reason       string
}

// WriteHalt records a goal-verify failure: it writes the halt to the
// repo-resident lane-state record SYNCHRONOUSLY — durable before anything else,
// so every subsequent act gate reads it (DINV-7) — and then alarms on the
// affected drive's delivery ledger. The order is load-bearing: the halt record
// is persisted FIRST; only then is the alarm raised. A failed record write is
// fail-closed — WriteHalt returns the error and the caller must treat the lane
// as un-safely-halted (stop acting); a failed alarm is returned too, but the
// halt is already durable, so the circuit breaker holds even when the
// notification does not land.
//
// ledger names the affected drive's delivery ledger; proposal and head pin the
// act whose badge lied; reason names why the goal-object verification failed.
func WriteHalt(ctx context.Context, store Store, alarm Alarm, ledger, proposal, head, reason string) error {
	if store == nil {
		return fmt.Errorf("lane-state: WriteHalt needs a Store to persist the halt (DINV-7)")
	}
	if strings.TrimSpace(proposal) == "" || strings.TrimSpace(head) == "" {
		return fmt.Errorf("lane-state: WriteHalt needs the proposal and head the voided act bound to")
	}

	// Synchronous halt write FIRST — durable before any further act (DINV-7). A
	// failure here fails closed: the lane is not safely halted, so the caller
	// must stop acting rather than proceed.
	if err := store.AppendState(ctx, Halt(proposal, head, reason)); err != nil {
		return fmt.Errorf("writing the lane-state halt for %q (fail-closed, stop acting): %w", proposal, err)
	}

	// Alarm on the delivery ledger AFTER the halt is durable. The halt record is
	// the circuit breaker every act gate reads; the alarm is the notification.
	if alarm != nil {
		ev := AlarmEvent{Kind: AlarmHalt, Ledger: ledger, Proposal: proposal, HeadRevision: head, Reason: reason}
		if err := alarm.Raise(ctx, ev); err != nil {
			return fmt.Errorf("halt recorded for %q, but raising the delivery-ledger alarm failed: %w", proposal, err)
		}
	}
	return nil
}
