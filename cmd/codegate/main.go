package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/baseline"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/ladder"
	"github.com/agenticlab-ai/pyxis/internal/verify/runner"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
	"github.com/agenticlab-ai/pyxis/internal/verify/seed"
	"github.com/agenticlab-ai/pyxis/internal/verify/worktree"
)

func main() {
	os.Exit(runMain(os.Args[1:]))
}

// runMain owns command dispatch separately from process termination so the
// CLI contract can be exercised by unit tests without exiting the test binary.
func runMain(args []string) int {
	// The hidden confinement re-exec mode (runner.ConfineExecArg) is checked
	// BEFORE any normal argument parsing: it is not a documented subcommand
	// (deliberately absent from usage()) — only runner.Confine itself ever
	// constructs an invocation shaped like this, re-exec'ing codegate to
	// apply Linux write-scope confinement to a throwaway child process
	// (design codegate-v1 §3.5). RunConfinedExec never returns on success
	// (it execve's into the real verb) and never falls through to normal
	// dispatch on failure either (it exits directly) — the guard below is
	// purely so this is checked ahead of the `len(os.Args) < 2` usage path.
	if len(args) >= 1 && args[0] == runner.ConfineExecArg {
		runner.RunConfinedExec(args[1:])
		return invoke.ExitPass
	}

	if len(args) < 1 {
		usage()
		return invoke.ExitConfigErr
	}

	switch args[0] {
	case "run":
		return runCmd(args[1:])
	case "capture-baseline":
		return captureBaselineCmd(args[1:])
	case "seed":
		return seedCmd(args[1:])
	case "--version", "-v", "version":
		fmt.Printf("codegate %s\n", invoke.CodegateVersion)
		return invoke.ExitPass
	case "--help", "-h", "help":
		usage()
		return invoke.ExitPass
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		return invoke.ExitConfigErr
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `codegate %s - the verification harness

usage:
  codegate run --workspace <path> --diff-base <ref> [--incremental]
  codegate capture-baseline --workspace <path>
  codegate seed [--workspace <path>]
  codegate --help
  codegate --version
`, invoke.CodegateVersion)
}

// runCmd runs the `codegate run` invocation contract and returns its process
// exit code. It RETURNS (rather than calling os.Exit) so deferred cleanup — the
// workspace-lock release — actually runs.
func runCmd(args []string) int {
	inv, cerr := invoke.ParseInvocation(args)
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}

	// Validate the workspace BEFORE acquiring the lock / creating scratch, so a
	// typo'd --workspace can never fabricate a .codegate directory or lock file.
	if cerr := invoke.ValidateWorkspacePath(inv.Workspace); cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}

	lock, cerr := invoke.AcquireWorkspaceLock(inv.Workspace)
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}
	defer lock.Release()

	// Residue reaping (AP-12) runs ONLY once exclusive ownership is
	// confirmed — the same moment the design's janitor is scoped to — and
	// before anything else, so a prior invocation's leftovers are never
	// mistaken for this run's own state. The two janitors are asymmetric:
	// a stale worktree registration is shared .git metadata a git-lock
	// collision could plausibly leave inconsistent for THIS run's own later
	// plumbing (assertCleanTree, resolveDiffBase), so its failure fails
	// closed (T9: "shared-.git worktree operations ... fail-closed"); old
	// run-scratch is pure filesystem housekeeping with no bearing on this
	// run's correctness, so its failure is logged and does not block
	// verification.
	if err := invoke.PruneRunScratch(inv.Workspace); err != nil {
		fmt.Fprintf(os.Stderr, "codegate: warning: failed to prune old run scratch: %v\n", err)
	}
	if err := worktree.PruneStaleWorktrees(context.Background(), inv.Workspace); err != nil {
		cerr := &invoke.ConfigError{
			Message:    fmt.Sprintf("cannot prune stale worktree registrations: %v", err),
			Workspace:  inv.Workspace,
			StdoutOnly: true,
			Code:       invoke.ExitRungErr,
		}
		cerr.Emit()
		return cerr.ExitCode()
	}

	// Bindings resolution is git plumbing (fast); it gets its own short bound
	// distinct from the ladder's — the ladder must NOT inherit this deadline,
	// since every verb already bounds itself (DINV-5) and a blanket top-level
	// timeout would misclassify a legitimately slow multi-rung run as a
	// cancellation fault.
	bindCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	bindings, cerr := invoke.ResolveBindings(bindCtx, inv.Workspace, inv.DiffBase)
	cancel()
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}

	manifest, _, cerr := invoke.LoadManifest(inv.Workspace)
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}

	var verdict *schema.Verdict
	if inv.Incremental {
		// A missing or unparseable prior verdict silently degrades to a full
		// run (design codegate-v1 §3.8, DINV-8): LoadVerdict's error is
		// intentionally discarded here rather than surfaced — a nil prior is
		// RunIncremental's own cue to fall back, never a new refusal class.
		prior, _ := schema.LoadVerdict(filepath.Join(inv.Workspace, invoke.VerdictRelPath))
		verdict = ladder.RunIncremental(context.Background(), inv.Workspace, manifest, *bindings, prior)
		if verdict.Mode != "incremental" {
			fmt.Fprintln(os.Stderr, "codegate: --incremental requested but the prior verdict does not qualify as state (design codegate-v1 DINV-8); ran the full ladder")
		}
	} else {
		verdict = ladder.Run(context.Background(), inv.Workspace, manifest, *bindings, "full")
	}

	if err := writeVerdict(inv.Workspace, verdict); err != nil {
		fmt.Fprintf(os.Stderr, "codegate: failed to write verdict: %v\n", err)
		return invoke.ExitRungErr
	}
	printSummary(verdict)

	return exitCodeForState(verdict.State)
}

// exitCodeForState maps a completed verdict's overall state to the process
// exit code the invocation contract promises (§3.6): pass-class states exit
// 0, a genuine check failure exits 1, and anything else — an execution-layer
// fault, or an unrecognized future state — exits 3 (fail closed) so a broken
// harness can never be read as either "the code is fine" or "the code is
// broken."
func exitCodeForState(state string) int {
	switch state {
	case "green", "green-degraded":
		return invoke.ExitPass
	case "red":
		return invoke.ExitRed
	default:
		return invoke.ExitRungErr
	}
}

// writeVerdict writes the run's verdict atomically to the harness's fixed
// scratch location (§3.6) — the same path a configuration-error verdict uses.
func writeVerdict(workspace string, v *schema.Verdict) error {
	scratchDir := filepath.Join(workspace, ".codegate")
	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return fmt.Errorf("create scratch directory: %w", err)
	}
	return v.WriteAtomicToFile(filepath.Join(scratchDir, "verdict.json"))
}

// maxSummaryFindings bounds stdout's findings listing: the artifact is the
// truth (§3.6) and hosts must read it; stdout carries a bounded summary only.
const maxSummaryFindings = 20

func printSummary(v *schema.Verdict) {
	fmt.Printf("verdict: %s\n", v.State)
	fmt.Printf("artifact: %s\n", filepath.Join(v.Workspace, ".codegate", "verdict.json"))
	if len(v.Findings) == 0 {
		return
	}

	fmt.Println("findings:")
	shown, truncated := v.Findings, 0
	if len(shown) > maxSummaryFindings {
		truncated = len(shown) - maxSummaryFindings
		shown = shown[:maxSummaryFindings]
	}
	for _, f := range shown {
		loc := f.Path
		if loc != "" && f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.Path, f.Line)
		}
		if loc != "" {
			loc += ": "
		}
		fmt.Printf("  [rung %s: %s] %s%s\n", f.Rung, f.CheckID, loc, f.Message)
	}
	if truncated > 0 {
		fmt.Printf("  ... and %d more (see the artifact)\n", truncated)
	}
}

// captureBaselineCmd implements `codegate capture-baseline --workspace <path>`:
// it runs the bound test verb once, reduces its output to the current set of
// failure identities, and writes them as the manifest-declared baseline (the
// no-new-failures ratchet rung 2 evaluates against). This is the ONLY codegate
// path that writes a baseline — `run` never does (DINV-6). It refuses (exit 2)
// when there is nothing it can capture: no declared baseline_path, a stack with
// no failure-identity parser, or no bound test verb.
func captureBaselineCmd(args []string) int {
	workspace, cerr := parseCaptureArgs(args)
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}
	if cerr := invoke.ValidateWorkspacePath(workspace); cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}
	manifest, _, cerr := invoke.LoadManifest(workspace)
	if cerr != nil {
		cerr.Emit()
		return cerr.ExitCode()
	}

	baselinePath := strings.TrimSpace(manifest.BaselinePath)
	if baselinePath == "" {
		return captureFail("the manifest declares no baseline_path — declare one in .pyxis/codegate.json before capturing", invoke.ExitConfigErr)
	}
	eco := ecosystem.New()
	if !eco.HasFailureIdentityParser(manifest.Stack) {
		return captureFail(fmt.Sprintf("stack %q has no failure-identity parser — its test output cannot be reduced to a baseline", manifest.Stack), invoke.ExitConfigErr)
	}
	verb, ok := eco.ResolveVerbs(manifest.Stack, manifest.Verbs)["test"]
	if !ok || verb == nil {
		return captureFail("no \"test\" verb is bound for this stack — capture-baseline needs one to run", invoke.ExitConfigErr)
	}

	res := runner.Exec(context.Background(), workspace, verb)
	if res.Class != "" {
		return captureFail(fmt.Sprintf("the test verb could not be run: %v", res.Err), invoke.ExitRungErr)
	}
	failures, err := eco.ParseFailureIdentities(manifest.Stack, res.Stdout)
	if err != nil {
		return captureFail(fmt.Sprintf("cannot capture a baseline from unparseable test output: %v", err), invoke.ExitRungErr)
	}

	identities := make([]string, 0, len(failures))
	for _, f := range failures {
		identities = append(identities, f.Identity)
	}
	sum := sha256.Sum256([]byte(res.Stdout))
	b := baseline.Capture(identities, invoke.CodegateVersion, "codegate capture-baseline", hex.EncodeToString(sum[:]), time.Now())
	if err := b.WriteAtomicToFile(filepath.Join(workspace, filepath.FromSlash(baselinePath))); err != nil {
		return captureFail(fmt.Sprintf("write baseline %q: %v", baselinePath, err), invoke.ExitRungErr)
	}

	fmt.Printf("codegate: captured %d failing test(s) into %s\n", len(identities), baselinePath)
	return invoke.ExitPass
}

// parseCaptureArgs reads capture-baseline's only flag, --workspace, rejecting a
// missing value, a repeat, or any unexpected argument (closed parse, like the
// run invocation's).
func parseCaptureArgs(args []string) (string, *invoke.ConfigError) {
	workspace := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace":
			if i+1 >= len(args) {
				return "", &invoke.ConfigError{Message: "flag --workspace requires a value", Code: invoke.ExitConfigErr, StdoutOnly: true}
			}
			if workspace != "" {
				return "", &invoke.ConfigError{Message: "flag --workspace specified multiple times", Code: invoke.ExitConfigErr, StdoutOnly: true}
			}
			workspace = args[i+1]
			i++
		default:
			return "", &invoke.ConfigError{Message: fmt.Sprintf("unexpected argument %q", args[i]), Code: invoke.ExitConfigErr, StdoutOnly: true}
		}
	}
	if workspace == "" {
		return "", &invoke.ConfigError{Message: "required flag --workspace not provided", Code: invoke.ExitConfigErr, StdoutOnly: true}
	}
	return workspace, nil
}

// captureFail reports a capture-baseline failure on stderr and returns code.
func captureFail(message string, code int) int {
	fmt.Fprintf(os.Stderr, "codegate capture-baseline: %s\n", message)
	return code
}

// seedCmd runs `codegate seed` — FR-20's one-shot manifest authoring aid
// (prd:implementation-verification amendment 001). The workspace defaults
// to the current directory because both truth surfaces (the welcome item,
// doctor's code-without-manifest advisory) route users here as a bare
// `pyxis exec codegate -- seed`. Unlike run/capture-baseline it emits no
// verdict artifact: it verifies nothing, and its audience is a human, so
// every refusal is a plain named line on stderr (UB-10 / UB-11).
func seedCmd(args []string) int {
	workspace := "."
	workspaceSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workspace":
			if workspaceSet {
				return seedFail("--workspace specified multiple times", invoke.ExitConfigErr)
			}
			if i+1 >= len(args) {
				return seedFail("--workspace requires a value", invoke.ExitConfigErr)
			}
			i++
			if args[i] == "" {
				return seedFail("--workspace requires a non-empty value", invoke.ExitConfigErr)
			}
			workspace = args[i]
			workspaceSet = true
		default:
			return seedFail(fmt.Sprintf("unknown argument %q (usage: codegate seed [--workspace <path>])", args[i]), invoke.ExitConfigErr)
		}
	}
	if cerr := invoke.ValidateWorkspacePath(workspace); cerr != nil {
		// The human surface, not run's verdict protocol — same defect text.
		return seedFail(cerr.Message, cerr.ExitCode())
	}

	res, ref := seed.Seed(workspace, ecosystem.New())
	if ref != nil {
		return seedFail(ref.Message, ref.Code)
	}
	if res.Placeholder {
		fmt.Printf("codegate: wrote %s — stack %s; this stack ships no default commands, so each verb is a named placeholder: edit every command, the manifest is yours to own\n", res.Path, res.Stack)
	} else {
		fmt.Printf("codegate: wrote %s — stack %s, verb commands filled from the ecosystem defaults; edit freely, the manifest is yours to own\n", res.Path, res.Stack)
	}
	// The manifest is itself rung-0 protected machinery (FR-8), so the very
	// diff that introduces it reds the gate by design — without this note
	// the natural smoke test (seed → run) reads as a seed defect.
	fmt.Println("codegate: note — this manifest is protected machinery: the diff introducing it lands through the human lane, and `codegate run` passes once the diff base carries it")
	return invoke.ExitPass
}

// seedFail reports a seed refusal or failure on stderr and returns code.
func seedFail(message string, code int) int {
	fmt.Fprintf(os.Stderr, "codegate seed: %s\n", message)
	return code
}
