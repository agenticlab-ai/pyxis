package forgejocodehost

import (
	"context"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func (h *Host) PublishIssueSummary(ctx context.Context, proposal port.Proposal, summary issueentry.Summary) error {
	if summary.Proposal != proposal.Number {
		return fmt.Errorf("execution summary proposal identity mismatch")
	}
	read := func(ctx context.Context) (string, error) {
		pr, err := h.Pull(ctx, proposal.Number)
		return pr.Body, err
	}
	write := func(ctx context.Context, body string) error {
		return h.Client.Request(ctx, "PATCH", h.path(fmt.Sprintf("pulls/%d", proposal.Number)), struct {
			Body string `json:"body"`
		}{body}, nil)
	}
	return issueentry.PublishSummary(ctx, summary, read, write)
}
