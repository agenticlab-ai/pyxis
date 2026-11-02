package preflight_test

// The GitHub-mode + scope-prediction fixture matrix (design
// init-hardening-v1 §3.1 checks 3/4; plan init-hardening/001 T3's
// acceptance criteria): applicability derives from the declared operation
// table (never the tracker knob alone); gh presence/auth/existence refuse
// pre-write naming a remedy; carina-mode wiring-free runs touch gh zero
// times; scope advisories warn on knowable gaps and stay silent on
// unknowable ones, never blocking.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

// scriptedResp is one canned gh invocation's answer.
type scriptedResp struct {
	stdout, stderr string
	exit           int
	err            error
}

// scriptedRunner is a hermetic wire.Runner fake: version/auth/user are each
// one scripted answer; repoView is a QUEUE (each call consumes the next
// entry; the last repeats once exhausted) so retry sequences (transient-
// then-ok, transient-forever) are expressible. failOnCall fails the test
// outright on any invocation — the "zero GitHub calls" acceptance
// criterion's tool.
type scriptedRunner struct {
	t *testing.T

	version  scriptedResp
	auth     scriptedResp
	user     scriptedResp
	repoView []scriptedResp

	calls         [][]string
	repoViewCalls int
	failOnCall    bool
}

// newHappyRunner returns a runner scripted for the all-clear path: gh
// present, authenticated, a classic token carrying "repo", and any
// existence probe succeeding — tests override just the field(s) their
// scenario needs to fail.
func newHappyRunner(t *testing.T) *scriptedRunner {
	t.Helper()
	return &scriptedRunner{
		t:        t,
		version:  scriptedResp{stdout: "gh version 2.40.0 (2026-01-01)\n", exit: 0},
		auth:     scriptedResp{stderr: "Logged in to github.com as octocat\n", exit: 0},
		user:     scriptedResp{stdout: "HTTP/2.0 200 OK\nX-Oauth-Scopes: repo, read:org\n\n{\"login\":\"octocat\"}", exit: 0},
		repoView: []scriptedResp{{stdout: `{"name":"widget"}`, exit: 0}},
	}
}

func (r *scriptedRunner) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	r.t.Helper()
	if r.failOnCall {
		r.t.Fatalf("RunGH called with %v — want zero gh calls", args)
	}
	r.calls = append(r.calls, append([]string(nil), args...))
	switch {
	case len(args) >= 1 && args[0] == "--version":
		return r.version.stdout, r.version.stderr, r.version.exit, r.version.err
	case len(args) >= 2 && args[0] == "auth" && args[1] == "status":
		return r.auth.stdout, r.auth.stderr, r.auth.exit, r.auth.err
	case len(args) >= 2 && args[0] == "repo" && args[1] == "view":
		idx := r.repoViewCalls
		if idx >= len(r.repoView) {
			idx = len(r.repoView) - 1
		}
		r.repoViewCalls++
		resp := r.repoView[idx]
		return resp.stdout, resp.stderr, resp.exit, resp.err
	case len(args) >= 2 && args[0] == "api" && args[1] == "-i":
		return r.user.stdout, r.user.stderr, r.user.exit, r.user.err
	}
	r.t.Fatalf("scriptedRunner: unhandled gh args %v", args)
	return "", "", 0, nil
}

// outcomeFor finds id's outcome in rep, reporting whether it appeared at
// all (an inapplicable check never does).
func outcomeFor(t *testing.T, rep preflight.Report, id string) (preflight.Outcome, bool) {
	t.Helper()
	for _, o := range rep {
		if o.Check == id {
			return o, true
		}
	}
	return preflight.Outcome{}, false
}

// ghModeKnobs is a representative always-applicable, never-wedged GitHub-
// mode knob set (tracker=github, wiring.repo=use, protection=required-
// checks, ci_mode=both) — the fixture most single-defect tests start from.
func ghModeKnobs() config.Knobs {
	return config.Knobs{
		Tracker: "github",
		CIMode:  "both",
		Wiring:  config.Wiring{Repo: "use", Protection: "required-checks"},
	}
}

func TestRegistryOrderMatchesDesign(t *testing.T) {
	var order []string
	for _, c := range preflight.Registry() {
		order = append(order, c.ID)
	}
	want := []string{preflight.AdmissionID, preflight.ToolchainID, preflight.ProcessBoundaryID, preflight.GitHubModeID, preflight.ScopePredictionID, preflight.ArtifactSourceID, preflight.CrossKnobCoherenceID}
	if len(order) != len(want) {
		t.Fatalf("Registry() order = %v, want %v (design §3.1's complete numbered list)", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("Registry() order = %v, want %v", order, want)
		}
	}
}

func TestGitHubModeApplicabilityDerivesFromOperationTable(t *testing.T) {
	cases := []struct {
		name           string
		knobs          config.Knobs
		wantApplicable bool
	}{
		{
			name:           "carina, wiring-free — inapplicable (wire's own escape hatch)",
			knobs:          config.Knobs{Tracker: "carina", CIMode: "local", Wiring: config.Wiring{Repo: "use", Protection: "none"}},
			wantApplicable: false,
		},
		{
			name:           "github tracker alone is applicable (ensure-labels/welcome-item)",
			knobs:          config.Knobs{Tracker: "github", CIMode: "local", Wiring: config.Wiring{Repo: "use", Protection: "none"}},
			wantApplicable: true,
		},
		{
			name:           "carina tracker with branch protection is STILL applicable — wiring is tracker-independent",
			knobs:          config.Knobs{Tracker: "carina", CIMode: "both", Wiring: config.Wiring{Repo: "use", Protection: "required-checks"}},
			wantApplicable: true,
		},
		{
			name:           "carina tracker with wiring.repo=create is STILL applicable",
			knobs:          config.Knobs{Tracker: "carina", CIMode: "local", Wiring: config.Wiring{Repo: "create", Protection: "none"}},
			wantApplicable: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := t.TempDir()
			runner := newHappyRunner(t)
			rep, err := preflight.Run(context.Background(), preflight.Input{
				Target: target, Knobs: tc.knobs, Coordinate: "octo/widget", WireRunner: runner,
			})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if _, ok := outcomeFor(t, rep, preflight.GitHubModeID); ok != tc.wantApplicable {
				t.Errorf("github-mode present in report = %v, want %v (report: %+v)", ok, tc.wantApplicable, rep)
			}
			if _, ok := outcomeFor(t, rep, preflight.ScopePredictionID); ok != tc.wantApplicable {
				t.Errorf("scope-prediction present in report = %v, want %v", ok, tc.wantApplicable)
			}
		})
	}
}

// TestCarinaModeZeroGitHubCalls is T3's AC1: carina-mode wiring-free runs
// perform ZERO GitHub calls — proven with a runner that fails the test the
// instant it is invoked at all.
func TestCarinaModeZeroGitHubCalls(t *testing.T) {
	target := t.TempDir()
	runner := &scriptedRunner{t: t, failOnCall: true}
	knobs := config.Knobs{Tracker: "carina", CIMode: "local", Wiring: config.Wiring{Repo: "use", Protection: "none"}}
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target, Knobs: knobs, WireRunner: runner})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := outcomeFor(t, rep, preflight.GitHubModeID); ok {
		t.Error("github-mode check appeared in the report for wiring-free carina knobs")
	}
	if len(runner.calls) != 0 {
		t.Errorf("gh was called %d time(s): %v — carina-mode wiring-free knobs must perform zero GitHub calls", len(runner.calls), runner.calls)
	}
}

// TestGHAbsentRefusesPreWrite is T3's AC1: logged-out (here, entirely
// absent) gh refuses pre-write naming the remedy.
func TestGHAbsentRefusesPreWrite(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.version = scriptedResp{err: errors.New(`exec: "gh": executable file not found in $PATH`)}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v (found=%v), want refuse", out, ok)
	}
	if !strings.Contains(out.Remedy, "install gh") {
		t.Errorf("remedy %q does not name the install remedy", out.Remedy)
	}
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) || refusal.Outcome.Check != preflight.GitHubModeID {
		t.Errorf("returned error does not wrap the github-mode refusal: %v", err)
	}
}

// TestGHNotAuthenticatedRefusesPreWrite is T3's AC1's other half: gh
// present but logged out refuses naming `gh auth login`.
func TestGHNotAuthenticatedRefusesPreWrite(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.auth = scriptedResp{stderr: "You are not logged into any GitHub hosts.\n", exit: 1}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v (found=%v), want refuse", out, ok)
	}
	if !strings.Contains(out.Remedy, "gh auth login") {
		t.Errorf("remedy %q does not name `gh auth login`", out.Remedy)
	}
}

// TestUseAgainstMissingRepoRefuses is T3's AC1: `use` against a missing or
// inaccessible repository refuses pre-write with the remedy.
func TestUseAgainstMissingRepoRefuses(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.repoView = []scriptedResp{{stderr: "gh: Could not resolve to a Repository with the name 'octo/widget'. (HTTP 404)", exit: 1}}
	knobs := ghModeKnobs() // Wiring.Repo == "use"
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: knobs, Coordinate: "octo/widget", WireRunner: runner,
	})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v (found=%v), want refuse", out, ok)
	}
	if !strings.Contains(out.Remedy, "wiring.repo=create") {
		t.Errorf("remedy %q does not name the create alternative", out.Remedy)
	}
}

// TestCreateModeSkipsExistenceProbe: absence is the expected state under
// wiring.repo=create (design §3.1) — no existence probe runs at all.
func TestCreateModeSkipsExistenceProbe(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.repoView = []scriptedResp{{stderr: "should never be invoked", exit: 1}}
	knobs := config.Knobs{Tracker: "github", CIMode: "local", Wiring: config.Wiring{Repo: "create", Protection: "none"}}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: knobs, Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusPass {
		t.Fatalf("github-mode outcome = %+v (found=%v), want pass", out, ok)
	}
	if runner.repoViewCalls != 0 {
		t.Errorf("repo view was called %d time(s) under wiring.repo=create — absence is expected, no probe needed", runner.repoViewCalls)
	}
}

// TestExistenceProbeRetriesTransientThenSucceeds proves the bounded
// transient retry (design §3.10, risk R4): two transient failures then a
// clean success still passes, having retried through wire.IsTransient's
// pattern vocabulary.
func TestExistenceProbeRetriesTransientThenSucceeds(t *testing.T) {
	defer preflight.SetProbeSleepForTest()()
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.repoView = []scriptedResp{
		{stderr: "gh: Internal Server Error (HTTP 500)", exit: 1},
		{stderr: "gh: Internal Server Error (HTTP 500)", exit: 1},
		{stdout: `{"name":"widget"}`, exit: 0},
	}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, _ := outcomeFor(t, rep, preflight.GitHubModeID)
	if out.Status != preflight.StatusPass {
		t.Fatalf("github-mode outcome = %+v, want pass after transient retries", out)
	}
	if runner.repoViewCalls != 3 {
		t.Errorf("repo view called %d time(s), want 3 (2 transient failures + 1 success)", runner.repoViewCalls)
	}
}

// TestExistenceProbeBoundsTransientRetries proves the retry bound itself:
// a probe that is ALWAYS transient exhausts exactly the bounded attempt
// count, then refuses naming the retry.
func TestExistenceProbeBoundsTransientRetries(t *testing.T) {
	defer preflight.SetProbeSleepForTest()()
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.repoView = []scriptedResp{{stderr: "gh: Internal Server Error (HTTP 500)", exit: 1}} // always transient
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal after exhausting retries")
	}
	out, _ := outcomeFor(t, rep, preflight.GitHubModeID)
	if out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v, want refuse", out)
	}
	if !strings.Contains(out.Reason, "attempts") {
		t.Errorf("reason %q does not name the retry attempts", out.Reason)
	}
	if runner.repoViewCalls != 3 {
		t.Errorf("repo view called %d time(s), want 3 (the bounded max)", runner.repoViewCalls)
	}
}

// TestCoordinateResolvesFromConfiguredRemote: a configured remote wins over
// the promptless Coordinate input (design §3.7's resolution order).
func TestCoordinateResolvesFromConfiguredRemote(t *testing.T) {
	isolateGit(t)
	target := t.TempDir()
	git(t, target, "init", "-b", "main")
	git(t, target, "remote", "add", "origin", "https://github.com/octo/widget.git")
	runner := newHappyRunner(t)
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "ignored/shouldnotbeused", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, _ := outcomeFor(t, rep, preflight.GitHubModeID)
	if out.Status != preflight.StatusPass {
		t.Fatalf("github-mode outcome = %+v, want pass", out)
	}
	if !calledRepoView(runner, "octo/widget") {
		t.Errorf("gh calls %v never targeted octo/widget (the configured remote) — the promptless Coordinate must not win over it", runner.calls)
	}
}

// TestCoordinatePromptlessFallbackWhenNoRemote: no remote configured falls
// through to the promptless Coordinate input.
func TestCoordinatePromptlessFallbackWhenNoRemote(t *testing.T) {
	isolateGit(t)
	target := t.TempDir() // no git repo at all — no remote to parse
	runner := newHappyRunner(t)
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, _ := outcomeFor(t, rep, preflight.GitHubModeID)
	if out.Status != preflight.StatusPass {
		t.Fatalf("github-mode outcome = %+v, want pass", out)
	}
	if !calledRepoView(runner, "octo/widget") {
		t.Errorf("gh calls %v never targeted the promptless coordinate octo/widget", runner.calls)
	}
}

// TestCoordinateMissingBothRefusesNamingFlagAndAnswersFile is T3's AC2: a
// promptless GitHub-mode run with neither a remote nor a coordinate
// refuses pre-write naming the flag and answers-file remedies (FR-7).
func TestCoordinateMissingBothRefusesNamingFlagAndAnswersFile(t *testing.T) {
	isolateGit(t)
	target := t.TempDir() // no remote, no promptless coordinate
	runner := newHappyRunner(t)
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target, Knobs: ghModeKnobs(), WireRunner: runner})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v (found=%v), want refuse", out, ok)
	}
	if !strings.Contains(out.Remedy, "--coordinate") {
		t.Errorf("remedy %q does not name the --coordinate flag", out.Remedy)
	}
	if !strings.Contains(out.Remedy, "answers file") {
		t.Errorf("remedy %q does not name the answers-file remedy", out.Remedy)
	}
	if runner.repoViewCalls != 0 {
		t.Error("an existence probe ran with no resolved coordinate")
	}
}

// TestCoordinateMalformedRefuses proves ABUSE-1's grammar mitigation
// extends to the promptless door: a hostile/malformed coordinate refuses
// and never reaches an existence probe's argv.
func TestCoordinateMalformedRefuses(t *testing.T) {
	isolateGit(t)
	target := t.TempDir()
	runner := newHappyRunner(t)
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "--upload-pack=/bin/sh", WireRunner: runner,
	})
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	out, ok := outcomeFor(t, rep, preflight.GitHubModeID)
	if !ok || out.Status != preflight.StatusRefuse {
		t.Fatalf("github-mode outcome = %+v (found=%v), want refuse", out, ok)
	}
	if !strings.Contains(out.Reason, "not a valid owner/name") {
		t.Errorf("reason %q does not name the grammar defect", out.Reason)
	}
	if runner.repoViewCalls != 0 {
		t.Errorf("repo view was called %d time(s) — a malformed coordinate must never reach an existence probe's argv", runner.repoViewCalls)
	}
}

// calledRepoView reports whether runner saw a `repo view <slug>` call.
func calledRepoView(runner *scriptedRunner, slug string) bool {
	for _, c := range runner.calls {
		if len(c) >= 3 && c[0] == "repo" && c[1] == "view" && c[2] == slug {
			return true
		}
	}
	return false
}

// TestScopePredictionSilentWhenRepoScopePresent: the happy-path classic
// token already carries "repo" — no advisory.
func TestScopePredictionSilentWhenRepoScopePresent(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, ok := outcomeFor(t, rep, preflight.ScopePredictionID)
	if !ok || out.Status != preflight.StatusPass {
		t.Fatalf("scope-prediction outcome = %+v (found=%v), want pass", out, ok)
	}
}

// TestScopePredictionAdvisesOnKnowableGap is T3's AC3's "warns on knowable
// gaps" half: a classic token missing "repo" advises, never refuses.
func TestScopePredictionAdvisesOnKnowableGap(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.user = scriptedResp{stdout: "HTTP/2.0 200 OK\nX-Oauth-Scopes: read:org\n\n{}", exit: 0}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v (scope advisories must never block, DINV-7)", err)
	}
	out, ok := outcomeFor(t, rep, preflight.ScopePredictionID)
	if !ok || out.Status != preflight.StatusAdvise {
		t.Fatalf("scope-prediction outcome = %+v (found=%v), want advise", out, ok)
	}
	if !strings.Contains(out.Warning, "repo") {
		t.Errorf("warning %q does not name the missing repo scope", out.Warning)
	}
}

// TestScopePredictionSilentWhenUnknowable is T3's AC3's "silent on
// unknowable gaps" half: a fine-grained-token-shaped response (no
// X-Oauth-Scopes header at all) never advises.
func TestScopePredictionSilentWhenUnknowable(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.user = scriptedResp{stdout: "HTTP/2.0 200 OK\nContent-Type: application/json\n\n{}", exit: 0}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, ok := outcomeFor(t, rep, preflight.ScopePredictionID)
	if !ok || out.Status != preflight.StatusPass {
		t.Fatalf("scope-prediction outcome = %+v (found=%v), want pass (silent, unknowable)", out, ok)
	}
}

// TestScopePredictionSilentWhenAPICallFails: the scope probe itself
// failing is ALSO unknowable, never a refusal — check 3 already names any
// real gh/auth defect on its own terms.
func TestScopePredictionSilentWhenAPICallFails(t *testing.T) {
	target := t.TempDir()
	runner := newHappyRunner(t)
	runner.user = scriptedResp{stderr: "gh: Bad Gateway (HTTP 502)", exit: 1}
	rep, err := preflight.Run(context.Background(), preflight.Input{
		Target: target, Knobs: ghModeKnobs(), Coordinate: "octo/widget", WireRunner: runner,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out, ok := outcomeFor(t, rep, preflight.ScopePredictionID)
	if !ok || out.Status != preflight.StatusPass {
		t.Fatalf("scope-prediction outcome = %+v (found=%v), want pass (unknowable on failure — never refuse, DINV-7)", out, ok)
	}
}
