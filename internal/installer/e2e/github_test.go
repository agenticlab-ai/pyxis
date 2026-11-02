package e2e

// Leg 1 — the GitHub-mode combos (A-8/FR-20; design §3.6/§3.11, ABUSE-3). The
// wiring leg is "remote-but-owned" (installer-v1 §3.11): the operation table is
// the single contract, and a fake gh transcript asserts the executed set equals
// the declared set. Live gh is the deferred remote leg — you cannot run real
// GitHub Actions hermetically — so this proves the two things a hermetic harness
// CAN prove, exactly as the design bins them:
//
//   - THE WIRING SET. For every ci-mode × wiring knob combination under
//     tracker=github, drive the REAL init pipeline (internal/installer.Init, the
//     same engine the binary runs) with an INJECTED wire.Runner transcript fake
//     — no live gh, no network. Assert (a) the ops init handed the wiring stage
//     EQUAL wire.Table's declared set for that combo, and (b) the resulting
//     audit's executed set equals that declared set, walked in table order, with
//     the executed set never exceeding the declared set (ABUSE-3). The transcript
//     runner records every gh argv, so a run that reached gh outside the table
//     would show up here.
//
//   - FR-20's "first push required check green", the hermetic half. Real Actions
//     cannot run here, so instead assert the INSTALLED WORKFLOW VARIANT lands and
//     is correct (it calls the exec-grammar gate and carries zero source-toolchain
//     steps — it WOULD run), AND that the required-check context the wiring's
//     set-protection op configures matches the workflow's job id. wire pins
//     RequiredCheckContext ↔ the shipped workflow job at the unit level
//     (TestRequiredCheckContextMatchesInstalledWorkflow); this leg pins it again
//     at the e2e level — a github-mode install lands BOTH the workflow (job
//     docs-gate) and a set-protection op whose contexts name exactly that job —
//     so the first push's required check and the workflow that satisfies it can
//     never drift apart in a real install.
//
// Why in-process init (not the subprocess binary): cmd/pyxis wires a real
// wire.GHRunner{} that is not overridable across a subprocess boundary, and an
// init that always used real gh could not run hermetically. So this leg drives
// internal/installer.Init directly with the wire seam swapped (the honest thing
// the T14 charter names) — every OTHER leg drives the subprocess binary; this
// one needs the injected runner for the github-wiring assertion. It is the same
// Init the binary calls, exercised end to end (converge from the fixture source,
// seed, hooks, the transcript-faked wiring, the green proof, the welcome).

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// ghTranscript is the injected wire.Runner: a fully hermetic gh fake that
// records every argv it is asked to run and returns benign, applied-path
// outcomes so every DECLARED gh op APPLIES (empty probe JSON → nothing exists
// → create runs and succeeds). It NEVER shells a real gh and NEVER touches the
// network — the executed set is exactly what wire.Execute walks, and this
// record is the ABUSE-3 evidence that nothing outside the table reached gh.
//
// It also answers the PREFLIGHT GitHub-mode/scope checks (design
// init-hardening-v1 §3.1 checks 3/4; plan init-hardening/001 T3), which run
// before the wiring stage ever starts: gh present, an authenticated identity,
// and a classic token carrying "repo" (the scope advisory never fires — this
// leg proves the wiring set, not the advisory; internal/preflight's own
// github_test.go owns that matrix).
type ghTranscript struct {
	calls [][]string // every gh argv, in invocation order

	// repoExists answers the `repo view` call's ONE caller for a given combo:
	// preflight's existence probe (wiring.repo=use — it wants "found", read
	// access confirmed) or the wiring stage's own create-repo probe
	// (wiring.repo=create — it wants the expected "not found" miss so
	// create-repo actually creates it, AP-22). The two callers are mutually
	// exclusive per combo (preflight only probes under use; create-repo only
	// exists under create), so one field serves both.
	repoExists bool

	// authFails answers the preflight GitHub-mode check's `gh auth status`
	// probe with a logged-out result instead of the happy path — the
	// injection matrix's gh-logged-out defect class (plan
	// init-hardening/001 T11, design init-hardening-v1 §3.1 check 3). Zero
	// value (false) is the unchanged happy path every existing combo relies
	// on.
	authFails bool

	// protectionForbidden answers the branch-protection PUT with GitHub's
	// admin-rights 403 — the prd:init-hardening/UB-4 permission class
	// (amendment 001, the operator-ruled hand-off): wire classifies it
	// permission, hands the op off, and the run completes green with no
	// rollback. Zero value keeps every mutation succeeding.
	protectionForbidden bool
}

func (g *ghTranscript) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	g.calls = append(g.calls, append([]string(nil), args...))
	// Preflight: `gh --version` must succeed so the walk starts.
	if len(args) == 1 && args[0] == "--version" {
		return "gh version 2.40.0 (fake transcript)\n", "", 0, nil
	}
	// Preflight: an authenticated identity (check 3).
	if len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
		if g.authFails {
			return "", "You are not logged into any GitHub hosts.\n", 1, nil
		}
		return "", "Logged in to github.com as e2e-fake\n", 0, nil
	}
	// Preflight: the best-effort scope prediction (check 4).
	if len(args) >= 3 && args[0] == "api" && args[1] == "-i" && args[2] == "user" {
		return "HTTP/2.0 200 OK\nX-Oauth-Scopes: repo\n\n{}", "", 0, nil
	}
	// The read-only existence probes: return an EMPTY set so every op proceeds
	// to its create/PUT and applies (the "nothing exists yet" fresh-install
	// path). label list / issue list want a JSON array; repo view answers
	// EITHER the preflight's existence check or the wiring stage's create-repo
	// probe per repoExists (see the field doc above).
	if len(args) >= 2 && args[0] == "label" && args[1] == "list" {
		return "[]", "", 0, nil
	}
	if len(args) >= 2 && args[0] == "issue" && args[1] == "list" {
		return "[]", "", 0, nil
	}
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		if g.repoExists {
			return `{"name":"e2e-repo"}`, "", 0, nil
		}
		// A not-found miss so create-repo proceeds (gh spells it on stderr,
		// nonzero exit — a completed run, not a transport error).
		return "", "gh: Could not resolve to a Repository with the name.", 1, nil
	}
	// The permission class (UB-4): the protection PUT — and only it —
	// answers with the admin-rights 403 wire's permission patterns match.
	if g.protectionForbidden {
		for _, a := range args {
			if strings.Contains(a, "/protection") {
				return "", "HTTP 403: Must have admin rights to Repository.", 1, nil
			}
		}
	}
	// Every mutation (repo create, label create, protection PUT, issue create)
	// succeeds cleanly.
	return "", "", 0, nil
}

// ghCombo is one github-mode knob combination the leg drives end to end.
type ghCombo struct {
	name       string
	ciMode     string // local | actions | both
	repo       string // use | create
	protection string // none | required-checks | checks+review
}

// TestGitHubModeCombos drives the real init pipeline in github mode across the
// ci-mode × wiring knob space with an injected transcript runner, and asserts
// the executed wiring set equals wire.Table's declared set every time.
func TestGitHubModeCombos(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	// A representative, non-trivial spread of the knob space: every wiring.repo
	// value, every protection level, and every ci-mode, so the declared table
	// varies row-by-row across the runs (create-repo on/off, push-workflow
	// on/off, set-protection on/off/with-review).
	combos := []ghCombo{
		{"use+required+both", "both", "use", "required-checks"},
		{"create+review+actions", "actions", "create", "checks+review"},
		{"use+none+local", "local", "use", "none"},
		{"create+required+both", "both", "create", "required-checks"},
	}
	for _, c := range combos {
		t.Run(c.name, func(t *testing.T) {
			// The declared table for this combo, derived independently from the
			// same knobs — the yardstick the executed set must equal. The
			// onboarding standing set comes from the installer's ONE builder
			// (#947), exactly as init declares it.
			declared := wire.Table(wire.Config{
				Repo:            repoConfigForCombo(c),
				Slug:            fixtureSlug,
				OnboardingItems: installer.OnboardingWorkItems(repoConfigForCombo(c)),
			})

			// Inject the transcript fake as the wiring transport through the
			// production Options.WireRunner seam — the same wire.Execute the
			// binary runs, over a hermetic gh (no live gh, no network).
			// repoExists mirrors wiring.repo: "use" wires up an existing
			// repository (preflight's existence probe wants "found"); "create"
			// wants the create-repo probe's expected "not found" miss.
			transcript := &ghTranscript{repoExists: c.repo == "use"}
			repo := githubScratchRepo(t)
			rep := runGitHubInit(t, repo, r, c, transcript)

			if rep.T8.Wiring == nil {
				t.Fatalf("github-mode init recorded no wiring audit")
			}
			audit := *rep.T8.Wiring

			// (a) The audit's executed set equals wire.Table's declared set, in
			// table order — the executed set never exceeds the declared set
			// (ABUSE-3). Init built the walked table from wire.Table(the captured
			// knobs); the yardstick is wire.Table(the same knobs).
			assertAuditMatchesDeclared(t, declared, audit)

			// (b) Every op the audit walked APPLIED or was converge-carried (the
			// transcript returns benign outcomes) — no partial, no hand-off.
			if audit.Partial || audit.HandOff != "" {
				t.Fatalf("clean transcript yielded a partial/hand-off audit: partial=%v handoff=%q", audit.Partial, audit.HandOff)
			}

			// (c) The transcript is the ABUSE-3 egress record: every gh argv it
			// saw belongs to the declared table's op vocabulary (or the
			// preflight) — nothing outside the table reached gh.
			assertTranscriptWithinTable(t, declared, c, transcript)

			// FR-20 (hermetic half): a github-mode install lands the installed
			// workflow variant, and — when a protection level is set — the
			// set-protection op's required-check context matches the workflow job.
			assertWorkflowAndRequiredCheck(t, repo, c, declared)
		})
	}
}

// repoConfigForCombo builds the operand config wire.Table reads for a combo —
// exactly the knobs runGitHubInit resolves, so the yardstick table is derived
// from the same inputs the pipeline uses (tracker=github; the branch defaults
// to main as the fixture repo does).
func repoConfigForCombo(c ghCombo) config.Config {
	return config.Config{
		Knobs: config.Knobs{
			Tracker: "github",
			CIMode:  c.ciMode,
			Wiring:  config.Wiring{Repo: c.repo, Protection: c.protection},
		},
	}
}

// runGitHubInit drives internal/installer.Init in github mode against a
// pre-inited scratch repo carrying an origin remote (so the wiring stage
// resolves the slug), fully flag-driven (promptless), converging from the
// fixture file:// source, with the wiring transport injected via the production
// Options.WireRunner seam. It asserts init ends green (a real Report, EVT-1
// fired). Identity travels in Answers so no ambient git identity is needed.
func runGitHubInit(t *testing.T, repo string, r fixtureRelease, c ghCombo, runner wire.Runner) installer.Report {
	t.Helper()
	argus := false
	opts := installer.Options{
		Target:         repo,
		Pin:            installer.Pin{Version: r.version, ManifestSHA256: r.pin},
		ArtifactSource: r.source(),
		Answers: installer.Answers{
			Tracker:          "github",
			CIMode:           c.ciMode,
			WiringRepo:       c.repo,
			WiringProtection: c.protection,
			CLISurfaces:      []string{}, // explicit empty: PATH-independent
			Argus:            &argus,
			GitName:          "E2E Operator",
			GitEmail:         "e2e@example.com",
		},
		AnswersProvided: true, // promptless (FR-4)
		Fetcher:         resolve.FileFetcher{},
		WireRunner:      runner, // the hermetic gh transcript fake
		PublicationGit:  localPublicationTransport(t),
		Stdout:          discardWriter{},
		Logger:          slog.New(slog.NewTextHandler(discardWriter{}, nil)),
	}
	rep, err := installer.Init(context.Background(), opts)
	if err != nil {
		t.Fatalf("github-mode init (%s) did not end green: %v", c.name, err)
	}
	if !rep.T8.GreenProof.Green {
		t.Fatalf("github-mode init (%s) did not reach a green proof", c.name)
	}
	if rep.Knobs.Tracker != "github" {
		t.Fatalf("resolved tracker = %q, want github", rep.Knobs.Tracker)
	}
	return rep
}

// fixtureSlug is the owner/name coordinate the scratch repo's origin remote
// points at — the target every declared gh op names.
const fixtureSlug = "e2e-owner/e2e-repo"

// githubScratchRepo creates a fresh git repo carrying ONLY an origin remote
// (zero tracked content, clean worktree) — admitted by init's UB-11 probe as an
// empty repo, and carrying the origin the wiring stage's slug discovery reads.
// Git identity/config is isolated via t.Setenv so no developer identity leaks
// and the in-process git subprocesses are hermetic.
func githubScratchRepo(t *testing.T) string {
	t.Helper()
	// Isolate the ambient git environment the in-process init's git subprocesses
	// inherit (they run with os.Environ()): a HOME with no gitconfig, a
	// neutralized global/system config, and pinned commit dates.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-02T03:04:05Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T03:04:05Z")

	repo := t.TempDir()
	// git init on main + an origin remote; NO tracked content (init admits it
	// as an empty repo and does not git-init again).
	mustGit(t, repo, "init", "-b", "main")
	mustGit(t, repo, "remote", "add", "origin", "https://github.com/"+fixtureSlug+".git")
	return repo
}

// mustGit runs a git subcommand in repo, fataling on error.
func mustGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// assertAuditMatchesDeclared asserts the audit walked exactly the declared
// table, in order — the executed set equals the declared set and never exceeds
// it (ABUSE-3).
func assertAuditMatchesDeclared(t *testing.T, declared []wire.Op, audit wire.Audit) {
	t.Helper()
	names := audit.OperationNames()
	if len(names) != len(declared) {
		t.Fatalf("audit walked %d op(s), the declared table has %d\n declared: %s\n walked:   %s",
			len(names), len(declared), opNames(declared), strings.Join(names, ", "))
	}
	for i, op := range declared {
		if names[i] != op.Name {
			t.Fatalf("audit op[%d] = %q, declared %q (executed set diverged from the declared set)", i, names[i], op.Name)
		}
	}
}

// assertTranscriptWithinTable asserts every gh argv the transcript saw maps to
// a declared op's gh surface or the preflight's own fixed vocabulary (design
// init-hardening-v1 §3.1 checks 3/4; plan init-hardening/001 T3) — nothing
// outside the table or the preflight ever reached gh (ABUSE-3, held by
// construction and checked here as evidence).
func assertTranscriptWithinTable(t *testing.T, declared []wire.Op, c ghCombo, transcript *ghTranscript) {
	t.Helper()
	allowed := map[string]bool{
		// The preflight's fixed vocabulary: gh presence is exempted below
		// (single-token "--version"); an authenticated identity and the
		// best-effort scope prediction both run whenever github-mode is
		// applicable, which every combo in this leg is (tracker=github).
		"auth status": true,
		"api -i":      true,
	}
	if c.repo == "use" {
		// Preflight's existence probe fires only under wiring.repo=use —
		// absence is the expected state under create, so create alone never
		// triggers it (the wiring stage's own create-repo probe below does).
		allowed["repo view"] = true
	}
	for _, op := range declared {
		switch op.Name {
		case wire.OpCreateRepo:
			allowed["repo view"] = true
			allowed["repo create"] = true
		case wire.OpEnsureLabels:
			allowed["label list"] = true
			allowed["label create"] = true
		case wire.OpSetProtection:
			allowed["api --method"] = true
		case wire.OpWelcomeItem:
			allowed["issue list"] = true
			allowed["issue create"] = true
		}
	}
	for _, argv := range transcript.calls {
		if len(argv) == 1 && argv[0] == "--version" {
			continue // the preflight's gh-presence probe
		}
		key := ghKey(argv)
		if !allowed[key] {
			t.Fatalf("transcript saw a gh call %q outside the declared table's surface %v — the executed set exceeded the declared set (ABUSE-3)", strings.Join(argv, " "), allowedKeys(allowed))
		}
	}
}

// ghKey reduces a gh argv to its "verb subverb" surface key (or just "verb"
// for a single-token surface). "api" alone would conflate the scope
// prediction's read (`api -i user`) with set-protection's PUT (`api
// --method PUT ...`) — two very different operations — so both keep their
// second token rather than collapsing to a bare "api".
func ghKey(argv []string) string {
	if len(argv) >= 2 {
		return argv[0] + " " + argv[1]
	}
	return argv[0]
}

func allowedKeys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// assertWorkflowAndRequiredCheck is FR-20's hermetic half: the installed-repo
// workflow variant landed in the operand (job id docs-gate, exec-grammar gate,
// zero source-toolchain steps), and — when the combo sets a protection level —
// the declared set-protection op names exactly that job as its required-check
// context. The workflow-that-satisfies-the-required-check and the required
// check cannot drift apart in a real github-mode install.
func assertWorkflowAndRequiredCheck(t *testing.T, repo string, c ghCombo, declared []wire.Op) {
	t.Helper()

	// The operand lands the workflow pointer at .github/workflows/docs-gate.yml
	// (converge materialized it — engine-verbatim payload) exactly when ci_mode
	// includes hosted CI (init-hardening T6, §3.8's desired-set knob predicate,
	// mirrored from wire.Table's own OpPushWorkflow condition above); "local"
	// excludes it from the desired set entirely — unlisted, never written. In
	// THIS harness the fixture payload packs a stub at that path (
	// buildFixtureRelease keeps the release cheap), so the operand-side
	// assertion is only that the pointer landed or didn't; the SUBSTANCE of the
	// shipped variant is asserted against the real source-repo artifact below —
	// the same file the class policy maps to that target and the wire
	// consistency test pins.
	included := c.ciMode == "actions" || c.ciMode == "both"
	_, err := os.Stat(at(repo, payload.InstalledWorkflowTarget))
	if included && err != nil {
		t.Fatalf("operand landed no workflow at %s: %v", payload.InstalledWorkflowTarget, err)
	}
	if !included {
		if err == nil {
			t.Fatalf("ci_mode=%s excludes hosted CI but the operand landed a workflow at %s", c.ciMode, payload.InstalledWorkflowTarget)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("stat %s: %v", payload.InstalledWorkflowTarget, err)
		}
		return // excluded: nothing further to assert against an unmaterialized operand file
	}

	// The REAL installed workflow variant that ships in the payload
	// (.pyxis/dist/github-workflows/docs-gate.yml): it drives the gate through
	// the exec grammar and holds zero source-toolchain steps (FR-20; §3.10).
	wfs := string(readSourceFile(t, payload.InstalledWorkflow))
	if !strings.Contains(wfs, "pyxis exec docgate -- gate") {
		t.Fatalf("installed workflow variant does not invoke the exec-grammar gate:\n%s", wfs)
	}
	for _, forbidden := range []string{"setup-go", "go vet", "go test", "gofmt"} {
		if strings.Contains(wfs, forbidden) {
			t.Fatalf("installed workflow variant carries a source-toolchain step %q (must be source-repo-only):\n%s", forbidden, wfs)
		}
	}
	// The workflow's job id — the required-check context a first push must clear.
	jobID := workflowJobID(t, wfs)
	if jobID != wire.RequiredCheckContext {
		t.Fatalf("installed workflow job id %q != wire.RequiredCheckContext %q", jobID, wire.RequiredCheckContext)
	}

	// The set-protection op (present iff protection ≠ none) names exactly the
	// workflow's job as its required check — the FR-20 "first push required
	// check green" contract's structural half: the required check IS the job the
	// installed workflow runs.
	var prot *wire.Op
	for i := range declared {
		if declared[i].Name == wire.OpSetProtection {
			prot = &declared[i]
			break
		}
	}
	if c.protection == "none" {
		if prot != nil {
			t.Fatalf("protection=none declared a set-protection op")
		}
		return
	}
	if prot == nil {
		t.Fatalf("protection=%s declared no set-protection op", c.protection)
	}
	if len(prot.Contexts) != 1 || prot.Contexts[0] != jobID {
		t.Fatalf("set-protection contexts = %v, want [%s] (the installed workflow's job) — the required check and the workflow that satisfies it would drift", prot.Contexts, jobID)
	}
}

// workflowJobID extracts the single job id under `jobs:` from the installed
// workflow (the required-check context). The installed variant defines exactly
// one job; this parses its key.
func workflowJobID(t *testing.T, wf string) string {
	t.Helper()
	lines := strings.Split(wf, "\n")
	inJobs := false
	for _, ln := range lines {
		if strings.TrimRight(ln, " ") == "jobs:" {
			inJobs = true
			continue
		}
		if inJobs {
			if strings.HasPrefix(ln, "  ") && strings.HasSuffix(strings.TrimRight(ln, " "), ":") {
				return strings.TrimSuffix(strings.TrimSpace(ln), ":")
			}
		}
	}
	t.Fatalf("no job id found under jobs: in the installed workflow:\n%s", wf)
	return ""
}

// opNames renders a declared-op slice as "name(condition)" for failure output.
func opNames(ops []wire.Op) string {
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		parts = append(parts, op.Name+"("+op.Condition+")")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
