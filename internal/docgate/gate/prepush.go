// The pre-push verb's ref handling (FR-29; docs/tech-design/
// prepush-ref-awareness.md): a deletion push carries no content, so it
// skips the content gate; anything else fails closed into the full gate.
package gate

import (
	"bufio"
	"io"
	"strings"
)

// ParsePushRefs reads the pre-push hook protocol from r — lines of
// "<local-ref> <local-sha> <remote-ref> <remote-sha>" — and returns the
// distinct pushed (non-deletion) local SHAs in input order, plus whether the
// push is deletion-only (at least one parseable line, every one an all-zeros
// local SHA). Empty or malformed input yields (nil, false): the caller runs
// the full working-tree gate — fail closed, exactly the old behavior (DINV-2).
// The SHAs let the pre-push gate judge the committed range <merge-base>..<sha>
// instead of the working tree, so another session's uncommitted state in a
// shared clone can neither fail nor mask a push (#357). Classification uses
// only the zero-SHA field, never ref names (DINV-1).
func ParsePushRefs(r io.Reader) (heads []string, deletionOnly bool) {
	sc := bufio.NewScanner(r)
	seen := false
	all := true
	seenSHA := map[string]bool{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, false // malformed → full working-tree gate
		}
		seen = true
		if isZeroSHA(fields[1]) {
			continue
		}
		all = false
		if !seenSHA[fields[1]] {
			seenSHA[fields[1]] = true
			heads = append(heads, fields[1])
		}
	}
	if sc.Err() != nil {
		return nil, false
	}
	return heads, seen && all
}

// DeletionOnlyPush reports whether the pre-push input is a deletion-only push.
// It is ParsePushRefs' second return, kept as the FR-29 entry point.
func DeletionOnlyPush(r io.Reader) bool {
	_, deletionOnly := ParsePushRefs(r)
	return deletionOnly
}

func isZeroSHA(s string) bool {
	if len(s) < 40 {
		return false
	}
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}
