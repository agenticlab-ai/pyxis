// This file owns the maintenance sweep's rollup-staleness detector
// (maintenance-sweep-v1 §3.2). A living rollup is stale iff the newest commit
// touching the configured capability paths is newer than the rollup's own
// newest commit by more than the policy's staleness bound (default 14 days).
// The rollup set and the capability paths come from the EXISTING configuration
// surface's keys (paths.rollup_prd, paths.rollup_techspec, capability_paths) —
// never new policy (§3.2). Ages derive from committed timestamps only, read
// through the injectable runner seam, so the result is deterministic per
// evaluated state, with no wall clock anywhere (DINV-2). A rollup with no
// commits is not living and is skipped; a git fault is named in-band and the
// detector yields no finding (DINV-4). Stdlib only, per the repo Go rules; the
// package holds no write-capable client (DINV-1).
package sweep

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// DetectRollupStaleness reports each living rollup whose newest commit trails the
// newest capability-path commit by more than the policy's staleness bound
// (maintenance-sweep-v1 §3.2). The rollup set (paths.rollup_prd,
// paths.rollup_techspec) and the capability paths (capability_paths) come from
// the existing config surface, never new policy. A rollup with no commits is not
// living — the pre-first-capability state where docs/prd.md and docs/techspec.md
// do not yet exist — and is skipped. A git fault is named in-band and the
// detector yields no finding (DINV-4). It is a pure function of (state, policy)
// and performs no write (DINV-1).
func DetectRollupStaleness(ctx context.Context, st State, pol Policy) DetectorResult {
	res := DetectorResult{Name: DetectorRollupStaleness}

	// The newest committed change under the configured capability paths — the
	// shipped-structure clock every rollup is measured against.
	structTime, shipped, err := newestCommit(ctx, st, st.Config.CapabilityPaths)
	if err != nil {
		res.Fault = err.Error()
		return res
	}
	if !shipped {
		// No capability-path history: nothing has shipped, so nothing can be
		// stale against it.
		return res
	}

	for _, rollup := range []string{st.Config.RollupPRD, st.Config.RollupTechspec} {
		if strings.TrimSpace(rollup) == "" {
			continue
		}
		rollupTime, living, err := newestCommit(ctx, st, []string{rollup})
		if err != nil {
			res.Fault = err.Error()
			return res
		}
		if !living {
			// A rollup with no commits is not living — skip it, never a finding.
			continue
		}
		lag := structTime.Sub(rollupTime)
		if lag <= pol.StalenessBound {
			continue
		}
		f, ferr := NewFinding(
			DetectorRollupStaleness,
			rollup,
			fmt.Sprintf("newest capability-path commit is %s newer than the rollup's newest commit (bound: %s)",
				humanizeDelta(lag), humanizeDelta(pol.StalenessBound)),
			"refresh the rollup to the shipped structure, or retune the bound in the policy file",
		)
		if ferr != nil {
			// A finding that cannot be constructed is a detector fault, never a
			// silently dropped signal (DINV-4/DINV-5).
			res.Fault = fmt.Sprintf("rollup-staleness: %v", ferr)
			return res
		}
		res.Findings = append(res.Findings, f)
	}
	return res
}

// newestCommit returns the newest committer timestamp among the given paths in
// the evaluated state, read through the runner seam
// (`git log -1 --format=%ct -- <paths...>`). shipped is false when no commit
// touches any path — an empty set or an all-untracked one — the not-living /
// nothing-shipped state, which is not a fault. A git error or an unparsable
// timestamp is a fault. The returned time is UTC so the value never leaks a
// local zone (DINV-2).
func newestCommit(ctx context.Context, st State, paths []string) (time.Time, bool, error) {
	ps := nonEmptyPaths(paths)
	if len(ps) == 0 {
		return time.Time{}, false, nil
	}
	args := append([]string{"log", "-1", "--format=%ct", "--"}, ps...)
	lines, err := st.Runner.Run(ctx, st.Root, "git", args...)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("rollup-staleness: reading newest commit for %s: %w", strings.Join(ps, " "), err)
	}
	if len(lines) == 0 {
		return time.Time{}, false, nil
	}
	s := strings.TrimSpace(lines[0])
	if s == "" {
		return time.Time{}, false, nil
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("rollup-staleness: unparsable commit timestamp %q: %w", s, err)
	}
	return time.Unix(secs, 0).UTC(), true, nil
}

// nonEmptyPaths drops blank entries so an empty or whitespace-only config key
// never becomes an empty git pathspec (which would match the whole tree).
func nonEmptyPaths(paths []string) []string {
	var out []string
	for _, p := range paths {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// humanizeDelta renders a duration as a stable, deterministic day/hour string
// for a finding's evidence (e.g. "31d", "14d6h"). It is a pure function of its
// input, so equal deltas always render identically (DINV-2). Sub-hour deltas
// fall back to the exact Go duration so a small over-bound lag is never shown as
// "0h".
func humanizeDelta(d time.Duration) string {
	days := int64(d / (24 * time.Hour))
	hours := int64((d % (24 * time.Hour)) / time.Hour)
	switch {
	case days == 0 && hours == 0:
		return d.String()
	case hours == 0:
		return fmt.Sprintf("%dd", days)
	default:
		return fmt.Sprintf("%dd%dh", days, hours)
	}
}
