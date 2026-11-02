// plangate — the deterministic half of plan-new: validate and fold plan
// documents, verify their committed machine faces, and (next increment)
// project frozen plans onto the tracker. Thin main in the docgate shape:
// verb dispatch, human defects on stderr, --defects-json JSON lines, exit
// codes 0 (clean) / 1 (defects) / 2 (usage or environment).
// Contract: docs/prds/plan-new/ + docs/tech-design/plan-new-v1.md.
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

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	forgejotracker "github.com/agenticlab-ai/pyxis/internal/adapters/tracker/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
	"github.com/agenticlab-ai/pyxis/internal/plangate/project"
	"github.com/agenticlab-ai/pyxis/internal/plangate/render"
	"github.com/agenticlab-ai/pyxis/internal/plangate/tracker"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
	"github.com/agenticlab-ai/pyxis/internal/toolbin"
)

var processExit = os.Exit
var currentWorkingDir = os.Getwd
var planningPrecheck = configuredPlanningPrecheck

func main() { processExit(run(os.Args[1:])) }

func usage() int {
	fmt.Fprintln(os.Stderr, `usage: plangate <verb> [flags] [plan-dir…]
verbs:
  dimensions          print allowed task dimension values and derivation policy as JSON
  precheck <feature>   planning-entry gate: document-pipeline features use
                       contract-gate; draft-born features use readiness of the
                       latest binding lock against current mainline and print
                       its Basis on success
  fold <plan-dir…>     parse + gate each NNN-plan.md; emit NNN-plan.json
  verify <plan-dir…>   byte-compare committed machine faces vs recomputed folds
  project <plan-dir>   project the highest merged plan onto the tracker
                       (create → spec → link → wire → route-last; convergent)
  report <plan-dir>    re-probe the tracker; print the projection report
  coverage <plan-dir>  feature-level Must completeness — the union across the
                       feature's plans (a live Must covered by no plan is named)
  render <plan-dir>    fold in memory; emit one self-contained review page
                       (fold-red / stale face / missing bundle → refuse)
flags:
  --defects-json <path> also emit one JSON line per defect ("-" = stdout)
  --dry-run             project: print the write set, write nothing
  --no-routing          project: stop before the routing signal (stage 2 /
                        kill-switch posture)
  --version <NNN>       render: the plan version to render (default: highest)
  --out <path>          render: output page path (default:
                        .pyxis/scratch/plan-<feature>-<NNN>.html)
plan-dir defaults to every feature directory under the configured plans root.`)
	return 2
}

func run(args []string) int {
	if len(args) == 0 {
		return usage()
	}
	verb := args[0]
	if verb == "dimensions" {
		if len(args) != 1 {
			return usage()
		}
		if err := printDimensions(os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	defectsJSON := fs.String("defects-json", "", "path for JSON-line defects (- = stdout)")
	dryRun := fs.Bool("dry-run", false, "project: print the write set, write nothing")
	noRouting := fs.Bool("no-routing", false, "project: stop before the routing signal")
	version := fs.String("version", "", "render: plan version to render (default: highest)")
	out := fs.String("out", "", "render: output page path (default: .pyxis/scratch/plan-<feature>-<NNN>.html)")
	trackerFlag := fs.String("tracker", "gh", "project: legacy tracker default (gh|carina); integrations.bindings.tasks takes precedence")
	taskConnection := fs.String("task-connection", "", "project: override the named task connection")
	repository := fs.String("repo", "", "project: task repository owner/name (overrides the task binding repository)")
	carinaRoot := fs.String("carina-root", "", "project: carina store root (default: repo root)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	// Fail closed on flag-like positionals: stdlib flag parsing stops at
	// the first positional, so a trailing --dry-run would otherwise be
	// silently reinterpreted as a plan directory and a preview would run
	// live (incident I3). Flags precede directories, or the verb refuses.
	for _, a := range fs.Args() {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(os.Stderr, "plangate: flag %q after a positional argument — flags precede plan directories (nothing was run)\n", a)
			return 2
		}
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
		return 2
	}
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			fmt.Fprintln(os.Stderr, d.String())
		}
		return 1
	}
	// precheck takes a FEATURE SLUG, not a plan directory — it gates one
	// feature's contract before any plan for it exists, so it reads the raw
	// positional and never inherits the all-plan-dirs defaulting below (FR-27).
	if verb == "precheck" {
		slug := fs.Args()
		if len(slug) != 1 {
			fmt.Fprintln(os.Stderr, "plangate: precheck takes exactly one positional argument — the feature slug (e.g. plangate precheck contract-gate)")
			return 2
		}
		basis, defects := planningPrecheck(context.Background(), root, slug[0], draftshell.Run)
		code := emit(defects, *defectsJSON)
		if code == 0 && basis != "" {
			if _, err := fmt.Fprintln(os.Stdout, basis); err != nil {
				fmt.Fprintf(os.Stderr, "plangate: print binding Basis: %v\n", err)
				return 2
			}
		}
		return code
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		if dirs, err = planDirs(root, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
			return 2
		}
	}

	var defects []schema.Defect
	switch verb {
	case "fold":
		for _, dir := range dirs {
			files, err := plan.Files(root, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
				return 2
			}
			for _, rel := range files {
				res := plan.FoldFile(root, cfg, rel)
				defects = append(defects, res.Defects...)
				if len(res.Defects) > 0 {
					continue
				}
				out := filepath.Join(root, filepath.FromSlash(plan.JSONPath(rel)))
				if err := os.WriteFile(out, res.JSON, 0o644); err != nil {
					fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
					return 2
				}
				fmt.Printf("folded %s — %d task(s), %s\n", rel, len(res.Doc.Tasks), res.Doc.Fingerprint)
			}
		}
	case "verify":
		for _, dir := range dirs {
			files, err := plan.Files(root, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
				return 2
			}
			for _, rel := range files {
				defects = append(defects, plan.Verify(root, cfg, rel)...)
			}
		}
	case "project", "report":
		tr, err := projectTracker(root, *trackerFlag, *carinaRoot, *taskConnection, *repository)
		if err != nil {
			fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
			return 2
		}
		m := dialect.Harbormaster()
		for _, dir := range dirs {
			rel, derr := highestPlan(root, dir)
			if derr != nil {
				fmt.Fprintf(os.Stderr, "plangate: %v\n", derr)
				return 2
			}
			if rel == "" {
				continue
			}
			if verb == "report" {
				rep, pdef := project.Probe(root, cfg, rel, tr, m)
				defects = append(defects, pdef...)
				if code := printReport(rep); code != 0 {
					return code
				}
				continue
			}
			opts := project.Options{DryRun: *dryRun, NoRouting: *noRouting,
				// DINV-1: projection reads only the frozen (merged) plan, from
				// the configured mainline ref (config-driven-portability-v1
				// §3.2, FR-2) — "origin/main" on a default config, byte-identical.
				MergedBytes: func(rel string) ([]byte, error) {
					return exec.Command("git", "-C", root, "show", cfg.MainlineRef()+":"+rel).Output()
				}}
			rep, pdef := project.Project(root, cfg, rel, tr, m, opts)
			defects = append(defects, pdef...)
			if code := printReport(rep); code != 0 {
				return code
			}
		}
	case "coverage":
		// Feature-level Must completeness — the union across a feature's plans
		// (plangate-slice-coverage DINV-2); independent of any single plan's fold.
		for _, dir := range dirs {
			defects = append(defects, plan.FeatureCoverage(root, cfg, filepath.Base(dir))...)
		}
	case "render":
		// A single plan directory: the page is one plan's review surface.
		if len(dirs) != 1 {
			fmt.Fprintln(os.Stderr, "plangate: render takes exactly one plan directory (e.g. plangate render docs/plans/<feature>)")
			return 2
		}
		r := render.Render(root, cfg, dirs[0], *version, *out)
		defects = append(defects, r.Defects...)
		if len(r.Defects) == 0 {
			fmt.Printf("rendered %s → %s\n", dirs[0], r.OutPath)
		}
	default:
		return usage()
	}

	return emit(defects, *defectsJSON)
}

func projectTracker(root, legacyTracker, carinaRoot, connection, repository string) (tracker.Tracker, error) {
	if legacyTracker != "" && legacyTracker != "gh" && legacyTracker != "github" && legacyTracker != "carina" {
		return nil, fmt.Errorf("unknown --tracker %q (want gh|carina; select Forgejo with a task connection)", legacyTracker)
	}
	icfg, err := integrations.Load(root)
	if err != nil {
		return nil, err
	}
	if connection != "" {
		icfg.Bindings.Tasks.Connection = connection
		icfg.Bindings.Tasks.Provider = ""
		icfg.Bindings.Tasks.Root = ""
	}
	if repository != "" && icfg.Bindings.Tasks != (integrations.Binding{}) {
		icfg.Bindings.Tasks.Repository = repository
	}
	if carinaRoot == "" {
		carinaRoot = root
	}
	if icfg.Bindings.Tasks == (integrations.Binding{}) && legacyTracker != "carina" && repository == "" {
		repository, err = tracker.ResolveRepo(root)
		if err != nil {
			return nil, err
		}
	}
	resolved, err := icfg.Tasks(repository, legacyTracker, carinaRoot)
	if err != nil {
		return nil, err
	}
	switch resolved.Provider {
	case integrations.Carina:
		bin, err := resolveCarinaBin(root)
		if err != nil {
			return nil, err
		}
		return tracker.NewCarina(resolved.Root, bin), nil
	case integrations.GitHub:
		return &tracker.GH{Repo: resolved.Repository}, nil
	case integrations.Forgejo:
		client, err := forgejo.New(resolved.BaseURL, resolved.Token())
		if err != nil {
			return nil, err
		}
		return forgejotracker.NewForgejo(client, resolved.Repository), nil
	default:
		return nil, fmt.Errorf("unsupported task provider %q", resolved.Provider)
	}
}

// emit renders defects — human lines to stderr, plus one JSON line each to
// defectsJSON ("-" = stdout) when set — and maps them to the exit code: 1 if
// any defect, else 0. Shared by the normal switch tail and precheck's early
// return so both render defects identically.
func emit(defects []schema.Defect, defectsJSON string) (code int) {
	for _, d := range defects {
		fmt.Fprintln(os.Stderr, d.String())
	}
	if defectsJSON != "" {
		out := os.Stdout
		if defectsJSON != "-" {
			f, err := os.Create(defectsJSON)
			if err != nil {
				fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
				return 2
			}
			defer func() {
				if err := f.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "plangate: close defects output: %v\n", err)
					code = 2
				}
			}()
			out = f
		}
		enc := json.NewEncoder(out)
		for _, d := range defects {
			if err := enc.Encode(d); err != nil {
				fmt.Fprintf(os.Stderr, "plangate: %v\n", err)
				return 2
			}
		}
	}
	if len(defects) > 0 {
		return 1
	}
	return 0
}

// printReport writes a report's canonical JSON to stdout.
func printReport(rep *project.Report) int {
	if rep == nil {
		return 0
	}
	js, err := rep.JSON()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plangate: encode report: %v\n", err)
		return 2
	}
	if _, err := os.Stdout.Write(js); err != nil {
		fmt.Fprintf(os.Stderr, "plangate: write report: %v\n", err)
		return 2
	}
	return 0
}

// highestPlan picks the dense-numbering tip of a feature's plan dir — the
// only projectable version (design R5's cheap fail-closed check).
func highestPlan(root, dir string) (string, error) {
	files, err := plan.Files(root, dir)
	if err != nil || len(files) == 0 {
		return "", err
	}
	return files[len(files)-1], nil
}

// planDirs enumerates feature directories under the configured plans root;
// an absent root is an empty set, not an error (no plans exist yet).
func planDirs(root string, cfg config.Config) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(cfg.Plans)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read plans root %s: %w", cfg.Plans, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, cfg.Plans+"/"+e.Name())
		}
	}
	return out, nil
}

// repoRoot shares the operand discovery used by pyxis and docgate. Planning
// consumes installed repositories regardless of their implementation language.
func repoRoot() (string, error) {
	dir, err := currentWorkingDir()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	return reporoot.From(dir)
}

// resolveCarinaBin locates a runnable carina binary for the Carina tracker:
// the checkout's source or release resolution, else PATH. Absence with no
// toolchain is a clear defect (design R1/Q1).
func resolveCarinaBin(root string) (string, error) {
	return toolbin.Ensure(context.Background(), root, "carina")
}
