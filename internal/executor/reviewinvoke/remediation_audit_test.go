package reviewinvoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview/protocol"
)

// remediationRepo commits a reviewed revision (a.go and c.go of 200 numbered
// lines, gone.go), then a remediation on top of it, and returns the repository
// and the reviewed sha.
func remediationRepo(t *testing.T, remediate func(dir string)) (dir, reviewed string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	var lines strings.Builder
	for n := 1; n <= 200; n++ {
		fmt.Fprintf(&lines, "line %d\n", n)
	}
	for _, name := range []string{"a.go", "c.go"} {
		writeFile(t, filepath.Join(dir, name), lines.String())
	}
	writeFile(t, filepath.Join(dir, "gone.go"), "package gone\n")
	writeFile(t, filepath.Join(dir, "logo.bin"), "\x00\x01binary")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "reviewed")
	reviewed = strings.TrimSpace(string(mustGit(t, dir, "rev-parse", "HEAD")))
	remediate(dir)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "--allow-empty", "-m", "remediated")
	return dir, reviewed
}

// replaceLine rewrites line n (1-based) of a numbered file.
func replaceLine(t *testing.T, path string, n int, text string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	lines[n-1] = text
	writeFile(t, path, strings.Join(lines, "\n"))
}

func auditInvoker() *Invoker { return &Invoker{log: slog.New(slog.DiscardHandler)} }

// A change near the finding's line is the fix; a change far from it in the
// same file, a change in a file no finding names, a new file, a deleted file
// and a changed binary are scope drift, each reported where it is in the
// remediated head.
func TestAuditRemediation_FlagsChangesAwayFromTheFindings(t *testing.T) {
	dir, reviewed := remediationRepo(t, func(dir string) {
		replaceLine(t, filepath.Join(dir, "a.go"), 12, "fixed near the finding")
		replaceLine(t, filepath.Join(dir, "a.go"), 150, "rewritten far from it")
		replaceLine(t, filepath.Join(dir, "c.go"), 5, "touched in another file")
		writeFile(t, filepath.Join(dir, "b.go"), "package b\n")
		writeFile(t, filepath.Join(dir, "logo.bin"), "\x00\x02binary")
		if err := os.Remove(filepath.Join(dir, "gone.go")); err != nil {
			t.Fatal(err)
		}
	})
	review := worker.PrePublishReview{Revision: reviewed, BlockingFindings: []worker.PrePublishFinding{{ID: "FND-1", Path: "a.go", Line: 10}}}

	audit, err := auditInvoker().AuditRemediation(context.Background(), localWorkdir{dir: dir}, review, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go:150", "b.go (new file)", "c.go:5", "gone.go (deleted)", "logo.bin (whole file)"}
	if strings.Join(audit.ScopeDrift, "|") != strings.Join(want, "|") || audit.Escalation != "" {
		t.Fatalf("audit = %+v, want drift %q and no escalation", audit, want)
	}
}

// Line insertions and removals are measured at their reviewed-revision position
// and reported at the remediated head; a remediation confined to the finding's
// window reports nothing, and an untouched head reports nothing.
func TestAuditRemediation_InsertionsRemovalsAndCleanFixes(t *testing.T) {
	dir, reviewed := remediationRepo(t, func(dir string) {
		path := filepath.Join(dir, "a.go")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		// Insert two lines after line 11 (inside the window) and remove lines
		// 100-102 (outside it).
		out := append([]string{}, lines[:11]...)
		out = append(out, "guard 1", "guard 2")
		out = append(out, lines[11:99]...)
		out = append(out, lines[102:]...)
		writeFile(t, path, strings.Join(out, "\n"))
	})
	review := worker.PrePublishReview{Revision: reviewed, BlockingFindings: []worker.PrePublishFinding{{ID: "FND-1", Path: "a.go", Line: 10}}}
	audit, err := auditInvoker().AuditRemediation(context.Background(), localWorkdir{dir: dir}, review, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.go:101 (3 line(s) removed)"}; strings.Join(audit.ScopeDrift, "|") != strings.Join(want, "|") {
		t.Fatalf("drift = %q, want %q", audit.ScopeDrift, want)
	}

	clean, reviewed := remediationRepo(t, func(dir string) {
		replaceLine(t, filepath.Join(dir, "a.go"), 40, "inside the forty-line window")
	})
	review.Revision = reviewed
	if audit, err := auditInvoker().AuditRemediation(context.Background(), localWorkdir{dir: clean}, review, nil); err != nil || len(audit.ScopeDrift) != 0 {
		t.Fatalf("audit = %+v err = %v, want no drift for a fix inside the window", audit, err)
	}

	review.Revision = strings.TrimSpace(string(mustGit(t, clean, "rev-parse", "HEAD")))
	if audit, err := auditInvoker().AuditRemediation(context.Background(), localWorkdir{dir: clean}, review, nil); err != nil || len(audit.ScopeDrift) != 0 {
		t.Fatalf("audit = %+v err = %v, want nothing to measure at an unchanged head", audit, err)
	}
}

// A dispute of a blocking finding is recorded through the protocol and becomes
// the operator's decision brief; disputes naming no finding of the round,
// repeating one, or giving no reason are refused.
func TestAuditRemediation_DisputesEscalateThroughTheProtocol(t *testing.T) {
	dir, reviewed := remediationRepo(t, func(string) {})
	review := worker.PrePublishReview{Revision: reviewed, BlockingFindings: []worker.PrePublishFinding{
		{ID: "FND-1", Path: "a.go", Line: 10, RuleID: "correctness-critic/race", CalRow: "CAL-11", Message: "two processes can claim the same branch"},
		{ID: "FND-2", Path: "c.go", Line: 3, RuleID: "spec-fidelity/charter-must-unmet", CalRow: "CAL-1", Message: "missing refusal"},
		{ID: "FND-3", Path: "gone.go", RuleID: "design-conformance/invariant-violated", CalRow: "CAL-4", Message: "package boundary crossed"},
	}}
	ctx, work := context.Background(), localWorkdir{dir: dir}

	audit, err := auditInvoker().AuditRemediation(ctx, work, review, []worker.RemediationDispute{{FindingID: "FND-1", Reason: "D-1 names git worktree add; an atomic claim would replace it"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"conflict: the worker disputes 1 blocking finding(s) of the review at " + reviewed,
		"FND-1 at a.go:10 (rule correctness-critic/race, CAL-11): two processes can claim the same branch",
		"the worker's reason: D-1 names git worktree add",
		"option: uphold the finding", "option: withdraw the finding", "option: change the charter or the governing design",
		"decision needed: whether each disputed finding stands",
	} {
		if !strings.Contains(audit.Escalation, want) {
			t.Fatalf("escalation lacks %q:\n%s", want, audit.Escalation)
		}
	}
	if strings.Contains(audit.Escalation, "FND-2") {
		t.Fatalf("the undisputed finding must not enter the brief:\n%s", audit.Escalation)
	}
	audit, err = auditInvoker().AuditRemediation(ctx, work, review, []worker.RemediationDispute{{FindingID: "FND-3", Reason: "the design names this boundary"}})
	if err != nil || !strings.Contains(audit.Escalation, "FND-3 at gone.go (rule design-conformance/invariant-violated, CAL-4)") {
		t.Fatalf("file-level dispute = %q err = %v, want the finding located by its file", audit.Escalation, err)
	}
	if audit, err := auditInvoker().AuditRemediation(ctx, work, review, nil); err != nil || audit.Escalation != "" {
		t.Fatalf("no disputes = %+v err = %v, want no escalation", audit, err)
	}

	_, err = auditInvoker().AuditRemediation(ctx, work, review, []worker.RemediationDispute{{FindingID: "FND-1", Reason: "  "}})
	if !errors.Is(err, protocol.ErrDisputeNoReason) {
		t.Fatalf("reason-less dispute err = %v, want protocol.ErrDisputeNoReason", err)
	}
	if _, err := auditInvoker().AuditRemediation(ctx, work, review, []worker.RemediationDispute{{FindingID: "FND-9", Reason: "r"}}); err == nil || !strings.Contains(err.Error(), "FND-9, which is not a blocking finding") {
		t.Fatalf("unknown finding err = %v", err)
	}
	if _, err := auditInvoker().AuditRemediation(ctx, work, review, []worker.RemediationDispute{{FindingID: "FND-2", Reason: "r"}, {FindingID: "FND-2", Reason: "r"}}); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("repeated dispute err = %v", err)
	}
	if _, err := auditInvoker().AuditRemediation(ctx, nil, review, nil); err == nil {
		t.Fatal("a nil workdir must be refused")
	}
	if _, err := auditInvoker().AuditRemediation(ctx, localWorkdir{dir: t.TempDir()}, review, nil); err == nil || !strings.Contains(err.Error(), "remediated head") {
		t.Fatalf("a workdir that is no repository err = %v", err)
	}
	unknown := review
	unknown.Revision = strings.Repeat("0", 40)
	if _, err := auditInvoker().AuditRemediation(ctx, work, unknown, nil); err == nil || !strings.Contains(err.Error(), "list remediated files") {
		t.Fatalf("an unknown reviewed revision err = %v", err)
	}
}
