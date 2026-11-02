package events

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func fixtureEvent(t *testing.T, name string) Event {
	t.Helper()
	event, err := New(name, "widget", "sha256:fixture", time.Date(2026, 9, 25, 1, 2, 3, 4, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	switch name {
	case DraftOpened:
		event.Origin = "new"
	case DraftStepFailed:
		event.Step, event.ErrorClass = "prepare-lock", "transport"
	case RequirementChanged:
		event.Key, event.Requirement, event.RulingAuthor, event.ReasonPresent, event.Problem = "ruling:commit:P-1", "FR-1", "mdarabi", true, "P-1"
	case LockRefused:
		event.UnmetConditions = []string{"unanswered:P-1", "stale-review:prd"}
	case DraftReopened:
		event.FromVersion, event.Cause, event.CauseRef = 1, "dependency-change", "issue:42"
	}
	return event
}

func TestEventLogRoundTripsAndDeduplicatesByRulingCommit(t *testing.T) {
	var additions []Event
	for _, name := range []string{DraftOpened, DraftStepFailed, RequirementChanged, LockRefused, DraftReopened} {
		additions = append(additions, fixtureEvent(t, name))
	}
	openedAgain := fixtureEvent(t, DraftOpened)
	openedAgain.Origin = "reopen"
	basis := 1
	openedAgain.BasisVersion = &basis
	additions = append(additions, openedAgain)
	log, changed, err := Append([]byte(heading), additions...)
	if err != nil || !changed {
		t.Fatalf("append events changed=%v err=%v", changed, err)
	}
	got, err := Parse(log)
	if err != nil || len(got) != len(additions) {
		t.Fatalf("parsed %d events: %v", len(got), err)
	}
	second := fixtureEvent(t, RequirementChanged)
	if next, changed, err := Append(log, second); err != nil || changed || string(next) != string(log) {
		t.Fatalf("repeated ruling changed=%v err=%v", changed, err)
	}
	if next, changed, err := Append(log, additions[0]); err != nil || changed || string(next) != string(log) {
		t.Fatalf("repeated ID changed=%v err=%v", changed, err)
	}
}

func TestEventValidationAndLogRefusals(t *testing.T) {
	valid := fixtureEvent(t, DraftStepFailed)
	checks := []struct {
		name string
		edit func(*Event)
	}{
		{"schema", func(e *Event) { e.Schema = "wrong" }},
		{"id", func(e *Event) { e.ID = strings.Repeat("z", 24) }},
		{"feature", func(e *Event) { e.Feature = "bad/feature" }},
		{"time", func(e *Event) { e.OccurredAt = "yesterday" }},
		{"unknown", func(e *Event) { e.Name = "surprise" }},
		{"failed step", func(e *Event) { e.Step = "" }},
		{"failed class", func(e *Event) { e.ErrorClass = "" }},
		{"opened origin", func(e *Event) { e.Name = DraftOpened }},
		{"changed ruling", func(e *Event) { e.Name = RequirementChanged }},
		{"refused conditions", func(e *Event) { e.Name = LockRefused }},
		{"reopen cause", func(e *Event) { e.Name = DraftReopened }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			event := valid
			tc.edit(&event)
			if err := event.Validate(); err == nil {
				t.Fatalf("invalid event accepted: %+v", event)
			}
		})
	}
	if _, changed, err := Append(nil, valid); err != nil || !changed {
		t.Fatalf("empty log append changed=%v err=%v", changed, err)
	}
	if _, _, err := Append([]byte("wrong heading\n"), valid); err == nil {
		t.Fatal("invalid heading accepted")
	}
	if _, _, err := Append([]byte(heading), Event{}); err == nil {
		t.Fatal("invalid addition accepted")
	}
	encoded, _ := json.Marshal(valid)
	for _, body := range []string{
		"wrong heading\n",
		heading + "plain text\n",
		heading + "- {\n",
		heading + "- " + string(encoded) + " {}\n",
		heading + "- {}\n",
		heading + "- " + string(encoded) + "\n- " + string(encoded) + "\n",
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Fatalf("invalid log accepted: %q", body)
		}
	}
	if got, err := Parse(nil); err != nil || len(got) != 0 {
		t.Fatalf("empty log = %+v, %v", got, err)
	}
}

func TestCheckedInEventTemplateAcceptsFirstEvent(t *testing.T) {
	base, err := os.ReadFile("../../../.pyxis/templates/draft-events-template.md")
	if err != nil {
		t.Fatal(err)
	}
	if parsed, err := Parse(base); err != nil || len(parsed) != 0 {
		t.Fatalf("checked-in template is not an empty event log: %+v, %v", parsed, err)
	}
	opened := fixtureEvent(t, DraftOpened)
	if body, changed, err := Append(base, opened); err != nil || !changed {
		t.Fatalf("append first event changed=%t err=%v body=%q", changed, err, body)
	}
}
