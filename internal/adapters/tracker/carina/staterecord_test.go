package carinatracker

import (
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestLanding_Open(t *testing.T) {
	w := model.WorkItem{State: model.StateOpen}
	l, rev := landing(w)
	if l != port.LandingOpen || rev != "" {
		t.Fatalf("landing(open) = (%v, %q), want (open, \"\")", l, rev)
	}
}

func TestLanding_ClosedCompletedIsLanded(t *testing.T) {
	w := model.WorkItem{
		State:       model.StateClosed,
		CloseReason: model.CloseCompleted,
		Updated:     at("2026-07-20T10:00:00Z"),
		Log: []model.LogEntry{
			{Kind: model.LogTransition, Author: "a", At: at("2026-07-20T09:59:00Z")},
		},
	}
	l, rev := landing(w)
	if l != port.LandingLanded {
		t.Fatalf("landing(closed/completed) = %v, want landed", l)
	}
	if want := "2026-07-20T09:59:00Z"; rev != want {
		t.Fatalf("LandedRevision = %q, want the transition's timestamp %q", rev, want)
	}
}

func TestLanding_ClosedRejectedIsClosedUnlanded(t *testing.T) {
	w := model.WorkItem{State: model.StateClosed, CloseReason: model.CloseRejected}
	l, rev := landing(w)
	if l != port.LandingClosedUnlanded || rev != "" {
		t.Fatalf("landing(closed/rejected) = (%v, %q), want (closed-unlanded, \"\")", l, rev)
	}
}

// A hand-edited file can close an item with no reason at all (close_reason
// is not in localfs's requiredKeys). The safe-direction default is
// closed-unlanded, never a landing claim with no positive evidence for it.
func TestLanding_ClosedWithNoReasonIsClosedUnlanded(t *testing.T) {
	w := model.WorkItem{State: model.StateClosed, CloseReason: model.CloseReasonNone}
	l, rev := landing(w)
	if l != port.LandingClosedUnlanded || rev != "" {
		t.Fatalf("landing(closed/none) = (%v, %q), want (closed-unlanded, \"\")", l, rev)
	}
}

// landedRevision falls back to the item's own Updated stamp when no
// transition entry was ever recorded (e.g. a hand-authored file that sets
// state/close_reason directly).
func TestLandedRevision_FallsBackToUpdatedWithNoTransition(t *testing.T) {
	w := model.WorkItem{
		State:       model.StateClosed,
		CloseReason: model.CloseCompleted,
		Updated:     at("2026-07-20T10:00:00Z"),
	}
	_, rev := landing(w)
	if want := "2026-07-20T10:00:00Z"; rev != want {
		t.Fatalf("LandedRevision fallback = %q, want %q", rev, want)
	}
}

// A reopen-then-reclose must be answered by the *last* closing transition,
// never an earlier, stale one.
func TestLandedRevision_PrefersLastTransition(t *testing.T) {
	w := model.WorkItem{
		State:       model.StateClosed,
		CloseReason: model.CloseCompleted,
		Log: []model.LogEntry{
			{Kind: model.LogTransition, At: at("2026-07-01T00:00:00Z")}, // first close
			{Kind: model.LogTransition, At: at("2026-07-10T00:00:00Z")}, // reopen
			{Kind: model.LogComment, At: at("2026-07-15T00:00:00Z")},    // unrelated comment in between
			{Kind: model.LogTransition, At: at("2026-07-20T00:00:00Z")}, // final close
		},
	}
	_, rev := landing(w)
	if want := "2026-07-20T00:00:00Z"; rev != want {
		t.Fatalf("LandedRevision = %q, want the last transition %q", rev, want)
	}
}

func TestChecksFor(t *testing.T) {
	tests := []struct {
		name    string
		signals []string
		want    []port.CheckContext
	}{
		{"no signals", nil, nil},
		{
			"pending signal",
			[]string{"verify-pending"},
			[]port.CheckContext{{Name: "verify-pending", State: port.CheckPending}},
		},
		{
			"two red signals sorted by name",
			[]string{"merge-conflict", "worker-failed"},
			[]port.CheckContext{
				{Name: "merge-conflict", State: port.CheckRed},
				{Name: "worker-failed", State: port.CheckRed},
			},
		},
		{
			"non-check signals excluded",
			[]string{"hold", "agent", "kind:bug"},
			nil,
		},
		{
			"mixed: only the check-relevant signal surfaces",
			[]string{"hold", "verify-pending", "kind:bug"},
			[]port.CheckContext{{Name: "verify-pending", State: port.CheckPending}},
		},
		{
			"every named status signal",
			[]string{"worker-failed", "merge-conflict", "verify-pending", "feedback-stalled", "empty-proposal"},
			[]port.CheckContext{
				{Name: "empty-proposal", State: port.CheckRed},
				{Name: "feedback-stalled", State: port.CheckPending},
				{Name: "merge-conflict", State: port.CheckRed},
				{Name: "verify-pending", State: port.CheckPending},
				{Name: "worker-failed", State: port.CheckRed},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checksFor(model.WorkItem{Signals: tt.signals})
			if !equalChecks(got, tt.want) {
				t.Fatalf("checksFor(%v) = %+v, want %+v", tt.signals, got, tt.want)
			}
		})
	}
}

func equalChecks(a, b []port.CheckContext) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCommentsFor_OnlyLogCommentEntries(t *testing.T) {
	w := model.WorkItem{
		Log: []model.LogEntry{
			{Kind: model.LogComment, Author: "alice", At: at("2026-07-01T00:00:00Z"), Payload: "first note"},
			{Kind: model.LogDecisionRecord, Author: "bob", At: at("2026-07-02T00:00:00Z"), Namespace: "charter", Payload: "some judgment"},
			{Kind: model.LogTransition, Author: "carina", At: at("2026-07-03T00:00:00Z"), Payload: "open → closed"},
			{Kind: model.LogComment, Author: "dana", At: at("2026-07-04T00:00:00Z"), Payload: "second note"},
		},
	}
	got := commentsFor(w)
	want := []port.Comment{
		{Author: "alice", CreatedAt: at("2026-07-01T00:00:00Z"), Surface: port.SurfaceDiscussion, Body: "first note"},
		{Author: "dana", CreatedAt: at("2026-07-04T00:00:00Z"), Surface: port.SurfaceDiscussion, Body: "second note"},
	}
	if len(got) != len(want) {
		t.Fatalf("commentsFor = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commentsFor[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDeriveState_Landed(t *testing.T) {
	w := model.WorkItem{
		State:       model.StateClosed,
		CloseReason: model.CloseCompleted,
		Updated:     at("2026-07-20T10:00:00Z"),
	}
	st := deriveState(w)
	if st.Landing != port.LandingLanded || st.LandedRevision == "" {
		t.Fatalf("deriveState(landed) = %+v", st)
	}
	if st.Unknown != nil {
		t.Fatalf("deriveState(landed) carries Unknown: %+v", st)
	}
}

func TestDeriveState_OpenWithChecksAndComments(t *testing.T) {
	w := model.WorkItem{
		State:   model.StateOpen,
		Signals: []string{"verify-pending"},
		Log: []model.LogEntry{
			{Kind: model.LogComment, Author: "alice", At: at("2026-07-01T00:00:00Z"), Payload: "note"},
		},
	}
	st := deriveState(w)
	if st.Landing != port.LandingOpen {
		t.Fatalf("deriveState(open).Landing = %v, want open", st.Landing)
	}
	if len(st.Checks) != 1 || st.Checks[0].Name != "verify-pending" || st.Checks[0].State != port.CheckPending {
		t.Fatalf("deriveState(open).Checks = %+v", st.Checks)
	}
	if len(st.Comments) != 1 || st.Comments[0].Body != "note" {
		t.Fatalf("deriveState(open).Comments = %+v", st.Comments)
	}
}
