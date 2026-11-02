package localgit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

// Seed helpers establish a test's preconditions directly in its private real
// Git repository. Tests of Prepare, Retain and Record still call those public
// operations; landing and corruption tests need not replay all three workflows
// just to obtain their input. No reads or mutations under test are cached.
func seedPreparedProposal(t *testing.T, o Options, previous codehost.ProposalGeneration, p codehost.Proposal) codehost.ProposalGeneration {
	t.Helper()
	p.Sequence, p.PreviousOID, p.State = previous.Proposal.Sequence+1, previous.OID, "prepared"
	oid, err := prepareProposal(t.Context(), o.Origin, p)
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "update-ref", proposalPrefix+p.ID, oid, previous.OID)
	fixtureGit(t, o.Root, "fetch", "--no-tags", "--no-write-fetch-head", o.Origin, oid)
	return codehost.ProposalGeneration{OID: oid, Proposal: p}
}

func seedReviewArtifact(t *testing.T, origin string, g codehost.ProposalGeneration, kind string, body []byte) codehost.ReviewArtifact {
	t.Helper()
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	ref, file, err := artifactLocation(g, kind, digest)
	if err != nil {
		t.Fatal(err)
	}
	oid, err := prepareReviewArtifact(t.Context(), origin, g.OID, kind, file, body)
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, origin, "update-ref", ref, oid)
	return codehost.ReviewArtifact{
		Identity: codehost.ArtifactRef{Ref: ref, OID: oid, Digest: digest},
		Kind:     kind, Body: body, Trust: codehost.Trust{SharedAccount: true},
	}
}

func seedTaskReview(t *testing.T, o Options, reader *taskReviewReader, opened codehost.ProposalGeneration) reviewgate.Packet {
	t.Helper()
	preparation, err := codehost.NewTaskPreparation()
	if err != nil {
		t.Fatal(err)
	}
	p := opened.Proposal
	p.TaskPreparation = &preparation
	g := seedPreparedProposal(t, o, opened, p)
	binding, err := reviewgate.Bind(o.Origin, g)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := reader.Snapshot(t.Context(), binding)
	if err != nil {
		t.Fatal(err)
	}
	body, err := reviewgate.Canonical(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := seedReviewArtifact(t, o.Origin, g, codehost.TaskAggregate, body)
	// Retained evidence must not alias the mutable reader's policy slices.
	snapshot, err = reviewgate.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	p = g.Proposal
	request := codehost.TaskReviewRequest{SchemaVersion: codehost.TaskRequestSchema, Binding: codehost.ReviewBinding{
		SchemaVersion: codehost.ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence,
		Operation: preparation.Operation, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID,
		Digest: aggregate.Identity.Digest, Nonce: preparation.Nonce, Evidence: aggregate.Identity,
	}}
	body, err = request.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	retained := seedReviewArtifact(t, o.Origin, g, codehost.TaskRequest, body)
	request.Binding.Request = retained.Identity
	return reviewgate.Packet{SchemaVersion: reviewgate.PacketSchema, Binding: request.Binding, Snapshot: snapshot,
		Aggregate: aggregate, Request: retained, Trust: codehost.Trust{SharedAccount: true}}
}

func seedCandidateReview(t *testing.T, o Options, reader *candidateReviewSource, opened codehost.ProposalGeneration) candidatereview.Packet {
	t.Helper()
	candidate, scenarios := codehost.FinalCandidate(reader.plan.Candidate), reader.plan.ScenarioSetFingerprint
	preparation, err := codehost.NewCandidatePreparation(candidate, scenarios)
	if err != nil {
		t.Fatal(err)
	}
	p := opened.Proposal
	p.CandidatePreparation = &preparation
	fixtureGit(t, o.Origin, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, candidate.Commit)
	g := seedPreparedProposal(t, o, opened, p)
	binding, err := candidatereview.Bind(o.Origin, g)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := candidatereview.Capture(t.Context(), binding, reader)
	if err != nil {
		t.Fatal(err)
	}
	body, err := snapshot.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	evidence := seedReviewArtifact(t, o.Origin, g, codehost.CandidateEvidence, body)
	snapshot, err = candidatereview.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	p = g.Proposal
	request := codehost.CandidateReviewRequest{SchemaVersion: codehost.CandidateRequestSchema, Binding: codehost.ReviewBinding{
		SchemaVersion: codehost.ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence,
		Operation: preparation.Operation, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID,
		Digest: evidence.Identity.Digest, Nonce: preparation.Nonce, Evidence: evidence.Identity, Candidate: candidate, ScenarioSetFingerprint: scenarios,
	}}
	body, err = request.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	retained := seedReviewArtifact(t, o.Origin, g, codehost.CandidateRequest, body)
	request.Binding.Request = retained.Identity
	return candidatereview.Packet{SchemaVersion: candidatereview.PacketSchema, Binding: request.Binding, Snapshot: snapshot,
		Evidence: evidence, Request: retained, Trust: codehost.Trust{SharedAccount: true}}
}

func seedReceipt(t *testing.T, o Options, binding codehost.ReviewBinding) {
	t.Helper()
	body, err := json.Marshal(codehost.ReviewReceipt{SchemaVersion: codehost.ReceiptSchema, Binding: binding, Reviewer: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	oid, err := prepareReceipt(t.Context(), o.Origin, binding.ProposalOID, append(body, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "update-ref", receiptRef(binding), oid)
	fixtureGit(t, o.Root, "fetch", "--no-tags", "--no-write-fetch-head", o.Origin, oid)
}
