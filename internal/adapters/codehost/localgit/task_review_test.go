package localgit

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// The reader supplies completed host artifacts, never a fake summary-only
// gate. Native execution and runtime composition are separate acceptance work.
type taskReviewReader struct {
	policy       reviewgate.Policy
	snapshots    int
	failSnapshot bool
	mutate       func(*reviewgate.Snapshot)
	policyOrigin string
	policyRef    string
	policyOID    string
	beforeLease  func()
}

func (r *taskReviewReader) Policy(context.Context, reviewgate.Binding) (reviewgate.Policy, error) {
	return r.policy, nil
}

func (r *taskReviewReader) WithSnapshot(ctx context.Context, binding reviewgate.Binding, expected reviewgate.Snapshot, mutate func() error) error {
	if r.beforeLease != nil {
		r.beforeLease()
	}
	current, err := r.Snapshot(ctx, binding)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, expected) || !reflect.DeepEqual(r.policy, expected.Policy) || r.policyOrigin == "" {
		return reviewgate.ErrPolicyStale
	}
	// This fixture's version covers policy, aggregate and check sources. The
	// real Git lease prevents source publication during the callback.
	return withUnchangedRefs(ctx, r.policyOrigin, []unchangedRef{{r.policyRef, r.policyOID}}, mutate)
}

func (r *taskReviewReader) Snapshot(_ context.Context, b reviewgate.Binding) (reviewgate.Snapshot, error) {
	r.snapshots++
	if r.failSnapshot {
		return reviewgate.Snapshot{}, errors.New("native process unavailable")
	}
	review := &report.Report{SchemaVersion: report.SchemaVersion, ProposalRef: b.Subject(), ReviewID: "native-process-" + b.ProposalOID, Band: r.policy.ReviewBand, Verdict: report.VerdictApprove,
		Rounds: []report.Round{{ReviewedRevision: b.Head, ContextID: "native-context", Provenance: report.Provenance{Model: r.policy.NativeModel, PromptsVersion: r.policy.Prompts, CalVersion: calibration.Version}}},
	}
	body, err := review.Marshal()
	if err != nil {
		return reviewgate.Snapshot{}, err
	}
	snapshot := reviewgate.Snapshot{SchemaVersion: reviewgate.SnapshotSchema, Binding: b, Policy: r.policy,
		Review:   implreview.AggregateHandoff{Status: implreview.HandoffCompleted, DiagnosticDigest: fmt.Sprintf("sha256:%x", sha256.Sum256(body)), Aggregate: &implreview.CompletedAggregate{ProcessID: review.ReviewID, ProposalID: b.Subject(), ReviewedRevision: b.Head, Report: review}},
		CodeGate: reviewgate.FreezeCodeGateReport(codegatemirror.Report{Passed: true, Outcomes: []codegatemirror.Outcome{{Name: "task-test", Class: codegatemirror.ClassRequired, Passed: true}}}),
		Checks:   []reviewgate.Check{{Name: "task-policy", State: port.CheckGreen}},
	}
	if r.mutate != nil {
		r.mutate(&snapshot)
	}
	return snapshot, nil
}

func taskReviewFixture(t *testing.T) (Options, *ProposalStore, *ReviewArtifactStore, *taskReviewReader, *reviewgate.TaskRequests, codehost.ProposalGeneration) {
	t.Helper()
	o, proposals, change := proposalFixture(t)
	proposals.options.Kind, proposals.options.IntegrationRef = "implementation", "refs/heads/feature/widget"
	fixtureGit(t, o.Origin, "update-ref", proposals.options.IntegrationRef, change.ExpectedTargetOID)
	change.ID, change.Task = "task-widget-t1", "T1"
	opened, err := proposals.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := os.ReadFile("../../../../.pyxis/skills/impl-review/calibration.md")
	if err != nil {
		t.Fatal(err)
	}
	reader := &taskReviewReader{policy: reviewgate.Policy{SchemaVersion: reviewgate.PolicySchema, Required: []string{"task-policy"}, ReviewBand: report.BandStandard, NativeModel: "native-test-model", Prompts: "prompts-v1", Calibration: string(cal), CodeGate: reviewgate.FreezeCodeGatePolicy(codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "task-test", Command: "go test ./...", Class: codegatemirror.ClassRequired}}})}}
	reader.policyOrigin, reader.policyRef, reader.policyOID = o.Origin, "refs/pyxis/test-review-policy", change.ExpectedTargetOID
	fixtureGit(t, o.Origin, "update-ref", reader.policyRef, reader.policyOID)
	artifacts := NewReviewArtifacts(proposals.options)
	requests, err := reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, reader)
	if err != nil {
		t.Fatal(err)
	}
	return o, proposals, artifacts, reader, requests, opened
}

func TestTaskReviewRetainsCompletePacketAndReceiptsAfterCheckoutDeletion(t *testing.T) {
	t.Parallel()
	o, proposals, _, reader, requests, opened := taskReviewFixture(t)
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Binding.Validate() != nil || packet.Binding.Sequence != 2 || !packet.Trust.SharedAccount || packet.Trust.ApproverAuthenticated || packet.Trust.MainlineProtected || reader.snapshots != 2 {
		t.Fatalf("wrong retained packet: %+v", packet)
	}
	if fixtureGit(t, o.Origin, "rev-parse", proposals.targetRef()) != opened.Proposal.TargetOID {
		t.Fatal("review preparation moved the feature target")
	}
	for _, artifact := range []codehost.ReviewArtifact{packet.Aggregate, packet.Request} {
		if fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", artifact.Identity.OID) != artifact.Identity.OID+" "+packet.Binding.ProposalOID {
			t.Fatal("retained object does not parent the prepared generation")
		}
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	resumed, err := requests.Prepare(t.Context(), opened.Proposal.ID)
	if err != nil || !reflect.DeepEqual(resumed, packet) || reader.snapshots != 3 || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("resume replaced a prepared packet: %v", err)
	}
	options := proposals.options
	options.Git = proposalClone(t, o)
	if err := os.RemoveAll(o.Root); err != nil {
		t.Fatal(err)
	}
	reader.failSnapshot = true
	clone, err := reviewgate.NewTaskRequests(t.Context(), NewProposals(options), NewReviewArtifacts(options), reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clone.Observe(t.Context(), opened.Proposal.ID); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("unavailable current aggregate reused retained approval proof")
	}
	reader.failSnapshot = false
	recovered, err := clone.Observe(t.Context(), opened.Proposal.ID)
	if err != nil || !reflect.DeepEqual(recovered, packet) || reader.snapshots != 5 {
		t.Fatalf("fresh clone cannot recover complete packet: %v", err)
	}
	receipts := NewTaskReceipts(options, clone)
	binding, err := receipts.Binding(t.Context(), opened.Proposal.ID)
	if err != nil || binding != packet.Binding {
		t.Fatalf("receipt is not bound to retained request/proof: %+v %v", binding, err)
	}
	receipt, err := receipts.Record(t.Context(), binding, "declared operator", "approved complete packet")
	if err != nil {
		t.Fatal(err)
	}
	if verified, err := receipts.Verify(t.Context(), binding); err != nil || verified != receipt {
		t.Fatalf("fresh retained receipt is unverifiable: %v", err)
	}
	before = fixtureGit(t, o.Origin, "show-ref")
	wrong := binding
	wrong.Nonce = strings.Repeat("f", 32)
	if _, err := receipts.Record(t.Context(), wrong, "declared operator", ""); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("unpresented nonce obtained a receipt: %v", err)
	}
}

func TestTaskReviewInterruptedPreparationAndLostRequestAcknowledgement(t *testing.T) {
	t.Parallel()
	for _, lostAcknowledgement := range []bool{false, true} {
		t.Run(fmt.Sprint(lostAcknowledgement), func(t *testing.T) {
			t.Parallel()
			o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
			interrupted := false
			artifacts.commit = func(ctx context.Context, origin string, input []byte) error {
				isRequest := bytesContainRequest(input)
				if isRequest && !interrupted {
					interrupted = true
					if !lostAcknowledgement {
						return errors.New("request publication interrupted")
					}
					if _, err := proposalGit(ctx, origin, input, nil, "update-ref", "--stdin"); err != nil {
						return err
					}
					return errors.New("request acknowledgement lost")
				}
				_, err := proposalGit(ctx, origin, input, nil, "update-ref", "--stdin")
				return err
			}
			if _, err := requests.Prepare(t.Context(), opened.Proposal.ID); err == nil {
				t.Fatal("interrupted request was presented")
			}
			prepared, err := proposals.Read(t.Context(), opened.Proposal.ID)
			if err != nil {
				t.Fatal(err)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
			if err != nil {
				t.Fatal(err)
			}
			if lostAcknowledgement {
				if packet.Binding.ProposalOID != prepared.OID || packet.Binding.Nonce != prepared.Proposal.TaskPreparation.Nonce || reader.snapshots != 2 || fixtureGit(t, o.Origin, "show-ref") != before {
					t.Fatal("lost response replaced the origin request")
				}
			} else if packet.Binding.Sequence != prepared.Proposal.Sequence+1 || packet.Binding.Nonce == prepared.Proposal.TaskPreparation.Nonce || reader.snapshots != 3 {
				t.Fatal("incomplete preparation reused its old generation or nonce")
			}
		})
	}
}

func bytesContainRequest(input []byte) bool {
	return strings.Contains(string(input), "create refs/pyxis/review-requests/")
}

func TestTaskReviewIncompleteGateAndChangedPolicyRefusePresentation(t *testing.T) {
	t.Parallel()
	o, _, _, reader, requests, opened := taskReviewFixture(t)
	reader.mutate = func(s *reviewgate.Snapshot) { s.CodeGate.Outcomes = nil }
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID); !errors.Is(err, reviewgate.ErrIncomplete) || fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/review-aggregates", "refs/pyxis/review-requests") != "" {
		t.Fatalf("partial gate retained reviewable proof: %v", err)
	}
	reader.mutate = nil
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	reader.policy.Prompts = "new-prompts"
	if _, err := requests.Observe(t.Context(), opened.Proposal.ID); !errors.Is(err, reviewgate.ErrIncomplete) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("old policy remained reviewable: %v", err)
	}
	if packet.Binding.Sequence != 3 || reader.snapshots != 3 {
		t.Fatal("failed preparation did not require a fresh generation")
	}
}

func TestTaskReviewMissingOrMovedRetainedObjectsBlockReceiptVerification(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{codehost.TaskAggregate, codehost.TaskRequest} {
		for _, remove := range []bool{true, false} {
			t.Run(kind+fmt.Sprint(remove), func(t *testing.T) {
				t.Parallel()
				o, proposals, _, reader, requests, opened := taskReviewFixture(t)
				packet := seedTaskReview(t, o, reader, opened)
				receipts := NewTaskReceipts(proposals.options, requests)
				seedReceipt(t, o, packet.Binding)
				ref := packet.Aggregate.Identity.Ref
				if kind == codehost.TaskRequest {
					ref = packet.Request.Identity.Ref
				}
				if remove {
					fixtureGit(t, o.Origin, "update-ref", "-d", ref)
				} else {
					fixtureGit(t, o.Origin, "update-ref", ref, opened.Proposal.Head)
				}
				before := fixtureGit(t, o.Origin, "show-ref")
				if _, err := requests.Observe(t.Context(), opened.Proposal.ID); err == nil {
					t.Fatal("missing or moved proof remained presentable")
				}
				if _, err := receipts.Verify(t.Context(), packet.Binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
					t.Fatalf("altered retained proof remained approved: %v", err)
				}
			})
		}
	}
}
