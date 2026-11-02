// Package invoke implements the `codegate run` invocation contract: parse and
// fail-closed-refuse malformed invocations, resolve the six revision bindings,
// and hold a per-workspace lock so exactly one invocation runs at a time.
package invoke

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/runlogs"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// Exit classes per the design (§3.6): 0 pass, 1 red, 2 configuration error,
// 3 infra/rung error (spawn failure, timeout, environment).
const (
	ExitPass      = 0
	ExitRed       = 1 // reserved for T3: a red verification
	ExitConfigErr = 2
	ExitRungErr   = 3
)

// CodegateVersion is the single source of truth for the version stamped into
// every verdict's bindings; cmd/codegate reads it for `--version` too.
const CodegateVersion = "1.0.0-dev"

// ManifestRelPath is the manifest's fixed, workspace-relative location
// (design codegate-v1 §3.10). It is also one of machinery protection's
// derived paths (§3.7) — checks.MachineryPaths reads this constant rather
// than a second, hand-kept copy of the literal.
const ManifestRelPath = ".pyxis/codegate.json"

// ScratchDirRelPath is codegate's fixed, workspace-relative scratch location
// (design §3.6). The verdict artifact and the workspace lock both live under
// it; assertCleanTree exempts this directory (and only this directory) from
// the dirty-tree refusal.
const ScratchDirRelPath = ".codegate"

// VerdictFileName is the verdict artifact's file name within the scratch
// directory.
const VerdictFileName = "verdict.json"

// VerdictRelPath is the verdict artifact's fixed, workspace-relative path —
// the composition git-diff path matching needs (forward-slash, matching
// git's own path convention, never filepath.Join's OS-dependent separator).
// Design §3.6: "the verdict path also joins the machinery-protection set as
// belt-and-braces" for DINV-2 (never committing the verdict that certifies
// the tree it would be committed into) — checks.MachineryPaths reads this
// constant rather than a second, hand-kept copy of the literal. Only this
// concrete file is protected, not the whole scratch directory: the rest of
// scratch (lock file, cache pins) carries no DINV-2 self-certification risk,
// and is kept out of version control by exclusion (§3.6), not by this check.
const VerdictRelPath = ScratchDirRelPath + "/" + VerdictFileName

// RunsRelPath is shared with the ladder so pruning preserves the log directory
// referenced by the current verdict after execution logs move out of scratch.
const RunsRelPath = runlogs.Codegate

// gitFloorVersion is the minimum git version required (worktree, rename
// detection, apply --reverse). Compared by real numeric parse, never lexically.
const gitFloorVersion = "2.5.0"

// nowFunc is the injectable clock (tests override it for deterministic verdicts).
var nowFunc = time.Now

// errLockBusy is the sentinel a platform lock returns when the workspace is
// already held by another invocation; classified with errors.Is, never by text.
var errLockBusy = errors.New("workspace lock is held by another invocation")

// Invocation is a parsed `codegate run` invocation.
type Invocation struct {
	Workspace   string
	DiffBase    string
	Incremental bool
}

// ConfigError is a refusal verdict. Code is the process exit code (defaults to
// ExitConfigErr); infra/operational failures set ExitRungErr so a caller can
// tell a retryable environment/contention failure from a real config defect.
type ConfigError struct {
	Message    string          // the defect description
	Workspace  string          // empty when the workspace flag itself is the defect
	StdoutOnly bool            // emit to stdout only (no artifact)
	Code       int             // exit code; 0 means ExitConfigErr
	Artifact   *schema.Verdict // optional pre-built verdict
}

// ExitCode returns the process exit code for this refusal.
func (ce *ConfigError) ExitCode() int {
	if ce.Code != 0 {
		return ce.Code
	}
	return ExitConfigErr
}

// WorkspaceLock holds an flock on a workspace's lock file for the run's
// duration; the kernel releases it on process death, so a crash never wedges.
type WorkspaceLock struct {
	file *os.File
}

// ParseInvocation parses `codegate run` flags. It returns a *ConfigError on any
// invalid, missing, ambiguous, or flag-like input. It is pure (no IO).
func ParseInvocation(args []string) (*Invocation, *ConfigError) {
	inv := &Invocation{}
	var workspaceFlag, diffBaseFlag string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if !strings.HasPrefix(arg, "--") {
			return nil, parseErr("flag-like positional argument: %s", arg)
		}
		if arg == "--" {
			if i+1 < len(args) {
				return nil, parseErr("flag-like positional argument: %s", args[i+1])
			}
			break
		}
		if arg == "--incremental" {
			inv.Incremental = true
			continue
		}
		if strings.HasPrefix(arg, "--incremental=") {
			return nil, parseErr("flag --incremental takes no value")
		}

		flagName := arg
		var flagValue string
		if eq := strings.IndexByte(arg, '='); eq >= 0 {
			flagName, flagValue = arg[:eq], arg[eq+1:]
		} else {
			if i+1 >= len(args) {
				return nil, parseErr("flag %s requires a value", flagName)
			}
			i++
			flagValue = args[i]
		}

		// A value that looks like a flag is rejected in BOTH forms (space-
		// separated and --flag=value). Without this the `=` form let an
		// attacker-controlled ref inject git options (e.g. --diff-base=--all).
		if strings.HasPrefix(flagValue, "-") {
			return nil, parseErr("flag %s: value looks like a flag (%s)", flagName, flagValue)
		}
		if flagValue == "" {
			return nil, parseErr("flag %s: empty value", flagName)
		}

		switch flagName {
		case "--workspace":
			if workspaceFlag != "" {
				return nil, parseErr("flag --workspace specified multiple times")
			}
			workspaceFlag = flagValue
		case "--diff-base":
			if diffBaseFlag != "" {
				return nil, parseErr("flag --diff-base specified multiple times")
			}
			diffBaseFlag = flagValue
		default:
			return nil, parseErr("unknown flag: %s", flagName)
		}
	}

	if workspaceFlag == "" {
		return nil, parseErr("required flag --workspace not provided")
	}
	if diffBaseFlag == "" {
		return nil, parseErr("required flag --diff-base not provided")
	}

	inv.Workspace = workspaceFlag
	inv.DiffBase = diffBaseFlag
	return inv, nil
}

// parseErr builds a stdout-only configuration error (a parse defect has no
// resolved workspace to write an artifact into).
func parseErr(format string, a ...any) *ConfigError {
	return &ConfigError{Message: fmt.Sprintf(format, a...), StdoutOnly: true, Code: ExitConfigErr}
}

// ValidateWorkspacePath cheaply verifies the workspace is an existing git
// working tree BEFORE any scratch/lock is created, so a typo'd --workspace can
// never fabricate a .codegate directory or lock in an arbitrary location.
func ValidateWorkspacePath(workspace string) *ConfigError {
	if _, err := os.Stat(filepath.Join(workspace, ".git")); err != nil {
		return &ConfigError{
			Message:    fmt.Sprintf("workspace is not a git repository: %v", err),
			StdoutOnly: true, // the workspace flag itself is the defect
			Code:       ExitConfigErr,
		}
	}
	return nil
}

// ManifestPath is the workspace-relative location of the verification
// manifest — the one place this convention is spelled out; every reader
// (bindings resolution here, the ladder's caller) goes through LoadManifest
// rather than re-deriving the path.
const ManifestPath = ".pyxis/codegate.json"

// LoadManifest reads and parses the workspace's manifest, returning both the
// parsed value and its raw bytes. ResolveBindings hashes the bytes for
// DINV-1; a caller that goes on to run the ladder needs the parsed value.
// This is the one place a workspace's manifest is read from disk, so bindings
// resolution and ladder execution can never observe two different manifest
// contents from two independent reads of a workspace that (by the invocation
// contract's clean-tree precondition) cannot change underneath a single run.
func LoadManifest(workspace string) (*schema.Manifest, []byte, *ConfigError) {
	manifestData, err := os.ReadFile(filepath.Join(workspace, ManifestPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, configErr(workspace, "manifest not found: %s", ManifestPath)
		}
		return nil, nil, infraErr(workspace, "cannot read manifest: %v", err)
	}
	manifest, err := schema.UnmarshalManifest(manifestData)
	if err != nil {
		return nil, nil, configErr(workspace, "manifest is malformed: %v", err)
	}
	return manifest, manifestData, nil
}

// ResolveBindings resolves the six bindings for a clean workspace, resolving the
// diff base to a SHA. It returns a ConfigError (config vs infra classified via
// Code) on any defect. Bindings are the frozen schema.Bindings so DINV-1's
// BindingsPresent can validate the live set downstream.
func ResolveBindings(ctx context.Context, workspace, diffBaseRef string) (*schema.Bindings, *ConfigError) {
	bindings := &schema.Bindings{CodegateVersion: CodegateVersion}

	if cerr := ValidateWorkspacePath(workspace); cerr != nil {
		cerr.StdoutOnly = false
		cerr.Workspace = workspace
		return nil, cerr
	}

	gitVer, err := getGitVersion(ctx)
	if err != nil {
		return nil, infraErr(workspace, "cannot determine git version: %v", err)
	}
	bindings.GitVersion = gitVer
	if !isGitVersionAtLeast(gitVer, gitFloorVersion) {
		return nil, infraErr(workspace, "git version %s is below the required minimum %s", gitVer, gitFloorVersion)
	}

	manifest, manifestData, cerr := LoadManifest(workspace)
	if cerr != nil {
		return nil, cerr
	}
	bindings.ManifestHash = hashData(manifestData)

	// Baseline binding: honor a declared baseline (refuse if missing/malformed),
	// else record the no-baseline hash. The declared-baseline refusal is a named
	// class the doc promises; it is not faked to a constant.
	baselineHash, cerr := resolveBaselineHash(workspace, manifest)
	if cerr != nil {
		return nil, cerr
	}
	bindings.BaselineHash = baselineHash

	if err := assertCleanTree(ctx, workspace); err != nil {
		if errors.Is(err, errDirtyTree) {
			return nil, configErr(workspace, "workspace is not clean: %v", err)
		}
		return nil, infraErr(workspace, "cannot check workspace tree: %v", err)
	}

	headSHA, err := getHeadSHA(ctx, workspace)
	if err != nil {
		return nil, infraErr(workspace, "cannot determine HEAD: %v", err)
	}
	bindings.HeadSHA = headSHA

	diffBaseSHA, err := resolveDiffBase(ctx, workspace, diffBaseRef)
	if err != nil {
		return nil, configErr(workspace, "cannot resolve diff base %q: %v", diffBaseRef, err)
	}
	bindings.DiffBase = diffBaseSHA

	return bindings, nil
}

// resolveBaselineHash reads and hashes the manifest-declared baseline, refusing
// if it is declared but missing or not valid JSON. An undeclared baseline hashes
// the empty content (documented no-baseline value).
func resolveBaselineHash(workspace string, manifest *schema.Manifest) (string, *ConfigError) {
	bp := strings.TrimSpace(manifest.BaselinePath)
	if bp == "" {
		return hashData(nil), nil
	}
	data, err := os.ReadFile(filepath.Join(workspace, bp))
	if err != nil {
		return "", configErr(workspace, "declared baseline %q is missing or unreadable: %v", bp, err)
	}
	if !json.Valid(data) {
		return "", configErr(workspace, "declared baseline %q is not valid JSON", bp)
	}
	return hashData(data), nil
}

func configErr(workspace, format string, a ...any) *ConfigError {
	return &ConfigError{Message: fmt.Sprintf(format, a...), Workspace: workspace, Code: ExitConfigErr}
}

func infraErr(workspace, format string, a ...any) *ConfigError {
	return &ConfigError{Message: fmt.Sprintf(format, a...), Workspace: workspace, Code: ExitRungErr}
}

// AcquireWorkspaceLock takes the per-workspace flock (non-blocking). A held lock
// refuses busy (retryable, ExitRungErr). The caller must have validated the
// workspace first (ValidateWorkspacePath) so scratch is never created elsewhere.
func AcquireWorkspaceLock(workspace string) (*WorkspaceLock, *ConfigError) {
	scratchDir := filepath.Join(workspace, ScratchDirRelPath)
	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("cannot create scratch directory: %v", err), Workspace: workspace, StdoutOnly: true, Code: ExitRungErr}
	}
	lockPath := filepath.Join(scratchDir, "workspace.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("cannot open lock file: %v", err), Workspace: workspace, StdoutOnly: true, Code: ExitRungErr}
	}
	if err := tryLockFile(f); err != nil {
		f.Close()
		if errors.Is(err, errLockBusy) {
			return nil, &ConfigError{Message: "workspace is already locked by another invocation", Workspace: workspace, StdoutOnly: true, Code: ExitRungErr}
		}
		return nil, &ConfigError{Message: fmt.Sprintf("cannot acquire workspace lock: %v", err), Workspace: workspace, StdoutOnly: true, Code: ExitRungErr}
	}
	return &WorkspaceLock{file: f}, nil
}

// Release closes the lock file, releasing the flock. Best-effort: the kernel
// releases on process death regardless.
func (wl *WorkspaceLock) Release() error {
	if wl.file != nil {
		return wl.file.Close()
	}
	return nil
}

// PruneRunScratch reaps every run-scoped scratch directory under the
// workspace's RunsRelPath EXCEPT the one tied to whatever verdict currently
// sits on disk (design §3.4: "the current run's scratch plus the prior
// verdict survive; older run scratch is reaped at the start of the next
// run") — the AP-12 janitor for per-run logs (findings' log_ref targets),
// meant to run once at the start of every invocation, after the workspace
// lock is held. A missing, unreadable, or unparseable verdict keeps nothing:
// every run directory is reaped, since there is then no prior verdict whose
// log_ref could still matter — the normal first-ever-run case, never an
// error. One unremovable directory does not stop the rest from being reaped.
func PruneRunScratch(workspace string) error {
	keep := currentVerdictRunID(workspace)

	root := filepath.Join(workspace, filepath.FromSlash(RunsRelPath))
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("prune run scratch: read %s: %w", root, err)
	}

	var errs []error
	for _, e := range entries {
		if !e.IsDir() || e.Name() == keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", e.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// currentVerdictRunID best-effort reads the workspace's current verdict
// artifact and returns its RunID — the one run-scratch directory
// PruneRunScratch must never reap. Any read or parse failure (including "no
// verdict yet," the ordinary first-run case) returns "", never an error.
func currentVerdictRunID(workspace string) string {
	data, err := os.ReadFile(filepath.Join(workspace, VerdictRelPath))
	if err != nil {
		return ""
	}
	var v schema.Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.RunID
}

// Emit writes the refusal verdict: stdout-only when the workspace flag is the
// defect, else an artifact under .codegate plus a stdout summary.
func (ce *ConfigError) Emit() error {
	verdict := ce.Artifact
	if verdict == nil {
		verdict = ce.buildVerdict()
	}

	if ce.StdoutOnly {
		data, err := verdict.Marshal()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error emitting configuration error: %v\n", err)
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if ce.Workspace == "" {
		return fmt.Errorf("emit: workspace is empty but StdoutOnly is false")
	}
	scratchDir := filepath.Join(ce.Workspace, ScratchDirRelPath)
	if err := os.MkdirAll(scratchDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating scratch directory: %v\n", err)
		return err
	}
	verdictPath := filepath.Join(scratchDir, VerdictFileName)
	// Skip only the bindings-present check (no bindings yet at config-error
	// time); state invariants are still enforced by the shared schema writer.
	if err := verdict.WriteAtomicToFileSkipBindings(verdictPath); err != nil {
		fmt.Fprintf(os.Stderr, "error writing verdict artifact: %v\n", err)
		return err
	}
	fmt.Printf("verdict: %s\n", verdict.State)
	fmt.Printf("artifact: %s\n", verdictPath)
	return nil
}

// buildVerdict constructs the single config-error verdict shape used by both
// emission paths.
func (ce *ConfigError) buildVerdict() *schema.Verdict {
	now := nowFunc()
	return &schema.Verdict{
		RunID:     fmt.Sprintf("error-%d", now.Unix()),
		Workspace: ce.Workspace,
		Mode:      "run",
		State:     "error",
		StartTime: now,
		EndTime:   now,
		Rungs: map[string]*schema.Rung{
			"0": {Name: "0", Order: 0, Checks: map[string]*schema.CheckResult{}, State: "error", Skipped: true, SkipReason: "configuration-error"},
		},
		Findings: []schema.Finding{
			{CheckID: "config-error", Class: "configuration-error", Rung: "0", Message: ce.Message},
		},
		Bindings: schema.Bindings{CodegateVersion: CodegateVersion},
	}
}

// --- git + version helpers ----------------------------------------------------

// runGit runs a git subcommand under ctx with credential prompts and optional
// locks disabled, returning trimmed stdout and wrapping stderr into the error.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	out, err := runGitRaw(ctx, dir, args...)
	return strings.TrimSpace(string(out)), err
}

func runGitRaw(ctx context.Context, dir string, args ...string) ([]byte, error) {
	full := args
	if dir != "" {
		full = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

func getGitVersion(ctx context.Context) (string, error) {
	out, err := runGit(ctx, "", "--version")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(out)
	if len(parts) >= 3 {
		return parts[2], nil
	}
	return "", fmt.Errorf("unexpected git version format: %q", out)
}

func getHeadSHA(ctx context.Context, workspace string) (string, error) {
	return runGit(ctx, workspace, "rev-parse", "HEAD")
}

func resolveDiffBase(ctx context.Context, workspace, ref string) (string, error) {
	// --verify guarantees a single object; ParseInvocation already rejected a
	// flag-like ref, so no option injection reaches here.
	return runGit(ctx, workspace, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
}

// errDirtyTree marks a clean-tree failure that is the workspace's state (a
// config error) rather than an operational git failure (infra).
var errDirtyTree = errors.New("dirty tree")

// assertCleanTree fails closed: any change outside the .codegate scratch dir —
// or any porcelain line it cannot parse — is treated as dirty. NUL-delimited
// (-z) output tolerates paths with spaces or newlines.
func assertCleanTree(ctx context.Context, workspace string) error {
	out, err := runGitRaw(ctx, workspace, "status", "--porcelain", "-z")
	if err != nil {
		return err
	}
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		// A porcelain record is "XY PATH"; a rename/copy adds a following bare
		// source-path record. Extract a candidate path from either shape and
		// fail closed on anything not under the scratch dir.
		path := rec
		if len(rec) >= 3 && rec[2] == ' ' {
			path = rec[3:]
		}
		if path == ScratchDirRelPath || strings.HasPrefix(path, ScratchDirRelPath+"/") {
			continue
		}
		return fmt.Errorf("%w: %s", errDirtyTree, rec)
	}
	return nil
}

// isGitVersionAtLeast compares two dotted versions NUMERICALLY (never lexically:
// "2.43.0" is not below "2.5.0").
func isGitVersionAtLeast(version, minimum string) bool {
	v, m := parseVersion(version), parseVersion(minimum)
	for i := 0; i < 3; i++ {
		if v[i] != m[i] {
			return v[i] > m[i]
		}
	}
	return true
}

// parseVersion extracts up to three leading numeric components, tolerating
// suffixes like "2.39.3 (Apple Git-146)" or "2.5.0-rc0".
func parseVersion(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(v, ".", 4)
	for i := 0; i < 3 && i < len(parts); i++ {
		digits := parts[i]
		for j, c := range digits {
			if c < '0' || c > '9' {
				digits = digits[:j]
				break
			}
		}
		if digits == "" {
			continue
		}
		if n, err := strconv.Atoi(digits); err == nil {
			out[i] = n
		}
	}
	return out
}

// hashData returns the hex SHA-256 of data.
func hashData(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
