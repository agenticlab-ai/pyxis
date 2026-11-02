package codehost

import (
	"errors"
	"strings"
	"testing"
)

func TestLocalDraftLockValidateRequiresOneConsistentReviewedLanding(t *testing.T) {
	oid := strings.Repeat("a", 40)
	binding := ReviewBinding{SchemaVersion: ReviewBindingSchema, ProposalID: "draft-widget", Kind: "draft", ProposalOID: oid, Sequence: 1, Operation: DraftLock,
		Branch: "refs/heads/draft/widget", Head: oid, TargetRef: "refs/heads/main", TargetOID: oid, Digest: "sha256:" + strings.Repeat("b", 64), Nonce: strings.Repeat("c", 32)}
	valid := LocalDraftLock{SchemaVersion: DraftLockSchema, Feature: "widget", Version: 1, MergeCommit: oid, ProductApprover: "operator", EngineeringApprover: "operator", Lineless: true, ProposalsSinceOpened: 1,
		Review: ReceiptRecord{OID: oid, Digest: "sha256:" + strings.Repeat("d", 64), Receipt: ReviewReceipt{SchemaVersion: ReceiptSchema, Binding: binding, Reviewer: "operator"}, Trust: Trust{SharedAccount: true}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid lock: %v", err)
	}
	for name, mutate := range map[string]func(*LocalDraftLock){
		"version":         func(l *LocalDraftLock) { l.Version = 0 },
		"approver":        func(l *LocalDraftLock) { l.EngineeringApprover = "someone else" },
		"capture line":    func(l *LocalDraftLock) { l.Lineless = false },
		"trust":           func(l *LocalDraftLock) { l.Review.Trust = Trust{} },
		"binding kind":    func(l *LocalDraftLock) { l.Review.Receipt.Binding.Kind = "implementation" },
		"receipt schema":  func(l *LocalDraftLock) { l.Review.Receipt.SchemaVersion = "" },
		"review digest":   func(l *LocalDraftLock) { l.Review.Digest = "sha256:short" },
		"unopened drafts": func(l *LocalDraftLock) { l.ProposalsSinceOpened = 0 },
	} {
		lock := valid
		mutate(&lock)
		if err := lock.Validate(); !errors.Is(err, ErrReviewInvalid) {
			t.Errorf("%s: Validate() = %v, want ErrReviewInvalid", name, err)
		}
	}
}
