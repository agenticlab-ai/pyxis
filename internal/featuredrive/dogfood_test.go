package featuredrive

import (
	"context"
	"testing"
)

func TestSixTaskDogfoodEquivalentAcrossOwners(t *testing.T) {
	f := NewSixTaskDriveFixture("acceptance")
	local, err := f.Run(context.Background(), "local", "machine-1")
	if err != nil {
		t.Fatal(err)
	}
	remote, err := f.Run(context.Background(), "remote", "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := local.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := remote.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(local.Frontiers) != len(remote.Frontiers) || local.TaskMerges != remote.TaskMerges || local.DeliveredTree != remote.DeliveredTree {
		t.Fatalf("local and remote journeys diverged: %#v %#v", local, remote)
	}
	for _, j := range []DogfoodJournal{local, remote} {
		for _, a := range j.Attempts {
			if a.Domain != j.Domain {
				t.Fatalf("foreign attempt: %#v", a)
			}
		}
	}
}

func TestSixTaskDogfoodRejectsForeignOwner(t *testing.T) {
	j, err := NewSixTaskDriveFixture("acceptance").Run(context.Background(), "remote", "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	j.Attempts[0].OwnerID = "machine-1"
	if err := j.Validate(); err == nil {
		t.Fatal("expected owner violation")
	}
}

func TestFeatureDriveConformancePublishesEquivalentProof(t *testing.T) {
	c, err := RunFeatureDriveConformance(context.Background(), NewSixTaskDriveFixture("acceptance"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.RaceRepetitions != 100 || c.FailureDispositions != 11 {
		t.Fatalf("proof does not cover the required corpus: %#v", c)
	}
}

func TestFeatureDriveConformanceRejectsUnauthorizedProof(t *testing.T) {
	c, err := RunFeatureDriveConformance(context.Background(), NewSixTaskDriveFixture("acceptance"))
	if err != nil {
		t.Fatal(err)
	}
	c.UnauthorizedEffects = 1
	if err := c.Validate(); err == nil {
		t.Fatal("expected unauthorized effect to fail closed")
	}
}

func TestDogfoodRejectsOpenOrMissingFailureDisposition(t *testing.T) {
	for _, mutate := range []func(*DogfoodJournal){
		func(j *DogfoodJournal) { j.FailureMatrix["UB-1"] = "open" },
		func(j *DogfoodJournal) {
			delete(j.FailureMatrix, "UB-11")
			j.FailureMatrix["not-a-required-case"] = "closed"
		},
	} {
		j, err := NewSixTaskDriveFixture("acceptance").Run(context.Background(), "local", "machine-1")
		if err != nil {
			t.Fatal(err)
		}
		mutate(&j)
		if err := j.Validate(); err == nil {
			t.Fatal("expected forged failure matrix to fail closed")
		}
	}
}

func TestDogfoodRejectsForgedRaceRestartAndShipRuleEvidence(t *testing.T) {
	j, err := NewSixTaskDriveFixture("acceptance").Run(context.Background(), "local", "machine-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*DogfoodJournal){
		func(j *DogfoodJournal) { j.Events = removeDogfoodEvent(j.Events, "race_attempt") },
		func(j *DogfoodJournal) { j.Events = removeDogfoodEvent(j.Events, "restart_boundary") },
		func(j *DogfoodJournal) {
			for i := range j.Events {
				if j.Events[i].Kind == "failure_disposition" && j.Events[i].TaskID == "UB-11" {
					j.Events[i].TaskID = "not-a-required-case"
					break
				}
			}
		},
		func(j *DogfoodJournal) { j.ShipRules.M4Delivery = false },
	} {
		forged := j
		forged.Events = append([]DogfoodEvent(nil), j.Events...)
		mutate(&forged)
		if err := forged.Validate(); err == nil {
			t.Fatal("expected forged conformance evidence to fail closed")
		}
	}
}

func TestFeatureDriveConformanceRejectsSkeletalJourneys(t *testing.T) {
	c := FeatureDriveConformance{
		Journeys: []DogfoodJournal{
			{TerminalPhase: "delivered", DeliveredTree: "tree"},
			{TerminalPhase: "delivered", DeliveredTree: "tree"},
		},
		RaceRepetitions:     100,
		FailureDispositions: 11,
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected counters without journey evidence to fail closed")
	}
}

func removeDogfoodEvent(events []DogfoodEvent, kind string) []DogfoodEvent {
	out := make([]DogfoodEvent, 0, len(events))
	removed := false
	for _, event := range events {
		if !removed && event.Kind == kind {
			removed = true
			continue
		}
		out = append(out, event)
	}
	return out
}
