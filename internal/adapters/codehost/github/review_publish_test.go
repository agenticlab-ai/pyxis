package ghcodehost

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

// PublishReview lands the pre-publish review artifact as ONE conversation
// comment on the exact proposal and returns its medium reference.
func TestPublishReview_PostsOneCommentOnTheProposal(t *testing.T) {
	fc := &fakeClient{commentID: "9001"}
	h := newHost(fc, "main")
	ref, err := h.PublishReview(context.Background(), port.Proposal{Number: 77, Branch: "feat/x"}, "# review\n```impl-review\n{}\n```\n")
	if err != nil {
		t.Fatalf("PublishReview: %v", err)
	}
	if ref != "github-comment:9001" {
		t.Fatalf("ref = %q, want github-comment:9001", ref)
	}
	if fc.commentCalls != 1 || fc.commentNumber != 77 || !strings.Contains(fc.commentBody, "impl-review") {
		t.Fatalf("comment write = calls %d number %d body %q", fc.commentCalls, fc.commentNumber, fc.commentBody)
	}
}

func TestPublishReview_RefusesEmptyArtifactAndSurfacesWriteFailure(t *testing.T) {
	fc := &fakeClient{commentID: "1"}
	h := newHost(fc, "main")
	if _, err := h.PublishReview(context.Background(), port.Proposal{Number: 77}, "  \n"); err == nil || fc.commentCalls != 0 {
		t.Fatalf("empty artifact: err=%v calls=%d, want a refusal with no write", err, fc.commentCalls)
	}
	fc = &fakeClient{commentErr: errors.New("gh: HTTP 403")}
	h = newHost(fc, "main")
	if _, err := h.PublishReview(context.Background(), port.Proposal{Number: 77}, "body"); err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("write failure must surface: %v", err)
	}
}

// ProposalHead is the exact fresh head read the Worker compares its reviewed
// revision against; an absent SHA is an error, never an empty match.
func TestProposalHead_ReadsExactHeadAndFailsClosedOnEmpty(t *testing.T) {
	fc := &fakeClient{headSHA: "abc123"}
	h := newHost(fc, "main")
	head, err := h.ProposalHead(context.Background(), port.Proposal{Number: 5})
	if err != nil || head != "abc123" || fc.pulledNumber != 5 {
		t.Fatalf("ProposalHead = %q, %v (pulled #%d), want abc123 from #5", head, err, fc.pulledNumber)
	}
	fc = &fakeClient{}
	h = newHost(fc, "main")
	if _, err := h.ProposalHead(context.Background(), port.Proposal{Number: 5}); err == nil {
		t.Fatal("an empty head SHA must be an error")
	}
	fc = &fakeClient{headErr: errors.New("gh: HTTP 404")}
	h = newHost(fc, "main")
	if _, err := h.ProposalHead(context.Background(), port.Proposal{Number: 5}); err == nil {
		t.Fatal("a read failure must surface")
	}
}
