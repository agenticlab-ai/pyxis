package codehost

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPreparedRequestNonceAndClosedProtocol(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	first, err := newDraftReviewRequest(digest, strings.NewReader(strings.Repeat("a", 16)))
	if err != nil || first.Validate() != nil {
		t.Fatalf("new draft-lock request: %+v, %v", first, err)
	}
	second, err := newDraftReviewRequest(digest, strings.NewReader(strings.Repeat("b", 16)))
	if err != nil || first.Nonce == second.Nonce {
		t.Fatalf("new preparation reused a nonce: %+v, %v", second, err)
	}
	for _, bad := range []DraftReviewRequest{
		{},
		{SchemaVersion: "other", Operation: DraftLock, Nonce: first.Nonce, Digest: digest},
		{SchemaVersion: ReviewRequestSchema, Operation: "task-merge", Nonce: first.Nonce, Digest: digest},
		{SchemaVersion: ReviewRequestSchema, Operation: DraftLock, Nonce: strings.Repeat("A", 32), Digest: digest},
		{SchemaVersion: ReviewRequestSchema, Operation: DraftLock, Nonce: strings.Repeat("g", 32), Digest: digest},
		{SchemaVersion: ReviewRequestSchema, Operation: DraftLock, Nonce: first.Nonce, Digest: "sha256:" + strings.Repeat("f", 63)},
		{SchemaVersion: ReviewRequestSchema, Operation: DraftLock, Nonce: first.Nonce, Digest: strings.Repeat("a", 64)},
	} {
		if !errors.Is(bad.Validate(), ErrReviewInvalid) {
			t.Fatalf("malformed request admitted: %+v", bad)
		}
	}
	if _, err := NewDraftReviewRequest("bad digest"); !errors.Is(err, ErrReviewInvalid) {
		t.Fatalf("constructor admitted malformed digest: %v", err)
	}
	if _, err := newDraftReviewRequest(digest, strings.NewReader("")); !errors.Is(err, io.EOF) {
		t.Fatalf("nonce read failure was lost: %v", err)
	}
}
