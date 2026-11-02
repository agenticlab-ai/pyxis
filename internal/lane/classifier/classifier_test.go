package classifier

import (
	"bytes"
	"errors"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// testPolicy is the well-formed ratified policy the corpus classifies against:
// the real self-unlock-minimum floor (reused verbatim from the policy package,
// so the floor the classifier gates on is exactly the design-bound minimum),
// two disjoint AUTO classes — fleet-task over code, drive-doc over docs — and
// the kill switch off.
func testPolicy() *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   policy.SelfUnlockMinPaths,
			HardGatedClasses: policy.SelfUnlockMinClasses,
		},
		AutoClasses: []policy.AutoClass{
			{Name: "fleet-task", Paths: []string{"cmd/**", "internal/**"}},
			{Name: "drive-doc", Paths: []string{"docs/**"}, RequiresReviewerVerdict: true},
		},
	}
}

// diff is a terse Inputs builder — the pinned revisions are fixed strings so
// every fixture also exercises the pinned-input record.
func in(paths []string, classes ...string) Inputs {
	return Inputs{
		Diff:       Diff{Paths: paths, Classes: classes},
		Base:       "base0",
		Head:       "head0",
		CheckState: "checks0",
	}
}

// TestClassificationCorpus is the table-driven fixture corpus: every proposal
// receives exactly one recorded lane, every unclassifiable/fault-injected case
// lands HUMAN with its reason recorded, and each shipped AUTO class carries a
// whole-diff-inside/out-of-perimeter fixture pair (A-1; T2 acceptance).
func TestClassificationCorpus(t *testing.T) {
	p := testPolicy()

	cases := []struct {
		name      string
		in        Inputs
		wantLane  Lane
		wantClass string   // AUTO class name when AUTO
		wantHits  []string // floor hit tokens, when the floor fired
		reason    string   // substring the recorded reason must contain
	}{
		// --- fleet-task class: the whole-diff-inside / out-of-perimeter pair ---
		{
			name:      "fleet-task whole diff inside → AUTO",
			in:        in([]string{"internal/foo/a.go", "cmd/tool/main.go"}),
			wantLane:  Auto,
			wantClass: "fleet-task",
			reason:    "contained by AUTO class fleet-task",
		},
		{
			name:     "fleet-task out-of-perimeter hunk → HUMAN",
			in:       in([]string{"internal/foo/a.go", "README.md"}),
			wantLane: Human,
			reason:   "no AUTO class perimeter contains the whole diff",
		},
		// --- drive-doc class: the whole-diff-inside / out-of-perimeter pair ---
		{
			name:      "drive-doc whole diff inside → AUTO",
			in:        in([]string{"docs/wiki/entry.md", "docs/research/study.md"}),
			wantLane:  Auto,
			wantClass: "drive-doc",
			reason:    "contained by AUTO class drive-doc",
		},
		{
			name:     "drive-doc diff spanning two classes → HUMAN",
			in:       in([]string{"docs/wiki/entry.md", "cmd/tool/main.go"}),
			wantLane: Human,
			reason:   "no AUTO class perimeter contains the whole diff",
		},
		// --- floor terminal, regardless of class membership (A-2/A-3) ---
		{
			name:     "floor hit inside a class perimeter → HUMAN, floor terminal",
			in:       in([]string{"internal/lane/classifier/x.go"}), // inside fleet-task's internal/** but floor-gated
			wantLane: Human,
			wantHits: []string{"path:internal/lane/**"},
			reason:   "floor hit",
		},
		{
			name:     "floor hit control plane → HUMAN",
			in:       in([]string{"cmd/tool/main.go", ".github/workflows/ci.yml"}),
			wantLane: Human,
			wantHits: []string{"path:.github/workflows/**"},
			reason:   "floor hit",
		},
		{
			name:     "floor hit build input go.mod (transitive-trust anchor) → HUMAN",
			in:       in([]string{"go.mod"}),
			wantLane: Human,
			wantHits: []string{"path:go.mod"},
			reason:   "floor hit",
		},
		{
			name:     "floor hit by diff-derived change class → HUMAN",
			in:       in([]string{"docs/wiki/entry.md"}, "governing-doc-edit"),
			wantLane: Human,
			wantHits: []string{"class:governing-doc-edit"},
			reason:   "floor hit",
		},
		// --- fail-closed defaults ---
		{
			name:     "empty diff → fail-closed HUMAN",
			in:       in(nil),
			wantLane: Human,
			reason:   "no parsed paths",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Classify(tc.in, p, nil)

			// DINV-1: exactly one lane, always a recorded reason.
			if v.Lane != Auto && v.Lane != Human {
				t.Fatalf("lane = %q, want AUTO or HUMAN", v.Lane)
			}
			if v.Reason == "" {
				t.Fatalf("verdict has no recorded reason")
			}
			if v.Lane != tc.wantLane {
				t.Fatalf("lane = %q, want %q (reason: %s)", v.Lane, tc.wantLane, v.Reason)
			}
			if tc.wantClass != "" && v.AutoClass != tc.wantClass {
				t.Fatalf("auto class = %q, want %q", v.AutoClass, tc.wantClass)
			}
			if tc.wantLane == Auto && len(v.FloorHits) != 0 {
				t.Fatalf("AUTO verdict recorded floor hits: %v", v.FloorHits)
			}
			if !contains(v.Reason, tc.reason) {
				t.Fatalf("reason = %q, want substring %q", v.Reason, tc.reason)
			}
			for _, h := range tc.wantHits {
				if !hasHit(v.FloorHits, h) {
					t.Fatalf("floor hits = %v, want to include %q", v.FloorHits, h)
				}
			}
			// The pinned inputs ride every verdict (the pinned-input record).
			if v.Pinned.Base != "base0" || v.Pinned.Head != "head0" ||
				v.Pinned.CheckState != "checks0" || v.Pinned.PolicyVersion != policy.SchemaVersion {
				t.Fatalf("pinned inputs not recorded: %+v", v.Pinned)
			}
		})
	}
}

// TestFloorTerminalRegardlessOfSignal — a floor-hit proposal routes HUMAN at
// full rate regardless of any downgrade opinion: the intelligence stage is
// never consulted on a floor-hit diff, so a downgrader that would "act" cannot
// flip a floor hit, and (structurally) it can only ever downgrade, never
// upgrade (A-3, DINV-2).
func TestFloorTerminalRegardlessOfSignal(t *testing.T) {
	p := testPolicy()
	// A downgrader that fails the test if the classifier ever calls it on a
	// HUMAN path — the floor must be terminal before the stage is reached.
	guard := downgraderFunc(func(Diff) (bool, string, error) {
		t.Fatalf("intelligence stage consulted on a floor-hit proposal")
		return false, "", nil
	})
	v := Classify(in([]string{"internal/lane/classifier/x.go"}), p, guard)
	if v.Lane != Human {
		t.Fatalf("floor-hit proposal routed %q, want HUMAN", v.Lane)
	}
	if !hasHit(v.FloorHits, "path:internal/lane/**") {
		t.Fatalf("floor hit not named: %v", v.FloorHits)
	}
}

// TestDowngradeOnlyStage — the configured intelligence stage may only turn an
// AUTO candidate HUMAN; its downgrade, ambiguity/error, and no-op branches all
// behave per the fail-closed contract, and a nil stage leaves AUTO intact (FR-4).
func TestDowngradeOnlyStage(t *testing.T) {
	p := testPolicy()
	autoDiff := in([]string{"internal/foo/a.go"})

	if v := Classify(autoDiff, p, nil); v.Lane != Auto {
		t.Fatalf("nil downgrader: lane = %q, want AUTO", v.Lane)
	}

	noop := downgraderFunc(func(Diff) (bool, string, error) { return false, "", nil })
	if v := Classify(autoDiff, p, noop); v.Lane != Auto || v.AutoClass != "fleet-task" {
		t.Fatalf("no-op downgrader: %+v, want AUTO fleet-task", v)
	}

	down := downgraderFunc(func(Diff) (bool, string, error) { return true, "smells risky", nil })
	if v := Classify(autoDiff, p, down); v.Lane != Human || !contains(v.Reason, "smells risky") {
		t.Fatalf("downgrading stage: %+v, want HUMAN naming the reason", v)
	}

	boom := downgraderFunc(func(Diff) (bool, string, error) { return false, "", errors.New("timeout") })
	if v := Classify(autoDiff, p, boom); v.Lane != Human || !contains(v.Reason, "timeout") {
		t.Fatalf("erroring stage: %+v, want fail-closed HUMAN naming the error", v)
	}
}

// TestFailClosedOnNoPolicyAndKill — an unloadable policy and an engaged kill
// switch both route every proposal HUMAN with the reason recorded (DINV-8, A-4).
func TestFailClosedOnNoPolicyAndKill(t *testing.T) {
	autoDiff := in([]string{"internal/foo/a.go"})

	if v := Classify(autoDiff, nil, nil); v.Lane != Human || !contains(v.Reason, "no risk policy") {
		t.Fatalf("nil policy: %+v, want HUMAN naming the missing policy", v)
	}

	killed := testPolicy()
	killed.KillSwitch = true
	if v := Classify(autoDiff, killed, nil); v.Lane != Human || !contains(v.Reason, "kill switch") {
		t.Fatalf("kill switch: %+v, want HUMAN-only", v)
	}
}

// TestAmbiguousMultiClassMembership — a diff whose whole contents fall inside
// more than one AUTO class's perimeter is ambiguous and fails closed to HUMAN,
// never AUTO under an arbitrary pick (DINV-1).
func TestAmbiguousMultiClassMembership(t *testing.T) {
	p := testPolicy()
	// Two overlapping classes both fully contain a docs/wiki diff.
	p.AutoClasses = []policy.AutoClass{
		{Name: "broad-doc", Paths: []string{"docs/**"}},
		{Name: "wiki-doc", Paths: []string{"docs/wiki/**"}},
	}
	v := Classify(in([]string{"docs/wiki/entry.md"}), p, nil)
	if v.Lane != Human || !contains(v.Reason, "multiple AUTO classes") {
		t.Fatalf("ambiguous membership: %+v, want fail-closed HUMAN", v)
	}
}

// TestDeterministicReplay — the same pinned inputs (and reconstructed diff)
// reproduce a byte-identical verdict record across runs, including after the
// proposal would have merged: Classify holds no hidden state (DINV-4).
func TestDeterministicReplay(t *testing.T) {
	p := testPolicy()
	corpus := []Inputs{
		in([]string{"internal/foo/a.go", "cmd/tool/main.go"}),
		in([]string{"docs/wiki/entry.md"}),
		in([]string{"internal/lane/classifier/x.go"}),
		in([]string{".github/workflows/ci.yml"}),
		in([]string{"docs/wiki/entry.md"}, "governing-doc-edit"),
		in(nil),
	}
	for _, c := range corpus {
		first := Classify(c, p, nil).Record()
		// Replay: the pinned base+head reconstruct the same diff; re-run.
		replay := Classify(c, p, nil).Record()
		if !bytes.Equal(first, replay) {
			t.Fatalf("verdict not byte-identical on replay:\n first=%s\nreplay=%s", first, replay)
		}
	}
}

// --- test helpers ---

type downgraderFunc func(Diff) (bool, string, error)

func (f downgraderFunc) Downgrade(d Diff) (bool, string, error) { return f(d) }

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }

func hasHit(hits []string, want string) bool {
	for _, h := range hits {
		if h == want {
			return true
		}
	}
	return false
}
