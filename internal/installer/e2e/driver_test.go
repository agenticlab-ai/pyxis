package e2e

// The subprocess driver: every leg drives the REAL built `pyxis` binary as a
// child process against a scratch repo, exactly as a shipped install would be
// invoked from a shell. This is the deployment context AP-14 demands — no
// in-process seam on the operand path (the offline egress scan's in-process
// leg is the one deliberate exception, and it exercises the library layer, not
// the packaged binary). Git identity is isolated so the developer's real
// identity never leaks into a scratch commit, and the commit dates are pinned
// so two entry states can be compared byte-for-byte.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runResult is one `pyxis` subprocess invocation's outcome.
type runResult struct {
	code   int
	stdout string
	stderr string
}

// pyxisEnv builds the isolated environment every scratch-repo subprocess runs
// under: a HOME with no gitconfig, a neutralized global/system git config
// carrying ONLY a written identity (so commits succeed with a deterministic
// author), pinned commit dates (so entry-state comparisons are byte-stable),
// and .pyxis/bin prepended to PATH so the operand hooks find the pinned
// `pyxis` binary. Offline legs add their own proxy settings to this environment.
//
// GIT_DIR is deliberately NOT pinned here: init runs `git init` inside the
// scratch repo, so GIT_DIR must stay unset for git to target the scratch tree.
// Every leg runs its verbs inside a scratch repo that carries its own
// .pyxis/config.yaml, so pyxis's repoRoot resolves via the config walk and
// never reaches the `git rev-parse --show-toplevel` fallback that could escape
// into an ancestor repo — no bogus GIT_DIR is needed on the operand path.
func pyxisEnv(t *testing.T, repo string) []string {
	t.Helper()
	home := t.TempDir()
	gc := filepath.Join(home, "gitconfig")
	writeFile(t, gc, []byte("[user]\n\tname = E2E Operator\n\temail = e2e@example.com\n"), 0o644)

	// Deliberately omit Node and npm: every real-binary install and doctor
	// leg must succeed without a package runtime. The operand cache
	// comes first so hooks resolve the pinned pyxis binary.
	pathDir := t.TempDir()
	for _, tool := range []string{"git", "go", "bash", "sh", "bwrap"} {
		if p, err := exec.LookPath(tool); err == nil {
			_ = os.Symlink(p, filepath.Join(pathDir, tool))
		}
	}
	if gitExec, err := exec.Command("git", "--exec-path").Output(); err == nil {
		for _, tool := range []string{"git-upload-pack", "git-receive-pack"} {
			if err := os.Symlink(filepath.Join(strings.TrimSpace(string(gitExec)), tool), filepath.Join(pathDir, tool)); err != nil {
				t.Fatal(err)
			}
		}
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
	// GOCACHE/GOPATH must survive for any go invocation the hooks or source
	// mode might trigger (not on the offline path, but keep the toolchain sane).
	for _, k := range []string{"GOCACHE", "GOPATH", "GOMODCACHE", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// pyxis runs `pyxis <args…>` in repo under the isolated env and returns the
// captured result. It never fatals on a nonzero exit — the caller asserts the
// code — so refusal legs can inspect the named stderr.
func pyxis(t *testing.T, repo string, env []string, args ...string) runResult {
	t.Helper()
	requireHarness(t)
	cmd := exec.Command(builtBins["pyxis"], args...)
	cmd.Dir = repo
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if asExit(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("pyxis %s: launch failed: %v\n%s", strings.Join(args, " "), err, stderr.String())
		}
	}
	return runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// asExit is a tiny errors.As for *exec.ExitError (avoids importing errors just
// for the one call across the driver).
func asExit(err error, target **exec.ExitError) bool {
	for err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			*target = ee
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// initArgs is the promptless carina+local install argv every scratch repo
// starts from: fully flag-driven (zero prompts), file:// artifact source, no
// GitHub wiring (carina tracker), no CLI surfaces wired (PATH-independent).
func initArgs(r fixtureRelease, extra ...string) []string {
	base := []string{
		// --verbose: the suite asserts the EVT slog events inline on
		// stderr; without it they land only in the run journal (#898).
		"init", "--yes", "--verbose",
		"--system-version", r.version,
		"--manifest-sha256", r.pin,
		"--artifact-source", r.source(),
		"--tracker", "carina",
		"--ci-mode", "local",
		"--wiring", "repo=use,protection=none",
		"--cli-surfaces", "",
	}
	// Every install has a configured publication destination. This fixture
	// default is a real local bare repository; explicit destinations win.
	configured := false
	for _, arg := range extra {
		if arg == "--remote-url" || arg == "--integrations" {
			configured = true
		}
	}
	if !configured {
		bare, err := os.MkdirTemp(r.src, "initial-git-*")
		if err != nil {
			panic(err)
		}
		if out, err := exec.Command("git", "init", "--bare", "--initial-branch=main", bare).CombinedOutput(); err != nil {
			panic(string(out) + err.Error())
		}
		base = append(base, "--remote-url", bare)
	}
	return append(base, extra...)
}

// scratchRepo creates an empty directory and drives the built `pyxis init`
// (carina+local, promptless) into it against r, asserting it ends green. It
// returns the converged repo directory and the env the subprocess ran under
// (re-used by follow-up verb invocations so the same HOME/PATH/identity holds).
// The converged tree is the starting point for the drift, kill, fresh-clone,
// misparse, and removal legs. extra folds over initArgs' own base flags
// (flag.Parse takes the last occurrence) — e.g. a leg that needs the hosted
// workflow present passes "--ci-mode", "both" to override the local-mode
// baseline.
func scratchRepo(t *testing.T, r fixtureRelease, extra ...string) (repo string, env []string) {
	t.Helper()
	repo = t.TempDir()
	env = pyxisEnv(t, repo)
	res := pyxis(t, repo, env, initArgs(r, extra...)...)
	if res.code != 0 {
		t.Fatalf("scratch install did not end green: exit %d\nstdout:\n%s\nstderr:\n%s", res.code, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "converged main") {
		t.Fatalf("scratch install summary lacks the converged line:\n%s", res.stdout)
	}
	// Sanity: the governance tree seeded, the config parses, AND the binary
	// cache warmed — converge's pre-warm fetch-verified the pinned pyxis +
	// docgate binaries + the manifest into .pyxis/bin (the offline legs then
	// need zero fetch, and the packaged exec path has a real binary to run).
	for _, want := range []string{
		".pyxis/config.yaml", ".gitignore", "AGENTS.md",
		"docs/product-vision.md", "docs/observability/incidents.md",
		"docs/tech-design/index.json",
		".githooks/pre-commit", ".githooks/pre-push",
		".pyxis/bin/release-manifest-" + r.version + ".json",
		".pyxis/bin/pyxis-" + r.version + "-" + r.plat,
		".pyxis/bin/docgate-" + r.version + "-" + r.plat,
	} {
		if _, err := os.Stat(at(repo, want)); err != nil {
			t.Fatalf("scratch install missing seeded/cached artifact %s: %v", want, err)
		}
	}
	loadConfig(t, repo) // must closed-parse
	return repo, env
}

// killableConverge starts `pyxis converge` as a child in repo and returns the
// running *exec.Cmd so a leg can interrupt it mid-flight (SIGKILL) then re-run.
func killableConverge(t *testing.T, repo string, env []string) *exec.Cmd {
	t.Helper()
	requireHarness(t)
	cmd := exec.Command(builtBins["pyxis"], "converge")
	cmd.Dir = repo
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = discardWriter{}, discardWriter{}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start converge: %v", err)
	}
	return cmd
}
