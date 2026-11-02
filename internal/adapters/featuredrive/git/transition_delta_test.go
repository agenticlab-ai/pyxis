package gitdrive

import (
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

func TestEffectIntentPreservesPriorEffectsAndLifecycle(t *testing.T) {
	prior := featuredrive.FeatureDrive{Ownership: featuredrive.Ownership{State: featuredrive.Active, Domain: "local", OwnerID: "machine-1", Epoch: 1}, Phase: featuredrive.Integrating,
		Effects: []featuredrive.EffectIntent{{ID: "existing", Kind: "dispatch", InputDigest: "sha256:original", RequiredEpoch: 1, Status: featuredrive.Pending}}}
	for _, tc := range []struct {
		name string
		edit func(*featuredrive.FeatureDrive)
	}{
		{"removed existing", func(d *featuredrive.FeatureDrive) { d.Effects = d.Effects[1:] }},
		{"rewritten input", func(d *featuredrive.FeatureDrive) { d.Effects[0].InputDigest = "sha256:changed" }},
		{"rewritten attempts", func(d *featuredrive.FeatureDrive) { d.Effects[0].Attempts++ }},
		{"reordered effects", func(d *featuredrive.FeatureDrive) { d.Effects[0], d.Effects[1] = d.Effects[1], d.Effects[0] }},
		{"completed existing", func(d *featuredrive.FeatureDrive) {
			d.Effects[0].Status = featuredrive.Observed
			d.Effects[0].Observation = &featuredrive.EvidenceRef{Kind: "result", ID: "invented"}
		}},
		{"new observation", func(d *featuredrive.FeatureDrive) {
			d.Effects[1].Status = featuredrive.Observed
			d.Effects[1].Observation = &featuredrive.EvidenceRef{Kind: "result", ID: "invented"}
		}},
		{"pending with observation", func(d *featuredrive.FeatureDrive) {
			d.Effects[1].Observation = &featuredrive.EvidenceRef{Kind: "result", ID: "invented"}
		}},
		{"no new intent", func(d *featuredrive.FeatureDrive) { d.Effects = d.Effects[:1] }},
		{"changed phase", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.Candidate }},
		{"changed frontier", func(d *featuredrive.FeatureDrive) { d.Frontier = []string{"T2"} }},
		{"changed evidence", func(d *featuredrive.FeatureDrive) {
			d.Evidence = []featuredrive.EvidenceRef{{Kind: "result", ID: "invented"}}
		}},
		{"changed wait", func(d *featuredrive.FeatureDrive) { d.Wait = &featuredrive.Wait{Kind: "human-review"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := step(prior, "effect-intent")
			next.Effects = append(append([]featuredrive.EffectIntent(nil), prior.Effects...), featuredrive.EffectIntent{ID: "new", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending})
			if err := validateOwnershipStep(prior, next); err != nil {
				t.Fatalf("valid append refused: %v", err)
			}
			tc.edit(&next)
			if err := validateOwnershipStep(prior, next); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Fatalf("intent changed an existing fact: %v", err)
			}
		})
	}
}

func TestHandoffPreservesLifecycleAndExistingEvidence(t *testing.T) {
	prior := featuredrive.FeatureDrive{Ownership: featuredrive.Ownership{State: featuredrive.Quiescing, Domain: "local", OwnerID: "machine-1", Epoch: 1}, Phase: featuredrive.Integrating,
		Evidence: []featuredrive.EvidenceRef{{Kind: "plan", ID: "retained"}}, Blocker: &featuredrive.EvidenceRef{Kind: "active-attempt", ID: "attempt"}}
	for _, tc := range []struct {
		name string
		edit func(*featuredrive.FeatureDrive)
	}{
		{"false delivery", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.Delivered }},
		{"changed executable phase", func(d *featuredrive.FeatureDrive) { d.Phase = featuredrive.Candidate }},
		{"rewritten evidence", func(d *featuredrive.FeatureDrive) { d.Evidence[0].ID = "invented" }},
		{"removed evidence", func(d *featuredrive.FeatureDrive) { d.Evidence = d.Evidence[1:] }},
		{"missing quiescence", func(d *featuredrive.FeatureDrive) { d.Evidence = d.Evidence[:1] }},
		{"extra evidence", func(d *featuredrive.FeatureDrive) {
			d.Evidence = append(d.Evidence, featuredrive.EvidenceRef{Kind: "result", ID: "invented"})
		}},
		{"changed frontier", func(d *featuredrive.FeatureDrive) { d.Frontier = []string{"T2"} }},
		{"changed wait", func(d *featuredrive.FeatureDrive) { d.Wait = &featuredrive.Wait{Kind: "human-review"} }},
		{"new effects", func(d *featuredrive.FeatureDrive) {
			d.Effects = []featuredrive.EffectIntent{{ID: "invented", Kind: "dispatch", RequiredEpoch: 2, Status: featuredrive.Pending}}
		}},
		{"changed blocker", func(d *featuredrive.FeatureDrive) {
			d.Blocker = &featuredrive.EvidenceRef{Kind: "other", ID: "invented"}
		}},
		{"changed failure", func(d *featuredrive.FeatureDrive) { d.Failure = &featuredrive.FailureDisposition{Trigger: "invented"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := step(prior, "ownership-changed")
			next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "remote", OwnerID: "worker-1", Epoch: 2}
			next.Last.NewEpoch = 2
			next.Evidence = append(append([]featuredrive.EvidenceRef(nil), prior.Evidence...), featuredrive.EvidenceRef{Kind: "checkpoint", ID: "attempt"})
			next.Blocker = nil
			if err := validateOwnershipStep(prior, next); err != nil {
				t.Fatalf("valid handoff refused: %v", err)
			}
			tc.edit(&next)
			if err := validateOwnershipStep(prior, next); !errors.Is(err, featuredrive.ErrMalformed) {
				t.Fatalf("handoff changed unrelated state: %v", err)
			}
		})
	}
}

func TestSignedStoreRejectsChangedIntentOnWriteAndColdRead(t *testing.T) {
	for _, change := range []string{"remove pending", "rewrite input", "append observed"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t, "sha1")
			prior := f.assign(t)
			intent := step(prior, "effect-intent")
			intent.Effects = []featuredrive.EffectIntent{{ID: "existing", Kind: "dispatch", InputDigest: "sha256:original", RequiredEpoch: 1, Status: featuredrive.Pending}}
			if err := f.newStore(t, f.repo, f.owner).CAS(t.Context(), prior.FeatureID, prior.Version, intent); err != nil {
				t.Fatal(err)
			}
			prior, err := f.store.Load(t.Context(), prior.FeatureID)
			if err != nil {
				t.Fatal(err)
			}
			next := step(prior, "effect-intent")
			next.Effects = append(append([]featuredrive.EffectIntent(nil), prior.Effects...), featuredrive.EffectIntent{ID: "new", Kind: "dispatch", RequiredEpoch: 1, Status: featuredrive.Pending})
			switch change {
			case "remove pending":
				next.Effects = next.Effects[1:]
			case "rewrite input":
				next.Effects[0].InputDigest = "sha256:changed"
			case "append observed":
				next.Effects[1].Status = featuredrive.Observed
				next.Effects[1].Observation = &featuredrive.EvidenceRef{Kind: "result", ID: "invented"}
			}
			before := git(t, f.origin, "show-ref")
			for _, signer := range []Signer{f.owner, f.operator} {
				if err := f.newStore(t, f.repo, signer).CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
					t.Errorf("%s accepted %s: %v", signer.ID, change, err)
				}
			}
			if git(t, f.origin, "show-ref") != before {
				t.Fatal("refused intent changed origin refs")
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
				t.Fatalf("signed invalid intent survived cold read: %v", err)
			}
			f.unchangedSource(t)
		})
	}
}

func TestSignedStoreRejectsFalseHandoffDeliveryOnWriteAndColdRead(t *testing.T) {
	f := newFixture(t, "sha1")
	prior := f.revokeRouting(t, f.assign(t), "handoff-quiescing")
	next := step(prior, "ownership-changed")
	next.Ownership = featuredrive.Ownership{State: featuredrive.Active, Domain: "remote", OwnerID: "worker-1", Epoch: 2}
	next.Last.NewEpoch = 2
	next.Evidence = []featuredrive.EvidenceRef{{Kind: "checkpoint", ID: "attempt"}}
	next.Phase = featuredrive.Delivered
	before := git(t, f.origin, "show-ref")
	if err := f.store.CAS(t.Context(), prior.FeatureID, prior.Version, next); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Errorf("handoff fabricated delivery: %v", err)
	}
	if git(t, f.origin, "show-ref") != before {
		t.Fatal("refused handoff changed origin refs")
	}
	record, parent, body := f.currentRecord(t)
	record.Parent, record.PreviousDigest, record.Drive = parent, contentDigest(body), next
	f.resign(t, &record, f.operator.ID)
	encoded, err := record.canonical()
	if err != nil {
		t.Fatal(err)
	}
	f.appendFiles(t, map[string][]byte{drivePath: encoded})
	if _, err := f.newStore(t, f.repo, Signer{}).Load(t.Context(), prior.FeatureID); !errors.Is(err, featuredrive.ErrMalformed) {
		t.Fatalf("signed false delivery survived cold read: %v", err)
	}
	f.unchangedSource(t)
}
