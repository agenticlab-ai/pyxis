package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
)

func (r *issueRun) emitCommand(event report.Event) {
	if r.reporter == nil {
		return
	}
	event.TaskID, event.Issue, event.Scope = r.task.ID, r.task.Number, r.scope()
	r.reporter.Emit(event)
}

func (r *issueRun) logPhase(phase string) func(error) {
	r.phase = phase
	if r.reporter == nil {
		return func(error) {}
	}
	started := r.now()
	r.emitCommand(report.Event{Kind: report.KindCommandPhaseStarted, Time: started, Phase: phase, Status: "running"})
	return func(err error) {
		finished := r.now()
		elapsed := max(int64(0), finished.Sub(started).Milliseconds())
		event := report.Event{Kind: report.KindCommandPhaseFinished, Time: finished, Phase: phase, Status: "completed", ElapsedMS: &elapsed}
		if err != nil {
			event.Status, event.Reason = "failed", err.Error()
		}
		r.emitCommand(event)
	}
}

func (r *issueRun) logCommandFinished(exitCode int) {
	finished := r.now()
	elapsed := max(int64(0), finished.Sub(r.started).Milliseconds())
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	event := report.Event{Kind: report.KindCommandFinished, Time: finished, Status: status, ElapsedMS: &elapsed, ExitCode: &exitCode}
	if c := r.closing; c != nil {
		event.Phase, event.Reason = c.phase, c.result+"; next: "+c.next
	}
	r.emitCommand(event)
}

func (r *issueRun) logCommandStarted(started time.Time) {
	r.emitCommand(report.Event{Kind: report.KindCommandStarted, Time: started, Status: "running"})
}

// issueTerminal is the closing account of a selected-issue command: its
// result and the phase it ended in, the task branch's last checkpoint, any
// proposal, and the concrete next action. Every worker the command started has
// stopped before it reports; active reports a claim another worker still
// holds.
type issueTerminal struct {
	result, phase, checkpoint, next string
	active                          bool
	proposal                        int
}

// close reports the command's terminal once; the command_finished event
// carries its phase and next action into the journal.
func (r *issueRun) close(w io.Writer, t issueTerminal) {
	if r.closing != nil {
		return
	}
	r.closing = &t
	checkpoint, proposal := t.checkpoint, "none"
	if checkpoint == "" {
		checkpoint = "none"
	}
	if t.proposal > 0 {
		proposal = fmt.Sprintf("#%d", t.proposal)
	}
	_, _ = fmt.Fprintf(w, "implement-now: issue #%d terminal: result=%s phase=%s checkpoint=%s worker_active=%t proposal=%s next=%q\n",
		r.task.Number, t.result, t.phase, checkpoint, t.active, proposal, t.next)
}

// closeUnpublished reports an implementation that ended without a proposal.
// The original attempt stays failed. A committed checkpoint whose implementer
// description this command retained can be published by an authorized
// recovery that launches no model; anything else needs a new attempt.
func (r *issueRun) closeUnpublished(ctx context.Context, w io.Writer) {
	head := r.branchHead(ctx)
	r.mu.Lock()
	described := r.described
	r.mu.Unlock()
	next := "rerun the issue command to authorize a new implementation attempt"
	if head != "" && described != "" && r.contains(ctx, described, head) {
		next = "verify and publish this checkpoint without inference: rerun the issue command with --recover-checkpoint " + head
	}
	r.close(w, issueTerminal{result: "no_proposal", phase: "implementation", checkpoint: head, next: next})
}

// closeFailed reports a failure that ended the command before any other
// terminal, in the last phase the command entered.
func (r *issueRun) closeFailed(ctx context.Context, w io.Writer) {
	phase := r.phase
	if phase == "" {
		phase = "setup"
	}
	r.close(w, issueTerminal{result: "failed", phase: phase, checkpoint: r.branchHead(ctx), next: "resolve the error above, then rerun the same command"})
}

// branchHead reads the selected task branch's committed head, or "" when no
// worktree branch exists.
func (r *issueRun) branchHead(ctx context.Context) string {
	if r.repo == "" {
		return ""
	}
	out, err := localGit(ctx, r.repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+dispatch.HeadBranch(r.task)+"^{commit}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// contains reports that revision is ancestor itself or descends from it.
func (r *issueRun) contains(ctx context.Context, ancestor, revision string) bool {
	if ancestor == revision {
		return true
	}
	_, err := localGit(ctx, r.repo, "merge-base", "--is-ancestor", ancestor, revision)
	return err == nil
}
