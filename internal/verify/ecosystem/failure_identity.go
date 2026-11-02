package ecosystem

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TestFailure is one failure a stack's failure-identity parser extracted
// from test-verb output: a stable Identity for baseline comparison (design
// codegate-v1 §3.10), plus best-effort location/detail for the finding
// schema's "file:line where the class has one" convention (§3.6). Path/Line
// are zero and Message may be empty when the underlying output carried no
// single call site (e.g. a package that failed to build).
type TestFailure struct {
	Identity string
	Path     string
	Line     int
	Message  string
}

// failureIdentityParsers maps a per-stack parser NAME (ecosystem table
// data — StackDefaults.FailureIdentityParser) to its compiled parsing
// function. Further stacks register their own entry here as they gain
// fixtures (T10); the dispatch stays data-shaped even though a parser's
// body is code, mirroring AssertionPatterns/SkipMarkers' per-stack table
// shape.
var failureIdentityParsers = map[string]func(output string) ([]TestFailure, error){
	"go": parseGoTestFailures,
}

// HasFailureIdentityParser reports whether stack has a registered, usable
// failure-identity parser — the precondition for rung 2's no-new-failures
// evaluation (design §3.7). A stack without one cannot produce per-test
// identities, so rung 2 skips it (checks.noNewFailures) rather than guessing:
// a bound baseline for such a stack is a misconfiguration, an unbound one has
// nothing to evaluate. This mirrors ParseFailureIdentities' own resolution so
// the two never disagree on which stacks are parseable.
func (d *Defaults) HasFailureIdentityParser(stack string) bool {
	sd := d.ForStack(stack)
	if sd == nil || sd.FailureIdentityParser == "" {
		return false
	}
	_, ok := failureIdentityParsers[sd.FailureIdentityParser]
	return ok
}

// ParseFailureIdentities dispatches to stack's registered failure-identity
// parser. An unknown stack, or a stack with no registered parser name, or a
// registered name with no compiled parser, all fail closed: a repo whose
// stack cannot be parsed can never evaluate a bound baseline (UB-9's
// spirit — unparseable is never silently read as zero failures).
func (d *Defaults) ParseFailureIdentities(stack, output string) ([]TestFailure, error) {
	sd := d.ForStack(stack)
	if sd == nil || sd.FailureIdentityParser == "" {
		return nil, fmt.Errorf("no failure-identity parser registered for stack %q", stack)
	}
	parser, ok := failureIdentityParsers[sd.FailureIdentityParser]
	if !ok {
		return nil, fmt.Errorf("failure-identity parser %q (stack %q) is not registered", sd.FailureIdentityParser, stack)
	}
	return parser(output)
}

// goTestEvent is the subset of `go test -json`'s TestEvent this parser
// reads. ImportPath is populated only on build-fail/build-output events —
// go test's own asymmetry, not this struct's.
type goTestEvent struct {
	Action     string
	Package    string
	Test       string
	ImportPath string
	Output     string
}

// goTestKey identifies one test within one package — the granularity a
// failure identity is keyed at, since two packages may both declare a test
// of the same name.
type goTestKey struct {
	Package string
	Test    string
}

// goTestLoc is a best-effort source location captured from a test's own
// "output" events.
type goTestLoc struct {
	Path string
	Line int
}

// goTestLocationRE matches a test's own file:line detail line inside a
// captured "output" event's text, e.g. "    foo_test.go:15: message" — the
// same "file.go:LINE:" convention build/lint diagnostics use
// (internal/verify/checks.parseDiagnostics), just carried inside a JSON
// Output field instead of raw stdout.
var goTestLocationRE = regexp.MustCompile(`^\s*(\S+\.go):(\d+):`)

// parseGoTestFailures extracts stable failure identities from `go test
// -json ./...`'s NDJSON event stream (design codegate-v1 §3.7 rung 2,
// §3.10): one "<package>::<test>" identity per LEAF failing (sub)test — go
// test emits a "fail" event for every ANCESTOR of a failing subtest too, so
// a name that is a strict "/"-prefix of another failing name in the same
// package is the same underlying failure reported twice, collapsed to its
// most specific descendant — or one "<package> [build failed]" identity for
// a package with no discrete test breakdown at all (its test binary never
// compiled).
//
// Unlike checks.parseGoTestOutcomes (rung 3's falsification, which
// tolerates a truncated stream because its caller treats every unresolved
// name as "not passed" regardless of why), this function fails CLOSED on
// any decode error and on any test that started but never reached a
// terminal outcome (e.g. a same-package panic that aborted the binary
// mid-run): rung 2's no-new-failures evaluation must never silently
// under-report a failure as baseline shrinkage just because something else
// in the same package crashed before that test's own verdict was recorded.
func parseGoTestFailures(output string) ([]TestFailure, error) {
	dec := json.NewDecoder(strings.NewReader(output))

	started := map[goTestKey]bool{}
	outcome := map[goTestKey]string{} // "pass" | "fail" | "skip"
	location := map[goTestKey]goTestLoc{}
	buildFailed := map[string]bool{}
	buildMessage := map[string]string{}

	sawEvent := false
	for {
		var ev goTestEvent
		err := dec.Decode(&ev)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("go test -json output: malformed event: %w", err)
		}
		sawEvent = true

		switch {
		case ev.Test != "" && ev.Action == "run":
			started[goTestKey{ev.Package, ev.Test}] = true

		case ev.Test != "" && (ev.Action == "pass" || ev.Action == "fail" || ev.Action == "skip"):
			k := goTestKey{ev.Package, ev.Test}
			if outcome[k] != "pass" { // a recorded pass is authoritative, never downgraded
				outcome[k] = ev.Action
			}

		case ev.Test != "" && ev.Action == "output":
			k := goTestKey{ev.Package, ev.Test}
			if _, ok := location[k]; !ok {
				if m := goTestLocationRE.FindStringSubmatch(ev.Output); m != nil {
					line, _ := strconv.Atoi(m[2]) // m[2] is the regex's (\d+) capture — all digits
					location[k] = goTestLoc{Path: m[1], Line: line}
				}
			}

		case ev.Action == "build-fail":
			buildFailed[cleanImportPath(ev.ImportPath)] = true

		case ev.Action == "build-output":
			pkg := cleanImportPath(ev.ImportPath)
			if buildMessage[pkg] == "" {
				if d := firstDiagnosticLine(ev.Output); d != "" {
					buildMessage[pkg] = d
				}
			}
		}
	}
	if !sawEvent {
		return nil, nil
	}

	var orphaned []string
	for k := range started {
		if _, ok := outcome[k]; !ok {
			orphaned = append(orphaned, k.Package+"::"+k.Test)
		}
	}
	if len(orphaned) > 0 {
		sort.Strings(orphaned)
		return nil, fmt.Errorf("go test -json output: %d test(s) started but never reached a terminal outcome: %s", len(orphaned), strings.Join(orphaned, ", "))
	}

	byPkg := map[string][]string{}
	for k, st := range outcome {
		if st == "fail" {
			byPkg[k.Package] = append(byPkg[k.Package], k.Test)
		}
	}

	var failures []TestFailure
	for pkg, names := range byPkg {
		for _, name := range leafNames(names) {
			loc := location[goTestKey{pkg, name}]
			failures = append(failures, TestFailure{
				Identity: pkg + "::" + name,
				Path:     loc.Path,
				Line:     loc.Line,
			})
		}
	}
	for pkg := range buildFailed {
		failures = append(failures, TestFailure{
			Identity: pkg + " [build failed]",
			Message:  firstNonEmptyString(buildMessage[pkg], "package failed to build for testing"),
		})
	}

	sort.Slice(failures, func(i, j int) bool { return failures[i].Identity < failures[j].Identity })
	return failures, nil
}

// leafNames filters names (failing test names within ONE package) to only
// the deepest (leaf) subtests: go test reports a "fail" for every ancestor
// of a failing subtest, so a name that is a strict "/"-prefix of another
// name in the same set is the same underlying failure reported twice.
func leafNames(names []string) []string {
	leaves := make([]string, 0, len(names))
	for i, n := range names {
		isAncestor := false
		for j, m := range names {
			if i != j && strings.HasPrefix(m, n+"/") {
				isAncestor = true
				break
			}
		}
		if !isAncestor {
			leaves = append(leaves, n)
		}
	}
	return leaves
}

// cleanImportPath strips go test -json's " [x.test]" build-binary suffix
// from an ImportPath field (e.g. "example.com/foo [example.com/foo.test]"),
// leaving the plain import path. Import paths never contain spaces, so
// splitting on the first one is exact.
func cleanImportPath(importPath string) string {
	return strings.SplitN(importPath, " ", 2)[0]
}

// firstDiagnosticLine returns the first non-empty line of a build-output
// event's captured text that is not a "# <package>" header — the
// compiler's actual diagnostic (e.g. "d/d_test.go:6:2: undefined: foo"),
// not go test's own framing.
func firstDiagnosticLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		return t
	}
	return ""
}

// firstNonEmptyString returns the first non-empty argument, or "" if all
// are empty.
func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
