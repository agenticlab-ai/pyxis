package contractgate

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// TestAffectedFeatures proves the continuous surface's affected-feature
// derivation (FR-9; §3.10): a changed PRD-ledger file names its feature directly,
// a changed design names every feature it SERVES (via its resolved Serves), and a
// non-contract path contributes nothing. Run over the wellformed fixture (whose
// design demo-v1 serves prd:demo/…), with the default config paths.
func TestAffectedFeatures(t *testing.T) {
	root := filepath.Join("testdata", "wellformed")
	cfg := config.Defaults()

	tests := []struct {
		name    string
		changed []string
		want    []string
	}{
		{
			name:    "a changed PRD-ledger file names its feature",
			changed: []string{"docs/prds/demo/000-base.md"},
			want:    []string{"demo"},
		},
		{
			name:    "a changed source map names its feature",
			changed: []string{"docs/prds/demo/effective.source.json"},
			want:    []string{"demo"},
		},
		{
			name:    "a changed design names the feature(s) it serves",
			changed: []string{"docs/tech-design/demo-v1.md"},
			want:    []string{"demo"},
		},
		{
			name:    "a non-contract path contributes nothing",
			changed: []string{"cmd/docgate/main.go", "README.md"},
			want:    nil,
		},
		{
			name:    "the generated design index is not a design",
			changed: []string{"docs/tech-design/index.md", "docs/tech-design/index.json"},
			want:    nil,
		},
		{
			name:    "a sibling-prefixed dir is not the PRD root",
			changed: []string{"docs/prdsX/demo/000-base.md"},
			want:    nil,
		},
		{
			name:    "mixed changes dedupe to the affected feature set",
			changed: []string{"docs/prds/demo/000-base.md", "docs/tech-design/demo-v1.md", "go.mod"},
			want:    []string{"demo"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := affectedFeatures(root, cfg, tc.changed)
			if !sameStrings(got, tc.want) {
				t.Errorf("affectedFeatures(%v) = %v, want %v", tc.changed, got, tc.want)
			}
		})
	}
}

// sameStrings compares two string slices treating nil and empty as equal (an
// empty affected-feature set is idiomatically either).
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestContinuousGatesEachFeatureOnDocPRTrigger proves the continuous run gates
// each affected feature under the contract-doc-pr split (FR-9): a clean contract
// passes non-blocking; a currency/link/structural defect BLOCKS; an uncovered-
// Must-only bounce is advisory (surfaced, non-blocking). The changed-file input
// derives the feature; the trigger is always contract-doc-pr.
func TestContinuousGatesEachFeatureOnDocPRTrigger(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		changed     []string
		wantFeats   []string
		wantBlocked bool
		wantClass   Class // the single feature's verdict class
	}{
		{
			name:        "a clean contract passes non-blocking",
			fixture:     "wellformed",
			changed:     []string{"docs/prds/demo/000-base.md"},
			wantFeats:   []string{"demo"},
			wantBlocked: false,
			wantClass:   ClassClean,
		},
		{
			name:        "a structural defect blocks the context",
			fixture:     "struct-nofit",
			changed:     []string{"docs/prds/demo/effective.source.json"},
			wantFeats:   []string{"demo"},
			wantBlocked: true,
			wantClass:   ClassIncomplete,
		},
		{
			name:        "a currency defect blocks the context",
			fixture:     "stale-serve",
			changed:     []string{"docs/tech-design/demo-v1.md"},
			wantFeats:   []string{"demo"},
			wantBlocked: true,
			wantClass:   ClassStale,
		},
		{
			name:        "a link defect blocks the context",
			fixture:     "dangling-citation",
			changed:     []string{"docs/prds/demo/000-base.md"},
			wantFeats:   []string{"demo"},
			wantBlocked: true,
			wantClass:   ClassUnresolved,
		},
		{
			name:        "an uncovered-Must-only bounce is advisory (non-blocking)",
			fixture:     "uncovered-must",
			changed:     []string{"docs/prds/demo/effective.source.json"},
			wantFeats:   []string{"demo"},
			wantBlocked: false,
			wantClass:   ClassIncomplete,
		},
		{
			name:        "no contract docs changed is a clean no-op",
			fixture:     "wellformed",
			changed:     []string{"cmd/docgate/main.go"},
			wantFeats:   nil,
			wantBlocked: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join("testdata", tc.fixture)
			before := snapshot(t, root)
			res := Continuous(root, tc.changed)
			after := snapshot(t, root)
			assertUnchanged(t, before, after) // read-only (DINV-1)

			if res.ConfigError != "" {
				t.Fatalf("unexpected config error: %s", res.ConfigError)
			}
			if res.Trigger != TriggerContractDocPR {
				t.Errorf("continuous trigger = %q, want contract-doc-pr", res.Trigger)
			}
			if !sameStrings(res.Features, tc.wantFeats) {
				t.Errorf("features = %v, want %v", res.Features, tc.wantFeats)
			}
			if res.Blocking != tc.wantBlocked {
				t.Errorf("Blocking = %v, want %v", res.Blocking, tc.wantBlocked)
			}
			if res.Failed() != tc.wantBlocked {
				t.Errorf("Failed() = %v, want %v", res.Failed(), tc.wantBlocked)
			}
			if len(tc.wantFeats) == 1 {
				if len(res.Verdicts) != 1 {
					t.Fatalf("expected 1 verdict, got %d", len(res.Verdicts))
				}
				v := res.Verdicts[0]
				if v.Class != tc.wantClass {
					t.Errorf("verdict class = %q, want %q", v.Class, tc.wantClass)
				}
				if v.Trigger != TriggerContractDocPR {
					t.Errorf("per-feature verdict trigger = %q, want contract-doc-pr", v.Trigger)
				}
			}
			if len(tc.wantFeats) == 0 && len(res.Verdicts) != 0 {
				t.Errorf("a no-op run produced %d verdicts, want 0", len(res.Verdicts))
			}
		})
	}
}

// TestContinuousFailsClosedOnMalformedSeverityMap proves the continuous context
// fails closed (UB-6) when the severity-map policy is malformed: no per-feature
// verdicts are produced and Failed() is true, with the policy defect named.
func TestContinuousFailsClosedOnMalformedSeverityMap(t *testing.T) {
	root := filepath.Join("testdata", "sevmap-malformed")
	res := Continuous(root, []string{"docs/prds/demo/000-base.md"})
	if res.ConfigError == "" {
		t.Fatalf("a malformed severity-map did not produce a config error (UB-6)")
	}
	if !res.Failed() {
		t.Errorf("Failed() = false on a malformed severity-map — must fail closed")
	}
	if len(res.Verdicts) != 0 {
		t.Errorf("produced %d per-feature verdicts despite a fail-closed policy", len(res.Verdicts))
	}
}

// TestContinuousDesignServesDerivesFeature proves the design→feature mapping on
// the file-driven flip fixture: changing ONLY the design derives the feature it
// serves and gates it. (The fixture's design serves prd:demo/…, so demo is
// affected.) A defensive proof that Serves-based derivation is exercised end to
// end, not only the direct PRD-path case.
func TestContinuousDesignServesDerivesFeature(t *testing.T) {
	root := filepath.Join("testdata", "wellformed")
	res := Continuous(root, []string{"docs/tech-design/demo-v1.md"})
	if res.ConfigError != "" {
		t.Fatalf("config error: %s", res.ConfigError)
	}
	if !reflect.DeepEqual(res.Features, []string{"demo"}) {
		t.Fatalf("design change derived features %v, want [demo] (via Serves)", res.Features)
	}
}
