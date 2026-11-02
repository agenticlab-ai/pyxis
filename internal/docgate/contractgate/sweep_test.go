package contractgate

import (
	"path/filepath"
	"testing"
)

// The dogfood sweep tests (plan T8; design contract-gate-v1 §3.11): the sweep
// discovers every LIVE feature contract, gates each under the planning-entry hard
// bar, and reports a per-feature summary with the named defects on a bounce —
// read-only, deterministic, and fail-closed on a bad config/severity-map. These
// exercise the sweep over fixture repos (each carrying one `demo` contract); the
// real-repo dogfood run is exercised by the DoD command in the PR, and its
// disposition is recorded at docs/plans/contract-gate/dogfood-disposition.md.

// sweepFixture runs the sweep over a fixture repo and snapshots the tree before/
// after to prove the sweep mutates NOTHING (DINV-1) — the read-only guarantee is
// verified, not asserted.
func sweepFixture(t *testing.T, fixture string) SweepResult {
	t.Helper()
	root := filepath.Join("testdata", fixture)
	before := snapshot(t, root)
	res := Sweep(root)
	after := snapshot(t, root)
	assertUnchanged(t, before, after)
	return res
}

// TestSweepDiscoversAndGatesLiveContracts proves the sweep over each fixture repo
// discovers its one live contract (demo), gates it, and reports the right per-
// feature disposition — clean on wellformed, blocking on each defect fixture, with
// the named check surfaced. The tallies (Clean/Advisory/Blocking) partition Total.
func TestSweepDiscoversAndGatesLiveContracts(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantClass   Class
		wantBlocked bool
		wantCheck   string // the check the demo contract reds (empty on clean)
	}{
		{"clean contract passes", "wellformed", ClassClean, false, ""},
		{"currency defect blocks", "stale-serve", ClassStale, true, "currency"},
		{"coverage defect blocks (planning-entry: all blocking)", "uncovered-must", ClassIncomplete, true, "inverse-coverage"},
		{"link defect blocks", "dangling-citation", ClassUnresolved, true, "link"},
		{"structural defect blocks", "struct-nofit", ClassIncomplete, true, "structural-completeness"},
		{"compilability defect blocks", "compil-orphan-serves", ClassMalformed, true, "compilability"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := sweepFixture(t, tc.fixture)

			if res.ConfigError != "" {
				t.Fatalf("unexpected config error: %s", res.ConfigError)
			}
			if res.Trigger != TriggerPlanningEntry {
				t.Errorf("sweep trigger = %q, want planning-entry (the hard bar)", res.Trigger)
			}
			if res.Total != 1 {
				t.Fatalf("sweep found %d live contracts, want 1 (demo): %+v", res.Total, res.Features)
			}
			if len(res.Features) != 1 {
				t.Fatalf("sweep reported %d feature verdicts, want 1", len(res.Features))
			}
			fv := res.Features[0]
			if fv.Feature != "demo" {
				t.Errorf("feature = %q, want demo", fv.Feature)
			}
			if fv.Class != tc.wantClass {
				t.Errorf("feature class = %q, want %q (reason: %q)", fv.Class, tc.wantClass, fv.Reason)
			}
			if fv.Blocking != tc.wantBlocked {
				t.Errorf("feature blocking = %v, want %v", fv.Blocking, tc.wantBlocked)
			}

			// The tallies partition Total exactly.
			if res.Clean+res.Advisory+res.Blocking != res.Total {
				t.Errorf("tallies do not partition Total: %d+%d+%d != %d", res.Clean, res.Advisory, res.Blocking, res.Total)
			}
			if res.Failed() != tc.wantBlocked {
				t.Errorf("Failed() = %v, want %v", res.Failed(), tc.wantBlocked)
			}

			if tc.wantClass == ClassClean {
				if len(fv.Defects) != 0 {
					t.Errorf("clean feature carries %d defects, want 0: %+v", len(fv.Defects), fv.Defects)
				}
				if res.Clean != 1 || res.Blocking != 0 {
					t.Errorf("clean sweep tallies wrong: clean=%d blocking=%d", res.Clean, res.Blocking)
				}
				return
			}

			// A bouncing feature names its defects, including the offending check.
			if len(fv.Defects) == 0 {
				t.Fatalf("bouncing feature named zero defects")
			}
			foundCheck := false
			for _, d := range fv.Defects {
				if d.Check == "" || d.Offender == "" || d.Reason == "" {
					t.Errorf("sweep defect missing a field: %+v", d)
				}
				if d.Check == tc.wantCheck {
					foundCheck = true
				}
			}
			if !foundCheck {
				t.Errorf("sweep did not name the failed check %q; defects: %+v", tc.wantCheck, fv.Defects)
			}
			if res.Blocking != 1 {
				t.Errorf("blocking tally = %d, want 1", res.Blocking)
			}
		})
	}
}

// TestSweepSkipsUnfoldedLedgers proves the sweep is over LIVE contracts only: a
// ledger directory with no folded source map (an unfolded contract) is NOT gated —
// it contributes no feature verdict rather than a spurious configuration-error.
// The `missing` fixture's demo ledger has a 000-base.md but no effective.source.json.
func TestSweepSkipsUnfoldedLedgers(t *testing.T) {
	res := sweepFixture(t, "missing")
	if res.ConfigError != "" {
		t.Fatalf("unexpected config error: %s", res.ConfigError)
	}
	if res.Total != 0 {
		t.Errorf("sweep gated %d contracts, want 0 (the only ledger is unfolded): %+v", res.Total, res.Features)
	}
	if res.Failed() {
		t.Errorf("a sweep over only-unfolded ledgers must not fail (no live contract to block)")
	}
}

// TestSweepFailsClosedOnMalformedSeverityMap proves the whole sweep fails closed
// (UB-6) when the severity-map policy is malformed: no per-feature verdicts, and
// Failed() is true with the policy defect named — mirroring the per-feature and
// continuous fail-closed behavior.
func TestSweepFailsClosedOnMalformedSeverityMap(t *testing.T) {
	res := sweepFixture(t, "sevmap-malformed")
	if res.ConfigError == "" {
		t.Fatalf("a malformed severity-map did not fail the sweep closed (UB-6)")
	}
	if !res.Failed() {
		t.Errorf("Failed() = false on a malformed severity-map — must fail closed")
	}
	if len(res.Features) != 0 {
		t.Errorf("produced %d feature verdicts despite a fail-closed policy", len(res.Features))
	}
}

// TestSweepMatchesPerFeatureVerdict proves the sweep never diverges from a direct
// `contract-gate <feature>` run: for each fixture, the sweep's FeatureVerdict
// carries the same class, blocking disposition, and defect count as RunTrigger
// produces for that feature under the same (planning-entry) trigger. The sweep is
// a projection of the per-feature verdicts, not a second judgment.
func TestSweepMatchesPerFeatureVerdict(t *testing.T) {
	for _, fixture := range []string{"wellformed", "stale-serve", "uncovered-must", "dangling-citation", "struct-nofit", "compil-orphan-serves"} {
		t.Run(fixture, func(t *testing.T) {
			root := filepath.Join("testdata", fixture)
			res := Sweep(root)
			if res.Total != 1 {
				t.Fatalf("expected 1 live contract, got %d", res.Total)
			}
			fv := res.Features[0]

			// The authoritative per-feature verdict for the same feature + trigger.
			v := RunTrigger(root, "demo", TriggerPlanningEntry)
			if fv.Class != v.Class {
				t.Errorf("sweep class %q != per-feature class %q", fv.Class, v.Class)
			}
			if fv.Blocking != v.Blocked() {
				t.Errorf("sweep blocking %v != per-feature Blocked() %v", fv.Blocking, v.Blocked())
			}
			// The sweep's defect set equals the verdict's failed-check defects.
			wantDefects := len(bounceDefects(v))
			if len(fv.Defects) != wantDefects {
				t.Errorf("sweep named %d defects, per-feature verdict has %d", len(fv.Defects), wantDefects)
			}
		})
	}
}
