package reviewinvoke

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

// localWorld builds the real git shape the pre-publish review runs against: a
// parent repository with a mainline commit and a task worktree on its own
// branch with one commit beyond the base. No remote exists — the base falls
// back to the local mainline branch.
type localWorld struct {
	repo, worktree, branch, tip, base string
}

func newLocalWorld(t *testing.T) localWorld {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "config", "user.email", "t@example.com")
	runGit(t, repo, "config", "user.name", "t")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	writeFile(t, filepath.Join(repo, "README.md"), "base\n")
	calDir := filepath.Join(repo, ".pyxis", "skills", "impl-review")
	if err := os.MkdirAll(calDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(calDir, "calibration.md"), "approved calibration\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "add", ".pyxis/skills/impl-review/calibration.md")
	runGit(t, repo, "commit", "-q", "-m", "base")
	base := strings.TrimSpace(string(mustGit(t, repo, "rev-parse", "HEAD")))

	worktree := filepath.Join(root, "wt")
	runGit(t, repo, "worktree", "add", "-q", "-b", "feat/x", worktree)
	writeFile(t, filepath.Join(worktree, "pkg.go"), "package pkg\n")
	runGit(t, worktree, "add", "pkg.go")
	runGit(t, worktree, "commit", "-q", "-m", "implement")
	tip := strings.TrimSpace(string(mustGit(t, worktree, "rev-parse", "HEAD")))
	return localWorld{repo: repo, worktree: worktree, branch: "feat/x", tip: tip, base: base}
}

type localWorkdir struct{ dir, branch string }

func (w localWorkdir) Dir() string                              { return w.dir }
func (w localWorkdir) Branch() string                           { return w.branch }
func (w localWorkdir) Checkpoint(context.Context) (bool, error) { return true, nil }
func (w localWorkdir) Push(context.Context) (bool, error)       { return true, nil }
func (w localWorkdir) Cleanup(context.Context) error            { return nil }

func localInvoker(t *testing.T, rt port.AgentRuntime, repoDir string) *Invoker {
	t.Helper()
	store, err := reviewobligation.NewFileStore(filepath.Join(t.TempDir(), "review.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	verifier := &fakeVerifier{}
	terminalVerifier, err := NewStoreTerminalVerifier(store, verifier)
	if err != nil {
		t.Fatal(err)
	}
	i, err := New(Dependencies{Runtime: rt, Harness: "claude", RepositoryDir: repoDir, RepositorySlug: "o/r", Remote: "origin",
		Budget: time.Minute, Processes: &fakeProcessCoordinator{}, Persistence: verifier,
		TerminalVerifier: terminalVerifier, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	return i.WithObligationStore(store).WithMainline("main").WithRouteResolver(func(context.Context, port.Task, string, string) (Route, error) {
		return Route{NativeProfileID: "review-maximum", CatalogID: "claude-review-catalog", PolicyIdentity: "sha256:catalog-policy", Model: "review-model", Effort: "max"}, nil
	})
}

func localArtifact(t *testing.T, ref, rev, verdict string, findings []implreport.Finding, brief *implreport.EscalationBrief) string {
	t.Helper()
	contextID, err := implruntime.ContextID(fixtureNativeReviewParent)
	if err != nil {
		t.Fatal(err)
	}
	body, err := persist.ComposeArtifact(&implreport.Report{
		SchemaVersion: implreport.SchemaVersion, ProposalRef: ref, ReviewID: "review-process-test",
		Band: implreport.BandStandard, Verdict: verdict, EscalationBrief: brief,
		Rounds: []implreport.Round{{ReviewedRevision: rev, ContextID: contextID,
			Provenance: implreport.Provenance{Model: "review-model", PromptsVersion: "v1", CalVersion: "v1"}, Findings: findings}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// An accepted local review: the reviewer ran in a fresh read-only checkout of the
// worktree's exact HEAD (not the worktree, no remotes), was told the exact diff
// range and the charter, and the Worker gets the verdict, revision and artifact.
func TestReviewLocal_ReviewsExactLocalRevisionInIsolatedCheckout(t *testing.T) {
	world := newLocalWorld(t)
	var launchedDir string
	rt := &fakeRuntime{result: port.RunResult{Tokens: 21}, launchCheck: func(spec port.LaunchSpec) error {
		launchedDir = spec.Workdir
		if spec.Workdir == world.worktree || spec.Workdir == world.repo {
			return errors.New("reviewer launched inside the worktree or the parent checkout")
		}
		head, err := gitOut(context.Background(), spec.Workdir, "rev-parse", "HEAD")
		if err != nil || head != world.tip {
			return errors.New("reviewer workspace is not at the exact local revision: " + head)
		}
		if remotes, _ := gitOut(context.Background(), spec.Workdir, "remote"); strings.TrimSpace(remotes) != "" {
			return errors.New("reviewer workspace still has a remote: " + remotes)
		}
		if _, err := gitOut(context.Background(), spec.Workdir, "cat-file", "-e", world.base+"^{commit}"); err != nil {
			return errors.New("base commit unreachable in the reviewer workspace")
		}
		return nil
	}}
	i := localInvoker(t, rt, world.repo)
	task := port.Task{ID: "T5", Number: 5, Brief: "Implement the widget per the charter."}

	result := i.ReviewLocal(context.Background(), task, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err != nil {
		t.Fatalf("ReviewLocal: %v (phase %s/%s)", result.Err, result.Phase, result.FailureClass)
	}
	wantRef := LocalProposalRef("o/r", "feat/x")
	if result.Verdict != implreport.VerdictApprove || result.Revision != world.tip || result.ProposalRef != wantRef || result.ReviewID != "review-process-test" || result.Round != 1 {
		t.Fatalf("result = %+v, want approve at %s for %s", result, world.tip, wantRef)
	}
	if result.Blocking != 0 || result.Findings != "" || !strings.Contains(result.Artifact, "```impl-review") || result.Tokens != 21 {
		t.Fatalf("result = %+v", result)
	}
	if rt.spec.Purpose != port.LaunchPurposeReview || rt.spec.HeadBranch != "" || rt.spec.Model != "review-model" || rt.spec.Effort != "max" {
		t.Fatalf("launch spec = %+v", rt.spec)
	}
	for _, want := range []string{"/impl-review " + wantRef + " " + world.tip, "git diff " + world.base + " " + world.tip, "do not look for a pull request", "Implement the widget per the charter.", "Review process: review-process-test", "Native reviewer model: \"" + rt.spec.Model + "\""} {
		if !strings.Contains(rt.assembled, want) {
			t.Errorf("brief missing %q:\n%s", want, rt.assembled)
		}
	}
	if _, err := os.Stat(launchedDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("isolated review workspace %s was not cleaned up: %v", launchedDir, err)
	}
	if _, err := os.Stat(rt.spec.RuntimeStateDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disposable reviewer runtime state was not cleaned: %v", err)
	}
	if rt.killed {
		t.Error("a completed review must not be killed")
	}
}

// Unresolved blocking findings mean request-changes — even when the reported
// envelope says approve — and the findings come back as remediation data.
func TestReviewLocal_BlockingFindingsRequestChangesAndFeedRemediation(t *testing.T) {
	world := newLocalWorld(t)
	ref := LocalProposalRef("o/r", "feat/x")
	rt := &fakeRuntime{}
	rt.result.Output = localArtifact(t, ref, world.tip, implreport.VerdictApprove, []implreport.Finding{{
		ID: "FND-1", Path: "pkg.go", Line: 1, Message: "package has no exported behavior", RuleID: "R1",
		Lens: implreport.LensSpecFidelity, Severity: implreport.SeverityBlocking, CalRow: "C-1", Acceptance: "export the widget",
	}, {
		ID: "FND-2", Path: "pkg.go", Message: "naming nit", RuleID: "R2", Lens: implreport.LensDesignConformance, Severity: implreport.SeverityAdvisory, CalRow: "C-9",
	}}, nil)
	i := localInvoker(t, rt, world.repo)

	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 2)
	if result.Err != nil {
		t.Fatalf("ReviewLocal: %v", result.Err)
	}
	if result.Verdict != implreport.VerdictRequestChanges || result.Blocking != 1 || result.Round != 2 {
		t.Fatalf("result = %+v, want request-changes with one blocking finding", result)
	}
	for _, want := range []string{"[FND-1]", "pkg.go:1", "no exported behavior", "acceptance: export the widget"} {
		if !strings.Contains(result.Findings, want) {
			t.Errorf("findings missing %q:\n%s", want, result.Findings)
		}
	}
	if strings.Contains(result.Findings, "FND-2") {
		t.Errorf("advisory finding leaked into the remediation data:\n%s", result.Findings)
	}
}

func TestReviewLocal_EscalateCarriesBrief(t *testing.T) {
	world := newLocalWorld(t)
	ref := LocalProposalRef("o/r", "feat/x")
	rt := &fakeRuntime{}
	rt.result.Output = localArtifact(t, ref, world.tip, implreport.VerdictEscalate, nil, &implreport.EscalationBrief{
		Conflict: "the charter contradicts the design on retries", Options: []string{"ratify A", "ratify B"}, DecisionNeeded: "which retry policy"})
	i := localInvoker(t, rt, world.repo)
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err != nil || result.Verdict != implreport.VerdictEscalate {
		t.Fatalf("result = %+v", result)
	}
	for _, want := range []string{"conflict: the charter contradicts", "option: ratify A", "decision needed: which retry policy"} {
		if !strings.Contains(result.EscalationBrief, want) {
			t.Errorf("brief missing %q:\n%s", want, result.EscalationBrief)
		}
	}
}

// No usable artifact ⇒ the round fails as artifact_unverified; a reviewer that
// stops early ⇒ infrastructure; neither yields a verdict the Worker could act on.
func TestReviewLocal_MissingOrMalformedArtifactFailsClosed(t *testing.T) {
	world := newLocalWorld(t)
	rt := &fakeRuntime{}
	rt.result.Output = "I reviewed it and it looks fine."
	i := localInvoker(t, rt, world.repo)
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if !result.Failed() || result.FailureClass != "artifact_unverified" || result.Phase != "artifact" || result.Verdict != "" {
		t.Fatalf("result = %+v, want an artifact_unverified failure with no verdict", result)
	}

	rt = &fakeRuntime{result: port.RunResult{Incomplete: true, Reason: "error_max_turns"}}
	i = localInvoker(t, rt, world.repo)
	result = i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if !result.Failed() || result.FailureClass != "infrastructure" || result.Phase != "await" {
		t.Fatalf("result = %+v, want an await infrastructure failure", result)
	}
}

// A worktree whose base cannot be determined refuses before any launch.
func TestReviewLocal_NoMainlineRefRefusesBeforeLaunch(t *testing.T) {
	world := newLocalWorld(t)
	rt := &fakeRuntime{}
	i := localInvoker(t, rt, world.repo).WithMainline("release/does-not-exist")
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if !result.Failed() || result.Phase != "preflight" || rt.launches != 0 {
		t.Fatalf("result = %+v launches=%d, want a preflight refusal with no launch", result, rt.launches)
	}
}

// The base prefers the remote-tracking mainline when one exists.
func TestReviewLocal_BasePrefersRemoteTrackingMainline(t *testing.T) {
	world := newLocalWorld(t)
	// Advance local main past the branch point; the remote-tracking ref stays at
	// the original base, so merge-base must come from refs/remotes/origin/main.
	writeFile(t, filepath.Join(world.repo, "later.md"), "later\n")
	runGit(t, world.repo, "add", "later.md")
	runGit(t, world.repo, "commit", "-q", "-m", "later on main")
	runGit(t, world.repo, "update-ref", "refs/remotes/origin/main", world.base)
	rt := &fakeRuntime{}
	i := localInvoker(t, rt, world.repo)
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err != nil {
		t.Fatalf("ReviewLocal: %v", result.Err)
	}
	if !strings.Contains(rt.assembled, "git diff "+world.base+" "+world.tip) {
		t.Fatalf("brief does not name the remote-tracking base %s:\n%s", world.base, rt.assembled)
	}
}

// Under an activated route the pre-publish reviewer launches only with a permit
// bound to the task, the local proposal identity and the revision, and its
// outcome is appended when the round returns.
func TestReviewLocal_RoutedAdmissionFencesLaunchAndRecordsOutcome(t *testing.T) {
	world := newLocalWorld(t)
	rt := &fakeRuntime{}
	permit := &fakeReviewPermit{}
	var admitted []string
	var terminals int
	i := localInvoker(t, rt, world.repo).
		WithReviewAdmission(func(_ context.Context, task port.Task, proposalRef, revision string, route Route) (routingevidence.BoundaryPermit, error) {
			admitted = append(admitted, task.ID+" "+proposalRef+" "+revision+" "+route.NativeProfileID)
			return permit, nil
		}).
		WithReviewOutcome(func(_ context.Context, terminal routingevidence.AttemptTerminal) error {
			terminals++
			if terminal.Outcome != routingevidence.OutcomeVerified {
				t.Errorf("routed terminal = %+v, want verified", terminal)
			}
			return nil
		})
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", DemandID: "demand-1", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err != nil {
		t.Fatalf("ReviewLocal: %v", result.Err)
	}
	wantRef := LocalProposalRef("o/r", "feat/x")
	if len(admitted) != 1 || admitted[0] != "T5 "+wantRef+" "+world.tip+" review-maximum" {
		t.Fatalf("admissions = %v", admitted)
	}
	if permit.calls != 1 || permit.binding.ProposalID != wantRef || permit.binding.TaskID != "T5" {
		t.Fatalf("permit = calls %d binding %+v", permit.calls, permit.binding)
	}
	if terminals != 1 || rt.spec.Model != "review-model" || rt.spec.Effort != "max" {
		t.Fatalf("terminals=%d spec=%+v", terminals, rt.spec)
	}

	// A refused admission never launches.
	rt = &fakeRuntime{}
	i = localInvoker(t, rt, world.repo).WithReviewAdmission(func(context.Context, port.Task, string, string, Route) (routingevidence.BoundaryPermit, error) {
		return nil, errors.New("policy invalidated")
	}).WithReviewOutcome(func(context.Context, routingevidence.AttemptTerminal) error { return nil })
	result = i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if !result.Failed() || result.Phase != "routing" || rt.launches != 0 {
		t.Fatalf("result = %+v launches=%d, want a routing refusal with no launch", result, rt.launches)
	}
}

// Launch, workspace-integrity and route failures are infrastructure failures
// that never yield a verdict; a bogus repository refuses at workspace
// preparation; a non-git worktree refuses at preflight.
func TestReviewLocal_InfrastructureFailuresNeverYieldAVerdict(t *testing.T) {
	world := newLocalWorld(t)
	work := localWorkdir{dir: world.worktree, branch: world.branch}
	task := port.Task{ID: "T5", Brief: "charter"}

	rt := &fakeRuntime{launchErr: errors.New("codex: not installed")}
	result := localInvoker(t, rt, world.repo).ReviewLocal(context.Background(), task, work, 1)
	if !result.Failed() || result.Phase != "launch" || result.Verdict != "" {
		t.Fatalf("launch failure result = %+v", result)
	}

	rt = &fakeRuntime{}
	result = localInvoker(t, rt, world.repo).WithWorkspaceVerifier(func(context.Context, string, string) error {
		return errors.New("tree modified")
	}).ReviewLocal(context.Background(), task, work, 1)
	if !result.Failed() || result.Phase != "await" || !strings.Contains(result.Err.Error(), "tree modified") {
		t.Fatalf("mutated workspace result = %+v", result)
	}

	rt = &fakeRuntime{}
	result = localInvoker(t, rt, world.repo).WithRouteResolver(func(context.Context, port.Task, string, string) (Route, error) {
		return Route{}, errors.New("no qualified review profile")
	}).ReviewLocal(context.Background(), task, work, 1)
	if !result.Failed() || result.Phase != "routing" || rt.launches != 0 {
		t.Fatalf("route failure result = %+v launches=%d", result, rt.launches)
	}

	rt = &fakeRuntime{}
	result = localInvoker(t, rt, world.repo).WithRouteResolver(func(context.Context, port.Task, string, string) (Route, error) {
		return Route{NativeProfileID: "p", Model: "", Effort: "max"}, nil
	}).ReviewLocal(context.Background(), task, work, 1)
	if !result.Failed() || result.Phase != "routing" || !strings.Contains(result.Err.Error(), "incomplete") {
		t.Fatalf("incomplete route result = %+v", result)
	}

	rt = &fakeRuntime{}
	result = localInvoker(t, rt, filepath.Join(t.TempDir(), "no-such-repo")).ReviewLocal(context.Background(), task, work, 1)
	if !result.Failed() || result.Phase != "launch" || !strings.Contains(result.Err.Error(), "clone isolated workspace") {
		t.Fatalf("bogus repository result = %+v", result)
	}

	rt = &fakeRuntime{}
	result = localInvoker(t, rt, world.repo).ReviewLocal(context.Background(), task, localWorkdir{dir: t.TempDir(), branch: "feat/x"}, 1)
	if !result.Failed() || result.Phase != "preflight" {
		t.Fatalf("non-git worktree result = %+v", result)
	}

	result = localInvoker(t, &fakeRuntime{}, world.repo).ReviewLocal(context.Background(), task, nil, 1)
	if !result.Failed() || result.Phase != "preflight" {
		t.Fatalf("nil workdir result = %+v", result)
	}
}

// The effective verdict is the most restrictive of the reported envelope and the
// one recomputed from findings; an empty envelope takes the computed one.
func TestEffectiveVerdictAndBlockingFindings(t *testing.T) {
	blocking := implreport.Finding{ID: "F1", Path: "a.go", Line: 3, Message: "m", RuleID: "R", Lens: implreport.LensCorrectnessCritic, Severity: implreport.SeverityBlocking, CalRow: "C"}
	applied := blocking
	applied.ID = "F2"
	applied.Disposition = &implreport.Disposition{Actor: "worker", Verdict: implreport.VerdictApplied}
	rep := &implreport.Report{Rounds: []implreport.Round{{ReviewedRevision: "r", Findings: []implreport.Finding{blocking, applied}}}}
	if got := effectiveVerdict(rep); got != implreport.VerdictRequestChanges {
		t.Fatalf("empty envelope → %q, want request-changes", got)
	}
	rep.Verdict = implreport.VerdictApprove
	if got := effectiveVerdict(rep); got != implreport.VerdictRequestChanges {
		t.Fatalf("approve envelope over blocking findings → %q, want request-changes", got)
	}
	n, text, listed := blockingFindings(rep)
	if n != 1 || !strings.Contains(text, "[F1] a.go:3") || strings.Contains(text, "F2") {
		t.Fatalf("blocking = %d %q", n, text)
	}
	if len(listed) != 1 || listed[0].ID != "F1" || listed[0].Path != "a.go" || listed[0].Line != 3 {
		t.Fatalf("listed blocking findings = %+v, want F1 at a.go:3 only", listed)
	}
	if n, text, listed := blockingFindings(nil); n != 0 || text != "" || listed != nil {
		t.Fatal("nil report must have no findings")
	}
	if n, _, _ := blockingFindings(&implreport.Report{}); n != 0 {
		t.Fatal("no rounds must have no findings")
	}
	rep = &implreport.Report{Verdict: implreport.VerdictEscalate, EscalationBrief: &implreport.EscalationBrief{Conflict: "c", Options: []string{"a"}, DecisionNeeded: "d"},
		Rounds: []implreport.Round{{ReviewedRevision: "r"}}}
	if got := effectiveVerdict(rep); got != implreport.VerdictEscalate {
		t.Fatalf("escalate envelope → %q", got)
	}
}

func TestEffectiveVerdictPreservesReportedValueWhenRecomputationCannotOrderIt(t *testing.T) {
	const reported = "future-verdict"
	if got := effectiveVerdict(&implreport.Report{Verdict: reported}); got != reported {
		t.Fatalf("no-round fallback = %q, want %q", got, reported)
	}
	withRound := &implreport.Report{Verdict: reported, Rounds: []implreport.Round{{ReviewedRevision: "abc"}}}
	if got := effectiveVerdict(withRound); got != reported {
		t.Fatalf("unknown-order fallback = %q, want %q", got, reported)
	}
}

// A refused handoff is explained from the host's side: which identity the
// artifact got wrong, or that it was not an artifact at all; an incomplete run
// carries the reviewer's (redacted, bounded) final message.
func TestDescribeHandoffAndOutputExcerpt(t *testing.T) {
	admitted := implreview.AdmittedProcess{ReviewProcessID: "review-process-test", Subject: implreview.ReviewSubject{
		ReviewObligationID: "ob", ProposalID: "local:o/r@feat/x", ReviewedRevision: "abc123"}}
	if got := describeHandoff("", admitted); !strings.Contains(got, "no artifact") {
		t.Fatalf("empty: %q", got)
	}
	if got := describeHandoff("I reviewed it, looks fine.", admitted); !strings.Contains(got, "no single parseable impl-review machine block") || !strings.Contains(got, "looks fine") {
		t.Fatalf("prose: %q", got)
	}
	body, err := persist.ComposeArtifact(&implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: "github:o/r#9", ReviewID: "self-minted",
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove, Rounds: []implreport.Round{{ReviewedRevision: "abc1", ContextID: "c",
			Provenance: implreport.Provenance{Model: "m", PromptsVersion: "v1", CalVersion: "v1"}}}})
	if err != nil {
		t.Fatal(err)
	}
	got := describeHandoff(body, admitted)
	for _, want := range []string{`proposal_ref "github:o/r#9" (admitted "local:o/r@feat/x")`, `reviewed_revision "abc1" (admitted "abc123")`, `review_id "self-minted" (admitted "review-process-test")`} {
		if !strings.Contains(got, want) {
			t.Errorf("mismatch description missing %q: %s", want, got)
		}
	}
	body, _ = persist.ComposeArtifact(&implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: "local:o/r@feat/x", ReviewID: "review-process-test",
		Band: implreport.BandStandard, Verdict: implreport.VerdictApprove, Rounds: []implreport.Round{{ReviewedRevision: "abc123", ContextID: "c",
			Provenance: implreport.Provenance{Model: "m", PromptsVersion: "v1", CalVersion: "v1"}}}})
	if got := describeHandoff(body, admitted); !strings.Contains(got, "identities match") {
		t.Fatalf("matching identities: %q", got)
	}
	if got := outputExcerpt("  "); got != "(empty)" {
		t.Fatalf("empty excerpt = %q", got)
	}
	long := strings.Repeat("x", 700)
	if got := outputExcerpt(long); !strings.HasSuffix(got, `…"`) || len(got) > 620 {
		t.Fatalf("excerpt not bounded: %d", len(got))
	}
	if got := outputExcerpt("token ghp_0123456789abcdef0123456789abcdef0123 leaked"); strings.Contains(got, "ghp_0123456789") {
		t.Fatalf("excerpt not redacted: %s", got)
	}
}

// The pre-publish brief binds the artifact identity explicitly and forbids a
// plan-mode exit prompt, since a headless reviewer has no one to ask.
func TestReviewLocal_BriefBindsIdentityAndForbidsPlanExit(t *testing.T) {
	world := newLocalWorld(t)
	rt := &fakeRuntime{}
	i := localInvoker(t, rt, world.repo)
	result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "charter"}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, want := range []string{"review_id must be exactly review-process-test", "reviewed_revision must be exactly the full sha " + world.tip, "never call ExitPlanMode"} {
		if !strings.Contains(rt.assembled, want) {
			t.Errorf("brief missing %q", want)
		}
	}
}

func (w localWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}
