package forgejocodehost

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

type nativeComment struct {
	ID             int64        `json:"id"`
	Body           string       `json:"body"`
	User           forgejo.User `json:"user"`
	CreatedAt      time.Time    `json:"created_at"`
	OriginalAuthor string       `json:"original_author"`
}

type nativeReview struct {
	OriginalAuthor string       `json:"original_author"`
	ID             int64        `json:"id"`
	Body           string       `json:"body"`
	User           forgejo.User `json:"user"`
	State          string       `json:"state"`
	CommitID       string       `json:"commit_id"`
	Official       bool         `json:"official"`
	Dismissed      bool         `json:"dismissed"`
	Stale          bool         `json:"stale"`
	SubmittedAt    time.Time    `json:"submitted_at"`
}

func (h *Host) association(ctx context.Context, login string, imported bool, cache map[string]string) (string, error) {
	if imported || login == "" {
		return "", nil
	}
	if value, ok := cache[login]; ok {
		return value, nil
	}
	owner, _, _ := strings.Cut(h.Repository, "/")
	if login == owner {
		cache[login] = "OWNER"
		return "OWNER", nil
	}
	for _, trusted := range h.TrustedActors {
		if trusted == login || trusted == h.BaseURL+"/"+login {
			cache[login] = "COLLABORATOR"
			return "COLLABORATOR", nil
		}
	}
	var permission struct {
		Permission string `json:"permission"`
	}
	err := h.Client.Request(ctx, "GET", h.path("collaborators/"+url.PathEscape(login)+"/permission"), nil, &permission)
	if err != nil {
		var api *forgejo.HTTPError
		if errors.As(err, &api) && (api.Status == 403 || api.Status == 404) {
			cache[login] = ""
			return "", nil
		}
		return "", err
	}
	value := ""
	switch permission.Permission {
	case "admin", "owner", "write":
		value = "COLLABORATOR"
	}
	cache[login] = value
	return value, nil
}

func (h *Host) state(ctx context.Context, ref port.ProposalRef, since port.Cursor) (port.ProposalState, error) {
	if ref.Repo != "" && ref.Repo != h.Repository && ref.Repo != h.Key() {
		return port.ProposalState{}, errors.New("forgejo: proposal belongs to another repository")
	}
	p, err := h.Pull(ctx, ref.Number)
	if err != nil {
		return port.ProposalState{}, err
	}
	l, err := landing(p)
	if err != nil {
		return port.ProposalState{}, err
	}
	checks, missing, rule, err := h.checks(ctx, p)
	if err != nil {
		return port.ProposalState{}, err
	}
	s := port.ProposalState{Landing: l, Checks: checks, RequiredMissing: missing}
	if p.Merged {
		s.LandedRevision = p.MergeCommitSHA
	}
	var comments []nativeComment
	if err = h.Client.List(ctx, h.path(fmt.Sprintf("issues/%d/comments", ref.Number)), &comments); err != nil {
		return port.ProposalState{}, err
	}
	cache := map[string]string{}
	// Trust the API's native user identity, rejecting explicit import markers
	// and missing user IDs. Forgejo v16 omits remapped-import provenance, so
	// those imports cannot be distinguished from native comments or reviews.
	appendComment := func(c nativeComment, surface port.CommentSurface, reviewState string) error {
		if c.ID <= 0 || c.CreatedAt.IsZero() {
			return errors.New("forgejo: malformed comment identity or timestamp")
		}
		id := h.Key() + ":" + strconv.Itoa(ref.Number) + ":" + surface.String() + ":" + strconv.FormatInt(c.ID, 10)
		position := since.Positions[surface]
		if c.CreatedAt.Before(position) || (c.CreatedAt.Equal(position) && (since.NativeIDs == nil || since.NativeIDs[id])) {
			return nil
		}
		association, err := h.association(ctx, c.User.Login, c.OriginalAuthor != "", cache)
		if err != nil {
			return err
		}
		s.Comments = append(s.Comments, port.Comment{NativeID: id, Untrusted: c.OriginalAuthor != "" || c.User.ID <= 0, Author: c.User.Login, AuthorAssociation: association, CreatedAt: c.CreatedAt, Surface: surface, Body: c.Body, ReviewState: reviewState})
		return nil
	}
	for _, c := range comments {
		if err = appendComment(c, port.SurfaceDiscussion, ""); err != nil {
			return port.ProposalState{}, err
		}
	}
	var reviews []nativeReview
	if err = h.Client.List(ctx, h.path(fmt.Sprintf("pulls/%d/reviews", ref.Number)), &reviews); err != nil {
		return port.ProposalState{}, err
	}
	approved := 0
	rejected := false
	requested := false
	for _, r := range reviews {
		if r.State == "PENDING" {
			continue
		}
		if r.Official && !r.Dismissed {
			switch r.State {
			case "APPROVED":
				if !rule.IgnoreStaleApprovals || (!r.Stale && r.CommitID == p.Head.SHA) {
					approved++
				}
			case "REQUEST_CHANGES":
				rejected = true
			case "REQUEST_REVIEW":
				requested = true
			}
		}
		state := r.State
		if r.Dismissed {
			state = "DISMISSED"
		} else {
			switch state {
			case "REQUEST_CHANGES":
				state = port.ReviewChangesRequested
			case "COMMENT":
				state = "COMMENTED"
			}
		}
		if r.State != "REQUEST_REVIEW" {
			if err = appendComment(nativeComment{ID: r.ID, Body: r.Body, User: r.User, CreatedAt: r.SubmittedAt, OriginalAuthor: r.OriginalAuthor}, port.SurfaceReviewSummary, state); err != nil {
				return port.ProposalState{}, err
			}
			var inline []nativeComment
			// Forgejo returns this review's complete inline thread in one read;
			// the endpoint does not implement page/limit or pagination headers.
			if err = h.Client.Request(ctx, "GET", h.path(fmt.Sprintf("pulls/%d/reviews/%d/comments", ref.Number, r.ID)), nil, &inline); err != nil {
				return port.ProposalState{}, err
			}
			for _, c := range inline {
				if err = appendComment(c, port.SurfaceInlineReview, ""); err != nil {
					return port.ProposalState{}, err
				}
			}
		}
	}
	switch {
	case rule.BlockOnRejectedReviews && rejected:
		s.ReviewDecision = port.ReviewChangesRequested
	case approved < rule.RequiredApprovals || (requested && rule.BlockOnOfficialReviewRequests):
		s.ReviewDecision = "REVIEW_REQUIRED"
	case approved > 0:
		s.ReviewDecision = port.ReviewApproved
	}
	// No successful snapshot can combine feedback/checks for different heads.
	after, err := h.ProposalHead(ctx, port.Proposal{Number: ref.Number})
	if err != nil {
		return port.ProposalState{}, err
	}
	if after != p.Head.SHA {
		return port.ProposalState{}, errors.New("forgejo: proposal head moved during state read")
	}
	return s, nil
}

func (h *Host) States(ctx context.Context, refs []port.ProposalRef, since port.Cursor) ([]port.ProposalState, error) {
	out := make([]port.ProposalState, len(refs))
	for i, ref := range refs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		s, err := h.state(ctx, ref, since)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == nil {
			out[i] = s
			continue
		}
		reason := port.UnknownParseDefect
		var api *forgejo.HTTPError
		if errors.Is(err, context.DeadlineExceeded) {
			reason = port.UnknownTimeout
		} else if errors.As(err, &api) {
			if api.Status == 429 {
				reason = port.UnknownRateLimited
			} else if api.Status >= 500 {
				reason = port.UnknownOffline
			}
		} else {
			var ue *url.Error
			if errors.As(err, &ue) {
				reason = port.UnknownOffline
			}
		}
		out[i] = port.ProposalState{Unknown: &reason}
	}
	return out, nil
}

var _ port.ProposalStateReader = (*Host)(nil)
