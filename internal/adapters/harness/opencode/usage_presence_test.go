package opencode

import "testing"

func TestUsagePresenceDistinguishesMissingAndObservedZero(t *testing.T) {
	for _, tc := range []struct {
		name, input   string
		tokens, turns bool
	}{
		{"zero", "{\"steps\":1,\"tokens\":{\"input\":0,\"output\":0}}", true, true},
		{"missing", "{\"steps\":1}", false, true},
		{"empty usage", "{\"steps\":1,\"tokens\":{}}", false, true},
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
