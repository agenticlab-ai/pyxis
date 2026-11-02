package cursor

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
)

func TestReviewStreamUsesTerminalResultAndDistinctNativeToolCalls(t *testing.T) {
	stream := "tip\n" + `{"type":"system","subtype":"init","session_id":"fresh-root"}` + "\n" +
		`{"type":"tool_call","subtype":"started","call_id":"call-1"}` + "\n" +
		`{"type":"tool_call","subtype":"started","call_id":"call-1"}` + "\n" +
		`{"type":"tool_call","subtype":"completed","call_id":"call-1"}` + "\n" +
		`{"type":"tool_call","subtype":"started","call_id":"call-2"}` + "\n" +
		`{"type":"result","is_error":false,"result":"artifact","usage":{"inputTokens":8,"outputTokens":2}}` + "\r\n\u001b[0m"
	result, err := parseResult([]byte(stream))
	if err != nil || result.Incomplete || result.Output != "artifact" || result.Tokens != 10 || result.ToolCalls != 2 || !result.ToolCallsObserved || result.SessionID != "fresh-root" {
		t.Fatalf("stream result: %+v, %v", result, err)
	}
	partial, err := parseResult([]byte(`{"type":"tool_call","subtype":"started","call_id":"call-1"}`))
	if err != nil || !partial.Incomplete || partial.ToolCalls != 1 || partial.ToolCallsObserved {
		t.Fatalf("truncated stream: %+v %v", partial, err)
	}
}

func TestStreamPreservesHandoffMessageBoundariesOnlyForExactTerminalText(t *testing.T) {
	handoff := "```impl-review\n" + `{"schema_version":"impl-review-report/v1","proposal_ref":"github:o/r#1","review_id":"process-1","band":"standard","verdict":"approve","rounds":[{"reviewed_revision":"abc123","context_id":"context-1","findings":[]}]}` + "\n```"
	message := func(text string) string {
		data, err := json.Marshal(struct {
			Type    string `json:"type"`
			Message struct {
				Content []map[string]string `json:"content"`
			} `json:"message"`
		}{Type: "assistant", Message: struct {
			Content []map[string]string `json:"content"`
		}{[]map[string]string{{"type": "text", "text": text}}}})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	terminal := func(text string) string {
		data, err := json.Marshal(map[string]any{"type": "result", "result": text, "usage": map[string]int{"inputTokens": 8, "outputTokens": 2}})
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	commentary := "Screening complete."
	stream := message(commentary) + "\n" + `{"type":"tool_call","subtype":"started","call_id":"call-1"}` + "\n" + message(handoff) + "\n"
	got, err := parseResult([]byte(stream + terminal(commentary+handoff)))
	if err != nil || got.Incomplete || got.Tokens != 10 || got.ToolCalls != 1 {
		t.Fatalf("terminal result: %+v, %v", got, err)
	}
	parsed, err := persist.ParseHandoff(got.Output)
	if err != nil || parsed.ReviewID != "process-1" {
		t.Fatalf("native message boundary lost: %v", err)
	}
	if got.Output != commentary+"\n\n"+handoff {
		t.Fatal("adapter changed text beyond native message separators")
	}
	got, err = parseResult([]byte(stream + terminal("different terminal text")))
	if err != nil || got.Output != "different terminal text" {
		t.Fatalf("stale stream replaced terminal output: %+v, %v", got, err)
	}
	got, err = parseResult([]byte(stream))
	if err != nil || !got.Incomplete {
		t.Fatalf("assistant alone completed the run: %+v, %v", got, err)
	}
	got, err = parseResult([]byte(message(handoff) + "\n" + message(handoff) + "\n" + terminal(handoff+handoff)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persist.ParseHandoff(got.Output); !errors.Is(err, persist.ErrMultipleMachineBlocks) {
		t.Fatalf("surplus handoffs accepted: %v", err)
	}
	if !strings.Contains(got.Output, "\n\n") {
		t.Fatal("message boundaries not preserved")
	}
}
