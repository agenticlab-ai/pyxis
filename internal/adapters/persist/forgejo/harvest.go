package forgejopersist

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// ForgejoAdapter harvests review artifacts from Forgejo pull-request comments.
type ForgejoAdapter struct{ client *Client }

var _ harvest.Adapter = (*ForgejoAdapter)(nil)

func NewForgejoAdapter(binding integrations.ResolvedRepository) (*ForgejoAdapter, error) {
	if binding.Provider != integrations.Forgejo {
		return nil, errors.New("harvest forgejo: wrong provider")
	}
	client, err := forgejo.New(binding.BaseURL, binding.Token())
	if err != nil {
		return nil, err
	}
	return &ForgejoAdapter{client: &Client{API: client, Repository: binding}}, nil
}

func (a *ForgejoAdapter) Enumerate(ctx context.Context) ([]harvest.Proposal, error) {
	var rows []struct {
		Number int `json:"number"`
	}
	if err := a.client.API.List(ctx, forgejo.RepoPath(a.client.Repository.Repository, "pulls")+"?state=all", &rows); err != nil {
		return nil, err
	}
	node, err := a.client.RepositoryIdentity(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]harvest.Proposal, 0, len(rows))
	for _, row := range rows {
		ref, err := persist.ProposalRef(a.client.Repository.Key(), row.Number)
		if err != nil {
			return nil, err
		}
		var comments []Comment
		if err := a.client.API.List(ctx, forgejo.RepoPath(a.client.Repository.Repository, fmt.Sprintf("issues/%d/comments", row.Number)), &comments); err != nil {
			return nil, err
		}
		proposal := harvest.Proposal{Ref: ref}
		var invalid *persist.Ref
		var newest int64
		for _, c := range comments {
			if c.ID <= 0 {
				return nil, errors.New("harvest forgejo: missing comment identity")
			}
			body := strings.ReplaceAll(c.Body, "\r\n", "\n")
			artifact := persist.Ref{Medium: persist.MediumForgejo, Locator: "forgejo-comment:" + node + ":" + strconv.FormatInt(c.ID, 10)}
			if parsed, err := persist.ParseArtifact(body); err == nil && parsed.ProposalRef == ref {
				if c.ID > newest {
					newest = c.ID
					proposal.Review = &artifact
					proposal.Absent = false
					proposal.AbsentReason = ""
				}
			} else if harvest.HasExactFence(body, persist.FenceTag) {
				invalid = &artifact
			}
			if reason, absent := harvest.ParseAbsenceMarker(body, ref); absent && proposal.Review == nil {
				proposal.Absent = true
				proposal.AbsentReason = reason
			}
		}
		if proposal.Review == nil && !proposal.Absent && invalid != nil {
			proposal.Review = invalid
		}
		result = append(result, proposal)
	}
	return result, nil
}

func (a *ForgejoAdapter) comment(ctx context.Context, receipt string) (Comment, error) {
	node, err := a.client.RepositoryIdentity(ctx)
	if err != nil {
		return Comment{}, err
	}
	prefix := "forgejo-comment:" + node + ":"
	if !strings.HasPrefix(receipt, prefix) {
		return Comment{}, errors.New("harvest forgejo: receipt belongs to another repository or server")
	}
	return a.client.Read(ctx, strings.TrimPrefix(receipt, prefix))
}

func (a *ForgejoAdapter) Read(ctx context.Context, ref persist.Ref) (*report.Report, error) {
	if ref.Medium != persist.MediumForgejo {
		return nil, errors.New("harvest forgejo: reference belongs to another medium")
	}
	c, err := a.comment(ctx, ref.Locator)
	if err != nil {
		return nil, err
	}
	return persist.ParseArtifact(strings.ReplaceAll(c.Body, "\r\n", "\n"))
}

func (a *ForgejoAdapter) AuthorOf(ctx context.Context, receipt string) (string, bool, error) {
	c, err := a.comment(ctx, receipt)
	if err != nil {
		var apiErr *forgejo.HTTPError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return "", false, nil
		}
		return "", false, err
	}
	if c.OriginalAuthor != "" || c.User.ID <= 0 || c.User.Login == "" {
		return "", false, nil
	}
	return a.client.Repository.BaseURL + "/" + c.User.Login, true, nil
}
