package localgit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestTaskLandingAdapterRefusesUnavailableInputsAndGitFailures(t *testing.T) {
	t.Parallel()
	o, store, packet := readyTaskAdapter(t)
	b := packet.Binding
	for _, method := range []func(context.Context, codehost.ReviewBinding) (codehost.TaskLandingResult, error){
		store.LandTask, NewTaskLandings(ProposalOptions{}, nil).LandTask,
	} {
		wrong := b
		wrong.ProposalID = "missing-task"
		before := fixtureGit(t, o.Origin, "show-ref")
		if _, err := method(t.Context(), wrong); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("unavailable task inputs permitted mutation")
		}
	}
	if _, err := NewTaskLandings(ProposalOptions{}, nil).VerifyTaskLanding(t.Context(), b.ProposalID); err == nil {
		t.Fatal("verification accepted an unavailable origin")
	}
	if _, err := store.VerifyTaskLanding(t.Context(), b.ProposalID); err == nil {
		t.Fatal("prepared task was presented as landed")
	}
	if _, err := store.VerifyTaskLanding(t.Context(), "missing-task"); err == nil {
		t.Fatal("missing task was presented as landed")
	}
	wrong := b
	wrong.Sequence++
	if _, err := store.LandTask(t.Context(), wrong); err == nil {
		t.Fatal("adapter used an approval from another generation")
	}
	for _, command := range []string{"fetch", "merge-base", "rev-parse", "commit-tree"} {
		t.Run(command, func(t *testing.T) {
			entered := false
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				if args[0] == command {
					entered = true
					return nil, errors.New("Git input or object storage unavailable")
				}
				return proposalGit(ctx, root, input, env, args...)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.LandTask(t.Context(), b); err == nil || !entered || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("Git failure advanced refs: %v", err)
			}
		})
	}
	store.run = proposalGit
	// Open proposals have no presentation or declared approval to land.
	_, proposals, _, _, _, opened := taskReviewFixture(t)
	if _, err := NewTaskLandings(proposals.options, nil).LandTask(t.Context(), codehost.ReviewBinding{ProposalID: opened.Proposal.ID}); err == nil {
		t.Fatal("open proposal was landed")
	}
}

func TestTaskLandingAdapterRechecksReceiptBeforePublishing(t *testing.T) {
	t.Parallel()
	o, store, packet := readyTaskAdapter(t)
	b := packet.Binding
	receipt, err := taskReceiptStore(store.options).Verify(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	changed := receipt.Receipt
	changed.Reviewer = "another declared reviewer"
	body, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := prepareReceipt(t.Context(), o.Root, b.ProposalOID, append(body, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"deleted", "replaced"} {
		t.Run(failure, func(t *testing.T) {
			entered := false
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				out, err := proposalGit(ctx, root, input, env, args...)
				if err == nil && args[0] == "commit-tree" {
					entered = true
					if failure == "deleted" {
						fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef(b))
					} else {
						fixtureGit(t, o.Root, "push", "--force", o.Origin, replacement+":"+receiptRef(b))
					}
				}
				return out, err
			}
			landRefs := []string{proposalPrefix + b.ProposalID, b.Branch, b.TargetRef, "refs/pyxis/proposal-archives/" + b.ProposalID + "/"}
			before, err := proposalRefs(t.Context(), o.Origin, landRefs...)
			if err != nil {
				t.Fatal(err)
			}
			_, landErr := store.LandTask(t.Context(), b)
			after, err := proposalRefs(t.Context(), o.Origin, landRefs...)
			if landErr == nil || !entered || err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("changed receipt advanced landing refs: %v", landErr)
			}
			fixtureGit(t, o.Origin, "update-ref", receiptRef(b), receipt.OID)
		})
	}
}

func TestTaskLandingRefusesPolicyChangedDuringObjectPreparation(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyTaskLanding(t)
	before := fixtureGit(t, o.Origin, "show-ref")
	withoutVerifier := NewTaskLandings(store.options, nil)
	if _, err := withoutVerifier.LandTask(t.Context(), packet.Binding); err == nil {
		t.Fatal("task landed without a current receipt verifier")
	}
	changed := false
	store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
		out, err := proposalGit(ctx, root, input, env, args...)
		if err == nil && args[0] == "commit-tree" {
			reader.policy.Prompts = "changed-during-preparation"
			changed = true
		}
		return out, err
	}
	if result, err := store.LandTask(t.Context(), packet.Binding); err == nil || result.Created || !changed || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("obsolete policy reached atomic landing: %v", err)
	}
}

func TestTaskLandingAdapterRecoversDirectRetriesAndRefusesBrokenProof(t *testing.T) {
	t.Parallel()
	o, store, packet := readyTaskAdapter(t)
	b := packet.Binding
	landed, err := store.LandTask(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	if r, err := store.LandTask(t.Context(), b); err != nil || r.Created || r.Landing.MergeCommit != landed.Landing.MergeCommit {
		t.Fatalf("direct retry did not recover the identical landing: %v", err)
	}
	wrong := b
	wrong.Sequence++
	if _, err := store.LandTask(t.Context(), wrong); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("direct retry accepted a different approval")
	}
	for _, failure := range []string{"parents", "landed tree", "reviewed tree", "feature ancestry"} {
		t.Run(failure, func(t *testing.T) {
			entered := false
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				match := args[0] == "rev-list" && failure == "parents" || args[0] == "merge-base" && failure == "feature ancestry" ||
					args[0] == "rev-parse" && (failure == "landed tree" && args[1] == landed.Landing.MergeCommit+"^{tree}" || failure == "reviewed tree" && args[1] == b.Head+"^{tree}")
				if match {
					entered = true
					return nil, errors.New("retained Git proof unavailable")
				}
				return proposalGit(ctx, root, input, env, args...)
			}
			if _, err := store.VerifyTaskLanding(t.Context(), b.ProposalID); err == nil || !entered || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("incomplete Git proof was accepted: %v", err)
			}
		})
	}
	store.run = proposalGit
	fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef(b))
	before = fixtureGit(t, o.Origin, "show-ref")
	if _, err := store.VerifyTaskLanding(t.Context(), b.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("missing retained approval was repaired or accepted")
	}
}

func TestTaskLandingAdapterRefusesAlteredEmbeddedTrustAndLostAncestry(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"aggregate trust", "receipt trust", "feature ancestry"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			o, store, packet := readyTaskAdapter(t)
			b := packet.Binding
			if _, err := store.LandTask(t.Context(), b); err != nil {
				t.Fatal(err)
			}
			if failure == "feature ancestry" {
				fixtureGit(t, o.Origin, "update-ref", b.TargetRef, b.TargetOID)
			} else {
				final, err := NewProposals(store.options).Read(t.Context(), b.ProposalID)
				if err != nil {
					t.Fatal(err)
				}
				p := final.Proposal
				if failure == "aggregate trust" {
					p.TaskLanding.Aggregate.Trust.ApproverAuthenticated = true
				} else {
					p.TaskLanding.Review.Trust.ApproverAuthenticated = true
				}
				oid, err := prepareProposal(t.Context(), o.Root, p)
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Root, "push", "--atomic", "--force", o.Origin, oid+":"+proposalPrefix+p.ID, oid+":"+p.ArchiveRef)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.VerifyTaskLanding(t.Context(), b.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("altered proof permitted progression: %v", err)
			}
		})
	}
}
