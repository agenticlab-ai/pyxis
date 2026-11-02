package codehost

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func candidateLandingFixture(t *testing.T) CandidateLanding {
	t.Helper()
	r, g := candidateRequestFixture()
	evidence := ReviewArtifact{Kind: CandidateEvidence, Body: []byte("complete opaque candidate evidence\n"), Identity: r.Binding.Evidence}
	evidence.Identity.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(evidence.Body))
	evidence.Identity.Ref = "refs/pyxis/candidate-evidence/" + g.Proposal.ID + "/" + strings.TrimPrefix(evidence.Identity.Digest, "sha256:")
	r.Binding.Evidence, r.Binding.Digest = evidence.Identity, evidence.Identity.Digest
	body, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	request := ReviewArtifact{Kind: CandidateRequest, Body: body, Identity: ArtifactRef{"refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, strings.Repeat("9", 40), fmt.Sprintf("sha256:%x", sha256.Sum256(body))}}
	binding := r.Binding
	binding.Request = request.Identity
	review := ReceiptRecord{OID: strings.Repeat("1", 40), Receipt: ReviewReceipt{SchemaVersion: ReceiptSchema, Binding: binding, Reviewer: "declared operator"}}
	body, err = json.Marshal(review.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	review.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n')))
	return CandidateLanding{CandidateLandingSchema, binding.Candidate, strings.Repeat("5", 40), strings.Repeat("7", 40), evidence, request, review}
}

func TestCandidateLandingRetainsWholeBoundProofAndDeclaration(t *testing.T) {
	valid := candidateLandingFixture(t)
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*CandidateLanding){
		func(l *CandidateLanding) { l.SchemaVersion = "unknown" },
		func(l *CandidateLanding) { l.MergeCommit = "short" },
		func(l *CandidateLanding) { l.FeatureMarker = l.MergeCommit },
		func(l *CandidateLanding) { l.Candidate.BuilderVersion += "-different" },
		func(l *CandidateLanding) { l.Review.Receipt.Binding.Nonce = strings.Repeat("9", 32) },
		func(l *CandidateLanding) { l.Evidence.Body = []byte("altered evidence") },
		func(l *CandidateLanding) { l.Request.Body = []byte("altered request") },
		func(l *CandidateLanding) { l.Evidence.Body = make([]byte, MaxReviewArtifactBytes+1) },
		func(l *CandidateLanding) { l.Review.Digest = "sha256:" + strings.Repeat("0", 64) },
		func(l *CandidateLanding) {
			l.Request.Body = []byte("invalid request\n")
			l.Request.Identity.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(l.Request.Body))
			l.Review.Receipt.Binding.Request = l.Request.Identity
		},
	} {
		l := valid
		mutate(&l)
		if err := l.Validate(); err == nil {
			t.Fatal("altered finalized proof accepted")
		}
	}
}
