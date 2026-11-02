package claude

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Task session continuity on the claude CLI: a first launch pre-assigns the
// conversation id, a later launch of the same task resumes it, a review never
// carries one, and the launch capture treats the flag as non-routing.
func TestLaunchArgs_SessionFlags(t *testing.T) {
	a := mustAdapter(t)
	fresh := strings.Join(a.launchArgs(port.LaunchSpec{Model: "m", Effort: "high", Session: &port.SessionSpec{ID: "11111111-2222-3333-4444-555555555555"}}), " ")
	if !strings.Contains(fresh, "--session-id 11111111-2222-3333-4444-555555555555") || strings.Contains(fresh, "--resume") {
		t.Fatalf("fresh session argv = %q", fresh)
	}
	resumed := strings.Join(a.launchArgs(port.LaunchSpec{Model: "m", Effort: "high", Session: &port.SessionSpec{ID: "abc", Resume: true}}), " ")
	if !strings.Contains(resumed, "--resume abc") || strings.Contains(resumed, "--session-id") {
		t.Fatalf("resumed argv = %q", resumed)
	}
	review := strings.Join(a.launchArgs(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Model: "m", Effort: "high", Session: &port.SessionSpec{ID: "abc", Resume: true}}), " ")
	if strings.Contains(review, "--resume") || strings.Contains(review, "--session-id") {
		t.Fatalf("review argv must never carry a session: %q", review)
	}
	none := strings.Join(a.launchArgs(port.LaunchSpec{Model: "m", Effort: "high"}), " ")
	if strings.Contains(none, "session") || strings.Contains(none, "resume") {
		t.Fatalf("no-session argv unchanged: %q", none)
	}
	for _, args := range [][]string{
		a.launchArgs(port.LaunchSpec{Model: "claude-opus-5", Effort: "high", Session: &port.SessionSpec{ID: "abc", Resume: true}}),
		a.launchArgs(port.LaunchSpec{Model: "claude-opus-5", Effort: "high", Session: &port.SessionSpec{ID: "abc"}}),
	} {
		capture, err := a.decodeLaunchCapture(args)
		if err != nil {
			t.Fatalf("capture with session flag: %v (%q)", err, args)
		}
		if capture.NativeModel != "claude-opus-5" || capture.NativeEffort != "high" {
			t.Fatalf("capture = %+v", capture)
		}
	}
	if _, err := a.decodeLaunchCapture([]string{"-p", "--model", "m", "--effort", "high", "--output-format", "json", "--permission-mode", "bypassPermissions", "--resume"}); err == nil {
		t.Fatal("a valueless --resume must not attest")
	}
}

func TestParseResult_ReportsSessionID(t *testing.T) {
	res, err := parseResult([]byte(`{"type":"result","subtype":"success","is_error":false,"num_turns":3,"session_id":"de1a3386-3e36-4809-b756-1523df81e9c0","result":"ok","usage":{"input_tokens":1,"output_tokens":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "de1a3386-3e36-4809-b756-1523df81e9c0" || res.Turns != 3 {
		t.Fatalf("result = %+v", res)
	}
}
