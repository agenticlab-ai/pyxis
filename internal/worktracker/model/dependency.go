package model

import (
	"strconv"
	"strings"
)

// A blocked_by reference is one of two forms (docs/tech-design/
// local-work-tracker.md §3.5, Q5): a local item id — a positive integer — or
// the cross-repo form owner/repo#id, which v1 stores and displays opaquely
// (never resolved; resolution is deferred to WR-4). Anything else is malformed
// and a named defect at add time (prd:local-work-tracker/UB-2). The grammar is
// medium-neutral so the CLI and the engine classify a reference identically.

// RefKind classifies a blocked_by reference.
type RefKind int

const (
	// RefMalformed is neither a local id nor the cross-repo form.
	RefMalformed RefKind = iota
	// RefLocal is a positive local item id.
	RefLocal
	// RefCrossRepo is the opaque owner/repo#id cross-repo form.
	RefCrossRepo
)

// ClassifyRef classifies a blocked_by reference and, for a local id, returns
// the parsed identifier (0 for the other kinds). The input is trimmed first, so
// surrounding whitespace never changes the classification.
func ClassifyRef(ref string) (RefKind, int) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return RefMalformed, 0
	}
	if n, err := strconv.Atoi(ref); err == nil {
		if n > 0 {
			return RefLocal, n
		}
		return RefMalformed, 0
	}
	if isCrossRepoRef(ref) {
		return RefCrossRepo, 0
	}
	return RefMalformed, 0
}

// CanonicalRef returns the normalized stored form of a reference — the decimal
// id for a local ref, the trimmed string for a cross-repo ref — so idempotency
// and the reverse scan compare exact bytes. A malformed ref canonicalizes to
// the empty string; callers refuse it before storing.
func CanonicalRef(ref string) (string, RefKind, int) {
	kind, id := ClassifyRef(ref)
	switch kind {
	case RefLocal:
		return strconv.Itoa(id), kind, id
	case RefCrossRepo:
		return strings.TrimSpace(ref), kind, 0
	default:
		return "", kind, 0
	}
}

// isCrossRepoRef reports whether ref is the owner/repo#id cross-repo form:
// exactly one '/', a single '#', non-empty owner and repo, and a positive
// integer tail. It carries no whitespace (a stored reference is a single token).
func isCrossRepoRef(ref string) bool {
	if strings.ContainsAny(ref, " \t\n") {
		return false
	}
	path, num, ok := strings.Cut(ref, "#")
	if !ok {
		return false
	}
	owner, repo, ok := strings.Cut(path, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return false
	}
	n, err := strconv.Atoi(num)
	return err == nil && n > 0
}
