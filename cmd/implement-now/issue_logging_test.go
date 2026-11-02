package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
)

func TestIssueCommandTimingIncludesPublicationAndObservedExit(t *testing.T) {
	for _, exitCode := range []int{0, 1, 2} {
		t.Run(string(rune('0'+exitCode)), func(t *testing.T) {
			var journal bytes.Buffer
			current := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
			r := &issueRun{contract: issueentry.Contract{ID: "sha256:test"}, task: port.Task{ID: "task", Number: 17}, started: current, now: func() time.Time { return current }, reporter: report.NewReporterWith("command-run", []report.Sink{report.NewArgusSink(&journal)})}
			r.logCommandStarted(current)
			current = current.Add(time.Second)
			verified := r.logPhase("final_verification")
			current = current.Add(2 * time.Second)
			verified(nil)
			published := r.logPhase("summary_publication")
			current = current.Add(3 * time.Second)
			var publicationErr error
			if exitCode != 0 {
				publicationErr = errors.New("publication failed")
			}
			published(publicationErr)
			r.logCommandFinished(exitCode)
			events := decodeCommandJournal(t, journal.Bytes())
			if len(events) != 6 {
				t.Fatalf("events=%+v", events)
			}
			for _, event := range events {
				if event.RunID != "command-run" || event.TaskID != "task" || event.Issue != 17 || event.Scope != r.scope() {
					t.Fatalf("lost command correlation: %+v", event)
				}
			}
			if events[2].ElapsedMS == nil || *events[2].ElapsedMS != 2000 || events[4].ElapsedMS == nil || *events[4].ElapsedMS != 3000 {
				t.Fatalf("stage durations=%+v", events)
			}
			terminal := events[5]
			if terminal.Kind != report.KindCommandFinished || terminal.ExitCode == nil || *terminal.ExitCode != exitCode || terminal.ElapsedMS == nil || *terminal.ElapsedMS != 6000 || !terminal.Time.Equal(current) {
				t.Fatalf("command excluded publication or lost exit: %+v", terminal)
			}
			if exitCode != 0 && (events[4].Status != "failed" || events[4].Reason != "publication failed" || terminal.Status != "failed") {
				t.Fatalf("failure reported completed: %+v", events)
			}
		})
	}
}

func decodeCommandJournal(t *testing.T, data []byte) []report.Event {
	t.Helper()
	var events []report.Event
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var event report.Event
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			return events
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
}

func assertIssueCommandJournal(t *testing.T, path string, exits ...int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events := decodeCommandJournal(t, data)
	command := -1
	var runID string
	phases := map[string]bool{}
	for _, event := range events {
		switch event.Kind {
		case report.KindCommandStarted:
			command++
			runID = event.RunID
			phases = map[string]bool{}
		case report.KindCommandPhaseStarted:
			phases[event.Phase] = false
		case report.KindCommandPhaseFinished:
			if _, started := phases[event.Phase]; !started || event.ElapsedMS == nil {
				t.Fatalf("missing stage start or duration: %+v", event)
			}
			phases[event.Phase] = true
		case report.KindCommandFinished:
			if command >= len(exits) || event.RunID != runID || event.ExitCode == nil || *event.ExitCode != exits[command] || event.ElapsedMS == nil {
				t.Fatalf("missing command terminal/exit: %+v", event)
			}
			for _, phase := range []string{"proposal_reconciliation", "final_verification", "summary_publication"} {
				if !phases[phase] {
					t.Fatalf("command %d excluded %s", command, phase)
				}
			}
		}
	}
	if command+1 != len(exits) || len(events) == 0 || events[len(events)-1].Kind != report.KindCommandFinished {
		t.Fatalf("journal ends before command exit: %+v", events)
	}
}
