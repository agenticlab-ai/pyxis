package codex

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func TestCoverage90OptionDefaultsAndFenceFailure(t *testing.T) {
	if _, err := New(WithFenceAuthority(nil)); err == nil {
		t.Fatal("New must reject a nil fence authority")
	}
}

func TestCoverage90KillFailures(t *testing.T) {
	want := errors.New("kill failed")
	a := mustAdapter(t)

	if _, err := a.Await(context.Background(), &handle{proc: &fakeProcess{
		stdout:  []byte(streamJSONL),
		killErr: want,
	}}); !errors.Is(err, want) {
		t.Fatalf("Await error = %v, want wrapped kill failure", err)
	}

	if err := a.Kill(context.Background(), &handle{proc: &fakeProcess{killErr: want}}); !errors.Is(err, want) {
		t.Fatalf("Kill error = %v, want wrapped kill failure", err)
	}
}

func TestCoverage90ParseResultBoundaries(t *testing.T) {
	stream := `{"type":"turn.started"}` + "\n \n" +
		`{"type":"turn.completed","usage":{"total_tokens":1}}`
	if got, err := parseResult([]byte(stream)); err != nil || got != (port.RunResult{Tokens: 1, Turns: 1, TokensObserved: true, TurnsObserved: true}) {
		t.Fatalf("parseResult with an internal blank line = %+v, %v", got, err)
	}

	if _, err := parseResult([]byte(`{"type":"turn.started"}`)); err == nil || !strings.Contains(err.Error(), "no completed turn") {
		t.Fatalf("parseResult error = %v, want missing-turn failure", err)
	}
}

func TestCoverage90DefaultAuthStoreLocation(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", t.TempDir())
	if authStoreExists() {
		t.Fatal("a fresh default Codex home must not report an auth store")
	}
}

func TestCoverage90CaptureNativeLaunchForBothRoles(t *testing.T) {
	a := mustAdapter(t)
	for _, role := range []nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview} {
		capture, err := a.CaptureNativeLaunch(port.NativeLaunchSpec{
			Role:         role,
			NativeModel:  "gpt-5.6-sol",
			NativeEffort: "high",
		})
		if err != nil {
			t.Fatalf("CaptureNativeLaunch(%s): %v", role, err)
		}
		if capture.NativeModel != "gpt-5.6-sol" || capture.NativeEffort != "high" {
			t.Fatalf("CaptureNativeLaunch(%s) = %+v", role, capture)
		}
	}
}
