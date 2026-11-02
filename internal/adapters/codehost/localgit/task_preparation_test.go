package localgit

import (
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestTaskPreparationRetainsUniqueGenerationNonceAcrossCloneAndRounds(t *testing.T) {
	t.Parallel()
	o, store, change := proposalFixture(t)
	store.options.Kind, store.options.IntegrationRef = "implementation", "refs/heads/feature-widget"
	fixtureGit(t, o.Origin, "update-ref", store.options.IntegrationRef, change.ExpectedTargetOID)
	change.ID, change.Task = "task-widget-t1", "T1"
	opened, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	preparation, err := codehost.NewTaskPreparation()
	if err != nil {
		t.Fatal(err)
	}
	change = revision(opened, opened.Proposal.Head)
	change.State, change.TaskPreparation = "prepared", &preparation
	prepared, err := store.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	clone := NewProposals(ProposalOptions{Git: proposalClone(t, o), Kind: "implementation", IntegrationRef: store.targetRef()})
	seen, err := clone.Read(t.Context(), change.ID)
	if err != nil || !reflect.DeepEqual(seen, prepared) || seen.Proposal.ReviewRequest != nil || seen.Proposal.TaskPreparation.Nonce != preparation.Nonce {
		t.Fatalf("clone lost task preparation: %+v %v", seen, err)
	}
	round, err := store.Publish(t.Context(), revision(prepared, prepared.Proposal.Head))
	if err != nil {
		t.Fatal(err)
	}
	change = revision(round, round.Proposal.Head)
	change.State, change.TaskPreparation = "prepared", &preparation
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := store.Publish(t.Context(), change); !errors.Is(err, codehost.ErrProposalInvalid) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("task preparation reused a retained nonce: %v", err)
	}
	fresh, err := codehost.NewTaskPreparation()
	if err != nil {
		t.Fatal(err)
	}
	change.TaskPreparation = &fresh
	if next, err := store.Publish(t.Context(), change); err != nil || next.OID == prepared.OID || next.Proposal.Sequence != 4 {
		t.Fatalf("fresh request did not advance its generation: %+v %v", next, err)
	}
}
