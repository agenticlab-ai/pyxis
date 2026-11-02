package watch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultThresholds(t *testing.T) {
	d := DefaultThresholds()
	if d.Review != 2*time.Hour || d.CI != 30*time.Minute || d.Worker != 2*time.Hour || d.Checker != 0 {
		t.Fatalf("DefaultThresholds = %+v, want review=2h ci=30m worker=2h checker=unset", d)
	}
}

// T9 acceptance: "absent config yields the ruled defaults" — no wake: block
// at all, and no wake.stale subsection either, both mean the shipped
// defaults, not a parse error.
func TestParseThresholds_AbsentWakeBlockYieldsDefaults(t *testing.T) {
	src := `paths:
  prds: docs/requirements
plan:
  max_acceptance_criteria: 3
`
	got, err := ParseThresholds([]byte(src))
	if err != nil {
		t.Fatalf("ParseThresholds: %v", err)
	}
	if got != DefaultThresholds() {
		t.Fatalf("got %+v, want the defaults", got)
	}

	got, err = ParseThresholds([]byte("wake:\n"))
	if err != nil {
		t.Fatalf("ParseThresholds (empty wake block): %v", err)
	}
	if got != DefaultThresholds() {
		t.Fatalf("empty wake: block = %+v, want the defaults", got)
	}
}

func TestParseThresholds_PartialOverride(t *testing.T) {
	src := `wake:
  stale:
    review: 4h
`
	got, err := ParseThresholds([]byte(src))
	if err != nil {
		t.Fatalf("ParseThresholds: %v", err)
	}
	want := Thresholds{Review: 4 * time.Hour, CI: defaultCIStale, Worker: defaultWorkerStale, ReviewRecoveryBound: defaultReviewRecoveryBound, ReviewRecoveryRef: DefaultRecoveryEvidenceRef}
	if got != want {
		t.Fatalf("got %+v, want %+v (only review overridden)", got, want)
	}
}

func TestParseThresholds_AllFourKeys(t *testing.T) {
	src := `wake:
  stale:
    review: 3h
    ci: 15m
    worker: 90m
    checker: 45s
`
	got, err := ParseThresholds([]byte(src))
	if err != nil {
		t.Fatalf("ParseThresholds: %v", err)
	}
	want := Thresholds{Review: 3 * time.Hour, CI: 15 * time.Minute, Worker: 90 * time.Minute, Checker: 45 * time.Second, ReviewRecoveryBound: defaultReviewRecoveryBound, ReviewRecoveryRef: DefaultRecoveryEvidenceRef}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// T9 acceptance: "unknown keys are named errors".
func TestParseThresholds_UnknownKeyUnderStale(t *testing.T) {
	_, err := ParseThresholds([]byte("wake:\n  stale:\n    bogus: 1h\n"))
	if !errors.Is(err, ErrUnknownThresholdKey) {
		t.Fatalf("err = %v, want ErrUnknownThresholdKey", err)
	}
}

func TestParseThresholds_UnknownKeyUnderWake(t *testing.T) {
	_, err := ParseThresholds([]byte("wake:\n  bogus: yes\n"))
	if !errors.Is(err, ErrUnknownThresholdKey) {
		t.Fatalf("err = %v, want ErrUnknownThresholdKey", err)
	}
}

func TestParseThresholds_ReviewRecoveryPolicy(t *testing.T) {
	got, err := ParseThresholds([]byte("wake:\n  review_recovery_bound: 7\n  review_recovery_ref: refs/pyxis/custom-recovery\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ReviewRecoveryBound != 7 || got.ReviewRecoveryRef != "refs/pyxis/custom-recovery" {
		t.Fatalf("recovery policy = %+v", got)
	}
	if _, err := ParseThresholds([]byte("wake:\n  review_recovery_ref: refs/heads/main\n")); !errors.Is(err, ErrInvalidThreshold) {
		t.Fatalf("unsafe recovery ref error = %v", err)
	}
}

func TestParseThresholds_InvalidDuration(t *testing.T) {
	cases := []string{"notaduration", "0h", "-5m", ""}
	for _, v := range cases {
		_, err := ParseThresholds([]byte("wake:\n  stale:\n    review: " + v + "\n"))
		if !errors.Is(err, ErrInvalidThreshold) {
			t.Fatalf("review=%q: err = %v, want ErrInvalidThreshold", v, err)
		}
	}
}

func TestParseThresholds_MalformedStructure(t *testing.T) {
	cases := map[string]string{
		"stale as a scalar": "wake:\n  stale: soon\n",
		"wake as a scalar":  "wake: soon\n",
		"duplicate wake":    "wake:\n  stale:\n    review: 1h\nwake:\n  stale:\n    ci: 1h\n",
		"tab indentation":   "wake:\n\tstale:\n    review: 1h\n",
		"odd indentation":   "wake:\n   stale:\n    review: 1h\n",
		"key under review":  "wake:\n  stale:\n    review:\n      nested: 1h\n",
		"stray non-mapping": "wake:\n  stale:\n    review 1h\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseThresholds([]byte(src)); err == nil {
				t.Fatalf("%s: want a parse error, got nil", name)
			}
		})
	}
}

// A foreign consumer's block on the shared surface (paths:, executor:,
// channel:) coexists with wake: exactly as it does for docgate's own parser
// and internal/executor/config (TestForeignBlockCoexists's sibling case).
func TestParseThresholds_ForeignSectionsCoexist(t *testing.T) {
	src := `
schema_version: pyxis-config/v2
paths:
  prds: docs/requirements
executor:
  routing:
    default: claude
  caps:
    worker_turns: 60
channel:
  tick_interval: 1h
  staleness_bound: 24h
wake:
  stale:
    ci: 10m
`
	got, err := ParseThresholds([]byte(src))
	if err != nil {
		t.Fatalf("ParseThresholds: %v", err)
	}
	want := Thresholds{Review: defaultReviewStale, CI: 10 * time.Minute, Worker: defaultWorkerStale, ReviewRecoveryBound: defaultReviewRecoveryBound, ReviewRecoveryRef: DefaultRecoveryEvidenceRef}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLoadThresholds_AbsentFileYieldsDefaults(t *testing.T) {
	got, err := LoadThresholds(t.TempDir())
	if err != nil {
		t.Fatalf("LoadThresholds: %v", err)
	}
	if got != DefaultThresholds() {
		t.Fatalf("got %+v, want the defaults", got)
	}
}

func TestLoadThresholds_ReadsRepoConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "wake:\n  stale:\n    worker: 6h\n"
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadThresholds(root)
	if err != nil {
		t.Fatalf("LoadThresholds: %v", err)
	}
	if got.Worker != 6*time.Hour {
		t.Fatalf("Worker = %s, want 6h", got.Worker)
	}
}

func TestThresholds_CheckerBound(t *testing.T) {
	cadence := 10 * time.Second
	unset := Thresholds{}
	if got := unset.CheckerBound(cadence); got != 30*time.Second {
		t.Fatalf("unset CheckerBound = %s, want 3x cadence = 30s", got)
	}
	overridden := Thresholds{Checker: 5 * time.Minute}
	if got := overridden.CheckerBound(cadence); got != 5*time.Minute {
		t.Fatalf("overridden CheckerBound = %s, want the configured 5m", got)
	}
}

func TestThresholds_ForKind(t *testing.T) {
	th := Thresholds{Review: time.Hour, CI: 2 * time.Hour, Worker: 3 * time.Hour}
	cases := []struct {
		kind WaitKind
		want time.Duration
	}{
		{WaitReview, time.Hour},
		{WaitCI, 2 * time.Hour},
		{WaitWorker, 3 * time.Hour},
		{WaitNone, 0},
	}
	for _, c := range cases {
		if got := th.forKind(c.kind); got != c.want {
			t.Errorf("forKind(%q) = %s, want %s", c.kind, got, c.want)
		}
	}
}

// design:wake-watch-v2 §3.8: "the primitive relation inherited from
// wake-channel-v1 holds" — every bound (the checker's own resolved against
// cadence) must strictly exceed the cadence serving it.
func TestThresholds_ValidateRelation(t *testing.T) {
	cadence := time.Minute
	if err := DefaultThresholds().Validate(cadence); err != nil {
		t.Fatalf("shipped defaults against a 1m cadence: %v", err)
	}
	tooTight := Thresholds{Review: 2 * time.Hour, CI: 30 * time.Second, Worker: 2 * time.Hour}
	if err := tooTight.Validate(cadence); err == nil {
		t.Fatal("a ci bound under the cadence must refuse, got nil")
	}
	tightChecker := Thresholds{Review: 2 * time.Hour, CI: 30 * time.Minute, Worker: 2 * time.Hour, Checker: 30 * time.Second}
	if err := tightChecker.Validate(cadence); err == nil {
		t.Fatal("a checker override under the cadence must refuse, got nil")
	}
}
