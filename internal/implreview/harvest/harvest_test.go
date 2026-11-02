package harvest_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/harvest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// fakePort is a hermetic persist.Port: it stores each persisted review as its
// COMPOSED artifact body (persist.ComposeArtifact — the identical serialization
// the real GitHub adapter writes) keyed by comment id, and reads it back through
// persist.ParseArtifact (the machine block authoritative — the same parser the
// real adapter uses). So the harvest reads REAL payload bytes off the medium, not
// a struct handed straight back — the "reconstruct from payloads alone" contract
// (DINV-8). No network, no `gh`.
type fakePort struct {
	mu       sync.Mutex
	bodies   map[string]string // comment id → artifact body
	next     int
	readErrs map[string]error // comment id → injected read error (a corrupt artifact)
}

func newFakePort() *fakePort {
	return &fakePort{bodies: map[string]string{}, readErrs: map[string]error{}}
}

// put stores a review payload as its composed artifact and returns the ref the
// enumerator will hand the harvest.
func (p *fakePort) put(t *testing.T, r *report.Report) persist.Ref {
	t.Helper()
	body, err := persist.ComposeArtifact(r)
	if err != nil {
		t.Fatalf("compose artifact: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	id := fmt.Sprintf("c%d", p.next)
	p.bodies[id] = body
	return persist.Ref{Medium: persist.MediumGitHub, Locator: id}
}

// corrupt stores a body that ParseArtifact will reject (no machine block),
// modeling an unreadable review artifact.
func (p *fakePort) corrupt(t *testing.T) persist.Ref {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	id := fmt.Sprintf("c%d", p.next)
	p.bodies[id] = "a human note with no machine block at all\n"
	return persist.Ref{Medium: persist.MediumGitHub, Locator: id}
}

func (p *fakePort) Persist(_ context.Context, _ *report.Report) (persist.Ref, error) {
	return persist.Ref{}, errors.New("fakePort.Persist not used by the harvest")
}

func (p *fakePort) Read(_ context.Context, ref persist.Ref) (*report.Report, error) {
	p.mu.Lock()
	body, ok := p.bodies[ref.Locator]
	rerr := p.readErrs[ref.Locator]
	p.mu.Unlock()
	if rerr != nil {
		return nil, rerr
	}
	if !ok {
		return nil, fmt.Errorf("fakePort: no artifact %s", ref.Locator)
	}
	return persist.ParseArtifact(body)
}

// fakeAuthors is a hermetic AuthorResolver: an in-memory map from an attribution
// reference to the login of the artifact's medium author. This is the anti-forgery
// oracle — a real GitHub API would report the PR review/comment's `user`; the fake
// reports what it was told, so a test can plant a FORGED disposition (attribution
// whose real author is NOT the operator). No network.
type fakeAuthors struct {
	byRef   map[string]string // attribution ref → author login
	failRef map[string]error  // attribution ref → injected transport error
}

func newFakeAuthors() *fakeAuthors {
	return &fakeAuthors{byRef: map[string]string{}, failRef: map[string]error{}}
}

func (a *fakeAuthors) AuthorOf(_ context.Context, attribution string) (string, bool, error) {
	if err := a.failRef[attribution]; err != nil {
		return "", false, err
	}
	login, ok := a.byRef[attribution]
	return login, ok, nil
}

const operatorLogin = "mdarabi"

// --- fixtures -------------------------------------------------------------

// reviewWith builds a valid single-round review payload for a proposal, with the
// given findings. The envelope band/verdict are standard/approve-with-advisories
// unless a finding forces otherwise — the harvest reads only the fields it needs.
func reviewWith(proposalRef, reviewID string, findings ...report.Finding) *report.Report {
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
				Findings:      findings,
				FindingsCount: len(findings),
				Candidates:    len(findings),
				Killed:        0,
			},
		},
	}
}

// blockingFinding builds a blocking finding carrying the given operator
// disposition (verdict + attribution). id is its FND anchor.
func blockingFinding(id, verdict, attribution string) report.Finding {
	return report.Finding{
		ID:         id,
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
			Verdict:     verdict,
			Attribution: attribution,
		},
	}
}

// advisoryFinding builds an advisory finding carrying the given operator
// disposition — used for the nit-rate (rejected advisory) leg.
func advisoryFinding(id, verdict, attribution string) report.Finding {
	return report.Finding{
		ID:         id,
		Path:       "internal/y.go",
		Line:       20,
		Message:    "an advisory finding",
		RuleID:     "correctness-critic/y",
		Lens:       report.LensCorrectnessCritic,
		Severity:   report.SeverityAdvisory,
		CalRow:     "CAL-7",
		Acceptance: "A-2",
		Disposition: &report.Disposition{
			Actor:       report.ActorOperator,
			Verdict:     verdict,
			Attribution: attribution,
		},
	}
}

// --- tests ----------------------------------------------------------------

// TestForgedOperatorDispositionExcludedFromPrecision — the ABUSE-5 / A-5 leg on
// the GitHub path: a disposition claiming actor=operator whose disposition-artifact
// author is NOT the operator (a worker forged it) is EXCLUDED from M2. A genuine
// operator-accepted blocking finding (author IS the operator) is counted. The
// forged one moves the ExcludedDispositions counter, never the precision numerator
// or denominator.
func TestForgedOperatorDispositionExcludedFromPrecision(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()

	// A genuine operator disposition: its artifact's author IS the operator.
	authors.byRef["github:pr-review/1001"] = operatorLogin
	// A FORGED operator disposition: the referenced artifact's author is a worker
	// bot, not the operator — the worker planted "operator accepted" (ABUSE-5).
	authors.byRef["github:pr-review/forged"] = "agenticlab-factory-bot"

	genuine := reviewWith("github:owner/repo#1", "rv-1",
		blockingFinding("FND-1", report.VerdictAccepted, "github:pr-review/1001"))
	forged := reviewWith("github:owner/repo#2", "rv-2",
		blockingFinding("FND-1", report.VerdictAccepted, "github:pr-review/forged"))

	universe := []harvest.Proposal{
		{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, genuine))},
		{Ref: "github:owner/repo#2", Review: refPtr(port.put(t, forged))},
	}

	resolver := harvest.NewGitHubResolver(operatorLogin, authors)
	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, resolver)
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}

	// Only the GENUINE operator-accepted blocking finding is counted: 1 disposed, 1
	// accepted. The forged one is excluded — not in either M2 term.
	if m.BlockingDisposed != 1 {
		t.Fatalf("BlockingDisposed=%d, want 1 (the forged disposition must be excluded)", m.BlockingDisposed)
	}
	if m.BlockingAccepted != 1 {
		t.Fatalf("BlockingAccepted=%d, want 1", m.BlockingAccepted)
	}
	if m.ExcludedDispositions != 1 {
		t.Fatalf("ExcludedDispositions=%d, want 1 (the forged disposition)", m.ExcludedDispositions)
	}
	// Precision over a floor of 1 is 1/1; the forged one never inflated it.
	got, ok := m.Precision(1)
	if !ok || got != 1.0 {
		t.Fatalf("Precision(1)=(%v,%v), want (1,true)", got, ok)
	}
}

// TestOperatorDispositionDerivesFromMediumArtifactNeverPayloadField — an operator
// disposition's validity is bound to the disposition-ARTIFACT'S OWN author (looked
// up through the code host), never to a payload-writable field. Two payloads carry
// byte-identical dispositions (same actor/verdict/attribution string); flipping ONLY
// what the code host reports the artifact's author to be flips whether the
// disposition counts. So the payload string is not the source of authority — the
// medium artifact's author is (FR-16/DINV-4).
func TestOperatorDispositionDerivesFromMediumArtifactNeverPayloadField(t *testing.T) {
	t.Parallel()

	const attribution = "github:pr-review/2002"

	build := func(authorReportedByHost string) harvest.Metrics {
		port := newFakePort()
		authors := newFakeAuthors()
		authors.byRef[attribution] = authorReportedByHost // the code host's answer

		// The PAYLOAD is identical in both runs — same attribution string, same
		// verdict. Only the HOST'S reported author differs.
		r := reviewWith("github:owner/repo#7", "rv-7",
			blockingFinding("FND-1", report.VerdictAccepted, attribution))
		universe := []harvest.Proposal{{Ref: "github:owner/repo#7", Review: refPtr(port.put(t, r))}}

		m, err := harvest.Harvest(context.Background(),
			harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
			port, harvest.NewGitHubResolver(operatorLogin, authors))
		if err != nil {
			t.Fatalf("harvest: %v", err)
		}
		return m
	}

	// Host reports the operator as the artifact's author → the disposition counts.
	whenOperator := build(operatorLogin)
	if whenOperator.BlockingDisposed != 1 || whenOperator.BlockingAccepted != 1 {
		t.Fatalf("operator-authored artifact: disposed=%d accepted=%d, want 1/1",
			whenOperator.BlockingDisposed, whenOperator.BlockingAccepted)
	}
	if whenOperator.ExcludedDispositions != 0 {
		t.Fatalf("operator-authored artifact excluded a disposition: %d", whenOperator.ExcludedDispositions)
	}

	// Host reports someone ELSE as the artifact's author (the identical payload
	// string notwithstanding) → the disposition is excluded. The payload field did
	// not decide it; the artifact's author did.
	whenImposter := build("someone-else")
	if whenImposter.BlockingDisposed != 0 || whenImposter.BlockingAccepted != 0 {
		t.Fatalf("imposter-authored artifact was counted: disposed=%d accepted=%d, want 0/0",
			whenImposter.BlockingDisposed, whenImposter.BlockingAccepted)
	}
	if whenImposter.ExcludedDispositions != 1 {
		t.Fatalf("imposter-authored artifact was not excluded: %d", whenImposter.ExcludedDispositions)
	}
}

// TestHarvestReconstructsCorpusFromPayloadsAlone — over a MIXED proposal set the
// harvest reconstructs coverage, precision-over-the-floor, the nit rate, and the
// miss corpus reading PAYLOAD FIELDS ONLY (DINV-8). The set mixes: covered reviews
// with attributable operator dispositions (accepted blocking, rejected blocking,
// rejected advisory), a covered review with no operator disposition, a
// never-reviewed proposal, and an absence-marked proposal.
func TestHarvestReconstructsCorpusFromPayloadsAlone(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()
	authors.byRef["op/a"] = operatorLogin
	authors.byRef["op/b"] = operatorLogin
	authors.byRef["op/c"] = operatorLogin
	authors.byRef["op/d"] = operatorLogin

	// Review 1: an operator-ACCEPTED blocking finding (true positive) — feeds M2 as
	// 1 disposed / 1 accepted.
	r1 := reviewWith("github:owner/repo#1", "rv-1",
		blockingFinding("FND-1", report.VerdictAccepted, "op/a"))
	// Review 2: an operator-REJECTED blocking finding (a nit that was blocking) —
	// feeds M2 as 1 disposed / 0 accepted, and the nit numerator as 1.
	r2 := reviewWith("github:owner/repo#2", "rv-2",
		blockingFinding("FND-1", report.VerdictRejected, "op/b"))
	// Review 3: an operator-REJECTED advisory finding — a pure nit (advisory, so NOT
	// a precision event) — feeds the nit numerator as 1 only.
	r3 := reviewWith("github:owner/repo#3", "rv-3",
		advisoryFinding("FND-1", report.VerdictRejected, "op/c"))
	// Review 4: an operator-ACCEPTED advisory finding — accepted but advisory, so it
	// is NOT a blocking precision event and NOT a nit. It is still a covered review.
	r4 := reviewWith("github:owner/repo#4", "rv-4",
		advisoryFinding("FND-1", report.VerdictAccepted, "op/d"))
	// Review 5: covered, but with NO operator disposition (only a worker one) — a
	// covered review that contributes nothing to precision or the nit rate.
	worker := report.Finding{
		ID: "FND-1", Path: "z.go", Line: 1, Message: "m", RuleID: "spec-fidelity/z",
		Lens: report.LensSpecFidelity, Severity: report.SeverityBlocking, CalRow: "CAL-3", Acceptance: "A-1",
		Disposition: &report.Disposition{Actor: report.ActorWorker, Verdict: report.VerdictApplied},
	}
	r5 := reviewWith("github:owner/repo#5", "rv-5", worker)

	universe := []harvest.Proposal{
		{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, r1))},
		{Ref: "github:owner/repo#2", Review: refPtr(port.put(t, r2))},
		{Ref: "github:owner/repo#3", Review: refPtr(port.put(t, r3))},
		{Ref: "github:owner/repo#4", Review: refPtr(port.put(t, r4))},
		{Ref: "github:owner/repo#5", Review: refPtr(port.put(t, r5))},
		// A never-reviewed proposal (no review artifact) — uncovered, a miss.
		{Ref: "github:owner/repo#6", Review: nil},
		// An absence-marked proposal — a recorded absence, NOT a review — uncovered.
		{Ref: "github:owner/repo#7", Review: nil, Absent: true, AbsentReason: "lens backend timeout"},
	}

	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, harvest.NewGitHubResolver(operatorLogin, authors))
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}

	// Universe = 7; covered = the 5 reviewed proposals (r1..r5); the never-reviewed
	// and the absence-marked ones are NOT covered.
	if m.Universe != 7 {
		t.Fatalf("Universe=%d, want 7", m.Universe)
	}
	if m.Covered != 5 {
		t.Fatalf("Covered=%d, want 5 (the two un-reviewed proposals are uncovered)", m.Covered)
	}
	if got := m.Coverage(); got != 5.0/7.0 {
		t.Fatalf("Coverage()=%v, want 5/7", got)
	}

	// Precision denominator = the BLOCKING operator-disposed findings = r1 (accepted)
	// + r2 (rejected blocking) = 2; numerator = the accepted blocking = 1 (r1). The
	// accepted ADVISORY (r4) is not a blocking precision event.
	if m.BlockingDisposed != 2 {
		t.Fatalf("BlockingDisposed=%d, want 2 (r1 accepted + r2 rejected blocking)", m.BlockingDisposed)
	}
	if m.BlockingAccepted != 1 {
		t.Fatalf("BlockingAccepted=%d, want 1 (r1)", m.BlockingAccepted)
	}
	if got, ok := m.Precision(2); !ok || got != 0.5 {
		t.Fatalf("Precision(2)=(%v,%v), want (0.5,true)", got, ok)
	}

	// Nit numerator = operator-REJECTED findings = r2 (rejected blocking) + r3
	// (rejected advisory) = 2; reviews = 5 (r1..r5). Nit rate = 2/5.
	if m.OperatorRejected != 2 {
		t.Fatalf("OperatorRejected=%d, want 2 (r2 + r3)", m.OperatorRejected)
	}
	if m.Reviews != 5 {
		t.Fatalf("Reviews=%d, want 5", m.Reviews)
	}
	if got := m.NitRate(); got != 2.0/5.0 {
		t.Fatalf("NitRate()=%v, want 2/5", got)
	}

	// No disposition was excluded — every operator disposition here is attributable.
	if m.ExcludedDispositions != 0 {
		t.Fatalf("ExcludedDispositions=%d, want 0", m.ExcludedDispositions)
	}

	// The miss corpus carries exactly the two un-reviewed proposals, in enumeration
	// order, each with its kind and cause.
	if len(m.Misses) != 2 {
		t.Fatalf("miss corpus has %d records, want 2:\n%+v", len(m.Misses), m.Misses)
	}
	if m.Misses[0] != (harvest.MissRecord{Proposal: "github:owner/repo#6", Kind: harvest.MissNeverReviewed}) {
		t.Fatalf("miss[0]=%+v, want a never-reviewed record for #6", m.Misses[0])
	}
	if m.Misses[1] != (harvest.MissRecord{Proposal: "github:owner/repo#7", Kind: harvest.MissAbsent, Cause: "lens backend timeout"}) {
		t.Fatalf("miss[1]=%+v, want a review-absent record for #7 with its cause", m.Misses[1])
	}
}

// TestNeverReviewedProposalCountsAgainstCoverage — the A-4 never-invoked leg: a
// proposal with no review artifact at all is ENUMERATED (the harvest sees it —
// only the harvest does) and counts AGAINST coverage. Coverage over a universe of
// {one reviewed, one never-reviewed} is 1/2, not 1/1.
func TestNeverReviewedProposalCountsAgainstCoverage(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()

	reviewed := reviewWith("github:owner/repo#1", "rv-1") // a covered review, no findings
	universe := []harvest.Proposal{
		{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, reviewed))},
		{Ref: "github:owner/repo#2", Review: nil}, // never reviewed
	}

	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, harvest.NewGitHubResolver(operatorLogin, authors))
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}

	if m.Universe != 2 {
		t.Fatalf("Universe=%d, want 2 (the never-reviewed proposal is enumerated)", m.Universe)
	}
	if m.Covered != 1 {
		t.Fatalf("Covered=%d, want 1", m.Covered)
	}
	if got := m.Coverage(); got != 0.5 {
		t.Fatalf("Coverage()=%v, want 0.5 (a never-reviewed proposal counts against it)", got)
	}
	if len(m.Misses) != 1 || m.Misses[0].Kind != harvest.MissNeverReviewed || m.Misses[0].Proposal != "github:owner/repo#2" {
		t.Fatalf("miss corpus=%+v, want one never-reviewed record for #2", m.Misses)
	}
}

// TestAbsenceMarkerIsNotAReview — a proposal carrying only a recorded
// review-absence (UB-5) is NOT covered: an absence marker is not a review (§3.3).
// It is a review-absent miss, distinct from a never-reviewed one, carrying the
// recorded cause.
func TestAbsenceMarkerIsNotAReview(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()

	universe := []harvest.Proposal{
		{Ref: "github:owner/repo#9", Review: nil, Absent: true, AbsentReason: "persist write failed past cap"},
	}
	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, harvest.NewGitHubResolver(operatorLogin, authors))
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	if m.Universe != 1 || m.Covered != 0 {
		t.Fatalf("Universe=%d Covered=%d, want 1/0 (an absence is not a review)", m.Universe, m.Covered)
	}
	if got := m.Coverage(); got != 0 {
		t.Fatalf("Coverage()=%v, want 0", got)
	}
	if len(m.Misses) != 1 || m.Misses[0].Kind != harvest.MissAbsent || m.Misses[0].Cause != "persist write failed past cap" {
		t.Fatalf("miss corpus=%+v, want one review-absent record with its cause", m.Misses)
	}
}

// TestUnreadableArtifactIsAMissNotSilentlyCovered — a proposal whose review
// artifact cannot be parsed back to a payload (a corrupt/tampered artifact) is NOT
// counted as covered; it is a MissUnreadable record carrying the parse error, and
// the rest of the harvest still completes (one corrupt artifact never aborts the
// whole harvest).
func TestUnreadableArtifactIsAMissNotSilentlyCovered(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()

	good := reviewWith("github:owner/repo#1", "rv-1")
	universe := []harvest.Proposal{
		{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, good))},
		{Ref: "github:owner/repo#2", Review: refPtr(port.corrupt(t))},
	}

	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, harvest.NewGitHubResolver(operatorLogin, authors))
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	if m.Universe != 2 {
		t.Fatalf("Universe=%d, want 2", m.Universe)
	}
	if m.Covered != 1 {
		t.Fatalf("Covered=%d, want 1 (the corrupt artifact is not covered)", m.Covered)
	}
	if len(m.Misses) != 1 || m.Misses[0].Kind != harvest.MissUnreadable || m.Misses[0].Proposal != "github:owner/repo#2" {
		t.Fatalf("miss corpus=%+v, want one unreadable record for #2", m.Misses)
	}
	if m.Misses[0].Cause == "" {
		t.Fatal("unreadable miss must carry the parse-error cause, never silent")
	}
}

// TestEnumerationErrorAborts — an enumeration failure aborts the harvest rather
// than returning a partial (silently-inflated-coverage) universe.
func TestEnumerationErrorAborts(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	boom := errors.New("code host unreachable")
	_, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return nil, boom }),
		port, harvest.NewGitHubResolver(operatorLogin, newFakeAuthors()))
	if !errors.Is(err, boom) {
		t.Fatalf("harvest error=%v, want it to wrap the enumeration failure", err)
	}
}

// TestLatestDispositionPerFindingGoverns — a finding re-disposed across rounds
// (an earlier round rejects it, a later round accepts it after a fix) counts ONCE,
// by its LATEST disposition — the harvest dedupes by finding id (§3.3: the payload
// governs; a later round supersedes an earlier one).
func TestLatestDispositionPerFindingGoverns(t *testing.T) {
	t.Parallel()
	port := newFakePort()
	authors := newFakeAuthors()
	authors.byRef["op/x"] = operatorLogin

	r := &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "github:owner/repo#1",
		ReviewID:      "rv-1",
		Band:          report.BandStandard,
		Verdict:       report.VerdictApproveWithAdvisories,
		Rounds: []report.Round{
			{
				ReviewedRevision: "1111111111111111111111111111111111111111",
				ContextID:        "ctx-1",
				Provenance:       report.Provenance{Model: "m", PromptsVersion: "p", CalVersion: "c"},
				Findings:         []report.Finding{blockingFinding("FND-1", report.VerdictRejected, "op/x")},
				FindingsCount:    1,
			},
			{
				ReviewedRevision: "2222222222222222222222222222222222222222",
				ContextID:        "ctx-2",
				Provenance:       report.Provenance{Model: "m", PromptsVersion: "p", CalVersion: "c"},
				Findings:         []report.Finding{blockingFinding("FND-1", report.VerdictAccepted, "op/x")},
				FindingsCount:    1,
			},
		},
	}
	universe := []harvest.Proposal{{Ref: "github:owner/repo#1", Review: refPtr(port.put(t, r))}}

	m, err := harvest.Harvest(context.Background(),
		harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return universe, nil }),
		port, harvest.NewGitHubResolver(operatorLogin, authors))
	if err != nil {
		t.Fatalf("harvest: %v", err)
	}
	// FND-1 counts ONCE, by its latest (round-2 accepted) disposition: 1 disposed, 1
	// accepted, 0 rejected — the round-1 rejection did not also count.
	if m.BlockingDisposed != 1 || m.BlockingAccepted != 1 {
		t.Fatalf("disposed=%d accepted=%d, want 1/1 (latest disposition governs)", m.BlockingDisposed, m.BlockingAccepted)
	}
	if m.OperatorRejected != 0 {
		t.Fatalf("OperatorRejected=%d, want 0 (the superseded round-1 rejection must not count)", m.OperatorRejected)
	}
}

// TestPrecisionFloorGate — Precision reports ok==false until the disposition floor
// is met, so the caller (the T13 checkpoint) never trusts a below-floor ratio.
func TestPrecisionFloorGate(t *testing.T) {
	t.Parallel()
	m := harvest.Metrics{BlockingDisposed: 3, BlockingAccepted: 3}
	if _, ok := m.Precision(5); ok {
		t.Fatal("Precision must report ok==false below the disposition floor")
	}
	if v, ok := m.Precision(3); !ok || v != 1.0 {
		t.Fatalf("Precision(3)=(%v,%v) at the floor, want (1,true)", v, ok)
	}
	// A zero-denominator never divides by zero and is never a basis.
	if _, ok := (harvest.Metrics{}).Precision(0); ok {
		t.Fatal("Precision over zero disposed findings must report ok==false")
	}
}

// TestNilSeamsFailClosed — a nil enumerator/port/resolver is a programming error
// the harvest refuses, never a silent empty corpus.
func TestNilSeamsFailClosed(t *testing.T) {
	t.Parallel()
	enum := harvest.EnumeratorFunc(func(context.Context) ([]harvest.Proposal, error) { return nil, nil })
	if _, err := harvest.Harvest(context.Background(), nil, newFakePort(), harvest.NewGitHubResolver("x", newFakeAuthors())); err == nil {
		t.Fatal("nil Enumerator must be refused")
	}
	if _, err := harvest.Harvest(context.Background(), enum, nil, harvest.NewGitHubResolver("x", newFakeAuthors())); err == nil {
		t.Fatal("nil persist.Port must be refused")
	}
	if _, err := harvest.Harvest(context.Background(), enum, newFakePort(), nil); err == nil {
		t.Fatal("nil OperatorResolver must be refused")
	}
}

// refPtr returns a pointer to a persist.Ref — the enumerator carries an optional
// review ref (nil = never reviewed).
func refPtr(r persist.Ref) *persist.Ref { return &r }
