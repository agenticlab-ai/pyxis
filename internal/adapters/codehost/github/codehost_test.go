package ghcodehost

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/budget"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"os"
	"path/filepath"
)

// fakeClient is a hermetic stand-in for the GitHub surface: it returns recorded
// responses and records inputs, so the adapter's idempotent-open flow,
// nothing-to-propose detection, and check aggregation are tested with no
// network and no gh.
type fakeClient struct {
	// findOpenPRByHead
	existingPR int
	existingOK bool
	findErr    error
	findHead   string         // recorded head (the last one asked about)
	findHeads  []string       // every head asked about, in order
	perHead    map[string]int // when set, answers per head instead of the single-valued pair

	// compareAhead
	ahead       int
	compareErr  error
	compareBase string
	compareHead string

	// createPR
	createNumber int
	createErr    error
	created      *createArg // nil until createPR is called — proves no duplicate

	// pullHeadSHA
	headSHA      string
	headErr      error
	pulledNumber int
	pullState    string
	pullMerged   bool
	pullHeadRef  string

	// checkRuns
	runs       []checkRun
	runsErr    error
	runsSHA    string
	checkCalls int

	// createComment
	commentID     string
	commentErr    error
	commentNumber int
	commentBody   string
	commentCalls  int

	// pullBody / setPullBody
	bodies     map[int]string
	bodyErr    error
	setBodyErr error
	bodyWrites int
}

type createArg struct{ head, base, title, body string }

func (f *fakeClient) findOpenPRByHead(_ context.Context, head string) (int, bool, error) {
	f.findHead = head
	f.findHeads = append(f.findHeads, head)
	if f.perHead != nil {
		number, found := f.perHead[head]
		return number, found, f.findErr
	}
	return f.existingPR, f.existingOK, f.findErr
}

func (f *fakeClient) compareAhead(_ context.Context, base, head string) (int, error) {
	f.compareBase, f.compareHead = base, head
	return f.ahead, f.compareErr
}

func (f *fakeClient) createPR(_ context.Context, head, base, title, body string) (int, error) {
	f.created = &createArg{head, base, title, body}
	return f.createNumber, f.createErr
}

func (f *fakeClient) pullHeadSHA(_ context.Context, number int) (string, error) {
	f.pulledNumber = number
	return f.headSHA, f.headErr
}

func (f *fakeClient) pullSnapshot(_ context.Context, number int) (pullSnapshot, error) {
	f.pulledNumber = number
	if f.headErr != nil {
		return pullSnapshot{}, f.headErr
	}
	state := f.pullState
	if state == "" {
		state = "open"
	}
	return pullSnapshot{State: state, Merged: f.pullMerged, HeadSHA: f.headSHA, HeadRef: f.pullHeadRef}, nil
}

func (f *fakeClient) checkRuns(_ context.Context, sha string) ([]checkRun, error) {
	f.checkCalls++
	f.runsSHA = sha
	return f.runs, f.runsErr
}

func (f *fakeClient) createComment(_ context.Context, number int, body string) (string, error) {
	f.commentCalls++
	f.commentNumber, f.commentBody = number, body
	return f.commentID, f.commentErr
}

func (f *fakeClient) pullBody(_ context.Context, number int) (string, error) {
	return f.bodies[number], f.bodyErr
}

func (f *fakeClient) setPullBody(_ context.Context, number int, body string) error {
	if f.setBodyErr != nil {
		return f.setBodyErr
	}
	if f.bodies == nil {
		f.bodies = map[int]string{}
	}
	f.bodies[number] = body
	f.bodyWrites++
	return nil
}

// testDescription is a complete description, as the Worker hands it over.
var testDescription = prdescription.Description{
	LinesOfCode: "3 (+3/-0)", Complexity: "low", BlastRadius: "low", Decision: "two-way", ReviewNotes: "None.",
	What: "- Change the thing.", Why: "It was needed.", How: "- Directly.", Testing: "- [x] go test: passed.",
}

// fakeWorkdir is a minimal port.Workdir for the head-branch fallback test.
type fakeWorkdir struct{ branch string }

func (w fakeWorkdir) Dir() string                              { return "/tmp/work" }
func (w fakeWorkdir) Branch() string                           { return w.branch }
func (w fakeWorkdir) Checkpoint(context.Context) (bool, error) { return true, nil }
func (w fakeWorkdir) Push(context.Context) (bool, error)       { return true, nil }
func (w fakeWorkdir) Cleanup(context.Context) error            { return nil }

// The proposal body carries trustworthy, tracker-native provenance and never a
// bare "#N" for a non-GitHub item (which GitHub would mis-link to its own issue).
func TestProposalBody_ProvenanceIsTrustworthy(t *testing.T) {
	// GitHub-tracked: a same-repo issue ref that links correctly.
	gh := proposalBody(port.Task{ID: "T5", Number: 42, SourceRef: "#42"}, "owner/repo", testDescription)
	if !strings.Contains(gh, "- Implements task T5 (#42), opened by the Pyxis executor, which never merges.") {
		t.Errorf("github body missing the linked issue ref:\n%s", gh)
	}
	if !strings.Contains(gh, "\n- Closes #42\n") {
		t.Errorf("github body missing the native closing directive:\n%s", gh)
	}

	// Carina-tracked: the trustworthy medium-native ref, and NO bare "#2" that
	// GitHub would mis-link to its own issue #2.
	ca := proposalBody(port.Task{ID: "T1", Number: 2, SourceRef: "Carina work-item 2"}, "owner/repo", testDescription)
	if !strings.Contains(ca, "- Implements task T1 (Carina work-item 2), opened by the Pyxis executor, which never merges.") {
		t.Errorf("carina body missing the trustworthy ref:\n%s", ca)
	}
	if strings.Contains(ca, "#2") {
		t.Errorf("carina body contains a bare #2 that GitHub would mis-link:\n%s", ca)
	}
	if strings.Contains(ca, "Closes ") {
		t.Errorf("carina body contains a GitHub closing directive:\n%s", ca)
	}

	// A tracker that set no ref falls back to a non-linking "work-item N".
	fb := proposalBody(port.Task{ID: "T9", Number: 7}, "owner/repo", testDescription)
	if strings.Contains(fb, "#7") {
		t.Errorf("fallback body must not emit a bare #7:\n%s", fb)
	}
	if !strings.Contains(fb, "work-item 7") {
		t.Errorf("fallback body should name the work item:\n%s", fb)
	}
}

func TestSameRepoGitHubIssueRef_IsClosedAndUnambiguous(t *testing.T) {
	for _, tc := range []struct {
		ref  string
		want bool
	}{
		{ref: "#1", want: true},
		{ref: "#42", want: true},
		{ref: "#0", want: false},
		{ref: "#042", want: false},
		{ref: " #42", want: false},
		{ref: "owner/repo#42", want: false},
		{ref: "Carina work-item 42", want: false},
		{ref: "42", want: false},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			if got := sameRepoGitHubIssueRef(tc.ref); got != tc.want {
				t.Fatalf("sameRepoGitHubIssueRef(%q) = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}
}

// A compare that 404s because the head branch was never published on the remote
// reads as nothing-to-propose (hollow), not a hard error (#397, defense in depth
// — the Worker publishes before OpenProposal in the Model-A flow, so a raw
// compare-404 should never surface as an operational failure).
func TestOpenProposal_UnpublishedHeadIsHollow(t *testing.T) {
	fc := &fakeClient{compareErr: errors.New("gh: Not Found (HTTP 404)")}
	h := newHost(fc, "main")
	task := port.Task{ID: "T5", HeadBranch: "feat/never-pushed"}

	_, err := h.OpenProposal(context.Background(), task, nil, testDescription)
	if !errors.Is(err, port.ErrNoProposal) {
		t.Fatalf("err = %v, want port.ErrNoProposal for an unpublished head", err)
	}
	if fc.created != nil {
		t.Error("createPR was called despite nothing to propose")
	}
}

// A non-404 compare error is still a hard failure (fail-closed): a transient API
// outage must not be silently swallowed as hollow.
func TestOpenProposal_NonNotFoundCompareErrorIsNotSwallowed(t *testing.T) {
	fc := &fakeClient{compareErr: errors.New("gh: HTTP 503 service unavailable")}
	h := newHost(fc, "main")
	task := port.Task{ID: "T5", HeadBranch: "feat/x"}

	_, err := h.OpenProposal(context.Background(), task, nil, testDescription)
	if err == nil || errors.Is(err, port.ErrNoProposal) {
		t.Fatalf("err = %v, want a hard error (not hollow) for a non-404 compare failure", err)
	}
}

// A-of-T5(2): OpenProposal opens a proposal from the exact head branch. No
// existing proposal, commits ahead of mainline ⇒ a fresh draft is created from
// task.HeadBranch verbatim, into the configured base.
func TestOpenProposal_OpensFromExactHead(t *testing.T) {
	fc := &fakeClient{ahead: 3, createNumber: 77}
	h := newHost(fc, "main")
	task := port.Task{ID: "T5", Number: 42, SourceRef: "#42", Title: "gh adapters", HeadBranch: "feat/implement-now-T5"}

	got, err := h.OpenProposal(context.Background(), task, nil, testDescription)
	if err != nil {
		t.Fatalf("OpenProposal: %v", err)
	}
	if got != (port.Proposal{Number: 77, Branch: "feat/implement-now-T5"}) {
		t.Errorf("Proposal = %+v, want {77, feat/implement-now-T5}", got)
	}
	if fc.created == nil {
		t.Fatal("createPR was not called")
	}
	if fc.created.head != "feat/implement-now-T5" {
		t.Errorf("createPR head = %q, want the exact task.HeadBranch feat/implement-now-T5", fc.created.head)
	}
	if fc.created.base != "main" {
		t.Errorf("createPR base = %q, want main", fc.created.base)
	}
	if fc.created.title != "chore: "+task.Title || !strings.Contains(fc.created.body, "T5 (#42)") {
		t.Errorf("proposal title/body lost the task identity: %q / %q", fc.created.title, fc.created.body)
	}
	// The nothing-to-propose compare ran against the configured mainline.
	if fc.compareBase != "main" || fc.compareHead != "feat/implement-now-T5" {
		t.Errorf("compare was %s...%s, want main...feat/implement-now-T5", fc.compareBase, fc.compareHead)
	}
}

// TestNewHostBaseResolution proves the base resolution honors config: an empty
// base resolves DefaultBaseBranch, which sources the config schema's single-source
// default (byte-identical to the old "main" literal, DINV-1), while a configured
// non-default base is used verbatim as BOTH the nothing-to-propose compare base
// and the created proposal's base (config-driven-portability lift, FR-2). A base
// still pinned to a baked "main" would open the proposal against the wrong branch.
func TestNewHostBaseResolution(t *testing.T) {
	if config.Defaults().Branch != "main" {
		t.Fatalf("config default branch is no longer byte-identical to the old literal (%q)", config.Defaults().Branch)
	}
	if DefaultBaseBranch != config.Defaults().Branch {
		t.Fatalf("DefaultBaseBranch %q drifted from the config default %q", DefaultBaseBranch, config.Defaults().Branch)
	}
	cases := []struct {
		name, ctorBase, wantBase string
	}{
		{"empty base resolves the config default", "", config.Defaults().Branch},
		{"configured non-default base is honored", "trunk", "trunk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{ahead: 1, createNumber: 9}
			h := newHost(fc, tc.ctorBase)
			task := port.Task{ID: "T5", HeadBranch: "feat/x"}
			if _, err := h.OpenProposal(context.Background(), task, nil, testDescription); err != nil {
				t.Fatalf("OpenProposal: %v", err)
			}
			if fc.compareBase != tc.wantBase {
				t.Errorf("compare base = %q, want %q", fc.compareBase, tc.wantBase)
			}
			if fc.created == nil || fc.created.base != tc.wantBase {
				t.Errorf("created proposal base = %+v, want base %q", fc.created, tc.wantBase)
			}
		})
	}
}

// Idempotency (prd:implement-now/FR-14, AP-22): a re-invocation over a head that
// already has an open proposal returns it — never a duplicate. createPR must
// not be called.
func TestOpenProposal_IdempotentReturnsExistingNoDuplicate(t *testing.T) {
	fc := &fakeClient{existingPR: 55, existingOK: true}
	h := newHost(fc, "main")
	task := port.Task{ID: "T5", HeadBranch: "feat/x"}

	got, err := h.OpenProposal(context.Background(), task, nil, testDescription)
	if err != nil {
		t.Fatalf("OpenProposal: %v", err)
	}
	if got != (port.Proposal{Number: 55, Branch: "feat/x"}) {
		t.Errorf("Proposal = %+v, want the existing {55, feat/x}", got)
	}
	if fc.created != nil {
		t.Error("createPR was called — a duplicate proposal was opened for an already-proposed head")
	}
}

// A clean run that changed nothing (0 commits ahead of mainline) is hollow, not
// a proposal (DINV-3): OpenProposal returns port.ErrNoProposal and opens nothing.
func TestOpenProposal_NothingToProposeIsErrNoProposal(t *testing.T) {
	fc := &fakeClient{ahead: 0}
	h := newHost(fc, "main")

	_, err := h.OpenProposal(context.Background(), port.Task{HeadBranch: "feat/empty"}, nil, testDescription)
	if !errors.Is(err, port.ErrNoProposal) {
		t.Fatalf("OpenProposal error = %v, want port.ErrNoProposal", err)
	}
	if fc.created != nil {
		t.Error("createPR was called for a head with nothing to propose")
	}
}

// The head branch and the workdir's branch denote the same isolated head; an
// empty task.HeadBranch falls back to the workdir's branch.
func TestOpenProposal_FallsBackToWorkdirBranch(t *testing.T) {
	fc := &fakeClient{ahead: 1, createNumber: 9}
	h := newHost(fc, "main")

	_, err := h.OpenProposal(context.Background(), port.Task{ID: "T5"}, fakeWorkdir{branch: "feat/from-work"}, testDescription)
	if err != nil {
		t.Fatalf("OpenProposal: %v", err)
	}
	if fc.created == nil || fc.created.head != "feat/from-work" {
		t.Errorf("createPR head = %v, want the workdir branch feat/from-work", fc.created)
	}
}

// No head branch anywhere is a wiring error, fail-closed (never a silent open).
func TestOpenProposal_NoHeadBranchErrors(t *testing.T) {
	fc := &fakeClient{}
	_, err := newHost(fc, "main").OpenProposal(context.Background(), port.Task{ID: "T5"}, nil, testDescription)
	if err == nil {
		t.Fatal("OpenProposal with no head branch should error")
	}
	if errors.Is(err, port.ErrNoProposal) {
		t.Error("a missing head branch is a wiring error, not the hollow ErrNoProposal")
	}
	if fc.created != nil {
		t.Error("createPR was called despite no head branch")
	}
}

func TestOpenProposal_PropagatesErrors(t *testing.T) {
	base := port.Task{HeadBranch: "feat/x"}
	sentinel := errors.New("gh down")
	cases := []struct {
		name string
		fc   *fakeClient
	}{
		{"lookup", &fakeClient{findErr: sentinel}},
		{"compare", &fakeClient{compareErr: sentinel}},
		{"create", &fakeClient{ahead: 2, createErr: sentinel}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newHost(tc.fc, "main").OpenProposal(context.Background(), base, nil, testDescription)
			if !errors.Is(err, sentinel) {
				t.Fatalf("error = %v, want wrapped sentinel", err)
			}
		})
	}
}

// Checks reads the proposal's live head SHA and then that commit's check runs
// through the Checks API — the fresh path, threading the SHA through.
func TestChecks_ThreadsFreshHeadSHA(t *testing.T) {
	fc := &fakeClient{headSHA: "deadbeefcafe", runs: []checkRun{{Status: "completed", Conclusion: "success"}}}
	h := newHost(fc, "main")

	st, err := h.Checks(context.Background(), port.Proposal{Number: 9, Branch: "feat/x"})
	if err != nil {
		t.Fatalf("Checks: %v", err)
	}
	if st != port.CheckGreen {
		t.Errorf("state = %v, want green", st)
	}
	if fc.pulledNumber != 9 {
		t.Errorf("head SHA read for #%d, want #9", fc.pulledNumber)
	}
	if fc.runsSHA != "deadbeefcafe" {
		t.Errorf("check runs read for %q, want the fresh head SHA deadbeefcafe", fc.runsSHA)
	}
}

// A-of-T5(3): the stale-read fixture. Checks holds no cache — a second call
// reflects check state that changed between calls, re-reading each time (AP-15).
func TestChecks_FreshReadNeverCached(t *testing.T) {
	fc := &fakeClient{headSHA: "sha1", runs: []checkRun{{Status: "in_progress"}}}
	h := newHost(fc, "main")

	if st, _ := h.Checks(context.Background(), port.Proposal{Number: 9}); st != port.CheckPending {
		t.Fatalf("first Checks = %v, want pending", st)
	}
	// CI settles green between calls. A cached adapter would still report pending.
	fc.runs = []checkRun{{Status: "completed", Conclusion: "success"}}
	if st, _ := h.Checks(context.Background(), port.Proposal{Number: 9}); st != port.CheckGreen {
		t.Fatalf("second Checks = %v, want fresh green (stale read would keep pending)", st)
	}
	if fc.checkCalls != 2 {
		t.Errorf("checkRuns called %d times, want 2 (one fresh read per Checks)", fc.checkCalls)
	}
}

func TestInspectProposal_JoinsLifecycleHeadAndChecks(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		merged  bool
		runs    []checkRun
		landing port.Landing
		checks  port.CheckState
	}{
		{"open red", "open", false, []checkRun{{Status: "completed", Conclusion: "failure"}}, port.LandingOpen, port.CheckRed},
		{"merged green", "closed", true, []checkRun{{Status: "completed", Conclusion: "success"}}, port.LandingLanded, port.CheckGreen},
		{"closed unmerged", "closed", false, []checkRun{{Status: "completed", Conclusion: "success"}}, port.LandingClosedUnlanded, port.CheckGreen},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc := &fakeClient{headSHA: "abc123", pullHeadRef: "feat/x", pullState: tc.state, pullMerged: tc.merged, runs: tc.runs}
			landing, head, checks, err := newHost(fc, "main").InspectProposal(context.Background(), port.Proposal{Number: 9, Branch: "feat/x"})
			if err != nil {
				t.Fatalf("InspectProposal: %v", err)
			}
			if landing != tc.landing || head != "abc123" || checks != tc.checks {
				t.Fatalf("observation=(%s,%q,%s), want (%s,%q,%s)", landing, head, checks, tc.landing, "abc123", tc.checks)
			}
			if fc.runsSHA != "abc123" {
				t.Fatalf("check runs read for %q, want observed head abc123", fc.runsSHA)
			}
		})
	}
}

func TestInspectProposal_RejectsForeignBranch(t *testing.T) {
	fc := &fakeClient{headSHA: "abc123", pullHeadRef: "feat/foreign", pullState: "open"}
	landing, _, checks, err := newHost(fc, "main").InspectProposal(context.Background(), port.Proposal{Number: 9, Branch: "feat/ours"})
	if err == nil || !strings.Contains(err.Error(), "head branch") {
		t.Fatalf("InspectProposal error = %v, want branch mismatch", err)
	}
	if landing != port.LandingUnknown || checks != port.CheckPending || fc.checkCalls != 0 {
		t.Fatalf("foreign branch was inspected past identity guard: landing=%s checks=%s calls=%d", landing, checks, fc.checkCalls)
	}
}

func TestChecks_PropagatesErrors(t *testing.T) {
	sentinel := errors.New("api down")
	t.Run("head sha", func(t *testing.T) {
		st, err := newHost(&fakeClient{headErr: sentinel}, "main").Checks(context.Background(), port.Proposal{Number: 1})
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want wrapped sentinel", err)
		}
		if st != port.CheckPending {
			t.Errorf("state on error = %v, want the fail-closed pending", st)
		}
	})
	t.Run("check runs", func(t *testing.T) {
		fc := &fakeClient{headSHA: "s", runsErr: sentinel}
		st, err := newHost(fc, "main").Checks(context.Background(), port.Proposal{Number: 1})
		if !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want wrapped sentinel", err)
		}
		if st != port.CheckPending {
			t.Errorf("state on error = %v, want the fail-closed pending", st)
		}
	})
}

// reduceChecks (fed by checkContexts) is the fail-closed reduction Checks now
// delegates through: GREEN only when every run completed successfully; a
// settled failure is RED and dominates; unresolved or empty is PENDING (never
// vacuously green — DINV-3, AP-13). This is the same fixture table the
// pre-delegation aggregate() carried; every case must still answer the same
// way through checkContexts+reduceChecks directly, proving the extracted
// per-context translation and reduction preserved aggregate()'s verdicts.
func TestCheckContextsAndReduceChecks_MatchPriorAggregation(t *testing.T) {
	tests := []struct {
		name string
		runs []checkRun
		want port.CheckState
	}{
		{"no runs reported is pending, never vacuously green", nil, port.CheckPending},
		{"all success is green", []checkRun{{Status: "completed", Conclusion: "success"}, {Status: "completed", Conclusion: "success"}}, port.CheckGreen},
		{"success+neutral+skipped is green", []checkRun{{Status: "completed", Conclusion: "success"}, {Status: "completed", Conclusion: "neutral"}, {Status: "completed", Conclusion: "skipped"}}, port.CheckGreen},
		{"a failure is red", []checkRun{{Status: "completed", Conclusion: "success"}, {Status: "completed", Conclusion: "failure"}}, port.CheckRed},
		{"a settled failure dominates a still-pending run", []checkRun{{Status: "in_progress"}, {Status: "completed", Conclusion: "failure"}}, port.CheckRed},
		{"timed_out is red", []checkRun{{Status: "completed", Conclusion: "timed_out"}}, port.CheckRed},
		{"cancelled is red", []checkRun{{Status: "completed", Conclusion: "cancelled"}}, port.CheckRed},
		{"action_required is red", []checkRun{{Status: "completed", Conclusion: "action_required"}}, port.CheckRed},
		{"stale is red (fail-closed)", []checkRun{{Status: "completed", Conclusion: "stale"}}, port.CheckRed},
		{"completed with empty conclusion is red (fail-closed)", []checkRun{{Status: "completed", Conclusion: ""}}, port.CheckRed},
		{"a still-running check keeps it pending", []checkRun{{Status: "completed", Conclusion: "success"}, {Status: "queued"}}, port.CheckPending},
		{"case-insensitive completed+success is green", []checkRun{{Status: "COMPLETED", Conclusion: "SUCCESS"}}, port.CheckGreen},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reduceChecks(checkContexts(tt.runs)); got != tt.want {
				t.Errorf("reduceChecks(checkContexts(%+v)) = %v, want %v", tt.runs, got, tt.want)
			}
		})
	}
}

// TestChecks_EquivalentToPriorAggregationEndToEnd drives the same fixture
// table through the public Host.Checks contract (SHA read, then checkRuns,
// then the delegated derivation) rather than the internal helpers directly —
// the plan's own acceptance bar: a green/red/pending fixture pair yields
// identical existing-read answers before and after delegation
// (self-drive-wake/001/T4, design:state-reader-v2#3.1).
func TestChecks_EquivalentToPriorAggregationEndToEnd(t *testing.T) {
	tests := []struct {
		name string
		runs []checkRun
		want port.CheckState
	}{
		{"green: all success", []checkRun{{Status: "completed", Conclusion: "success"}, {Status: "completed", Conclusion: "neutral"}}, port.CheckGreen},
		{"red: a settled failure dominates a still-pending run", []checkRun{{Status: "in_progress"}, {Status: "completed", Conclusion: "failure"}}, port.CheckRed},
		{"pending: no runs reported, never vacuously green", nil, port.CheckPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fc := &fakeClient{headSHA: "sha-equivalence", runs: tt.runs}
			h := newHost(fc, "main")
			got, err := h.Checks(context.Background(), port.Proposal{Number: 1})
			if err != nil {
				t.Fatalf("Checks: %v", err)
			}
			if got != tt.want {
				t.Errorf("Checks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (w fakeWorkdir) ChangeSize(context.Context) (changesize.Size, error) {
	return changesize.Size{}, nil
}

func TestProposalBodyIsADescriptionThenTheAssociation(t *testing.T) {
	body := proposalBody(port.Task{ID: "T5", Number: 42, SourceRef: "#42", SourceKey: "github:owner/repo"}, "owner/repo", testDescription)
	if _, problems := prdescription.Parse(body); len(problems) != 0 {
		t.Fatalf("the proposal body misses the template: %v\n%s", problems, body)
	}
	if !strings.HasPrefix(body, prdescription.BeginMarker) || !strings.Contains(body, prdescription.EndMarker+"\n<!-- pyxis-task-proposal:") {
		t.Fatalf("description first, association after it:\n%s", body)
	}
	for _, task := range []struct {
		task port.Task
		want string
	}{
		{port.Task{ID: "T1"}, "- Implements task T1, opened by the Pyxis executor, which never merges."},
		{port.Task{SourceRef: "Carina work-item 3"}, "- Implements Carina work-item 3, opened by the Pyxis executor, which never merges."},
		{port.Task{}, "- Opened by the Pyxis executor, which never merges."},
	} {
		if got := describeTask(task.task, "owner/repo", testDescription).What; !strings.HasSuffix(got, "\n"+task.want) {
			t.Errorf("What = %q, want it to end with %q", got, task.want)
		}
	}
}

func TestDescribeProposalReplacesOnlyTheDescription(t *testing.T) {
	stale := testDescription
	stale.LinesOfCode = "1 (+1/-0)"
	task := port.Task{ID: "T5", Number: 42, SourceRef: "#42", HeadBranch: "feat/x"}
	original := proposalBody(task, "owner/repo", stale) + "\n<!-- pyxis:issue-execution-summary/v1 -->\n## Pyxis execution\n<!-- /pyxis:issue-execution-summary/v1 -->\n"
	fc := &fakeClient{existingPR: 9, existingOK: true, bodies: map[int]string{9: original}}
	h := newHost(fc, "main")
	h.repo = "owner/repo"
	found, err := h.DescribeProposal(context.Background(), task, nil, testDescription)
	if err != nil || !found || fc.bodyWrites != 1 {
		t.Fatalf("DescribeProposal = %v, %v (%d writes)", found, err, fc.bodyWrites)
	}
	body := fc.bodies[9]
	if !strings.Contains(body, "- Lines of code: 3 (+3/-0)") || strings.Contains(body, "1 (+1/-0)") || !strings.Contains(body, "## Pyxis execution") || !strings.Contains(body, "- Closes #42") {
		t.Fatalf("described body:\n%s", body)
	}
	if found, err := h.DescribeProposal(context.Background(), task, nil, testDescription); err != nil || !found || fc.bodyWrites != 1 {
		t.Fatalf("an unchanged description is not rewritten: %v, %v (%d writes)", found, err, fc.bodyWrites)
	}

	if found, err := newHost(&fakeClient{}, "main").DescribeProposal(context.Background(), task, nil, testDescription); err != nil || found {
		t.Fatalf("no open proposal: %v, %v", found, err)
	}
	if _, err := newHost(&fakeClient{}, "main").DescribeProposal(context.Background(), port.Task{ID: "T5"}, nil, testDescription); err == nil {
		t.Fatal("a task without a head branch was described")
	}
	if _, err := newHost(&fakeClient{findErr: errors.New("down")}, "main").DescribeProposal(context.Background(), task, nil, testDescription); err == nil || !strings.Contains(err.Error(), "look up the proposal to describe") {
		t.Fatalf("a failed lookup: %v", err)
	}
	failing := &fakeClient{existingPR: 9, existingOK: true, bodies: map[int]string{9: original}, setBodyErr: errors.New("denied")}
	if found, err := newHost(failing, "main").DescribeProposal(context.Background(), task, nil, testDescription); !found || err == nil || !strings.Contains(err.Error(), "describe proposal #9") {
		t.Fatalf("a failed write: %v, %v", found, err)
	}
}

func TestClientPullBodyReadAndWrite(t *testing.T) {
	var gotArgs []string
	var gotInput []byte
	c := &ghClient{repo: "o/r", run: func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte(`{"number": 4, "body": "text"}`), nil
	}, runInput: func(_ context.Context, input []byte, args ...string) ([]byte, error) {
		gotInput, gotArgs = input, args
		return nil, nil
	}}
	if body, err := c.pullBody(context.Background(), 4); err != nil || body != "text" || gotArgs[len(gotArgs)-1] != "repos/o/r/pulls/4" {
		t.Fatalf("pullBody = %q, %v (%v)", body, err, gotArgs)
	}
	if _, err := c.pullBody(context.Background(), 5); err == nil || !strings.Contains(err.Error(), "answered for #4") {
		t.Fatalf("a read of another proposal: %v", err)
	}
	if err := c.setPullBody(context.Background(), 4, "new"); err != nil || string(gotInput) != `{"body":"new"}` || gotArgs[2] != "PATCH" {
		t.Fatalf("setPullBody: %v, %s, %v", err, gotInput, gotArgs)
	}
	broken := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) { return []byte("{"), nil }}
	if _, err := broken.pullBody(context.Background(), 4); err == nil {
		t.Fatal("an undecodable read was accepted")
	}
	denied := &ghClient{repo: "o/r", run: func(context.Context, ...string) ([]byte, error) { return nil, errors.New("denied") },
		runInput: func(context.Context, []byte, ...string) ([]byte, error) { return nil, errors.New("denied") }}
	if _, err := denied.pullBody(context.Background(), 4); err == nil {
		t.Fatal("a denied read was accepted")
	}
	if err := denied.setPullBody(context.Background(), 4, "x"); err == nil {
		t.Fatal("a denied write was accepted")
	}
	if err := (&ghClient{repo: "o/r", budgetErr: errors.New("no budget")}).setPullBody(context.Background(), 4, "x"); err == nil {
		t.Fatal("a write without a budget was accepted")
	}
	if err := (&ghClient{repo: "o/r"}).setPullBody(context.Background(), 4, "x"); err == nil {
		t.Fatal("a write without a stdin runner was accepted")
	}
}

// setPullBody is a content write: it charges the shared write budget, and a
// budget that refuses the charge stops the write before anything is sent.
func TestSetPullBodyChargesTheWriteBudget(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	b, err := budget.NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	host, err := budget.HostPath("github.com")
	if err != nil {
		t.Fatal(err)
	}
	// A ledger directory others can write is refused before any window is
	// consulted, so the charge fails without waiting.
	if err := os.MkdirAll(filepath.Dir(host), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(host), 0o777); err != nil {
		t.Fatal(err)
	}
	writes := 0
	c := &ghClient{repo: "o/r", budget: b, runInput: func(context.Context, []byte, ...string) ([]byte, error) {
		writes++
		return nil, nil
	}}
	if err := c.setPullBody(context.Background(), 4, "x"); err == nil || !strings.Contains(err.Error(), "reserve content write") || writes != 0 {
		t.Fatalf("a refused charge: %v (%d writes)", err, writes)
	}
}
