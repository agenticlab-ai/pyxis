package main

// The status verb — the read-only face of the wake capability
// (design:wake-watch-v2 §3.6; plan self-drive-wake/001 T9). It renders one
// pass of the same derivation the watch loop ticks on: per task its state,
// what it waits on, the wait's age and threshold flag, plus the checker's own
// liveness footer. The rendering engine (watch.BuildReport / WriteReport /
// LoadThresholds, #889) is complete; this verb is its CLI wiring — the piece
// FR-7's "when the operator runs the status verb" actually requires. It
// performs no tracker/repo/proposal write (DINV-4); the only append is the
// design's own EVT-5 stale-flagged diagnostic, emitted by BuildReport onto
// the local events sink (the FR-12 detection surface, sanctioned).

import (
	"context"
	"flag"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/github"
	"github.com/agenticlab-ai/pyxis/internal/adapters/tracker/carina"
	repoconfig "github.com/agenticlab-ai/pyxis/internal/docgate/config"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/watch"
)

// Exit codes distinguish the three renderable outcomes (§3.6): a clean render,
// a rendered-but-degraded render (stale/unknown rows or an unverifiable
// footer — the table still prints), and a cannot-render fault (the plan or its
// mapping could not be read at all).
const (
	statusClean       = 0
	statusDegraded    = 3 // rendered, but something warrants attention
	statusCannotBuild = 1 // could not render
	statusUsage       = 2
)

// statusNow is injected by tests; production uses the real clock.
var statusNow = time.Now

var (
	statusNewReviewStore      = reviewobligation.NewGitStore
	statusNewRecoveryLedger   = watch.NewGitRecoveryLedger
	statusBuildReport         = watch.BuildReport
	statusBuildCombinedReport = watch.BuildCombinedReport
)

func statusVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "local repo checkout")
	repoSlug := fs.String("repo-slug", "", "owner/name on the code host (required)")
	label := fs.String("label", "local-agent", "tracker label the projected plan routes on")
	planDir := fs.String("plan", "", "plan directory (e.g. docs/plans/<feature>) — required")
	trackerFlag := fs.String("tracker", "", "work-item medium: gh|carina (default: repo config knobs.tracker)")
	carinaRootFlag := fs.String("carina-root", "", "carina store root when the tracker is carina (default: repo config work_items, else --repo)")
	reportCmd := fs.String("report-cmd", "", "override the plan-report command (tests)")
	if err := fs.Parse(args); err != nil {
		return statusUsage
	}
	// I3's guard, inherited: a flag-looking positional is a usage refusal.
	for _, a := range fs.Args() {
		if strings.HasPrefix(a, "-") {
			terminalPrintf(stderr, "pyxis: status: flag %q after positional arguments — flags precede positionals\n", a)
			return statusUsage
		}
	}
	if len(fs.Args()) > 0 {
		terminalPrintf(stderr, "pyxis: status: unexpected positional %q\n", fs.Args()[0])
		return statusUsage
	}
	icfg, integrationErr := integrations.Load(*repo)
	if integrationErr != nil {
		terminalPrintf(stderr, "pyxis: status: %v\n", integrationErr) // Best-effort diagnostic; exit status carries failure.
		return statusUsage
	}
	if *planDir == "" || (*repoSlug == "" && icfg.Bindings.CodeHost == (integrations.Binding{})) {
		terminalPrintln(stderr, "pyxis: status: --plan and --repo-slug are required")
		return statusUsage
	}

	ctx := context.Background()
	thresholds, err := watch.LoadThresholds(*repo)
	if err != nil {
		// A malformed wake: block is a named, closed-parse refusal (AP-8) — the
		// verb cannot render against an ambiguous threshold, so it fails usage.
		terminalPrintf(stderr, "pyxis: status: %v\n", err)
		return statusUsage
	}

	repoCfg, _ := repoconfig.Load(*repo)
	executorCfg, executorCfgErr := executorconfig.Load(*repo)
	if executorCfgErr != nil {
		terminalPrintf(stderr, "pyxis: status: load executor config: %v\n", executorCfgErr)
		return statusUsage
	}
	// The status board reads the SAME medium watch drives (#895): the tracker
	// choice comes from repo config, so `pyxis status <feature>` renders a
	// GitHub or a Carina plan without the operator naming the medium.
	trackerKind, carinaRoot, terr := resolveTracker(*trackerFlag, *carinaRootFlag, *repo, repoCfg)
	if terr != nil {
		terminalPrintf(stderr, "pyxis: status: %v\n", terr)
		return statusUsage
	}

	mapping, err := planMapping(ctx, *repo, *planDir, trackerKind, carinaRoot, *reportCmd)
	if err != nil {
		terminalPrintf(stderr, "pyxis: status: %v\n", err)
		return statusCannotBuild
	}

	var reader port.ProposalStateReader
	var source watch.PlanSource
	var cadence time.Duration
	configuredReader, configuredSource, _, configured, integrationErr := configuredWatch(*repo, *repoSlug, trackerKind, carinaRoot, repoCfg, mapping, false)
	if integrationErr != nil {
		terminalPrintf(stderr, "pyxis: status: %v\n", integrationErr) // Best-effort diagnostic; exit status carries failure.
		return statusUsage
	}
	if configured {
		reader, source, cadence = configuredReader, configuredSource, configuredReader.cadence
	} else {
		switch trackerKind {
		case "carina":
			sr := carinatracker.NewStateReader(carinaRoot)
			reader, source, cadence = sr, carinaPlanSource(*repo, repoCfg.Remote, sr, mapping), carinatracker.AdviceCadence
		default: // gh
			sr := ghcodehost.NewStateReader(*repoSlug)
			reader, source, cadence = sr, ghPlanSource(*repo, *repoSlug, *label, repoCfg.Remote, mapping), sr.Cadence()
		}
	}
	sink := watch.NewSink(filepath.Join(*repo, watch.DefaultEventsDir), 0, nil)

	var report watch.Report
	if executorCfg.ReviewConfigured() {
		providerStore, storeErr := statusNewReviewStore(*repo, repoCfg.Remote, executorCfg.ReviewEvidenceRef())
		if storeErr != nil {
			terminalPrintf(stderr, "pyxis: status: review obligation store: %v\n", storeErr)
			return statusCannotBuild
		}
		recoveryLedger, ledgerErr := statusNewRecoveryLedger(*repo, repoCfg.Remote, thresholds.ReviewRecoveryRef)
		if ledgerErr != nil {
			terminalPrintf(stderr, "pyxis: status: review recovery ledger: %v\n", ledgerErr)
			return statusCannotBuild
		}
		reviews := watch.ReviewStateReader{Provider: reviewProviderAdapter{store: providerStore}, Recovery: recoveryLedger}
		report, err = statusBuildCombinedReport(ctx, source, reader, reviews, mapping.planRef, sink, thresholds, cadence, statusNow())
	} else {
		report, err = statusBuildReport(ctx, source, reader, sink, thresholds, cadence, statusNow())
	}
	if err != nil {
		terminalPrintf(stderr, "pyxis: status: %v\n", err)
		return statusCannotBuild
	}
	watch.WriteReport(stdout, report)
	if report.Degraded() {
		return statusDegraded
	}
	return statusClean
}
