package e2e

// The composed check-defect injection matrix (plan init-hardening/001 T11;
// design init-hardening-v1 §3.10, A-2): every per-check seeded defect the
// design's acceptance table names — toolchain absence, toolchain below-floor,
// logged-out gh, a missing repository under `use`, an unreachable artifact
// source, a missing promptless coordinate, and the wedged ci_mode/protection
// pair — driven through the REAL init flow (the shipped binary where no
// transport injection is needed; internal/installer.Init in-process, exactly
// as github_test.go's own leg does, where a hermetic gh transcript is the
// only way to exercise the defect at all). Each asserts three things
// together: the refusal fires naming that check, the target is left
// byte-identical (DINV-1 — zero writes), and the init_refused event (EVT-2)
// carries its typed properties. internal/preflight's own fixture matrices
// already prove each defect AT THE LIBRARY level in isolation; this proves
// the SAME defects reach the same refusal through the actual shipped
// pipeline, composed in one matrix.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// assertNoMutatingGH is A-2/FR-9's transport half (review:impl #812 FND-2):
// after a pre-write refusal the gh transcript may carry only the read-only
// probes the checks themselves run (--version · auth status · the scope
// header read · repo view) — a single mutating argv means a remote mutation
// preceded the refusal, the exact DINV-1 breach the fit forbids. Refusal
// classes upstream of the GitHub-mode checks (admission, toolchain) pass a
// fresh transcript and therefore assert FULLY empty here too.
func assertNoMutatingGH(t *testing.T, runner *ghTranscript, leg string) {
	t.Helper()
	for _, call := range runner.calls {
		if len(call) == 0 {
			continue
		}
		readOnly := call[0] == "--version" ||
			(len(call) >= 2 && call[0] == "auth" && call[1] == "status") ||
			(len(call) >= 3 && call[0] == "api" && call[1] == "-i" && call[2] == "user") ||
			(len(call) >= 2 && call[0] == "repo" && call[1] == "view")
		if !readOnly {
			t.Errorf("%s: a mutating gh call preceded the refusal (DINV-1): %v", leg, call)
		}
	}
}

func TestCheckDefectInjectionMatrix(t *testing.T) {
	requireHarness(t)
	r := buildFixtureRelease(t)

	// ---- subprocess-driven: no transport injection needed ------------------

	t.Run("toolchain absent (git missing)", func(t *testing.T) {
		repo := t.TempDir()
		env := toolchainProbeEnv(t, repo, "")
		before := snap(t, repo)
		res := pyxis(t, repo, env, initArgs(r)...)
		assertCheckRefusal(t, repo, before, res, preflight.ToolchainID, "git: not found on PATH")
	})

	t.Run("toolchain below floor (git too old)", func(t *testing.T) {
		repo := t.TempDir()
		env := toolchainProbeEnv(t, repo, "0.0.1")
		before := snap(t, repo)
		res := pyxis(t, repo, env, initArgs(r)...)
		assertCheckRefusal(t, repo, before, res, preflight.ToolchainID, "is below the required")
	})

	t.Run("artifact source unreachable (cold cache)", func(t *testing.T) {
		repo := t.TempDir()
		env := pyxisEnv(t, repo)
		before := snap(t, repo)
		dead := "file://" + filepath.Join(t.TempDir(), "no-such-release")
		res := pyxis(t, repo, env, initArgs(r, "--artifact-source", dead)...)
		assertCheckRefusal(t, repo, before, res, preflight.ArtifactSourceID, "check network access to")
	})

	// ---- in-process: the gh transport must be injected (hermetic) ----------

	t.Run("github-mode logged-out gh", func(t *testing.T) {
		target := t.TempDir()
		before := snap(t, target)
		runner := &ghTranscript{authFails: true}
		ans := ghDefectAnswers("octo/widget")
		_, logJSON, err := runInitChecks(t, r, target, ans, runner)
		assertInProcessRefusal(t, err, logJSON, preflight.GitHubModeID, "gh auth login")
		sameSnap(t, before, snap(t, target), "gh-logged-out")
		assertNoMutatingGH(t, runner, "gh-logged-out")
	})

	t.Run("github-mode missing repo under use", func(t *testing.T) {
		target := t.TempDir()
		before := snap(t, target)
		runner := &ghTranscript{repoExists: false}
		ans := ghDefectAnswers("octo/widget")
		ans.WiringRepo = "use"
		_, logJSON, err := runInitChecks(t, r, target, ans, runner)
		assertInProcessRefusal(t, err, logJSON, preflight.GitHubModeID, "wiring.repo=create")
		sameSnap(t, before, snap(t, target), "missing-repo-under-use")
		assertNoMutatingGH(t, runner, "missing-repo-under-use")
	})

	t.Run("github-mode coordinate missing promptless", func(t *testing.T) {
		target := t.TempDir()
		before := snap(t, target)
		runner := &ghTranscript{}
		ans := ghDefectAnswers("") // neither a remote nor a promptless coordinate
		_, logJSON, err := runInitChecks(t, r, target, ans, runner)
		assertInProcessRefusal(t, err, logJSON, preflight.GitHubModeID, "--coordinate")
		sameSnap(t, before, snap(t, target), "coordinate-missing")
		assertNoMutatingGH(t, runner, "coordinate-missing")
	})

	t.Run("cross-knob coherence wedged pair", func(t *testing.T) {
		target := t.TempDir()
		before := snap(t, target)
		runner := &ghTranscript{repoExists: true}
		ans := ghDefectAnswers("octo/widget")
		ans.WiringRepo = "use"
		ans.WiringProtection = "required-checks" // wedged against the default ci_mode=local
		_, logJSON, err := runInitChecks(t, r, target, ans, runner)
		assertInProcessRefusal(t, err, logJSON, preflight.CrossKnobCoherenceID, "wiring.protection to none")
		sameSnap(t, before, snap(t, target), "wedged-knob-pair")
		assertNoMutatingGH(t, runner, "wedged-knob-pair")
	})
}

// ---- shared fixtures ---------------------------------------------------

// ghDefectAnswers is the base promptless Answers every in-process gh-mode
// defect case starts from: github tracker (so a bare coordinate resolves
// meaningfully), local ci_mode (never itself wedged against the default
// protection), no CLI surfaces (PATH-independent), a fixed identity (UB-10
// must resolve before the checks under test ever run), and the given
// coordinate ("" reproduces the promptless "neither remote nor coordinate"
// defect).
func ghDefectAnswers(coordinate string) installer.Answers {
	argus := false
	return installer.Answers{
		Tracker:          "github",
		CIMode:           "local",
		WiringRepo:       "use",
		WiringProtection: "none",
		CLISurfaces:      []string{},
		Argus:            &argus,
		GitName:          "E2E Operator",
		GitEmail:         "e2e@example.com",
		Coordinate:       coordinate,
	}
}

// runInitChecks drives internal/installer.Init in-process against a fresh,
// git-less target (admitted as a bare empty directory) with the fixture
// release's pin and a hermetic file:// source — so the front-loaded
// admission/toolchain/artifact-source checks pass cleanly and the ONLY thing
// under test is whichever knob-dependent check the caller's runner/answers
// are shaped to trip. Returns the report, the run's captured JSON log
// (for the EVT-2/EVT-3 event assertions), and the error — never fatals on error,
// since a refusal is the expected outcome for every caller in this file.
func runInitChecks(t *testing.T, r fixtureRelease, target string, ans installer.Answers, runner wire.Runner) (installer.Report, string, error) {
	t.Helper()
	requireHarness(t)
	var logBuf bytes.Buffer
	opts := installer.Options{
		Target:          target,
		Pin:             installer.Pin{Version: r.version, ManifestSHA256: r.pin},
		ArtifactSource:  r.source(),
		Answers:         ans,
		AnswersProvided: true,
		Fetcher:         resolve.FileFetcher{},
		WireRunner:      runner,
		PublicationGit:  localPublicationTransport(t),
		Stdout:          discardWriter{},
		Logger:          slog.New(slog.NewJSONHandler(&logBuf, nil)),
	}
	rep, err := installer.Init(context.Background(), opts)
	return rep, logBuf.String(), err
}

// assertInProcessRefusal asserts an in-process Init call refused on
// wantCheck: the returned error wraps a *preflight.RefusalError naming that
// check, wantFragment appears in its reason or remedy, and the init_refused
// event (EVT-2) fired with typed check/remedy_present/route_present
// properties — none of the knob-dependent checks route anywhere, so
// route_present is always false here (only admission ever names a route).
func assertInProcessRefusal(t *testing.T, err error, logJSON, wantCheck, wantFragment string) {
	t.Helper()
	if err == nil {
		t.Fatal("Init = nil error, want a refusal")
	}
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("Init err = %v, want it to wrap a *preflight.RefusalError", err)
	}
	if refusal.Outcome.Check != wantCheck {
		t.Errorf("refused check = %q, want %q", refusal.Outcome.Check, wantCheck)
	}
	if !strings.Contains(refusal.Outcome.Reason, wantFragment) && !strings.Contains(refusal.Outcome.Remedy, wantFragment) {
		t.Errorf("refusal reason/remedy = %q / %q, want one to contain %q", refusal.Outcome.Reason, refusal.Outcome.Remedy, wantFragment)
	}

	evt := findLogEvent(t, logJSON, "init_refused")
	if evt == nil {
		t.Fatalf("no init_refused event in the captured log:\n%s", logJSON)
	}
	if evt["check"] != wantCheck {
		t.Errorf("init_refused check = %v, want %q", evt["check"], wantCheck)
	}
	if evt["remedy_present"] != true {
		t.Errorf("init_refused remedy_present = %v, want true", evt["remedy_present"])
	}
	if evt["route_present"] != false {
		t.Errorf("init_refused route_present = %v, want false (only admission ever names a route)", evt["route_present"])
	}
}

// findLogEvent scans a captured JSON-log buffer (one record per line, slog's
// JSONHandler shape) for the first record whose msg equals name, returning
// its fields — nil when absent. The in-process legs' event assertion; the
// subprocess legs instead grep the text-handler stderr directly (cmd/pyxis
// wires a TextHandler for a human terminal).
func findLogEvent(t *testing.T, logJSON, name string) map[string]any {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(logJSON), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["msg"] == name {
			return rec
		}
	}
	return nil
}

// assertCheckRefusal asserts a subprocess `pyxis init` refused pre-write on
// the named check: exit 1, the init_refused (EVT-2) text-log line naming
// that check with a remedy present and no route, wantFragment present in the
// human remedy prose on stderr, and — DINV-1 — a byte-identical target
// (snapshotted before the run).
func assertCheckRefusal(t *testing.T, repo string, before map[string]string, res runResult, wantCheck, wantFragment string) {
	t.Helper()
	if res.code != 1 {
		t.Fatalf("pyxis init = exit %d, want 1 (a named pre-write refusal)\nstdout: %s\nstderr: %s", res.code, res.stdout, res.stderr)
	}
	for _, want := range []string{"msg=init_refused", "check=" + wantCheck, "remedy_present=true", "route_present=false", wantFragment} {
		if !strings.Contains(res.stderr, want) {
			t.Errorf("refusal stderr does not contain %q:\n%s", want, res.stderr)
		}
	}
	sameSnap(t, before, snap(t, repo), "check-defect "+wantCheck)
	assertCLILogEvent(t, repo, "init", "init_refused")
}

// ---- toolchain PATH fixtures --------------------------------------------

// toolchainProbeEnv builds the same isolated subprocess env pyxisEnv does
// (driver_test.go), except Git is omitted from PATH entirely
// (fakeVersion == "") or shadowed by a tiny shim script reporting
// fakeVersion — the toolchain check's absence and below-floor fixtures
// (A-2), driven through the real `pyxis` binary so the refusal and its EVT-2
// event are observed exactly as a user would see them, not merely at the
// library level (internal/preflight's own toolchain_test.go already covers
// that). A near-duplicate of pyxisEnv's construction rather than a
// parameterization of it: pyxisEnv is shared by every other leg in this
// package and this is the only caller that needs to withhold a tool.
func toolchainProbeEnv(t *testing.T, repo, fakeVersion string) []string {
	t.Helper()
	home := t.TempDir()
	gc := filepath.Join(home, "gitconfig")
	writeFile(t, gc, []byte("[user]\n\tname = E2E Operator\n\temail = e2e@example.com\n"), 0o644)

	pathDir := t.TempDir()
	for _, name := range []string{"go", "bash", "sh"} {
		if p, err := exec.LookPath(name); err == nil {
			_ = os.Symlink(p, filepath.Join(pathDir, name))
		}
	}
	if fakeVersion != "" {
		writeFile(t, filepath.Join(pathDir, "git"), []byte("#!/bin/sh\necho 'git version "+fakeVersion+"'\n"), 0o755)
	}
	pathVal := filepath.Join(repo, ".pyxis", "bin") + string(os.PathListSeparator) + pathDir

	env := []string{
		"HOME=" + home,
		"GIT_CONFIG_GLOBAL=" + gc,
		"GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
		"PATH=" + pathVal,
	}
	for _, k := range []string{"GOCACHE", "GOPATH", "GOMODCACHE", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}
