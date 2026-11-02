package cursor

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestNativeChildOutcomesCannotBeReplacedByParentApproval(t *testing.T) {
	childOutput := `{"lens":"correctness-critic","candidates":[],"digest_mismatch":"expected body digest; observed whole file"}`
	wire, err := json.Marshal(childOutput)
	if err != nil {
		t.Fatal(err)
	}
	event := `{"type":"tool_call","subtype":"completed","call_id":"native-call","tool_call":{"taskToolCall":{"result":{"success":{"agentId":"actual-child","conversationSteps":[{"assistantMessage":{"text":` + string(wire) + `}}]}}}}}`
	data := event + "\n" + event + "\n" + `{"type":"result","subtype":"success","is_error":false,"result":"parent approves"}`
	result, err := parseResult([]byte(data))
	want := &port.NativeReviewChildren{Children: []port.NativeReviewChild{{ContextID: "actual-child", Completed: true, Output: childOutput}}}
	if err != nil || result.Output != "parent approves" || !reflect.DeepEqual(result.ReviewChildren, want) {
		t.Fatalf("native outcome was lost: %+v %v", result, err)
	}
}

func TestNativeChildCapturePreservesFailureAndUnknownCallIdentity(t *testing.T) {
	data := `{"type":"tool_call","subtype":"completed","tool_call":{"taskToolCall":{"result":{"error":{"message":"child failed"}}}}}` + "\n" +
		`{"type":"tool_call","subtype":"completed","tool_call":{"taskToolCall":{"result":{"success":{"agentId":"another-child","conversationSteps":[]}}}}}` + "\n" +
		`{"type":"result","result":"parent approves"}`
	result, err := parseResult([]byte(data))
	if err != nil || result.ReviewChildren == nil || len(result.ReviewChildren.Children) != 2 || result.ReviewChildren.Children[0].Completed || result.ReviewChildren.Children[1].ContextID != "another-child" {
		t.Fatalf("missing/failing native identity disappeared: %+v %v", result, err)
	}
}

func TestNativeChildCaptureRejectsMalformedNativeResult(t *testing.T) {
	_, err := parseResult([]byte(`{"type":"tool_call","subtype":"completed","tool_call":{"taskToolCall":{"result":{"success":true}}}}`))
	if err == nil || !strings.Contains(err.Error(), "native child") {
		t.Fatalf("malformed native child accepted: %v", err)
	}
}
