package port_test

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestLandingString(t *testing.T) {
	tests := []struct {
		l    port.Landing
		want string
	}{
		{port.LandingUnknown, "unknown"},
		{port.LandingOpen, "open"},
		{port.LandingLanded, "landed"},
		{port.LandingClosedUnlanded, "closed-unlanded"},
		{port.Landing(99), "Landing(99)"},
	}
	for _, tt := range tests {
		if got := tt.l.String(); got != tt.want {
			t.Errorf("Landing(%d).String() = %q, want %q", int(tt.l), got, tt.want)
		}
	}
}

// LandingUnknown is the zero value so an unset Landing reads as unknown rather
// than a false claim of open or landed (fail-closed, mirroring CheckPending's
// role — DINV-1).
func TestLandingUnknownIsZeroValue(t *testing.T) {
	var zero port.Landing
	if zero != port.LandingUnknown {
		t.Fatalf("zero Landing = %v, want unknown (fail-closed)", zero)
	}
	if zero == port.LandingOpen || zero == port.LandingLanded || zero == port.LandingClosedUnlanded {
		t.Fatal("the zero Landing must never read as open, landed, or closed-unlanded")
	}
}

func TestCommentSurfaceString(t *testing.T) {
	tests := []struct {
		s    port.CommentSurface
		want string
	}{
		{port.SurfaceDiscussion, "discussion"},
		{port.SurfaceInlineReview, "inline-review"},
		{port.SurfaceReviewSummary, "review-summary"},
		{port.CommentSurface(99), "CommentSurface(99)"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("CommentSurface(%d).String() = %q, want %q", int(tt.s), got, tt.want)
		}
	}
}

func TestUnknownReasonString(t *testing.T) {
	tests := []struct {
		r    port.UnknownReason
		want string
	}{
		{port.UnknownRateLimited, "rate-limited"},
		{port.UnknownOffline, "offline"},
		{port.UnknownParseDefect, "parse-defect"},
		{port.UnknownTimeout, "timeout"},
		{port.UnknownReason(0), "UnknownReason(0)"},
	}
	for _, tt := range tests {
		if got := tt.r.String(); got != tt.want {
			t.Errorf("UnknownReason(%d).String() = %q, want %q", int(tt.r), got, tt.want)
		}
	}
}

// The zero UnknownReason is reserved: it must never render as one of the four
// named reasons, so a forgotten assignment can never be mistaken for a real
// typed reason (DINV-1).
func TestUnknownReasonZeroValueIsNotNamed(t *testing.T) {
	var zero port.UnknownReason
	switch zero {
	case port.UnknownRateLimited, port.UnknownOffline, port.UnknownParseDefect, port.UnknownTimeout:
		t.Fatalf("zero UnknownReason must not equal a named reason, got %v", zero)
	}
}

func TestProposalRefString(t *testing.T) {
	tests := []struct {
		ref  port.ProposalRef
		want string
	}{
		{port.ProposalRef{Repo: "acme/widget", Number: 12}, "acme/widget#12"},
		{port.ProposalRef{Number: 7}, "#7"},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("ProposalRef.String() = %q, want %q", got, tt.want)
		}
	}
}
