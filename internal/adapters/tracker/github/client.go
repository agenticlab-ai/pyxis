package ghtracker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

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
// and CI already require it), so no token handling lives here. The run seam is
// injectable so the argv-construction and JSON-decoding logic is unit-tested
// without a network or a real `gh`.
type ghClient struct {
	repo      string // owner/name
	run       func(ctx context.Context, args ...string) ([]byte, error)
	sleep     func(ctx context.Context, d time.Duration) error // backoff between read retries; injectable for hermetic tests
	budget    *budget.Budget
	budgetErr error
}

// newGHClient binds a client to a repository slug, shelling the real `gh`.
func newGHClient(repo string) *ghClient {
	b, err := defaultBudget()
	return &ghClient{repo: repo, run: shellGH, sleep: ctxSleep, budget: b, budgetErr: err}
}

// defaultBudget binds the machine's shared ledgers for the authenticated
// identity — NOT one ledger per repository (design:state-reader-v4 §3.8).
func defaultBudget() (*budget.Budget, error) {
	host, login := ghidentity.Resolve(context.Background())
	return budget.NewForIdentityResolver(host, login, ghidentity.Resolve, nil, nil)
}

func newGHClientWithBudget(repo string, b *budget.Budget) *ghClient {
	c := &ghClient{repo: repo, run: shellGH, sleep: ctxSleep, budget: b}
	if b == nil {
		c.budgetErr = errors.New("content-write budget is not configured")
	}
	return c
}

// maxReadAttempts bounds transient-retry on an idempotent gh read: a few
// attempts, then fail closed (AP-16). It is the read path only — writes never
// retry.
const maxReadAttempts = 4

// readRetryBaseDelay is the first backoff step; it doubles each attempt
// (exponential backoff, AP-16).
const readRetryBaseDelay = 250 * time.Millisecond

// ctxSleep waits for d or until ctx is cancelled, returning ctx.Err() on
// cancellation — so a backoff never outlives a cancelled run.
func ctxSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// runRead performs an idempotent gh read with bounded transient-retry: a
// transient failure — an HTTP 5xx or a connection/timeout error — is retried up
// to maxReadAttempts with exponential backoff, honoring ctx cancellation
// (AP-16). Every attempt is still an exact, fresh read — the retry re-issues the
// same un-cached call, never a conditional or cached one (AP-15). A
// non-transient error (4xx, parse) returns immediately, and once the bound is
// exhausted the last error is returned (fail-closed). Only idempotent READs use
// this; a write (comment) must never be retried, since a retry could double-post.
func (c *ghClient) runRead(ctx context.Context, args ...string) ([]byte, error) {
	sleep := c.sleep
	if sleep == nil {
		sleep = ctxSleep
	}
	var lastErr error
	delay := readRetryBaseDelay
	for attempt := 1; ; attempt++ {
		out, err := c.run(ctx, args...)
		if err == nil {
			return out, nil
		}
		if !isTransient(err) {
			return nil, err
		}
		lastErr = err
		if attempt >= maxReadAttempts {
			return nil, fmt.Errorf("after %d attempts: %w", maxReadAttempts, lastErr)
		}
		if serr := sleep(ctx, delay); serr != nil {
			return nil, serr
		}
		delay *= 2
	}
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

// openIssuesWithLabel lists the OPEN issues carrying label via the exact REST
// issues endpoint (server-side state+label filter, full pagination) — never the
// search index (AP-15). Pull requests, which GitHub returns in the issues
// listing, are flagged so the adapter can exclude them.
func (c *ghClient) openIssuesWithLabel(ctx context.Context, label string) ([]item, error) {
	path := fmt.Sprintf("repos/%s/issues?state=open&labels=%s&per_page=100", c.repo, label)
	out, err := c.runRead(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, path)
	if err != nil {
		return nil, fmt.Errorf("list open issues labeled %q: %w", label, err)
	}
	return parseIssues(out)
}

// blockedBy lists the issue numbers blocking the given issue via the exact
// dependencies REST endpoint. A tracker without the dependency surface reads as
// "no blockers" (a 404 is tolerated, not fatal).
func (c *ghClient) blockedBy(ctx context.Context, number int) ([]int, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by?per_page=100", c.repo, number)
	out, err := c.runRead(ctx, "api", "--method", "GET", "--paginate",
		"-H", "Accept: application/vnd.github+json", "-H", noCacheHeader, path)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list blocked-by for #%d: %w", number, err)
	}
	var nums []int
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page []struct {
			Number int `json:"number"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode blocked-by for #%d: %w", number, err)
		}
		for _, e := range page {
			nums = append(nums, e.Number)
		}
	}
	return nums, nil
}

// comments lists issue-comment bodies via the exact issue-comments endpoint.
// The fs-spec:v1 capsule is a projected machine contract, so this read follows
// the same complete, exact, no-cache path as every other frontier input.
func (c *ghClient) comments(ctx context.Context, number int) ([]string, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments?per_page=100", c.repo, number)
	out, err := c.runRead(ctx, "api", "--method", "GET", "--paginate", "-H", noCacheHeader, path)
	if err != nil {
		return nil, fmt.Errorf("list comments for #%d: %w", number, err)
	}
	var bodies []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page []struct {
			Body string `json:"body"`
		}
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode comments for #%d: %w", number, err)
		}
		for _, comment := range page {
			bodies = append(bodies, comment.Body)
		}
	}
	return bodies, nil
}

// comment posts a progress note onto an issue.
func (c *ghClient) comment(ctx context.Context, number int, body string) error {
	if c.budgetErr != nil {
		return fmt.Errorf("reserve content write: %w", c.budgetErr)
	}
	if c.budget != nil {
		if err := c.budget.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("reserve content write: %w", err)
		}
	}
	path := fmt.Sprintf("repos/%s/issues/%d/comments", c.repo, number)
	if _, err := c.run(ctx, "api", "--method", "POST", "-f", "body="+body, path); err != nil {
		return fmt.Errorf("comment on #%d: %w", number, err)
	}
	return nil
}

// ghIssue is the wire shape of one issue in the REST listing — only the fields
// the frontier consumes (AP-6 compactness).
type ghIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	State  string `json:"state"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request"`
}

// parseIssues tolerates gh --paginate's concatenated JSON arrays.
func parseIssues(out []byte) ([]item, error) {
	var items []item
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page []ghIssue
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("decode issues: %w", err)
		}
		for _, gi := range page {
			names := make([]string, 0, len(gi.Labels))
			for _, l := range gi.Labels {
				names = append(names, l.Name)
			}
			items = append(items, item{
				Number: gi.Number,
				Title:  gi.Title,
				Body:   gi.Body,
				State:  strings.ToLower(strings.TrimSpace(gi.State)),
				Labels: names,
				IsPR:   gi.PullRequest != nil,
			})
		}
	}
	return items, nil
}

// isNotFound reports whether err is a gh HTTP 404 — the "surface not present"
// signal the dependency read tolerates.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "404")
}

// isTransient reports whether err is a retryable gh read failure: a GitHub 5xx
// (500/502/503/504) or a connection/timeout error. A 4xx (including 404) and a
// parse error are NOT transient — retrying them is futile (AP-16). gh surfaces
// the status as "HTTP 5xx" in its stderr, so the status is matched with the
// "http " prefix — never a bare "500" — so an issue number like #500 in the
// path is never mistaken for a server error.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, code := range []string{"http 500", "http 502", "http 503", "http 504"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	for _, sig := range []string{"timeout", "timed out", "connection refused", "connection reset", "no such host", "temporary failure in name resolution"} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}
