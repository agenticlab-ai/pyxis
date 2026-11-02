package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ghClient is the production client: it shells an authenticated `gh api` for the
// two calls the adapter needs — create a PR comment, read a comment body. The
// repo's standing assumption is that `gh` is installed and authenticated (hooks
// and CI already require it), so no token handling lives here (the ghtracker
// posture). The run seam is injectable so argv-construction and JSON-decoding
// are unit-tested without a network or a real `gh`; the adapter's own tests use
// a fake client and never reach this file.
type ghClient struct {
	repo string // owner/name
	run  func(ctx context.Context, stdin []byte, args ...string) ([]byte, error)
}

// newGHClient binds a client to a repository slug, shelling the real `gh`.
func newGHClient(repo string) *ghClient {
	return &ghClient{repo: repo, run: shellGH}
}

// compile-time proof the production client satisfies the adapter's port.
var _ hostClient = (*ghClient)(nil)

// postComment creates one issue/PR comment carrying body via the REST
// `POST /repos/{owner}/{name}/issues/{pr}/comments` (GitHub models a PR's
// conversation comments as issue comments), and returns the created comment's
// id. It is a WRITE: it never retries here (a retry could double-post) — the
// adapter owns the bounded retry (UB-5). The body is passed on stdin as a JSON
// field so arbitrary content (fences, newlines) never touches argv.
func (c *ghClient) postComment(ctx context.Context, pr int, body string) (string, error) {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return "", fmt.Errorf("ghclient: marshal comment body: %w", err)
	}
	endpoint := fmt.Sprintf("repos/%s/issues/%d/comments", c.repo, pr)
	out, err := c.run(ctx, payload, "api", "--method", "POST", endpoint, "--input", "-")
	if err != nil {
		return "", fmt.Errorf("ghclient: post comment on #%d: %w", pr, err)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if derr := json.Unmarshal(out, &created); derr != nil {
		return "", fmt.Errorf("ghclient: decode created comment: %w", derr)
	}
	if created.ID == 0 {
		return "", fmt.Errorf("ghclient: created comment carried no id")
	}
	return fmt.Sprintf("%d", created.ID), nil
}

// readComment reads a comment's current body by id via the REST
// `GET /repos/{owner}/{name}/issues/comments/{id}`. It is an idempotent read;
// the adapter treats a transient failure as a read error (Read returns it), and
// re-reads live on the next call — the adapter holds no cache (AP-15 posture).
func (c *ghClient) readComment(ctx context.Context, commentID string) (string, error) {
	endpoint := fmt.Sprintf("repos/%s/issues/comments/%s", c.repo, commentID)
	out, err := c.run(ctx, nil, "api", endpoint, "--header", noCacheHeader)
	if err != nil {
		return "", fmt.Errorf("ghclient: read comment %s: %w", commentID, err)
	}
	var got struct {
		Body string `json:"body"`
	}
	if derr := json.Unmarshal(out, &got); derr != nil {
		return "", fmt.Errorf("ghclient: decode comment %s: %w", commentID, derr)
	}
	return got.Body, nil
}

// currentLogin authenticates and returns the native author identity that the
// same gh process will use for the brokered write.
func (c *ghClient) currentLogin(ctx context.Context) (string, error) {
	out, err := c.run(ctx, nil, "api", "user", "--jq", ".login", "--header", noCacheHeader)
	if err != nil {
		return "", fmt.Errorf("ghclient: authenticate current user: %w", err)
	}
	login := strings.TrimSpace(string(out))
	if login == "" {
		return "", fmt.Errorf("ghclient: authenticated user carried no login")
	}
	return login, nil
}

func (c *ghClient) repositoryNodeID(ctx context.Context) (string, error) {
	out, err := c.run(ctx, nil, "api", "repos/"+c.repo, "--jq", ".node_id")
	if err != nil {
		return "", fmt.Errorf("github client: read repository node id: %w", err)
	}
	node := strings.TrimSpace(string(out))
	if node == "" || strings.ContainsAny(node, ":\r\n") {
		return "", errors.New("github client: repository node id is empty or malformed")
	}
	return node, nil
}

func (c *ghClient) canWriteProposalComments(ctx context.Context) (bool, error) {
	out, err := c.run(ctx, nil, "api", "repos/"+c.repo, "--jq", ".permissions | (.admin or .maintain or .push or .triage)")
	if err != nil {
		return false, fmt.Errorf("github client: read repository permissions: %w", err)
	}
	switch strings.TrimSpace(string(out)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("github client: malformed repository permission result %q", strings.TrimSpace(string(out)))
	}
}

// noCacheHeader forbids any intermediary from returning a cached representation
// on a read — the ghtracker freshness directive (AP-15).
const noCacheHeader = "Cache-Control: no-cache"

// shellGH runs the real `gh` with the given args, feeding stdin when non-nil,
// and returns stdout. A non-zero exit surfaces stderr in the error so a `gh`
// failure is diagnosable. This is the only place that touches a subprocess; it
// is never exercised by the adapter's hermetic tests.
func shellGH(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}
