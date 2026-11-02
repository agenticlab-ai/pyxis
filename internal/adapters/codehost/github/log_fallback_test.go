package ghcodehost

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

const bulkLogLimitMessage = "too many API requests needed to fetch logs; try narrowing down to a specific job with the `--job` option"

func TestFailedLogBulkLimitPreservesAvailableDiagnostics(t *testing.T) {
	for _, unavailable := range []string{"10", "20", "both"} {
		t.Run(unavailable, func(t *testing.T) {
			c := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) {
				return []byte(`{"workflow_runs":[{"id":10,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"},{"id":20,"workflow_id":2,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`), nil
			}, runLog: func(_ context.Context, limit int, args ...string) ([]byte, error) {
				if limit <= 0 || limit > maxFailedLogBytes {
					t.Fatalf("unbounded fallback request: %d", limit)
				}
				if unavailable == "both" || args[2] == unavailable {
					return nil, fmt.Errorf("GitHub failed-log command: exit status 1: %s", bulkLogLimitMessage)
				}
				return []byte("available failure " + args[2]), nil
			}}
			logs, err := c.failedLogs(t.Context(), "head")
			if err != nil {
				t.Fatalf("optional bulk-log limit blocked reproduction: %v", err)
			}
			if unavailable == "both" {
				if logs != "" {
					t.Fatalf("fabricated missing diagnostic: %q", logs)
				}
			} else if strings.Contains(logs, "available failure "+unavailable) || !strings.Contains(logs, "available failure "+map[string]string{"10": "20", "20": "10"}[unavailable]) {
				t.Fatalf("discarded another workflow's diagnostic: %q", logs)
			}
		})
	}
}

func TestFailedLogBulkLimitStillFencesHeadAndOperationalFailures(t *testing.T) {
	for _, scenario := range []string{"stable", "moved", "cancelled", "deadline", "overflow", "other failure", "cancelled context"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			reads := 0
			cause := errors.New(bulkLogLimitMessage)
			switch scenario {
			case "cancelled":
				cause = fmt.Errorf("%s: %w", bulkLogLimitMessage, context.Canceled)
			case "deadline":
				cause = fmt.Errorf("%s: %w", bulkLogLimitMessage, context.DeadlineExceeded)
			case "overflow":
				cause = fmt.Errorf("%s: %w", bulkLogLimitMessage, errGHLogLimit)
			case "other failure":
				cause = errors.New("credentials unavailable")
			}
			c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
				if strings.Contains(args[len(args)-1], "/actions/runs?") {
					return []byte(`{"workflow_runs":[{"id":10,"workflow_id":1,"event":"pull_request","head_sha":"head","status":"completed","conclusion":"failure"}]}`), nil
				}
				reads++
				if reads == 2 && scenario == "moved" {
					return []byte(`{"state":"open","head":{"sha":"other"}}`), nil
				}
				return []byte(`{"state":"open","head":{"sha":"head"}}`), nil
			}, runLog: func(context.Context, int, ...string) ([]byte, error) {
				if scenario == "cancelled context" {
					cancel()
				}
				return nil, cause
			}}
			logs, err := newHost(c, "main").FetchFailedLogs(ctx, port.Proposal{Number: 77})
			if scenario == "stable" {
				if err != nil || logs != "" || reads != 2 {
					t.Fatalf("fallback skipped the head recheck: logs=%q reads=%d err=%v", logs, reads, err)
				}
			} else if err == nil || logs != "" {
				t.Fatalf("unsafe fallback: logs=%q err=%v", logs, err)
			} else if scenario == "cancelled context" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost context cancellation: %v", err)
			} else if scenario != "moved" && scenario != "cancelled context" && !errors.Is(err, cause) {
				t.Fatalf("lost operational failure: %v", err)
			}
		})
	}
}
