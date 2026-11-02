package lifecycle

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type publishHost struct {
	calls              []string
	failPush, failDrop bool
	created            string
	createErr          error
	open               string // the open pull requests `gh pr list` reports, as JSON
	failReview         bool
	failLookup         bool
}

func (h *publishHost) run(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	h.calls = append(h.calls, call)
	switch {
	case strings.Contains(call, " push ") && strings.HasSuffix(call, " :refs/heads/draft/widget-v2"):
		if h.failDrop {
			return nil, errors.New("delete refused")
		}
	case strings.Contains(call, " push "):
		if h.failPush {
			return nil, errors.New("push refused")
		}
	case strings.HasPrefix(call, "gh pr create"):
		return []byte(h.created), h.createErr
	case strings.HasPrefix(call, "gh pr list"):
		if h.failLookup {
			return nil, errors.New("host unavailable")
		}
		if h.open == "" {
			return []byte("[]\n"), nil
		}
		return []byte(h.open + "\n"), nil
	case strings.HasPrefix(call, "gh pr edit") && h.failReview:
		return nil, errors.New("reviewer refused")
	}
	return nil, nil
}

func publish(h *publishHost) (int, error) {
	return PublishNew(context.Background(), h.run, "/work", nil, "https://github.com/o/r.git", "draft/widget-v2", "abc", "main", "widget — contract v2", "request", "operator")
}

func TestPublishNewPushesAnAbsentBranchAndOpensItsPullRequest(t *testing.T) {
	h := &publishHost{created: "https://github.com/o/r/pull/42\n"}
	n, err := publish(h)
	if err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	want := []string{
		"git -c credential.helper= -c credential.helper=!gh auth git-credential push --force-with-lease=refs/heads/draft/widget-v2: https://github.com/o/r.git abc:refs/heads/draft/widget-v2",
		"gh pr create --head draft/widget-v2 --base main --title widget — contract v2 --body request --reviewer operator",
	}
	if !slices.Equal(h.calls, want) {
		t.Fatalf("calls = %q", h.calls)
	}
}

func TestPublishNewWithdrawsTheBranchWhenThePullRequestFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		host *publishHost
		want string
	}{
		{"push", &publishHost{failPush: true}, "push draft/widget-v2"},
		{"create", &publishHost{createErr: errors.New("host outage")}, "its pushed branch was withdrawn"},
		{"unreadable URL", &publishHost{created: "done\n"}, "its pushed branch was withdrawn"},
		{"withdrawal", &publishHost{createErr: errors.New("host outage"), failDrop: true}, "delete it on the code host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := publish(tc.host); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if tc.name == "push" {
				return
			}
			last := tc.host.calls[len(tc.host.calls)-1]
			if !strings.Contains(last, "--force-with-lease=refs/heads/draft/widget-v2:abc") || !strings.HasSuffix(last, " :refs/heads/draft/widget-v2") {
				t.Fatalf("withdrawal = %q", last)
			}
		})
	}
}

// A repository that has not configured its operator still opens its pull request,
// and asks for nobody, on either route to an open pull request.
func TestPublishNewWithNoReviewerRequestsNobody(t *testing.T) {
	for name, h := range map[string]*publishHost{
		"created":                   {created: "https://github.com/o/r/pull/42\n"},
		"opened before the failure": {createErr: errors.New("pull request update failed"), open: ownPR42},
	} {
		t.Run(name, func(t *testing.T) {
			n, err := PublishNew(t.Context(), h.run, "", nil, "https://example.test/repo.git", "draft/widget-v2", "abc", "main", "title", "body", "")
			if err != nil || n != 42 {
				t.Fatalf("n=%d err=%v", n, err)
			}
			for _, call := range h.calls {
				if strings.Contains(call, "reviewer") {
					t.Fatalf("requested a reviewer: %q", call)
				}
			}
		})
	}
}

func TestPullRequestNumberReadsOnlyAPullRequestURL(t *testing.T) {
	if n, err := pullRequestNumber("Creating pull request\nhttps://github.com/o/r/pull/42\n"); err != nil || n != 42 {
		t.Fatalf("number = %d, err = %v", n, err)
	}
	for _, bad := range []string{"", "https://github.com/o/r/issues/42", "https://github.com/o/r/pull/x42", "https://github.com/o/r/pull/042", "https://github.com/o/r/pull/0"} {
		if _, err := pullRequestNumber(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestPublishNewKeepsAPullRequestTheHostOpenedBeforeFailing(t *testing.T) {
	h := &publishHost{createErr: errors.New("pull request update failed"), open: ownPR42}
	if n, err := publish(h); err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if last := h.calls[len(h.calls)-1]; last != "gh pr edit 42 --add-reviewer operator" {
		t.Fatalf("review not requested again: %q", last)
	}
	h = &publishHost{createErr: errors.New("pull request update failed"), open: ownPR42, failReview: true}
	n, err := publish(h)
	if n != 42 || !errors.Is(err, ErrReviewNotRequested) || !strings.Contains(err.Error(), "request it by hand") {
		t.Fatalf("n=%d err=%v", n, err)
	}
	for _, call := range h.calls {
		if strings.HasSuffix(call, " :refs/heads/draft/widget-v2") {
			t.Fatalf("an open pull request's branch was withdrawn: %q", call)
		}
	}
}

func TestPublishNewKeepsTheBranchWhenItCannotTellWhetherThePullRequestOpened(t *testing.T) {
	for _, h := range []*publishHost{
		{createErr: errors.New("timeout"), failLookup: true},
		{createErr: errors.New("timeout"), open: "x1"},
		{createErr: errors.New("timeout"), open: "[" + strings.Trim(ownPR42, "[]") + "," + strings.Trim(ownPR42, "[]") + "]"},
	} {
		if _, err := publish(h); err == nil || !strings.Contains(err.Error(), "branch is kept") {
			t.Fatalf("err = %v", err)
		}
		for _, call := range h.calls {
			if strings.HasSuffix(call, " :refs/heads/draft/widget-v2") {
				t.Fatalf("an indeterminate publication withdrew its branch: %q", call)
			}
		}
	}
}

// ownPR42 is this repository's open pull request from the draft branch.
const ownPR42 = `[{"number":42,"headRefName":"draft/widget-v2","baseRefName":"main","isCrossRepository":false}]`

func TestPublishNewIgnoresAForkBranchOfTheSameName(t *testing.T) {
	h := &publishHost{createErr: errors.New("timeout"), open: `[{"number":7,"headRefName":"draft/widget-v2","baseRefName":"main","isCrossRepository":true},{"number":8,"headRefName":"draft/widget-v2","baseRefName":"release","isCrossRepository":false}]`}
	if _, err := publish(h); err == nil || !strings.Contains(err.Error(), "branch was withdrawn") {
		t.Fatalf("err = %v", err)
	}
	for _, call := range h.calls {
		if strings.HasPrefix(call, "gh pr edit") {
			t.Fatalf("review requested on an unrelated pull request: %q", call)
		}
	}
}
