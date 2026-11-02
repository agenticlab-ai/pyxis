package worker

import "testing"

func TestReviewStateAccessorsAndNilObservers(t *testing.T) {
	w := &Worker{}
	WithReviewCapturer(nil)(w)
	WithGreenVerifiedObserver(nil)(w)
	WithGreenCapturedObserver(nil)(w)
	if got, ok := w.ReviewCapture(); ok || got != (ReviewCapture{}) {
		t.Fatalf("ReviewCapture() = %+v, %v", got, ok)
	}
	if got, ok := w.Proposal(); ok || got.Number != 0 || got.Branch != "" {
		t.Fatalf("Proposal() = %+v, %v", got, ok)
	}
}
