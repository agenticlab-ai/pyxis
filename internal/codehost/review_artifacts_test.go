package codehost

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func taskRequestFixture() (TaskReviewRequest, ProposalGeneration) {
	p := Proposal{SchemaVersion: ProposalSchema, ID: "task-widget-t1", Kind: "implementation", Feature: "widget", Task: "T1", State: "prepared", Sequence: 2, Branch: "refs/heads/task/widget-t1", Head: strings.Repeat("b", 40), TargetRef: "refs/heads/feature/widget", TargetOID: strings.Repeat("c", 40), TaskPreparation: &TaskPreparation{TaskPreparationSchema, TaskMerge, strings.Repeat("0", 32)}}
	g := ProposalGeneration{OID: strings.Repeat("a", 40), Proposal: p}
	digest := "sha256:" + strings.Repeat("d", 64)
	r := TaskReviewRequest{SchemaVersion: TaskRequestSchema, Binding: ReviewBinding{SchemaVersion: ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence, Operation: TaskMerge, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID, Digest: digest, Nonce: p.TaskPreparation.Nonce, Evidence: ArtifactRef{Ref: "refs/pyxis/review-aggregates/" + p.ID + "/" + strings.TrimPrefix(digest, "sha256:"), OID: strings.Repeat("e", 40), Digest: digest}}}
	return r, g
}

func TestTaskRequestCanonicalIntentAndAugmentedReceiptBinding(t *testing.T) {
	r, g := taskRequestFixture()
	if err := r.Matches(g); err != nil {
		t.Fatal(err)
	}
	body, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := ParseTaskRequest(body)
	if err != nil || recovered != r {
		t.Fatalf("canonical request changed: %+v %v", recovered, err)
	}
	// A receipt must also pin the request object. The request itself cannot.
	if err := r.Binding.Validate(); !errors.Is(err, ErrReviewInvalid) {
		t.Fatal("receipt accepted an absent retained request")
	}
	r.Binding.Request = ArtifactRef{Ref: "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, OID: strings.Repeat("f", 40), Digest: "sha256:" + strings.Repeat("1", 64)}
	if err := r.Binding.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Canonical(); !errors.Is(err, ErrReviewInvalid) {
		t.Fatal("request embedded its own later object identity")
	}
}

func TestTaskRequestRejectsStaleBindingsAndUnsupportedIntents(t *testing.T) {
	for _, mutate := range []func(*TaskReviewRequest){
		func(r *TaskReviewRequest) { r.SchemaVersion = "unknown" },
		func(r *TaskReviewRequest) { r.Binding.SchemaVersion = "unknown" },
		func(r *TaskReviewRequest) { r.Binding.Kind = "draft" },
		func(r *TaskReviewRequest) { r.Binding.Operation = "feature-final-merge" },
		func(r *TaskReviewRequest) { r.Binding.Nonce = "new unpresented nonce" },
		func(r *TaskReviewRequest) { r.Binding.Sequence++ },
		func(r *TaskReviewRequest) { r.Binding.ProposalOID = strings.Repeat("9", 40) },
		func(r *TaskReviewRequest) { r.Binding.TargetOID = strings.Repeat("9", 40) },
		func(r *TaskReviewRequest) { r.Binding.Head = strings.Repeat("9", 64) },
		func(r *TaskReviewRequest) { r.Binding.Branch = r.Binding.TargetRef },
		func(r *TaskReviewRequest) { r.Binding.Branch += "\n" },
		func(r *TaskReviewRequest) {
			r.Binding.Evidence.Ref = "refs/pyxis/review-aggregates/foreign/" + strings.Repeat("d", 64)
		},
		func(r *TaskReviewRequest) { r.Binding.Evidence.OID = "abbreviated" },
		func(r *TaskReviewRequest) { r.Binding.Evidence.Digest = "sha256:invalid" },
	} {
		r, g := taskRequestFixture()
		mutate(&r)
		if err := r.Matches(g); !errors.Is(err, ErrReviewInvalid) {
			t.Fatalf("stale or unsupported request accepted: %+v %v", r, err)
		}
	}
	r, g := taskRequestFixture()
	g.Proposal.State = "open"
	if err := r.Matches(g); !errors.Is(err, ErrReviewInvalid) {
		t.Fatal("request accepted an unprepared generation")
	}
	g.Proposal.State, g.Proposal.TaskPreparation = "prepared", nil
	if err := r.Matches(g); !errors.Is(err, ErrReviewInvalid) {
		t.Fatal("request accepted an absent preparation nonce")
	}
}

func TestTaskRequestClosedCanonicalDecoder(t *testing.T) {
	r, _ := taskRequestFixture()
	body, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{
		[]byte("{"),
		append(append([]byte{}, body...), []byte("{}")...),
		append([]byte(" "), body...),
		bytes.Replace(body, []byte(`"schema_version":`), []byte(`"unknown":true,"schema_version":`), 1),
		bytes.Replace(body, []byte(`"schema_version":`), []byte(`"schema_version":"duplicate","schema_version":`), 1),
		bytes.Repeat([]byte(" "), MaxReviewArtifactBytes+1),
	} {
		if _, err := ParseTaskRequest(invalid); !errors.Is(err, ErrReviewInvalid) {
			t.Fatalf("noncanonical or open request accepted: %v", err)
		}
	}
}
