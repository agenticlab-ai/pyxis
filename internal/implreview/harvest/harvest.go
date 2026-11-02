// Package harvest is the impl-review measurement harvest and the operator-
// disposition attribution binding (docs/tech-design/impl-review-v1.md §3.2
// attribution, §3.3 the measurement harvest, §3.7 DINV-4/DINV-8; plan T11 —
// FR-13, FR-16, NFR-4, M1, M2, A-4, A-5).
//
// The harvest ENUMERATES a repo's FULL proposal universe (through an injected
// [Enumerator] — the GitHub PR set is the v1 path, faked in tests), PARSES each
// persisted impl-review-report/v1 payload from its medium (through the injected
// [persist.Reader]), and computes the corpus reading PAYLOAD FIELDS ONLY, never
// rendered prose (FR-13/NFR-4, DINV-8):
//
//   - coverage (M1) — report-or-`skipped` present ÷ the FULL universe. A
//     NEVER-REVIEWED proposal counts AGAINST coverage; an `impl-review-absent`
//     marker is NOT a review, so a proposal with only an absence marker is
//     uncovered.
//   - precision (M2) — operator-`accepted` ÷ operator-`disposed` blocking
//     findings, over the disposition floor. An UNATTRIBUTABLE operator
//     disposition (whose disposition-artifact author is not the operator) is
//     EXCLUDED from the measured corpus (§3.1/DINV-4, A-5, ABUSE-5).
//   - the nit rate — operator `rejected` per review.
//   - the miss corpus — the EVT-6 review-miss records (a never-reviewed or
//     review-absent proposal).
//
// The attribution binding (FR-16/DINV-4) is the anti-forgery ground: an
// operator-class disposition is valid ONLY when its `attribution` resolves to
// the OPERATOR'S OWN medium-authored artifact — a GitHub PR review/comment whose
// `user` IS the operator — never a payload-writable field a worker could set. A
// FORGED "operator accepted" (disposition-artifact author ≠ the operator
// identity) is dropped from the corpus. Attribution is resolved through an
// injected code-host client ([AuthorResolver]), faked in tests — no network.
//
// This package reads; it never writes and never mutates a proposal. It is
// hermetic by construction: every external edge (the universe, the payload read,
// the author lookup) is an injected interface. Stdlib only, per the repo Go
// rules.
package harvest

import (
	"context"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// Proposal is one member of the enumerated universe — the harvest's medium-
// neutral view of a proposal and where (if anywhere) its review lives. The
// [Enumerator] yields the FULL universe as these records (never just the
// reviewed subset — the harvest is the only source that sees a never-reviewed
// proposal, so it enumerates all — §3.3).
//
// Review is nil when the proposal was NEVER reviewed (no review artifact on its
// medium) — that proposal is uncovered and a miss. A proposal whose medium
// carries only an `impl-review-absent` marker is likewise Review==nil with
// Absent==true: the absence is a recorded signal, but it is NOT a review, so it
// does not count toward coverage (§3.3, UB-5's absence-is-visible outcome).
type Proposal struct {
	// Ref is the proposal's own medium reference (e.g. "github:owner/repo#412") —
	// the identity the harvest keys a miss/coverage record by.
	Ref string
	// Review is the persist.Ref of the proposal's one self-contained review
	// artifact, or nil when the proposal was never reviewed. The harvest reads
	// the payload back through the injected [persist.Reader] using this Ref.
	Review *persist.Ref
	// Absent is true when the proposal carries a recorded review-absence (an
	// `impl-review-absent` alternate signal — UB-5) rather than a review. An
	// absence is a visible miss, never a review: coverage does not count it, and
	// its miss record carries the absence reason. Absent is meaningful only when
	// Review is nil.
	Absent bool
	// AbsentReason is the recorded cause of an absence (from the alternate
	// signal), surfaced verbatim in the miss corpus. Empty when Absent is false
	// or the cause was not recorded.
	AbsentReason string
}

// Enumerator yields a repo's FULL proposal universe — every proposal, reviewed
// or not (§3.3). It is the consumer-side seam over the code host's proposal set
// (GitHub: the PR set — the v1 path; Carina: a worktracker query — later). The
// production adapter enumerates the live code host; tests inject a fake, so the
// harvest is hermetic (no network).
//
// It returns the universe as a slice (today's scale is inside budget — §3.3, R6;
// incremental harvest is the named future optimization). An enumeration failure
// is an error, never a partial silent universe (a shortened universe would
// silently inflate coverage).
type Enumerator interface {
	Enumerate(ctx context.Context) ([]Proposal, error)
}

// EnumeratorFunc adapts a plain function to [Enumerator] — the ergonomic form a
// caller or test uses when a whole type would be ceremony.
type EnumeratorFunc func(ctx context.Context) ([]Proposal, error)

// Enumerate implements [Enumerator].
func (f EnumeratorFunc) Enumerate(ctx context.Context) ([]Proposal, error) { return f(ctx) }

// MissKind classifies a review-miss (EVT-6) — WHY a proposal is in the miss
// corpus. Both kinds count against coverage; they differ only in whether the
// absence was recorded visibly (UB-5) or the proposal was simply never invoked.
type MissKind string

const (
	// MissNeverReviewed — the proposal carries no review artifact and no
	// absence marker: the reviewer was never invoked on it. It counts against
	// coverage (§3.3).
	MissNeverReviewed MissKind = "never-reviewed"
	// MissAbsent — the proposal carries a recorded review-absence (an
	// `impl-review-absent` alternate signal, UB-5) but no review. The absence is
	// visible, but it is NOT a review, so it still counts against coverage.
	MissAbsent MissKind = "review-absent"
	// MissUnreadable — the proposal's review artifact could not be read or parsed
	// back to a payload (a corrupt/tampered artifact). It is not a completed
	// review, so it counts against coverage and is recorded as a miss with its
	// cause, never silently treated as covered.
	MissUnreadable MissKind = "unreadable"
)

// MissRecord is one entry of the miss corpus (EVT-6) — a proposal that was not
// reviewed to a usable payload. It carries the proposal ref, the kind, and the
// recorded cause (an absence reason or a read/parse error) so the corpus is
// self-describing without reading any prose (DINV-8).
type MissRecord struct {
	// Proposal is the proposal's medium ref.
	Proposal string
	// Kind is why the proposal is a miss.
	Kind MissKind
	// Cause is the recorded cause — the absence reason (MissAbsent) or the
	// read/parse error text (MissUnreadable); empty for MissNeverReviewed.
	Cause string
}

// Metrics is the harvested corpus reading — every field computed from payload
// fields alone over the FULL enumerated universe (DINV-8/NFR-4). A consumer (the
// T13 checkpoint) reads these; the harvest itself gates nothing (AP-24: the
// corpus is derived and governs nothing).
type Metrics struct {
	// Universe is the size of the FULL enumerated proposal set — the coverage
	// denominator (§3.3: coverage is over the full universe, not the reviewed
	// subset).
	Universe int
	// Covered is the count of proposals with a review outcome present —
	// report-or-`skipped` — the coverage numerator. A never-reviewed proposal or
	// a bare absence marker is NOT covered.
	Covered int

	// BlockingDisposed is the count of operator-DISPOSED blocking findings whose
	// operator disposition is ATTRIBUTABLE to the operator (accepted or rejected)
	// — the precision denominator (M2). An unattributable operator disposition is
	// excluded (DINV-4).
	BlockingDisposed int
	// BlockingAccepted is the count of attributable operator-ACCEPTED blocking
	// findings — the precision numerator (M2).
	BlockingAccepted int

	// Reviews is the count of proposals with a completed review payload — the nit
	// denominator (nit rate = operator `rejected` per review). An absence or a
	// never-reviewed proposal is not a review.
	Reviews int
	// OperatorRejected is the count of attributable operator-REJECTED findings —
	// the nit numerator (a rejected finding is a nit the operator waved off).
	OperatorRejected int

	// ExcludedDispositions is the count of operator dispositions DROPPED as
	// unattributable (a forged/mismatched author — ABUSE-5). Surfaced so the
	// exclusion is observable, never silent.
	ExcludedDispositions int

	// Misses is the miss corpus (EVT-6) — every never-reviewed / absent /
	// unreadable proposal, in enumeration order.
	Misses []MissRecord
}

// Coverage is M1 — report-or-`skipped` present ÷ the FULL universe (§3.3). A
// never-reviewed proposal counts against it. It is 0 for an empty universe
// (no proposals, no coverage claim), never a divide-by-zero.
func (m Metrics) Coverage() float64 {
	if m.Universe == 0 {
		return 0
	}
	return float64(m.Covered) / float64(m.Universe)
}

// Precision is M2 — operator-`accepted` ÷ operator-`disposed` blocking findings
// (§3.3), evaluated over the ATTRIBUTABLE dispositions only (DINV-4). The second
// return reports whether the disposition floor is met: a caller (the T13
// checkpoint) trusts the value only when ok is true — below the floor the ratio
// is not yet a basis (the design's ≥ 5 floor is applied by the caller with
// [Metrics.BlockingDisposed]; this method reports the raw ratio and floor==0
// guard). Precision is 0 with ok==false when no attributable blocking finding
// was disposed (never a divide-by-zero).
func (m Metrics) Precision(floor int) (value float64, ok bool) {
	if m.BlockingDisposed == 0 || m.BlockingDisposed < floor {
		return 0, false
	}
	return float64(m.BlockingAccepted) / float64(m.BlockingDisposed), true
}

// NitRate is the operator `rejected` count per completed review (§3.3). It is 0
// for zero reviews (no reviews, no nit signal), never a divide-by-zero.
func (m Metrics) NitRate() float64 {
	if m.Reviews == 0 {
		return 0
	}
	return float64(m.OperatorRejected) / float64(m.Reviews)
}

// Harvest enumerates the full proposal universe, reads each persisted payload
// back through the port, resolves operator dispositions through the operator
// resolver, and computes the corpus from payload fields alone (§3.3, DINV-8).
//
// The three edges are injected interfaces so the harvest is hermetic:
//   - enum enumerates the FULL universe (never just the reviewed subset).
//   - port reads a proposal's persisted payload back from its medium (the same
//     port the reviewer persisted through — one parser, §3.2).
//   - op resolves whether an operator disposition's attribution is the operator's
//     own medium-authored artifact (FR-16/DINV-4); an unattributable operator
//     disposition is excluded from the corpus.
//
// A never-reviewed proposal (Review==nil) is enumerated and counted against
// coverage, and recorded in the miss corpus — the harvest is the ONLY source
// that sees it, which is why it enumerates ALL, not just the reviewed set (§3.3).
//
// An enumeration error aborts (a partial universe would silently inflate
// coverage). A per-proposal READ/parse error does NOT abort the whole harvest:
// it is recorded as a MissUnreadable miss (uncovered) so one corrupt artifact
// never silently drops the metric or hides the rest of the corpus.
func Harvest(ctx context.Context, enum Enumerator, port persist.Reader, op OperatorResolver) (Metrics, error) {
	if enum == nil {
		return Metrics{}, fmt.Errorf("harvest: nil Enumerator")
	}
	if port == nil {
		return Metrics{}, fmt.Errorf("harvest: nil persist.Reader")
	}
	if op == nil {
		return Metrics{}, fmt.Errorf("harvest: nil OperatorResolver")
	}

	universe, err := enum.Enumerate(ctx)
	if err != nil {
		return Metrics{}, fmt.Errorf("harvest: enumerate universe: %w", err)
	}

	m := Metrics{Universe: len(universe)}

	for i := range universe {
		p := universe[i]

		// A never-reviewed proposal or a bare absence marker is NOT a review: it
		// counts against coverage and is a miss (§3.3). No payload to parse.
		if p.Review == nil {
			m.Misses = append(m.Misses, missFor(p))
			continue
		}

		payload, rerr := port.Read(ctx, *p.Review)
		if rerr != nil {
			// A corrupt/unreadable artifact is not a completed review: record it as a
			// miss (uncovered), never silently treat it as covered and never abort the
			// whole harvest over one proposal.
			m.Misses = append(m.Misses, MissRecord{
				Proposal: p.Ref,
				Kind:     MissUnreadable,
				Cause:    rerr.Error(),
			})
			continue
		}

		// A completed review payload: fold its coverage, precision, and nit
		// contributions — from payload fields ONLY (DINV-8).
		m.foldReview(ctx, payload, op)
	}

	return m, nil
}

// foldReview folds one completed review payload into the running metrics, reading
// PAYLOAD FIELDS ONLY (DINV-8): the envelope verdict for coverage, and each
// finding's severity + attributed operator disposition for precision and the nit
// rate. It never reads the rendered prose.
func (m *Metrics) foldReview(ctx context.Context, r *report.Report, op OperatorResolver) {
	// Coverage: a review with a report outcome present is covered. Every parsed
	// payload IS a report (report-or-`skipped` present — §3.3): a `skip`-band
	// review records a skip outcome and still counts as covered (the reviewer WAS
	// invoked; it is not a never-reviewed proposal). So a present payload is
	// covered regardless of band.
	m.Covered++
	m.Reviews++

	// Precision + nit: walk every finding across every round. A finding's
	// disposition is authoritative on the finding; the LATEST disposition per
	// finding governs (a finding re-disposed in a later round supersedes an
	// earlier one), so dedupe by finding id keeping the last-seen disposition.
	type dispo struct {
		severity string
		d        *report.Disposition
	}
	latest := make(map[string]dispo)
	order := make([]string, 0)
	for ri := range r.Rounds {
		round := &r.Rounds[ri]
		for fi := range round.Findings {
			f := &round.Findings[fi]
			if _, seen := latest[f.ID]; !seen {
				order = append(order, f.ID)
			}
			latest[f.ID] = dispo{severity: f.Severity, d: f.Disposition}
		}
	}

	for _, id := range order {
		fd := latest[id]
		d := fd.d
		if d == nil || d.Actor != report.ActorOperator {
			// Only OPERATOR dispositions feed precision and the nit rate; a worker
			// disposition (applied/declined) is the worker's own inline act, not the
			// operator's judgment (§3.3).
			continue
		}

		// FR-16/DINV-4: an operator disposition counts ONLY when its attribution
		// resolves to the operator's own medium-authored artifact. A forged one
		// (author ≠ operator) is EXCLUDED from the corpus — never counted toward
		// precision or the nit rate.
		attributed, aerr := op.IsOperator(ctx, d.Attribution)
		if aerr != nil || !attributed {
			m.ExcludedDispositions++
			continue
		}

		switch d.Verdict {
		case report.VerdictAccepted:
			// An operator-accepted blocking finding is a true positive; it feeds M2.
			// Only BLOCKING findings feed precision (M2 is over blocking findings —
			// §3.3): an accepted advisory is not a precision event.
			if fd.severity == report.SeverityBlocking {
				m.BlockingDisposed++
				m.BlockingAccepted++
			}
		case report.VerdictRejected:
			// An operator-rejected finding is a nit the operator waved off — the nit
			// numerator. A rejected BLOCKING finding is also a disposed-not-accepted
			// event for M2's denominator (it was operator-disposed but not accepted).
			m.OperatorRejected++
			if fd.severity == report.SeverityBlocking {
				m.BlockingDisposed++
			}
		default:
			// applied/declined are worker verdicts; an operator disposition carrying
			// one is malformed for the operator actor and contributes nothing (the
			// payload validator — T1 — is the authority that rejects a malformed
			// payload; the harvest simply does not count it).
		}
	}
}

// missFor builds the miss record for a proposal with no review payload — a
// recorded absence (visible but not a review) or a never-invoked proposal.
func missFor(p Proposal) MissRecord {
	if p.Absent {
		return MissRecord{Proposal: p.Ref, Kind: MissAbsent, Cause: p.AbsentReason}
	}
	return MissRecord{Proposal: p.Ref, Kind: MissNeverReviewed}
}

// ErrUnresolvedAttribution is returned by an [OperatorResolver] when a
// disposition's attribution cannot be resolved to any medium author (a dangling
// reference). The harvest treats an unresolved attribution the same as a
// non-operator one — the disposition is excluded — so a dangling reference never
// silently counts as the operator.
var ErrUnresolvedAttribution = errors.New("harvest: disposition attribution resolves to no medium author")
