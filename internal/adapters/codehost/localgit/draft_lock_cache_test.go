package localgit

import (
	"context"
	"errors"
	"testing"
)

func TestDraftLockCachedTagConflictsPreserveCompletedLandingRecovery(t *testing.T) {
	t.Parallel()
	for _, timing := range []string{"before publication", "after publication", "lost reply"} {
		t.Run(timing, func(t *testing.T) {
			t.Parallel()
			o, store, request := landingFixture(t, true)
			ref := "refs/tags/lock/widget/v1"
			if timing == "before publication" {
				fixtureGit(t, o.Root, "update-ref", ref, request.Binding.TargetOID)
			} else {
				store.push = func(ctx context.Context, root string, args ...string) error {
					if _, err := proposalGit(ctx, root, nil, nil, args...); err != nil {
						return err
					}
					fixtureGit(t, root, "update-ref", ref, request.Binding.TargetOID)
					if timing == "lost reply" {
						return errors.New("lost publication acknowledgement")
					}
					return nil
				}
			}
			result, err := store.LandDraft(t.Context(), request)
			if err != nil || (timing == "lost reply" && result.Created) {
				t.Fatalf("cached tag hid the completed landing: %+v %v", result, err)
			}
			if fixtureGit(t, o.Root, "rev-parse", ref) != request.Binding.TargetOID || fixtureGit(t, o.Origin, "rev-parse", request.Binding.TargetRef) != result.Lock.MergeCommit {
				t.Fatal("verification rewrote the cache or failed to land")
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			retry, err := store.LandDraft(t.Context(), request)
			if err != nil || retry.Created || retry.Lock != result.Lock || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("completed landing did not remain recoverable: %+v %v", retry, err)
			}
		})
	}
}
