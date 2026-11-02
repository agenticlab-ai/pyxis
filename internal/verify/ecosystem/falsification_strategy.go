package ecosystem

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// FocusedTestStrategy is a per-stack rule for rung 3's falsification two-leg
// (design codegate-v1 §3.9): how to resolve a changed test file to
// function-level targets, how to build a FOCUSED test command for them, and
// how to read per-test outcomes back from that command's output. It is
// ecosystem-table DATA — dispatched by stack via focusedTestStrategies,
// mirroring failureIdentityParsers' shape — even though a strategy's bodies
// are code. A stack with NO strategy (the common case for a stack whose test
// runner has no machine-parseable per-test stream) is not blocked: the
// falsification check falls back to a whole-suite leg keyed off the test
// verb's exit code alone (checks/falsification.go), so extending stack support
// (T10) is a data addition here, never a rewrite of the check.
type FocusedTestStrategy struct {
	// TargetablePath reports whether path is a test file this strategy can
	// resolve to function-level targets — the caller reads its content only
	// then. A false result means "cover this file whole-suite."
	TargetablePath func(path string) bool
	// ExtractTargetNames returns the new-or-changed test names in a targetable
	// file's content: every top-level test when the file was added, otherwise
	// those whose span a changed (added) line number falls in. An empty result
	// means "cover this file whole-suite" (no function-level identity found).
	ExtractTargetNames func(content []byte, addedLines []int, added bool) []string
	// BuildCommand builds the focused test command for one package dir and
	// (optionally) a set of function names to restrict the run to.
	BuildCommand func(dir string, names []string) string
	// ParseOutcomes maps every test name that reached a terminal outcome to
	// "pass" | "fail" | "skip", read from the focused command's stdout. A name
	// absent from the map never reached a terminal outcome and the caller must
	// treat it as not-passed.
	ParseOutcomes func(output string) map[string]string
}

// focusedTestStrategies maps a per-stack strategy NAME
// (StackDefaults.FocusedTestStrategy) to its implementation. The go strategy
// is the `go test -json` focusing/parsing relocated here from
// checks/falsification.go so the check itself carries no stack-specific code
// (FR-16 / A-1). Further stacks register their own entry as they gain
// fixtures.
var focusedTestStrategies = map[string]FocusedTestStrategy{
	"go-test-json": {
		TargetablePath:     goTargetablePath,
		ExtractTargetNames: goExtractTargetNames,
		BuildCommand:       goBuildCommand,
		ParseOutcomes:      parseGoTestOutcomes,
	},
}

// FocusedTestStrategyFor returns stack's focused-test strategy, or ok=false
// when the stack declares none (⇒ the whole-suite fallback). A declared-but-
// unregistered strategy name also reports ok=false rather than panicking: an
// unknown strategy simply degrades that stack to whole-suite, never a crash.
func (d *Defaults) FocusedTestStrategyFor(stack string) (FocusedTestStrategy, bool) {
	sd := d.ForStack(stack)
	if sd == nil || sd.FocusedTestStrategy == "" {
		return FocusedTestStrategy{}, false
	}
	strat, ok := focusedTestStrategies[sd.FocusedTestStrategy]
	return strat, ok
}

// --- the go strategy (relocated verbatim from checks/falsification.go so its
// behavior is unchanged; the falsification tests remain its guard) ---

// goTargetablePath: only a `_test.go` file yields function-level go targets; a
// `**/testdata/**` glob match (or any other non-`_test.go` file) is covered
// whole-suite, so the caller never reads its content.
func goTargetablePath(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// goExtractTargetNames returns the new-or-changed top-level Test functions in
// a `_test.go` file: every one when the file was added, else those whose span
// an added line falls in — so an untouched sibling test in a modified file is
// never swept in as a false target. An empty result makes the caller cover the
// file whole-suite.
func goExtractTargetNames(content []byte, addedLines []int, added bool) []string {
	var names []string
	for _, sp := range goTestFuncSpans(content) {
		if added || sp.touchedBy(addedLines) {
			names = append(names, sp.name)
		}
	}
	return names
}

// goBuildCommand builds `go test -json`, focused to dir and — when names are
// given — filtered to exactly those functions via -run, so a same-package
// sibling test (possibly itself pre-existing-failing) can never pollute a leg.
func goBuildCommand(dir string, names []string) string {
	if dir == "" {
		dir = "./..."
	}
	command := "go test -json"
	if len(names) > 0 {
		command += " -run '^(" + strings.Join(names, "|") + ")$'"
	}
	command += " " + dir
	return command
}

// parseGoTestOutcomes reads a `go test -json` stream and returns, for every
// test that reached a terminal per-test action, whether it passed/failed/
// skipped. A name absent from the result never ran to a terminal outcome
// (most commonly a revert broke the build) and a caller must treat that
// absence as "not passed." Malformed/truncated output degrades to whatever
// was decodable before the first bad token, never a panic. (Distinct from
// parseGoTestFailures, which fails CLOSED — rung 2's baseline needs that;
// rung 3's falsification treats every unresolved name as not-passed either
// way, so tolerating truncation here is safe.)
func parseGoTestOutcomes(output string) map[string]string {
	outcomes := map[string]string{}
	dec := json.NewDecoder(strings.NewReader(output))
	for {
		var ev goTestEvent
		if err := dec.Decode(&ev); err != nil {
			break
		}
		if ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass":
			outcomes[ev.Test] = "pass"
		case "fail", "skip":
			// A pass is authoritative and never regresses; otherwise record the
			// terminal fail/skip so a caller can tell a skip (not-applicable)
			// from a fail (bites). An absent name never reached a terminal
			// outcome and is treated as not-passed.
			if outcomes[ev.Test] != "pass" {
				outcomes[ev.Test] = ev.Action
			}
		}
	}
	return outcomes
}

// goTestFuncSpan is one top-level Go test function's line range within a
// file's content — the func line through the line before the next top-level
// Test function (or EOF).
type goTestFuncSpan struct {
	name  string
	start int
	end   int
}

// touchedBy reports whether any added line number falls within this function's
// span — true for a body-only change even though the function's signature line
// itself never appears in the added set.
func (s goTestFuncSpan) touchedBy(added []int) bool {
	for _, l := range added {
		if l >= s.start && l <= s.end {
			return true
		}
	}
	return false
}

// topLevelTestFuncRE matches a top-level (column-0) Go test function
// declaration. Anchored at line start: Go requires top-level funcs there, so
// this never matches a method, a closure, or an indented nested declaration.
var topLevelTestFuncRE = regexp.MustCompile(`(?m)^func\s+(Test[A-Za-z0-9_]*)\s*\(`)

// goTestFuncSpans scans content for every top-level Test function declaration
// and returns each one's line span, ordered by start line. TestMain is
// excluded: it is the package's test harness, not a runnable test (`go test
// -json` emits no per-test event for it), so targeting it by name would make
// the sanity leg read "did not pass" and error the whole check.
func goTestFuncSpans(content []byte) []goTestFuncSpan {
	matches := topLevelTestFuncRE.FindAllSubmatchIndex(content, -1)
	spans := make([]goTestFuncSpan, 0, len(matches))
	for _, m := range matches {
		name := string(content[m[2]:m[3]])
		if name == "TestMain" {
			continue
		}
		spans = append(spans, goTestFuncSpan{
			name:  name,
			start: bytes.Count(content[:m[0]], []byte("\n")) + 1,
		})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	totalLines := bytes.Count(content, []byte("\n")) + 1
	for i := range spans {
		if i+1 < len(spans) {
			spans[i].end = spans[i+1].start - 1
		} else {
			spans[i].end = totalLines
		}
	}
	return spans
}
