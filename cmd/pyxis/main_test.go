package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adopt"
	"github.com/agenticlab-ai/pyxis/internal/converge"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
	"github.com/agenticlab-ai/pyxis/internal/termstyle"
	"github.com/agenticlab-ai/pyxis/internal/upgrade"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// ---- seam scaffolding ------------------------------------------------------

// swap replaces a package-var seam for one test, restoring it on cleanup.
func swap[T any](t *testing.T, target *T, v T) {
	t.Helper()
	old := *target
	*target = v
	t.Cleanup(func() { *target = old })
}

// resolveRec is the counting acquisition fake: it records the call and
// returns a canned result, so tests can prove BOTH that refusal paths never
// resolve (I3) and that the happy path maps config → resolve.Config right.
type resolveRec struct {
	calls    int
	cfg      resolve.Config
	tool     string
	platform string
	path     string
	err      error
}

func (r *resolveRec) call(_ context.Context, cfg resolve.Config, tool, platform string) (string, error) {
	r.calls++
	r.cfg, r.tool, r.platform = cfg, tool, platform
	return r.path, r.err
}

// execRec captures the exec seam: the exact path/argv/env handed to execve.
type execRec struct {
	calls int
	path  string
	argv  []string
	env   []string
	code  int
	err   error
}

func (e *execRec) call(path string, argv, env []string) (int, error) {
	e.calls++
	e.path, e.argv, e.env = path, slices.Clone(argv), slices.Clone(env)
	return e.code, e.err
}

// goRunRec captures the source-mode seam.
type goRunRec struct {
	calls int
	sm    *resolve.SourceMode
	argv  []string
	code  int
	err   error
}

func (g *goRunRec) call(sm *resolve.SourceMode, argv []string) (int, error) {
	g.calls++
	g.sm, g.argv = sm, slices.Clone(argv)
	return g.code, g.err
}

// convergeRec captures the converge seam: the exact cfg/opts handed to the
// engine, plus a canned report/error.
type convergeRec struct {
	calls int
	cfg   resolve.Config
	opts  converge.Options
	rep   converge.Report
	err   error
}

func (c *convergeRec) call(_ context.Context, cfg resolve.Config, opts converge.Options) (converge.Report, error) {
	c.calls++
	c.cfg, c.opts = cfg, opts
	return c.rep, c.err
}

// doctorRec captures the doctor seam: the exact cfg handed to the library,
// plus a canned Health/error.
type doctorRec struct {
	calls int
	cfg   resolve.Config
	h     converge.Health
	err   error
}

func (d *doctorRec) call(_ context.Context, cfg resolve.Config) (converge.Health, error) {
	d.calls++
	d.cfg = cfg
	return d.h, d.err
}

// denySeams pins every consequential seam to a failing recorder: a refusal
// path that reaches resolution, execution, or the converge engine is the I3
// fail-open itself.
func denySeams(t *testing.T) {
	t.Helper()
	// Isolate the init pin sources: a machine (or CI) that ran install.sh
	// records $XDG_CONFIG_HOME/pyxis/release.env, and PYXIS_* may be exported —
	// a refusal-path test must see none of them, or init would resolve a pin and
	// proceed past the refusal it asserts.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PYXIS_SYSTEM_VERSION", "")
	t.Setenv("PYXIS_MANIFEST_SHA256", "")
	swap(t, &resolveTool, func(context.Context, resolve.Config, string, string) (string, error) {
		t.Error("resolveTool called on a refusal path (I3: refuse before any action)")
		return "", errors.New("denied")
	})
	swap(t, &execFn, func(string, []string, []string) (int, error) {
		t.Error("execFn called on a refusal path (zero execution)")
		return 0, errors.New("denied")
	})
	swap(t, &goRunFn, func(*resolve.SourceMode, []string) (int, error) {
		t.Error("goRunFn called on a refusal path (zero execution)")
		return 0, errors.New("denied")
	})
	swap(t, &convergeRun, func(context.Context, resolve.Config, converge.Options) (converge.Report, error) {
		t.Error("convergeRun called on a refusal path (I3: refuse before any action)")
		return converge.Report{}, errors.New("denied")
	})
	swap(t, &doctorRun, func(context.Context, resolve.Config) (converge.Health, error) {
		t.Error("doctorRun called on a refusal path (I3: refuse before any action)")
		return converge.Health{}, errors.New("denied")
	})
	swap(t, &installerInit, func(context.Context, installer.Options) (installer.Report, error) {
		t.Error("installerInit called on a refusal path (I3: refuse before any action)")
		return installer.Report{}, errors.New("denied")
	})
	swap(t, &adoptInspect, func(context.Context, adopt.Options) (adopt.Report, error) {
		t.Error("adoptInspect called on a refusal path (I3: refuse before any action)")
		return adopt.Report{}, errors.New("denied")
	})
}

// ---- fixture scaffolding ---------------------------------------------------

func hexOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// writeOperandConfig writes a clean pyxis-config/v2 release-mode config —
// the T2 surface exec loads and maps into resolve.Config.
func writeOperandConfig(t *testing.T, root, ver, pin, artifactSource string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "schema_version: pyxis-config/v2\nsystem:\n  version: %q\n  manifest_sha256: %q\n", ver, pin)
	if artifactSource != "" {
		fmt.Fprintf(&b, "  artifact_source: %q\n", artifactSource)
	}
	b.WriteString("  mode: release\n")
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte(b.String()))
}

// tempRepo creates an operand root carrying a valid v2 release-mode config,
// chdirs into it, and pins GIT_DIR to a bogus path so the git root fallback
// can never escape the fixture into the developer's real repo.
func tempRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeOperandConfig(t, root, "v0.2.0", strings.Repeat("ab", 32), "")
	t.Chdir(root)
	t.Setenv("GIT_DIR", filepath.Join(root, "no-such-git"))
	return root
}

// snapshot lists every path (+ file size) under root — the zero-writes probe
// refusal tests diff.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		if d.IsDir() {
			out = append(out, rel+"/")
			return nil
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out = append(out, fmt.Sprintf("%s %d", rel, fi.Size()))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// samePath compares two paths through EvalSymlinks (macOS tempdirs live
// under the /var → /private/var symlink).
func samePath(t *testing.T, a, b string) bool {
	t.Helper()
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", a, err)
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", b, err)
	}
	return ra == rb
}

// ---- the guard: misparse refuses pre-action on every verb (UB-9, A-7) ------

func TestMisparseRefusesPreActionOnEveryVerb(t *testing.T) {
	root := tempRepo(t)
	denySeams(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no verb", nil},
		{"unknown verb", []string{"bogus"}},
		// init carries a knob-flag surface plus the required pin pair; an
		// unknown flag, a positional (the target is cwd), a flag-like
		// positional, or the missing pin pair all refuse pre-action.
		{"init unknown flag", []string{"init", "--bogus", "--system-version", "v1", "--manifest-sha256", "ab"}},
		{"init stray positional", []string{"init", "extra"}},
		{"init flag-like positional", []string{"init", "-x"}},
		{"init missing pin pair", []string{"init", "--tracker", "github"}},
		{"init missing manifest sha", []string{"init", "--system-version", "v1.0.0"}},
		// The converge verbs carry exactly --repair/--preview (the shape's
		// grammar); anything else refuses pre-action.
		{"converge unknown flag", []string{"converge", "--force"}},
		{"converge stray positional", []string{"converge", "extra"}},
		{"converge flag after positional", []string{"converge", "extra", "--preview"}},
		{"converge malformed flag value", []string{"converge", "--preview=maybe"}},
		{"repair unknown flag", []string{"repair", "--force"}},
		{"repair does not take --repair", []string{"repair", "--repair"}},
		{"repair stray positional", []string{"repair", "extra"}},
		// doctor is flagless by the shape's grammar (`pyxis doctor`): any
		// flag or positional refuses pre-action.
		{"doctor unknown flag", []string{"doctor", "--json"}},
		{"doctor stray positional", []string{"doctor", "extra"}},
		{"doctor flag-like positional", []string{"doctor", "-v"}},
		// adopt takes no positionals — the target is always the working
		// directory (T3); an unknown flag or any positional refuses
		// pre-action.
		{"adopt unknown flag", []string{"adopt", "--bogus"}},
		{"adopt stray positional", []string{"adopt", "extra"}},
		{"adopt flag-like positional", []string{"adopt", "-x"}},
		{"version unknown flag", []string{"version", "--verbose"}},
		{"version stray positional", []string{"version", "extra"}},
		{"exec missing separator", []string{"exec", "docgate", "gate"}},
		{"exec bare", []string{"exec"}},
		{"exec no tool", []string{"exec", "--", "gate"}},
		{"exec two tools", []string{"exec", "docgate", "codegate", "--"}},
		{"exec flag-like in the tool slot", []string{"exec", "-docgate", "--"}},
		{"exec unknown flag", []string{"exec", "--tool", "docgate", "--", "gate"}},
		{"exec flag after the tool", []string{"exec", "docgate", "-v", "--", "gate"}},
		{"exec tool shape refused", []string{"exec", "DocGate", "--"}},
		{"exec empty tool", []string{"exec", "", "--"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != 2 {
				t.Fatalf("run(%q) = %d, want 2 (usage refusal before any action)", tt.args, got)
			}
			if stderr.Len() == 0 {
				t.Fatalf("run(%q) refused silently — the refusal must be named", tt.args)
			}
			if after := snapshot(t, root); !slices.Equal(before, after) {
				t.Fatalf("run(%q) mutated the tree on a refusal path:\nbefore %q\nafter  %q", tt.args, before, after)
			}
		})
	}
}

func TestHelpPrintsUsage(t *testing.T) {
	denySeams(t)
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}, {"version", "-h"}} {
		var stdout, stderr bytes.Buffer
		if got := run(args, &stdout, &stderr); got != 2 {
			t.Fatalf("run(%q) = %d, want 2 (the usage convention)", args, got)
		}
		if !strings.Contains(stderr.String(), "usage: pyxis") || !strings.Contains(stderr.String(), "argv-verbatim") {
			t.Fatalf("run(%q) usage output missing the grammar statement:\n%s", args, stderr.String())
		}
	}
}

// ---- init: the required pin pair refuses with a remedy, action-free ---------

func TestInitWithoutPinPairRefusesNamed(t *testing.T) {
	root := tempRepo(t)
	denySeams(t)
	before := snapshot(t, root)
	var stdout, stderr bytes.Buffer
	// A bare `init` carries no pin pair (DINV-2): a named usage refusal before
	// any probe or write — never a silent no-op, and the engine is never
	// reached (denySeams would fail the test if installerInit ran).
	if got := run([]string{"init"}, &stdout, &stderr); got != 2 {
		t.Fatalf("run(init) = %d, want 2 (usage refusal for the missing pin pair)", got)
	}
	for _, want := range []string{"--system-version", "--manifest-sha256"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("run(init) refusal lacks %q:\n%s", want, stderr.String())
		}
	}
	if after := snapshot(t, root); !slices.Equal(before, after) {
		t.Fatalf("run(init) mutated the tree on a refusal path")
	}
}

// initRec captures the init seam: the exact Options handed to the engine.
type initRec struct {
	calls int
	opts  installer.Options
	rep   installer.Report
	err   error
}

func (r *initRec) call(_ context.Context, opts installer.Options) (installer.Report, error) {
	r.calls++
	r.opts = opts
	return r.rep, r.err
}

// TestInitResolvesPinFromEnvAndFile proves the pin hand-off (Piece #2): with no
// --system-version/--manifest-sha256 flags, init resolves the pin from the
// PYXIS_* env vars, else the release.env install.sh records; a flag overrides
// both. So `pyxis init` needs no pin flags on a machine set up by the one-liner.
func TestInitResolvesPinFromFlagAndFile(t *testing.T) {
	sha := strings.Repeat("ab", 32) // 64 hex
	runInit := func(t *testing.T, args ...string) installer.Pin {
		t.Helper()
		root := t.TempDir()
		t.Chdir(root)
		rec := &initRec{rep: installer.Report{Branch: "main"}}
		swap(t, &installerInit, rec.call)
		var out, errb bytes.Buffer
		if got := run(append([]string{"init", "--yes"}, args...), &out, &errb); got != 0 {
			t.Fatalf("run(init %v) = %d, want 0 (%s)", args, got, errb.String())
		}
		if rec.calls != 1 {
			t.Fatalf("installerInit calls = %d, want 1", rec.calls)
		}
		return rec.opts.Pin
	}

	// A pin in the environment decides nothing: a setting that does not
	// survive a restart is not where a setting lives. With no record and no
	// flag there is no pin, and init refuses rather than inventing one.
	t.Run("the environment does not supply the pin", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no release.env
		t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
		t.Setenv("PYXIS_MANIFEST_SHA256", sha)
		var stdout, stderr bytes.Buffer
		if code := initVerb(nil, &stdout, &stderr); code == 0 {
			t.Fatal("init accepted a pin from the environment")
		}
		if !strings.Contains(stderr.String(), "system pin pair is required") {
			t.Errorf("refusal does not name the missing pin: %s", stderr.String())
		}
		if strings.Contains(stderr.String(), "PYXIS_SYSTEM_VERSION") {
			t.Errorf("refusal still offers the environment as a pin source: %s", stderr.String())
		}
	})

	t.Run("from release.env file", func(t *testing.T) {
		cfg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfg)
		t.Setenv("PYXIS_SYSTEM_VERSION", "")
		t.Setenv("PYXIS_MANIFEST_SHA256", "")
		writeFile(t, filepath.Join(cfg, "pyxis", "release.env"),
			[]byte("PYXIS_SYSTEM_VERSION=v0.9.0\nPYXIS_MANIFEST_SHA256="+sha+"\n"))
		if pin := runInit(t); pin.Version != "v0.9.0" || pin.ManifestSHA256 != sha {
			t.Fatalf("pin = %+v, want v0.9.0/%s from file", pin, sha)
		}
	})

	t.Run("a flag overrides the record", func(t *testing.T) {
		cfg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfg)
		t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
		t.Setenv("PYXIS_MANIFEST_SHA256", sha)
		writeFile(t, filepath.Join(cfg, "pyxis", "release.env"),
			[]byte("PYXIS_SYSTEM_VERSION=v0.9.0\n"))
		flagSHA := "cd" + strings.Repeat("cd", 31)
		if pin := runInit(t, "--system-version", "v1.2.3", "--manifest-sha256", flagSHA); pin.Version != "v1.2.3" || pin.ManifestSHA256 != flagSHA {
			t.Fatalf("pin = %+v, want the flag values v1.2.3/%s", pin, flagSHA)
		}
	})
}

// TestInitVerbMapsFlagsAndAnswersFile proves the flag/answers-file fold: flags
// win over the file, AnswersProvided is set, the pin pair and artifact-source
// map through, the target is the working directory, and a promptless run
// leaves Stdin nil.
func TestInitVerbMapsFlagsAndAnswersFile(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	// An answers file the flags then override (tracker: carina → github).
	answers := filepath.Join(root, "answers.yaml")
	writeFile(t, answers, []byte("tracker: carina\nci_mode: local\ngit_name: \"File Name\"\ngit_email: \"file@example.com\"\n"))

	rec := &initRec{rep: installer.Report{
		RunID: "abc123", Branch: "main", GitInited: true,
		Knobs:        config.Knobs{Tracker: "github", CIMode: "both", Wiring: config.Wiring{Repo: "use", Protection: "required-checks"}},
		Identity:     installer.Identity{Name: "File Name", Email: "file@example.com", NameSource: "answers", EmailSource: "answers"},
		Commits:      []installer.Commit{{SHA: "deadbeefcafe0000", Subject: "pyxis init — converged tree"}, {SHA: "feed0000", Subject: "pyxis init — epoch"}},
		EpochWritten: true,
		Stages:       []string{"converge", "commit-tree", "epoch"},
	}}
	swap(t, &installerInit, rec.call)

	var stdout, stderr bytes.Buffer
	args := []string{"init",
		"--answers", answers,
		"--tracker", "github", // wins over the file's carina
		"--wiring", "repo=use,protection=required-checks",
		"--cli-surfaces", "auto",
		"--system-version", "v0.4.0",
		"--manifest-sha256", strings.Repeat("ab", 32),
		"--artifact-source", "file:///mirror",
	}
	if got := run(args, &stdout, &stderr); got != 0 {
		t.Fatalf("run(init …) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if rec.calls != 1 {
		t.Fatalf("installerInit calls = %d, want exactly 1", rec.calls)
	}
	o := rec.opts
	if !samePath(t, o.Target, root) {
		t.Errorf("Options.Target = %q, want the working directory %q", o.Target, root)
	}
	if o.Pin.Version != "v0.4.0" || o.Pin.ManifestSHA256 != strings.Repeat("ab", 32) {
		t.Errorf("pin = %+v, want the flag pair", o.Pin)
	}
	if o.ArtifactSource != "file:///mirror" {
		t.Errorf("ArtifactSource = %q, want the flag override", o.ArtifactSource)
	}
	if !o.AnswersProvided {
		t.Error("AnswersProvided = false, want true (flags/answers input arrived)")
	}
	if o.Stdin != nil {
		t.Error("Stdin != nil on a promptless run (any input means no prompts)")
	}
	// The flag won over the file; the file's identity halves survived.
	if o.Answers.Tracker != "github" {
		t.Errorf("Answers.Tracker = %q, want github (the flag wins over the file's carina)", o.Answers.Tracker)
	}
	if o.Answers.CIMode != "local" {
		t.Errorf("Answers.CIMode = %q, want local (from the file, no overriding flag)", o.Answers.CIMode)
	}
	if o.Answers.GitName != "File Name" || o.Answers.GitEmail != "file@example.com" {
		t.Errorf("identity halves = (%q,%q), want the file's", o.Answers.GitName, o.Answers.GitEmail)
	}
	if o.Answers.WiringRepo != "use" || o.Answers.WiringProtection != "required-checks" {
		t.Errorf("wiring from --wiring wrong: %+v", o.Answers)
	}
	// The report printed a human summary.
	out := stdout.String()
	for _, want := range []string{"run abc123", "commit deadbeefcafe — pyxis init — converged tree", "epoch written", "converged main"} {
		if !strings.Contains(out, want) {
			t.Errorf("init summary lacks %q:\n%s", want, out)
		}
	}
}

// TestInitVerbYesIsPromptless proves --yes maps to AcceptDefaults with no
// answers input and no Stdin (the fully-defaulted promptless run).
func TestInitVerbYesIsPromptless(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &initRec{rep: installer.Report{RunID: "yes1", Branch: "main"}}
	swap(t, &installerInit, rec.call)
	var stdout, stderr bytes.Buffer
	args := []string{"init", "--yes", "--system-version", "v0.4.0", "--manifest-sha256", strings.Repeat("cd", 32)}
	if got := run(args, &stdout, &stderr); got != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if !rec.opts.AcceptDefaults {
		t.Error("AcceptDefaults = false, want true (--yes)")
	}
	if rec.opts.AnswersProvided {
		t.Error("AnswersProvided = true with only --yes (no knob input arrived)")
	}
	if rec.opts.Stdin != nil {
		t.Error("Stdin != nil under --yes (promptless)")
	}
}

// TestInitVerbBadWiringFlagRefuses proves a malformed --wiring value refuses
// pre-action (exit 2), before the engine.
func TestInitVerbBadWiringFlagRefuses(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	denySeams(t)
	var stdout, stderr bytes.Buffer
	args := []string{"init", "--wiring", "repo", "--system-version", "v1", "--manifest-sha256", "ab"}
	if got := run(args, &stdout, &stderr); got != 2 {
		t.Fatalf("run = %d, want 2 (malformed --wiring)", got)
	}
	if !strings.Contains(stderr.String(), "wiring") {
		t.Errorf("refusal not named: %s", stderr.String())
	}
}

// TestInitVerbRelaysEngineRefusalAsExitOne proves an engine refusal (e.g.
// identity/admission) is relayed as a named exit-1.
func TestInitVerbRelaysEngineRefusalAsExitOne(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &initRec{err: errors.New("init: git user.name is unset at every scope (UB-10)")}
	swap(t, &installerInit, rec.call)
	var stdout, stderr bytes.Buffer
	args := []string{"init", "--yes", "--system-version", "v0.4.0", "--manifest-sha256", strings.Repeat("ab", 32)}
	if got := run(args, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (engine refusal)", got)
	}
	if !strings.Contains(stderr.String(), "UB-10") {
		t.Errorf("refusal not relayed on stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a failed init still printed a summary:\n%s", stdout.String())
	}
}

// TestInitVerbPrintsRollbackReportOnFailure proves a stage failure past the
// point mutation began (plan init-hardening/001 T4, FR-10) surfaces the
// write-journal rollback's account on stderr — including the manual command
// to remove a repository created remotely, which DINV-4 never runs
// automatically, so a user would otherwise never learn it exists.
func TestInitVerbPrintsRollbackReportOnFailure(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &initRec{
		err: errors.New("init: stage green-proof: injected green-proof-stage failure (test)"),
		rep: installer.Report{Rollback: &installer.RollbackReport{
			FailedStage:      "green-proof",
			Restored:         true,
			RepoRemoved:      true,
			RemovedFiles:     []string{".pyxis/config.yaml", ".gitignore"},
			ForeignPreserved: []string{"sub/planted-foreign.txt"},
			RemoteCleanup:    []string{"gh repo delete acme/widgets --yes"},
		}},
	}
	swap(t, &installerInit, rec.call)
	var stdout, stderr bytes.Buffer
	args := []string{"init", "--yes", "--system-version", "v0.4.0", "--manifest-sha256", strings.Repeat("ab", 32)}
	if got := run(args, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (engine refusal)", got)
	}
	out := stderr.String()
	for _, want := range []string{
		"green-proof",
		"repository init created this run was removed",
		"sub/planted-foreign.txt",
		"gh repo delete acme/widgets --yes",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rollback report missing %q:\n%s", want, out)
		}
	}
}

// TestInitVerbEndToEndAgainstFixtureRelease runs the REAL init engine (no
// seams) over a file:// fixture release into a bare working directory: the
// tree materializes, two commits land, and a re-run (now a non-empty repo)
// refuses per UB-11.
func TestInitVerbEndToEndAgainstFixtureRelease(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", t.TempDir())

	srcDir := t.TempDir()
	plat := resolve.Platform()
	binBytes := []byte("#!/bin/sh\necho fixture\n")

	// The payload carries the REAL instantiables + installed hook variants the
	// T8 tail seeds from (plan installer/001 T8): a single .pyxis/pipeline.md
	// engine-verbatim file plus the source-repo skeletons/manual/welcome/hooks,
	// so the full init pipeline (front + tail) runs against the genuine
	// instantiables end to end.
	entries := []tarEntry{{name: ".pyxis/pipeline.md", b: []byte("# pipeline map\n")}}
	for _, rel := range []string{
		".pyxis/templates/product-vision-skeleton.md",
		".pyxis/templates/architecture-principles-skeleton.md",
		".pyxis/templates/operand-agents-manual.md",
		".pyxis/templates/harness-implement-now-skill.md",
		".pyxis/templates/welcome-item.md",
		".pyxis/skills/help/SKILL.md",
		".pyxis/skills/implement-now/SKILL.md",
		".pyxis/dist/operand-hooks/pre-commit",
		".pyxis/dist/operand-hooks/pre-push",
		".pyxis/dist/operand-hooks/prepare-commit-msg",
	} {
		entries = append(entries, tarEntry{name: rel, b: readRepoFile(t, rel), exec: strings.HasPrefix(rel, ".pyxis/dist/operand-hooks/")})
	}

	archive := tarGzMany(t, entries)
	var files []relmanifest.PayloadFile
	for _, e := range entries {
		sum := hexOf(e.b)
		files = append(files, relmanifest.PayloadFile{Path: e.name, SHA256: &sum, Class: relmanifest.EngineVerbatim})
	}
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: "v9.9.9",
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {plat: hexOf(binBytes)}},
		Payload: relmanifest.Payload{
			ArchiveSHA256: hexOf(archive),
			Files:         files,
		},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "release-manifest-v9.9.9.json"), mb)
	writeFile(t, filepath.Join(srcDir, "payload-v9.9.9.tar.gz"), archive)
	writeFile(t, filepath.Join(srcDir, "docgate-v9.9.9-"+plat), binBytes)
	fileBytes := entries[0].b

	target := t.TempDir()
	t.Chdir(target)

	var stdout, stderr bytes.Buffer
	// carina tracker: no GitHub wiring runs, so the E2E is fully local (no gh,
	// no network) — the offline combo's tracker. The tail then seeds, installs
	// hooks, wires the local store + welcome item, and the green proof runs the
	// real gate over the seeded tree.
	bare := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("bare origin: %v: %s", err, out)
	}
	args := []string{"init", "--yes", "--remote-url", bare,
		"--system-version", "v9.9.9",
		"--manifest-sha256", hexOf(mb),
		"--artifact-source", "file://" + srcDir,
		"--tracker", "carina",
		"--ci-mode", "local",
		"--wiring", "repo=use,protection=none",
		"--cli-surfaces", "", // explicit empty: no CLI wired (PATH-independent)
	}
	// Identity via the git-config-global we neutralized: supply it repo-wide
	// through a global config so the commit succeeds.
	gc := filepath.Join(t.TempDir(), "gc")
	writeFile(t, gc, []byte("[user]\n\tname = E2E\n\temail = e2e@example.com\n"))
	t.Setenv("GIT_CONFIG_GLOBAL", gc)

	if got := run(args, &stdout, &stderr); got != 0 {
		t.Fatalf("run(init) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "converged main") {
		t.Fatalf("summary = %q, want the converged line", stdout.String())
	}
	// The verbatim payload materialized and the tree is committed.
	if got, rerr := os.ReadFile(filepath.Join(target, ".pyxis", "pipeline.md")); rerr != nil || !bytes.Equal(got, fileBytes) {
		t.Fatalf("verbatim file not materialized byte-exact (err %v)", rerr)
	}
	if _, serr := os.Stat(filepath.Join(target, ".pyxis", "config.yaml")); serr != nil {
		t.Fatalf("config not written: %v", serr)
	}

	// T8 end-to-end through the real verb: the governance tree is seeded, the
	// registries + design index exist, AGENTS.md is substituted, the operand
	// hooks are installed with core.hooksPath, and the carina store carries the
	// welcome item — and Init succeeded, so the green proof ran the real gate
	// and it exited 0 (carried finding (a): lint-rollups vacuous-passed on the
	// rollup-absent tree).
	for _, want := range []string{
		"docs/product-vision.md", "docs/architecture-principles.md", "AGENTS.md",
		"docs/observability/incidents.md", "docs/tech-design/index.md", "docs/tech-design/index.json",
		".githooks/pre-commit", ".githooks/pre-push", ".githooks/prepare-commit-msg",
	} {
		if _, serr := os.Stat(filepath.Join(target, filepath.FromSlash(want))); serr != nil {
			t.Errorf("T8 seeded artifact %s absent after init: %v", want, serr)
		}
	}
	// Rollups and the opt-in living maps stay absent (rule 3 / finding a): only
	// the two governing docs are seeded — everything else is opt-in.
	for _, absent := range []string{"docs/prd.md", "docs/techspec.md", "docs/domain-model.md", "docs/system-design.md"} {
		if _, serr := os.Stat(filepath.Join(target, filepath.FromSlash(absent))); serr == nil {
			t.Errorf("non-seeded path %s exists — must stay absent", absent)
		}
	}
	// The carina store answers a query: its items dir carries the welcome item.
	itemsDir := filepath.Join(target, ".workitems", "items")
	its, _ := os.ReadDir(itemsDir)
	if len(its) == 0 {
		t.Errorf("carina store has no items after init (dir %s)", itemsDir)
	}
	// A re-run in the now-non-empty repo refuses per UB-11 (exit 1), routed
	// to adopt (FR-2) — never a claim that adoption is unavailable.
	stdout.Reset()
	stderr.Reset()
	if got := run(args, &stdout, &stderr); got != 1 {
		t.Fatalf("re-run = %d, want 1 (UB-11: the repo is no longer empty)", got)
	}
	if !strings.Contains(stderr.String(), "pyxis adopt") {
		t.Errorf("re-run refusal does not name the adopt route: %s", stderr.String())
	}
}

// ---- converge/repair: the T5 verb glue ----------------------------------------

func TestConvergeVerbMapsFlagsAndPrintsReport(t *testing.T) {
	root := tempRepo(t)
	cases := []struct {
		name string
		args []string
		want converge.Options
	}{
		{"converge plain", []string{"converge"}, converge.Options{}},
		{"converge --repair", []string{"converge", "--repair"}, converge.Options{Repair: true}},
		{"converge --preview", []string{"converge", "--preview"}, converge.Options{Preview: true}},
		{"converge both flags", []string{"converge", "--repair", "--preview"}, converge.Options{Repair: true, Preview: true}},
		{"repair is converge named repair", []string{"repair"}, converge.Options{Repair: true}},
		{"repair --preview", []string{"repair", "--preview"}, converge.Options{Repair: true, Preview: true}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &convergeRec{rep: converge.Report{
				RunID:    "cafe",
				Mode:     converge.ModeRepair,
				Repaired: []converge.Action{{Path: ".pyxis/templates/prd-template.md", Class: relmanifest.EngineVerbatim}},
				Reaped:   []string{"docgate-v0.1.0-x"},
			}}
			swap(t, &convergeRun, rec.call)
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != 0 {
				t.Fatalf("run(%q) = %d, want 0 (stderr: %s)", tt.args, got, stderr.String())
			}
			if rec.calls != 1 {
				t.Fatalf("convergeRun calls = %d, want exactly 1", rec.calls)
			}
			if rec.opts != tt.want {
				t.Fatalf("opts = %+v, want %+v", rec.opts, tt.want)
			}
			if !samePath(t, rec.cfg.Root, root) {
				t.Fatalf("cfg.Root = %q, want the operand root %q", rec.cfg.Root, root)
			}
			if rec.cfg.Logger == nil {
				t.Fatal("cfg.Logger = nil — the engine's EVT-2 event needs the verb's logger")
			}
			out := stdout.String()
			for _, want := range []string{"mode repair", "repaired 1", "reaped 1", "run cafe", "repaired .pyxis/templates/prd-template.md"} {
				if !strings.Contains(out, want) {
					t.Fatalf("summary lacks %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestConvergeVerbPreviewPrintsWouldSetAndPerformsNothing(t *testing.T) {
	tempRepo(t)
	rec := &convergeRec{rep: converge.Report{
		RunID:      "beef",
		Mode:       converge.ModePreview,
		WouldWrite: []converge.Action{{Path: ".pyxis/scripts/hook.sh", Class: relmanifest.EngineVerbatim}},
		WouldFetch: []converge.Fetch{{Artifact: "payload-v0.2.0.tar.gz", URL: "file:///src/payload-v0.2.0.tar.gz"}},
		PreWarmed:  []string{"docgate"},
		Incomplete: []string{"the release manifest is not verifiably cached"},
	}}
	swap(t, &convergeRun, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge", "--preview"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"converge preview (run beef)",
		"write .pyxis/scripts/hook.sh (engine-verbatim)",
		"fetch payload-v0.2.0.tar.gz from file:///src/payload-v0.2.0.tar.gz",
		"pre-warm docgate",
		"incomplete: the release manifest is not verifiably cached",
		"nothing was performed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("preview output lacks %q:\n%s", want, out)
		}
	}
}

func TestConvergeVerbRelaysEngineRefusalAsExitOne(t *testing.T) {
	tempRepo(t)
	rec := &convergeRec{
		rep: converge.Report{
			RunID: "dead", Mode: converge.ModeInstall,
			RefusedByName: []converge.Refusal{{Path: ".pyxis/config.yaml", Class: relmanifest.RepoOwned, Reason: "closed-schema parse failed"}},
		},
		err: errors.New("converge: 1 file(s) refused by name (.pyxis/config.yaml)"),
	}
	swap(t, &convergeRun, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (named refusal)", got)
	}
	if !strings.Contains(stderr.String(), "refused by name") {
		t.Fatalf("refusal not named on stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "refused .pyxis/config.yaml (repo-owned): closed-schema parse failed") {
		t.Fatalf("report not printed before the error:\n%s", stdout.String())
	}

	// A pre-reconcile refusal (zero-value report) prints no summary.
	rec = &convergeRec{err: errors.New("converge: source mode")}
	swap(t, &convergeRun, rec.call)
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"repair"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("zero-value report still printed a summary:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "source mode") {
		t.Fatalf("refusal not named:\n%s", stderr.String())
	}
}

// TestConvergeEndToEndAgainstFixtureRelease runs the REAL engine (no seams)
// over a file:// fixture release: the drifted verbatim file repairs
// byte-exact, the binary cache pre-warms, and a second run is a noop.
func TestConvergeEndToEndAgainstFixtureRelease(t *testing.T) {
	srcDir := t.TempDir()
	plat := resolve.Platform()
	fileBytes := []byte("# pipeline map\n")
	binBytes := []byte("#!/bin/sh\necho fixture\n")
	archive := tarGzOne(t, ".pyxis/pipeline.md", fileBytes)
	fileSum := hexOf(fileBytes)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: "v9.9.9",
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {plat: hexOf(binBytes)}},
		Payload: relmanifest.Payload{
			ArchiveSHA256: hexOf(archive),
			Files:         []relmanifest.PayloadFile{{Path: ".pyxis/pipeline.md", SHA256: &fileSum, Class: relmanifest.EngineVerbatim}},
		},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "release-manifest-v9.9.9.json"), mb)
	writeFile(t, filepath.Join(srcDir, "payload-v9.9.9.tar.gz"), archive)
	writeFile(t, filepath.Join(srcDir, "docgate-v9.9.9-"+plat), binBytes)

	root := t.TempDir()
	writeOperandConfig(t, root, "v9.9.9", hexOf(mb), "file://"+srcDir)
	t.Chdir(root)

	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(converge) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode install — repaired 1") {
		t.Fatalf("summary = %q, want the install-mode repair line", stdout.String())
	}
	got, rerr := os.ReadFile(filepath.Join(root, ".pyxis", "pipeline.md"))
	if rerr != nil || !bytes.Equal(got, fileBytes) {
		t.Fatalf("verbatim file not materialized byte-exact (err %v)", rerr)
	}
	if _, err := os.Stat(filepath.Join(root, ".pyxis", "bin", "docgate-v9.9.9-"+plat)); err != nil {
		t.Fatalf("pre-warm did not cache the pinned binary: %v", err)
	}
	stdout.Reset()
	if got := run([]string{"converge"}, &stdout, &stderr); got != 0 {
		t.Fatalf("second run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode noop") {
		t.Fatalf("second-run summary = %q, want noop", stdout.String())
	}
}

// tarGzOne packs a single-file gzip'd tar for the e2e fixture.
func tarGzOne(t *testing.T, name string, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// tarEntry is one payload file for the multi-file fixture archive.
type tarEntry struct {
	name string
	b    []byte
	exec bool
}

// tarGzMany packs a multi-file gzip'd tar for the T8 e2e fixture (the real
// instantiables + hook variants the tail seeds from).
func tarGzMany(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		mode := int64(0o644)
		if e.exec {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: mode, Size: int64(len(e.b))}); err != nil {
			t.Fatalf("tar header %s: %v", e.name, err)
		}
		if _, err := tw.Write(e.b); err != nil {
			t.Fatalf("tar write %s: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// readRepoFile reads a source-repo file (the real instantiables/hook variants
// the fixture payload carries) relative to the repo root two levels up.
func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read source repo file %s: %v", rel, err)
	}
	return b
}

func TestConvergeConfigDefectsRefuseBeforeTheEngine(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte("bogus:\n  x: y\n"))
	t.Chdir(root)
	denySeams(t) // convergeRun must not run on this path
	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (config defects)", got)
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Fatalf("config defect not named:\n%s", stderr.String())
	}
}

func TestConvergeRefusesOutsideARepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_DIR", filepath.Join(dir, "no-such-git"))
	denySeams(t)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge"}, &stdout, &stderr); got != 2 {
		t.Fatalf("run = %d, want 2 (environment)", got)
	}
	if !strings.Contains(stderr.String(), "no repo root") {
		t.Fatalf("root refusal not named:\n%s", stderr.String())
	}
}

// ---- doctor: the T6 verb glue --------------------------------------------------

func TestDoctorVerbHealthyExitsZero(t *testing.T) {
	root := tempRepo(t)
	rec := &doctorRec{h: converge.Health{
		RunID: "cafe", Mode: "source", SystemVersion: "0.1.0",
		ManifestPinned: false, Healthy: true,
		Findings: []converge.HealthFinding{
			{Class: converge.CheckDrift, Name: "manifest-drift", Detail: "skipped — source mode", Severity: converge.SeverityAdvisory},
		},
	}}
	swap(t, &doctorRun, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(doctor) = %d, want 0 (healthy) (stderr: %s)", got, stderr.String())
	}
	if rec.calls != 1 {
		t.Fatalf("doctorRun calls = %d, want exactly 1", rec.calls)
	}
	if !samePath(t, rec.cfg.Root, root) {
		t.Fatalf("cfg.Root = %q, want the operand root %q", rec.cfg.Root, root)
	}
	if rec.cfg.Logger == nil {
		t.Fatal("cfg.Logger = nil — the library's EVT-3 event needs the verb's logger")
	}
	out := stdout.String()
	for _, want := range []string{
		"doctor: mode source — system 0.1.0, manifest pin absent (run cafe)",
		"epoch: absent",
		"drift:",
		"advisory manifest-drift — skipped — source mode",
		"doctor: healthy — 0 findings, 1 advisory(ies)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("healthy report lacks %q:\n%s", want, out)
		}
	}
}

func TestDoctorVerbFindingsExitOneGroupedAndNamed(t *testing.T) {
	tempRepo(t)
	rec := &doctorRec{h: converge.Health{
		RunID: "beef", Mode: "release", SystemVersion: "v0.4.0",
		ManifestPinned: true, ArtifactSource: "file:///mirror",
		EpochPresent: true,
		Epoch:        config.Epoch{SystemVersion: "v0.4.0", Commit: "c0ffee", ManifestSHA256: "aa"},
		Healthy:      false,
		Findings: []converge.HealthFinding{
			{Class: converge.CheckDrift, Name: ".pyxis/templates/prd-template.md", Detail: "engine-verbatim modified", Severity: converge.SeverityFinding},
			{Class: converge.CheckPrerequisite, Name: "node", Detail: "not found on PATH", Severity: converge.SeverityFinding},
			{Class: converge.CheckCache, Name: "docgate-v0.1.0-x", Detail: "off-version cache entry", Severity: converge.SeverityAdvisory},
		},
	}}
	swap(t, &doctorRun, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(doctor) = %d, want 1 (findings)", got)
	}
	out := stdout.String()
	for _, want := range []string{
		"doctor: mode release — system v0.4.0, manifest pin present (run beef)",
		"artifact source override: file:///mirror",
		"epoch: system v0.4.0, commit c0ffee, manifest aa",
		"drift:",
		"finding  .pyxis/templates/prd-template.md — engine-verbatim modified",
		"prerequisite:",
		"finding  node — not found on PATH",
		"cache:",
		"advisory docgate-v0.1.0-x — off-version cache entry",
		"doctor: UNHEALTHY — 2 finding(s), 1 advisory(ies)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(stderr.String(), "doctor found 2 finding(s)") {
		t.Fatalf("stderr does not name the finding count:\n%s", stderr.String())
	}
}

func TestDoctorVerbRelaysLibraryErrorAsExitOne(t *testing.T) {
	tempRepo(t)
	rec := &doctorRec{err: errors.New("doctor: Config.Root is empty")}
	swap(t, &doctorRun, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(doctor) = %d, want 1 on a library error", got)
	}
	if !strings.Contains(stderr.String(), "Config.Root is empty") {
		t.Fatalf("library error not named:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("no report should print on a library error:\n%s", stdout.String())
	}
}

func TestDoctorVerbConfigDefectsRefuseBeforeTheLibrary(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte("bogus:\n  x: y\n"))
	t.Chdir(root)
	denySeams(t) // doctorRun must not run on this path
	var stdout, stderr bytes.Buffer
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (config defects)", got)
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Fatalf("config defect not named:\n%s", stderr.String())
	}
}

func TestDoctorVerbRefusesOutsideARepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_DIR", filepath.Join(dir, "no-such-git"))
	denySeams(t)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 2 {
		t.Fatalf("run = %d, want 2 (environment)", got)
	}
	if !strings.Contains(stderr.String(), "no repo root") {
		t.Fatalf("root refusal not named:\n%s", stderr.String())
	}
}

// TestDoctorEndToEndAgainstFixtureRelease runs the REAL library (no seams)
// through the verb: a converged file:// fixture install reports healthy
// (exit 0); a tampered verbatim file flips it to a named drift finding
// (exit 1). The environment legs are pinned: a real git repo with the hook
// path active, an isolated git config, a guaranteed node, a carina store.
func TestDoctorEndToEndAgainstFixtureRelease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	srcDir := t.TempDir()
	plat := resolve.Platform()
	fileBytes := []byte("# pipeline map\n")
	binBytes := []byte("#!/bin/sh\necho fixture\n")
	archive := tarGzOne(t, ".pyxis/pipeline.md", fileBytes)
	fileSum := hexOf(fileBytes)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: "v9.9.9",
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {plat: hexOf(binBytes)}},
		Payload: relmanifest.Payload{
			ArchiveSHA256: hexOf(archive),
			Files:         []relmanifest.PayloadFile{{Path: ".pyxis/pipeline.md", SHA256: &fileSum, Class: relmanifest.EngineVerbatim}},
		},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "release-manifest-v9.9.9.json"), mb)
	writeFile(t, filepath.Join(srcDir, "payload-v9.9.9.tar.gz"), archive)
	writeFile(t, filepath.Join(srcDir, "docgate-v9.9.9-"+plat), binBytes)

	root := t.TempDir()
	pin := hexOf(mb)
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), fmt.Appendf(nil,
		"schema_version: pyxis-config/v2\nsystem:\n  version: \"v9.9.9\"\n  manifest_sha256: %q\n  artifact_source: %q\n  mode: release\nknobs:\n  tracker: carina\nepoch:\n  system_version: \"v9.9.9\"\n  commit: %q\n  manifest_sha256: %q\n",
		pin, "file://"+srcDir, strings.Repeat("ab", 20), pin))
	t.Chdir(root)

	// Pin the environment legs: isolated git config, an initialized repo
	// with the hook path active, the carina store, a guaranteed node.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if out, gerr := exec.Command("git", "init", "-q", root).CombinedOutput(); gerr != nil {
		t.Skipf("git init: %v (%s)", gerr, out)
	}
	if out, gerr := exec.Command("git", "-C", root, "config", "core.hooksPath", ".githooks").CombinedOutput(); gerr != nil {
		t.Fatalf("git config: %v (%s)", gerr, out)
	}
	if err := os.MkdirAll(filepath.Join(root, ".workitems"), 0o755); err != nil {
		t.Fatal(err)
	}
	stub := t.TempDir()
	if err := os.WriteFile(filepath.Join(stub, "node"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	if got := run([]string{"converge"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(converge) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(doctor) = %d, want 0 on a clean install (stdout: %s stderr: %s)", got, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"doctor: mode release — system v9.9.9, manifest pin present",
		"artifact source override: file://" + srcDir,
		"epoch: system v9.9.9",
		"doctor: healthy — 0 findings, 0 advisory(ies)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("healthy report lacks %q:\n%s", want, out)
		}
	}

	// Tamper the one engine-verbatim file: doctor names the drift, exit 1.
	writeFile(t, filepath.Join(root, ".pyxis", "pipeline.md"), []byte("tampered\n"))
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"doctor"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(doctor) = %d, want 1 after tamper (stdout: %s)", got, stdout.String())
	}
	out = stdout.String()
	for _, want := range []string{"drift:", "finding  .pyxis/pipeline.md — engine-verbatim modified", "doctor: UNHEALTHY — 1 finding(s)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("tamper report lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(stderr.String(), "doctor found 1 finding(s)") {
		t.Fatalf("stderr does not name the finding count:\n%s", stderr.String())
	}
}

// ---- adopt: the T3 verb glue ------------------------------------------------

// adoptRec captures the adopt seam: the exact Options handed to the engine.
type adoptRec struct {
	calls int
	opts  adopt.Options
	rep   adopt.Report
	err   error
}

func (a *adoptRec) call(_ context.Context, opts adopt.Options) (adopt.Report, error) {
	a.calls++
	a.opts = opts
	return a.rep, a.err
}

// TestAdoptVerbResolvesPinFromEnvAndFile proves adopt's pin hand-off matches
// init's own three-way resolution (flag, then PYXIS_* env, then install.sh's
// release.env) — a second call site sharing firstNonEmpty/installReleaseDefaults,
// so a machine set up by the bootstrap needs no pin flags for adopt either.
func TestAdoptVerbResolvesPinFromFlagAndFile(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	runAdopt := func(t *testing.T, args ...string) installer.Pin {
		t.Helper()
		root := t.TempDir()
		t.Chdir(root)
		rec := &adoptRec{rep: adopt.Report{RoutedToInit: true, Init: installer.Report{Branch: "main"}}}
		swap(t, &adoptInspect, rec.call)
		var out, errb bytes.Buffer
		if got := run(append([]string{"adopt"}, args...), &out, &errb); got != 0 {
			t.Fatalf("run(adopt %v) = %d, want 0 (%s)", args, got, errb.String())
		}
		if rec.calls != 1 {
			t.Fatalf("adoptInspect calls = %d, want 1", rec.calls)
		}
		return rec.opts.Pin
	}

	// As for init: the environment is not a pin source.
	t.Run("the environment does not supply the pin", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
		t.Setenv("PYXIS_MANIFEST_SHA256", sha)
		var stdout, stderr bytes.Buffer
		if code := adoptVerb(nil, &stdout, &stderr); code == 0 {
			t.Fatal("adopt accepted a pin from the environment")
		}
	})

	t.Run("from release.env file", func(t *testing.T) {
		cfg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfg)
		t.Setenv("PYXIS_SYSTEM_VERSION", "")
		t.Setenv("PYXIS_MANIFEST_SHA256", "")
		writeFile(t, filepath.Join(cfg, "pyxis", "release.env"),
			[]byte("PYXIS_SYSTEM_VERSION=v0.9.0\nPYXIS_MANIFEST_SHA256="+sha+"\n"))
		if pin := runAdopt(t); pin.Version != "v0.9.0" || pin.ManifestSHA256 != sha {
			t.Fatalf("pin = %+v, want v0.9.0/%s from file", pin, sha)
		}
	})

	t.Run("a flag overrides the record", func(t *testing.T) {
		cfg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", cfg)
		t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
		t.Setenv("PYXIS_MANIFEST_SHA256", sha)
		writeFile(t, filepath.Join(cfg, "pyxis", "release.env"), []byte("PYXIS_SYSTEM_VERSION=v0.9.0\n"))
		flagSHA := "cd" + strings.Repeat("cd", 31)
		if pin := runAdopt(t, "--system-version", "v1.2.3", "--manifest-sha256", flagSHA); pin.Version != "v1.2.3" || pin.ManifestSHA256 != flagSHA {
			t.Fatalf("pin = %+v, want the flag values v1.2.3/%s", pin, flagSHA)
		}
	})
}

// TestAdoptVerbMapsFlagsAndTarget proves the flag → Options fold: the target
// is the working directory (adopt takes no positionals), --artifact-source
// passes through, and the library gets a non-nil logger.
func TestAdoptVerbMapsFlagsAndTarget(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &adoptRec{rep: adopt.Report{RoutedToInit: true, Init: installer.Report{Branch: "main"}}}
	swap(t, &adoptInspect, rec.call)

	var stdout, stderr bytes.Buffer
	args := []string{"adopt",
		"--system-version", "v0.4.0",
		"--manifest-sha256", strings.Repeat("ab", 32),
		"--artifact-source", "file:///mirror",
	}
	if got := run(args, &stdout, &stderr); got != 0 {
		t.Fatalf("run(adopt …) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if rec.calls != 1 {
		t.Fatalf("adoptInspect calls = %d, want exactly 1", rec.calls)
	}
	o := rec.opts
	if !samePath(t, o.Target, root) {
		t.Errorf("Options.Target = %q, want the working directory %q", o.Target, root)
	}
	if o.Pin.Version != "v0.4.0" || o.Pin.ManifestSHA256 != strings.Repeat("ab", 32) {
		t.Errorf("pin = %+v, want the flag pair", o.Pin)
	}
	if o.ArtifactSource != "file:///mirror" {
		t.Errorf("ArtifactSource = %q, want the flag override", o.ArtifactSource)
	}
	if o.Logger == nil {
		t.Error("Options.Logger = nil — the library's events need the verb's logger")
	}
}

// TestAdoptVerbRelaysEngineRefusalAsExitOne proves an Inspect refusal (e.g.
// UB-5/UB-7) is relayed as a named exit-1 with no summary printed.
func TestAdoptVerbRelaysEngineRefusalAsExitOne(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &adoptRec{err: errors.New("adopt: the prior pyxis config surface does not parse under the closed schema: bogus_section")}
	swap(t, &adoptInspect, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"adopt"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(adopt) = %d, want 1 (engine refusal)", got)
	}
	if !strings.Contains(stderr.String(), "bogus_section") {
		t.Errorf("refusal not relayed on stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a failed adopt still printed a summary:\n%s", stdout.String())
	}
}

// TestAdoptVerbPrintsRoutedToInitSummary proves the empty-tree leg reuses
// init's own report printer (adopt performed a real init, not a preview).
func TestAdoptVerbPrintsRoutedToInitSummary(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &adoptRec{rep: adopt.Report{RoutedToInit: true, Init: installer.Report{
		RunID: "abc123", Branch: "main", GitInited: true, EpochWritten: true,
	}}}
	swap(t, &adoptInspect, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"adopt", "--system-version", "v1", "--manifest-sha256", strings.Repeat("ab", 32)}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(adopt) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"adopt: the target is an empty repository — routed to init",
		"init: converged main (run abc123)",
		"epoch written",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("routed-to-init summary lacks %q:\n%s", want, out)
		}
	}
}

// TestAdoptVerbPrintsDraftPlanSummary proves the classify leg names the plan
// path it wrote and renders the SAME reviewable face (Plan.Render) the plan
// file itself carries.
func TestAdoptVerbPrintsDraftPlanSummary(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	plan := adopt.Plan{
		SchemaVersion: adopt.SchemaVersion,
		Status:        adopt.StatusDraft,
		Entries: []adopt.Entry{
			{Path: "src/**", Class: adopt.ClassNeverTouch},
			{Path: ".pyxis/templates/prd-template.md", Class: adopt.ClassEngineVerbatim},
		},
	}
	rec := &adoptRec{rep: adopt.Report{Plan: plan, PlanPath: adopt.PlanPath}}
	swap(t, &adoptInspect, rec.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"adopt", "--system-version", "v1", "--manifest-sha256", strings.Repeat("ab", 32)}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(adopt) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"adopt: wrote a draft plan to " + adopt.PlanPath + " — review it, then run `pyxis apply`",
		plan.Render(),
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("draft-plan summary lacks %q:\n%s", want, out)
		}
	}
}

// TestAdoptVerbRefusesOutsideARepo proves UB-5 (no .git entry) is relayed as
// a NAMED exit-1 engine refusal through the real, unseamed Inspect — not a
// usage/environment exit-2 the way converge/doctor's repo-root walk refuses;
// adopt's target is always the working directory (T3), and Inspect itself
// decides the target carries nothing to negotiate.
func TestAdoptVerbRefusesOutsideARepo(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"adopt"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(adopt) = %d, want 1 (UB-5 engine refusal)", got)
	}
	if !strings.Contains(stderr.String(), "not a git repository") {
		t.Fatalf("UB-5 refusal not named:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("a refused adopt still printed a summary:\n%s", stdout.String())
	}
}

// TestAdoptEndToEndAgainstFixtureRelease runs the REAL adopt.Inspect (no
// seams) through the verb: a non-empty git fixture repository classifies
// against a file:// fixture manifest and the CLI writes exactly the draft
// plan (FR-1/FR-2). Drift never fetches the payload archive (§3.1), so the
// fixture release needs only a manifest — no archive, no binaries.
func TestAdoptEndToEndAgainstFixtureRelease(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Fixture Author")
	t.Setenv("GIT_AUTHOR_EMAIL", "fixture@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Fixture Author")
	t.Setenv("GIT_COMMITTER_EMAIL", "fixture@example.com")
	t.Setenv("HOME", t.TempDir())

	const ver = "v0.5.0"
	tmplBytes := []byte("# PRD template\nbody\n")
	tmplSum := hexOf(tmplBytes)
	srcDir := t.TempDir()
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: ver,
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{},
		Payload: relmanifest.Payload{
			ArchiveSHA256: hexOf([]byte("never fetched by Drift")),
			Files:         []relmanifest.PayloadFile{{Path: ".pyxis/templates/prd-template.md", SHA256: &tmplSum, Class: relmanifest.EngineVerbatim}},
		},
		DepsBundleSHA256: strings.Repeat("ab", 32),
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "release-manifest-"+ver+".json"), mb)
	pin := hexOf(mb)

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), []byte("# my project\n"))
	t.Chdir(root)
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "-A"}, {"commit", "-m", "initial"}} {
		if out, gerr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); gerr != nil {
			t.Fatalf("git %v: %v\n%s", args, gerr, out)
		}
	}

	var stdout, stderr bytes.Buffer
	args := []string{"adopt",
		"--system-version", ver,
		"--manifest-sha256", pin,
		"--artifact-source", "file://" + srcDir,
	}
	if got := run(args, &stdout, &stderr); got != 0 {
		t.Fatalf("run(adopt) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "adopt: wrote a draft plan to "+adopt.PlanPath) {
		t.Fatalf("summary lacks the plan-path line:\n%s", out)
	}
	if !strings.Contains(out, "`.pyxis/templates/prd-template.md` — engine-verbatim") {
		t.Fatalf("summary lacks the classified engine-verbatim entry:\n%s", out)
	}
	if !strings.Contains(out, "`README.md` — never-touch") {
		t.Fatalf("summary lacks the never-touch entry for the adopter's own content:\n%s", out)
	}

	b, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(adopt.PlanPath)))
	if rerr != nil {
		t.Fatalf("read the written plan: %v", rerr)
	}
	p, perr := adopt.Parse(b)
	if perr != nil {
		t.Fatalf("parse the written plan: %v", perr)
	}
	if p.Status != adopt.StatusDraft {
		t.Errorf("Plan.Status = %q, want draft", p.Status)
	}
}

// ---- apply / undo: the T8/T10 verb glue -------------------------------------

// TestApplyVerbResolvesPinAndRendersReport proves `pyxis apply` shares the same
// three-way pin resolution as adopt/init and renders Apply's report — the epoch,
// the chained-hooks line, and each written path.
func TestApplyVerbResolvesPinAndRendersReport(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	applyCfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", applyCfg)
	writeFile(t, filepath.Join(applyCfg, "pyxis", "release.env"),
		[]byte("PYXIS_SYSTEM_VERSION=v0.7.0\nPYXIS_MANIFEST_SHA256="+strings.Repeat("ab", 32)+"\n"))

	var got adopt.Options
	calls := 0
	swap(t, &adoptApply, func(_ context.Context, o adopt.Options) (adopt.ApplyReport, error) {
		calls++
		got = o
		return adopt.ApplyReport{
			PlanPath:    adopt.PlanPath,
			EpochCommit: "abc123def456",
			Chained:     true,
			Actions:     []converge.Action{{Path: "docs/tech-design/index.md", Class: relmanifest.Generated}},
		}, nil
	})

	var out, errb bytes.Buffer
	if code := run([]string{"apply"}, &out, &errb); code != 0 {
		t.Fatalf("run(apply) = %d, want 0 (stderr: %s)", code, errb.String())
	}
	if calls != 1 {
		t.Fatalf("adoptApply calls = %d, want 1", calls)
	}
	if got.Pin.Version != "v0.7.0" || got.Pin.ManifestSHA256 != strings.Repeat("ab", 32) {
		t.Errorf("pin = %+v, want v0.7.0 from env", got.Pin)
	}
	if !samePath(t, got.Target, root) {
		t.Errorf("Options.Target = %q, want the working directory %q", got.Target, root)
	}
	for _, want := range []string{"materialised the plan", "epoch abc123def456", "chained the engine's hooks", "docs/tech-design/index.md (generated)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("apply output lacks %q:\n%s", want, out.String())
		}
	}
}

// TestApplyVerbRelaysRefusalAsExitOne proves a stale/absent-plan refusal is an
// exit-1 named on stderr with no report on stdout.
func TestApplyVerbRelaysRefusalAsExitOne(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
	t.Setenv("PYXIS_MANIFEST_SHA256", strings.Repeat("ab", 32))
	swap(t, &adoptApply, func(context.Context, adopt.Options) (adopt.ApplyReport, error) {
		return adopt.ApplyReport{}, errors.New("adopt: no approved plan at .pyxis/adopt/plan.json")
	})
	var out, errb bytes.Buffer
	if code := run([]string{"apply"}, &out, &errb); code != 1 {
		t.Fatalf("run(apply) = %d, want 1 (engine refusal)", code)
	}
	if !strings.Contains(errb.String(), "no approved plan") {
		t.Errorf("refusal not relayed on stderr: %s", errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refused apply still printed a report:\n%s", out.String())
	}
}

// TestUndoVerbResolvesPinAndRendersReport proves `pyxis undo` renders Undo's
// report — the removed paths, the preserved (operator-edited) ones, and the
// collide-manual entries the human must reverse by hand.
func TestUndoVerbResolvesPinAndRendersReport(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
	t.Setenv("PYXIS_MANIFEST_SHA256", strings.Repeat("ab", 32))

	calls := 0
	swap(t, &adoptUndo, func(_ context.Context, _ adopt.Options) (adopt.UndoReport, error) {
		calls++
		return adopt.UndoReport{
			Unchained: true,
			Removed:   []string{"docs/tech-design/index.md", ".pyxis/config.yaml"},
			Preserved: []string{".pyxis/templates/prd-template.md"},
			// A MULTI-LINE detail (the real shape of collide-manual instruction
			// lines) exercises indent's per-line prefixing, including a nested line.
			ManualEntries: []adopt.Entry{{Path: "AGENTS.md", Class: adopt.ClassCollideManual, Detail: "- remove the pyxis lines you added:\n  - the converge line\n  - the design-records line"}},
		}, nil
	})

	var out, errb bytes.Buffer
	if code := run([]string{"undo"}, &out, &errb); code != 0 {
		t.Fatalf("run(undo) = %d, want 0 (stderr: %s)", code, errb.String())
	}
	if calls != 1 {
		t.Fatalf("adoptUndo calls = %d, want 1", calls)
	}
	for _, want := range []string{
		"reversed the adoption — removed 2 path(s), preserved 1, 1 manual",
		"restored the adopter's prior hook path",
		"- docs/tech-design/index.md",
		"~ preserved (modified since adoption",
		"reverse by hand (AGENTS.md)",
		// indent prefixes every line of the detail: the first at 6 spaces, the
		// nested "  - …" line at 6+2 — the multi-line branch under test.
		"      - remove the pyxis lines you added:",
		"        - the converge line",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("undo output lacks %q:\n%s", want, out.String())
		}
	}
}

// TestUndoVerbRelaysRefusalAsExitOne proves an Undo refusal (e.g. no applied
// plan) is an exit-1 named on stderr with no report on stdout — the symmetry
// apply already asserts.
func TestUndoVerbRelaysRefusalAsExitOne(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("PYXIS_SYSTEM_VERSION", "v0.7.0")
	t.Setenv("PYXIS_MANIFEST_SHA256", strings.Repeat("ab", 32))
	swap(t, &adoptUndo, func(context.Context, adopt.Options) (adopt.UndoReport, error) {
		return adopt.UndoReport{}, errors.New("adopt: no applied plan at .pyxis/adopt/plan.json to undo")
	})
	var out, errb bytes.Buffer
	if code := run([]string{"undo"}, &out, &errb); code != 1 {
		t.Fatalf("run(undo) = %d, want 1 (engine refusal)", code)
	}
	if !strings.Contains(errb.String(), "no applied plan") {
		t.Errorf("refusal not relayed on stderr: %s", errb.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refused undo still printed a report:\n%s", out.String())
	}
}

// TestApplyUndoRefuseStrayPositional proves both verbs share adopt's
// no-positional grammar (I3: refuse before any action).
func TestApplyUndoRefuseStrayPositional(t *testing.T) {
	for _, verb := range []string{"apply", "undo"} {
		var out, errb bytes.Buffer
		if code := run([]string{verb, "extra"}, &out, &errb); code != 2 {
			t.Errorf("run(%s extra) = %d, want 2 (usage refusal)", verb, code)
		}
		if !strings.Contains(errb.String(), "takes no arguments") {
			t.Errorf("%s stray-positional refusal not named: %s", verb, errb.String())
		}
	}
}

// ---- version ----------------------------------------------------------------

func TestVersionPrintsTheBuildVersion(t *testing.T) {
	denySeams(t)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"version"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(version) = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if got := stdout.String(); got != "pyxis source\n" {
		t.Fatalf("version printed %q, want %q (the ldflags-settable default)", got, "pyxis source\n")
	}
}

// ---- exec: argv-verbatim, exit relay, refusals -------------------------------

func TestExecPassesArgvVerbatim(t *testing.T) {
	tempRepo(t)
	res := &resolveRec{path: "/fake/docgate-bin"}
	swap(t, &resolveTool, res.call)
	cases := []struct {
		name string
		args []string
		want []string // the bytes after --, exactly
	}{
		{"tool flags pass untouched", []string{"exec", "docgate", "--", "gate", "--merge-base", "origin/main"}, []string{"gate", "--merge-base", "origin/main"}},
		{"an inner separator passes", []string{"exec", "docgate", "--", "fold", "--", "docs/prds/x"}, []string{"fold", "--", "docs/prds/x"}},
		{"a flag-like first token passes", []string{"exec", "docgate", "--", "--defects-json", "-"}, []string{"--defects-json", "-"}},
		{"empty argv is legal", []string{"exec", "docgate", "--"}, nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ex := &execRec{}
			swap(t, &execFn, ex.call)
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != 0 {
				t.Fatalf("run(%q) = %d, want 0 (stderr: %s)", tt.args, got, stderr.String())
			}
			if ex.calls != 1 {
				t.Fatalf("execFn calls = %d, want exactly 1", ex.calls)
			}
			if ex.path != res.path {
				t.Fatalf("exec path = %q, want the resolved %q", ex.path, res.path)
			}
			want := append([]string{res.path}, tt.want...)
			if !slices.Equal(ex.argv, want) {
				t.Fatalf("argv = %q, want %q — bytes after -- pass verbatim, never interpreted (DINV-5)", ex.argv, want)
			}
		})
	}
	if res.tool != "docgate" || res.platform != "" {
		t.Fatalf("resolve called with (%q, %q), want (docgate, \"\" → derived platform)", res.tool, res.platform)
	}
}

func TestExecRelaysToolExitCode(t *testing.T) {
	tempRepo(t)
	swap(t, &resolveTool, (&resolveRec{path: "/fake/tool"}).call)
	ex := &execRec{code: 3}
	swap(t, &execFn, ex.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 3 {
		t.Fatalf("run = %d, want the tool's exit code 3 relayed", got)
	}
}

func TestExecReportsExecFailure(t *testing.T) {
	tempRepo(t)
	swap(t, &resolveTool, (&resolveRec{path: "/fake/tool"}).call)
	ex := &execRec{err: errors.New("execve /fake/tool: permission denied")}
	swap(t, &execFn, ex.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 on an exec failure", got)
	}
	if !strings.Contains(stderr.String(), "exec docgate") {
		t.Fatalf("exec failure not named:\n%s", stderr.String())
	}
}

func TestExecRefusesTypedResolutionErrors(t *testing.T) {
	tempRepo(t)
	cases := []struct {
		name string
		err  error
		want string // the named refusal fragment on stderr
	}{
		{"tamper", &resolve.TamperError{Artifact: "docgate-v0.2.0-x", Expected: "aa", Actual: "bb"}, "sha256 mismatch"},
		{"offline", &resolve.OfflineError{Artifact: "a", URL: "file:///nope", Err: errors.New("dial")}, "pre-warm while online"},
		{"missing pin", &resolve.MissingPinError{Key: "system.manifest_sha256"}, "system.manifest_sha256"},
		{"wrong platform", &resolve.WrongPlatformError{Tool: "docgate", Platform: "plan9-386", Version: "v0.2.0"}, "no binary named"},
		{"source tree absent", &resolve.SourceTreeAbsentError{Tool: "docgate", CmdDir: "/x/cmd/docgate"}, "DINV-8"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			swap(t, &resolveTool, (&resolveRec{err: tt.err}).call)
			ex := &execRec{}
			swap(t, &execFn, ex.call)
			gr := &goRunRec{}
			swap(t, &goRunFn, gr.call)
			var stdout, stderr bytes.Buffer
			if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 1 {
				t.Fatalf("run = %d, want 1 (named refusal, distinct from usage's 2)", got)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("refusal does not name %q:\n%s", tt.want, stderr.String())
			}
			if ex.calls != 0 || gr.calls != 0 {
				t.Fatal("an execution seam ran after a resolution refusal (DINV-1: zero execution)")
			}
		})
	}
}

// ---- exec: source mode (real resolve fall-through) ---------------------------

func TestExecSourceModeFallsThroughToGoRun(t *testing.T) {
	root := t.TempDir()
	// A source-mode operand: the config file is the root marker (its absence
	// of keys means defaults — mode "source"), and the tool's source lives.
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte("# defaults: source mode\n"))
	if err := os.MkdirAll(filepath.Join(root, "cmd", "docgate"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	gr := &goRunRec{code: 5}
	swap(t, &goRunFn, gr.call)
	ex := &execRec{}
	swap(t, &execFn, ex.call)
	var stdout, stderr bytes.Buffer
	got := run([]string{"exec", "docgate", "--", "gate", "--merge-base", "origin/main"}, &stdout, &stderr)
	if got != 5 {
		t.Fatalf("run = %d, want go-run's exit code 5 relayed (stderr: %s)", got, stderr.String())
	}
	if gr.calls != 1 || gr.sm == nil {
		t.Fatalf("goRunFn calls = %d (sm %v), want exactly 1 with the typed fall-through", gr.calls, gr.sm)
	}
	if gr.sm.Tool != "docgate" || !samePath(t, gr.sm.RepoRoot, root) {
		t.Fatalf("SourceMode = %+v, want tool docgate rooted at %s", gr.sm, root)
	}
	if want := []string{"gate", "--merge-base", "origin/main"}; !slices.Equal(gr.argv, want) {
		t.Fatalf("source-mode argv = %q, want %q verbatim (DINV-5 holds on this leg too)", gr.argv, want)
	}
	if ex.calls != 0 {
		t.Fatal("execFn must not run in source mode")
	}
}

// ---- exec: the full acquisition path, no seams -------------------------------

func TestExecEndToEndResolvesVerifiedBinary(t *testing.T) {
	srcDir := t.TempDir() // the file:// artifact source
	plat := resolve.Platform()
	binBytes := []byte("#!/bin/sh\necho fixture\n")
	dummy := strings.Repeat("ab", 32)
	man := relmanifest.Manifest{
		SchemaVersion: relmanifest.ManifestSchemaVersion,
		SystemVersion: "v9.9.9",
		SourceCommit:  strings.Repeat("cd", 20),
		Binaries:      map[string]map[string]string{"docgate": {plat: hexOf(binBytes)}},
		Payload: relmanifest.Payload{
			ArchiveSHA256: dummy,
			Files:         []relmanifest.PayloadFile{{Path: ".pyxis/pipeline.md", SHA256: &dummy, Class: relmanifest.EngineVerbatim}},
		},
		DepsBundleSHA256: dummy,
	}
	mb, err := man.Encode()
	if err != nil {
		t.Fatalf("encode fixture manifest: %v", err)
	}
	writeFile(t, filepath.Join(srcDir, "docgate-v9.9.9-"+plat), binBytes)
	writeFile(t, filepath.Join(srcDir, "release-manifest-v9.9.9.json"), mb)

	root := t.TempDir()
	writeOperandConfig(t, root, "v9.9.9", hexOf(mb), "file://"+srcDir)
	t.Chdir(root)

	ex := &execRec{}
	swap(t, &execFn, ex.call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	wantPath := filepath.Join(root, ".pyxis", "bin", "docgate-v9.9.9-"+plat)
	if !samePath(t, ex.path, wantPath) {
		t.Fatalf("exec path = %q, want the verified cache entry %q", ex.path, wantPath)
	}
	cached, rerr := os.ReadFile(ex.path)
	if rerr != nil || !bytes.Equal(cached, binBytes) {
		t.Fatalf("cached binary differs from the fixture (err %v)", rerr)
	}
	if want := []string{ex.path, "gate"}; !slices.Equal(ex.argv, want) {
		t.Fatalf("argv = %q, want %q", ex.argv, want)
	}
}

// ---- exec: environment and config refusals -----------------------------------

func TestExecConfigDefectsRefuse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"), []byte("bogus:\n  x: y\n"))
	t.Chdir(root)
	denySeams(t)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run = %d, want 1 (config defects, closed parse)", got)
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Fatalf("config defect not named:\n%s", stderr.String())
	}
}

func TestExecRefusesOutsideARepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("GIT_DIR", filepath.Join(dir, "no-such-git"))
	denySeams(t)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 2 {
		t.Fatalf("run = %d, want 2 (environment)", got)
	}
	if !strings.Contains(stderr.String(), "no repo root") {
		t.Fatalf("root refusal not named:\n%s", stderr.String())
	}
}

// ---- root location -----------------------------------------------------------

func TestRepoRootWalksUpFromASubdirectory(t *testing.T) {
	root := t.TempDir()
	writeOperandConfig(t, root, "v0.2.0", strings.Repeat("ab", 32), "")
	sub := filepath.Join(root, "docs", "prds")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	res := &resolveRec{path: "/fake/tool"}
	swap(t, &resolveTool, res.call)
	swap(t, &execFn, (&execRec{}).call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if res.calls != 1 || !samePath(t, res.cfg.Root, root) {
		t.Fatalf("resolve.Config.Root = %q (calls %d), want the walked-up root %q", res.cfg.Root, res.calls, root)
	}
}

func TestRepoRootFallsBackToGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	if out, ierr := exec.Command("git", "init", "-q", root).CombinedOutput(); ierr != nil {
		t.Skipf("git init: %v (%s)", ierr, out)
	}
	sub := filepath.Join(root, "inner")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub) // no .pyxis/config.yaml anywhere above — the git probe decides
	res := &resolveRec{path: "/fake/tool"}
	swap(t, &resolveTool, res.call)
	swap(t, &execFn, (&execRec{}).call)
	var stdout, stderr bytes.Buffer
	if got := run([]string{"exec", "docgate", "--", "gate"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", got, stderr.String())
	}
	if res.calls != 1 || !samePath(t, res.cfg.Root, root) {
		t.Fatalf("resolve.Config.Root = %q (calls %d), want the git toplevel %q", res.cfg.Root, res.calls, root)
	}
}

// ---- the real go-run leg -------------------------------------------------------

func TestGoRunRunsFromRepoRootAndRelaysExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: compiles a fixture tool with the real toolchain")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.21\n"))
	writeFile(t, filepath.Join(root, "cmd", "echoargs", "main.go"), []byte(`package main

import (
	"os"
	"strings"
)

// echoargs proves the go-run leg: it records its argv relative to its working
// directory (so the file lands at the repo root) and exits 7.
func main() {
	_ = os.WriteFile("argv.txt", []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	os.Exit(7)
}
`))
	code, err := goRun(&resolve.SourceMode{Tool: "echoargs", RepoRoot: root, CmdDir: filepath.Join(root, "cmd", "echoargs")}, []string{"gate", "--merge-base", "origin/main"})
	if err != nil {
		t.Fatalf("goRun: %v", err)
	}
	if code != 7 {
		t.Fatalf("goRun exit code = %d, want the tool's 7 relayed", code)
	}
	b, rerr := os.ReadFile(filepath.Join(root, "argv.txt"))
	if rerr != nil {
		t.Fatalf("argv.txt missing at the repo root — goRun must run with cwd at RepoRoot (the shell resolvers' contract): %v", rerr)
	}
	if want := "gate\n--merge-base\norigin/main"; string(b) != want {
		t.Fatalf("child argv = %q, want %q verbatim", b, want)
	}

	// Exit-code FIDELITY is the point of the build-then-run leg: plain
	// `go run` flattens every nonzero exit to 1, which would erase the
	// contractual 0/1/2 verdict split (usage 2 vs defects 1) in source mode
	// and diverge the parity suite (NR-2). Exit 2 must come through as 2.
	writeFile(t, filepath.Join(root, "cmd", "usage2", "main.go"), []byte("package main\n\nimport \"os\"\n\nfunc main() { os.Exit(2) }\n"))
	code, err = goRun(&resolve.SourceMode{Tool: "usage2", RepoRoot: root, CmdDir: filepath.Join(root, "cmd", "usage2")}, nil)
	if err != nil {
		t.Fatalf("goRun(usage2): %v", err)
	}
	if code != 2 {
		t.Fatalf("goRun exit code = %d, want 2 relayed exactly (go run would flatten it to 1)", code)
	}
}

func TestGoRunNamesACompileFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: invokes the real toolchain")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.21\n"))
	writeFile(t, filepath.Join(root, "cmd", "broken", "main.go"), []byte("package main\n\nfunc main() { this does not compile }\n"))
	code, err := goRun(&resolve.SourceMode{Tool: "broken", RepoRoot: root, CmdDir: filepath.Join(root, "cmd", "broken")}, nil)
	if err == nil {
		t.Fatalf("goRun(broken) = %d with nil error, want a named build failure", code)
	}
	if !strings.Contains(err.Error(), "build ./cmd/broken") {
		t.Fatalf("build failure not named: %v", err)
	}
}

// The COMPOSED init screen must END on the epilogue (#937): every summary
// line — the stages row included — then any hand-off commands, then the
// welcome block with the next actions, so "what do I do now" is the last
// thing on screen, never buried mid-scroll. Exercises printInitReport, not
// just the tail: the first version of this change tested only printInitTail
// and missed the caller's stages row printing after the epilogue.
func TestPrintInitReportEndsOnTheEpilogue(t *testing.T) {
	rep := installer.Report{
		RunID:  "run1234",
		Branch: "main",
		Stages: []string{"remote", "converge", "welcome"},
	}
	rep.T8.Wiring = &wire.Audit{
		Operations: []wire.OpResult{
			{Name: wire.OpCreateRepo, Outcome: wire.OutcomeApplied},
			{Name: wire.OpSetProtection, Outcome: wire.OutcomeHandedOff, Reason: "branch main does not exist on GitHub yet"},
		},
		Partial: true,
		HandOff: "# wiring steps that need you\ngh api --method PUT repos/o/r/branches/main/protection\n",
	}
	rep.T8.GreenProof.Green = true
	rep.T8.Welcome = installer.WelcomeResult{
		Emitted:   true,
		NextSteps: []string{"gates: docs gate live at your desk", "pending: the code gate"},
	}

	var out bytes.Buffer
	printInitReport(&out, rep)
	s := out.String()

	positions := []struct {
		label string
		index int
	}{
		{"stages row", strings.Index(s, "stages: remote → converge → welcome")},
		{"wiring summary", strings.Index(s, "wiring: 1 of 2 steps done — 1 waiting on you")},
		{"hand-off block", strings.Index(s, "# wiring steps that need you")},
		{"epilogue banner", strings.Index(s, installer.WelcomeBanner)},
		{"deferred-step pointer", strings.Index(s, "waiting on you: 1 wiring step(s)")},
	}
	last := -1
	for _, p := range positions {
		if p.index < 0 {
			t.Fatalf("%s missing from the report:\n%s", p.label, s)
		}
		if p.index <= last {
			t.Fatalf("%s out of order (the epilogue must close the screen):\n%s", p.label, s)
		}
		last = p.index
	}
	if tail := s[positions[3].index:]; strings.Contains(tail, "stages:") {
		t.Fatalf("internals print after the epilogue:\n%s", s)
	}
}

// Styling only ever wraps (#948): forced-on rendering carries the same
// words as plain rendering plus ANSI escapes; plain stays escape-free.
func TestWelcomeEpilogueStylingOnlyWraps(t *testing.T) {
	var rep installer.Report
	rep.T8.Welcome = installer.WelcomeResult{
		Emitted:   true,
		NextSteps: []string{"gates: docs gate live", "your first feature is one /feature-brainstorm away ✨"},
	}
	plain := welcomeEpilogue(termstyle.Forced(false), rep, 1)
	styled := welcomeEpilogue(termstyle.Forced(true), rep, 1)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain epilogue carries escapes:\n%q", plain)
	}
	if !strings.Contains(styled, "\x1b[") {
		t.Fatal("forced styling emitted no escapes")
	}
	for _, want := range []string{installer.WelcomeBanner, "gates: docs gate live", "waiting on you: 1 wiring step(s)"} {
		stripped := strings.NewReplacer("\x1b[1m", "", "\x1b[2m", "", "\x1b[32m", "", "\x1b[33m", "", "\x1b[36m", "", "\x1b[0m", "").Replace(styled)
		if !strings.Contains(stripped, want) {
			t.Errorf("styled epilogue (escapes stripped) lost %q:\n%s", want, stripped)
		}
	}
}

// An interrupted INTERVIEW reports the interruption and states — provably,
// because checks-then-go puts every write after the interview — that the
// target is untouched (#954). The operator's real run instead reported
// "run git --version: context canceled" with git perfectly healthy.
func TestInitInterruptedInterviewSaysNothingWasWritten(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	rec := &initRec{err: installer.ErrInterrupted}
	swap(t, &installerInit, rec.call)

	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--yes",
		"--system-version", "v0.4.0",
		"--manifest-sha256", strings.Repeat("ab", 32),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run(init) = %d, want 1 on an interrupted run", code)
	}
	s := stderr.String()
	if !strings.Contains(s, "interrupted") || !strings.Contains(s, "nothing was written") {
		t.Errorf("stderr does not name the interruption and the untouched target:\n%s", s)
	}
}

// A subprocess the run context KILLS mid-flight reports "signal: killed" and
// carries no cancellation in its error chain — so the context decides, or
// init blames an innocent subprocess exactly as #954 describes. Also pins
// the first-interrupt acknowledgement, and that "nothing was written" is
// NOT claimed once the write phase could have started.
func TestInitInterruptedMidRunNamesTheInterruptWithoutOverclaiming(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	swap(t, &installerInit, func(ctx context.Context, _ installer.Options) (installer.Report, error) {
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGINT); err != nil {
			t.Fatalf("signal the operator interrupt: %v", err)
		}
		<-ctx.Done() // the watcher cancels the run
		return installer.Report{}, errors.New("init: stage wiring: gh issue create: signal: killed")
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--yes",
		"--system-version", "v0.4.0",
		"--manifest-sha256", strings.Repeat("ab", 32),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run(init) = %d, want 1", code)
	}
	s := stderr.String()
	if !strings.Contains(s, "interrupt received") {
		t.Errorf("the first interrupt was not acknowledged, so an operator sees nothing and keeps pressing:\n%s", s)
	}
	if !strings.Contains(s, "pyxis: init: interrupted") {
		t.Errorf("a killed-subprocess interrupt was not named as an interruption:\n%s", s)
	}
	if strings.Contains(s, "nothing was written") {
		t.Errorf("claimed an untouched target after the write phase could have begun:\n%s", s)
	}
	if !strings.Contains(s, "check this directory before re-running") {
		t.Errorf("no rollback ran and the operator was not told to check the target:\n%s", s)
	}
}

// The upgrade verb's surface (#1028): it reports the replacement, records
// the pin, and — binary scope by ruling — points at the repo's OWN path
// rather than advertising a flag that does not exist (the #923 class).
func TestUpgradeVerbReportsAndPointsAtTheRepoPath(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	// An installed repo left behind by this upgrade.
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"),
		[]byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v0.1.42\"\n  mode: release\n"))

	swap(t, &upgradeRun, func(_ context.Context, opts upgrade.Options) (upgrade.Report, error) {
		if opts.Current == "" {
			t.Error("the verb passed no current version, so up-to-date can never be detected")
		}
		return upgrade.Report{
			Current: "v0.1.42", Target: "v0.1.43",
			BinaryPath:  "/home/u/.local/bin/pyxis",
			Artifact:    "pyxis-v0.1.43-darwin-arm64",
			SHA256:      strings.Repeat("ab", 32),
			PinRecorded: "/home/u/.config/pyxis/release.env",
		}, nil
	})

	var stdout, stderr bytes.Buffer
	if code := run([]string{"upgrade"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(upgrade) = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	s := stdout.String()
	for _, want := range []string{"v0.1.42 → v0.1.43", "replaced ✓", "release.env", "recorded ✓"} {
		if !strings.Contains(s, want) {
			t.Errorf("upgrade output lacks %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "this repo pins v0.1.42") || !strings.Contains(s, "pyxis converge") {
		t.Errorf("the behind repo was not pointed at its own upgrade path:\n%s", s)
	}
	if strings.Contains(s, "--repo") {
		t.Errorf("the pointer advertises a flag this verb does not have (#923 class):\n%s", s)
	}
}

// The verb hands the engine the installed binary's shell setup and relays its
// account, whether the binary moved or was already current.
func TestUpgradeVerbReinstallsAndReportsTheShellSetup(t *testing.T) {
	t.Chdir(t.TempDir())
	const account = "interactive worktrees enabled for zsh in /home/u/.zshrc"
	for _, upToDate := range []bool{false, true} {
		swap(t, &upgradeRun, func(_ context.Context, opts upgrade.Options) (upgrade.Report, error) {
			if opts.ShellSetup == nil {
				t.Error("the verb wired no shell setup, so an upgrade would leave the old launcher in place")
			}
			return upgrade.Report{Current: "v0.1.42", Target: "v0.1.43", UpToDate: upToDate, BinaryPath: "/bin/pyxis", SHA256: strings.Repeat("ab", 32), Shell: account}, nil
		})
		var stdout, stderr bytes.Buffer
		if code := run([]string{"upgrade"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "  shell   "+account+" ✓") {
			t.Fatalf("upToDate=%v code=%d stdout=%s stderr=%s", upToDate, code, &stdout, &stderr)
		}
	}
}

// A failure after the swap discloses the swap, and says which of the later
// steps the operator can still rely on.
func TestUpgradeVerbDisclosesWhatAFailureLeftDone(t *testing.T) {
	t.Chdir(t.TempDir())
	for pin, want := range map[string]string{
		"":                                  "the release pin may still name the previous version",
		"/home/u/.config/pyxis/release.env": "the release pin is recorded",
	} {
		swap(t, &upgradeRun, func(context.Context, upgrade.Options) (upgrade.Report, error) {
			return upgrade.Report{Current: "v0.1.42", Target: "v0.1.43", BinaryPath: "/bin/pyxis", BinaryReplaced: true, PinRecorded: pin}, errors.New("upgrade: a later step failed")
		})
		var stdout, stderr bytes.Buffer
		code := run([]string{"upgrade"}, &stdout, &stderr)
		s := stderr.String()
		if code != 1 || !strings.Contains(s, "WAS already replaced with v0.1.43") || !strings.Contains(s, want) || !strings.Contains(s, "re-run `pyxis upgrade`") {
			t.Fatalf("pin=%q code=%d stderr=%s", pin, code, s)
		}
	}
}

// Preview prints the would-do set and says nothing was performed.
func TestUpgradeVerbPreviewSaysNothingWasPerformed(t *testing.T) {
	t.Chdir(t.TempDir())
	swap(t, &upgradeRun, func(_ context.Context, opts upgrade.Options) (upgrade.Report, error) {
		if !opts.Preview {
			t.Error("--preview did not reach the engine")
		}
		return upgrade.Report{Current: "v0.1.42", Target: "v0.1.43", Preview: true,
			BinaryPath: "/bin/pyxis", Artifact: "pyxis-v0.1.43-linux-amd64", SHA256: strings.Repeat("cd", 32)}, nil
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"upgrade", "--preview"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d (stderr: %s)", code, stderr.String())
	}
	if s := stdout.String(); !strings.Contains(s, "preview") || !strings.Contains(s, "nothing was performed") {
		t.Errorf("preview output:\n%s", s)
	}
}

// An already-current machine says so, and never mentions a replacement.
func TestUpgradeVerbUpToDate(t *testing.T) {
	t.Chdir(t.TempDir())
	swap(t, &upgradeRun, func(context.Context, upgrade.Options) (upgrade.Report, error) {
		return upgrade.Report{Current: "v0.1.43", Target: "v0.1.43", UpToDate: true}, nil
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"upgrade"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d (stderr: %s)", code, stderr.String())
	}
	s := stdout.String()
	if !strings.Contains(s, "already on v0.1.43") {
		t.Errorf("up-to-date output:\n%s", s)
	}
	if strings.Contains(s, "replaced") {
		t.Errorf("an up-to-date run claimed a replacement:\n%s", s)
	}
}

// The verb parses fail-closed like every other (UB-9 / incident I3).
func TestUpgradeVerbRefusesMalformedArguments(t *testing.T) {
	t.Chdir(t.TempDir())
	swap(t, &upgradeRun, func(context.Context, upgrade.Options) (upgrade.Report, error) {
		t.Error("the engine ran on a refusal path (I3: refuse before any action)")
		return upgrade.Report{}, nil
	})
	for _, args := range [][]string{{"upgrade", "extra"}, {"upgrade", "--nope"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
	}
}

// The pointer must fire when the BINARY is already current but the repo is
// behind — the steady state after any converge lag, and the case the first
// cut suppressed (#1028 review).
func TestUpgradeVerbPointsAtABehindRepoEvenWhenCurrent(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".pyxis", "config.yaml"),
		[]byte("schema_version: pyxis-config/v2\nsystem:\n  version: \"v0.1.20\"\n  mode: release\n"))
	swap(t, &upgradeRun, func(context.Context, upgrade.Options) (upgrade.Report, error) {
		return upgrade.Report{Current: "v0.1.43", Target: "v0.1.43", UpToDate: true}, nil
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"upgrade"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d (stderr: %s)", code, stderr.String())
	}
	s := stdout.String()
	if !strings.Contains(s, "already on v0.1.43") {
		t.Errorf("output:\n%s", s)
	}
	if !strings.Contains(s, "this repo pins v0.1.20") {
		t.Errorf("a behind repo went unmentioned on an up-to-date machine:\n%s", s)
	}
}

// An engine failure exits 1 — a failed upgrade must never look like success
// to a wrapper script or CI step.
func TestUpgradeVerbExitsOneOnEngineFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	swap(t, &upgradeRun, func(context.Context, upgrade.Options) (upgrade.Report, error) {
		return upgrade.Report{}, errors.New("upgrade: checksum mismatch for pyxis-v9-linux-amd64 — refusing to install")
	})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"upgrade"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run = %d, want 1 on an engine failure", code)
	}
	if !strings.Contains(stderr.String(), "checksum mismatch") {
		t.Errorf("stderr does not carry the engine's reason:\n%s", stderr.String())
	}
}
