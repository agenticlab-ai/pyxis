package reviewinvoke

import (
	"context"
	"errors"
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

type missingProcessReader interface {
	MissingProcesses(context.Context, implreview.ReviewSubject) ([]implreview.AdmittedProcess, error)
}

// recoverMissingProcesses closes the process-terminal/attempt-terminal gap.
// A canonical missing handoff proves the process ended; neither an absent
// local PID nor a released local flock can supply this cross-clone evidence.
// This repairs only failure, and cannot mint a review or publication pair.
func (i *Invoker) recoverMissingProcesses(ctx context.Context, obligation reviewobligation.Obligation) error {
	reader, ok := i.processes.(missingProcessReader)
	if !ok {
		return nil
	}
	subject := implreview.ReviewSubject{ReviewObligationID: obligation.ID, ProposalID: obligation.ProposalRef, ReviewedRevision: obligation.Revision}
	processes, err := reader.MissingProcesses(ctx, subject)
	if err != nil {
		return fmt.Errorf("reviewinvoke: read missing process terminals: %w", err)
	}
	for _, process := range processes {
		if process.ReviewProcessID == "" || process.Subject != subject {
			return errors.New("reviewinvoke: missing process terminal mismatches obligation")
		}
		_, _, found, err := i.store.Attempt(ctx, obligation.ID, process.ReviewProcessID)
		if err != nil {
			return fmt.Errorf("reviewinvoke: read abandoned attempt: %w", err)
		}
		if !found {
			continue // Process preparation may have failed before attempt admission.
		}
		lease, err := i.store.Resume(ctx, obligation.ID, process.ReviewProcessID)
		if errors.Is(err, reviewobligation.ErrAttemptSettled) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reviewinvoke: fence abandoned attempt: %w", err)
		}
		recordErr := func() error {
			state, found, err := i.store.State(ctx, obligation.ID)
			if err != nil {
				return err
			}
			if !found || state.Obligation.ProposalRef != subject.ProposalID || state.Obligation.Revision != subject.ReviewedRevision || state.Status != "review_pending" || state.Pair != nil {
				return errors.New("reviewinvoke: abandoned attempt acquired publication evidence or changed subject")
			}
			return i.store.AppendTerminal(ctx, reviewobligation.Terminal{SchemaVersion: reviewobligation.TerminalSchema,
				ObligationID: obligation.ID, AttemptID: process.ReviewProcessID, Status: "failed", FailureClass: worker.ReviewFailureInfrastructure})
		}()
		if err := errors.Join(recordErr, lease.Close()); err != nil {
			return fmt.Errorf("reviewinvoke: settle abandoned attempt: %w", err)
		}
		i.log.InfoContext(ctx, "recovered missing impl-review attempt terminal", "task", obligation.TaskID, "issue", obligation.TaskNumber,
			"obligation", obligation.ID, "attempt", process.ReviewProcessID, "proposal", obligation.ProposalRef, "revision", obligation.Revision)
	}
	return nil
}
