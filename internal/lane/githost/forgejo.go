package githost

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/lane/verdict"
)

// NewConfigured selects the code-host binding independently of task tracking.
func NewConfigured(repo, root, remote, mainline string) (*Host, error) {
	cfg, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	binding, err := cfg.CodeHost(repo)
	if err != nil {
		return nil, err
	}
	if binding.Provider == integrations.LocalGit {
		return nil, fmt.Errorf("%w: local Git landing is not available in this build", codehost.ErrWorkflowUnavailable)
	}
	h := New(binding.Repository, root, remote, mainline)
	if binding.Provider == integrations.Forgejo {
		client, err := forgejo.New(binding.BaseURL, binding.Token())
		if err != nil {
			return nil, err
		}
		h.Forgejo = forgejocodehost.New(client, binding.BaseURL, binding.Repository, mainline)
	} else if binding.TokenEnv != "" || cfg.Bindings.CodeHost != (integrations.Binding{}) {
		h.Run = func(ctx context.Context, args ...string) ([]byte, error) {
			return githubauth.RunForHost(ctx, binding.TokenEnv, "github.com", nil, args...)
		}
	}
	return h, nil
}

func proposalID(value string) (port.Proposal, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return port.Proposal{}, errors.New("lane: invalid proposal number")
	}
	return port.Proposal{Number: n}, nil
}
func (h *Host) forgejoHead(ctx context.Context, proposal string) (string, error) {
	p, err := proposalID(proposal)
	if err != nil {
		return "", err
	}
	return h.Forgejo.ProposalHead(ctx, p)
}
func (h *Host) forgejoGreen(ctx context.Context, proposal, head string) (bool, error) {
	p, err := proposalID(proposal)
	if err != nil {
		return false, err
	}
	current, err := h.Forgejo.ProposalHead(ctx, p)
	if err != nil || current != head {
		return false, err
	}
	state, err := h.Forgejo.Checks(ctx, p)
	if err != nil {
		return false, err
	}
	current, err = h.Forgejo.ProposalHead(ctx, p)
	return err == nil && current == head && state == port.CheckGreen, err
}
func (h *Host) forgejoReviewVerdict(ctx context.Context, proposal, head string) (verdict.CheckState, error) {
	result := verdict.CheckState{Context: verdict.CheckContext, Status: verdict.NonApproving, Reason: "no published review-verdict status for this head"}
	current, err := h.forgejoHead(ctx, proposal)
	if err != nil || current != head {
		return result, err
	}
	var rows []struct {
		ID          int64  `json:"id"`
		Context     string `json:"context"`
		Status      string `json:"status"`
		Description string `json:"description"`
	}
	if err := h.Forgejo.Client.List(ctx, forgejo.RepoPath(h.Repo, "commits/"+url.PathEscape(head)+"/statuses"), &rows); err != nil {
		return result, err
	}
	var latest int64
	for _, r := range rows {
		if r.Context == verdict.CheckContext && r.ID > latest {
			latest = r.ID
			result.Status = verdict.NonApproving
			result.Reason = r.Description
			if r.Status == "success" {
				result.Status = verdict.Approving
			}
		}
	}
	current, err = h.forgejoHead(ctx, proposal)
	if err != nil {
		return verdict.CheckState{}, err
	}
	if current != head {
		return verdict.CheckState{Context: verdict.CheckContext, Status: verdict.NonApproving, Reason: "proposal head moved while reading review verdict"}, nil
	}
	return result, nil
}
func (h *Host) forgejoMerged(ctx context.Context, proposal, head string) (bool, error) {
	p, err := proposalID(proposal)
	if err != nil {
		return false, err
	}
	pr, err := h.Forgejo.Pull(ctx, p.Number)
	if err != nil {
		return false, err
	}
	if !pr.Merged || pr.MergeCommitSHA == "" || pr.Head.SHA != head {
		return false, nil
	}
	if _, err := h.git(ctx, "fetch", h.remote(), h.mainline(), "--quiet"); err != nil {
		return false, fmt.Errorf("fetch mainline: %w", err)
	}
	if _, err := h.git(ctx, "merge-base", "--is-ancestor", pr.MergeCommitSHA, h.remote()+"/"+h.mainline()); err != nil {
		return false, nil
	}
	return true, nil
}
func (h *Host) forgejoProposals(ctx context.Context) ([]Proposal, error) {
	rows, err := h.Forgejo.Pulls(ctx, "open")
	if err != nil {
		return nil, err
	}
	result := make([]Proposal, 0, len(rows))
	for _, p := range rows {
		result = append(result, Proposal{Number: p.Number, Head: p.Head.SHA})
	}
	return result, nil
}
func (h *Host) forgejoBlock(ctx context.Context, proposal string) (int64, []byte, error) {
	if _, err := proposalID(proposal); err != nil {
		return 0, nil, err
	}
	actor, err := h.Forgejo.Client.CurrentUser(ctx)
	if err != nil {
		return 0, nil, err
	}
	var rows []struct {
		ID             int64        `json:"id"`
		Body           string       `json:"body"`
		User           forgejo.User `json:"user"`
		OriginalAuthor string       `json:"original_author"`
	}
	if err := h.Forgejo.Client.List(ctx, forgejo.RepoPath(h.Repo, "issues/"+proposal+"/comments"), &rows); err != nil {
		return 0, nil, err
	}
	var block []byte
	var latest int64
	for _, r := range rows {
		if r.ID > latest && r.User.ID == actor.ID && r.OriginalAuthor == "" && strings.HasPrefix(r.Body, BlockMarker) {
			latest = r.ID
			block = extractBlock(r.Body)
		}
	}
	return latest, block, nil
}
func (h *Host) forgejoRecord(ctx context.Context, proposal string, record verdict.Record) error {
	p, err := proposalID(proposal)
	if err != nil {
		return err
	}
	// Each evaluation is its own immutable comment; concurrent evaluations do
	// not overwrite one another's read-modify-write JSONL history.
	body := BlockMarker + "\n```json\n" + strings.TrimSpace(string(verdict.AppendBlock(nil, record))) + "\n```"
	_, err = h.Forgejo.PostComment(ctx, p.Number, body)
	return err
}
func (m *Machine) forgejoAct(ctx context.Context, proposal, head string) error {
	p, err := proposalID(proposal)
	if err != nil {
		return err
	}
	actor, err := m.Host.Forgejo.Client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	if actor.Login != m.Name {
		return errors.New("lane: authenticated Forgejo actor differs from machine identity")
	}
	current, err := m.Host.Forgejo.ProposalHead(ctx, p)
	if err != nil {
		return err
	}
	if current != head {
		return errors.New("lane: proposal head moved before approval")
	}
	if err := m.Host.Forgejo.Approve(ctx, p.Number, head); err != nil {
		return err
	}
	return m.Host.Forgejo.Merge(ctx, p.Number, head)
}
