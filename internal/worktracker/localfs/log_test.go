package localfs

import (
	"context"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// TestCommentAppendsLegiblyAndRoundTrips: a comment writes one append-only log
// entry that Get reads back, in time order; a second comment appends after it
// (FR-5, DINV-5).
func TestCommentAppendsLegiblyAndRoundTrips(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "talk to me"})

	if _, defects := s.Comment(ctx, w.ID, "alice", "first thought"); len(defects) > 0 {
		t.Fatalf("comment 1: %v", defects)
	}
	if _, defects := s.Comment(ctx, w.ID, "bob", "second thought"); len(defects) > 0 {
		t.Fatalf("comment 2: %v", defects)
	}

	on := raw(t, s, w.ID)
	if !strings.Contains(on, "- comment · alice · ") || !strings.Contains(on, "first thought") {
		t.Fatalf("comment not written legibly:\n%s", on)
	}

	got, defects := s.Get(ctx, w.ID)
	if len(defects) > 0 {
		t.Fatalf("get: %v", defects)
	}
	if len(got.Log) != 2 {
		t.Fatalf("want 2 comments, got %d: %+v", len(got.Log), got.Log)
	}
	if got.Log[0].Kind != model.LogComment || got.Log[0].Author != "alice" || got.Log[0].Payload != "first thought" {
		t.Fatalf("comment 0 wrong: %+v", got.Log[0])
	}
	if got.Log[1].Payload != "second thought" || !got.Log[1].At.After(got.Log[0].At) {
		t.Fatalf("comment 1 out of order or wrong: %+v", got.Log[1])
	}
}

// TestMultiLineCommentRoundTrips: a comment carrying newlines and a backslash
// survives the single-line log grammar via payload escaping (§3.1).
func TestMultiLineCommentRoundTrips(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "prose"})

	body := "line one\nline two\nwith a \\ backslash and a · middot"
	if _, defects := s.Comment(ctx, w.ID, "author", body); len(defects) > 0 {
		t.Fatalf("comment: %v", defects)
	}
	// On disk the entry stays on one physical line (no raw newline in the log).
	on := raw(t, s, w.ID)
	logRegion := on[strings.Index(on, "## Log"):]
	if strings.Count(logRegion, "\n- ") != 0 && strings.Count(logRegion, "- comment") != 1 {
		t.Fatalf("multi-line comment broke the single-line log grammar:\n%s", logRegion)
	}
	got, defects := s.Get(ctx, w.ID)
	if len(defects) > 0 {
		t.Fatalf("get: %v", defects)
	}
	if len(got.Log) != 1 || got.Log[0].Payload != body {
		t.Fatalf("multi-line payload did not round-trip: %q", got.Log[0].Payload)
	}
}

// TestRecordIdempotentRepeatLeavesFileByteIdentical: appending the same latest
// record again writes nothing at all — the file and its updated stamp are frozen
// (FR-9, AP-22).
func TestRecordIdempotentRepeatLeavesFileByteIdentical(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "decide"})

	if _, defects := s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "t", Payload: "plan"}); len(defects) > 0 {
		t.Fatalf("record: %v", defects)
	}
	before := raw(t, s, w.ID)

	// An identical repeat of the current latest writes nothing.
	if _, defects := s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "t", Payload: "plan"}); len(defects) > 0 {
		t.Fatalf("record repeat: %v", defects)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("identical repeat mutated the file:\nbefore=%q\nafter=%q", before, after)
	}

	// A different author with the same payload is still the same judgment — no-op.
	if _, defects := s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "someone-else", Payload: "plan"}); len(defects) > 0 {
		t.Fatalf("record repeat other author: %v", defects)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("same-payload repeat mutated the file")
	}
}

// TestRecordSupersedesLatestWins: a different payload appends a fresh record and
// becomes the authoritative one for the namespace (DINV-5), on disk and via read.
func TestRecordSupersedesLatestWins(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "decide"})

	s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "t", Payload: "plan"})
	s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.NamespaceTriageDisposition, Author: "t", Payload: "operator"})

	on := raw(t, s, w.ID)
	if strings.Count(on, "· triage-disposition · ") != 2 {
		t.Fatalf("want two triage-disposition records on disk, got:\n%s", on)
	}
	rec, ok, defects := s.LatestRecord(ctx, w.ID, model.NamespaceTriageDisposition)
	if len(defects) > 0 || !ok {
		t.Fatalf("latest: ok=%v defects=%v", ok, defects)
	}
	if rec.Payload != "operator" {
		t.Fatalf("latest-wins failed: %q", rec.Payload)
	}
}

// TestOutOfSetNamespaceNoWrite: an out-of-set namespace is refused before any
// write, on both the write and read side (DINV-11, fail closed).
func TestOutOfSetNamespaceNoWrite(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	ctx := context.Background()
	w, _ := s.Create(ctx, model.CreateSpec{Title: "guarded"})
	before := raw(t, s, w.ID)

	_, defects := s.Record(ctx, w.ID, model.DecisionRecord{Namespace: model.Namespace("bogus"), Author: "t", Payload: "x"})
	if len(defects) == 0 || defects[0].Verb != "record" || !strings.Contains(defects[0].Reason, "bogus") {
		t.Fatalf("out-of-set namespace defect wrong: %v", defects)
	}
	if after := raw(t, s, w.ID); after != before {
		t.Fatalf("refused record still wrote the file")
	}
	if _, _, defects := s.LatestRecord(ctx, w.ID, model.Namespace("bogus")); len(defects) == 0 {
		t.Fatalf("LatestRecord accepted an out-of-set namespace")
	}
}

// TestCommentMissingItem: commenting on a missing item is a defect naming the
// comment verb, and nothing is written.
func TestCommentMissingItem(t *testing.T) {
	s := New(t.TempDir(), advancingClock())
	_, defects := s.Comment(context.Background(), 42, "a", "hi")
	if len(defects) != 1 || defects[0].Verb != "comment" || !strings.Contains(defects[0].Reason, "no work item") {
		t.Fatalf("missing-item comment defect wrong: %v", defects)
	}
}
