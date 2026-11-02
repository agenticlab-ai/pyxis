// Package forgejocodehost implements proposal and verification ports using
// Forgejo's native REST API. It never invokes the GitHub CLI.
package forgejocodehost

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/gobwas/glob"
)

type Host struct {
	Client        *forgejo.Client
	Repository    string
	BaseURL       string
	Base          string
	DraftPrefix   string
	TrustedActors []string
}

func New(client *forgejo.Client, baseURL, repository, base string) *Host {
	return &Host{Client: client, Repository: repository, BaseURL: strings.TrimRight(baseURL, "/"), Base: base, DraftPrefix: "WIP:"}
}

type Pull struct {
	Number         int    `json:"number"`
	State          string `json:"state"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	Draft          bool   `json:"draft"`
	Merged         bool   `json:"merged"`
	MergeCommitSHA string `json:"merge_commit_sha"`
	Head           struct {
		Ref  string `json:"ref"`
		SHA  string `json:"sha"`
		Repo struct {
			ID       int64  `json:"id"`
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (h *Host) path(suffix string) string { return forgejo.RepoPath(h.Repository, suffix) }
func (h *Host) Key() string               { return "forgejo:" + h.BaseURL + ":" + h.Repository }
func (h *Host) Cadence() time.Duration    { return 10 * time.Second }

func (h *Host) Pull(ctx context.Context, number int) (Pull, error) {
	var p Pull
	if number <= 0 {
		return p, errors.New("forgejo: proposal number must be positive")
	}
	err := h.Client.Request(ctx, "GET", h.path(fmt.Sprintf("pulls/%d", number)), nil, &p)
	if err == nil && (p.Number != number || p.Head.SHA == "") {
		err = errors.New("forgejo: malformed proposal identity or head")
	}
	return p, err
}

func (h *Host) Pulls(ctx context.Context, state string) ([]Pull, error) {
	var rows []Pull
	err := h.Client.List(ctx, h.path("pulls")+"?state="+url.QueryEscape(state), &rows)
	return rows, err
}

func (h *Host) OpenProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (port.Proposal, error) {
	head, err := proposalHead(task, work)
	if err != nil {
		return port.Proposal{}, err
	}
	number, err := h.openFrom(ctx, head)
	if err != nil {
		return port.Proposal{}, err
	}
	if number > 0 {
		return port.Proposal{Number: number, Branch: head}, nil
	}
	var comparison struct {
		TotalCommits *int `json:"total_commits"`
	}
	err = h.Client.Request(ctx, "GET", h.path("compare/"+url.PathEscape(h.Base)+"..."+url.PathEscape(head)), nil, &comparison)
	if err != nil {
		return port.Proposal{}, fmt.Errorf("forgejo: compare proposal branches: %w", err)
	}
	if comparison.TotalCommits == nil {
		return port.Proposal{}, errors.New("forgejo: comparison lacks total_commits")
	}
	if *comparison.TotalCommits == 0 {
		return port.Proposal{}, port.ErrNoProposal
	}
	prefix := h.DraftPrefix
	if prefix == "" {
		prefix = "WIP:"
	}
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = task.ID
	}
	body := h.describeTask(task, description).Render() + "\n"
	association, err := json.Marshal(struct {
		Schema string `json:"schema"`
		Task   string `json:"task"`
		Source string `json:"source"`
		Number int    `json:"number"`
	}{"pyxis-task-proposal/v1", task.ID, task.SourceRef, task.Number})
	if err != nil {
		return port.Proposal{}, fmt.Errorf("encode task association: %w", err)
	}
	body += "<!-- pyxis-task-proposal:" + base64.RawURLEncoding.EncodeToString(association) + " -->\n"
	request := struct{ Title, Head, Base, Body string }{prefix + " " + title, head, h.Base, body}
	// Named tags are required: Forgejo fields are lowercase.
	var created Pull
	err = h.Client.Request(ctx, "POST", h.path("pulls"), struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
	}{request.Title, request.Head, request.Base, request.Body}, &created)
	if err != nil {
		return port.Proposal{}, err
	}
	if created.Number <= 0 {
		return port.Proposal{}, errors.New("forgejo: created proposal lacks number")
	}
	actual, err := h.Pull(ctx, created.Number)
	if err != nil {
		return port.Proposal{}, err
	}
	if !actual.Draft {
		return port.Proposal{}, errors.New("forgejo: draft prefix was not recognized; proposal requires manual inspection")
	}
	return port.Proposal{Number: actual.Number, Branch: head}, nil
}

// DescribeProposal replaces the description of the proposal open from the
// task's head, keeping the rest of its body: the task association, the
// execution summary, and anything another writer appended.
func (h *Host) DescribeProposal(ctx context.Context, task port.Task, work port.Workdir, description prdescription.Description) (bool, error) {
	head, err := proposalHead(task, work)
	if err != nil {
		return false, err
	}
	number, err := h.openFrom(ctx, head)
	if err != nil || number == 0 {
		return false, err
	}
	read := func(ctx context.Context) (string, error) {
		pr, err := h.Pull(ctx, number)
		return pr.Body, err
	}
	write := func(ctx context.Context, body string) error {
		return h.Client.Request(ctx, "PATCH", h.path(fmt.Sprintf("pulls/%d", number)), struct {
			Body string `json:"body"`
		}{body}, nil)
	}
	if err := prdescription.Publish(ctx, h.describeTask(task, description).Render(), read, write); err != nil {
		return true, fmt.Errorf("forgejo: describe proposal #%d: %w", number, err)
	}
	return true, nil
}

// openFrom is the number of the open proposal from head in this repository,
// or 0 when there is none.
func (h *Host) openFrom(ctx context.Context, head string) (int, error) {
	pulls, err := h.Pulls(ctx, "open")
	if err != nil {
		return 0, err
	}
	for _, p := range pulls {
		if p.Head.Ref == head && p.Head.Repo.FullName == h.Repository {
			return p.Number, nil
		}
	}
	return 0, nil
}

func proposalHead(task port.Task, work port.Workdir) (string, error) {
	head := task.HeadBranch
	if head == "" && work != nil {
		head = work.Branch()
	}
	if head == "" {
		return "", errors.New("forgejo: task has no head branch")
	}
	return head, nil
}

// describeTask ends the description's What section with the task's
// provenance. Only a task on this exact host and repository gets Forgejo's
// native closing directive.
func (h *Host) describeTask(task port.Task, d prdescription.Description) prdescription.Description {
	line := fmt.Sprintf("- Implements task %s (%s), opened by the Pyxis executor, which never merges.", task.ID, task.SourceRef)
	issueURL := h.BaseURL + "/" + h.Repository + "/issues/" + strconv.Itoa(task.Number)
	if task.Number > 0 && task.SourceRef == issueURL {
		line += fmt.Sprintf("\n- Closes #%d", task.Number)
	}
	d.What = strings.TrimRight(d.What, "\n") + "\n" + line
	return d
}

func (h *Host) OpenProposals(ctx context.Context, tasks []port.Task) (map[string]port.OpenProposal, error) {
	rows, err := h.Pulls(ctx, "open")
	if err != nil {
		return nil, err
	}
	byHead := map[string]int{}
	for _, p := range rows {
		if p.Head.Repo.FullName == h.Repository {
			byHead[p.Head.Ref] = p.Number
		}
	}
	out := map[string]port.OpenProposal{}
	for _, t := range tasks {
		if n := byHead[t.HeadBranch]; n > 0 {
			out[t.ID] = port.OpenProposal{Number: n}
		}
	}
	return out, nil
}

func (h *Host) ProposalHead(ctx context.Context, p port.Proposal) (string, error) {
	r, e := h.Pull(ctx, p.Number)
	return r.Head.SHA, e
}

func (h *Host) PostComment(ctx context.Context, number int, body string) (string, error) {
	if number <= 0 || strings.TrimSpace(body) == "" {
		return "", errors.New("forgejo: invalid comment")
	}
	var c struct {
		ID int64 `json:"id"`
	}
	err := h.Client.Request(ctx, "POST", h.path(fmt.Sprintf("issues/%d/comments", number)), struct {
		Body string `json:"body"`
	}{body}, &c)
	if err != nil {
		return "", err
	}
	if c.ID <= 0 {
		return "", errors.New("forgejo: comment receipt missing")
	}
	return strconv.FormatInt(c.ID, 10), nil
}

func (h *Host) PublishReview(ctx context.Context, p port.Proposal, body string) (string, error) {
	if p.Number <= 0 || strings.TrimSpace(body) == "" {
		return "", errors.New("forgejo: review requires a proposal and nonempty artifact")
	}
	// Use the same immutable repository identity as durable publication and
	// harvest, so every emitted receipt can be exact-read by that adapter.
	var repository struct {
		ID int64 `json:"id"`
	}
	if err := h.Client.Request(ctx, "GET", h.path(""), nil, &repository); err != nil {
		return "", err
	}
	if repository.ID <= 0 {
		return "", errors.New("forgejo: review repository identity missing")
	}
	id, err := h.PostComment(ctx, p.Number, body)
	if err != nil {
		return "", err
	}
	node := h.BaseURL + "/repositories/" + strconv.FormatInt(repository.ID, 10)
	return "forgejo-comment:" + base64.RawURLEncoding.EncodeToString([]byte(node)) + ":" + id, nil
}

type protection struct {
	RuleName                      string   `json:"rule_name"`
	BranchName                    string   `json:"branch_name"`
	EnableStatusCheck             bool     `json:"enable_status_check"`
	StatusCheckContexts           []string `json:"status_check_contexts"`
	RequiredApprovals             int      `json:"required_approvals"`
	IgnoreStaleApprovals          bool     `json:"ignore_stale_approvals"`
	BlockOnRejectedReviews        bool     `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool     `json:"block_on_official_review_requests"`
}

func (h *Host) protection(ctx context.Context, branch string) (protection, error) {
	var selected struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
		Effective string `json:"effective_branch_protection_name"`
	}
	if err := h.Client.Request(ctx, "GET", h.path("branches/"+url.PathEscape(branch)), nil, &selected); err != nil {
		return protection{}, err
	}
	if selected.Name != branch {
		return protection{}, errors.New("forgejo: branch protection identity mismatch")
	}
	if !selected.Protected {
		return protection{}, nil
	}
	if selected.Effective == "" {
		return protection{}, errors.New("forgejo: protected branch has no effective rule")
	}
	var rule protection
	if err := h.Client.Request(ctx, "GET", h.path("branch_protections/"+url.PathEscape(selected.Effective)), nil, &rule); err != nil {
		return protection{}, err
	}
	if rule.RuleName != selected.Effective && rule.BranchName != selected.Effective {
		return protection{}, errors.New("forgejo: effective protection rule mismatch")
	}
	return rule, nil
}

type status struct {
	ID      int64  `json:"id"`
	Context string `json:"context"`
	Status  string `json:"status"`
}

func (h *Host) checks(ctx context.Context, p Pull) ([]port.CheckContext, []string, protection, error) {
	rule, err := h.protection(ctx, p.Base.Ref)
	if err != nil {
		return nil, nil, rule, err
	}
	var statuses []status
	if err = h.Client.List(ctx, h.path("commits/"+url.PathEscape(p.Head.SHA)+"/statuses"), &statuses); err != nil {
		return nil, nil, rule, err
	}
	latest := map[string]status{}
	for _, s := range statuses {
		if s.Context == "" || s.ID <= 0 {
			return nil, nil, rule, errors.New("forgejo: malformed commit status")
		}
		if old, ok := latest[s.Context]; !ok || s.ID > old.ID {
			latest[s.Context] = s
		}
	}
	var checks []port.CheckContext
	var missing []string
	names := make([]string, 0, len(latest))
	if rule.EnableStatusCheck && len(rule.StatusCheckContexts) > 0 {
		matched := map[string]bool{}
		for _, pattern := range rule.StatusCheckContexts {
			// Forgejo uses this exact matcher with no path separators; braces,
			// Unicode classes, and slashes must match its protected-check policy.
			matcher, err := glob.Compile(pattern)
			if err != nil {
				return nil, nil, rule, fmt.Errorf("forgejo: invalid required check pattern: %w", err)
			}
			found := false
			for name := range latest {
				if matcher.Match(name) {
					found = true
					matched[name] = true
				}
			}
			if !found {
				missing = append(missing, pattern)
			}
		}
		for name := range matched {
			names = append(names, name)
		}
	} else {
		for name := range latest {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	// Forgejo's required-context reduction starts at success, which skipped
	// matches cannot worsen. Without required contexts, its worst-status
	// reduction accepts success plus skipped, but an all-skipped set stays
	// skipped and does not satisfy IsSuccess.
	skippedPasses := rule.EnableStatusCheck && len(rule.StatusCheckContexts) > 0
	for _, name := range names {
		if latest[name].Status != "skipped" {
			skippedPasses = true
		}
	}
	for _, name := range names {
		s, ok := latest[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		state := port.CheckPending
		switch s.Status {
		case "success":
			state = port.CheckGreen
		case "skipped":
			if skippedPasses {
				state = port.CheckGreen
			}
		case "failure", "error":
			state = port.CheckRed
		}
		checks = append(checks, port.CheckContext{Name: name, State: state})
	}
	return checks, missing, rule, nil
}

func aggregate(checks []port.CheckContext, missing []string) port.CheckState {
	if len(missing) > 0 || len(checks) == 0 {
		return port.CheckPending
	}
	state := port.CheckGreen
	for _, c := range checks {
		if c.State == port.CheckRed {
			return port.CheckRed
		}
		if c.State != port.CheckGreen {
			state = port.CheckPending
		}
	}
	return state
}

func (h *Host) Checks(ctx context.Context, p port.Proposal) (port.CheckState, error) {
	r, err := h.Pull(ctx, p.Number)
	if err != nil {
		return port.CheckPending, err
	}
	c, m, _, err := h.checks(ctx, r)
	if err != nil {
		return port.CheckPending, err
	}
	after, err := h.ProposalHead(ctx, p)
	if err != nil {
		return port.CheckPending, err
	}
	if after != r.Head.SHA {
		return port.CheckPending, errors.New("forgejo: head changed during verification")
	}
	return aggregate(c, m), nil
}

func landing(p Pull) (port.Landing, error) {
	if p.Merged {
		if p.MergeCommitSHA == "" {
			return port.LandingUnknown, errors.New("forgejo: merged proposal missing merge commit")
		}
		return port.LandingLanded, nil
	}
	switch p.State {
	case "open":
		return port.LandingOpen, nil
	case "closed":
		return port.LandingClosedUnlanded, nil
	}
	return port.LandingUnknown, fmt.Errorf("forgejo: unknown proposal state %q", p.State)
}

func (h *Host) InspectProposal(ctx context.Context, p port.Proposal) (port.Landing, string, port.CheckState, error) {
	r, err := h.Pull(ctx, p.Number)
	if err != nil {
		return port.LandingUnknown, "", port.CheckPending, err
	}
	if p.Branch != "" && p.Branch != r.Head.Ref {
		return port.LandingUnknown, r.Head.SHA, port.CheckPending, errors.New("forgejo: proposal branch mismatch")
	}
	l, err := landing(r)
	if err != nil {
		return l, r.Head.SHA, port.CheckPending, err
	}
	checks, missing, _, err := h.checks(ctx, r)
	if err != nil {
		return l, r.Head.SHA, port.CheckPending, err
	}
	after, err := h.Pull(ctx, p.Number)
	if err != nil {
		return l, r.Head.SHA, port.CheckPending, err
	}
	if after.Head.SHA != r.Head.SHA || after.Merged != r.Merged || after.State != r.State {
		return port.LandingUnknown, r.Head.SHA, port.CheckPending, errors.New("forgejo: proposal changed during inspection")
	}
	return l, r.Head.SHA, aggregate(checks, missing), nil
}

func (h *Host) SetStatus(ctx context.Context, sha, name, state, target, description string) error {
	if sha == "" || name == "" {
		return errors.New("forgejo: status needs revision and context")
	}
	return h.Client.Request(ctx, "POST", h.path("statuses/"+url.PathEscape(sha)), struct {
		State       string `json:"state"`
		Context     string `json:"context"`
		TargetURL   string `json:"target_url"`
		Description string `json:"description"`
	}{state, name, target, description}, nil)
}

// ReadStatus reads the latest native result for one exact revision/context.
func (h *Host) ReadStatus(ctx context.Context, sha, name string) (port.CheckState, error) {
	var rows []status
	if err := h.Client.List(ctx, h.path("commits/"+url.PathEscape(sha)+"/statuses"), &rows); err != nil {
		return port.CheckPending, err
	}
	var latest status
	for _, row := range rows {
		if row.Context == name && row.ID > latest.ID {
			latest = row
		}
	}
	switch latest.Status {
	case "success":
		return port.CheckGreen, nil
	case "failure", "error":
		return port.CheckRed, nil
	default:
		return port.CheckPending, nil
	}
}

func (h *Host) Approve(ctx context.Context, number int, sha string) error {
	return h.Client.Request(ctx, "POST", h.path(fmt.Sprintf("pulls/%d/reviews", number)), struct {
		Event    string `json:"event"`
		CommitID string `json:"commit_id"`
		Body     string `json:"body"`
	}{"APPROVED", sha, "Pyxis approval of evaluated revision " + sha}, nil)
}

func (h *Host) Merge(ctx context.Context, number int, sha string) error {
	if sha == "" {
		return errors.New("forgejo: merge requires evaluated head")
	}
	return h.Client.Request(ctx, "POST", h.path(fmt.Sprintf("pulls/%d/merge", number)), struct {
		Do           string `json:"Do"`
		HeadCommitID string `json:"head_commit_id"`
		Force        bool   `json:"force_merge"`
		WhenGreen    bool   `json:"merge_when_checks_succeed"`
	}{"squash", sha, false, false}, nil)
}

var _ port.CodeHost = (*Host)(nil)
var _ port.ProposalProbe = (*Host)(nil)
