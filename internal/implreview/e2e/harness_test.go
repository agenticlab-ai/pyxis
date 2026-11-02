// Package e2e is the hermetic end-to-end integration test for the impl-review
// reviewer feature (docs/prds/impl-review/effective.md §4.4; docs/tech-design/
// impl-review-v1.md §3.4 the flow, §3.7 DINV-1..9, §3.11 testing strategy).
//
// The fifteen impl-review task packages are each unit-tested in isolation with
// their own fakes. This package proves they COMPOSE into a correct full review
// flow, and drives each of the PRD §4.4 scenarios (the common case, the risky
// diff, the gamed diff, the reviewer-is-down) end to end over the REAL packages,
// wiring deterministic fakes ONLY at the true-external seams:
//
//   - the LLM lens backend: a fake runtime.LensRunner seeded per (diff, lens)
//     (there is no real subagent spawn, no network) — §3.11 "the LLM lens
//     backend: true-external, injected/faked".
//   - the persistence code-host: a fake persist.Port storing the composed
//     artifact bytes in memory and recovering them through the SAME
//     persist.ComposeArtifact / persist.ParseArtifact the GitHub adapter uses,
//     so the payload round-trips byte-identically as one self-contained artifact
//     (FR-6, DINV-1/2/9) without touching a code host.
//   - the harvest enumerator + author-lookup: fake harvest.Enumerator and
//     harvest.AuthorResolver over the full proposal universe (§3.11 "the harvest
//     over a mixed proposal set").
//
// Everything ELSE is the real package: charter.Validate, band.Select,
// pack.Build, runtime.Review with the floor.Screener composed in, verdict.Compute
// / SetEnvelope / SignalFor, protocol dispositions + scope-lock, checkpoint's
// advisory gate, lifecycle's resume + in-flight predicate, and the real
// calibration table + lens instructions read off disk. The test is hermetic: no
// network, no real LLM, no clock or randomness without injection.
//
// This file holds the shared harness the four scenario files drive; each scenario
// lives in its own file (common_case_test.go, risky_diff_test.go,
// gamed_diff_test.go, reviewer_down_test.go).
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview/band"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/floor"
	"github.com/agenticlab-ai/pyxis/internal/implreview/lens"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// -----------------------------------------------------------------------------
// The true-external seam #1: the LLM lens backend.
//
// fakeLensRunner is a deterministic stand-in for the real sibling-subagent lens
// runner (runtime.LensRunner). It reads the proposal diff out of the pack the
// runtime built (the pack IS the review context — §3.4) and returns the seeded
// findings a CORRECT reviewer would produce for that diff, filtered to the lens
// being run (a real lens is finding-blind to the others — DINV-5/FR-3, and the
// runner honors that by lens-filtering, exactly as the eval harness's fake does).
//
// It can be told to ERROR (past the bounded retry, the reviewer-is-down
// scenario) and it never obeys a directive embedded in the diff (the gamed-diff
// scenario surfaces the directive as a finding CANDIDATE, never a command — the
// verdict is invariant to it).
// -----------------------------------------------------------------------------

// seededFinding is one finding the fake runner emits when a diff matches — the
// finding a correct reviewer would produce. The runner stamps nothing extra; the
// runtime stamps the lens.
type seededFinding struct {
	id       string
	ruleID   string
	lens     string
	severity string
	calRow   string
	message  string
	path     string
	line     int
}

func (sf seededFinding) toFinding() report.Finding {
	return report.Finding{
		ID:         sf.id,
		Path:       sf.path,
		Line:       sf.line,
		Message:    sf.message,
		RuleID:     sf.ruleID,
		Lens:       sf.lens,
		Severity:   sf.severity,
		CalRow:     sf.calRow,
		Acceptance: "seeded e2e finding",
	}
}

// fakeLensRunner maps a diff (read out of the pack) to the findings a correct
// reviewer produces for it. Keyed by a diff marker the pack carries verbatim (the
// pack embeds the proposal diff bytes; the fake looks for a marker substring),
// so the runner is driven by the SAME pack the runtime built — not by any side
// channel.
type fakeLensRunner struct {
	mu sync.Mutex

	// byMarker maps a diff-marker substring to the findings emitted when the pack
	// contains that marker. The marker makes the mapping robust to the pack's
	// manifest framing (the diff bytes appear verbatim inside the pack).
	byMarker map[string][]seededFinding

	// err, when non-nil, makes every Run return it — the lens-backend failure the
	// reviewer-is-down scenario injects. The runtime surfaces it (bounded retry is
	// the caller's/lifecycle's job); a review over an erroring runner never
	// silently passes.
	err error

	// calls records each Run for assertions (lens + whether the pack carried the
	// deep-band mandatory context).
	calls []lensCall
}

type lensCall struct {
	lens         string
	packHasAbuse bool
}

func newFakeLensRunner() *fakeLensRunner {
	return &fakeLensRunner{byMarker: map[string][]seededFinding{}}
}

// seed registers the findings a correct reviewer produces for a diff carrying the
// given marker. The marker is any stable substring of the diff (the fixtures'
// diffs carry unique file paths / messages that serve as markers).
func (r *fakeLensRunner) seed(marker string, findings ...seededFinding) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byMarker[marker] = findings
}

// failWith makes every subsequent Run return err — the reviewer-is-down injection.
func (r *fakeLensRunner) failWith(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// Run implements runtime.LensRunner. It reads the shared pack, finds the seeded
// findings for whatever marker the pack's diff carries, and returns ONLY the
// findings whose lens matches this Run's lens (finding-blindness — DINV-5/FR-3).
func (r *fakeLensRunner) Run(_ context.Context, lensID string, packBytes []byte, _ string) ([]report.Finding, error) {
	r.mu.Lock()
	// Record the call and whether the pack carried the deep-band mandatory context
	// (band.EnrichInputs injects the served ABUSE rows + incident registry into
	// the pack — the deep-band audit, FR-10).
	r.calls = append(r.calls, lensCall{lens: lensID, packHasAbuse: containsBytes(packBytes, deepAbuseMarker)})
	err := r.err
	// Snapshot the marker→findings map under the lock, then release it before
	// building the result (Run holds no lock while it allocates).
	var matched []seededFinding
	for marker, fs := range r.byMarker {
		if containsBytes(packBytes, marker) {
			matched = fs
			break
		}
	}
	r.mu.Unlock()

	if err != nil {
		return nil, err
	}

	var out []report.Finding
	for _, sf := range matched {
		if sf.lens != lensID {
			continue // finding-blind: only this lens's findings on this lens's Run.
		}
		out = append(out, sf.toFinding())
	}
	return out, nil
}

// deepAbuseMarker is a stable substring of the served ABUSE rows band.EnrichInputs
// injects into the deep-band pack — the fake runner asserts its presence to prove
// the deep-band mandatory context reached the lenses (FR-10).
const deepAbuseMarker = "ABUSE-5"

// containsBytes reports whether haystack contains the needle substring — a tiny
// helper so the fake runner reads the pack the runtime built without importing a
// heavier matcher.
func containsBytes(haystack []byte, needle string) bool {
	if needle == "" {
		return false
	}
	nb := []byte(needle)
	for i := 0; i+len(nb) <= len(haystack); i++ {
		if string(haystack[i:i+len(nb)]) == needle {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// The true-external seam #2: the persistence code-host.
//
// memPort is an in-memory persist.Port. It stores the COMPOSED artifact bytes
// (persist.ComposeArtifact — the exact serialization the GitHub adapter writes:
// the rendered human face plus the fenced machine block) and recovers the
// byte-identical payload through persist.ParseArtifact. This exercises FR-6 and
// DINV-1/2/9 (one self-contained artifact, byte-identical round-trip, render
// fidelity) at the real serialization boundary, while the code host itself stays
// a fake — no network, no `gh`.
//
// A given proposal writes exactly ONE artifact (DINV-1/9): a second Persist for a
// review re-writes the same slot's body (append-round semantics live in the
// payload, not in a second artifact). Persist keys the stored body by
// (ProposalRef, ReviewID) so re-persisting a review with an appended round
// overwrites its one artifact rather than creating a second.
// -----------------------------------------------------------------------------

type memPort struct {
	mu sync.Mutex

	// bodies maps a locator to its stored artifact body. One locator per
	// (proposal, review) — the one-self-contained-artifact rule (DINV-1/9).
	bodies map[string]string
	// keyToLocator maps a "proposal\x00review" key to the locator holding that
	// review's one artifact, so a re-persist of an appended-round review overwrites
	// the single artifact.
	keyToLocator map[string]string
	next         int

	// writeFailsLeft, when > 0, makes the next N Persist calls fail (the
	// write-failure injection for UB-5; the harness does not depend on it, but it
	// is available for a write-failure scenario).
	writeFailsLeft int
}

func newMemPort() *memPort {
	return &memPort{bodies: map[string]string{}, keyToLocator: map[string]string{}}
}

// Persist composes the review into one artifact and stores it, returning the ref.
// It overwrites the review's one existing artifact (keyed by proposal+review) so
// an appended-round re-persist stays one artifact (DINV-1/9).
func (p *memPort) Persist(_ context.Context, r *report.Report) (persist.Ref, error) {
	body, err := persist.ComposeArtifact(r)
	if err != nil {
		return persist.Ref{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeFailsLeft > 0 {
		p.writeFailsLeft--
		return persist.Ref{}, errWriteFailed
	}
	key := r.ProposalRef + "\x00" + r.ReviewID
	loc, ok := p.keyToLocator[key]
	if !ok {
		p.next++
		loc = locatorFor(p.next)
		p.keyToLocator[key] = loc
	}
	p.bodies[loc] = body
	return persist.Ref{Medium: persist.MediumGitHub, Locator: loc}, nil
}

// Read recovers the byte-identical payload from the stored artifact through the
// real parser (persist.ParseArtifact — which also enforces render-fidelity,
// DINV-2).
func (p *memPort) Read(_ context.Context, ref persist.Ref) (*report.Report, error) {
	p.mu.Lock()
	body, ok := p.bodies[ref.Locator]
	p.mu.Unlock()
	if !ok {
		return nil, errNoArtifact
	}
	return persist.ParseArtifact(body)
}

// artifactCount reports how many distinct review artifacts are stored — the
// one-artifact-per-proposal assertion (DINV-1/9).
func (p *memPort) artifactCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.bodies)
}

// rawBody returns the stored artifact body for a ref — for asserting the machine
// block + rendered face are one artifact.
func (p *memPort) rawBody(ref persist.Ref) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, ok := p.bodies[ref.Locator]
	return b, ok
}

// -----------------------------------------------------------------------------
// The true-external seam #3: the harvest edges (enumerator + author-lookup).
// -----------------------------------------------------------------------------

// fakeAuthors is a deterministic harvest.AuthorResolver: it maps a disposition
// artifact's attribution reference to the medium login that authored it. The
// harvest resolves an operator disposition's attribution against the operator
// identity through this; a forged disposition (author != operator) is excluded
// (FR-16/DINV-4, ABUSE-5). No network.
type fakeAuthors struct {
	byRef map[string]string // attribution ref -> author login
}

func newFakeAuthors() *fakeAuthors { return &fakeAuthors{byRef: map[string]string{}} }

func (a *fakeAuthors) set(ref, login string) { a.byRef[ref] = login }

// AuthorOf implements harvest.AuthorResolver.
func (a *fakeAuthors) AuthorOf(_ context.Context, attribution string) (string, bool, error) {
	login, ok := a.byRef[attribution]
	return login, ok, nil
}

// -----------------------------------------------------------------------------
// Shared real-data loaders — the calibration table and lens instructions read
// off disk, so the pipeline validates findings against the REAL calibration
// (DINV-3) and runs with the REAL four lens instructions (FR-3), not synthetic
// stand-ins.
// -----------------------------------------------------------------------------

// repoRoot walks up from the test's cwd to the module root (the dir holding
// go.mod). The fixtures + skill data live under the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod above %s", dir)
		}
		dir = parent
	}
}

// skillDir is the impl-review skill's data dir (the calibration table + lens
// instruction files live here).
func skillDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), ".pyxis", "skills", "impl-review")
}

// loadCalibration loads the REAL impl-review calibration table — a
// report.CalLookup the pipeline validates every finding's severity against
// (DINV-3). Loaded once per test.
func loadCalibration(t *testing.T) *calibration.Table {
	t.Helper()
	tbl, err := calibration.LoadFile(filepath.Join(skillDir(t), "calibration.md"))
	if err != nil {
		t.Fatalf("load real calibration table: %v", err)
	}
	return tbl
}

// loadLensInstructions loads the REAL four lens instructions — the per-lens
// instruction map runtime.Review requires (every lens must have a non-empty one).
func loadLensInstructions(t *testing.T) map[string]string {
	t.Helper()
	inst, err := lens.LoadInstructions(filepath.Join(skillDir(t), "lenses"))
	if err != nil {
		t.Fatalf("load real lens instructions: %v", err)
	}
	return inst
}

// -----------------------------------------------------------------------------
// The pipeline harness — the shared composition every scenario drives.
//
// runReviewRound wires the real packages into ONE review round for a proposal
// revision, exactly in the design's order (§3.4): build the shared context pack
// (DINV-5: no authoring trail), then runtime.Review with the floor.Screener
// composed onto the runtime's default screen (FR-4) and a fresh-instance
// provenance (DINV-5 sibling/fresh, FR-2). It returns the assembled round; the
// caller assembles it into a report.Report envelope and computes the verdict.
// -----------------------------------------------------------------------------

// reviewInputs is the per-round input surface the harness maps onto the pack + the
// runtime — the proposal at its reviewed revision, its charter, the codegate
// verdict, the governing design bindings, the reviewed revision sha, the band,
// and (deep band) the mandatory context.
type reviewInputs struct {
	subject          string
	proposalDiff     []byte
	codegateVerdict  []byte
	charter          []byte
	designBindings   []byte
	reviewedRevision string
	band             string
	deep             band.DeepContext // zero for non-deep bands
}

// harness holds the shared seams + real data a scenario drives the pipeline with.
type harness struct {
	runner *fakeLensRunner
	port   *memPort
	cal    *calibration.Table
	instr  map[string]string

	// reviewerRoot / implementerRoot are the sibling-session roots the runtime's
	// provenance check reads (A-6, DINV-5): the reviewer is a fresh top-level
	// SIBLING of the implementing worker, never a child. They are DISTINCT so the
	// review is independent; an inherited reviewer context fails the check.
	reviewerRoot    string
	implementerRoot string
}

func newHarness(t *testing.T) *harness {
	return &harness{
		runner:          newFakeLensRunner(),
		port:            newMemPort(),
		cal:             loadCalibration(t),
		instr:           loadLensInstructions(t),
		reviewerRoot:    "session:impl-review-reviewer-root",
		implementerRoot: "session:implementing-worker-root",
	}
}

// buildPack assembles the one shared review context pack for a round (pack.Build),
// enriching it with the deep-band mandatory context when the band is deep
// (band.EnrichInputs — FR-10). It asserts DINV-5 structurally: pack.ReviewInputs
// has no authoring-trail field, so no prior finding/disposition can enter (the
// exclusion is by construction; there is nothing to filter).
func (h *harness) buildPack(t *testing.T, in reviewInputs) []byte {
	t.Helper()
	base := pack.ReviewInputs{
		Subject:         in.subject,
		Proposal:        in.proposalDiff,
		CodegateVerdict: in.codegateVerdict,
		Charter:         in.charter,
		DesignBindings:  in.designBindings,
		Calibration:     []byte(renderCalibration(t, h)),
	}
	if in.band == report.BandDeep {
		enriched, err := band.EnrichInputs(base, report.BandDeep, in.deep)
		if err != nil {
			t.Fatalf("band.EnrichInputs (deep mandatory context, FR-10): %v", err)
		}
		base = enriched
		if !band.HasDeepContext(base) {
			t.Fatal("deep-band inputs must carry the mandatory context after EnrichInputs (FR-10)")
		}
	}
	packBytes, err := pack.Build(base)
	if err != nil {
		t.Fatalf("pack.Build: %v", err)
	}
	if len(packBytes) == 0 {
		t.Fatal("pack.Build returned an empty pack")
	}
	return packBytes
}

// runRound runs runtime.Review for one round with the floor.Screener composed in
// and a fresh-instance provenance, and returns the assembled round. The screener
// is floor.Compose(DefaultScreener, floor.NewScreenerForVerdict(flagged)) — the
// runtime's structural screen AND the mechanical-floor exclusion, exactly as
// production composes them (FR-3 + FR-4). flaggedClasses are the classes codegate
// flagged on the reviewed diff (from floor.FlaggedClasses over the codegate
// verdict the pack carries).
func (h *harness) runRound(t *testing.T, packBytes []byte, reviewedRevision string, flaggedClasses []string) runtime.RoundResult {
	t.Helper()
	screener := floor.Compose(runtime.DefaultScreener{}, floor.NewScreenerForVerdict(flaggedClasses))
	req := runtime.ReviewRequest{
		ReviewerRoot:     h.reviewerRoot,
		ImplementerRoot:  h.implementerRoot,
		Pack:             packBytes,
		ReviewedRevision: reviewedRevision,
		Instructions:     h.instr,
		Provenance: report.Provenance{
			Model:          "fake-hermetic-lens-model",
			PromptsVersion: "impl-review-lenses/v1",
			CalVersion:     calibration.Version,
		},
		Runner:   h.runner,
		Screener: screener,
	}
	res, err := runtime.Review(context.Background(), req)
	if err != nil {
		t.Fatalf("runtime.Review: %v", err)
	}
	if res == nil {
		t.Fatal("runtime.Review returned a nil result")
	}
	return *res
}

// renderCalibration renders the real calibration table's rows into the bytes the
// pack carries as its calibration kind. The pack just needs the table's bytes;
// the harness re-renders the loaded rows so the pack's calibration matches the
// table the pipeline validates against.
func renderCalibration(t *testing.T, h *harness) string {
	t.Helper()
	// The pack carries the table verbatim; the loaded table's row set is what
	// matters. Read the raw file so the pack byte-mirrors the on-disk table.
	data, err := os.ReadFile(filepath.Join(skillDir(t), "calibration.md"))
	if err != nil {
		t.Fatalf("read calibration.md for the pack: %v", err)
	}
	return string(data)
}

// assembleReport builds a report.Report envelope over the given rounds, computes
// the live verdict from them + the escalate signal (verdict.Compute), stamps it
// onto the envelope (verdict.SetEnvelope), and validates the whole payload against
// the REAL calibration table (report.Validate — DINV-3) plus the uniqueness and
// render-fidelity rules (DINV-1/2). It returns the validated report and the
// computed outcome. This is the "assemble + verdict + validate" spine every
// scenario shares (§3.4 steps 2 + the T7 verdict layer).
func (h *harness) assembleReport(t *testing.T, proposalRef, reviewID, bnd string, sig verdict.Signals, rounds ...report.Round) (*report.Report, verdict.Outcome) {
	t.Helper()
	outcome, err := verdict.Compute(rounds, sig)
	if err != nil {
		t.Fatalf("verdict.Compute: %v", err)
	}
	r := &report.Report{
		SchemaVersion: report.SchemaVersion,
		ProposalRef:   proposalRef,
		ReviewID:      reviewID,
		Band:          bnd,
		Rounds:        rounds,
	}
	if _, err := verdict.SetEnvelope(r, outcome); err != nil {
		t.Fatalf("verdict.SetEnvelope: %v", err)
	}
	// DINV-3 (+ decline-reason, brief-iff-escalate, at-least-one-round): the whole
	// payload validates against the REAL calibration table.
	if err := report.Validate(r, h.cal); err != nil {
		t.Fatalf("report.Validate against the real calibration table (DINV-3): %v", err)
	}
	// DINV-1 (cross-report uniqueness) and DINV-2 (render fidelity) on the payload.
	if err := report.ValidateUnique([]*report.Report{r}); err != nil {
		t.Fatalf("report.ValidateUnique (DINV-1): %v", err)
	}
	if err := report.ValidateRenderFidelity(r, report.Render(r)); err != nil {
		t.Fatalf("report.ValidateRenderFidelity (DINV-2): %v", err)
	}
	return r, outcome
}

// persistAndReadBack persists the report as ONE self-contained artifact through
// the fake code-host port and reads it back, asserting the recovered payload is
// byte-identical to what was persisted (FR-6, DINV-1/2/9). It returns the ref.
func (h *harness) persistAndReadBack(t *testing.T, r *report.Report) persist.Ref {
	t.Helper()
	before := h.port.artifactCount()
	ref, err := h.port.Persist(context.Background(), r)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if got := h.port.artifactCount(); got != before+1 {
		t.Fatalf("persist wrote %d new artifacts, want exactly 1 (DINV-1/9)", got-before)
	}
	got, err := h.port.Read(context.Background(), ref)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	// Byte-identity: the recovered payload re-marshals to the exact bytes the
	// persisted one did (FR-6, DINV-1/2).
	wantBytes, err := r.Marshal()
	if err != nil {
		t.Fatalf("marshal persisted: %v", err)
	}
	gotBytes, err := got.Marshal()
	if err != nil {
		t.Fatalf("marshal read-back: %v", err)
	}
	if string(wantBytes) != string(gotBytes) {
		t.Fatalf("round-trip is not byte-identical (FR-6/DINV-1/2):\n--- persisted ---\n%s\n--- read back ---\n%s", wantBytes, gotBytes)
	}
	// The stored body is one self-contained artifact: the rendered human face plus
	// exactly one fenced machine block (DINV-9). ParseArtifact already enforced
	// render-fidelity on read; assert the fence is present in the raw body too.
	body, ok := h.port.rawBody(ref)
	if !ok {
		t.Fatal("no raw body for the persisted ref")
	}
	if !containsBytes([]byte(body), "```"+persist.FenceTag) {
		t.Fatal("the persisted artifact carries no impl-review machine block (FR-6)")
	}
	return ref
}
