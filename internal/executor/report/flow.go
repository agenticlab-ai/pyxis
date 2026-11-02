package report

import (
	"fmt"
	"io"
	"sync"
)

// FlowSink renders the executor event stream as a small, human-readable
// timeline. The JSONL journal remains available for machines; this sink is the
// log an operator can read from top to bottom.
type FlowSink struct {
	mu sync.Mutex
	w  io.Writer
}

func NewFlowSink(w io.Writer) *FlowSink { return &FlowSink{w: w} }

func (s *FlowSink) Deliver(e Event) error {
	if s == nil || s.w == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	message := flowMessage(e)
	if message == "" {
		return nil
	}
	_, err := fmt.Fprintf(s.w, "%s | %s\n", e.Time.UTC().Format("2006-01-02T15:04:05Z"), message)
	return err
}

func flowMessage(e Event) string {
	task := ""
	if e.TaskID != "" {
		task = "task " + e.TaskID + ": "
	}
	switch e.Kind {
	case KindCommandStarted:
		return fmt.Sprintf("command %s started (scope %s)", e.RunID, value(e.Scope))
	case KindCommandPhaseStarted:
		return fmt.Sprintf("command %s: %s started", e.RunID, value(e.Phase))
	case KindCommandPhaseFinished:
		if e.ElapsedMS != nil {
			return fmt.Sprintf("command %s: %s %s after %d ms", e.RunID, value(e.Phase), value(e.Status), *e.ElapsedMS)
		}
		return fmt.Sprintf("command %s: %s %s", e.RunID, value(e.Phase), value(e.Status))
	case KindCommandFinished:
		if e.ElapsedMS != nil && e.ExitCode != nil {
			return fmt.Sprintf("command %s finished with exit %d after %d ms", e.RunID, *e.ExitCode, *e.ElapsedMS)
		}
		return fmt.Sprintf("command %s finished", e.RunID)
	case KindRunStarted:
		return fmt.Sprintf("run %s started (%s, scope %s)", e.RunID, value(e.Initiator), value(e.Scope))
	case KindTaskDispatched:
		return fmt.Sprintf("%simplementation started with %s", task, value(e.Harness.String()))
	case KindTaskSession:
		return fmt.Sprintf("%s%s session %s", task, value(e.Purpose), value(e.Action))
	case KindLocalGateAttempt:
		return fmt.Sprintf("%slocal CI failed; recovery attempt %d/%d started", task, e.Attempt, e.Cap)
	case KindGateCheckProgress:
		return fmt.Sprintf("%scheck %s %s", task, value(e.Phase), value(e.Status))
	case KindLocalGateTerminal:
		if e.Verdict == VerdictGreen {
			return fmt.Sprintf("%slocal CI is green after %d recovery attempt(s)", task, e.Attempt)
		}
		return fmt.Sprintf("%slocal CI failed after %d/%d recovery attempt(s): %s", task, e.Attempt, e.Cap, value(e.Reason))
	case KindReviewRound:
		return fmt.Sprintf("%slocal CI green; review round %d finished: %s (%d blocking)", task, e.Round, value(e.Verdict), e.Blocking)
	case KindRemediationAttempt:
		return fmt.Sprintf("%sreview recovery %d/%d started", task, e.Attempt, e.Cap)
	case KindPrePublishReviewTerminal:
		if len(e.ScopeDrift) > 0 {
			return fmt.Sprintf("%sreview finished: %s after %d round(s); fixes changed %d region(s) outside their findings", task, value(e.Verdict), e.Round, len(e.ScopeDrift))
		}
		return fmt.Sprintf("%sreview finished: %s after %d round(s)", task, value(e.Verdict), e.Round)
	case KindCIFixAttempt:
		return fmt.Sprintf("%sremote CI failed; recovery attempt %d/%d started", task, e.Attempt, e.Cap)
	case KindTaskVerified:
		switch e.Verdict {
		case VerdictGreen:
			return fmt.Sprintf("%sproposal remote CI is green", task)
		case VerdictRed:
			return fmt.Sprintf("%sproposal remote CI is red", task)
		default:
			return fmt.Sprintf("%sverification finished: %s", task, value(e.Verdict))
		}
	case KindTaskImplementationCompleted:
		return fmt.Sprintf("%simplementation verified; awaiting merge", task)
	case KindTaskCompleted:
		return fmt.Sprintf("%sdelivery verified", task)
	case KindPlanCompleted:
		return fmt.Sprintf("plan %s completed; parent status will be re-evaluated", value(e.Plan))
	case KindTaskOutcome:
		if e.Reason != "" {
			return fmt.Sprintf("%sfinished %s: %s", task, value(e.Class), e.Reason)
		}
		return fmt.Sprintf("%sfinished %s", task, value(e.Class))
	case KindImplReviewTerminal:
		return fmt.Sprintf("%sconfigured review %s (%s)", task, value(e.Status), value(e.ObligationStatus))
	case KindFeedbackRound:
		return fmt.Sprintf("%shuman feedback round %d started (%d comment(s))", task, e.Round, e.Comments)
	case KindRunFinished:
		if e.Counts == nil {
			return fmt.Sprintf("run %s finished", e.RunID)
		}
		return fmt.Sprintf("run %s finished: %d succeeded, %d failed, %d blocked, %d review pending", e.RunID, e.Counts.Succeeded, e.Counts.Failed, e.Counts.Blocked, e.Counts.ReviewPending)
	}
	return ""
}

func value(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
