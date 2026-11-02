package e2e

// Leg 4 records installed-binary latency (NFR-1/NFR-2/A-9) alongside successful
// execution and preflight-event assertions. The source is local and needs no
// network, but process startup and filesystem cost still depend on the host.
// AP-29: measurements are advisory in every build mode, including coverage.

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// perInvocationBatch is how many warm `pyxis exec` invocations feed the p95 —
// enough for a stable p95 without making the leg slow.
const perInvocationBatch = 20

// TestPerInvocationOverheadMeasurement records the warm-cache exec hop (A-9/§3.4).
func TestPerInvocationOverheadMeasurement(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r) // cache warmed by init's pre-warm

	// One warm-up so the very first fetch/JIT cost is out of the sample (the
	// budget is the STEADY-STATE hop, not a cold first call — the fresh-clone
	// leg covers the cold path).
	if res := pyxis(t, repo, env, "exec", "pyxis", "--", "version"); res.code != 0 {
		t.Fatalf("warm-up exec failed: %d\n%s", res.code, res.stderr)
	}

	durs := make([]time.Duration, 0, perInvocationBatch)
	for i := range perInvocationBatch {
		start := time.Now()
		res := pyxis(t, repo, env, "exec", "pyxis", "--", "version")
		elapsed := time.Since(start)
		if res.code != 0 {
			t.Fatalf("exec invocation %d failed: %d\n%s", i, res.code, res.stderr)
		}
		durs = append(durs, elapsed)
	}

	p50 := percentile(durs, 50)
	p95 := percentile(durs, 95)
	mx := maxDur(durs)
	t.Logf("per-invocation overhead (n=%d, warm cache, subprocess incl. process spawn): p50=%s p95=%s max=%s — reference target ≤100ms p95 (§3.4/A-9); advisory (AP-29)",
		perInvocationBatch, p50, p95, mx)
}

// TestInstallWallTimeMeasurement records a full install and its preflight phase.
func TestInstallWallTimeMeasurement(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)
	repo := t.TempDir()
	env := pyxisEnv(t, repo)

	start := time.Now()
	res := pyxis(t, repo, env, initArgs(r)...) // a full carina+local install from the file:// source
	elapsed := time.Since(start)
	if res.code != 0 {
		t.Fatalf("install did not end green: exit %d\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}

	t.Logf("install wall-time (carina+local, promptless, file:// source, pre-warm incl.): %s — reference target ≤5min p95 on the matrix (A-3); advisory single sample (AP-29)", elapsed)

	// NFR-1/A-13 (plan init-hardening/001 T11): the install budget is
	// RE-MEASURED with the full front-loaded check pass now included by
	// construction (every init call runs the whole preflight registry) —
	// record the checks' OWN contribution explicitly (init_checks_passed's
	// duration_ms) rather than leaving it an unmeasured assumption buried
	// inside the total. This phase has no separate latency requirement.
	checksMS := checksPassedDurationMS(t, res.stderr)
	checksDur := time.Duration(checksMS) * time.Millisecond
	t.Logf("check-pass duration (init_checks_passed: admission+toolchain+artifact-source+cross-knob-coherence): %s — included in installation; advisory (AP-29)", checksDur)
}

// checksPassedDurationMS extracts init_checks_passed's duration_ms field
// from a subprocess's text-handler stderr (cmd/pyxis wires
// slog.NewTextHandler) — the front-loaded check pass's own wall-clock cost.
// Fatals when the event or field is absent: a successful init fires this
// event exactly once (installer.go's emitInitChecksPassed), so its absence
// here is itself a defect, not a benign miss.
func checksPassedDurationMS(t *testing.T, stderrText string) int64 {
	t.Helper()
	for line := range strings.SplitSeq(stderrText, "\n") {
		if !strings.Contains(line, "msg=init_checks_passed") {
			continue
		}
		const field = "duration_ms="
		_, v, found := strings.Cut(line, field)
		if !found {
			t.Fatalf("init_checks_passed line carries no duration_ms field:\n%s", line)
		}
		if sp := strings.IndexByte(v, ' '); sp >= 0 {
			v = v[:sp]
		}
		ms, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			t.Fatalf("parse init_checks_passed duration_ms %q: %v", v, err)
		}
		if ms < 0 {
			t.Fatalf("init_checks_passed duration_ms must be nonnegative: %d", ms)
		}
		return ms
	}
	t.Fatalf("no init_checks_passed event in the install's stderr:\n%s", stderrText)
	return 0
}

// percentile returns the p-th percentile of durs (nearest-rank), 0 ≤ p ≤ 100.
// The sample is copied before sorting so the caller's slice is untouched.
func percentile(durs []time.Duration, p int) time.Duration {
	if len(durs) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), durs...)
	slices.Sort(s)
	// Nearest-rank: rank = ceil(p/100 * n), 1-indexed.
	rank := (p*len(s) + 99) / 100
	rank = min(max(rank, 1), len(s))
	return s[rank-1]
}

// maxDur returns the largest duration in the sample.
func maxDur(durs []time.Duration) time.Duration {
	var m time.Duration
	for _, d := range durs {
		if d > m {
			m = d
		}
	}
	return m
}
