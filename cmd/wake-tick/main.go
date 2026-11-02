// wake-tick — the wake channel's bounded backstop pass (wake-channel-v1 §3.2;
// plan T8's wiring; #515). One invocation = one tick: read the registered
// awaits from the delivery ledgers, probe each awaited goal object with an
// exact read, invoke the agentic CLI once per due touch, print the cold-
// readable report, exit. Nothing resident, nothing between ticks (AP-19); the
// host scheduler binding (launchd/cron) is the operator's and is documented in
// the shim, never installed by this binary.
//
// usage: wake-tick [--repo <root>] [--gh-repo <owner/name>] [--dry-run]
//
//	[--json] [-- <front-door argv…>]
//
// The front-door command follows `--`: the tick appends exactly one argument —
// the normalized wire signal `wake:<slug>:<reason>` — and executes it once per
// due touch (DINV-1: the invoked CLI receives slug + reason, nothing else).
// With --dry-run (or no `--` command) the tick probes, reports what would be
// delivered, and invokes nothing.
//
// Exit codes: 0 — tick completed (the report carries any named defects,
// staleness findings, and touches); 1 — the ledger source itself was
// unreadable (a hard fault); 2 — usage.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/wake"
	"github.com/agenticlab-ai/pyxis/internal/wake/host"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr *os.File) int {
	root := "."
	ghRepo := ""
	dryRun := false
	var door []string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--repo":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "wake-tick: --repo needs a value")
				return 2
			}
			root = args[i+1]
			i += 2
		case "--gh-repo":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "wake-tick: --gh-repo needs a value")
				return 2
			}
			ghRepo = args[i+1]
			i += 2
		case "--dry-run":
			dryRun = true
			i++
		case "--":
			door = args[i+1:]
			i = len(args)
		case "-h", "--help":
			usage(stderr)
			return 2
		default:
			fmt.Fprintf(stderr, "wake-tick: unknown flag %q\n", args[i])
			usage(stderr)
			return 2
		}
	}
	// No configured front door means the tick can only report — the dry posture
	// is explicit, never an accidental no-op that looked like delivery.
	if len(door) == 0 {
		dryRun = true
	}

	cfg, _ := config.Load(root)
	d := &host.Door{Argv: door, DryRun: dryRun}
	b := &wake.Backstop{
		Ledgers:  host.Ledgers{Root: root, Problems: cfg.Problems},
		Prober:   host.Prober{Root: root, GHRepo: ghRepo, Remote: cfg.Remote, Branch: cfg.Branch},
		Door:     d,
		Problems: cfg.Problems,
		Bound:    cfg.Channel.StalenessBound,
		Now:      time.Now,
	}

	rep, err := b.Tick(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "wake-tick: %v\n", err)
		return 1
	}

	out := struct {
		wake.TickReport
		DryRun  bool     `json:"dry_run"`
		Invoked []string `json:"invoked,omitempty"`
	}{TickReport: rep, DryRun: dryRun, Invoked: d.Invoked}
	b2, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(stderr, "wake-tick: report marshal: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(b2))
	return 0
}

func usage(stderr *os.File) {
	fmt.Fprintln(stderr, `usage: wake-tick [--repo <root>] [--gh-repo <owner/name>] [--dry-run] [-- <front-door argv…>]
one bounded backstop pass: read awaits, probe goal objects exactly, invoke the
front-door CLI once per due touch (appending the wire signal), report, exit.
No command after -- (or --dry-run) probes and reports without invoking.`)
}
