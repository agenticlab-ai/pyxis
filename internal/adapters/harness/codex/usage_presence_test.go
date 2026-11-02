package codex

import "testing"

func TestUsagePresenceDistinguishesMissingAndObservedZero(t *testing.T) {
	for _, tc := range []struct {
		name, input   string
		tokens, turns bool
	}{
		{"zero", "{\"type\":\"turn.completed\",\"usage\":{\"total_tokens\":0}}", true, true},
		{"missing", "{\"type\":\"turn.completed\"}", false, true},
		{"empty usage", "{\"type\":\"turn.completed\",\"usage\":{}}", false, true},
		{"partial", "{\"type\":\"turn.completed\",\"usage\":{\"total_tokens\":7}}\n{\"type\":\"turn.completed\"}", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseResult([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if result.TokensObserved != tc.tokens || result.TurnsObserved != tc.turns {
				t.Fatalf("presence %+v; want tokens=%t turns=%t", result, tc.tokens, tc.turns)
			}
		})
	}
}
