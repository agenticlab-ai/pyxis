package opencode

import (
	"strings"
	"testing"
)

func TestNativeProviderErrorRetainsRemedyAndRetryPolicy(t *testing.T) {
	input := `{"type":"error","error":{"name":"APIError","data":{"message":"Select Global in workspace Privacy settings","statusCode":400,"isRetryable":false,"responseHeaders":{"authorization":"secret"}}}}`
	result, err := parseResult([]byte(input))
	if err != nil || !result.Incomplete || !result.NonRetryable || !strings.Contains(result.Reason, "HTTP 400: Select Global") || strings.Contains(result.Reason, "secret") {
		t.Fatalf("native failure: %+v, %v", result, err)
	}
	for _, value := range []string{"true", "null"} {
		result, err := parseResult([]byte(strings.Replace(input, "\"isRetryable\":false", "\"isRetryable\":"+value, 1)))
		if err != nil || result.NonRetryable {
			t.Fatalf("unknown/retryable policy became terminal: %+v, %v", result, err)
		}
	}
}

func TestUnfinishedRepairTracePreservesPartialUsage(t *testing.T) {
	trace := `{"type":"step_finish","part":{"type":"step-finish","reason":"tool-calls","tokens":{"input":8,"output":3}}}` + "\n" + `{"type":"tool_use","part":{"state":{"status":"error","error":"external_directory rejected"}}}`
	result, err := parseResult([]byte(trace))
	if err != nil || !result.Incomplete || result.Tokens != 11 || result.ToolCalls != 1 || result.ToolCallsObserved {
		t.Fatalf("partial repair: %+v, %v", result, err)
	}
	if partialUsage([]byte(trace)).Tokens != 11 || partialUsage([]byte("invalid")).TokensObserved {
		t.Fatal("operational failure lost native usage or invented unavailable usage")
	}
}
