// Package verdict is the merge lane's reviewer-verdict check arm and its
// structured, versioned lane-verdict record (merge-lane-v1 §3.6, DINV-3/DINV-9;
// plan T3). Two things live here:
//
//   - The registered review-report validation arm (Validate). Where an AUTO
//     class requires an approving review, this arm — not the classifier, and
//     never the proposal's own tree — validates the artifact class's review
//     report and PUBLISHES a check state (approving | non-approving). The lane
//     consumes that published state through its already-pinned check-state
//     snapshot; it never parses the proposal's content for the verdict, which is
//     exactly where a planted verdict is told apart from a produced one (DINV-3,
//     A-7's red fixture). A report that fails structural validity, that carries a
//     non-approving verdict, whose recorded subject fingerprint does not match
//     the evaluated content, or whose authoring commits lack the reviewer-flow
//     attribution the shipped attribution arms enforce, publishes a NON-approving
//     state — fail-closed, naming why.
//
//   - The lane-verdict record (record.go): the structured, versioned block a
//     fresh record appends to per evaluation.
//
// This arm is part of the lane's trust base: its code home is inside the
// self-unlock minimum (internal/lane/**, policy.SelfUnlockMinPaths), so an edit
// to it floor-hits HUMAN under the mainline policy, and — like the classifier —
// it runs from the trusted mainline context, never built or executed from the
// evaluated proposal's tree (DINV-9). The pure core takes every input already
// derived from that mainline context; the git-backed derivation of a report's
// authoring-commit attribution is a thin seam the acting adapter (T4) supplies.
// Stdlib only, per the repo Go rules; the parse is closed, never a lenient guess.
package verdict

import (
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// CheckContext is the required check context name the arm publishes under. The
// lane consumes THIS context's state through the pinned check-state snapshot; a
// proposal cannot rename or forge it because the classifier reads the medium's
// published check, never a proposal-supplied artifact (DINV-3).
const CheckContext = "lane/review-verdict"

// ReportTemplateVersion is the one review-report shape this arm validates
// against. A report declaring any other version fails structural validity — it
// is never parsed leniently against a guessed shape (closed parse, DINV-8's
// posture applied to the report).
const ReportTemplateVersion = "review-report/v1"

// Status is the arm's published, consumed verdict on a review report.
type Status string

const (
	// Approving — a produced, attributed, fingerprint-matched, structurally
	// valid, approving report. Only this state satisfies the reviewer-verdict
	// precondition (PreconditionMet).
	Approving Status = "approving"
	// NonApproving — every other case (structural failure, a non-approving
	// verdict, a fingerprint mismatch, missing/underivable attribution, or no
	// published state at all). Fail-closed: the precondition is unmet.
	NonApproving Status = "non-approving"
)

// CheckState is the arm's published output — the datum the lane consumes. It is
// deliberately NOT the report itself: the lane reads this published state, so a
// verdict planted in the proposal's tree (which produces no published Approving
// state) can never satisfy the precondition (DINV-3).
type CheckState struct {
	Context string `json:"context"`
	Status  Status `json:"status"`
	Reason  string `json:"reason"`
}

// IsApproving reports whether the published state is approving.
func (s CheckState) IsApproving() bool { return s.Status == Approving }

// Attribution is the derived reviewer-flow attribution verdict over a report's
// authoring commits, mirroring the shipped attribution arms' trailer discipline
// (internal/argus/attribution): a produced, content-bound reviewer-flow stamp is
// Attested; its absence is Unattributed (a report the proposal author planted,
// not a reviewer flow produced); a malformed, replayed, or unreadable stamp is
// Underivable. Only Attested clears the attribution check; the other two fail
// closed (DINV-9's provenance bar applied to the report's producer).
type Attribution string

const (
	Attested     Attribution = "attested"
	Unattributed Attribution = "unattributed"
	Underivable  Attribution = "underivable"
)

// ReportInput is everything the arm validates for one evaluation — all read from
// the trusted mainline context by the caller, never taken as the verdict from
// parsing the proposal's own tree (DINV-9). EvaluatedFingerprint is the
// fingerprint of the content under evaluation, recomputed independently (see
// record.go's Fingerprint), and compared against the report's own recorded
// subject fingerprint. Attribution is the derived reviewer-flow verdict over the
// report's authoring commits.
type ReportInput struct {
	Report               []byte
	EvaluatedFingerprint string
	Attribution          Attribution
}

// Validate runs the review-report validation arm and publishes a check state.
// The checks run fail-closed and in a fixed order; the FIRST failure names the
// non-approving reason, and only a report clearing all four publishes Approving:
//
//  1. structural validity against the review-report template (a parseable
//     review-report/v1 header carrying Subject, Verdict, and a recorded
//     Subject-fingerprint);
//  2. an approving verdict (approve · approve-with-advisories);
//  3. the report's recorded subject fingerprint matching the evaluated content;
//  4. reviewer-flow attribution over the report's authoring commits (Attested).
//
// Every path returns CheckContext-named state, so the published context is
// stable whatever the outcome.
func Validate(in ReportInput) CheckState {
	nonApproving := func(reason string) CheckState {
		return CheckState{Context: CheckContext, Status: NonApproving, Reason: reason}
	}

	rep, err := parseReport(in.Report)
	if err != nil {
		return nonApproving("the review report is structurally invalid: " + err.Error())
	}
	if !approvingVerdicts[rep.verdict] {
		return nonApproving("the review report's verdict is not approving: " + quote(rep.verdict))
	}
	// Fail closed if we were handed no content to compare against — an absent
	// evaluated fingerprint can never be shown to MATCH, so it is not a pass.
	if strings.TrimSpace(in.EvaluatedFingerprint) == "" {
		return nonApproving("no evaluated-content fingerprint supplied; cannot confirm the report reviewed this content")
	}
	if rep.fingerprint != in.EvaluatedFingerprint {
		return nonApproving("the report's recorded subject fingerprint does not match the evaluated content: " +
			quote(rep.fingerprint) + " vs " + quote(in.EvaluatedFingerprint))
	}
	if in.Attribution != Attested {
		return nonApproving("the report's authoring commits do not carry reviewer-flow attribution (" +
			string(in.Attribution) + "); a planted verdict is not a produced one")
	}
	return CheckState{Context: CheckContext, Status: Approving,
		Reason: "the review report is valid, approving, fingerprint-matched, and reviewer-attributed"}
}

// PreconditionMet reports whether an AUTO class's reviewer-verdict precondition
// is satisfied, reading ONLY the arm's published check state — never the
// proposal's tree (DINV-3). A class that carries no reviewer verdict has nothing
// to require; a class that does is satisfied only by a published Approving
// state. This is the seam the acting adapter's act gate (T4) calls; it is the
// definition of the precondition, not the act.
func PreconditionMet(class policy.AutoClass, state CheckState) bool {
	if !class.RequiresReviewerVerdict {
		return true
	}
	return state.IsApproving()
}

// approvingVerdicts is the closed set of review-report verdict tokens that count
// as approving (prd-review-report-template Verdict history vocabulary). Anything
// else — request-changes, escalate, or an unknown token — is non-approving.
var approvingVerdicts = map[string]bool{
	"approve":                 true,
	"approve-with-advisories": true,
}

// report is the arm's parsed view of a review report: the header fields it
// gates on. Structural validity is defined by parseReport succeeding.
type report struct {
	verdict     string // the live Verdict header value, lowercased
	fingerprint string // the recorded Subject-fingerprint header value
}

// headerRowRe matches a review-report header table row `| **Field** | Value |`.
var headerRowRe = regexp.MustCompile(`^\|\s*\*\*(.+?)\*\*\s*\|\s*(.*?)\s*\|\s*$`)

// parseReport reads a review report's header table and returns the fields the
// arm gates on, or an error naming the first structural defect. It is a closed
// parse: a report that does not declare the pinned template version, that omits
// a required field, or that carries an empty required value is structurally
// invalid — never coerced into a lenient shape.
func parseReport(b []byte) (report, error) {
	fields := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if m := headerRowRe.FindStringSubmatch(line); m != nil {
			key := strings.TrimSpace(m[1])
			// The first occurrence wins so a later prose table cannot shadow the
			// header's Verdict (the header mirrors the live verdict by contract).
			if _, seen := fields[key]; !seen {
				fields[key] = strings.TrimSpace(m[2])
			}
		}
	}
	tv := fields["Template version"]
	if tv == "" {
		return report{}, errIf("missing the Template version header field")
	}
	if tv != ReportTemplateVersion {
		return report{}, errIf("Template version is " + quote(tv) + ", not " + quote(ReportTemplateVersion))
	}
	for _, req := range []string{"Subject", "Verdict", "Subject-fingerprint"} {
		if strings.TrimSpace(fields[req]) == "" {
			return report{}, errIf("missing the " + req + " header field")
		}
	}
	return report{
		verdict:     strings.ToLower(fields["Verdict"]),
		fingerprint: fields["Subject-fingerprint"],
	}, nil
}

// structuralError is the arm's structural-validity failure — a closed, named
// defect, distinct from a git/environment error, so callers can tell a bad
// report from an unreadable one.
type structuralError struct{ msg string }

func (e structuralError) Error() string { return e.msg }

func errIf(msg string) error { return structuralError{msg: msg} }

// quote wraps a value in single quotes for a readable, unambiguous reason.
func quote(s string) string { return "'" + s + "'" }
