package model

import "time"

// The append log carries two new entry kinds in this task: free-form comments
// (dialogue, appended forever) and decision-records (frozen, typed judgments
// resolved latest-wins per namespace) — docs/tech-design/local-work-tracker.md
// §3.1, §3.14, DINV-5. Comments reuse LogComment with a free payload; records
// reuse LogDecisionRecord and carry a closed Namespace naming which judgment the
// record freezes. The model owns the latest-wins resolution so the port, the CLI,
// and the later triage predicate share exactly one implementation (DINV-6).

// Namespace names which frozen judgment a decision-record carries — the closed,
// engine-recognized set (§3.14, DINV-11). A record whose namespace is out of set
// is a named defect, never a lenient write.
type Namespace string

const (
	// NamespaceTriageDisposition freezes the triage decision (its value is a
	// triage-disposition: plan · operator · reject · skip — §3.14).
	NamespaceTriageDisposition Namespace = "triage-disposition"
	// NamespaceCharter freezes the plan-time charter for the item.
	NamespaceCharter Namespace = "charter"
	// NamespaceFoundation freezes a foundational judgment the item rests on.
	NamespaceFoundation Namespace = "foundation"
)

// Namespaces is the closed decision-record namespace set, in canonical order
// (§3.14, DINV-11).
var Namespaces = []Namespace{NamespaceTriageDisposition, NamespaceCharter, NamespaceFoundation}

// ValidNamespace reports whether ns is in the closed namespace set.
func ValidNamespace(ns Namespace) bool { return in(Namespaces, ns) }

// DecisionRecord is one typed, frozen judgment on a work item — a namespaced,
// latest-wins entry in the append log (§3.1, FR-9). The payload is the judgment
// itself (e.g. the triage-disposition value); the store stamps At on append.
// Records are never rewritten: a new judgment appends a fresh record and the
// latest one per namespace is authoritative.
type DecisionRecord struct {
	Namespace Namespace
	Author    string
	At        time.Time
	Payload   string
}

// LatestRecord returns the authoritative decision-record for namespace ns — the
// last one appended (latest-wins, DINV-5) — and whether one exists. It is the
// single medium-neutral resolution the port, the CLI, and the triage predicate
// all read, so they can never disagree on the frozen judgment (DINV-6). The log
// is scanned in append order, so the final matching entry wins.
func (w WorkItem) LatestRecord(ns Namespace) (DecisionRecord, bool) {
	var latest DecisionRecord
	found := false
	for _, e := range w.Log {
		if e.Kind == LogDecisionRecord && Namespace(e.Namespace) == ns {
			latest = DecisionRecord{Namespace: ns, Author: e.Author, At: e.At, Payload: e.Payload}
			found = true
		}
	}
	return latest, found
}
