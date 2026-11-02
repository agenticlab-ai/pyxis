package watch

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestBuildCombinedReportRendersKnownAndUnknownReviewState(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	provider := &mutableReviewProvider{rows: []ReviewObligationState{pendingReview("o-1")}}
	reviews, sink := combinedFixture(t, provider)
	sink.clock = func() time.Time { return now }
	sink.Append(Event{Kind: EventCheckRan})
	report, err := BuildCombinedReport(t.Context(), constantSource(Plan{}), port.NewFakeStateReader(), reviews, "feature/001", sink, DefaultThresholds(), time.Minute, now)
	if err != nil || len(report.Reviews) != 1 || report.ReviewUnknown != nil || !report.Degraded() {
		t.Fatalf("known report=%+v err=%v", report, err)
	}

	provider.err = context.DeadlineExceeded
	report, err = BuildCombinedReport(t.Context(), constantSource(Plan{}), port.NewFakeStateReader(), reviews, "feature/001", sink, DefaultThresholds(), time.Minute, now)
	if err != nil || report.ReviewUnknown == nil || report.ReviewUnknown.Kind != "unavailable" {
		t.Fatalf("unknown report=%+v err=%v", report, err)
	}

	if _, err := BuildCombinedReport(t.Context(), func(context.Context) (Plan, error) { return Plan{}, context.Canceled }, port.NewFakeStateReader(), reviews, "feature/001", sink, DefaultThresholds(), time.Minute, now); err == nil {
		t.Fatal("combined report swallowed a plan-source failure")
	}
	if _, err := BuildCombinedReport(t.Context(), constantSource(Plan{}), port.NewFakeStateReader(), ReviewStateReader{}, "feature/001", sink, DefaultThresholds(), time.Minute, now); err == nil {
		t.Fatal("combined report accepted an incomplete review reader")
	}
}

func TestWriteReportCoversEveryReviewEligibility(t *testing.T) {
	eligible := pendingReview("eligible")
	eligible.AttemptCount = 1
	exhausted := pendingReview("exhausted")
	exhausted.LastFailureClass = "infrastructure"
	rows := []ReviewWakeRow{
		{ReviewObligationState: eligible, Eligibility: ReviewEligible, AgeS: 61, RecoveryEpoch: "epoch-1", RecoveryCount: 1, RecoveryBound: 3},
		{ReviewObligationState: exhausted, Eligibility: ReviewExhausted, AgeS: 3600, RecoveryEpoch: "epoch-2", RecoveryCount: 3, RecoveryBound: 3},
		{ReviewObligationState: pendingReview("handoff"), Eligibility: ReviewHandoffRepair},
		{ReviewObligationState: pendingReview("unobserved"), Eligibility: ReviewUnobserved},
	}
	var out strings.Builder
	WriteReport(&out, Report{Reviews: rows, Liveness: Liveness{Defect: "cannot-verify-liveness"}})
	for _, want := range []string{"OBLIGATION", "eligible", "exhausted", "handoff repair required", "unobserved", "1m", "1/3 (epoch-1)", "infrastructure"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("render omitted %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	WriteReport(&out, Report{ReviewUnknown: &ReviewUnknown{Source: "store", Kind: "permission_denied", Reason: "denied"}})
	if !strings.Contains(out.String(), "review state unknown") || !strings.Contains(out.String(), "permission_denied") {
		t.Fatalf("unknown render=%q", out.String())
	}
}

func TestRepositoryRecoveryLedgerUsesGitCommonDirectory(t *testing.T) {
	repo := t.TempDir()
	cmd := exec.Command("git", "-C", repo, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	ledger, err := NewRepositoryRecoveryLedger(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repo, ".git", "pyxis", "review-recovery.jsonl")
	if filepath.Clean(ledger.path) != filepath.Clean(want) {
		t.Fatalf("ledger path=%q want=%q", ledger.path, want)
	}
	if _, err := NewRepositoryRecoveryLedger(t.Context(), ""); err == nil {
		t.Fatal("empty repository path was accepted")
	}
	if _, err := NewRepositoryRecoveryLedger(t.Context(), t.TempDir()); err == nil {
		t.Fatal("non-repository path was accepted")
	}
}

func TestRoundIDsAndSettledChecksHaveClosedSemantics(t *testing.T) {
	first, err := NewRoundID()
	if err != nil || !strings.HasPrefix(first, "drive-round-") {
		t.Fatalf("first round id=%q err=%v", first, err)
	}
	second, err := randomReviewID("review")
	if err != nil || !strings.HasPrefix(second, "review-") || second == first {
		t.Fatalf("second id=%q err=%v", second, err)
	}
	if !checksSettled(port.ProposalState{Checks: []port.CheckContext{{Name: "test", State: port.CheckGreen}}}) {
		t.Fatal("green required checks were not settled")
	}
	if checksSettled(port.ProposalState{RequiredMissing: []string{"test"}}) {
		t.Fatal("missing required check was settled")
	}
	if checksSettled(port.ProposalState{Checks: []port.CheckContext{{Name: "test", State: port.CheckPending}}}) {
		t.Fatal("pending required check was settled")
	}
}
