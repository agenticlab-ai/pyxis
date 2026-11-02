package main

// The watch verb — the thin shipped host of the wake loop
// (design:wake-watch-v2 §2/§3.1; plan self-drive-wake/001 T8). It derives the
// awaited set from durable state each tick (plan mapping + tracker facts +
// batched proposal reads), and a wake's only effect is invoking the
// executor's existing one-sweep entry as a subprocess — the deployed public
// entry, so the sweep's own wiring (claims, worktrees, drive-to-green,
// journal) is never re-implemented here (AP-11; basis NR-1: the executor is
// invoked, never modified). Every check and wake lands in the local,
// gitignored wake-events sink before its effect proceeds (DINV-8), and the
// verb holds no claims, opens no proposals, merges nothing (DINV-4).

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/github"
	"github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/runlog"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

// watchDeps are the verb's injectable seams — swapped by the CLI tests the
// way the sibling verbs swap their engines.
var (
	watchRunLive         = watch.RunLive
	watchRunCombinedLive = watch.RunCombinedLive
	// listClaims is the claim-ref lister (git ls-remote), a package var so a
	// test can substitute it without a real git remote — the same seam idiom as
	// watchRunLive/statusNow. Both plan sources read claims through it.
	listClaims = claimedIssues
	watchExit  = func() <-chan os.Signal {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		return ch
	}
)

func watchVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "local repo checkout the sweep runs in")
	repoSlug := fs.String("repo-slug", "", "owner/name on the code host (required)")
	label := fs.String("label", "local-agent", "tracker label the projected plan routes on")
	planDir := fs.String("plan", "", "plan directory (e.g. docs/plans/<feature>) — required")
	ambient := fs.String("ambient-backend", "", "invoking harness backend forwarded to every implement-now sweep; fallback: PYXIS_AGENT_BACKEND")
	trackerFlag := fs.String("tracker", "", "work-item medium: gh|carina (default: repo config knobs.tracker — the runtime choice #895 forwards to every sweep)")
	carinaRootFlag := fs.String("carina-root", "", "carina store root when the tracker is carina (default: repo config work_items, else --repo)")
	sweepCmd := fs.String("sweep-cmd", "", "override the sweep command (tests); default: this binary's exec grammar running implement-now")
	reportCmd := fs.String("report-cmd", "", "override the plan-report command (tests); default: the exec grammar running plangate report")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	// I3's guard, inherited: a flag-looking positional is a usage refusal.
	for _, a := range fs.Args() {
		if strings.HasPrefix(a, "-") {
			terminalPrintf(stderr, "pyxis: watch: flag %q after positional arguments — flags precede positionals\n", a)
			return 2
		}
	}
	if len(fs.Args()) > 0 {
		terminalPrintf(stderr, "pyxis: watch: unexpected positional %q\n", fs.Args()[0])
		return 2
	}
	icfg, integrationErr := integrations.Load(*repo)
	if integrationErr != nil {
		terminalPrintf(stderr, "pyxis: watch: %v\n", integrationErr) // Best-effort diagnostic; exit status carries failure.
		return 2
	}
	if *planDir == "" || (*repoSlug == "" && icfg.Bindings.CodeHost == (integrations.Binding{})) {
		terminalPrintln(stderr, "pyxis: watch: --plan and --repo-slug are required")
		return 2
	}

	// The claim remote resolves from the repo's own config — never a
	// target-facing literal (config-driven-portability DINV-1); a default
	// config yields "origin", byte-identical.
	repoCfg, configDefects := repoconfig.Load(*repo)
	probe, admissionErr := localCodeHostDiagnostics(*repo, repoCfg)
	if admissionErr == nil && probe != nil {
		if len(configDefects) != 0 {
			admissionErr = fmt.Errorf("local Git configuration: %v", configDefects)
		} else {
			admissionErr = codehost.RequireWorkflow(context.Background(), probe)
		}
	}
	if admissionErr != nil {
		terminalPrintf(stderr, "pyxis: watch: code host: %v\n", admissionErr)
		return 2
	}
	remote := repoCfg.Remote
	executorCfg, executorCfgErr := executorconfig.Load(*repo)
	if executorCfgErr != nil {
		terminalPrintf(stderr, "pyxis: watch: load executor config: %v\n", executorCfgErr)
		return 2
	}
	thresholds, thresholdErr := watch.LoadThresholds(*repo)
	if thresholdErr != nil {
		terminalPrintf(stderr, "pyxis: watch: %v\n", thresholdErr)
		return 2
	}

	// The work-item medium is the runtime choice #895 forwards to every sweep:
	// it comes from repo config (knobs.tracker), so `/implement-now <feature>`
	// invokes the SAME `pyxis watch` command regardless of medium and the
	// runtime picks GitHub or Carina. An explicit --tracker overrides.
	trackerKind, carinaRoot, terr := resolveTracker(*trackerFlag, *carinaRootFlag, *repo, repoCfg)
	if terr != nil {
		terminalPrintf(stderr, "pyxis: watch: %v\n", terr)
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-watchExit()
		cancel()
	}()

	sink := watch.NewSink(filepath.Join(*repo, watch.DefaultEventsDir), 0, nil)

	mapping, err := planMapping(ctx, *repo, *planDir, trackerKind, carinaRoot, *reportCmd)
	if err != nil {
		terminalPrintf(stderr, "pyxis: watch: %v\n", err)
		return 1
	}
	scope := mapping.planRef
	flowLog, flowLogPath, err := runlog.Open(*repo, executorCfg.RunLogDirectory(), featureName(scope), time.Now())
	if err != nil {
		// Best-effort diagnostic; the exit code still reports the failure.
		terminalPrintf(stderr, "pyxis: watch: open execution log: %v\n", err)
		return 1
	}
	defer func() {
		// Log cleanup must not replace the watch result.
		_ = flowLog.Close()
	}()
	// Best-effort telemetry; the durable watch state governs recovery.
	_ = runlog.WriteLine(flowLog, time.Now(), "plan %s watch started with %d task(s)", scope, len(mapping.issueByTask))
	// A failed announcement does not prevent writing the execution log.
	terminalPrintf(stdout, "watch: execution log %s\n", flowLogPath)

	// One reader + one plan source, selected by medium. Both poll the SELECTED
	// tracker for the frontier's durable state (Carina reads the local store;
	// GitHub reads issues/PRs/claims); the sweep the wake invokes carries the
	// same medium selection, so watch and its sweeps never disagree on the
	// tracker (#895).
	var reader *watch.InstrumentReader
	var source watch.PlanSource
	var cadence time.Duration
	configuredReader, configuredSource, configuredMedium, configured, integrationErr := configuredWatch(*repo, *repoSlug, trackerKind, carinaRoot, repoCfg, mapping, true)
	if integrationErr != nil {
		terminalPrintf(stderr, "pyxis: watch: %v\n", integrationErr) // Best-effort diagnostic; exit status carries failure.
		return 2
	}
	if configured {
		reader = &watch.InstrumentReader{Inner: configuredReader, Sink: sink, Medium: configuredMedium}
		source, cadence = configuredSource, configuredReader.cadence
	} else {
		switch trackerKind {
		case "carina":
			sr := carinatracker.NewStateReader(carinaRoot)
			reader = &watch.InstrumentReader{Inner: carinaCadenceAdapter{sr}, Sink: sink, Medium: "carina"}
			source = carinaPlanSource(*repo, remote, sr, mapping)
			cadence = carinatracker.AdviceCadence
		default: // gh
			sr := ghcodehost.NewStateReader(*repoSlug)
			reader = &watch.InstrumentReader{Inner: cadenceAdapter{sr}, Sink: sink, Medium: "github"}
			source = ghPlanSource(*repo, *repoSlug, *label, remote, mapping)
			cadence = sr.Cadence()
		}
	}

	ambientBackend := strings.TrimSpace(*ambient)
	if ambientBackend == "" {
		ambientBackend = strings.TrimSpace(os.Getenv("PYXIS_AGENT_BACKEND"))
	}
	sweep := watch.WrapSweep(sink, scope, subprocessSweep(sweepArgs{
		override: *sweepCmd, repo: *repo, repoSlug: *repoSlug, label: *label,
		scope: scope, ambientBackend: ambientBackend, trackerKind: trackerKind, carinaRoot: carinaRoot,
	}, stdout, stderr))

	terminalPrintf(stdout, "watch: %s — %d tasks · backend %s · cadence %s\n", scope, len(mapping.issueByTask), trackerKind, cadence)
	// The push slot ships reserved and empty in v1 (design:wake-watch-v2#3.7,
	// watch.PushSlot): no adapter exists yet, so no hints channel is wired —
	// behavior is cadence-only, byte-identical to a dead adapter.
	var sum watch.Summary
	if executorCfg.ReviewConfigured() {
		providerStore, storeErr := reviewobligation.NewGitStore(*repo, remote, executorCfg.ReviewEvidenceRef())
		if storeErr != nil {
			terminalPrintf(stderr, "pyxis: watch: review obligation store: %v\n", storeErr)
			return 1
		}
		recoveryLedger, ledgerErr := watch.NewGitRecoveryLedger(*repo, remote, thresholds.ReviewRecoveryRef)
		if ledgerErr != nil {
			terminalPrintf(stderr, "pyxis: watch: review recovery ledger: %v\n", ledgerErr)
			return 1
		}
		reviewStates := watch.ReviewStateReader{Provider: reviewProviderAdapter{store: providerStore}, Recovery: recoveryLedger}
		reviewSweep := subprocessReviewSweep(sweepArgs{
			override: *sweepCmd, repo: *repo, repoSlug: *repoSlug, label: *label,
			scope: scope, ambientBackend: ambientBackend, trackerKind: trackerKind, carinaRoot: carinaRoot,
		}, stdout, stderr)
		sum, err = watchRunCombinedLive(ctx, source, reader, reviewStates, watch.RecoveryPolicy{Bound: thresholds.ReviewRecoveryBound}, scope, sweep, reviewSweep, watch.NewRoundID, sink, nil, nil)
	} else {
		sum, err = watchRunLive(ctx, source, reader, sweep, nil, nil)
	}
	if err != nil {
		if ctx.Err() != nil {
			// Best-effort telemetry; the durable watch state governs recovery.
			_ = runlog.WriteLine(flowLog, time.Now(), "plan %s watch stopped; %d task wait(s) remain and the next run will recover", scope, sum.Awaits)
			terminalPrintf(stdout, "watch: stopped — %d sweep(s), %d poll(s), %d await(s) outstanding; state is durable, re-run to continue\n", sum.Sweeps, sum.Polls, sum.Awaits)
			return 0
		}
		// Preserve the watch failure even if its diagnostic cannot be written.
		_ = runlog.WriteLine(flowLog, time.Now(), "plan %s watch failed: %v", scope, err)
		terminalPrintf(stderr, "pyxis: watch: %v\n", err)
		return 1
	}
	// Best-effort telemetry; a write failure cannot undo completed tasks.
	_ = runlog.WriteLine(flowLog, time.Now(), "plan %s complete: every task is merged and configured review is verified", scope)
	terminalPrintf(stdout, "watch: complete — every plan task closed and configured review verified (%d sweep(s) delivered)\n", sum.Sweeps)
	return 0
}

// resolveTracker picks the work-item medium: an explicit --tracker wins,
// otherwise repo config's knobs.tracker ("github"→gh, "carina"→carina), else
// gh. It also resolves the Carina store root (--carina-root, else config
// work_items, else the repo). Returns "gh" or "carina" — the exact tokens
// implement-now and plangate accept — so watch forwards a valid selection.
func resolveTracker(flagVal, carinaRootFlag, repo string, cfg repoconfig.Config) (kind, carinaRoot string, err error) {
	sel := strings.ToLower(strings.TrimSpace(flagVal))
	if sel == "" {
		sel = strings.ToLower(strings.TrimSpace(cfg.Knobs.Tracker))
	}
	switch sel {
	case "carina":
		kind = "carina"
	case "", "github", "gh":
		kind = "gh"
	default:
		return "", "", fmt.Errorf("unknown tracker %q (want gh|carina)", sel)
	}
	carinaRoot = strings.TrimSpace(carinaRootFlag)
	if carinaRoot == "" {
		carinaRoot = strings.TrimSpace(cfg.WorkItems)
	}
	if carinaRoot == "" {
		carinaRoot = repo
	}
	return kind, carinaRoot, nil
}

func featureName(scope string) string {
	scope = strings.Trim(strings.TrimSpace(scope), "/")
	if before, _, ok := strings.Cut(scope, "/"); ok {
		return before
	}
	return scope
}

// carinaCadenceAdapter lifts the Carina reader onto the loop's ctx-taking
// Reader shape, declaring the Carina adapter's 1-second advice.
type carinaCadenceAdapter struct{ *carinatracker.StateReader }

func (carinaCadenceAdapter) Cadence(context.Context) time.Duration {
	return carinatracker.AdviceCadence
}

// carinaPlanSource builds the per-tick durable facts from the LOCAL Carina
// store — mirroring ghPlanSource exactly, medium-swapped: an item's state
// stands in for issue open/closed (the reader's landing), claims are the same
// git refs (one ls-remote), and a claimed open item is its own in-flight
// "proposal". Exact reads each tick, no loop memory (AP-15; DINV-5). Closed is
// "not open" — any terminated item, matching ghPlanSource treating every
// closed issue as done regardless of reason.
func carinaPlanSource(repo, remote string, sr *carinatracker.StateReader, m planMap) watch.PlanSource {
	return func(ctx context.Context) (watch.Plan, error) {
		refs := make([]port.ProposalRef, 0, len(m.issueByTask))
		order := make([]string, 0, len(m.issueByTask))
		for task, item := range m.issueByTask {
			refs = append(refs, port.ProposalRef{Number: item})
			order = append(order, task)
		}
		states, err := sr.States(ctx, refs, port.Cursor{})
		if err != nil {
			return watch.Plan{}, fmt.Errorf("read carina item states: %w", err)
		}
		claims, err := listClaims(ctx, repo, remote)
		if err != nil {
			return watch.Plan{}, err
		}
		var plan watch.Plan
		for i, task := range order {
			item := m.issueByTask[task]
			open := states[i].Unknown == nil && states[i].Landing == port.LandingOpen
			t := watch.Task{ID: task, BlockedBy: m.blockedBy[task], Closed: !open, Claimed: claims[item]}
			if open && claims[item] {
				t.Proposal = &port.ProposalRef{Number: item} // the in-flight item is its own proposal
			}
			plan.Tasks = append(plan.Tasks, t)
		}
		return plan, nil
	}
}

// cadenceAdapter lifts the adapter's advisory Cadence() onto the loop's
// ctx-taking Reader shape.
type cadenceAdapter struct{ *ghcodehost.StateReader }

func (c cadenceAdapter) Cadence(context.Context) time.Duration { return c.StateReader.Cadence() }

// planMap holds the projection's frozen task↔issue mapping plus the DAG edges
// from the plan document — both immutable after the plan PR merges, so read
// once at startup.
type planMap struct {
	planRef     string
	issueByTask map[string]int
	blockedBy   map[string][]string
}

// planMapping reads the projection report (task_id → issue) through the
// plangate report verb and the DAG edges from the plan JSON beside the plan
// document. plangate owns the mapping (one predicate, one place) — the verb
// never re-derives it from tracker search.
func planMapping(ctx context.Context, repo, planDir, trackerKind, carinaRoot, override string) (planMap, error) {
	reportArgs := []string{"report", "--tracker", trackerKind}
	if trackerKind == "carina" {
		reportArgs = append(reportArgs, "--carina-root", carinaRoot)
	}
	reportArgs = append(reportArgs, planDir)
	out, err := runTool(ctx, repo, override, "plangate", reportArgs...)
	if err != nil {
		return planMap{}, fmt.Errorf("plan report: %w", err)
	}
	var rep struct {
		Feature string `json:"feature"`
		Version string `json:"plan_version"`
		Items   []struct {
			TaskID string `json:"task_id"`
			Issue  int    `json:"issue"`
		} `json:"items"`
	}
	if err := json.Unmarshal(findJSON(out), &rep); err != nil {
		return planMap{}, fmt.Errorf("parse plan report: %w", err)
	}
	m := planMap{planRef: rep.Feature + "/" + rep.Version, issueByTask: map[string]int{}, blockedBy: map[string][]string{}}
	for _, it := range rep.Items {
		if it.Issue == 0 {
			return planMap{}, fmt.Errorf("plan report: task %s has no projected issue — project the plan before watching", it.TaskID)
		}
		m.issueByTask[it.TaskID] = it.Issue
	}
	planJSON, err := os.ReadFile(filepath.Join(repo, planDir, rep.Version+"-plan.json"))
	if err != nil {
		return planMap{}, fmt.Errorf("read plan json: %w", err)
	}
	var doc struct {
		Tasks []struct {
			ID        string   `json:"id"`
			BlockedBy []string `json:"blocked_by"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(planJSON, &doc); err != nil {
		return planMap{}, fmt.Errorf("parse plan json: %w", err)
	}
	for _, t := range doc.Tasks {
		m.blockedBy[t.ID] = t.BlockedBy
	}
	if len(m.issueByTask) == 0 || len(m.blockedBy) == 0 {
		return planMap{}, fmt.Errorf("plan mapping empty — report items %d, plan tasks %d", len(m.issueByTask), len(m.blockedBy))
	}
	return m, nil
}

// ghPlanSource builds the per-tick durable facts: issue open/closed (one
// list), open proposals by closing issue (one list), claims (one ls-remote).
// Exact reads of the live tracker — never a cache, never loop memory
// (AP-15; DINV-5).
func ghPlanSource(repo, repoSlug, label, remote string, m planMap) watch.PlanSource {
	return func(ctx context.Context) (watch.Plan, error) {
		closed, err := closedIssues(ctx, repo, repoSlug, label)
		if err != nil {
			return watch.Plan{}, err
		}
		proposals, err := openProposalsByIssue(ctx, repo, repoSlug)
		if err != nil {
			return watch.Plan{}, err
		}
		claims, err := listClaims(ctx, repo, remote)
		if err != nil {
			return watch.Plan{}, err
		}
		var plan watch.Plan
		for task, issue := range m.issueByTask {
			t := watch.Task{ID: task, BlockedBy: m.blockedBy[task], Closed: closed[issue], Claimed: claims[issue]}
			if n, ok := proposals[issue]; ok {
				t.Proposal = &port.ProposalRef{Number: n}
			}
			plan.Tasks = append(plan.Tasks, t)
		}
		return plan, nil
	}
}

func closedIssues(ctx context.Context, repo, repoSlug, label string) (map[int]bool, error) {
	out, err := runGH(ctx, repo, "issue", "list", "--repo", repoSlug, "--label", label, "--state", "closed", "--limit", "200", "--json", "number")
	if err != nil {
		return nil, fmt.Errorf("list closed issues: %w", err)
	}
	var rows []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse closed issues: %w", err)
	}
	set := map[int]bool{}
	for _, r := range rows {
		set[r.Number] = true
	}
	return set, nil
}

// openProposalsByIssue maps closing-issue → open PR number by the executor's
// own deterministic head-branch grammar (…-<issue>) — an exact structural
// read, never a search index (AP-15; the predecessor's closes-link gap is
// hm#1165's class, avoided by not depending on link inference).
func openProposalsByIssue(ctx context.Context, repo, repoSlug string) (map[int]int, error) {
	out, err := runGH(ctx, repo, "pr", "list", "--repo", repoSlug, "--state", "open", "--limit", "200", "--json", "number,headRefName")
	if err != nil {
		return nil, fmt.Errorf("list open proposals: %w", err)
	}
	var rows []struct {
		Number int    `json:"number"`
		Head   string `json:"headRefName"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse open proposals: %w", err)
	}
	set := map[int]int{}
	for _, r := range rows {
		if issue := headIssue(r.Head); issue > 0 {
			set[issue] = r.Number
		}
	}
	return set, nil
}

// headIssue extracts the trailing issue number from the executor's
// deterministic head-branch grammar (…-<issue>); 0 = no match.
func headIssue(head string) int {
	i := strings.LastIndex(head, "-")
	if i < 0 || i+1 >= len(head) {
		return 0
	}
	var issue int
	if _, err := fmt.Sscanf(head[i+1:], "%d", &issue); err != nil {
		return 0
	}
	return issue
}

func claimedIssues(ctx context.Context, repo, remote string) (map[int]bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "ls-remote", remote, "refs/claim/*")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	set := map[int]bool{}
	for line := range strings.SplitSeq(string(out), "\n") {
		_, ref, ok := strings.Cut(line, "\trefs/claim/")
		if !ok {
			continue
		}
		var issue int
		if _, err := fmt.Sscanf(strings.TrimSpace(ref), "%d", &issue); err == nil && issue > 0 {
			set[issue] = true
		}
	}
	return set, nil
}

// sweepArgs carries the medium-parameterized inputs to one sweep, so the tracker
// selection watch resolved from config reaches every implement-now invocation
// (#895) — the watcher and its sweeps never disagree on the medium.
type sweepArgs struct {
	override, repo, repoSlug, label, scope, ambientBackend, trackerKind, carinaRoot string
}

type reviewProviderAdapter struct{ store reviewobligation.Store }

func (a reviewProviderAdapter) Open(ctx context.Context, scope string) ([]watch.ReviewObligationState, error) {
	states, err := a.store.Open(ctx, scope)
	if err != nil {
		return nil, err
	}
	out := make([]watch.ReviewObligationState, 0, len(states))
	for _, state := range states {
		providerState := watch.ReviewProviderState(state.Status)
		out = append(out, watch.ReviewObligationState{
			ID: state.Obligation.ID, TaskID: state.Obligation.TaskID,
			ProposalRef: state.Obligation.ProposalRef, Revision: state.Obligation.Revision,
			Harness: state.Obligation.Harness, ProviderState: providerState,
			AttemptCount: state.AttemptCount, LastFailureClass: state.LastFailureClass,
		})
	}
	return out, nil
}

// subprocessSweep invokes the executor's shipped one-sweep entry — the
// implement-now binary through this binary's own exec grammar — carrying the
// wake attribution (initiator=drive-wake, scope=the plan ref; basis EVT-6) and
// the resolved work-item medium (--tracker, and --carina-root for Carina).
func subprocessSweep(a sweepArgs, stdout, stderr io.Writer) watch.Sweep {
	return func(ctx context.Context) error {
		args := []string{
			"--repo", a.repo, "--repo-slug", a.repoSlug, "--label", a.label,
			"--tracker", a.trackerKind, "--plan-ref", a.scope,
			"--unattended", "--initiator", "drive-wake", "--scope", a.scope,
		}
		if a.trackerKind == "carina" {
			args = append(args, "--carina-root", a.carinaRoot)
		}
		if a.ambientBackend != "" {
			args = append(args, "--ambient-backend", a.ambientBackend)
		}
		var cmd *exec.Cmd
		if a.override != "" {
			parts := strings.Fields(a.override)
			cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...)
		} else {
			self, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve self for exec grammar: %w", err)
			}
			cmd = exec.CommandContext(ctx, self, append([]string{"exec", "implement-now", "--"}, args...)...)
		}
		cmd.Dir = a.repo
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sweep: %w", err)
		}
		return nil
	}
}

func subprocessReviewSweep(a sweepArgs, stdout, stderr io.Writer) watch.ReviewSweep {
	return func(ctx context.Context, request watch.ReviewRecoveryRequest) error {
		args := []string{
			"--repo", a.repo, "--repo-slug", a.repoSlug, "--label", a.label,
			"--tracker", a.trackerKind, "--plan-ref", a.scope,
			"--unattended", "--initiator", "drive-wake", "--scope", a.scope,
			"--review-only", "--drive-round-id", request.DriveRoundID,
		}
		if a.trackerKind == "carina" {
			args = append(args, "--carina-root", a.carinaRoot)
		}
		if a.ambientBackend != "" {
			args = append(args, "--ambient-backend", a.ambientBackend)
		}
		for _, id := range request.ObligationIDs {
			args = append(args, "--review-obligation", id)
		}
		cmd, err := implementNowCommand(ctx, a.override, args)
		if err != nil {
			return err
		}
		cmd.Dir, cmd.Stdout, cmd.Stderr = a.repo, stdout, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("review-only sweep: %w", err)
		}
		return nil
	}
}

func implementNowCommand(ctx context.Context, override string, args []string) (*exec.Cmd, error) {
	if override != "" {
		parts := strings.Fields(override)
		return exec.CommandContext(ctx, parts[0], append(parts[1:], args...)...), nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve self for exec grammar: %w", err)
	}
	return exec.CommandContext(ctx, self, append([]string{"exec", "implement-now", "--"}, args...)...), nil
}

// runTool runs a pyxis tool through the exec grammar (or an override in
// tests), returning stdout.
func runTool(ctx context.Context, repo, override, tool string, toolArgs ...string) ([]byte, error) {
	var cmd *exec.Cmd
	if override != "" {
		parts := strings.Fields(override)
		cmd = exec.CommandContext(ctx, parts[0], append(parts[1:], toolArgs...)...)
	} else {
		self, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve self for exec grammar: %w", err)
		}
		cmd = exec.CommandContext(ctx, self, append([]string{"exec", tool, "--"}, toolArgs...)...)
	}
	cmd.Dir = repo
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", tool, err, strings.TrimSpace(errBuf.String()))
	}
	return out, nil
}

func runGH(ctx context.Context, repo string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = repo
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args[:2], " "), err, strings.TrimSpace(errBuf.String()))
	}
	return out, nil
}

// findJSON returns the first top-level JSON object in mixed tool output —
// plangate prints human lines before its machine report.
func findJSON(out []byte) []byte {
	if i := strings.Index(string(out), "\n{"); i >= 0 {
		return out[i+1:]
	}
	if len(out) > 0 && out[0] == '{' {
		return out
	}
	return out
}
