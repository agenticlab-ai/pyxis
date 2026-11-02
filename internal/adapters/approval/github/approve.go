package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

type pullBinding struct {
	State string `json:"state"`
	Draft bool   `json:"draft"`
	User  struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		SHA string `json:"sha"`
		Ref string `json:"ref"`
	} `json:"base"`
}

type submittedReview struct {
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
	Body     string `json:"body"`
}

type approvalObservation struct {
	pullBinding
	MainlineBranch string
	PolicyCommit   string
}

func (r *Reader) binding(ctx context.Context, pull int) (pullBinding, error) {
	var p pullBinding
	body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, "repos/"+r.Repo+"/pulls/"+strconv.Itoa(pull))
	if err != nil {
		return p, fmt.Errorf("read approval target: %w", err)
	}
	if err := json.Unmarshal(body, &p); err != nil || !fullCommit(p.Head.SHA) || !fullCommit(p.Base.SHA) || p.Base.Ref == "" || p.User.Login == "" {
		return p, fmt.Errorf("invalid approval target response")
	}
	return p, nil
}

// The host's default branch is the external mainline anchor. A proposed base
// branch cannot grant authority by declaring its own name in configuration.
func (r *Reader) approvalTarget(ctx context.Context, pull int) (approvalObservation, error) {
	p, err := r.binding(ctx, pull)
	if err != nil {
		return approvalObservation{}, err
	}
	body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, "repos/"+r.Repo)
	if err != nil {
		return approvalObservation{}, fmt.Errorf("read trusted mainline branch: %w", err)
	}
	var repository struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &repository); err != nil || strings.TrimSpace(repository.DefaultBranch) == "" {
		return approvalObservation{}, fmt.Errorf("invalid trusted mainline branch response")
	}
	body, err = r.run(ctx, "api", "--method", "GET", "-H", noCache, "repos/"+r.Repo+"/branches/"+url.PathEscape(repository.DefaultBranch))
	if err != nil {
		return approvalObservation{}, fmt.Errorf("read trusted mainline commit: %w", err)
	}
	var branch struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &branch); err != nil || branch.Name != repository.DefaultBranch || !fullCommit(branch.Commit.SHA) {
		return approvalObservation{}, fmt.Errorf("invalid trusted mainline commit response")
	}
	if p.Base.Ref == repository.DefaultBranch && p.Base.SHA != branch.Commit.SHA {
		return approvalObservation{}, fmt.Errorf("approve: mainline moved during target observation")
	}
	return approvalObservation{pullBinding: p, MainlineBranch: branch.Name, PolicyCommit: branch.Commit.SHA}, nil
}

// Approve publishes one host-authenticated driver decision. It does not merge,
// alter protection, change authority, run a reviewer, or impersonate an operator.
// Review evidence is the driver's accountable summary of completed independent
// review; contract readiness and required CI remain their own mandatory gates.
func (r *Reader) Approve(ctx context.Context, d approval.DriverDecision) (approval.Approval, error) {
	if r == nil || r.run == nil || r.write == nil || !repoSlug.MatchString(r.Repo) || d.Repository != r.Repo || d.PullRequest < 1 || !fullCommit(d.Head) {
		return approval.Approval{}, fmt.Errorf("approve: repository, pull, exact head, and host adapter are required")
	}
	pull, err := r.approvalTarget(ctx, d.PullRequest)
	if err != nil {
		return approval.Approval{}, err
	}
	if pull.State != "open" || pull.Draft || pull.Head.SHA != d.Head || d.PolicyCommit != "" && d.PolicyCommit != pull.PolicyCommit {
		return approval.Approval{}, fmt.Errorf("approve: target is not the open, ready PR at the reviewed head and policy version")
	}
	d.PolicyCommit = pull.PolicyCommit
	cfg, authority, err := r.policy(ctx, d.PolicyCommit)
	if err != nil {
		return approval.Approval{}, err
	}
	if cfg.Branch != pull.MainlineBranch || cfg.Approval.ArgusLogin == "" || !approval.DriverGranted(authority) {
		return approval.Approval{}, fmt.Errorf("approve: mainline must configure approval.argus_login and grant Argus approve-merge; candidate and local policy cannot grant it")
	}
	identity, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, "user")
	if err != nil {
		return approval.Approval{}, fmt.Errorf("authenticate driver approval: %w", err)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(identity, &user); err != nil || !strings.EqualFold(user.Login, cfg.Approval.ArgusLogin) {
		return approval.Approval{}, fmt.Errorf("approve: authenticated account does not match the mainline Argus account")
	}
	if strings.HasPrefix(pull.Head.Ref, cfg.Draft.BranchPrefix) {
		if pull.Base.Ref != pull.MainlineBranch {
			return approval.Approval{}, fmt.Errorf("approve: prepared draft must target trusted mainline")
		}
		body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, "repos/"+r.Repo+"/commits/"+d.Head)
		if err != nil {
			return approval.Approval{}, fmt.Errorf("read prepared contract binding: %w", err)
		}
		var commit struct {
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		}
		if err := json.Unmarshal(body, &commit); err != nil {
			return approval.Approval{}, fmt.Errorf("decode prepared contract binding: %w", err)
		}
		lines := strings.Split(commit.Commit.Message, "\n")
		matches := 0
		for _, line := range lines[1:] {
			if value, ok := strings.CutPrefix(line, "Draft-Digest: "); ok {
				if value != d.Digest {
					return approval.Approval{}, fmt.Errorf("approve: reviewed digest differs from prepared head")
				}
				matches++
			}
		}
		if d.Digest == "" || lines[0] != "docs(draft): prepare lock "+d.Drive || matches != 1 {
			return approval.Approval{}, fmt.Errorf("approve: draft PR requires its prepared head and explicit reviewed contract digest")
		}
	} else if d.Digest != "" {
		return approval.Approval{}, fmt.Errorf("approve: a contract digest requires a prepared draft PR")
	}
	body, err := d.Body()
	if err != nil {
		return approval.Approval{}, err
	}
	// Respect the operator's explicit changes request before publishing an act.
	reviews, err := r.reviews(ctx, d.PullRequest)
	if err != nil {
		return approval.Approval{}, err
	}
	var operator *approval.Review
	for i := range reviews {
		review := &reviews[i]
		if strings.EqualFold(review.Author, cfg.Approval.OperatorLogin) && review.ID > 0 && !strings.EqualFold(review.State, "COMMENTED") && !strings.EqualFold(review.State, "PENDING") && (operator == nil || review.ID > operator.ID) {
			operator = review
		}
	}
	if operator != nil && strings.EqualFold(operator.State, "CHANGES_REQUESTED") {
		return approval.Approval{}, fmt.Errorf("approve: operator changes-requested review is still outstanding")
	}
	current, err := r.approvalTarget(ctx, d.PullRequest)
	if err != nil || current != pull {
		return approval.Approval{}, fmt.Errorf("approve: PR or mainline changed before publication")
	}
	event := "APPROVE"
	if strings.EqualFold(user.Login, pull.User.Login) {
		event = "COMMENT"
	}
	request, err := json.Marshal(struct {
		CommitID string `json:"commit_id"`
		Event    string `json:"event"`
		Body     string `json:"body"`
	}{d.Head, event, body})
	if err != nil {
		return approval.Approval{}, fmt.Errorf("encode review request: %w", err)
	}
	path := "repos/" + r.Repo + "/pulls/" + strconv.Itoa(d.PullRequest) + "/reviews"
	published, err := r.write(ctx, request, "api", "--method", "POST", path, "--input", "-")
	if err != nil {
		return approval.Approval{}, fmt.Errorf("publish Argus approval (no automatic retry): %w", err)
	}
	var review submittedReview
	if err := json.Unmarshal(published, &review); err != nil || review.ID < 1 {
		return approval.Approval{}, fmt.Errorf("approval publication returned no review identity; inspect the PR before retrying")
	}
	publishedID := review.ID
	observed, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, path+"/"+strconv.FormatInt(review.ID, 10))
	if err != nil {
		return approval.Approval{}, fmt.Errorf("exact-read published approval: %w", err)
	}
	if err := json.Unmarshal(observed, &review); err != nil {
		return approval.Approval{}, fmt.Errorf("decode published approval: %w", err)
	}
	selected, err := approval.SelectDriver(cfg.Approval, r.Repo, d.PullRequest, d.Head, d.PolicyCommit, d.Digest, true,
		[]approval.Review{{ID: review.ID, Author: review.User.Login, State: review.State, CommitID: review.CommitID, Body: review.Body}})
	if err != nil || review.Body != body || review.ID != publishedID {
		return approval.Approval{}, fmt.Errorf("published approval does not match the authenticated decision")
	}
	current, err = r.approvalTarget(ctx, d.PullRequest)
	if err != nil || current != pull {
		return approval.Approval{}, fmt.Errorf("approval was published for the reviewed head but the PR or mainline changed; review the new version")
	}
	return selected, nil
}
