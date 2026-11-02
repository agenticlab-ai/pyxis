package lifecycle

import "testing"

func TestCoverage90DecisionString(t *testing.T) {
	if got := (Decision{Action: ActionResume, AtRound: 2}).String(); got != "resume@round2" {
		t.Fatalf("Decision.String() = %q", got)
	}
}
