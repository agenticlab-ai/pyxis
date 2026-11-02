package ghcodehost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type logClientStub struct {
	*fakeClient
	heads      []string
	headErrors []error
	reads      int
	logs       string
	logErr     error
	logHead    string
}

func (c *logClientStub) pullHeadSHA(context.Context, int) (string, error) {
	i := c.reads
	c.reads++
	return c.heads[i], c.headErrors[i]
}

func (c *logClientStub) failedLogs(_ context.Context, head string) (string, error) {
	c.logHead = head
	return c.logs, c.logErr
}

func TestFailedLogsFenceTheProposalHead(t *testing.T) {
	for _, scenario := range []string{"success", "initial read", "empty head", "logs", "final read", "moved"} {
		t.Run(scenario, func(t *testing.T) {
			boom := errors.New("read failed")
			c := &logClientStub{fakeClient: &fakeClient{}, heads: []string{"head", "head"}, headErrors: []error{nil, nil}, logs: "unredacted host diagnostic"}
			switch scenario {
			case "initial read":
				c.headErrors[0] = boom
			case "empty head":
				c.heads[0] = ""
			case "logs":
				c.logErr = boom
			case "final read":
				c.headErrors[1] = boom
			case "moved":
				c.heads[1] = "other"
			}
			logs, err := newHost(c, "main").FetchFailedLogs(t.Context(), port.Proposal{Number: 77})
			if scenario == "success" {
				if err != nil || logs != c.logs || c.logHead != "head" || c.reads != 2 {
					t.Fatalf("logs=%q err=%v client=%+v", logs, err, c)
				}
			} else if err == nil || logs != "" {
				t.Fatalf("unsafe failed-log handoff: %q %v", logs, err)
			}
		})
	}
	if _, err := newHost(&fakeClient{headSHA: "head"}, "main").FetchFailedLogs(t.Context(), port.Proposal{Number: 77}); err == nil {
		t.Fatal("unsupported log client silently became an empty diagnostic")
	}
}

func TestFailedLogsSelectLatestExactHeadWorkflow(t *testing.T) {
	pages := `{"workflow_runs":[{"id":12,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"success"},{"id":11,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},{"id":20,"workflow_id":2,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}
{"workflow_runs":[{"id":21,"workflow_id":3,"event":"pull_request","head_sha":"head","status":"in_progress","conclusion":""},{"id":22,"workflow_id":4,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"timed_out"}]}`
	var ids []string
	c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
		if args[0] == "api" {
			assertFreshRead(t, args)
			if args[len(args)-1] != "repos/o/r/actions/runs?head_sha=head&per_page=100" || !hasArg(args, "--paginate") {
				t.Fatalf("wrong exact-head request: %v", args)
			}
			return []byte(pages), nil
		}
		if strings.Join(args, " ") != "run view "+args[2]+" --repo o/r --log-failed" {
			t.Fatalf("wrong failed-step request: %v", args)
		}
		ids = append(ids, args[2])
		return []byte("failed step " + args[2]), nil
	}}
	logs, err := c.failedLogs(t.Context(), "head")
	if err != nil || strings.Join(ids, ",") != "20,22" || !strings.Contains(logs, "failed step 20") || !strings.Contains(logs, "failed step 22") {
		t.Fatalf("logs=%q ids=%v err=%v", logs, ids, err)
	}
}

func TestFailedLogsRetainCompletedFailureDuringPendingRetry(t *testing.T) {
	for _, status := range []string{"queued", "in_progress"} {
		t.Run(status, func(t *testing.T) {
			payload := fmt.Sprintf(`{"workflow_runs":[{"id":12,"workflow_id":1,"event":"pull_request","head_sha":"head","status":%q,"conclusion":""},{"id":11,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`, status)
			var requested []string
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				if args[0] == "api" {
					return []byte(payload), nil
				}
				requested = append(requested, args[2])
				return []byte("actual completed failure"), nil
			}}
			logs, err := c.failedLogs(t.Context(), "head")
			if err != nil || strings.Join(requested, ",") != "11" || !strings.Contains(logs, "actual completed failure") {
				t.Fatalf("pending retry hid completed diagnostic: logs=%q runs=%v error=%v", logs, requested, err)
			}
		})
	}
}

func TestFailedLogsReadFailuresAndMalformedIdentity(t *testing.T) {
	for _, scenario := range []string{"request", "empty", "decode", "missing", "foreign", "bad id", "bad workflow", "log read", "no failures"} {
		t.Run(scenario, func(t *testing.T) {
			payload := `{"workflow_runs":[{"id":10,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`
			switch scenario {
			case "empty":
				payload = ""
			case "decode":
				payload = "not json"
			case "missing":
				payload = "{}"
			case "foreign":
				payload = strings.ReplaceAll(payload, `"head"`, `"foreign"`)
			case "bad id":
				payload = strings.ReplaceAll(payload, `"id":10`, `"id":0`)
			case "bad workflow":
				payload = strings.ReplaceAll(payload, `"workflow_id":1`, `"workflow_id":0`)
			case "no failures":
				payload = `{"workflow_runs":[]}`
			}
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				if scenario == "request" || (scenario == "log read" && args[0] == "run") {
					return nil, fmt.Errorf("unavailable")
				}
				return []byte(payload), nil
			}}
			logs, err := c.failedLogs(t.Context(), "head")
			if scenario == "no failures" {
				if err != nil || logs != "" {
					t.Fatalf("unexpected failure: %q %v", logs, err)
				}
			} else if err == nil || logs != "" {
				t.Fatalf("bad diagnostic accepted: %q %v", logs, err)
			}
		})
	}
}
