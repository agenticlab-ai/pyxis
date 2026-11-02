package githost

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

func TestLastRecordedHeadBoundaries(t *testing.T) {
	record := verdict.Record{RecordVersion: verdict.RecordVersion, Lane: "HUMAN", PolicyVersion: "risk-policy/v1", HeadRevision: "abc123"}
	block := verdict.AppendBlock(nil, record)
	body := BlockMarker + "\n```json\n" + strings.TrimRight(string(block), "\n") + "\n```"
	comments, err := json.Marshal([]map[string]any{{"id": 42, "body": body}})
	if err != nil {
		t.Fatal(err)
	}
	s := &script{responses: map[string]string{"comments?per_page=100": string(comments)}}
	if got, err := host(s).LastRecordedHead(context.Background(), "7"); err != nil || got != "abc123" {
		t.Fatalf("LastRecordedHead = %q, %v", got, err)
	}

	s = &script{responses: map[string]string{"comments?per_page=100": `[]`}}
	if got, err := host(s).LastRecordedHead(context.Background(), "7"); err != nil || got != "" {
		t.Fatalf("missing verdict block = %q, %v", got, err)
	}

	malformedBody := BlockMarker + "\n```json\nnot-json\n```"
	comments, err = json.Marshal([]map[string]any{{"id": 42, "body": malformedBody}})
	if err != nil {
		t.Fatal(err)
	}
	s = &script{responses: map[string]string{"comments?per_page=100": string(comments)}}
	if got, err := host(s).LastRecordedHead(context.Background(), "7"); err != nil || got != "" {
		t.Fatalf("malformed verdict block = %q, %v", got, err)
	}
}

func TestOpenProposalsDecodesTheExactWorkList(t *testing.T) {
	s := &script{responses: map[string]string{
		"pulls?state=open&per_page=100": `[{"number":7,"head":{"sha":"abc123"}},{"number":8,"head":{"sha":"def456"}}]`,
	}}
	got, err := host(s).OpenProposals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Number != 7 || got[0].Head != "abc123" || got[1].Number != 8 || got[1].Head != "def456" {
		t.Fatalf("OpenProposals = %+v", got)
	}
}
