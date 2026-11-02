package ghcodehost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/ghidentity"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
)

// noCacheHeader is the freshness directive attached to every read: it forbids
// any intermediary from returning a cached representation. Together with the
// exact-REST endpoint (never `search/…`) and the absence of any conditional
// (If-None-Match) header, it makes each read exact and fresh — never a search
// index, never a cacheable conditional GET (AP-15).
const noCacheHeader = "Cache-Control: no-cache"

// ghClient is the production client: it shells an authenticated `gh api`. The
// repo's standing assumption is that `gh` is installed and authenticated (hooks
// and CI already require it). The run seam is injectable so the argv
// construction and JSON decoding are unit-tested without a network or a real
// `gh`.
type ghClient struct {
	repo   string // owner/name
	run    func(ctx context.Context, args ...string) ([]byte, error)
	runLog func(ctx context.Context, limit int, args ...string) ([]byte, error)
	// runInput is the stdin-bearing form, for writes whose body must never
	// touch argv (a review artifact carries fences and can be many KB).
	runInput  func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)
	budget    *budget.Budget
	budgetErr error
}

// newGHClient binds a client to a repository slug, shelling the real `gh`.
func newGHClient(repo string) *ghClient {
	b, err := defaultBudget()
	return &ghClient{repo: repo, run: shellGH, runLog: shellGHLog, runInput: shellGHInput, budget: b, budgetErr: err}
}

func newGHClientWithBudget(repo string, b *budget.Budget) *ghClient {
	c := &ghClient{repo: repo, run: shellGH, runLog: shellGHLog, runInput: shellGHInput, budget: b}
	if b == nil {
		c.budgetErr = errors.New("content-write budget is not configured")
	}
	return c
}

// defaultBudget binds the machine's shared ledgers for the authenticated
// identity — NOT one ledger per repository. GitHub publishes the
// content-write caps but not the entity they are scoped to, so a
// per-repository ledger could let N repositories each spend a full window
// against what may be one identity's quota (design:state-reader-v4 §3.8).
func defaultBudget() (*budget.Budget, error) {
	host, login := ghidentity.Resolve(context.Background())
	return budget.NewForIdentityResolver(host, login, ghidentity.Resolve, nil, nil)
}

// shellGH execs `gh <args…>` and returns stdout; a non-zero exit becomes an
// error carrying gh's stderr.
func shellGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// shellGHInput execs `gh <args…>` with stdin and returns stdout; a non-zero
// exit becomes an error carrying gh's stderr.
func shellGHInput(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// owner is the "owner" half of the "owner/name" repo slug.
func (c *ghClient) owner() string {
	if i := strings.Index(c.repo, "/"); i > 0 {
		return c.repo[:i]
	}
	return c.repo
}

// findOpenPRByHead lists the open pull requests opened from head via the exact
// REST pulls endpoint (server-side head+state filter) — never the search index
// (AP-15). It returns the first match, if any.
func (c *ghClient) findOpenPRByHead(ctx context.Context, head string) (int, bool, error) {
	path := fmt.Sprintf("repos/%s/pulls?head=%s:%s&state=open&per_page=100", c.repo, c.owner(), head)
	out, err := c.run(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, path)
	if err != nil {
		return 0, false, fmt.Errorf("list open pulls for head %q: %w", head, err)
	}
	pulls, err := parsePulls(out)
	if err != nil {
		return 0, false, err
	}
	if len(pulls) == 0 {
		return 0, false, nil
	}
	return pulls[0].Number, true, nil
}

// compareAhead reads how many commits head is ahead of base via the exact,
// fresh compare endpoint.
func (c *ghClient) compareAhead(ctx context.Context, base, head string) (int, error) {
	path := fmt.Sprintf("repos/%s/compare/%s...%s", c.repo, base, head)
	out, err := c.run(ctx, "api", "--method", "GET", "-H", noCacheHeader, path)
	if err != nil {
		return 0, fmt.Errorf("compare %s...%s: %w", base, head, err)
	}
	var cmp struct {
		AheadBy int `json:"ahead_by"`
	}
	if err := json.Unmarshal(out, &cmp); err != nil {
		return 0, fmt.Errorf("decode compare %s...%s: %w", base, head, err)
	}
	return cmp.AheadBy, nil
}

// createPR opens a draft proposal from head into base. It is opened as a draft
// so it cannot be merged until a later stage marks it ready — reinforcing the
// never-merge boundary (DINV-6).
func (c *ghClient) createPR(ctx context.Context, head, base, title, body string) (int, error) {
	if c.budgetErr != nil {
		return 0, fmt.Errorf("reserve content write: %w", c.budgetErr)
	}
	if c.budget != nil {
		if err := c.budget.Acquire(ctx, 1); err != nil {
			return 0, fmt.Errorf("reserve content write: %w", err)
		}
	}
	path := fmt.Sprintf("repos/%s/pulls", c.repo)
	out, err := c.run(ctx, "api", "--method", "POST",
		"-f", "title="+title,
		"-f", "head="+head,
		"-f", "base="+base,
		"-f", "body="+body,
		"-F", "draft=true",
		path)
	if err != nil {
		return 0, fmt.Errorf("create pull from %q: %w", head, err)
	}
	var pr ghPull
	if err := json.Unmarshal(out, &pr); err != nil {
		return 0, fmt.Errorf("decode created pull: %w", err)
	}
	return pr.Number, nil
}

// createComment posts one conversation comment on proposal number via
// `POST /repos/{owner}/{name}/issues/{number}/comments` (GitHub models a PR's
// conversation comments as issue comments) and returns the created comment's
// id. It is a content write and reserves one unit of the shared write budget
// exactly as createPR does.
func (c *ghClient) createComment(ctx context.Context, number int, body string) (string, error) {
	if c.budgetErr != nil {
		return "", fmt.Errorf("reserve content write: %w", c.budgetErr)
	}
	if c.budget != nil {
		if err := c.budget.Acquire(ctx, 1); err != nil {
			return "", fmt.Errorf("reserve content write: %w", err)
		}
	}
	// The body rides stdin as a JSON field (`--input -`), the same call shape
	// impl-review's own publisher uses: arbitrary content (fences, newlines,
	// size) never touches argv.
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", fmt.Errorf("marshal comment body: %w", err)
	}
	if c.runInput == nil {
		return "", errors.New("create comment: client has no stdin-capable runner")
	}
	path := fmt.Sprintf("repos/%s/issues/%d/comments", c.repo, number)
	out, err := c.runInput(ctx, payload, "api", "--method", "POST", path, "--input", "-")
	if err != nil {
		return "", fmt.Errorf("create comment on #%d: %w", number, err)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(out, &created); err != nil {
		return "", fmt.Errorf("decode created comment: %w", err)
	}
	if created.ID == 0 {
		return "", fmt.Errorf("create comment on #%d: response carried no id", number)
	}
	return strconv.FormatInt(created.ID, 10), nil
}

// pullBody reads a proposal's body via an exact, fresh read, checking that
// the response is the proposal asked for.
func (c *ghClient) pullBody(ctx context.Context, number int) (string, error) {
	out, err := c.run(ctx, "api", "--method", "GET", "-H", noCacheHeader, fmt.Sprintf("repos/%s/pulls/%d", c.repo, number))
	if err != nil {
		return "", fmt.Errorf("read the body of #%d: %w", number, err)
	}
	var pr struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return "", fmt.Errorf("decode the body of #%d: %w", number, err)
	}
	if pr.Number != number {
		return "", fmt.Errorf("read of #%d answered for #%d", number, pr.Number)
	}
	return pr.Body, nil
}

// setPullBody replaces a proposal's body. It is a content write and reserves
// one unit of the shared write budget, as createPR does; the body rides stdin
// as a JSON field, so its content never touches argv.
func (c *ghClient) setPullBody(ctx context.Context, number int, body string) error {
	if c.budgetErr != nil {
		return fmt.Errorf("reserve content write: %w", c.budgetErr)
	}
	if c.budget != nil {
		if err := c.budget.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("reserve content write: %w", err)
		}
	}
	if c.runInput == nil {
		return errors.New("update a body: client has no stdin-capable runner")
	}
	input, err := json.Marshal(struct {
		Body string `json:"body"`
	}{body})
	if err != nil {
		return fmt.Errorf("marshal the body of #%d: %w", number, err)
	}
	if _, err := c.runInput(ctx, input, "api", "--method", "PATCH", "--input", "-", fmt.Sprintf("repos/%s/pulls/%d", c.repo, number)); err != nil {
		return fmt.Errorf("update the body of #%d: %w", number, err)
	}
	return nil
}

// pullHeadSHA reads a proposal's current head commit SHA via an exact, fresh
// read — the ref the Checks API is then queried against.
func (c *ghClient) pullHeadSHA(ctx context.Context, number int) (string, error) {
	snapshot, err := c.pullSnapshot(ctx, number)
	if err != nil {
		return "", err
	}
	return snapshot.HeadSHA, nil
}

// pullSnapshot reads the proposal lifecycle and exact current head in one
// no-cache request. drive-to-green uses this observation to stop before writing
// when a human merged/closed the proposal or another writer advanced its head.
func (c *ghClient) pullSnapshot(ctx context.Context, number int) (pullSnapshot, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", c.repo, number)
	out, err := c.run(ctx, "api", "--method", "GET", "-H", noCacheHeader, path)
	if err != nil {
		return pullSnapshot{}, fmt.Errorf("get pull #%d: %w", number, err)
	}
	var pr struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
		Head   struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return pullSnapshot{}, fmt.Errorf("decode pull #%d: %w", number, err)
	}
	if pr.Head.SHA == "" {
		return pullSnapshot{}, fmt.Errorf("pull #%d: empty head sha", number)
	}
	return pullSnapshot{State: pr.State, Merged: pr.Merged, HeadSHA: pr.Head.SHA, HeadRef: pr.Head.Ref}, nil
}

// checkRuns reads the check runs on a commit through the Checks API
// (commits/{sha}/check-runs), filtered to the latest run per check so a re-run
// supersedes its predecessor — an exact, fresh read, never a status rollup
// (AP-13, AP-15).
func (c *ghClient) checkRuns(ctx context.Context, sha string) ([]checkRun, error) {
	path := fmt.Sprintf("repos/%s/commits/%s/check-runs?filter=latest&per_page=100", c.repo, sha)
	out, err := c.run(ctx, "api", "--method", "GET", "--paginate",
		"-H", "Accept: application/vnd.github+json", "-H", noCacheHeader, path)
	if err != nil {
		return nil, fmt.Errorf("list check runs for %s: %w", sha, err)
	}
	return parseCheckRuns(out)
}

// ghPull is the wire shape of a pull request — only the number is consumed.
type ghPull struct {
	Number int `json:"number"`
}

type pullSnapshot struct {
	State   string
	Merged  bool
	HeadSHA string
	HeadRef string
}

// parsePulls tolerates gh --paginate's concatenated JSON arrays.
func parsePulls(out []byte) ([]ghPull, error) {
	var pulls []ghPull
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page []ghPull
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode pulls: %w", err)
		}
		pulls = append(pulls, page...)
	}
	return pulls, nil
}

// ghCheckRunsPage is the wire shape of one page of the Checks-API response — a
// top-level object with a check_runs array.
type ghCheckRunsPage struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// parseCheckRuns tolerates gh --paginate's concatenated JSON objects, stitching
// each page's check_runs array.
func parseCheckRuns(out []byte) ([]checkRun, error) {
	var runs []checkRun
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page ghCheckRunsPage
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode check runs: %w", err)
		}
		for _, r := range page.CheckRuns {
			runs = append(runs, checkRun{Name: r.Name, Status: r.Status, Conclusion: r.Conclusion})
		}
	}
	return runs, nil
}
