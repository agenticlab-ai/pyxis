package opencode

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestCoverage90OptionDefaultsAndFenceFailure(t *testing.T) {
	if _, err := New(WithFenceAuthority(nil)); err == nil {
		t.Fatal("New must reject a nil fence authority")
	}
}

func TestCoverage90ClosedLaunchCaptureFailures(t *testing.T) {
	tests := [][]string{
		{"bad", "--format", "json", "--model", "model", "brief"},
		{"run", "--format", "json", "--model", "model", "--agent", "edit", "brief"},
		{"run", "--format", "json", "--model", "model", "   "},
	}
	for _, args := range tests {
		if _, err := captureLaunch(args); err == nil {
			t.Fatalf("captureLaunch(%q) must fail closed", args)
		}
	}

	a := mustAdapter(t)
	if _, err := a.LaunchCapture(foreignHandle{}); err == nil {
		t.Fatal("LaunchCapture must reject a foreign handle")
	}
}

func TestCoverage90KillFailures(t *testing.T) {
	want := errors.New("kill failed")
	a := mustAdapter(t)

	if _, err := a.Await(context.Background(), &handle{proc: &fakeProcess{
		stdout:  []byte(resultJSON),
		killErr: want,
	}}); !errors.Is(err, want) {
		t.Fatalf("Await error = %v, want wrapped kill failure", err)
	}

	if err := a.Kill(context.Background(), &handle{proc: &fakeProcess{killErr: want}}); !errors.Is(err, want) {
		t.Fatalf("Kill error = %v, want wrapped kill failure", err)
	}
}

func TestCoverage90ParseResultBoundaries(t *testing.T) {
	stream := `{"type":"metadata","text":"draft"}` + "\n \n" +
		`{"type":"step_finish","part":{"type":"step-finish","tokens":{"input":1}}}`
	if got, err := parseResult([]byte(stream)); err != nil || got != (port.RunResult{Tokens: 1, Turns: 1, Output: "draft", TokensObserved: true, TurnsObserved: true, Incomplete: true, Reason: "opencode_trace_incomplete: no terminal stop event"}) {
		t.Fatalf("parseResult with text and blank line = %+v, %v", got, err)
	}

	got, err := parseResult([]byte(`{"type":"error","status":"failed"}`))
	if err != nil || !got.Incomplete || got.Reason != "failed" {
		t.Fatalf("parseResult status failure = %+v, %v", got, err)
	}

	if _, err := parseResult([]byte(`{"type":"metadata"}`)); err == nil || !strings.Contains(err.Error(), "no completed step") {
		t.Fatalf("parseResult error = %v, want missing-step failure", err)
	}
}
