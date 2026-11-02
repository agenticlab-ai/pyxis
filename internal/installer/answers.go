package installer

import (
	"fmt"
	"os"
	"strings"
)

// LoadAnswersFile parses an init answers file (installer-v1 §3.5; FR-4): the
// closed YAML subset carrying the five knob keys plus the optional UB-10
// identity halves. It reuses the repo config's yaml-subset spelling so an
// operator writes knobs the same way the config shows them — top-level
// scalars, the wiring flow map, and the cli_surfaces flow list. The grammar
// is CLOSED: an unknown key, malformed structure, or bad enum is a named
// error and nothing is folded (UB-6-style fail-closed).
//
// Recognized keys (all optional; an absent key stays unset and takes its
// recommended default at resolution):
//
//	tracker: github|carina
//	ci_mode: local|actions|both
//	wiring: { repo: use|create, protection: none|required-checks|checks+review }
//	cli_surfaces: [claude, codex]      # or: auto
//	argus: true|false
//	git_name: "Ada Lovelace"           # UB-10 identity half
//	git_email: "ada@example.com"       # UB-10 identity half
//	coordinate: "octo/widget"          # promptless GitHub coordinate (T3, FR-7);
//	                                   # only consulted when no remote is configured
//	remote_url: "/path/to/local.git"   # initial publication destination
func LoadAnswersFile(path string) (Answers, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Answers{}, fmt.Errorf("init: read answers file %s: %w", path, err)
	}
	return ParseAnswers(b)
}

// ParseAnswers parses the answers-file bytes under the closed subset. Split
// from LoadAnswersFile so tests exercise the grammar without a file.
func ParseAnswers(b []byte) (Answers, error) {
	var ans Answers
	for i, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimRight(raw, " \t")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if len(line) != len(t) {
			return Answers{}, answersErr(i, "unexpected indentation — the answers file is a flat set of top-level keys")
		}
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			return Answers{}, answersErr(i, fmt.Sprintf("%q is not a `key: value` line", t))
		}
		key := strings.TrimSpace(k)
		val := answersUnquote(v)
		switch key {
		case "tracker":
			ans.Tracker = val
		case "ci_mode":
			ans.CIMode = val
		case "wiring":
			repo, prot, werr := parseWiringFlow(val)
			if werr != nil {
				return Answers{}, answersErr(i, werr.Error())
			}
			ans.WiringRepo, ans.WiringProtection = repo, prot
		case "cli_surfaces":
			surfaces, serr := parseSurfacesFlow(val)
			if serr != nil {
				return Answers{}, answersErr(i, serr.Error())
			}
			ans.CLISurfaces = surfaces
		case "argus":
			switch val {
			case "true":
				b := true
				ans.Argus = &b
			case "false":
				b := false
				ans.Argus = &b
			default:
				return Answers{}, answersErr(i, fmt.Sprintf("argus %q is not a boolean (write true or false)", val))
			}
		case "git_name":
			ans.GitName = val
		case "git_email":
			ans.GitEmail = val
		case "coordinate":
			ans.Coordinate = val
		case "remote_url":
			ans.RemoteURL = val
		default:
			return Answers{}, answersErr(i, fmt.Sprintf("unknown key %q — the answers file is closed: tracker, ci_mode, wiring, cli_surfaces, argus, git_name, git_email, coordinate, remote_url", key))
		}
	}
	return ans, nil
}

// parseWiringFlow parses the `{ repo: …, protection: … }` flow map exactly as
// the config shape spells it. Enums are not validated here — resolveKnobs
// validates every value against the closed set uniformly — but the structure
// is closed (closed keys, block style rejected).
func parseWiringFlow(v string) (repo, protection string, err error) {
	if !strings.HasPrefix(v, "{") || !strings.HasSuffix(v, "}") {
		return "", "", fmt.Errorf("wiring %q is not a `{ repo: …, protection: … }` flow map", v)
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return "", "", nil
	}
	for part := range strings.SplitSeq(inner, ",") {
		pk, pv, ok := strings.Cut(part, ":")
		if !ok {
			return "", "", fmt.Errorf("wiring entry %q is not `key: value`", strings.TrimSpace(part))
		}
		pk, pv = strings.TrimSpace(pk), strings.Trim(strings.TrimSpace(pv), `"`)
		switch pk {
		case "repo":
			repo = pv
		case "protection":
			protection = pv
		default:
			return "", "", fmt.Errorf("unknown wiring key %q (repo and protection only)", pk)
		}
	}
	return repo, protection, nil
}

// parseSurfacesFlow parses either the `[a, b]` flow list or the bare word
// "auto" (the detect sentinel). Items are free strings — the value set is open
// — so only the structure is checked.
func parseSurfacesFlow(v string) ([]string, error) {
	if v == surfaceAuto {
		return []string{surfaceAuto}, nil
	}
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, fmt.Errorf("cli_surfaces %q is not a `[a, b]` flow list or the word auto", v)
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return []string{}, nil // an explicit empty set — legal (no surface wired)
	}
	var items []string
	for part := range strings.SplitSeq(inner, ",") {
		it := strings.Trim(strings.TrimSpace(part), `"`)
		if it == "" {
			return nil, fmt.Errorf("cli_surfaces carries an empty item")
		}
		items = append(items, it)
	}
	return items, nil
}

// ParseWiringArg parses the --wiring FLAG value: a `repo=…,protection=…` k=v
// CSV (the flag surface's spelling, distinct from the answers file's flow-map
// form). Enums are validated later by resolveKnobs; only the structure and
// keys are closed here. An unset key stays "" (unset → recommended default).
func ParseWiringArg(v string) (repo, protection string, err error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", "", nil
	}
	for part := range strings.SplitSeq(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pk, pv, ok := strings.Cut(part, "=")
		if !ok {
			return "", "", fmt.Errorf("--wiring entry %q is not key=value (want repo=…,protection=…)", part)
		}
		pk, pv = strings.TrimSpace(pk), strings.TrimSpace(pv)
		switch pk {
		case "repo":
			repo = pv
		case "protection":
			protection = pv
		default:
			return "", "", fmt.Errorf("--wiring: unknown key %q (repo and protection only)", pk)
		}
	}
	return repo, protection, nil
}

// SplitSurfacesArg parses the --cli-surfaces FLAG value: a comma-separated
// list, or the bare word auto (kept as the ["auto"] detect sentinel that
// resolveSurfaces expands). An empty flag value yields a nil set (unset →
// detect).
func SplitSurfacesArg(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if v == surfaceAuto {
		return []string{surfaceAuto}
	}
	return splitCSV(v)
}

// answersUnquote trims surrounding quotes and a trailing inline comment,
// matching the config parser's scalar handling.
func answersUnquote(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"`)
}

func answersErr(line int, msg string) error {
	return fmt.Errorf("init: answers file line %d: %s", line+1, msg)
}
