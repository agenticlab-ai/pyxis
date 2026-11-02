// Package runner: confine.go is the platform-neutral half of DINV-5's
// write-scope leg (design codegate-v1 §3.5): the Confine seam Exec calls, and
// the post-execution classification that turns a completed-but-failed verb
// into a ClassViolation result when confinement was active. The platform
// split lives in confine_linux.go (real Landlock enforcement) and
// confine_other.go (the named residual — every other platform).
package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Confine rewrites cmd — already fully configured (Dir, Env, Stdout, Stderr,
// SysProcAttr) — so that, where the platform provides enforcement, the
// process it starts can WRITE only beneath allowedPaths; reads and execution
// stay unrestricted everywhere (DINV-5 scopes only the write-class access
// rights, never reads — a verb still needs to read its toolchain, stdlib,
// etc. system-wide). It returns whether confinement will be attempted for
// this exec: true only where the platform supports it AND cmd was
// successfully rewritten. A caller uses this to decide whether a completed
// run's output should be scanned for a write-scope violation signature
// (finalizeConfinement) — never to decide whether to run the verb at all;
// Confine's own failure (as opposed to "this platform has no enforcement,"
// which is not a failure) always refuses fail-closed rather than silently
// running the verb unconfined.
func Confine(cmd *exec.Cmd, allowedPaths []string) (bool, error) {
	return confine(cmd, allowedPaths)
}

// landlockEnforced reports whether the Landlock write-scope leg is active. It
// is OPT-IN, off by default: the enforcement machinery (confine_linux.go) is
// complete and compiled, but a real toolchain verb writes build-cache and temp
// files outside the workspace, and getting that allow-set (WriteAllowedRoots)
// exactly right needs the Linux-CI enforcement matrix design codegate-v1 pins
// A-6 to — not a blind allow-list validated only on macOS, where Landlock does
// not exist to catch a hole. Until that matrix runs green the Linux runner
// matches the macOS residual (confine_other.go); CODEGATE_LANDLOCK=1 turns the
// real enforcement on. This lives in the platform-neutral file (though only
// confine_linux.go consults it) so its default is unit-testable on every
// platform — the one behavior a macOS developer CAN verify about the leg.
func landlockEnforced() bool {
	switch os.Getenv("CODEGATE_LANDLOCK") {
	case "1", "true", "on":
		return true
	default:
		return false
	}
}

// WriteAllowedRoots is the set of directories a confined verb may write to: the
// workspace it verifies (DINV-5's core scope) PLUS the toolchain's own
// out-of-workspace scratch/cache locations a real build command legitimately
// needs. A Go verb (`go build`/`vet`/`test`) writes its build cache and temp
// files to $GOCACHE and the temp dir — the toolchain's own infrastructure, not
// the source tree DINV-5 protects — so a workspace-only allow-set denies those
// writes and reds an honest verb (the write-scope leg's first Linux CI run
// surfaced exactly this). A verb writing into an actual source tree ELSEWHERE
// is still denied: none of these roots is a source location.
//
// Only EXISTING directories are returned: Landlock's path_beneath rule needs an
// openable directory, so a non-existent root would fail the WHOLE ruleset closed
// (restrictSelfToPaths) — reding every verb rather than the one stray write.
func WriteAllowedRoots(workspace string) []string {
	roots := []string{workspace}
	seen := map[string]bool{workspace: true}
	for _, p := range toolchainScratchDirs() {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil || seen[abs] {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			roots = append(roots, abs)
			seen[abs] = true
		}
	}
	return roots
}

// toolchainScratchDirs names the out-of-workspace directories a build toolchain
// writes cache and temp data to. The v1 set covers the Go toolchain (this repo's
// stack and the fixtures') and the POSIX temp dir every toolchain uses: the
// user cache dir (Go's build cache default lives beneath it), the default GOPATH
// (module downloads write there), and the temp dir — plus the standard override
// env vars, so a repo that relocates them is honored. Reads are never restricted
// (DINV-5), so a cache the verb only reads needs no entry — only write targets.
func toolchainScratchDirs() []string {
	dirs := []string{os.TempDir()}
	if c, err := os.UserCacheDir(); err == nil {
		dirs = append(dirs, c)
	}
	if h, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(h, "go"))
	}
	for _, env := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOTMPDIR", "TMPDIR", "XDG_CACHE_HOME"} {
		dirs = append(dirs, os.Getenv(env))
	}
	return dirs
}

// finalizeConfinement is Exec's single post-execution hook: when confinement
// was NOT active for this call, res is returned unchanged. When it was,
// finalizeConfinement first checks for the confinement wrapper's OWN setup
// failure (reclassified as ClassInfra — the harness's own machinery broke,
// never a silent unconfined fallback and never the verb's own red) and
// otherwise, for a verb that completed but did not cleanly pass, scans its
// captured output for a write-scope violation signature (best-effort: the
// enforcement itself is real and unconditional regardless of whether this
// heuristic successfully names the offending path — see
// detectWriteScopeViolation's doc for why no stronger signal is available
// without heavier machinery this design rejects, §3.5's ALT table).
func finalizeConfinement(confined bool, res Result) Result {
	if !confined {
		return res
	}
	if reason, ok := wrapperFailureReason(res); ok {
		res.Class = ClassInfra
		res.Err = &wrapperSetupError{reason: reason}
		return res
	}
	if res.Passed || res.Class != "" {
		// A clean pass has nothing to reclassify; a result that already
		// carries a Class (infra spawn/signal fault, or a bound timeout) is
		// its own distinct, already-correct classification — never
		// overwritten by a content-level heuristic.
		return res
	}
	if path, ok := detectWriteScopeViolation(res.Stdout, res.Stderr); ok {
		res.Class = ClassViolation
		res.Err = &violationError{path: path}
	}
	return res
}

// wrapperSetupError is finalizeConfinement's ClassInfra error when the
// confinement wrapper itself failed before ever reaching the real verb.
type wrapperSetupError struct{ reason string }

func (e *wrapperSetupError) Error() string {
	return "confinement setup failed: " + e.reason
}

// violationError is finalizeConfinement's ClassViolation error. path is the
// best-effort extracted offending path, or empty when the signature matched
// but no path token could be recovered from the verb's own output.
type violationError struct{ path string }

func (e *violationError) Error() string {
	if e.path == "" {
		return "write-scope violation: a filesystem write outside the confined workspace was denied (path undetermined)"
	}
	return "write-scope violation: a filesystem write outside the confined workspace was denied: " + e.path
}

// confineWrapperFailExitCode and confineWrapperFailMarker are the Linux
// confinement wrapper's (confine_linux.go) fixed failure protocol: any setup
// fault (Landlock unsupported, a syscall error, a malformed internal
// invocation) exits with this code and prints this marker prefix — paired,
// never matched by number alone, so a REAL verb that happens to exit with
// the same number for its own unrelated reasons can never be misread as a
// confinement-machinery failure.
const (
	confineWrapperFailExitCode = 121
	confineWrapperFailMarker   = "codegate-confine-exec: "
)

// wrapperFailureReason reports whether res is the confinement wrapper
// reporting its OWN setup failure (as opposed to the real verb's own exit) —
// gated on BOTH the reserved exit code and the marker text, so a coincidental
// same-numbered exit from the real verb is never misclassified.
func wrapperFailureReason(res Result) (string, bool) {
	if res.Class != "" || res.ExitCode != confineWrapperFailExitCode {
		return "", false
	}
	idx := strings.Index(res.Stderr, confineWrapperFailMarker)
	if idx < 0 {
		return "", false
	}
	reason := res.Stderr[idx+len(confineWrapperFailMarker):]
	if nl := strings.IndexByte(reason, '\n'); nl >= 0 {
		reason = reason[:nl]
	}
	return strings.TrimSpace(reason), true
}

// violationSignatureRE recognizes the OS-level permission-denied shape
// Landlock's EACCES denial surfaces as in virtually every mainstream
// toolchain's own error reporting (Go: "permission denied"; Python:
// "PermissionError: ... Permission denied"; Node: "EACCES: permission
// denied, ..."; POSIX coreutils: "... Permission denied"). Landlock itself
// provides no synchronous, self-contained channel naming which access it
// denied (that is what the kernel audit subsystem is for, which requires
// privileged log access this harness does not assume — see design §3.5's
// residual note); this is a best-effort classification layer over the
// denied verb's OWN reporting, never the enforcement itself, which is
// unconditional and real regardless of whether this pattern matches.
var violationSignatureRE = regexp.MustCompile(`(?i)permission denied|EACCES`)

// violationPathRE extracts a plausible offending path from a line that
// already matched violationSignatureRE: a quoted path first (most
// ecosystems quote the path in their own error text), else a bare
// slash-containing token — stopping short of trailing punctuation a
// path itself never carries (Go's own "open /path: permission denied"
// shape ends the path at a colon, not whitespace).
var violationPathRE = regexp.MustCompile(`'([^']+)'|"([^"]+)"|(\S*/[^\s:,'"()\[\]]+)`)

// detectWriteScopeViolation scans a completed, non-passing verb's combined
// output for a permission-denied signature and best-effort extracts the
// offending path. Gated by the caller (finalizeConfinement) to only ever run
// on a verb that did NOT cleanly pass, so a legitimately green verb whose
// output happens to mention "permission denied" in an unrelated context
// (e.g. its own test log) is never reclassified — this can only turn an
// ALREADY-failing result's classification from a plain check red into a
// named security violation, never a pass into a failure.
func detectWriteScopeViolation(stdout, stderr string) (string, bool) {
	combined := stdout + "\n" + stderr
	for _, line := range strings.Split(combined, "\n") {
		if !violationSignatureRE.MatchString(line) {
			continue
		}
		if m := violationPathRE.FindStringSubmatch(line); m != nil {
			for _, g := range m[1:] {
				if g != "" {
					return g, true
				}
			}
		}
		return "", true
	}
	return "", false
}
