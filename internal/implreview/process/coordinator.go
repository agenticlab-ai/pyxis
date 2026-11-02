// Package process implements impl-review's trusted outer-process coordinator.
// It owns review-process identity and is the only component that interprets an
// isolated reviewer's terminal handoff.
package process

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// Store is the canonical process admission/terminal evidence boundary.
type Store interface {
	Preflight(context.Context) error
	Prepare(context.Context, implreview.ReviewSubject) (implreview.AdmittedProcess, error)
	Finish(context.Context, implreview.AdmittedProcess, implreview.HandoffStatus, string) error
}

// Coordinator validates process handoffs against the exact admitted subject.
type Coordinator struct {
	store       Store
	calibration *calibration.Table
}

func New(store Store, table *calibration.Table) (*Coordinator, error) {
	if store == nil {
		return nil, errors.New("impl-review process: nil store")
	}
	if table == nil {
		return nil, errors.New("impl-review process: nil calibration")
	}
	return &Coordinator{store: store, calibration: table}, nil
}

func (c *Coordinator) Preflight(ctx context.Context) error {
	return c.store.Preflight(ctx)
}

func (c *Coordinator) Prepare(ctx context.Context, subject implreview.ReviewSubject) (implreview.AdmittedProcess, error) {
	if err := validateSubject(subject); err != nil {
		return implreview.AdmittedProcess{}, err
	}
	return c.store.Prepare(ctx, subject)
}

// MissingProcesses exposes only canonical process terminals, never an inference
// from a released clone-local lock. Stores without this evidence keep their
// existing attempt fence and cannot authorize this recovery.
func (c *Coordinator) MissingProcesses(ctx context.Context, subject implreview.ReviewSubject) ([]implreview.AdmittedProcess, error) {
	if err := validateSubject(subject); err != nil {
		return nil, err
	}
	reader, ok := c.store.(interface {
		MissingProcesses(context.Context, implreview.ReviewSubject) ([]implreview.AdmittedProcess, error)
	})
	if !ok {
		return nil, nil
	}
	return reader.MissingProcesses(ctx, subject)
}

func (c *Coordinator) Complete(ctx context.Context, admitted implreview.AdmittedProcess, handoff implreview.ProcessHandoff) (implreview.AggregateHandoff, error) {
	if err := validateAdmitted(admitted); err != nil {
		return implreview.AggregateHandoff{}, err
	}
	status := handoff.Status
	if status == "" {
		status = implreview.HandoffMissing
	}
	if status != implreview.HandoffCompleted {
		if !validHandoffStatus(status) {
			status = implreview.HandoffMalformed
		}
		digest := diagnosticDigest(status, handoff.DiagnosticDigest)
		if err := c.store.Finish(ctx, admitted, status, digest); err != nil {
			return implreview.AggregateHandoff{}, err
		}
		return implreview.AggregateHandoff{Status: status, DiagnosticDigest: digest}, nil
	}

	body := strings.ReplaceAll(handoff.ArtifactBody, "\r\n", "\n")
	// The handoff is the reviewer's raw final message: its machine block is
	// authoritative and its prose is never published (the persistence adapter
	// recomposes the artifact from the validated payload), so the byte-exact
	// render-fidelity rule applies to published artifacts, not here.
	parsed, err := persist.ParseHandoff(body)
	if err != nil {
		status = implreview.HandoffMalformed
		if errors.Is(err, persist.ErrMultipleMachineBlocks) {
			status = implreview.HandoffSurplus
		}
		return c.finishInvalid(ctx, admitted, status, err)
	}
	if err := implreport.Validate(parsed, c.calibration); err != nil {
		return c.finishInvalid(ctx, admitted, implreview.HandoffMalformed, err)
	}
	if parsed.ProposalRef != admitted.Subject.ProposalID {
		return c.finishInvalid(ctx, admitted, implreview.HandoffMismatched,
			fmt.Errorf("proposal %q does not match admitted %q", parsed.ProposalRef, admitted.Subject.ProposalID))
	}
	revision, ok := verdict.LatestReviewedRevision(parsed)
	if !ok || revision != admitted.Subject.ReviewedRevision {
		return c.finishInvalid(ctx, admitted, implreview.HandoffMismatched,
			fmt.Errorf("reviewed revision %q does not match admitted %q", revision, admitted.Subject.ReviewedRevision))
	}
	if parsed.ReviewID != admitted.ReviewProcessID {
		return c.finishInvalid(ctx, admitted, implreview.HandoffMismatched,
			fmt.Errorf("review process %q does not match admitted %q", parsed.ReviewID, admitted.ReviewProcessID))
	}
	if nativeModel, bound := implreview.NativeModel(ctx); bound {
		model := parsed.Rounds[len(parsed.Rounds)-1].Provenance.Model
		if strings.TrimSpace(nativeModel) == "" || model != nativeModel {
			return c.finishInvalid(ctx, admitted, implreview.HandoffMismatched,
				fmt.Errorf("current-round model %q does not match native launch %q", model, nativeModel))
		}
	}
	payload, err := parsed.Marshal()
	if err != nil {
		return implreview.AggregateHandoff{}, fmt.Errorf("impl-review process: marshal canonical aggregate: %w", err)
	}
	digest := digestBytes(payload)
	if err := c.store.Finish(ctx, admitted, implreview.HandoffCompleted, digest); err != nil {
		return implreview.AggregateHandoff{}, err
	}
	return implreview.AggregateHandoff{Status: implreview.HandoffCompleted, Aggregate: &implreview.CompletedAggregate{
		ProcessID: admitted.ReviewProcessID, ProposalID: admitted.Subject.ProposalID,
		ReviewedRevision: admitted.Subject.ReviewedRevision, Report: parsed,
	}, DiagnosticDigest: digest}, nil
}

func (c *Coordinator) finishInvalid(ctx context.Context, admitted implreview.AdmittedProcess, status implreview.HandoffStatus, cause error) (implreview.AggregateHandoff, error) {
	digest := diagnosticDigest(status, cause.Error())
	if err := c.store.Finish(ctx, admitted, status, digest); err != nil {
		return implreview.AggregateHandoff{}, err
	}
	return implreview.AggregateHandoff{Status: status, DiagnosticDigest: digest}, nil
}

func validateSubject(subject implreview.ReviewSubject) error {
	if strings.TrimSpace(subject.ReviewObligationID) == "" || strings.TrimSpace(subject.ProposalID) == "" || strings.TrimSpace(subject.ReviewedRevision) == "" {
		return errors.New("impl-review process: obligation, proposal, and revision are required")
	}
	return nil
}

func validateAdmitted(admitted implreview.AdmittedProcess) error {
	if strings.TrimSpace(admitted.ReviewProcessID) == "" {
		return errors.New("impl-review process: empty admitted process identity")
	}
	return validateSubject(admitted.Subject)
}

func validHandoffStatus(status implreview.HandoffStatus) bool {
	switch status {
	case implreview.HandoffCompleted, implreview.HandoffMissing, implreview.HandoffSurplus, implreview.HandoffMalformed, implreview.HandoffMismatched:
		return true
	default:
		return false
	}
}

func diagnosticDigest(status implreview.HandoffStatus, detail string) string {
	return digestBytes([]byte("impl-review-process-diagnostic/v1\x00" + string(status) + "\x00" + detail))
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newProcessID(subject implreview.ReviewSubject) (string, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("impl-review process: read identity entropy: %w", err)
	}
	value := subject.ReviewObligationID + "\x00" + subject.ProposalID + "\x00" + subject.ReviewedRevision + "\x00" + string(nonce[:])
	sum := sha256.Sum256([]byte(value))
	return "review-process-" + hex.EncodeToString(sum[:]), nil
}
