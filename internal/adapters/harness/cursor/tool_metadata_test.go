package cursor

import (
	"strings"
	"testing"
)

func TestNativeToolMetadataDoesNotBreakResultDecoding(t *testing.T) {
	for _, metadata := range []string{
		`"hookAdditionalContexts":[]`,
		`"hookAdditionalContexts":[{"context":"hook output"}]`,
		`"futureMetadata":true`,
		`"futureMetadata":42`,
		`"futureMetadata":null`,
	} {
		t.Run(metadata, func(t *testing.T) {
			payload := `{"readToolCall":{"args":{"toolCallId":"native-1"}},` + metadata + `}`
			stream := strings.Join([]string{
				`{"type":"tool_call","subtype":"started","tool_call":` + payload + `}`,
				`{"type":"tool_call","subtype":"completed","tool_call":` + payload + `}`,
				`{"type":"result","result":"done","is_error":false}`,
			}, "\n")
			got, err := parseResult([]byte(stream))
			if err != nil || got.ToolCalls != 1 || !got.ToolCallsObserved || got.Incomplete || got.Output != "done" {
				t.Fatalf("native metadata broke the completed review: %+v, %v", got, err)
			}
		})
	}
}

func TestToolMetadataCannotInventLegacyToolIdentities(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"tool_call","subtype":"started","call_id":"legacy-1","tool_call":{"annotation":{"args":{"toolCallId":"fake-1"}}}}`,
		`{"type":"tool_call","subtype":"started","call_id":"legacy-1","tool_call":{"annotation":{"args":{"toolCallId":"fake-2"}}}}`,
		`{"type":"result","result":"done"}`,
	}, "\n")
	got, err := parseResult([]byte(stream))
	if err != nil || got.ToolCalls != 1 || !got.ToolCallsObserved {
		t.Fatalf("metadata invented tool identities: %+v, %v", got, err)
	}
}

func TestMalformedNativeToolPayloadStillFailsDecoding(t *testing.T) {
	for _, payload := range []string{`[]`, `{"args":{"toolCallId":42}}`} {
		stream := `{"type":"tool_call","subtype":"started","tool_call":{"readToolCall":` + payload + `}}` + "\n" + `{"type":"result","result":"done"}`
		if _, err := parseResult([]byte(stream)); err == nil {
			t.Fatal("malformed native tool payload was accepted")
		}
	}
}
