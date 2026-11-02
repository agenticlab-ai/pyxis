package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

type memoryPublicationStore struct {
	mu           sync.Mutex
	record       publication.Record
	preflightErr error
	recoverErr   error
}

func (s *memoryPublicationStore) Preflight(context.Context) error { return s.preflightErr }

func (s *memoryPublicationStore) Admit(_ context.Context, admission publication.Admission) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.PublicationAttemptID != "" {
		if s.record.Admission != admission {
			return publication.Record{}, errors.New("conflicting admission")
		}
		return s.record, nil
	}
	s.record = publication.Record{Admission: admission, PublicationAttemptID: "publication-1", IdempotencyID: "idempotency-1", Stage: publication.StageAccepted}
	return s.record, nil
}

func (s *memoryPublicationStore) Arm(_ context.Context, prior publication.Record) (publication.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.Stage != publication.StageAccepted {
		return s.record, false, nil
	}
	if s.record.IdempotencyID != prior.IdempotencyID {
		return publication.Record{}, false, errors.New("stale arm")
	}
	s.record.Stage, s.record.CreateArmed = publication.StageArmed, true
	return s.record, true, nil
}

func (s *memoryPublicationStore) Recover(_ context.Context, subject implreview.ReviewSubject) (publication.Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recoverErr != nil {
		return publication.Record{}, false, s.recoverErr
	}
	r := s.record
	found := r.CreateArmed && r.Binding.ReviewObligationID == subject.ReviewObligationID && r.Binding.ProposalID == subject.ProposalID && r.Binding.ReviewedRevision == subject.ReviewedRevision
	return r, found, nil
}

func (s *memoryPublicationStore) RecordReceipt(_ context.Context, _ publication.Record, receipt string) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.Stage, s.record.NativeReceipt = publication.StageReceiptRecorded, receipt
	return s.record, nil
}

func (s *memoryPublicationStore) Finish(_ context.Context, _ publication.Record, terminal implreview.PublicationTerminal) (publication.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.Stage == publication.StageTerminal && s.record.Terminal != nil {
		if *s.record.Terminal != terminal {
			return publication.Record{}, errors.New("conflicting terminal")
		}
		return s.record, nil
	}
	s.record.Stage, s.record.Terminal = publication.StageTerminal, &terminal
	if terminal.NativeReceipt != "" {
		s.record.NativeReceipt = terminal.NativeReceipt
	}
	return s.record, nil
}

type hostFakeClient struct {
	mu                sync.Mutex
	comments          map[string]string
	posts             int
	login             string
	node              string
	writable          bool
	postErrAfterStore bool
	crlfRead          bool
	readFailures      int
	readCalls         int
	loginErr          error
	nodeErr           error
	permissionErr     error
}

func (c *hostFakeClient) postComment(_ context.Context, _ int, body string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.posts++
	id := fmt.Sprint(70 + c.posts)
	c.comments[id] = body
	if c.postErrAfterStore {
		return "", errors.New("response lost")
	}
	return id, nil
}

func (c *hostFakeClient) readComment(_ context.Context, id string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readCalls++
	if c.readFailures > 0 {
		c.readFailures--
		return "", errors.New("transient replica miss")
	}
	body, ok := c.comments[id]
	if !ok {
		return "", errors.New("comment missing")
	}
	if c.crlfRead {
		body = strings.ReplaceAll(body, "\n", "\r\n")
	}
	return body, nil
}

func (c *hostFakeClient) currentLogin(context.Context) (string, error) { return c.login, c.loginErr }
func (c *hostFakeClient) repositoryNodeID(context.Context) (string, error) {
	return c.node, c.nodeErr
}
func (c *hostFakeClient) canWriteProposalComments(context.Context) (bool, error) {
	return c.writable, c.permissionErr
}

func hostTestReport(t *testing.T, processID, proposal, revision string) *implreport.Report {
	t.Helper()
	return &implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: proposal, ReviewID: processID,
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: revision, ContextID: "context-1", Findings: []implreport.Finding{}}}}
}

func hostTestBinding() implreview.VerifiedPublicationBinding {
	return implreview.VerifiedPublicationBinding{PublicationBinding: implreview.PublicationBinding{
		ReviewObligationID: "obligation-1", ReviewProcessID: "process-1", ProposalID: "github:o/r#7",
		ReviewedRevision: "abc123", AttemptProofRef: "proof-1",
	}, ProofEvidenceRef: "evidence:attempt:1"}
}

func hostTestHandoff(t *testing.T) implreview.AggregateHandoff {
	t.Helper()
	binding := hostTestBinding()
	return implreview.AggregateHandoff{Status: implreview.HandoffCompleted, DiagnosticDigest: "sha256:aggregate",
		Aggregate: &implreview.CompletedAggregate{ProcessID: binding.ReviewProcessID, ProposalID: binding.ProposalID,
			ReviewedRevision: binding.ReviewedRevision, Report: hostTestReport(t, binding.ReviewProcessID, binding.ProposalID, binding.ReviewedRevision)}}
}

func newHostTestAdapter(t *testing.T, client *hostFakeClient, state *memoryPublicationStore) *HostAdapter {
	t.Helper()
	adapter, err := newHostAdapter("o/r", client, state)
	if err != nil {
		t.Fatal(err)
	}
	adapter.readBackoff = 0
	adapter.sleep = func(context.Context, time.Duration) error { return nil }
	return adapter
}

func TestHostAdapterPreflightRequiresAuthenticatedReadCreateCapability(t *testing.T) {
	cases := []struct {
		name     string
		login    string
		node     string
		writable bool
		wantErr  bool
	}{
		{name: "ready", login: "engineer", node: "R_test", writable: true},
		{name: "missing actor", node: "R_test", writable: true, wantErr: true},
		{name: "read only", login: "engineer", node: "R_test", wantErr: true},
		{name: "missing repository identity", login: "engineer", writable: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &hostFakeClient{comments: map[string]string{}, login: tc.login, node: tc.node, writable: tc.writable}
			err := newHostTestAdapter(t, client, &memoryPublicationStore{}).Preflight(context.Background(), "o/r")
			if (err != nil) != tc.wantErr {
				t.Fatalf("Preflight error = %v, wantErr=%v", err, tc.wantErr)
			}
			if client.posts != 0 {
				t.Fatal("side-effect-free preflight wrote a proposal comment")
			}
		})
	}
}

func TestHostAdapterRecoversVerifiedPublicationWithOriginalProcessAndNoPost(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true}
	state := &memoryPublicationStore{}
	adapter := newHostTestAdapter(t, client, state)
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	recovered, found, err := adapter.RecoverPublication(context.Background(), implreview.ReviewSubject{
		ReviewObligationID: terminal.ReviewObligationID, ProposalID: terminal.ProposalID, ReviewedRevision: terminal.ReviewedRevision,
	})
	if err != nil || !found || recovered.Terminal != terminal || recovered.Binding.ReviewProcessID != "process-1" {
		t.Fatalf("recovered = %+v found=%v err=%v", recovered, found, err)
	}
	if client.posts != 1 {
		t.Fatalf("recovery republished proposal comment: posts=%d", client.posts)
	}
}

func TestHostAdapterPublishesOnceAndExactReadsCRLFReceipt(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true, crlfRead: true}
	state := &memoryPublicationStore{}
	adapter := newHostTestAdapter(t, client, state)
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationVerified || terminal.NativeReceipt != "github-comment:R_test:71" || terminal.AttemptProofRef != "proof-1" || terminal.ArtifactRef == "" || terminal.EVT13Ref == "" {
		t.Fatalf("verified terminal = %+v", terminal)
	}
	read, err := adapter.ExactRead(context.Background(), terminal.NativeReceipt)
	if err != nil || read.ReviewID != "process-1" || read.ProposalRef != "github:o/r#7" {
		t.Fatalf("exact read = %+v, %v", read, err)
	}
	replayed, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil || replayed != terminal || client.posts != 1 {
		t.Fatalf("replay = %+v err=%v posts=%d; want same terminal and one post", replayed, err, client.posts)
	}
}

func TestHostAdapterHarvestReadsLegacyRawCommentLocatorWithoutPublisherPreflight(t *testing.T) {
	handoff := hostTestHandoff(t)
	body, err := persist.ComposeArtifact(handoff.Aggregate.Report)
	if err != nil {
		t.Fatal(err)
	}
	client := &hostFakeClient{comments: map[string]string{"71": body}}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	read, err := adapter.Read(context.Background(), persist.Ref{Medium: persist.MediumGitHub, Locator: "71"})
	if err != nil || read.ReviewID != "process-1" || read.ProposalRef != "github:o/r#7" {
		t.Fatalf("harvest read = %+v, %v", read, err)
	}
}

func TestHostAdapterRetriesOnlyExactReceiptVisibility(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true, readFailures: 2}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationVerified || client.posts != 1 || client.readCalls != 3 {
		t.Fatalf("terminal=%+v posts=%d reads=%d; want one write and bounded visibility retry", terminal, client.posts, client.readCalls)
	}
}

func TestHostAdapterClassifiesExhaustedExactReadAsInfrastructure(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true, readFailures: 3}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationFailed || terminal.FailureClass != implreview.FailureInfrastructure || client.posts != 1 || client.readCalls != 3 {
		t.Fatalf("terminal=%+v posts=%d reads=%d; want infrastructure after bounded exact-read retries", terminal, client.posts, client.readCalls)
	}
}

func TestHostAdapterInvalidHandoffWritesNothing(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), implreview.AggregateHandoff{Status: implreview.HandoffMismatched, DiagnosticDigest: "sha256:mismatch"})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationFailed || terminal.FailureClass != implreview.FailureArtifactUnverified || client.posts != 0 || terminal.EVT13Ref == "" {
		t.Fatalf("invalid handoff terminal=%+v posts=%d", terminal, client.posts)
	}
}

func TestHostAdapterUnknownAcknowledgementNeverRetries(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true, postErrAfterStore: true}
	state := &memoryPublicationStore{}
	adapter := newHostTestAdapter(t, client, state)
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	terminal, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Status != implreview.PublicationFailed || terminal.FailureClass != implreview.FailureInfrastructure || client.posts != 1 {
		t.Fatalf("lost acknowledgement terminal=%+v posts=%d", terminal, client.posts)
	}
	if _, err := adapter.Complete(context.Background(), hostTestBinding(), hostTestHandoff(t)); err != nil {
		t.Fatal(err)
	}
	if client.posts != 1 || len(client.comments) != 1 {
		t.Fatalf("ambiguous publication retried: posts=%d comments=%d", client.posts, len(client.comments))
	}
}

func TestHostAdapterRejectsReceiptSubstitutionWithoutProposalScan(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{"99": "unrelated"}, login: "engineer", node: "R_test", writable: true}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	if err := adapter.Preflight(context.Background(), "o/r"); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.ExactRead(context.Background(), "github-comment:R_other:99"); err == nil {
		t.Fatal("foreign-repository receipt was accepted")
	}
	if _, err := persist.ParseArtifact(client.comments["99"]); err == nil {
		t.Fatal("fixture must remain unrelated to an impl-review artifact")
	}
}

func TestHostAdapterConstructorAndPreflightDependencyFailures(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}, login: "engineer", node: "R_test", writable: true}
	state := &memoryPublicationStore{}
	for name, build := range map[string]func() error{
		"bad repository": func() error { _, err := newHostAdapter("repository", client, state); return err },
		"nil client":     func() error { _, err := newHostAdapter("o/r", nil, state); return err },
		"nil state":      func() error { _, err := newHostAdapter("o/r", client, nil); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := build(); err == nil {
				t.Fatal("invalid adapter dependencies were accepted")
			}
		})
	}

	wantErr := errors.New("dependency unavailable")
	for _, tc := range []struct {
		name       string
		repository string
		client     *hostFakeClient
		state      *memoryPublicationStore
		readOnly   bool
	}{
		{"repository mismatch", "other/repo", client, state, true},
		{"login transport", "o/r", &hostFakeClient{loginErr: wantErr}, state, true},
		{"node transport", "o/r", &hostFakeClient{login: "engineer", nodeErr: wantErr}, state, true},
		{"state preflight", "o/r", client, &memoryPublicationStore{preflightErr: wantErr}, true},
		{"permission transport", "o/r", &hostFakeClient{login: "engineer", node: "R_test", permissionErr: wantErr}, state, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := newHostTestAdapter(t, tc.client, tc.state)
			var err error
			if tc.readOnly {
				err = adapter.PreflightRead(t.Context(), tc.repository)
			} else {
				err = adapter.Preflight(t.Context(), tc.repository)
			}
			if err == nil {
				t.Fatal("dependency failure was accepted")
			}
		})
	}
}

type hostStoreOnly struct{ state *memoryPublicationStore }

func (s hostStoreOnly) Preflight(ctx context.Context) error { return s.state.Preflight(ctx) }
func (s hostStoreOnly) Admit(ctx context.Context, a publication.Admission) (publication.Record, error) {
	return s.state.Admit(ctx, a)
}
func (s hostStoreOnly) Arm(ctx context.Context, r publication.Record) (publication.Record, bool, error) {
	return s.state.Arm(ctx, r)
}
func (s hostStoreOnly) RecordReceipt(ctx context.Context, r publication.Record, receipt string) (publication.Record, error) {
	return s.state.RecordReceipt(ctx, r, receipt)
}
func (s hostStoreOnly) Finish(ctx context.Context, r publication.Record, terminal implreview.PublicationTerminal) (publication.Record, error) {
	return s.state.Finish(ctx, r, terminal)
}

func TestHostAdapterRecoveryInspectionBoundaries(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}}
	adapter, err := newHostAdapter("o/r", client, hostStoreOnly{state: &memoryPublicationStore{}})
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: "o", ProposalID: "github:o/r#1", ReviewedRevision: "rev"}
	if _, found, err := adapter.RecoverPublication(t.Context(), subject); err != nil || found {
		t.Fatalf("unsupported recovery = found:%v err:%v", found, err)
	}
	if _, found, err := adapter.InspectPublication(t.Context(), subject); err != nil || found {
		t.Fatalf("unsupported inspection = found:%v err:%v", found, err)
	}

	wantErr := errors.New("recover failed")
	adapter = newHostTestAdapter(t, client, &memoryPublicationStore{recoverErr: wantErr})
	if _, _, err := adapter.RecoverPublication(t.Context(), subject); !errors.Is(err, wantErr) {
		t.Fatalf("recovery error = %v", err)
	}
	if _, _, err := adapter.InspectPublication(t.Context(), subject); !errors.Is(err, wantErr) {
		t.Fatalf("inspection error = %v", err)
	}

	for stage, want := range map[publication.Stage]implreview.PublicationRecoveryState{
		publication.StageArmed:           implreview.PublicationRecoveryArmed,
		publication.StageReceiptRecorded: implreview.PublicationRecoveryReceiptRecorded,
		publication.StageTerminal:        implreview.PublicationRecoveryTerminal,
	} {
		got, err := publicationRecoveryState(stage)
		if err != nil || got != want {
			t.Errorf("publicationRecoveryState(%q) = %q, %v", stage, got, err)
		}
	}
	if _, err := publicationRecoveryState(publication.StageAccepted); err == nil {
		t.Fatal("accepted stage was treated as recoverable")
	}
}

func TestHostAdapterExactReadAndReceiptFailureBoundaries(t *testing.T) {
	handoff := hostTestHandoff(t)
	body, err := persist.ComposeArtifact(handoff.Aggregate.Report)
	if err != nil {
		t.Fatal(err)
	}

	client := &hostFakeClient{comments: map[string]string{"71": body}}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	adapter.repositoryNode = "R_test"
	adapter.readAttempts = 0
	if _, err := adapter.ExactRead(t.Context(), "github-comment:R_test:71"); err != nil {
		t.Fatalf("single-attempt exact read: %v", err)
	}

	client.readFailures = 1
	adapter.readAttempts = 2
	adapter.sleep = func(context.Context, time.Duration) error { return errors.New("sleep cancelled") }
	if _, err := adapter.ExactRead(t.Context(), "github-comment:R_test:71"); err == nil {
		t.Fatal("retry sleep failure was accepted")
	}

	client.readFailures = 0
	client.comments["72"] = "not an impl-review artifact"
	if _, err := adapter.ExactRead(t.Context(), "github-comment:R_test:72"); err == nil || !strings.Contains(err.Error(), "parse exact receipt") {
		t.Fatalf("malformed exact receipt error = %v", err)
	}

	for _, ref := range []persist.Ref{
		{},
		{Medium: persist.MediumCarina, Locator: "71"},
		{Medium: persist.MediumGitHub, Locator: "not-a-number"},
	} {
		if _, err := adapter.Read(t.Context(), ref); err == nil {
			t.Errorf("Read(%+v) succeeded", ref)
		}
	}

	transportCause := errors.New("transport")
	transport := &exactReadTransportError{commentID: "71", cause: transportCause}
	if !strings.Contains(transport.Error(), "71") || !errors.Is(transport, transportCause) {
		t.Fatalf("transport error = %v", transport)
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := sleepContext(cancelled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sleep = %v", err)
	}
	if err := sleepContext(t.Context(), 0); err != nil {
		t.Fatalf("zero sleep = %v", err)
	}
}

func TestHostReceiptBindingAndProposalGrammar(t *testing.T) {
	client := &hostFakeClient{comments: map[string]string{}}
	adapter := newHostTestAdapter(t, client, &memoryPublicationStore{})
	if _, err := adapter.nativeReceipt("bad"); err == nil {
		t.Fatal("non-numeric comment ID was accepted")
	}
	if _, err := adapter.nativeReceipt("7"); err == nil {
		t.Fatal("receipt before repository preflight was accepted")
	}
	adapter.repositoryNode = "R_test"
	if got, err := adapter.nativeReceipt(" 7 "); err != nil || got != "github-comment:R_test:7" {
		t.Fatalf("native receipt = %q, %v", got, err)
	}
	for _, receipt := range []string{
		"bad",
		"github-comment:",
		"github-comment:R_test:",
		"github-comment:R_test:7:extra",
		"github-comment:R_other:7",
		"github-comment:R_test:bad",
	} {
		if _, err := adapter.parseReceipt(receipt); err == nil {
			t.Errorf("parseReceipt(%q) succeeded", receipt)
		}
	}
	if got, err := adapter.parseReceipt("github-comment:R_test:7"); err != nil || got != "7" {
		t.Fatalf("parseReceipt(valid) = %q, %v", got, err)
	}

	binding := hostTestBinding()
	if err := validateBinding(binding); err != nil {
		t.Fatal(err)
	}
	incomplete := binding
	incomplete.ReviewProcessID = ""
	if err := validateBinding(incomplete); err == nil {
		t.Fatal("incomplete binding was accepted")
	}
	aliased := binding
	aliased.ProofEvidenceRef = aliased.AttemptProofRef
	if err := validateBinding(aliased); err == nil {
		t.Fatal("aliased proof binding was accepted")
	}

	if got, err := proposalNumber("github:o/r#7", "o/r"); err != nil || got != 7 {
		t.Fatalf("proposalNumber(valid) = %d, %v", got, err)
	}
	for _, proposal := range []string{"carina:o/r#7", "github:o/r#0", "github:o/r#bad"} {
		if _, err := proposalNumber(proposal, "o/r"); err == nil {
			t.Errorf("proposalNumber(%q) succeeded", proposal)
		}
	}
	if digestStrings("a", "b") == "" || digestBytes([]byte("a")) == "" {
		t.Fatal("digest helpers returned empty output")
	}
}
