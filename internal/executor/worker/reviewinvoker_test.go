package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func TestReviewResultRequiresVerifiedArtifact(t *testing.T) {
	for _, result := range []ReviewResult{
		{},
		{Status: "verified", ObligationStatus: "verified"},
		{Status: "failed", ObligationStatus: "pending", ArtifactRef: "github:7"},
	} {
		if !result.Failed() {
			t.Fatalf("artifact-incomplete result passed: %+v", result)
		}
	}
	publication := &ReviewPublicationTerminal{ReviewAttemptID: "attempt-1", ReviewObligationID: "obligation-1", ProposalRef: "github:o/r#7", Revision: "abc123", Status: "verified", ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_repo:7", AttemptProofRef: "proof:attempt-1"}
	implTerminal := &implreview.PublicationTerminal{ReviewProcessID: "attempt-1", ReviewObligationID: "obligation-1", PublicationAttemptID: "publication-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123", Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationVerified, ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_repo:7", AttemptProofRef: "proof:attempt-1", EVT13Ref: "sha256:evt13"}
	if (ReviewResult{Status: "verified", ObligationStatus: "verified", AttemptID: "attempt-1", ObligationID: "obligation-1", ArtifactRef: "sha256:artifact", ArtifactProofRef: "proof:attempt-1", NativeReceipt: "github-comment:R_repo:7", EVT13Ref: "sha256:evt13", EVT17Ref: "sha256:evt17", ImplReviewTerminal: implTerminal, Publication: publication}).Failed() {
		t.Fatal("authenticated verified artifact result failed")
	}
	if !(ReviewResult{Status: "verified", ObligationStatus: "verified", AttemptID: "attempt-1", ObligationID: "obligation-1", ArtifactRef: "github:7"}).Failed() {
		t.Fatal("verified result without authenticated artifact proof reported success")
	}
}

type spyReviewCapturer struct {
	called   int
	task     port.Task
	proposal port.Proposal
	result   ReviewCapture
}

func (s *spyReviewCapturer) Capture(_ context.Context, task port.Task, proposal port.Proposal) ReviewCapture {
	s.called++
	s.task, s.proposal = task, proposal
	return s.result
}

func (s *spyReviewCapturer) CompleteHandoff(context.Context, ReviewCapture) error { return nil }

func TestRun_ReviewCaptureOccursExactlyOnceAtGreen(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.proposal, host.checkState = port.Proposal{Number: 7}, port.CheckGreen
	capturer := &spyReviewCapturer{result: ReviewCapture{ProposalRef: "github:o/r#7", Revision: "abc123", ObligationID: "o-7", Harness: "claude"}}
	observed := false
	w := mustWorker(t, deps, WithReviewCapturer(capturer), WithGreenCapturedObserver(func(task port.Task, proposal port.Proposal, capture ReviewCapture) error {
		observed = task.ID == "T-42" && proposal.Number == 7 && capture.ObligationID == "o-7"
		return nil
	}))
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	if capturer.called != 1 || !observed {
		t.Fatalf("capture calls=%d observed=%v", capturer.called, observed)
	}
	got, ok := w.ReviewCapture()
	if !ok || got.Revision != "abc123" {
		t.Fatalf("capture = %+v, %v", got, ok)
	}
}

func TestRun_ReviewCaptureFailurePreservesImplementationTruthButSuppressesTerminalObserver(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckGreen
	capturer := &spyReviewCapturer{result: ReviewCapture{Err: errors.New("store unavailable")}}
	observed := false
	w := mustWorker(t, deps, WithReviewCapturer(capturer), WithGreenCapturedObserver(func(port.Task, port.Proposal, ReviewCapture) error { observed = true; return nil }))
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
	got, ok := w.ReviewCapture()
	if !ok || !got.Failed() || observed {
		t.Fatalf("capture=%+v present=%v observed=%v", got, ok, observed)
	}
}

func TestRun_ReviewCaptureOccursOnDrivenGreen(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.proposal, host.checkState = port.Proposal{Number: 12}, port.CheckRed
	driver := &fakeGreenDriver{result: GreenDriveResult{Green: true, Attempts: 1, Cap: 3}}
	capturer := &spyReviewCapturer{result: ReviewCapture{ProposalRef: "github:o/r#12", Revision: "def", ObligationID: "o-12", Harness: "claude"}}
	w := mustWorker(t, deps, WithGreenDriver(driver), WithReviewCapturer(capturer))
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded || driver.called != 1 || capturer.called != 1 {
		t.Fatalf("outcome=%v err=%v drive=%d capture=%d", outcome, err, driver.called, capturer.called)
	}
}

func TestRun_ReviewCaptureNotCalledOffGreen(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*fakeHost)
	}{
		{name: "red", configure: func(h *fakeHost) { h.checkState = port.CheckRed }},
		{name: "hollow", configure: func(h *fakeHost) { h.openErr = port.ErrNoProposal }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, _, _, _, _, host := greenDeps(t)
			tc.configure(host)
			capturer := &spyReviewCapturer{}
			w := mustWorker(t, deps, WithReviewCapturer(capturer))
			_, _ = w.Run(context.Background(), newTestTask())
			if capturer.called != 0 {
				t.Fatalf("capture calls=%d", capturer.called)
			}
		})
	}
}

func TestRun_NoReviewCaptureLeavesGreenPathUnchanged(t *testing.T) {
	deps, _, _, _, _, host := greenDeps(t)
	host.checkState = port.CheckGreen
	w := mustWorker(t, deps, WithReviewCapturer(nil))
	if w.reviewCapturer != nil {
		t.Fatal("nil capturer installed")
	}
	outcome, err := w.Run(context.Background(), newTestTask())
	if err != nil || outcome != OutcomeSucceeded {
		t.Fatalf("Run = %v, %v", outcome, err)
	}
}
