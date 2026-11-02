package port

import "testing"

func TestScopeTokenPreservesLegacyAndSeparatesOrigins(t *testing.T) {
	if got := (Task{}).ScopeToken(); got != "" {
		t.Fatalf("legacy scope=%q", got)
	}
	one := Task{SourceKey: "forgejo:http://localhost:3000/a:o/r"}
	two := Task{SourceKey: "forgejo:http://localhost:3001/a:o/r"}
	if one.ScopeToken() == two.ScopeToken() || len(one.ScopeToken()) != 64 {
		t.Fatal("scope not host-qualified")
	}
}
