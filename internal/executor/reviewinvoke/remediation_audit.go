package reviewinvoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview/protocol"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// This file is the reviewer side of a pre-publish remediation
// (worker.PrePublishReviewer.AuditRemediation). It records the worker's
// disputes through the impl-review protocol, where a disputed blocking finding
// escalates to the operator (prd:impl-review/FR-20 → FR-31), and flags what the
// remediation changed away from the findings it was sent to fix as scope drift
// (prd:impl-review/UB-19). Drift is reported, never blocking: the next round
// still reviews the whole change.

// remediationDriftTolerance is how many lines either side of a finding's line a
// remediation may change and still count as fixing that finding. It is wide
// enough for a fix inside the function around the finding and narrow enough
// that a rewrite elsewhere in the file still shows.
const remediationDriftTolerance = 40

// AuditRemediation records the worker's disputes of review's blocking findings
// and lists the regions the remediation changed in work since review.Revision
// that no finding licenses.
func (i *Invoker) AuditRemediation(ctx context.Context, work port.Workdir, review worker.PrePublishReview, disputes []worker.RemediationDispute) (worker.RemediationAudit, error) {
	var audit worker.RemediationAudit
	if work == nil {
		return audit, errors.New("reviewinvoke: audit remediation with a nil workdir")
	}
	escalation, err := disputeEscalation(review, disputes)
	if err != nil {
		return audit, err
	}
	drift, err := remediationDrift(ctx, work.Dir(), review)
	if err != nil {
		return audit, err
	}
	audit.Escalation, audit.ScopeDrift = escalation, drift
	if len(drift) > 0 {
		i.log.Warn("pre-publish remediation changed code outside its findings",
			slog.String("reviewed_revision", review.Revision), slog.Int("regions", len(drift)), slog.String("scope_drift", strings.Join(drift, ", ")))
	}
	return audit, nil
}

// disputeEscalation records each dispute against its finding with
// protocol.RecordDisposition and, when a disputed finding is blocking, renders
// the decision brief for the operator. A dispute that names no blocking finding
// of the round, repeats one, or gives no reason is refused.
func disputeEscalation(review worker.PrePublishReview, disputes []worker.RemediationDispute) (string, error) {
	byID := make(map[string]worker.PrePublishFinding, len(review.BlockingFindings))
	for _, f := range review.BlockingFindings {
		byID[f.ID] = f
	}
	seen := make(map[string]bool, len(disputes))
	var conflict strings.Builder
	escalated := 0
	for _, d := range disputes {
		f, ok := byID[d.FindingID]
		if !ok {
			return "", fmt.Errorf("reviewinvoke: the worker disputed %s, which is not a blocking finding of the review at %s", d.FindingID, review.Revision)
		}
		if seen[d.FindingID] {
			return "", fmt.Errorf("reviewinvoke: the worker disputed %s more than once", d.FindingID)
		}
		seen[d.FindingID] = true
		finding := implreport.Finding{ID: f.ID, Path: f.Path, Line: f.Line, Message: f.Message, RuleID: f.RuleID,
			CalRow: f.CalRow, Severity: implreport.SeverityBlocking}
		recorded, err := protocol.RecordDisposition(finding, protocol.WorkerDisposition{FindingID: f.ID, Reason: d.Reason})
		if err != nil {
			return "", fmt.Errorf("reviewinvoke: record the worker's dispute of %s: %w", f.ID, err)
		}
		if recorded.Escalation != nil {
			escalated++
			fmt.Fprintf(&conflict, "\n- %s at %s (rule %s, %s): %s\n  the worker's reason: %s",
				f.ID, findingLocation(f), f.RuleID, f.CalRow, strings.TrimSpace(f.Message), strings.TrimSpace(recorded.Disposition.Reason))
		}
	}
	if escalated == 0 {
		return "", nil
	}
	return renderBrief(implreport.EscalationBrief{
		Conflict: fmt.Sprintf("the worker disputes %d blocking finding(s) of the review at %s instead of fixing them:%s", escalated, review.Revision, conflict.String()),
		Options: []string{
			"uphold the finding: the worker makes the change it asks for and the work is reviewed again",
			"withdraw the finding: the worker's reason stands and the work is reviewed again as it is",
			"change the charter or the governing design so the finding and the contract agree",
		},
		DecisionNeeded: "whether each disputed finding stands",
	}), nil
}

// findingLocation renders a finding's location as path or path:line.
func findingLocation(f worker.PrePublishFinding) string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.Path, f.Line)
	}
	return f.Path
}

// changedRegion is one region a remediation changed: its span in the reviewed
// revision, which DetectDrift measures against the findings' lines, and where
// it is at the remediated head, for the report.
type changedRegion struct {
	old   protocol.ChangedSpan
	where string
}

// remediationDrift lists the regions the remediation changed between the
// reviewed revision and the worktree's HEAD that no blocking finding licenses
// (protocol.DetectDrift).
func remediationDrift(ctx context.Context, dir string, review worker.PrePublishReview) ([]string, error) {
	head, err := gitOut(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("reviewinvoke: read remediated head: %w", err)
	}
	if head == review.Revision {
		return nil, nil
	}
	regions, err := changedRegions(ctx, dir, review.Revision, head)
	if err != nil {
		return nil, err
	}
	findings := make([]implreport.Finding, 0, len(review.BlockingFindings))
	for _, f := range review.BlockingFindings {
		findings = append(findings, implreport.Finding{ID: f.ID, Path: f.Path, Line: f.Line})
	}
	defects := protocol.DefectSpansFromFindings(findings, remediationDriftTolerance)
	var drift []string
	for _, r := range regions {
		if protocol.DetectDrift(defects, protocol.SpanSet{r.old}).HasDrift() {
			drift = append(drift, r.where)
		}
	}
	return drift, nil
}

var driftHunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// changedRegions lists every region changed between two revisions. An added or
// deleted file, and a file changed without text hunks (binary, mode), is one
// whole-file region; a modified text file yields one region per hunk.
func changedRegions(ctx context.Context, dir, from, to string) ([]changedRegion, error) {
	out, err := gitOut(ctx, dir, "-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-status", "-z", from, to, "--")
	if err != nil {
		return nil, fmt.Errorf("reviewinvoke: list remediated files: %w", err)
	}
	// -z output is status NUL path NUL pairs; a trailing NUL leaves one empty
	// field, which the pair loop skips.
	fields := strings.Split(out, "\x00")
	var regions []changedRegion
	for i := 0; i+1 < len(fields); i += 2 {
		status, path := fields[i], fields[i+1]
		switch {
		case strings.HasPrefix(status, "A"):
			regions = append(regions, changedRegion{old: protocol.ChangedSpan{Path: path}, where: path + " (new file)"})
		case strings.HasPrefix(status, "D"):
			regions = append(regions, changedRegion{old: protocol.ChangedSpan{Path: path}, where: path + " (deleted)"})
		default:
			hunks, err := fileHunks(ctx, dir, from, to, path)
			if err != nil {
				return nil, err
			}
			if len(hunks) == 0 {
				hunks = []changedRegion{{old: protocol.ChangedSpan{Path: path}, where: path + " (whole file)"}}
			}
			regions = append(regions, hunks...)
		}
	}
	return regions, nil
}

// fileHunks reads one modified file's zero-context hunks.
func fileHunks(ctx context.Context, dir, from, to, path string) ([]changedRegion, error) {
	out, err := gitOut(ctx, dir, "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--unified=0", from, to, "--", ":(literal)"+path)
	if err != nil {
		return nil, fmt.Errorf("reviewinvoke: diff remediated file %s: %w", path, err)
	}
	var regions []changedRegion
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		m := driftHunkHeader.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("reviewinvoke: diff remediated file %s: unparseable hunk header %q", path, line)
		}
		oldStart, oldCount := hunkNumber(m[1], 0), hunkNumber(m[2], 1)
		newStart, newCount := hunkNumber(m[3], 0), hunkNumber(m[4], 1)
		old := protocol.ChangedSpan{Path: path, StartLine: max(oldStart, 1), EndLine: max(oldStart, 1)}
		if oldCount > 0 {
			old.EndLine = oldStart + oldCount - 1
		}
		var where string
		switch newCount {
		case 0:
			where = fmt.Sprintf("%s:%d (%d line(s) removed)", path, max(newStart, 1), oldCount)
		case 1:
			where = fmt.Sprintf("%s:%d", path, newStart)
		default:
			where = fmt.Sprintf("%s:%d-%d", path, newStart, newStart+newCount-1)
		}
		regions = append(regions, changedRegion{old: old, where: where})
	}
	return regions, nil
}

// hunkNumber parses a hunk header number; an omitted count reads as fallback.
func hunkNumber(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}
