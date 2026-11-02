// This file embeds and extracts a sealed Capsule from an ordinary tracker
// item body (design: execution-demand-contract-v1 §3.5): the one HTML
// comment channel every supported tracker medium (GitHub, Carina) already
// uses for machine markers — see internal/plangate/dialect.PlanTaskMarker,
// which this mirrors without importing (this is a port-side package; that
// is adapter-facing rendering).
package executiondemand

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// capsuleMarkerTag is the marker's machine key.
const capsuleMarkerTag = "pyxis:execution-demand"

// capsuleMarkerRe locates the marker's payload span. (?s) lets the payload
// span multiple lines: a tracker medium or a human editor may soft-wrap a
// long line without changing its logical bytes (design §3.5's "presentation
// normalization... must not alter logical bytes") — Extract strips all
// whitespace from the captured span before decoding, so wrapping never
// corrupts the payload. The payload is base64url (brainstorm ALT-14):
// its alphabet (A-Za-z0-9,-,_) never produces '>', so a capsule can never
// contain "-->" and prematurely close its own comment, regardless of what
// bytes it encodes.
var capsuleMarkerRe = regexp.MustCompile(`(?s)<!--\s*` + regexp.QuoteMeta(capsuleMarkerTag) + `\s+(.*?)\s*-->`)

// maxCapsuleWireBytes bounds the decoded capsule size Extract will attempt
// to structurally parse — a defensive limit against an oversized marker
// (accidental or adversarial) forcing unbounded decode work on untrusted
// tracker input. A genuine capsule (assessment, citations, profile, policy
// identity) runs from a few hundred bytes to a few KB; this is generous
// headroom, well inside a GitHub issue body's own 65536-character cap.
const maxCapsuleWireBytes = 32 * 1024

// GitHub measures its issue-body limit in characters, not UTF-8 bytes. Keep
// that medium limit as the semantic bound while using UTFMax times the limit
// as the cheap pre-scan byte bound. This admits every valid multibyte body,
// yet still bounds the rune scan, regexp, and copies on untrusted input.
const (
	maxTrackerBodyCharacters = 65_536
	maxTrackerBodyBytes      = maxTrackerBodyCharacters * utf8.UTFMax
)

// maxCapsuleEncodedBytes is the largest raw-base64 token a maximum-sized
// capsule can produce. The span limit permits generous presentation wrapping
// while refusing whitespace amplification before stripWhitespace allocates.
const (
	maxCapsuleEncodedBytes     = (maxCapsuleWireBytes*8 + 5) / 6
	maxCapsulePayloadSpanBytes = maxCapsuleEncodedBytes + 4*1024
)

// Embed appends c's one canonical marker to body. The independently
// reconstructed authority is mandatory so every refusal carries trusted
// repository/task/demand correlation; c itself is never a diagnostic source.
// It refuses a body that already carries a capsule marker: a producer calls
// Embed exactly once per tracker item, before the item becomes routable
// (design §3.5); a second call is a producer defect, not something to paper
// over by silently emitting a body Extract would later refuse as duplicate.
func Embed(body string, c Capsule, want AuthoritativeDemand) (string, error) {
	trusted, err := want.trustedCapsule()
	if err != nil {
		return "", err
	}
	refuse := func(boundary, field, code, message string) error {
		return newCapsuleMismatch(boundary, field, code, message).withTrustedCapsule(trusted)
	}
	if trackerBodyTooLarge(body) {
		return "", refuse(BoundaryTransport, "body", CodeOversizedBody, "tracker body exceeds the transport parsing limit")
	}
	if capsuleTagOccurrences(body, 1) != 0 {
		return "", refuse(BoundaryTransport, "capsule", CodeDuplicateCapsule, "body already carries an execution-demand capsule marker")
	}
	wire, err := EncodeCapsuleCanonical(c)
	if err != nil {
		return "", refuse(BoundarySelf, "shape", CodeInvalidCapsule, "capsule shape is invalid")
	}
	if len(wire) > maxCapsuleWireBytes {
		return "", refuse(BoundaryTransport, "capsule", CodeOversizedCapsule, fmt.Sprintf("capsule is %d bytes, exceeds the %d-byte transport limit", len(wire), maxCapsuleWireBytes))
	}
	token := base64.RawURLEncoding.EncodeToString(wire)
	sep := ""
	if body != "" && !strings.HasSuffix(body, "\n") {
		sep = "\n"
	}
	result := fmt.Sprintf("%s%s\n<!-- %s %s -->\n", body, sep, capsuleMarkerTag, token)
	if trackerBodyTooLarge(result) {
		return "", refuse(BoundaryTransport, "body", CodeOversizedBody, "tracker body plus capsule exceeds the transport parsing limit")
	}
	if err := VerifyCapsule(c, want); err != nil {
		return "", err
	}
	return result, nil
}

// Extract reads body's exactly-one capsule marker, decodes it, and checks
// self-consistency before returning (design §3.5's readback: "accepts
// exactly one capsule"). Authority is required up front so even a refusal
// that happens before decoding carries trusted repository/task/demand
// correlation. Extract does not compare capsule content to that authority;
// callers still invoke VerifyCapsule after a successful extraction.
func Extract(body string, want AuthoritativeDemand) (Capsule, error) {
	trusted, err := want.trustedCapsule()
	if err != nil {
		return Capsule{}, err
	}
	refuse := func(boundary, field, code, message string) error {
		return newCapsuleMismatch(boundary, field, code, message).withTrustedCapsule(trusted)
	}
	if trackerBodyTooLarge(body) {
		return Capsule{}, refuse(BoundaryTransport, "body", CodeOversizedBody, "tracker body exceeds the transport parsing limit")
	}
	tagCount := capsuleTagOccurrences(body, 2)
	if tagCount == 0 {
		return Capsule{}, refuse(BoundaryTransport, "capsule", CodeMissingCapsule, "no execution-demand capsule marker found in body")
	}
	if tagCount > 1 {
		return Capsule{}, refuse(BoundaryTransport, "capsule", CodeDuplicateCapsule, "multiple execution-demand capsule markers found; want exactly one")
	}
	indices := capsuleMarkerRe.FindStringSubmatchIndex(body)
	if indices == nil {
		code := CodeMalformedCapsule
		tagAt := strings.Index(body, capsuleMarkerTag)
		if tagAt >= 0 && !strings.Contains(body[tagAt+len(capsuleMarkerTag):], "-->") {
			code = CodeTruncatedCapsule
		}
		return Capsule{}, refuse(BoundaryTransport, "capsule", code, "execution-demand capsule marker is not well formed")
	}
	payload := body[indices[2]:indices[3]]
	if len(payload) > maxCapsulePayloadSpanBytes {
		return Capsule{}, refuse(BoundaryTransport, "capsule", CodeOversizedCapsule, "capsule marker span exceeds the transport limit")
	}
	token := stripWhitespace(payload)
	if len(token) > maxCapsuleEncodedBytes || base64.RawURLEncoding.DecodedLen(len(token)) > maxCapsuleWireBytes {
		return Capsule{}, refuse(BoundaryTransport, "capsule", CodeOversizedCapsule, "capsule payload exceeds the transport limit")
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Capsule{}, refuse(BoundaryTransport, "capsule", CodeMalformedCapsule, "capsule payload is not valid base64url")
	}
	c, err := DecodeCapsuleCanonical(raw)
	if err != nil {
		code := CodeMalformedCapsule
		if looksTruncated(err) {
			code = CodeTruncatedCapsule
		}
		message := "capsule payload is not a valid canonical capsule"
		if code == CodeTruncatedCapsule {
			message = "capsule payload is truncated"
		}
		return Capsule{}, refuse(BoundaryTransport, "capsule", code, message)
	}
	if err := c.selfConsistent(); err != nil {
		return Capsule{}, withAuthority(err, trusted)
	}
	return c, nil
}

func trackerBodyTooLarge(body string) bool {
	return len(body) > maxTrackerBodyBytes || utf8.RuneCountInString(body) > maxTrackerBodyCharacters
}

// capsuleTagOccurrences counts only up to limit occurrences of the reserved
// machine tag. Counting the raw tag, rather than only fully matched comments,
// makes malformed and unterminated duplicate attempts visible. Callers use a
// limit of two because the only meaningful states are none, exactly one, and
// duplicate; attacker-controlled repetitions never trigger an unbounded scan.
func capsuleTagOccurrences(body string, limit int) int {
	count := 0
	for from := 0; from < len(body) && count < limit; {
		i := strings.Index(body[from:], capsuleMarkerTag)
		if i < 0 {
			break
		}
		count++
		from += i + len(capsuleMarkerTag)
	}
	return count
}

// stripWhitespace removes every Unicode whitespace rune from s — the
// counterpart to capsuleMarkerRe's (?s) tolerance: whatever soft-wrapping
// a medium introduced into the payload span is not logical payload.
func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// looksTruncated recognizes codec.go's own truncation-shaped error text
// ("truncated length prefix", "truncated field: want N bytes, have M",
// "missing field N") — a payload that decodes as a genuine PREFIX of a
// valid capsule, as copy-paste or a byte-limited transport would produce.
// Every other decode failure (bad tag order, trailing bytes, unknown
// schema/kind — a complete-length payload that is simply not shaped like a
// capsule) is malformed, not truncated: base64url corruption from a
// mis-typed or bit-flipped character is indistinguishable from that class
// and is classified malformed too, honestly reflecting that nothing here
// can know why an unaligned corruption happened.
func looksTruncated(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "truncated") || strings.Contains(msg, "missing field")
}
