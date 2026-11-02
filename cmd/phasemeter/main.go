// phasemeter — the pipeline-measurement recorder and reader
// (docs/tech-design/pipeline-measurement-v2.md §3.11): a thin main wiring one
// harness's spend-source adapter to the port (internal/phasemeter/spend) and
// the record store (internal/phasemeter/store), plus the operator-facing
// read (internal/phasemeter/reader). This is the tool's composition root —
// the one place a harness-specific adapter package is imported (AP-1/AP-3:
// the port itself imports none).
//
// Verb surface:
//
//	record --run-id <id> --feature-id <id> [--harness claude-code]
//	       [--store-dir <dir>] <transcript-path>
//	  Parse <transcript-path> — a harness's own session record — bracket it
//	  into phases, capture each phase's per-model spend through the named
//	  harness's adapter, and append one record.Record per phase to the store.
//	  Post-hoc and read-only throughout: the transcript is opened read-only
//	  and nothing about the harness's own state or the measured run's
//	  artifacts is written (DINV-2); no model is invoked (DINV-1).
//
//	read [--store-dir <dir>] [--repo-root <dir>] [--price-file <path>] <run-id>
//	  Load <run-id>'s records from the store, compute each phase's
//	  efficiency signals from repo-root's frozen artifacts + git, derive
//	  each model's dollar cost from the vendored price snapshot, and print
//	  the per-phase operator-facing report (FR-9): tokens by model, cost,
//	  reconciliation tier, and efficiency signals, or the declared
//	  unavailable markers. Query-only: no model is invoked (DINV-1) and
//	  nothing on disk is written.
//
// Argument discipline (incident I3, DINV-9): flags precede the one
// positional, parsed fail-closed through internal/cliguard — a flag-like
// positional or an unknown flag refuses with a usage error before any write
// (record) or read (read).
//
// Exit codes: 0 recorded/read (zero phases found is reported, not a fault),
// 1 named refusal (an unreadable/malformed transcript, a store write
// failure, an unloadable run or price snapshot), 2 usage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/claudecode"
	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/codex"
	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/cursor"
	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/opencode"
	"github.com/agenticlab-ai/pyxis/internal/cliguard"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/price"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/reader"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/store"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// adapters is the recorder's harness registry: the --harness value mapped to
// the loader that reads that harness's own session record at a path into a
// spend.Source. All four appetite harnesses are wired: Claude Code (T3),
// Codex (T6), Cursor CLI (T7), and OpenCode (T8).
//
// OpenCode has no native transcript file (its session lives in a local SQLite
// store, surfaced only via `opencode export <id>`), so for --harness opencode
// the positional is a file holding that export's JSON — produce it with
// `opencode export <id> > session.json`. It is parsed read-only like any other
// transcript; see openExport.
var adapters = map[string]func(path string) (spend.Source, error){
	claudecode.HarnessID: func(path string) (spend.Source, error) { return claudecode.Load(path) },
	codex.HarnessID:      func(path string) (spend.Source, error) { return codex.Load(path) },
	cursor.HarnessID:     func(path string) (spend.Source, error) { return cursor.Load(path) },
	opencode.HarnessID:   openExport,
}

// openExport reads a file holding `opencode export` output into a spend.Source.
// OpenCode is the one wired harness with no file-based native transcript, so the
// recorder consumes the JSON the operator captured with `opencode export <id>`
// rather than shelling out itself — the adapter's own subprocess Load stays
// available for callers that want the export driven for them.
func openExport(path string) (spend.Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opencode: open export file: %w", err)
	}
	defer func() { _ = f.Close() }() // Parse consumes the reader; a close error cannot change the result.
	return opencode.Parse(f)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stderr)
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "record":
		return recordVerb(rest, stdout, stderr)
	case "read":
		return readVerb(rest, stdout, stderr)
	case "help", "-h", "--help":
		return usage(stderr)
	default:
		writeOut(stderr, "phasemeter: unknown verb %q\n", verb)
		return usage(stderr)
	}
}

func usage(w io.Writer) int {
	writeOut(w, `usage: phasemeter <verb> …
verbs:
  record --run-id <id> --feature-id <id> [--harness claude-code]
         [--store-dir <dir>] <transcript-path>
                          measure every phase bracket in a harness's own
                          session record and append one record per phase to
                          the store (default store dir: <repo-root>/%s)
  read [--store-dir <dir>] [--repo-root <dir>] [--price-file <path>] <run-id>
                          print <run-id>'s per-phase report: tokens by
                          model, cost, and efficiency signals, or the
                          declared unavailable markers (FR-9)
supported harnesses: %s
`, store.DefaultRelDir, strings.Join(supportedHarnesses(), ", "))
	return 2
}

// verbUsage is the one-line grammar per verb, printed with every refusal.
var verbUsage = map[string]string{
	"record": "phasemeter record --run-id <id> --feature-id <id> [--harness claude-code] [--store-dir <dir>] <transcript-path>",
	"read":   "phasemeter read [--store-dir <dir>] [--repo-root <dir>] [--price-file <path>] <run-id>",
}

// refusalNoun is what verb's refusal message says never happened — the I3
// discipline applies to record's writes and read's own side-effecting IO
// alike, but "nothing was written" would misdescribe a query-only verb.
var refusalNoun = map[string]string{
	"record": "written",
	"read":   "read",
}

// refuse renders a usage refusal — the named reason plus the verb's grammar —
// and returns 2. Nothing has been written/read when this prints (I3: refusal
// before action).
func refuse(stderr io.Writer, verb, reason string) int {
	writeOut(stderr, "phasemeter: %s: %s — nothing was %s\nusage: %s\n", verb, reason, refusalNoun[verb], verbUsage[verb])
	return 2
}

// misparse renders a cliguard refusal through refuse; -h/-help gets the full
// usage instead of a one-liner.
func misparse(stderr io.Writer, verb string, err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return usage(stderr)
	}
	var ue *cliguard.UsageError
	if errors.As(err, &ue) {
		return refuse(stderr, verb, ue.Reason)
	}
	return refuse(stderr, verb, err.Error())
}

// recordVerb is `phasemeter record`. Guard discipline is the I3 house rule:
// the flags and the one positional parse fail-closed through cliguard before
// any transcript read or store write.
func recordVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	runID := fs.String("run-id", "", "the run this phase belongs to (required)")
	featureID := fs.String("feature-id", "", "the feature this run measures (required)")
	harness := fs.String("harness", claudecode.HarnessID, "the harness that produced the transcript")
	storeDir := fs.String("store-dir", "", "the run-record store directory inside .pyxis/logs (default: <repo-root>/"+store.DefaultRelDir+")")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "record", err)
	}
	if len(pos) != 1 {
		return refuse(stderr, "record", fmt.Sprintf("takes exactly one positional argument — the harness's session-transcript path (got %d)", len(pos)))
	}
	if *runID == "" || *featureID == "" {
		return refuse(stderr, "record", "--run-id and --feature-id are required")
	}
	load, ok := adapters[*harness]
	if !ok {
		return refuse(stderr, "record", fmt.Sprintf("unsupported --harness %q — supported: %s", *harness, strings.Join(supportedHarnesses(), ", ")))
	}

	root, rerr := reporoot.Find()
	if rerr != nil {
		writeOut(stderr, "phasemeter: record: %v\n", rerr)
		return 2
	}
	dir, rerr := runlogs.Resolve(context.Background(), root, store.DefaultRelDir, *storeDir)
	if rerr != nil {
		writeOut(stderr, "phasemeter: record: %v\n", rerr)
		return 2
	}

	src, lerr := load(pos[0])
	if lerr != nil {
		writeOut(stderr, "phasemeter: record: %v\n", lerr)
		return 1
	}

	recs := spend.Records(src, *runID, *featureID)
	st := store.New(dir)
	for _, rec := range recs {
		if aerr := st.Append(rec); aerr != nil {
			writeOut(stderr, "phasemeter: record: %v\n", aerr)
			return 1
		}
	}
	printSummary(stdout, recs, dir)
	return 0
}

// readVerb is `phasemeter read`. Same I3 guard discipline as recordVerb:
// the flags and the one positional (the run id) parse fail-closed through
// cliguard before any store, price, or git read — a query-only verb still
// refuses before its own reads, not just before a write.
func readVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	storeDir := fs.String("store-dir", "", "the run-record store directory (default: <repo-root>/"+store.DefaultRelDir+")")
	repoRootFlag := fs.String("repo-root", "", "the repo root efficiency signals are computed against (default: the discovered repo root)")
	priceFile := fs.String("price-file", "", "the vendored price snapshot (default: <repo-root>/"+price.DefaultRelPath+")")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "read", err)
	}
	if len(pos) != 1 {
		return refuse(stderr, "read", fmt.Sprintf("takes exactly one positional argument — the run id (got %d)", len(pos)))
	}
	runID := pos[0]

	root := *repoRootFlag
	if root == "" {
		r, rerr := reporoot.Find()
		if rerr != nil {
			writeOut(stderr, "phasemeter: read: %v\n", rerr)
			return 2
		}
		root = r
	}

	dir := *storeDir
	if dir == "" {
		dir = filepath.Join(root, filepath.FromSlash(store.DefaultRelDir))
	}

	pf := *priceFile
	if pf == "" {
		pf = filepath.Join(root, filepath.FromSlash(price.DefaultRelPath))
	}
	prices, perr := price.Load(pf)
	if perr != nil {
		writeOut(stderr, "phasemeter: read: %v\n", perr)
		return 1
	}

	views, verr := reader.Read(store.New(dir), prices, root, runID)
	if verr != nil {
		writeOut(stderr, "phasemeter: read: %v\n", verr)
		return 1
	}
	printReport(stdout, runID, views)
	return 0
}

// printSummary renders the recorded phases for a human — a rough index of
// what record just wrote, not the operator-facing presentation (that is
// printReport, driven by the read verb below). Each line names the phase,
// its harness, and either its per-model token quartet or the figures it
// declared unavailable (DINV-3: never a silent zero).
func printSummary(w io.Writer, recs []record.Record, dir string) {
	if len(recs) == 0 {
		writeLn(w, "phasemeter: no phase boundaries found in the transcript — nothing recorded")
		return
	}
	writeOut(w, "phasemeter: recorded %d phase(s) to %s\n", len(recs), dir)
	for _, r := range recs {
		models := make([]string, 0, len(r.Spend))
		for model, ms := range r.Spend {
			models = append(models, fmt.Sprintf("%s(input=%d output=%d cache_creation=%d cache_read=%d)",
				model, ms.Input, ms.Output, ms.CacheCreation, ms.CacheRead))
		}
		sort.Strings(models)
		detail := strings.Join(models, ", ")
		if len(r.Unavailable) > 0 {
			missing := make([]string, 0, len(r.Unavailable))
			for _, u := range r.Unavailable {
				missing = append(missing, u.MissingFigure)
			}
			sort.Strings(missing)
			if detail != "" {
				detail += ", "
			}
			detail += "unavailable: " + strings.Join(missing, ", ")
		}
		writeOut(w, "  %-24s %-12s %s\n", r.PhaseID, r.Harness, detail)
	}
}

// printReport renders runID's per-phase operator-facing report (FR-9,
// design §3.11's "present" step): each phase names its harness and
// feature, then each model's tokens, dollar cost (or "unpriced", UB-2),
// and reconciliation tier (design §3.4's "self-describing" debugging
// surface — "not-reconciled" names the record's own unset zero value
// rather than hiding it), then any declared unavailable markers (DINV-3),
// then the phase's efficiency signals (T9).
func printReport(w io.Writer, runID string, views []reader.View) {
	if len(views) == 0 {
		writeOut(w, "phasemeter: no phases recorded for run %q\n", runID)
		return
	}
	writeOut(w, "phasemeter: run %s — %d phase(s)\n", runID, len(views))
	for _, v := range views {
		writeOut(w, "  %s  (harness=%s, feature=%s)\n", v.PhaseID, v.Harness, v.FeatureID)
		for _, m := range v.Models {
			cost := fmt.Sprintf("$%.6f", m.Cost)
			if m.Unpriced {
				cost = "unpriced"
			}
			tier := m.ReconciliationTier
			if tier == "" {
				tier = "not-reconciled"
			}
			writeOut(w, "    %-24s tokens(input=%d output=%d cache_creation=%d cache_read=%d) cost=%s reconciliation=%s\n",
				m.Model, m.Tokens.Input, m.Tokens.Output, m.Tokens.CacheCreation, m.Tokens.CacheRead, cost, tier)
		}
		for _, u := range v.Unavailable {
			writeOut(w, "    unavailable: %s (%s)\n", u.MissingFigure, u.Harness)
		}
		writeOut(w, "    efficiency: questions=%d fog=%d rework=%d defects=%d coverage=%.2f latency=%s\n",
			v.Efficiency.Questions, v.Efficiency.Fog, v.Efficiency.Rework, v.Efficiency.Defects,
			v.Efficiency.CoverageRatio, time.Duration(v.Efficiency.LatencySeconds)*time.Second)
	}
}

// supportedHarnesses lists the recorder's wired harnesses, sorted for a
// stable usage/refusal message.
func supportedHarnesses() []string {
	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeOut(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) // CLI output is best-effort; the exit status is the contract.
}

func writeLn(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...) // CLI output is best-effort; the exit status is the contract.
}
