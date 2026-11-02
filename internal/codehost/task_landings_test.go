package codehost

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func taskLandingFixture(t *testing.T) TaskLanding {
	t.Helper()
	r, g := taskRequestFixture()
	// This port stores opaque complete aggregate bytes. The reviewgate consumer
	// separately validates their snapshot schema and current native/check policy.
	aggregate := ReviewArtifact{Kind: TaskAggregate, Body: []byte("complete opaque aggregate\n")}
	aggregate.Identity = r.Binding.Evidence
	aggregate.Identity.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(aggregate.Body))
	aggregate.Identity.Ref = "refs/pyxis/review-aggregates/" + g.Proposal.ID + "/" + strings.TrimPrefix(aggregate.Identity.Digest, "sha256:")
	r.Binding.Evidence, r.Binding.Digest = aggregate.Identity, aggregate.Identity.Digest
	body, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	request := ReviewArtifact{Kind: TaskRequest, Body: body, Identity: ArtifactRef{Ref: "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, OID: strings.Repeat("f", 40), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(body))}}
	binding := r.Binding
	binding.Request = request.Identity
	review := ReceiptRecord{OID: strings.Repeat("1", 40), Receipt: ReviewReceipt{SchemaVersion: ReceiptSchema, Binding: binding, Reviewer: "declared operator"}}
	body, err = json.Marshal(review.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	review.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n')))
	return TaskLanding{SchemaVersion: TaskLandingSchema, Feature: g.Proposal.Feature, Task: g.Proposal.Task, MergeCommit: strings.Repeat("2", 40), Aggregate: aggregate, Request: request, Review: review}
}

func TestTaskLandingEmbedsIdenticalBoundProofAndReceipt(t *testing.T) {
	valid := taskLandingFixture(t)
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*TaskLanding){
		func(l *TaskLanding) { l.SchemaVersion = "unknown" },
		func(l *TaskLanding) { l.MergeCommit = "abbreviated" },
		func(l *TaskLanding) { l.Review.Receipt.Binding.Nonce = strings.Repeat("9", 32) },
		func(l *TaskLanding) { l.Aggregate.Body = []byte("altered aggregate") },
		func(l *TaskLanding) { l.Request.Body = []byte("altered request") },
		func(l *TaskLanding) {
			l.Request.Body = []byte("invalid request\n")
			l.Request.Identity.Digest = fmt.Sprintf("sha256:%x", sha256.Sum256(l.Request.Body))
			l.Review.Receipt.Binding.Request = l.Request.Identity
		},
		func(l *TaskLanding) { l.Aggregate.Body = make([]byte, MaxReviewArtifactBytes+1) },
		func(l *TaskLanding) { l.Review.Digest = "sha256:" + strings.Repeat("0", 64) },
	} {
		l := valid
		mutate(&l)
		if err := l.Validate(); err == nil {
			t.Fatal("altered finalized proof was accepted")
		}
	}
}
