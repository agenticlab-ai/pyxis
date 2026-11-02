package wake

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

// PushResult is one push-or-manual delivery's cold-readable outcome. Delivered
// is true when the front door was touched; NoOp names why a well-formed signal
// was not delivered (a closed ledger, never reopened); Rejected is true when the
// raw wake surface was not a well-formed signal at all — the injection boundary
// in force, no touch attempted (DINV-1).
type PushResult struct {
	Signal    Signal
	Source    Source // the delivery path (SourceSignal | SourceManual), set when Delivered
	Delivered bool
	NoOp      NoOpReason
	Rejected  bool
}

// Push delivers a raw wake surface — whatever a push adapter (or a poisoned
// copy, or a forged sender) wrote — through the front door. It reduces the raw
// bytes to a signal (slug + reason, everything else discarded) and, unless the
// named ledger is closed, touches the front door once. A raw surface that is not
// a well-formed `wake:<slug>:<reason>` token is rejected with no touch; a signal
// naming a closed ledger is a no-op that leaves the frozen file byte-untouched
// (DINV-3). This is the optimization path — never the guarantee; the backstop
// delivers the same completion within one interval regardless (§3.2). problems
// is the configured problems root the delivery ledgers live under (config-
// driven-portability-v1 §3.2, FR-1; default docs/problems).
func Push(ctx context.Context, door FrontDoor, ledgers Ledgers, problems, raw string) (PushResult, error) {
	sig, ok := ParseWire(raw)
	if !ok {
		return PushResult{Rejected: true}, nil
	}
	return dispatchSignal(ctx, door, ledgers, problems, sig, SourceSignal)
}

// Manual is the stated sequential fallback (DINV-7): a human front-door touch on
// a bare host — files, a shell, and the CLI only. It validates the slug and
// reason and delivers through the same path as a push, so the manual leg honors
// the same closed-ledger guard. problems is the configured problems root
// (default docs/problems).
func Manual(ctx context.Context, door FrontDoor, ledgers Ledgers, problems, slug string, reason Reason) (PushResult, error) {
	sig, ok := NewSignal(slug, reason)
	if !ok {
		return PushResult{Rejected: true}, nil
	}
	return dispatchSignal(ctx, door, ledgers, problems, sig, SourceManual)
}

// dispatchSignal is the shared delivery core: locate the named ledger, refuse to
// reopen a closed one, else touch the front door. A closed ledger short-circuits
// before any touch, so the frozen file is never reopened (DINV-3); an absent
// ledger still delivers (the front door also bootstraps a session). A front-door
// failure is returned as an infrastructure error — a single explicit delivery's
// caller decides whether to retry, unlike the backstop's resilient sweep. The
// ledger's class is recognized under the configured problems root.
func dispatchSignal(ctx context.Context, door FrontDoor, ledgers Ledgers, problems string, sig Signal, source Source) (PushResult, error) {
	res := PushResult{Signal: sig}
	doc, found, err := findLedger(ctx, ledgers, sig.Slug)
	if err != nil {
		return PushResult{}, fmt.Errorf("wake: locating the delivery ledger for %q: %w", sig.Slug, err)
	}
	if found {
		if l, defects := argusledger.Parse(problems, verb, doc.Path, doc.Bytes); len(defects) == 0 && Dispose(l) == Closed {
			res.NoOp = NoOpClosed
			return res, nil
		}
	}
	if err := door.Touch(ctx, sig); err != nil {
		return PushResult{}, fmt.Errorf("wake: front-door touch for %q: %w", sig.Wire(), err)
	}
	res.Delivered = true
	res.Source = source
	return res, nil
}

// findLedger locates the delivery ledger whose path names the given slug.
func findLedger(ctx context.Context, ledgers Ledgers, slug string) (Doc, bool, error) {
	docs, err := ledgers.List(ctx)
	if err != nil {
		return Doc{}, false, err
	}
	for _, d := range docs {
		if slugFromPath(d.Path) == slug {
			return d, true, nil
		}
	}
	return Doc{}, false, nil
}
