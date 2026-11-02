package featuredrive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const coverage90HookMode = "PYXIS_FEATUREDRIVE_HOOK_MODE"

func TestMain(m *testing.M) {
	switch os.Getenv(coverage90HookMode) {
	case "fail":
		os.Exit(1)
	case "fail-commit":
		state := os.Getenv("PYXIS_FEATUREDRIVE_HOOK_STATE")
		target, _ := strconv.Atoi(os.Getenv("PYXIS_FEATUREDRIVE_HOOK_TARGET"))
		count := 0
		if wire, err := os.ReadFile(state); err == nil {
			count, _ = strconv.Atoi(strings.TrimSpace(string(wire)))
		}
		count++
		_ = os.WriteFile(state, []byte(strconv.Itoa(count)), 0o600)
		if count == target {
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func coverage90InstallGitHook(t *testing.T, name, mode string, target int) {
	t.Helper()
	template := t.TempDir()
	hooks := filepath.Join(template, "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	hook := []byte("#!/bin/sh\nexec \"$PYXIS_FEATUREDRIVE_HOOK_HELPER\"\n")
	if err := os.WriteFile(filepath.Join(hooks, name), hook, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_TEMPLATE_DIR", template)
	t.Setenv("PYXIS_FEATUREDRIVE_HOOK_HELPER", executable)
	t.Setenv(coverage90HookMode, mode)
	if target > 0 {
		t.Setenv("PYXIS_FEATUREDRIVE_HOOK_STATE", filepath.Join(t.TempDir(), "count"))
		t.Setenv("PYXIS_FEATUREDRIVE_HOOK_TARGET", strconv.Itoa(target))
	}
}

func TestCoverage90ExecutableDogfoodFailureBoundaries(t *testing.T) {
	t.Run("workspace resolution", func(t *testing.T) {
		old := dogfoodAbs
		dogfoodAbs = func(string) (string, error) { return "", errors.New("resolve failed") }
		t.Cleanup(func() { dogfoodAbs = old })
		if _, err := RunExecutableFeatureDriveConformance(t.Context(), "workspace"); err == nil {
			t.Fatal("workspace resolution failure was ignored")
		}
	})

	t.Run("invalid owner domain", func(t *testing.T) {
		if _, err := runExecutableJourney(t.Context(), t.TempDir(), "invalid", "owner"); err == nil {
			t.Fatal("invalid owner domain completed a journey")
		}
	})

	for _, tc := range []struct {
		name   string
		target int
	}{
		{name: "initial commit", target: 1},
		{name: "task commit", target: 2},
		{name: "repair commit", target: 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coverage90InstallGitHook(t, "pre-commit", "fail-commit", tc.target)
			if _, err := runExecutableJourney(t.Context(), t.TempDir(), "local", "owner"); err == nil {
				t.Fatal("journey ignored a commit-hook failure")
			}
		})
	}

	t.Run("checkout hook", func(t *testing.T) {
		coverage90InstallGitHook(t, "post-checkout", "fail", 0)
		if _, err := runExecutableJourney(t.Context(), t.TempDir(), "local", "owner"); err == nil {
			t.Fatal("journey ignored a checkout-hook failure")
		}
	})
}

func TestCoverage90DogfoodHelpersFailClosed(t *testing.T) {
	if got := NewSixTaskDriveFixture(""); got.FeatureID == "" {
		t.Fatal("empty dogfood fixture did not receive its stable ID")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewSixTaskDriveFixture("feature").Run(cancelled, "local", "owner"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled fixture run = %v", err)
	}
	if _, err := NewSixTaskDriveFixture("feature").Run(t.Context(), "foreign", "owner"); err == nil {
		t.Fatal("foreign dogfood owner was accepted")
	}

	if _, err := executeKickoffRaces(t.Context(), "repo", SixTaskDriveFixture{FeatureID: "bad"}); err == nil {
		t.Fatal("race harness accepted an incomplete fixture")
	}
	want := errors.New("load failed")
	if _, err := executeRestartBoundaries(t.Context(), &ownershipStore{loadErr: want}, "feature"); !errors.Is(err, want) {
		t.Fatalf("restart load error = %v", err)
	}
	if _, err := executeRestartBoundaries(t.Context(), &ownershipStore{d: FeatureDrive{}}, "feature"); err == nil {
		t.Fatal("restart reconstructed an invalid drive")
	}

	repo := t.TempDir()
	if _, err := gitOutput(t.Context(), repo, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.name", "Coverage Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.email", "coverage@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := writeAndCommit(t.Context(), repo, "file", "body", "initial"); err != nil {
		t.Fatal(err)
	}
	head, err := gitOutput(t.Context(), repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	verifier := dogfoodDeliveryVerifier{repo: repo}
	if _, err := verifier.VerifyDelivery(t.Context(), AcceptedCandidate{}, LandedRevision{Revision: "missing"}, CurrentMain{Revision: head}); err == nil {
		t.Fatal("non-ancestor landed revision was accepted")
	}
}

func TestCoverage90CandidateAndMemoryStoreBoundaries(t *testing.T) {
	repo := t.TempDir()
	if _, err := gitOutput(t.Context(), repo, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.name", "Coverage Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.email", "coverage@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := writeAndCommit(t.Context(), repo, "one", "one", "one"); err != nil {
		t.Fatal(err)
	}
	first, _ := gitOutput(t.Context(), repo, "rev-parse", "HEAD")
	if err := writeAndCommit(t.Context(), repo, "two", "two", "two"); err != nil {
		t.Fatal(err)
	}
	second, _ := gitOutput(t.Context(), repo, "rev-parse", "HEAD")
	publisher := NewGitCandidatePublisher(repo)
	ref := "refs/pyxis/candidates/coverage"
	if err := publisher.CreateOnly(t.Context(), ref, CandidateIdentity{Commit: first}); err != nil {
		t.Fatal(err)
	}
	if err := publisher.CreateOnly(t.Context(), ref, CandidateIdentity{Commit: first}); err != nil {
		t.Fatalf("idempotent publication = %v", err)
	}
	if err := publisher.CreateOnly(t.Context(), ref, CandidateIdentity{Commit: second}); !errors.Is(err, ErrCandidateExists) {
		t.Fatalf("conflicting publication = %v", err)
	}

	store := NewMemoryStore()
	invalid := fixture()
	invalid.FeatureID = ""
	if err := store.Create(t.Context(), invalid); err == nil {
		t.Fatal("invalid drive was created")
	}
	drive := fixture()
	if err := store.Create(t.Context(), drive); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(t.Context(), drive); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate create = %v", err)
	}
	if err := store.CAS(t.Context(), "missing", 1, drive); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing CAS = %v", err)
	}
	if err := store.CAS(t.Context(), drive.FeatureID, drive.Version+1, drive); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale CAS = %v", err)
	}
	invalid = drive
	invalid.Schema = ""
	if err := store.CAS(t.Context(), drive.FeatureID, drive.Version, invalid); err == nil {
		t.Fatal("invalid CAS replacement was accepted")
	}
	planErr := errors.New("plan failed")
	reconciler := Reconciler{Store: store, Executor: &countingEffect{}, Plan: func(context.Context, FeatureDrive, string) ([]EffectIntent, error) {
		return nil, planErr
	}}
	if err := reconciler.Reconcile(t.Context(), drive.FeatureID, "coverage"); !errors.Is(err, planErr) {
		t.Fatalf("planner error = %v", err)
	}
}

func TestCoverage90EvidenceDeliveryKickoffAndSurfaceBoundaries(t *testing.T) {
	plan := evidencePlan(evidenceCandidate("feature", "candidate"))
	wrongSpecialist := &fakeScenarioRunner{id: "wrong"}
	if _, err := ExecuteScenario(t.Context(), plan, "special", nil, map[string]ScenarioRunner{"gpu": wrongSpecialist}); err == nil {
		t.Fatal("wrong specialized runner was accepted")
	}
	runErr := errors.New("runner failed")
	owner := &fakeScenarioRunner{id: "owner", err: runErr}
	if _, err := ExecuteScenario(t.Context(), plan, "owner", owner, nil); !errors.Is(err, runErr) {
		t.Fatalf("runner error = %v", err)
	}
	invalidPlan := plan
	invalidPlan.Schema = ""
	if _, err := ResultFromManualAttestation(invalidPlan, ManualAttestation{}); err == nil {
		t.Fatal("manual attestation accepted an invalid plan")
	}
	if _, err := NewCandidateEvidence(invalidPlan, nil); err == nil {
		t.Fatal("candidate evidence accepted an invalid plan")
	}

	accepted, _, _ := deliveryFixture()
	evidence := EvidenceVerdict{FeatureID: accepted.Proposal.FeatureID, CandidateID: accepted.Candidate.Digest(), ScenarioSet: accepted.Proposal.ScenarioSetFingerprint, Accepted: true}
	validCapability := FinalActivationCapability{RequiresCandidateVerdict: true, EnforcesCurrentBase: true}
	if _, _, err := ActivateFinalProposal(t.Context(), accepted.Proposal, evidence, FinalActivationCapability{}, nil, nil); err == nil {
		t.Fatal("insufficient final activation capability was accepted")
	}
	if _, _, err := ActivateFinalProposal(t.Context(), accepted.Proposal, evidence, validCapability, nil, nil); err == nil {
		t.Fatal("missing final publishers were accepted")
	}
	publishErr := errors.New("publish failed")
	if _, _, err := ActivateFinalProposal(t.Context(), accepted.Proposal, evidence, validCapability, finalProposalPublisherFunc(func(context.Context, FinalProposal) (FinalProposal, error) {
		return FinalProposal{}, publishErr
	}), verdictPublisherFunc(func(context.Context, CandidateVerdict) error { return nil })); !errors.Is(err, publishErr) {
		t.Fatalf("proposal publisher error = %v", err)
	}

	drive, frozen := kickoffFixture()
	drive.Ownership.Epoch = ^uint64(0)
	store := NewMemoryStore()
	if err := store.Create(t.Context(), drive); err != nil {
		t.Fatal(err)
	}
	refsErr := errors.New("ref failed")
	k := Kickoff{Resolver: kickoffResolver{feature: frozen}, Identity: kickoffIdentity{}, Refs: kickoffRefFunc(func(context.Context, string, string) error { return refsErr }), Routing: &kickoffRouting{}, Executor: &kickoffExecutor{}, Drives: store}
	if _, err := k.KickoffFeature(t.Context(), frozen.ID, ExecutionAssignment{Domain: "local", OwnerID: "machine"}); !errors.Is(err, refsErr) {
		t.Fatalf("integration ref error = %v", err)
	}

	loadErr := errors.New("load failed")
	lifecycle := OwnershipLifecycle{Store: &ownershipStore{loadErr: loadErr}}
	if _, err := lifecycle.change(t.Context(), "feature", 1, func(*FeatureDrive) error { return nil }); !errors.Is(err, loadErr) {
		t.Fatalf("ownership change load error = %v", err)
	}

	if err := (TouchRouter{}).Route(t.Context(), DriveTouch{}); err == nil {
		t.Fatal("invalid touch was accepted")
	}
	touch := PushTouch("feature", OwnerEpoch{Owner: Owner{Domain: "local", ID: "owner"}, Epoch: 1}, time.Time{})
	if err := (TouchRouter{}).Route(t.Context(), touch); err == nil {
		t.Fatal("incomplete touch router was accepted")
	}
	if err := (TouchRouter{Store: &ownershipStore{loadErr: loadErr}, Reconcile: func(context.Context, string, string) error { return nil }}).Route(t.Context(), touch); !errors.Is(err, loadErr) {
		t.Fatalf("touch load error = %v", err)
	}
	if _, err := (OwnerBackstop{Store: nonEnumerableStore{}, Domain: "local", OwnerID: "owner"}).Scan(t.Context()); err == nil {
		t.Fatal("non-enumerable owner store was accepted")
	}
}

type nonEnumerableStore struct{}

func (nonEnumerableStore) Load(context.Context, string) (FeatureDrive, error) {
	return FeatureDrive{}, nil
}
func (nonEnumerableStore) Create(context.Context, FeatureDrive) error { return nil }
func (nonEnumerableStore) CAS(context.Context, string, uint64, FeatureDrive) error {
	return nil
}

type kickoffRefFunc func(context.Context, string, string) error

func (f kickoffRefFunc) CreateOnly(ctx context.Context, ref, base string) error {
	return f(ctx, ref, base)
}
