package drivetogreen

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// DefaultMaxLogBytes is the clamp applied to a redacted log before it is inlined
// into a fixer brief when Config.MaxLogBytes is unset. Failed-CI logs can be
// megabytes; a fixer needs the failing region, not the whole build. The clamp
// keeps the head (the invocation/context) and the tail (where the error usually
// lands), eliding the middle (§3.5, ABUSE-3).
const DefaultMaxLogBytes = 16 * 1024

// redactionMark replaces every secret-shaped run a Redactor scrubs.
const redactionMark = "[REDACTED]"

// builtinRedactions are the secret shapes scrubbed from every log before it
// reaches a fixer (ABUSE-3): a seeded secret in a build log must not ride the
// egress into an external agent CLI. The set is deliberately shape-based (known
// credential formats + secret-named assignments), not blanket high-entropy
// scrubbing, so genuinely useful context — commit SHAs, file paths, stack traces
// — survives for the fixer. Repo-specific shapes are added via
// Config.ExtraRedactions.
//
// Each pattern either matches a whole secret token (replaced wholesale) or
// captures a leading key/prefix in group 1 (preserved) with the secret value
// after it (replaced) — the two are distinguished by valueRedaction below.
var builtinRedactions = []secretPattern{
	// PEM private-key blocks (multiline), scrubbed whole.
	{re: regexp.MustCompile(`(?s)-----BEGIN[^-]*PRIVATE KEY-----.*?-----END[^-]*PRIVATE KEY-----`)},
	// GitHub tokens: classic (ghp_/gho_/ghu_/ghs_/ghr_) and fine-grained PATs.
	{re: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`)},
	{re: regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`)},
	// Slack tokens.
	{re: regexp.MustCompile(`xox[abprs]-[A-Za-z0-9-]{10,}`)},
	// AWS access-key IDs.
	{re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	// Google API keys.
	{re: regexp.MustCompile(`AIza[0-9A-Za-z_\-]{20,}`)},
	// user:password@host credentials embedded in a URL — scrub the password.
	{re: regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://[^:@/\s]+:)[^@/\s]+(@)`), value: true},
	// Secret-carrying query parameters (?token=… &api_key=…) — keep the key.
	{re: regexp.MustCompile(`(?i)([?&](?:access_token|api[_-]?key|token|secret|key)=)[^&\s]+`), value: true},
	// Authorization header — scrub the whole credential value (scheme + token) to
	// end of line, keeping only the header name. Runs before the assignment rule so
	// a "Bearer <jwt>" value is scrubbed whole, not just the scheme word.
	{re: regexp.MustCompile(`(?im)(authorization\s*:\s*).+$`), value: true},
	// Bearer tokens outside a header — keep the scheme, scrub the token.
	{re: regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=\-]+`), value: true},
	// Secret-named assignments (FOO_TOKEN=…, api-key: "…") — keep the name, scrub
	// the value. (Authorization is handled above.)
	{re: regexp.MustCompile(`(?i)([A-Z0-9_.\-]*(?:TOKEN|SECRET|PASSWORD|PASSWD|API[_-]?KEY|ACCESS[_-]?KEY|PRIVATE[_-]?KEY|CREDENTIAL)[A-Z0-9_.\-]*\s*[:=]\s*)("[^"]*"|'[^']*'|` + "`" + `[^` + "`" + `]*` + "`" + `|\S+)`), value: true},
}

// secretPattern is one redaction rule. When value is false the whole match is
// replaced by the mark; when value is true group 1 (a key/prefix) is preserved
// and the remainder replaced, so a fixer still learns WHICH variable leaked
// without seeing its value.
type secretPattern struct {
	re    *regexp.Regexp
	value bool
}

// Redactor scrubs secret-shaped substrings and clamps size, making a raw log safe
// to inline into a fixer brief (ABUSE-3, §3.5). It is deterministic and hermetic
// — a pure function of its input and its configured patterns, no clock, no
// network — so the redaction drill (a seeded secret must not reach the fixer) is a
// unit test.
type Redactor struct {
	maxBytes int
	patterns []secretPattern
}

// NewRedactor builds a Redactor. maxBytes ≤ 0 falls back to DefaultMaxLogBytes.
// extra patterns are appended to the built-in set; each extra is scrubbed
// wholesale (matched runs replaced by the mark).
func NewRedactor(maxBytes int, extra []*regexp.Regexp) *Redactor {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxLogBytes
	}
	patterns := make([]secretPattern, len(builtinRedactions), len(builtinRedactions)+len(extra))
	copy(patterns, builtinRedactions)
	for _, re := range extra {
		if re != nil {
			patterns = append(patterns, secretPattern{re: re})
		}
	}
	return &Redactor{maxBytes: maxBytes, patterns: patterns}
}

// Redact makes a raw log safe for a fixer: it scrubs every configured secret
// shape, THEN clamps the result to the byte budget. Order matters — scrubbing
// before clamping guarantees a secret cannot survive by sitting in the elided
// middle (the whole input is scrubbed first, then the middle is dropped).
func (r *Redactor) Redact(raw string) string {
	if raw == "" {
		return ""
	}
	scrubbed := r.scrub(raw)
	return clamp(scrubbed, r.maxBytes)
}

// scrub applies every redaction pattern in order.
func (r *Redactor) scrub(s string) string {
	for _, p := range r.patterns {
		if p.value {
			s = p.re.ReplaceAllString(s, "${1}"+redactionMark)
		} else {
			s = p.re.ReplaceAllString(s, redactionMark)
		}
	}
	return s
}

// clamp bounds s to about max bytes, keeping the head and (larger) tail with an
// elision marker between — a failed-CI log's error usually lands at the tail, but
// the head carries the invocation and early context. Cuts snap to a line boundary
// within budget for readability, falling back to a hard byte cut when a single
// line exceeds the budget. A string already within budget is returned unchanged.
func clamp(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	// Reserve room for the marker; if the budget is tiny, hard-truncate the tail.
	const minSplit = 64
	if max < minSplit {
		return tailBytes(s, max)
	}
	headBudget := max * 40 / 100
	tailBudget := max - headBudget
	if failures := failureExcerpts(s, max/2); failures != "" {
		headBudget = max / 4
		tailBudget = max / 4
		head := snapForward(s[:headBudget])
		tail := snapBackward(s[len(s)-tailBudget:])
		return head + "\n… [clamped log; failure excerpts follow] …\n" + failures + "\n… [log tail] …\n" + tail
	}
	head := snapForward(s[:headBudget])
	tail := snapBackward(s[len(s)-tailBudget:])
	dropped := len(s) - len(head) - len(tail)
	return head + fmt.Sprintf("\n… [clamped %d bytes] …\n", dropped) + tail
}

// Successful package output can bury a failed Go test in the middle of a CI
// log. Prefer concrete test/runtime failures over generic CI annotations and
// package summaries. If too many equally strong anchors fit, sample across the
// whole log, including the first and last, rather than keeping only a prefix.
// Each sample reserves its anchor before nearby context. This sees only text
// after scrubbing; shared context lines are emitted once within the byte budget.
func failureExcerpts(log string, budget int) string {
	if budget <= 0 {
		return ""
	}
	lines := strings.Split(log, "\n")
	var anchors []int
	priority := 0
	for i, line := range lines {
		rank := failurePriority(line)
		if rank == 0 || rank < priority {
			continue
		}
		if rank > priority {
			anchors = anchors[:0]
			priority = rank
		}
		anchors = append(anchors, i)
	}
	if len(anchors) == 0 {
		return ""
	}
	count := min(len(anchors), max(1, budget/128))
	selected := make(map[int]string)
	for sample := range count {
		index := len(anchors) - 1
		if count > 1 {
			index = sample * (len(anchors) - 1) / (count - 1)
		}
		anchor := anchors[index]
		remaining := budget / count
		// Keep the anchor, then its closest diagnostic on each side. More
		// distant output cannot consume a sample before its actual failure.
		for _, offset := range []int{0, -1, 1, -2, 2, 3, 4, 5, 6, 7, 8} {
			line := anchor + offset
			if line < 0 || line >= len(lines) || remaining <= 1 {
				continue
			}
			if offset != 0 && failurePriority(lines[line]) == priority {
				continue // another anchor reserves its own sample before context
			}
			if _, exists := selected[line]; exists {
				continue
			}
			text := lines[line]
			if len(text) >= remaining {
				text = text[:remaining-1]
				for len(text) > 0 && !utf8Start(lines[line][len(text)]) {
					text = text[:len(text)-1]
				}
			}
			selected[line] = text
			remaining -= len(text) + 1
		}
	}
	indices := make([]int, 0, len(selected))
	for line := range selected {
		indices = append(indices, line)
	}
	slices.Sort(indices)
	var out strings.Builder
	for _, line := range indices {
		out.WriteString(selected[line])
		out.WriteByte('\n')
	}
	return out.String()
}

func failurePriority(line string) int {
	switch {
	case strings.Contains(line, "--- FAIL:"), strings.Contains(line, "panic:"),
		strings.Contains(line, "fatal error:"), strings.Contains(line, "WARNING: DATA RACE"):
		return 3
	case strings.Contains(line, "FAIL\t"):
		return 2
	case strings.Contains(line, "##[error]"), strings.Contains(line, ": error:"):
		return 1
	default:
		return 0
	}
}

// snapForward trims a head slice back to the last newline within it, so the head
// ends on a whole line. It keeps the slice as-is when it has no newline.
func snapForward(head string) string {
	if i := strings.LastIndexByte(head, '\n'); i >= 0 {
		return head[:i]
	}
	return head
}

// snapBackward trims a tail slice forward to just after the first newline, so the
// tail starts on a whole line. It keeps the slice as-is when it has no newline.
func snapBackward(tail string) string {
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i+1 < len(tail) {
		return tail[i+1:]
	}
	return tail
}

// tailBytes returns the last n bytes of s, snapped to a rune boundary so the
// result is valid UTF-8.
func tailBytes(s string, n int) string {
	if n >= len(s) {
		return s
	}
	t := s[len(s)-n:]
	for len(t) > 0 && !utf8Start(t[0]) {
		t = t[1:]
	}
	return t
}

// utf8Start reports whether b is a valid UTF-8 leading byte (ASCII, or a
// multibyte lead — never a 10xxxxxx continuation byte).
func utf8Start(b byte) bool {
	return b&0xC0 != 0x80
}
