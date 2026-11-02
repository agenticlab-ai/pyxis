package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	forgejocodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	ghcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/github"
	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/ghidentity"
	carinatracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
	ghtracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/github"
	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/dispatch"
	"github.com/agenticlab-ai/pyxis/internal/executor/issueentry"
	"github.com/agenticlab-ai/pyxis/internal/executor/lease"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewinvoke"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/runlog"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/executor/tasksession"
	"github.com/agenticlab-ai/pyxis/internal/executor/trackeridentity"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/executor/workspace"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/plangate/planref"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
	"github.com/agenticlab-ai/pyxis/internal/toolbin"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

type stringList []string

type executionHost interface {
	port.CodeHost
	port.ProposalProbe
	reviewinvoke.ProposalInspector
	worker.ProposalHeadReader
	worker.ReviewPublisher
}

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("empty value")
	}
	*s = append(*s, value)
	return nil
}

const (
	// defaultRoutingLabel marks the tracker items the executor may pick up — the
	// automated-implementation frontier (T5). A repo overrides it with --label.
	defaultRoutingLabel = "automated-implementation"
	// defaultJournalRel is the local event journal's default path under the repo
	// when --journal is unset in either execution mode.
	defaultJournalRel = runlogs.Executor + "/events.jsonl"
	// defaultWorkerBudget is the uniform per-Worker wall-clock bound (§3.7) — the
	// mechanical turn bound for the cap-less backends (codex, opencode), uniform
	// across all backends. Sized for a substantial task using a model with
	// high deliberation effort, which can take longer per turn; a
	// worker that exhausts it checkpoints and is respawned to resume from its
	// worktree (DINV-9, AP-22). A repo overrides it with --worker-budget.
	defaultWorkerBudget = config.DefaultWorkerBudget
)

// run wires the executor and runs one frontier-sweep, printing the classed report.
// It returns a process exit code: 0 = the sweep completed with no failed task or
// configured review, 1 = at least one task/review failed or escalated, 2 = a
// usage or wiring/operational error. The sweep NEVER merges — it opens genuinely-
// green proposals and reports the blocked set; the driver lands blockers and
// re-invokes (§3.6).
func run(args []string, stdout, stderr io.Writer) int {
	return runWithClock(args, stdout, stderr, time.Now)
}

func runWithClock(args []string, stdout, stderr io.Writer, now func() time.Time) (exitCode int) {
	if len(args) > 0 && args[0] == "--internal-review-pack" {
		return pack.RunCommand(context.Background(), args[1:], stderr)
	}
	if len(args) > 0 && args[0] == "--internal-review-context" {
		return pack.RunContextIDCommand(context.Background(), args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "--internal-review-schema" {
		return reviewSchemaTool(args[1:], stdout, stderr)
	}
	started := now()
	fs := flag.NewFlagSet("implement-now", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintln(stderr, "usage: implement-now --repo checkout [--repo-slug owner/name] [flags]")
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintln(stderr, "  implements one selected issue or runs one frozen-plan frontier sweep to green proposals.")
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintln(stderr, "  attended (default): a live progress view; both modes retain a local JSONL event journal.")
		fs.PrintDefaults()
	}
	var (
		repoPath      = fs.String("repo", ".", "local repo checkout (default: pyxis-operated repo root above cwd)")
		repoSlug      = fs.String("repo-slug", "", "legacy owner/name fallback; explicit integration bindings take precedence")
		base          = fs.String("base", "", "mainline base branch: proposals open against it, worktrees branch off it (default: repo config `branch`)")
		remote        = fs.String("remote", "", "git remote for the shared claim refs and the mainline fetch (default: repo config `remote`)")
		label         = fs.String("label", defaultRoutingLabel, "tracker label marking automated-implementation items (the frontier)")
		initiator     = fs.String("initiator", "operator", "who invoked this sweep (operator, drive-wake, …) — the run_started attribution (EVT-6)")
		scope         = fs.String("scope", "", "what this sweep targets (a plan ref, a repo slug) — the run_started attribution (EVT-6)")
		_             = fs.Bool("unattended", false, "compatibility flag; both modes retain the local JSONL event journal")
		journalPath   = fs.String("journal", "", "event journal path inside .pyxis/logs (default <repo>/"+defaultJournalRel+")")
		workerBudget  = fs.Duration("worker-budget", defaultWorkerBudget, "uniform per-Worker wall-clock bound (the mechanical cap for cap-less backends)")
		trackerKind   = fs.String("tracker", "gh", "legacy tracker: gh or carina; configure integrations.bindings.tasks for Forgejo")
		carinaRoot    = fs.String("carina-root", "", "carina store root when --tracker carina (default: --repo)")
		carinaBin     = fs.String("carina-bin", "", "carina executable when --tracker carina (default: source/release resolution, then PATH)")
		planRef       = fs.String("plan-ref", "", "limit pickup to one projected plan (<feature>/<NNN>) on the configured tracker")
		ambient       = fs.String("ambient-backend", "", "preferred model backend; fallback: PYXIS_AGENT_BACKEND, then the first compatible authenticated provider")
		reviewOnly    = fs.Bool("review-only", false, "run only the closed set of selected pending review obligations")
		driveRoundID  = fs.String("drive-round-id", "", "driver round joining a review-only recovery invocation")
		issue         = fs.Int("issue", 0, "exact issue number on the configured task tracker; requires --issue-input")
		issueInput    = fs.String("issue-input", "", "bounded issue-task-input/v1 JSON charter")
		inspectNumber = fs.Int("inspect-issue", 0, "read one issue and emit its exact source binding and comment citations; no execution")
		recoverHead   = fs.String("recover-checkpoint", "", "with --issue: verify and publish this exact committed, unpublished checkpoint of the admitted issue; launches no model")
	)
	var selectedReviews stringList
	fs.Var(&selectedReviews, "review-obligation", "pending review obligation to recover (repeatable; requires --review-only)")
	if err := fs.Parse(args); err != nil {
		return 2 // flag already printed the usage/error to stderr
	}
	normalizedPlanRef := strings.TrimSpace(*planRef)
	issueMode := *issue > 0
	issueOperand, inspectOperand := false, false
	fs.Visit(func(f *flag.Flag) {
		issueOperand = issueOperand || f.Name == "issue"
		inspectOperand = inspectOperand || f.Name == "inspect-issue"
	})
	if (issueOperand && *issue <= 0) || (inspectOperand && *inspectNumber <= 0) || (issueMode && (normalizedPlanRef != "" || *reviewOnly || *inspectNumber != 0 || strings.TrimSpace(*issueInput) == "")) || (!issueMode && *issueInput != "") || (*inspectNumber > 0 && (normalizedPlanRef != "" || *reviewOnly || *issueInput != "")) {
		_, _ = fmt.Fprintln(stderr, "implement-now: --issue N --issue-input file, --inspect-issue N, and plan/review modes are mutually exclusive; issue numbers must be positive")
		return 2
	}
	if *recoverHead != "" && (!issueMode || !issueentry.ValidRevision(*recoverHead)) {
		_, _ = fmt.Fprintln(stderr, "implement-now: --recover-checkpoint requires --issue N --issue-input file and a full lowercase commit id")
		return 2
	}
	if *reviewOnly {
		if strings.TrimSpace(*driveRoundID) == "" || len(selectedReviews) == 0 || normalizedPlanRef == "" {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintln(stderr, "implement-now: --review-only requires --plan-ref, --drive-round-id, and at least one --review-obligation")
			return 2
		}
	} else if *driveRoundID != "" || len(selectedReviews) != 0 {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintln(stderr, "implement-now: review recovery flags require --review-only")
		return 2
	}
	resolvedRepo, err := resolveRepo(*repoPath)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: --repo: %v\n", err)
		return 2
	}
	*repoPath = resolvedRepo
	integrationCfg, integrationErr := integrations.Load(*repoPath)
	if integrationErr != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: integrations: %v\n", integrationErr)
		return 2
	}
	if strings.TrimSpace(*repoSlug) == "" && integrationCfg.Bindings.CodeHost == (integrations.Binding{}) {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintln(stderr, "implement-now: --repo-slug owner/name is required")
		fs.Usage()
		return 2
	}
	codeBinding, integrationErr := integrationCfg.CodeHost(*repoSlug)
	if integrationErr != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: code host: %v\n", integrationErr)
		return 2
	}
	taskBinding, integrationErr := integrationCfg.Tasks(*repoSlug, *trackerKind, *carinaRoot)
	if integrationErr != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: task tracker (unknown --tracker or invalid binding): %v\n", integrationErr)
		return 2
	}
	*repoSlug = codeBinding.Repository
	if issueMode || *inspectNumber > 0 {
		if taskBinding.Provider == integrations.Carina && taskBinding.Root == "" {
			taskBinding.Root = *repoPath
		} else if taskBinding.Provider == integrations.Carina && !filepath.IsAbs(taskBinding.Root) {
			taskBinding.Root = filepath.Join(*repoPath, taskBinding.Root)
		}
	}
	codeGitHubHost, taskGitHubHost := "", ""
	if integrationCfg.Bindings.CodeHost != (integrations.Binding{}) {
		codeGitHubHost = "github.com"
	}
	if integrationCfg.Bindings.Tasks != (integrations.Binding{}) {
		taskGitHubHost = "github.com"
	}
	if issueMode || *inspectNumber > 0 {
		if taskBinding.Provider == integrations.GitHub {
			taskGitHubHost = "github.com"
		}
		if codeBinding.Provider == integrations.GitHub {
			codeGitHubHost = "github.com"
		}
	}
	reviewRepository := *repoSlug
	if codeBinding.Provider == integrations.Forgejo {
		reviewRepository = codeBinding.Key()
	}
	if normalizedPlanRef != "" {
		// The scope grammar is tracker-neutral (planref, shared by all
		// plan-scoped trackers), so a plan is admitted identically across them.
		if _, _, err := planref.ParsePlanRef(normalizedPlanRef); err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: --plan-ref: %v\n", err)
			return 2
		}
	}
	reviewScope := normalizedPlanRef
	if reviewScope == "" {
		reviewScope = strings.TrimSpace(*scope)
	}
	if reviewScope == "" {
		reviewScope = strings.TrimSpace(*repoSlug)
	}

	// The mainline branch and remote resolve from the repo config (config-driven-
	// portability-v1 §3.2, FR-2): an unset --base/--remote takes the configured
	// `branch`/`remote`, so a repo whose default branch is not "main" (or whose
	// remote is not "origin") is honored. An explicit flag still overrides. A
	// default config yields "main"/"origin", byte-identical (DINV-2). Config-load
	// defects are non-fatal here (the config's own load path names them elsewhere);
	// the resolved values feed the workspace, lease, and code-host wiring below.
	repoCfg, _ := repoconfig.Load(*repoPath)
	if strings.TrimSpace(*base) == "" {
		*base = repoCfg.Branch
	}
	if strings.TrimSpace(*remote) == "" {
		*remote = repoCfg.Remote
	}

	// Coarse intervention: a SIGINT/SIGTERM cancels the run, which kills in-flight
	// Workers and releases their claims (the only intervention headless execution
	// affords — §3.9).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var hostCredentialNames []string
	for _, connection := range integrationCfg.Connections {
		hostCredentialNames = append(hostCredentialNames, connection.TokenEnv)
	}
	ctx = runtime.WithHostCredentialNames(ctx, hostCredentialNames...)
	if codeBinding.Provider == integrations.LocalGit {
		_, defects := repoconfig.Load(*repoPath)
		if len(defects) != 0 {
			// The exit code carries this pre-write refusal.
			_, _ = fmt.Fprintf(stderr, "implement-now: local Git configuration: %v\n", defects)
			return 2
		}
		probe := localcodehost.NewDiagnostics(localcodehost.Options{Root: *repoPath, Origin: codeBinding.Root, Remote: repoCfg.Remote, Mainline: repoCfg.Branch})
		if err := codehost.RequireWorkflow(ctx, probe); err != nil {
			// No execution log, claim, agent, or hosted adapter has started.
			_, _ = fmt.Fprintf(stderr, "implement-now: local Git code host: %v\n", err)
			return 2
		}
	}

	cfg, err := config.Load(*repoPath)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: load executor config: %v\n", err)
		return 2
	}
	var selectedTracker port.WorkTracker
	var issueSession *issueRun
	var legacyClaimSource string
	if issueMode || *inspectNumber > 0 {
		if issueMode {
			if err := issuePreflight(cfg, *repoPath); err != nil {
				_, _ = fmt.Fprintf(stderr, "implement-now: issue preflight: %v\n", err)
				return 2
			}
		}
		var code int
		selectedTracker, code = tracker(ctx, taskBinding, repoPath, carinaBin, stderr, "", repoSlug, label, nil, taskGitHubHost, trackerKind)
		if code != 0 {
			return code
		}
		if *inspectNumber > 0 {
			if err := inspectIssue(ctx, selectedTracker, taskBinding, *inspectNumber, *repoPath, *remote, stdout); err != nil {
				_, _ = fmt.Fprintf(stderr, "implement-now: inspect issue: %v\n", err)
				return 2
			}
			return 0
		}
		// Matching hosted bindings reserve both claim formats at execution, so
		// unrelated legacy work need not block this provider-qualified issue.
		legacyClaimSource, err = trackeridentity.CheckIssueMigration(ctx, *repoPath, *remote, taskBinding, codeBinding, *issue)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: issue claim namespace: %v\n", err)
			return 2
		}
		issueSession, selectedTracker, err = admitSelectedIssue(ctx, selectedTracker, taskBinding, *issue, *issueInput, *repoPath, *remote, started, now, *recoverHead != "", cfg.CISelfHealAttempts())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: admit issue: %v\n", err)
			return 2
		}
		// Every exit after admission names its terminal: finish and recovery
		// report their own, and any other failure the phase it ended in.
		defer func() {
			if exitCode != 0 {
				issueSession.closeFailed(ctx, stderr)
			}
		}()
		reviewScope = issueSession.scope()
		*scope = reviewScope
		ctx = port.WithInvocationObserver(ctx, &issueSession.collector)
		// One selected issue owns one protected verification scratch throughout
		// worker, publication, and final proof. Nested gates retain the same cache
		// and launch boundary instead of rebuilding the unchanged test tree.
		protected, cleanup, protectErr := runtime.VerificationContext(ctx)
		if protectErr != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: issue verification boundary: %v\n", protectErr)
			return 2
		}
		defer cleanup()
		ctx = protected
	}
	flowLog, flowLogPath, err := runlog.Open(*repoPath, cfg.RunLogDirectory(), featureName(reviewScope), time.Now())
	if err != nil {
		// Best-effort diagnostic; the exit code still reports the failure.
		_, _ = fmt.Fprintf(stderr, "implement-now: open execution log: %v\n", err)
		return 2
	}
	defer func() {
		// Log cleanup must not replace the execution result.
		_ = flowLog.Close()
	}()
	// Announcing the path is best-effort; the reporter handles sink failures.
	_, _ = fmt.Fprintf(stdout, "implement-now: execution log %s\n", flowLogPath)
	hc, err := harnessconfig.Load(*repoPath)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: load harness-config: %v\n", err)
		return 2
	}
	ambientBackend := resolveAmbientBackend(*ambient, os.Getenv)

	// The two-sink reporting surface (§3.9): one Reporter, an operator live view
	// and an append-only event journal in both attended and unattended modes.
	operatorLog, err := runlogs.Create(ctx, *repoPath, runlogs.Executor, "operator-*.log")
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: open execution log: %v\n", err)
		return 2
	}
	defer func() {
		if err := operatorLog.Close(); err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: close execution log: %v\n", err)
		}
	}()
	logger := slog.New(slog.NewTextHandler(io.MultiWriter(stderr, operatorLog), &slog.HandlerOptions{Level: slog.LevelInfo}))
	journalW, closeJournal, err := openJournalFor(*journalPath, *repoPath)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: open argus journal: %v\n", err)
		return 2
	}
	defer closeJournal()
	sinks, argusSink := buildSinks(logger, journalW, flowLog)
	runID := report.NewRunID()
	reporter := report.NewReporterWith(runID, sinks, report.WithDeliveryErrorHandler(reportDeliveryErrorLogger(logger)))
	if issueSession != nil {
		issueSession.reporter = reporter
		ctx = codegatemirror.WithProgressObserver(ctx, func(progress codegatemirror.Progress) {
			elapsed := progress.Duration.Milliseconds()
			reporter.Emit(report.Event{Kind: report.KindGateCheckProgress, TaskID: issueSession.task.ID, Issue: issueSession.task.Number, Phase: progress.Name, Status: progress.Status, ElapsedMS: &elapsed, Reason: progress.Detail, ArtifactRef: progress.LogPath})
		})
		issueSession.logCommandStarted(started)
		defer func() { issueSession.logCommandFinished(exitCode) }()
	}
	// One budget instance is shared by every metered GitHub adapter in this
	// process; the sidecar locks extend that sharing to every other pyxis
	// process on the machine. The ledgers are keyed by (host, identity) under
	// the machine state dir — never this checkout, whose lifetime is
	// unrelated to the window GitHub meters (incident I7).
	var ghHost, ghLogin string
	var writeBudget *budget.Budget
	var taskBudget *budget.Budget
	if codeBinding.Provider == integrations.GitHub || taskBinding.Provider == integrations.GitHub {
		primaryTokenEnv := codeBinding.TokenEnv
		primaryGitHubHost := codeGitHubHost
		if codeBinding.Provider != integrations.GitHub {
			primaryTokenEnv = taskBinding.TokenEnv
			primaryGitHubHost = taskGitHubHost
		}
		resolver := ghidentity.ResolverForTokenEnv(primaryTokenEnv, primaryGitHubHost)
		ghHost, ghLogin = resolver(ctx)
		if ghLogin == "" {
			logger.Warn("github identity unresolved — writes meter against the host-wide window alone (coarser, never finer)",
				slog.String("host", ghHost))
		}
		for _, stale := range budget.StaleLegacyPaths(*repoPath) {
			logger.Info("a legacy content-budget file is present and ignored — safe to delete once its window has passed",
				slog.String("path", stale))
		}
		budgetEvents := watch.NewSink(filepath.Join(*repoPath, watch.DefaultEventsDir), 0, nil)
		var budgetErr error
		writeBudget, budgetErr = budget.NewForIdentityResolver(ghHost, ghLogin, resolver, nil, contentBudgetObserver(logger, budgetEvents))
		if budgetErr != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: resolve the content-budget ledgers: %v\n", budgetErr)
			return 2
		}
		taskBudget = writeBudget
		if codeBinding.Provider == integrations.GitHub && taskBinding.Provider == integrations.GitHub && (taskBinding.TokenEnv != codeBinding.TokenEnv || taskGitHubHost != codeGitHubHost) {
			taskResolver := ghidentity.ResolverForTokenEnv(taskBinding.TokenEnv, taskGitHubHost)
			taskHost, taskLogin := taskResolver(ctx)
			taskBudget, budgetErr = budget.NewForIdentityResolver(taskHost, taskLogin, taskResolver, nil, contentBudgetObserver(logger, budgetEvents))
			if budgetErr != nil {
				// Best-effort operator output; the command outcome is carried by its exit status.
				_, _ = fmt.Fprintf(stderr, "implement-now: task content budget: %v\n", budgetErr)
				return 2
			}
		}
	}
	var host executionHost
	var stateReader port.ProposalStateReader
	if codeBinding.Provider == integrations.GitHub {
		host = ghcodehost.NewGHWithTokenEnv(*repoSlug, *base, codeBinding.TokenEnv, writeBudget, codeGitHubHost)
		stateReader = ghcodehost.NewStateReaderWithTokenEnv(*repoSlug, codeBinding.TokenEnv, codeGitHubHost)
	}
	engineActor := ghLogin
	if codeBinding.Provider == integrations.Forgejo {
		client, err := forgejo.New(codeBinding.BaseURL, codeBinding.Token())
		if err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: Forgejo: %v\n", err)
			return 2
		}
		forgejoHost := forgejocodehost.New(client, codeBinding.BaseURL, codeBinding.Repository, *base)
		forgejoHost.DraftPrefix, forgejoHost.TrustedActors = codeBinding.DraftPrefix, cfg.FeedbackTrusted()
		host, stateReader = forgejoHost, forgejoHost
		actor, err := client.CurrentUser(ctx)
		if err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: authenticate Forgejo actor: %v\n", err)
			return 2
		}
		engineActor = actor.Login
	}
	ciMode := integrationCfg.CI.Mode
	if issueSession != nil {
		var ok bool
		issueSession.publisher, ok = host.(issueentry.SummaryPublisher)
		if !ok {
			_, _ = fmt.Fprintln(stderr, "implement-now: code host has no issue summary publisher")
			return 2
		}
	}
	if ciMode == "" && integrationCfg.Bindings.CodeHost != (integrations.Binding{}) {
		ciMode = repoCfg.Knobs.CIMode
	}
	host, err = withLocalCI(host, ciMode)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: verification: %v\n", err)
		return 2
	}
	active, err := loadActiveRouting(ctx, cfg, *repoPath, reviewRepository, *remote, *base)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: initialize routing-evidence activation: %v\n", err)
		return 2
	}
	defer active.Close()
	if *reviewOnly {
		return runSelectedReviews(ctx, reporter, stdout, stderr, cfg, hc, *repoPath, reviewRepository, *remote, *base, normalizedPlanRef, host, strings.TrimSpace(*driveRoundID), selectedReviews, logger, active)
	}
	// The five ports. The single Deps.Runtime is left nil: the resolver provides a
	// per-task runtime from the config's routing, so one sweep can span backends.
	space, err := workspace.New(ctx, workspace.Config{Parent: *repoPath, Remote: *remote, Mainline: *base})
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: prepare workspace adapter: %v\n", err)
		return 2
	}
	leaseAdapter, err := lease.New(lease.Config{WorkDir: *repoPath, Remote: *remote, LegacySourceKey: legacyClaimSource})
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: prepare lease adapter: %v\n", err)
		return 2
	}
	// Session continuity is also needed by an associated issue's CI-only fixer,
	// before the ordinary dispatcher is constructed.
	var continuityOpts []dispatch.Option
	var resumeBackend resumableBackendLookup
	var taskContinuity worker.SessionContinuity
	if cfg.SessionContinuity() {
		ledgerPath, pathErr := tasksession.DefaultPath(ctx, *repoPath)
		if pathErr != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: resolve task-session ledger: %v\n", pathErr)
			return 2
		}
		sessionStore, storeErr := tasksession.NewFileStore(ledgerPath)
		if storeErr != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: open task-session ledger: %v\n", storeErr)
			return 2
		}
		continuity, contErr := tasksession.New(sessionStore, reviewRepository)
		if contErr != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: build task session continuity: %v\n", contErr)
			return 2
		}
		taskContinuity = continuity
		continuityOpts = append(continuityOpts, dispatch.WithSessionContinuity(continuity))
		if issueSession == nil {
			continuityOpts = append(continuityOpts, dispatch.WithFeedbackDetection(stateReader, reviewRepository, engineActor, continuity, cfg.FeedbackTrusted()...))
		}
		resumeBackend = continuity.ResumableHarness
	}

	// Keep the existing local-CI and log-fetch collaborators on the convergence
	// factory; the issue wrapper only owns association and attribution writes.
	greenFactory := newGreenFactoryWithReservation(host, cfg, reporter, *remote, issueRepairReservation(issueSession))
	var issueReviews reviewobligation.Store
	if issueSession != nil {
		issueSession.collector.RunID = runID + "/"
		issueReviews, err = reviewobligation.NewGitStore(*repoPath, *remote, cfg.ReviewEvidenceRef())
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: issue review store: %v\n", err)
			return 2
		}
		reconciliationFinished := issueSession.logPhase("proposal_reconciliation")
		p, exists, readErr := issueSession.reconcileProposal(ctx, host, *repoPath, *remote)
		reconciliationFinished(readErr)
		if readErr != nil {
			_, _ = fmt.Fprintf(stderr, "implement-now: issue association: %v\n", readErr)
			return 1
		}
		if *recoverHead != "" {
			recovery := checkpointRecovery{host: host, space: space, lease: leaseAdapter, reviews: issueReviews, repository: reviewRepository, remote: *remote, revision: *recoverHead}
			return issueSession.recoverCheckpoint(ctx, recovery, p, exists, stdout, stderr)
		}
		if exists {
			// A remote-create interruption may have left this tracker unmarked.
			// No existing proposal can reenter implementation after reconciliation.
			selectedTracker.(*issueentry.ScopedTracker).Associated = true
			resolve := newRuntimeResolver(ctx, cfg, ambientBackend, hc, resumeBackend)
			continuationFinished := issueSession.logPhase("ci_continuation")
			continuationErr := issueSession.continueAssociatedCI(ctx, host, space, leaseAdapter, cfg, reporter, *remote, resolve, taskContinuity, *workerBudget, p)
			continuationFinished(continuationErr)
			if continuationErr != nil {
				_, _ = fmt.Fprintf(stderr, "implement-now: continue associated issue CI: %v\n", continuationErr)
				return issueSession.finish(ctx, host, issueReviews, reviewRepository, stdout, stderr)
			}
			states, stateErr := issueSession.currentReviewStates(ctx, host, issueReviews, reviewRepository, p)
			if stateErr != nil {
				_, _ = fmt.Fprintf(stderr, "implement-now: issue review state: %v\n", stateErr)
				return 1
			}
			if len(states) == 0 {
				inspect := issueVerifiedInspector{executionHost: host, session: issueSession}
				factory := newReviewFactory(cfg, hc, *repoPath, reviewRepository, *remote, *base, reviewScope, inspect, logger, active)
				if err := issueSession.captureMissingReview(ctx, p, cfg.ReviewHarness(), resolve, factory); err != nil {
					_, _ = fmt.Fprintf(stderr, "implement-now: recover issue review: %v\n", err)
					return issueSession.finish(ctx, host, issueReviews, reviewRepository, stdout, stderr)
				}
				states, stateErr = issueSession.currentReviewStates(ctx, host, issueReviews, reviewRepository, p)
				if stateErr != nil {
					_, _ = fmt.Fprintf(stderr, "implement-now: recovered issue review state: %v\n", stateErr)
					return 1
				}
			}
			selected := []string{}
			handoffPending := false
			for _, state := range states {
				if state.Status == "review_pending" || state.Status == "pair_verified" {
					selected = append(selected, state.Obligation.ID)
				}
				handoffPending = handoffPending || state.Status == "handoff_pending"
			}
			if len(selected) > 1 {
				_, _ = fmt.Fprintln(stderr, "implement-now: conflicting selected issue review obligations")
				return 1
			}
			if len(selected) == 1 {
				code := runSelectedReviews(ctx, reporter, stdout, stderr, cfg, hc, *repoPath, reviewRepository, *remote, *base, reviewScope, host, "issue-"+runID, selected, logger, active)
				final := issueSession.finish(ctx, host, issueReviews, reviewRepository, stdout, stderr)
				if code != 0 {
					return code
				}
				return final
			}
			if !handoffPending {
				return issueSession.finish(ctx, host, issueReviews, reviewRepository, stdout, stderr)
			}
		}
		host = &issueHost{executionHost: host, session: issueSession}
	}

	if selectedTracker == nil {
		var exitCode int
		selectedTracker, exitCode = tracker(ctx, taskBinding, repoPath, carinaBin, stderr, normalizedPlanRef, repoSlug, label, taskBudget, taskGitHubHost, trackerKind)
		if exitCode != 0 {
			return exitCode
		}
	}
	if issueSession == nil && (integrationCfg.Bindings.Tasks != (integrations.Binding{}) || integrationCfg.Bindings.CodeHost != (integrations.Binding{})) {
		selectedTracker = trackeridentity.WrapWithGuard(selectedTracker, taskBinding, func(ctx context.Context) error {
			return trackeridentity.CheckLegacy(ctx, *repoPath, *remote, codeBinding)
		})
	}
	reviewHarness := cfg.ReviewHarness()
	reviewFactory := newReviewFactory(cfg, hc, *repoPath, reviewRepository, *remote, *base, reviewScope, host, logger, active)
	handoffRepairer, err := newReviewHandoffRepairer(cfg, *repoPath, reviewRepository, *remote, reviewScope)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: build review handoff recovery: %v\n", err)
		reporter.Emit(report.RunStarted(*initiator, reviewScope))
		emitReviewResult(reporter, worker.ReviewResult{Phase: worker.ReviewPhasePreflight,
			Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationNotCreated,
			FailureClass: worker.ReviewFailureInfrastructure, Err: err}, "")
		rep := dispatch.RunReport{ReviewFailed: 1, ReviewInfrastructureFailed: 1}
		reporter.Emit(report.RunFinished(reviewCounts(rep)))
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprint(stdout, formatSummary(runID, rep, 1))
		return 1
	}
	if issueSession != nil {
		repairer, ok := handoffRepairer.(reviewHandoffRepairer)
		if !ok {
			_, _ = fmt.Fprintln(stderr, "implement-now: exact issue review recovery capability unavailable")
			return 1
		}
		repairer.store = issueReviewStore{Store: repairer.store, session: issueSession, repository: reviewRepository}
		handoffRepairer = repairer
	}

	// The local-first convergence loop (worker/prepublish.go): with a
	// configured review, each task is reviewed at its exact local revision
	// after the local gate and before any push, remediating on
	// request-changes within the configured bound, so the remote CI run is
	// spent once per candidate. `review.pre_publish: false` keeps review
	// post-green only.
	dispatchOpts := []dispatch.Option{
		dispatch.WithRuntimeResolver(newRuntimeResolver(ctx, cfg, ambientBackend, hc, resumeBackend)),
		dispatch.WithGreenDriverFactory(greenFactory),
		dispatch.WithReviewInvokerFactory(reviewFactory),
		dispatch.WithImplementationAdmissionFactory(active.implementationFactory()),
		dispatch.WithImplementationOutcomeFactory(active.implementationOutcomeFactory()),
		dispatch.WithReviewHarness(reviewHarness),
		dispatch.WithReviewHandoffRepairer(handoffRepairer),
		dispatch.WithReporter(reporter),
		dispatch.WithAttribution(*initiator, *scope),
		// The pre-claim duplicate-dispatch guard: a task whose proposal is
		// already open and not red is awaited, never restarted — whatever
		// invoked this sweep (operator, wake loop, or a scheduler that thinks
		// the task is still unstarted). The GitHub Host is both CodeHost and
		// ProposalProbe.
		dispatch.WithProposalProbe(host),
		// The loop's two code-host reads/writes: the exact head read that decides
		// whether the pre-publish review covers what was published, and the
		// proposal comment the covering artifact lands through.
		dispatch.WithProposalHeadReader(host),
		dispatch.WithReviewPublisher(host),
	}
	dispatchOpts = append(dispatchOpts, continuityOpts...)
	prePublish := cfg.ReviewPrePublish()
	if issueSession != nil {
		prePublish = false
		dispatchOpts = append(dispatchOpts, dispatch.WithDescribedCheckpointObserver(issueSession.retainCheckpoint))
	}
	d, err := dispatch.New(
		dispatch.Deps{Tracker: selectedTracker, Lease: leaseAdapter, Space: space, Host: host},
		dispatch.Config{Width: cfg.ParallelWidth(), ReviewWidth: cfg.ReviewWidth(), WorkerBudget: *workerBudget,
			PrePublishReview: prePublish, RemediationRounds: cfg.ReviewRemediationRounds()},
		dispatchOpts...,
	)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: build dispatcher: %v\n", err)
		return 2
	}

	if issueSession != nil {
		issueSession.phase = "implementation"
	}
	rep, err := d.Sweep(ctx)
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: sweep: %v\n", err)
		return 2
	}

	escalations := 0
	if argusSink != nil {
		escalations = argusSink.Escalations()
	}
	// Best-effort operator output; the command outcome is carried by its exit status.
	_, _ = fmt.Fprint(stdout, formatSummary(runID, rep, escalations))
	if issueSession != nil {
		final := issueSession.finish(ctx, host, issueReviews, reviewRepository, stdout, stderr)
		if runExitCode(rep) != 0 {
			return runExitCode(rep)
		}
		return final
	}
	// A withheld-because-unreadable frontier reddens the run exactly as a failed
	// task does: the sweep found ready work and could not judge it, which must
	// never exit 0 and read as a clean, idle pass (AP-16).
	return runExitCode(rep)
}

func tracker(ctx context.Context,
	taskBinding integrations.ResolvedRepository,
	repoPath *string, carinaBin *string, stderr io.Writer,
	normalizedPlanRef string, repoSlug *string, label *string,
	taskBudget *budget.Budget, taskGitHubHost string, trackerKind *string) (port.WorkTracker, int) {
	var tracker port.WorkTracker
	switch taskBinding.Provider {
	case "carina":
		root := strings.TrimSpace(taskBinding.Root)
		if root == "" {
			root = *repoPath
		}
		bin, binErr := resolveCarinaBinary(ctx, *repoPath, *carinaBin)
		if binErr != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: resolve carina: %v\n", binErr)
			return nil, 2
		}
		if normalizedPlanRef == "" {
			tracker = carinatracker.New(root, carinatracker.WithBinary(bin), carinatracker.WithRepository(*repoSlug))
			break
		}
		scoped, scopeErr := carinatracker.NewForPlan(root, normalizedPlanRef, carinatracker.WithBinary(bin), carinatracker.WithRepository(*repoSlug))
		if scopeErr != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: build plan-scoped tracker: %v\n", scopeErr)
			return nil, 2
		}
		tracker = scoped
	case integrations.Forgejo:
		client, err := forgejo.New(taskBinding.BaseURL, taskBinding.Token())
		if err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: task tracker: %v\n", err)
			return nil, 2
		}
		tracker, err = forgejotracker.New(client, taskBinding.Repository, *label, normalizedPlanRef)
		if err != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: task tracker: %v\n", err)
			return nil, 2
		}
	case integrations.GitHub:
		if normalizedPlanRef == "" {
			tracker = ghtracker.NewGHWithTokenEnv(taskBinding.Repository, *label, taskBinding.TokenEnv, taskBudget, taskGitHubHost)
			break
		}
		scoped, scopeErr := ghtracker.NewGHForPlanWithTokenEnv(taskBinding.Repository, *label, normalizedPlanRef, taskBinding.TokenEnv, taskBudget, taskGitHubHost)
		if scopeErr != nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: build plan-scoped tracker: %v\n", scopeErr)
			return nil, 2
		}
		tracker = scoped
	default:
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: unknown --tracker %q (want gh or carina)\n", *trackerKind)
		return nil, 2
	}
	return tracker, 0
}

func reportDeliveryErrorLogger(logger *slog.Logger) func(report.Sink, report.Event, error) {
	return func(_ report.Sink, e report.Event, err error) {
		logger.Warn("event delivery failed", slog.String("kind", string(e.Kind)), slog.Any("err", err))
	}
}

func contentBudgetObserver(logger *slog.Logger, events *watch.Sink) func(budget.Event) {
	return func(e budget.Event) {
		if e.Granted {
			logger.Info("github content write proceeding after backpressure",
				slog.Int("queue_depth", e.QueueDepth), slog.Duration("waited", e.Deferred))
			return
		}
		// The projected wait, the blocking ledger and the reset deadline —
		// never elapsed time, which is indistinguishable from a hang (DINV-8).
		logger.Warn("github content write deferred",
			slog.String("ledger", string(e.Ledger)),
			slog.Int("queue_depth", e.QueueDepth),
			slog.Duration("deferred", e.Deferred),
			slog.Time("reset_at", e.ResetAt))
		events.Append(watch.Event{
			Kind:       watch.EventThrottleDeferred,
			Medium:     "github",
			Ledger:     string(e.Ledger),
			QueueDepth: e.QueueDepth,
			DeferredMS: watch.ProjectedWaitMillis(e.Deferred),
			ResetAt:    e.ResetAt,
		})
	}
}

func runSelectedReviews(ctx context.Context, emitter report.Emitter, stdout, stderr io.Writer, cfg config.Config, hc harnessconfig.Config, repoDir, repoSlug, remote, mainline, scope string, inspect reviewinvoke.ProposalInspector, driveRoundID string, selected []string, logger *slog.Logger, active *activeRouting) int {
	emitter.Emit(report.RunStarted("drive-wake", repoSlug))
	store, err := reviewobligation.NewGitStore(repoDir, remote, cfg.ReviewEvidenceRef())
	if err != nil {
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprintf(stderr, "implement-now: review store: %v\n", err)
		for _, obligationID := range selected {
			emitReviewResult(emitter, worker.ReviewResult{DriveRoundID: driveRoundID,
				ObligationID: obligationID, Phase: worker.ReviewPhasePreflight,
				Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending,
				FailureClass: worker.ReviewFailureInfrastructure, Err: err}, "")
		}
		rep := dispatch.RunReport{ReviewFailed: len(selected), ReviewPending: len(selected), ReviewInfrastructureFailed: len(selected)}
		emitter.Emit(report.RunFinished(reviewCounts(rep)))
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprint(stdout, formatSummary("review-recovery-"+driveRoundID, rep, len(selected)))
		return 1
	}
	seen := map[string]bool{}
	states := make([]reviewobligation.State, 0, len(selected))
	for _, id := range selected {
		if seen[id] {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: duplicate review obligation %q\n", id)
			return 2
		}
		seen[id] = true
		state, ok, readErr := store.State(ctx, id)
		o := state.Obligation
		if readErr != nil || !ok || o.Repository != repoSlug || o.Scope != scope {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: selected review obligation %q is unavailable or out of scope: %v\n", id, readErr)
			return 1
		}
		if state.Status != "review_pending" && state.Status != "pair_verified" {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: selected review obligation %q is not pending: %v\n", id, readErr)
			return 1
		}
		states = append(states, state)
	}

	rep := dispatch.RunReport{}
	obligations := make([]reviewobligation.Obligation, 0, len(states))
	var pairRepairer *reviewinvoke.PairRepairer
	var pairRepairSetupErr error
	for _, state := range states {
		if state.Status == "review_pending" {
			obligations = append(obligations, state.Obligation)
			continue
		}
		pair := state.Pair
		if pair == nil {
			// Best-effort operator output; the command outcome is carried by its exit status.
			_, _ = fmt.Fprintf(stderr, "implement-now: selected pair-verified obligation %q lacks its publication pair\n", state.Obligation.ID)
			return 1
		}
		if pairRepairer == nil && pairRepairSetupErr == nil {
			pairRepairer, pairRepairSetupErr = newReviewPairRepairer(cfg, repoDir, repoSlug, remote)
			if pairRepairSetupErr == nil {
				pairRepairSetupErr = pairRepairer.Preflight(ctx, repoSlug)
			}
		}
		var result worker.ReviewResult
		var repairErr error
		if pairRepairSetupErr != nil {
			repairErr = pairRepairSetupErr
			result = worker.ReviewResult{DriveRoundID: driveRoundID, AttemptID: pair.AttemptID,
				ObligationID: pair.ObligationID, Harness: state.Obligation.Harness,
				ProposalRef: pair.ProposalRef, Revision: pair.Revision, Phase: worker.ReviewPhaseObligation,
				Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending,
				FailureClass: worker.ReviewFailureInfrastructure}
		} else {
			result, repairErr = pairRepairer.Repair(ctx, state.Obligation, *pair, driveRoundID)
		}
		if repairErr != nil {
			result.Err = repairErr
			rep.ReviewFailed++
			rep.ReviewPending++
			if result.FailureClass == worker.ReviewFailureArtifactUntrusted {
				rep.ReviewArtifactUntrusted++
			} else {
				rep.ReviewInfrastructureFailed++
			}
		} else {
			rep.ReviewCompleted++
		}
		rep.Tokens += result.Tokens
		emitReviewResult(emitter, result, state.Obligation.TaskID)
	}

	invokers := map[entity.Harness]worker.ReviewService{}
	setupErrs := map[entity.Harness]error{}
	factory := newReviewFactory(cfg, hc, repoDir, repoSlug, remote, mainline, scope, inspect, logger, active)
	for _, o := range obligations {
		if _, done := invokers[o.Harness]; done || setupErrs[o.Harness] != nil {
			continue
		}
		if !cfg.ReviewConfigured() {
			setupErrs[o.Harness] = errors.New("executor.review is not configured")
			continue
		}
		if factory == nil {
			setupErrs[o.Harness] = errors.New("executor.review is not configured")
			continue
		}
		inv, buildErr := factory(o.Harness)
		if buildErr == nil {
			buildErr = inv.Preflight(ctx)
		}
		if buildErr != nil {
			setupErrs[o.Harness] = buildErr
			continue
		}
		invokers[o.Harness] = inv
	}
	if len(setupErrs) != 0 {
		for _, o := range obligations {
			setupErr := setupErrs[o.Harness]
			if setupErr == nil {
				setupErr = errors.New("review preflight union failed before admission")
			}
			emitReviewResult(emitter, worker.ReviewResult{DriveRoundID: driveRoundID, ObligationID: o.ID, Harness: o.Harness, ProposalRef: o.ProposalRef, Revision: o.Revision, Phase: worker.ReviewPhasePreflight, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: setupErr}, o.TaskID)
		}
		failed := len(obligations)
		rep.ReviewFailed += failed
		rep.ReviewPending += failed
		rep.ReviewInfrastructureFailed += failed
		emitter.Emit(report.RunFinished(reviewCounts(rep)))
		// Best-effort operator output; the command outcome is carried by its exit status.
		_, _ = fmt.Fprint(stdout, formatSummary("review-recovery-"+driveRoundID, rep, failed))
		return 1
	}

	for _, o := range obligations {
		inv := invokers[o.Harness]
		task, proposal, prepareErr := inv.PrepareSelected(ctx, o.ID, driveRoundID)
		var result worker.ReviewResult
		if prepareErr != nil {
			result = worker.ReviewResult{DriveRoundID: driveRoundID, ObligationID: o.ID, Harness: o.Harness, ProposalRef: o.ProposalRef, Revision: o.Revision, Phase: worker.ReviewPhaseObligation, Status: report.ReviewStatusFailed, ObligationStatus: report.ReviewObligationPending, FailureClass: worker.ReviewFailureInfrastructure, Err: prepareErr}
		} else {
			result = inv.Invoke(ctx, task, proposal)
			result.DriveRoundID = driveRoundID
		}
		emitReviewResult(emitter, result, o.TaskID)
		if result.Failed() {
			rep.ReviewFailed++
			rep.ReviewPending++
			switch result.FailureClass {
			case worker.ReviewFailureArtifactUnverified:
				rep.ReviewArtifactUnverified++
			case worker.ReviewFailureArtifactUntrusted:
				rep.ReviewArtifactUntrusted++
			default:
				rep.ReviewInfrastructureFailed++
			}
		} else {
			rep.ReviewCompleted++
		}
		rep.Tokens += result.Tokens
	}
	emitter.Emit(report.RunFinished(reviewCounts(rep)))
	// Best-effort operator output; the command outcome is carried by its exit status.
	_, _ = fmt.Fprint(stdout, formatSummary("review-recovery-"+driveRoundID, rep, rep.ReviewFailed))
	return runExitCode(rep)
}

func emitReviewResult(emitter report.Emitter, r worker.ReviewResult, taskID string) {
	reason := ""
	if r.Err != nil {
		reason = r.Err.Error()
	}
	if r.Publication != nil {
		if p := r.ImplReviewTerminal; p != nil {
			emitter.Emit(report.ReviewArtifactPublicationTerminal(report.ReviewArtifactPublicationTerminalFields{
				ProcessID: p.ReviewProcessID, ObligationID: p.ReviewObligationID,
				PublicationAttemptID: p.PublicationAttemptID, ProposalID: p.ProposalID,
				ReviewedRevision: p.ReviewedRevision, Medium: p.Medium, IdempotencyID: p.IdempotencyID,
				Status: string(p.Status), ArtifactRef: p.ArtifactRef, NativeReceipt: p.NativeReceipt,
				AttemptProofRef: p.AttemptProofRef, FailureClass: string(p.FailureClass), Reason: p.Reason,
			}))
		}
		p := r.Publication
		emitter.Emit(report.HostedReviewPublicationTerminal(report.HostedReviewPublicationTerminalFields{
			AttemptID: p.ReviewAttemptID, ObligationID: p.ReviewObligationID,
			ProposalRef: p.ProposalRef, Revision: p.Revision, Status: p.Status,
			ArtifactRef: p.ArtifactRef, NativeReceipt: p.NativeReceipt,
			AttemptProofRef: p.AttemptProofRef, FailureClass: p.FailureClass,
			Reason: p.Reason, Escalated: p.Escalated,
		}))
	}
	emitter.Emit(report.ImplReviewTerminal(report.ImplReviewTerminalFields{DriveRoundID: r.DriveRoundID, AttemptID: r.AttemptID, ObligationID: r.ObligationID, TaskID: taskID, ProposalRef: r.ProposalRef, Revision: r.Revision, Harness: r.Harness, ReviewerPrincipal: r.ReviewerPrincipal, NativeProfile: r.NativeProfileID, Catalog: r.CatalogID, Policy: r.PolicyIdentity, Model: r.Model, Effort: r.Effort, Phase: r.Phase, Status: r.Status, ObligationStatus: r.ObligationStatus, FailureClass: r.FailureClass, ArtifactRef: r.ArtifactRef, ArtifactProofRef: r.ArtifactProofRef, Reason: reason, Escalated: r.Failed()}))
}

func reviewCounts(rep dispatch.RunReport) report.Counts {
	return report.Counts{Tokens: rep.Tokens, ReviewCompleted: rep.ReviewCompleted, ReviewFailed: rep.ReviewFailed, ReviewPending: rep.ReviewPending, ReviewArtifactUnverified: rep.ReviewArtifactUnverified, ReviewArtifactUntrusted: rep.ReviewArtifactUntrusted, ReviewInfrastructureFailed: rep.ReviewInfrastructureFailed}
}

func runExitCode(rep dispatch.RunReport) int {
	if rep.Failed > 0 || rep.ProbeUnavailable > 0 || rep.ReviewFailed > 0 {
		return 1
	}
	return 0
}

func resolveAmbientBackend(flagValue string, getenv func(string) string) string {
	if backend := strings.TrimSpace(flagValue); backend != "" {
		return backend
	}
	return strings.TrimSpace(getenv("PYXIS_AGENT_BACKEND"))
}

// buildSinks builds the reporting sinks: the operator live view (always — the
// attended progress log) plus the argus JSONL journal when a journal writer is
// configured. The argus sink is returned separately so the caller can read its
// fail-closed escalation count for the summary. Its escalation handler logs a
// prominent warning — argus's real escalation-brief/wake surfaces are argus's own
// (out of scope here; this feeds its journal, §3.9).
func buildSinks(logger *slog.Logger, journal, flow io.Writer) ([]report.Sink, *report.ArgusSink) {
	sinks := []report.Sink{report.NewOperatorSink(logger), report.NewFlowSink(flow)}
	if journal == nil {
		return sinks, nil
	}
	argus := report.NewArgusSink(journal, report.WithEscalationHandler(func(e report.Event) {
		reason := e.Reason
		logger.Warn("fail-closed escalation — human attention required",
			slog.String("task", e.TaskID), slog.Int("issue", e.Issue), slog.String("reason", reason))
	}))
	return append(sinks, argus), argus
}

func featureName(scope string) string {
	scope = strings.Trim(strings.TrimSpace(scope), "/")
	if before, _, ok := strings.Cut(scope, "/"); ok {
		return before
	}
	return scope
}

// resolveRepo turns --repo into an absolute checkout root. The default "." is
// the process cwd, which for `go test` and the Go debugger is the package
// directory (cmd/implement-now), not the operand; that would write
// .pyxis/logs under the package. Empty/default values use the shared
// reporoot discovery (nearest .pyxis/config.yaml ancestor, else git toplevel).
// An explicit --repo stays as given so hermetic tests keep their temp dirs.
func resolveRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" || repo == "." {
		return reporoot.Find()
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// resolveCarinaBinary returns the carina executable. An explicit --carina-bin
// wins; otherwise the checkout's source or release resolution is used so a
// debug session and every worktree find ./cmd/carina without a global install.
func resolveCarinaBinary(ctx context.Context, repo, override string) (string, error) {
	if bin := strings.TrimSpace(override); bin != "" && bin != "carina" {
		return bin, nil
	}
	return toolbin.Ensure(ctx, repo, "carina")
}

// openJournalFor resolves and opens the local event journal.
// Both attended and unattended runs retain their event stream
// under the repository's ignored log directory. The returned closer is always safe to call.
func openJournalFor(journalPath, repoPath string) (io.Writer, func(), error) {
	path, err := runlogs.Resolve(context.Background(), repoPath, defaultJournalRel, strings.TrimSpace(journalPath))
	if err != nil {
		return nil, func() {}, err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, func() {}, fmt.Errorf("create journal dir %q: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open journal %q: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

// formatSummary renders the classed final report (§3.9) — the headline counts, a
// line per failed task, and the fail-closed escalation count. It is a pure function over the report, so it is
// unit-tested directly.
func formatSummary(runID string, rep dispatch.RunReport, escalations int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "implement-now: run %s finished\n", runID)
	fmt.Fprintf(&b, "  succeeded: %d\n  failed:    %d\n  blocked:   %d\n  skipped:   %d\n",
		rep.Succeeded, rep.Failed, rep.Blocked, rep.Skipped)
	if rep.AwaitingProposal > 0 {
		fmt.Fprintf(&b, "  awaiting-proposal: %d (already proposed — not restarted)\n", rep.AwaitingProposal)
	}
	if rep.ProbeUnavailable > 0 {
		fmt.Fprintf(&b, "  probe-unavailable: %d (preflight unreadable — withheld, NOT idle)\n", rep.ProbeUnavailable)
	}
	if rep.PrePublishReviewed > 0 {
		fmt.Fprintf(&b, "  pre-publish-reviewed: %d (reviewed at the local revision before the push; no post-green review owed)\n", rep.PrePublishReviewed)
	}
	if rep.FeedbackRounds > 0 {
		fmt.Fprintf(&b, "  feedback-rounds: %d (human review activity on open proposals answered by the tasks' own sessions)\n", rep.FeedbackRounds)
	}
	if rep.ReviewCompleted > 0 || rep.ReviewFailed > 0 {
		fmt.Fprintf(&b, "  review-completed: %d\n  review-failed:    %d\n", rep.ReviewCompleted, rep.ReviewFailed)
		fmt.Fprintf(&b, "  review-pending:   %d\n  review-artifact-unverified: %d\n  review-artifact-untrusted: %d\n  review-infrastructure-failed: %d\n",
			rep.ReviewPending, rep.ReviewArtifactUnverified, rep.ReviewArtifactUntrusted, rep.ReviewInfrastructureFailed)
	}
	if rep.NothingReady() {
		b.WriteString("  (nothing ready — empty frontier; zero model calls)\n")
	}
	for _, r := range rep.Results {
		reason := ""
		if r.Err != nil {
			reason = ": " + r.Err.Error()
		}
		switch r.Class {
		case dispatch.ClassFailed:
			fmt.Fprintf(&b, "  FAILED %s (#%d)%s\n", r.Task.ID, r.Task.Number, reason)
		case dispatch.ClassAwaitingProposal:
			fmt.Fprintf(&b, "  AWAITING-PROPOSAL %s (#%d)%s\n", r.Task.ID, r.Task.Number, reason)
		case dispatch.ClassProbeUnavailable:
			fmt.Fprintf(&b, "  PROBE-UNAVAILABLE %s (#%d)%s\n", r.Task.ID, r.Task.Number, reason)
		}
		if r.Review != nil && r.Review.Failed() {
			fmt.Fprintf(&b, "  REVIEW-FAILED %s (#%d) [%s/%s]: %v\n",
				r.Task.ID, r.Task.Number, r.Review.Harness, r.Review.Phase, r.Review.Err)
		}
	}
	if escalations > 0 {
		fmt.Fprintf(&b, "  fail-closed escalations: %d — see the argus journal\n", escalations)
	}
	return b.String()
}
