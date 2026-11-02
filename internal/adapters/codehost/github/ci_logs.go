package ghcodehost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type failedLogClient interface {
	failedLogs(context.Context, string) (string, error)
}

// FetchFailedLogs implements the optional self-heal diagnostic seam. Raw logs
// remain host data; the drive redacts and bounds them before native inference.
func (h *Host) FetchFailedLogs(ctx context.Context, proposal port.Proposal) (string, error) {
	head, err := h.gh.pullHeadSHA(ctx, proposal.Number)
	if err != nil {
		return "", fmt.Errorf("ghcodehost: failed-log head: %w", err)
	}
	if head == "" {
		return "", errors.New("ghcodehost: failed-log head is empty")
	}
	client, ok := h.gh.(failedLogClient)
	if !ok {
		return "", errors.New("ghcodehost: failed-log retrieval unavailable")
	}
	logs, err := client.failedLogs(ctx, head)
	if err != nil {
		return "", fmt.Errorf("ghcodehost: failed logs at %s: %w", head, err)
	}
	current, err := h.gh.pullHeadSHA(ctx, proposal.Number)
	if err != nil {
		return "", fmt.Errorf("ghcodehost: recheck failed-log head: %w", err)
	}
	if current != head {
		return "", errors.New("ghcodehost: proposal head changed while fetching failed logs")
	}
	return logs, nil
}

type workflowLogRun struct {
	ID         int64  `json:"id"`
	WorkflowID int64  `json:"workflow_id"`
	Event      string `json:"event"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

func (c *ghClient) failedLogs(ctx context.Context, head string) (string, error) {
	path := fmt.Sprintf("repos/%s/actions/runs?head_sha=%s&per_page=100", c.repo, url.QueryEscape(head))
	out, err := c.run(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, path)
	if err != nil {
		return "", fmt.Errorf("list exact-head workflow runs: %w", err)
	}
	type workflowLogKey struct {
		workflowID int64
		event      string
	}
	latest := make(map[workflowLogKey]workflowLogRun)
	decoder := json.NewDecoder(bytes.NewReader(out))
	havePage := false
	for {
		var page struct {
			Runs []workflowLogRun `json:"workflow_runs"`
		}
		if err := decoder.Decode(&page); errors.Is(err, io.EOF) {
			if !havePage {
				return "", errors.New("empty workflow-run response")
			}
			break
		} else if err != nil {
			return "", fmt.Errorf("decode workflow runs: %w", err)
		}
		if page.Runs == nil {
			return "", errors.New("missing workflow_runs in log response")
		}
		havePage = true
		for _, run := range page.Runs {
			if run.ID <= 0 || run.WorkflowID <= 0 || strings.TrimSpace(run.Event) == "" || run.HeadSHA != head {
				return "", errors.New("invalid or foreign-head workflow log identity")
			}
			// A pending retry has no terminal diagnostic yet. Keep the last
			// completed outcome until a newer completed run supersedes it.
			if run.Status != "completed" {
				continue
			}
			key := workflowLogKey{workflowID: run.WorkflowID, event: run.Event}
			if prior, ok := latest[key]; !ok || run.ID > prior.ID {
				latest[key] = run
			}
		}
	}
	var failed []int64
	for _, run := range latest {
		switch run.Conclusion {
		case "failure", "timed_out", "cancelled", "action_required", "startup_failure", "stale":
			failed = append(failed, run.ID)
		}
	}
	slices.Sort(failed)
	var logs strings.Builder
	for _, id := range failed {
		header := fmt.Sprintf("Workflow run %d at %s:\n", id, head)
		remaining := maxFailedLogBytes - logs.Len() - len(header) - 1
		if remaining <= 0 {
			return "", errGHLogLimit
		}
		args := []string{"run", "view", strconv.FormatInt(id, 10), "--repo", c.repo, "--log-failed"}
		var out []byte
		var err error
		if c.runLog != nil {
			out, err = c.runLog(ctx, remaining, args...)
		} else {
			out, err = c.run(ctx, args...) // injectable clients still obey the aggregate bound below
		}
		if err != nil {
			if cause := ctx.Err(); cause != nil {
				return "", fmt.Errorf("failed steps of workflow run %d: %w", id, cause)
			}
			// gh refuses aggregate retrieval when its archive requires more
			// than 25 per-job fallback requests. Diagnostics are optional:
			// retain other workflows' logs, or let the fixer reproduce locally.
			// The proposal-head recheck still runs after this fallback.
			// https://cli.github.com/manual/gh_run_view
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) &&
				!errors.Is(err, errGHLogLimit) && strings.Contains(err.Error(), "too many API requests needed to fetch logs;") {
				slog.InfoContext(ctx, "workflow log aggregation unavailable", "workflow_run_id", id, "revision", head)
				continue
			}
			return "", fmt.Errorf("failed steps of workflow run %d: %w", id, err)
		}
		if len(out) > remaining {
			return "", fmt.Errorf("failed steps of workflow run %d: %w", id, errGHLogLimit)
		}
		logs.WriteString(header)
		logs.Write(out)
		logs.WriteByte('\n')
	}
	return logs.String(), nil
}
