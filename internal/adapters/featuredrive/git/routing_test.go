package gitdrive

import (
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestSignedStoreAcceptsCoreCreationTransition(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := newFixture(t, format)
			initial := f.initial()
			initial.Created.Kind = "create"
			initial.Last = initial.Created
			if err := f.store.Create(t.Context(), initial); err != nil {
				t.Fatalf("core creation rejected: %v", err)
			}
			got, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), initial.FeatureID)
			if err != nil || got.Created.Kind != "create" || got.Created != got.Last {
				t.Fatalf("core creation not retained: %+v, %v", got, err)
			}
			f.unchangedSource(t)
		})
	}
}

func (f *fixture) revokeRouting(t *testing.T, prior featuredrive.FeatureDrive, kind string) featuredrive.FeatureDrive {
	t.Helper()
	next := step(prior, kind)
	if kind == "handoff-quiescing" {
		next.Ownership.State = featuredrive.Quiescing
	} else {
		next.Phase = featuredrive.CancelPending
	}
	if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSignedStoreFencesRoutingRevocationAndRetainsCompletion(t *testing.T) {
	for _, kind := range []string{"handoff-quiescing", "cancel-pending"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, "sha1")
			prior := f.assign(t)
			owner := f.newStore(t, f.repo, f.owner)
			pending := step(prior, "effect-intent")
			pending.Effects = []featuredrive.EffectIntent{{ID: "existing", Kind: "request-task-proposal", Subject: "T1", InputDigest: "sha256:input", RequiredEpoch: 1, Status: featuredrive.Pending}}
			if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, pending); err != nil {
				t.Fatal(err)
			}
			prior, err := owner.Load(t.Context(), prior.FeatureID)
			if err != nil {
				t.Fatal(err)
			}
			prior = f.revokeRouting(t, prior, kind)
			before := git(t, f.origin, "rev-parse", f.store.ref)
			for _, signer := range []Signer{f.owner, f.operator} {
				intent := step(prior, "effect-intent")
				intent.Effects = append(append([]featuredrive.EffectIntent(nil), prior.Effects...), featuredrive.EffectIntent{ID: "fresh", Kind: "dispatch", Subject: "T2", RequiredEpoch: 1, Status: featuredrive.Pending})
				if err := f.newStore(t, f.repo, signer).CAS(t.Context(), prior.FeatureID, prior.Version, intent); !errors.Is(err, featuredrive.ErrMalformed) {
					t.Errorf("%s persisted fresh work after %s: %v", signer.ID, kind, err)
				}
			}
			// An observation may complete the already persisted pending effect,
			// but cannot smuggle in new work or restart the revoked lifecycle.
			completion := step(prior, "effect-observed")
			completion.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
			completion.Effects[0].Status = featuredrive.Observed
			completion.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "local-proposal", ID: "existing", Digest: "sha256:result"}
			for _, tc := range []struct {
				name string
				edit func(*featuredrive.FeatureDrive)
			}{
				{"fresh observation", func(d *featuredrive.FeatureDrive) {
					d.Effects = append(d.Effects, featuredrive.EffectIntent{ID: "fresh", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Observed, Observation: completion.Effects[0].Observation})
				}},
				{"removed effect", func(d *featuredrive.FeatureDrive) { d.Effects = nil }},
				{"changed input", func(d *featuredrive.FeatureDrive) { d.Effects[0].InputDigest = "sha256:changed" }},
				{"changed attempts", func(d *featuredrive.FeatureDrive) { d.Effects[0].Attempts++ }},
				{"pending retry", func(d *featuredrive.FeatureDrive) {
					d.Effects[0].Status, d.Effects[0].Observation = featuredrive.Pending, nil
					d.Effects[0].Attempts++
				}},
				{"changed phase", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.Blocked }},
				{"changed frontier", func(d *featuredrive.FeatureDrive) { d.Frontier = []string{"T2"} }},
				{"no completion", func(d *featuredrive.FeatureDrive) {
					d.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
				}},
			} {
				next := completion
				next.Effects = append([]featuredrive.EffectIntent(nil), completion.Effects...)
				tc.edit(&next)
				if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
					t.Errorf("%s accepted during %s: %v", tc.name, kind, err)
				}
			}
			if git(t, f.origin, "rev-parse", f.store.ref) != before {
				t.Fatal("refused post-revocation writes changed the origin")
			}
			if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, completion); err != nil {
				t.Fatalf("existing completion rejected during %s: %v", kind, err)
			}
			got, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID)
			if err != nil || got.Phase != prior.Phase || got.Ownership != prior.Ownership || !reflect.DeepEqual(got.Effects, completion.Effects) {
				t.Fatalf("completion lost revocation or its evidence: %+v, %v", got, err)
			}
			// A valid signature cannot bypass the same fence on cold reads.
			record, parent, body := f.currentRecord(t)
			record.Parent, record.PreviousDigest = parent, contentDigest(body)
			record.Drive = step(got, "effect-intent")
			record.Drive.Effects = append(append([]featuredrive.EffectIntent(nil), got.Effects...), featuredrive.EffectIntent{ID: "cold-fresh", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending})
			f.resign(t, &record, f.owner.ID)
			encoded, err := record.canonical()
			if err != nil {
				t.Fatal(err)
			}
			f.appendFiles(t, map[string][]byte{drivePath: encoded})
			if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Fatalf("cold reader accepted signed work after %s: %v", kind, err)
			}
			f.unchangedSource(t)
		})
	}
}
