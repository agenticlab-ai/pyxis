package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/credentialstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence/gitstore"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

func TestCoverageLoadActiveRoutingFailuresAndSuccess(t *testing.T) {
	checkpointA := strings.Repeat("a", 64)
	checkpointB := strings.Repeat("b", 64)
	wire := coverageActivationConfigWire(checkpointA)
	cfg, err := config.Parse(wire)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("synthetic dependency failure")

	cases := []string{
		"mainline", "mainline-config", "worktree-mismatch", "policy", "cross-boundary",
		"store", "checkpoint", "resolver", "owner", "admission", "success",
	}
	for _, failAt := range cases {
		t.Run(failAt, func(t *testing.T) {
			original := activeRoutingDeps
			t.Cleanup(func() { activeRoutingDeps = original })

			mainlineWire := wire
			if failAt == "mainline-config" {
				mainlineWire = []byte("executor:\n  unknown: yes\n")
			}
			if failAt == "worktree-mismatch" {
				mainlineWire = coverageActivationConfigWire(checkpointB)
			}
			capabilityCalls := 0
			activeRoutingDeps = activeRoutingDependencies{
				readMainline: func(context.Context, string, string, string, string) ([]byte, []byte, error) {
					if failAt == "mainline" {
						return nil, nil, wantErr
					}
					return mainlineWire, []byte("policy\n"), nil
				},
				decodePolicy: func([]byte) (routingevidence.ProducerPolicy, error) {
					if failAt == "policy" {
						return routingevidence.ProducerPolicy{}, wantErr
					}
					repository := "owner/repo"
					if failAt == "cross-boundary" {
						repository = "other/repo"
					}
					return routingevidence.ProducerPolicy{Repository: repository, EvidenceRef: "refs/pyxis/routing-evidence"}, nil
				},
				newStore: func(string, string, routingevidence.ProducerPolicy) (*gitstore.Adapter, error) {
					if failAt == "store" {
						return nil, wantErr
					}
					return nil, nil
				},
				verify: func(context.Context, activationStateReader, routingevidence.ProducerPolicy, string) error {
					if failAt == "checkpoint" {
						return wantErr
					}
					return nil
				},
				newSecrets: func() credentialstore.Store { return activationMemoryCredentials{} },
				loadCapability: func(_ context.Context, _ credentialstore.Store, _ routingevidence.ProducerPolicy, class routingevidence.ProducerClass, _ []routingevidence.EventKind, _ []string) (routingevidence.SigningCapability, error) {
					capabilityCalls++
					if (failAt == "resolver" && class == routingevidence.ProducerResolver) ||
						(failAt == "owner" && class == routingevidence.ProducerAttemptOwner) {
						return routingevidence.SigningCapability{}, wantErr
					}
					return routingevidence.SigningCapability{PrivateKey: []byte("secret")}, nil
				},
				newAdmission: func(routingevidence.Store, routingevidence.ResolutionTerminalWriter) (*routingevidence.AdmissionVerifier, error) {
					if failAt == "admission" {
						return nil, wantErr
					}
					return &routingevidence.AdmissionVerifier{}, nil
				},
				randRead: func(p []byte) (int, error) {
					for i := range p {
						p[i] = byte(i)
					}
					return len(p), nil
				},
			}

			active, loadErr := loadActiveRouting(t.Context(), cfg, t.TempDir(), "owner/repo", "upstream", "trunk")
			if failAt != "success" {
				if loadErr == nil {
					t.Fatalf("failure at %s was accepted", failAt)
				}
				return
			}
			if loadErr != nil || active == nil || active.admission == nil || capabilityCalls != 2 {
				t.Fatalf("success = active:%+v calls:%d err:%v", active, capabilityCalls, loadErr)
			}
			active.Close()
		})
	}
}

type coverageActivationStateReader struct{ err error }

func (r coverageActivationStateReader) ReadHead(context.Context) (routingevidence.ExactRead, error) {
	return routingevidence.ExactRead{}, r.err
}

func TestCoverageVerifyActivationCheckpointReadFailure(t *testing.T) {
	want := errors.New("evidence unavailable")
	err := verifyActivationCheckpoint(t.Context(), coverageActivationStateReader{err: want}, routingevidence.ProducerPolicy{}, "checkpoint")
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "exact-read evidence head") {
		t.Fatalf("checkpoint read error = %v", err)
	}
}

func TestCoverageInactiveRoutingAndTrackerBoundaries(t *testing.T) {
	active, err := loadActiveRouting(t.Context(), config.Defaults(), t.TempDir(), "owner/repo", "origin", "main")
	if err != nil || active != nil {
		t.Fatalf("inactive routing = %+v, %v", active, err)
	}
	var nilActive *activeRouting
	nilActive.Close()
	if nilActive.implementationFactory() != nil || nilActive.implementationOutcomeFactory() != nil {
		t.Fatal("nil routing installed implementation hooks")
	}
	if admission, err := nilActive.reviewAdmission("codex", coverageAgentRuntime{}); err != nil || admission != nil {
		t.Fatalf("nil review admission = %v, %v", admission, err)
	}
	if outcome, err := nilActive.reviewOutcome(coverageAgentRuntime{}); err != nil || outcome != nil {
		t.Fatalf("nil review outcome = %v, %v", outcome, err)
	}

	one, two := []byte("secret-one"), []byte("secret-two")
	active = &activeRouting{privateKey: [][]byte{one, two}}
	active.Close()
	if strings.Trim(string(one), "\x00") != "" || strings.Trim(string(two), "\x00") != "" {
		t.Fatalf("Close did not clear secrets: %v %v", one, two)
	}

}

func TestCoverageCatalogAuthorizationAndSelection(t *testing.T) {
	authorizer := nativerouting.ReleasedCatalogAuthorization{}
	validProof := nativerouting.AuthorizationProof{Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "owner/repo", OperatorID: "operator"}
	if err := authorizer.Authorize(validProof); err != nil {
		t.Fatal(err)
	}
	for _, proof := range []nativerouting.AuthorizationProof{
		{},
		{Kind: nativerouting.AuthorizationContributor, Repository: "owner/repo", OperatorID: "operator"},
		{Kind: nativerouting.AuthorizationRepositoryOperator, OperatorID: "operator"},
		{Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "owner/repo"},
	} {
		if err := authorizer.Authorize(proof); err == nil {
			t.Errorf("Authorize(%+v) succeeded", proof)
		}
	}
	if (modelRoutingClock{}).Now().Location() != time.UTC {
		t.Fatal("active clock is not UTC")
	}

	if _, _, err := validateModelCatalog(coverageAgentRuntime{}, nativerouting.RoleImplementation); err == nil {
		t.Fatal("runtime without native surfaces validated")
	}
	native := &coverageNativeRuntime{catalogErr: errors.New("catalog unavailable")}
	if _, _, err := validateModelCatalog(native, nativerouting.RoleImplementation); !errors.Is(err, native.catalogErr) {
		t.Fatalf("catalog error = %v", err)
	}
	native = &coverageNativeRuntime{}
	if _, _, err := validateModelCatalog(native, nativerouting.RoleImplementation); err == nil || !strings.Contains(err.Error(), "validate") {
		t.Fatalf("invalid catalog error = %v", err)
	}
	catalog, qualifications := coverageActiveCatalogFixture(nativerouting.RoleImplementation)
	native = &coverageNativeRuntime{catalog: catalog, qualifications: qualifications}
	gotNative, validated, err := validateModelCatalog(native, nativerouting.RoleImplementation)
	if err != nil || gotNative != native || validated.Identity == "" {
		t.Fatalf("validated catalog = %#v native=%T err=%v", validated, gotNative, err)
	}

	if err := verifyModelSelection(native, nativerouting.Resolution{Class: nativerouting.ResolutionUnsupported}); err == nil {
		t.Fatal("unresolved selection succeeded")
	}
	selected := nativerouting.SelectedProfile{NativeProfileID: "economy", NativeModel: "native-economy", NativeEffort: "low"}
	resolution := nativerouting.Resolution{Class: nativerouting.ResolutionExact, NativeProfile: &selected}
	native.buildErr = errors.New("build failed")
	if err := verifyModelSelection(native, resolution); !errors.Is(err, native.buildErr) {
		t.Fatalf("build error = %v", err)
	}
	native.buildErr = nil
	native.captureErr = errors.New("capture failed")
	if err := verifyModelSelection(native, resolution); !errors.Is(err, native.captureErr) {
		t.Fatalf("capture error = %v", err)
	}
	native.captureErr = nil
	native.capture = &port.LaunchCapture{NativeModel: "other", NativeEffort: "low"}
	if err := verifyModelSelection(native, resolution); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("mismatch error = %v", err)
	}
	native.capture = nil
	if err := verifyModelSelection(native, resolution); err != nil {
		t.Fatalf("matching selection: %v", err)
	}
}

func TestCoverageActiveFactoriesFailClosedBeforeAdmission(t *testing.T) {
	active := &activeRouting{}
	if _, err := active.implementationFactory()(coverageAgentRuntime{}); err == nil {
		t.Fatal("implementation factory accepted a non-native runtime")
	}
	catalog, qualifications := coverageActiveCatalogFixture(nativerouting.RoleImplementation)
	native := &coverageNativeRuntime{catalog: catalog, qualifications: qualifications, buildErr: errors.New("build refused")}
	admit, err := active.implementationFactory()(native)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admit(t.Context(), port.Task{ID: "T1"}); err == nil || !strings.Contains(err.Error(), "persisted semantic demand") {
		t.Fatalf("incomplete implementation task error = %v", err)
	}
	task := coverageActiveRoutingTask()
	if _, err := admit(t.Context(), task); !errors.Is(err, native.buildErr) {
		t.Fatalf("native build refusal = %v", err)
	}

	reviewCatalog, reviewQualifications := coverageActiveCatalogFixture(nativerouting.RoleReview)
	native = &coverageNativeRuntime{catalog: reviewCatalog, qualifications: reviewQualifications}
	reviewAdmit, err := active.reviewAdmission("codex", native)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reviewAdmit(t.Context(), port.Task{ID: "T2"}, "12", "abc", reviewinvoke.Route{}); err == nil || !strings.Contains(err.Error(), "persisted semantic demand") {
		t.Fatalf("incomplete review task error = %v", err)
	}
	wrongRoute := reviewinvoke.Route{NativeProfileID: "wrong", Model: "native-economy", Effort: "low"}
	if _, err := reviewAdmit(t.Context(), task, "12", "abc", wrongRoute); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("review route mismatch = %v", err)
	}

	if _, err := active.implementationOutcomeFactory()(coverageAgentRuntime{}); err == nil || !strings.Contains(err.Error(), "outcome fence") {
		t.Fatalf("implementation outcome runtime error = %v", err)
	}
	if _, err := active.reviewOutcome(coverageAgentRuntime{}); err == nil || !strings.Contains(err.Error(), "outcome fence") {
		t.Fatalf("review outcome runtime error = %v", err)
	}
	fenced := coverageFencedRuntime{coverageAgentRuntime: coverageAgentRuntime{}}
	implementationOutcome, err := active.implementationOutcomeFactory()(fenced)
	if err != nil {
		t.Fatalf("implementation outcome factory: %v", err)
	}
	if err := implementationOutcome(t.Context(), routingevidence.AttemptTerminal{}); err == nil {
		t.Fatal("implementation outcome accepted an empty terminal")
	}
	reviewOutcome, err := active.reviewOutcome(fenced)
	if err != nil {
		t.Fatalf("review outcome factory: %v", err)
	}
	if err := reviewOutcome(t.Context(), routingevidence.AttemptTerminal{}); err == nil {
		t.Fatal("review outcome accepted an empty terminal")
	}
}

func TestCoverageRoutingInputAndAttemptBoundaries(t *testing.T) {
	if err := verifyRequestedProfile(coverageActiveRoutingTask()); err != nil {
		t.Fatal(err)
	}
	for _, task := range []port.Task{{ID: "empty"}, {ID: "no-demand", DemandProfile: coverageActiveRoutingTask().DemandProfile}} {
		if err := verifyRequestedProfile(task); err == nil {
			t.Errorf("verifyRequestedProfile(%+v) succeeded", task)
		}
	}
	native := coverageActiveRoutingTask()
	native.NativeRouting = &taskrouting.Spec{}
	native.DemandID = ""
	if err := verifyRequestedProfile(native); err == nil || !strings.Contains(err.Error(), "frozen native identity") {
		t.Fatalf("missing native identity error = %v", err)
	}
	native.DemandID = "demand-1"
	if err := verifyRequestedProfile(native); err == nil {
		t.Fatal("invalid native routing succeeded")
	}
	native.NativeRouting = &taskrouting.Spec{Schema: taskrouting.Schema,
		Implementation: taskrouting.Route{Harness: entity.Codex, Model: "gpt-6.1-sol", Effort: "high"},
		Review:         taskrouting.Route{Harness: entity.Claude, Model: "claude-opus-4-6", Effort: "high"},
		Rationale:      "coverage fixture"}
	if err := verifyRequestedProfile(native); err != nil {
		t.Fatalf("valid native routing: %v", err)
	}
	first, err := freshRoutingAttempt("review")
	if err != nil || !strings.HasPrefix(first, "review-") || len(first) != len("review-")+32 {
		t.Fatalf("fresh attempt = %q, %v", first, err)
	}
	second, err := freshRoutingAttempt("review")
	if err != nil || first == second {
		t.Fatalf("second fresh attempt = %q, %v", second, err)
	}
	original := activeRoutingDeps
	activeRoutingDeps.randRead = func([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
	if _, err := freshRoutingAttempt("review"); err == nil || !strings.Contains(err.Error(), "entropy unavailable") {
		t.Fatalf("entropy failure = %v", err)
	}
	activeRoutingDeps = original

	for _, args := range [][4]string{{}, {t.TempDir(), "-bad", "main", "policy.json"}, {t.TempDir(), "origin", "-bad", "policy.json"}, {t.TempDir(), "origin\nother", "main", "policy.json"}} {
		if _, _, err := readActivationMainline(t.Context(), args[0], args[1], args[2], args[3]); err == nil {
			t.Errorf("readActivationMainline(%q, %q, %q) succeeded", args[0], args[1], args[2])
		}
	}
	if _, err := activationGit(t.Context(), filepath.Join(t.TempDir(), "missing"), "status"); err == nil || !strings.Contains(err.Error(), "git status") {
		t.Fatalf("activationGit error = %v", err)
	}
	if got := string(trimCanonicalNewline([]byte("policy\n"))); got != "policy" {
		t.Fatalf("trimCanonicalNewline = %q", got)
	}
}

func coverageActivationConfigWire(checkpoint string) []byte {
	return []byte("executor:\n  review:\n    budget_minutes: 15\n  routing_evidence:\n    evidence_ref: refs/pyxis/routing-evidence\n    producer_policy_ref: .pyxis/routing/producer-policy.json\n    activation_checkpoint: " + checkpoint + "\n")
}

type coverageAgentRuntime struct{}

func (coverageAgentRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return nil, nil
}
func (coverageAgentRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (coverageAgentRuntime) Kill(context.Context, port.Handle) error { return nil }
func (coverageAgentRuntime) AuthCheck(context.Context) error         { return nil }
func (coverageAgentRuntime) ModelRoute() port.ModelRoute {
	return port.ModelRoute{Model: "coverage-model", Effort: "low"}
}

type coverageFencedRuntime struct{ coverageAgentRuntime }

func (coverageFencedRuntime) Activate(string, port.OwnerGeneration, port.OwnershipProof) (port.LaunchCapability, error) {
	return port.LaunchCapability{}, nil
}
func (coverageFencedRuntime) LaunchFenced(context.Context, port.LaunchSpec, port.LaunchCapability) (port.Handle, error) {
	return nil, nil
}
func (coverageFencedRuntime) FenceAndObserve(context.Context, string, port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	return port.RuntimeFenceProof{}, port.RuntimeObservation{}, nil
}
func (coverageFencedRuntime) AuthorizeTerminal(port.LaunchCapability) error { return nil }

type coverageNativeRuntime struct {
	coverageAgentRuntime
	catalog        nativerouting.Catalog
	qualifications nativerouting.QualificationSet
	catalogErr     error
	buildErr       error
	captureErr     error
	capture        *port.LaunchCapture
}

func (n *coverageNativeRuntime) Catalog(nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	return n.catalog, n.qualifications, n.catalogErr
}
func (*coverageNativeRuntime) ValidateProfile(nativerouting.CatalogEntry) error { return nil }
func (n *coverageNativeRuntime) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if n.buildErr != nil {
		return port.NativeLaunchSpec{}, n.buildErr
	}
	return port.NativeLaunchSpec{NativeProfileID: selected.NativeProfileID, Role: selected.Role, NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort, Controls: selected.Controls}, nil
}
func (n *coverageNativeRuntime) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	if n.captureErr != nil {
		return port.LaunchCapture{}, n.captureErr
	}
	if n.capture != nil {
		return *n.capture, nil
	}
	return port.LaunchCapture{NativeModel: spec.NativeModel, NativeEffort: spec.NativeEffort, Controls: spec.Controls}, nil
}

func coverageActiveRoutingTask() port.Task {
	return port.Task{ID: "T1", Harness: "codex", DemandID: executiondemand.DemandID("demand-1"), DemandProfile: executiondemand.ProfileMinimum{
		Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary,
	}}
}

func coverageActiveCatalogFixture(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet) {
	now := time.Now().UTC()
	provides := nativerouting.Provides{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight,
		Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary}
	entry := nativerouting.CatalogEntry{NativeProfileID: "economy", NativeModel: "native-economy", NativeEffort: "low", Provides: provides,
		CostRank: 1, QualificationID: "qual-economy", AdapterGeneration: "adapter-gen-1", Strongest: true}
	claims := make([]nativerouting.QualificationClaim, 0, 4)
	levels := map[nativerouting.Component]string{
		nativerouting.ComponentCapability:   string(provides.Capability),
		nativerouting.ComponentDeliberation: string(provides.Deliberation),
		nativerouting.ComponentContext:      string(provides.Context),
		nativerouting.ComponentAssurance:    string(provides.Assurance),
	}
	for _, component := range []nativerouting.Component{nativerouting.ComponentCapability, nativerouting.ComponentDeliberation, nativerouting.ComponentContext, nativerouting.ComponentAssurance} {
		claims = append(claims, nativerouting.QualificationClaim{Component: component, AssertedLevel: levels[component], Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: coverageActiveDigest("fixture-" + string(component)), ResultDigest: coverageActiveDigest("result-" + string(component)), Threshold: "score>=0.80", Result: "score=0.95", Satisfied: true,
			EvidenceReference: "repo:evals/" + string(component), EvaluatorVersion: nativerouting.EvaluatorDecodePinnedV1,
			ResponsibleEvaluator: "evaluator-1", EvidenceOrigin: nativerouting.EvidenceIndependentEvaluator})
	}
	qualification := nativerouting.Qualification{SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID, Harness: "codex", Role: role,
		NativeProfileID: entry.NativeProfileID, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, AdapterGeneration: entry.AdapterGeneration,
		CatalogSchema: nativerouting.CatalogSchema, LaunchCaptureDigest: coverageActiveDigest("launch"), Claims: claims,
		Validity:      nativerouting.Validity{ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), Invalidations: []nativerouting.Invalidation{}},
		Authorization: nativerouting.AuthorizationProof{Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "owner/repo", OperatorID: "operator-1", EvidenceDigest: coverageActiveDigest("authorization"), AuthorizedAt: now.Add(-2 * time.Hour)}}
	entry.QualificationDigest = nativerouting.QualificationDigest(qualification)
	return nativerouting.Catalog{SchemaVersion: nativerouting.CatalogSchema, CatalogID: "codex-" + string(role), Harness: "codex", Role: role,
		AdapterGeneration: "adapter-gen-1", Entries: []nativerouting.CatalogEntry{entry}}, nativerouting.QualificationSet{qualification}
}

func coverageActiveDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
