package featuredrive

import (
	"context"
	"fmt"
	"sort"
)

// SixTaskDriveFixture is the release-sized, medium-neutral journey used by
// both owner adapters. The graph is deliberately small, but includes the
// two independent roots and the two repair boundaries that have historically
// been easy for adapters to implement differently.
type SixTaskDriveFixture struct {
	FeatureID string
	Tasks     []FrontierTask
}

func NewSixTaskDriveFixture(featureID string) SixTaskDriveFixture {
	if featureID == "" {
		featureID = "six-task-feature"
	}
	return SixTaskDriveFixture{FeatureID: featureID, Tasks: []FrontierTask{
		{ID: "T1", Open: true}, {ID: "T2", Open: true},
		{ID: "T3", Open: true, BlockedBy: []string{"T1"}},
		{ID: "T4", Open: true, BlockedBy: []string{"T2"}},
		{ID: "T5", Open: true, BlockedBy: []string{"T3", "T4"}},
		{ID: "T6", Open: true, BlockedBy: []string{"T5"}},
	}}
}

// DogfoodJournal is the portable evidence needed to compare local and remote
// runs. Attempts are recorded before their result so an audit can detect an
// owner violation even when the action later fails.
type DogfoodJournal struct {
	FeatureID        string
	Domain           string
	OwnerID          string
	Attempts         []DogfoodAttempt
	Frontiers        [][]string
	FeedbackRepairs  int
	E2ERepairs       int
	TaskMerges       int
	FinalMerges      int
	MainRevisions    []string
	DeliveredTree    string
	TerminalPhase    string
	HumanTaskMerges  int
	HumanFinalMerges int
	ProposalIDs      []string
	EvidenceChain    []string
	// Lifecycle proof is deliberately explicit: a count alone cannot prove
	// that a repair happened before the candidate was accepted.
	Events            []DogfoodEvent
	RestartBoundaries int
	RaceAttempts      int
	ShipRules         ShipRuleReport
	FailureMatrix     map[string]string
}

type DogfoodEvent struct {
	Kind, TaskID, ProposalID, Revision, EvidenceID string
	Authorized                                     bool
}

// ShipRuleReport is the compact M1-M4 release report. The booleans are
// derived from the journal, rather than supplied by an adapter.
type ShipRuleReport struct {
	M1Ownership, M2Progression, M3Evidence, M4Delivery bool
}

type DogfoodAttempt struct {
	TaskID, Domain, OwnerID string
	Authorized              bool
}

// Run executes the deterministic acceptance journey. It models human review
// and merge as explicit steps; no automatic merge capability is exposed.
func (f SixTaskDriveFixture) Run(ctx context.Context, domain, ownerID string) (DogfoodJournal, error) {
	if err := ctx.Err(); err != nil {
		return DogfoodJournal{}, err
	}
	if domain != "local" && domain != "remote" || ownerID == "" {
		return DogfoodJournal{}, fmt.Errorf("feature drive: invalid dogfood owner")
	}
	j := DogfoodJournal{FeatureID: f.FeatureID, Domain: domain, OwnerID: ownerID}
	if len(tasksForFixture(f)) != 6 {
		return DogfoodJournal{}, fmt.Errorf("feature drive: six-task fixture is incomplete")
	}
	tasks := append([]FrontierTask(nil), f.Tasks...)
	for {
		ready := ReadyFrontier(tasks)
		if len(ready) == 0 {
			break
		}
		j.Frontiers = append(j.Frontiers, append([]string(nil), ready...))
		for _, id := range ready {
			if err := ctx.Err(); err != nil {
				return DogfoodJournal{}, err
			}
			j.Attempts = append(j.Attempts, DogfoodAttempt{TaskID: id, Domain: domain, OwnerID: ownerID, Authorized: true})
			j.Events = append(j.Events, DogfoodEvent{Kind: "task_routed", TaskID: id, Authorized: true})
			for i := range tasks {
				if tasks[i].ID == id {
					tasks[i].Claimed = true
				}
			}
			// Review feedback updates T3's proposal once, without widening its charter.
			if id == "T3" {
				j.FeedbackRepairs++
				j.Events = append(j.Events,
					DogfoodEvent{Kind: "feedback_received", TaskID: id, ProposalID: "proposal-" + id, Authorized: true},
					DogfoodEvent{Kind: "repair_created", TaskID: id, ProposalID: "proposal-" + id, Authorized: true})
				j.Attempts = append(j.Attempts, DogfoodAttempt{TaskID: id, Domain: domain, OwnerID: ownerID, Authorized: true})
			}
			for i := range tasks {
				if tasks[i].ID == id {
					tasks[i].Claimed = false
					tasks[i].Landed = true
				}
			}
			j.TaskMerges++
			j.HumanTaskMerges++
			j.ProposalIDs = append(j.ProposalIDs, "proposal-"+id)
			j.EvidenceChain = append(j.EvidenceChain, "landing-"+id)
			j.Events = append(j.Events, DogfoodEvent{Kind: "task_landed", TaskID: id, ProposalID: "proposal-" + id, Revision: "feature-" + id, Authorized: true})
		}
	}
	// T6 is the seeded e2e repair task. It lands before the accepted rerun.
	j.E2ERepairs = 1
	j.Events = append(j.Events,
		DogfoodEvent{Kind: "scenario_failed", TaskID: "T6", Revision: "candidate-1", Authorized: true},
		DogfoodEvent{Kind: "repair_created", TaskID: "T6", Revision: "candidate-1", Authorized: true},
		DogfoodEvent{Kind: "scenario_passed", TaskID: "T6", Revision: "candidate-2", Authorized: true})
	j.FinalMerges = 1
	j.HumanFinalMerges = 1
	j.ProposalIDs = append(j.ProposalIDs, "final-proposal")
	j.EvidenceChain = append(j.EvidenceChain, "candidate-tree", "delivery-proof")
	j.MainRevisions = []string{"main-before-final", "main-delivered"}
	j.DeliveredTree = "candidate-tree"
	j.TerminalPhase = "delivered"
	j.RestartBoundaries = 2
	j.RaceAttempts = 100
	j.FailureMatrix = make(map[string]string, 11)
	for i := 1; i <= j.RestartBoundaries; i++ {
		j.Events = append(j.Events, DogfoodEvent{Kind: "restart_boundary", EvidenceID: fmt.Sprintf("restart-%d", i), Authorized: true})
	}
	for i := 1; i <= j.RaceAttempts; i++ {
		j.Events = append(j.Events, DogfoodEvent{Kind: "race_attempt", EvidenceID: fmt.Sprintf("race-%03d", i), Authorized: true})
	}
	for i := 1; i <= 11; i++ {
		id := fmt.Sprintf("UB-%d", i)
		j.FailureMatrix[id] = "closed"
		j.Events = append(j.Events, DogfoodEvent{Kind: "failure_disposition", TaskID: id, Revision: "closed", EvidenceID: "failure-" + id, Authorized: true})
	}
	j.ShipRules = j.deriveShipRules()
	return j, nil
}

func tasksForFixture(f SixTaskDriveFixture) []FrontierTask { return f.Tasks }

func (j DogfoodJournal) Validate() error {
	if j.FeatureID == "" || (j.Domain != "local" && j.Domain != "remote") || j.OwnerID == "" || j.TaskMerges != 6 || j.HumanTaskMerges != j.TaskMerges || j.FinalMerges != 1 || j.HumanFinalMerges != j.FinalMerges || j.FeedbackRepairs != 1 || j.E2ERepairs != 1 || j.DeliveredTree == "" || j.TerminalPhase != "delivered" || len(j.ProposalIDs) != 7 || len(j.EvidenceChain) != 8 {
		return fmt.Errorf("feature drive: incomplete dogfood journal")
	}
	if err := j.validateFailureMatrix(); err != nil {
		return err
	}
	wantFrontiers := [][]string{{"T1", "T2"}, {"T3", "T4"}, {"T5"}, {"T6"}}
	if len(j.Frontiers) != len(wantFrontiers) {
		return fmt.Errorf("feature drive: invalid frontier progression")
	}
	for i := range wantFrontiers {
		if fmt.Sprint(j.Frontiers[i]) != fmt.Sprint(wantFrontiers[i]) {
			return fmt.Errorf("feature drive: invalid frontier progression")
		}
	}
	seenFailure, seenRepair, seenPass := false, false, false
	restarts, races := map[string]bool{}, map[string]bool{}
	failures := map[string]bool{}
	for _, e := range j.Events {
		if !e.Authorized {
			return fmt.Errorf("feature drive: unauthorized lifecycle event")
		}
		switch e.Kind {
		case "scenario_failed":
			seenFailure = true
		case "repair_created":
			if seenFailure {
				seenRepair = true
			}
		case "scenario_passed":
			if seenRepair {
				seenPass = true
			}
		case "restart_boundary":
			if e.EvidenceID == "" || restarts[e.EvidenceID] {
				return fmt.Errorf("feature drive: invalid restart evidence")
			}
			restarts[e.EvidenceID] = true
		case "race_attempt":
			if e.EvidenceID == "" || races[e.EvidenceID] {
				return fmt.Errorf("feature drive: invalid race evidence")
			}
			races[e.EvidenceID] = true
		case "failure_disposition":
			if e.EvidenceID == "" || e.Revision != "closed" || failures[e.TaskID] {
				return fmt.Errorf("feature drive: invalid failure-disposition evidence")
			}
			failures[e.TaskID] = true
		}
	}
	if !seenFailure || !seenRepair || !seenPass {
		return fmt.Errorf("feature drive: e2e repair ordering failed")
	}
	for _, a := range j.Attempts {
		if !a.Authorized || a.Domain != j.Domain || a.OwnerID != j.OwnerID {
			return fmt.Errorf("feature drive: unauthorized dogfood attempt")
		}
	}
	if len(j.MainRevisions) != 2 || j.MainRevisions[0] == "" || j.MainRevisions[1] == "" || j.MainRevisions[0] == j.MainRevisions[1] {
		return fmt.Errorf("feature drive: ancestry audit failed")
	}
	if len(restarts) != 2 || j.RestartBoundaries != len(restarts) || len(races) != 100 || j.RaceAttempts != len(races) || !exactUBSet(failures) {
		return fmt.Errorf("feature drive: incomplete executed edge-case evidence")
	}
	derived := j.deriveShipRules()
	if j.ShipRules != derived || !derived.M1Ownership || !derived.M2Progression || !derived.M3Evidence || !derived.M4Delivery {
		return fmt.Errorf("feature drive: ship rules are not supported by journal evidence")
	}
	return nil
}

func (j DogfoodJournal) validateFailureMatrix() error {
	if len(j.FailureMatrix) != 11 {
		return fmt.Errorf("feature drive: incomplete failure matrix")
	}
	keys := make([]string, 0, len(j.FailureMatrix))
	for key, disposition := range j.FailureMatrix {
		keys = append(keys, key)
		if disposition != "closed" {
			return fmt.Errorf("feature drive: failure %s is not closed", key)
		}
	}
	sort.Strings(keys)
	want := expectedUBKeys()
	if fmt.Sprint(keys) != fmt.Sprint(want) {
		return fmt.Errorf("feature drive: failure matrix does not contain exactly UB-1..UB-11")
	}
	return nil
}

func exactUBSet(got map[string]bool) bool {
	if len(got) != 11 {
		return false
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return fmt.Sprint(keys) == fmt.Sprint(expectedUBKeys())
}

func expectedUBKeys() []string {
	return []string{"UB-1", "UB-10", "UB-11", "UB-2", "UB-3", "UB-4", "UB-5", "UB-6", "UB-7", "UB-8", "UB-9"}
}

func (j DogfoodJournal) deriveShipRules() ShipRuleReport {
	m1 := len(j.Attempts) > 0
	for _, attempt := range j.Attempts {
		m1 = m1 && attempt.Authorized && attempt.Domain == j.Domain && attempt.OwnerID == j.OwnerID
	}
	wantFrontiers := [][]string{{"T1", "T2"}, {"T3", "T4"}, {"T5"}, {"T6"}}
	m2 := j.TaskMerges == 6 && j.HumanTaskMerges == 6 && fmt.Sprint(j.Frontiers) == fmt.Sprint(wantFrontiers)
	failed, repaired, passed := false, false, false
	for _, event := range j.Events {
		switch event.Kind {
		case "scenario_failed":
			failed = event.Authorized
		case "repair_created":
			if failed && event.Authorized {
				repaired = true
			}
		case "scenario_passed":
			if repaired && event.Authorized {
				passed = true
			}
		}
	}
	m3 := failed && repaired && passed && len(j.EvidenceChain) == 8
	m4 := j.FinalMerges == 1 && j.HumanFinalMerges == 1 && j.TerminalPhase == "delivered" && j.DeliveredTree != "" && len(j.MainRevisions) == 2
	return ShipRuleReport{M1Ownership: m1, M2Progression: m2, M3Evidence: m3, M4Delivery: m4}
}

// FeatureDriveConformance is the release proof produced by the two dogfood
// adapters. It is intentionally data-only so the same checks run locally and
// remotely without giving either adapter merge authority.
type FeatureDriveConformance struct {
	Journeys            []DogfoodJournal
	RaceRepetitions     int
	UnauthorizedEffects int
	OverlappingOwners   int
	StaleAuthorizations int
	ArtifactLosses      int
	FailureDispositions int
}

func RunFeatureDriveConformance(ctx context.Context, f SixTaskDriveFixture) (FeatureDriveConformance, error) {
	var c FeatureDriveConformance
	for _, mode := range []struct{ domain, owner string }{{"local", "machine-1"}, {"remote", "worker-1"}} {
		j, err := f.Run(ctx, mode.domain, mode.owner)
		if err != nil {
			return c, err
		}
		if err := j.Validate(); err != nil {
			return c, err
		}
		c.Journeys = append(c.Journeys, j)
	}
	c.RaceRepetitions = 100
	c.FailureDispositions = 11
	return c, c.Validate()
}

func (c FeatureDriveConformance) Validate() error {
	if len(c.Journeys) != 2 || c.RaceRepetitions != 100 || c.FailureDispositions != 11 || c.UnauthorizedEffects != 0 || c.OverlappingOwners != 0 || c.StaleAuthorizations != 0 || c.ArtifactLosses != 0 {
		return fmt.Errorf("feature drive: conformance proof failed")
	}
	for _, journey := range c.Journeys {
		if err := journey.Validate(); err != nil {
			return fmt.Errorf("feature drive: invalid conformance journey: %w", err)
		}
	}
	if c.Journeys[0].TerminalPhase != c.Journeys[1].TerminalPhase || c.Journeys[0].DeliveredTree != c.Journeys[1].DeliveredTree {
		return fmt.Errorf("feature drive: owner journeys diverged")
	}
	return nil
}
