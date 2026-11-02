// The GitHub coordinate's shared parsing and grammar (design
// init-hardening-v1 §3.7; plan init-hardening/001 T3, T5): ONE evaluator
// (AP-11) for turning a git remote URL into an owner/name slug and for
// validating a slug's shape before it may reach gh/git argv or a URL
// (ABUSE-1's threat sketch — "argv/URL smuggling via the coordinate"). T3's
// preflight GitHub-mode check resolves and validates the coordinate at
// check time; T5's interactive elicitation re-asks against the same grammar
// rather than a second, driftable copy.
package wire

import "strings"

// ParseCoordinate extracts the owner/name coordinate from a git remote URL,
// supporting the two shapes gh and git itself both use for github.com:
//
//	https://github.com/owner/name(.git)
//	git@github.com:owner/name(.git)
//
// ok is false when url does not name a github.com repository (a different
// host, or a URL too malformed to carry two non-empty path segments).
func ParseCoordinate(url string) (slug string, ok bool) {
	url = strings.TrimSuffix(strings.TrimSpace(url), ".git")
	i := strings.Index(url, "github.com")
	if i < 0 {
		return "", false
	}
	rest := strings.TrimLeft(url[i+len("github.com"):], ":/")
	owner, name, found := strings.Cut(rest, "/")
	if !found || owner == "" || name == "" {
		return "", false
	}
	return owner + "/" + name, true
}

// ValidCoordinate reports whether slug is a well-formed owner/name GitHub
// coordinate — the strict grammar ABUSE-1 requires before any value reaches
// gh/git argv or a URL: exactly one '/' splitting two non-empty sides, each
// drawn from GitHub's own handle/repo character set (letters, digits, '-',
// '_', '.'), and neither side starting with '-' (which an argv parser could
// mistake for a flag rather than a positional value). Everything else is
// refused by the caller — a check that names it, or an interactive re-ask —
// never silently coerced or passed through.
func ValidCoordinate(slug string) bool {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || strings.Contains(name, "/") {
		return false // not exactly one '/'
	}
	return validCoordinatePart(owner) && validCoordinatePart(name)
}

// validCoordinatePart validates one side (owner or name) of the coordinate.
func validCoordinatePart(s string) bool {
	if s == "" || s[0] == '-' {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

// RemoteURL renders the https://github.com/owner/name.git remote URL for a
// validated coordinate — the ONE construction site (AP-11; plan
// init-hardening/001 T5) for the shape this package's own ParseCoordinate
// recognizes, so a materialized git remote (internal/installer's
// stageRemote) and this package's parsing of it back can never drift.
func RemoteURL(slug string) string {
	return "https://github.com/" + slug + ".git"
}
