package gitdrive

import (
	"context"
	"errors"
	"testing"
)

func TestSignedStorePreservesCredentialFailureAndMidSignCancellation(t *testing.T) {
	credentialUnavailable := errors.New("credential unavailable")
	for _, failure := range []error{context.Canceled, context.DeadlineExceeded, credentialUnavailable} {
		t.Run(failure.Error(), func(t *testing.T) {
			f := newFixture(t, "sha1")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			signer := f.operator
			calls := 0
			signer.Sign = func(context.Context, []byte) ([]byte, error) {
				calls++
				if errors.Is(failure, context.Canceled) {
					cancel()
				}
				return nil, failure
			}
			before := git(t, f.origin, "show-ref")
			err := f.newStore(t, f.repo, signer).Create(ctx, f.initial())
			if calls != 1 || !errors.Is(err, ErrUnauthorized) || !errors.Is(err, failure) {
				t.Fatalf("credential error lost: calls=%d, error=%v, want %v", calls, err, failure)
			}
			if git(t, f.origin, "show-ref") != before {
				t.Fatal("failed signer changed origin refs")
			}
			f.unchangedSource(t)
		})
	}
}
