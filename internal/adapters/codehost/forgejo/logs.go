package forgejocodehost

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type actionRun struct {
	ID         int64  `json:"id"`
	CommitSHA  string `json:"commit_sha"`
	WorkflowID string `json:"workflow_id"`
	Event      string `json:"event"`
	Status     string `json:"status"`
}

func (h *Host) actionRuns(ctx context.Context, sha string) ([]actionRun, error) {
	var result []actionRun
	seen := map[int64]bool{}
	for page := 1; page <= 1000; page++ {
		var response struct {
			Total *int        `json:"total_count"`
			Runs  []actionRun `json:"workflow_runs"`
		}
		endpoint := h.path("actions/runs") + "?head_sha=" + url.QueryEscape(sha) + "&limit=50&page=" + strconv.Itoa(page)
		if err := h.Client.Request(ctx, "GET", endpoint, nil, &response); err != nil {
			return nil, err
		}
		if response.Total == nil || *response.Total < 0 {
			return nil, errors.New("forgejo: action run listing has no count")
		}
		for _, run := range response.Runs {
			if run.ID <= 0 || seen[run.ID] || run.CommitSHA != sha {
				return nil, errors.New("forgejo: action run listing has inconsistent identity or revision")
			}
			seen[run.ID] = true
			result = append(result, run)
		}
		if len(result) >= *response.Total {
			return result, nil
		}
		if len(response.Runs) == 0 {
			return nil, errors.New("forgejo: action run listing is incomplete")
		}
	}
	return nil, errors.New("forgejo: action run pagination exceeded bound")
}

// FetchFailedLogs supplies real, revision-bound Actions logs to the bounded CI
// repair loop. It returns no partial success on an API or archive read failure.
func (h *Host) FetchFailedLogs(ctx context.Context, proposal port.Proposal) (string, error) {
	sha, err := h.ProposalHead(ctx, proposal)
	if err != nil {
		return "", err
	}
	runs, err := h.actionRuns(ctx, sha)
	if err != nil {
		return "", err
	}
	latest := map[string]int64{}
	for _, run := range runs {
		key := run.WorkflowID + "\x00" + run.Event
		if run.ID > latest[key] {
			latest[key] = run.ID
		}
	}
	var result strings.Builder
	for _, run := range runs {
		if latest[run.WorkflowID+"\x00"+run.Event] != run.ID || run.Status != "failure" {
			continue
		}
		response, err := h.Client.Stream(ctx, "GET", h.path(fmt.Sprintf("actions/runs/%d/logs", run.ID)), "", nil)
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			return "", errors.Join(readErr, closeErr)
		}
		if len(data) > 8<<20 {
			return "", errors.New("forgejo: log archive exceeds size bound")
		}
		logs, err := decodeLogs(data)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&result, "Workflow %s at %s (run %d):\n%s\n", run.WorkflowID, sha, run.ID, logs)
		if result.Len() > 4<<20 {
			return "", errors.New("forgejo: combined failed logs exceed size bound")
		}
	}
	after, err := h.ProposalHead(ctx, proposal)
	if err != nil {
		return "", err
	}
	if after != sha {
		return "", errors.New("forgejo: proposal head changed while reading failed logs")
	}
	return result.String(), nil
}

func decodeLogs(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("forgejo: invalid Actions log archive: %w", err)
	}
	var result strings.Builder
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.UncompressedSize64 > 4<<20 {
			return "", errors.New("forgejo: expanded Actions log exceeds size bound")
		}
		r, err := entry.Open()
		if err != nil {
			return "", err
		}
		b, readErr := io.ReadAll(io.LimitReader(r, (4<<20)+1))
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			return "", errors.Join(readErr, closeErr)
		}
		if result.Len()+len(b) > 4<<20 {
			return "", errors.New("forgejo: expanded Actions logs exceed size bound")
		}
		fmt.Fprintf(&result, "%s:\n%s\n", entry.Name, b)
	}
	return result.String(), nil
}
