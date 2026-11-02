package forgejocodehost

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func logArchive(t *testing.T, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("test.log")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestFailedLogsUseLatestWorkflowRunAtExactHead(t *testing.T) {
	archive := logArchive(t, "FAIL TestBehavior: wrong result")
	reads := 0
	h := testHost(t, func(r *http.Request) (any, int) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pulls/7"):
			return pullFixture(), 200
		case strings.HasSuffix(r.URL.Path, "/actions/runs"):
			return map[string]any{"total_count": 3, "workflow_runs": []actionRun{{ID: 1, CommitSHA: "abc", WorkflowID: "test", Event: "push", Status: "failure"}, {ID: 2, CommitSHA: "abc", WorkflowID: "test", Event: "push", Status: "success"}, {ID: 3, CommitSHA: "abc", WorkflowID: "build", Event: "push", Status: "failure"}}}, 200
		case strings.HasSuffix(r.URL.Path, "/runs/3/logs"):
			reads++
			return rawBody(archive), 200
		default:
			t.Fatalf("unexpected API %s", r.URL)
			return nil, 500
		}
	})
	logs, err := h.FetchFailedLogs(t.Context(), port.Proposal{Number: 7})
	if err != nil || reads != 1 || !strings.Contains(logs, "FAIL TestBehavior") || !strings.Contains(logs, "at abc") {
		t.Fatalf("logs=%s reads=%d err=%v", logs, reads, err)
	}
}

func TestLogReadFailureNeverReturnsPartialSuccess(t *testing.T) {
	archive := logArchive(t, "failure")
	for _, fault := range []string{"head", "runs", "count", "identity", "revision", "duplicate", "incomplete", "logs", "archive", "compressed bound", "expanded bound", "head moved", "final read"} {
		t.Run(fault, func(t *testing.T) {
			reads := 0
			h := testHost(t, func(r *http.Request) (any, int) {
				if strings.HasSuffix(r.URL.Path, "/pulls/7") {
					reads++
					if fault == "head" || fault == "final read" && reads > 1 {
						return nil, 503
					}
					p := pullFixture()
					if fault == "head moved" && reads > 1 {
						p.Head.SHA = "def"
					}
					return p, 200
				}
				if strings.HasSuffix(r.URL.Path, "/actions/runs") {
					if fault == "runs" {
						return nil, 503
					}
					if fault == "count" {
						return struct{}{}, 200
					}
					run := actionRun{ID: 1, CommitSHA: "abc", WorkflowID: "test", Event: "push", Status: "failure"}
					if fault == "identity" {
						run.ID = 0
					}
					if fault == "revision" {
						run.CommitSHA = "def"
					}
					rows := []actionRun{run}
					total := 1
					if fault == "duplicate" {
						rows = append(rows, run)
						total = 2
					}
					if fault == "incomplete" {
						rows = nil
					}
					return map[string]any{"total_count": total, "workflow_runs": rows}, 200
				}
				switch fault {
				case "logs":
					return nil, 503
				case "archive":
					return rawBody("not zip"), 200
				case "compressed bound":
					return rawBody(make([]byte, (8<<20)+1)), 200
				case "expanded bound":
					return rawBody(logArchive(t, strings.Repeat("x", (4<<20)+1))), 200
				default:
					return rawBody(archive), 200
				}
			})
			if logs, err := h.FetchFailedLogs(t.Context(), port.Proposal{Number: 7}); err == nil || logs != "" {
				t.Fatalf("partial logs=%q err=%v", logs, err)
			}
		})
	}
}

func TestActionRunPaginationAndBounds(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		t.Run(strconv.FormatBool(incomplete), func(t *testing.T) {
			h := testHost(t, func(r *http.Request) (any, int) {
				page, err := strconv.Atoi(r.URL.Query().Get("page"))
				if err != nil {
					t.Fatal(err)
				}
				total := 2
				if incomplete {
					total = 2000
				}
				return map[string]any{"total_count": total, "workflow_runs": []actionRun{{ID: int64(page), CommitSHA: "abc"}}}, 200
			})
			runs, err := h.actionRuns(t.Context(), "abc")
			if incomplete {
				if err == nil || runs != nil {
					t.Fatal("unbounded listing accepted")
				}
			} else if err != nil || len(runs) != 2 {
				t.Fatalf("pages=%v err=%v", runs, err)
			}
		})
	}
}

func TestLocalStatusReadsAreExactAndLatest(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  port.CheckState
	}{{"success", port.CheckGreen}, {"failure", port.CheckRed}, {"error", port.CheckRed}, {"pending", port.CheckPending}, {"skipped", port.CheckPending}, {"", port.CheckPending}} {
		t.Run(tc.state, func(t *testing.T) {
			h := testHost(t, func(*http.Request) (any, int) {
				return []status{{1, "pyxis/local-gate", "failure"}, {2, "pyxis/local-gate", tc.state}, {3, "other", "success"}}, 200
			})
			got, err := h.ReadStatus(t.Context(), "abc", "pyxis/local-gate")
			if err != nil || got != tc.want {
				t.Fatalf("status=%v %v", got, err)
			}
		})
	}
	h := testHost(t, func(*http.Request) (any, int) { return nil, 503 })
	if _, err := h.ReadStatus(t.Context(), "abc", "build"); err == nil {
		t.Fatal("status outage swallowed")
	}
}
