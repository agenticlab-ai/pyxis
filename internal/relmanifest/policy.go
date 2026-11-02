package relmanifest

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// ClassRule is one authored glob→class row. Globs are slash-separated and
// relative to the payload root; a `**` segment matches zero or more whole
// segments, every other segment matches its counterpart via path.Match, so
// `*` and `?` never cross a `/`.
type ClassRule struct {
	Glob  string
	Class Class
}

// ClassPolicy is the parsed class-policy/v1 document: an ORDERED rule list —
// first match wins, so specific rows precede general catch-alls.
type ClassPolicy struct {
	SchemaVersion string
	Classes       []ClassRule
}

// ParseClassPolicy parses class-policy/v1 bytes. The grammar is a CLOSED
// hand-rolled YAML subset (the internal/docgate/config approach — stdlib
// only): the schema_version scalar, the classes list of single-line
// `- { glob: …, class: … }` flow maps at two-space indent, comments, and
// blank lines. Unknown keys, unknown classes, malformed globs, duplicate
// keys, or any richer YAML are named defects; defects accumulate so one
// parse names every problem.
func ParseClassPolicy(b []byte) (ClassPolicy, error) {
	var p ClassPolicy
	var errs []error
	add := func(n int, err error) { errs = append(errs, fmt.Errorf("line %d: %w", n, err)) }
	seen := map[string]bool{}
	section := ""
	for i, raw := range strings.Split(normalizeNewlines(b), "\n") {
		n := i + 1
		line := strings.TrimRight(raw, " \t")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch {
		case indent == 0:
			section = ""
			key, rest, found := strings.Cut(t, ":")
			key = strings.TrimSpace(key)
			if !found || key == "" || strings.ContainsAny(key, " \t\"{") {
				add(n, fmt.Errorf("unrecognized structure %q: class-policy/v1 is one top-level scalar plus one list (closed subset)", t))
				continue
			}
			if seen[key] {
				add(n, fmt.Errorf("duplicate top-level key %q", key))
				continue
			}
			seen[key] = true
			switch key {
			case "schema_version":
				v, err := scalarValue(rest)
				if err != nil {
					add(n, err)
					continue
				}
				p.SchemaVersion = v
			case "classes":
				if v, err := scalarValue(rest); err != nil || v != "" {
					add(n, errors.New("classes takes no inline value; entries are `- { glob: …, class: … }` lines"))
					continue
				}
				section = "classes"
			default:
				add(n, fmt.Errorf("unknown top-level key %q: class-policy/v1 carries exactly schema_version and classes (closed parse)", key))
			}
		case indent == 2 && strings.HasPrefix(t, "- "):
			if section != "classes" {
				add(n, errors.New("list item outside classes"))
				continue
			}
			rule, err := parseClassRule(strings.TrimSpace(strings.TrimPrefix(t, "- ")))
			if err != nil {
				add(n, err)
				continue
			}
			p.Classes = append(p.Classes, rule)
		default:
			add(n, fmt.Errorf("unrecognized structure %q: flat top-level keys and two-space-indented `- { … }` entries only (closed subset)", t))
		}
	}
	switch {
	case !seen["schema_version"]:
		errs = append(errs, errors.New("missing schema_version"))
	case p.SchemaVersion != PolicySchemaVersion:
		errs = append(errs, fmt.Errorf("schema_version %q: this parser speaks exactly %q (closed parse)", p.SchemaVersion, PolicySchemaVersion))
	}
	if !seen["classes"] {
		errs = append(errs, errors.New("missing classes"))
	}
	if len(errs) > 0 {
		return ClassPolicy{}, fmt.Errorf("class-policy: %w", errors.Join(errs...))
	}
	return p, nil
}

// ClassOf returns the first-match-wins ownership class for a slash-separated
// payload-relative path; ok is false when no glob matches — the DINV-6
// refusal belongs to the caller, which knows the file set. The error leg
// exists only for hand-built policies carrying a malformed glob; a parsed
// policy's globs are vetted at parse time.
func (p ClassPolicy) ClassOf(name string) (Class, bool, error) {
	for _, r := range p.Classes {
		ok, err := matchGlob(r.Glob, name)
		if err != nil {
			return "", false, fmt.Errorf("class-policy glob %q: %w", r.Glob, err)
		}
		if ok {
			return r.Class, true, nil
		}
	}
	return "", false, nil
}

// parseClassRule parses one `{ glob: …, class: … }` flow map: single line,
// double-quote aware, optional trailing comment. Exactly the two keys, both
// required; anything else is a named defect.
func parseClassRule(s string) (ClassRule, error) {
	if !strings.HasPrefix(s, "{") {
		return ClassRule{}, fmt.Errorf("class entry %q is not a `{ glob: …, class: … }` flow map", s)
	}
	end := closingBrace(s)
	if end < 0 {
		return ClassRule{}, fmt.Errorf("unterminated flow map %q", s)
	}
	if tail := strings.TrimSpace(s[end+1:]); tail != "" && !strings.HasPrefix(tail, "#") {
		return ClassRule{}, fmt.Errorf("trailing content %q after class entry", tail)
	}
	var rule ClassRule
	seen := map[string]bool{}
	for _, pair := range splitOutsideQuotes(s[1:end], ',') {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			return ClassRule{}, errors.New("empty pair in class entry")
		}
		key, rest, found := strings.Cut(pair, ":")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return ClassRule{}, fmt.Errorf("pair %q is not `key: value`", pair)
		}
		if seen[key] {
			return ClassRule{}, fmt.Errorf("duplicate key %q in class entry", key)
		}
		seen[key] = true
		v, err := scalarValue(rest)
		if err != nil {
			return ClassRule{}, fmt.Errorf("key %q: %w", key, err)
		}
		switch key {
		case "glob":
			rule.Glob = v
		case "class":
			c, err := parseClass(v)
			if err != nil {
				return ClassRule{}, err
			}
			rule.Class = c
		default:
			return ClassRule{}, fmt.Errorf("unknown key %q in class entry: exactly glob and class (closed parse)", key)
		}
	}
	if !seen["glob"] || rule.Glob == "" {
		return ClassRule{}, errors.New("class entry missing glob")
	}
	if !seen["class"] {
		return ClassRule{}, fmt.Errorf("class entry %q missing class", rule.Glob)
	}
	if err := checkGlob(rule.Glob); err != nil {
		return ClassRule{}, fmt.Errorf("glob %q: %w", rule.Glob, err)
	}
	return rule, nil
}

// scalarValue extracts one scalar token: optionally double-quoted, with an
// optional trailing ` #` comment; a bare comment is an absent value. Escape
// sequences are outside the closed subset.
func scalarValue(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "#") {
		return "", nil
	}
	if strings.HasPrefix(s, `"`) {
		rest := s[1:]
		end := strings.Index(rest, `"`)
		if end < 0 {
			return "", fmt.Errorf("unterminated quote in %q", s)
		}
		if tail := strings.TrimSpace(rest[end+1:]); tail != "" && !strings.HasPrefix(tail, "#") {
			return "", fmt.Errorf("trailing content %q after quoted value", tail)
		}
		return rest[:end], nil
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s), nil
}

// closingBrace returns the index of the '}' closing the flow map opened at
// s[0], honoring double quotes; -1 when unterminated.
func closingBrace(s string) int {
	inQuote := false
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '}':
			if !inQuote {
				return i
			}
		}
	}
	return -1
}

// splitOutsideQuotes splits s on sep occurrences outside double quotes.
func splitOutsideQuotes(s string, sep byte) []string {
	var parts []string
	inQuote, start := false, 0
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			inQuote = !inQuote
		case s[i] == sep && !inQuote:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// checkGlob vets an authored glob at parse time: non-empty, relative,
// slash-separated with no empty segments, and every non-`**` segment a valid
// path.Match pattern — so ClassOf can never fail on a parsed policy.
func checkGlob(g string) error {
	if strings.HasPrefix(g, "/") {
		return errors.New("absolute pattern: payload paths are relative to the payload root")
	}
	for _, seg := range strings.Split(g, "/") {
		if seg == "" {
			return errors.New("empty path segment")
		}
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, "probe"); err != nil {
			return fmt.Errorf("bad pattern segment %q: %w", seg, err)
		}
	}
	return nil
}

// matchGlob reports whether a slash-separated payload path matches a policy
// glob under the doublestar-style semantics ClassRule documents.
func matchGlob(pattern, name string) (bool, error) {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

// matchSegments matches pattern segments against path segments: a `**`
// segment consumes zero or more whole segments; any other segment must
// path.Match its counterpart.
func matchSegments(pat, name []string) (bool, error) {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for skip := 0; skip <= len(name); skip++ {
				ok, err := matchSegments(pat[1:], name[skip:])
				if ok || err != nil {
					return ok, err
				}
			}
			return false, nil
		}
		if len(name) == 0 {
			return false, nil
		}
		ok, err := path.Match(pat[0], name[0])
		if err != nil {
			return false, fmt.Errorf("pattern segment %q: %w", pat[0], err)
		}
		if !ok {
			return false, nil
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0, nil
}

// normalizeNewlines maps CRLF/CR to LF so line numbers stay stable across
// checkouts.
func normalizeNewlines(b []byte) string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}
