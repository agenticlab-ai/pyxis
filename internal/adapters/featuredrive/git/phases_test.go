package gitdrive

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

type phaseCapture struct{ prior, next featuredrive.FeatureDrive }

func (s *phaseCapture) Load(context.Context, string) (featuredrive.FeatureDrive, error) {
	return s.prior, nil
}
func (*phaseCapture) Create(context.Context, featuredrive.FeatureDrive) error {
	return featuredrive.ErrMalformed
}
func (s *phaseCapture) CAS(_ context.Context, _ string, _ uint64, next featuredrive.FeatureDrive) error {
	s.next = next
	return nil
}

type phaseVerifier struct{}

func (phaseVerifier) VerifyPhaseGoal(_ context.Context, _ featuredrive.FeatureDrive, effect featuredrive.EffectIntent) (featuredrive.EvidenceRef, error) {
	return *effect.Observation, nil
}

func (f *fixture) phaseCandidate(t *testing.T) (featuredrive.FeatureDrive, featuredrive.FeatureDrive) {
	t.Helper()
	prior := f.assign(t)
	owner := f.newStore(t, f.repo, f.owner)
	intent := step(prior, "effect-intent")
	intent.Effects = []featuredrive.EffectIntent{{ID: "integration-goal", Kind: "tasks-integrated", Subject: "all-tasks", RequiredEpoch: 1, Status: featuredrive.Pending}}
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
		t.Fatal(err)
	}
	prior, err := owner.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	observed := step(prior, "effect-observed")
	observed.Effects = slices.Clone(prior.Effects)
	observed.Effects[0].Status = featuredrive.Observed
	observed.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "task-landings", ID: "verified-task-landings", Digest: "sha256:landings"}
	if err := owner.CAS(t.Context(), prior.FeatureID, prior.Version, observed); err != nil {
		t.Fatal(err)
	}
	prior, err = owner.Load(t.Context(), prior.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	capture := &phaseCapture{prior: prior}
	_, err = (featuredrive.PhaseLifecycle{Store: capture, Goals: phaseVerifier{}}).Advance(t.Context(), prior.FeatureID, featuredrive.OwnerEpoch{Owner: featuredrive.Owner{Domain: "local", ID: "machine-1"}, Epoch: 1}, "integration-goal")
	if err != nil {
		t.Fatal(err)
	}
	return prior, capture.next
}

func TestSignedStorePhaseRefusalsOnWriteAndColdReplay(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor string
		edit  func(*featuredrive.FeatureDrive)
		want  error
	}{
		{"operator cannot execute", "operator", func(*featuredrive.FeatureDrive) {}, ErrUnauthorized},
		{"foreign owner cannot execute", "worker-1", func(*featuredrive.FeatureDrive) {}, ErrUnauthorized},
		{"skipped phase", "machine-1", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.FinalReview }, featuredrive.ErrMalformed},
		{"missing proof", "machine-1", func(d *featuredrive.FeatureDrive) { d.Evidence = nil }, featuredrive.ErrMalformed},
		{"changed proof", "machine-1", func(d *featuredrive.FeatureDrive) { d.Evidence[0].Digest = "sha256:invented" }, featuredrive.ErrMalformed},
		{"changed effects", "machine-1", func(d *featuredrive.FeatureDrive) { d.Effects = nil }, featuredrive.ErrMalformed},
		{"stale epoch", "machine-1", func(d *featuredrive.FeatureDrive) { d.Last.PriorEpoch = 0 }, featuredrive.ErrMalformed},
		{"changed plan", "machine-1", func(d *featuredrive.FeatureDrive) { d.PlanFingerprint = "other-plan" }, featuredrive.ErrMalformed},
	} {
		for _, route := range []string{"write", "cold-replay"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				f := newFixture(t, "sha1")
				prior, next := f.phaseCandidate(t)
				tc.edit(&next)
				before := git(t, f.origin, "show-ref")
				if route == "write" {
					signer := f.owner
					if tc.actor == "operator" {
						signer = f.operator
					}
					if tc.actor == "worker-1" {
						signer = f.foreign
					}
					if err := f.newStore(t, f.repo, signer).CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, tc.want) {
						t.Fatalf("refused phase accepted: %v", err)
					}
					if git(t, f.origin, "show-ref") != before {
						t.Fatal("refused phase changed origin refs")
					}
				} else {
					record, parent, body := f.currentRecord(t)
					record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
					f.resign(t, &record, tc.actor)
					encoded, err := record.canonical()
					if err != nil {
						t.Fatal(err)
					}
					f.appendFiles(t, map[string][]byte{drivePath: encoded})
					inserted := git(t, f.origin, "show-ref")
					if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, tc.want) {
						t.Fatalf("signed invalid phase survived cold read: %v", err)
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
