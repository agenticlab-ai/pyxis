package argusledger

// The wake-staleness defect — silence made loud. An await whose age exceeds
// the configured staleness bound with no resume delivered is a *defect*, never
// a resting state (docs/tech-design/wake-channel-v1.md §3.2, DINV-5;
// prd:argus-unattended-drive UB-9). It is computed from ledger data alone —
// the outstanding await set re-derived from the journal (DeriveAwaits) and each
// await's since-date — so it surfaces identically at any state read or drive
// touchpoint, designed to fire precisely when both delivery paths are dead
// (the both-paths-dead drill). The clock is injected (`now`), never read here,
// so the computation is hermetic and deterministic (the repo Go rules).
//
// The bound is the config surface's `channel.staleness_bound` (config.Channel),
// pinned there to strictly exceed the tick interval so this alarm can never
// fire on an await the very next tick would legally serve (§3.2).

import (
	"fmt"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// sinceLayout is the closed since-date layout the await grammar registers
// (YYYY-MM-DD), parsed as a UTC midnight so ages are wall-clock-day stable.
const sinceLayout = "2006-01-02"

// WakeStaleness returns one named wake-staleness defect per outstanding await
// whose age at `now` exceeds `bound` (DINV-5). The awaits are the still-
// outstanding set — a resumed (cleared) await has already left it, so presence
// in the set is exactly "no resume delivered". Each defect names the stale
// await's goal object, its condition, and its age; an await within the bound
// yields nothing. An await whose since-date does not parse is itself a named
// defect (fail loud, never a silent skip) — the grammar admits the digit shape
// but not a nonexistent calendar date. The result is empty (never nil-panic)
// on an empty set. It is a pure read over `awaits`, `bound`, and `now`.
func WakeStaleness(verb, filePath string, awaits []Await, bound time.Duration, now time.Time) []schema.Defect {
	var defects []schema.Defect
	for _, a := range awaits {
		since, err := time.Parse(sinceLayout, a.Since)
		if err != nil {
			defects = append(defects, def(verb, filePath, "await-since",
				fmt.Sprintf("await %q carries an unparseable since-date %q", a.GoalObject, a.Since),
				"an await's since-date is a real calendar date in YYYY-MM-DD form (wake-channel-v1 §3.1)"))
			continue
		}
		age := now.Sub(since.UTC())
		if age > bound {
			defects = append(defects, def(verb, filePath, "wake-staleness",
				fmt.Sprintf("await %q (%s) is stale: age %s exceeds the staleness bound %s with no resume delivered",
					a.GoalObject, a.Condition, humanizeAge(age), bound),
				"deliver the awaited completion (a manual front-door touch resumes the drive), or fix the wake channel — an unresumed await past its bound is a defect, never a resting state (wake-channel-v1 DINV-5)"))
		}
	}
	return defects
}

// humanizeAge renders an await's age in whole days and hours for the defect
// message — "3d", "1d16h", "10h" — truncated to the hour. Ages below an hour
// (only reachable under a sub-hour bound) render "0h".
func humanizeAge(d time.Duration) string {
	d = d.Truncate(time.Hour)
	days := int(d / (24 * time.Hour))
	hours := int((d % (24 * time.Hour)) / time.Hour)
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case days > 0:
		return fmt.Sprintf("%dd", days)
	default:
		return fmt.Sprintf("%dh", hours)
	}
}
