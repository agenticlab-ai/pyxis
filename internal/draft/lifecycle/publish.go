package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrReviewNotRequested reports a pull request that opened but whose
// reviewer could not be requested; the caller keeps it and its branch.
var ErrReviewNotRequested = errors.New("review not requested")

// PublishNew pushes head to a branch the code host does not have yet and
// opens its pull request against base with the reviewer requested, in one
// step. A pull request that cannot be opened takes its branch back down, so
// the caller's local branch stays as it was and a retry starts from the same
// unpublished state. The host may open the pull request and still fail, for
// instance while requesting the reviewer, so an open pull request on the
// branch is kept: its reviewer is requested again, and if that fails too the
// error wraps ErrReviewNotRequested with the pull request's number. An empty
// reviewer opens the pull request and requests nobody: a repository may not have
// configured its operator yet.
func PublishNew(ctx context.Context, run RunCommand, dir string, env []string, pushURL, branch, head, base, title, body, reviewer string) (int, error) {
	push := func(lease, refspec string) error {
		_, err := run(ctx, dir, env, "git", "-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential", "push", "--force-with-lease=refs/heads/"+branch+":"+lease, pushURL, refspec)
		return err
	}
	if err := push("", head+":refs/heads/"+branch); err != nil {
		return 0, fmt.Errorf("push %s: %w", branch, err)
	}
	create := []string{"pr", "create", "--head", branch, "--base", base, "--title", title, "--body", body}
	if reviewer != "" {
		create = append(create, "--reviewer", reviewer)
	}
	created, err := run(ctx, dir, env, "gh", create...)
	number := 0
	if err == nil {
		number, err = pullRequestNumber(string(created))
	}
	if err == nil {
		return number, nil
	}
	open, lookupErr := openPullRequest(ctx, run, dir, env, branch, base)
	if lookupErr != nil {
		return 0, fmt.Errorf("open the pull request for %s: %w; whether it opened is unknown (%v), so the pushed branch is kept — delete it on the code host if no pull request opened, then retry", branch, err, lookupErr)
	}
	if open > 0 && reviewer == "" {
		return open, nil
	}
	if open > 0 {
		if _, reviewErr := run(ctx, dir, env, "gh", "pr", "edit", strconv.Itoa(open), "--add-reviewer", reviewer); reviewErr != nil {
			return open, fmt.Errorf("%w: pull request #%d is open, but requesting %s's review failed: %v; request it by hand", ErrReviewNotRequested, open, reviewer, reviewErr)
		}
		return open, nil
	}
	if withdrawErr := push(head, ":refs/heads/"+branch); withdrawErr != nil {
		return 0, errors.Join(fmt.Errorf("open the pull request for %s: %w", branch, err),
			fmt.Errorf("withdraw pushed branch %s: %w; delete it on the code host before retrying", branch, withdrawErr))
	}
	return 0, fmt.Errorf("open the pull request for %s: %w; its pushed branch was withdrawn, so a retry starts again", branch, err)
}

// openPullRequest returns the open pull request from this repository's
// branch into base, or 0 when the host reports none. `gh pr list --head`
// also matches a fork's branch of the same name, so only an in-repository
// pull request with the expected head and base counts, and several are an
// error. A failed lookup is an error, never an absence: only an established
// absence lets the caller take the branch down.
func openPullRequest(ctx context.Context, run RunCommand, dir string, env []string, branch, base string) (int, error) {
	out, err := run(ctx, dir, env, "gh", "pr", "list", "--head", branch, "--state", "open", "--json", "number,headRefName,baseRefName,isCrossRepository")
	if err != nil {
		return 0, err
	}
	var listed []struct {
		Number int    `json:"number"`
		Head   string `json:"headRefName"`
		Base   string `json:"baseRefName"`
		Fork   bool   `json:"isCrossRepository"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		return 0, fmt.Errorf("read the open pull requests: %w", err)
	}
	var matches []int
	for _, pr := range listed {
		if !pr.Fork && pr.Head == branch && pr.Base == base && pr.Number > 0 {
			matches = append(matches, pr.Number)
		}
	}
	switch len(matches) {
	case 0:
		return 0, nil
	case 1:
		return matches[0], nil
	}
	return 0, fmt.Errorf("several open pull requests come from %s into %s: %v", branch, base, matches)
}

// pullRequestNumber reads the number from the URL `gh pr create` prints.
func pullRequestNumber(created string) (int, error) {
	fields := strings.Fields(created)
	if len(fields) == 0 {
		return 0, fmt.Errorf("opening the pull request printed no URL")
	}
	link := fields[len(fields)-1]
	_, number, ok := strings.Cut(link, "/pull/")
	if !ok {
		return 0, fmt.Errorf("opening the pull request printed %q, not a pull request URL", link)
	}
	var n int
	if _, err := fmt.Sscanf(number, "%d", &n); err != nil || n < 1 || fmt.Sprint(n) != number {
		return 0, fmt.Errorf("opening the pull request printed %q, not a pull request URL", link)
	}
	return n, nil
}
