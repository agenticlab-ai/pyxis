package localgit

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestFeatureFinalListsIsolateSourceRefsAndKeepStaleDetection(t *testing.T) {
	t.Parallel()
	for _, equalHeads := range []bool{false, true} {
		t.Run(map[bool]string{false: "different heads", true: "equal heads"}[equalHeads], func(t *testing.T) {
			o, drive, _ := featureProposalFixture(t)
			base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "different feature source")
			next := fixtureGit(t, o.Root, "rev-parse", "HEAD")
			other := "refs/heads/integration/other-custom-ref"
			otherHead := next
			if equalHeads {
				otherHead = base
			}
			fixtureGit(t, o.Root, "push", o.Remote, otherHead+":"+other)
			stores := []*ProposalStore{
				NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: drive.TargetMainRef, HeadRef: drive.IntegrationRef}),
				NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: drive.TargetMainRef, HeadRef: other}),
			}
			ids, heads := []string{"final-widget", "final-other"}, []string{base, otherHead}
			for i, store := range stores {
				if _, err := store.Publish(t.Context(), codehost.ProposalChange{ID: ids[i], Feature: ids[i], Branch: "refs/heads/" + ids[i], Head: heads[i], State: "open", ExpectedTargetOID: base}); err != nil {
					t.Fatal(err)
				}
			}
			clone := proposalClone(t, o)
			before := fixtureGit(t, o.Origin, "show-ref")
			for i, store := range stores {
				options := store.options
				options.Git = clone
				for _, observer := range []*ProposalStore{store, NewProposals(options)} {
					rows, err := observer.List(t.Context())
					if err != nil || len(rows) != 1 || rows[0].Proposal.ID != ids[i] || rows[0].Proposal.SourceRef != options.HeadRef {
						t.Fatalf("listing crossed feature sources: %+v %v", rows, err)
					}
				}
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("listing modified origin refs")
			}
			fixtureGit(t, o.Root, "push", o.Remote, next+":"+drive.IntegrationRef)
			if _, err := stores[0].List(t.Context()); !errors.Is(err, codehost.ErrProposalConflict) {
				t.Fatalf("matching stale source was filtered out: %v", err)
			}
			if rows, err := stores[1].List(t.Context()); err != nil || len(rows) != 1 || rows[0].Proposal.ID != ids[1] {
				t.Fatalf("another stale feature blocked this feature: %+v %v", rows, err)
			}
		})
	}
}
