// Package cliguard is the shared fail-closed argument guard for this repo's
// thin mains — the incident-I3 discipline as one implementation
// (docs/observability/incidents.md I3; prd:installer/UB-9; installer-v1
// §3.4, DINV-5).
//
// The failure class it closes: stdlib flag parsing stops at the first
// positional, so a flag placed after one is silently reinterpreted as a
// positional — the way `plangate project <dir> --dry-run` once ignored
// --dry-run and executed the full write set (I3). Consequential CLIs
// refuse on misparse BEFORE any action instead. Three disciplines:
//
//   - Positionals — the strict grammar: flags precede positionals. An
//     unknown flag, a malformed value, or a flag-like token (leading "-",
//     the bare "-" included) left in a positional slot is a typed
//     *UsageError with nothing run.
//   - Interspersed — the docgate-shaped grammar: flags may appear anywhere
//     between positionals (`contract-gate <feature> --trigger x` is legal
//     there); unknown flags and unconsumed flag-like leftovers still refuse.
//   - CutArgv — the pass-through split: argv destined for a child process
//     is cut out at the FIRST "--" before any parsing, so those bytes are
//     never inspected, parsed, or rewritten (DINV-5: argv-verbatim).
//
// Refusals are *UsageError values — exit-2 material under the house
// convention — printed exactly once by the caller: the guard silences the
// FlagSet's own output so a refusal is never double-printed. flag.ErrHelp
// stays reachable through errors.Is for callers that want -h to render
// full usage.
//
// Adoption: cmd/pyxis consumes Positionals + CutArgv (plan installer/001
// T4). cmd/plangate (strict) and cmd/docgate (interspersed) still carry
// their original inline copies of this discipline; migrating them onto
// this package is deliberately outside T4's scope and remains a follow-up.
package cliguard

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// UsageError is the typed misparse refusal: the verb whose grammar was
// violated and the named reason, with nothing run. Mains print it once and
// exit 2 (the usage convention).
type UsageError struct {
	Verb   string // the FlagSet's name — the verb whose grammar was violated
	Reason string // the named refusal
	Err    error  // the underlying flag error when one exists (flag.ErrHelp is meaningful to callers); nil for the guard's own refusals
}

func (e *UsageError) Error() string {
	if e.Verb == "" {
		return e.Reason
	}
	return e.Verb + ": " + e.Reason
}

func (e *UsageError) Unwrap() error { return e.Err }

// Positionals parses args against fs under the strict grammar — flags
// precede positionals — and returns the positionals. It refuses fail-closed
// with a *UsageError when fs.Parse errors (unknown flag, malformed value,
// -h/-help) or when any remaining token is flag-like: parsing stops at the
// first positional, so a flag after one would otherwise be silently
// demoted to data (I3). fs must use flag.ContinueOnError; its own output
// is silenced so the caller prints the refusal exactly once.
func Positionals(fs *flag.FlagSet, args []string) ([]string, error) {
	silence(fs)
	if err := fs.Parse(args); err != nil {
		return nil, &UsageError{Verb: fs.Name(), Reason: err.Error(), Err: err}
	}
	return sweep(fs.Name(), fs.Args(), "flags precede positionals")
}

// Interspersed parses args against fs allowing flags anywhere between
// positionals — docgate's documented contract — by re-parsing after each
// positional. Unknown flags and malformed values refuse out of fs.Parse;
// a flag-like token that no parse pass could consume (e.g. one stranded
// behind a "--" terminator) refuses out of the sweep. Same FlagSet
// requirements as Positionals.
func Interspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	silence(fs)
	var pos []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, &UsageError{Verb: fs.Name(), Reason: err.Error(), Err: err}
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		pos = append(pos, rest[0])
		rest = rest[1:]
	}
	return sweep(fs.Name(), pos, "not consumable as a flag or a flag value")
}

// CutArgv splits args at the FIRST "--": own is the caller's to parse
// fail-closed, argv passes to the child process verbatim — never parsed,
// never interpreted, bytes through (DINV-5); a second "--" stays inside
// argv untouched. found reports whether the separator was present at all,
// so grammars that REQUIRE it (pyxis exec) can refuse its absence by name.
func CutArgv(args []string) (own, argv []string, found bool) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:], true
		}
	}
	return args, nil, false
}

// sweep refuses any flag-like token left in a positional slot — the I3
// refusal itself. hint names the grammar that was violated.
func sweep(verb string, pos []string, hint string) ([]string, error) {
	for _, a := range pos {
		if strings.HasPrefix(a, "-") {
			return nil, &UsageError{Verb: verb, Reason: fmt.Sprintf("flag-like token %q in a positional slot — %s", a, hint)}
		}
	}
	return pos, nil
}

// silence routes the FlagSet's own printing to io.Discard so the typed
// refusal is the single surface, and pins the one FlagSet mode the guard
// can work with: with ExitOnError or PanicOnError, fs.Parse never returns
// a refusable error — the process would exit silently (output discarded)
// before the guard could fail closed. That is a programmer error, so it
// panics rather than degrades.
func silence(fs *flag.FlagSet) {
	if fs.ErrorHandling() != flag.ContinueOnError {
		panic("cliguard: FlagSet must use flag.ContinueOnError — ExitOnError/PanicOnError would bypass the typed refusal and exit silently")
	}
	fs.SetOutput(io.Discard)
}
