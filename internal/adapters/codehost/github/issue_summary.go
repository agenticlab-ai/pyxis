package ghcodehost

import (
	"context"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func (h *Host) PublishIssueSummary(ctx context.Context, proposal port.Proposal, summary issueentry.Summary) error {
	c, ok := h.gh.(*ghClient)
	if !ok {
		return fmt.Errorf("GitHub execution summary capability unavailable")
	}
	if summary.Proposal != proposal.Number {
		return fmt.Errorf("execution summary proposal identity mismatch")
	}
	read := func(ctx context.Context) (string, error) {
		body, err := c.pullBody(ctx, proposal.Number)
		if err != nil {
			return "", fmt.Errorf("read execution summary proposal: %w", err)
		}
		return body, nil
	}
	write := func(ctx context.Context, body string) error {
		if err := c.setPullBody(ctx, proposal.Number, body); err != nil {
			return fmt.Errorf("publish execution summary: %w", err)
		}
		return nil
	}
	return issueentry.PublishSummary(ctx, summary, read, write)
}
