package association

import (
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

type records map[string]*record.Record

func (r records) Get(id string) *record.Record { return r[id] }

func TestScopes(t *testing.T) {
	codeAnchor := &record.Record{ID: "Need-code", Subject: "code/executor", Tags: []string{"anchor", "code:cmd/executor", "code:internal/executor"}}
	fallbackAnchor := &record.Record{ID: "Need-fallback", Subject: "code/fallback", Tags: []string{"anchor"}}
	emptyTaggedAnchor := &record.Record{ID: "Need-empty", Subject: "code/empty-fallback", Tags: []string{"code:"}}
	nestedAnchor := &record.Record{ID: "Need-nested", Subject: "code/nested", Tags: []string{"code:internal/nested"}}
	nonRecursiveAnchor := &record.Record{ID: "Need-one-hop", Subject: "code/one-hop", Relations: []record.Relation{{Type: record.About, Target: nestedAnchor.ID}}}
	notCode := &record.Record{ID: "Need-prd", Subject: "prd/executor"}
	r := &record.Record{
		Tags: []string{"review", "code:internal/executor", "code:", "code:internal/executor", "code:cmd/executor"},
		Relations: []record.Relation{
			{Type: record.About, Target: codeAnchor.ID},
			{Type: record.About, Target: fallbackAnchor.ID},
			{Type: record.About, Target: emptyTaggedAnchor.ID},
			{Type: record.About, Target: nonRecursiveAnchor.ID},
			{Type: record.About, Target: notCode.ID},
			{Type: record.About, Target: "Need-missing"},
			{Type: record.DependsOn, Target: codeAnchor.ID},
		},
	}
	got := Scopes(r,
		records{notCode.ID: notCode},
		records{codeAnchor.ID: codeAnchor, fallbackAnchor.ID: fallbackAnchor, emptyTaggedAnchor.ID: emptyTaggedAnchor, nonRecursiveAnchor.ID: nonRecursiveAnchor, nestedAnchor.ID: nestedAnchor},
	)
	want := []string{"internal/executor", "cmd/executor", "fallback", "one-hop"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scopes() = %v, want %v", got, want)
	}
	if got := Scopes(nil, records{}); got != nil {
		t.Fatalf("Scopes(nil) = %v, want nil", got)
	}
	unlinked := &record.Record{Subject: "code/own", Statement: "internal/body.go", Provenance: record.Provenance{Source: "internal/source.go"}}
	if got := Scopes(unlinked, records{}); got != nil {
		t.Fatalf("Scopes() inferred from subject, prose, or provenance: %v", got)
	}
}

func TestCoversAndMatches(t *testing.T) {
	tests := []struct {
		name        string
		a           string
		b           string
		wantCovers  bool
		wantMatches bool
	}{
		{name: "exact", a: "internal/foo", b: "internal/foo", wantCovers: true, wantMatches: true},
		{name: "scope contains file", a: "internal/foo", b: "internal/foo/file.go", wantCovers: true, wantMatches: true},
		{name: "file relates to parent", a: "internal/foo/file.go", b: "internal/foo", wantCovers: false, wantMatches: true},
		{name: "sibling prefix", a: "internal/foo", b: "internal/foobar"},
		{name: "nested sibling prefix", a: "internal/foo/bar", b: "internal/foo/barista"},
		{name: "different", a: "cmd/foo", b: "internal/foo"},
		{name: "empty left", a: "", b: "internal/foo"},
		{name: "empty right", a: "internal/foo", b: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Covers(tt.a, tt.b); got != tt.wantCovers {
				t.Errorf("Covers(%q, %q) = %t, want %t", tt.a, tt.b, got, tt.wantCovers)
			}
			if got := Matches(tt.a, tt.b); got != tt.wantMatches {
				t.Errorf("Matches(%q, %q) = %t, want %t", tt.a, tt.b, got, tt.wantMatches)
			}
		})
	}
}
