package efficiency

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Latency spans a phase's git-visible lifetime: from the earliest to the
// latest commit touching any of paths, in the repository rooted at
// repoRoot (design §3.10: "latency from git timestamps"). A path git has
// never committed contributes nothing — a corpus artifact absent reads as
// no signal, never a fabricated span; if none of paths has history,
// Latency returns zero. Times are git's own committer timestamps, so two
// checkouts of the same history (e.g. two harnesses replaying the same
// reference run) always agree.
func Latency(repoRoot string, paths []string) (time.Duration, error) {
	var earliest, latest int64
	seen := false
	for _, p := range paths {
		times, err := commitTimes(repoRoot, p)
		if err != nil {
			return 0, err
		}
		for _, t := range times {
			if !seen || t < earliest {
				earliest = t
			}
			if !seen || t > latest {
				latest = t
			}
			seen = true
		}
	}
	if !seen {
		return 0, nil
	}
	return time.Duration(latest-earliest) * time.Second, nil
}

// commitTimes returns the committer-timestamp (unix seconds) of every
// commit touching path within repoRoot, in no particular order. A path
// with no history returns nil, not an error — git log exits 0 with empty
// output for a pathspec that never matched.
func commitTimes(repoRoot, path string) ([]int64, error) {
	out, err := exec.Command("git", "-C", repoRoot, "log", "--format=%ct", "--", path).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", path, err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	lines := strings.Split(trimmed, "\n")
	times := make([]int64, 0, len(lines))
	for _, line := range lines {
		t, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse commit time %q for %s: %w", line, path, err)
		}
		times = append(times, t)
	}
	return times, nil
}
