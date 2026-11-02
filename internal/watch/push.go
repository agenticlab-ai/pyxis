package watch

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The reserved push-channel slot (design:wake-watch-v2#3.7;
// prd:self-drive-wake/FR-11): a narrow, hint-only contract a future push
// adapter fills to shorten RunLive's wait — never to carry state. A
// delivered hint's only effect is causing the next derivation to run at
// once, exactly as if the cadence timer had just fired: the same reads run
// and the same sweep decision follows, so pull, push-hint, and manual
// invocation stay indistinguishable to everything downstream (AP-2). An
// absent slot (a nil hints channel wired into RunLive) and a dead one (a
// live Stub nobody delivers to any more) are provably the same state to
// RunLive — proved by TestKillStubMatchesNoSlotAtAll — so killing an
// adapter, at any point in a drive, changes nothing but the latency of the
// next check (FR-11).

// Hint is the push slot's payload: closed and field-less by construction. A
// delivery means exactly "check now"; there is no field for a payload to
// occupy, so the loop can never read a byte of adapter-supplied state or
// instruction off it (FR-10's no-inbound-content discipline, extended to
// this surface).
type Hint struct{}

// hintVerb names this parser in every defect it emits (schema.Defect.Verb).
const hintVerb = "watch-push"

// ParseHint reduces a raw push-adapter payload to the closed Hint schema.
// The only well-formed wire form is the empty JSON object — any key at all
// is content this contract has no field for, so it is rejected as a named
// defect rather than silently discarded, never forwarded (mirrors
// internal/wake.Grammar's ParseClosed: a keyed representation has no
// harmless place for an extra key to hide, so an unrecognized key is itself
// the defect).
func ParseHint(raw string) (Hint, *schema.Defect) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var fields struct{}
	if err := dec.Decode(&fields); err != nil {
		d := hintParseDefect(raw, "closed parse: "+err.Error())
		return Hint{}, &d
	}
	if dec.More() {
		d := hintParseDefect(raw, "closed parse: trailing data after the hint object")
		return Hint{}, &d
	}
	return Hint{}, nil
}

func hintParseDefect(raw, why string) schema.Defect {
	return schema.Defect{
		Verb:   hintVerb,
		File:   raw,
		Op:     "parse",
		Reason: "push hint failed closed parse: " + why,
		Remedy: `a push hint is exactly {} — no field exists for a push adapter to carry a payload on; the loop reads nothing beyond "check now"`,
	}
}

// PushSlot is the one interface a future push adapter implements
// (design:wake-watch-v2#3.7's "one narrow interface"). Deliver is the
// adapter's only write: a well-formed raw payload advances RunLive's next
// check to now; a malformed one is refused with a named defect and never
// reaches the loop. This is the conformance bar every adapter is held to —
// Stub below is v1's only implementation, proving it; a new adapter is
// conformant exactly when it passes the same drills Stub does
// (TestRunLiveHintAdvancesNextCheckOnly proves delivery only ever
// accelerates; TestKillStubMatchesNoSlotAtAll proves death is inert).
type PushSlot interface {
	Deliver(ctx context.Context, raw string) *schema.Defect
}

// Stub is the push slot's only v1 implementation: a real, working in-process
// mailbox, never a network listener or any other live transport (a real
// adapter is WR-1's future ground — reserved, not built). It exists to
// prove the contract: production wiring (cmd/pyxis/watch.go) passes a nil
// hints channel into RunLive, so v1 ships the slot reserved and empty.
type Stub struct {
	hints chan struct{}
}

// NewStub constructs an empty mailbox — nothing pending, nothing delivered.
func NewStub() *Stub {
	return &Stub{hints: make(chan struct{}, 1)}
}

// Deliver parses raw and, if well-formed, wakes RunLive's next wait.
// Multiple pending deliveries coalesce to one wake: RunLive re-derives the
// whole frontier on any wake regardless (DINV-2/DINV-3), so a second queued
// hint ahead of the loop draining the first would teach it nothing new.
func (s *Stub) Deliver(ctx context.Context, raw string) *schema.Defect {
	if _, defect := ParseHint(raw); defect != nil {
		return defect
	}
	select {
	case s.hints <- struct{}{}:
	default:
	}
	return nil
}

// Hints is the channel RunLive selects on. To run the kill-stub drill,
// simply stop calling Deliver: the channel falls silent, which RunLive
// cannot distinguish from having no slot at all — a nil channel never fires
// either. Never close this channel to simulate death: a closed channel is
// always ready to receive, which would hot-loop the select instead of
// falling back to cadence.
func (s *Stub) Hints() <-chan struct{} { return s.hints }

var _ PushSlot = (*Stub)(nil)
