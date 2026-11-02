// pyxis-release — the BUILD-ONLY release tool (installer plan T11;
// installer-v1 §3.1/§3.10): the thin main over internal/release. It packs,
// verifies, and upload-or-verify-publishes the atomic system release, and
// prints the scan-derived shipped-tool list the system-release workflow
// builds (AP-27 — the shipped-set rule lives in internal/release, one
// place; this binary itself is excluded from the shipped set).
//
// Verbs (every flag long-form; the I3 discipline: unknown flags refuse, and
// a positional token on any verb refuses — these verbs take none):
//
//	pyxis-release tools   [--repo-root <dir>]
//	pyxis-release build   --version vX.Y.Z --out <dir> [--repo-root <dir>] [--jobs <n>]
//	pyxis-release pack    --version vX.Y.Z --out <dir> --binaries <dir>
//	                      [--repo-root <dir>]
//	pyxis-release verify  --version vX.Y.Z --out <dir>
//	pyxis-release publish --tag vX.Y.Z --out <dir>
//	pyxis-release pins    --version vX.Y.Z [--out <dir>] [--repo-root <dir>]
//	                      [--source-repo <owner/name>] [--check]
//	pyxis-release mirror  --tag vX.Y.Z [--out <dir>] [--repo-root <dir>]
//	                      [--source-repo <owner/name>]
//	pyxis-release cut     --version vX.Y.Z [--out <dir>] [--repo-root <dir>]
//	                      [--binaries <dir>] [--repack] [--push-tag] [--mirror]
//	                      [--jobs <n>] [--source-repo o/n]
//
// cut is the whole runbook in one line — build, pack, verify, publish, and
// regenerate install.sh's pins — stopping at the human gate that pin PR is.
// That is the complete public GitHub release; every stage re-verifies rather
// than redoing work (AP-22).
//
// mirror and cut --mirror publish only to an explicit releases.distribution
// binding, the Forgejo path. Without one they refuse: there is no implicit
// distribution target.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

var (
	processExit         = os.Exit
	releaseShippedTools = release.ShippedTools
	releaseBuild        = release.Build
	releasePack         = release.Pack
	releaseVerify       = release.Verify
	releasePublish      = release.Publish
	releasePins         = release.Pins
	releaseMirror       = release.Mirror
	releaseCut          = release.Cut
)

func main() {
	processExit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	verb := args[0]
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	repoRoot := fs.String("repo-root", ".", "source checkout to operate on")
	version := fs.String("version", "", "system tag being packed/verified (vX.Y.Z)")
	out := fs.String("out", "", "artifact directory")
	binaries := fs.String("binaries", "", "pack: prebuilt <tool>-<version>-<platform> binaries dir")
	tag := fs.String("tag", "", "publish/mirror: the release tag")
	sourceRepo := fs.String("source-repo", "agenticlab-ai/pyxis", "mirror: the canonical release repo the set is fetched from")
	repo := fs.String("repo", "", "publish: override the repository of an explicit release binding")
	distRepo := fs.String("dist-repo", "", "mirror/cut: override an explicit distribution repository binding")
	check := fs.Bool("check", false, "pins: compare only — a divergent install.sh refuses instead of being rewritten")
	jobs := fs.Int("jobs", 0, "build/cut: concurrent compiler invocations (0 = GOMAXPROCS)")
	repack := fs.Bool("repack", false, "cut: rebuild and repack even when --out already carries a verified set")
	pushTag := fs.Bool("push-tag", false, "cut: create the release tag on the source repo when absent, at the packed source commit")
	doMirror := fs.Bool("mirror", false, "cut: publish to the explicit releases.distribution binding (requires committed release pins)")
	targetName := fs.String("target", "source", "publish/pins: configured release target source|distribution")
	legacyTool := fs.String("tool", "", "legacy: docgate|codegate")
	requireBindings := fs.Bool("require-bindings", false, "release workflows: refuse legacy target defaults when configured bindings are absent")

	switch verb {
	case "tools", "build", "pack", "verify", "publish", "pins", "mirror", "cut", "legacy", "workflows":
	case "--help", "-h", "help":
		usage(stderr)
		return 0
	default:
		printMessage(stderr, "pyxis-release: unknown verb %q\n", verb)
		usage(stderr)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2 // flag already printed the unknown-flag refusal
	}
	// Fail closed on any positional: these verbs take flags only, so a
	// stray token — flag-like or not — means a mangled invocation
	// (incident I3's class: a dropped flag must never silently change what
	// runs).
	if rest := fs.Args(); len(rest) > 0 {
		printMessage(stderr, "pyxis-release: unexpected argument %q — %s takes flags only (nothing was run)\n", rest[0], verb)
		return 2
	}

	ctx := context.Background()
	var sourceTarget, distTarget, selectedTarget *integrations.ResolvedRepository
	if verb == "publish" || verb == "pins" || verb == "mirror" || verb == "cut" || verb == "legacy" {
		ic, err := integrations.Load(*repoRoot)
		if err != nil {
			printLine(stderr, err)
			return 2
		}
		if *targetName != "source" && *targetName != "distribution" {
			printLine(stderr, "pyxis-release: --target must be source or distribution")
			return 2
		}
		seen := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
		resolveTarget := func(name, legacy string, configured integrations.Binding, override bool) (*integrations.ResolvedRepository, error) {
			if configured == (integrations.Binding{}) {
				return nil, nil
			}
			r, err := ic.Release(name, legacy)
			if err != nil {
				return nil, err
			}
			if override {
				r.Repository = legacy
				if err := integrations.ValidateRepository(legacy); err != nil {
					return nil, err
				}
			}
			return &r, nil
		}
		sourceTarget, err = resolveTarget("source", *sourceRepo, ic.Bindings.Releases.Source, seen["source-repo"])
		if err != nil {
			printLine(stderr, err)
			return 2
		}
		distTarget, err = resolveTarget("distribution", *distRepo, ic.Bindings.Releases.Distribution, seen["dist-repo"])
		if err != nil {
			printLine(stderr, err)
			return 2
		}
		if sourceTarget != nil {
			*sourceRepo = sourceTarget.Repository
		}
		if distTarget != nil {
			*distRepo = distTarget.Repository
		}
		selectedTarget = sourceTarget
		if *targetName == "distribution" {
			selectedTarget = distTarget
		}
		if selectedTarget != nil && seen["repo"] {
			copy := *selectedTarget
			copy.Repository = *repo
			if err := integrations.ValidateRepository(*repo); err != nil {
				printLine(stderr, err)
				return 2
			}
			selectedTarget = &copy
		}
		if *requireBindings {
			missing := selectedTarget == nil
			if verb == "mirror" || verb == "cut" {
				missing = sourceTarget == nil || ((verb == "mirror" || *doMirror) && distTarget == nil)
			}
			if missing {
				printLine(stderr, "pyxis-release: required release binding is absent; update the workflow or restore its explicit source/distribution binding (nothing was published)")
				return 2
			}
			for _, target := range []*integrations.ResolvedRepository{sourceTarget, distTarget, selectedTarget} {
				if target != nil && !release.WorkflowCredentialAllowed(target.TokenEnv) {
					printLine(stderr, "pyxis-release: release credential reference conflicts with a reserved workflow environment key")
					return 2
				}
			}
		}
		if (seen["repo"] && selectedTarget == nil) || (seen["dist-repo"] && distTarget == nil) || (*targetName == "distribution" && selectedTarget == nil) {
			printLine(stderr, "pyxis-release: repository publication requires an explicit release binding")
			return 2
		}
	}
	switch verb {
	case "workflows":
		paths, err := release.MaterializeSourceWorkflows(*repoRoot)
		if err != nil {
			printLine(stderr, err)
			return 1
		}
		for _, path := range paths {
			printLine(stdout, path)
		}
		return 0
	case "legacy":
		if err := release.BuildLegacy(ctx, *legacyTool, *tag, *repoRoot, *out); err != nil {
			printLine(stderr, err)
			return 1
		}
		if _, err := releasePublish(ctx, release.PublishOptions{LegacyTool: *legacyTool, Tag: *tag, OutDir: *out, Target: selectedTarget, Repo: *repo, Runner: wire.GHRunner{}}); err != nil {
			printLine(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "published legacy %s\n", *tag) // Reporting is best effort; publication already completed.
		return 0
	case "tools":
		tools, err := releaseShippedTools(*repoRoot)
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		for _, t := range tools {
			printLine(stdout, t)
		}
		return 0
	case "build":
		res, err := releaseBuild(ctx, release.BuildOptions{
			Version:  *version,
			RepoRoot: *repoRoot,
			OutDir:   *out,
			Jobs:     *jobs,
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		printMessage(stdout, "built %s: %d binaries (%d tools × %d platforms) in %s\n",
			*version, len(res.Binaries), len(res.Tools), len(release.Platforms), *out)
		return 0
	case "pack":
		res, err := releasePack(ctx, release.PackOptions{
			Version:     *version,
			RepoRoot:    *repoRoot,
			OutDir:      *out,
			BinariesDir: *binaries,
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		printMessage(stdout, "packed %s: %d artifact(s), %d payload file(s), source commit %s\n",
			*version, len(res.Artifacts), len(res.Manifest.Payload.Files), res.Manifest.SourceCommit)
		return 0
	case "verify":
		if err := releaseVerify(*out, *version); err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		printMessage(stdout, "verified %s against %s and the release manifest\n", *out, release.SumsName)
		return 0
	case "publish":
		res, err := releasePublish(ctx, release.PublishOptions{
			Tag:    *tag,
			OutDir: *out,
			Runner: wire.GHRunner{},
			Repo:   *repo,
			Target: selectedTarget,
			Mirror: *targetName == "distribution",
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		outcome := "completed"
		if res.Created {
			outcome = "created"
		}
		printMessage(stdout, "publish %s for %s: %d artifact(s) uploaded-or-verified\n", outcome, *tag, len(res.Actions))
		return 0
	case "pins":
		res, err := releasePins(ctx, release.PinsOptions{
			Version:    *version,
			RepoRoot:   *repoRoot,
			OutDir:     *out,
			SourceRepo: *sourceRepo,
			Runner:     wire.GHRunner{},
			Check:      *check,
			Target:     selectedTarget,
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		printMessage(stdout, "pins %s: install.sh %s from %s\n", *version, res.Action, res.Source)
		if res.Action == "written" {
			printMessage(stdout, "  commit it — the pins are the consumer trust anchor, and mirror refuses until they land\n")
		}
		return 0
	case "mirror":
		res, err := releaseMirror(ctx, release.MirrorOptions{
			Tag:          *tag,
			OutDir:       *out,
			RepoRoot:     *repoRoot,
			SourceRepo:   *sourceRepo,
			DistRepo:     *distRepo,
			Runner:       wire.GHRunner{},
			SourceTarget: sourceTarget, DistTarget: distTarget,
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		set := "the prefetched set in " + *out
		if res.Downloaded {
			set = "the canonical set from " + *sourceRepo
		}
		printMessage(stdout, "mirror repository %s for %s: %s, %d artifact(s) uploaded-or-verified, install.sh %s\n",
			*distRepo, *tag, set, len(res.Publish.Actions), res.InstallSH)
		return 0
	case "cut":
		res, err := releaseCut(ctx, release.CutOptions{
			Version:      *version,
			RepoRoot:     *repoRoot,
			OutDir:       *out, // "" = release.DefaultOutDir under --repo-root
			BinariesDir:  *binaries,
			Repack:       *repack,
			PushTag:      *pushTag,
			Mirror:       *doMirror,
			Jobs:         *jobs,
			SourceRepo:   *sourceRepo,
			DistRepo:     *distRepo,
			Runner:       wire.GHRunner{},
			Log:          func(line string) { printMessage(stdout, "cut %s: %s\n", *version, line) },
			SourceTarget: sourceTarget, DistTarget: distTarget,
		})
		if err != nil {
			printMessage(stderr, "pyxis-release: %v\n", err)
			return 1
		}
		printCutOutcome(stdout, *version, res, distTarget != nil)
		return 0
	}
	return 2 // unreachable: the verb set is closed above
}

// printCutOutcome names what remains of the release. The pin PR is a human
// gate, so the verb never exits green on a half-delivered release without
// saying so.
func printCutOutcome(w io.Writer, version string, res release.CutResult, bound bool) {
	switch {
	case res.Mirror != nil:
		printMessage(w, "cut %s: DONE — canonical release, pins, and the bound distribution are current\n", version)
	case res.Pins.Action == "written":
		printMessage(w, `cut %s: canonical release published; install.sh now pins it.
  Remaining (the human gate — the pin PR):
    git add install.sh && git commit -m "chore(release): point install.sh at %s"
    open the PR and get it approved and merged; installs and upgrades read the pins from main
`, version, version)
		if bound {
			printMessage(w, "    then, for the bound distribution target: go run ./cmd/pyxis-release cut --version %s --mirror\n", version)
		}
	case bound:
		printMessage(w, "cut %s: canonical release published; install.sh already pins it — re-run with --mirror to publish the bound distribution\n", version)
	default:
		printMessage(w, "cut %s: DONE — canonical release published; install.sh already pins it\n", version)
	}
}

// printMessage keeps diagnostics and progress separate from the release outcome.
func printMessage(w io.Writer, format string, args ...any) {
	// Best-effort output: run returns the operation result, and a failed receipt
	// cannot undo completed publication.
	_, _ = fmt.Fprintf(w, format, args...)
}

func usage(w io.Writer) {
	printMessage(w, "%s", `pyxis-release - the build-only system-release tool (never shipped)

usage:
  pyxis-release cut     --version vX.Y.Z [--out dist] [--repo-root <dir>] [--binaries <dir>] [--repack]
                        [--push-tag] [--mirror] [--jobs <n>] [--source-repo o/n]
                                                                  # THE ONE-LINER: build -> pack -> verify -> publish -> regenerate
                                                                  # install.sh's pins, then stop at the pin PR (the human gate). That
                                                                  # is the complete public GitHub release. --mirror continues to an
                                                                  # explicit releases.distribution binding once that PR merges,
                                                                  # reusing the already-verified set instead of repacking.
  pyxis-release tools   [--repo-root <dir>]                      # scan-derived shipped-tool list (AP-27)
  pyxis-release workflows [--repo-root <dir>]                    # materialize selected Forgejo source CI templates
  pyxis-release legacy --tool docgate|codegate --tag <tool>/vX.Y.Z --out <empty-dir> [--target source|distribution]
  pyxis-release build   --version vX.Y.Z --out <dir> [--repo-root <dir>] [--jobs <n>]
                                                                  # the shipped set x platforms, -trimpath/CGO_ENABLED=0
  pyxis-release pack    --version vX.Y.Z --out <dir> --binaries <dir> [--repo-root <dir>]
  pyxis-release verify  --version vX.Y.Z --out <dir>              # re-hash the set against SHA256SUMS + manifest
  pyxis-release publish --tag vX.Y.Z --out <dir> [--repo <owner/name>]  # upload-or-verify through the selected provider (never clobbers)
  pyxis-release pins    --version vX.Y.Z [--out <dir>] [--repo-root <dir>] [--source-repo o/n] [--check]
                                                                  # regenerate install.sh's BEGIN PINS block (the consumer trust
                                                                  # anchor) from a packed set (--out) or, with no --out, from the
                                                                  # PUBLISHED release's own bytes. --check compares, never writes.
  pyxis-release mirror  --tag vX.Y.Z [--out <dir>] [--repo-root <dir>] [--source-repo o/n]
                                                                  # fetch and verify the canonical set, then publish it and install.sh to
                                                                  # the explicit releases.distribution binding; refuses without one.
                                                                  # --dist-repo only overrides that binding's repository.

  Release workflows pass --require-bindings to refuse legacy defaults if
  an explicit source/distribution binding is removed after generation.
`)
}

// printLine preserves command outcomes when a diagnostic sink is closed.
func printLine(w io.Writer, values ...any) {
	_, _ = fmt.Fprintln(w, values...) // Reporting is best effort; the action determines the exit status.
}
