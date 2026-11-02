// Package stamp is the Argus authorship producer — the commit-time stamp that
// appends the contractual `Argus-Drive: <slug>` authorship trailer when the
// commit hook receives an active drive slug (docs/tech-design/argus-draft-attribution.md
// §3.2). The shared-account workflow supplies that slug per drive commit.
//
// The stamp decision and the trailer contract live here once; each supported
// agent CLI's commit-authoring path is wired to it by a pointer/adapter
// (adapters.go + a .pyxis/stamp config), never a content copy (AP-25). The
// producer sits on the commit-authoring surface (a git commit-message hook on
// the repo's own core.hooksPath). The current drive can also invoke Git; the
// stamp is audit evidence, not an isolation boundary against that drive (§3.4).
//
// This is the produced signal the attribution gate (internal/argus/attribution)
// reads at the merge boundary: the trailer key is single-sourced from
// attribution.TrailerKey, and the slug grammar matches the reader's, so a
// produced trailer derives as Argus and never as an unverifiable (underivable)
// one. The content binding detects stale or replayed trailers when the gate
// verifies them; this
// package only produces the slug-keyed signal.
package stamp

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
)

// TrailerKey is the contractual authorship-trailer key the producer stamps —
// single-sourced from the gate's reader so the produced signal and the read
// signal can never drift (one contractual key, one place; AP-6). The producer
// emits `Argus-Drive: <slug>`; the gate reads that same key structurally.
const TrailerKey = attribution.TrailerKey

// BindTrailerKey is the contractual content-binding trailer key the producer
// stamps ALONGSIDE the authorship trailer — single-sourced from the gate's
// reader (AP-6). The producer emits `Argus-Bind: <binding>` where the binding
// is attribution.ComputeBinding over the commit's own tree+parents; the gate
// re-derives and compares it, so a replayed trailer fails closed (§3.2, DINV-4).
const BindTrailerKey = attribution.BindTrailerKey

// slugRe is the well-formed problem-slug grammar a produced trailer must carry.
// It is identical to the grammar the attribution gate's reader accepts
// (internal/argus/attribution derive.go slugRe): the producer refuses to stamp
// a slug outside it (below) rather than emit a trailer the reader would treat
// as present-but-invalid — an underivable signal. The end-to-end producer→gate
// test (cmd/argus-stamp) is the guarantee the two grammars stay in lockstep.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// commentChar is git's default comment character. Lines beginning with it are
// stripped by git's message cleanup (they are the `# Please enter…` template on
// an editor commit); the producer places its trailer before any trailing
// comment block so the stamp survives cleanup and lands in the final paragraph
// the gate's reader scans. The primary drive path (`git commit -m`) carries no
// comment block at hook time, so this only guards the editor-commit case.
const commentChar = '#'

// Stamp appends the Argus authorship trailer AND its content binding to a commit
// message, keyed on the active drive's slug, and returns the stamped message. It
// is the one canonical producer decision — authored here once, wired to each
// commit-authoring path by a pointer (adapters.go, .pyxis/stamp), never a
// content copy (§3.1, AP-25).
//
// slug is the active drive's slug the commit-authoring actor holds; binding is
// the in-tree content binding the caller computed over the commit's own
// tree+parents (attribution.ComputeBinding), so the pure producer core reads no
// git (the caller — cmd/argus-stamp — does that I/O):
//
//   - empty slug — the commit is authored OUTSIDE any drive: the message is
//     returned unchanged, carrying no trailer. Such a commit derives as human
//     downstream (correct — it is not a drive's landed work; §3.1). binding is
//     ignored.
//   - a well-formed slug with a binding — `Argus-Drive: <slug>` and
//     `Argus-Bind: <binding>` are appended to the message's final trailer block
//     (the block attribution.parseTrailer reads). It is idempotent on a re-run
//     (an amend, a hook fired twice): any pre-existing Argus trailers are
//     stripped and re-appended, so the DRIVE never doubles AND the binding is
//     REFRESHED to the current tree — a stale binding must never survive an
//     amend, else an out-of-scope amend could derive underivable and slip the
//     gate (which refuses Argus, not underivable, out-of-scope diffs).
//   - a malformed slug, or a well-formed slug with an empty binding — an error:
//     the producer fails closed rather than emit a trailer the reader would treat
//     as underivable, so a misconfigured drive context or a failed binding
//     computation surfaces as an aborted commit, never a silently underivable one.
func Stamp(message, slug, binding string) (string, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return message, nil // authored outside a drive → no trailer
	}
	if !slugRe.MatchString(slug) {
		return "", fmt.Errorf("stamp: malformed drive slug %q (must match %s)", slug, slugRe)
	}
	if strings.TrimSpace(binding) == "" {
		return "", fmt.Errorf("stamp: no content binding for drive slug %q (a produced trailer must be content-bound; §3.2)", slug)
	}
	// Strip any pre-existing Argus trailers so a re-run/amend refreshes the
	// binding to the current tree rather than carrying a stale one.
	message = stripArgusTrailers(message)
	return appendTrailer(message, TrailerKey+": "+slug, BindTrailerKey+": "+binding), nil
}

// trailerLineRe matches a git trailer line `Token: value` — the same shape the
// gate's reader uses to recognize the final trailer block.
var trailerLineRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*):[ \t]*(.*)$`)

// stripArgusTrailers removes any Argus-Drive / Argus-Bind lines from the
// message's final trailer block, preserving every other line (a Co-Authored-By
// footer, the body, an editor comment block). It is the refresh step: on a
// re-run/amend the pre-existing stamp is dropped so Stamp can re-append a fresh
// one bound to the current tree. Only OUR keys are stripped; a message carrying
// none is returned effectively unchanged. Trailing blank/comment cruft is kept
// in place after the block.
func stripArgusTrailers(message string) string {
	hadFinalNewline := strings.HasSuffix(message, "\n")
	lines := splitLines(message)

	contentEnd := len(lines)
	for contentEnd > 0 && isTrailingCruft(lines[contentEnd-1]) {
		contentEnd--
	}
	start, _ := finalBlock(lines[:contentEnd])

	// Filter our own keys out of the final block, preserving every other line
	// (a Co-Authored-By footer, the body).
	var block []string
	for _, line := range lines[start:contentEnd] {
		if m := trailerLineRe.FindStringSubmatch(line); m != nil && (m[1] == TrailerKey || m[1] == BindTrailerKey) {
			continue
		}
		block = append(block, line)
	}
	head := lines[:start]
	// If our stamp WAS the whole final block, drop the now-orphaned blank
	// separator that preceded it, so a strip→re-append round-trip is a clean
	// no-op (a re-stamp/amend never accumulates trailing blank lines).
	if len(block) == 0 {
		for len(head) > 0 && strings.TrimSpace(head[len(head)-1]) == "" {
			head = head[:len(head)-1]
		}
	}

	kept := make([]string, 0, len(lines))
	kept = append(kept, head...)
	kept = append(kept, block...)
	kept = append(kept, lines[contentEnd:]...)

	res := strings.Join(kept, "\n")
	if hadFinalNewline && !strings.HasSuffix(res, "\n") {
		res += "\n"
	}
	return res
}

// appendTrailer inserts trailers at the end of the message's log content — into
// the final paragraph when that paragraph is already a trailer block (so the
// stamp joins the existing trailers, e.g. a Co-Authored-By footer), else as a
// new trailer paragraph separated by a blank line. Multiple trailers are
// appended together into the same block (the Argus-Drive + Argus-Bind pair). Any
// trailing comment/blank lines (an editor commit's `# …` template) are preserved
// after the stamp, so git's cleanup drops them and leaves the trailers in the
// final paragraph.
func appendTrailer(message string, trailers ...string) string {
	hadFinalNewline := strings.HasSuffix(message, "\n")
	lines := splitLines(message)

	// contentEnd is the index just past the last log-content line (non-blank,
	// non-comment). Everything from contentEnd on is trailing blanks/comments.
	contentEnd := len(lines)
	for contentEnd > 0 && isTrailingCruft(lines[contentEnd-1]) {
		contentEnd--
	}

	start, _ := finalBlock(lines[:contentEnd])
	// Join the final paragraph only when it is a trailer block preceded by a
	// body (start > 0) — a git trailer block must follow a preceding paragraph.
	// A subject-only message (start == 0), even one whose subject is shaped like
	// a trailer, gets the stamp as its own blank-separated paragraph.
	insert := trailers
	if start <= 0 || !isTrailerBlock(lines[start:contentEnd]) {
		insert = append([]string{""}, trailers...)
	}

	out := make([]string, 0, len(lines)+len(insert))
	out = append(out, lines[:contentEnd]...)
	out = append(out, insert...)
	out = append(out, lines[contentEnd:]...)

	res := strings.Join(out, "\n")
	if hadFinalNewline && !strings.HasSuffix(res, "\n") {
		res += "\n"
	}
	return res
}

// finalBlock returns the [start, end) line range of the final non-empty
// paragraph — the maximal run of non-blank lines ending at the last non-blank
// line, matching the gate reader's trailer-block convention.
func finalBlock(lines []string) (start, end int) {
	end = len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start = end
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	return start, end
}

// isTrailerBlock reports whether every line of a block is a git trailer line —
// the condition under which the stamp joins the block rather than starting a
// new paragraph. An empty block is not a trailer block (a bare stamp gets its
// own paragraph).
func isTrailerBlock(block []string) bool {
	if len(block) == 0 {
		return false
	}
	for _, line := range block {
		if !trailerLineRe.MatchString(line) {
			return false
		}
	}
	return true
}

// isTrailingCruft reports whether a line is a blank line or a git comment line —
// the trailing material the stamp is placed before.
func isTrailingCruft(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, string(commentChar))
}

// splitLines splits a message into lines after normalizing CRLF, without
// dropping content.
func splitLines(message string) []string {
	return strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
}
