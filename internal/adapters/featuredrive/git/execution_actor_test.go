package gitdrive

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestSignedStoreRefusesOperatorExecutionOnWriteAndColdRead(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, kind := range []string{"effect-intent", "effect-observed", "frontier-observed"} {
			for _, route := range []string{"write", "cold signed record"} {
				t.Run(format+"/"+kind+"/"+route, func(t *testing.T) {
					f := newFixture(t, format)
					prior := f.assign(t)
					owner := f.newStore(t, f.repo, f.owner)
					pending := step(prior, "effect-intent")
					if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, pending); err != nil {
						t.Fatal(err)
					}
					prior, err := owner.Load(t.Context(), prior.FeatureID)
					if err != nil {
						t.Fatal(err)
					}
					next := step(prior, kind)
					switch kind {
					case "effect-observed":
						next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
						next.Effects[0].Status = featuredrive.Observed
						next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "proposal", ID: "retained"}
					case "frontier-observed":
						next.Frontier = []string{"T1"}
					}
					before := git(t, f.origin, "show-ref")
					if route == "write" {
						if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, ErrUnauthorized) {
							t.Errorf("operator executed owner transition %s: %v", kind, err)
						}
						if git(t, f.origin, "show-ref") != before {
							t.Fatal("refused operator execution changed origin refs")
						}
						if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); err != nil {
							t.Fatalf("current execution owner refused: %v", err)
						}
						got, err := owner.Load(t.Context(), prior.FeatureID)
						if err != nil || got.Last.Actor != f.owner.ID {
							t.Fatalf("execution owner attribution lost: %v", err)
						}
					} else {
						record, parent, body := f.currentRecord(t)
						record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
						f.resign(t, &record, f.operator.ID)
						encoded, err := record.canonical()
						if err != nil {
							t.Fatal(err)
						}
						f.appendFiles(t, map[string][]byte{drivePath: encoded})
						if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, ErrUnauthorized) {
							t.Errorf("operator execution survived signed cold replay: %v", err)
						}
					}
					f.unchangedSource(t)
				})
			}
		}
	}
}

func TestSignedStoreAuthorizesDeliveryReleaseWithCompletedHistory(t *testing.T) {
	f := newFixture(t, "sha1")
	prior := f.initial()
	prior.Version = 3
	prior.Last = featuredrive.Transition{Kind: "effect-intent", PriorVersion: 2, NewVersion: 3, PriorEpoch: 1, NewEpoch: 1}
	prior.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: f.owner.ID, Epoch: 1}
	prior.Phase = featuredrive.DeliveryVerifying
	prior.Effects = []featuredrive.EffectIntent{{ID: "verify-delivery", Kind: "delivery-routing-released", RequiredEpoch: 1, Status: featuredrive.Pending}}
	if err := prior.Validate(); err != nil {
		t.Fatal(err)
	}
	next := step(prior, "effect-observed")
	next.Phase = featuredrive.Delivered
	next.Ownership = featuredrive.Ownership{State: featuredrive.Released, Epoch: 2}
	next.Last.NewEpoch = 2
	next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
	next.Effects[0].Status = featuredrive.Observed
	next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "delivery", ID: "verified-mainline"}
	if err := f.store.authorize(f.store.principals[f.owner.ID], &prior, next); err != nil {
		t.Fatalf("completed delivery history cannot cross release epoch: %v", err)
	}
	if next.Effects[0].RequiredEpoch != 1 {
		t.Fatal("delivery rewrote the effect's execution epoch")
	}
}
