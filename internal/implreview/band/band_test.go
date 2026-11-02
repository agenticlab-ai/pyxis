package band_test

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

// lanePolicy is a well-formed ratified risk policy whose AUTO classes exercise
// both band outcomes: "drive-doc" is in the band layer's default SKIP set (a
// docs-only trivial perimeter), and "fleet-task" is NOT (a code perimeter that
// still gets the standard four-lens pass). The floor is the real self-unlock
// minimum, so a floor hit is the lane's own hard-gated signal the band reads as
// deep. This is the LANE's classification — the band reads it, it does not
// re-derive risk.
func lanePolicy() *policy.Policy {
	return &policy.Policy{
		SchemaVersion: policy.SchemaVersion,
		RatifiedBy:    "@mdarabi",
		Floor: policy.Floor{
			HardGatedPaths:   policy.SelfUnlockMinPaths,
			HardGatedClasses: policy.SelfUnlockMinClasses,
		},
		AutoClasses: []policy.AutoClass{
			// A code perimeter — AUTO but NOT a band-skip class (standard review).
			{Name: "fleet-task", Paths: []string{"internal/carina/**", "cmd/carina/**"}},
			// A docs perimeter — AUTO and a band-skip class (skip review).
			{Name: "drive-doc", Paths: []string{"docs/wiki/**"}, RequiresReviewerVerdict: true},
		},
	}
}

// classifyDiff runs the REAL lane classifier over a diff, so a test's band
// selection is exercised against the lane's actual Verdict (not a hand-built
// one) — the FR-9 acceptance "band selection equals the lane's class on a
// fixture per class".
func classifyDiff(paths []string, classes ...string) classifier.Verdict {
	return classifier.Classify(
		classifier.Inputs{
			Diff:       classifier.Diff{Paths: paths, Classes: classes},
			Base:       "base0",
			Head:       "head0",
			CheckState: "checks0",
		},
		lanePolicy(),
		nil,
	)
}

// TestSelectPerLaneClass is the FR-9 corpus: a fixture per lane class, asserting
// the band the lane's class maps to, run over the REAL classifier verdict.
func TestSelectPerLaneClass(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		verdict    classifier.Verdict
		wantBand   band.Band
		wantSkip   string // the skipped:<class> marker, when a skip
		classHas   string // substring the recorded Class must contain (when set)
		reasonHas  string // substring the recorded Reason must contain
		wantLaneOK func(v classifier.Verdict) bool
	}{
		{
			name:      "floor hit (control plane) → DEEP",
			verdict:   classifyDiff([]string{"cmd/carina/main.go", ".github/workflows/ci.yml"}),
			wantBand:  report.BandDeep,
			classHas:  "path:.github/workflows/**",
			reasonHas: "deep-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Human && len(v.FloorHits) > 0
			},
		},
		{
			name:      "floor hit (governing-doc class) → DEEP",
			verdict:   classifyDiff([]string{"docs/wiki/entry.md"}, "governing-doc-edit"),
			wantBand:  report.BandDeep,
			classHas:  "class:governing-doc-edit",
			reasonHas: "deep-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Human && len(v.FloorHits) > 0
			},
		},
		{
			name:      "trivial AUTO class (drive-doc) → SKIP recording its class",
			verdict:   classifyDiff([]string{"docs/wiki/entry.md", "docs/wiki/other.md"}),
			wantBand:  report.BandSkip,
			wantSkip:  "skipped:drive-doc",
			classHas:  "drive-doc",
			reasonHas: "skip-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Auto && v.AutoClass == "drive-doc"
			},
		},
		{
			name:      "non-trivial AUTO class (fleet-task) → STANDARD",
			verdict:   classifyDiff([]string{"internal/carina/a.go", "cmd/carina/main.go"}),
			wantBand:  report.BandStandard,
			classHas:  "fleet-task",
			reasonHas: "standard-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Auto && v.AutoClass == "fleet-task"
			},
		},
		{
			name:      "HUMAN without a floor hit (out-of-perimeter) → STANDARD (fail-closed default)",
			verdict:   classifyDiff([]string{"internal/carina/a.go", "README.md"}),
			wantBand:  report.BandStandard,
			reasonHas: "standard-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Human && len(v.FloorHits) == 0
			},
		},
		{
			name:      "HUMAN ambiguous membership (two classes) → STANDARD",
			verdict:   classifyDiff([]string{"docs/wiki/entry.md", "internal/carina/a.go"}),
			wantBand:  report.BandStandard,
			reasonHas: "standard-band",
			wantLaneOK: func(v classifier.Verdict) bool {
				return v.Lane == classifier.Human && len(v.FloorHits) == 0
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// The fixture must actually be the lane class it claims — otherwise the
			// band assertion tests nothing.
			if tc.wantLaneOK != nil && !tc.wantLaneOK(tc.verdict) {
				t.Fatalf("fixture is not the intended lane class: %+v", tc.verdict)
			}

			d := band.Select(tc.verdict)

			if d.Band != tc.wantBand {
				t.Fatalf("band = %q, want %q (reason: %s)", d.Band, tc.wantBand, d.Reason)
			}
			if tc.wantSkip != "" {
				if d.SkipMarker != tc.wantSkip {
					t.Fatalf("skip marker = %q, want %q", d.SkipMarker, tc.wantSkip)
				}
				// The skip marker names its class (no silent caps).
				cls, ok := band.SkipClass(d.SkipMarker)
				if !ok || cls == "" {
					t.Fatalf("skip marker %q does not name a class", d.SkipMarker)
				}
			} else if d.SkipMarker != "" {
				t.Fatalf("non-skip band recorded a skip marker: %q", d.SkipMarker)
			}
			if tc.classHas != "" && !strings.Contains(d.Class, tc.classHas) {
				t.Fatalf("class = %q, want to contain %q", d.Class, tc.classHas)
			}
			if !strings.Contains(d.Reason, tc.reasonHas) {
				t.Fatalf("reason = %q, want to contain %q", d.Reason, tc.reasonHas)
			}
		})
	}
}

// TestSelectSkipMarkerNamesClass — every skip decision records a
// skipped:<class> marker whose class is the lane's AUTO class (no silent cap).
func TestSelectSkipMarkerNamesClass(t *testing.T) {
	t.Parallel()
	v := classifyDiff([]string{"docs/wiki/a.md"})
	if v.Lane != classifier.Auto || v.AutoClass != "drive-doc" {
		t.Fatalf("fixture not the drive-doc AUTO class: %+v", v)
	}
	d := band.Select(v)
	if d.Band != report.BandSkip {
		t.Fatalf("band = %q, want skip", d.Band)
	}
	cls, ok := band.SkipClass(d.SkipMarker)
	if !ok {
		t.Fatalf("skip marker %q is not a skipped:<class> marker", d.SkipMarker)
	}
	if cls != v.AutoClass {
		t.Fatalf("skip marker names class %q, want the lane's AUTO class %q", cls, v.AutoClass)
	}
}

func TestSkipClassRejectsIncompleteAndForeignMarkers(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{"", "skipped:", "skip:drive-doc"} {
		if class, ok := band.SkipClass(marker); ok || class != "" {
			t.Fatalf("SkipClass(%q) = %q, %v", marker, class, ok)
		}
	}
}

// TestSelectClassDataAbsentFailsClosedStandard — the PRD §5.4 rule "class-data
// absent ⇒ standard band, recorded": a verdict carrying no readable lane (a
// malformed/absent classification) maps to STANDARD with a recorded reason,
// never a guessed skip or deep.
func TestSelectClassDataAbsentFailsClosedStandard(t *testing.T) {
	t.Parallel()
	// A zero Verdict — no lane, no class, no floor hits: the class-data-absent
	// case. The band must not guess a depth.
	d := band.Select(classifier.Verdict{})
	if d.Band != report.BandStandard {
		t.Fatalf("class-data-absent band = %q, want standard (fail-closed)", d.Band)
	}
	if d.SkipMarker != "" {
		t.Fatalf("class-data-absent recorded a skip marker (guessed a skip): %q", d.SkipMarker)
	}
	if d.Reason == "" {
		t.Fatalf("class-data-absent standard band has no recorded reason")
	}
}

// TestNoReviewSideRiskDerivation is the FR-9 inspection leg made mechanical: the
// band's ONLY risk input is the classifier.Verdict. A verdict with the SAME lane
// class always yields the same band regardless of any other content the band
// could hypothetically inspect — because Select reads nothing but Lane,
// FloorHits, and AutoClass. Two verdicts identical in those three fields (but
// differing in Reason/Pinned) select the identical band: the band derives no
// risk of its own.
func TestNoReviewSideRiskDerivation(t *testing.T) {
	t.Parallel()

	a := classifier.Verdict{
		Lane:      classifier.Auto,
		AutoClass: "fleet-task",
		Reason:    "one recorded reason",
		Pinned:    classifier.Pinned{Base: "b1", Head: "h1"},
	}
	b := classifier.Verdict{
		Lane:      classifier.Auto,
		AutoClass: "fleet-task",
		Reason:    "a DIFFERENT recorded reason",
		Pinned:    classifier.Pinned{Base: "b2", Head: "h2", CheckState: "cs"},
	}
	da := band.Select(a)
	db := band.Select(b)
	if da.Band != db.Band {
		t.Fatalf("band depends on non-class content: %q vs %q — a review-side risk derivation exists", da.Band, db.Band)
	}
	if da.Band != report.BandStandard {
		t.Fatalf("fleet-task band = %q, want standard", da.Band)
	}
}
