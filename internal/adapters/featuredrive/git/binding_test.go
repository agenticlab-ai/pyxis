package gitdrive

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestSignedStoreRefusesIncompleteOrConflictingAuthorityBindings(t *testing.T) {
	f := newFixture(t, "sha1")
	for _, alter := range []func(*Binding, *Signer){
		func(b *Binding, _ *Signer) { b.Repository = "" },
		func(b *Binding, _ *Signer) { b.FeatureID = "" },
		func(b *Binding, _ *Signer) { b.FeatureID = "../other" },
		func(b *Binding, _ *Signer) { b.FeatureID = "bad..ref" },
		func(b *Binding, _ *Signer) { b.Genesis.Authorization = "" },
		func(b *Binding, _ *Signer) { b.Genesis.Commit = "short" },
		func(b *Binding, _ *Signer) { b.Genesis.Tree = strings.Repeat("a", 64) },
		func(b *Binding, _ *Signer) { b.Principals = nil },
		func(b *Binding, _ *Signer) { b.Principals = append(b.Principals, b.Principals[0]) },
		func(b *Binding, _ *Signer) { b.Principals[0].PublicKey = nil },
		func(b *Binding, _ *Signer) { b.Principals[0].ID = "" },
		func(b *Binding, _ *Signer) { b.Principals[1].Owner.Domain = "foreign" },
		func(b *Binding, _ *Signer) { b.Principals[0].Owner = featuredrive.Owner{Domain: "local"} },
		func(b *Binding, _ *Signer) {
			b.Principals[0].Owner = featuredrive.Owner{Domain: "foreign", ID: "machine"}
		},
		func(_ *Binding, s *Signer) { s.ID = "unregistered" },
		func(_ *Binding, s *Signer) { s.Sign = nil },
		func(_ *Binding, s *Signer) { s.ID = "" },
	} {
		binding, signer := f.binding, f.operator
		binding.Principals = append([]Principal(nil), binding.Principals...)
		alter(&binding, &signer)
		if _, err := NewStore(f.repo, "origin", binding, signer); err == nil {
			t.Fatal("incomplete or conflicting registration was accepted")
		}
	}
	if _, err := NewStore("", "origin", f.binding, f.operator); err == nil {
		t.Fatal("missing storage operand was accepted")
	}
	if err := f.newStore(t, f.repo, f.owner).Create(t.Context(), f.initial()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("execution owner initialized a feature without operator authority: %v", err)
	}
	if err := f.store.CAS(t.Context(), f.binding.FeatureID, 0, f.initial()); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("unbound CAS version: %v", err)
	}
	if err := f.store.CAS(t.Context(), "foreign", 1, f.initial()); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("foreign CAS identity: %v", err)
	}
	initial := f.initial()
	initial.Created.Actor = "operator"
	initial.Created = featuredrive.SignTransition(initial.Created, f.keys["operator"])
	initial.Last = initial.Created
	// The store owns a copy of the registered public key, so a caller's later
	// mutation cannot silently change who authenticates an already-bound write.
	f.binding.Principals[0].PublicKey[0] ^= 1
	if err := f.store.Create(t.Context(), initial); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.Load(t.Context(), f.binding.FeatureID)
	if err != nil || !featuredrive.VerifyTransition(got.Created, f.keys["operator"].Public().(ed25519.PublicKey)) {
		t.Fatalf("registration alias changed authenticated creation: %+v, %v", got, err)
	}
	f.unchangedSource(t)
}

func TestSignedStoreRejectsCombinedOperatorAndOwnerRoles(t *testing.T) {
	for _, format := range []string{"sha1", "sha256"} {
		for _, index := range []int{1, 2} {
			t.Run(format+"/"+[]string{"local", "remote"}[index-1], func(t *testing.T) {
				f := newFixture(t, format)
				before := git(t, f.origin, "show-ref")
				binding := f.binding
				binding.Principals = append([]Principal(nil), binding.Principals...)
				binding.Principals[index].Operator = true
				for _, signer := range []Signer{f.operator, f.owner, f.foreign, {}} {
					store, err := NewStore(f.repo, "origin", binding, signer)
					if store != nil || !errors.Is(err, ErrUnauthorized) {
						t.Errorf("dual-role registration accepted for signer %q: %v", signer.ID, err)
					}
				}
				if git(t, f.origin, "show-ref") != before {
					t.Fatal("refused registration changed origin refs")
				}
				f.unchangedSource(t)
			})
		}
	}
}

func TestSignedStoreRequiresRegisteredAssignmentAndHandoffTargets(t *testing.T) {
	for _, kind := range []string{"assignment", "handoff"} {
		for _, target := range []featuredrive.Owner{{Domain: "local", ID: "unknown-machine"}, {Domain: "remote", ID: "unknown-worker"}, {Domain: "unknown", ID: "machine-1"}} {
			t.Run(kind+"/"+target.Domain, func(t *testing.T) {
				f := newFixture(t, "sha1")
				if err := f.store.Create(t.Context(), f.initial()); err != nil {
					t.Fatal(err)
				}
				prior, err := f.store.Load(t.Context(), f.binding.FeatureID)
				if err != nil {
					t.Fatal(err)
				}
				transition := "ownership-assigned"
				if kind == "handoff" {
					assigned := step(prior, transition)
					assigned.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}
					assigned.Last.NewEpoch = 1
					if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, assigned); err != nil {
						t.Fatal(err)
					}
					prior, err = f.store.Load(t.Context(), prior.FeatureID)
					if err != nil {
						t.Fatal(err)
					}
					quiescing := step(prior, "handoff-quiescing")
					quiescing.Ownership.State = featuredrive.Quiescing
					if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, quiescing); err != nil {
						t.Fatal(err)
					}
					prior, err = f.store.Load(t.Context(), prior.FeatureID)
					if err != nil {
						t.Fatal(err)
					}
					transition = "ownership-changed"
				}
				next := step(prior, transition)
				next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: target.Domain, OwnerID: target.ID, Epoch: prior.Ownership.Epoch + 1}
				next.Last.NewEpoch = next.Ownership.Epoch
				before := git(t, f.origin, "rev-parse", f.store.ref)
				if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, ErrUnauthorized) || git(t, f.origin, "rev-parse", f.store.ref) != before {
					t.Fatalf("unregistered ownership target published: %+v, %v", target, err)
				}
				f.unchangedSource(t)
			})
		}
	}
}

func TestSignedStoreColdReadRequiresRegisteredCurrentOwner(t *testing.T) {
	f := newFixture(t, "sha1")
	f.assign(t)
	binding := f.binding
	binding.Principals = []Principal{f.binding.Principals[0], f.binding.Principals[2]}
	reader, err := NewStore(f.repo, "origin", binding, Signer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Load(t.Context(), f.binding.FeatureID); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("operator signature authenticated an unregistered execution owner: %v", err)
	}
	f.unchangedSource(t)
}

type completedAttempt struct{}

func (completedAttempt) Quiesce(context.Context, string, featuredrive.Owner, uint64) (featuredrive.CheckpointEvidence, error) {
	return featuredrive.CheckpointEvidence{AttemptID: "attempt-one", Evidence: featuredrive.EvidenceRef{Kind: "checkpoint", ID: "attempt-one", Digest: "sha256:checkpoint"}, Complete: true}, nil
}

func TestSignedStoreSupportsCoreCancellationResumeAndAbandonment(t *testing.T) {
	for _, action := range []string{"resume", "abandon"} {
		t.Run(action, func(t *testing.T) {
			f := newFixture(t, "sha1")
			prior := f.assign(t)
			lifecycle := featuredrive.OwnershipLifecycle{Store: f.store, Quiescer: completedAttempt{}}
			cancelled, err := lifecycle.RequestCancellation(t.Context(), featuredrive.RequestCancellation{FeatureID: prior.FeatureID, Owner: featuredrive.OwnerEpoch{Owner: featuredrive.Owner{Domain: "local", ID: "machine-1"}, Epoch: 1}})
			if err != nil || cancelled.Phase != featuredrive.Cancelled || cancelled.Ownership.State != featuredrive.Released || cancelled.Ownership.Epoch != 2 {
				t.Fatalf("authenticated cancellation failed: %+v, %v", cancelled, err)
			}
			if action == "resume" {
				_, err = lifecycle.ResumeFeature(t.Context(), featuredrive.ResumeFeature{FeatureID: prior.FeatureID})
			} else {
				_, err = lifecycle.AbandonFeature(t.Context(), featuredrive.AbandonFeature{FeatureID: prior.FeatureID})
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.store.Load(t.Context(), prior.FeatureID)
			if err != nil || got.Ownership.State != featuredrive.Released || got.Ownership.Epoch != 2 || action == "resume" && got.Phase != featuredrive.Integrating || action == "abandon" && got.Phase != featuredrive.Abandoned {
				t.Fatalf("recovery changed released ownership: %+v, %v", got, err)
			}
			f.unchangedSource(t)
		})
	}
}

type cancellationAttempt struct{ complete bool }

func (q cancellationAttempt) Quiesce(ctx context.Context, id string, owner featuredrive.Owner, epoch uint64) (featuredrive.CheckpointEvidence, error) {
	evidence, err := (completedAttempt{}).Quiesce(ctx, id, owner, epoch)
	evidence.Complete = q.complete
	return evidence, err
}

func TestSignedCancellationRetryCanResumeAndAdvanceAfterColdRead(t *testing.T) {
	f := newFixture(t, "sha1")
	assigned := f.assign(t)
	lifecycle := featuredrive.OwnershipLifecycle{Store: f.store, Quiescer: cancellationAttempt{}}
	request := featuredrive.RequestCancellation{FeatureID: assigned.FeatureID, Owner: featuredrive.OwnerEpoch{Owner: featuredrive.Owner{Domain: "local", ID: "machine-1"}, Epoch: 1}}
	blocked, err := lifecycle.RequestCancellation(t.Context(), request)
	if !errors.Is(err, featuredrive.ErrQuiescence) || blocked.Blocker == nil {
		t.Fatalf("incomplete attempt was not retained: %+v, %v", blocked, err)
	}
	lifecycle.Quiescer = cancellationAttempt{complete: true}
	if _, err := lifecycle.RequestCancellation(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	reader := f.newStore(t, f.repo, Signer{})
	cancelled, err := reader.Load(t.Context(), assigned.FeatureID)
	if err != nil || cancelled.Blocker != nil || cancelled.Phase != featuredrive.Cancelled || cancelled.Ownership.Epoch != 2 {
		t.Fatalf("cold reader retained resolved cancellation state: %+v, %v", cancelled, err)
	}
	resumed, err := lifecycle.ResumeFeature(t.Context(), featuredrive.ResumeFeature{FeatureID: assigned.FeatureID})
	if err != nil {
		t.Fatal(err)
	}
	reassigned := step(resumed, "ownership-assigned")
	reassigned.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 3}
	reassigned.Last.NewEpoch = 3
	if err := f.store.CAS(t.Context(), resumed.FeatureID, resumed.Version, reassigned); err != nil {
		t.Fatal(err)
	}
	owner := f.newStore(t, f.repo, f.owner)
	prior, err := owner.Load(t.Context(), assigned.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	intent := step(prior, "effect-intent")
	intent.Effects = []featuredrive.EffectIntent{{ID: "integration-goal", Kind: "tasks-integrated", Subject: "all-tasks", RequiredEpoch: 3, Status: featuredrive.Pending}}
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
		t.Fatal(err)
	}
	prior, err = owner.Load(t.Context(), assigned.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	observed := step(prior, "effect-observed")
	observed.Effects = append([]featuredrive.EffectIntent(nil), prior.Effects...)
	observed.Effects[0].Status = featuredrive.Observed
	observed.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-landings", ID: "verified-task-landings", Digest: "sha256:landings"}
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, observed); err != nil {
		t.Fatal(err)
	}
	phase := featuredrive.PhaseLifecycle{Store: owner, Goals: phaseVerifier{}}
	if _, err := phase.Advance(t.Context(), assigned.FeatureID, featuredrive.OwnerEpoch{Owner: request.Owner.Owner, Epoch: 3}, "integration-goal"); err != nil {
		t.Fatalf("resumed drive could not advance its phase: %v", err)
	}
	current, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), assigned.FeatureID)
	if err != nil || current.Phase != featuredrive.Candidate || current.Blocker != nil || current.Ownership.Epoch != 3 {
		t.Fatalf("resumed phase did not survive cold replay: %+v, %v", current, err)
	}
	f.unchangedSource(t)
}

func TestSignedStoreHonorsCancellationAndNeverReinitializesMissingState(t *testing.T) {
	f := newFixture(t, "sha1")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := f.store.Load(ctx, f.binding.FeatureID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled state read: %v", err)
	}
	if _, err := f.store.sign(ctx, []byte("cancelled")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled signing operation: %v", err)
	}
	if err := f.store.CAS(t.Context(), f.binding.FeatureID, 1, f.initial()); !errors.Is(err, featuredrive.ErrNotFound) {
		t.Fatalf("CAS manufactured a missing feature: %v", err)
	}
	invalid := f.initial()
	invalid.Phase = featuredrive.Delivered
	if err := f.store.Create(t.Context(), invalid); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("creation invented a delivered feature: %v", err)
	}
	if git(t, f.origin, "rev-parse", f.store.ref) != f.binding.Genesis.Commit {
		t.Fatal("refused operations modified the initialized ref")
	}
	f.unchangedSource(t)
}
