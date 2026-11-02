// Package codegatemirror is the single-source code gate: it reads a repo's
// code-gate check list from the pyxis config surface and runs each check
// through the shared internal/verify runner, emitting one aggregate verdict.
// It is the mechanism docs/tech-design/code-gate-mirror.md decides — one list
// run by every surface (the executor gate, the pre-push hook, required CI) so
// the local gate mirrors required CI by construction (DINV-1), holding only
// hermetic correctness checks (DINV-2) with required-vs-advisory gating
// (§3.10), and failing closed on a malformed config (DINV-4). It is distinct
// from the existing `codegate` verification-ladder harness and never runs the
// doc-gate (docgate) (DINV-5).
package codegatemirror

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
)

// FailOn selects how a check's outcome is read from its process result.
type FailOn string

const (
	// FailOnExit (the default) fails the check on a non-zero exit.
	FailOnExit FailOn = "exit"
	// FailOnOutput fails the check when it names offenders on stdout even on a
	// zero exit — the `gofmt -l .` shape, whose list mode exits 0 while listing
	// non-conforming files.
	FailOnOutput FailOn = "output"
)

// Class is a check's gating class.
type Class string

const (
	// ClassRequired (the default) blocks: a required check's failure fails the
	// gate, and the required set is what mirrors CI (DINV-1).
	ClassRequired Class = "required"
	// ClassAdvisory never blocks: it is run and reported, but its failure never
	// fails the gate — the nice-to-have bucket, outside the mirror (§3.10).
	ClassAdvisory Class = "advisory"
)

// Check is one code-gate check: a named command with its failure semantics,
// gating class, optional wall-clock bound, and any extra environment names it
// needs passed through (beyond the always-present PATH/HOME).
type Check struct {
	Name     string
	Command  string
	Group    string // empty uses DefaultGroup; groups run concurrently
	Phase    int    // empty/0 uses phase 1; later phases start only after earlier phases are green
	FailOn   FailOn
	Class    Class
	TimeoutS int
	Env      []string
}

// Config is the parsed code-gate block. File order is preserved within each
// execution group and in the report; independent groups run concurrently
// inside one phase. A later phase starts only after every required check in
// earlier phases passed.
type Config struct {
	Checks []Check
}

var (
	// ErrMalformed is a structural defect: bad indentation, a non-mapping line,
	// a duplicate block, or an out-of-range field value.
	ErrMalformed = errors.New("code-gate config: malformed")
	// ErrUnknownKey is a closed-schema violation: a field not in the check
	// schema (AP-6).
	ErrUnknownKey = errors.New("code-gate config: unknown key")
	// ErrInvalidCheck is a semantic defect: a check with no command, or a
	// duplicate check name.
	ErrInvalidCheck = errors.New("code-gate config: invalid check")
)

// Load merges root/.pyxis/config.local.yaml over root/.pyxis/config.yaml and
// parses its effective code-gate block. Missing files or an absent block yield
// an empty Config (no checks), exactly as the executor gate does.
func Load(root string) (Config, error) {
	data, err := configlayer.Load(root)
	if err != nil {
		return Config{}, fmt.Errorf("code-gate: load configuration: %w", err)
	}
	return Parse(data)
}

// Parse reads the `code-gate:` block from a pyxis-config document. It owns only
// that subtree — every sibling top-level block belongs to another consumer of
// the shared surface and is skipped, never rejected — and parses CLOSED within
// it: an unknown field, a malformed line, or a check missing a command is a
// defect (AP-6, DINV-4). An absent block is an empty Config.
//
// Grammar (two-space steps): `code-gate:` at column 0, each check name at
// indent 2 opening a block, and its fields at indent 4 —
//
//	code-gate:
//	  <check-name>:
//	    command: <raw command, run directly — no shell>
//	    fail-on: exit | output      # default exit
//	    class: required | advisory   # default required
//	    timeout-s: <int>             # optional
//	    env: FOO BAR                 # optional, space-separated pass-throughs
//	    group: <name>                # optional, default "default"
//	    phase: <int >= 1>            # optional, default 1; later phases wait
func Parse(cfg []byte) (Config, error) {
	var out Config
	inBlock, seenBlock := false, false
	var cur *Check
	names := map[string]bool{}

	// finish validates the check under construction and appends it. It runs at
	// every check boundary (a new name, the block's end, or EOF).
	finish := func(lineNo int) error {
		if cur == nil {
			return nil
		}
		if strings.TrimSpace(cur.Command) == "" {
			return fmt.Errorf("%w: line %d: check %q has no command", ErrInvalidCheck, lineNo, cur.Name)
		}
		out.Checks = append(out.Checks, *cur)
		cur = nil
		return nil
	}

	lines := strings.Split(strings.ReplaceAll(string(cfg), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.ContainsRune(lead, '\t') {
			return Config{}, fmt.Errorf("%w: line %d: tab in indentation (use spaces)", ErrMalformed, lineNo)
		}
		indent := len(lead)

		// Top-level key: only `code-gate:` is ours; a sibling belongs to another
		// consumer of the shared surface and its whole subtree is skipped.
		if indent == 0 {
			if err := finish(lineNo); err != nil {
				return Config{}, err
			}
			key, val, ok := splitKV(trimmed)
			if !ok {
				return Config{}, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformed, lineNo, trimmed)
			}
			inBlock = false
			if key != "code-gate" {
				continue
			}
			if seenBlock {
				return Config{}, fmt.Errorf("%w: line %d: duplicate code-gate block", ErrMalformed, lineNo)
			}
			if val != "" {
				return Config{}, fmt.Errorf("%w: line %d: code-gate is a block, not a scalar", ErrMalformed, lineNo)
			}
			inBlock, seenBlock = true, true
			continue
		}

		// Indented lines matter only inside our block; anything under a foreign
		// top-level block is skipped before any structural check.
		if !inBlock {
			continue
		}
		if indent%2 != 0 || indent > 4 {
			return Config{}, fmt.Errorf("%w: line %d: unexpected indentation %d (two-space steps, max 4)", ErrMalformed, lineNo, indent)
		}
		key, val, ok := splitKV(trimmed)
		if !ok {
			return Config{}, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformed, lineNo, trimmed)
		}

		switch indent {
		case 2: // a check name — opens its block
			if err := finish(lineNo); err != nil {
				return Config{}, err
			}
			if key == "" {
				return Config{}, fmt.Errorf("%w: line %d: empty check name", ErrMalformed, lineNo)
			}
			if val != "" {
				return Config{}, fmt.Errorf("%w: line %d: check %q is a block, not a scalar (put its command under `command:`)", ErrMalformed, lineNo, key)
			}
			if names[key] {
				return Config{}, fmt.Errorf("%w: line %d: duplicate check %q", ErrInvalidCheck, lineNo, key)
			}
			names[key] = true
			cur = &Check{Name: key, FailOn: FailOnExit, Class: ClassRequired}

		case 4: // a field of the current check
			if cur == nil {
				return Config{}, fmt.Errorf("%w: line %d: %q is only valid under a check", ErrMalformed, lineNo, key)
			}
			if err := setField(cur, key, val, lineNo); err != nil {
				return Config{}, err
			}
		}
	}

	if err := finish(len(lines)); err != nil {
		return Config{}, err
	}
	return out, nil
}

// setField applies one indent-4 field to the check under construction, closed
// against the check schema.
func setField(c *Check, key, val string, lineNo int) error {
	switch key {
	case "command":
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("%w: line %d: check %q has an empty command", ErrInvalidCheck, lineNo, c.Name)
		}
		c.Command = val
	case "fail-on":
		switch FailOn(val) {
		case FailOnExit, FailOnOutput:
			c.FailOn = FailOn(val)
		default:
			return fmt.Errorf("%w: line %d: fail-on %q (want exit or output)", ErrMalformed, lineNo, val)
		}
	case "class":
		switch Class(val) {
		case ClassRequired, ClassAdvisory:
			c.Class = Class(val)
		default:
			return fmt.Errorf("%w: line %d: class %q (want required or advisory)", ErrMalformed, lineNo, val)
		}
	case "timeout-s":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return fmt.Errorf("%w: line %d: timeout-s %q (want a non-negative integer)", ErrMalformed, lineNo, val)
		}
		c.TimeoutS = n
	case "env":
		c.Env = strings.Fields(val)
	case "group":
		if !validGroupName(val) {
			return fmt.Errorf("%w: line %d: group %q must be a literal name (letters, digits, hyphens, underscores; starting with a letter)", ErrMalformed, lineNo, val)
		}
		c.Group = val
	case "phase":
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 {
			return fmt.Errorf("%w: line %d: phase %q (want an integer >= 1)", ErrMalformed, lineNo, val)
		}
		c.Phase = n
	default:
		return fmt.Errorf("%w: line %d: %q under check %q (expected command, fail-on, class, timeout-s, env, group, or phase)", ErrUnknownKey, lineNo, key, c.Name)
	}
	return nil
}

// splitKV splits a `key: value` line on its first colon, trimming both sides.
func splitKV(s string) (key, val string, ok bool) {
	key, val, ok = strings.Cut(s, ":")
	if !ok {
		return "", "", false
	}
	return strings.TrimSpace(key), strings.TrimSpace(val), true
}
