package preflight

import (
	"errors"
	"fmt"
	"strings"
)

// ErrRefused is the generic refusal sentinel: errors.Is(err, ErrRefused)
// matches any check's refusal regardless of which one fired — a caller
// that only needs "was this refused" branches on this; one that needs the
// specifics uses errors.As for *RefusalError.
var ErrRefused = errors.New("preflight: a check refused")

// RefusalError renders a refusing check's Outcome as an error — the
// registry's refusal renderer (design §3.1's Key interfaces). The message
// carries the cause, the conflicting evidence, and the remedy (which, for
// every check that has one, already names its route in prose); the
// structured Outcome fields remain available via errors.As for a caller
// that renders its own report (EVT-2's remedy_present/route_present, a
// future preview table).
type RefusalError struct {
	Outcome Outcome
}

func (e *RefusalError) Error() string {
	var b strings.Builder
	b.WriteString(e.Outcome.Reason)
	if len(e.Outcome.Conflicts) > 0 {
		fmt.Fprintf(&b, " — conflicts: %s", strings.Join(e.Outcome.Conflicts, ", "))
	}
	if e.Outcome.Remedy != "" {
		fmt.Fprintf(&b, "; %s", e.Outcome.Remedy)
	}
	return b.String()
}

// Is reports whether target is ErrRefused, so errors.Is(err, ErrRefused)
// matches any *RefusalError regardless of which check produced it.
func (e *RefusalError) Is(target error) bool { return target == ErrRefused }
