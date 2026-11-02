package ghcodehost

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type commitStatus struct {
	ID      int64  `json:"id"`
	Context string `json:"context"`
	State   string `json:"state"`
}

type statusClient interface {
	commitStatuses(context.Context, string) ([]commitStatus, error)
	setStatus(context.Context, string, string, string, string, string) error
}

// SetStatus publishes one exact-commit local verification result. Writes use
// the same selected credential and shared content budget as proposal writes.
func (h *Host) SetStatus(ctx context.Context, sha, name, state, target, description string) error {
	if err := validateStatus(sha, name, state); err != nil {
		return err
	}
	c, ok := h.gh.(statusClient)
	if !ok {
		return fmt.Errorf("ghcodehost: commit status publication is unavailable")
	}
	return c.setStatus(ctx, sha, name, state, target, description)
}

// ReadStatus reads the newest status for one context at exactly sha. Absent
// or unknown contexts remain pending.
func (h *Host) ReadStatus(ctx context.Context, sha, name string) (port.CheckState, error) {
	if err := validateStatus(sha, name, "pending"); err != nil {
		return port.CheckPending, err
	}
	c, ok := h.gh.(statusClient)
	if !ok {
		return port.CheckPending, fmt.Errorf("ghcodehost: commit status reads are unavailable")
	}
	statuses, err := c.commitStatuses(ctx, sha)
	if err != nil {
		return port.CheckPending, err
	}
	for _, status := range latestStatuses(statuses) {
		if status.Context == name {
			return statusState(status.State), nil
		}
	}
	return port.CheckPending, nil
}

func validateStatus(sha, name, state string) error {
	if len(sha) != 40 && len(sha) != 64 {
		return fmt.Errorf("ghcodehost: commit status requires a full commit SHA")
	}
	if _, err := hex.DecodeString(sha); err != nil {
		return fmt.Errorf("ghcodehost: commit status requires a hexadecimal SHA")
	}
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\r\n") {
		return fmt.Errorf("ghcodehost: commit status requires a context name")
	}
	switch state {
	case "pending", "success", "failure", "error":
		return nil
	default:
		return fmt.Errorf("ghcodehost: invalid commit status state %q", state)
	}
}

func (c *ghClient) setStatus(ctx context.Context, sha, name, state, target, description string) error {
	if c.budgetErr != nil {
		return fmt.Errorf("reserve status write: %w", c.budgetErr)
	}
	if c.budget != nil {
		if err := c.budget.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("reserve status write: %w", err)
		}
	}
	if c.runInput == nil {
		return fmt.Errorf("ghcodehost: status writer has no stdin transport")
	}
	payload, err := json.Marshal(struct {
		State       string `json:"state"`
		Context     string `json:"context"`
		TargetURL   string `json:"target_url,omitempty"`
		Description string `json:"description,omitempty"`
	}{state, name, target, description})
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	_, err = c.runInput(ctx, payload, "api", "--method", "POST", "repos/"+c.repo+"/statuses/"+url.PathEscape(sha), "--input", "-")
	if err != nil {
		return fmt.Errorf("publish commit status: %w", err)
	}
	return nil
}

func (c *ghClient) commitStatuses(ctx context.Context, sha string) ([]commitStatus, error) {
	out, err := c.run(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, "repos/"+c.repo+"/commits/"+url.PathEscape(sha)+"/statuses?per_page=100")
	if err != nil {
		return nil, fmt.Errorf("read commit statuses: %w", err)
	}
	var statuses []commitStatus
	d := json.NewDecoder(bytes.NewReader(out))
	for d.More() {
		var page []commitStatus
		if err := d.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode commit statuses: %w", err)
		}
		statuses = append(statuses, page...)
	}
	return statuses, nil
}

func latestStatuses(statuses []commitStatus) []commitStatus {
	byName := make(map[string]commitStatus)
	var order []string
	for _, s := range statuses {
		prior, found := byName[s.Context]
		if !found {
			order = append(order, s.Context)
		}
		if !found || s.ID > prior.ID {
			byName[s.Context] = s
		}
	}
	latest := make([]commitStatus, 0, len(order))
	for _, name := range order {
		latest = append(latest, byName[name])
	}
	return latest
}

func statusState(state string) port.CheckState {
	switch strings.ToLower(state) {
	case "success":
		return port.CheckGreen
	case "failure", "error":
		return port.CheckRed
	default:
		return port.CheckPending
	}
}

func (h *Host) checksAt(ctx context.Context, sha string) (port.CheckState, error) {
	runs, err := h.gh.checkRuns(ctx, sha)
	if err != nil {
		return port.CheckPending, err
	}
	contexts := checkContexts(runs)
	if c, ok := h.gh.(statusClient); ok {
		statuses, err := c.commitStatuses(ctx, sha)
		if err != nil {
			return port.CheckPending, err
		}
		for _, status := range latestStatuses(statuses) {
			contexts = append(contexts, port.CheckContext{Name: status.Context, State: statusState(status.State)})
		}
	}
	return reduceChecks(contexts), nil
}
