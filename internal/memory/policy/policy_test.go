package policy

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestDefaultRoundTrips(t *testing.T) {
	b, err := Default().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	p, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.Consequence.ByKind[record.Policy] != record.High || p.Ask.Similarity != 0.6 || p.Pack.DefaultBudgetTokens != 12000 || p.Consequence.ManyClaims != 10 {
		t.Fatalf("default did not round trip: %+v", p)
	}
	if d, err := Parse(nil); err != nil || d.Pack.DefaultBudgetTokens != 12000 {
		t.Fatal("nil bytes mean the default")
	}
}

func TestParseFillsGapsAndRefusesNonsense(t *testing.T) {
	p, err := Parse([]byte(`{"schema":1,"consequence":{"by_kind":{"Promise":"high"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Consequence.ByKind[record.Promise] != record.High || len(p.Consequence.HighSubjectPrefixes) != 2 || p.Ask.Similarity != 0.6 {
		t.Fatalf("gaps not filled: %+v", p)
	}
	cases := map[string]string{
		`{"schema":2}`:                "schema 2",
		`{"schema":1,"colour":"red"}`: "unknown field",
		`{"schema":1,"consequence":{"by_kind":{"X":"low"}}}`:        "unknown kind",
		`{"schema":1,"consequence":{"by_kind":{"Promise":"huge"}}}`: "not low, medium or high",
		`{"schema":1,"ask":{"similarity":2}}`:                       "within 0..1",
		`{"schema":1,"pack":{"default_budget_tokens":-1}}`:          "negative",
		`{"schema":1`: "policy.json",
	}
	for in, want := range cases {
		if _, err := Parse([]byte(in)); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: got %v, want %q", in, err, want)
		}
	}
}

func TestTiers(t *testing.T) {
	if Rank(record.Low) != 0 || Rank(record.Medium) != 1 || Rank(record.High) != 2 || Rank("x") != -1 {
		t.Fatal("Rank")
	}
	if Max(record.Low, record.High) != record.High || Max(record.Medium, record.Low) != record.Medium || Max("", record.Low) != record.Low {
		t.Fatal("Max")
	}
	if Raise(record.Low) != record.Medium || Raise(record.Medium) != record.High || Raise(record.High) != record.High {
		t.Fatal("Raise")
	}
	p := Default()
	cases := []struct {
		r    record.Record
		want string
	}{
		{record.Record{Kind: record.Need, Subject: "prd/x"}, record.Low},
		{record.Record{Kind: record.Promise, Subject: "prd/x"}, record.Medium},
		{record.Record{Kind: record.Need, Subject: "governance/principles"}, record.High},
		{record.Record{Kind: record.Learning, Subject: "prd/x", Consequence: record.High}, record.High},
		{record.Record{Kind: record.Promise, Subject: "prd/x", Consequence: record.Low}, record.Medium},
		{record.Record{Kind: "Unknown", Subject: "prd/x"}, record.Low},
	}
	for _, c := range cases {
		if got := p.ForRecord(&c.r); got != c.want {
			t.Fatalf("%s on %s declared %q: got %s want %s", c.r.Kind, c.r.Subject, c.r.Consequence, got, c.want)
		}
	}
}
