// docgate — the deterministic document gate and fold for PRD ledgers.
// Thin main: verb dispatch, human defects on stderr, --defects-json JSON
// lines, exit codes 0 (clean) / 1 (defects) / 2 (usage or environment).
// Contract: docs/prds/docgate/ (the PRD) + docs/tech-design/docgate-v1.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/journal"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/contextpack"
	"github.com/agenticlab-ai/pyxis/internal/docgate/contractgate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/incidents"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func usage() int {
	fmt.Fprintln(os.Stderr, `usage: docgate <verb> [flags] [ledger-dir…]
verbs:
  fold <dir>            regenerate a ledger's projection (PRD: effective.md +
                        effective.source.json; review-feedback: effective.source.json).
                        --dry-run: compute and print the projection to stdout,
                        writing nothing — the merge-target approval check does
                        not apply, so a draft amendment can be previewed and
                        reviewed before it is approved
  gate                  run every applicable check (the umbrella; hook + CI entry)
  pre-push              hook entry: skip deletion-only pushes, else run the gate
  lint-structure        ledger layout: one base, dense numbering, headers
  validate-amendments   envelope + ops + computed blast radius (via fold)
  verify-generated      byte-compare committed projection vs recomputed fold
  gen-index             regenerate the tech-design index (index.md + index.json)
  context-pack          build a stage's shared context pack (--stage --subject [--out])
  coverage              requirements coverage on read (--json = machine face)
  contract-gate <feature>  feature-scoped contract verdict (mechanical stage-6;
                        reads machine faces, writes nothing; exit 0 non-blocking / 1
                        blocking). --trigger picks the severity split; --changed
                        <files|range> runs the continuous contract-doc-PR surface;
                        --all sweeps every live contract in the repo (dogfood)
  add-incident          coded writer: append the next dense I<n> entry
  correct-journal <slug> append an atomic batch of historical event-type corrections;
                        --correction E-n=canonical-type (repeatable), --date,
                        --principal, --authority-version and --reason required;
                        --dry-run prints proposed bytes without writing
  graduate              append the Graduated: pointer to an entry's Links line
  verify-rebase         rebase checkpoints are semantic no-ops
  diff                  folded effect of the last amendment`)
	fmt.Fprintln(os.Stderr, "registered checks (self-contributed, verb-sorted):")
	for _, c := range gate.Checks() {
		suffix := ""
		if c.NeedsMergeBase {
			suffix = "  (needs --merge-base)"
		}
		fmt.Fprintf(os.Stderr, "  %-23s %s%s\n", c.Verb, c.Help, suffix)
	}
	fmt.Fprintln(os.Stderr, `flags:
  --merge-base <ref>    git ref for history checks (e.g. origin/main)
  --defects-json <path> also emit one JSON line per defect ("-" = stdout)
  --json                coverage: emit the machine (JSON) face
  --report-out <dir>    contract-gate: defect-report sink on a bounce
                        (default: gitignored scratch; "-" = no file)
  --trigger <t>         contract-gate: severity split — planning-entry (default,
                        all blocking) | contract-doc-pr (coverage advisory)
  --changed <files>     contract-gate: continuous surface — a comma/newline file
                        list or a git range (e.g. origin/main...HEAD)
  --all / --sweep       contract-gate: dogfood sweep over every live contract
  add-incident: --date YYYY-MM-DD --tags a,b --decision … --broke … --lesson … --links …
  graduate:     --id <n> --principle AP-<n>`)
	return 2
}

func run(args []string) int {
	if len(args) == 0 {
		return usage()
	}
	verb := args[0]
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	mergeBase := fs.String("merge-base", "", "git ref for history checks")
	defectsJSON := fs.String("defects-json", "", "path for JSON-line defects (- = stdout)")
	incDate := fs.String("date", "", "add-incident: entry date (YYYY-MM-DD)")
	incTags := fs.String("tags", "", "add-incident: comma-separated kebab-case tags")
	incDecision := fs.String("decision", "", "add-incident: the Decision field")
	incBroke := fs.String("broke", "", "add-incident: the Broke field")
	incLesson := fs.String("lesson", "", "add-incident: the Lesson field")
	incLinks := fs.String("links", "", "add-incident: the Links field")
	incID := fs.Int("id", 0, "graduate: incident number")
	incPrinciple := fs.String("principle", "", "graduate: principle ID (AP-<n>)")
	journalPrincipal := fs.String("principal", "", "correct-journal: actual caller principal")
	journalAuthority := fs.String("authority-version", "", "correct-journal: authority-contract version")
	journalReason := fs.String("reason", "", "correct-journal: reason for the corrections")
	var journalRequests []journal.CorrectionRequest
	fs.Func("correction", "correct-journal: E-n=canonical-type (repeatable)", func(value string) error {
		target, kind, ok := strings.Cut(value, "=")
		if !ok {
			return fmt.Errorf("correction must read E-n=canonical-type")
		}
		journalRequests = append(journalRequests, journal.CorrectionRequest{Target: target, To: kind})
		return nil
	})
	covJSON := fs.Bool("json", false, "coverage: emit the machine (JSON) face")
	foldDryRun := fs.Bool("dry-run", false, "fold: compute the projection and print it to stdout, writing nothing; the merge-target approval check does not apply")
	cpStage := fs.String("stage", "", "context-pack: the pipeline stage")
	cpSubject := fs.String("subject", "", "context-pack: the subject slug")
	cpOut := fs.String("out", "", "context-pack: output path (default: stdout)")
	reportOut := fs.String("report-out", "", "contract-gate: report sink dir (default: gitignored scratch; '-' = stderr only)")
	cgTrigger := fs.String("trigger", "planning-entry", "contract-gate: severity-split trigger (planning-entry | contract-doc-pr)")
	cgChanged := fs.String("changed", "", "contract-gate: continuous mode — run over features whose contract docs changed (comma/newline file list, or a git range like origin/main...HEAD)")
	cgAll := fs.Bool("all", false, "contract-gate: dogfood sweep — gate every live feature contract in the repo, print a per-feature table")
	cgSweep := fs.Bool("sweep", false, "contract-gate: alias of --all (the dogfood sweep over every live contract)")
	// Parse flags allowing them to be INTERSPERSED with positionals — stdlib flag
	// parsing stops at the first positional, so a trailing flag (e.g.
	// `contract-gate <feature> --trigger contract-doc-pr`) would otherwise be
	// silently dropped as a ledger directory (incident I3's class — a preview flag
	// ignored means a run goes live). The loop consumes flags wherever they appear
	// and collects the positionals; a genuinely-unknown flag still errors out of
	// fs.Parse (fail closed, nothing runs).
	var positionals []string
	rest := args[1:]
	for {
		if err := fs.Parse(rest); err != nil {
			return 2
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positionals = append(positionals, rest[0])
		rest = rest[1:]
	}
	// A remaining flag-like positional is a token fs.Parse could not consume as a
	// value — reject it rather than treat it as a directory (belt-and-suspenders).
	for _, a := range positionals {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "docgate: flag %q after a positional argument — flags precede directories (nothing was run)\n", a)
			return 2
		}
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
		return 2
	}
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			fmt.Fprintln(os.Stderr, d.String())
		}
		return 1
	}
	dirs := positionals
	if len(dirs) == 0 {
		if dirs, err = gate.LedgerDirs(root, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
			return 2
		}
	}

	// Resolve --merge-base <ref> to the fork point (git merge-base <ref> HEAD)
	// once, for every history check — never the ref's moving tip. Against the
	// tip, a behind-but-additive branch reads every doc merged onto main after
	// its base as a phantom deletion, false-failing check-immutable et al.
	base := *mergeBase
	if base != "" {
		resolved, rdef := gate.ResolveMergeBase(root, base)
		if len(rdef) > 0 {
			return report(rdef, *defectsJSON)
		}
		base = resolved
	}

	var defects []schema.Defect
	switch verb {
	case "correct-journal":
		if len(positionals) != 1 {
			fmt.Fprintln(os.Stderr, "docgate: correct-journal requires exactly one problem slug")
			return 2
		}
		meta := journal.Metadata{Date: *incDate, Principal: *journalPrincipal, AuthorityVersion: *journalAuthority, Reason: *journalReason}
		candidate, err := journal.CorrectFile(context.Background(), root, cfg, positionals[0], journalRequests, meta, *foldDryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
			return 1
		}
		if *foldDryRun {
			return writeStdout(candidate)
		}
		return writeStdout(fmt.Appendf(nil, "corrected journal %s with %d append-only entries\n", positionals[0], len(journalRequests)))
	case "fold":
		for _, dir := range dirs {
			if gate.IsFeedbackDir(root, cfg, dir) {
				entries, edef := gate.ReadFeedbackEntries(dir)
				defects = append(defects, edef...)
				if entries == nil {
					continue
				}
				res := feedback.Fold(filepath.Base(dir), entries)
				defects = append(defects, res.Defects...)
				if len(res.Defects) == 0 {
					if err := os.WriteFile(filepath.Join(dir, feedback.ProjectionFile), res.SourceJSON, 0o644); err != nil {
						fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
						return 2
					}
					fmt.Printf("folded %s — as of %s, %s\n", dir, res.AsOf, res.Fingerprint)
				}
				continue
			}
			entries, edef := gate.ReadEntries(dir)
			defects = append(defects, edef...)
			if entries == nil {
				continue
			}
			// --dry-run computes and prints; it never writes, so the approval
			// bar (a merge-target check, FR-5) does not apply. Every other
			// defect still counts — a preview that hid them would mislead the
			// reviewer it exists to serve. The projection goes to stdout so a
			// caller can capture it; the status line goes to stderr.
			if *foldDryRun {
				res := fold.Fold(entries, fold.AllowUnapproved())
				defects = append(defects, res.Defects...)
				if len(res.Defects) == 0 {
					if code := writeStdout(res.EffectiveMD); code != 0 {
						return code
					}
					fmt.Fprintf(os.Stderr, "dry-run %s — as of %s, %s (nothing written)\n", dir, res.AsOf, res.Fingerprint)
				}
				continue
			}
			res := fold.Fold(entries)
			defects = append(defects, res.Defects...)
			if len(res.Defects) == 0 {
				if err := os.WriteFile(filepath.Join(dir, "effective.md"), res.EffectiveMD, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
					return 2
				}
				if err := os.WriteFile(filepath.Join(dir, "effective.source.json"), res.SourceJSON, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
					return 2
				}
				fmt.Printf("folded %s — as of %s, %s\n", dir, res.AsOf, res.Fingerprint)
			}
		}
	case "gate":
		defects = gate.Gate(root, base, dirs, cfg)
	case "pre-push":
		// The hook no longer bakes `--merge-base origin/main` (config-driven-
		// portability-v1 §3.1, DINV-1/DINV-4): an absent --merge-base resolves
		// the default mainline ref from the configured branch+remote here, so a
		// repo whose default branch is not "main" (or whose remote is not
		// "origin") is honored and the hook body stays byte-identical across
		// repos. An explicit --merge-base still overrides (CI passes one; a
		// default config yields "origin/main", byte-identical — DINV-2).
		mbRef := *mergeBase
		if mbRef == "" {
			// The shared gate supplies the proposal's trusted base for
			// stacked PRs. Keep the hook on the same range as memory and CI;
			// an unavailable supplied ref must fail, never fall back to main.
			mbRef = os.Getenv("PYXIS_CI_BASE")
		}
		if mbRef == "" {
			mbRef = cfg.MainlineRef()
		}
		heads, deletionOnly := gate.ParsePushRefs(os.Stdin)
		if deletionOnly {
			fmt.Fprintln(os.Stderr, "pre-push: deletion-only push — no content to gate")
			return 0
		}
		if len(heads) == 0 {
			// No parseable pushed sha (a hookless/manual invocation): the old
			// working-tree scope against the resolved mainline, fail closed
			// (DINV-2). --merge-base was resolved above only when set; resolve
			// mbRef here so the working-tree fallback honors config too.
			hbase, rdef := gate.ResolveMergeBase(root, mbRef)
			if len(rdef) > 0 {
				return report(rdef, *defectsJSON)
			}
			defects = gate.Gate(root, hbase, dirs, cfg)
			break
		}
		// Judge exactly the pushed content: per pushed sha, the committed
		// range fork-point..sha — never the working tree, so a shared clone's
		// uncommitted state neither fails nor masks a push (#357).
		for _, head := range heads {
			hbase, mdef := gate.ResolveMergeBaseAt(root, mbRef, head)
			if len(mdef) > 0 {
				defects = append(defects, mdef...)
				continue
			}
			defects = append(defects, gate.GateAt(root, hbase, head, dirs, cfg)...)
		}
	case "lint-structure":
		for _, dir := range dirs {
			entries, edef := gate.ReadEntries(dir)
			defects = append(defects, edef...)
			if entries != nil {
				defects = append(defects, gate.LintStructure(dir, entries)...)
			}
		}
	case "validate-amendments":
		for _, dir := range dirs {
			if entries, edef := gate.ReadEntries(dir); entries != nil {
				defects = append(defects, edef...)
				defects = append(defects, fold.Fold(entries).Defects...)
			} else {
				defects = append(defects, edef...)
			}
		}
	case "verify-generated":
		for _, dir := range dirs {
			if gate.IsFeedbackDir(root, cfg, dir) {
				if entries, edef := gate.ReadFeedbackEntries(dir); entries != nil {
					defects = append(defects, edef...)
					res := feedback.Fold(filepath.Base(dir), entries)
					defects = append(defects, res.Defects...)
					defects = append(defects, gate.VerifyFeedbackGenerated(dir, res)...)
				} else {
					defects = append(defects, edef...)
				}
				continue
			}
			if entries, edef := gate.ReadEntries(dir); entries != nil {
				defects = append(defects, edef...)
				res := fold.Fold(entries)
				defects = append(defects, res.Defects...)
				defects = append(defects, gate.VerifyGenerated(dir, res)...)
			} else {
				defects = append(defects, edef...)
			}
		}
	case "gen-index":
		md, js, gdef := gate.GenIndexBytes(root, cfg)
		defects = gdef
		if len(defects) == 0 {
			dir := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns))
			if err := os.WriteFile(filepath.Join(dir, "index.md"), md, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
				return 2
			}
			if err := os.WriteFile(filepath.Join(dir, "index.json"), js, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
				return 2
			}
			fmt.Printf("generated %s/index.{md,json}\n", cfg.TechDesigns)
		}
	case "coverage":
		md, js, cdef := gate.Coverage(root, cfg)
		defects = cdef
		if len(defects) == 0 {
			wire := md
			if *covJSON {
				wire = js
			}
			if code := writeStdout(wire); code != 0 {
				return code
			}
		}
	case "contract-gate":
		// A feature-scoped, verdict-producing mode (design contract-gate-v1
		// §3.1), distinct from the tree-wide defect-stream gate. Two shapes:
		//
		//   docgate contract-gate <feature> [--trigger …]   single feature
		//   docgate contract-gate --changed <files|range>    continuous surface
		//
		// The single-feature form takes the feature slug as its one positional,
		// resolves the contract read-only, prints the verdict (human summary to
		// stderr + structured JSON to stdout), and returns the exit code — 0 when
		// the run does not block (clean, or an advisory-only bounce under the
		// active trigger), 1 when a blocking defect fails it, 2 a usage error. The
		// --trigger flag selects the severity split (default planning-entry —
		// every class blocking, unchanged from before the split existed). Its only
		// write is the defect report it EMITS to the sink on a bounce; it makes no
		// git or tracker mutation, and the HOST retains the report to git (DINV-1).
		//
		// The --changed form is the continuous contract-doc-PR surface (FR-9): it
		// derives the affected feature(s) from the changed contract-doc paths and
		// gates each under the contract-doc-pr trigger (coverage advisory), failing
		// (exit 1) iff any feature blocks. It reads its own positional/flag, never
		// the ledger-dir defaulting above.
		trig, terr := contractgate.ParseTrigger(*cgTrigger)
		if terr != "" {
			fmt.Fprintf(os.Stderr, "docgate: contract-gate: %s\n", terr)
			return 2
		}
		// The dogfood sweep (--all / --sweep): the "local CI" over the repo's own
		// contracts. It gates EVERY live feature contract under the planning-entry
		// hard bar, prints a per-feature table (human summary to stderr) + a
		// structured result (JSON to stdout), and returns non-zero iff any feature
		// blocks. Read-only (it emits no report; the sweep is a diagnostic view). It
		// reads no positional/flag beyond --all; it is mutually exclusive with the
		// single-feature and --changed forms.
		if *cgAll || *cgSweep {
			if len(positionals) != 0 || *cgChanged != "" {
				fmt.Fprintln(os.Stderr, "docgate: contract-gate --all sweeps every live contract — pass no feature and no --changed")
				return 2
			}
			return contractGateSweep(root)
		}
		if *cgChanged != "" {
			return contractGateContinuous(root, *cgChanged, *reportOut)
		}
		fargs := positionals
		if len(fargs) != 1 {
			fmt.Fprintln(os.Stderr, "docgate: contract-gate takes exactly one positional argument — the feature slug (or --changed for the continuous surface)")
			return 2
		}
		v := contractgate.RunTrigger(root, fargs[0], trig)
		fmt.Fprintln(os.Stderr, v.Summary())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(v); err != nil {
			fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
			return 2
		}
		// A bounce (blocking OR advisory) emits one dual-face defect report to the
		// sink (design §3.8): even an advisory-only bounce is recorded so the drift
		// is captured; only the EXIT CODE distinguishes them. A clean verdict
		// materializes nothing (DINV-6). The sink is --report-out when given, else
		// the gitignored scratch default; "-" suppresses the file emission.
		if !v.Clean() && *reportOut != "-" {
			if err := emitContractReport(root, *reportOut, v); err != nil {
				fmt.Fprintf(os.Stderr, "docgate: emitting defect report: %v\n", err)
				return 2
			}
		}
		if v.Blocked() {
			return 1
		}
		return 0 // clean, or an advisory-only bounce — non-blocking (FR-9)
	case "add-incident":
		var tags []string
		for t := range strings.SplitSeq(*incTags, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
		id, adef := incidents.Add(root, cfg, incidents.Entry{
			Date: *incDate, Tags: tags, Decision: *incDecision,
			Broke: *incBroke, Lesson: *incLesson, Links: *incLinks,
		})
		defects = adef
		if len(defects) == 0 {
			fmt.Printf("I%d\n", id)
		}
	case "graduate":
		if *incID <= 0 || *incPrinciple == "" {
			fmt.Fprintln(os.Stderr, "docgate: graduate requires --id and --principle")
			return 2
		}
		defects = incidents.Graduate(root, cfg, *incID, *incPrinciple)
	case "verify-rebase":
		for _, dir := range dirs {
			if entries, edef := gate.ReadEntries(dir); entries != nil {
				defects = append(defects, edef...)
				defects = append(defects, gate.VerifyRebase(entries)...)
			} else {
				defects = append(defects, edef...)
			}
		}
	case "diff":
		for _, dir := range dirs {
			if entries, edef := gate.ReadEntries(dir); entries != nil {
				defects = append(defects, edef...)
				report, ddef := gate.Diff(entries)
				defects = append(defects, ddef...)
				fmt.Print(report)
			} else {
				defects = append(defects, edef...)
			}
		}
	case "context-pack":
		if *cpStage == "" || *cpSubject == "" {
			fmt.Fprintln(os.Stderr, "docgate: context-pack requires --stage and --subject")
			return 2
		}
		pack, cdef := contextpack.Run(root, *cpStage, *cpSubject)
		defects = cdef
		if len(defects) == 0 {
			if *cpOut == "" {
				if code := writeStdout(pack); code != 0 {
					return code
				}
			} else if err := os.WriteFile(*cpOut, pack, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
				return 2
			} else {
				fmt.Fprintf(os.Stderr, "context-pack: wrote %s (%d bytes)\n", *cpOut, len(pack))
			}
		}
	default:
		// A verb with no explicit case is a registered, self-contributed check.
		// Individual dispatch and the Gate() umbrella share the same Run, so they
		// cannot drift — and adding a check needs no case here.
		chk, ok := gate.Lookup(verb)
		if !ok {
			return usage()
		}
		if chk.NeedsMergeBase && *mergeBase == "" {
			fmt.Fprintf(os.Stderr, "docgate: %s requires --merge-base\n", verb)
			return 2
		}
		defects = chk.Run(gate.Ctx{Root: root, MergeBase: base, Cfg: cfg})
	}

	return report(defects, *defectsJSON)
}

// report emits defects — human lines to stderr, plus one JSON line each to
// defectsJSON ("-" = stdout) when set — and maps them to the exit code: 1 if
// any defect, else 0. Shared by the normal dispatch tail and the early
// merge-base-resolution failure path so both render defects identically.
func report(defects []schema.Defect, defectsJSON string) (code int) {
	for _, d := range defects {
		fmt.Fprintln(os.Stderr, d.String())
	}
	if defectsJSON != "" {
		out := os.Stdout
		if defectsJSON != "-" {
			f, err := os.Create(defectsJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
				return 2
			}
			defer func() {
				if err := f.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "docgate: close defect report: %v\n", err)
					code = 2
				}
			}()
			out = f
		}
		enc := json.NewEncoder(out)
		for _, d := range defects {
			if err := enc.Encode(d); err != nil {
				fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
				return 2
			}
		}
	}
	if len(defects) > 0 {
		return 1
	}
	return 0
}

func writeStdout(data []byte) int {
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "docgate: write stdout: %v\n", err)
		return 2
	}
	return 0
}

// emitContractReport writes a bounce's dual-face defect report to the sink —
// --report-out when given, else the gitignored scratch default — and logs the
// written paths. The gate never commits the report; the host does (DINV-1).
func emitContractReport(root, reportOut string, v contractgate.Verdict) error {
	dir := reportOut
	if dir == "" {
		dir = filepath.Join(root, filepath.FromSlash(contractgate.DefaultReportDir))
	}
	paths, err := contractgate.EmitReport(dir, v)
	if err != nil {
		return err
	}
	for _, p := range paths {
		fmt.Fprintf(os.Stderr, "contract-gate: wrote defect report %s\n", p)
	}
	return nil
}

// contractGateContinuous runs the continuous contract-doc-PR surface (FR-9): it
// resolves the --changed argument to a changed-file list, derives the affected
// feature(s), gates each under the contract-doc-pr trigger, prints one verdict
// per feature (human summary to stderr + a structured JSON result to stdout),
// emits a defect report per bouncing feature, and returns the exit code — 1 iff
// any feature blocks or the context failed closed (config/severity-map), else 0.
func contractGateContinuous(root, changed, reportOut string) int {
	files, ferr := changedFilesFromArg(root, changed)
	if ferr != "" {
		fmt.Fprintf(os.Stderr, "docgate: contract-gate --changed: %s\n", ferr)
		return 2
	}
	res := contractgate.Continuous(root, files)

	if res.ConfigError != "" {
		fmt.Fprintf(os.Stderr, "contract-gate [continuous]: configuration-error — %s\n", res.ConfigError)
	} else if len(res.Features) == 0 {
		fmt.Fprintln(os.Stderr, "contract-gate [continuous]: no contract docs changed — no feature to gate")
	} else {
		for _, v := range res.Verdicts {
			fmt.Fprintln(os.Stderr, v.Summary())
			if !v.Clean() && reportOut != "-" {
				if err := emitContractReport(root, reportOut, v); err != nil {
					fmt.Fprintf(os.Stderr, "docgate: emitting defect report: %v\n", err)
					return 2
				}
			}
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
		return 2
	}
	if res.Failed() {
		return 1
	}
	return 0
}

// contractGateSweep runs the dogfood sweep over every live feature contract in
// the repo (design contract-gate-v1 §3.11; prd:contract-gate/M2 — the "local CI"
// over the repo's own contracts). It prints a per-feature table to stderr — each
// feature's verdict class, its blocking/advisory/clean disposition, and (on a
// bounce) the named defects — plus a structured SweepResult to stdout, and
// returns the exit code: non-zero iff any feature blocks (or the sweep failed
// closed on a bad config/severity-map). Read-only: the sweep emits no defect
// report; it is a diagnostic view, not a bounce event.
func contractGateSweep(root string) int {
	res := contractgate.Sweep(root)
	if res.ConfigError != "" {
		fmt.Fprintf(os.Stderr, "contract-gate [sweep]: configuration-error — %s\n", res.ConfigError)
		return 1
	}

	// The per-feature table: one row per live contract, aligned, in feature order.
	// A reader sees at a glance which contracts are clean, which bounce advisory,
	// and which BLOCK — with the offending defects named under each bounce.
	fmt.Fprintf(os.Stderr, "contract-gate dogfood sweep [%s] — %d live contract(s)\n\n", res.Trigger, res.Total)
	width := 0
	for _, f := range res.Features {
		if len(f.Feature) > width {
			width = len(f.Feature)
		}
	}
	for _, f := range res.Features {
		state := "clean"
		switch {
		case f.Blocking:
			state = "BLOCK"
		case f.Class != contractgate.ClassClean:
			state = "advisory"
		}
		fmt.Fprintf(os.Stderr, "  %-*s  %-9s  %s\n", width, f.Feature, state, f.Class)
		for _, d := range f.Defects {
			fmt.Fprintf(os.Stderr, "      - [%s] %s\n", d.Check, d.Reason)
		}
	}
	fmt.Fprintf(os.Stderr, "\nsummary: %d clean, %d advisory, %d blocking (of %d)\n",
		res.Clean, res.Advisory, res.Blocking, res.Total)
	if res.Blocking > 0 {
		fmt.Fprintln(os.Stderr, "result: BLOCKING — one or more live contracts carry a blocking defect (see the named offenders above)")
	} else {
		fmt.Fprintln(os.Stderr, "result: clean — no live contract carries a blocking defect")
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		fmt.Fprintf(os.Stderr, "docgate: %v\n", err)
		return 2
	}
	if res.Failed() {
		return 1
	}
	return 0
}

// changedFilesFromArg turns the --changed value into a repo-relative changed-file
// list. A git RANGE (a token containing ".." — e.g. "origin/main...HEAD") is
// resolved via `git diff --name-only`; otherwise the value is a comma/newline-
// separated FILE LIST taken verbatim (the hermetic form CI can pass directly).
// Returns a non-empty error string on a git failure.
func changedFilesFromArg(root, changed string) ([]string, string) {
	changed = strings.TrimSpace(changed)
	if strings.Contains(changed, "..") && !strings.ContainsAny(changed, ",\n") {
		out, err := exec.Command("git", "-C", root, "diff", "--name-only", changed).Output()
		if err != nil {
			return nil, fmt.Sprintf("git diff --name-only %q failed: %v", changed, err)
		}
		return splitFileList(string(out)), ""
	}
	return splitFileList(changed), ""
}

// splitFileList splits a changed-file argument on commas and newlines, trimming
// blanks — the shared parse for both the literal-list and git-range paths.
func splitFileList(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// repoRoot locates the pyxis-operated repo root via the one shared discovery
// predicate (internal/reporoot): the nearest ancestor holding
// .pyxis/config.yaml, else the git working-tree root. It does NOT key on
// go.mod — an installed operand ships no Go source, and docgate must find its
// root there when the operand's hooks and CI run `pyxis exec docgate -- gate`.
func repoRoot() (string, error) {
	return reporoot.Find()
}
