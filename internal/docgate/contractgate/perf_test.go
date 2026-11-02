package contractgate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Performance (plan T8; prd:contract-gate/NFR-1; design contract-gate-v1 §3.11,
// A-7). NFR-1: the gate completes within p95 ≤ 5 s on the repo's largest live
// contract. These tests run the gate over the repo's own live contracts and
// assert only that each produces a valid verdict — the hermetic correctness
// smoke — while MEASURING (never asserting) the wall-time and logging it so a
// regression is visible.
//
// Wall-clock is not a hermetic unit-test assertion: the same code clocks
// differently on a warm dev box vs. the confined/-race runner path vs. a loaded
// CI box, so a time budget as a pass/fail gate is a category error that flakes
// the mirror (incident I5; DINV-3, "measurements never block"; cf. #606, which
// neutralized the same defect in TestRung0Latency). NFR-1's budget is evidenced
// by BenchmarkGateLargestLiveContract and the logged times, not gated here.
//
// The tests resolve the real repo root (walking up for go.mod) so they gate the
// repo's actual live contracts, not a fixture. When run outside the repo (no
// go.mod above the cwd) they skip — determinism/hermeticity are proven on
// fixtures; performance is meaningful only over the real corpus.

// perfCeiling is the NFR-1 reference target (p95 ≤ 5 s on the largest contract)
// the per-contract measurement is LOGGED against — never asserted (see the file
// header: wall-clock is a measurement, not a gate; I5/DINV-3).
const perfCeiling = 5 * time.Second

// findRepoRoot walks up from the test's cwd to the directory containing go.mod —
// the repo root the real live contracts live under. Returns ("", false) when none
// is found (the test then skips).
func findRepoRoot(t *testing.T) (string, bool) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// TestGatePerformanceOnLargestLiveContract runs the gate over the repo's live
// contracts, asserting only that every run yields a valid verdict (the hermetic
// correctness smoke), and MEASURES each run's wall-time — logging every
// contract's time and the slowest against the NFR-1 reference target, so a
// regression stays visible without gating on a non-hermetic clock (I5/DINV-3).
//
// Corpus scope is race-conditional. A-7's subject is "the largest live
// contract", so under -race that is the ONLY contract gated: the race detector
// inflates each run ~17x, and the whole-corpus loop cost ≈88 s of every
// docs-gate job when required CI used `go test -race ./...`, so this test set
// the suite's critical path. The detector buys
// nothing here to offset that — internal/docgate has no goroutine, channel, or
// mutex anywhere in its non-test tree, so there is no concurrent access for it
// to observe. Every non-race run — the local `go test ./...` default — still
// sweeps the FULL corpus at ≈5 s, so no contract loses its smoke.
func TestGatePerformanceOnLargestLiveContract(t *testing.T) {
	root, ok := findRepoRoot(t)
	if !ok {
		t.Skip("no go.mod above cwd — performance is measured over the real repo corpus only")
	}
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		t.Fatalf("repo config does not parse: %v", cdef)
	}
	features, ferr := liveFeatures(root, cfg)
	if ferr != "" {
		t.Fatalf("enumerating live features: %s", ferr)
	}
	if len(features) == 0 {
		t.Skip("no live contracts in the repo")
	}
	if raceDetectorEnabled {
		// A-7's own subject, and the contract most likely to surface a defect.
		largest := largestLiveContract(t, root, cfg)
		if largest == "" {
			t.Skip("no live contracts in the repo")
		}
		t.Logf("-race: gating the largest live contract only (%d live contracts swept in full on a non-race run)", len(features))
		features = []string{largest}
	}

	sev := defaultSeverityMap()
	var slowest time.Duration
	var slowestFeature string
	for _, f := range features {
		start := time.Now()
		v := runResolved(root, f, TriggerPlanningEntry, cfg, sev)
		elapsed := time.Since(start)
		// The gate must produce a real verdict (a mechanical class), never crash or
		// hang — the timing is only meaningful over a completed run.
		if d := v.Validate(); len(d) != 0 {
			t.Errorf("gate on %q produced an invalid verdict: %v", f, d)
		}
		t.Logf("  %-32s %v  (%s)", f, elapsed.Round(time.Microsecond), v.Class)
		if elapsed > slowest {
			slowest = elapsed
			slowestFeature = f
		}
	}

	// Measured, never asserted (see header): the confined/-race runner path has
	// clocked the slowest contract ~7 s where a warm dev box is <2 s, so a
	// wall-clock budget as a unit-test assertion flakes the gate (I5/DINV-3).
	t.Logf("largest/slowest live contract: %q at %v (NFR-1 reference target p95 ≤ %v; measured, not asserted)", slowestFeature, slowest.Round(time.Microsecond), perfCeiling)
}

// TestSweepPerformanceMeasurement records the live-corpus sweep cost (AP-29).
// The fixture suite and TestSweepMatchesPerFeatureVerdict cover correctness;
// avoid repeating this expensive measurement under the race detector.
func TestSweepPerformanceMeasurement(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("advisory whole-corpus measurement; correctness is covered by the sweep tests")
	}
	root, ok := findRepoRoot(t)
	if !ok {
		t.Skip("no go.mod above cwd — sweep performance is measured over the real repo only")
	}

	start := time.Now()
	res := Sweep(root)
	elapsed := time.Since(start)
	if res.ConfigError != "" {
		t.Fatalf("sweep failed closed: %s", res.ConfigError)
	}

	t.Logf("dogfood sweep: %d live contracts in %v (%d clean, %d advisory, %d blocking); advisory timing (AP-29)",
		res.Total, elapsed.Round(time.Millisecond), res.Clean, res.Advisory, res.Blocking)
}

// BenchmarkGateLargestLiveContract benchmarks a single gate run over the repo's
// largest live contract (by source-map + serving-design fan-in), for a precise
// per-op time a reviewer can track over releases (NFR-1 evidence). It resolves the
// largest contract at setup and times only the gate call. Skips outside the repo.
func BenchmarkGateLargestLiveContract(b *testing.B) {
	dir, err := os.Getwd()
	if err != nil {
		b.Skip("getwd failed")
	}
	var root string
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			root = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Skip("no go.mod above cwd — benchmark runs over the real repo only")
		}
		dir = parent
	}

	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		b.Fatalf("repo config does not parse: %v", cdef)
	}
	feature := largestLiveContract(b, root, cfg)
	if feature == "" {
		b.Skip("no live contracts in the repo")
	}
	sev := defaultSeverityMap()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = runResolved(root, feature, TriggerPlanningEntry, cfg, sev)
	}
}

// largestLiveContract returns the live feature whose folded source map is the
// largest by byte size — a stable proxy for "the repo's largest live contract"
// (more units + more serving designs → more gate work). Returns "" when the repo
// has no live contract.
func largestLiveContract(tb testing.TB, root string, cfg config.Config) string {
	tb.Helper()
	features, ferr := liveFeatures(root, cfg)
	if ferr != "" {
		tb.Fatalf("enumerating live features: %s", ferr)
	}
	var best string
	var bestSize int64
	for _, f := range features {
		p := filepath.Join(root, filepath.FromSlash(cfg.PRDs), f, "effective.source.json")
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			best = f
		}
	}
	return best
}
