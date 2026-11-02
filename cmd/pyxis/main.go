// pyxis — the front-door orchestrator (installer-v1 §3.4; plan
// installer/001 T4): the product face of a pyxis-operated repo and the
// single acquisition-and-execution path for every engine tool.
//
// Verb surface (frozen by the design; the solution shape's spelling):
//
//	converge [--repair] [--preview] — reconcile the tree against the pinned
//	manifest per ownership class via internal/converge (plan installer/001
//	T5); --preview reports every write and external operation and performs
//	none (FR-19)
//	repair [--preview] — the same converge, named for a damaged tree
//	doctor — the read-only drift/health/advisory report via converge.Doctor
//	(plan installer/001 T6): zero mutations, exit 0 healthy / 1 findings
//	review-rearm — explicitly create a fresh attributable recovery epoch for
//	one exhausted configured-review obligation without erasing prior evidence
//	init [knob flags] [--preview] [--yes] --system-version … --manifest-sha256 …
//	— bring the working directory onto the pinned release via
//	internal/installer (plan installer/001 T7): admit the target (UB-11),
//	resolve the five knobs (flags/answers-file promptless, else interactive)
//	and the committer identity (UB-10), converge, and write the two-commit
//	epoch; --preview reports the full would-do set and performs none of it
//	adopt --system-version … --manifest-sha256 … [--artifact-source url] —
//	negotiate factory management for an existing, non-empty repository via
//	internal/adopt (plan existing-repo-onboarding/001 T3): dispatch by tree
//	state (a non-git target refuses by name, UB-5; an empty tree routes to
//	init, UB-1; a malformed prior config refuses by name, UB-7), classify the
//	tree against the pinned manifest by reusing converge's classification,
//	and write a draft adoption-plan naming every acted-on path and the
//	never-touch set — the manifest's complement. Mutates no tracked file
//	(apply and undo are separate, later verbs)
//	exec <tool> -- <argv…> — resolve → verify → execute via internal/resolve
//	version — the build version (ldflags-settable)
//
// Argument discipline (incident I3; prd:installer/UB-9; DINV-5): every
// verb parses fail-closed through internal/cliguard — flags precede
// positionals; a flag-like positional or an unknown flag refuses with
// usage BEFORE any action. exec REQUIRES the "--" separator; everything
// after it passes argv-verbatim to the resolved tool — never parsed,
// never interpreted, and the environment passes through untouched the
// same way.
//
// Exit codes, the plangate convention: 0 clean (or the tool's own code
// relayed by the non-execve fallback), 1 named refusals (init's
// admission/identity/converge refusals, config defects, resolution
// refusals), 2 usage or environment. On unix a successful exec never
// returns — the verified tool image replaces the process via execve and
// owns the exit code outright.
//
// No verb performs model inference (prd:installer/NFR-6): this binary is
// purely mechanical.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	_ "github.com/agenticlab-ai/pyxis/internal/adapters/release/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/cliguard"
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
	"github.com/agenticlab-ai/pyxis/internal/reporoot"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/termstyle"
	"github.com/agenticlab-ai/pyxis/internal/upgrade"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// version is what `pyxis version` prints — set at release build time via
// `-ldflags "-X main.version=vX.Y.Z"`; "source" marks a source-tree build.
var version = "source"

// The consequential seams, package vars so tests can (a) prove NO
// resolution or execution happens on any refusal path (I3: refuse before
// action) and (b) capture the exact argv/exit-code contract without
// replacing the test process. Defaults are the real thing.
var (
	// resolveTool is the acquisition core (installer/001 T3).
	resolveTool = resolve.Tool
	// execFn runs the verified binary: exec_unix.go (true execve — never
	// returns on success) or exec_other.go (child process + exit-code relay).
	execFn = execTool
	// goRunFn is the source-mode leg: `go run ./cmd/<tool> <argv…>` from
	// the repo root (DINV-8).
	goRunFn = goRun
	// convergeRun is the ownership-class reconcile engine (installer/001 T5).
	convergeRun = converge.Run
	// doctorRun is the read-only health face (installer/001 T6).
	doctorRun = converge.Doctor
	// installerInit is the init engine's front half (installer/001 T7).
	installerInit = installer.Init
	// upgradeRun brings the installed binary forward (#1028) — a package var
	// so a CLI test can swap it, exactly as the engines above are swapped.
	upgradeRun = upgrade.Run
	// adoptInspect is the adopt verb's dispatch+classify+plan read
	// (existing-repo-onboarding/001 T3).
	adoptInspect = adopt.Inspect
	// adoptApply / adoptUndo are the apply and undo verbs' engines
	// (existing-repo-onboarding/001 T8, T10) — package vars so a CLI test can
	// swap them, exactly as adoptInspect is swapped.
	adoptApply = adopt.Apply
	adoptUndo  = adopt.Undo
)

func run(args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()
	if len(args) > 0 {
		if _, registered := rootCommands.commands[args[0]]; registered {
			var stop context.CancelFunc
			ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()
		}
	}
	return runContext(ctx, args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stderr)
	}
	verb, rest := args[0], args[1:]
	if code, ok := rootCommands.dispatch(ctx, verb, rest, stdout, stderr); ok {
		return code
	}
	switch verb {
	case "credential":
		return credentialVerb(rest, os.Stdin, stdout, stderr)
	case "init":
		return initVerb(rest, stdout, stderr)
	case "surfaces":
		return surfacesVerb(rest, stdout, stderr)
	case "doctor":
		return doctorVerb(rest, stdout, stderr)
	case "converge", "repair":
		return convergeVerb(verb, rest, stdout, stderr)
	case "adopt":
		return adoptVerb(rest, stdout, stderr)
	case "apply":
		return applyVerb(rest, stdout, stderr)
	case "undo":
		return undoVerb(rest, stdout, stderr)
	case "watch":
		return watchVerb(rest, stdout, stderr)
	case "status":
		return statusVerb(rest, stdout, stderr)
	case "draft":
		return draftVerb(rest, stdout, stderr)
	case "approve":
		return approveVerb(rest, stdout, stderr)
	case "review-rearm":
		return reviewRearmVerb(rest, stdout, stderr)
	case "exec":
		return execVerb(rest, stderr)
	case "upgrade":
		return upgradeVerb(rest, stdout, stderr)
	case "version":
		return versionVerb(rest, stdout, stderr)
	case "shell":
		return shellVerb(rest, stdout, stderr)
	case "help", "-h", "--help":
		return usage(stderr)
	default:
		terminalPrintf(stderr, "pyxis: unknown verb %q\n", verb)
		return usage(stderr)
	}
}

func usage(w io.Writer) int {
	registered := strings.Join(rootCommands.usages(), "\n")
	terminalPrintln(w, `usage: pyxis <verb> …
verbs:
`+registered+`
	  draft open <feature> | draft reopen <feature> | draft abandon <feature> --reason <text> | draft status [<feature>] | draft digest <feature> | draft notice <feature> | draft checkpoint
	  draft prototype <feature> --idea <file> | draft prototype <feature> --remove
	                           manage feature contract drafts and record the first-three-feature checkpoint
  init [--answers f] [--integrations f] [--tracker t] [--ci-mode m] [--wiring repo=…,protection=…]
       [--cli-surfaces a,b|auto] [--argus] [--preview] [--yes] [--verbose]
       --system-version vX.Y.Z --manifest-sha256 <hex>
       [--artifact-source url] [--remote-url path-or-URL]
                           bring the working directory onto the pinned release:
                           admit the target, resolve the five knobs (flags/answers
                           promptless, else interactive) and the committer identity,
                           converge, commit and verify the initial tree, then push
                           to the configured destination; --preview
                           reports the full would-do set and performs none of it
  converge [--repair] [--preview] [--verbose]
                           reconcile the tree against the pinned release manifest
                           per ownership class; --preview reports every write and
                           external operation and performs none of them
  repair [--preview] [--verbose]
                           the same converge, named for a damaged tree
  doctor [--verbose]       read-only drift, health, and advisory report over
                           the same manifest predicate — zero mutations;
                           exit 0 healthy, 1 findings
  review-rearm --repo <path> --scope <feature/NNN> --obligation <id>
               --principal <operator-id> --reason <text>
                           explicitly rearm one exhausted configured-review
                           obligation into a fresh, attributable recovery epoch
  surfaces [--all|--detected|--set a,b] [--preview]
                           which agent CLIs get pyxis's skills. init resolves
                           this once by probing PATH, so a machine that had no
                           agent CLI then records none — and installing one
                           later wires nothing until this verb says so. Bare, it
                           reports and performs nothing; --detected takes what
                           is installed here, --all takes every supported
                           surface (installed or not, for a repo you are
                           provisioning for others), --set names them exactly.
  adopt --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]
                           negotiate factory management for an existing,
                           non-empty repository: a non-git target refuses by
                           name, an empty tree routes to init, a malformed
                           prior config refuses by name; otherwise classify
                           the tree and write a draft adoption-plan naming
                           every acted-on path and the never-touch set —
                           mutates no tracked file
  apply --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]
                           materialise the approved adoption plan: refuse an
                           absent or stale plan, write the managed paths and
                           collision resolutions, chain the hooks, record the
                           install epoch without a commit, stamp the plan applied
  undo --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]
                           reverse an applied adoption, preserving post-adoption
                           work: unchain, remove the installation's pristine
                           paths, clear the epoch, name the manual entries — an
                           operator-edited artifact is preserved, never deleted
  upgrade [--preview] [--allow-downgrade] [--channel url]
                           bring the INSTALLED pyxis binary onto the release
                           the channel pins: fetch this platform's binary,
                           verify its sha256 before anything is written, and
                           replace the running executable, then reinstall
                           the zsh worktree launcher from that binary, as
                           pyxis shell install does. --preview reports
                           it all and performs none of it; --allow-downgrade
                           admits a pinned release older than the installed
                           one (never implicit); --channel takes the channel
                           ROOT of a mirror (https, the directory serving
                           install.sh) and defaults to the channel the install
                           recorded, else public GitHub Releases.
                           --to installs an explicit release and requires its
                           --sha256 (the same pairing install.sh requires of
                           PYXIS_VERSION + PYXIS_SHA256), plus
                           --manifest-sha256 unless --to is the release the
                           channel pins — the pin hand-off is unconditional,
                           so every install leaves pyxis init resolving what
                           was just installed. With all three, the channel is
                           never consulted. An installed
                           repository keeps its own pin and moves by converge —
                           upgrade names that path, never edits it
  approve <pull-number> --repo <owner/repo> --head <full-sha>
          --drive <slug> --evidence <review-summary-and-references>
          [--digest <prepared-contract-digest>] [--capture <text>]
                           publish an authenticated exact-head Argus approval
                           under pre-merge mainline policy and approve-merge;
                           same-account authors use a typed COMMENTED review;
                           required checks and host protection still apply
  exec <tool> -- <argv…>   resolve, verify, and execute a pinned engine tool
  shell install            enable interactive worktree startup in zsh
  shell mainline <repository>
                           print the repository's configured remote and
                           default branch, one per line
  version                  print the build version
the exec grammar (installer-v1 §3.4, DINV-5): everything before -- belongs to
pyxis and parses fail-closed — a flag-like positional, an unknown flag, or a
missing -- refuses before any action (incident I3); everything after -- passes
to the tool argv-verbatim — pyxis never interprets it.`)
	return 2
}

// verbUsage is the one-line grammar per verb, printed with every refusal.
var verbUsage = map[string]string{
	"init":     "pyxis init [--answers f] [--integrations f] [--tracker t] [--ci-mode m] [--wiring repo=…,protection=…] [--cli-surfaces a,b|auto] [--argus] [--preview] [--yes] --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url]",
	"converge": "pyxis converge [--repair] [--preview] [--verbose]",
	"repair":   "pyxis repair [--preview] [--verbose]",
	"doctor":   "pyxis doctor [--verbose]",
	"surfaces": "pyxis surfaces [--all|--detected|--set a,b] [--preview]",
	"adopt":    "pyxis adopt --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]",
	"apply":    "pyxis apply --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]",
	"undo":     "pyxis undo --system-version vX.Y.Z --manifest-sha256 <hex> [--artifact-source url] [--verbose]",
	"upgrade":  "pyxis upgrade [--preview] [--allow-downgrade] [--to vX.Y.Z --sha256 hex [--manifest-sha256 hex]] [--channel url]",
	"exec":     "pyxis exec <tool> -- <argv…>",
	"version":  "pyxis version",
	"shell":    "pyxis shell install | pyxis shell mainline <repository>",
}

// refuse renders a usage refusal — the named reason plus the verb's grammar
// — and returns 2. Nothing has run when this prints (I3: refusal before
// action).
func refuse(stderr io.Writer, verb, reason string) int {
	terminalPrintf(stderr, "pyxis: %s: %s — nothing was run\nusage: %s\n", verb, reason, verbUsage[verb])
	return 2
}

// misparse renders a cliguard refusal through refuse; -h/-help gets the
// full usage instead of a one-liner.
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

// initVerb is `pyxis init` — the install pipeline's front half (installer-v1
// §3.5; plan installer/001 T7) over internal/installer. Guard discipline is
// the I3 house rule: every knob flag, the required pin pair, and --preview/
// --yes parse fail-closed through cliguard before any probe or write; init
// takes no positional (the target is the working directory). An answers file
// is loaded first, flags fold over it (flags win), and AnswersProvided is set
// when ANY flag or answers-file input arrived — a promptless run then (FR-4).
// Exit: 0 success (or a completed preview), 1 named refusals (admission,
// identity, converge, config), 2 usage/environment.
func initVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	answersPath := fs.String("answers", "", "answers file (the five knobs + optional git_name/git_email; flags win)")
	tracker := fs.String("tracker", "", "issue tracker: github|forgejo|carina")
	remoteURL := fs.String("remote-url", "", "Git publication destination when no remote is configured (local bare path or URL)")
	integrationPath := fs.String("integrations", "", "configuration file carrying the integrations capability bindings to install")
	ciMode := fs.String("ci-mode", "", "CI mode: local|actions|both")
	wiring := fs.String("wiring", "", "code-host wiring: repo=use|create,protection=none|required-checks|checks+review")
	cliSurfaces := fs.String("cli-surfaces", "", "agent CLI surfaces: a comma-separated list, or auto to detect")
	argus := fs.Bool("argus", false, "enable the delegated-authority AI Engineer (opt-in; off by default)")
	preview := fs.Bool("preview", false, "report the full would-do set — every check outcome and the declared wiring operations included — and perform nothing (FR-24)")
	yes := fs.Bool("yes", false, "accept the recommended default for every unset knob; never prompt")
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	systemVersion := fs.String("system-version", "", "the system release pin (defaults to install.sh's record / PYXIS_SYSTEM_VERSION)")
	manifestSHA256 := fs.String("manifest-sha256", "", "the release manifest sha256, the trust root (defaults to install.sh's record / PYXIS_MANIFEST_SHA256)")
	artifactSource := fs.String("artifact-source", "", "FR-22 mirror/fork override for the artifact host")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "init", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "init", fmt.Sprintf("takes no arguments — the target is the working directory (got %q)", pos[0]))
	}

	// The pin pair (DINV-2). Resolve each from the flag, else the PYXIS_* env
	// var, else the release.env install.sh recorded
	// — so `pyxis init` needs no pin flags on a machine set up by the one-liner.
	// Refuse by name before any probe if none of the three supply it.
	fileVer, fileSHA := installReleaseDefaults()
	pinVersion := firstNonEmpty(*systemVersion, fileVer)
	pinSHA := firstNonEmpty(*manifestSHA256, fileSHA)
	if pinVersion == "" || pinSHA == "" {
		return refuse(stderr, "init", "the system pin pair is required — pass --system-version and --manifest-sha256, or install via install.sh (which records them at $XDG_CONFIG_HOME/pyxis/release.env)")
	}

	// The answers file, then flags folded over it (flags win). Any flag or
	// file input marks the run promptless (FR-4).
	ans := installer.Answers{}
	provided := false
	if *answersPath != "" {
		loaded, lerr := installer.LoadAnswersFile(*answersPath)
		if lerr != nil {
			terminalPrintf(stderr, "pyxis: %v\n", lerr)
			return 1
		}
		ans = loaded
		provided = true
	}
	// Which knob flags were actually set (flag.Visit reports only those). A
	// set knob overrides the file; the closure returns no error, so the
	// wiring/surfaces parses that CAN fail happen after it.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	for _, name := range []string{"tracker", "ci-mode", "wiring", "cli-surfaces", "argus", "remote-url"} {
		if set[name] {
			provided = true
		}
	}
	if set["tracker"] {
		ans.Tracker = *tracker
	}
	if set["remote-url"] {
		ans.RemoteURL = *remoteURL
	}
	if set["ci-mode"] {
		ans.CIMode = *ciMode
	}
	if set["wiring"] {
		repo, prot, werr := installer.ParseWiringArg(*wiring)
		if werr != nil {
			return refuse(stderr, "init", werr.Error())
		}
		ans.WiringRepo, ans.WiringProtection = repo, prot
	}
	if set["cli-surfaces"] {
		ans.CLISurfaces = installer.SplitSurfacesArg(*cliSurfaces)
	}
	if set["argus"] {
		v := *argus
		ans.Argus = &v
	}

	target, err := os.Getwd()
	if err != nil {
		terminalPrintf(stderr, "pyxis: init: getwd: %v\n", err)
		return 2
	}

	// Arm signal handling before journal setup makes the first filesystem
	// write. Otherwise an early signal can leave a partial log directory.
	// A SIGINT/SIGTERM mid-apply must still unwind the journal rather than
	// leave debris (UB-3's signal-interruption clause; plan init-hardening/001
	// T4): git/gh subprocess calls already honor ctx cancellation
	// (exec.CommandContext), so catching the signal here is what turns a real
	// Ctrl-C into that cancellation instead of an immediate, uncaught exit.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	runDone := make(chan struct{})
	defer close(runDone)
	// The signal watcher, owned by this function: it exits on the first
	// signal or when runDone closes (deferred, so every return path ends
	// it). Catching the signal is what lets an interrupted run unwind its
	// journal instead of dying mid-write (UB-3). Two things the first cut
	// of this got wrong (#954): the interrupt was silent, so an operator
	// seeing nothing happen kept pressing — and every later press was
	// swallowed too, because a registered handler disables Go's default
	// exit. So: say the run is stopping, then unregister, which makes a
	// second press exit immediately. The window it opens is honest — a
	// second, deliberate interrupt can land mid-rollback, which is the
	// operator overriding the unwind after being told it is running.
	go func() {
		select {
		case <-sigCh:
			terminalPrintln(stderr, "pyxis: interrupt received — stopping and rolling back; press Ctrl-C again to exit immediately")
			cancel()
			signal.Stop(sigCh)
		case <-runDone:
		}
	}()
	// The interactive seam: a real run wires the process streams; a promptless
	// run (any input, or --yes) leaves Stdin nil so the engine never prompts.
	logRoot := target
	if *preview {
		logRoot = ""
	}
	logger, journalPath := initRunLogger(logRoot, stderr, "init", *verbose)
	opts := installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: pinVersion, ManifestSHA256: pinSHA},
		ArtifactSource:  *artifactSource,
		Answers:         ans,
		AnswersProvided: provided,
		AcceptDefaults:  *yes,
		Preview:         *preview,
		Stdout:          stdout,
		Logger:          logger,
	}
	if *integrationPath != "" {
		b, err := os.ReadFile(*integrationPath)
		if err != nil {
			terminalPrintf(stderr, "pyxis: init integrations: %v\n", err)
			return 1
		}
		profile, err := integrations.Parse(b)
		if err != nil {
			terminalPrintf(stderr, "pyxis: init integrations: %v\n", err)
			return 1
		}
		if profile.SchemaVersion == "" {
			return refuse(stderr, "init", "--integrations file must contain an integrations block")
		}
		opts.Integrations = profile
		opts.AnswersProvided = true
		provided = true
	}
	if !provided && !*yes {
		opts.Stdin = os.Stdin // pure-interactive elicitation (FR-3)
	}

	rep, ierr := installerInit(ctx, opts)
	if ierr != nil {
		// An interrupted run names the interruption, never whichever
		// subprocess happened to notice first (#954). Both shapes count: a
		// context.Canceled chain, and a subprocess the run context KILLED
		// mid-flight — which reports "signal: killed" and carries no
		// cancellation in its chain, so ctx decides.
		interrupted := errors.Is(ierr, context.Canceled) || ctx.Err() != nil
		switch {
		case errors.Is(ierr, installer.ErrInterrupted):
			// Interrupted during the interview: checks-then-go puts every
			// write after it, so "nothing was written" is provable here and
			// claimed nowhere else.
			terminalPrintln(stderr, "pyxis: init: interrupted — nothing was written; this directory is unchanged")
		case interrupted:
			terminalPrintln(stderr, "pyxis: init: interrupted — the run stopped before it finished")
			terminalPrintf(stderr, "  where it stopped: %v\n", ierr)
		default:
			terminalPrintf(stderr, "pyxis: %v\n", ierr)
		}
		if rep.Rollback != nil {
			printRollbackReport(stderr, *rep.Rollback)
		} else if interrupted && !errors.Is(ierr, installer.ErrInterrupted) {
			terminalPrintln(stderr, "  no rollback ran — check this directory before re-running")
		}
		printJournalPointer(stderr, journalPath)
		return 1
	}
	printInitReport(stdout, rep)
	printJournalPointer(stdout, journalPath)
	return 0
}

// upgradeVerb brings the INSTALLED pyxis binary forward (#1028). Binary
// scope by ruling: an installed repository keeps its own pin and moves by
// converge, so this prints a pointer at that path rather than touching it.
func upgradeVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	preview := fs.Bool("preview", false, "report the would-do set and perform none of it")
	allowDowngrade := fs.Bool("allow-downgrade", false, "install the pinned release even when it is older than the installed one")
	to := fs.String("to", "", "install this exact release instead of the channel's pin (needs --sha256)")
	sha := fs.String("sha256", "", "the sha256 of --to's binary for this platform")
	manifestSHA := fs.String("manifest-sha256", "", "the release manifest's sha256 for --to (required unless --to is the channel's pin)")
	channel := fs.String("channel", "", "artifact channel root of a mirror (default: the channel this machine installed from, else public GitHub Releases)")
	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "upgrade", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "upgrade", fmt.Sprintf("takes no arguments (got %q)", pos[0]))
	}

	// Fall back to the channel this machine installed from: a mirror install
	// already knows where releases live, and upgrading must not demand a flag
	// carrying what the install recorded (prd:installer/FR-30). No channel at
	// all selects public GitHub Releases, anonymously.
	ch := strings.TrimRight(strings.TrimSpace(*channel), "/")
	root, err := os.Getwd()
	if err != nil {
		return refuse(stderr, "upgrade", err.Error())
	}
	var artifactRelease *integrations.ResolvedRepository
	if ch == "" {
		cfg, err := integrations.Load(root)
		if err != nil {
			return refuse(stderr, "upgrade", err.Error())
		}
		repository, found, err := cfg.Artifacts()
		if err != nil {
			return refuse(stderr, "upgrade", err.Error())
		}
		if !found {
			repository, found, err = releaseenv.Read().ArtifactRepository()
		}
		if err != nil {
			return refuse(stderr, "upgrade", err.Error())
		}
		if found {
			artifactRelease = &repository
		}
	}
	if ch == "" && artifactRelease == nil {
		ch = releaseenv.Channel()
	}

	rep, uerr := upgradeRun(context.Background(), upgrade.Options{
		Root:           root,
		Release:        artifactRelease,
		Channel:        ch,
		To:             *to,
		SHA256:         *sha,
		ManifestSHA256: *manifestSHA,
		Current:        version,
		Preview:        *preview,
		AllowDowngrade: *allowDowngrade,
		ShellSetup:     upgrade.InstalledShellSetup,
	})
	if uerr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", uerr)
		// Never let a failure hide a mutation that already happened: the
		// binary can be swapped before a later step fails, and an operator
		// told only "it failed" would keep running the new one unaware.
		if rep.BinaryReplaced {
			terminalPrintf(stderr, "  note: %s WAS already replaced with %s — only the steps after it failed\n", rep.BinaryPath, rep.Target)
			if rep.PinRecorded == "" {
				terminalPrintln(stderr, "  the release pin may still name the previous version; re-run `pyxis upgrade` once the cause above is fixed")
			} else {
				terminalPrintln(stderr, "  the release pin is recorded; re-run `pyxis upgrade` once the cause above is fixed")
			}
		}
		return 1
	}

	st := termstyle.Detect(stdout)
	switch {
	case rep.UpToDate && rep.PinRepaired:
		terminalPrintf(stdout, "pyxis is already on %s — binary unchanged\n", st.Bold(rep.Current))
		terminalPrintf(stdout, "  pin     %s  repaired ✓ (it no longer described this install)\n", rep.PinRecorded)
	case rep.UpToDate:
		terminalPrintf(stdout, "pyxis is already on %s — nothing to do\n", st.Bold(rep.Current))
	case rep.Preview:
		terminalPrintf(stdout, "pyxis upgrade preview — %s %s %s\n", rep.Current, upgradeArrow(rep), st.Bold(rep.Target))
		terminalPrintf(stdout, "  would replace %s\n", rep.BinaryPath)
		terminalPrintf(stdout, "  would verify  %s against sha256 %s\n", rep.Artifact, short(rep.SHA256))
		terminalPrintln(stdout, "nothing was performed")
	default:
		terminalPrintf(stdout, "pyxis upgrade — %s %s %s\n", rep.Current, upgradeArrow(rep), st.Bold(st.Green(rep.Target)))
		terminalPrintf(stdout, "  binary  %s  replaced ✓ (sha256 %s verified)\n", rep.BinaryPath, short(rep.SHA256))
		if rep.PinRecorded != "" {
			terminalPrintf(stdout, "  pin     %s  recorded ✓\n", rep.PinRecorded)
		}
	}
	if rep.Shell != "" {
		terminalPrintf(stdout, "  shell   %s ✓\n", rep.Shell)
	}

	// The repo half is a separate, deliberate step: name its real path
	// whenever this working tree is an installed repo pinned somewhere else
	// — including when the BINARY was already current, which is the steady
	// state after any converge lag and the case that most needs saying.
	if !rep.Preview {
		if root, rerr := reporoot.Find(); rerr == nil {
			if pin := upgrade.RepoPin(root); pin != "" && pin != rep.Target {
				terminalPrintf(stdout, "\nthis repo pins %s, not %s — set system.version (and its manifest_sha256)\n", pin, rep.Target)
				terminalPrintln(stdout, "in .pyxis/config.yaml, then run `pyxis converge` to bring the tree with it")
			}
		}
	}
	return 0
}

// upgradeArrow renders the direction of travel, so a downgrade reads as one
// in EVERY mode — preview included, where hiding it would break the
// whole-would-do-set contract.
func upgradeArrow(rep upgrade.Report) string {
	if rep.Downgrade {
		return "↓ (downgrade)"
	}
	return "→"
}

// short renders a digest's leading bytes — enough to compare by eye, never
// the whole wall of hex.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12] + "…"
	}
	return sha
}

// printRollbackReport renders the write-journal rollback's account (plan
// init-hardening/001 T4, FR-10) after a stage failure past the point
// mutation began: what was restored, anything foreign left untouched and
// named, and — never run automatically (DINV-4) — the manual command to
// remove a repository this run created remotely.
func printRollbackReport(w io.Writer, rb installer.RollbackReport) {
	terminalPrintf(w, "pyxis: rolled back after the %q stage failed\n", rb.FailedStage)
	if rb.RepoRemoved {
		terminalPrintln(w, "  the repository init created this run was removed; the target is empty again")
	}
	if n := len(rb.RemovedFiles) + len(rb.RemovedDirs); n > 0 {
		terminalPrintf(w, "  restored %d file(s)/director(ies) to their pre-run state\n", n)
	}
	for _, p := range rb.ForeignPreserved {
		terminalPrintf(w, "  preserved (not init's): %s\n", p)
	}
	for _, cmd := range rb.RemoteCleanup {
		terminalPrintf(w, "  a repository was created remotely and was NOT deleted — remove it yourself: %s\n", cmd)
	}
	if !rb.Restored {
		terminalPrintln(w, "  the rollback itself hit an error — the target may not be fully restored:")
		for _, e := range rb.Errors {
			terminalPrintf(w, "    %s\n", e)
		}
	}
}

// firstNonEmpty returns the first trimmed non-empty string, or "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// installReleaseDefaults reads the system version + manifest sha256 that
// install.sh recorded at $XDG_CONFIG_HOME/pyxis/release.env (else
// ~/.config/pyxis/release.env) — the pin install.sh delivered, letting
// `pyxis init` run without pin flags. An absent
// file or key yields empties (init then refuses, naming every pin source).
//
// The same record's channel is not read here: artifact-source precedence
// belongs to one owner, and that is resolve.EffectiveSource, which every
// fetching path already goes through (prd:installer/FR-30).
func installReleaseDefaults() (version, manifestSHA string) {
	r := releaseenv.Read()
	return r.Version, r.ManifestSHA
}

// printInitReport renders the init result for humans: the admission verdict,
// the resolved knobs and identity, and either the performed commits/epoch
// (real run) or the full would-do set (preview). The machine-readable half is
// the EVT-1 install_completed slog event the engine emitted on success.
func printInitReport(w io.Writer, r installer.Report) {
	if r.Preview {
		terminalPrintf(w, "init preview (run %s) — a real run would:\n", r.RunID)
		if r.WouldGitInit {
			terminalPrintf(w, "  git init on branch %s\n", r.Branch)
		} else {
			terminalPrintf(w, "  adopt the empty repository on branch %s\n", r.Branch)
		}
		terminalPrintf(w, "  write %s and %s\n", r.ConfigPath, ".gitignore")
		printKnobs(w, r.Knobs)
		printIdentity(w, r.Identity)
		printChecks(w, r.Checks)
		for _, a := range r.Converge.WouldWrite {
			terminalPrintf(w, "  converge %s (%s)\n", a.Path, a.Class)
		}
		for _, seed := range r.WouldSeed {
			terminalPrintf(w, "  seed %s from %s (create once at floor; existing policy refuses)\n", seed.Path, seed.Source)
		}
		for _, f := range r.Converge.WouldFetch {
			terminalPrintf(w, "  fetch %s from %s\n", f.Artifact, f.URL)
		}
		for _, c := range r.Converge.Credentials {
			terminalPrintf(w, "  configure repository Git key %s\n", c.Key)
		}
		for _, inc := range r.Converge.Incomplete {
			terminalPrintf(w, "  incomplete: %s\n", inc)
		}
		printWiringOps(w, r.WiringOps)
		if p := r.Publication; p.WouldPush {
			terminalPrintf(w, "  first push: %s to %s (%s), after the green proof\n", p.Branch, p.Remote, p.Destination)
		}
		terminalPrintf(w, "  stages: %s\n", strings.Join(r.Stages, " → "))
		terminalPrintln(w, "nothing was performed")
		return
	}
	st := termstyle.Detect(w)
	if r.Resumed {
		terminalPrintf(w, "init: resumed installation publication on %s (run %s)\n", r.Branch, r.RunID)
		printInitTail(w, r)
		return
	}
	terminalPrintf(w, "%s\n", st.Bold(fmt.Sprintf("init: converged %s (run %s)", r.Branch, r.RunID)))
	if r.GitInited {
		terminalPrintf(w, "  git init on branch %s\n", r.Branch)
	}
	printKnobs(w, r.Knobs)
	printIdentity(w, r.Identity)
	terminalPrintf(w, "  config %s, .gitignore written\n", r.ConfigPath)
	terminalPrintf(w, "  converge: repaired %d, regenerated %d, pre-warmed %d\n",
		len(r.Converge.Repaired), len(r.Converge.Regenerated), len(r.Converge.PreWarmed))
	for _, c := range r.Converge.Credentials {
		terminalPrintf(w, "  configured repository Git key %s\n", c.Key)
	}
	for _, c := range r.Commits {
		terminalPrintf(w, "  commit %s — %s\n", shortSHA(c.SHA), c.Subject)
	}
	if r.EpochWritten {
		terminalPrintln(w, "  epoch written (commit → the converged tree)")
	}
	// The stages row belongs to the summary — it must precede the tail,
	// whose closing epilogue is the LAST thing on screen (#937).
	terminalPrintf(w, "  stages: %s\n", strings.Join(r.Stages, " → "))
	printInitTail(w, r)
}

// printInitTail renders the T8 tail's outcomes (installer-v1 §3.5 steps 4–7 +
// 9): seeding, hooks, wiring, the green proof, and the welcome.
// Ordered so the LAST thing on screen is the epilogue with the next actions
// (#937): summary lines, then any hand-off commands, then the welcome block —
// never internals after the guidance.
func printInitTail(w io.Writer, r installer.Report) {
	if n := len(r.T8.Seeded); n > 0 {
		terminalPrintf(w, "  seeded %d document(s)/registr(ies) (rollups correctly absent)\n", n)
	}
	if h := r.T8.Hooks; len(h.Installed) > 0 {
		terminalPrintf(w, "  hooks: %s active via core.hooksPath %s\n", strings.Join(h.Installed, ", "), h.HooksPath)
	}
	waiting := 0
	if r.T8.Wiring != nil {
		for _, op := range r.T8.Wiring.Operations {
			if op.Outcome == wire.OutcomeHandedOff {
				waiting++
			}
		}
		done := len(r.T8.Wiring.Operations) - waiting
		if waiting > 0 {
			terminalPrintf(w, "  wiring: %d of %d steps done — %d waiting on you (commands below)\n", done, len(r.T8.Wiring.Operations), waiting)
		} else {
			terminalPrintf(w, "  wiring: all %d steps done\n", len(r.T8.Wiring.Operations))
		}
	}
	if r.T8.GreenProof.Green {
		terminalPrintln(w, "  green proof: the gate exits green on the seeded tree ✓")
	}
	if p := r.Publication; p.Published {
		terminalPrintf(w, "  first push: %s published to %s (%s) at %s ✓\n", p.Branch, p.Remote, p.Destination, shortSHA(p.Head))
	}
	if r.T8.Welcome.Emitted && r.T8.Welcome.ItemID > 0 {
		terminalPrintf(w, "  welcome: item #%d created in the local store\n", r.T8.Welcome.ItemID)
	}
	if r.T8.Onboarding != nil {
		terminalPrintf(w, "  onboarding: %s\n", r.T8.Onboarding.Detail)
	}
	if r.T8.Wiring != nil && r.T8.Wiring.HandOff != "" {
		// The block carries its own trailing newline.
		terminalPrintf(w, "\n%s", r.T8.Wiring.HandOff)
	}
	if r.T8.Welcome.Emitted {
		terminalPrintf(w, "\n%s\n", welcomeEpilogue(termstyle.Detect(w), r, waiting))
	}
}

// welcomeEpilogue renders the final block on an init screen: a rule, the
// banner, the computed next steps, and — when wiring left steps waiting —
// one pointer up at the hand-off commands, so the run always ENDS on what
// to do next. Styling only wraps (#948): the words are byte-identical on a
// TTY and in a pipe.
func welcomeEpilogue(st termstyle.Style, r installer.Report, waiting int) string {
	var b strings.Builder
	b.WriteString(st.Dim(strings.Repeat("─", 62)) + "\n")
	b.WriteString(st.Bold(st.Green(installer.WelcomeBanner)))
	for _, s := range r.T8.Welcome.NextSteps {
		b.WriteString("\n  " + styleStepLine(st, s))
	}
	if waiting > 0 {
		b.WriteString("\n  " + st.Yellow(fmt.Sprintf("waiting on you: %d wiring step(s) — the commands are right above", waiting)))
	}
	return b.String()
}

// styleStepLine colors a "key: rest" epilogue row's key so the eye can jump
// the left column; the keyless send-off line is bolded whole.
func styleStepLine(st termstyle.Style, line string) string {
	key, rest, found := strings.Cut(line, ": ")
	if !found {
		return st.Bold(line)
	}
	return st.Cyan(key) + ": " + rest
}

func printKnobs(w io.Writer, k config.Knobs) {
	surfaces := "(none detected)"
	if len(k.CLISurfaces) > 0 {
		surfaces = strings.Join(k.CLISurfaces, ", ")
	}
	terminalPrintf(w, "  knobs: tracker=%s ci_mode=%s wiring={repo=%s protection=%s} cli_surfaces=[%s] argus=%t\n",
		k.Tracker, k.CIMode, k.Wiring.Repo, k.Wiring.Protection, surfaces, k.Argus)
}

func printIdentity(w io.Writer, id installer.Identity) {
	terminalPrintf(w, "  identity: %s <%s> (name: %s, email: %s)\n", id.Name, id.Email, id.NameSource, id.EmailSource)
}

// printChecks renders the preflight registry's full outcome set (design
// init-hardening-v1 §3.1; prd:installer/FR-24; plan init-hardening/001 T9):
// one line per evaluated check — pass, advise, or refuse — so a preview
// surfaces every problem in one pass (FR-16) rather than just the first
// that would stop a real run.
func printChecks(w io.Writer, checks preflight.Report) {
	for _, c := range checks {
		switch c.Status {
		case preflight.StatusPass:
			terminalPrintf(w, "  check %s: pass\n", c.Check)
		case preflight.StatusAdvise:
			terminalPrintf(w, "  check %s: advise — %s\n", c.Check, c.Warning)
		case preflight.StatusRefuse:
			terminalPrintf(w, "  check %s: refuse — %s", c.Check, c.Reason)
			if len(c.Conflicts) > 0 {
				terminalPrintf(w, " (conflicts: %s)", strings.Join(c.Conflicts, ", "))
			}
			if c.Remedy != "" {
				terminalPrintf(w, "; remedy: %s", c.Remedy)
			}
			if c.Route != "" {
				terminalPrintf(w, " — %s", c.Route)
			}
			terminalPrintln(w)
		}
	}
}

// printWiringOps renders preview's declared wiring operation table (design
// init-hardening-v1 §3.1/§3.7; prd:installer/FR-24; plan init-hardening/001
// T9) through wire's own Preview renderer (internal/wire, AP-11: one
// source) — the identical probe/ensure commands a real run's Wiring stage
// would walk, or a failure's hand-off block would print, shown here
// unexecuted.
func printWiringOps(w io.Writer, ops []wire.Op) {
	for _, entry := range wire.Preview(ops) {
		for line := range strings.SplitSeq(entry, "\n") {
			terminalPrintf(w, "  wiring: %s\n", line)
		}
	}
}

// shortSHA abbreviates a commit sha for the human summary; a short or empty
// sha passes through unchanged.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func versionVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "version", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "version", fmt.Sprintf("takes no arguments (got %q)", pos[0]))
	}
	terminalPrintf(stdout, "pyxis %s\n", version)
	return 0
}

// convergeVerb is `pyxis converge [--repair] [--preview]` and its damage
// name `pyxis repair [--preview]` — one converge (installer-v1 §3.4): the
// repair verb (and the --repair flag) only names the run's mode. Guard
// discipline unchanged (I3): the two flags parse fail-closed through
// cliguard before any root probe, config read, or engine call; exit 2
// usage/environment, 1 named refusals (config defects, acquisition
// refusals, per-file refusals, source mode), 0 converged.
func convergeVerb(verb string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	preview := fs.Bool("preview", false, "report every write and external operation; perform none (FR-19)")
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	repair := verb == "repair"
	if verb == "converge" {
		fs.BoolVar(&repair, "repair", false, "name the run a repair (one converge either way)")
	}
	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, verb, err)
	}
	if len(pos) != 0 {
		return refuse(stderr, verb, fmt.Sprintf("takes no arguments (got %q)", pos[0]))
	}

	root, err := repoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: %v\n", err)
		return 2
	}
	repo, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			terminalPrintln(stderr, d.String())
		}
		return 1
	}
	logRoot := root
	if *preview {
		logRoot = ""
	}
	logger, journalPath := runLogger(logRoot, stderr, verb, *verbose)
	rep, cerr := convergeRun(context.Background(), resolve.Config{Root: root, Repo: repo, Logger: logger},
		converge.Options{Preview: *preview, Repair: repair})
	if rep.Mode != "" {
		printConvergeReport(stdout, rep)
	}
	if cerr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", cerr)
		printJournalPointer(stderr, journalPath)
		return 1
	}
	printJournalPointer(stdout, journalPath)
	return 0
}

// printConvergeReport renders the report for humans: preview enumerates the
// exact would-do set (FR-19's surface), a real run summarizes what happened
// per class. The machine-readable half is the EVT-2 slog event the engine
// already emitted.
func printConvergeReport(w io.Writer, r converge.Report) {
	if r.Mode == converge.ModePreview {
		terminalPrintf(w, "converge preview (run %s) — a real run would:\n", r.RunID)
		n := 0
		for _, a := range r.WouldWrite {
			terminalPrintf(w, "  write %s (%s)\n", a.Path, a.Class)
			n++
		}
		for _, f := range r.WouldFetch {
			terminalPrintf(w, "  fetch %s from %s\n", f.Artifact, f.URL)
			n++
		}
		for _, c := range r.Credentials {
			terminalPrintf(w, "  configure repository Git key %s\n", c.Key)
			n++
		}
		for _, tool := range r.PreWarmed {
			terminalPrintf(w, "  pre-warm %s into the binary cache\n", tool)
			n++
		}
		for _, e := range r.Reaped {
			terminalPrintf(w, "  reap off-version cache entry %s\n", e)
			n++
		}
		for _, ref := range r.RefusedByName {
			terminalPrintf(w, "  refuse %s by name (%s): %s\n", ref.Path, ref.Class, ref.Reason)
			n++
		}
		if n == 0 && len(r.Incomplete) == 0 {
			terminalPrintln(w, "  nothing — the tree and cache are converged")
		}
		for _, inc := range r.Incomplete {
			terminalPrintf(w, "  incomplete: %s\n", inc)
		}
		terminalPrintln(w, "nothing was performed")
		return
	}
	terminalPrintf(w, "converge: mode %s — repaired %d, regenerated %d, refused %d, pre-warmed %d, reaped %d (run %s)\n",
		r.Mode, len(r.Repaired), len(r.Regenerated), len(r.RefusedByName), len(r.PreWarmed), len(r.Reaped), r.RunID)
	for _, a := range r.Repaired {
		terminalPrintf(w, "  repaired %s (%s)\n", a.Path, a.Class)
	}
	for _, a := range r.Regenerated {
		terminalPrintf(w, "  regenerated %s\n", a.Path)
	}
	for _, c := range r.Credentials {
		terminalPrintf(w, "  configured repository Git key %s\n", c.Key)
	}
	for _, ref := range r.RefusedByName {
		terminalPrintf(w, "  refused %s (%s): %s\n", ref.Path, ref.Class, ref.Reason)
	}
}

// adoptVerb is `pyxis adopt --system-version … --manifest-sha256 …
// [--artifact-source url]` — the negotiation's read (existing-repo-onboarding
// plan 001 T3) over internal/adopt.Inspect: dispatch by tree state, classify
// a non-empty repository by reusing converge's classification, and write a
// draft adoption-plan. Guard discipline unchanged (I3): flags parse
// fail-closed through cliguard before any probe; the pin resolves the same
// three-way way init's does (flag, then PYXIS_* env, then install.sh's
// release.env) so a machine set up by the bootstrap needs no pin flags here
// either. Exit: 0 success (routed to init, or a draft plan written), 1 named
// refusals (not a git repository, an invalid prior config, a missing pin, an
// engine refusal), 2 usage/environment.
func adoptVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("adopt", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	systemVersion := fs.String("system-version", "", "the system release pin (defaults to install.sh's record / PYXIS_SYSTEM_VERSION)")
	manifestSHA256 := fs.String("manifest-sha256", "", "the release manifest sha256, the trust root (defaults to install.sh's record / PYXIS_MANIFEST_SHA256)")
	artifactSource := fs.String("artifact-source", "", "FR-22 mirror/fork override for the artifact host")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "adopt", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "adopt", fmt.Sprintf("takes no arguments — the target is the working directory (got %q)", pos[0]))
	}

	target, err := os.Getwd()
	if err != nil {
		terminalPrintf(stderr, "pyxis: adopt: getwd: %v\n", err)
		return 2
	}

	fileVer, fileSHA := installReleaseDefaults()
	pin := installer.Pin{
		Version:        firstNonEmpty(*systemVersion, fileVer),
		ManifestSHA256: firstNonEmpty(*manifestSHA256, fileSHA),
	}

	logger, _ := runLogger(target, stderr, "adopt", *verbose)
	rep, aerr := adoptInspect(context.Background(), adopt.Options{
		Target:         target,
		Pin:            pin,
		ArtifactSource: *artifactSource,
		Logger:         logger,
	})
	if aerr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", aerr)
		return 1
	}
	printAdoptReport(stdout, rep)
	return 0
}

// printAdoptReport renders Inspect's result for humans: the empty-tree leg
// reuses init's own report printer verbatim (adopt performed a real init,
// not a preview of one — printInitReport's zero-value Preview branch is
// exactly right); the classify leg names the plan it wrote and renders it,
// the SAME reviewable face (Plan.Render) the plan file itself carries.
func printAdoptReport(w io.Writer, r adopt.Report) {
	if r.RoutedToInit {
		terminalPrintln(w, "adopt: the target is an empty repository — routed to init")
		printInitReport(w, r.Init)
		return
	}
	terminalPrintf(w, "adopt: wrote a draft plan to %s — review it, then run `pyxis apply`\n\n", r.PlanPath)
	terminalPrintf(w, "%s", r.Plan.Render())
}

// applyVerb is `pyxis apply` — materialise the approved adoption plan (plan
// existing-repo-onboarding/001 T8) over internal/adopt.Apply. It resolves the
// pin the same three-way way adopt/init do (flag, then PYXIS_* env, then
// install.sh's release.env), takes no positional arguments (the target is the
// working directory), and drives the whole apply: refuse an absent or stale
// plan, materialise the managed writes and collision resolutions, record the
// epoch without a commit, and stamp the plan applied. Exit: 0 applied, 1 named
// refusals (no plan, a stale plan, an invalid config, an engine refusal),
// 2 usage/environment.
func applyVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	systemVersion := fs.String("system-version", "", "the system release pin (defaults to install.sh's record / PYXIS_SYSTEM_VERSION)")
	manifestSHA256 := fs.String("manifest-sha256", "", "the release manifest sha256, the trust root (defaults to install.sh's record / PYXIS_MANIFEST_SHA256)")
	artifactSource := fs.String("artifact-source", "", "FR-22 mirror/fork override for the artifact host")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "apply", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "apply", fmt.Sprintf("takes no arguments — the target is the working directory (got %q)", pos[0]))
	}
	target, err := os.Getwd()
	if err != nil {
		terminalPrintf(stderr, "pyxis: apply: getwd: %v\n", err)
		return 2
	}
	fileVer, fileSHA := installReleaseDefaults()
	pin := installer.Pin{
		Version:        firstNonEmpty(*systemVersion, fileVer),
		ManifestSHA256: firstNonEmpty(*manifestSHA256, fileSHA),
	}
	logger, _ := runLogger(target, stderr, "apply", *verbose)
	rep, aerr := adoptApply(context.Background(), adopt.Options{
		Target:         target,
		Pin:            pin,
		ArtifactSource: *artifactSource,
		Logger:         logger,
	})
	if aerr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", aerr)
		return 1
	}
	printApplyReport(stdout, rep)
	return 0
}

// printApplyReport renders Apply's result for humans: the epoch it recorded,
// whether it chained a prior hook path, and every path it wrote or refused.
func printApplyReport(w io.Writer, r adopt.ApplyReport) {
	terminalPrintf(w, "apply: materialised the plan (%s) — epoch %s, no commit created\n", r.PlanPath, r.EpochCommit)
	if r.Chained {
		terminalPrintln(w, "  chained the engine's hooks ahead of the adopter's prior hook path")
	}
	for _, a := range r.Actions {
		terminalPrintf(w, "  + %s (%s)\n", a.Path, a.Class)
	}
	for _, ref := range r.Refusals {
		terminalPrintf(w, "  ! refused %s: %s\n", ref.Path, ref.Reason)
	}
	terminalPrintln(w, "\ncommit the adoption to record the epoch in history; run `pyxis undo` to reverse it.")
}

// undoVerb is `pyxis undo` — reverse an applied adoption plan (plan existing-
// repo-onboarding/001 T10) over internal/adopt.Undo, preserving post-adoption
// work. Pin resolution and the no-positional grammar match apply. Exit: 0
// reversed, 1 named refusals (no applied plan, an invalid config, an engine
// refusal), 2 usage/environment.
func undoVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("undo", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	systemVersion := fs.String("system-version", "", "the system release pin (defaults to install.sh's record / PYXIS_SYSTEM_VERSION)")
	manifestSHA256 := fs.String("manifest-sha256", "", "the release manifest sha256, the trust root (defaults to install.sh's record / PYXIS_MANIFEST_SHA256)")
	artifactSource := fs.String("artifact-source", "", "FR-22 mirror/fork override for the artifact host")

	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "undo", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "undo", fmt.Sprintf("takes no arguments — the target is the working directory (got %q)", pos[0]))
	}
	target, err := os.Getwd()
	if err != nil {
		terminalPrintf(stderr, "pyxis: undo: getwd: %v\n", err)
		return 2
	}
	fileVer, fileSHA := installReleaseDefaults()
	pin := installer.Pin{
		Version:        firstNonEmpty(*systemVersion, fileVer),
		ManifestSHA256: firstNonEmpty(*manifestSHA256, fileSHA),
	}
	logger, _ := runLogger(target, stderr, "undo", *verbose)
	rep, uerr := adoptUndo(context.Background(), adopt.Options{
		Target:         target,
		Pin:            pin,
		ArtifactSource: *artifactSource,
		Logger:         logger,
	})
	if uerr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", uerr)
		return 1
	}
	printUndoReport(stdout, rep)
	return 0
}

// printUndoReport renders Undo's result: the paths removed, the operator-edited
// paths preserved-and-named (never deleted), and the collide-manual entries the
// human must reverse by hand.
func printUndoReport(w io.Writer, r adopt.UndoReport) {
	terminalPrintf(w, "undo: reversed the adoption — removed %d path(s), preserved %d, %d manual\n", len(r.Removed), len(r.Preserved), len(r.ManualEntries))
	if r.Unchained {
		terminalPrintln(w, "  restored the adopter's prior hook path")
	}
	for _, p := range r.Removed {
		terminalPrintf(w, "  - %s\n", p)
	}
	for _, p := range r.Preserved {
		terminalPrintf(w, "  ~ preserved (modified since adoption, left in place): %s\n", p)
	}
	for _, e := range r.ManualEntries {
		terminalPrintf(w, "  ? reverse by hand (%s):\n%s\n", e.Path, indent(e.Detail, "      "))
	}
}

// indent prefixes every non-empty line of s with pre.
func indent(s, pre string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(pre + line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// doctorVerb is `pyxis doctor` — the shape's flagless read-only verb (plan
// installer/001 T6). Guard discipline unchanged (I3): the empty grammar
// parses fail-closed through cliguard before any root probe or config read.
// Exit semantics: 0 healthy, 1 findings or named refusals (config defects,
// a doctor error), 2 usage/environment. The report itself performs zero
// filesystem and zero remote mutations (converge.Doctor's contract).
func doctorVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	verbose := fs.Bool("verbose", false, "show the full log stream inline (it lands in the run journal either way)")
	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "doctor", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "doctor", fmt.Sprintf("takes no arguments (got %q)", pos[0]))
	}

	root, err := repoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: %v\n", err)
		return 2
	}
	repo, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			terminalPrintln(stderr, d.String())
		}
		return 1
	}
	logger, _ := runLogger(root, stderr, "doctor", *verbose)
	// Doctor renders integration load defects alongside the remaining checks.
	// A failed composition must not replace that partial health report.
	probe, _ := localCodeHostDiagnostics(root, repo)
	h, derr := doctorRun(context.Background(), resolve.Config{Root: root, Repo: repo, Logger: logger, CodeHostDiagnostics: probe})
	if derr != nil {
		terminalPrintf(stderr, "pyxis: %v\n", derr)
		return 1
	}
	printHealth(stdout, h)
	if !h.Healthy {
		n := 0
		for _, f := range h.Findings {
			if f.Severity == converge.SeverityFinding {
				n++
			}
		}
		terminalPrintf(stderr, "pyxis: doctor found %d finding(s) — see the report above; doctor performed no mutations (repair is `pyxis converge`)\n", n)
		return 1
	}
	return 0
}

// printHealth renders the Health report for humans: the mode/pin/epoch
// summary, then the rows grouped by check class with advisories marked.
// The machine-readable half is the EVT-3 doctor_run slog event the library
// already emitted.
func printHealth(w io.Writer, h converge.Health) {
	pin := "present"
	if !h.ManifestPinned {
		pin = "absent"
	}
	sys := h.SystemVersion
	if sys == "" {
		sys = "(unset)"
	}
	terminalPrintf(w, "doctor: mode %s — system %s, manifest pin %s (run %s)\n", h.Mode, sys, pin, h.RunID)
	if h.ArtifactSource != "" {
		terminalPrintf(w, "  artifact source override: %s\n", h.ArtifactSource)
	}
	if h.EpochPresent {
		terminalPrintf(w, "  epoch: system %s, commit %s, manifest %s\n", h.Epoch.SystemVersion, h.Epoch.Commit, h.Epoch.ManifestSHA256)
	} else {
		terminalPrintln(w, "  epoch: absent")
	}
	var class converge.CheckClass
	findings, advisories := 0, 0
	for _, f := range h.Findings {
		if f.Class != class {
			class = f.Class
			terminalPrintf(w, "%s:\n", class)
		}
		mark := "finding "
		if f.Severity == converge.SeverityAdvisory {
			mark = "advisory"
			advisories++
		} else {
			findings++
		}
		terminalPrintf(w, "  %s %s — %s\n", mark, f.Name, f.Detail)
	}
	if h.Healthy {
		terminalPrintf(w, "doctor: healthy — 0 findings, %d advisory(ies)\n", advisories)
		return
	}
	terminalPrintf(w, "doctor: UNHEALTHY — %d finding(s), %d advisory(ies)\n", findings, advisories)
}

// toolNameRe is a lax admission shape for the tool positional — kebab
// lowercase, the grammar every cmd/-scanned tool name fits. It exists to
// refuse obvious misparses early (a path, an option fragment, uppercase
// noise); the release manifest is the real authority on which tools exist
// (resolve refuses unknown tools by name).
var toolNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// execVerb is `pyxis exec <tool> -- <argv…>` (installer-v1 §3.4): the one
// grammar every installed-repo invocation speaks. Order is the I3
// discipline — grammar refusals first (exit 2, zero action), then root +
// config (environment 2 / defects 1), and only then the acquisition core;
// resolution refusals are the typed errors printed as named refusals
// (exit 1, zero execution — DINV-1).
func execVerb(args []string, stderr io.Writer) int {
	own, argv, found := cliguard.CutArgv(args)
	if !found {
		return refuse(stderr, "exec", "missing the `--` separator (everything after -- passes argv-verbatim to the tool)")
	}
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	pos, err := cliguard.Positionals(fs, own)
	if err != nil {
		return misparse(stderr, "exec", err)
	}
	if len(pos) != 1 {
		return refuse(stderr, "exec", fmt.Sprintf("takes exactly one tool name before -- (got %d)", len(pos)))
	}
	tool := pos[0]
	if !toolNameRe.MatchString(tool) {
		return refuse(stderr, "exec", fmt.Sprintf("tool name %q does not match [a-z0-9-]+", tool))
	}

	root, err := repoRoot()
	if err != nil {
		terminalPrintf(stderr, "pyxis: %v\n", err)
		return 2
	}
	repo, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			terminalPrintln(stderr, d.String())
		}
		return 1
	}
	logger, _ := runLogger(root, stderr, "exec-"+tool, true)
	logger.Info("tool_invocation_started", "tool", tool)
	rcfg := resolve.Config{Root: root, Repo: repo, Logger: logger}

	path, rerr := resolveTool(context.Background(), rcfg, tool, "")
	if rerr != nil {
		var sm *resolve.SourceMode
		if errors.As(rerr, &sm) {
			logger.Info("tool_delegated", "tool", tool, "mode", "source", "argument_count", len(argv))
			code, gerr := goRunFn(sm, argv)
			if gerr != nil {
				logger.Warn("tool_execution_failed", "tool", tool, "mode", "source", "error", gerr)
				terminalPrintf(stderr, "pyxis: exec %s: %v\n", tool, gerr)
				return 1
			}
			logger.Info("tool_finished", "tool", tool, "mode", "source", "exit_code", code)
			return code
		}
		// A typed refusal from the acquisition core (tamper, offline,
		// missing pin, wrong platform, absent source tree) — named, with
		// zero execution (DINV-1).
		logger.Warn("tool_resolution_failed", "tool", tool, "error", rerr)
		terminalPrintf(stderr, "pyxis: %v\n", rerr)
		return 1
	}
	// Persist the handoff before execve replaces this process. Successful Unix
	// exec has no return event; the delegated tool retains its own exit status.
	logger.Info("tool_delegated", "tool", tool, "mode", "binary", "argument_count", len(argv))
	code, xerr := execFn(path, append([]string{path}, argv...), os.Environ())
	if xerr != nil {
		logger.Warn("tool_execution_failed", "tool", tool, "mode", "binary", "error", xerr)
		terminalPrintf(stderr, "pyxis: exec %s: %v\n", tool, xerr)
		return 1
	}
	logger.Info("tool_finished", "tool", tool, "mode", "binary", "exit_code", code)
	return code // reached only where execve is unavailable (the child-relay fallback) or under a test seam
}

// repoRoot locates the operand repo root via the one shared discovery
// predicate (internal/reporoot): the nearest ancestor carrying
// .pyxis/config.yaml, else the git working-tree root. The go.mod walk the
// sibling mains once used would find nothing here — an installed operand
// ships no Go source — so every pyxis binary shares this one predicate
// (AP-11): the orchestrator and the tools it execs agree on the operand.
func repoRoot() (string, error) {
	return reporoot.Find()
}

// goRun is the source-mode leg (DINV-8): run ./cmd/<tool> from source with
// the working directory at the repo root — the retiring shell resolvers'
// contract (`cd "$ROOT" && exec go run ./cmd/<tool> "$@"`). It is `go run`'s
// two halves done explicitly — `go build` into a scratch binary (same build
// cache, same cost), then run it — because `go run` itself flattens every
// nonzero child exit to 1 (documented: "the exit status of Run is not the
// exit status of the compiled binary"), and the tool's exit code IS the
// caller's verdict: the 0/1/2 convention is contractual, and the parity
// suite (NR-2/A-6) diffs verdicts between source and installed repos. The
// child inherits this process's std streams; its exact exit code is relayed
// (a signal-killed child relays 1); argv passes verbatim, untouched
// (DINV-5). SIGINT/SIGTERM are relayed to the child (runRelaying) so killing
// this launcher drives the tool's own graceful shutdown rather than orphaning
// it: installed mode gets that for free by exec-replacing the tool, but source
// mode runs it as a child (to relay the true exit code, above) and so must relay
// the signal itself — the `go run` launch gotcha.
func goRun(sm *resolve.SourceMode, argv []string) (int, error) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return 0, fmt.Errorf("source mode needs the Go toolchain to run ./cmd/%s (DINV-8): %w", sm.Tool, err)
	}
	scratch, err := os.MkdirTemp("", "pyxis-gorun-")
	if err != nil {
		return 0, fmt.Errorf("source mode scratch dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(scratch) }() // best-effort: scratch under TMPDIR, never trusted or reused
	bin := filepath.Join(scratch, sm.Tool)
	build := exec.Command(goBin, "build", "-o", bin, "./cmd/"+sm.Tool)
	build.Dir = sm.RepoRoot
	if out, berr := build.CombinedOutput(); berr != nil {
		return 0, fmt.Errorf("build ./cmd/%s from source: %v\n%s", sm.Tool, berr, strings.TrimSpace(string(out)))
	}
	cmd := exec.Command(bin, argv...)
	cmd.Dir = sm.RepoRoot
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	code, rerr := runRelaying(cmd, sigCh)
	if rerr != nil {
		return 0, fmt.Errorf("run ./cmd/%s from source: %w", sm.Tool, rerr)
	}
	return code, nil
}

// runRelaying starts cmd, relays every signal received on sigs to the child
// while it runs, and returns the child's exit code — a signal-killed child (no
// code) relays 1, matching goRun's contract. The signal relay is the source-mode
// fix for the `go run` launch gotcha: a bare child does not receive a signal
// sent to its launcher directly (only one shared through a terminal's foreground
// group), so killing `pyxis exec` would otherwise leave the tool running
// headless. Relaying drives the tool's OWN SIGINT/SIGTERM handler — its graceful
// shutdown, which tears down its workers — instead. sigs is the launcher's real
// signal stream in production; a test injects synthetic signals through it. cmd
// must not have been started.
func runRelaying(cmd *exec.Cmd, sigs <-chan os.Signal) (int, error) {
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	relayDone := make(chan struct{})
	defer close(relayDone)
	go func() {
		for {
			select {
			case s := <-sigs:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(s) // best-effort: the child may have already exited
				}
			case <-relayDone:
				return
			}
		}
	}()
	err := cmd.Wait()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if code := ee.ExitCode(); code >= 0 {
			return code, nil
		}
		return 1, nil // killed by a signal: no exit code to relay — nonzero, honestly
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}
