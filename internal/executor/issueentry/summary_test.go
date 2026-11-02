package issueentry

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestObservedUsageAndOwnedSectionRoundTrip(t *testing.T) {
	c := Collector{RunID: "run-a/"}
	finish := c.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "actual-model", NativeEffort: "high"})
	finish(port.RunResult{TokensObserved: true, TurnsObserved: true}, nil)
	finish(port.RunResult{Tokens: 999}, nil) // A repeated Await cannot double-count.
	c.Launched("claude", port.LaunchPurposeReview, port.LaunchCapture{NativeModel: "actual-review", NativeEffort: "deep"})(port.RunResult{Incomplete: true}, nil)
	s := c.Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "revision", "review_pending", 123)
	if len(s.Attempts) != 2 || s.Attempts[0].Tokens.Value == nil || *s.Attempts[0].Tokens.Value != 0 || s.Attempts[1].Tokens.State != "unavailable" || s.TotalTokens.State != "partial" || s.IntakeTokens.State != "unavailable" {
		t.Fatalf("invented usage: %+v", s)
	}
	body := "Authored explanation\n\n<!-- pyxis-task-proposal: preserved -->\n"
	merged, err := MergeSummary(body, s)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("testdata/summary.golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if merged != string(golden) {
		t.Fatalf("readable/machine summary changed:\n%s", merged)
	}
	again, err := MergeSummary(merged, s)
	if err != nil || again != merged || !strings.Contains(merged, "pyxis-task-proposal: preserved") {
		t.Fatalf("non-idempotent/destructive merge: %v", err)
	}
	wire, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSummary(wire); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSummary(append([]byte(`{"unknown":0,`), wire[1:]...)); err == nil {
		t.Fatal("open summary codec")
	}
	for _, bad := range []string{body + summaryStart, body + summaryEnd, merged + summaryStart, summaryEnd + summaryStart} {
		if _, err := MergeSummary(bad, s); err == nil {
			t.Fatal("accepted malformed markers")
		}
	}
}

func TestRetainedObservationsSurviveRetryWithoutDoubleCounting(t *testing.T) {
	left, right := stores(t)
	input, snapshot := intake(t)
	contract, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := left.Admit(t.Context(), contract); err != nil {
		t.Fatal(err)
	}
	c := Collector{RunID: "implementation/"}
	c.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "actual", NativeEffort: "high"})(port.RunResult{Tokens: 17, TokensObserved: true, Turns: 1, TurnsObserved: true, ToolCalls: 3, ToolCallsObserved: true}, nil)
	s := c.Summary(contract, "https://github.com/o/r/issues/17", 77, "head", "review_pending", 123)
	for range 2 {
		if _, err := left.RetainSummary(t.Context(), contract, s); err != nil {
			t.Fatal(err)
		}
	}
	retry := Collector{RunID: "review/"}
	retry.Launched("claude", port.LaunchPurposeReview, port.LaunchCapture{NativeModel: "review-model", NativeEffort: "deep"})(port.RunResult{Tokens: 23, TokensObserved: true}, nil)
	merged, err := right.RetainSummary(t.Context(), contract, retry.Summary(contract, s.Source, 77, "head", "verified_pr", 321))
	if err != nil {
		t.Fatal(err)
	}
	if len(merged.Attempts) != 2 || merged.TotalTokens.Value == nil || *merged.TotalTokens.Value != 40 || merged.Terminal != "verified_pr" || merged.ExecutionElapsedMS != 321 {
		t.Fatalf("retry lost/counted observations: %+v", merged)
	}
	if tools := merged.Attempts[0].ToolCalls; tools.State != "observed" || tools.Value == nil || *tools.Value != 3 {
		t.Fatalf("retry lost or counted tool observations twice: %+v", tools)
	}
	if _, err := right.RetainSummary(t.Context(), contract, retry.Summary(contract, s.Source, 88, "head", "verified_pr", 321)); err == nil {
		t.Fatal("summary aliased another PR")
	}
}
