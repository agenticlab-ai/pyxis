// This file owns the maintenance sweep's human face (maintenance-sweep-v1 §3.3):
// the stderr rendering of the same report the machine face carries on stdout —
// face parity by construction (both read one Report). It is explicit on
// emptiness — every clean detector prints its own "0 findings" line, never
// silence (prd NFR-3, the PRD's F1 empty state) — and it warns, never hides: a
// non-clean working tree prints a dirty warning naming the evaluated base, and
// every fault (pass-level or detector-level) is named. Like the machine face it
// carries no wall-clock content, so two runs on one state render identically
// (DINV-2). Stdlib only.
package sweep

import (
	"fmt"
	"io"
	"strings"
)

// WriteHuman renders r's human face to w (the pass writes it to stderr). It
// draws from the same Report the machine face marshals, so the two faces carry
// the same findings, waivers, and faults — parity is structural, not a parallel
// code path (prd NFR-3). Detectors are rendered in the fixed canonical order and
// each detector's findings/waivers lexicographically, matching the machine face.
func WriteHuman(w io.Writer, r Report) {
	commit := r.EvaluatedState.Commit
	if commit == "" {
		commit = "(unknown)"
	}
	fmt.Fprintf(w, "maintenance sweep — evaluated commit %s\n", commit)
	if r.EvaluatedState.Dirty {
		// A non-clean tree never certifies silently: the findings are measured
		// against the committed base, not the exact working bytes (§3.3).
		fmt.Fprintf(w, "  warning: working tree is dirty — findings are measured against the committed base %s, not the uncommitted changes\n", commit)
	}

	// Pass-level faults precede the detectors: the pass short-circuited before or
	// around them (misparse, unreadable state, malformed policy — DINV-3).
	for _, f := range canonicalFaults(r.Faults) {
		fmt.Fprintf(w, "fault: %s\n", f)
	}

	for _, d := range canonicalDetectors(r.Detectors) {
		writeDetectorFace(w, d)
	}

	fmt.Fprintf(w, "outcome: %s (exit %d)\n", r.Outcome, r.ExitCode())
}

// writeDetectorFace renders one detector's contribution: its fault if it faulted,
// else its finding count with an explicit "0 findings" line when clean, each
// finding's three descriptive fields, and every applied waiver with its reason
// (suppression is always visible — DINV-5).
func writeDetectorFace(w io.Writer, d DetectorResult) {
	if d.Fault != "" {
		// A faulted detector yields no finding; its fault is named, never masked
		// (DINV-4). The remaining detectors are unaffected.
		fmt.Fprintf(w, "detector %s: FAULT — %s\n", d.Name, d.Fault)
	} else if len(d.Findings) == 0 {
		// The explicit empty-state line — never silence (prd NFR-3, F1).
		fmt.Fprintf(w, "detector %s: 0 findings\n", d.Name)
	} else {
		fmt.Fprintf(w, "detector %s: %s\n", d.Name, plural(len(d.Findings), "finding"))
		for _, f := range d.Findings {
			fmt.Fprintf(w, "  - %s\n", f.Subject)
			fmt.Fprintf(w, "      evidence: %s\n", f.Evidence)
			fmt.Fprintf(w, "      remedy:   %s\n", f.Remedy)
		}
	}
	for _, wv := range d.WaiversApplied {
		fmt.Fprintf(w, "  waiver applied (%s): %s — %s\n", wv.Kind, wv.Subject, wv.Reason)
	}
}

// plural renders "1 finding" / "2 findings" without a wall-clock or locale leak.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// HumanFace renders r's human face to a string — the in-process form the tests
// assert face parity against.
func (r Report) HumanFace() string {
	var b strings.Builder
	WriteHuman(&b, r)
	return b.String()
}
