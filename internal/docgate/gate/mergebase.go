package gate

import (
	"fmt"
	"os/exec"
	"strings"
)

// ResolveMergeBase turns a --merge-base ref (e.g. origin/main) into the commit
// where the current branch forked from it: `git merge-base <ref> HEAD`. Every
// history check diffs against this commit, never the ref's moving tip. Diffing
// against the tip makes a branch cut from an older main read each document
// merged onto main *after* the cut as a deletion (or modification) this branch
// never made — a phantom defect that false-fails check-immutable and its
// siblings on any behind-but-additive branch (INV-41's CI half; docgate-v1
// §3.1's "git diff --name-status <merge-base>" is a merge base, not a tip).
// Because the fork point is stable however far the tip advances, it also
// erases the pre-push/CI asymmetry (a stale local origin/main and a freshly
// fetched one resolve to the same base). Resolved once, at the CLI boundary,
// so every history check inherits one honest base.
func ResolveMergeBase(root, ref string) (string, Defects) {
	return ResolveMergeBaseAt(root, ref, "HEAD")
}

// ResolveMergeBaseAt is ResolveMergeBase against an explicit tip — the pushed
// sha at pre-push, so a push of a non-checked-out branch still resolves the
// fork point of exactly the pushed history (#357).
func ResolveMergeBaseAt(root, ref, tip string) (string, Defects) {
	if tip == "" {
		tip = "HEAD"
	}
	out, err := exec.Command("git", "-C", root, "merge-base", ref, tip).Output()
	if err != nil {
		return "", Defects{def("merge-base", ref, "git",
			fmt.Sprintf("git merge-base %q %s failed: %v", ref, tip, err),
			"pass a resolvable --merge-base ref (e.g. origin/main) fetched with full history (fetch-depth: 0)")}
	}
	base := strings.TrimSpace(string(out))
	if base == "" {
		return "", Defects{def("merge-base", ref, "git",
			fmt.Sprintf("no common ancestor between %q and %s", ref, tip),
			"ensure the base ref is fetched and shares history with this branch")}
	}
	return base, nil
}
