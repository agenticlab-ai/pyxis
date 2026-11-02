package watch

// The status verb's text render (design:wake-watch-v2 §3.6) — a plain table
// plus the checker's liveness footer. Rendering never fails: every field
// this file cannot fill in (an absent age, an unresolved wait kind) prints a
// placeholder rather than an error, because a render that stops halfway is
// worse than one that shows what it knows (UB-2's "never presented as
// healthy" cuts the other way too — a placeholder here is always visibly a
// placeholder, never a fabricated zero).

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// WriteReport renders r as a fixed-width table (one row per task, TaskID
// order) followed by the checker's liveness line.
func WriteReport(w io.Writer, r Report) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	// A partial table is still the status the operator asked for; write errors are not a render failure.
	_, _ = fmt.Fprintln(tw, "TASK\tSTATE\tWAIT\tAGE\tFLAG")
	for _, t := range r.Tasks {
		_, _ = fmt.Fprintln(tw, taskRow(t))
	}
	_ = tw.Flush()
	if r.ReviewUnknown != nil {
		_, _ = fmt.Fprintf(w, "\nREVIEWS\nreview state unknown — source %s · %s: %s\n", r.ReviewUnknown.Source, r.ReviewUnknown.Kind, r.ReviewUnknown.Reason)
	} else if len(r.Reviews) > 0 {
		_, _ = fmt.Fprintln(w, "\nREVIEWS")
		rw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		_, _ = fmt.Fprintln(rw, "OBLIGATION\tTASK\tPROPOSAL\tREVISION\tBACKEND\tSTATE\tAGE\tATTEMPTS\tRECOVERY\tFAILURE\tFLAG")
		for _, review := range r.Reviews {
			_, _ = fmt.Fprintln(rw, reviewRow(review))
		}
		_ = rw.Flush()
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, livenessLine(r.Liveness))
}

func reviewRow(r ReviewWakeRow) string {
	age := "unknown"
	recovery := "unknown"
	if r.Eligibility != ReviewUnobserved && r.Eligibility != ReviewHandoffRepair {
		age = formatAge(time.Duration(r.AgeS) * time.Second)
		recovery = fmt.Sprintf("%d/%d (%s)", r.RecoveryCount, r.RecoveryBound, r.RecoveryEpoch)
	}
	failure := r.LastFailureClass
	if failure == "" {
		failure = "-"
	}
	flag := "pending"
	switch r.Eligibility {
	case ReviewExhausted:
		flag = "⚠ exhausted — rearm or verify"
	case ReviewHandoffRepair:
		flag = "⚠ handoff repair required"
	case ReviewUnobserved:
		flag = "⚠ unobserved"
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s", r.ID, r.TaskID, r.ProposalRef, r.Revision, r.Harness, r.Eligibility, age, r.AttemptCount, recovery, failure, flag)
}

func taskRow(t TaskStatus) string {
	state := string(t.State)
	switch t.State {
	case StateLanded:
		state = "landed ✓" // ✓
	case StateUnknown:
		if t.UnknownReason == "offline" {
			state = "unknown (offline, retrying)" // UB-1's exact row text
		} else {
			state = fmt.Sprintf("unknown (%s)", t.UnknownReason)
		}
	}
	wait := "-"
	if t.Wait != WaitNone {
		wait = string(t.Wait)
	}
	age := "-"
	if t.HasAge {
		age = formatAge(t.Age)
	}
	flag := ""
	if t.Stale {
		flag = "⚠ stale" // ⚠
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", t.TaskID, state, wait, age, flag)
}

func livenessLine(l Liveness) string {
	if l.Defect != "" {
		return fmt.Sprintf("checker: ⚠ %s", l.Defect)
	}
	switch {
	case l.BoundStale:
		return fmt.Sprintf("checker: ⚠ last check %s ago — past the %s alarm bound (cadence %s)",
			formatAge(l.Age), formatAge(l.Bound), formatAge(l.Cadence))
	case l.DisplayStale:
		return fmt.Sprintf("checker: ⚠ last check %s ago — missed a check (cadence %s)",
			formatAge(l.Age), formatAge(l.Cadence))
	default:
		return fmt.Sprintf("checker: last check %s ago (cadence %s)", formatAge(l.Age), formatAge(l.Cadence))
	}
}

// formatAge renders a duration at the coarsest useful granularity — "3d5h",
// "1h20m", "12m", "45s" — so both day-scale review/worker waits and
// second-scale checker cadences read naturally in the same table. Negative
// durations (a clock skew edge) clamp to zero rather than printing a
// confusing sign.
func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Truncate(time.Second)
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	mins := d / time.Minute
	d -= mins * time.Minute
	secs := d / time.Second
	switch {
	case days > 0:
		return fmt.Sprintf("%dd%dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm", mins)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}
