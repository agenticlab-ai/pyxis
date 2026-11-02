// Package local reads version-bound approval fixtures without a code host.
package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

type Reader struct {
	Path     string
	Operator string
}

type fixture struct {
	Schema       string        `json:"schema"`
	PullRequests []pullRequest `json:"pull_requests"`
}

type pullRequest struct {
	Number  int               `json:"number"`
	Head    string            `json:"head"`
	Reviews []approval.Review `json:"reviews"`
}

// Approval reads the fixture anew on every call, so a test can simulate a
// push or review between observations without replacing the adapter.
func (r Reader) Approval(ctx context.Context, pull int, head string) (approval.Approval, error) {
	if err := ctx.Err(); err != nil {
		return approval.Approval{}, err
	}
	if pull < 1 || r.Path == "" {
		return approval.Approval{}, fmt.Errorf("read local draft approval: pull number and fixture path are required")
	}
	body, err := os.ReadFile(r.Path)
	if err != nil {
		return approval.Approval{}, fmt.Errorf("read approval fixture: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var data fixture
	if err := dec.Decode(&data); err != nil {
		return approval.Approval{}, fmt.Errorf("decode approval fixture: %w", err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return approval.Approval{}, fmt.Errorf("approval fixture has trailing content")
	}
	if data.Schema != "draft-approval-fixture/v1" {
		return approval.Approval{}, fmt.Errorf("approval fixture has unsupported schema %q", data.Schema)
	}
	seen := map[int]bool{}
	var selected *pullRequest
	for _, item := range data.PullRequests {
		if item.Number < 1 || item.Head == "" || seen[item.Number] {
			return approval.Approval{}, fmt.Errorf("approval fixture has invalid or duplicate pull request %d", item.Number)
		}
		seen[item.Number] = true
		if item.Number == pull {
			selected = &item
		}
	}
	if selected == nil || selected.Head != head {
		return approval.Approval{}, approval.ErrNoApproval
	}
	return approval.Select(r.Operator, head, selected.Reviews)
}
