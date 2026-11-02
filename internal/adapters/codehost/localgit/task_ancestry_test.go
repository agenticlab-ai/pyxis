package localgit

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestTaskReviewRefusesHeadThatDoesNotContainItsRecordedFeatureBase(t *testing.T) {
	t.Parallel()
	o, proposals, artifacts, _, requests, opened := taskReviewFixture(t)
	// The feature target advances on a sibling of the original task head.
	base := opened.Proposal.TargetOID
	advanced := fixtureGit(t, o.Root, "commit-tree", base+"^{tree}", "-p", base, "-m", "another task landed")
	fixtureGit(t, o.Root, "push", o.Remote, advanced+":"+proposals.options.IntegrationRef)
	change := revision(opened, opened.Proposal.Head)
	change.ExpectedTargetOID = advanced
	stale, err := proposals.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requests.Prepare(t.Context(), stale.Proposal.ID); !errors.Is(err, codehost.ErrReviewInvalid) {
		t.Fatalf("unrebased task presented for review: %v", err)
	}
	prepared, err := proposals.Read(t.Context(), stale.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, kind := range []string{codehost.TaskAggregate, codehost.TaskRequest} {
		if _, err := artifacts.Retain(t.Context(), prepared, kind, []byte("invalid review proof\n")); !errors.Is(err, codehost.ErrReviewInvalid) {
			t.Fatalf("non-ancestor proof retained: %v", err)
		}
	}
	if _, err := requests.Observe(t.Context(), stale.Proposal.ID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("observation accepted or repaired an unrebased task")
	}
	if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/review-aggregates", "refs/pyxis/review-requests", "refs/pyxis/receipts") != "" {
		t.Fatal("unrebased head created review or approval evidence")
	}
	// Once rebased, the same task gets a fresh generation and complete request.
	fixtureGit(t, o.Root, "fetch", "--no-tags", o.Origin, advanced)
	fixtureGit(t, o.Root, "switch", "--detach", opened.Proposal.Head)
	fixtureGit(t, o.Root, "rebase", "--onto", advanced, base)
	change = revision(prepared, fixtureGit(t, o.Root, "rev-parse", "HEAD"))
	change.ExpectedTargetOID = advanced
	if _, err := proposals.Publish(t.Context(), change); err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), stale.Proposal.ID)
	if err != nil || packet.Binding.TargetOID != advanced || packet.Binding.ProposalOID == prepared.OID {
		t.Fatalf("rebased task was refused: %v", err)
	}
}
