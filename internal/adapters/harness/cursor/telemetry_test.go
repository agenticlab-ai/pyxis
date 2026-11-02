package cursor

import "testing"

func TestParseCurrentAndLegacyCursorUsage(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		tokens      int
		observed    bool
	}{
		{"live current result", `noise {"type":"result","usage":{"inputTokens":116916,"outputTokens":8322,"cacheReadTokens":1572774,"cacheWriteTokens":0}} prompt`, 1698012, true},
		{"current observed zero", `{"usage":{"inputTokens":0,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0}}`, 0, true},
		{"legacy", `{"usage":{"input_tokens":2,"output_tokens":3,"cache_read_input_tokens":5,"cache_creation_input_tokens":7}}`, 17, true},
		{"aliases do not double count", `{"usage":{"inputTokens":2,"input_tokens":2,"outputTokens":3,"output_tokens":3}}`, 5, true},
		{"current zero takes precedence", `{"usage":{"inputTokens":0,"input_tokens":9}}`, 0, true},
		{"null current falls back", `{"usage":{"inputTokens":null,"input_tokens":9}}`, 9, true},
		{"empty", `{"usage":{}}`, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseResult([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if result.Tokens != tc.tokens || result.TokensObserved != tc.observed {
				t.Fatalf("usage: %+v; want tokens=%d observed=%t", result, tc.tokens, tc.observed)
			}
		})
	}
}
