package featuredrive

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

const ExecutableDogfoodSchema = "feature-drive-dogfood/v1"

var dogfoodAbs = filepath.Abs

// ExecutableDogfoodReport is the inspectable proof emitted by the dogfood
// command. Each owner mode runs in its own real Git repository; CandidateOne
// and CandidateTwo are built by the production Git candidate adapter.
type ExecutableDogfoodReport struct {
	Schema   string                     `json:"schema"`
	Root     string                     `json:"root"`
	Journeys []ExecutableDogfoodJourney `json:"journeys"`
}

type ExecutableDogfoodJourney struct {
	Domain        string            `json:"domain"`
	OwnerID       string            `json:"owner_id"`
	Repository    string            `json:"repository"`
	TaskHeads     map[string]string `json:"task_heads"`
	CandidateOne  CandidateIdentity `json:"candidate_one"`
	CandidateTwo  CandidateIdentity `json:"candidate_two"`
	MainBefore    string            `json:"main_before"`
	MainDelivered string            `json:"main_delivered"`
	Journal       DogfoodJournal    `json:"journal"`
}

func RunExecutableFeatureDriveConformance(ctx context.Context, root string) (ExecutableDogfoodReport, error) {
	if root == "" {
		return ExecutableDogfoodReport{}, fmt.Errorf("feature drive: dogfood workspace is required")
	}
	abs, err := dogfoodAbs(root)
	if err != nil {
		return ExecutableDogfoodReport{}, fmt.Errorf("feature drive: resolve dogfood workspace: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return ExecutableDogfoodReport{}, fmt.Errorf("feature drive: create dogfood workspace: %w", err)
	}
	report := ExecutableDogfoodReport{Schema: ExecutableDogfoodSchema, Root: abs}
	for _, owner := range []struct{ domain, id string }{{"local", "machine-1"}, {"remote", "worker-1"}} {
		journey, err := runExecutableJourney(ctx, abs, owner.domain, owner.id)
		if err != nil {
			return report, fmt.Errorf("feature drive: %s dogfood: %w", owner.domain, err)
		}
		report.Journeys = append(report.Journeys, journey)
	}
	if err := report.Validate(ctx); err != nil {
		return report, err
	}
	return report, nil
}

func runExecutableJourney(ctx context.Context, root, domain, ownerID string) (ExecutableDogfoodJourney, error) {
	repo := filepath.Join(root, domain)
	if err := os.Mkdir(repo, 0o755); err != nil {
		return ExecutableDogfoodJourney{}, fmt.Errorf("create repository: %w", err)
	}
	for _, args := range [][]string{{"init"}, {"config", "user.name", "Pyxis Dogfood"}, {"config", "user.email", "dogfood@invalid"}} {
		if _, err := gitOutput(ctx, repo, args...); err != nil {
			return ExecutableDogfoodJourney{}, err
		}
	}
	if err := writeAndCommit(ctx, repo, "README.md", "sample feature\n", "sample: establish main"); err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	targetRef, err := gitOutput(ctx, repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	mainBefore, err := gitOutput(ctx, repo, "rev-parse", "HEAD")
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	fixture := NewSixTaskDriveFixture("sample-feature")
	store := NewMemoryStore()
	created := Transition{Kind: "create", NewVersion: 1}
	if err := store.Create(ctx, FeatureDrive{Schema: Schema, Repository: repo, FeatureID: fixture.FeatureID, PlanFingerprint: "sample-plan", TargetMainRef: mainBefore, IntegrationRef: "refs/heads/feature/" + fixture.FeatureID, Version: 1, Created: created, Last: created, Ownership: Ownership{State: Unowned}, Phase: Integrating}); err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	kickoff := Kickoff{
		Resolver: dogfoodResolver{feature: FrozenFeature{ID: fixture.FeatureID, Repository: repo, PlanFingerprint: "sample-plan", TargetMainRef: mainBefore, Eligible: true}},
		Identity: dogfoodIdentity{}, Refs: dogfoodIntegrationRef{repo: repo}, Routing: dogfoodRouting{tasks: fixture.Tasks},
		Executor: dogfoodExecutor{}, Drives: store,
	}
	if _, err := kickoff.KickoffFeature(ctx, fixture.FeatureID, ExecutionAssignment{Domain: domain, OwnerID: ownerID}); err != nil {
		return ExecutableDogfoodJourney{}, fmt.Errorf("kick off feature: %w", err)
	}
	if _, err := gitOutput(ctx, repo, "checkout", "feature/"+fixture.FeatureID); err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	taskHeads := make(map[string]string, 6)
	landings := &dogfoodLandingRecorder{landed: map[string]string{}}
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("T%d", i)
		path := filepath.Join("tasks", id+".txt")
		if err := writeAndCommit(ctx, repo, path, "implemented "+id+"\n", "feat(sample): land "+id); err != nil {
			return ExecutableDogfoodJourney{}, err
		}
		head, err := gitOutput(ctx, repo, "rev-parse", "HEAD")
		if err != nil {
			return ExecutableDogfoodJourney{}, err
		}
		taskHeads[id] = head
		proposal := ProposalObservation{TaskID: id, ProposalID: "proposal-" + id, Head: head, Base: "refs/heads/feature/" + fixture.FeatureID, Landed: head, ChecksGreen: true, Open: true}
		review := ReviewObservation{ProposalID: proposal.ProposalID, Head: head, State: "approved"}
		if err := RecordLanding(ctx, dogfoodLandingVerifier{}, landings, proposal, review, head); err != nil {
			return ExecutableDogfoodJourney{}, fmt.Errorf("record reviewed landing %s: %w", id, err)
		}
	}
	builder := NewGitCandidateBuilder(repo)
	publisher := NewGitCandidatePublisher(repo)
	featureHead, err := gitOutput(ctx, repo, "rev-parse", "feature/sample-feature")
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	candidateOne, err := BuildCandidate(ctx, "sample-feature", featureHead, mainBefore, MergePolicy{Version: "dogfood-v1"}, "feature-drive-dogfood/v1", builder, publisher)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	if err := writeAndCommit(ctx, repo, filepath.Join("repairs", "e2e.txt"), "seeded e2e repair\n", "fix(sample): repair seeded e2e failure"); err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	featureHead, err = gitOutput(ctx, repo, "rev-parse", "feature/sample-feature")
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	candidateTwo, err := BuildCandidate(ctx, "sample-feature", featureHead, mainBefore, MergePolicy{Version: "dogfood-v1"}, "feature-drive-dogfood/v1", builder, publisher)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	if candidateOne.Digest() == candidateTwo.Digest() {
		return ExecutableDogfoodJourney{}, fmt.Errorf("repair did not invalidate candidate")
	}
	if _, err := gitOutput(ctx, repo, "update-ref", "refs/heads/"+targetRef, candidateTwo.Commit, mainBefore); err != nil {
		return ExecutableDogfoodJourney{}, fmt.Errorf("deliver candidate to main: %w", err)
	}
	mainDelivered, err := gitOutput(ctx, repo, "rev-parse", targetRef)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	proposal := FinalProposal{FeatureID: fixture.FeatureID, ProposalID: "final-proposal", Head: candidateTwo.Commit, FeatureHead: candidateTwo.FeatureHead, MainHead: candidateTwo.MainHead, Candidate: candidateTwo, ScenarioSetFingerprint: "dogfood-scenarios/v1", EvidenceID: candidateTwo.Digest()}
	accepted := AcceptedCandidate{Candidate: candidateTwo, Proposal: proposal}
	landed := LandedRevision{Revision: mainDelivered, Base: candidateTwo.MainHead, Tree: candidateTwo.Tree}
	if err := VerifyDelivery(ctx, accepted, landed, CurrentMain{Revision: mainDelivered}, dogfoodDeliveryVerifier{repo: repo}); err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	journal, err := fixture.Run(ctx, domain, ownerID)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	raceEvents, err := executeKickoffRaces(ctx, repo, fixture)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	restartEvents, err := executeRestartBoundaries(ctx, store, fixture.FeatureID)
	if err != nil {
		return ExecutableDogfoodJourney{}, err
	}
	journal.Events = withoutDogfoodEvents(journal.Events, "race_attempt")
	journal.Events = withoutDogfoodEvents(journal.Events, "restart_boundary")
	journal.Events = append(journal.Events, raceEvents...)
	journal.Events = append(journal.Events, restartEvents...)
	journal.RaceAttempts = len(raceEvents)
	journal.RestartBoundaries = len(restartEvents)
	journal.MainRevisions = []string{mainBefore, mainDelivered}
	journal.DeliveredTree = candidateTwo.Tree
	journal.EvidenceChain[len(journal.EvidenceChain)-2] = candidateTwo.Digest()
	journal.EvidenceChain[len(journal.EvidenceChain)-1] = mainDelivered
	journal.ShipRules = journal.deriveShipRules()
	return ExecutableDogfoodJourney{Domain: domain, OwnerID: ownerID, Repository: repo, TaskHeads: taskHeads, CandidateOne: candidateOne, CandidateTwo: candidateTwo, MainBefore: mainBefore, MainDelivered: mainDelivered, Journal: journal}, nil
}

func writeAndCommit(ctx context.Context, repo, relative, body, message string) error {
	path := filepath.Join(repo, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	if _, err := gitOutput(ctx, repo, "add", "--", relative); err != nil {
		return err
	}
	_, err := gitOutput(ctx, repo, "commit", "-m", message)
	return err
}

func (r ExecutableDogfoodReport) Validate(ctx context.Context) error {
	if r.Schema != ExecutableDogfoodSchema || r.Root == "" || len(r.Journeys) != 2 {
		return fmt.Errorf("feature drive: incomplete executable dogfood report")
	}
	for _, journey := range r.Journeys {
		if journey.Repository == "" || len(journey.TaskHeads) != 6 || !validCandidateIdentity(journey.CandidateOne) || !validCandidateIdentity(journey.CandidateTwo) || journey.CandidateOne.Digest() == journey.CandidateTwo.Digest() || journey.MainBefore == "" || journey.MainDelivered == "" || journey.MainBefore == journey.MainDelivered {
			return fmt.Errorf("feature drive: incomplete executable %s journey", journey.Domain)
		}
		if err := journey.Journal.Validate(); err != nil {
			return err
		}
		mainTree, err := gitOutput(ctx, journey.Repository, "show", "-s", "--format=%T", journey.MainDelivered)
		if err != nil || mainTree != journey.CandidateTwo.Tree {
			return fmt.Errorf("feature drive: delivered tree is not the accepted candidate")
		}
		if _, err := gitOutput(ctx, journey.Repository, "merge-base", "--is-ancestor", journey.CandidateTwo.Commit, journey.MainDelivered); err != nil {
			return fmt.Errorf("feature drive: accepted candidate is not on main: %w", err)
		}
	}
	if r.Journeys[0].Journal.DeliveredTree != r.Journeys[1].Journal.DeliveredTree {
		return fmt.Errorf("feature drive: local and remote delivered trees differ")
	}
	return nil
}

func (r ExecutableDogfoodReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func executeKickoffRaces(ctx context.Context, repository string, fixture SixTaskDriveFixture) ([]DogfoodEvent, error) {
	events := make([]DogfoodEvent, 0, 100)
	for i := 1; i <= 100; i++ {
		featureID := fmt.Sprintf("race-feature-%03d", i)
		store := NewMemoryStore()
		created := Transition{Kind: "create", NewVersion: 1}
		if err := store.Create(ctx, FeatureDrive{Schema: Schema, Repository: repository, FeatureID: featureID, PlanFingerprint: "sample-plan", TargetMainRef: "target-revision", IntegrationRef: "refs/heads/feature/" + featureID, Version: 1, Created: created, Last: created, Ownership: Ownership{State: Unowned}, Phase: Integrating}); err != nil {
			return nil, err
		}
		kickoff := Kickoff{Resolver: dogfoodResolver{feature: FrozenFeature{ID: featureID, Repository: repository, PlanFingerprint: "sample-plan", TargetMainRef: "target-revision", Eligible: true}}, Identity: dogfoodIdentity{}, Refs: dogfoodNoopRef{}, Routing: dogfoodRouting{tasks: fixture.Tasks}, Executor: dogfoodExecutor{}, Drives: store}
		assignments := []ExecutionAssignment{{Domain: "local", OwnerID: "machine-race"}, {Domain: "remote", OwnerID: "worker-race"}}
		errs := make([]error, len(assignments))
		var wg sync.WaitGroup
		for n := range assignments {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				_, errs[index] = kickoff.KickoffFeature(ctx, featureID, assignments[index])
			}(n)
		}
		wg.Wait()
		wins, conflicts := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				wins++
			case errors.Is(err, ErrOwnershipConflict):
				conflicts++
			default:
				return nil, fmt.Errorf("feature drive: race %d returned unexpected error: %w", i, err)
			}
		}
		if wins != 1 || conflicts != 1 {
			return nil, fmt.Errorf("feature drive: race %d did not fence to one owner", i)
		}
		drive, err := store.Load(ctx, featureID)
		if err != nil || drive.Ownership.State != Active || drive.Ownership.Epoch != 1 {
			return nil, fmt.Errorf("feature drive: race %d produced invalid ownership", i)
		}
		events = append(events, DogfoodEvent{Kind: "race_attempt", Revision: drive.Ownership.Domain + "/" + drive.Ownership.OwnerID, EvidenceID: fmt.Sprintf("race-%03d", i), Authorized: true})
	}
	return events, nil
}

func withoutDogfoodEvents(events []DogfoodEvent, kind string) []DogfoodEvent {
	out := make([]DogfoodEvent, 0, len(events))
	for _, event := range events {
		if event.Kind != kind {
			out = append(out, event)
		}
	}
	return out
}

func executeRestartBoundaries(ctx context.Context, store FeatureDriveStore, featureID string) ([]DogfoodEvent, error) {
	drive, err := store.Load(ctx, featureID)
	if err != nil {
		return nil, err
	}
	events := make([]DogfoodEvent, 0, 2)
	for i := 1; i <= 2; i++ {
		payload, err := json.Marshal(drive)
		if err != nil {
			return nil, err
		}
		reconstructed, err := Decode(payload)
		if err != nil || reconstructed.Version != drive.Version || reconstructed.Ownership != drive.Ownership {
			return nil, fmt.Errorf("feature drive: restart boundary %d did not reconstruct", i)
		}
		digest := sha256.Sum256(payload)
		events = append(events, DogfoodEvent{Kind: "restart_boundary", Revision: fmt.Sprintf("version-%d", drive.Version), EvidenceID: fmt.Sprintf("restart-%d-%x", i, digest[:8]), Authorized: true})
		if i == 1 {
			next := drive
			next.Frontier = []string{"T1", "T2"}
			next.Version++
			next.Last = Transition{Kind: "frontier-observed", PriorVersion: drive.Version, NewVersion: next.Version, PriorEpoch: drive.Ownership.Epoch, NewEpoch: drive.Ownership.Epoch}
			if err := store.CAS(ctx, featureID, drive.Version, next); err != nil {
				return nil, err
			}
			drive = next
		}
	}
	return events, nil
}

type dogfoodResolver struct{ feature FrozenFeature }

func (r dogfoodResolver) Resolve(context.Context, string) (FrozenFeature, error) {
	return r.feature, nil
}

type dogfoodIdentity struct{}

func (dogfoodIdentity) Authenticate(_ context.Context, assignment ExecutionAssignment) Owner {
	return Owner{Domain: assignment.Domain, ID: assignment.OwnerID}
}

type dogfoodIntegrationRef struct{ repo string }

func (r dogfoodIntegrationRef) CreateOnly(ctx context.Context, ref, base string) error {
	_, err := gitOutput(ctx, r.repo, "update-ref", ref, base, "0000000000000000000000000000000000000000")
	return err
}

type dogfoodNoopRef struct{}

func (dogfoodNoopRef) CreateOnly(context.Context, string, string) error { return nil }

type dogfoodRouting struct{ tasks []FrontierTask }

func (r dogfoodRouting) Project(_ context.Context, _ FrozenFeature, scope FeatureRoutingScope) ([]port.Task, error) {
	ready := ReadyFrontier(r.tasks)
	out := make([]port.Task, 0, len(ready))
	for i, id := range ready {
		out = append(out, port.Task{ID: id, Number: i + 1, SourceRef: "dogfood:" + id, Title: "Sample " + id, Brief: "Implement " + id, HeadBranch: scope.IntegrationRef})
	}
	return out, nil
}

type dogfoodExecutor struct{}

func (dogfoodExecutor) Execute(_ context.Context, tasks []port.Task, assignment ExecutionAssignment, scope FeatureRoutingScope) error {
	if len(tasks) != 2 || assignment.Domain != scope.Owner.Domain || assignment.OwnerID != scope.Owner.ID || scope.Epoch == 0 {
		return fmt.Errorf("feature drive: dogfood executor received invalid kickoff routing")
	}
	return nil
}

type dogfoodLandingVerifier struct{}

func (dogfoodLandingVerifier) VerifyLanding(_ context.Context, proposal ProposalObservation, review ReviewObservation, featureHead string) (LandingProof, error) {
	if review.State != "approved" || review.Head != proposal.Head {
		return LandingProof{}, ErrLandingNotProven
	}
	return LandingProof{ReviewedHead: proposal.Head, ReviewedBase: proposal.Base, LandedRevision: proposal.Landed, FeatureHead: featureHead, FeatureBase: proposal.Base}, nil
}

type dogfoodLandingRecorder struct{ landed map[string]string }

func (r *dogfoodLandingRecorder) RecordLanding(_ context.Context, taskID, proposalID string, proof LandingProof) error {
	key := taskID + ":" + proposalID
	if prior, ok := r.landed[key]; ok && prior != proof.LandedRevision {
		return ErrLandingNotProven
	}
	r.landed[key] = proof.LandedRevision
	return nil
}

type dogfoodDeliveryVerifier struct{ repo string }

func (v dogfoodDeliveryVerifier) VerifyDelivery(ctx context.Context, accepted AcceptedCandidate, landed LandedRevision, current CurrentMain) (DeliveryProof, error) {
	mainTree, err := gitOutput(ctx, v.repo, "show", "-s", "--format=%T", current.Revision)
	if err != nil {
		return DeliveryProof{}, err
	}
	if _, err := gitOutput(ctx, v.repo, "merge-base", "--is-ancestor", landed.Revision, current.Revision); err != nil {
		return DeliveryProof{}, err
	}
	return DeliveryProof{AcceptedBase: landed.Base == accepted.Candidate.MainHead, CandidateTree: mainTree == accepted.Candidate.Tree && landed.Tree == accepted.Candidate.Tree, MainAncestor: true}, nil
}
