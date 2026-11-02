package ecosystem

import (
	"maps"
	"regexp"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// Defaults holds the compiled ecosystem defaults for all supported stacks.
type Defaults struct {
	Version            string
	Stacks             map[string]*StackDefaults
	SecretPatterns     []*regexp.Regexp
	AssertionPatterns  map[string][]*regexp.Regexp
	SkipMarkers        map[string][]*regexp.Regexp
	ForbiddenArtifacts ArtifactRules
}

// FormatOutputMode declares how a successful format-check verb reports its
// verdict. A missing or unknown mode cannot certify formatting.
type FormatOutputMode string

const (
	// FormatOutputExitStatus permits informational stdout on a successful exit.
	FormatOutputExitStatus FormatOutputMode = "exit-status"
	// FormatOutputFileList treats each nonblank stdout line as an unformatted file.
	FormatOutputFileList FormatOutputMode = "file-list"
)

// StackDefaults holds stack-specific configuration, including the default verb
// set a minimal manifest inherits (T1 acceptance criterion 1: a manifest
// declaring only a stack and a baseline path resolves the full verb set here).
type StackDefaults struct {
	Name string
	// MarkerFiles are the root-relative filenames whose presence identifies
	// a repository as this stack — the registry adopt's stack detection
	// reads (design:existing-repo-onboarding-v2#3.6). Presence is checked by
	// filename only; content is never parsed.
	MarkerFiles           []string
	Verbs                 map[string]*schema.Verb
	FormatOutputMode      FormatOutputMode
	TestGlobs             []string
	StateDirs             []string
	CachePins             map[string]string
	FailureIdentityParser string
	// FocusedTestStrategy names this stack's rung-3 focused-test strategy in
	// focusedTestStrategies (falsification_strategy.go); "" means the stack
	// has none and falsification runs a whole-suite, exit-code-only leg.
	FocusedTestStrategy string
}

// ArtifactRules are the default forbidden-artifact rules (T4 consumes them): a
// ceiling on added-file size and patterns matching baked absolute paths in added
// content. Binary-content detection is a runtime heuristic (T4), not table data.
type ArtifactRules struct {
	MaxFileBytes         int64
	AbsolutePathPatterns []*regexp.Regexp
}

// The pattern sets are compile-time constants, compiled once at package init via
// regexp.MustCompile so a malformed pattern fails LOUDLY at startup rather than
// being silently dropped from the scan set (a silently-dropped secret or
// assertion pattern is a fail-open detector — #402 review).
var (
	secretPatterns = mustCompileAll(
		// key: value / key = value. Case-INSENSITIVE (so API_KEY, Password,
		// TOKEN are caught, not just lowercase), and the keyword may sit at the
		// end of a longer identifier (aws_secret_access_key, MY_API_KEY) via the
		// leading [\w.-]* — the keyword must still abut the delimiter, so an
		// unrelated `notsecretfile: x` does not match. A non-empty value (\S+)
		// is required, so a bare key line does not trip the floor.
		`(?i)[\w.-]*(password|passwd|secret|api[_-]?key|access[_-]?key|auth[_-]?token|token|credential|private[_-]?key|rsa[_-]?key)\s*[:=]\s*\S+`,
		// Bearer / Authorization header carrying a token.
		`(?i)\bbearer\s+\S{10,}`,
		`(?i)\bauthorization\s*[:=]\s*\S+`,
		// AWS access-key IDs (AKIA/ASIA + 16 base32) — value-only, no keyword.
		`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`,
		// PEM private-key blocks, any key type.
		`-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----`,
	)
	goAssertionPatterns = mustCompileAll(
		`\bt\.Assert`,
		`\bt\.Fatalf?\(`, // t.Fatal(...) and t.Fatalf(...) — both drop a running test (T7 review: bare Fatal/Error went unmatched, 99/35 real occurrences in this repo's own tests)
		`\bt\.Errorf?\(`, // t.Error(...) and t.Errorf(...)
		`\bassert\.`,     // testify: assert.Equal(...), etc.
		`\brequire\.`,    // testify: require.NoError(...), etc.
	)
	goSkipMarkers = mustCompileAll(
		`\bt\.Skipf?\(`, // t.Skip(...) and t.Skipf(...) — the prior second entry (`\t\.Skip\(`, a literal tab before ".Skip(") matched no realistic source line; Skipf is a real, used-in-this-repo gap it also missed (T7 review)
		`\bt\.SkipNow\(`,
		// Build-constraint exclusion: `//go:build ignore` (and the legacy
		// `// +build ignore`) drop the whole file from `go test` as surely as a
		// t.Skip drops one test — the canonical way to silence a test file
		// without deleting it. Anchored at line start and requiring the `ignore`
		// tag as a whole word, so a legitimate `//go:build linux` constraint or
		// an `ignore` substring in a comment does not trip the floor.
		`(?m)^//go:build\b.*\bignore\b`,
		`(?m)^// \+build\b.*\bignore\b`,
	)
	// python (T10 fixture-python): a bare `assert`, unittest's `self.assert*`,
	// and pytest's `pytest.raises` context — the assertion forms real python
	// tests (and the fixture) use. Data, per FR-16; grows by data PR (Q2).
	pythonAssertionPatterns = mustCompileAll(
		`\bassert\b`,
		`\bself\.assert\w*\(`,
		`\bpytest\.raises\b`,
	)
	pythonSkipMarkers = mustCompileAll(
		`@pytest\.mark\.skip\b`,
		`@unittest\.skip\b`,
		`\bunittest\.skip\b`,
		`\bpytest\.skip\(`,
		`\bself\.skipTest\(`,
	)
	// node (T10 fixture-node): node:assert's assertion calls, a bare `assert(`,
	// and chai/jest's `expect(`/`.should` — the forms real node tests use.
	nodeAssertionPatterns = mustCompileAll(
		`\bassert\.\w+\(`,
		`\bassert\(`,
		`\bexpect\(`,
		`\.should\b`,
	)
	nodeSkipMarkers = mustCompileAll(
		`\.skip\(`, // it.skip(, describe.skip(, test.skip(
		`\bxit\(`,  // mocha/jasmine pending-test forms
		`\bxdescribe\(`,
		`\bit\.only\b`, // .only silences every OTHER test — a silencing marker too
		`\bdescribe\.only\b`,
	)
	forbiddenArtifacts = ArtifactRules{
		MaxFileBytes: 1 << 20, // 1 MiB ceiling on an added file
		AbsolutePathPatterns: mustCompileAll(
			// A machine-specific unix absolute path baked into content. Matched
			// after start-of-line OR a common delimiter (whitespace, quote, =,
			// (, comma, colon) so a path inside a string literal or assignment
			// — os.Open("/Users/…"), dir="/tmp/…", path=/etc/… — is caught, not
			// just a whitespace-led one.
			`(^|[\s"'=(,:])/(Users|home|root|tmp|private/tmp|var|etc|opt|usr|mnt|srv)/`,
			`(^|[\s"'=(,:])[A-Za-z]:\\`, // windows absolute paths
		),
	}
)

// mustCompileAll compiles every pattern, panicking on the first malformed one.
// Its inputs are compile-time constants, so a compile failure is a programmer
// error that must surface at startup — never a quietly-omitted detector.
func mustCompileAll(patterns ...string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}
	return compiled
}

// goVerbEnv is the default environment for the go stack's verbs: pass the
// essential ambient names through (go tooling needs PATH and HOME) — the runner
// strips everything else by default (DINV-5). A manifest verb overrides this.
var goVerbEnv = map[string]schema.EnvEntry{
	"PATH": {Mode: schema.EnvPass},
	"HOME": {Mode: schema.EnvPass},
}

// New returns the compiled v1 ecosystem defaults. The pattern sets are shared,
// immutable, package-level compiled regexps (no per-call recompilation).
func New() *Defaults {
	return &Defaults{
		Version: "1.0.0",
		Stacks: map[string]*StackDefaults{
			"go": {
				Name:             "go",
				MarkerFiles:      []string{"go.mod"},
				FormatOutputMode: FormatOutputFileList, // gofmt -l exits zero even when it lists unformatted files.
				// Verb keys match design:codegate-v1#3.7's "verbs consumed" column
				// ("format-check", "lint", "build") exactly — the ladder's format and
				// lint checks (rung 0) resolve these names from the bound verb set, so
				// a mismatch here leaves a check permanently verb-absent against the
				// very defaults meant to bind it.
				Verbs: map[string]*schema.Verb{
					"format-check": {Command: "gofmt -l .", Env: goVerbEnv},
					"lint":         {Command: "go vet ./...", Env: goVerbEnv},
					"build":        {Command: "go build ./...", Env: goVerbEnv},
					// -json: rung 2's no-new-failures evaluation (design §3.7/§3.10)
					// needs exact per-test pass/fail identity, not plain-text scraping —
					// the structured NDJSON stream is what
					// ecosystem.parseGoTestFailures consumes (falsification's rung-3
					// two-leg protocol already established -json as this repo's
					// precedent for machine-parseable go test output).
					"test": {Command: "go test -json ./...", Env: goVerbEnv},
				},
				TestGlobs: []string{
					"**/*_test.go",
					"**/testdata/**",
				},
				StateDirs: []string{},
				CachePins: map[string]string{
					"GOCACHE": ".codegate/cache/go",
				},
				FailureIdentityParser: "go",
				FocusedTestStrategy:   "go-test-json",
			},
			// python and node (T10) are the two non-go stacks FR-16's matrix
			// binds. Their verbs are scripted and declared in each fixture's own
			// manifest (fixture-specific commands); only the per-stack DATA — test
			// globs, state dirs, and the skip/assertion tables below — lives here.
			// No FailureIdentityParser (rung 2's baseline is a T5 stub, never
			// called) and no FocusedTestStrategy (falsification runs whole-suite,
			// keyed off the bound test verb's exit code — checks/falsification.go).
			"python": {
				Name:             "python",
				MarkerFiles:      []string{"requirements.txt", "pyproject.toml", "setup.py"},
				Verbs:            map[string]*schema.Verb{},
				FormatOutputMode: FormatOutputExitStatus,
				TestGlobs:        []string{"**/test_*.py", "**/*_test.py"},
				StateDirs:        []string{},
				CachePins:        map[string]string{},
			},
			"node": {
				Name:             "node",
				MarkerFiles:      []string{"package.json"},
				Verbs:            map[string]*schema.Verb{},
				FormatOutputMode: FormatOutputExitStatus,
				TestGlobs:        []string{"**/*.test.js", "**/*.spec.js"},
				// node_modules is the canonical node_modules-class state dir a bare
				// checkout lacks (design §3.9): the fixture's local dep lives here,
				// so falsification's worktree must be provisioned with it or the
				// sanity leg cannot run — exercising Provision per stack (A-2/A-3).
				StateDirs: []string{"node_modules"},
				CachePins: map[string]string{},
			},
		},
		SecretPatterns: secretPatterns,
		AssertionPatterns: map[string][]*regexp.Regexp{
			"go":     goAssertionPatterns,
			"python": pythonAssertionPatterns,
			"node":   nodeAssertionPatterns,
		},
		SkipMarkers: map[string][]*regexp.Regexp{
			"go":     goSkipMarkers,
			"python": pythonSkipMarkers,
			"node":   nodeSkipMarkers,
		},
		ForbiddenArtifacts: forbiddenArtifacts,
	}
}

// ForStack returns the defaults for a given stack.
func (d *Defaults) ForStack(stack string) *StackDefaults {
	if s, ok := d.Stacks[stack]; ok {
		return s
	}
	return nil
}

// StackNames returns every known stack name, sorted. Stacks is a map — Go
// map iteration order is not stable — so a caller that must check every
// known stack in a fixed, deterministic order (adopt's stack detection,
// design:existing-repo-onboarding-v2#3.6) enumerates through this rather
// than ranging over Stacks directly.
func (d *Defaults) StackNames() []string {
	names := make([]string, 0, len(d.Stacks))
	for name := range d.Stacks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveVerbs returns the effective verb set for a stack: the stack's default
// verbs, with any manifest-declared verb of the same name overriding the default
// and any manifest-only verb added. A manifest that declares no verbs resolves
// the full default set (T1 acceptance criterion 1). The result is a fresh map of
// fresh Verb values, so a caller can never mutate the shared defaults.
func (d *Defaults) ResolveVerbs(stack string, manifestVerbs map[string]*schema.Verb) map[string]*schema.Verb {
	resolved := map[string]*schema.Verb{}
	if sd := d.ForStack(stack); sd != nil {
		for name, v := range sd.Verbs {
			resolved[name] = copyVerb(v)
		}
	}
	for name, v := range manifestVerbs {
		resolved[name] = copyVerb(v)
	}
	return resolved
}

// copyVerb deep-copies a Verb (including its Env map) so resolved verbs never
// alias the shared defaults.
func copyVerb(v *schema.Verb) *schema.Verb {
	if v == nil {
		return nil
	}
	cp := *v
	cp.Env = maps.Clone(v.Env)
	return &cp
}

// ResolveManifestDefaults merges manifest settings with ecosystem defaults.
func (d *Defaults) ResolveManifestDefaults(stack string, manifestTestGlobs []string, manifestStateDirs []string) (testGlobs, stateDirs []string) {
	stackDefaults := d.ForStack(stack)
	if stackDefaults == nil {
		return manifestTestGlobs, manifestStateDirs
	}

	if len(manifestTestGlobs) > 0 {
		testGlobs = manifestTestGlobs
	} else {
		testGlobs = stackDefaults.TestGlobs
	}

	if len(manifestStateDirs) > 0 {
		stateDirs = manifestStateDirs
	} else {
		stateDirs = stackDefaults.StateDirs
	}

	return
}
