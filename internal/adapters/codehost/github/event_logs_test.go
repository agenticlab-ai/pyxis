package ghcodehost

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestFailedLogsKeepDistinctTriggerFailuresWhileChecksAreRed(t *testing.T) {
	for _, pushStatus := range []string{"completed", "in_progress"} {
		t.Run(pushStatus, func(t *testing.T) {
			pushConclusion := ""
			if pushStatus == "completed" {
				pushConclusion = "success"
			}
			payload := fmt.Sprintf(`{"workflow_runs":[
{"id":10,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},
{"id":12,"workflow_id":1,"event":"push","head_sha":"head","status":%q,"conclusion":%q},
{"id":11,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},
{"id":20,"workflow_id":2,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},
{"id":22,"workflow_id":2,"event":"push","head_sha":"head","status":"completed","conclusion":"failure"},
{"id":30,"workflow_id":3,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},
{"id":31,"workflow_id":3,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"success"}]}`, pushStatus, pushConclusion)
			var requested []string
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				endpoint := args[len(args)-1]
				switch {
				case strings.Contains(endpoint, "/pulls/77"):
					return []byte(`{"state":"open","head":{"sha":"head"}}`), nil
				case strings.Contains(endpoint, "/check-runs?"):
					return fmt.Appendf(nil, `{"check_runs":[{"name":"PR required tests","status":"completed","conclusion":"failure"},{"name":"push smoke","status":%q,"conclusion":%q}]}`, pushStatus, pushConclusion), nil
				case strings.Contains(endpoint, "/statuses?"):
					return []byte(`[]`), nil
				case strings.Contains(endpoint, "/actions/runs?"):
					return []byte(payload), nil
				default:
					t.Fatalf("unexpected API request: %v", args)
					return nil, nil
				}
			}, runLog: func(_ context.Context, _ int, args ...string) ([]byte, error) {
				requested = append(requested, args[2])
				return []byte("actual failed run " + args[2]), nil
			}}
			host := newHost(c, "main")
			if state, err := host.Checks(t.Context(), port.Proposal{Number: 77}); err != nil || state != port.CheckRed {
				t.Fatalf("distinct failed required context was lost: %s, %v", state, err)
			}
			logs, err := host.FetchFailedLogs(t.Context(), port.Proposal{Number: 77})
			if err != nil || strings.Join(requested, ",") != "11,20,22" {
				t.Fatalf("another trigger hid failed required diagnostics: runs=%v logs=%q err=%v", requested, logs, err)
			}
		})
	}
}

func TestFailedLogsRefuseMissingTriggerIdentity(t *testing.T) {
	for _, event := range []string{"", " "} {
		t.Run(fmt.Sprintf("event-%q", event), func(t *testing.T) {
			c := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) {
				return fmt.Appendf(nil, `{"workflow_runs":[{"id":10,"workflow_id":1,"event":%q,"head_sha":"head","status":"completed","conclusion":"failure"}]}`, event), nil
			}, runLog: func(context.Context, int, ...string) ([]byte, error) {
				t.Fatal("ambiguous workflow identity reached log retrieval")
				return nil, nil
			}}
			if logs, err := c.failedLogs(t.Context(), "head"); err == nil || logs != "" {
				t.Fatalf("missing trigger identity was admitted: %q, %v", logs, err)
			}
		})
	}
}
