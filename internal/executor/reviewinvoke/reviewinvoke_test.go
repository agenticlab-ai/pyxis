package reviewinvoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	publicationpkg "github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

func TestRoutingIdentityDigestsAreFullSHA256(t *testing.T) {
	digest := stableDigest("task\x00proposal\x00revision")
	if len(digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatalf("digest is not lowercase SHA-256 hex: %q: %v", digest, err)
	}
	id := stableID("impl-review-attempt", "attempt")
	if wantLen := len("impl-review-attempt-") + 64; len(id) != wantLen {
		t.Fatalf("attempt id length = %d, want %d", len(id), wantLen)
	}
}

func TestCatalogAuthorizationAcceptsReleasedPolicyAcrossInvocationRepositories(t *testing.T) {
	policy := nativerouting.ReleasedCatalogAuthorization{}
	proof := nativerouting.AuthorizationProof{
		Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis", OperatorID: "operator",
	}
	if err := policy.Authorize(proof); err != nil {
		t.Fatalf("released catalog authorization: %v", err)
	}
	proof.Repository = "other/repository"
	if err := policy.Authorize(proof); err != nil {
		t.Fatalf("invocation repository authorization: %v", err)
	}
	proof.Repository = "released/policy-repository"
	if err := policy.Authorize(proof); err != nil {
		t.Fatalf("released policy must be reusable outside its subject repository: %v", err)
	}
}

// runGit runs a git command in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	mustGit(t, dir, args...)
}

// mustGit runs a git command in dir, returning its stdout and failing on error.
func mustGit(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	// A hermetic identity so commits succeed regardless of the host's git config.
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@pyxis.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@pyxis.invalid")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, ee.Stderr)
		}
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

type fakeHandle struct{}

func (fakeHandle) RuntimeHandle() {}

type fakeReviewPermit struct {
	err     error
	calls   int
	binding routingevidence.BoundaryBinding
}

func (p *fakeReviewPermit) Authorize(boundary routingevidence.BoundaryOperation, binding routingevidence.BoundaryBinding) error {
	p.calls++
	p.binding = binding
	if boundary != routingevidence.OperationReviewLaunch {
		return errors.New("wrong review boundary")
	}
	return p.err
}

func (*fakeReviewPermit) RuntimeOwnershipProof() (port.OwnershipProof, error) {
	return port.OwnershipProof{Kind: port.OwnershipProofResolution, Reference: "route-event", Digest: strings.Repeat("a", 64),
		AttemptID: "review-route-attempt", Harness: "claude", OwnerGeneration: 1}, nil
}

func (*fakeReviewPermit) NativeLaunch(nativerouting.Role) (port.NativeLaunchSpec, error) {
	return port.NativeLaunchSpec{NativeProfileID: "review-maximum", Role: nativerouting.RoleReview,
		NativeModel: "review-model", NativeEffort: "max"}, nil
}

// fakeRuntime is the review launch double. Review routing resolves its model
// from the role catalog and does not ask this runtime to select one.
type fakeRuntime struct {
	spec               port.LaunchSpec
	assembled          string
	launches           int
	killed             bool
	launchErr          error
	awaitErr           error
	killErr            error
	result             port.RunResult
	blockUntil         context.Context
	launched           chan struct{}
	launchCheck        func(port.LaunchSpec) error
	launchContextCheck func(context.Context, port.LaunchSpec) error
}

type preflightRuntime struct {
	*claude.Adapter
	authErr error
	calls   int
}

type fakeProposalInspector struct{}

func (fakeProposalInspector) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return port.LandingOpen, "abc123", port.CheckGreen, nil
}

func (r *preflightRuntime) AuthCheck(context.Context) error {
	r.calls++
	return r.authErr
}

func TestPreflightAuthenticatesRuntimeAndArtifactBroker(t *testing.T) {
	adapter, err := claude.New(claude.WithProfiles(fixtureHarnessConfig(t).Profiles("claude")))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		authErr   error
		brokerErr error
		want      string
	}{
		{name: "authenticated"},
		{name: "runtime auth failure", authErr: errors.New("login missing"), want: "login missing"},
		{name: "broker auth failure", brokerErr: errors.New("comment login missing"), want: "comment login missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := &preflightRuntime{Adapter: adapter, authErr: tc.authErr}
			inv := lifecycleInvoker(t, rt, &fakeVerifier{authErr: tc.brokerErr}, time.Minute)
			inv.WithProposalInspector(fakeProposalInspector{}).WithClock(func() time.Time {
				return time.Date(2030, time.August, 5, 12, 0, 0, 0, time.UTC)
			})
			err = inv.Preflight(context.Background())
			if tc.want == "" && err != nil {
				t.Fatalf("Preflight: %v", err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("Preflight error = %v, want %q", err, tc.want)
			}
			if rt.calls != 1 {
				t.Fatalf("AuthCheck calls = %d, want 1", rt.calls)
			}
		})
	}
}

func (*fakeRuntime) AuthCheck(context.Context) error { return nil }

func (f *fakeRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	f.launches++
	f.spec = spec
	assembled, err := executorruntime.AssembleBrief(spec)
	if err != nil {
		return nil, err
	}
	f.assembled = assembled
	if f.launchContextCheck != nil {
		if err := f.launchContextCheck(ctx, spec); err != nil {
			return nil, err
		}
	}
	if f.launchCheck != nil {
		if err := f.launchCheck(spec); err != nil {
			return nil, err
		}
	}
	if f.launchErr != nil {
		return nil, f.launchErr
	}
	if f.launched != nil {
		select {
		case f.launched <- struct{}{}:
		default:
		}
	}
	return fakeHandle{}, nil
}

func (*fakeRuntime) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	return port.LaunchCapability{AttemptID: attempt, Harness: proof.Harness, OwnerGeneration: generation,
		ProofReference: proof.Reference, ProofDigest: proof.Digest, Nonce: "nonce", CapabilityRef: "capability", Signature: "signature"}, nil
}

func (f *fakeRuntime) LaunchFenced(ctx context.Context, spec port.LaunchSpec, _ port.LaunchCapability) (port.Handle, error) {
	return f.Launch(ctx, spec)
}

func (*fakeRuntime) FenceAndObserve(context.Context, string, port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	return port.RuntimeFenceProof{}, port.RuntimeObservation{}, nil
}

func (*fakeRuntime) AuthorizeTerminal(port.LaunchCapability) error { return nil }

func (f *fakeRuntime) Await(ctx context.Context, _ port.Handle) (port.RunResult, error) {
	if f.blockUntil != nil {
		<-ctx.Done()
		return port.RunResult{}, ctx.Err()
	}
	result := f.result
	if result.Output == "" && !result.Incomplete && f.awaitErr == nil {
		result.Output = artifactFromBrokerBrief(f.spec.Brief)
	}
	// Real-input fixtures model a reviewer writing the remaining design input
	// and complete canonical pack. Never replace a deliberately forged pack.
	root := filepath.Join(f.spec.RuntimeStateDir, "review-evidence", "input-root")
	packPath := filepath.Join(f.spec.RuntimeStateDir, "review-evidence", "pack.txt")
	if _, err := os.Stat(root); err == nil && !result.Incomplete && f.awaitErr == nil {
		if _, err := os.Stat(packPath); os.IsNotExist(err) {
			design := filepath.Join(root, "review", "design-bindings.md")
			if _, err := os.Stat(design); os.IsNotExist(err) {
				if err := os.WriteFile(design, []byte("test governing design bindings\n"), 0o600); err != nil {
					return result, err
				}
			}
			report, err := persist.ParseHandoff(artifactFromBrokerBrief(f.spec.Brief))
			if err != nil {
				return result, err
			}
			body, err := pack.BuildFromRoot(ctx, root, report.ProposalRef)
			if err != nil {
				return result, err
			}
			if err := os.WriteFile(packPath, body, 0o600); err != nil {
				return result, err
			}
		}
		if result.ReviewChildren == nil {
			if result.SessionID == "" {
				result.SessionID = fixtureNativeReviewParent
			}
			var err error
			result.ReviewChildren, err = fixtureNativeLenses(f.spec.RuntimeStateDir, artifactFromBrokerBrief(f.spec.Brief))
			if err != nil {
				return result, err
			}
		}
	}
	return result, f.awaitErr
}

func artifactFromBrokerBrief(brief string) string {
	lines := strings.Split(brief, "\n")
	fields := strings.Fields(lines[0])
	if len(fields) != 3 {
		return ""
	}
	const marker = "Review process: "
	_, rest, found := strings.Cut(brief, marker)
	if !found {
		return ""
	}
	reviewID := strings.Fields(rest)[0]
	contextID, err := implruntime.ContextID(fixtureNativeReviewParent)
	if err != nil {
		return ""
	}
	body, _ := persist.ComposeArtifact(&implreport.Report{
		SchemaVersion: implreport.SchemaVersion, ProposalRef: fields[1], ReviewID: reviewID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{
			ReviewedRevision: fields[2], ContextID: contextID,
			Provenance: implreport.Provenance{Model: "review-model", PromptsVersion: "v1", CalVersion: "v1"},
			Findings:   []implreport.Finding{},
		}},
	})
	return body
}

func TestHostedTerminalVerifierRejectsMissingReceipt(t *testing.T) {
	publication := implreview.PublicationTerminal{ReviewProcessID: "process-1", ReviewObligationID: "obligation-1",
		ProposalID: "github:o/r#548", ReviewedRevision: "abc123", Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:artifact", AttemptProofRef: "proof-1"}
	host := worker.ReviewPublicationTerminal{ReviewAttemptID: "process-1", ReviewObligationID: "obligation-1",
		ProposalRef: "github:o/r#548", Revision: "abc123", Status: "verified",
		ArtifactRef: "sha256:artifact", AttemptProofRef: "proof-1"}
	v := &StoreTerminalVerifier{publications: terminalRecordReader(publication)}
	if err := v.VerifyPair(context.Background(), publication, host); err == nil {
		t.Fatal("verified publication pair without native receipt was accepted")
	}
}

func (f *fakeRuntime) Kill(context.Context, port.Handle) error {
	f.killed = true
	return f.killErr
}

type fakeVerifier struct {
	artifactRef     string
	nativeReceipt   string
	attemptProofRef string
	err             error
	authErr         error
	calls           int
	publishes       int
	binding         implreview.VerifiedPublicationBinding
	handoff         implreview.AggregateHandoff
	record          publicationpkg.Record
	recovery        *implreview.PublicationTerminal
	recoveryState   implreview.PublicationRecoveryState
	consumeRecovery bool
	recoveryErr     error
	inspectCalls    int
	recoverCalls    int
}

func (v *fakeVerifier) RecoverPublication(context.Context, implreview.ReviewSubject) (implreview.RecoveredPublication, bool, error) {
	v.recoverCalls++
	if v.recovery == nil {
		return implreview.RecoveredPublication{}, false, nil
	}
	terminal := *v.recovery
	if v.consumeRecovery {
		v.recovery = nil
	}
	if v.recoveryErr != nil {
		return implreview.RecoveredPublication{}, false, v.recoveryErr
	}
	return implreview.RecoveredPublication{Binding: implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: terminal.ReviewObligationID, ReviewProcessID: terminal.ReviewProcessID,
		ProposalID: terminal.ProposalID, ReviewedRevision: terminal.ReviewedRevision, AttemptProofRef: terminal.AttemptProofRef,
	}, ProofEvidenceRef: "evidence:attempt:recovery"}, Terminal: terminal}, true, nil
}

func (v *fakeVerifier) InspectPublication(context.Context, implreview.ReviewSubject) (implreview.PublicationRecovery, bool, error) {
	v.inspectCalls++
	if v.recovery == nil {
		return implreview.PublicationRecovery{}, false, nil
	}
	terminal := *v.recovery
	state := v.recoveryState
	if state == "" {
		state = implreview.PublicationRecoveryTerminal
	}
	return implreview.PublicationRecovery{Binding: implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: terminal.ReviewObligationID, ReviewProcessID: terminal.ReviewProcessID,
		ProposalID: terminal.ProposalID, ReviewedRevision: terminal.ReviewedRevision, AttemptProofRef: terminal.AttemptProofRef,
	}, ProofEvidenceRef: "evidence:attempt:recovery"}, State: state}, true, nil
}

func (v *fakeVerifier) Preflight(context.Context, string) error {
	return v.authErr
}

type terminalFailStore struct{ reviewobligation.Store }

func (terminalFailStore) AppendTerminal(context.Context, reviewobligation.Terminal) error {
	return errors.New("terminal store unavailable")
}

func (v *fakeVerifier) Complete(_ context.Context, binding implreview.VerifiedPublicationBinding, handoff implreview.AggregateHandoff) (implreview.PublicationTerminal, error) {
	v.calls++
	v.publishes++
	v.binding, v.handoff = binding, handoff
	if v.err != nil {
		return implreview.PublicationTerminal{}, v.err
	}
	if handoff.Status != implreview.HandoffCompleted || handoff.Aggregate == nil {
		terminal := implreview.PublicationTerminal{ReviewProcessID: binding.ReviewProcessID, ReviewObligationID: binding.ReviewObligationID,
			PublicationAttemptID: "publication-1", ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
			Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationFailed,
			FailureClass: implreview.FailureArtifactUnverified, Reason: "invalid process handoff"}
		terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
		v.record = publicationpkg.Record{PublicationAttemptID: terminal.PublicationAttemptID, Stage: publicationpkg.StageTerminal, Terminal: &terminal}
		return terminal, nil
	}
	ref := v.artifactRef
	if ref == "" {
		body, err := persist.ComposeArtifact(handoff.Aggregate.Report)
		if err != nil {
			return implreview.PublicationTerminal{}, err
		}
		digest := sha256.Sum256([]byte(body))
		ref = "sha256:" + hex.EncodeToString(digest[:])
	}
	receipt := v.nativeReceipt
	if receipt == "" {
		receipt = "github-comment:R_test:77"
	}
	proof := v.attemptProofRef
	if proof == "" {
		proof = binding.AttemptProofRef
	}
	terminal := implreview.PublicationTerminal{ReviewProcessID: binding.ReviewProcessID, ReviewObligationID: binding.ReviewObligationID,
		PublicationAttemptID: "publication-1", ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
		Medium: "github", IdempotencyID: "idempotency-1", Status: implreview.PublicationVerified,
		ArtifactRef: ref, NativeReceipt: receipt, AttemptProofRef: proof}
	terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
	v.record = publicationpkg.Record{PublicationAttemptID: terminal.PublicationAttemptID, Stage: publicationpkg.StageTerminal, Terminal: &terminal}
	return terminal, nil
}

func (v *fakeVerifier) Record(_ context.Context, id string) (publicationpkg.Record, bool, error) {
	return v.record, v.record.PublicationAttemptID == id, nil
}

func (v *fakeVerifier) ExactRead(_ context.Context, receipt string) (*implreport.Report, error) {
	if receipt != "github-comment:R_test:77" {
		return nil, errors.New("native receipt does not exist")
	}
	if v.handoff.Aggregate == nil {
		return nil, errors.New("no aggregate")
	}
	return v.handoff.Aggregate.Report, nil
}

type fakeProcessCoordinator struct {
	preflightErr error
	nativeErr    error
	processID    string
	prepares     int
	completes    int
}

func (p *fakeProcessCoordinator) Preflight(context.Context) error { return p.preflightErr }

func (p *fakeProcessCoordinator) RecordNativeLensEvidence(context.Context, implreview.AdmittedProcess, implreview.NativeLensEvidence) error {
	return p.nativeErr
}

func (p *fakeProcessCoordinator) VerifyNativeLensEvidence(context.Context, implreview.AdmittedProcess) error {
	return p.nativeErr
}

func (p *fakeProcessCoordinator) Prepare(_ context.Context, subject implreview.ReviewSubject) (implreview.AdmittedProcess, error) {
	p.prepares++
	processID := p.processID
	if processID == "" {
		processID = "review-process-test"
	}
	return implreview.AdmittedProcess{ReviewProcessID: processID, Subject: subject}, nil
}

func TestInvokeRecoversOriginalPublicationIdentityWithoutLaunchingReviewer(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	processes := &fakeProcessCoordinator{}
	i.processes = processes
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	captureForInvoke(i, task, proposal)
	proof := "proof-original"
	lease, err := i.store.Begin(context.Background(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", RunID: "run-original", AttemptProofRef: proof,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	report := hostTestReportForRecovery("process-original", "github:o/r#548", "abc123")
	body, err := persist.ComposeArtifact(report)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(body))
	terminal := implreview.PublicationTerminal{
		ReviewProcessID: "process-original", ReviewObligationID: "review-obligation-test",
		PublicationAttemptID: "publication-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-original", Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:" + hex.EncodeToString(digest[:]), NativeReceipt: "github-comment:R_test:77", AttemptProofRef: proof,
	}
	terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
	verifier.recovery = &terminal
	verifier.record = publicationpkg.Record{PublicationAttemptID: terminal.PublicationAttemptID, Stage: publicationpkg.StageTerminal, Terminal: &terminal}
	verifier.handoff = implreview.AggregateHandoff{Status: implreview.HandoffCompleted, Aggregate: &implreview.CompletedAggregate{
		ProcessID: "process-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123", Report: report,
	}}

	result := i.Invoke(context.Background(), task, proposal)
	if result.Err != nil || result.Status != "verified" || result.AttemptID != "process-original" {
		t.Fatalf("recovered result = %+v", result)
	}
	if rt.launches != 0 || processes.prepares != 0 || verifier.publishes != 0 {
		t.Fatalf("recovery launched work: runtime=%d prepares=%d publishes=%d", rt.launches, processes.prepares, verifier.publishes)
	}
}

func TestInvokeTerminalizesArmedRecoveryThenFreshAttemptConverges(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{recoveryState: implreview.PublicationRecoveryArmed, consumeRecovery: true}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	processes := &fakeProcessCoordinator{}
	i.processes = processes
	proof := "proof-original"
	lease, err := i.store.Begin(context.Background(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", RunID: "run-original", AttemptProofRef: proof,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	terminal := implreview.PublicationTerminal{
		ReviewProcessID: "process-original", ReviewObligationID: "review-obligation-test",
		PublicationAttemptID: "publication-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-original", Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureInfrastructure, Reason: "publication acknowledgement is indeterminate",
	}
	terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
	verifier.recovery = &terminal
	verifier.record = publicationpkg.Record{PublicationAttemptID: terminal.PublicationAttemptID, Stage: publicationpkg.StageTerminal, Terminal: &terminal}

	first := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if first.Status != "failed" || first.AttemptID != "process-original" || rt.launches != 0 || verifier.recoverCalls != 1 {
		t.Fatalf("first recovery=%+v launches=%d recoveries=%d", first, rt.launches, verifier.recoverCalls)
	}
	second := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if second.Err != nil || second.Status != "verified" || second.AttemptID == "process-original" || rt.launches != 1 || processes.prepares != 1 {
		t.Fatalf("fresh retry=%+v launches=%d prepares=%d", second, rt.launches, processes.prepares)
	}
}

func TestInvokeRecoveryErrorSettlesOriginalAttemptBeforeFreshRetry(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{recoveryState: implreview.PublicationRecoveryArmed, consumeRecovery: true,
		recoveryErr: errors.New("publication state unavailable after inspection")}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	lease, err := i.store.Begin(context.Background(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", RunID: "run-original", AttemptProofRef: "proof-original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	verifier.recovery = &implreview.PublicationTerminal{
		ReviewProcessID: "process-original", ReviewObligationID: "review-obligation-test",
		PublicationAttemptID: "publication-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-original", Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureInfrastructure, Reason: "indeterminate",
	}
	first := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if first.Status != "failed" || first.AttemptID != "process-original" || rt.launches != 0 {
		t.Fatalf("failed recovery=%+v launches=%d", first, rt.launches)
	}
	verifier.recoveryErr = nil
	second := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if second.Err != nil || second.Status != "verified" || second.AttemptID == "process-original" || rt.launches != 1 {
		t.Fatalf("fresh retry=%+v launches=%d", second, rt.launches)
	}
}

func TestInvokeDoesNotRewriteSettledRecoveryAttempt(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	proof := "proof-original"
	lease, err := i.store.Begin(context.Background(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", RunID: "run-original", AttemptProofRef: proof,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := reviewobligation.Terminal{SchemaVersion: reviewobligation.TerminalSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", Status: "failed", FailureClass: worker.ReviewFailureInfrastructure}
	if err := i.store.AppendTerminal(context.Background(), failed); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	report := hostTestReportForRecovery("process-original", "github:o/r#548", "abc123")
	body, _ := persist.ComposeArtifact(report)
	digest := sha256.Sum256([]byte(body))
	terminal := implreview.PublicationTerminal{
		ReviewProcessID: "process-original", ReviewObligationID: "review-obligation-test",
		PublicationAttemptID: "publication-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-original", Status: implreview.PublicationVerified,
		ArtifactRef: "sha256:" + hex.EncodeToString(digest[:]), NativeReceipt: "github-comment:R_test:77", AttemptProofRef: proof,
	}
	terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
	verifier.recovery = &terminal

	result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err != nil || result.Status != "verified" || result.AttemptID == "process-original" || rt.launches != 1 || verifier.recoverCalls != 0 {
		t.Fatalf("fresh result=%+v launches=%d recoveries=%d", result, rt.launches, verifier.recoverCalls)
	}
	got, found, err := i.store.Terminal(context.Background(), "review-obligation-test", "process-original")
	if err != nil || !found || got != failed {
		t.Fatalf("original terminal was rewritten: %+v found=%v err=%v", got, found, err)
	}
}

func TestInvokeRecoveryCannotMutateWithoutOriginalAttemptLease(t *testing.T) {
	verifier := &fakeVerifier{recoveryState: implreview.PublicationRecoveryArmed}
	i := lifecycleInvoker(t, &fakeRuntime{}, verifier, time.Minute)
	lease, err := i.store.Begin(context.Background(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "process-original", RunID: "run-original", AttemptProofRef: "proof-original",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	}()
	terminal := implreview.PublicationTerminal{
		ReviewProcessID: "process-original", ReviewObligationID: "review-obligation-test",
		PublicationAttemptID: "publication-original", ProposalID: "github:o/r#548", ReviewedRevision: "abc123",
		Medium: "github", IdempotencyID: "idempotency-original", Status: implreview.PublicationFailed,
		FailureClass: implreview.FailureInfrastructure, Reason: "indeterminate",
	}
	terminal.EVT13Ref, _ = publicationpkg.TerminalRef(terminal)
	verifier.recovery = &terminal
	result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if !errors.Is(result.Err, reviewobligation.ErrAttemptActive) || verifier.recoverCalls != 0 {
		t.Fatalf("unleased recovery=%+v mutations=%d", result, verifier.recoverCalls)
	}
}

func hostTestReportForRecovery(processID, proposal, revision string) *implreport.Report {
	return &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: proposal, ReviewID: processID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: revision, ContextID: "context-recovery", Findings: []implreport.Finding{}}}}
}

func (p *fakeProcessCoordinator) Complete(_ context.Context, admitted implreview.AdmittedProcess, handoff implreview.ProcessHandoff) (implreview.AggregateHandoff, error) {
	p.completes++
	if handoff.Status != implreview.HandoffCompleted || strings.TrimSpace(handoff.ArtifactBody) == "" {
		return implreview.AggregateHandoff{Status: implreview.HandoffMissing, DiagnosticDigest: "sha256:missing"}, nil
	}
	review, err := persist.ParseArtifact(strings.ReplaceAll(handoff.ArtifactBody, "\r\n", "\n"))
	if err != nil {
		return implreview.AggregateHandoff{Status: implreview.HandoffMalformed, DiagnosticDigest: "sha256:malformed"}, nil
	}
	if review.ReviewID != admitted.ReviewProcessID || review.ProposalRef != admitted.Subject.ProposalID {
		return implreview.AggregateHandoff{Status: implreview.HandoffMismatched, DiagnosticDigest: "sha256:mismatched"}, nil
	}
	return implreview.AggregateHandoff{Status: implreview.HandoffCompleted, DiagnosticDigest: "sha256:aggregate", Aggregate: &implreview.CompletedAggregate{
		ProcessID: admitted.ReviewProcessID, ProposalID: admitted.Subject.ProposalID,
		ReviewedRevision: admitted.Subject.ReviewedRevision, Report: review,
	}}, nil
}

func TestInvokeDelegatesPublicationToHostPersistence(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	captureForInvoke(i, task, proposal)
	result := i.Invoke(context.Background(), task, proposal)
	if result.Err != nil || result.ArtifactRef == "" || verifier.calls != 1 || verifier.handoff.Aggregate == nil {
		t.Fatalf("result=%+v persistence calls=%d, want one canonical aggregate publication", result, verifier.calls)
	}
}

func TestInvokeRejectsPublisherFabricatedArtifactEvidence(t *testing.T) {
	verifier := &fakeVerifier{artifactRef: "sha256:forged"}
	i := lifecycleInvoker(t, &fakeRuntime{}, verifier, time.Minute)
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	captureForInvoke(i, task, proposal)
	result := i.Invoke(context.Background(), task, proposal)
	if result.Err == nil || result.FailureClass != worker.ReviewFailureArtifactUntrusted || !strings.Contains(result.Err.Error(), "field-identical") {
		t.Fatalf("fabricated publication evidence was accepted: %+v", result)
	}
}

func TestInvokeRejectsPublisherFabricatedReceiptAndAttemptProof(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verifier *fakeVerifier
	}{
		{name: "native receipt", verifier: &fakeVerifier{nativeReceipt: "github-comment:R_forged:99"}},
		{name: "attempt proof", verifier: &fakeVerifier{attemptProofRef: "proof-forged"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := lifecycleInvoker(t, &fakeRuntime{}, tc.verifier, time.Minute)
			task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
			captureForInvoke(i, task, proposal)
			result := i.Invoke(context.Background(), task, proposal)
			if result.Err == nil || result.FailureClass != worker.ReviewFailureArtifactUntrusted {
				t.Fatalf("fabricated %s was accepted: %+v", tc.name, result)
			}
		})
	}
}

func TestInvokeWorkspaceMutationFailsBeforeArtifactVerification(t *testing.T) {
	verifier := &fakeVerifier{artifactRef: "github:99"}
	i := lifecycleInvoker(t, &fakeRuntime{}, verifier, time.Minute).
		WithWorkspaceVerifier(func(context.Context, string, string) error { return errors.New("dirty tree") })
	result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || result.Phase != worker.ReviewPhaseAwait || result.FailureClass != worker.ReviewFailureInfrastructure {
		t.Fatalf("workspace mutation result = %+v", result)
	}
	if verifier.calls != 0 {
		t.Fatalf("artifact verifier calls = %d, want zero after workspace mutation", verifier.calls)
	}
}

func TestInvokeDoesNotReportVerifiedWhenTerminalCannotBecomeDurable(t *testing.T) {
	i := lifecycleInvoker(t, &fakeRuntime{}, &fakeVerifier{}, time.Minute)
	i.WithObligationStore(terminalFailStore{Store: i.store})
	result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Status != "failed" || result.ObligationStatus != "pending" || result.FailureClass != worker.ReviewFailureInfrastructure {
		t.Fatalf("terminal persistence result = %+v", result)
	}
	if result.ArtifactRef == "" || result.ArtifactProofRef == "" || result.NativeReceipt == "" || result.Publication == nil || !strings.Contains(result.Err.Error(), "terminal store unavailable") {
		t.Fatalf("lifecycle failure lost the already durable publication pair: %+v", result)
	}
}

func lifecycleInvoker(t *testing.T, rt port.AgentRuntime, persistenceAdapter *fakeVerifier, budget time.Duration) *Invoker {
	t.Helper()
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	terminalVerifier, err := NewStoreTerminalVerifier(store, persistenceAdapter)
	if err != nil {
		t.Fatal(err)
	}
	i, err := New(Dependencies{Runtime: rt, Harness: "claude", RepositoryDir: "/repo", RepositorySlug: "o/r", Remote: "origin",
		Budget: budget, Processes: &fakeProcessCoordinator{}, Persistence: persistenceAdapter,
		TerminalVerifier: terminalVerifier, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	// Production prepares a disposable workspace and keeps writable runtime
	// state beside it. The fake must also stay within the test-owned scratch.
	workdir := t.TempDir()
	i = i.WithObligationStore(store).WithRouteResolver(func(context.Context, port.Task, string, string) (Route, error) {
		return Route{
			NativeProfileID: "review-maximum",
			CatalogID:       "claude-review-catalog",
			PolicyIdentity:  "sha256:catalog-policy",
			Model:           "review-model",
			Effort:          "max",
		}, nil
	}).WithWorkspacePreparer(func(context.Context, string) (string, func(), error) { return workdir, func() {}, nil }).
		WithWorkspaceVerifier(func(context.Context, string, string) error { return nil })
	// Lifecycle fixtures use synthetic revisions; real input preparation is
	// exercised separately against committed repositories.
	i.prepareInputs = func(context.Context, reviewInputPreparation) (*preparedReviewInputs, error) { return nil, nil }
	captureForInvoke(i, port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	return i
}

func captureForInvoke(i *Invoker, task port.Task, proposal port.Proposal) {
	ref, _ := persist.GitHubProposalRef("o/r", proposal.Number)
	c := worker.ReviewCapture{ProposalRef: ref, Revision: "abc123", ObligationID: "review-obligation-test", Harness: "claude"}
	if err := i.store.Append(context.Background(), reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: c.ObligationID, Repository: "o/r", TaskID: task.ID, ProposalRef: c.ProposalRef, Revision: c.Revision, Harness: c.Harness}); err != nil {
		panic(err)
	}
	if err := i.store.CompleteHandoff(context.Background(), c.ObligationID); err != nil {
		panic(err)
	}
	i.captures[captureKey(task, proposal)] = c
}

func TestInvokeLaunchesBareReviewBriefAndVerifiesArtifact(t *testing.T) {
	rt := &fakeRuntime{result: port.RunResult{Tokens: 37}, launchCheck: func(spec port.LaunchSpec) error {
		if strings.TrimSpace(spec.RuntimeStateDir) == "" {
			return errors.New("review launch omitted writable runtime state")
		}
		rel, err := filepath.Rel(spec.Workdir, spec.RuntimeStateDir)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return errors.New("review runtime state is inside the reviewed tree")
		}
		return os.WriteFile(filepath.Join(spec.RuntimeStateDir, "home", "writable-probe"), []byte("ok\n"), 0o600)
	}}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute)
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	captureForInvoke(i, task, proposal)
	result := i.Invoke(context.Background(), task, proposal)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if rt.launches != 1 {
		t.Fatalf("reviewer launched %d times, want exactly once", rt.launches)
	}
	if rt.spec.Purpose != port.LaunchPurposeReview || rt.spec.Workdir == "" || rt.spec.Workdir == "/repo" {
		t.Fatalf("launch identity = purpose %q workdir %q, want isolated review workspace", rt.spec.Purpose, rt.spec.Workdir)
	}
	if _, err := os.Stat(rt.spec.RuntimeStateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disposable reviewer runtime state was not cleaned: %v", err)
	}
	if rt.spec.HeadBranch != "" {
		t.Fatalf("review launch leaked implementation fields: %+v", rt.spec)
	}
	if rt.spec.Model != "review-model" || rt.spec.Effort != "max" {
		t.Fatalf("launch route = model %q effort %q, want review-model/max", rt.spec.Model, rt.spec.Effort)
	}
	for _, forbidden := range []string{"delivery protocol", "COMMIT", "Do NOT push", "nothing-to-propose"} {
		if strings.Contains(rt.assembled, forbidden) {
			t.Errorf("review brief contains implementation protocol %q:\n%s", forbidden, rt.assembled)
		}
	}
	for _, want := range []string{"/impl-review github:o/r#548 abc123", "sibling", "host persistence adapter alone publishes", "Native reviewer model: \"" + rt.spec.Model + "\""} {
		if !strings.Contains(rt.assembled, want) {
			t.Errorf("review brief missing %q:\n%s", want, rt.assembled)
		}
	}
	if verifier.calls != 1 || verifier.binding.ProposalID != "github:o/r#548" || verifier.binding.ReviewedRevision != "abc123" {
		t.Fatalf("host persistence = calls %d binding %+v", verifier.calls, verifier.binding)
	}
	if verifier.publishes != 1 || verifier.handoff.Aggregate == nil {
		t.Fatalf("host persistence = publishes %d handoff %+v", verifier.publishes, verifier.handoff)
	}
	if verifier.binding.ReviewProcessID == "" || verifier.binding.ReviewProcessID != result.AttemptID || !strings.Contains(rt.assembled, result.AttemptID) {
		t.Fatalf("attribution proof = persistence process %q result attempt %q brief=%q", verifier.binding.ReviewProcessID, result.AttemptID, rt.assembled)
	}
	if !strings.HasPrefix(result.ArtifactRef, "sha256:") || result.NativeReceipt == "" || result.EVT13Ref == "" || result.EVT17Ref == "" || result.Phase != worker.ReviewPhaseArtifact {
		t.Fatalf("review result = %+v, want verified receipted publication pair", result)
	}
	if result.Tokens != 37 {
		t.Fatalf("review tokens = %d, want 37", result.Tokens)
	}
	if rt.killed {
		t.Error("a verified review must not be killed")
	}
}

func TestInvoke_RoutingPermitMechanicallyGatesReviewerLaunch(t *testing.T) {
	for _, tc := range []struct {
		name       string
		admitErr   error
		permitErr  error
		wantLaunch int
	}{
		{name: "admission unavailable", admitErr: errors.New("evidence unavailable")},
		{name: "invalid permit", permitErr: errors.New("stale exact proof")},
		{name: "valid permit", wantLaunch: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime := &fakeRuntime{}
			permit := &fakeReviewPermit{err: tc.permitErr}
			var terminal *routingevidence.AttemptTerminal
			invoker := lifecycleInvoker(t, runtime, &fakeVerifier{}, time.Minute).WithReviewAdmission(
				func(_ context.Context, task port.Task, proposalRef, revision string, route Route) (routingevidence.BoundaryPermit, error) {
					if task.ID != "T5" || proposalRef != "github:o/r#548" || revision != "abc123" || route.NativeProfileID != "review-maximum" {
						t.Fatalf("review admission inputs: task=%+v proposal=%q revision=%q route=%+v", task, proposalRef, revision, route)
					}
					if tc.admitErr != nil {
						return nil, tc.admitErr
					}
					return permit, nil
				}).WithReviewOutcome(func(_ context.Context, got routingevidence.AttemptTerminal) error {
				terminal = &got
				return nil
			})
			result := invoker.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
			if runtime.launches != tc.wantLaunch {
				t.Fatalf("review result=%+v launches=%d, want %d", result, runtime.launches, tc.wantLaunch)
			}
			if tc.wantLaunch == 0 && (result.Err == nil || result.Phase != worker.ReviewPhaseRouting) {
				t.Fatalf("denied review result=%+v", result)
			}
			if tc.wantLaunch == 1 && result.Err != nil {
				t.Fatalf("admitted review result=%+v", result)
			}
			if tc.admitErr == nil {
				if permit.calls != 1 || permit.binding.Role != nativerouting.RoleReview || permit.binding.Harness != "claude" ||
					permit.binding.TaskID != "T5" || permit.binding.ProposalID != "github:o/r#548" {
					t.Fatalf("review permit binding: calls=%d binding=%+v", permit.calls, permit.binding)
				}
			}
			if tc.wantLaunch == 1 && (terminal == nil || terminal.Outcome != routingevidence.OutcomeVerified || terminal.AttemptID != "review-route-attempt" || terminal.LaunchCapabilityRef == "") {
				t.Fatalf("review routed terminal = %+v", terminal)
			}
			if tc.wantLaunch == 0 && terminal != nil {
				t.Fatalf("denied review emitted routed terminal %+v", terminal)
			}
		})
	}
}

func TestInvokeCleanExitWithoutArtifactFails(t *testing.T) {
	rt := &fakeRuntime{result: port.RunResult{Output: "reviewer returned no artifact"}}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || result.Phase != worker.ReviewPhaseArtifact || result.FailureClass != worker.ReviewFailureArtifactUnverified {
		t.Fatalf("result = %+v, want artifact_unverified", result)
	}
	if result.ArtifactRef != "" {
		t.Fatalf("failed verification asserted artifact %q", result.ArtifactRef)
	}
}

func TestInvokeRejectsMismatchedProcessHandoff(t *testing.T) {
	rt := &fakeRuntime{result: port.RunResult{Output: artifactFromBrokerBrief(
		"/impl-review github:o/r#548 abc123\n\nReview process: attacker-chosen\n",
	)}}
	broker := &fakeVerifier{}
	result := lifecycleInvoker(t, rt, broker, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.FailureClass != worker.ReviewFailureArtifactUnverified || result.Phase != worker.ReviewPhaseArtifact {
		t.Fatalf("wrong-process handoff = %+v", result)
	}
	if broker.calls != 1 || broker.handoff.Status != implreview.HandoffMismatched {
		t.Fatalf("mismatched handoff was not closed by host persistence: calls=%d handoff=%+v", broker.calls, broker.handoff)
	}
}

func TestInvokeHostPersistenceFailureIsInfrastructureAndNeverVerifies(t *testing.T) {
	writeErr := errors.New("proposal write unavailable")
	broker := &fakeVerifier{err: writeErr}
	result := lifecycleInvoker(t, &fakeRuntime{}, broker, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if !errors.Is(result.Err, writeErr) || result.FailureClass != worker.ReviewFailureInfrastructure || result.Phase != worker.ReviewPhaseArtifact {
		t.Fatalf("broker write failure = %+v", result)
	}
	if broker.publishes != 1 || broker.calls != 1 {
		t.Fatalf("host persistence failure calls = publish:%d complete:%d, want 1/1", broker.publishes, broker.calls)
	}
}

func TestInvokeBudgetOverrunKillsAndPreservesDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rt := &fakeRuntime{blockUntil: context.Background()}
		result := lifecycleInvoker(t, rt, &fakeVerifier{}, 30*time.Minute).Invoke(
			context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
		if !errors.Is(result.Err, context.DeadlineExceeded) || result.Phase != worker.ReviewPhaseAwait {
			t.Fatalf("result = %+v, want await deadline", result)
		}
		if !rt.killed {
			t.Fatal("an overrun reviewer must be killed")
		}

	})
}

func TestInvokeConcurrentProcessesFenceOneObligationBeforeLaunch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered := make(chan struct{}, 1)
		firstRuntime := &fakeRuntime{blockUntil: context.Background(), launched: entered}
		first := lifecycleInvoker(t, firstRuntime, &fakeVerifier{}, time.Minute)
		secondRuntime := &fakeRuntime{}
		second := lifecycleInvoker(t, secondRuntime, &fakeVerifier{}, time.Minute).WithObligationStore(first.store)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan worker.ReviewResult, 1)
		go func() {
			done <- first.Invoke(ctx, port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
		}()
		<-entered

		duplicate := second.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
		if !errors.Is(duplicate.Err, reviewobligation.ErrAttemptActive) || duplicate.AttemptID != "" {
			t.Fatalf("duplicate result = %+v, want non-admitted active-attempt refusal", duplicate)
		}
		if secondRuntime.launches != 0 {
			t.Fatalf("duplicate reviewer launches = %d, want zero", secondRuntime.launches)
		}
		cancel()
		result := <-done
		if result.FailureClass != worker.ReviewFailureInfrastructure || result.ObligationStatus != "pending" {
			t.Fatalf("cancelled owner result = %+v", result)
		}

	})
}

func TestInvokeAwaitFailureKillsReviewer(t *testing.T) {
	rt := &fakeRuntime{awaitErr: errors.New("process disappeared")}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "process disappeared") || result.Phase != worker.ReviewPhaseAwait || !rt.killed {
		t.Fatalf("result = %+v killed=%v, want killed await failure", result, rt.killed)
	}
}

func TestInvokeIncompleteReapedRunFailsWithoutKillingReusedPID(t *testing.T) {
	rt := &fakeRuntime{result: port.RunResult{Incomplete: true, Reason: "error_max_turns"}}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "error_max_turns") || result.Phase != worker.ReviewPhaseAwait {
		t.Fatalf("result = %+v, want incomplete await failure", result)
	}
	if rt.killed {
		t.Fatal("Await already reaped the incomplete process; Kill risks signaling a reused PID")
	}
}

func TestInvokePreservesAwaitAndKillErrors(t *testing.T) {
	awaitErr := errors.New("await sentinel")
	killErr := errors.New("kill sentinel")
	rt := &fakeRuntime{awaitErr: awaitErr, killErr: killErr}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if !errors.Is(result.Err, awaitErr) || !errors.Is(result.Err, killErr) {
		t.Fatalf("result error %v does not preserve await and kill causes", result.Err)
	}
}

func TestInvokeFailureCorrelationReflectsOnlyReachedBoundaries(t *testing.T) {
	launchErr := errors.New("spawn rejected")
	rt := &fakeRuntime{launchErr: launchErr}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if !errors.Is(result.Err, launchErr) || result.Phase != worker.ReviewPhaseLaunch {
		t.Fatalf("result = %+v, want launch failure", result)
	}
	if result.ProposalRef != "github:o/r#548" || result.Revision != "abc123" || result.NativeProfileID != "review-maximum" {
		t.Fatalf("launch failure lost reached correlation: %+v", result)
	}
	if result.ArtifactRef != "" {
		t.Fatalf("launch failure asserted unreached artifact %q", result.ArtifactRef)
	}
}

func TestInvokeRefusesHollowProposalWithIdentifiablePhase(t *testing.T) {
	rt := &fakeRuntime{}
	result := lifecycleInvoker(t, rt, &fakeVerifier{}, time.Minute).Invoke(
		context.Background(), port.Task{ID: "T6"}, port.Proposal{Number: 549})
	if result.Err == nil || result.Phase != worker.ReviewPhaseObligation {
		t.Fatalf("result = %+v, want identifiable missing-obligation failure", result)
	}
	if rt.launches != 0 || result.Revision != "" || result.NativeProfileID != "" {
		t.Fatalf("hollow proposal crossed a later boundary: launches=%d result=%+v", rt.launches, result)
	}
}

func TestInvokeRouteFailureDoesNotAssertRoutingDecision(t *testing.T) {
	rt := &fakeRuntime{}
	verifier := &fakeVerifier{}
	i := lifecycleInvoker(t, rt, verifier, time.Minute).WithRouteResolver(
		func(context.Context, port.Task, string, string) (Route, error) {
			return Route{}, errors.New("catalog invalid")
		})
	result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || result.Phase != worker.ReviewPhaseRouting {
		t.Fatalf("result = %+v, want routing failure", result)
	}
	if result.NativeProfileID != "" || result.Model != "" || result.Effort != "" || rt.launches != 0 {
		t.Fatalf("routing failure asserted an unmade route: launches=%d result=%+v", rt.launches, result)
	}
}

func TestNewValidates(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Dependencies)
	}{
		{name: "nil runtime", mutate: func(d *Dependencies) { d.Runtime = nil }},
		{name: "empty backend", mutate: func(d *Dependencies) { d.Harness = "" }},
		{name: "empty repo dir", mutate: func(d *Dependencies) { d.RepositoryDir = "" }},
		{name: "empty repo slug", mutate: func(d *Dependencies) { d.RepositorySlug = "" }},
		{name: "zero budget", mutate: func(d *Dependencies) { d.Budget = 0 }},
		{name: "nil process coordinator", mutate: func(d *Dependencies) { d.Processes = nil }},
		{name: "nil host persistence", mutate: func(d *Dependencies) { d.Persistence = nil }},
		{name: "nil terminal verifier", mutate: func(d *Dependencies) { d.TerminalVerifier = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			persistenceAdapter := &fakeVerifier{}
			terminalVerifier, err := NewStoreTerminalVerifier(store, persistenceAdapter)
			if err != nil {
				t.Fatal(err)
			}
			deps := Dependencies{Runtime: &fakeRuntime{}, Harness: "claude", RepositoryDir: "/r", RepositorySlug: "o/r", Remote: "origin", Budget: time.Minute,
				Processes: &fakeProcessCoordinator{}, Persistence: persistenceAdapter, TerminalVerifier: terminalVerifier, Log: slog.New(slog.DiscardHandler)}
			tc.mutate(&deps)
			if _, err := New(deps); err == nil {
				t.Fatal("New accepted invalid input")
			}
		})
	}
}

func TestPrepareReviewWorkspacePinsRevisionAndCannotMutateParent(t *testing.T) {
	upstream := t.TempDir()
	runGit(t, upstream, "init", "-q", "--initial-branch=main")
	runGit(t, upstream, "config", "user.email", "t@pyxis.invalid")
	runGit(t, upstream, "config", "user.name", "t")
	writeFile(t, filepath.Join(upstream, "subject.txt"), "parent\n")
	runGit(t, upstream, "add", "subject.txt")
	runGit(t, upstream, "commit", "-q", "-m", "seed")
	revision := strings.TrimSpace(string(mustGit(t, upstream, "rev-parse", "HEAD")))
	parent := filepath.Join(t.TempDir(), "parent")
	if out, err := exec.Command("git", "clone", "-q", upstream, parent).CombinedOutput(); err != nil {
		t.Fatalf("clone parent: %v: %s", err, out)
	}
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	persistenceAdapter := &fakeVerifier{}
	terminalVerifier, err := NewStoreTerminalVerifier(store, persistenceAdapter)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := New(Dependencies{Runtime: &fakeRuntime{}, Harness: "claude", RepositoryDir: parent, RepositorySlug: "o/r", Remote: "origin", Budget: time.Minute,
		Processes: &fakeProcessCoordinator{}, Persistence: persistenceAdapter, TerminalVerifier: terminalVerifier, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	workdir, cleanup, err := inv.prepareReviewWorkspace(context.Background(), revision)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got := strings.TrimSpace(string(mustGit(t, workdir, "rev-parse", "HEAD"))); got != revision {
		t.Fatalf("workspace HEAD=%s want %s", got, revision)
	}
	if err := os.WriteFile(filepath.Join(workdir, "subject.txt"), []byte("review mutation\n"), 0o644); err == nil {
		t.Fatal("review workspace accepted a repository mutation")
	}
	parentBytes, err := os.ReadFile(filepath.Join(parent, "subject.txt"))
	if err != nil || string(parentBytes) != "parent\n" {
		t.Fatalf("parent changed: %q %v", parentBytes, err)
	}
	if remotes := strings.TrimSpace(string(mustGit(t, workdir, "remote"))); remotes != "" {
		t.Fatalf("review workspace retained writable remote %q", remotes)
	}
	if _, err := os.Stat(filepath.Join(workdir, ".git", "objects", "info", "alternates")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("review workspace retained a parent-object-store alternate: %v", err)
	}
	if err := verifyReviewWorkspace(context.Background(), workdir, revision); err != nil {
		t.Fatalf("unchanged read-only review workspace failed verification: %v", err)
	}
}

func TestReviewWorkspaceRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "review")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "operator-secret")
	writeFile(t, outside, "do not expose\n")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := makeReviewTreeWritable(root); err != nil {
			t.Error(err)
		}
	}()
	if err := makeReviewTreeReadOnly(root); err == nil || !strings.Contains(err.Error(), "escapes disposable review root") {
		t.Fatalf("symlink escape error = %v", err)
	}
}

func TestReviewWorkspaceAcceptsInternalSymlinkThroughSymlinkedTempRoot(t *testing.T) {
	realParent := t.TempDir()
	realRoot := filepath.Join(realParent, "review")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(realRoot, ".pyxis"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(realRoot, ".pyxis", "skill.md"), "safe\n")
	if err := os.Symlink(".pyxis", filepath.Join(realRoot, ".claude")); err != nil {
		t.Fatal(err)
	}
	aliasParent := t.TempDir()
	aliasRoot := filepath.Join(aliasParent, "review-alias")
	if err := os.Symlink(realRoot, aliasRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := makeReviewTreeWritable(realRoot); err != nil {
			t.Error(err)
		}
	}()
	if err := makeReviewTreeReadOnly(aliasRoot); err != nil {
		t.Fatalf("internal symlink through canonicalized root was rejected: %v", err)
	}
}

func TestReviewWorkspaceAcceptsDanglingInternalSymlink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "review")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("future/generated-file", filepath.Join(root, "generated")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := makeReviewTreeWritable(root); err != nil {
			t.Error(err)
		}
	}()
	if err := makeReviewTreeReadOnly(root); err != nil {
		t.Fatalf("tracked dangling internal symlink was rejected: %v", err)
	}
}

func TestReviewWorkspaceAcceptsAbsoluteDanglingSymlinkThroughAliasedRoot(t *testing.T) {
	realParent := t.TempDir()
	realRoot := filepath.Join(realParent, "review")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	aliasBase := t.TempDir()
	aliasParent := filepath.Join(aliasBase, "parent-alias")
	if err := os.Symlink(realParent, aliasParent); err != nil {
		t.Fatal(err)
	}
	aliasRoot := filepath.Join(aliasParent, "review")
	dangling := filepath.Join(aliasRoot, "future", "generated-file")
	if err := os.Symlink(dangling, filepath.Join(realRoot, "generated")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := makeReviewTreeWritable(realRoot); err != nil {
			t.Error(err)
		}
	}()
	if err := makeReviewTreeReadOnly(aliasRoot); err != nil {
		t.Fatalf("absolute dangling internal symlink through alias was rejected: %v", err)
	}
}

func TestFrozenReviewCharterSurvivesRestartAndChangedCaller(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review.jsonl")
	store, err := reviewobligation.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := lifecycleInvoker(t, &fakeRuntime{}, &fakeVerifier{}, time.Minute).WithObligationStore(store)
	initial.WithProposalInspector(fakeProposalInspector{})
	task := coverageSemanticTask()
	task.Brief = "Frozen charter: preserve issue scope.\nAcceptance: independent review at the captured revision."
	proposal := port.Proposal{Number: 77}
	capture := initial.Capture(t.Context(), task, proposal)
	if capture.Err != nil {
		t.Fatal(capture.Err)
	}
	if err := initial.CompleteHandoff(t.Context(), capture); err != nil {
		t.Fatal(err)
	}
	reopened, err := reviewobligation.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	captured, found, err := reopened.Get(t.Context(), capture.ObligationID)
	if err != nil || !found {
		t.Fatalf("read captured obligation: found=%v err=%v", found, err)
	}
	captured.TaskCharter = "Replacement charter must be rejected."
	if err := reopened.Append(t.Context(), captured); err == nil {
		t.Fatal("same obligation accepted a replacement charter")
	}
	runtime := &fakeRuntime{}
	restarted := lifecycleInvoker(t, runtime, &fakeVerifier{}, time.Minute).WithObligationStore(reopened)
	recovered, selected, err := restarted.PrepareSelected(t.Context(), capture.ObligationID, "restart")
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Brief != task.Brief {
		t.Fatalf("recovery lost frozen charter: %q", recovered.Brief)
	}
	recovered.Brief = "Changed caller text must not replace the captured charter."
	result := restarted.Invoke(t.Context(), recovered, selected)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if !strings.Contains(runtime.spec.Brief, task.Brief) || strings.Contains(runtime.spec.Brief, recovered.Brief) {
		t.Fatalf("review did not use the captured charter: %q", runtime.spec.Brief)
	}
	if !strings.Contains(runtime.spec.Brief, "All required repository checks were green at exactly "+capture.Revision) {
		t.Fatalf("review lacks its exact captured gate verdict: %q", runtime.spec.Brief)
	}
}

func TestPrepareSelectedReadsOnlyPendingExactObligation(t *testing.T) {
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	o := reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: "obligation-1", Repository: "o/r", TaskID: "T7", ProposalRef: "github:o/r#77", Revision: "abc123", Harness: "claude"}
	o.DemandID = "demand-1"
	o.DemandProfile = coverageSemanticTask().DemandProfile
	if err := store.Append(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	inv := lifecycleInvoker(t, &fakeRuntime{}, &fakeVerifier{}, time.Minute).WithObligationStore(store)
	task, proposal, err := inv.PrepareSelected(context.Background(), o.ID, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "T7" || proposal.Number != 77 {
		t.Fatalf("selected task/proposal = %+v %+v", task, proposal)
	}
	if task.DemandID == "" || task.DemandProfile.Capability == "" || task.DemandProfile.Assurance == "" {
		t.Fatalf("persisted obligation demand was lost: %+v", task)
	}
	capture, ok := inv.captureFor(task, proposal)
	if !ok || capture.Revision != "abc123" || capture.ObligationID != o.ID {
		t.Fatalf("selected capture = %+v, %v", capture, ok)
	}
	proof := "review-attempt-proof-1"
	lease, err := store.Begin(context.Background(), reviewobligation.Attempt{SchemaVersion: reviewobligation.AttemptSchema, ObligationID: o.ID, AttemptID: "a-1", RunID: "run-1", AttemptProofRef: proof})
	if err != nil {
		t.Fatal(err)
	}
	pair := reviewobligation.PublicationPair{SchemaVersion: reviewobligation.PublicationPairSchema, ObligationID: o.ID, AttemptID: "a-1",
		PublicationAttemptID: "publication-a-1", ProposalRef: o.ProposalRef, Revision: o.Revision,
		Medium: "github", IdempotencyID: "idempotency-1", ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_test:7",
		AttemptProofRef: proof, EVT13Ref: "evidence:evt13:1", EVT17Ref: "evidence:evt17:1"}
	if err := store.RecordPair(context.Background(), pair); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTerminal(context.Background(), reviewobligation.Terminal{SchemaVersion: reviewobligation.TerminalSchema, ObligationID: o.ID, AttemptID: "a-1", Status: "verified",
		ArtifactRef: pair.ArtifactRef, NativeReceipt: pair.NativeReceipt, AttemptProofRef: pair.AttemptProofRef, EVT13Ref: pair.EVT13Ref, EVT17Ref: pair.EVT17Ref}); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := inv.PrepareSelected(context.Background(), o.ID, "round-2"); err == nil || !strings.Contains(err.Error(), "already verified") {
		t.Fatalf("verified selection error = %v", err)
	}
}

func TestCapturePersistsExactScopeAndRevision(t *testing.T) {
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	persistenceAdapter := &fakeVerifier{}
	terminalVerifier, err := NewStoreTerminalVerifier(store, persistenceAdapter)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := New(Dependencies{Runtime: &fakeRuntime{}, Harness: "claude", RepositoryDir: "/repo", RepositorySlug: "o/r", Remote: "origin", Budget: time.Minute,
		Processes: &fakeProcessCoordinator{}, Persistence: persistenceAdapter, TerminalVerifier: terminalVerifier, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	inv.WithObligationStore(store).WithProposalInspector(fakeProposalInspector{}).WithScope("feature/001")
	task, proposal := port.Task{ID: "T7", Number: 7, DemandID: executiondemand.DemandID(strings.Repeat("a", 64)), DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}}, port.Proposal{Number: 77}
	capture := inv.Capture(context.Background(), task, proposal)
	if capture.Err != nil {
		t.Fatal(capture.Err)
	}
	stored, ok, err := store.Get(context.Background(), capture.ObligationID)
	if err != nil || !ok {
		t.Fatalf("stored obligation ok=%v err=%v", ok, err)
	}
	if stored.Scope != "feature/001" || stored.Revision != "abc123" || stored.TaskID != "T7" || stored.TaskNumber != 7 || stored.ProposalRef != "github:o/r#77" || stored.DemandID != task.DemandID || stored.DemandProfile != task.DemandProfile {
		t.Fatalf("captured obligation = %+v", stored)
	}
	otherDemand := task
	otherDemand.DemandID = executiondemand.DemandID(strings.Repeat("b", 64))
	otherDemand.DemandProfile = executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityEconomy, Deliberation: executiondemand.DeliberationLight, Context: executiondemand.ContextStandard, Assurance: executiondemand.AssuranceOrdinary}
	otherCapture := inv.Capture(context.Background(), otherDemand, proposal)
	if otherCapture.Err != nil {
		t.Fatal(otherCapture.Err)
	}
	if otherCapture.ObligationID == capture.ObligationID {
		t.Fatal("semantic demand change did not change the durable obligation identity")
	}
}

func TestPreparePendingRefusesRetiredDemandWithoutRewritingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.jsonl")
	original := []byte(`{"schema_version":"review-obligation/v1","review_obligation_id":"old","repository":"o/r","task_id":"T1","proposal_ref":"github:o/r#7","revision":"abc123","backend":"claude","rung":3}` + "\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := reviewobligation.NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteHandoff(t.Context(), "old"); err != nil {
		t.Fatal(err)
	}
	inv := lifecycleInvoker(t, &fakeRuntime{}, &fakeVerifier{}, time.Minute).WithObligationStore(store)
	beforeCaptures := len(inv.captures)
	if _, _, err := inv.PreparePending(t.Context(), "old"); err == nil || !strings.Contains(err.Error(), "persisted semantic demand") {
		t.Fatalf("retired review was not refused: %v", err)
	}
	if len(inv.captures) != beforeCaptures {
		t.Fatal("prepared a review capture before demand validation")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("historical obligation was rewritten")
	}
}
