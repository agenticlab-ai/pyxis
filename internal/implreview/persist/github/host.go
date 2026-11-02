package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

type hostClient interface {
	postComment(context.Context, int, string) (string, error)
	readComment(context.Context, string) (string, error)
	currentLogin(context.Context) (string, error)
	repositoryNodeID(context.Context) (string, error)
	canWriteProposalComments(context.Context) (bool, error)
}

// HostAdapter is impl-review's sole GitHub proposal publisher. Its state store
// durably arms one POST before the call; an acknowledgement-ambiguous armed
// attempt is failed on replay and is never posted twice.
type HostAdapter struct {
	repository string
	medium     persist.Medium
	gh         hostClient
	state      publication.Store

	mu             sync.Mutex
	repositoryNode string
	readAttempts   int
	readBackoff    time.Duration
	sleep          func(context.Context, time.Duration) error
}

var _ implreview.HostPersistenceAdapter = (*HostAdapter)(nil)
var _ implreview.HostPublicationRecovery = (*HostAdapter)(nil)
var _ persist.Reader = (*HostAdapter)(nil)

func NewHostAdapter(repository string, state publication.Store) (*HostAdapter, error) {
	return newHostAdapter(repository, newGHClient(repository), state)
}

func newHostAdapter(repository string, gh hostClient, state publication.Store) (*HostAdapter, error) {
	repository = strings.TrimSpace(repository)
	if _, _, ok := strings.Cut(repository, "/"); !ok || gh == nil || state == nil {
		return nil, errors.New("github host persistence: repository, client, and state store are required")
	}
	return &HostAdapter{repository: repository, medium: persist.MediumGitHub, gh: gh, state: state, readAttempts: 3, readBackoff: 200 * time.Millisecond, sleep: sleepContext}, nil
}

func (a *HostAdapter) Preflight(ctx context.Context, repository string) error {
	if err := a.PreflightRead(ctx, repository); err != nil {
		return err
	}
	writable, err := a.gh.canWriteProposalComments(ctx)
	if err != nil {
		return fmt.Errorf("github host persistence: inspect proposal-comment permission: %w", err)
	}
	if !writable {
		return errors.New("github host persistence: authenticated actor cannot create proposal comments")
	}
	return nil
}

// PreflightRead prepares exact-receipt lifecycle repair without requiring the
// create permission that repair is forbidden to use.
func (a *HostAdapter) PreflightRead(ctx context.Context, repository string) error {
	if strings.TrimSpace(repository) != a.repository {
		return fmt.Errorf("github host persistence: repository %q does not match configured %q", repository, a.repository)
	}
	login, err := a.gh.currentLogin(ctx)
	if err != nil {
		return fmt.Errorf("github host persistence: authenticate current actor: %w", err)
	}
	if strings.TrimSpace(login) == "" {
		return errors.New("github host persistence: authenticated actor identity is empty")
	}
	node, err := a.gh.repositoryNodeID(ctx)
	if err != nil {
		return fmt.Errorf("github host persistence: read repository node identity: %w", err)
	}
	if strings.TrimSpace(node) == "" {
		return errors.New("github host persistence: repository node identity is empty")
	}
	if err := a.state.Preflight(ctx); err != nil {
		return fmt.Errorf("github host persistence: publication state: %w", err)
	}
	a.mu.Lock()
	a.repositoryNode = strings.TrimSpace(node)
	a.mu.Unlock()
	return nil
}

func (a *HostAdapter) Complete(ctx context.Context, binding implreview.VerifiedPublicationBinding, handoff implreview.AggregateHandoff) (implreview.PublicationTerminal, error) {
	if err := validateBinding(binding); err != nil {
		return implreview.PublicationTerminal{}, err
	}
	handoffDigest := strings.TrimSpace(handoff.DiagnosticDigest)
	if handoffDigest == "" {
		handoffDigest = digestStrings("impl-review-handoff/v1", binding.ReviewObligationID, binding.ReviewProcessID, string(handoff.Status))
	}
	admission := publication.Admission{Binding: binding, HandoffDigest: handoffDigest, Medium: string(a.medium)}

	var body string
	validAggregate := handoff.Status == implreview.HandoffCompleted && handoff.Aggregate != nil && handoff.Aggregate.Report != nil &&
		handoff.Aggregate.ProcessID == binding.ReviewProcessID && handoff.Aggregate.ProposalID == binding.ProposalID && handoff.Aggregate.ReviewedRevision == binding.ReviewedRevision
	if validAggregate {
		var err error
		body, err = persist.ComposeArtifact(handoff.Aggregate.Report)
		if err != nil {
			return implreview.PublicationTerminal{}, fmt.Errorf("github host persistence: compose canonical artifact: %w", err)
		}
		admission.ArtifactBodyDigest = digestBytes([]byte(body))
	}
	record, err := a.state.Admit(ctx, admission)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	if record.Binding != binding {
		return a.recoverRecord(ctx, record)
	}
	if record.Stage == publication.StageTerminal && record.Terminal != nil {
		if record.Terminal.Status == implreview.PublicationVerified {
			return a.verifyReceipt(ctx, record, body)
		}
		return *record.Terminal, nil
	}
	if !validAggregate {
		return a.fail(ctx, record, implreview.FailureArtifactUnverified, "process handoff did not contain the exact completed aggregate")
	}

	switch record.Stage {
	case publication.StageArmed:
		// A prior host died or lost the response after arming. GitHub offers no
		// reusable create idempotency key, so another POST is forbidden.
		return a.fail(ctx, record, implreview.FailureInfrastructure, "publication acknowledgement is indeterminate")
	case publication.StageReceiptRecorded:
		return a.verifyReceipt(ctx, record, body)
	case publication.StageAccepted:
		var owner bool
		record, owner, err = a.state.Arm(ctx, record)
		if err != nil {
			return implreview.PublicationTerminal{}, err
		}
		if !owner {
			return implreview.PublicationTerminal{}, errors.New("github host persistence: publication fence is owned by another host")
		}
	default:
		return implreview.PublicationTerminal{}, fmt.Errorf("github host persistence: unexpected state %q", record.Stage)
	}

	proposal, err := a.proposalNumber(binding.ProposalID)
	if err != nil {
		return a.fail(ctx, record, implreview.FailureArtifactUntrusted, err.Error())
	}
	commentID, err := a.gh.postComment(ctx, proposal, body)
	if err != nil {
		return a.fail(ctx, record, implreview.FailureInfrastructure, "proposal write failed or acknowledgement was lost")
	}
	receipt, err := a.nativeReceipt(commentID)
	if err != nil {
		return a.fail(ctx, record, implreview.FailureInfrastructure, err.Error())
	}
	record, err = a.state.RecordReceipt(ctx, record, receipt)
	if err != nil {
		return implreview.PublicationTerminal{}, fmt.Errorf("github host persistence: persist native receipt: %w", err)
	}
	return a.verifyReceipt(ctx, record, body)
}

func (a *HostAdapter) RecoverPublication(ctx context.Context, subject implreview.ReviewSubject) (implreview.RecoveredPublication, bool, error) {
	recoverer, ok := a.state.(publication.Recoverer)
	if !ok {
		return implreview.RecoveredPublication{}, false, nil
	}
	record, found, err := recoverer.Recover(ctx, subject)
	if err != nil || !found {
		return implreview.RecoveredPublication{}, found, err
	}
	terminal, err := a.recoverRecord(ctx, record)
	return implreview.RecoveredPublication{Binding: record.Binding, Terminal: terminal}, true, err
}

func (a *HostAdapter) InspectPublication(ctx context.Context, subject implreview.ReviewSubject) (implreview.PublicationRecovery, bool, error) {
	recoverer, ok := a.state.(publication.Recoverer)
	if !ok {
		return implreview.PublicationRecovery{}, false, nil
	}
	record, found, err := recoverer.Recover(ctx, subject)
	if err != nil || !found {
		return implreview.PublicationRecovery{}, found, err
	}
	state, err := publicationRecoveryState(record.Stage)
	if err != nil {
		return implreview.PublicationRecovery{}, false, err
	}
	return implreview.PublicationRecovery{Binding: record.Binding, State: state}, true, nil
}

func publicationRecoveryState(stage publication.Stage) (implreview.PublicationRecoveryState, error) {
	switch stage {
	case publication.StageArmed:
		return implreview.PublicationRecoveryArmed, nil
	case publication.StageReceiptRecorded:
		return implreview.PublicationRecoveryReceiptRecorded, nil
	case publication.StageTerminal:
		return implreview.PublicationRecoveryTerminal, nil
	default:
		return "", fmt.Errorf("github host persistence: unexpected recovery state %q", stage)
	}
}

func (a *HostAdapter) recoverRecord(ctx context.Context, record publication.Record) (implreview.PublicationTerminal, error) {
	switch record.Stage {
	case publication.StageTerminal:
		if record.Terminal == nil {
			return implreview.PublicationTerminal{}, errors.New("github host persistence: recovered terminal is empty")
		}
		if record.Terminal.Status == implreview.PublicationVerified {
			return a.verifyReceipt(ctx, record, "")
		}
		return *record.Terminal, nil
	case publication.StageReceiptRecorded:
		return a.verifyReceipt(ctx, record, "")
	case publication.StageArmed:
		return a.fail(ctx, record, implreview.FailureInfrastructure, "publication acknowledgement is indeterminate")
	default:
		return implreview.PublicationTerminal{}, fmt.Errorf("github host persistence: recovered unexpected state %q", record.Stage)
	}
}

func (a *HostAdapter) ExactRead(ctx context.Context, nativeReceipt string) (*implreport.Report, error) {
	commentID, err := a.parseReceipt(nativeReceipt)
	if err != nil {
		return nil, err
	}
	return a.exactReadComment(ctx, commentID)
}

func (a *HostAdapter) exactReadComment(ctx context.Context, commentID string) (*implreport.Report, error) {
	attempts := max(a.readAttempts, 1)
	var (
		body string
		err  error
	)
	for attempt := range attempts {
		body, err = a.gh.readComment(ctx, commentID)
		if err == nil {
			break
		}
		if attempt+1 == attempts {
			return nil, &exactReadTransportError{commentID: commentID, cause: err}
		}
		if sleepErr := a.sleep(ctx, a.readBackoff*time.Duration(1<<attempt)); sleepErr != nil {
			return nil, &exactReadTransportError{commentID: commentID, cause: sleepErr}
		}
	}
	report, err := persist.ParseArtifact(strings.ReplaceAll(body, "\r\n", "\n"))
	if err != nil {
		return nil, fmt.Errorf("github host persistence: parse exact receipt: %w", err)
	}
	return report, nil
}

// Read adapts harvest's opaque medium reference to the authenticated exact-read
// path. It never writes and therefore does not reintroduce the superseded
// reviewer-owned publisher.
func (a *HostAdapter) Read(ctx context.Context, ref persist.Ref) (*implreport.Report, error) {
	if ref.Medium != a.medium || strings.TrimSpace(ref.Locator) == "" {
		return nil, errors.New("github host persistence: invalid harvest reference")
	}
	receipt := ref.Locator
	if strings.HasPrefix(receipt, string(a.medium)+"-comment:") {
		return a.ExactRead(ctx, receipt)
	}
	if a.medium != persist.MediumGitHub {
		return nil, errors.New("host persistence: a repository-qualified receipt is required")
	}
	if _, err := strconv.ParseInt(receipt, 10, 64); err != nil {
		return nil, fmt.Errorf("github host persistence: invalid harvest comment id %q", receipt)
	}
	return a.exactReadComment(ctx, receipt)
}

func (a *HostAdapter) verifyReceipt(ctx context.Context, record publication.Record, expectedBody string) (implreview.PublicationTerminal, error) {
	report, err := a.ExactRead(ctx, record.NativeReceipt)
	if err != nil {
		if record.Stage == publication.StageTerminal {
			return implreview.PublicationTerminal{}, fmt.Errorf("github host persistence: re-read durable verified terminal: %w", err)
		}
		var transport *exactReadTransportError
		if errors.As(err, &transport) {
			return a.fail(ctx, record, implreview.FailureInfrastructure, "exact native receipt could not be read within the bounded visibility budget")
		}
		return a.fail(ctx, record, implreview.FailureArtifactUnverified, "exact native receipt did not contain a valid artifact")
	}
	revision, ok := verdict.LatestReviewedRevision(report)
	canonical, composeErr := persist.ComposeArtifact(report)
	bodyMatches := expectedBody == "" || canonical == strings.ReplaceAll(expectedBody, "\r\n", "\n")
	if !ok || composeErr != nil || report.ProposalRef != record.Binding.ProposalID || report.ReviewID != record.Binding.ReviewProcessID || revision != record.Binding.ReviewedRevision || !bodyMatches || digestBytes([]byte(canonical)) != record.ArtifactBodyDigest {
		if record.Stage == publication.StageTerminal {
			return implreview.PublicationTerminal{}, errors.New("github host persistence: durable verified terminal no longer matches exact native receipt")
		}
		return a.fail(ctx, record, implreview.FailureArtifactUnverified, "exact native receipt did not match the admitted aggregate")
	}
	terminal := a.terminal(record)
	terminal.Status = implreview.PublicationVerified
	terminal.ArtifactRef = digestBytes([]byte(canonical))
	terminal.NativeReceipt = record.NativeReceipt
	terminal.AttemptProofRef = record.Binding.AttemptProofRef
	ref, err := publication.TerminalRef(terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	terminal.EVT13Ref = ref
	finished, err := a.state.Finish(ctx, record, terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	return *finished.Terminal, nil
}

type exactReadTransportError struct {
	commentID string
	cause     error
}

func (e *exactReadTransportError) Error() string {
	return fmt.Sprintf("github host persistence: exact-read comment %s: %v", e.commentID, e.cause)
}

func (e *exactReadTransportError) Unwrap() error { return e.cause }

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *HostAdapter) fail(ctx context.Context, record publication.Record, class implreview.FailureClass, reason string) (implreview.PublicationTerminal, error) {
	terminal := a.terminal(record)
	terminal.Status, terminal.FailureClass, terminal.Reason = implreview.PublicationFailed, class, strings.TrimSpace(reason)
	if record.NativeReceipt != "" {
		terminal.NativeReceipt = record.NativeReceipt
	}
	ref, err := publication.TerminalRef(terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	terminal.EVT13Ref = ref
	finished, err := a.state.Finish(ctx, record, terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	return *finished.Terminal, nil
}

func (a *HostAdapter) terminal(record publication.Record) implreview.PublicationTerminal {
	b := record.Binding
	return implreview.PublicationTerminal{ReviewProcessID: b.ReviewProcessID, ReviewObligationID: b.ReviewObligationID,
		PublicationAttemptID: record.PublicationAttemptID, ProposalID: b.ProposalID, ReviewedRevision: b.ReviewedRevision,
		Medium: string(a.medium), IdempotencyID: record.IdempotencyID}
}

func (a *HostAdapter) nativeReceipt(commentID string) (string, error) {
	commentID = strings.TrimSpace(commentID)
	if _, err := strconv.ParseInt(commentID, 10, 64); err != nil {
		return "", fmt.Errorf("github host persistence: invalid comment database id %q", commentID)
	}
	a.mu.Lock()
	node := a.repositoryNode
	a.mu.Unlock()
	if node == "" {
		return "", errors.New("github host persistence: repository identity was not preflighted")
	}
	return string(a.medium) + "-comment:" + node + ":" + commentID, nil
}

func (a *HostAdapter) parseReceipt(receipt string) (string, error) {
	prefix := string(a.medium) + "-comment:"
	if !strings.HasPrefix(receipt, prefix) {
		return "", errors.New("github host persistence: invalid native receipt grammar")
	}
	rest := strings.TrimPrefix(receipt, prefix)
	node, commentID, ok := strings.Cut(rest, ":")
	if !ok || node == "" || commentID == "" || strings.Contains(commentID, ":") {
		return "", errors.New("github host persistence: invalid native receipt components")
	}
	a.mu.Lock()
	wantNode := a.repositoryNode
	a.mu.Unlock()
	if wantNode == "" || node != wantNode {
		return "", errors.New("github host persistence: native receipt repository mismatch")
	}
	if _, err := strconv.ParseInt(commentID, 10, 64); err != nil {
		return "", errors.New("github host persistence: invalid native receipt comment id")
	}
	return commentID, nil
}

func validateBinding(binding implreview.VerifiedPublicationBinding) error {
	b := binding.PublicationBinding
	if b.ReviewObligationID == "" || b.ReviewProcessID == "" || b.ProposalID == "" || b.ReviewedRevision == "" || b.AttemptProofRef == "" || binding.ProofEvidenceRef == "" {
		return errors.New("github host persistence: incomplete verified binding")
	}
	if b.AttemptProofRef == binding.ProofEvidenceRef {
		return errors.New("github host persistence: attempt proof aliases evidence reference")
	}
	return nil
}

func proposalNumber(proposalID, repository string) (int, error) {
	prefix := string(persist.MediumGitHub) + ":" + repository + "#"
	if !strings.HasPrefix(proposalID, prefix) {
		return 0, fmt.Errorf("proposal %q is outside repository %q", proposalID, repository)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(proposalID, prefix))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("proposal %q has invalid number", proposalID)
	}
	return n, nil
}

func (a *HostAdapter) proposalNumber(proposalID string) (int, error) {
	if a.medium == persist.MediumGitHub {
		return proposalNumber(proposalID, a.repository)
	}
	prefix := a.repository + "#"
	if !strings.HasPrefix(proposalID, prefix) {
		return 0, errors.New("host persistence: proposal belongs to another repository")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(proposalID, prefix))
	if err != nil || n <= 0 {
		return 0, errors.New("host persistence: invalid proposal number")
	}
	return n, nil
}

func digestStrings(parts ...string) string { return digestBytes([]byte(strings.Join(parts, "\x00"))) }

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
