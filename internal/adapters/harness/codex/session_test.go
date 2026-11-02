package codex

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// Task session continuity on the codex CLI: a resumed implementation launch is
// `exec resume <thread> -` with the same closed flags, a first launch is
// unchanged (codex mints the thread), a review never resumes, and the capture
// admits the resumed shape without treating the thread id as routing.
func TestLaunchArgs_ResumeThread(t *testing.T) {
	a := mustAdapter(t)
	resumed := a.launchArgs(port.LaunchSpec{Model: "gpt-5.6", Effort: "high", Session: &port.SessionSpec{ID: "thread-1", Resume: true}})
	joined := strings.Join(resumed, " ")
	if !strings.HasPrefix(joined, "--no-daemon exec resume --json -m gpt-5.6 -c model_reasoning_effort=\"high\" --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check thread-1 -") {
		t.Fatalf("resumed argv = %q", joined)
	}
	capture, err := captureNativeLaunch(resumed)
	if err != nil || capture.NativeModel != "gpt-5.6" || capture.NativeEffort != "high" {
		t.Fatalf("capture = %+v, %v", capture, err)
	}
	fresh := strings.Join(a.launchArgs(port.LaunchSpec{Model: "gpt-5.6", Effort: "high", Session: &port.SessionSpec{ID: "pre-assigned"}}), " ")
	if strings.Contains(fresh, "resume") || strings.Contains(fresh, "pre-assigned") {
		t.Fatalf("a first launch cannot pre-assign a codex thread: %q", fresh)
	}
	review := strings.Join(a.launchArgs(port.LaunchSpec{Purpose: port.LaunchPurposeReview, Model: "m", Effort: "high", Session: &port.SessionSpec{ID: "t", Resume: true}}), " ")
	if strings.Contains(review, "resume") {
		t.Fatalf("review argv must never resume: %q", review)
	}
	if _, err := captureNativeLaunch([]string{"exec", "resume", "--json", "-m", "m", "-c", "model_reasoning_effort=\"high\"", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", " ", "-"}); err == nil {
		t.Fatal("an empty thread id must not attest")
	}
}

func TestParseResult_ReportsThreadID(t *testing.T) {
	stream := "{\"type\":\"thread.started\",\"thread_id\":\"0199-thread\"}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5,\"total_tokens\":15}}\n" +
		"{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n"
	res, err := parseResult([]byte(stream))
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != "0199-thread" || res.Turns != 1 || res.Output != "done" {
		t.Fatalf("result = %+v", res)
	}
}
