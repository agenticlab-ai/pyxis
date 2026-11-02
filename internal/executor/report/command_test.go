package report

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestCommandEventsReachAllSinksWithObservedZeroAndFailure(t *testing.T) {
	elapsed, exitCode := int64(0), 0
	for _, tc := range []struct {
		event Event
		flow  string
		level string
	}{
		{Event{Kind: KindCommandStarted, Scope: "issue/contract"}, "command run started (scope issue/contract)", "INFO"},
		{Event{Kind: KindCommandPhaseStarted, Phase: "final_verification", Status: "running"}, "command run: final_verification started", "INFO"},
		{Event{Kind: KindCommandPhaseFinished, Phase: "final_verification", Status: "completed", ElapsedMS: &elapsed}, "command run: final_verification completed after 0 ms", "INFO"},
		{Event{Kind: KindCommandPhaseFinished, Phase: "summary_publication", Status: "failed", Reason: "denied"}, "command run: summary_publication failed", "WARN"},
		{Event{Kind: KindCommandFinished, Status: "completed", ElapsedMS: &elapsed, ExitCode: &exitCode}, "command run finished with exit 0 after 0 ms", "INFO"},
		{Event{Kind: KindCommandFinished}, "command run finished", "INFO"},
	} {
		t.Run(string(tc.event.Kind)+tc.event.Status, func(t *testing.T) {
			var journal, operator, flow bytes.Buffer
			reporter := NewReporterWith("run", []Sink{NewArgusSink(&journal), NewOperatorSink(slog.New(slog.NewJSONHandler(&operator, nil))), NewFlowSink(&flow)})
			reporter.Emit(tc.event)
			var event Event
			if err := json.Unmarshal(bytes.TrimSpace(journal.Bytes()), &event); err != nil {
				t.Fatal(err)
			}
			if event.Kind != tc.event.Kind || event.RunID != "run" {
				t.Fatalf("journal lost command event: %+v", event)
			}
			if tc.event.ElapsedMS != nil && (event.ElapsedMS == nil || *event.ElapsedMS != 0) || tc.event.ExitCode != nil && (event.ExitCode == nil || *event.ExitCode != 0) {
				t.Fatalf("observed zero omitted: %s", &journal)
			}
			if !strings.Contains(flow.String(), tc.flow) || !strings.Contains(operator.String(), `"level":"`+tc.level+`"`) || !strings.Contains(operator.String(), `"run":"run"`) {
				t.Fatalf("lost command event: flow=%s operator=%s", &flow, &operator)
			}
			if tc.event.Reason != "" && !strings.Contains(operator.String(), `"reason":"denied"`) {
				t.Fatalf("failure reason omitted: %s", &operator)
			}
		})
	}
}
