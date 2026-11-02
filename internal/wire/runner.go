package wire

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
)

// Runner is the gh transport seam — the ONE boundary between the declared
// operation table and the world. The production implementation shells the
// ambient gh CLI (GHRunner); tests inject a transcript fake, keeping every
// execution path hermetic (installer-v1 §3.11's remote-but-owned bin).
type Runner interface {
	// RunGH executes `gh args...` to completion. err is non-nil ONLY for
	// transport-level failure — gh absent (wrapping exec.ErrNotFound), a
	// start failure, or context cancellation; a gh process that ran and
	// exited non-zero is NOT an error here: its exitCode and stderr carry
	// the outcome for classification.
	RunGH(ctx context.Context, args ...string) (stdout, stderr string, exitCode int, err error)
}

// GHRunner is the production Runner: it shells the user's ambient,
// authenticated gh (installer-v1 §3.9 leg 3 — GitHub mutations run under
// the user's own credential; no token ever lives here).
type GHRunner struct{ TokenEnv, Host string }

var _ Runner = GHRunner{}

// RunGH locates gh on PATH and executes it. A missing gh returns an error
// wrapping exec.ErrNotFound — the signal Execute's preflight turns into
// the named GHAbsentError refusal.
func (r GHRunner) RunGH(ctx context.Context, args ...string) (string, string, int, error) {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "", "", 0, fmt.Errorf("wire: locate gh: %w", err)
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, path, args...)
	if r.TokenEnv != "" || r.Host != "" {
		cmd.Env, err = githubauth.EnvironmentForHost(r.TokenEnv, r.Host)
		if err != nil {
			return "", "", 0, fmt.Errorf("wire: %w", err)
		}
	}
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if runErr := cmd.Run(); runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			return stdout.String(), stderr.String(), ee.ExitCode(), nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return stdout.String(), stderr.String(), 0, fmt.Errorf("wire: gh %s: %w", strings.Join(args, " "), ctxErr)
		}
		return stdout.String(), stderr.String(), 0, fmt.Errorf("wire: run gh %s: %w", strings.Join(args, " "), runErr)
	}
	return stdout.String(), stderr.String(), 0, nil
}

// GHAbsentError is the named GitHub-mode prerequisite refusal (the
// prd:installer/UB-7 class, surfaced through installer-v1 §3.6's "gh is a
// GitHub-mode-only prerequisite"): the gh CLI is absent or unrunnable, so
// Execute refuses BEFORE any declared operation runs — zero operations,
// zero partial trust (AP-17). A wiring-free table never triggers it: the
// offline floor never touches gh.
type GHAbsentError struct {
	Reason string // what the preflight probe observed
	Err    error  // the underlying lookup/run error, when any
}

func (e *GHAbsentError) Error() string {
	return fmt.Sprintf("wire: the gh CLI is unavailable (%s) — GitHub-mode wiring refuses with zero operations performed (prd:installer/UB-7); remediation: install gh (https://cli.github.com) and authenticate (`gh auth login`), then re-run — or choose wiring-free knobs (tracker=carina, wiring.protection=none, wiring.repo=use)", e.Reason)
}

func (e *GHAbsentError) Unwrap() error { return e.Err }

// Failure classification — the patterns are DATA, the stated contract of
// UB-4's two legs (the T10 charter: "document the patterns as data"). They
// match case-insensitively against gh's stderr; gh spells HTTP statuses
// "(HTTP 403)", so statuses are matched with the "http " prefix — never a
// bare number that an issue id in a path could satisfy (the ghtracker
// precedent).
//
// Order matters: transient is checked FIRST, because GitHub surfaces rate
// limits as HTTP 403/429 with a "rate limit" phrase — a rate-limited call
// is a retry case, never a permission verdict.
var (
	// transientPatterns — bounded-retry-then-hand-off (UB-4's transient
	// leg): rate limiting, server errors, and network failure.
	transientPatterns = []string{
		"http 429",
		"http 500", "http 502", "http 503", "http 504",
		"rate limit", // "API rate limit exceeded", "secondary rate limit" — even when carried by a 403
		"timeout", "timed out",
		"connection refused", "connection reset",
		"no such host", "temporary failure in name resolution",
		"network is unreachable",
	}

	// permissionPatterns — complete-the-rest-and-hand-off (UB-4's
	// permission leg): the credential ran but lacks the right.
	permissionPatterns = []string{
		"http 403",
		"forbidden",
		"resource not accessible", // fine-grained token / app-installation scope gaps
		"must have admin rights",  // the branch-protection admin message
	}

	// notFoundPatterns — the miss signal. Its meaning is per-op: an
	// existence probe consumes it as "object absent, proceed to create";
	// the set-protection PUT consumes it as a PERMISSION verdict, because
	// GitHub answers 404 — not 403 — on admin-only endpoints when the
	// credential lacks admin (the charter's "404-admin" case); elsewhere
	// it is a hard defect (the target repository does not exist).
	notFoundPatterns = []string{
		"http 404",
		"could not resolve to a repository", // gh's GraphQL miss spelling
		"not found",
	}
)

// failureClass is the classification of a completed, non-zero gh exit.
type failureClass int

const (
	classOther failureClass = iota // unclassified — a hard, named failure
	classTransient
	classPermission
	classNotFound
)

func (c failureClass) String() string {
	switch c {
	case classTransient:
		return "transient"
	case classPermission:
		return "permission"
	case classNotFound:
		return "not-found"
	}
	return "unclassified"
}

// classify maps a non-zero gh exit's stderr onto UB-4's failure classes.
// The exit code itself does not discriminate (gh exits 1 for nearly
// everything); the stderr patterns above are the contract.
func classify(stderr string) failureClass {
	msg := strings.ToLower(stderr)
	for _, p := range transientPatterns {
		if strings.Contains(msg, p) {
			return classTransient
		}
	}
	for _, p := range permissionPatterns {
		if strings.Contains(msg, p) {
			return classPermission
		}
	}
	for _, p := range notFoundPatterns {
		if strings.Contains(msg, p) {
			return classNotFound
		}
	}
	return classOther
}

// IsTransient reports whether a completed gh exit's stderr matches this
// package's transient-failure patterns (rate limiting, server errors,
// network failure) — the retry-worthy class (UB-4). Exported so a gh probe
// outside this package's own Execute walk (the preflight GitHub-mode
// existence check, design init-hardening-v1 §3.1 check 3; risk R4) retries
// through the SAME pattern vocabulary this package already owns, rather
// than a second, driftable copy.
func IsTransient(stderr string) bool {
	return classify(stderr) == classTransient
}

// matchesAny reports whether stderr matches any pattern,
// case-insensitively — the benign-outcome probe (e.g. "already exists" on
// a create, which IS the ensured end state, AP-22).
func matchesAny(stderr string, patterns []string) bool {
	msg := strings.ToLower(stderr)
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// firstLine trims stderr to its first non-empty line for reason strings —
// the audit keeps outcomes readable; the full command argv is recorded
// beside it, redacting nothing.
func firstLine(s string) string {
	for line := range strings.SplitSeq(strings.TrimSpace(s), "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
