package watch

// The status verb's staleness-threshold config (design:wake-watch-v2 §3.8;
// plan self-drive-wake/001 T9): closed-parse `wake.stale.{review,ci,worker,
// checker}` duration keys on the shared pyxis-config surface
// (.pyxis/config.yaml). This package owns exactly the `wake:` subtree —
// mirroring internal/executor/config's convention for the same shared file —
// so every other top-level block (paths:, plan:, executor:, channel:, …) is
// simply skipped, never validated. An absent file, an absent wake: block, or
// an absent stale: subsection all yield the ruled defaults (D-5: "an absent
// file or section means the defaults"); any other key under wake: at any
// depth is a named, refusing error (AP-8 — config parses closed).
//
// The predicate consuming these bounds is the one shared implementation in
// internal/wake (DINV-7) — this file contributes only the parameter set and
// the wait-kind → bound mapping (forKind), never a second "is this stale?"
// computation.

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/wake"
)

// configRelPath is the shared pyxis-config surface, resolved under the repo
// root — the same path docgate's own parser reads (internal/docgate/config).
const configRelPath = configlayer.GlobalPath

// Sentinel errors — part of the package contract, matched with errors.Is.
var (
	// ErrUnknownThresholdKey is a closed-schema violation: a key under wake:
	// that is not part of the fixed review/ci/worker/checker grammar (AP-8).
	ErrUnknownThresholdKey = errors.New("watch: unknown key under wake:")
	// ErrInvalidThreshold is a wake.stale.* value that is not a positive Go
	// duration (e.g. "2h", "30m").
	ErrInvalidThreshold = errors.New("watch: invalid wake.stale duration")
	// ErrMalformedThresholds is a structural defect: bad indentation, a tab
	// in leading whitespace, or a line that is neither a mapping key nor a
	// value the grammar expects.
	ErrMalformedThresholds = errors.New("watch: malformed wake: block")
)

// The ruled defaults (design:wake-watch-v2 §3.8; the authority's own §0
// sanity-check row: "threshold defaults: review 2 h · CI 30 m · worker 2 h").
// Checker has no fixed-duration default — it defaults to 3x the adapter's
// advised cadence, resolved by CheckerBound once a reader is chosen.
const (
	defaultReviewStale         = 2 * time.Hour
	defaultCIStale             = 30 * time.Minute
	defaultWorkerStale         = 2 * time.Hour
	defaultReviewRecoveryBound = 3
)

// Thresholds is the status verb's closed-parse staleness-threshold set. A
// zero-value Config field is never observed by a caller — Thresholds is
// always obtained from DefaultThresholds, LoadThresholds, or ParseThresholds.
type Thresholds struct {
	Review time.Duration
	CI     time.Duration
	Worker time.Duration
	// Checker is zero when unset (the config declared no override) — callers
	// resolve the effective bound through CheckerBound, never this field
	// directly, so "unset" can never be mistaken for "zero tolerance".
	Checker             time.Duration
	ReviewRecoveryBound int
	ReviewRecoveryRef   string
}

// DefaultThresholds are the ruled defaults a repo with no wake: block (or no
// .pyxis/config.yaml at all) runs under.
func DefaultThresholds() Thresholds {
	return Thresholds{Review: defaultReviewStale, CI: defaultCIStale, Worker: defaultWorkerStale, ReviewRecoveryBound: defaultReviewRecoveryBound, ReviewRecoveryRef: DefaultRecoveryEvidenceRef}
}

// CheckerBound resolves the checker's own defect-event staleness bound: the
// configured override when wake.stale.checker was set, else 3x the adapter-
// advised cadence (design:wake-watch-v2 §3.8's default) — computed here
// rather than at parse time because the cadence is only known once a medium
// adapter is chosen.
func (t Thresholds) CheckerBound(cadence time.Duration) time.Duration {
	if t.Checker > 0 {
		return t.Checker
	}
	return 3 * cadence
}

// forKind resolves the bound governing one wait kind. WaitNone — blocked,
// landed, or unknown rows carry no configured threshold — reads as zero,
// which callers never compare a real age against (see TaskStatus.Stale).
func (t Thresholds) forKind(k WaitKind) time.Duration {
	switch k {
	case WaitReview:
		return t.Review
	case WaitCI:
		return t.CI
	case WaitWorker:
		return t.Worker
	default:
		return 0
	}
}

// Validate enforces the bound-strictly-exceeds-cadence relation every
// staleness consumer shares (wake-channel-v1 §3.2, restated at wake-watch-v2
// §3.8: "the primitive relation inherited from wake-channel-v1 holds"): each
// of the four bounds — the checker's own resolved against cadence — must
// strictly exceed the reader's advised cadence, so an alarm can never fire on
// an await the very next tick would legally serve. A violation is a named,
// refusing error, never a silently-inverted alarm.
func (t Thresholds) Validate(cadence time.Duration) error {
	for _, kv := range []struct {
		key   string
		bound time.Duration
	}{
		{"wake.stale.review", t.Review},
		{"wake.stale.ci", t.CI},
		{"wake.stale.worker", t.Worker},
		{"wake.stale.checker", t.CheckerBound(cadence)},
	} {
		if err := wake.ValidateBound(kv.bound, cadence); err != nil {
			return fmt.Errorf("watch: %s: %w", kv.key, err)
		}
	}
	return nil
}

// LoadThresholds merges root/.pyxis/config.local.yaml over
// root/.pyxis/config.yaml and parses its effective wake: block. Absent files
// yield DefaultThresholds; an unreadable or malformed layer is an error.
func LoadThresholds(root string) (Thresholds, error) {
	b, err := configlayer.Load(root)
	if err != nil {
		return DefaultThresholds(), fmt.Errorf("watch: load configuration: %w", err)
	}
	return ParseThresholds(b)
}

// ParseThresholds validates the closed wake: grammar over cfg bytes, starting
// from DefaultThresholds so an omitted key keeps its default (partial
// override):
//
//	wake:
//	  review_recovery_bound: 3
//	  review_recovery_ref: refs/pyxis/review-recovery
//	  stale:
//	    review: <duration>
//	    ci: <duration>
//	    worker: <duration>
//	    checker: <duration>
//
// Every top-level key other than wake: is another consumer's block on the
// shared surface and is skipped wholesale, structural checks included — the
// same posture internal/executor/config takes for its own executor: block.
// Any key inside wake: outside this exact shape is ErrUnknownThresholdKey.
func ParseThresholds(cfg []byte) (Thresholds, error) {
	t := DefaultThresholds()

	const (
		secNone = iota
		secStale
	)
	inWake, seenWake := false, false
	section := secNone

	for i, raw := range strings.Split(strings.ReplaceAll(string(cfg), "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lineNo := i + 1

		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if strings.ContainsRune(lead, '\t') {
			return t, fmt.Errorf("%w: line %d: tab in indentation (use spaces)", ErrMalformedThresholds, lineNo)
		}
		indent := len(lead)

		// Top-level keys choose whether the following indented lines are
		// ours. Only wake: is; every sibling belongs to another consumer of
		// the shared surface and its whole subtree is skipped unchecked.
		if indent == 0 {
			key, val, ok := splitThresholdKV(trimmed)
			if !ok {
				return t, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformedThresholds, lineNo, trimmed)
			}
			inWake, section = false, secNone
			if key != "wake" {
				continue
			}
			if seenWake {
				return t, fmt.Errorf("%w: line %d: duplicate wake block", ErrMalformedThresholds, lineNo)
			}
			if val != "" {
				return t, fmt.Errorf("%w: line %d: wake is a block, not a scalar", ErrMalformedThresholds, lineNo)
			}
			inWake, seenWake = true, true
			continue
		}

		if !inWake {
			continue // indented under a foreign top-level block
		}
		if indent%2 != 0 || indent > 4 {
			return t, fmt.Errorf("%w: line %d: unexpected indentation %d (two-space steps, max 4)", ErrMalformedThresholds, lineNo, indent)
		}
		key, val, ok := splitThresholdKV(trimmed)
		if !ok {
			return t, fmt.Errorf("%w: line %d: %q is not a mapping key", ErrMalformedThresholds, lineNo, trimmed)
		}

		switch indent {
		case 2:
			if key == "review_recovery_ref" {
				if !safeRecoveryRef(val) {
					return t, fmt.Errorf("%w: line %d: wake.review_recovery_ref must be a safe refs/pyxis/* ref", ErrInvalidThreshold, lineNo)
				}
				t.ReviewRecoveryRef = val
				section = secNone
				continue
			}
			if key == "review_recovery_bound" {
				if val == "" || strings.Trim(val, "0123456789") != "" {
					return t, fmt.Errorf("%w: line %d: wake.review_recovery_bound = %q", ErrInvalidThreshold, lineNo, val)
				}
				var bound int
				if _, err := fmt.Sscanf(val, "%d", &bound); err != nil || bound < 1 || bound > 100 {
					return t, fmt.Errorf("%w: line %d: wake.review_recovery_bound = %q outside [1,100]", ErrInvalidThreshold, lineNo, val)
				}
				t.ReviewRecoveryBound = bound
				section = secNone
				continue
			}
			if key != "stale" {
				return t, fmt.Errorf("%w: line %d: %q under wake (expected stale, review_recovery_bound, or review_recovery_ref)", ErrUnknownThresholdKey, lineNo, key)
			}
			if val != "" {
				return t, fmt.Errorf("%w: line %d: stale is a block, not a scalar", ErrMalformedThresholds, lineNo)
			}
			section = secStale
		case 4:
			if section != secStale {
				return t, fmt.Errorf("%w: line %d: %q outside wake.stale", ErrUnknownThresholdKey, lineNo, key)
			}
			dst, known := map[string]*time.Duration{
				"review":  &t.Review,
				"ci":      &t.CI,
				"worker":  &t.Worker,
				"checker": &t.Checker,
			}[key]
			if !known {
				return t, fmt.Errorf("%w: line %d: %q under wake.stale (expected review, ci, worker, or checker)", ErrUnknownThresholdKey, lineNo, key)
			}
			d, err := time.ParseDuration(val)
			if err != nil || d <= 0 {
				return t, fmt.Errorf("%w: line %d: wake.stale.%s = %q", ErrInvalidThreshold, lineNo, key, val)
			}
			*dst = d
		}
	}
	return t, nil
}

func safeRecoveryRef(ref string) bool {
	if !strings.HasPrefix(ref, "refs/pyxis/") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") || strings.ContainsAny(ref, " \t\r\n~^:?*[\\") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for _, component := range strings.Split(ref, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

// splitThresholdKV splits "key: value" at the first colon. val is
// scalar-cleaned (inline comment and surrounding quotes stripped). ok is
// false when the line carries no colon at all.
func splitThresholdKV(s string) (key, val string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:i])
	val = strings.TrimSpace(s[i+1:])
	if j := strings.Index(val, " #"); j >= 0 {
		val = strings.TrimSpace(val[:j])
	}
	return key, strings.Trim(val, `"'`), true
}
