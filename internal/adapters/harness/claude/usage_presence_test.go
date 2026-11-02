package claude

import "testing"

func TestUsagePresenceDistinguishesMissingAndObservedZero(t *testing.T) {
	for _, tc := range []struct {
		name, input   string
		tokens, turns bool
	}{
		{"zero", "{\"num_turns\":0,\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}", true, true},
		{"missing", "{}", false, false},
		{"empty usage", "{\"usage\":{}}", false, false},
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
