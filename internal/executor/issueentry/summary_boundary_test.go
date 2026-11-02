package issueentry

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestSummaryClosedCodecRejectsInvalidCountersAndStates(t *testing.T) {
	base := (&Collector{}).Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "", "review_pending", 12)
	zero := int64(0)
	for _, counter := range []Counter{
		{State: "unknown"},
		{State: "observed"},
		{State: "observed", Value: &zero, Reason: "invented"},
		{State: "partial", Value: &zero},
		{State: "unavailable", Value: &zero, Reason: "missing"},
	} {
		bad := base
		bad.TotalTokens = counter
		if b, err := json.Marshal(bad); err != nil {
			t.Fatal(err)
		} else if _, err := DecodeSummary(b); err == nil {
			t.Fatalf("accepted invalid counter %+v", counter)
		}
	}
	attempt := Attempt{ID: "attempt", Role: "implementation", Harness: "codex", Status: "running", Tokens: Unavailable("missing"), Turns: Unavailable("missing"), ToolCalls: Unavailable("missing")}
	for _, tc := range []struct {
		name   string
		change func(*Summary)
	}{
		{"wrong schema", func(s *Summary) { s.Schema = "future/v9" }},
		{"unknown terminal", func(s *Summary) { s.Terminal = "success-by-exit-code" }},
		{"unknown attempt role", func(s *Summary) { s.Attempts[0].Role = "intake-model" }},
		{"empty attempt route", func(s *Summary) { s.Attempts[0].Harness = "" }},
		{"duplicate attempt", func(s *Summary) { s.Attempts = append(s.Attempts, s.Attempts[0]) }},
		{"unknown attempt status", func(s *Summary) { s.Attempts[0].Status = "verified" }},
		{"invalid attempt counter", func(s *Summary) { s.Attempts[0].Tokens = Counter{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.Attempts = []Attempt{attempt}
			tc.change(&s)
			if _, err := MergeSummary("authored body", s); err == nil {
				t.Fatal("rendered an invalid schema/state")
			}
		})
	}
}

func TestSummaryUsesPartialAndUnavailableNativeUsageWithoutInventingZeros(t *testing.T) {
	c := &Collector{}
	c.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "model|native\nversion"})(port.RunResult{Tokens: 10, Turns: 1}, errors.New("interrupted"))
	c.Launched("claude", port.LaunchPurposeReview, port.LaunchCapture{})(port.RunResult{Tokens: -1, Turns: -1, TokensObserved: true, TurnsObserved: true}, nil)
	s := c.Summary(Contract{ID: "sha256:contract"}, "carina:opaque#17", 77, "", "review_pending", 12)
	if s.Attempts[0].Tokens.State != "partial" || s.Attempts[0].Turns.State != "partial" || s.Attempts[0].Status != "failed" || s.TotalTokens.State != "partial" || s.Attempts[1].Tokens.State != "unavailable" {
		t.Fatalf("invented complete native usage %+v", s)
	}
	rendered, err := MergeSummary("authored body", s)
	if err != nil || !strings.Contains(rendered, "10 (partial)") || !strings.Contains(rendered, "model\\|native version") || !strings.Contains(rendered, "| unavailable |") {
		t.Fatalf("untruthful readable attribution %s %v", rendered, err)
	}
}

func TestSummaryRetentionRefusesCorruptAndConflictingLaunchFacts(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	collector := &Collector{RunID: "host-run/"}
	collector.Launched("codex", port.LaunchPurposeImplementation, port.LaunchCapture{NativeModel: "native", NativeEffort: "high"})(port.RunResult{Tokens: 10}, nil)
	s := collector.Summary(c, "https://github.com/o/r/issues/17", 77, "", "review_pending", 12)
	for _, bad := range []Summary{{}, func() Summary { changed := s; changed.ContractID = "different"; return changed }()} {
		if _, err := store.RetainSummary(t.Context(), c, bad); err == nil {
			t.Fatal("retained invalid summary")
		}
	}
	badContract := c
	badContract.ID = "changed"
	if _, err := store.RetainSummary(t.Context(), badContract, s); err == nil {
		t.Fatal("retained observations against invalid contract")
	}
	if _, err := store.RetainSummary(t.Context(), c, s); err != nil {
		t.Fatal(err)
	}
	changed := s
	changed.Attempts = append([]Attempt{}, s.Attempts...)
	changed.Attempts[0].Model = "other"
	if _, err := fresh.RetainSummary(t.Context(), c, changed); err == nil {
		t.Fatal("rebound an observed launch")
	}
	path, err := contractPath(c, "summaries")
	if err != nil {
		t.Fatal(err)
	}
	replaceEvidence(t, store, path+".json", []byte("broken"))
	if _, err := fresh.RetainSummary(t.Context(), c, s); err == nil {
		t.Fatal("replaced corrupt retained observations")
	}
}
