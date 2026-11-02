package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// --- the FAKE LensRunner (no network, no real subagents) ---
//
// recordingRunner captures EXACTLY what each lens received — the pack pointer,
// the instruction, and every finding argument it was ever handed — so the
// blindness test can assert (a) each lens got only its own instruction + the
// shared pack, and (b) no lens's findings were ever passed into another lens's
// Run (the signature has no such parameter, but the fake records the full call
// surface to prove it at runtime too). It returns seeded candidates per lens.

type lensCall struct {
	lens        string
	pack        []byte
	instruction string
}

type recordingRunner struct {
	mu sync.Mutex
	// candidates seeded per lens — what that lens "finds".
	candidates map[string][]report.Finding
	// err seeded per lens — a lens backend error.
	errs map[string]error
	// calls records every Run invocation in order.
	calls []lensCall
}

func newRecordingRunner() *recordingRunner {
	return &recordingRunner{
		candidates: map[string][]report.Finding{},
		errs:       map[string]error{},
	}
}

func (r *recordingRunner) Run(_ context.Context, lens string, pack []byte, instruction string) ([]report.Finding, error) {
	r.mu.Lock()
	r.calls = append(r.calls, lensCall{lens: lens, pack: pack, instruction: instruction})
	r.mu.Unlock()
	if err := r.errs[lens]; err != nil {
		return nil, err
	}
	return r.candidates[lens], nil
}

// validFinding is a well-formed candidate that survives the DefaultScreener and
// validates under report.Validate (a resolvable cal_row + matching severity).
func validFinding(id, lens, calRow, severity string) report.Finding {
	return report.Finding{
		ID:         id,
		Message:    "a claim: " + id,
		RuleID:     lens + "/rule-1",
		Lens:       lens,
		Severity:   severity,
		CalRow:     calRow,
		Acceptance: "AC-1",
	}
}

// instructionsForAll returns a distinct, non-empty instruction per lens — the
// T5-content stand-in, distinct so the blindness test can prove each lens got
// only ITS instruction.
func instructionsForAll() map[string]string {
	m := map[string]string{}
	for _, lens := range Lenses {
		m[lens] = "instruction for " + lens
	}
	return m
}

// stubCal is a CalLookup for validation: it maps the two cal rows the tests use
// to their severities. Real calibration is T5; the runtime tests only need a
// stub to validate the assembled report (report.Validate needs a CalLookup).
func stubCal() report.CalLookup {
	return report.CalLookupFunc(func(calRow string) (string, bool) {
		switch calRow {
		case "CAL-BLOCK":
			return report.SeverityBlocking, true
		case "CAL-ADV":
			return report.SeverityAdvisory, true
		default:
			return "", false
		}
	})
}

func baseRequest(runner LensRunner) ReviewRequest {
	return ReviewRequest{
		ReviewerRoot:     "reviewer-session-root",
		ImplementerRoot:  "implementer-session-root",
		Pack:             []byte("# context-pack/v1\nthe one shared pack body\n"),
		ReviewedRevision: "deadbeefcafe",
		Instructions:     instructionsForAll(),
		Provenance:       report.Provenance{Model: "test-model", PromptsVersion: "p1", CalVersion: "c1"},
		Runner:           runner,
	}
}

// --- (1) blindness: each lens gets only (pack, its instruction); no lens's
// findings are ever passed to another lens ---

func TestReviewLensBlindness(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	// Seed every lens with a distinct finding so, if any leaked, a later lens's
	// recorded call would carry it.
	for _, lens := range Lenses {
		runner.candidates[lens] = []report.Finding{validFinding("F-"+lens, lens, "CAL-ADV", report.SeverityAdvisory)}
	}
	req := baseRequest(runner)

	res, err := Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	_ = res

	// All four lenses ran, exactly once each, in the fixed order.
	if len(runner.calls) != len(Lenses) {
		t.Fatalf("want %d lens calls, got %d", len(Lenses), len(runner.calls))
	}
	for i, lens := range Lenses {
		call := runner.calls[i]
		if call.lens != lens {
			t.Errorf("call %d: want lens %q, got %q (order not fixed)", i, lens, call.lens)
		}
		// (a) Each lens got ONLY its own instruction.
		wantInstr := req.Instructions[lens]
		if call.instruction != wantInstr {
			t.Errorf("lens %q got instruction %q, want its own %q", lens, call.instruction, wantInstr)
		}
		// No OTHER lens's instruction leaked into this call.
		for _, other := range Lenses {
			if other == lens {
				continue
			}
			if strings.Contains(call.instruction, other) {
				t.Errorf("lens %q instruction leaked another lens %q: %q", lens, other, call.instruction)
			}
		}
		// (b) Each lens got the IDENTICAL shared pack (the cost move, NFR-1) —
		// same bytes as the request pack.
		if !bytesEqual(call.pack, req.Pack) {
			t.Errorf("lens %q did not receive the shared pack", lens)
		}
	}

	// (b) Structural blindness: the LensRunner signature is (lens, pack,
	// instruction) — there is NO finding parameter, so a lens PHYSICALLY cannot
	// receive another lens's findings. The recorded calls confirm the only
	// inputs were the pack + the instruction (asserted above); no findings
	// channel exists to leak.
}

// TestReviewSharedPackIdenticalAcrossLenses — the four lenses read the SAME pack
// bytes (one build, not four re-reads — NFR-1/DINV-5).
func TestReviewSharedPackIdenticalAcrossLenses(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	for _, lens := range Lenses {
		runner.candidates[lens] = nil
	}
	req := baseRequest(runner)
	if _, err := Review(context.Background(), req); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(runner.calls) != len(Lenses) {
		t.Fatalf("want %d calls, got %d", len(Lenses), len(runner.calls))
	}
	first := runner.calls[0].pack
	for _, call := range runner.calls[1:] {
		if !bytesEqual(call.pack, first) {
			t.Fatalf("lenses did not read the identical shared pack")
		}
	}
}

// --- (3) the screen kills a seeded plausible-but-wrong candidate before
// assembly ---

func TestReviewAdversarialScreenKillsCandidate(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()

	// The security lens emits TWO candidates: one real (survives) and one
	// plausible-but-wrong (must be killed before assembly). The other lenses
	// emit nothing.
	killMe := validFinding("F-KILL", report.LensSecurityAbuse, "CAL-BLOCK", report.SeverityBlocking)
	keepMe := validFinding("F-KEEP", report.LensSecurityAbuse, "CAL-BLOCK", report.SeverityBlocking)
	runner.candidates[report.LensSecurityAbuse] = []report.Finding{keepMe, killMe}

	req := baseRequest(runner)
	// Inject a Screener that kills exactly the seeded plausible-but-wrong
	// candidate (by id) — the adversarial screen's structure; T5 supplies the
	// real judgment content.
	req.Screener = ScreenerFunc(func(_ string, c report.Finding) bool {
		return c.ID != "F-KILL"
	})

	res, err := Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}

	// The killed candidate never reached the assembled round.
	for _, f := range res.Round.Findings {
		if f.ID == "F-KILL" {
			t.Fatalf("plausible-but-wrong candidate F-KILL survived into the round")
		}
	}
	// The survivor did.
	if !hasFinding(res.Round.Findings, "F-KEEP") {
		t.Fatalf("survivor F-KEEP missing from the round")
	}
	// The EVT counts reflect the screen: 2 candidates, 1 killed, 1 finding.
	if res.Round.Candidates != 2 {
		t.Errorf("candidates: want 2, got %d", res.Round.Candidates)
	}
	if res.Round.Killed != 1 {
		t.Errorf("killed: want 1, got %d", res.Round.Killed)
	}
	if res.Round.FindingsCount != 1 {
		t.Errorf("findings_count: want 1, got %d", res.Round.FindingsCount)
	}
	// The killed candidate is surfaced for the debugging surface.
	if len(res.Killed) != 1 || res.Killed[0].ID != "F-KILL" {
		t.Errorf("killed set: want [F-KILL], got %v", res.Killed)
	}
}

// TestDefaultScreenerKillsMalformedCandidate — the structural default screen
// (T4) kills a candidate missing the evidence a real finding must carry (no
// message; no cal_row), before it becomes an invalid round.
func TestDefaultScreenerKillsMalformedCandidate(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	noMessage := validFinding("F-NOMSG", report.LensSpecFidelity, "CAL-ADV", report.SeverityAdvisory)
	noMessage.Message = ""
	noCal := validFinding("F-NOCAL", report.LensSpecFidelity, "CAL-ADV", report.SeverityAdvisory)
	noCal.CalRow = ""
	good := validFinding("F-GOOD", report.LensSpecFidelity, "CAL-ADV", report.SeverityAdvisory)
	runner.candidates[report.LensSpecFidelity] = []report.Finding{noMessage, noCal, good}

	req := baseRequest(runner) // no Screener → DefaultScreener
	res, err := Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if res.Round.FindingsCount != 1 || !hasFinding(res.Round.Findings, "F-GOOD") {
		t.Fatalf("DefaultScreener should keep only F-GOOD, got %+v", res.Round.Findings)
	}
	if res.Round.Killed != 2 {
		t.Errorf("DefaultScreener should kill the two malformed candidates, killed=%d", res.Round.Killed)
	}
}

// --- (4) assembly: survivors become a VALID report.Report round ---

func TestReviewAssemblesValidReportRound(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	runner.candidates[report.LensSpecFidelity] = []report.Finding{
		validFinding("F-1", report.LensSpecFidelity, "CAL-ADV", report.SeverityAdvisory),
	}
	runner.candidates[report.LensSecurityAbuse] = []report.Finding{
		validFinding("F-2", report.LensSecurityAbuse, "CAL-BLOCK", report.SeverityBlocking),
	}
	req := baseRequest(runner)

	res, err := Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}

	// The round carries the reviewed revision, the derived context_id, and the
	// provenance (§3.1).
	if res.Round.ReviewedRevision != req.ReviewedRevision {
		t.Errorf("round reviewed_revision: want %q, got %q", req.ReviewedRevision, res.Round.ReviewedRevision)
	}
	wantCtx, _ := ContextID(req.ReviewerRoot)
	if res.Round.ContextID != wantCtx {
		t.Errorf("round context_id: want %q (reviewer-root-derived), got %q", wantCtx, res.Round.ContextID)
	}
	if res.Round.Provenance != req.Provenance {
		t.Errorf("round provenance mismatch: %+v", res.Round.Provenance)
	}

	// Wrap the round in a report envelope and validate it via T1's report.Validate
	// with the stub CalLookup — the survivors form a VALID report round.
	rep := &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   "pr-42",
		ReviewID:      "rev-1",
		Band:          report.BandStandard,
		Verdict:       report.VerdictRequestChanges, // a blocking finding is present
		Rounds:        []report.Round{res.Round},
	}
	if err := report.Validate(rep, stubCal()); err != nil {
		t.Fatalf("assembled round does not validate as a report: %v", err)
	}
	// Both survivors are present, each attributed to its own lens.
	if !hasFinding(res.Round.Findings, "F-1") || !hasFinding(res.Round.Findings, "F-2") {
		t.Fatalf("survivors missing from the round: %+v", res.Round.Findings)
	}
	for _, f := range res.Round.Findings {
		switch f.ID {
		case "F-1":
			if f.Lens != report.LensSpecFidelity {
				t.Errorf("F-1 lens: want %q, got %q", report.LensSpecFidelity, f.Lens)
			}
		case "F-2":
			if f.Lens != report.LensSecurityAbuse {
				t.Errorf("F-2 lens: want %q, got %q", report.LensSecurityAbuse, f.Lens)
			}
		}
	}
}

// TestReviewStampsLensOntoCandidate — the runtime attributes each candidate to
// the lens that produced it, overriding any lens the candidate claims (the Run
// contract owns the lens identity).
func TestReviewStampsLensOntoCandidate(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	// A candidate that lies about its lens — the runtime must correct it.
	liar := validFinding("F-LIAR", report.LensCorrectnessCritic, "CAL-ADV", report.SeverityAdvisory)
	liar.Lens = report.LensSpecFidelity // claims a different lens
	runner.candidates[report.LensCorrectnessCritic] = []report.Finding{liar}
	req := baseRequest(runner)
	res, err := Review(context.Background(), req)
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(res.Round.Findings) != 1 || res.Round.Findings[0].Lens != report.LensCorrectnessCritic {
		t.Fatalf("candidate lens not stamped to its producing lens: %+v", res.Round.Findings)
	}
}

// --- fail-closed paths ---

func TestReviewFailsClosedOnInheritedContext(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	req := baseRequest(runner)
	req.ImplementerRoot = req.ReviewerRoot // the reviewer is the implementer
	_, err := Review(context.Background(), req)
	if !errors.Is(err, ErrInheritedContext) {
		t.Fatalf("want ErrInheritedContext, got %v", err)
	}
	// The lenses NEVER ran — a non-independent reviewer produces no review.
	if len(runner.calls) != 0 {
		t.Errorf("lenses ran despite an inherited context (%d calls)", len(runner.calls))
	}
}

func TestReviewFailsClosedOnBadInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*ReviewRequest)
		wantErr error
	}{
		{"empty pack", func(r *ReviewRequest) { r.Pack = nil }, ErrEmptyPack},
		{"missing reviewed revision", func(r *ReviewRequest) { r.ReviewedRevision = "" }, nil},
		{"nil runner", func(r *ReviewRequest) { r.Runner = nil }, nil},
		{"missing a lens instruction", func(r *ReviewRequest) {
			delete(r.Instructions, report.LensSecurityAbuse)
		}, ErrNoInstruction},
		{"empty a lens instruction", func(r *ReviewRequest) {
			r.Instructions[report.LensSecurityAbuse] = ""
		}, ErrNoInstruction},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := newRecordingRunner()
			req := baseRequest(runner)
			// Fresh instructions map per case (the base shares one map).
			req.Instructions = instructionsForAll()
			tc.mutate(&req)
			_, err := Review(context.Background(), req)
			if err == nil {
				t.Fatalf("want an error, got nil")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestReviewPropagatesLensError(t *testing.T) {
	t.Parallel()
	runner := newRecordingRunner()
	sentinel := errors.New("lens backend timeout")
	runner.errs[report.LensDesignConformance] = sentinel
	req := baseRequest(runner)
	_, err := Review(context.Background(), req)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the lens backend error wrapped, got %v", err)
	}
}

func TestReviewContextCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// A runner that honors cancellation.
	runner := &cancelRunner{}
	req := baseRequest(runner)
	_, err := Review(ctx, req)
	if err == nil {
		t.Fatalf("want a cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// cancelRunner honors ctx cancellation — proves Review threads the context to
// the LensRunner (the real one makes a cancellable subagent call).
type cancelRunner struct{}

func (cancelRunner) Run(ctx context.Context, _ string, _ []byte, _ string) ([]report.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("lens cancelled: %w", err)
	}
	return nil, nil
}

// TestLensesAreTheFourEnumValues — the fixed lens order is exactly the four
// report.Lens enum values, no duplicate, no omission (T4 owns the structure).
func TestLensesAreTheFourEnumValues(t *testing.T) {
	t.Parallel()
	want := []string{
		report.LensSpecFidelity,
		report.LensDesignConformance,
		report.LensSecurityAbuse,
		report.LensCorrectnessCritic,
	}
	got := append([]string(nil), Lenses[:]...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Fatalf("Lenses must be exactly the four enum values: want %v, got %v", want, Lenses)
	}
	// No duplicate.
	seen := map[string]bool{}
	for _, l := range Lenses {
		if seen[l] {
			t.Fatalf("duplicate lens %q in Lenses", l)
		}
		seen[l] = true
	}
}

// --- small hermetic helpers ---

func hasFinding(fs []report.Finding, id string) bool {
	for _, f := range fs {
		if f.ID == id {
			return true
		}
	}
	return false
}

func bytesEqual(a, b []byte) bool {
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
