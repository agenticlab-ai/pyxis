package localgit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func retainedTaskFixture(t *testing.T) (Options, *ProposalStore, *ReviewArtifactStore, codehost.ProposalGeneration, codehost.ReviewArtifact, codehost.ReviewArtifact) {
	t.Helper()
	o, proposals, artifacts, reader, _, opened := taskReviewFixture(t)
	packet := seedTaskReview(t, o, reader, opened)
	g, err := proposals.Read(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	return o, proposals, artifacts, g, packet.Aggregate, packet.Request
}

func TestReviewArtifactsIdempotentPublicationAndClosedInputs(t *testing.T) {
	t.Parallel()
	o, _, artifacts, g, aggregate, request := retainedTaskFixture(t)
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, record := range []codehost.ReviewArtifact{aggregate, request} {
		got, err := artifacts.Retain(t.Context(), g, record.Kind, record.Body)
		if err != nil || !reflect.DeepEqual(got, record) {
			t.Fatalf("identical publication was not idempotent: %v", err)
		}
	}
	for _, body := range [][]byte{nil, []byte("{"), []byte(strings.Repeat("x", codehost.MaxReviewArtifactBytes+1))} {
		if _, err := artifacts.Retain(t.Context(), g, codehost.TaskRequest, body); err == nil {
			t.Fatal("unbounded or malformed request published")
		}
	}
	if _, err := artifacts.Retain(t.Context(), g, "unknown", aggregate.Body); err == nil {
		t.Fatal("unsupported artifact kind published")
	}
	wrong, err := codehost.ParseTaskRequest(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	wrong.Binding.Nonce = strings.Repeat("f", 32)
	body, err := wrong.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.Retain(t.Context(), g, codehost.TaskRequest, body); err == nil {
		t.Fatal("unpresented nonce published")
	}
	if _, err := artifacts.ReadRetained(t.Context(), g, codehost.TaskRequest, codehost.ArtifactRef{}); err == nil {
		t.Fatal("absent identity accepted")
	}
	if _, err := artifacts.ReadRetained(t.Context(), g, "unknown", aggregate.Identity); err == nil {
		t.Fatal("unknown retained kind accepted")
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("refused or idempotent publication changed origin refs")
	}
	fixtureGit(t, o.Origin, "update-ref", request.Identity.Ref, g.Proposal.Head)
	before = fixtureGit(t, o.Origin, "show-ref")
	if _, err := artifacts.Retain(t.Context(), g, codehost.TaskRequest, request.Body); !errors.Is(err, codehost.ErrReviewInvalid) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("immutable request was replaced: %v", err)
	}
	fixtureGit(t, o.Origin, "update-ref", "-d", aggregate.Identity.Ref)
	if _, err := artifacts.Retain(t.Context(), g, codehost.TaskRequest, request.Body); err == nil {
		t.Fatal("request published without its retained aggregate")
	}
}

func TestReviewArtifactsRequireTheConfiguredPreparedTask(t *testing.T) {
	t.Parallel()
	o, proposals, artifacts, g, aggregate, _ := retainedTaskFixture(t)
	for _, change := range []func(*ProposalOptions){
		func(o *ProposalOptions) { o.Kind = "draft" },
		func(o *ProposalOptions) { o.IntegrationRef = "refs/heads/" + o.Git.Mainline },
		func(o *ProposalOptions) { o.HeadRef = "refs/heads/feature/source" },
		func(o *ProposalOptions) { o.Git.Root = t.TempDir() },
	} {
		options := proposals.options
		change(&options)
		if _, err := NewReviewArtifacts(options).Retain(t.Context(), g, codehost.TaskAggregate, aggregate.Body); err == nil {
			t.Fatal("unbound artifact store accepted the task")
		}
	}
	for _, change := range []func(*codehost.ProposalGeneration){
		func(g *codehost.ProposalGeneration) { g.Proposal.State = "open" },
		func(g *codehost.ProposalGeneration) { g.Proposal.Sequence++ },
		func(g *codehost.ProposalGeneration) { g.Proposal.TaskPreparation = nil },
	} {
		bad := g
		change(&bad)
		if _, err := artifacts.Retain(t.Context(), bad, codehost.TaskAggregate, aggregate.Body); err == nil {
			t.Fatal("wrong generation published proof")
		}
		if _, err := artifacts.ReadRequest(t.Context(), bad); err == nil {
			t.Fatal("wrong generation read a request")
		}
		if _, err := artifacts.ReadRetained(t.Context(), bad, codehost.TaskAggregate, aggregate.Identity); err == nil {
			t.Fatal("wrong generation read retained proof")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := artifacts.Retain(ctx, g, codehost.TaskAggregate, aggregate.Body); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("cancelled artifact publication changed refs")
	}
}

func TestReviewArtifactRequestLeasesEveryPreparedInputAtPublication(t *testing.T) {
	t.Parallel()
	for _, moved := range []string{"proposal", "branch", "target", "evidence"} {
		t.Run(moved, func(t *testing.T) {
			o, proposals, artifacts, _, requests, opened := taskReviewFixture(t)
			artifacts.commit = func(ctx context.Context, origin string, input []byte) error {
				if bytesContainRequest(input) {
					switch moved {
					case "proposal":
						fixtureGit(t, origin, "update-ref", proposalPrefix+opened.Proposal.ID, opened.OID)
					case "branch":
						fixtureGit(t, origin, "update-ref", opened.Proposal.Branch, opened.Proposal.TargetOID)
					case "target":
						fixtureGit(t, origin, "update-ref", proposals.targetRef(), opened.Proposal.Head)
					case "evidence":
						ref := fixtureGit(t, origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/review-aggregates/")
						fixtureGit(t, origin, "update-ref", ref, opened.Proposal.Head)
					}
				}
				_, err := proposalGit(ctx, origin, input, nil, "update-ref", "--stdin")
				return err
			}
			if _, err := requests.Prepare(t.Context(), opened.Proposal.ID); !errors.Is(err, codehost.ErrProposalConflict) {
				t.Fatalf("changed input accepted: %s %v", moved, err)
			}
			if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/review-requests/", "refs/pyxis/receipts/") != "" {
				t.Fatal("lease failure published request or approval")
			}
		})
	}
}

func TestReviewArtifactCorruptObjectsRefuseClosedReads(t *testing.T) {
	t.Parallel()
	o, _, artifacts, g, _, request := retainedTaskFixture(t)
	for _, corrupt := range []string{"missing file", "oversized", "malformed", "wrong nonce", "wrong parent", "extra file", "wrong digest"} {
		t.Run(corrupt, func(t *testing.T) {
			body, parent, file := request.Body, g.OID, "request.json"
			switch corrupt {
			case "missing file":
				file = "other.json"
			case "oversized":
				body = []byte(strings.Repeat("x", codehost.MaxReviewArtifactBytes+1))
			case "malformed":
				body = []byte("{}\n")
			case "wrong nonce":
				r, err := codehost.ParseTaskRequest(body)
				if err != nil {
					t.Fatal(err)
				}
				r.Binding.Nonce = strings.Repeat("f", 32)
				body, err = r.Canonical()
				if err != nil {
					t.Fatal(err)
				}
			case "wrong parent":
				parent = g.Proposal.PreviousOID
			case "wrong digest":
				identity := request.Identity
				identity.Digest = "sha256:" + strings.Repeat("0", 64)
				if _, err := artifacts.ReadRetained(t.Context(), g, codehost.TaskRequest, identity); err == nil {
					t.Fatal("wrong digest accepted")
				}
				return
			}
			oid, err := prepareReviewArtifact(t.Context(), o.Root, parent, codehost.TaskRequest, file, body)
			if err != nil {
				t.Fatal(err)
			}
			if corrupt == "extra file" {
				blob := fixtureGit(t, o.Root, "rev-parse", oid+":request.json")
				tree, err := proposalGit(t.Context(), o.Root, fmt.Appendf(nil, "100644 blob %s\trequest.json\n100644 blob %s\textra.json\n", blob, blob), nil, "mktree")
				if err != nil {
					t.Fatal(err)
				}
				oid = fixtureGit(t, o.Root, "commit-tree", strings.TrimSpace(string(tree)), "-p", g.OID, "-m", "corrupt extra file")
			}
			fixtureGit(t, o.Origin, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, oid)
			fixtureGit(t, o.Origin, "update-ref", request.Identity.Ref, oid)
			if _, err := artifacts.ReadRequest(t.Context(), g); err == nil {
				t.Fatalf("corrupt retained request accepted: %s", corrupt)
			}
			fixtureGit(t, o.Origin, "update-ref", request.Identity.Ref, request.Identity.OID)
		})
	}
}

func TestTaskReceiptLeasesRetainedRequestAndCompleteEvidence(t *testing.T) {
	t.Parallel()
	for _, moved := range []string{"request", "aggregate"} {
		t.Run(moved, func(t *testing.T) {
			o, proposals, _, _, requests, opened := taskReviewFixture(t)
			packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
			if err != nil {
				t.Fatal(err)
			}
			receipts := NewTaskReceipts(proposals.options, requests)
			receipts.commit = func(ctx context.Context, origin string, body []byte) error {
				ref := packet.Request.Identity.Ref
				if moved == "aggregate" {
					ref = packet.Aggregate.Identity.Ref
				}
				fixtureGit(t, origin, "update-ref", ref, opened.Proposal.Head)
				_, err := proposalGit(ctx, origin, body, nil, "update-ref", "--stdin")
				return err
			}
			if _, err := receipts.Record(t.Context(), packet.Binding, "operator", ""); !errors.Is(err, codehost.ErrReceiptConflict) {
				t.Fatalf("moved retained proof approved: %v", err)
			}
			if fixtureGit(t, o.Origin, "for-each-ref", "--format=%(refname)", "refs/pyxis/receipts/") != "" {
				t.Fatal("failed proof lease left a receipt")
			}
		})
	}
}

func TestTaskReviewMovedTargetBlocksResumedPacketsAndReceipts(t *testing.T) {
	t.Parallel()
	o, proposals, artifacts, _, requests, opened := taskReviewFixture(t)
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipts := NewTaskReceipts(proposals.options, requests)
	if _, err := receipts.Record(t.Context(), packet.Binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "update-ref", proposals.targetRef(), opened.Proposal.Head)
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := requests.Observe(t.Context(), opened.Proposal.ID); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("moved target remained reviewable: %v", err)
	}
	if _, err := requests.Prepare(t.Context(), opened.Proposal.ID); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("moved target reused old preparation: %v", err)
	}
	if _, err := receipts.Verify(t.Context(), packet.Binding); !errors.Is(err, codehost.ErrProposalConflict) {
		t.Fatalf("moved target retained current approval: %v", err)
	}
	// Historical proof remains readable for the later landed-result verifier;
	// its presence is never current eligibility to perform another landing.
	g, err := proposals.read(t.Context(), o.Origin, opened.Proposal.ID, packet.Binding.ProposalOID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.ReadRetained(t.Context(), g, codehost.TaskAggregate, packet.Aggregate.Identity); err != nil {
		t.Fatalf("historical proof was lost: %v", err)
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("refused stale review changed refs")
	}
}
