// Package github reads and publishes authorized exact-head approvals on GitHub.
package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

const noCache = "Cache-Control: no-cache"

var repoSlug = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type Reader struct {
	Repo     string
	Operator string
	run      func(context.Context, ...string) ([]byte, error)
	write    func(context.Context, []byte, ...string) ([]byte, error)
}

// New uses the selected gh credential to observe the code host. The credential
// can be the draft agent's: approval identity comes from GitHub's review data.
func New(repo, operator, tokenEnv string) *Reader {
	return &Reader{Repo: repo, Operator: operator, run: func(ctx context.Context, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, "", nil, args...)
	}, write: func(ctx context.Context, body []byte, args ...string) ([]byte, error) {
		return githubauth.RunForHost(ctx, tokenEnv, "", body, args...)
	}}
}

// NewPolicyReader reads approval identities from the pre-merge mainline, through
// ApprovalFor's explicit policy commit and digest binding. It carries no operator
// of its own, so the identity-free Port method refuses rather than guess one.
func NewPolicyReader(repo, tokenEnv string) *Reader {
	return New(repo, "", tokenEnv)
}

// Approval observes the pull request's head before and after listing reviews.
// A concurrent push therefore cannot make an older approval appear current.
func (r *Reader) Approval(ctx context.Context, pull int, head string) (approval.Approval, error) {
	if r == nil || r.run == nil || !repoSlug.MatchString(r.Repo) || r.Operator == "" || pull < 1 || len(head) != 40 || strings.Trim(head, "0123456789abcdefABCDEF") != "" {
		return approval.Approval{}, fmt.Errorf("read GitHub draft approval: repository, operator, pull, and full head are required")
	}
	current, err := r.pullHead(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	if current != head {
		return approval.Approval{}, approval.ErrNoApproval
	}
	reviews, err := r.reviews(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	current, err = r.pullHead(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	if current != head {
		return approval.Approval{}, approval.ErrNoApproval
	}
	return approval.Select(r.Operator, head, reviews)
}

func (r *Reader) reviews(ctx context.Context, pull int) ([]approval.Review, error) {
	var reviews []approval.Review
	for page := 1; page <= 100; page++ {
		path := "repos/" + r.Repo + "/pulls/" + strconv.Itoa(pull) + "/reviews?per_page=100&page=" + strconv.Itoa(page)
		body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, path)
		if err != nil {
			return nil, fmt.Errorf("read reviews of pull #%d page %d: %w", pull, page, err)
		}
		var pageReviews []struct {
			ID   int64 `json:"id"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			State    string `json:"state"`
			CommitID string `json:"commit_id"`
			Body     string `json:"body"`
		}
		if err := json.Unmarshal(body, &pageReviews); err != nil {
			return nil, fmt.Errorf("decode reviews of pull #%d page %d: %w", pull, page, err)
		}
		for _, item := range pageReviews {
			reviews = append(reviews, approval.Review{ID: item.ID, Author: item.User.Login, State: item.State, CommitID: item.CommitID, Body: item.Body})
		}
		if len(pageReviews) < 100 {
			return reviews, nil
		}
	}
	return nil, fmt.Errorf("pull #%d has more than 10000 reviews; cannot verify the latest review", pull)
}

// ApprovalFor observes host-authenticated review identities and never reads
// candidate configuration or .pyxis/config.local.yaml for authority.
func (r *Reader) ApprovalFor(ctx context.Context, pull int, head, base, digest string) (approval.Approval, error) {
	if r == nil || r.run == nil || !repoSlug.MatchString(r.Repo) || pull < 1 || !fullCommit(head) || !fullCommit(base) {
		return approval.Approval{}, fmt.Errorf("read bound approval: repository, pull, full head, and policy commit are required")
	}
	cfg, authority, err := r.policy(ctx, base)
	if err != nil {
		return approval.Approval{}, err
	}
	current, err := r.pullHead(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	if current != head {
		return approval.Approval{}, approval.ErrNoApproval
	}
	reviews, err := r.reviews(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	current, err = r.pullHead(ctx, pull)
	if err != nil {
		return approval.Approval{}, err
	}
	if current != head {
		return approval.Approval{}, approval.ErrNoApproval
	}
	if selected, err := approval.Select(cfg.Approval.OperatorLogin, head, reviews); err == nil {
		return selected, nil
	}
	// An outstanding operator changes-requested review remains a stop. Argus
	// cannot silently override the operator's explicit review decision.
	var operator *approval.Review
	for i := range reviews {
		review := &reviews[i]
		if strings.EqualFold(review.Author, cfg.Approval.OperatorLogin) && review.ID > 0 && !strings.EqualFold(review.State, "COMMENTED") && !strings.EqualFold(review.State, "PENDING") && (operator == nil || review.ID > operator.ID) {
			operator = review
		}
	}
	if operator != nil && strings.EqualFold(operator.State, "CHANGES_REQUESTED") {
		return approval.Approval{}, approval.ErrNoApproval
	}
	return approval.SelectDriver(cfg.Approval, r.Repo, pull, head, base, digest, approval.DriverGranted(authority), reviews)
}

func fullCommit(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

func (r *Reader) policy(ctx context.Context, base string) (config.Config, []byte, error) {
	body, err := r.file(ctx, base, ".pyxis/config.yaml")
	if err != nil {
		return config.Config{}, nil, err
	}
	cfg, defects := config.Parse(body)
	if len(defects) != 0 {
		return cfg, nil, fmt.Errorf("invalid mainline approval configuration: %v", defects)
	}
	// Fail closed here rather than at config load: with no named operator there
	// is no one whose review counts, and no changes request Argus could honour.
	if cfg.Approval.OperatorLogin == "" {
		return cfg, nil, fmt.Errorf("mainline %s names no approval.operator_login; set it in .pyxis/config.yaml to the GitHub account that approves", base)
	}
	if cfg.Approval.ArgusLogin == "" {
		return cfg, nil, nil
	}
	authority, err := r.file(ctx, base, schema.AuthorityContractPath)
	return cfg, authority, err
}

func (r *Reader) file(ctx context.Context, base, path string) ([]byte, error) {
	body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, "repos/"+r.Repo+"/contents/"+path+"?ref="+base)
	if err != nil {
		return nil, fmt.Errorf("read mainline policy %s at %s: %w", path, base, err)
	}
	var file struct {
		Type     string `json:"type"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		Size     int    `json:"size"`
	}
	if err := json.Unmarshal(body, &file); err != nil || file.Type != "file" || file.Encoding != "base64" || file.Size < 0 || file.Size > 1<<20 {
		return nil, fmt.Errorf("invalid mainline policy response for %s", path)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil || len(decoded) != file.Size {
		return nil, fmt.Errorf("invalid mainline policy bytes for %s", path)
	}
	return decoded, nil
}

func (r *Reader) pullHead(ctx context.Context, pull int) (string, error) {
	path := "repos/" + r.Repo + "/pulls/" + strconv.Itoa(pull)
	body, err := r.run(ctx, "api", "--method", "GET", "-H", noCache, path)
	if err != nil {
		return "", fmt.Errorf("read pull #%d head: %w", pull, err)
	}
	var data struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("decode pull #%d head: %w", pull, err)
	}
	if len(data.Head.SHA) != 40 || strings.Trim(data.Head.SHA, "0123456789abcdefABCDEF") != "" {
		return "", fmt.Errorf("pull #%d has no valid head", pull)
	}
	return data.Head.SHA, nil
}
