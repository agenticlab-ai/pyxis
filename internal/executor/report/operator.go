package report

import (
	"context"
	"log/slog"
	"strings"
)

// OperatorSink is the attended sink: a live progress view rendered as one
// structured log line per event via log/slog (§3.9 — the operator's in-session
// view). It carries the run's trace context on every line (AP-5) so a human
// following the run sees each task move dispatched → verified → outcome, and a
// stuck/failed task stands out at Warn.
type OperatorSink struct {
	log *slog.Logger
}

var _ Sink = (*OperatorSink)(nil)

// NewOperatorSink builds an operator sink over the given logger. A nil logger
// falls back to slog.Default, so the sink is never a silent no-op.
func NewOperatorSink(log *slog.Logger) *OperatorSink {
	if log == nil {
		log = slog.Default()
	}
	return &OperatorSink{log: log}
}

// Deliver renders one event as a structured log line. Escalations and failed
// terminals log at Warn (they demand attention); every other event logs at Info
// (steady progress). It never returns an error — slog delivery does not fail — so
// the operator view can never stall the run.
func (s *OperatorSink) Deliver(e Event) error {
	attrs := []any{slog.String("run", e.RunID)}
	if e.ID != "" {
		attrs = append(attrs, slog.String("event_id", e.ID))
	}
	if e.TaskID != "" {
		attrs = append(attrs, slog.String("task", e.TaskID))
	}
	if e.Issue != 0 {
		attrs = append(attrs, slog.Int("issue", e.Issue))
	}

	level := slog.LevelInfo
	msg := string(e.Kind)

	switch e.Kind {
	case KindAgentProgress, KindRecoveryHandoff:
		attrs = append(attrs, slog.String("phase", e.Phase), slog.String("status", e.Status), slog.String("reason", e.Reason))
		if e.ElapsedMS != nil {
			attrs = append(attrs, slog.Int64("elapsed_ms", *e.ElapsedMS))
		}
		if e.RemainingMS != nil {
			attrs = append(attrs, slog.Int64("remaining_ms", *e.RemainingMS))
		}
		if e.StdoutLog != "" {
			attrs = append(attrs, slog.String("stdout_log", e.StdoutLog), slog.String("stderr_log", e.StderrLog))
		}
		if e.SessionID != "" {
			attrs = append(attrs, slog.String("session_id", e.SessionID))
		}
		if e.Usage != nil {
			attrs = append(attrs, slog.Int("tokens", e.Usage.Tokens), slog.Bool("tokens_observed", e.Usage.TokensObserved), slog.Int("turns", e.Usage.Turns), slog.Bool("turns_observed", e.Usage.TurnsObserved))
		}
		if e.Status == "failed" || e.Status == "budget_exhausted" {
			level = slog.LevelWarn
		}
	case KindCommandStarted, KindCommandPhaseStarted, KindCommandPhaseFinished, KindCommandFinished:
		attrs = append(attrs, slog.String("scope", e.Scope))
		if e.Phase != "" {
			attrs = append(attrs, slog.String("phase", e.Phase))
		}
		if e.Status != "" {
			attrs = append(attrs, slog.String("status", e.Status))
		}
		if e.ElapsedMS != nil {
			attrs = append(attrs, slog.Int64("elapsed_ms", *e.ElapsedMS))
		}
		if e.ExitCode != nil {
			attrs = append(attrs, slog.Int("exit_code", *e.ExitCode))
		}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		if e.Status == "failed" {
			level = slog.LevelWarn
		}
	case KindRunStarted:
		msg = "run started"
		if e.Initiator != "" {
			attrs = append(attrs, slog.String("initiator", e.Initiator))
		}
		if e.Scope != "" {
			attrs = append(attrs, slog.String("scope", e.Scope))
		}
	case KindTaskDispatched:
		msg = "task dispatched"
		attrs = append(attrs, slog.String("harness", e.Harness.String()))
		if e.Model != "" {
			attrs = append(attrs, slog.String("model", e.Model), slog.String("effort", e.Effort))
		}
	case KindCIFixAttempt:
		msg = "ci fix attempt"
		attrs = append(attrs, slog.Int("attempt", e.Attempt), slog.Int("cap", e.Cap))
	case KindTaskVerified:
		msg = "task verified"
		attrs = append(attrs, slog.String("verdict", e.Verdict))
	case KindTaskImplementationCompleted:
		msg = "task implementation completed"
		attrs = append(attrs, slog.String("plan", e.Plan), slog.String("revision", e.Revision), slog.String("proposal", e.ProposalRef))
	case KindTaskCompleted:
		msg = "task completed"
		attrs = append(attrs, slog.String("plan", e.Plan), slog.String("revision", e.Revision))
	case KindPlanCompleted:
		msg = "plan completed"
		attrs = append(attrs, slog.String("plan", e.Plan), slog.String("revision", e.Revision))
	case KindLocalGateAttempt:
		msg = "local gate fix attempt"
		attrs = append(attrs, slog.Int("attempt", e.Attempt), slog.Int("cap", e.Cap))
	case KindGateCheckProgress:
		msg = "gate check progress"
		attrs = append(attrs, slog.String("check", e.Phase), slog.String("status", e.Status))
		if e.ElapsedMS != nil {
			attrs = append(attrs, slog.Int64("elapsed_ms", *e.ElapsedMS))
		}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		if e.ArtifactRef != "" {
			attrs = append(attrs, slog.String("diagnostic_log", e.ArtifactRef))
		}
	case KindLocalGateTerminal:
		msg = "local gate finished"
		attrs = append(attrs, slog.String("verdict", e.Verdict), slog.Int("attempts", e.Attempt), slog.Int("cap", e.Cap))
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
			level = slog.LevelWarn
		}
	case KindReviewRound:
		msg = "pre-publish review round"
		attrs = append(attrs, slog.Int("round", e.Round), slog.String("verdict", e.Verdict),
			slog.String("revision", e.Revision), slog.Int("blocking", e.Blocking))
		if e.Reason != "" {
			attrs = append(attrs, slog.String("phase", e.Phase), slog.String("failure_class", e.FailureClass), slog.String("reason", e.Reason))
			level = slog.LevelWarn
		}
	case KindRemediationAttempt:
		msg = "review remediation attempt"
		attrs = append(attrs, slog.Int("attempt", e.Attempt), slog.Int("cap", e.Cap))
	case KindTaskSession:
		msg = "task session"
		attrs = append(attrs, slog.String("action", e.Action), slog.String("harness", e.Harness.String()), slog.String("purpose", e.Purpose))
		if e.SessionID != "" {
			attrs = append(attrs, slog.String("session", e.SessionID))
		}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
			level = slog.LevelWarn
		}
	case KindFeedbackRound:
		msg = "human feedback round"
		attrs = append(attrs, slog.String("proposal", e.ProposalRef), slog.Int("round", e.Round), slog.Int("comments", e.Comments), slog.Bool("changes_requested", e.ChangesRequested))
	case KindPrePublishReviewTerminal:
		msg = "pre-publish review terminal"
		attrs = append(attrs, slog.Int("rounds", e.Round), slog.Int("remediations", e.Remediations), slog.Int("cap", e.Cap),
			slog.String("verdict", e.Verdict), slog.String("revision", e.Revision), slog.Bool("covered", e.Covered))
		if e.ArtifactRef != "" {
			attrs = append(attrs, slog.String("artifact", e.ArtifactRef))
		}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		if len(e.ScopeDrift) > 0 {
			attrs = append(attrs, slog.String("scope_drift", strings.Join(e.ScopeDrift, "; ")))
			level = slog.LevelWarn
		}
		if e.Escalated {
			attrs = append(attrs, slog.Bool("escalated", true))
			level = slog.LevelWarn
		}
	case KindTaskOutcome:
		msg = "task outcome"
		attrs = append(attrs, slog.String("class", e.Class))
		if e.Escalated {
			attrs = append(attrs, slog.Bool("escalated", true))
		}
		if e.Reason != "" {
			attrs = append(attrs, slog.String("reason", e.Reason))
		}
		if IsEscalation(e) {
			level = slog.LevelWarn
		}
	case KindImplReviewTerminal:
		msg = "impl review terminal"
		attrs = append(attrs, slog.String("status", e.Status), slog.String("obligation_status", e.ObligationStatus), slog.String("harness", e.Harness.String()), slog.String("phase", e.Phase))
		if e.Escalated {
			attrs = append(attrs, slog.Bool("escalated", true))
		}
		for _, attr := range []struct {
			key   string
			value string
		}{
			{"drive_round_id", e.DriveRoundID},
			{"review_attempt_id", e.ReviewAttemptID},
			{"review_obligation_id", e.ReviewObligationID},
			{"reviewer_principal", e.ReviewerPrincipal},
			{"proposal", e.ProposalRef},
			{"revision", e.Revision},
			{"native_profile", e.NativeProfile},
			{"catalog", e.Catalog},
			{"policy", e.Policy},
			{"model", e.Model},
			{"effort", e.Effort},
			{"failure_class", e.FailureClass},
			{"artifact", e.ArtifactRef},
			{"artifact_proof", e.ArtifactProofRef},
			{"reason", e.Reason},
		} {
			if attr.value != "" {
				attrs = append(attrs, slog.String(attr.key, attr.value))
			}
		}
		if IsEscalation(e) {
			level = slog.LevelWarn
		}
	case KindHostedReviewPublicationTerminal:
		msg = "hosted review publication terminal"
		attrs = append(attrs,
			slog.String("status", e.Status),
			slog.String("review_attempt_id", e.ReviewAttemptID),
			slog.String("review_obligation_id", e.ReviewObligationID),
			slog.String("proposal", e.ProposalRef),
			slog.String("revision", e.Revision),
		)
		for _, attr := range []struct{ key, value string }{
			{"artifact", e.ArtifactRef}, {"native_receipt", e.NativeReceipt},
			{"attempt_proof", e.AttemptProofRef}, {"failure_class", e.FailureClass}, {"reason", e.Reason},
		} {
			if attr.value != "" {
				attrs = append(attrs, slog.String(attr.key, attr.value))
			}
		}
		if e.Escalated {
			attrs = append(attrs, slog.Bool("escalated", true))
			level = slog.LevelWarn
		}
	case KindReviewArtifactPublicationTerminal:
		msg = "review artifact publication terminal"
		attrs = append(attrs,
			slog.String("status", e.Status),
			slog.String("review_process_id", e.ReviewProcessID),
			slog.String("review_obligation_id", e.ReviewObligationID),
			slog.String("publication_attempt_id", e.PublicationAttemptID),
			slog.String("proposal", e.ProposalID),
			slog.String("revision", e.ReviewedRevision),
			slog.String("medium", e.Medium),
			slog.String("idempotency_id", e.IdempotencyID),
		)
		for _, attr := range []struct{ key, value string }{
			{"artifact", e.ArtifactRef}, {"native_receipt", e.NativeReceipt},
			{"attempt_proof", e.AttemptProofRef}, {"failure_class", e.FailureClass}, {"reason", e.Reason},
		} {
			if attr.value != "" {
				attrs = append(attrs, slog.String(attr.key, attr.value))
			}
		}
	case KindRunFinished:
		msg = "run finished"
		if e.Counts != nil {
			attrs = append(attrs,
				slog.Int("succeeded", e.Counts.Succeeded),
				slog.Int("failed", e.Counts.Failed),
				slog.Int("blocked", e.Counts.Blocked),
				slog.Int("skipped", e.Counts.Skipped),
				slog.Int("awaiting_proposal", e.Counts.AwaitingProposal),
				slog.Int("probe_unavailable", e.Counts.ProbeUnavailable),
				slog.Int("review_completed", e.Counts.ReviewCompleted),
				slog.Int("review_failed", e.Counts.ReviewFailed),
				slog.Int("review_pending", e.Counts.ReviewPending),
				slog.Int("review_artifact_unverified", e.Counts.ReviewArtifactUnverified),
				slog.Int("review_artifact_untrusted", e.Counts.ReviewArtifactUntrusted),
				slog.Int("review_infrastructure_failed", e.Counts.ReviewInfrastructureFailed),
			)
		}
	}

	// The event carries its own time; log it as an attribute rather than relying
	// on the handler's wall clock, so the operator view and the argus journal
	// agree on when each event happened.
	s.log.LogAttrs(context.Background(), level, msg, toLogAttrs(attrs)...)
	return nil
}

// toLogAttrs narrows a []any of slog.Attr values to []slog.Attr for LogAttrs.
// Every element built above is an slog.Attr; a stray non-attr is dropped rather
// than panicking, keeping the sink fail-open.
func toLogAttrs(attrs []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if at, ok := a.(slog.Attr); ok {
			out = append(out, at)
		}
	}
	return out
}
