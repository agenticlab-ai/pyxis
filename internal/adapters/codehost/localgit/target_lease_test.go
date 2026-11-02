package localgit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestProposalsHoldUnchangedRefsAcrossAtomicPublication(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"open", "round", "prepared", "abandoned", "implementation"} {
		t.Run(operation, func(t *testing.T) {
			o, store, change := proposalFixture(t)
			if operation == "implementation" {
				store.options.Kind, store.options.IntegrationRef = "implementation", "refs/heads/feature-widget"
				fixtureGit(t, o.Origin, "update-ref", store.options.IntegrationRef, change.ExpectedTargetOID)
				change.ID, change.Task = "task-widget-5", "carina:5"
			} else if operation != "open" {
				first, err := store.Publish(t.Context(), change)
				if err != nil {
					t.Fatal(err)
				}
				change = revision(first, first.Proposal.Head)
				if operation == "round" {
					fixtureGit(t, o.Root, "commit", "--allow-empty", "-m", "next round")
					change.Head = fixtureGit(t, o.Root, "rev-parse", "HEAD")
				} else {
					change.State = operation
					if operation == "abandoned" {
						change.AbandonmentReason = "No longer needed."
					} else {
						request, err := codehost.NewDraftReviewRequest("sha256:" + strings.Repeat("a", 64))
						if err != nil {
							t.Fatal(err)
						}
						change.ReviewRequest = &request
					}
				}
			}
			target, clone := store.targetRef(), proposalClone(t, o)
			next := fixtureGit(t, clone.Root, "commit-tree", change.ExpectedTargetOID+"^{tree}", "-p", change.ExpectedTargetOID, "-m", "competing target advance")
			entered := false
			store.push = func(ctx context.Context, root string, args ...string) error {
				entered = true
				for _, ref := range []string{target, change.Branch} {
					if ref == change.Branch && change.Head != change.ExpectedBranchOID {
						continue
					}
					_, err := proposalGit(ctx, clone.Root, nil, nil, "push", "--force", o.Origin, next+":"+ref)
					if !refLockRejected(err) {
						t.Fatalf("another clone moved a ref during publication: %s: %v", ref, err)
					}
				}
				_, err := proposalGit(ctx, root, nil, nil, args...)
				return err
			}
			published, err := store.Publish(t.Context(), change)
			if err != nil || !entered || fixtureGit(t, o.Origin, "rev-parse", target) != change.ExpectedTargetOID {
				t.Fatalf("leased publication: %+v %v", published, err)
			}
			if got, err := store.Read(t.Context(), change.ID); err != nil || !reflect.DeepEqual(got, published) {
				t.Fatalf("publication lost its paired generation: %+v %v", got, err)
			}
			// The verification transaction releases both locks after publication.
			fixtureGit(t, clone.Root, "push", o.Origin, next+":"+target)
			if change.Head == change.ExpectedBranchOID {
				fixtureGit(t, clone.Root, "push", "--force", o.Origin, next+":"+change.Branch)
			}
		})
	}
}

func TestUnchangedRefLeaseRefusesChangedAbsentOrBusyTarget(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"changed", "absent", "busy"} {
		t.Run(failure, func(t *testing.T) {
			o, _, change := proposalFixture(t)
			target, expected := "refs/heads/trunk", change.ExpectedTargetOID
			switch failure {
			case "changed":
				expected = change.Head
			case "absent":
				fixtureGit(t, o.Origin, "update-ref", "-d", target)
			case "busy":
				if err := os.WriteFile(filepath.Join(o.Origin, target+".lock"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, entered := fixtureGit(t, o.Origin, "for-each-ref"), false
			err := withUnchangedRefs(t.Context(), o.Origin, []unchangedRef{{target, expected}}, func() error { entered = true; return nil })
			if !errors.Is(err, codehost.ErrProposalConflict) || entered || fixtureGit(t, o.Origin, "for-each-ref") != before {
				t.Fatalf("invalid target lease reached publication: %v, entered=%v", err, entered)
			}
		})
	}
}

func TestUnchangedRefLeaseReleasesAfterCancellationAndFailure(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"cancelled", "push failed"} {
		t.Run(failure, func(t *testing.T) {
			o, _, change := proposalFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := errors.New("push interrupted")
			before := fixtureGit(t, o.Origin, "show-ref")
			err := withUnchangedRefs(ctx, o.Origin, []unchangedRef{{"refs/heads/trunk", change.ExpectedTargetOID}}, func() error {
				if failure == "cancelled" {
					cancel()
				}
				return cause
			})
			if !errors.Is(err, cause) || failure == "cancelled" && !errors.Is(err, context.Canceled) || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("interrupted lease lost its cause or changed refs: %v", err)
			}
			if _, err := os.Stat(filepath.Join(o.Origin, "refs/heads/trunk.lock")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("lease retained a lock after exit: %v", err)
			}
			fixtureGit(t, o.Root, "push", o.Origin, change.Head+":refs/heads/trunk")
		})
	}
}

func TestUnchangedRefLeaseReportsUnavailableGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := withUnchangedRefs(t.Context(), t.TempDir(), nil, func() error {
		t.Fatal("publication ran without Git")
		return nil
	})
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing Git cause was lost: %v", err)
	}
}

// refLockRejected reports whether a competing push was refused because a
// verification transaction holds the ref lock. Git 2.52 reports the lock
// collision as "reference already exists"; other versions say "cannot lock ref".
func refLockRejected(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "cannot lock ref") || strings.Contains(err.Error(), "reference already exists"))
}
