package gitdrive

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestExecutableObservationCannotRewriteLifecycle(t *testing.T) {
	prior := featuredrive.FeatureDrive{Ownership: featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}, Phase: featuredrive.Integrating,
		Effects: []featuredrive.EffectIntent{{ID: "pending", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending}}}
	for _, tc := range []struct {
		name string
		edit func(*featuredrive.FeatureDrive)
	}{
		{"delivered with active owner", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.Delivered }},
		{"changed wait", func(d *featuredrive.FeatureDrive) { d.Wait = &featuredrive.Wait{Kind: "human-review"} }},
		{"changed frontier", func(d *featuredrive.FeatureDrive) { d.Frontier = []string{"T2"} }},
		{"changed blocker", func(d *featuredrive.FeatureDrive) {
			d.Blocker = &featuredrive.EvidenceRef{Kind: "blocker", ID: "invented"}
		}},
		{"changed evidence", func(d *featuredrive.FeatureDrive) {
			d.Evidence = []featuredrive.EvidenceRef{{Kind: "delivered", ID: "unverified"}}
		}},
		{"changed failure", func(d *featuredrive.FeatureDrive) { d.Failure = &featuredrive.FailureDisposition{Trigger: "invented"} }},
		{"changed secondary failures", func(d *featuredrive.FeatureDrive) {
			d.SecondaryFailures = []featuredrive.FailureDisposition{{Trigger: "invented"}}
		}},
		{"changed input", func(d *featuredrive.FeatureDrive) { d.Effects[0].InputDigest = "invented" }},
		{"changed attempt count", func(d *featuredrive.FeatureDrive) { d.Effects[0].Attempts++ }},
		{"removed effects", func(d *featuredrive.FeatureDrive) { d.Effects = nil }},
		{"new observed effect", func(d *featuredrive.FeatureDrive) {
			d.Effects = append(d.Effects, featuredrive.EffectIntent{ID: "fresh", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Observed, Observation: d.Effects[0].Observation})
		}},
		{"no completion", func(d *featuredrive.FeatureDrive) {
			d.Effects[0].Status, d.Effects[0].Observation = featuredrive.Pending, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := step(prior, "effect-observed")
			next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
			next.Effects[0].Status = featuredrive.Observed
			next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-proposal", ID: "existing", Digest: "sha256:result"}
			tc.edit(&next)
			if err := validateOwnershipStep(prior, next); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Fatalf("executable observation accepted unrelated state: %v", err)
			}
		})
	}
}

func TestObservationAndFrontierPermittedDeltas(t *testing.T) {
	prior := featuredrive.FeatureDrive{Ownership: featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}, Phase: featuredrive.Integrating,
		Effects: []featuredrive.EffectIntent{{ID: "pending", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending}}}
	completion := step(prior, "effect-observed")
	completion.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
	completion.Effects[0].Status = featuredrive.Observed
	completion.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-proposal", ID: "existing", Digest: "sha256:result"}
	if err := validateOwnershipStep(prior, completion); err != nil {
		t.Fatalf("existing active effect completion refused: %v", err)
	}
	verifying := prior
	verifying.Phase = featuredrive.DeliveryVerifying
	verifying.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
	verifying.Effects[0].Kind = "delivery-routing-released"
	delivered := completion
	delivered.Effects = append([]featuredrive.EffectIntent(nil), completion.Effects...)
	delivered.Effects[0].Kind = "delivery-routing-released"
	delivered.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "delivery", ID: "verified-main", Digest: "sha256:result"}
	delivered.Phase = featuredrive.Delivered
	delivered.Ownership = featuredrive.Ownership{State: featuredrive.Released, Epoch: 2}
	if err := validateOwnershipStep(verifying, delivered); err != nil {
		t.Fatalf("delivery release lost its pending effect completion: %v", err)
	}
	delivered.Frontier = []string{"invented"}
	if err := validateOwnershipStep(verifying, delivered); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("delivery release rewrote unrelated state: %v", err)
	}
	frontier := step(prior, "frontier-observed")
	frontier.Frontier = []string{"T1", "T2"}
	if err := validateOwnershipStep(prior, frontier); err != nil {
		t.Fatalf("core observed frontier refused: %v", err)
	}
	frontier.Phase = featuredrive.Delivered
	if err := validateOwnershipStep(prior, frontier); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("frontier observation rewrote lifecycle: %v", err)
	}
	frontier.Phase = prior.Phase
	prior.Ownership.State = featuredrive.Quiescing
	frontier.Ownership = prior.Ownership
	if err := validateOwnershipStep(prior, frontier); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("quiescing owner changed executable frontier: %v", err)
	}
}

func TestDeliveryReleaseRejectsUnrelatedCompletedEffects(t *testing.T) {
	for _, tc := range []struct{ effect, evidence string }{
		{"dispatch", "task-proposal"},
		{"notification", "delivery"},
		{"delivery-routing-released", "task-proposal"},
	} {
		t.Run(tc.effect+"/"+tc.evidence, func(t *testing.T) {
			prior := featuredrive.FeatureDrive{Ownership: featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1},
				Phase:   featuredrive.DeliveryVerifying,
				Effects: []featuredrive.EffectIntent{{ID: "pending", Kind: tc.effect, RequiredEpoch: 1, Status: featuredrive.Pending}}}
			next := step(prior, "effect-observed")
			next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
			next.Effects[0].Status = featuredrive.Observed
			next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: tc.evidence, ID: "unrelated-goal"}
			next.Phase = featuredrive.Delivered
			next.Ownership = featuredrive.Ownership{State: featuredrive.Released, Epoch: 2}
			if err := validateOwnershipStep(prior, next); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Fatalf("unrelated effect released the feature owner: %v", err)
			}
		})
	}
}

func (f *fixture) deliveryVerifying(t *testing.T) featuredrive.FeatureDrive {
	t.Helper()
	prior, candidate := f.phaseCandidate(t)
	owner := f.newStore(t, f.repo, f.owner)
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, candidate); err != nil {
		t.Fatal(err)
	}
	for _, goal := range []struct{ effect, evidence string }{
		{"candidate-published", "candidate"},
		{"final-proposal-activated", "final-proposal"},
		{"final-landing-verified", "final-landing"},
	} {
		prior, err := owner.Load(t.Context(), f.binding.FeatureID)
		if err != nil {
			t.Fatal(err)
		}
		intent := step(prior, "effect-intent")
		i := len(intent.Effects) - 1
		intent.Effects[i].Kind = goal.effect
		if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
			t.Fatal(err)
		}
		prior, err = owner.Load(t.Context(), f.binding.FeatureID)
		if err != nil {
			t.Fatal(err)
		}
		observed := step(prior, "effect-observed")
		observed.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
		observed.Effects[i].Status = featuredrive.Observed
		observed.Effects[i].Observation = &featuredrive.EvidenceRef{Kind: goal.evidence, ID: "verified-" + goal.effect}
		if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, observed); err != nil {
			t.Fatal(err)
		}
		phase := featuredrive.PhaseLifecycle{Store: owner, Goals: phaseVerifier{}}
		if _, err := phase.Advance(t.Context(), prior.FeatureID, featuredrive.OwnerEpoch{Owner: featuredrive.Owner{Domain: "local", ID: f.owner.ID}, Epoch: 1}, intent.Effects[i].ID); err != nil {
			t.Fatal(err)
		}
	}
	prior, err := owner.Load(t.Context(), f.binding.FeatureID)
	if err != nil || prior.Phase != featuredrive.DeliveryVerifying {
		t.Fatalf("did not reach delivery verification: %v", err)
	}
	return prior
}

func TestSignedDeliveryReleaseRejectsUnrelatedEffectsOnWriteAndColdRead(t *testing.T) {
	for _, tc := range []struct{ effect, evidence string }{
		{"dispatch", "task-proposal"},
		{"notification", "delivery"},
		{"delivery-routing-released", "task-proposal"},
	} {
		for _, route := range []string{"write", "cold-read"} {
			t.Run(tc.effect+"/"+tc.evidence+"/"+route, func(t *testing.T) {
				f := newFixture(t, "sha1")
				prior := f.deliveryVerifying(t)
				owner := f.newStore(t, f.repo, f.owner)
				intent := step(prior, "effect-intent")
				i := len(intent.Effects) - 1
				intent.Effects[i].Kind = tc.effect
				if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
					t.Fatal(err)
				}
				prior, err := owner.Load(t.Context(), prior.FeatureID)
				if err != nil {
					t.Fatal(err)
				}
				next := step(prior, "effect-observed")
				next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
				next.Effects[i].Status = featuredrive.Observed
				next.Effects[i].Observation = &featuredrive.EvidenceRef{Kind: tc.evidence, ID: "unrelated-goal"}
				next.Phase, next.Ownership = featuredrive.Delivered, featuredrive.Ownership{State: featuredrive.Released, Epoch: 2}
				next.Last.NewEpoch = 2
				before := git(t, f.origin, "show-ref")
				if route == "write" {
					if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); err == nil {
						t.Error("unrelated effect released the owner")
					}
					if git(t, f.origin, "show-ref") != before {
						t.Fatal("refused release changed origin refs")
					}
				} else {
					record, parent, body := f.currentRecord(t)
					record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
					f.resign(t, &record, f.owner.ID)
					encoded, err := record.canonical()
					if err != nil {
						t.Fatal(err)
					}
					f.appendFiles(t, map[string][]byte{drivePath: encoded})
					inserted := git(t, f.origin, "show-ref")
					if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); err == nil {
						t.Error("signed unrelated effect certified delivery on cold read")
					}
					if git(t, f.origin, "show-ref") != inserted {
						t.Fatal("cold refusal changed origin refs")
					}
				}
				f.unchangedSource(t)
			})
		}
	}
}

func TestSignedExecutableObservationRejectsFalseDeliveryOnWriteAndColdRead(t *testing.T) {
	f := newFixture(t, "sha1")
	prior := f.assign(t)
	owner := f.newStore(t, f.repo, f.owner)
	intent := step(prior, "effect-intent")
	intent.Effects = []featuredrive.EffectIntent{{ID: "existing", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending}}
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
		t.Fatal(err)
	}
	prior, err := owner.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	next := step(prior, "effect-observed")
	next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
	next.Effects[0].Status = featuredrive.Observed
	next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-proposal", ID: "existing", Digest: "sha256:result"}
	next.Phase = featuredrive.Delivered
	before := git(t, f.origin, "rev-parse", f.store.ref)
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) || git(t, f.origin, "rev-parse", f.store.ref) != before {
		t.Errorf("false delivery was published: %v", err)
		return
	}
	record, parent, body := f.currentRecord(t)
	record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
	f.resign(t, &record, f.owner.ID)
	encoded, err := record.canonical()
	if err != nil {
		t.Fatal(err)
	}
	f.appendFiles(t, map[string][]byte{drivePath: encoded})
	if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("signed false delivery survived a cold read: %v", err)
	}
	f.unchangedSource(t)
}

func TestSignedStoreRefusesEmptyEffectObservationsOnWriteAndColdRead(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, tc := range []struct {
			name string
			ref  featuredrive.EvidenceRef
		}{
			{"zero", featuredrive.EvidenceRef{}},
			{"missing kind", featuredrive.EvidenceRef{ID: "actual-result"}},
			{"missing id", featuredrive.EvidenceRef{Kind: "task-proposal"}},
			{"blank identity", featuredrive.EvidenceRef{Kind: " ", ID: "\t"}},
		} {
			for _, route := range []string{"write", "cold signed record"} {
				t.Run(format+"/"+tc.name+"/"+route, func(t *testing.T) {
					f := newFixture(t, format)
					assigned := f.assign(t)
					owner := f.newStore(t, f.repo, f.owner)
					pending := step(assigned, "effect-intent")
					if err := owner.CAS(t.Context(), assigned.FeatureID, assigned.Version, pending); err != nil {
						t.Fatal(err)
					}
					prior, err := owner.Load(t.Context(), assigned.FeatureID)
					if err != nil {
						t.Fatal(err)
					}
					next := step(prior, "effect-observed")
					next.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
					next.Effects[0].Status = featuredrive.Observed
					next.Effects[0].Observation = &tc.ref
					before := git(t, f.origin, "show-ref")
					if route == "write" {
						if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
							t.Errorf("empty observation was published: %v", err)
						}
						if git(t, f.origin, "show-ref") != before {
							t.Fatal("refused empty observation changed origin refs")
						}
						// Digest is optional for an observation whose typed ID is
						// already a retained goal object. Do not require one here.
						next.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-proposal", ID: "actual-result"}
						if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, next); err != nil {
							t.Fatalf("valid typed observation refused: %v", err)
						}
					} else {
						r, parent, body := f.currentRecord(t)
						r.Parent, r.PreviousDigest, r.Drive = parent, contentDigest(body), next
						f.resign(t, &r, f.owner.ID)
						encoded, err := r.canonical()
						if err != nil {
							t.Fatal(err)
						}
						f.appendFiles(t, map[string][]byte{drivePath: encoded})
						if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
							t.Errorf("empty observation survived signed cold replay: %v", err)
						}
					}
					f.unchangedSource(t)
				})
			}
		}
	}
}

type observationExecutor func(context.Context, featuredrive.EffectIntent) (featuredrive.EvidenceRef, error)

func (e observationExecutor) Execute(ctx context.Context, intent featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
	return e(ctx, intent)
}

func TestSignedReconcileRetainsPendingEffectUntilValidObservation(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			f := newFixture(t, format)
			assigned := f.assign(t)
			owner := f.newStore(t, f.repo, f.owner)
			pending := step(assigned, "effect-intent")
			if err := owner.CAS(t.Context(), assigned.FeatureID, assigned.Version, pending); err != nil {
				t.Fatal(err)
			}
			prior, err := owner.Load(t.Context(), assigned.FeatureID)
			if err != nil {
				t.Fatal(err)
			}
			before, calls := git(t, f.origin, "show-ref"), 0
			r := featuredrive.Reconciler{Store: owner,
				Plan: func(context.Context, featuredrive.FeatureDrive, string) ([]featuredrive.EffectIntent, error) {
					return nil, nil
				},
				Executor: observationExecutor(func(_ context.Context, intent featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
					calls++
					if calls == 1 {
						return featuredrive.EvidenceRef{}, nil
					}
					return featuredrive.EvidenceRef{Kind: "task-proposal", ID: intent.ID}, nil
				}),
			}
			if err := r.Reconcile(t.Context(), prior.FeatureID, "observe"); !errors.Is(err, featuredrive.ErrMalformed) || git(t, f.origin, "show-ref") != before {
				t.Fatalf("zero executor result completed a durable effect: %v", err)
			}
			got, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID)
			if err != nil || !reflect.DeepEqual(got, prior) {
				t.Fatalf("empty result changed pending effect on restart: %v", err)
			}
			if err := r.Reconcile(t.Context(), prior.FeatureID, "retry"); err != nil {
				t.Fatal(err)
			}
			if err := r.Reconcile(t.Context(), prior.FeatureID, "already observed"); err != nil || calls != 2 {
				t.Fatalf("valid retry failed to converge: calls=%d %v", calls, err)
			}
			got, err = owner.Load(t.Context(), prior.FeatureID)
			if err != nil || got.Effects[0].Status != featuredrive.Observed || got.Effects[0].Observation == nil || got.Effects[0].Observation.ID != prior.Effects[0].ID {
				t.Fatalf("valid retry lost its actual observation: %v", err)
			}
			f.unchangedSource(t)
		})
	}
}
