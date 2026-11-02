package opencode

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Task session continuity on the opencode CLI: a resumed implementation launch
// adds `--session <id>`, a first launch does not (opencode mints ids), a review
// never resumes, and the capture strips the session before its closed check.
func TestLaunchArgs_SessionResume(t *testing.T) {
	a := mustAdapter(t)
	resumed := a.launchArgs(port.LaunchSpec{Model: "anthropic/claude", Effort: "high", Session: &port.SessionSpec{ID: "ses_1", Resume: true}}, "brief")
	joined := strings.Join(resumed, " ")
	if !strings.Contains(joined, "--model anthropic/claude --session ses_1 brief") {
		t.Fatalf("resumed argv = %q", joined)
	}
	capture, err := captureLaunch(resumed)
	if err != nil || capture.NativeModel != "anthropic/claude" {
		t.Fatalf("capture = %+v, %v", capture, err)
	}
	fresh := strings.Join(a.launchArgs(port.LaunchSpec{Model: "anthropic/claude", Effort: "high", Session: &port.SessionSpec{ID: "pre"}}, "brief"), " ")
	if strings.Contains(fresh, "--session") {
		t.Fatalf("a first launch cannot pre-assign an opencode session: %q", fresh)
	}
	review := strings.Join(a.launchArgs(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Model: "anthropic/claude", Effort: "high", Session: &port.SessionSpec{ID: "ses_1", Resume: true}}, "brief"), " ")
	if strings.Contains(review, "--session") {
		t.Fatalf("review argv must never resume: %q", review)
	}
}

func TestParseResult_ReportsSessionID(t *testing.T) {
	stream := "{\"type\":\"step_start\",\"sessionID\":\"ses_abc\",\"part\":{\"type\":\"step-start\"}}\n" +
		"{\"type\":\"text\",\"sessionID\":\"ses_abc\",\"part\":{\"type\":\"text\",\"text\":\"done\"}}\n" +
		"{\"type\":\"step_finish\",\"sessionID\":\"ses_abc\",\"part\":{\"type\":\"step-finish\",\"tokens\":{\"input\":10,\"output\":5}}}\n"
	res, err := parseResult([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "ses_abc" || res.Turns != 1 || res.Output != "done" {
		t.Fatalf("result = %+v", res)
	}
}
