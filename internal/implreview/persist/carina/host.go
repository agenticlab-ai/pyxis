package carina

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// HostAdapter is impl-review's sole publisher for the local Carina proposal
// medium. The design-owned record store supplies atomic create-if-absent, so an
// armed replay can safely recover the exact deterministic receipt.
type HostAdapter struct {
	repository   string
	state        publication.Store
	records      ReviewRecordStore
	readAttempts int
	readBackoff  time.Duration
	sleep        func(context.Context, time.Duration) error
}

var _ implreview.HostPersistenceAdapter = (*HostAdapter)(nil)
var _ implreview.HostPublicationRecovery = (*HostAdapter)(nil)
var _ persist.Reader = (*HostAdapter)(nil)

func NewHostAdapter(repository string, state publication.Store, records ReviewRecordStore) (*HostAdapter, error) {
	repository = strings.TrimSpace(repository)
	if repository == "" || state == nil || records == nil {
		return nil, errors.New("carina host persistence: repository, publication state, and record store are required")
	}
	return &HostAdapter{repository: repository, state: state, records: records, readAttempts: 3, readBackoff: 200 * time.Millisecond, sleep: sleepContext}, nil
}

func (a *HostAdapter) Preflight(ctx context.Context, repository string) error {
	if strings.TrimSpace(repository) != a.repository {
		return fmt.Errorf("carina host persistence: repository %q does not match configured %q", repository, a.repository)
	}
	if err := a.records.Preflight(ctx, repository); err != nil {
		return fmt.Errorf("carina host persistence: proposal record store: %w", err)
	}
	if err := a.state.Preflight(ctx); err != nil {
		return fmt.Errorf("carina host persistence: publication state: %w", err)
	}
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
	admission := publication.Admission{Binding: binding, HandoffDigest: handoffDigest, Medium: string(persist.MediumCarina)}
	validAggregate := handoff.Status == implreview.HandoffCompleted && handoff.Aggregate != nil && handoff.Aggregate.Report != nil &&
		handoff.Aggregate.ProcessID == binding.ReviewProcessID && handoff.Aggregate.ProposalID == binding.ProposalID && handoff.Aggregate.ReviewedRevision == binding.ReviewedRevision
	body := ""
	if validAggregate {
		var err error
		body, err = persist.ComposeArtifact(handoff.Aggregate.Report)
		if err != nil {
			return implreview.PublicationTerminal{}, fmt.Errorf("carina host persistence: compose canonical artifact: %w", err)
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

	proposal, err := proposalNumber(binding.ProposalID, a.repository)
	if err != nil {
		return a.fail(ctx, record, implreview.FailureArtifactUntrusted, err.Error())
	}
	if record.Stage == publication.StageAccepted {
		var owner bool
		record, owner, err = a.state.Arm(ctx, record)
		if err != nil {
			return implreview.PublicationTerminal{}, err
		}
		// The local create is itself atomic/idempotent. A CAS loser does not
		// create concurrently; the winner will return the receipt. A later call
		// enters StageArmed and safely reconciles by the same identity below.
		if !owner {
			return implreview.PublicationTerminal{}, errors.New("carina host persistence: publication fence is owned by another host")
		}
	}
	if record.Stage == publication.StageReceiptRecorded {
		return a.verifyReceipt(ctx, record, body)
	}
	if record.Stage != publication.StageArmed {
		return implreview.PublicationTerminal{}, fmt.Errorf("carina host persistence: unexpected state %q", record.Stage)
	}

	nativeReceipt, err := a.records.CreateIfAbsent(ctx, ReviewRecord{
		Repository: a.repository, Proposal: proposal, IdempotencyID: record.IdempotencyID,
		PublicationAttemptID: record.PublicationAttemptID, ReviewObligationID: binding.ReviewObligationID,
		ReviewProcessID: binding.ReviewProcessID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
		ArtifactDigest: admission.ArtifactBodyDigest, ArtifactBody: body,
	})
	if err != nil {
		return a.fail(ctx, record, implreview.FailureInfrastructure, "proposal record create or recovery failed")
	}
	record, err = a.state.RecordReceipt(ctx, record, nativeReceipt)
	if err != nil {
		return implreview.PublicationTerminal{}, err
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
	var state implreview.PublicationRecoveryState
	switch record.Stage {
	case publication.StageArmed:
		state = implreview.PublicationRecoveryArmed
	case publication.StageReceiptRecorded:
		state = implreview.PublicationRecoveryReceiptRecorded
	case publication.StageTerminal:
		state = implreview.PublicationRecoveryTerminal
	default:
		return implreview.PublicationRecovery{}, false, fmt.Errorf("carina host persistence: unexpected recovery state %q", record.Stage)
	}
	return implreview.PublicationRecovery{Binding: record.Binding, State: state}, true, nil
}

func (a *HostAdapter) recoverRecord(ctx context.Context, record publication.Record) (implreview.PublicationTerminal, error) {
	switch record.Stage {
	case publication.StageTerminal:
		if record.Terminal == nil {
			return implreview.PublicationTerminal{}, errors.New("carina host persistence: recovered terminal is empty")
		}
		if record.Terminal.Status == implreview.PublicationVerified {
			return a.verifyReceipt(ctx, record, "")
		}
		return *record.Terminal, nil
	case publication.StageReceiptRecorded:
		return a.verifyReceipt(ctx, record, "")
	case publication.StageArmed:
		proposal, err := proposalNumber(record.Binding.ProposalID, a.repository)
		if err != nil {
			return a.fail(ctx, record, implreview.FailureArtifactUntrusted, err.Error())
		}
		// Carina's receipt is a pure function of the atomic create identity. An
		// armed replay therefore exact-reads that one location: a present record
		// proves the earlier create, while absence is terminalized without issuing
		// another create. This is deliberately different from GitHub, whose POST
		// offers no reusable native idempotency key.
		receipt := receiptFor(a.repository, proposal, record.IdempotencyID)
		if _, err := a.exactReadRecord(ctx, receipt); err != nil {
			var invalid *invalidRecordError
			if errors.As(err, &invalid) {
				return a.fail(ctx, record, implreview.FailureArtifactUnverified, "deterministic native receipt contained invalid review evidence")
			}
			return a.fail(ctx, record, implreview.FailureInfrastructure, "deterministic native receipt was absent after bounded recovery")
		}
		record, err = a.state.RecordReceipt(ctx, record, receipt)
		if err != nil {
			return implreview.PublicationTerminal{}, err
		}
		return a.verifyReceipt(ctx, record, "")
	default:
		return implreview.PublicationTerminal{}, fmt.Errorf("carina host persistence: recovered unexpected state %q", record.Stage)
	}
}

func (a *HostAdapter) ExactRead(ctx context.Context, nativeReceipt string) (*implreport.Report, error) {
	record, err := a.exactReadRecord(ctx, nativeReceipt)
	if err != nil {
		return nil, err
	}
	report, err := persist.ParseArtifact(strings.ReplaceAll(record.ArtifactBody, "\r\n", "\n"))
	if err != nil {
		return nil, fmt.Errorf("carina host persistence: parse exact receipt: %w", err)
	}
	return report, nil
}

func (a *HostAdapter) Read(ctx context.Context, ref persist.Ref) (*implreport.Report, error) {
	if ref.Medium != persist.MediumCarina || strings.TrimSpace(ref.Locator) == "" {
		return nil, errors.New("carina host persistence: invalid harvest reference")
	}
	return a.ExactRead(ctx, ref.Locator)
}

func (a *HostAdapter) verifyReceipt(ctx context.Context, state publication.Record, expectedBody string) (implreview.PublicationTerminal, error) {
	record, err := a.exactReadRecord(ctx, state.NativeReceipt)
	if err != nil {
		if state.Stage == publication.StageTerminal {
			return implreview.PublicationTerminal{}, fmt.Errorf("carina host persistence: re-read durable verified terminal: %w", err)
		}
		var transport *exactReadTransportError
		if errors.As(err, &transport) {
			return a.fail(ctx, state, implreview.FailureInfrastructure, "exact native receipt could not be read within the bounded visibility budget")
		}
		return a.fail(ctx, state, implreview.FailureArtifactUnverified, "exact native receipt did not contain a valid artifact")
	}
	report, err := persist.ParseArtifact(strings.ReplaceAll(record.ArtifactBody, "\r\n", "\n"))
	if err != nil {
		return a.fail(ctx, state, implreview.FailureArtifactUnverified, "exact native receipt did not contain a valid artifact")
	}
	revision, revisionOK := verdict.LatestReviewedRevision(report)
	canonical, composeErr := persist.ComposeArtifact(report)
	if !revisionOK || composeErr != nil || record.Repository != a.repository || record.IdempotencyID != state.IdempotencyID ||
		record.PublicationAttemptID != state.PublicationAttemptID || record.ReviewObligationID != state.Binding.ReviewObligationID ||
		record.ReviewProcessID != state.Binding.ReviewProcessID || record.ProposalID != state.Binding.ProposalID ||
		record.ReviewedRevision != state.Binding.ReviewedRevision || record.ArtifactDigest != state.ArtifactBodyDigest ||
		report == nil || report.ProposalRef != state.Binding.ProposalID || report.ReviewID != state.Binding.ReviewProcessID ||
		revision != state.Binding.ReviewedRevision || (expectedBody != "" && canonical != strings.ReplaceAll(expectedBody, "\r\n", "\n")) || digestBytes([]byte(canonical)) != state.ArtifactBodyDigest {
		if state.Stage == publication.StageTerminal {
			return implreview.PublicationTerminal{}, errors.New("carina host persistence: durable verified terminal no longer matches exact native receipt")
		}
		return a.fail(ctx, state, implreview.FailureArtifactUnverified, "exact native receipt did not match the admitted aggregate")
	}
	terminal := a.terminal(state)
	terminal.Status = implreview.PublicationVerified
	terminal.ArtifactRef = state.ArtifactBodyDigest
	terminal.NativeReceipt = state.NativeReceipt
	terminal.AttemptProofRef = state.Binding.AttemptProofRef
	ref, err := publication.TerminalRef(terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	terminal.EVT13Ref = ref
	finished, err := a.state.Finish(ctx, state, terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	return *finished.Terminal, nil
}

func (a *HostAdapter) exactReadRecord(ctx context.Context, nativeReceipt string) (ReviewRecord, error) {
	attempts := a.readAttempts
	if attempts < 1 {
		attempts = 1
	}
	var (
		record ReviewRecord
		err    error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		record, err = a.records.ExactRead(ctx, nativeReceipt)
		if err == nil {
			return record, nil
		}
		var invalid *invalidRecordError
		if errors.As(err, &invalid) {
			return ReviewRecord{}, err
		}
		if attempt+1 < attempts {
			if sleepErr := a.sleep(ctx, a.readBackoff*time.Duration(1<<attempt)); sleepErr != nil {
				return ReviewRecord{}, &exactReadTransportError{receipt: nativeReceipt, cause: sleepErr}
			}
		}
	}
	return ReviewRecord{}, &exactReadTransportError{receipt: nativeReceipt, cause: err}
}

type exactReadTransportError struct {
	receipt string
	cause   error
}

func (e *exactReadTransportError) Error() string {
	return fmt.Sprintf("carina host persistence: exact-read receipt %s: %v", e.receipt, e.cause)
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

func (a *HostAdapter) fail(ctx context.Context, state publication.Record, class implreview.FailureClass, reason string) (implreview.PublicationTerminal, error) {
	terminal := a.terminal(state)
	terminal.Status, terminal.FailureClass, terminal.Reason = implreview.PublicationFailed, class, strings.TrimSpace(reason)
	if state.NativeReceipt != "" {
		terminal.NativeReceipt = state.NativeReceipt
	}
	ref, err := publication.TerminalRef(terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	terminal.EVT13Ref = ref
	finished, err := a.state.Finish(ctx, state, terminal)
	if err != nil {
		return implreview.PublicationTerminal{}, err
	}
	return *finished.Terminal, nil
}

func (a *HostAdapter) terminal(state publication.Record) implreview.PublicationTerminal {
	binding := state.Binding
	return implreview.PublicationTerminal{ReviewProcessID: binding.ReviewProcessID, ReviewObligationID: binding.ReviewObligationID,
		PublicationAttemptID: state.PublicationAttemptID, ProposalID: binding.ProposalID, ReviewedRevision: binding.ReviewedRevision,
		Medium: string(persist.MediumCarina), IdempotencyID: state.IdempotencyID}
}

func validateBinding(binding implreview.VerifiedPublicationBinding) error {
	b := binding.PublicationBinding
	if b.ReviewObligationID == "" || b.ReviewProcessID == "" || b.ProposalID == "" || b.ReviewedRevision == "" || b.AttemptProofRef == "" || binding.ProofEvidenceRef == "" || b.AttemptProofRef == binding.ProofEvidenceRef {
		return errors.New("carina host persistence: incomplete or aliased verified binding")
	}
	return nil
}

func proposalNumber(proposalID, repository string) (int, error) {
	prefix := string(persist.MediumCarina) + ":" + repository + "#"
	if !strings.HasPrefix(proposalID, prefix) {
		return 0, fmt.Errorf("proposal %q is outside repository %q", proposalID, repository)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(proposalID, prefix))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("proposal %q has invalid number", proposalID)
	}
	return n, nil
}

func digestStrings(parts ...string) string { return digestBytes([]byte(strings.Join(parts, "\x00"))) }

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
