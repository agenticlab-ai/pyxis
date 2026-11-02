package sessionlog

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestOptionalLogPaths(t *testing.T) {
	for _, tt := range []struct {
		path string
		want bool
	}{
		{"logs/work.md", true}, {"logs/README.md", true}, {"logs/runtime/run.jsonl", true},
		{"docs/logs/requirement.md", false}, {"logs-old/work.md", false},
		{"logs/../docs/prd.md", false}, {"../logs/work.md", false}, {"/logs/work.md", false},
	} {
		if got := Path(tt.path); got != tt.want {
			t.Errorf("Path(%q) = %t", tt.path, got)
		}
	}
}

func TestEmptyRequiresOnlyBookkeeping(t *testing.T) {
	const empty = "# Session log — work\n\nStarted: 2026-09-09 18:10 · CLI: Codex\n\nNothing escalated this session.\n"
	for _, tt := range []struct {
		name, body string
		want       bool
	}{
		{"skeleton", empty, true}, {"CRLF", strings.ReplaceAll(empty, "\n", "\r\n"), true},
		{"metadata correction", empty + "\nCorrection: CLI: Codex · Stage: follow-up\n", true},
		{"decision correction", empty + "\nCorrection: the operator decided to keep the contract.\n", false},
		{"wrapped task", strings.Replace(empty, " · CLI: Codex", " · Stage: plan task\nexample/001/T1", 1), true},
		{"appended decision", empty + "\n## P-1\nDecided to preserve source contracts.\n", false},
		{"substantive summary", empty + "\nFixed a runtime boundary.\n", false},
		{"unknown continuation", strings.Replace(empty, " · CLI: Codex", "\nWe decided to keep this requirement.", 1), false},
		{"ordinary document", strings.Replace(empty, "# Session log", "# Requirement", 1), false},
		{"empty file", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := Empty([]byte(tt.body)); got != tt.want {
				t.Fatalf("Empty = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRetirementPinsExactEmptyImport(t *testing.T) {
	envelope, err := os.ReadFile("testdata/empty.json")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("testdata/empty.md")
	if err != nil {
		t.Fatal(err)
	}
	var r record.Record
	if err := json.Unmarshal(envelope, &r); err != nil {
		t.Fatal(err)
	}
	if !Retired(&r, envelope, body) {
		t.Fatal("approved exact empty import was refused")
	}
	for _, tt := range []struct {
		name   string
		mutate func(*record.Record)
	}{
		{"requirement", func(r *record.Record) { r.Kind = record.Promise }},
		{"native", func(r *record.Record) { r.LiteralBody = false }},
		{"different source", func(r *record.Record) { r.Provenance.Source = "docs/requirements.md" }},
		{"different subject", func(r *record.Record) { r.Subject = "prd/widget" }},
		{"new envelope", func(r *record.Record) { r.Statement = "Newly authored record" }},
		{"outgoing relation", func(r *record.Record) { r.Relations = []record.Relation{{Type: record.DependsOn, Target: "other"}} }},
		{"embed", func(r *record.Record) { r.Embeds = []string{"prd/widget#FR-1"} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			candidate := r
			tt.mutate(&candidate)
			b, err := json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if Retired(&candidate, b, body) {
				t.Fatal("unapproved record admitted")
			}
		})
	}
	if Retired(nil, envelope, body) || Retired(&r, append(envelope, ' '), body) || Retired(&r, envelope, append(body, 'x')) {
		t.Fatal("non-exact bytes admitted")
	}
	original := retiredJSON
	t.Cleanup(func() { retiredJSON = original })
	retiredJSON = []byte("invalid")
	if Retired(&r, envelope, body) {
		t.Fatal("corrupt migration data admitted deletion")
	}
}
