package capture

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func TestDerivedPreservesKnowledgeBeyondCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    *record.Record
		want bool
	}{
		{name: "nil"},
		{name: "generated inventory", r: &record.Record{Kind: record.Need, Family: "anchor", Tags: []string{"reconcile:anchor"}}, want: true},
		{name: "backfilled package summary", r: &record.Record{Kind: record.Learning, Intent: "Preserve the implemented responsibility of internal/widget in durable software memory.", Rationale: "Source-code audit found production code without a dedicated semantic memory record.", Tags: []string{"reconcile:code-semantics"}}, want: true},
		{name: "testing lesson", r: &record.Record{Kind: record.Learning, Statement: "Use virtual time for deadline transitions.", Rationale: "Elapsed-time assertions conflate timer policy with host scheduling.", Tags: []string{"code:internal/verify/runner"}}},
		{name: "backfill with independent rationale", r: &record.Record{Kind: record.Learning, Intent: "Preserve the implemented responsibility of internal/widget in durable software memory.", Rationale: "A one-use scope prevents an authorization from widening.", Tags: []string{"reconcile:code-semantics"}}},
		{name: "architectural constraint", r: &record.Record{Kind: record.Decision, Statement: "The engine depends on consumer-side ports.", Rationale: "A provider-specific dependency would prevent offline adapters."}},
		{name: "requirement referencing inventory", r: &record.Record{Kind: record.Promise, Family: "FR-1", Tags: []string{"reconcile:anchor"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Derived(tc.r); got != tc.want {
				t.Fatalf("Derived = %t, want %t", got, tc.want)
			}
			if err := CheckNew(tc.r); (err != nil) != tc.want {
				t.Fatalf("admission = %v, want refusal %t", err, tc.want)
			}
		})
	}
}
