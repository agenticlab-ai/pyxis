package lock

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestWriteRecordsResolvesRelationsAndKeepsLiteralBodies(t *testing.T) {
	root := t.TempDir()
	prd := []byte("# PRD\r\n")
	tech := []byte("# Tech spec\n")
	result := compile.Result{Feature: "widget", Records: []compile.Record{
		{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Choose a queue."}, Links: []compile.Link{{Type: record.Serves, Target: "prd/widget#FR-1"}}},
		{Draft: reconcile.Draft{Kind: record.Answer, Subject: "prd/widget", Statement: "Operator ruled."}, Links: []compile.Link{{Type: record.Answers, Target: "prd/widget#P-1"}}},
		{Draft: reconcile.Draft{Kind: record.Question, Subject: "prd/widget", Family: "P-1", Statement: "Who owns the queue?"}},
		{Draft: reconcile.Draft{Kind: record.Promise, Subject: "prd/widget", Family: "FR-1", Statement: "The queue shall open."}},
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", Statement: "Draft PRD", LiteralBody: true, Body: prd}},
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "design/widget", Family: "design", Statement: "Draft tech spec", LiteralBody: true, Body: tech}},
	}}
	documents, paths, err := writeRecords(root, "agent:test", result, readiness.Sources{})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 || documents["prd"] == "" || documents["tech-spec"] == "" {
		t.Fatalf("paths=%v documents=%v", paths, documents)
	}
	for key, want := range map[string][]byte{"prd": prd, "tech-spec": tech} {
		got, err := os.ReadFile(filepath.Join(root, documents[key]))
		if err != nil || string(got) != string(want) {
			t.Fatalf("%s literal body = %q, %v", key, got, err)
		}
	}
	set, err := store.Load(context.Background(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(set.Problems) != 0 || len(set.Records) != 6 {
		t.Fatalf("load records: %v %+v", err, set)
	}
	proj := resolve.New(set)
	decision := proj.Head("design/widget", "D-1")
	promise := proj.Head("prd/widget", "FR-1")
	if decision == nil || promise == nil || len(decision.Relations) != 1 || decision.Relations[0].Target != promise.ID {
		t.Fatalf("serves relation: %+v promise=%+v", decision, promise)
	}
	var answer, question *record.Record
	for _, candidate := range set.Records {
		if candidate.Kind == record.Answer {
			answer = candidate
		}
		if candidate.Kind == record.Question {
			question = candidate
		}
	}
	if question == nil || answer == nil || len(answer.Relations) != 1 || answer.Relations[0].Target != question.ID {
		t.Fatalf("answer relation: %+v question=%+v", answer, question)
	}
}

func TestWriteRecordsResolvesPinnedMainlineTarget(t *testing.T) {
	baseRoot := filepath.Join(t.TempDir(), store.Dir)
	at := time.Now().Add(-time.Hour).UTC()
	claim := &record.Record{
		Schema: record.Schema, ID: record.NewID(record.Promise, at, [10]byte{}),
		Kind: record.Promise, Subject: "prd/other", Family: "FR-2", Statement: "Queue safely.",
		Issuer: "agent:test", CreatedAt: at.Format(time.RFC3339),
	}
	if _, err := store.Write(baseRoot, claim); err != nil {
		t.Fatal(err)
	}
	base, err := store.Load(context.Background(), store.DirSource{Root: baseRoot})
	if err != nil {
		t.Fatal(err)
	}
	result := compile.Result{Feature: "widget", Records: []compile.Record{
		{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Use the other contract."}, Links: []compile.Link{{Type: record.Serves, Target: "prd/other#FR-2"}}},
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", Statement: "PRD", LiteralBody: true, Body: []byte("# PRD\n")}},
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "design/widget", Family: "design", Statement: "Spec", LiteralBody: true, Body: []byte("# Spec\n")}},
	}}
	root := t.TempDir()
	if _, _, err := writeRecords(root, "agent:test", result, readiness.Sources{Mainline: resolve.New(base)}); err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(context.Background(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	decision := resolve.New(set).Head("design/widget", "D-1")
	if decision == nil || len(decision.Relations) != 1 || decision.Relations[0].Target != claim.ID {
		t.Fatalf("external target was not pinned: %+v", decision)
	}
}

func TestWriteRecordsRejectsInvalidRecordsAndAbsentDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, issuer, statement, want string
		documents                     bool
	}{
		{"no documents", "agent:test", "Choice.", "PRD and tech spec Documents", false},
		{"invalid decision", "agent:test", "", "write compiled Decision", true},
		{"invalid issuer", "", "Choice.", "write compiled Decision", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := compile.Result{Feature: "widget", Records: []compile.Record{{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: tc.statement}}}}
			if tc.documents {
				result.Records = append(result.Records,
					compile.Record{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", Statement: "PRD", LiteralBody: true, Body: []byte("# PRD\n")}},
					compile.Record{Draft: reconcile.Draft{Kind: record.Document, Subject: "design/widget", Family: "design", Statement: "Spec", LiteralBody: true, Body: []byte("# Spec\n")}})
			}
			if _, _, err := writeRecords(t.TempDir(), tc.issuer, result, readiness.Sources{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	result := compile.Result{Feature: "widget", Records: []compile.Record{{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", LiteralBody: true, Body: []byte("# PRD\n")}}}}
	if _, _, err := writeRecords(t.TempDir(), "agent:test", result, readiness.Sources{}); err == nil || !strings.Contains(err.Error(), "write compiled Document") {
		t.Fatalf("invalid Document error=%v", err)
	}
}

func TestWriteRecordsRefusesMissingTargetsAndCycles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record []compile.Record
		want   string
	}{
		{"missing", []compile.Record{{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Choose."}, Links: []compile.Link{{Type: record.Serves, Target: "prd/widget#FR-99"}}}}, "target absent"},
		{"cycle", []compile.Record{
			{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Choose."}, Links: []compile.Link{{Type: record.DependsOn, Target: "design/widget#D-2"}}},
			{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-2", Statement: "Choose again."}, Links: []compile.Link{{Type: record.DependsOn, Target: "design/widget#D-1"}}},
		}, "cycle"},
		{"duplicate", []compile.Record{
			{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Choose."}},
			{Draft: reconcile.Draft{Kind: record.Decision, Subject: "design/widget", Family: "D-1", Statement: "Choose again."}},
		}, "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := compile.Result{Feature: "widget", Records: tc.record}
			if _, _, err := writeRecords(t.TempDir(), "agent:test", result, readiness.Sources{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
