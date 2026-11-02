package opencode

import "testing"

func TestNativeToolUsageDistinguishesZeroMissingAndPartialTrace(t *testing.T) {
	const finished = `{"type":"step_finish","part":{"type":"step-finish","reason":"stop","tokens":{"input":0,"output":0}}}`
	for _, tc := range []struct {
		name, input string
		calls       int
		observed    bool
	}{
		{"two tool events", "{\"type\":\"tool_use\"}\n{\"type\":\"tool_use\"}\n" + finished, 2, true},
		{"native zero", finished, 0, true},
		{"legacy has no trace", `{"steps":1,"tokens":{"input":0}}`, 0, false},
		{"error has no trace", `{"type":"error","error":"failed"}`, 0, false},
		{"partial error trace", "{\"type\":\"tool_use\"}\n{\"type\":\"error\",\"error\":\"failed\"}", 1, false},
		{"unfinished trace after step", finished + "\n{\"type\":\"tool_use\"}", 1, false},
		{"nonterminal step", `{"type":"step_finish","part":{"type":"step-finish","reason":"tool-calls"}}`, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseResult([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if result.ToolCalls != tc.calls || result.ToolCallsObserved != tc.observed {
				t.Fatalf("usage=%+v; want calls=%d observed=%t", result, tc.calls, tc.observed)
			}
		})
	}
}
