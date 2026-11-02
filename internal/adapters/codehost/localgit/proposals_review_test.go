package localgit

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestProposalsBindReadsAndListsToTheirWorkflow(t *testing.T) {
	t.Parallel()
	o, drafts, change := proposalFixture(t)
	draft, err := drafts.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	generations := []codehost.ProposalGeneration{draft}
	for _, name := range []string{"alpha", "beta"} {
		target := "refs/heads/feature-" + name
		fixtureGit(t, o.Origin, "update-ref", target, change.ExpectedTargetOID)
		store := NewProposals(ProposalOptions{Git: o, Kind: "implementation", IntegrationRef: target})
		c := change
		c.ID, c.Task, c.Branch = "task-"+name, "carina:"+name, "refs/heads/task-"+name
		generation, err := store.Publish(t.Context(), c)
		if err != nil {
			t.Fatal(err)
		}
		generations = append(generations, generation)
	}
	clone := proposalClone(t, o)
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, own := range generations {
		store := NewProposals(ProposalOptions{Git: clone, Kind: own.Proposal.Kind, IntegrationRef: own.Proposal.TargetRef})
		list, err := store.List(t.Context())
		if err != nil || !slices.Equal(list, []codehost.ProposalGeneration{own}) {
			t.Fatalf("workflow %s listed foreign proposals: %+v, %v", own.Proposal.ID, list, err)
		}
		for _, candidate := range generations {
			got, err := store.Read(t.Context(), candidate.Proposal.ID)
			if candidate == own {
				if err != nil || got != own {
					t.Fatalf("own workflow read: %+v, %v", got, err)
				}
			} else if !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("foreign kind or destination admitted: %+v, %v", got, err)
			}
		}
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("workflow reads changed origin refs")
	}
}

func TestProposalsValidateEveryRetainedGeneration(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"unknown field", "noncanonical bytes", "invalid prepared request", "missing retained parents"} {
		t.Run(name, func(t *testing.T) {
			o, store, _, binding := preparedReceiptFixture(t)
			prepared, err := store.Read(t.Context(), binding.ProposalID)
			if err != nil {
				t.Fatal(err)
			}
			bad := prepared.Proposal
			if name == "invalid prepared request" {
				bad.ReviewRequest = nil
			}
			body, err := json.Marshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, '\n')
			parents := proposalParents(bad)
			switch name {
			case "unknown field":
				body = []byte(strings.TrimSuffix(string(body), "}\n") + ",\"unknown\":true}\n")
			case "noncanonical bytes":
				body = append([]byte(" "), body...)
			case "missing retained parents":
				parents = parents[:1]
			}
			badOID := proposalReviewObject(t, o, body, parents)
			current := codehost.ProposalGeneration{OID: badOID, Proposal: bad}
			// Two well-formed successors conceal the malformed record from any
			// reader that checks only its immediate predecessor.
			for range 2 {
				p := current.Proposal
				p.PreviousOID, p.Sequence, p.State, p.ReviewRequest = current.OID, p.Sequence+1, "open", nil
				oid, err := prepareProposal(t.Context(), o.Root, p)
				if err != nil {
					t.Fatal(err)
				}
				current = codehost.ProposalGeneration{OID: oid, Proposal: p}
			}
			fixtureGit(t, o.Root, "push", "--force", o.Remote, current.OID+":"+proposalPrefix+binding.ProposalID)
			clone := NewProposals(ProposalOptions{Git: proposalClone(t, o), Kind: "draft"})
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := clone.Read(t.Context(), binding.ProposalID); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("malformed retained generation read: %v", err)
			}
			if _, err := clone.List(t.Context()); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("malformed retained generation listed: %v", err)
			}
			if _, err := store.Publish(t.Context(), revision(current, current.Proposal.Head)); !errors.Is(err, codehost.ErrProposalInvalid) {
				t.Fatalf("malformed lineage extended: %v", err)
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("lineage refusal changed origin refs")
			}
		})
	}
}

func TestProposalsRejectRetainedNonceReuseAcrossOpenRound(t *testing.T) {
	t.Parallel()
	o, store, _, binding := preparedReceiptFixture(t)
	prepared, err := store.Read(t.Context(), binding.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	round, err := store.Publish(t.Context(), revision(prepared, prepared.Proposal.Head))
	if err != nil {
		t.Fatal(err)
	}
	bad := round.Proposal
	bad.PreviousOID, bad.Sequence, bad.State, bad.ReviewRequest = round.OID, round.Proposal.Sequence+1, "prepared", prepared.Proposal.ReviewRequest
	oid, err := prepareProposal(t.Context(), o.Root, bad)
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "push", "--force", o.Remote, oid+":"+proposalPrefix+binding.ProposalID)
	clone := NewProposals(ProposalOptions{Git: proposalClone(t, o), Kind: "draft"})
	if _, err := clone.Read(t.Context(), binding.ProposalID); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("retained nonce reuse admitted: %v", err)
	}
	if _, err := clone.List(t.Context()); !errors.Is(err, codehost.ErrProposalInvalid) {
		t.Fatalf("retained nonce reuse listed: %v", err)
	}
}

func proposalReviewObject(t *testing.T, o Options, body []byte, parents []string) string {
	t.Helper()
	blob, err := proposalGit(t.Context(), o.Root, body, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := proposalGit(t.Context(), o.Root, []byte("100644 blob "+strings.TrimSpace(string(blob))+"\tproposal.json\n"), nil, "mktree")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"commit-tree", strings.TrimSpace(string(tree))}
	for _, parent := range parents {
		args = append(args, "-p", parent)
	}
	oid, err := proposalGit(t.Context(), o.Root, []byte("retained record fixture\n"), nil, args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(oid))
}
