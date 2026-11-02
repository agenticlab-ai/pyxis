package issueentry

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCollectorRetainsAndRendersNativeToolUsage(t *testing.T) {
	for _, tc := range []struct {
		name, state, rendered string
		calls                 int
		observed              bool
	}{
		{"observed", "observed", "7", 7, true},
		{"observed zero", "observed", "0", 0, true},
		{"missing", "unavailable", "unavailable", 0, false},
		{"partial", "partial", "7 (partial)", 7, false},
		{"invalid", "unavailable", "unavailable", -1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := Collector{RunID: "test/"}
			finish := collector.Launched("opencode", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "model", NativeEffort: "none"})
			finish(port.RunResult{ToolCalls: tc.calls, ToolCallsObserved: tc.observed}, nil)
			finish(port.RunResult{ToolCalls: 99, ToolCallsObserved: true}, nil)
			summary := collector.Summary(Contract{ID: "contract"}, "github:owner/repo#1", 1, "head", "verified_pr", 0)
			counter := summary.Attempts[0].ToolCalls
			if counter.State != tc.state || counter.Value != nil && *counter.Value != int64(tc.calls) {
				t.Fatalf("tool usage=%+v", counter)
			}
			body, err := MergeSummary("", summary)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body, "| implementation | opencode | model | none | unavailable | "+tc.rendered+" |") {
				t.Fatalf("tool usage absent from readable summary: %s", body)
			}
		})
	}
}
