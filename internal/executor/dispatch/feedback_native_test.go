package dispatch

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/tasksession"
)

func TestSweepNativeFeedbackSameTimestampSurvivesRestart(t *testing.T) {
	tr, ls, ws, rt, hs := greenFakes(t)
	reader := port.NewFakeStateReader()
	repository := "forgejo:http://localhost:3000:o/r"
	ref := port.ProposalRef{Repo: repository, Number: 77}
	task := port.Task{ID: "T1", Number: 1, SourceRef: "http://localhost:3000/o/r/issues/1", SourceKey: repository, Title: "task", Brief: "task"}
	tr.setFrontier(task)
	path := filepath.Join(t.TempDir(), "sessions.jsonl")
	newLedger := func() *tasksession.Continuity {
		t.Helper()
		store, err := tasksession.NewFileStore(path)
		if err != nil {
			t.Fatal(err)
		}
		ledger, err := tasksession.New(store, repository)
		if err != nil {
			t.Fatal(err)
		}
		return ledger
	}
	ledger := newLedger()
	d, err := New(Deps{Tracker: tr, Lease: ls, Space: ws, Runtime: rt, Host: hs}, Config{Width: 1}, WithProposalProbe(openProbe{open: map[string]int{"T1": 77}}), WithFeedbackDetection(reader, repository, "engine-bot", ledger, "allowlisted"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	first := port.Comment{Author: "owner", AuthorAssociation: "OWNER", NativeID: "discussion:1", CreatedAt: at, Surface: port.SurfaceDiscussion, Body: "first request"}
	second := first
	second.NativeID, second.Body = "inline:2", "second request in the same second"
	forged := port.Comment{Author: "allowlisted", AuthorAssociation: "OWNER", Untrusted: true, NativeID: "discussion:99", CreatedAt: at.Add(time.Second), Body: "imported author must never execute"}
	reader.Set(ref, stateWith(first, forged))
	result, err := d.Sweep(t.Context())
	if err != nil || result.FeedbackRounds != 1 || rt.launchCount() != 1 {
		t.Fatalf("first admission=%+v launches=%d %v", result, rt.launchCount(), err)
	}
	if fb := rt.specsSnapshot()[0].Feedback; fb == nil || len(fb.Comments) != 1 || fb.Comments[0].NativeID != first.NativeID {
		t.Fatalf("untrusted comment reached runtime: %+v", fb)
	}
	// Restart the ledger reader. A later API read can reveal a different native
	// comment with exactly the timestamp already committed by the first round.
	ledger = newLedger()
	WithFeedbackDetection(reader, repository, "engine-bot", ledger, "allowlisted")(d)
	reader.Set(ref, stateWith(first, second, forged))
	result, err = d.Sweep(t.Context())
	if err != nil || result.FeedbackRounds != 1 || rt.launchCount() != 2 {
		t.Fatalf("same-time admission=%+v launches=%d %v", result, rt.launchCount(), err)
	}
	if fb := rt.specsSnapshot()[1].Feedback; fb == nil || fb.Round != 2 || len(fb.Comments) != 1 || fb.Comments[0].NativeID != second.NativeID {
		t.Fatalf("duplicate or missing native feedback: %+v", fb)
	}
	ledger = newLedger()
	WithFeedbackDetection(reader, repository, "engine-bot", ledger, "allowlisted")(d)
	result, err = d.Sweep(t.Context())
	if err != nil || result.AwaitingProposal != 1 || result.FeedbackRounds != 0 || rt.launchCount() != 2 {
		t.Fatalf("native replay/untrusted comment woke a round: %+v launches=%d %v", result, rt.launchCount(), err)
	}
	seen, err := ledger.FeedbackSeen(t.Context(), task)
	if err != nil || len(seen) != 2 || !seen[first.NativeID] || !seen[second.NativeID] || seen[forged.NativeID] {
		t.Fatalf("durable admitted identities=%v %v", seen, err)
	}
}

type failingNativeFeedbackLedger struct {
	*fakeLedger
	seenErr, writeErr error
}

func (l *failingNativeFeedbackLedger) FeedbackSeen(context.Context, port.Task) (map[string]bool, error) {
	return map[string]bool{}, l.seenErr
}
func (l *failingNativeFeedbackLedger) RecordFeedbackComments(context.Context, port.Task, time.Time, []port.Comment) error {
	return l.writeErr
}

func TestNativeFeedbackCursorFailuresWithhold(t *testing.T) {
	reader := port.NewFakeStateReader()
	reader.Set(port.ProposalRef{Repo: "o/r", Number: 7}, stateWith(port.Comment{Author: "owner", AuthorAssociation: "OWNER", CreatedAt: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC), NativeID: "discussion:1", Body: "request"}))
	ledger := &failingNativeFeedbackLedger{fakeLedger: newFakeLedger(), seenErr: errors.New("cannot read identities")}
	d := &Dispatcher{}
	WithFeedbackDetection(reader, "o/r", "engine", ledger)(d)
	if _, admitted, err := d.admitFeedback(t.Context(), task("T1", 1), 7); err == nil || admitted {
		t.Fatal("unreadable native identity cursor admitted feedback")
	}
	ledger.seenErr, ledger.writeErr = nil, errors.New("cannot record identities")
	if _, admitted, err := d.admitFeedback(t.Context(), task("T1", 1), 7); err == nil || admitted {
		t.Fatal("unpersisted native identity cursor admitted feedback")
	}
}
