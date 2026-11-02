package checkpoint_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/checkpoint"
	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// fullyCovered builds a harvest.Metrics with 100% coverage over `universe`
// proposals (M1 met) and the given precision/nit corpus. It is the fixture the
// checkpoint reads — the checkpoint CONSUMES harvest.Metrics; it never recomputes
// a metric, so a test constructs the corpus reading the harvest would produce.
func fullyCovered(universe, blockingDisposed, blockingAccepted, reviews, rejected int) harvest.Metrics {
	return harvest.Metrics{
		Universe:         universe,
		Covered:          universe, // 100% coverage
		BlockingDisposed: blockingDisposed,
		BlockingAccepted: blockingAccepted,
		Reviews:          reviews,
		OperatorRejected: rejected,
	}
}

// TestCheckpointComputesFromHarvestedCorpus — DOD-4: the checkpoint computes
// coverage, floor-gated precision, and the nit rate FROM the harvested corpus and
// emits the EVT-5 record. A corpus over the floor with everything clearing lands
// ship-rule-met; the EVT-5 fields mirror the harvest exactly.
func TestCheckpointComputesFromHarvestedCorpus(t *testing.T) {
	t.Parallel()

	// 10 reviews, 10 proposals all covered; 6 operator-disposed blocking findings
	// (≥ the floor of 5), 6 accepted → precision 1.0; nit p50 0.
	m := fullyCovered(10, 6, 6, 10, 0)
	rec := checkpoint.Evaluate(checkpoint.Inputs{
		Reviews: 10,
		Metrics: m,
		M3Green: true,
		NitP50:  0,
	})

	if rec.Outcome != checkpoint.OutcomeShipRuleMet {
		t.Fatalf("outcome = %q, want ship-rule-met (hold reason: %q)", rec.Outcome, rec.HoldReason)
	}
	if !rec.ShipRuleMet {
		t.Fatalf("ShipRuleMet = false, want true")
	}
	// The EVT-5 fields are read FROM the harvest, not recomputed here.
	if rec.Coverage != 1.0 {
		t.Errorf("Coverage = %v, want 1.0 (M1 met)", rec.Coverage)
	}
	if rec.BlockingDisposed != 6 {
		t.Errorf("BlockingDisposed = %d, want 6 (from the harvest)", rec.BlockingDisposed)
	}
	if !rec.PrecisionOverFloor || rec.Precision == nil {
		t.Fatalf("precision not over the floor; want it present (6 ≥ floor 5)")
	}
	if *rec.Precision != 1.0 {
		t.Errorf("Precision = %v, want 1.0", *rec.Precision)
	}
	if rec.Reviews != 10 {
		t.Errorf("Reviews = %d, want 10", rec.Reviews)
	}
	// The checkpoint EVALUATES eligibility; it never promotes as a side effect.
	if rec.Promoted {
		t.Errorf("Promoted = true from Evaluate; want false (promotion is the operator's recorded act)")
	}
}

// TestBelowFloorWindowIsInconclusiveAndExtends — the DOD-4 heart: a below-floor
// window is INCONCLUSIVE → the window EXTENDS, never a vacuous pass. Even when
// every accepted-so-far disposition is accepted (a naive precision of 1.0) and
// coverage + evals are perfect, a window under the disposition floor does NOT
// ship — precision is not yet a basis, so the record carries a NIL precision and
// the outcome is extend-window.
func TestBelowFloorWindowIsInconclusiveAndExtends(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		blockingDisposed int
		blockingAccepted int
	}{
		{"one-below-floor", 4, 4},      // 4 < 5: one short of the floor, all accepted
		{"single-disposed", 1, 1},      // far below the floor, a naive 1.0 precision
		{"zero-blocking-window", 0, 0}, // a ZERO-blocking window: holds, never a vacuous pass
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// 100% coverage, evals green, nit clean — every OTHER term is perfect, so
			// the ONLY thing that can hold the ship rule is the unmet floor.
			m := fullyCovered(10, tc.blockingDisposed, tc.blockingAccepted, 10, 0)
			rec := checkpoint.Evaluate(checkpoint.Inputs{
				Reviews: 10,
				Metrics: m,
				M3Green: true,
				NitP50:  0,
			})

			if rec.Outcome != checkpoint.OutcomeExtendWindow {
				t.Fatalf("outcome = %q, want extend-window (below the disposition floor is inconclusive)", rec.Outcome)
			}
			if rec.ShipRuleMet {
				t.Fatalf("ShipRuleMet = true below the floor; want false — never a vacuous pass")
			}
			// Precision is NOT a basis below the floor: the record carries no precision
			// (nil), not a misleading zero-or-one.
			if rec.Precision != nil {
				t.Errorf("Precision = %v below the floor; want nil (not yet a basis)", *rec.Precision)
			}
			if rec.PrecisionOverFloor {
				t.Errorf("PrecisionOverFloor = true below the floor; want false")
			}
			if rec.HoldReason == "" {
				t.Errorf("HoldReason empty on an extend-window; want the floor named")
			}
		})
	}
}

// TestShipRuleHoldsAtAdvisoryOnAnyBreach — with the floor MET, any single
// ship-rule term failing HOLDS the ramp at advisory (§3.3), and the record names
// the failing term. Coverage short (M1), evals red (M3), nit over the guardrail,
// and precision below the threshold each independently hold — while every other
// term is perfect, isolating the one under test.
func TestShipRuleHoldsAtAdvisoryOnAnyBreach(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metrics harvest.Metrics
		m3Green bool
		nitP50  float64
	}{
		{
			// M1: one of ten proposals never reviewed → coverage 90% < 100%.
			name:    "coverage-short-M1",
			metrics: harvest.Metrics{Universe: 10, Covered: 9, BlockingDisposed: 6, BlockingAccepted: 6, Reviews: 9},
			m3Green: true,
			nitP50:  0,
		},
		{
			// M3: evals not green under protocol.
			name:    "evals-red-M3",
			metrics: fullyCovered(10, 6, 6, 10, 0),
			m3Green: false,
			nitP50:  0,
		},
		{
			// Nit guardrail: p50 4 > the ≤ 3 bound.
			name:    "nit-breach",
			metrics: fullyCovered(10, 6, 6, 10, 40),
			m3Green: true,
			nitP50:  4,
		},
		{
			// M2: precision 3/6 = 0.5 < the 0.70 threshold, over a MET floor.
			name:    "precision-below-threshold-M2",
			metrics: fullyCovered(10, 6, 3, 10, 0),
			m3Green: true,
			nitP50:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := checkpoint.Evaluate(checkpoint.Inputs{
				Reviews: 10,
				Metrics: tc.metrics,
				M3Green: tc.m3Green,
				NitP50:  tc.nitP50,
			})
			if rec.Outcome != checkpoint.OutcomeHoldAdvisory {
				t.Fatalf("outcome = %q, want hold-advisory (a breach holds the ramp)", rec.Outcome)
			}
			if rec.ShipRuleMet {
				t.Fatalf("ShipRuleMet = true on a breach; want false — the ramp holds at advisory")
			}
			if rec.HoldReason == "" {
				t.Errorf("HoldReason empty on a hold; want the failing term named (a flagged checkpoint is discussed)")
			}
		})
	}
}

// TestCadenceNotReachedHolds — a window short of the ten-review cadence holds
// (the checkpoint has not arrived), never a vacuous early pass — even with a
// perfect corpus.
func TestCadenceNotReachedHolds(t *testing.T) {
	t.Parallel()

	// A perfect corpus, but only 7 reviews < the window of 10.
	rec := checkpoint.Evaluate(checkpoint.Inputs{
		Reviews: 7,
		Metrics: fullyCovered(7, 6, 6, 7, 0),
		M3Green: true,
		NitP50:  0,
	})
	if rec.Outcome != checkpoint.OutcomeHoldAdvisory {
		t.Fatalf("outcome = %q, want hold-advisory (cadence not reached)", rec.Outcome)
	}
	if rec.ShipRuleMet {
		t.Fatalf("ShipRuleMet = true short of the cadence; want false")
	}
}

// TestEngagementSignalIsAdvisoryFlagNotAutoFail — the §3.3 rule: the
// disposition-engagement signal (dwell + accept-burst) surfaces at the checkpoint
// as an ADVISORY FLAG, NOT an auto-fail. A burst-flagged checkpoint with an
// otherwise-clearing corpus still lands ship-rule-met (it is discussed, not
// auto-failed); the flag rides into the EVT-5 record either way.
func TestEngagementSignalIsAdvisoryFlagNotAutoFail(t *testing.T) {
	t.Parallel()

	base := checkpoint.Inputs{
		Reviews: 10,
		Metrics: fullyCovered(10, 6, 6, 10, 0),
		M3Green: true,
		NitP50:  0,
	}

	// WITHOUT a burst flag: ship-rule-met, flag false in the record.
	clean := base
	clean.Engagement = checkpoint.Engagement{DwellP50: 120, BurstFlag: false}
	cleanRec := checkpoint.Evaluate(clean)
	if cleanRec.Outcome != checkpoint.OutcomeShipRuleMet {
		t.Fatalf("clean-engagement outcome = %q, want ship-rule-met", cleanRec.Outcome)
	}
	if cleanRec.BurstFlag {
		t.Errorf("BurstFlag = true with no burst; want false")
	}
	if cleanRec.DwellP50 != 120 {
		t.Errorf("DwellP50 = %v, want 120 (surfaced from the engagement input)", cleanRec.DwellP50)
	}

	// WITH a burst flag: the SAME clearing corpus still lands ship-rule-met — the
	// flag does NOT auto-fail the checkpoint; it is surfaced for discussion. The
	// operator's recorded promotion act attests engagement, not this flag (§3.3).
	flagged := base
	flagged.Engagement = checkpoint.Engagement{DwellP50: 3, BurstFlag: true}
	flaggedRec := checkpoint.Evaluate(flagged)
	if flaggedRec.Outcome != checkpoint.OutcomeShipRuleMet {
		t.Fatalf("burst-flagged outcome = %q, want ship-rule-met (a burst flag is advisory, not an auto-fail)", flaggedRec.Outcome)
	}
	if !flaggedRec.ShipRuleMet {
		t.Fatalf("ShipRuleMet = false under a burst flag; want true (the flag never gates)")
	}
	if !flaggedRec.BurstFlag {
		t.Errorf("BurstFlag = false; want true (surfaced into the EVT-5 record)")
	}
	if flaggedRec.DwellP50 != 3 {
		t.Errorf("DwellP50 = %v, want 3 (surfaced)", flaggedRec.DwellP50)
	}
}

// TestPolicyIsDataDrivenNotHardCoded — the thresholds are DATA (§3.2/§3.3
// starting values), so a repo Policy tunes the ship rule without a code change. A
// custom policy with a lower floor and threshold flips a below-default-floor
// window from extend to ship-rule-met.
func TestPolicyIsDataDrivenNotHardCoded(t *testing.T) {
	t.Parallel()

	// 3 operator-disposed blocking findings, all accepted — BELOW the default floor
	// of 5, so under the default policy this extends.
	m := fullyCovered(4, 3, 3, 4, 0)

	defaultRec := checkpoint.Evaluate(checkpoint.Inputs{Reviews: 10, Metrics: m, M3Green: true})
	if defaultRec.Outcome != checkpoint.OutcomeExtendWindow {
		t.Fatalf("default-policy outcome = %q, want extend-window (3 < default floor 5)", defaultRec.Outcome)
	}

	// A custom policy: a 4-review window, a floor of 3, a 0.70 bar — now the SAME
	// corpus clears, purely by the data change.
	custom := checkpoint.Policy{WindowSize: 4, DispositionFloor: 3, PrecisionThreshold: 0.70, NitP50Max: 3}
	if err := custom.Validate(); err != nil {
		t.Fatalf("custom policy invalid: %v", err)
	}
	customRec := checkpoint.Evaluate(checkpoint.Inputs{Reviews: 4, Metrics: m, M3Green: true, Policy: custom})
	if customRec.Outcome != checkpoint.OutcomeShipRuleMet {
		t.Fatalf("custom-policy outcome = %q, want ship-rule-met (floor lowered to 3 by data)", customRec.Outcome)
	}
	if !customRec.PrecisionOverFloor || customRec.Precision == nil || *customRec.Precision != 1.0 {
		t.Fatalf("custom-policy precision not present/1.0 over the lowered floor")
	}
}

// TestDefaultPolicyMatchesPreRegisteredStartingValues — the §3.2/§3.3 starting
// values are the ones the ramp ships with: a ten-review window, a ≥ 5 floor, a
// 0.70 precision bar, a ≤ 3 nit p50. Pins the data so a silent drift is caught.
func TestDefaultPolicyMatchesPreRegisteredStartingValues(t *testing.T) {
	t.Parallel()
	p := checkpoint.DefaultPolicy()
	if p.WindowSize != 10 {
		t.Errorf("WindowSize = %d, want 10 (the ten-review checkpoint)", p.WindowSize)
	}
	if p.DispositionFloor != 5 {
		t.Errorf("DispositionFloor = %d, want 5 (§3.2's ≥ 5)", p.DispositionFloor)
	}
	if p.PrecisionThreshold != 0.70 {
		t.Errorf("PrecisionThreshold = %v, want 0.70 (§3.2's ≥ 70%%)", p.PrecisionThreshold)
	}
	if p.NitP50Max != 3 {
		t.Errorf("NitP50Max = %v, want 3 (§3.3's ≤ 3 per review)", p.NitP50Max)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("DefaultPolicy invalid: %v", err)
	}
}

// TestPolicyValidateRejectsMalformedData — a config surface parses the policy
// closed and validates it; a malformed data file fails loudly rather than
// silently skewing the ship rule.
func TestPolicyValidateRejectsMalformedData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    checkpoint.Policy
	}{
		{"zero-window", checkpoint.Policy{WindowSize: 0, DispositionFloor: 5, PrecisionThreshold: 0.7, NitP50Max: 3}},
		{"zero-floor", checkpoint.Policy{WindowSize: 10, DispositionFloor: 0, PrecisionThreshold: 0.7, NitP50Max: 3}},
		{"precision-over-one", checkpoint.Policy{WindowSize: 10, DispositionFloor: 5, PrecisionThreshold: 1.5, NitP50Max: 3}},
		{"precision-negative", checkpoint.Policy{WindowSize: 10, DispositionFloor: 5, PrecisionThreshold: -0.1, NitP50Max: 3}},
		{"nit-negative", checkpoint.Policy{WindowSize: 10, DispositionFloor: 5, PrecisionThreshold: 0.7, NitP50Max: -1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.p.Validate(); err == nil {
				t.Fatalf("Validate(%+v) = nil; want an error", tc.p)
			}
		})
	}
}

// TestCheckpointReadsAGenuinelyHarvestedCorpus — the integration leg: the
// checkpoint reads a corpus produced by the REAL harvest.Harvest over a mixed
// proposal universe (a genuine end-to-end read, not a hand-built Metrics), and
// its ship-rule decision matches the harvested reality. This proves the
// checkpoint consumes the harvest faithfully (DOD-4: "from the harvested corpus
// alone").
func TestCheckpointReadsAGenuinelyHarvestedCorpus(t *testing.T) {
	t.Parallel()

	port := newFakePort()
	authors := newFakeAuthors()

	const operatorLogin = "mdarabi"

	// Ten proposals, each reviewed with one operator-accepted blocking finding
	// whose disposition artifact is authored by the operator (genuine, counted).
	universe := make([]harvest.Proposal, 0, 10)
	for i := 0; i < 10; i++ {
		ref := attributionRef(i)
		authors.byRef[ref] = operatorLogin
		r := reviewOneAcceptedBlocking(proposalRef(i), reviewID(i), ref)
		universe = append(universe, harvest.Proposal{Ref: proposalRef(i), Review: refPtr(port.put(t, r))})
	}

	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port,
		harvest.NewGitHubResolver(operatorLogin, authors),
	)
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}

	// Sanity: the harvest genuinely saw 10 reviews, 10 disposed-accepted blocking.
	if m.Universe != 10 || m.Covered != 10 || m.BlockingDisposed != 10 || m.BlockingAccepted != 10 {
		t.Fatalf("harvested corpus unexpected: %+v", m)
	}

	rec := checkpoint.Evaluate(checkpoint.Inputs{
		Reviews: m.Reviews,
		Metrics: m,
		M3Green: true,
		NitP50:  0,
	})
	if rec.Outcome != checkpoint.OutcomeShipRuleMet {
		t.Fatalf("outcome over a genuine 10/10-accepted corpus = %q, want ship-rule-met (reason: %q)", rec.Outcome, rec.HoldReason)
	}
	if rec.Coverage != 1.0 {
		t.Errorf("Coverage = %v over a fully-covered harvest; want 1.0", rec.Coverage)
	}
	if rec.Precision == nil || *rec.Precision != 1.0 {
		t.Fatalf("Precision not 1.0 over the harvested corpus")
	}
}

// --- hermetic fixtures for the integration leg ----------------------------

func proposalRef(i int) string    { return "github:owner/repo#" + strconv.Itoa(i) }
func reviewID(i int) string       { return "rv-" + strconv.Itoa(i) }
func attributionRef(i int) string { return "github:pr-review/" + strconv.Itoa(i) }

// reviewOneAcceptedBlocking builds a valid single-round review with one
// operator-accepted blocking finding whose attribution references an operator-
// authored artifact.
func reviewOneAcceptedBlocking(proposalRef, reviewID, attribution string) *report.Report {
	return &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   proposalRef,
		ReviewID:      reviewID,
		Band:          report.BandStandard,
		Verdict:       report.VerdictApproveWithAdvisories,
		Rounds: []report.Round{
			{
				ReviewedRevision: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				ContextID:        "ctx-" + reviewID,
				Provenance: report.Provenance{
					Model:          "claude-opus-4-8",
					PromptsVersion: "impl-review-prompts/v1",
					CalVersion:     "impl-review-calibration/v1",
				},
				Findings: []report.Finding{{
					ID:         "FND-1",
					Path:       "internal/x.go",
					Line:       10,
					Message:    "a blocking finding",
					RuleID:     "spec-fidelity/x",
					Lens:       report.LensSpecFidelity,
					Severity:   report.SeverityBlocking,
					CalRow:     "CAL-3",
					Acceptance: "A-1",
					Disposition: &report.Disposition{
						Actor:       report.ActorOperator,
						Verdict:     report.VerdictAccepted,
						Attribution: attribution,
					},
				}},
				FindingsCount: 1,
				Candidates:    1,
			},
		},
	}
}
