package model

import (
	"testing"
	"time"
)

func TestValidNamespace(t *testing.T) {
	for _, ns := range Namespaces {
		if !ValidNamespace(ns) {
			t.Errorf("seed namespace %q reported out of set", ns)
		}
	}
	if ValidNamespace(Namespace("bogus")) {
		t.Errorf("out-of-set namespace accepted")
	}
	if ValidNamespace(Namespace("")) {
		t.Errorf("empty namespace accepted")
	}
}

// TestLatestRecordResolvesLatestWins: the last decision-record appended under a
// namespace is the authoritative one, and namespaces do not cross (DINV-5).
func TestLatestRecordResolvesLatestWins(t *testing.T) {
	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	w := WorkItem{Log: []LogEntry{
		{Kind: LogComment, Author: "a", At: at, Payload: "chatter"},
		{Kind: LogDecisionRecord, Author: "t", At: at.Add(time.Second), Namespace: string(NamespaceTriageDisposition), Payload: "plan"},
		{Kind: LogDecisionRecord, Author: "p", At: at.Add(2 * time.Second), Namespace: string(NamespaceCharter), Payload: "ship"},
		{Kind: LogDecisionRecord, Author: "t", At: at.Add(3 * time.Second), Namespace: string(NamespaceTriageDisposition), Payload: "operator"},
	}}

	rec, ok := w.LatestRecord(NamespaceTriageDisposition)
	if !ok || rec.Payload != "operator" {
		t.Fatalf("triage-disposition latest = %+v (ok=%v), want payload %q", rec, ok, "operator")
	}
	if rec.Author != "t" || !rec.At.Equal(at.Add(3*time.Second)) {
		t.Fatalf("latest record metadata wrong: %+v", rec)
	}
	charter, ok := w.LatestRecord(NamespaceCharter)
	if !ok || charter.Payload != "ship" {
		t.Fatalf("charter latest = %+v, want %q", charter, "ship")
	}
	if _, ok := w.LatestRecord(NamespaceFoundation); ok {
		t.Fatalf("foundation namespace reported a record where none exists")
	}
}

// TestLatestRecordIgnoresComments: a comment whose payload looks like a record
// is never resolved as one — only decision-record entries count (DINV-5).
func TestLatestRecordIgnoresComments(t *testing.T) {
	w := WorkItem{Log: []LogEntry{
		{Kind: LogComment, Author: "a", Namespace: string(NamespaceCharter), Payload: "not a record"},
	}}
	if _, ok := w.LatestRecord(NamespaceCharter); ok {
		t.Fatalf("a comment was resolved as a decision-record")
	}
}
