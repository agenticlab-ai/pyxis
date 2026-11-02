package verdict

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// reportOpts builds a review report artifact with overridable header fields, so
// each fixture flips exactly one axis (verdict, fingerprint, template version,
// or a missing field) off an otherwise-valid report.
type reportOpts struct {
	templateVersion string
	verdict         string
	fingerprint     string
	omit            string // a required field name to drop entirely
}

func reportDoc(o reportOpts) []byte {
	tv := o.templateVersion
	if tv == "" {
		tv = ReportTemplateVersion
	}
	rows := []struct{ k, v string }{
		{"In one line", "approved — no blocking findings"},
		{"Subject", "prd:demo/000"},
		{"Moment", "new-base"},
		{"Reviewer", "prd-review · v1"},
		{"Verdict", o.verdict},
		{"Subject-fingerprint", o.fingerprint},
		{"Template version", tv},
	}
	var b strings.Builder
	b.WriteString("# Review: demo — review report\n\n| Field | Value |\n|---|---|\n")
	for _, r := range rows {
		if r.k == o.omit {
			continue
		}
		b.WriteString("| **" + r.k + "** | " + r.v + " |\n")
	}
	b.WriteString("\n## Round 1\n\n(findings)\n")
	return []byte(b.String())
}

// The evaluated content and its independently-recomputed fingerprint — a real
// caller passes exactly this pair (content read from the mainline merge result,
// its fingerprint recomputed here).
var evalFiles = []File{{Path: "docs/demo.md", Content: []byte("the reviewed content\n")}}

func evalFingerprint() string { return Fingerprint(evalFiles) }

// TestValidate_ApprovesOnlyProducedReports pins acceptance (a)+(b): the arm
// publishes Approving ONLY for a structurally valid, approving, fingerprint-
// matched, reviewer-attributed report, and a NON-approving state naming the
// failure for every other case — including the planted-verdict case where the
// proposal author wrote an approving-looking report with no reviewer attribution.
func TestValidate_ApprovesOnlyProducedReports(t *testing.T) {
	fp := evalFingerprint()
	valid := reportOpts{verdict: "approve", fingerprint: fp}

	cases := []struct {
		name       string
		in         ReportInput
		wantStatus Status
		reasonHas  string
	}{
		{
			name:       "produced: valid, approving, matched, attested → approving",
			in:         ReportInput{Report: reportDoc(valid), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: Approving,
		},
		{
			name:       "approve-with-advisories is approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "approve-with-advisories", fingerprint: fp}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: Approving,
		},
		{
			// The planted case: an approving report the proposal author wrote into
			// its own tree, fingerprint even matched — but no reviewer flow produced
			// it, so its authoring commits carry no reviewer-flow attribution.
			name:       "planted: approving + matched but unattributed → non-approving",
			in:         ReportInput{Report: reportDoc(valid), EvaluatedFingerprint: fp, Attribution: Unattributed},
			wantStatus: NonApproving,
			reasonHas:  "reviewer-flow attribution",
		},
		{
			name:       "underivable (replayed/malformed) attribution → non-approving",
			in:         ReportInput{Report: reportDoc(valid), EvaluatedFingerprint: fp, Attribution: Underivable},
			wantStatus: NonApproving,
			reasonHas:  "reviewer-flow attribution",
		},
		{
			name:       "non-approving verdict (request-changes) → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "request-changes", fingerprint: fp}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "verdict is not approving",
		},
		{
			name:       "escalate verdict → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "escalate", fingerprint: fp}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "verdict is not approving",
		},
		{
			name:       "fingerprint mismatch (report reviewed other content) → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "approve", fingerprint: "sha256:deadbeef"}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "does not match the evaluated content",
		},
		{
			name:       "no evaluated fingerprint supplied → non-approving",
			in:         ReportInput{Report: reportDoc(valid), EvaluatedFingerprint: "", Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "no evaluated-content fingerprint",
		},
		{
			name:       "structurally invalid: wrong template version → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{templateVersion: "review-report/v2", verdict: "approve", fingerprint: fp}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "structurally invalid",
		},
		{
			name:       "structurally invalid: missing Subject-fingerprint field → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "approve", fingerprint: fp, omit: "Subject-fingerprint"}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "structurally invalid",
		},
		{
			name:       "structurally invalid: missing Verdict field → non-approving",
			in:         ReportInput{Report: reportDoc(reportOpts{verdict: "approve", fingerprint: fp, omit: "Verdict"}), EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "structurally invalid",
		},
		{
			name:       "empty report → non-approving",
			in:         ReportInput{Report: nil, EvaluatedFingerprint: fp, Attribution: Attested},
			wantStatus: NonApproving,
			reasonHas:  "structurally invalid",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate(tc.in)
			if got.Context != CheckContext {
				t.Errorf("context = %q, want %q", got.Context, CheckContext)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q (reason: %s)", got.Status, tc.wantStatus, got.Reason)
			}
			if tc.reasonHas != "" && !strings.Contains(got.Reason, tc.reasonHas) {
				t.Errorf("reason = %q, want it to contain %q", got.Reason, tc.reasonHas)
			}
			if got.Status == NonApproving && strings.TrimSpace(got.Reason) == "" {
				t.Errorf("a non-approving state must name why; reason was empty")
			}
		})
	}
}

// TestPreconditionMet_ReadsOnlyPublishedState pins acceptance (a)'s core: the
// reviewer-verdict precondition is satisfied only by the arm's published
// Approving check state, never by anything in the proposal's tree. A class that
// requires a reviewer verdict is unmet under a non-approving or empty state; a
// class that requires none is always met.
func TestPreconditionMet_ReadsOnlyPublishedState(t *testing.T) {
	requiring := policy.AutoClass{Name: "drive-doc", RequiresReviewerVerdict: true}
	free := policy.AutoClass{Name: "fleet-task", RequiresReviewerVerdict: false}

	approving := CheckState{Context: CheckContext, Status: Approving}
	nonApproving := CheckState{Context: CheckContext, Status: NonApproving}
	// The state a proposal with a planted (but never arm-published) verdict is
	// evaluated under: no published Approving state exists.
	var unpublished CheckState

	cases := []struct {
		name  string
		class policy.AutoClass
		state CheckState
		want  bool
	}{
		{"requiring class + published approving → met", requiring, approving, true},
		{"requiring class + non-approving → unmet", requiring, nonApproving, false},
		{"requiring class + no published state (planted-only) → unmet", requiring, unpublished, false},
		{"free class + no state → met (nothing required)", free, unpublished, true},
		{"free class + non-approving → met (nothing required)", free, nonApproving, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PreconditionMet(tc.class, tc.state); got != tc.want {
				t.Errorf("PreconditionMet = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestFoldAttribution pins the fail-closed fold over a report's authoring
// commits: attested only when every commit carries the reviewer-flow stamp;
// worst wins otherwise; empty is unattributed.
func TestFoldAttribution(t *testing.T) {
	cases := []struct {
		name string
		in   []Attribution
		want Attribution
	}{
		{"empty → unattributed", nil, Unattributed},
		{"all attested → attested", []Attribution{Attested, Attested}, Attested},
		{"one unattributed → unattributed", []Attribution{Attested, Unattributed}, Unattributed},
		{"one underivable outranks unattributed → underivable", []Attribution{Attested, Unattributed, Underivable}, Underivable},
		{"single underivable → underivable", []Attribution{Underivable}, Underivable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FoldAttribution(tc.in); got != tc.want {
				t.Errorf("FoldAttribution(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFingerprint_DeterministicAndOrderInsensitive pins the subject-fingerprint
// contract: deterministic, path-order-insensitive, whitespace-normalized, and
// content-sensitive — so a report's recorded fingerprint matches iff it reviewed
// this exact content.
func TestFingerprint_DeterministicAndOrderInsensitive(t *testing.T) {
	a := []File{{Path: "a.md", Content: []byte("x\n")}, {Path: "b.md", Content: []byte("y\n")}}
	reordered := []File{{Path: "b.md", Content: []byte("y  \n")}, {Path: "a.md", Content: []byte("x\n")}} // trailing ws normalized away
	if Fingerprint(a) != Fingerprint(reordered) {
		t.Errorf("fingerprint not order/whitespace-stable: %s vs %s", Fingerprint(a), Fingerprint(reordered))
	}
	changed := []File{{Path: "a.md", Content: []byte("x\n")}, {Path: "b.md", Content: []byte("Y\n")}}
	if Fingerprint(a) == Fingerprint(changed) {
		t.Errorf("fingerprint insensitive to content change")
	}
	if !strings.HasPrefix(Fingerprint(a), "sha256:") {
		t.Errorf("fingerprint = %q, want sha256: prefix", Fingerprint(a))
	}
}
