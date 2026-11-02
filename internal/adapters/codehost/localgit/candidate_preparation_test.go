package localgit

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestCandidatePreparationRetainsExactIdentityAndCandidateObject(t *testing.T) {
	t.Parallel()
	o, d, port := featureProposalFixture(t)
	base := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "feature task result")
	head := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", o.Remote, head+":"+d.IntegrationRef)
	change := codehost.ProposalChange{ID: "final-widget", Feature: d.FeatureID, Branch: "refs/heads/final/widget", Head: head, State: "open", ExpectedTargetOID: base}
	opened, err := port.RequestFinal(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := featuredrive.NewGitCandidateBuilder(o.Root).Build(t.Context(), head, base, featuredrive.MergePolicy{Version: "merge-v1"}, "builder-v1")
	if err != nil {
		t.Fatal(err)
	}
	candidate.FeatureID = d.FeatureID
	preparation, err := codehost.NewCandidatePreparation(codehost.FinalCandidate(candidate), "required-v1")
	if err != nil {
		t.Fatal(err)
	}
	change.State, change.ExpectedProposalOID, change.ExpectedBranchOID, change.CandidatePreparation = "prepared", opened.Generation.OID, head, &preparation
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := port.RequestFinal(t.Context(), change); !errors.Is(err, codehost.ErrProposalInvalid) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("drive received candidate preparation authority")
	}
	store := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: d.TargetMainRef, HeadRef: d.IntegrationRef})
	for _, fault := range []string{"feature", "main", "tree", "missing candidate", "wrong parents", "operation"} {
		bad := preparation
		switch fault {
		case "feature":
			bad.Candidate.FeatureID = "other"
		case "main":
			bad.Candidate.MainHead = head
		case "tree":
			bad.Candidate.Tree = strings.Repeat("9", len(head))
		case "missing candidate":
			bad.Candidate.Commit = strings.Repeat("9", len(head))
		case "wrong parents":
			bad.Candidate.Commit = head
		case "operation":
			bad.Operation = codehost.TaskMerge
		}
		change.CandidatePreparation = &bad
		if _, err := store.Publish(t.Context(), change); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatalf("invalid candidate preparation mutated refs: %s %v", fault, err)
		}
	}
	change.CandidatePreparation = &preparation
	prepared, err := store.Publish(t.Context(), change)
	if err != nil || prepared.Proposal.CandidatePreparation == nil || *prepared.Proposal.CandidatePreparation != preparation {
		t.Fatalf("prepare candidate: %+v %v", prepared, err)
	}
	if !strings.Contains(fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", prepared.OID), candidate.Commit) {
		t.Fatal("prepared generation lost its candidate object")
	}
	cloneOptions := store.options
	cloneOptions.Git = proposalClone(t, o)
	read, err := NewProposals(cloneOptions).Read(t.Context(), change.ID)
	if err != nil || !reflect.DeepEqual(read, prepared) {
		t.Fatalf("clone lost complete preparation: %v", err)
	}
	change.ExpectedProposalOID, change.ExpectedBranchOID = prepared.OID, prepared.Proposal.Head
	before = fixtureGit(t, o.Origin, "show-ref")
	if _, err := store.Publish(t.Context(), change); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("new final preparation reused an old nonce")
	}
}
