package gitstore

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

func TestAdapterAppendRepeatExactReadAndFreshClone(t *testing.T) {
	fx := newFixture(t, true)
	adapter := fx.adapter(t, fx.first)
	event := fx.demand("demand-1", "task-1")
	request := routingevidence.AppendRequest{ObservedHead: fx.genesisHead(), Event: &event}
	proof, err := adapter.Append(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Commit == fx.genesis.Commit || proof.EvidenceRef != fx.policy.EvidenceRef || len(proof.Entries) != 1 {
		t.Fatalf("proof = %+v", proof)
	}
	repeat := request
	repeat.ObservedHead.Commit = proof.Commit
	replayed, err := adapter.Append(context.Background(), repeat)
	if err != nil || replayed.Commit != proof.Commit {
		t.Fatalf("repeat = %+v, %v", replayed, err)
	}
	second := fx.demand("demand-2", "task-2")
	advanced, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: head(proof), Event: &second})
	if err != nil || advanced.Commit == proof.Commit {
		t.Fatalf("second append = %+v, %v", advanced, err)
	}
	exact, err := adapter.ReadProof(context.Background(), proof)
	if err != nil || len(exact.Events) != 1 || exact.Events[0].EventID != proof.Entries[0].EventID {
		t.Fatalf("exact read = %+v, %v", exact, err)
	}

	freshRepo := fx.newClient(t, "fresh")
	fresh := fx.adapter(t, freshRepo)
	view, err := fresh.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	if err != nil || view.Subject == nil || view.Subject.Demand == nil || view.ObservedHead.Commit != advanced.Commit {
		t.Fatalf("fresh subject = %+v, %v", view, err)
	}
	reader, err := routingevidence.NewReader(fresh, routingevidence.NewMemoryViewCache())
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := routingevidence.NewFreshCloneVerifier(reader)
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifier.Verify(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	if err != nil || report.ExactGitObject != advanced.Commit || report.TaskID != "task-1" || report.ProofEntries != 2 {
		t.Fatalf("fresh verifier = %+v, %v", report, err)
	}
}

func TestAdapterAtomicCompositeAndPartialExistingFailClosed(t *testing.T) {
	fx := newFixture(t, false)
	adapter := fx.adapter(t, fx.first)
	demand := fx.demand("demand-1", "task-1")
	demandProof, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: fx.genesisHead(), Event: &demand})
	if err != nil {
		t.Fatal(err)
	}
	resolution := fx.resolution(demand, "attempt-1")
	resolutionProof, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: head(demandProof), Event: &resolution})
	if err != nil {
		t.Fatal(err)
	}
	parent, child := fx.outcome(resolution)
	composite := routingevidence.AtomicComposite{Events: []routingevidence.Event{parent, child}}
	proof, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: head(resolutionProof), Composite: &composite})
	if err != nil || len(proof.Entries) != 2 {
		t.Fatalf("composite proof = %+v, %v", proof, err)
	}
	read, err := adapter.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectAttempt, ID: "attempt-1"})
	if err != nil || read.Subject.Attempt.State != routingevidence.StateResolvedTerminal {
		t.Fatalf("terminal subject = %+v, %v", read.Subject, err)
	}

	partialFX := newFixture(t, false)
	partial := partialFX.adapter(t, partialFX.first)
	demand = partialFX.demand("demand-1", "task-1")
	demandProof, _ = partial.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: partialFX.genesisHead(), Event: &demand})
	resolution = partialFX.resolution(demand, "attempt-1")
	resolutionProof, _ = partial.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: head(demandProof), Event: &resolution})
	parent, child = partialFX.outcome(resolution)
	wire, _ := routingevidence.EncodeCanonical(parent)
	path, _ := routingevidence.EventPath(mustID(t, parent))
	raw, _ := gitrefstore.New(partialFX.first, "origin", partialFX.policy.EvidenceRef)
	if _, err := raw.Apply(context.Background(), "events", "bypass partial", func(gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		return gitrefstore.Mutation{Files: map[string][]byte{path: wire}, Changed: true}, nil
	}); err != nil {
		t.Fatal(err)
	}
	_, err = partial.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: head(resolutionProof), Composite: &routingevidence.AtomicComposite{Events: []routingevidence.Event{parent, child}}})
	if !errors.Is(err, routingevidence.ErrReducerIntegrity) {
		t.Fatalf("partial existing chain was trusted: %v", err)
	}
}

func TestAdapterConcurrentWritersConvergeAndConflictPreservesWinner(t *testing.T) {
	fx := newFixture(t, true)
	secondRepo := fx.newClient(t, "second")
	first, second := fx.adapter(t, fx.first), fx.adapter(t, secondRepo)
	one, two := fx.demand("demand-1", "task-1"), fx.demand("demand-2", "task-2")
	requests := []struct {
		adapter *Adapter
		event   routingevidence.Event
	}{{first, one}, {second, two}}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, item := range requests {
		wg.Add(1)
		go func(item struct {
			adapter *Adapter
			event   routingevidence.Event
		}) {
			defer wg.Done()
			_, err := item.adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: fx.genesisHead(), Event: &item.event})
			errs <- err
		}(item)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"demand-1", "demand-2"} {
		if _, err := first.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: id}); err != nil {
			t.Fatalf("converged demand %s: %v", id, err)
		}
	}

	winnerRead, _ := first.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	conflict := fx.demand("demand-1", "task-1")
	conflict.Payload.ExecutionDemandFrozen.AssuranceRaise = true
	conflict = fx.sign(conflict, routingevidence.ProducerPlanGate)
	before := winnerRead.ObservedHead.Commit
	_, err := first.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: winnerRead.ObservedHead, Event: &conflict})
	var typed *routingevidence.StoreError
	if !errors.As(err, &typed) || typed.Code != routingevidence.StoreIdentityConflict {
		t.Fatalf("conflict error = %v", err)
	}
	after, _ := first.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	if after.ObservedHead.Commit != before {
		t.Fatalf("conflict advanced ref: %s -> %s", before, after.ObservedHead.Commit)
	}
}

func TestAdapterUnavailableAuthorizationAndGenesisDefectsFailClosed(t *testing.T) {
	fx := newFixture(t, false)
	event := fx.demand("demand-1", "task-1")
	event.ProducerProof.Signature = "00"
	_, err := fx.adapter(t, fx.first).Append(context.Background(), routingevidence.AppendRequest{ObservedHead: fx.genesisHead(), Event: &event})
	if !errors.Is(err, routingevidence.ErrProducerIntegrity) {
		t.Fatalf("authorization defect = %v", err)
	}

	missingPolicy := fx.policy
	missingPolicy.EvidenceRef = "refs/pyxis/missing-evidence"
	missingPolicy.Genesis.EvidenceRef = missingPolicy.EvidenceRef
	setPolicyDigest(t, &missingPolicy)
	missing, err := New(fx.first, "origin", missingPolicy)
	if err != nil {
		t.Fatal(err)
	}
	missingEvent := event
	missingEvent.ProducerProof = routingevidence.ProducerProof{}
	missingEvent.EvidenceRef = missingPolicy.EvidenceRef
	missingEvent = fx.signWithPolicy(missingEvent, routingevidence.ProducerPlanGate, missingPolicy)
	_, err = missing.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: routingevidence.ObservedHead{EvidenceRef: missingPolicy.EvidenceRef, Commit: fx.genesis.Commit}, Event: &missingEvent})
	var storeErr *routingevidence.StoreError
	if !errors.As(err, &storeErr) || storeErr.Code != routingevidence.StoreRefUnavailable {
		t.Fatalf("missing ref = %v", err)
	}

	rogue := newFixture(t, false)
	wrongPolicy := rogue.policy
	wrongPolicy.Genesis.Commit = strings.Repeat("a", 40)
	setPolicyDigest(t, &wrongPolicy)
	wrong, _ := New(rogue.first, "origin", wrongPolicy)
	wrongEvent := rogue.demand("demand-rogue", "task-rogue")
	wrongEvent = rogue.signWithPolicy(wrongEvent, routingevidence.ProducerPlanGate, wrongPolicy)
	_, err = wrong.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: rogue.genesisHead(), Event: &wrongEvent})
	if !errors.Is(err, routingevidence.ErrProducerIntegrity) {
		t.Fatalf("unreviewed genesis = %v", err)
	}
}

func TestProtectedRemoteDeniesForceDeleteAndSecondInitialization(t *testing.T) {
	fx := newFixture(t, true)
	adapter := fx.adapter(t, fx.first)
	event := fx.demand("demand-1", "task-1")
	proof, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: fx.genesisHead(), Event: &event})
	if err != nil {
		t.Fatal(err)
	}
	if err := runGitErr(fx.first, "push", "--force", "origin", fx.genesis.Commit+":"+fx.policy.EvidenceRef); err == nil {
		t.Fatal("protected ref accepted force update")
	}
	if err := runGitErr(fx.first, "push", "origin", ":"+fx.policy.EvidenceRef); err == nil {
		t.Fatal("protected ref accepted deletion")
	}
	if read, err := adapter.ReadProof(context.Background(), proof); err != nil || read.Proof.Commit != proof.Commit {
		t.Fatalf("denied destructive pushes damaged authority: %+v, %v", read, err)
	}

	root := t.TempDir()
	remote := filepath.Join(root, "guarded.git")
	runGit(t, root, "init", "--bare", "-q", remote)
	installProtectionHook(t, remote, fx.policy.EvidenceRef)
	repo := filepath.Join(root, "client")
	runGit(t, root, "init", "-q", repo)
	runGit(t, repo, "remote", "add", "origin", remote)
	raw, _ := gitrefstore.New(repo, "origin", fx.policy.EvidenceRef)
	if _, err := raw.EstablishGenesis(context.Background(), "ordinary-writer"); err == nil {
		t.Fatal("protected ref accepted unreviewed initialization")
	}
}

func TestObservedForceOrDeletePoisonsAuthority(t *testing.T) {
	forced := newFixture(t, false)
	adapter := forced.adapter(t, forced.first)
	event := forced.demand("demand-1", "task-1")
	if _, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: forced.genesisHead(), Event: &event}); err != nil {
		t.Fatal(err)
	}
	runGit(t, forced.first, "push", "--force", "origin", forced.genesis.Commit+":"+forced.policy.EvidenceRef)
	_, err := adapter.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	if !errors.Is(err, routingevidence.ErrProducerIntegrity) {
		t.Fatalf("force update remained authoritative: %v", err)
	}

	deleted := newFixture(t, false)
	adapter = deleted.adapter(t, deleted.first)
	event = deleted.demand("demand-1", "task-1")
	if _, err := adapter.Append(context.Background(), routingevidence.AppendRequest{ObservedHead: deleted.genesisHead(), Event: &event}); err != nil {
		t.Fatal(err)
	}
	runGit(t, deleted.first, "push", "origin", ":"+deleted.policy.EvidenceRef)
	_, err = adapter.ReadSubject(context.Background(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand-1"})
	var storeErr *routingevidence.StoreError
	if !errors.As(err, &storeErr) || storeErr.Code != routingevidence.StoreRefUnavailable {
		t.Fatalf("deleted ref remained authoritative: %v", err)
	}
}

type fixture struct {
	t       *testing.T
	remote  string
	first   string
	genesis gitrefstore.Genesis
	policy  routingevidence.ProducerPolicy
	keys    map[routingevidence.ProducerClass]ed25519.PrivateKey
	profile routingevidence.Profile
}

func newFixture(t *testing.T, protect bool) *fixture {
	t.Helper()
	root := t.TempDir()
	remote, first := filepath.Join(root, "remote.git"), filepath.Join(root, "first")
	runGit(t, root, "init", "--bare", "-q", remote)
	runGit(t, root, "init", "-q", first)
	runGit(t, first, "remote", "add", "origin", remote)
	raw, err := gitrefstore.New(first, "origin", "refs/pyxis/routing-evidence")
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := raw.EstablishGenesis(context.Background(), "operator:init-1")
	if err != nil {
		t.Fatal(err)
	}
	if protect {
		installProtectionHook(t, remote, "refs/pyxis/routing-evidence")
	}
	fx := &fixture{t: t, remote: remote, first: first, genesis: genesis, keys: map[routingevidence.ProducerClass]ed25519.PrivateKey{}, profile: routingevidence.Profile{Capability: "strong", Deliberation: "deep", Context: "large", Assurance: "elevated"}}
	operatorSeed := sha256.Sum256([]byte("git-store-activation-operator"))
	operatorKey := ed25519.NewKeyFromSeed(operatorSeed[:])
	fx.policy = routingevidence.ProducerPolicy{
		ActivationPoint: "policy-point/1", Authorizations: []routingevidence.AuthorizationReference{
			{ID: "auth-policy", Kind: routingevidence.AuthorizationPolicy},
			{ID: "auth-fence", Kind: routingevidence.AuthorizationRuntimeFence, Reference: "fence-proof"},
			{ID: "auth-native", Kind: routingevidence.AuthorizationNativeAuthor, Reference: "native-proof", ActorClass: "worker", ActorIdentity: "github:worker"},
			{ID: "auth-invalidation", Kind: routingevidence.AuthorizationInvalidation, Reference: "operator-decision"},
			{ID: "auth-activation", Kind: routingevidence.AuthorizationActivationManifest, ActorClass: "operator", ActorIdentity: "operator-key-1", PublicKey: hex.EncodeToString(operatorKey.Public().(ed25519.PublicKey))},
		},
		EvidenceRef: "refs/pyxis/routing-evidence", Repository: "agenticlab-ai/pyxis", Schema: routingevidence.ProducerPolicySchema, Version: "policy/v1",
		Genesis: routingevidence.GenesisCheckpoint{AuthorizationReference: genesis.Authorization, Commit: genesis.Commit, EvidenceRef: "refs/pyxis/routing-evidence", Repository: "agenticlab-ai/pyxis", Tree: genesis.Tree},
	}
	classes := []routingevidence.ProducerClass{
		routingevidence.ProducerPlanGate, routingevidence.ProducerResolver,
		routingevidence.ProducerAttemptOwner, routingevidence.ProducerRecoveryCoordinator, routingevidence.ProducerReviewOrchestrator,
		routingevidence.ProducerMediumAdapter, routingevidence.ProducerEvidenceOwner, routingevidence.ProducerReleaseGate,
	}
	for _, class := range classes {
		seed := sha256.Sum256([]byte("git-store-fixture:" + class))
		private := ed25519.NewKeyFromSeed(seed[:])
		fx.keys[class] = private
		fx.policy.Keys = append(fx.policy.Keys, routingevidence.KeyGeneration{Algorithm: "ed25519", ImplementationGeneration: generation(class), KeyID: keyID(class), PolicyActivationPoints: []string{fx.policy.ActivationPoint}, ProducerClass: class, PublicKey: hex.EncodeToString(private.Public().(ed25519.PublicKey))})
	}
	fx.policy.Matrix = []routingevidence.ProducerAuthorization{
		{AuthorizationID: "auth-policy", EventKind: routingevidence.ExecutionDemandFrozen, ImplementationGeneration: generation(routingevidence.ProducerPlanGate), KeyID: keyID(routingevidence.ProducerPlanGate), ProducerClass: routingevidence.ProducerPlanGate},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.RoutingResolved, ImplementationGeneration: generation(routingevidence.ProducerResolver), KeyID: keyID(routingevidence.ProducerResolver), ProducerClass: routingevidence.ProducerResolver, Role: "implementation"},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.RoutingResolved, ImplementationGeneration: generation(routingevidence.ProducerResolver), KeyID: keyID(routingevidence.ProducerResolver), ProducerClass: routingevidence.ProducerResolver, Role: "review"},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.RoutingResolutionFailed, ImplementationGeneration: generation(routingevidence.ProducerResolver), KeyID: keyID(routingevidence.ProducerResolver), ProducerClass: routingevidence.ProducerResolver, Role: "implementation"},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.RoutedAttemptCompleted, ImplementationGeneration: generation(routingevidence.ProducerAttemptOwner), KeyID: keyID(routingevidence.ProducerAttemptOwner), ProducerClass: routingevidence.ProducerAttemptOwner},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.RoutedTaskOutcome, ImplementationGeneration: generation(routingevidence.ProducerAttemptOwner), KeyID: keyID(routingevidence.ProducerAttemptOwner), ProducerClass: routingevidence.ProducerAttemptOwner},
		{AuthorizationID: "auth-fence", EventKind: routingevidence.AttemptOwnerTransferred, ImplementationGeneration: generation(routingevidence.ProducerRecoveryCoordinator), KeyID: keyID(routingevidence.ProducerRecoveryCoordinator), ProducerClass: routingevidence.ProducerRecoveryCoordinator},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.AssurancePassCompleted, ImplementationGeneration: generation(routingevidence.ProducerReviewOrchestrator), KeyID: keyID(routingevidence.ProducerReviewOrchestrator), ProducerClass: routingevidence.ProducerReviewOrchestrator},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.FindingPersisted, ImplementationGeneration: generation(routingevidence.ProducerReviewOrchestrator), KeyID: keyID(routingevidence.ProducerReviewOrchestrator), ProducerClass: routingevidence.ProducerReviewOrchestrator},
		{AuthorizationID: "auth-native", EventKind: routingevidence.DispositionRecorded, ImplementationGeneration: generation(routingevidence.ProducerMediumAdapter), KeyID: keyID(routingevidence.ProducerMediumAdapter), ProducerClass: routingevidence.ProducerMediumAdapter, Role: "worker"},
		{AuthorizationID: "auth-policy", EventKind: routingevidence.ReviewProcessCompleted, ImplementationGeneration: generation(routingevidence.ProducerReviewOrchestrator), KeyID: keyID(routingevidence.ProducerReviewOrchestrator), ProducerClass: routingevidence.ProducerReviewOrchestrator},
		{AuthorizationID: "auth-invalidation", EventKind: routingevidence.RoutingEvidenceInvalidated, ImplementationGeneration: generation(routingevidence.ProducerEvidenceOwner), KeyID: keyID(routingevidence.ProducerEvidenceOwner), ProducerClass: routingevidence.ProducerEvidenceOwner, Role: "rerun_required"},
		{AuthorizationID: "auth-activation", EventKind: routingevidence.ActivationCheckpointEvent, ImplementationGeneration: generation(routingevidence.ProducerReleaseGate), KeyID: keyID(routingevidence.ProducerReleaseGate), ProducerClass: routingevidence.ProducerReleaseGate},
	}
	setPolicyDigest(t, &fx.policy)
	if err := routingevidence.ValidateProducerPolicy(fx.policy); err != nil {
		t.Fatal(err)
	}
	return fx
}

func (f *fixture) adapter(t *testing.T, repo string) *Adapter {
	t.Helper()
	adapter, err := New(repo, "origin", f.policy)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func (f *fixture) newClient(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	runGit(t, filepath.Dir(repo), "init", "-q", repo)
	runGit(t, repo, "remote", "add", "origin", f.remote)
	return repo
}

func (f *fixture) genesisHead() routingevidence.ObservedHead {
	return routingevidence.ObservedHead{EvidenceRef: f.policy.EvidenceRef, Commit: f.genesis.Commit}
}

func (f *fixture) demand(demandID, taskID string) routingevidence.Event {
	event := routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.ExecutionDemandFrozen, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{ExecutionDemandFrozen: &routingevidence.ExecutionDemandFrozenPayload{
			Assessment:             routingevidence.Assessment{ArchitecturalRole: "shared_substrate", Coupling: "composed", ImpactScope: "multi_surface", Novelty: "adapt", Reasoning: "complex", Verification: "composed"},
			AssessmentEvidenceRefs: []string{"plan:003/T4"}, DemandID: demandID, PlanVersion: "plan/v2", PolicyVersion: "routing-policy/v1", Profile: f.profile, TaskID: taskID,
		}},
	}
	return f.sign(event, routingevidence.ProducerPlanGate)
}

func (f *fixture) resolution(demand routingevidence.Event, attemptID string) routingevidence.Event {
	p := demand.Payload.ExecutionDemandFrozen
	event := routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.RoutingResolved, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{RoutingResolved: &routingevidence.RoutingResolvedPayload{AttemptID: attemptID, DemandID: p.DemandID, Harness: "codex", NativeProfileID: "codex/strong", OwnerGeneration: 1, PolicyVersion: p.PolicyVersion, PromotedComponents: []string{}, RequestedProfile: p.Profile, Resolution: "exact", Role: "implementation", TaskID: p.TaskID}},
	}
	return f.sign(event, routingevidence.ProducerResolver)
}

func (f *fixture) outcome(resolution routingevidence.Event) (routingevidence.Event, routingevidence.Event) {
	p := resolution.Payload.RoutingResolved
	parent := routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.RoutedAttemptCompleted, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{RoutedAttemptCompleted: &routingevidence.RoutedAttemptCompletedPayload{AttemptID: p.AttemptID, Cause: "terminal_verified", DemandID: p.DemandID, EvidenceRef: string(mustID(f.t, resolution)), LaunchCapabilityRef: "launch-capability:" + p.AttemptID, LaunchFidelity: "exact", Outcome: "verified", OwnerGeneration: 1, TaskID: p.TaskID, UsageProvenance: "runtime_observed"}},
	}
	parent = f.sign(parent, routingevidence.ProducerAttemptOwner)
	child := routingevidence.Event{
		EvidenceRef: f.policy.EvidenceRef, Kind: routingevidence.RoutedTaskOutcome, Repository: f.policy.Repository, Schema: routingevidence.Schema, Version: routingevidence.EventVersion,
		Payload: routingevidence.Payload{RoutedTaskOutcome: &routingevidence.RoutedTaskOutcomePayload{AttemptID: p.AttemptID, Cause: "terminal_verified", Class: "succeeded", DemandID: p.DemandID, EvidenceRef: string(mustID(f.t, parent)), LaunchCapabilityRef: "launch-capability:" + p.AttemptID, LaunchFidelity: "exact", OwnerGeneration: 1, TaskID: p.TaskID, UsageProvenance: "runtime_observed", Verification: "verified"}},
	}
	return parent, f.sign(child, routingevidence.ProducerAttemptOwner)
}

func (f *fixture) sign(event routingevidence.Event, class routingevidence.ProducerClass) routingevidence.Event {
	return f.signWithPolicy(event, class, f.policy)
}

func (f *fixture) signWithPolicy(event routingevidence.Event, class routingevidence.ProducerClass, policy routingevidence.ProducerPolicy) routingevidence.Event {
	event.EventID, event.ContentDigest, event.ProducerProof = "", "", routingevidence.ProducerProof{}
	event.Producer = routingevidence.Producer{Class: string(class), Generation: generation(class)}
	unsigned, err := routingevidence.NewUnsignedEnvelope(event)
	if err != nil {
		f.t.Fatal(err)
	}
	roles := []string(nil)
	switch event.Kind {
	case routingevidence.RoutingResolved:
		roles = []string{event.Payload.RoutingResolved.Role}
	case routingevidence.RoutingResolutionFailed:
		roles = []string{event.Payload.RoutingResolutionFailed.Role}
	case routingevidence.DispositionRecorded:
		roles = []string{event.Payload.DispositionRecorded.Actor}
	case routingevidence.RoutingEvidenceInvalidated:
		roles = []string{event.Payload.RoutingEvidenceInvalidated.Disposition}
	}
	proof, err := routingevidence.Sign(unsigned, routingevidence.SigningCapability{Repository: policy.Repository, EvidenceRef: policy.EvidenceRef, ProducerClass: class, ImplementationGeneration: generation(class), KeyID: keyID(class), PrivateKey: f.keys[class], EventKinds: []routingevidence.EventKind{event.Kind}, Roles: roles})
	if err != nil {
		f.t.Fatal(err)
	}
	event.ProducerProof = proof
	return event
}

func generation(class routingevidence.ProducerClass) string { return string(class) + "/v1" }
func keyID(class routingevidence.ProducerClass) string      { return "key-" + string(class) }

func setPolicyDigest(t *testing.T, policy *routingevidence.ProducerPolicy) {
	t.Helper()
	policy.Digest = ""
	digest, err := routingevidence.DigestProducerPolicy(*policy)
	if err != nil {
		t.Fatal(err)
	}
	policy.Digest = digest
}

func mustID(t *testing.T, event routingevidence.Event) routingevidence.EventID {
	t.Helper()
	id, err := routingevidence.Identify(event)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func head(proof routingevidence.EvidenceProof) routingevidence.ObservedHead {
	return routingevidence.ObservedHead{EvidenceRef: proof.EvidenceRef, Commit: proof.Commit}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func runGitErr(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@invalid")
	return cmd.Run()
}

func installProtectionHook(t *testing.T, remote, ref string) {
	t.Helper()
	hook := `#!/bin/sh
ref="$1"
old="$2"
new="$3"
zero=0000000000000000000000000000000000000000
test "$ref" = "` + ref + `" || exit 0
test "$old" != "$zero" || exit 1
test "$new" != "$zero" || exit 1
git merge-base --is-ancestor "$old" "$new"
`
	path := filepath.Join(remote, "hooks", "update")
	if err := os.WriteFile(path, []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
}
