package localgit

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	featurelocal "github.com/agenticlab-ai/pyxis/internal/adapters/featuredrive/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func readyTaskLanding(t *testing.T) (Options, *TaskLandingStore, *taskReviewReader, *reviewgate.TaskLandingGate, reviewgate.Packet) {
	t.Helper()
	o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
	packet := seedTaskReview(t, o, reader, opened)
	seedReceipt(t, o, packet.Binding)
	receipts := NewTaskReceipts(proposals.options, requests)
	store := NewTaskLandings(proposals.options, receipts)
	gate, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, receipts, store)
	if err != nil {
		t.Fatal(err)
	}
	return o, store, reader, gate, packet
}

// The lifecycle test retains the complete preparation and approval workflow.
func readyTaskLandingWorkflow(t *testing.T) (Options, *TaskLandingStore, *taskReviewReader, *reviewgate.TaskLandingGate, reviewgate.Packet) {
	t.Helper()
	o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipts := NewTaskReceipts(proposals.options, requests)
	if _, err := receipts.Record(t.Context(), packet.Binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	store := NewTaskLandings(proposals.options, receipts)
	gate, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, receipts, store)
	if err != nil {
		t.Fatal(err)
	}
	return o, store, reader, gate, packet
}

type taskLandingRecorder struct{ calls int }

func (r *taskLandingRecorder) RecordLanding(context.Context, string, string, featuredrive.LandingProof) error {
	r.calls++
	return nil
}

func taskLandingObservation(packet reviewgate.Packet, result codehost.TaskLandingResult) (featuredrive.ProposalObservation, featuredrive.ReviewObservation) {
	b := packet.Binding
	return featuredrive.ProposalObservation{TaskID: result.Landing.Task, ProposalID: b.ProposalID, Head: b.Head, Base: b.TargetRef, Landed: result.Landing.MergeCommit, ChecksGreen: true},
		featuredrive.ReviewObservation{ProposalID: b.ProposalID, Head: b.Head, State: "approved"}
}

func TestTaskLandingRetainsExactSquashAndProofAfterCheckoutDeletion(t *testing.T) {
	t.Parallel()
	o, store, reader, gate, packet := readyTaskLandingWorkflow(t)
	main := fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk")
	result, err := gate.LandTask(t.Context(), packet.Binding)
	if err != nil || !result.Created || result.Landing.Validate() != nil {
		t.Fatalf("task landing: %+v %v", result, err)
	}
	b, l := packet.Binding, result.Landing
	if fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", l.MergeCommit) != l.MergeCommit+" "+b.TargetOID ||
		fixtureGit(t, o.Origin, "rev-parse", l.MergeCommit+"^{tree}") != fixtureGit(t, o.Origin, "rev-parse", b.Head+"^{tree}") ||
		fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk") != main ||
		!reflect.DeepEqual(l.Aggregate, packet.Aggregate) || !reflect.DeepEqual(l.Request, packet.Request) {
		t.Fatal("landing changed the reviewed parent, tree, or retained proof")
	}
	final, err := NewProposals(store.options).Read(t.Context(), b.ProposalID)
	if err != nil || final.Proposal.PreviousOID != b.ProposalOID || final.Proposal.Sequence != b.Sequence+1 || final.Proposal.TaskPreparation != nil {
		t.Fatalf("reviewed predecessor not retained: %+v %v", final, err)
	}
	refs, err := proposalRefs(t.Context(), o.Origin, b.Branch, final.Proposal.ArchiveRef)
	if err != nil || refs[b.Branch] != "" || refs[final.Proposal.ArchiveRef] != final.OID {
		t.Fatal("landing did not delete the branch and archive its final state")
	}
	options := store.options
	options.Git = proposalClone(t, o)
	if err := os.RemoveAll(o.Root); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "gc", "--prune=now")
	reader.failSnapshot = true
	readsBeforeRecovery := reader.snapshots
	proposals, artifacts := NewProposals(options), NewReviewArtifacts(options)
	requests, err := reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, reader)
	if err != nil {
		t.Fatal(err)
	}
	receipts := NewTaskReceipts(options, requests)
	clone, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, receipts, NewTaskLandings(options, receipts))
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	recovered, err := clone.LandTask(t.Context(), b)
	if err != nil || recovered.Created || !reflect.DeepEqual(recovered.Landing, l) || reader.snapshots != readsBeforeRecovery || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("recovery performed another landing or review: %+v %v", recovered, err)
	}
	proposal, review := taskLandingObservation(packet, recovered)
	recorder := &taskLandingRecorder{}
	verifier := featurelocal.NewTaskLandingVerifier(clone)
	if err := featuredrive.RecordLanding(t.Context(), verifier, recorder, proposal, review, recovered.FeatureHead); err != nil || recorder.calls != 1 {
		t.Fatalf("verified landed task did not progress: %v", err)
	}
	fixtureGit(t, o.Origin, "update-ref", "-d", b.Evidence.Ref)
	if err := featuredrive.RecordLanding(t.Context(), verifier, recorder, proposal, review, recovered.FeatureHead); err == nil || recorder.calls != 1 {
		t.Fatalf("missing proof unlocked a dependent: %v", err)
	}
}

func TestTaskLandingRefusesChangedPolicyReceiptAndGitInputs(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"policy", "check rerun red", "aggregate changed", "receipt", "aggregate", "request", "target", "branch", "binding", "cancelled"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			o, _, reader, gate, packet := readyTaskLanding(t)
			binding := packet.Binding
			ctx := t.Context()
			switch change {
			case "policy":
				reader.policy.Prompts = "changed-prompts"
			case "check rerun red":
				reader.mutate = func(s *reviewgate.Snapshot) { s.Checks[0].State = port.CheckRed }
			case "aggregate changed":
				reader.mutate = func(s *reviewgate.Snapshot) {
					s.Review.Aggregate.Report.Rounds[0].ContextID += "-new"
					body, err := s.Review.Aggregate.Report.Marshal()
					if err != nil {
						t.Fatal(err)
					}
					s.Review.DiagnosticDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(body))
				}
			case "receipt":
				fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef(binding))
			case "aggregate":
				fixtureGit(t, o.Origin, "update-ref", "-d", binding.Evidence.Ref)
			case "request":
				fixtureGit(t, o.Origin, "update-ref", "-d", binding.Request.Ref)
			case "target":
				fixtureGit(t, o.Origin, "update-ref", binding.TargetRef, binding.Head)
			case "branch":
				fixtureGit(t, o.Origin, "update-ref", binding.Branch, binding.TargetOID)
			case "binding":
				binding.Nonce = strings.Repeat("f", 32)
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := gate.LandTask(ctx, binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("invalid landing mutated refs: %v", err)
			}
		})
	}
}

func TestTaskLandingAtomicRefusalLostAcknowledgementAndProofLeases(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"target race", "branch race", "atomic unavailable", "lost acknowledgement", "proof leases"} {
		t.Run(change, func(t *testing.T) {
			o, store, packet := readyTaskAdapter(t)
			b := packet.Binding
			before, err := proposalRefs(t.Context(), o.Origin, proposalPrefix+b.ProposalID)
			if err != nil {
				t.Fatal(err)
			}
			store.push = func(ctx context.Context, root string, args ...string) error {
				switch change {
				case "target race":
					fixtureGit(t, o.Origin, "update-ref", b.TargetRef, b.Head)
				case "branch race":
					fixtureGit(t, o.Origin, "update-ref", b.Branch, b.TargetOID)
				case "atomic unavailable":
					return errors.New("the receiving end does not support --atomic push")
				case "proof leases":
					for _, ref := range []string{b.Evidence.Ref, b.Request.Ref, receiptRef(b)} {
						if _, err := proposalGit(ctx, o.Origin, nil, nil, "update-ref", "-d", ref); err == nil {
							t.Fatalf("landing did not hold proof ref %s", ref)
						}
					}
				}
				_, err := proposalGit(ctx, root, nil, nil, args...)
				if err == nil && change == "lost acknowledgement" {
					return errors.New("lost landing reply")
				}
				return err
			}
			result, err := store.LandTask(t.Context(), b)
			if change == "lost acknowledgement" || change == "proof leases" {
				if err != nil || (change == "lost acknowledgement" && result.Created) {
					t.Fatalf("complete retained landing was not recovered: %v", err)
				}
				return
			}
			want := codehost.ErrProposalConflict
			if change == "atomic unavailable" {
				want = codehost.ErrAtomicUnavailable
			}
			if !errors.Is(err, want) {
				t.Fatalf("invalid atomic result: %v", err)
			}
			after, err := proposalRefs(t.Context(), o.Origin, proposalPrefix+b.ProposalID, "refs/pyxis/proposal-archives/"+b.ProposalID+"/")
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("failed atomic transaction advanced proposal or archive")
			}
		})
	}
}

func TestTaskLandingParallelRootsRequireFreshRebaseReviewAndReceipt(t *testing.T) {
	t.Parallel()
	for _, first := range []string{"T1", "T2"} {
		t.Run(first+" first", func(t *testing.T) {
			t.Parallel()
			o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
			base := opened.Proposal.TargetOID
			writeTask := func(file string) string {
				if err := os.WriteFile(filepath.Join(o.Root, file), []byte(file+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Root, "add", file)
				fixtureGit(t, o.Root, "commit", "-m", "task "+file)
				return fixtureGit(t, o.Root, "rev-parse", "HEAD")
			}
			one, err := proposals.Publish(t.Context(), revision(opened, writeTask("one.txt")))
			if err != nil {
				t.Fatal(err)
			}
			fixtureGit(t, o.Root, "switch", "--detach", base)
			two, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: "task-widget-t2", Feature: "widget", Task: "T2", Branch: "refs/heads/task-widget-t2", Head: writeTask("two.txt"), State: "open", ExpectedTargetOID: base})
			if err != nil {
				t.Fatal(err)
			}
			receipts := NewTaskReceipts(proposals.options, requests)
			packets := map[string]reviewgate.Packet{}
			for _, g := range []codehost.ProposalGeneration{one, two} {
				packet, err := requests.Prepare(t.Context(), g.Proposal.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := receipts.Record(t.Context(), packet.Binding, "operator", ""); err != nil {
					t.Fatal(err)
				}
				packets[g.Proposal.Task] = packet
			}
			gate, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, receipts, NewTaskLandings(proposals.options, receipts))
			if err != nil {
				t.Fatal(err)
			}
			landed, err := gate.LandTask(t.Context(), packets[first].Binding)
			if err != nil {
				t.Fatal(err)
			}
			second := "T2"
			if first == "T2" {
				second = "T1"
			}
			stale := packets[second]
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := gate.LandTask(t.Context(), stale.Binding); !errors.Is(err, codehost.ErrProposalConflict) || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("stale parallel approval was used: %v", err)
			}
			fixtureGit(t, o.Root, "fetch", "--no-tags", o.Origin, landed.Landing.MergeCommit)
			fixtureGit(t, o.Root, "switch", "--detach", stale.Binding.Head)
			fixtureGit(t, o.Root, "rebase", "--onto", landed.Landing.MergeCommit, base)
			old, err := proposals.Read(t.Context(), stale.Binding.ProposalID)
			if err != nil {
				t.Fatal(err)
			}
			change := revision(old, fixtureGit(t, o.Root, "rev-parse", "HEAD"))
			change.ExpectedTargetOID = landed.Landing.MergeCommit
			if _, err := proposals.Publish(t.Context(), change); err != nil {
				t.Fatal(err)
			}
			fresh, err := requests.Prepare(t.Context(), stale.Binding.ProposalID)
			if err != nil || fresh.Binding.Nonce == stale.Binding.Nonce || fresh.Binding.ProposalOID == stale.Binding.ProposalOID {
				t.Fatalf("rebase reused approval generation: %v", err)
			}
			if _, err := gate.LandTask(t.Context(), fresh.Binding); !errors.Is(err, codehost.ErrReceiptNotFound) {
				t.Fatalf("rebase landed before new approval: %v", err)
			}
			if _, err := receipts.Record(t.Context(), fresh.Binding, "operator", ""); err != nil {
				t.Fatal(err)
			}
			last, err := gate.LandTask(t.Context(), fresh.Binding)
			if err != nil || fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", last.Landing.MergeCommit) != last.Landing.MergeCommit+" "+landed.Landing.MergeCommit {
				t.Fatalf("parallel rebase lost earlier landing: %v", err)
			}
			for _, file := range []string{"one.txt", "two.txt"} {
				if fixtureGit(t, o.Origin, "show", last.Landing.MergeCommit+":"+file) != file {
					t.Fatal("parallel landing lost task content")
				}
			}
		})
	}
}
