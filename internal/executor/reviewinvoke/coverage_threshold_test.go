package reviewinvoke

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type coverageReviewStore struct {
	reviewobligation.Store
	state        reviewobligation.State
	stateFound   bool
	stateErr     error
	preflightErr error
	appendErr    error
	handoffErr   error
	terminalErr  error
}

func (s *coverageReviewStore) Preflight(context.Context) error { return s.preflightErr }

func (s *coverageReviewStore) Append(context.Context, reviewobligation.Obligation) error {
	return s.appendErr
}

func (s *coverageReviewStore) State(context.Context, string) (reviewobligation.State, bool, error) {
	return s.state, s.stateFound, s.stateErr
}

func (s *coverageReviewStore) CompleteHandoff(context.Context, string) error { return s.handoffErr }

func (s *coverageReviewStore) AppendTerminal(context.Context, reviewobligation.Terminal) error {
	return s.terminalErr
}

type coverageInspector struct {
	landing  port.Landing
	revision string
	checks   port.CheckState
	err      error
}

type coverageExactReader struct {
	implreview.HostPersistenceAdapter
	report *implreport.Report
	err    error
}

func (r coverageExactReader) ExactRead(context.Context, string) (*implreport.Report, error) {
	return r.report, r.err
}

func (i coverageInspector) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return i.landing, i.revision, i.checks, i.err
}

func coverageSemanticTask() port.Task {
	return port.Task{
		ID:     "T-coverage",
		Number: 7,

		DemandID:      executiondemand.DemandID(strings.Repeat("a", 64)),
		DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical},
	}
}

func coverageInvoker(store reviewobligation.Store) *Invoker {
	return &Invoker{
		runtime:         &fakeRuntime{},
		harness:         "claude",
		repoSlug:        "o/r",
		scope:           "feature/coverage",
		budget:          time.Minute,
		processes:       &fakeProcessCoordinator{},
		persistence:     &fakeVerifier{},
		inspect:         fakeProposalInspector{},
		store:           store,
		now:             time.Now,
		captures:        make(map[string]worker.ReviewCapture),
		log:             slog.New(slog.DiscardHandler),
		prepare:         func(context.Context, string) (string, func(), error) { return "", func() {}, nil },
		verifyWorkspace: func(context.Context, string, string) error { return nil },
	}
}

func TestPreflightRejectsUnavailableBoundaries(t *testing.T) {
	t.Run("process coordinator", func(t *testing.T) {
		inv := coverageInvoker(&coverageReviewStore{})
		inv.processes = &fakeProcessCoordinator{preflightErr: errors.New("coordinator unavailable")}
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "coordinator unavailable") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
	t.Run("proposal inspector", func(t *testing.T) {
		inv := coverageInvoker(&coverageReviewStore{})
		inv.inspect = nil
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "inspector") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
	t.Run("store construction", func(t *testing.T) {
		inv := coverageInvoker(nil)
		inv.storeInitErr = errors.New("git common dir unavailable")
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "git common dir unavailable") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
	t.Run("store missing", func(t *testing.T) {
		inv := coverageInvoker(nil)
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "store is unavailable") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
	t.Run("store preflight", func(t *testing.T) {
		inv := coverageInvoker(&coverageReviewStore{preflightErr: errors.New("store unavailable")})
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "store unavailable") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
	t.Run("native catalog capability", func(t *testing.T) {
		inv := coverageInvoker(&coverageReviewStore{})
		if err := inv.Preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "does not implement role-qualified catalog") {
			t.Fatalf("Preflight error = %v", err)
		}
	})
}

func TestCatalogAuthorizationAndSemanticDemandRejectIncompleteInputs(t *testing.T) {
	for _, proof := range []struct {
		kind       string
		repository string
		operator   string
	}{
		{kind: "wrong", repository: "o/r", operator: "operator"},
		{repository: "o/r", operator: "operator"},
		{kind: string("repository_operator"), operator: "operator"},
	} {
		if err := (nativerouting.ReleasedCatalogAuthorization{}).Authorize(nativeroutingAuthorizationProof(proof.kind, proof.repository, proof.operator)); err == nil {
			t.Fatalf("authorization accepted %+v", proof)
		}
	}
	if _, err := requestedReviewProfile(port.Task{ID: "missing-demand"}); err == nil {
		t.Fatal("task without persisted semantic demand was accepted")
	}
}

func nativeroutingAuthorizationProof(kind, repository, operator string) nativerouting.AuthorizationProof {
	return nativerouting.AuthorizationProof{Kind: nativerouting.AuthorizationKind(kind), Repository: repository, OperatorID: operator}
}

func TestCaptureReportsEachFailClosedBoundary(t *testing.T) {
	validInspector := coverageInspector{landing: port.LandingOpen, revision: "abc123", checks: port.CheckGreen}
	validTask := coverageSemanticTask()
	proposal := port.Proposal{Number: 7}

	cases := []struct {
		name   string
		mutate func(*Invoker, *port.Task, *port.Proposal)
		want   string
	}{
		{name: "semantic demand", mutate: func(_ *Invoker, task *port.Task, _ *port.Proposal) { task.DemandID = "" }, want: "persisted semantic demand"},
		{name: "proposal identity", mutate: func(_ *Invoker, _ *port.Task, proposal *port.Proposal) { proposal.Number = 0 }, want: "invalid GitHub proposal identity"},
		{name: "inspector missing", mutate: func(inv *Invoker, _ *port.Task, _ *port.Proposal) { inv.inspect = nil }, want: "inspector unavailable"},
		{name: "inspector error", mutate: func(inv *Invoker, _ *port.Task, _ *port.Proposal) {
			inv.inspect = coverageInspector{err: errors.New("host unavailable")}
		}, want: "host unavailable"},
		{name: "not green", mutate: func(inv *Invoker, _ *port.Task, _ *port.Proposal) {
			inv.inspect = coverageInspector{landing: port.LandingOpen, revision: "abc123", checks: port.CheckPending}
		}, want: "exact-green capture refused"},
		{name: "store missing", mutate: func(inv *Invoker, _ *port.Task, _ *port.Proposal) { inv.store = nil }, want: "store unavailable"},
		{name: "append failure", mutate: func(inv *Invoker, _ *port.Task, _ *port.Proposal) {
			inv.store = &coverageReviewStore{appendErr: errors.New("append unavailable")}
		}, want: "append unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := coverageInvoker(&coverageReviewStore{})
			inv.inspect = validInspector
			task, candidate := validTask, proposal
			tc.mutate(inv, &task, &candidate)
			capture := inv.Capture(context.Background(), task, candidate)
			if capture.Err == nil || !strings.Contains(capture.Err.Error(), tc.want) {
				t.Fatalf("Capture = %+v, want error containing %q", capture, tc.want)
			}
		})
	}
}

func TestHandoffAndTerminalRejectIncompleteDurability(t *testing.T) {
	validCapture := worker.ReviewCapture{ProposalRef: "github:o/r#7", Revision: "abc123", ObligationID: "o-1", Harness: "claude", Persisted: true}
	inv := coverageInvoker(nil)
	if err := inv.CompleteHandoff(context.Background(), worker.ReviewCapture{}); err == nil {
		t.Fatal("invalid capture handoff succeeded")
	}
	if err := inv.CompleteHandoff(context.Background(), validCapture); err == nil {
		t.Fatal("handoff without a store succeeded")
	}
	inv.store = &coverageReviewStore{handoffErr: errors.New("handoff unavailable")}
	if err := inv.CompleteHandoff(context.Background(), validCapture); err == nil || !strings.Contains(err.Error(), "handoff unavailable") {
		t.Fatalf("CompleteHandoff error = %v", err)
	}

	inv.store = nil
	if err := inv.RecordTerminal(context.Background(), worker.ReviewResult{}); err == nil {
		t.Fatal("terminal without store succeeded")
	}
	inv.store = &coverageReviewStore{}
	if err := inv.RecordTerminal(context.Background(), worker.ReviewResult{}); err == nil {
		t.Fatal("identity-free terminal succeeded")
	}
	inv.store = &coverageReviewStore{terminalErr: errors.New("terminal unavailable")}
	if err := inv.RecordTerminal(context.Background(), worker.ReviewResult{AttemptID: "a-1", ObligationID: "o-1", Status: "failed", FailureClass: "infrastructure"}); err == nil || !strings.Contains(err.Error(), "terminal unavailable") {
		t.Fatalf("RecordTerminal error = %v", err)
	}
}

func TestPreparePendingRejectsInvalidSelectedState(t *testing.T) {
	profile := executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}
	o := reviewobligation.Obligation{
		SchemaVersion: reviewobligation.Schema,
		ID:            "o-pending",
		Repository:    "o/r",
		Scope:         "feature/coverage",
		TaskID:        "T-pending",
		ProposalRef:   "github:o/r#7",
		Revision:      "abc123",
		Harness:       "claude",
		DemandID:      executiondemand.DemandID(strings.Repeat("b", 64)),
		DemandProfile: profile,
	}
	validState := reviewobligation.State{Obligation: o, Status: "review_pending"}

	inv := coverageInvoker(nil)
	if _, _, err := inv.PrepareSelected(context.Background(), o.ID, "round-1"); err == nil {
		t.Fatal("selection without store succeeded")
	}
	inv.store = &coverageReviewStore{state: validState, stateFound: true}
	if _, _, err := inv.PrepareSelected(context.Background(), o.ID, " "); err == nil {
		t.Fatal("selection without drive round succeeded")
	}

	cases := []struct {
		name  string
		state reviewobligation.State
		found bool
		err   error
	}{
		{name: "state read", state: validState, found: true, err: errors.New("state unavailable")},
		{name: "not found"},
		{name: "verified", state: func() reviewobligation.State { s := validState; s.Status = "verified"; return s }(), found: true},
		{name: "scope", state: func() reviewobligation.State { s := validState; s.Obligation.Scope = "feature/other"; return s }(), found: true},
		{name: "not pending", state: func() reviewobligation.State { s := validState; s.Status = "handoff_pending"; return s }(), found: true},
		{name: "proposal separator", state: func() reviewobligation.State { s := validState; s.Obligation.ProposalRef = "github:o/r"; return s }(), found: true},
		{name: "proposal number", state: func() reviewobligation.State { s := validState; s.Obligation.ProposalRef = "github:o/r#bad"; return s }(), found: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := coverageInvoker(&coverageReviewStore{state: tc.state, stateFound: tc.found, stateErr: tc.err})
			if _, _, err := inv.PreparePending(context.Background(), o.ID); err == nil {
				t.Fatal("invalid selected state was accepted")
			}
		})
	}
}

func TestWorkspaceVerificationRejectsMissingMovedAndDirtyTrees(t *testing.T) {
	if err := makeReviewTreeReadOnly(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing review tree was accepted")
	}
	if err := makeReviewTreeWritable(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing review tree was made writable")
	}
	if err := verifyReviewWorkspace(context.Background(), filepath.Join(t.TempDir(), "missing"), "abc123"); err == nil {
		t.Fatal("missing Git workspace was accepted")
	}

	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	writeFile(t, filepath.Join(repo, "subject.txt"), "clean\n")
	runGit(t, repo, "add", "subject.txt")
	runGit(t, repo, "commit", "-qm", "seed")
	revision := strings.TrimSpace(string(mustGit(t, repo, "rev-parse", "HEAD")))
	if err := verifyReviewWorkspace(context.Background(), repo, "different"); err == nil {
		t.Fatal("moved review head was accepted")
	}
	writeFile(t, filepath.Join(repo, "subject.txt"), "dirty\n")
	if err := verifyReviewWorkspace(context.Background(), repo, revision); err == nil {
		t.Fatal("dirty review tree was accepted")
	}
}

func TestConstructorNativeRouteAndHostedTerminalBoundaries(t *testing.T) {
	persistence := &fakeVerifier{}
	terminalVerifier, err := NewStoreTerminalVerifier(&coverageReviewStore{}, persistence)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Dependencies{
		Runtime:          &fakeRuntime{},
		Harness:          "claude",
		RepositoryDir:    t.TempDir(),
		RepositorySlug:   "o/r",
		Budget:           time.Minute,
		Processes:        &fakeProcessCoordinator{},
		Persistence:      persistence,
		TerminalVerifier: terminalVerifier,
	}); err != nil {
		t.Fatalf("New with default logger: %v", err)
	}

	inv := coverageInvoker(&coverageReviewStore{})
	if _, err := inv.ResolveRoute(context.Background(), coverageSemanticTask(), "github:o/r#7", "abc123"); err == nil {
		t.Fatal("runtime without native review catalog resolved a route")
	}

	obligation := reviewobligation.Obligation{ID: "o-hosted", ProposalRef: "github:o/r#7", Revision: "abc123"}
	binding := implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: obligation.ID,
		ReviewProcessID:    "process-hosted",
		ProposalID:         obligation.ProposalRef,
		ReviewedRevision:   obligation.Revision,
		AttemptProofRef:    "proof-hosted",
	}}
	publication := implreview.PublicationTerminal{Status: implreview.PublicationVerified, ReviewProcessID: binding.ReviewProcessID}
	if _, err := inv.hostedTerminalFromEvidence(context.Background(), obligation, binding, publication); err == nil {
		t.Fatal("verified publication without receipt was accepted")
	}
	publication.NativeReceipt = "receipt-hosted"
	inv.persistence = coverageExactReader{}
	if _, err := inv.hostedTerminalFromEvidence(context.Background(), obligation, binding, publication); err == nil {
		t.Fatal("nil exact-read report was accepted")
	}
	inv.persistence = coverageExactReader{report: hostTestReportForRecovery("wrong-process", obligation.ProposalRef, obligation.Revision)}
	if _, err := inv.hostedTerminalFromEvidence(context.Background(), obligation, binding, publication); err == nil {
		t.Fatal("receipt bound to another process was accepted")
	}
	publication.Status = implreview.PublicationStatus("unknown")
	if _, err := inv.hostedTerminalFromEvidence(context.Background(), obligation, binding, publication); err == nil {
		t.Fatal("unknown publication status was accepted")
	}
}
