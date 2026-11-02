package gitdrive

import (
	"context"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type creationExecutor struct{ calls int }

func (e *creationExecutor) Execute(ctx context.Context, intent featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
	e.calls++
	return featuredrive.EvidenceRef{Kind: "unowned-execution", ID: intent.ID}, ctx.Err()
}

func TestSignedStoreRejectsPostCreationStateOnWriteAndColdRead(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, tc := range []struct {
			name string
			edit func(*featuredrive.FeatureDrive)
		}{
			{"pending effect", func(d *featuredrive.FeatureDrive) {
				d.Effects = []featuredrive.EffectIntent{{ID: "unowned", Kind: "dispatch", RequiredEpoch: 0, Status: featuredrive.Pending}}
			}},
			{"frontier", func(d *featuredrive.FeatureDrive) { d.Frontier = []string{"T1"} }},
			{"wait", func(d *featuredrive.FeatureDrive) { d.Wait = &featuredrive.Wait{Kind: "human-review"} }},
			{"blocker", func(d *featuredrive.FeatureDrive) {
				d.Blocker = &featuredrive.EvidenceRef{Kind: "blocked", ID: "invented"}
			}},
			{"evidence", func(d *featuredrive.FeatureDrive) {
				d.Evidence = []featuredrive.EvidenceRef{{Kind: "landed", ID: "invented"}}
			}},
			{"failure", func(d *featuredrive.FeatureDrive) { d.Failure = &featuredrive.FailureDisposition{Trigger: "invented"} }},
			{"secondary failure", func(d *featuredrive.FeatureDrive) {
				d.SecondaryFailures = []featuredrive.FailureDisposition{{Trigger: "invented"}}
			}},
		} {
			t.Run(format+"/"+tc.name, func(t *testing.T) {
				for _, route := range []string{"write", "cold signed record"} {
					t.Run(route, func(t *testing.T) {
						f := newFixture(t, format)
						initial := f.initial()
						tc.edit(&initial)
						if err := initial.Validate(); err != nil {
							t.Fatalf("fixture does not reach creation authorization: %v", err)
						}
						before := git(t, f.origin, "show-ref")
						store := f.store
						if route == "write" {
							if err := store.Create(t.Context(), initial); !errors.Is(err, featuredrive.ErrMalformed) {
								t.Errorf("creation accepted post-creation state: %v", err)
							}
							if git(t, f.origin, "show-ref") != before {
								t.Error("refused creation changed origin refs")
							}
						} else {
							initial.Last.Actor = f.operator.ID
							initial.Last = featuredrive.SignTransition(initial.Last, f.keys[f.operator.ID])
							initial.Created = initial.Last
							record := Record{SchemaVersion: RecordSchema, Ref: f.store.ref, Authorization: f.binding.Genesis.Authorization, Parent: f.binding.Genesis.Commit, Drive: initial}
							f.resign(t, &record, f.operator.ID)
							body, err := record.canonical()
							if err != nil {
								t.Fatal(err)
							}
							// Use the raw atomic transport so this first record has
							// valid signatures and matching state/checkpoint refs.
							f.appendFiles(t, map[string][]byte{drivePath: body})
							store = f.newStore(t, f.repo, Signer{})
							if _, err := store.Load(t.Context(), initial.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
								t.Errorf("signed invalid creation survived cold read: %v", err)
							}
							before = git(t, f.origin, "show-ref")
						}
						if tc.name == "pending effect" {
							executor := &creationExecutor{}
							reconciler := featuredrive.Reconciler{Store: store, Executor: executor}
							if err := reconciler.Reconcile(t.Context(), initial.FeatureID, "unowned creation"); err == nil || executor.calls != 0 {
								t.Errorf("unowned creation executed pending work: calls=%d error=%v", executor.calls, err)
							}
						}
						if git(t, f.origin, "show-ref") != before {
							t.Error("invalid creation observation changed origin refs")
						}
						f.unchangedSource(t)
					})
				}
			})
		}
	}
}
