package featuredrive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutableDogfoodUsesRealGitCandidatesAndDelivery(t *testing.T) {
	report, err := RunExecutableFeatureDriveConformance(context.Background(), filepath.Join(t.TempDir(), "dogfood"))
	if err != nil {
		t.Fatal(err)
	}
	if err := report.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, journey := range report.Journeys {
		if journey.CandidateOne.Commit == journey.CandidateTwo.Commit {
			t.Fatalf("%s repair did not change candidate", journey.Domain)
		}
		if journey.MainDelivered != journey.CandidateTwo.Commit {
			t.Fatalf("%s main did not land accepted candidate", journey.Domain)
		}
	}
}

func TestExecutableDogfoodRefusesNonEmptyOwnerRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "dogfood")
	if _, err := RunExecutableFeatureDriveConformance(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecutableFeatureDriveConformance(context.Background(), root); err == nil {
		t.Fatal("expected rerun over existing repositories to fail closed")
	}
}

func TestExecutableDogfoodBoundaryFailures(t *testing.T) {
	if _, err := RunExecutableFeatureDriveConformance(t.Context(), ""); err == nil {
		t.Fatal("dogfood accepted an empty workspace")
	}
	workspaceFile := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(workspaceFile, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecutableFeatureDriveConformance(t.Context(), workspaceFile); err == nil {
		t.Fatal("dogfood accepted a file as its workspace")
	}

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := runExecutableJourney(t.Context(), root, "local", "owner"); err == nil {
		t.Fatal("journey accepted an existing repository directory")
	}

	t.Run("git unavailable", func(t *testing.T) {
		t.Setenv("PATH", "")
		if _, err := runExecutableJourney(t.Context(), t.TempDir(), "local", "owner"); err == nil {
			t.Fatal("journey hid the git invocation failure")
		}
	})

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAndCommit(t.Context(), blocked, "child/file", "body", "message"); err == nil {
		t.Fatal("writeAndCommit accepted a file as a repository root")
	}
	directoryTarget := t.TempDir()
	if err := os.Mkdir(filepath.Join(directoryTarget, "target"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeAndCommit(t.Context(), directoryTarget, "target", "body", "message"); err == nil {
		t.Fatal("writeAndCommit overwrote a directory")
	}
	nonRepo := t.TempDir()
	if err := writeAndCommit(t.Context(), nonRepo, "file", "body", "message"); err == nil {
		t.Fatal("writeAndCommit hid git add outside a repository")
	}

	report, err := RunExecutableFeatureDriveConformance(t.Context(), filepath.Join(t.TempDir(), "dogfood"))
	if err != nil {
		t.Fatal(err)
	}
	clone := func() ExecutableDogfoodReport {
		copyReport := report
		copyReport.Journeys = append([]ExecutableDogfoodJourney(nil), report.Journeys...)
		return copyReport
	}

	bad := clone()
	bad.Schema = "wrong"
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate accepted an incomplete report identity")
	}
	bad = clone()
	bad.Journeys[0].TaskHeads = nil
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate accepted an incomplete journey")
	}
	bad = clone()
	bad.Journeys[0].Journal.TaskMerges = 0
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate hid an invalid journal")
	}
	bad = clone()
	bad.Journeys[0].Repository = filepath.Join(t.TempDir(), "missing")
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate hid the delivered-tree read failure")
	}
	bad = clone()
	bad.Journeys[0].CandidateTwo.Commit = strings.Repeat("f", 40)
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate accepted a candidate absent from main")
	}
	bad = clone()
	bad.Journeys[1].Journal.DeliveredTree += "-different"
	if err := bad.Validate(t.Context()); err == nil {
		t.Fatal("Validate accepted divergent local and remote delivered trees")
	}

	stop := errors.New("store failed")
	if _, err := executeRestartBoundaries(t.Context(), &ownershipStore{loadErr: stop}, "feature"); !errors.Is(err, stop) {
		t.Fatalf("restart load error = %v", err)
	}
	if _, err := executeRestartBoundaries(t.Context(), &ownershipStore{d: fixture(), casErr: stop}, "f-1"); !errors.Is(err, stop) {
		t.Fatalf("restart CAS error = %v", err)
	}

	if err := (dogfoodExecutor{}).Execute(t.Context(), nil, ExecutionAssignment{}, FeatureRoutingScope{}); err == nil {
		t.Fatal("dogfood executor accepted invalid routing")
	}
	if _, err := (dogfoodLandingVerifier{}).VerifyLanding(t.Context(), ProposalObservation{Head: "head"}, ReviewObservation{State: "changes-requested"}, "feature"); !errors.Is(err, ErrLandingNotProven) {
		t.Fatalf("landing verifier error = %v", err)
	}
	recorder := &dogfoodLandingRecorder{landed: map[string]string{"task:proposal": "old"}}
	if err := recorder.RecordLanding(t.Context(), "task", "proposal", LandingProof{LandedRevision: "new"}); !errors.Is(err, ErrLandingNotProven) {
		t.Fatalf("landing recorder conflict = %v", err)
	}
	if _, err := (dogfoodDeliveryVerifier{repo: t.TempDir()}).VerifyDelivery(t.Context(), AcceptedCandidate{}, LandedRevision{}, CurrentMain{Revision: "missing"}); err == nil {
		t.Fatal("delivery verifier hid the git read failure")
	}
}
