package cursor

import (
	"strings"
	"testing"
)

func TestNativeNestedToolIDsAreCountedOnce(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","session_id":"native-root"}`,
		`{"type":"tool_call","subtype":"started","tool_call":{"readToolCall":{"args":{"path":"file.go","toolCallId":"native-1"}}}}`,
		`{"type":"tool_call","subtype":"started","tool_call":{"readToolCall":{"args":{"toolCallId":"native-1"}}}}`,
		`{"type":"tool_call","subtype":"completed","tool_call":{"readToolCall":{"args":{"toolCallId":"native-1"}}}}`,
		`{"type":"tool_call","subtype":"started","call_id":"outer-alias","tool_call":{"shellToolCall":{"args":{"toolCallId":"native-2"}}}}`,
		`{"type":"result","result":"done","is_error":false}`,
	}, "\n")
	got, err := parseResult([]byte(stream))
	if err != nil || got.ToolCalls != 2 || !got.ToolCallsObserved || got.Incomplete {
		t.Fatalf("native tool accounting = %+v, %v", got, err)
	}
}

func TestUnidentifiedNativeToolLeavesAccountingUnavailable(t *testing.T) {
	for _, payload := range []string{`{}`, `{"futureToolCall":{"args":{}}}`, `{"readToolCall":{"args":{"toolCallId":" "}}}`} {
		stream := `{"type":"tool_call","subtype":"started","tool_call":` + payload + `}` + "\n" + `{"type":"result","result":"done"}`
		got, err := parseResult([]byte(stream))
		if err != nil || got.ToolCallsObserved || got.Incomplete {
			t.Fatalf("unidentified tool claimed complete accounting: %+v, %v", got, err)
		}
	}
}
