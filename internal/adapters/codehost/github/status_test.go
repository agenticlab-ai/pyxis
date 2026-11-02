package ghcodehost

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

const statusTestSHA = "1234567890abcdef1234567890abcdef12345678"

func TestStatusPublishesExactRevisionOnceAndReadsLatestContext(t *testing.T) {
	writes := 0
	c := &ghClient{repo: "o/r", runInput: func(_ context.Context, body []byte, args ...string) ([]byte, error) {
		writes++
		if !strings.Contains(strings.Join(args, " "), "repos/o/r/statuses/"+statusTestSHA) || !hasArg(args, "--input") {
			t.Fatalf("wrong publication argv %v", args)
		}
		var value struct {
			Context string `json:"context"`
			State   string `json:"state"`
		}
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}
		if value.Context != "pyxis/local-gate" || value.State != "success" {
			t.Fatalf("wrong payload %s", body)
		}
		return []byte(`{"id":3}`), nil
	}, run: func(_ context.Context, args ...string) ([]byte, error) {
		assertFreshRead(t, args)
		if strings.Contains(strings.Join(args, " "), "check-runs") {
			return []byte(`{"check_runs":[{"name":"docs-gate","status":"completed","conclusion":"success"}]}`), nil
		}
		return []byte(`[{"id":3,"context":"pyxis/local-gate","state":"success"},{"id":2,"context":"other","state":"success"}][{"id":1,"context":"pyxis/local-gate","state":"failure"}]`), nil
	}}
	h := newHost(c, "main")
	if err := h.SetStatus(context.Background(), statusTestSHA, "pyxis/local-gate", "success", "", "passed"); err != nil {
		t.Fatal(err)
	}
	if writes != 1 {
		t.Fatalf("writes %d", writes)
	}
	if got, err := h.ReadStatus(context.Background(), statusTestSHA, "pyxis/local-gate"); err != nil || got != port.CheckGreen {
		t.Fatalf("readback %v %v", got, err)
	}
	if got, err := h.ReadStatus(context.Background(), statusTestSHA, "missing"); err != nil || got != port.CheckPending {
		t.Fatalf("missing %v %v", got, err)
	}
	if got, err := h.checksAt(context.Background(), statusTestSHA); err != nil || got != port.CheckGreen {
		t.Fatalf("combined checks %v %v", got, err)
	}
	c.runInput = func(context.Context, []byte, ...string) ([]byte, error) { writes++; return nil, errors.New("HTTP 503") }
	if err := h.SetStatus(context.Background(), statusTestSHA, "pyxis/local-gate", "success", "", ""); err == nil || writes != 2 {
		t.Fatalf("ambiguous write retried: %d %v", writes, err)
	}
}

func TestStatusFailuresRemainPendingOrRed(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  port.CheckState
	}{{"success", port.CheckGreen}, {"failure", port.CheckRed}, {"error", port.CheckRed}, {"pending", port.CheckPending}, {"unknown", port.CheckPending}} {
		if got := statusState(tc.state); got != tc.want {
			t.Fatalf("%s = %v", tc.state, got)
		}
	}
	for _, tc := range []struct{ sha, name, state string }{{"main", "gate", "success"}, {strings.Repeat("z", 40), "gate", "success"}, {statusTestSHA, "", "success"}, {statusTestSHA, "gate", "green"}} {
		if err := validateStatus(tc.sha, tc.name, tc.state); err == nil {
			t.Fatalf("invalid status admitted %#v", tc)
		}
	}
	c := &ghClient{repo: "o/r", budgetErr: errors.New("budget unavailable")}
	h := newHost(c, "main")
	if err := h.SetStatus(context.Background(), statusTestSHA, "gate", "success", "", ""); err == nil {
		t.Fatal("write bypassed budget error")
	}
	c.budgetErr = nil
	if err := h.SetStatus(context.Background(), statusTestSHA, "gate", "success", "", ""); err == nil {
		t.Fatal("write without stdin transport")
	}
	c.run = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("offline") }
	if got, err := h.ReadStatus(context.Background(), statusTestSHA, "gate"); got != port.CheckPending || err == nil {
		t.Fatalf("offline green: %v %v", got, err)
	}
	c.run = func(context.Context, ...string) ([]byte, error) { return []byte(`bad json`), nil }
	if got, err := h.ReadStatus(context.Background(), statusTestSHA, "gate"); got != port.CheckPending || err == nil {
		t.Fatalf("malformed green: %v %v", got, err)
	}
	c.run = func(_ context.Context, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "check-runs") {
			return []byte(`{"check_runs":[{"name":"docs-gate","status":"completed","conclusion":"success"}]}`), nil
		}
		return []byte(`[{"id":1,"context":"pyxis/local-gate","state":"failure"}]`), nil
	}
	if got, err := h.checksAt(context.Background(), statusTestSHA); got != port.CheckRed || err != nil {
		t.Fatalf("failed status ignored: %v %v", got, err)
	}
}
