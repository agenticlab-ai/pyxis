package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCoverage90PrompterAndGitBoundaries(t *testing.T) {
	pr := newPrompter(nil, nil, nil)
	if got, err := pr.ask("question", "default"); err != nil || got != "default" {
		t.Fatalf("nil-backed prompt = (%q, %v)", got, err)
	}

	readErr := errors.New("stdin failed")
	pr = &prompter{out: &bytes.Buffer{}, ctx: context.Background(), lines: make(chan lineRead, 1), readErr: readErr}
	if _, err := pr.ask("question", ""); !errors.Is(err, readErr) {
		t.Fatalf("latched read error = %v", err)
	}
	if _, err := askValidated(pr, trackerQuestion(), func(v string) (string, bool) { return v, true }, &collectedAnswers{}); !errors.Is(err, readErr) {
		t.Fatalf("validated prompt read error = %v", err)
	}

	echoLocalDerivation(nil, config.Knobs{})
	if _, err := resolveSurfaces([]string{surfaceAuto, "codex"}, nil, &collectedAnswers{}); err == nil {
		t.Fatal("mixed auto and explicit CLI surfaces were accepted")
	}
	if _, err := gitCommitPaths(t.Context(), t.TempDir(), nil, "empty"); err == nil {
		t.Fatal("empty commit path set was accepted")
	}
	if _, err := gitCommitPaths(t.Context(), t.TempDir(), []string{"missing"}, "bad repo"); err == nil {
		t.Fatal("staging in a non-repository succeeded")
	}

	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ambientIdentity(t.Context(), notDir, "user.name"); err == nil {
		t.Fatal("git probe against a file target succeeded")
	}
}

func TestCoverage90HeadStateAndOptionValidation(t *testing.T) {
	isolateGitEnv(t)
	repo := t.TempDir()
	if err := gitInitMain(t.Context(), repo, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.name", "Coverage Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "config", "user.email", "coverage@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "add", "tracked"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), repo, "commit", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	if sha, born := headState(t.Context(), repo); !born || strings.TrimSpace(sha) == "" {
		t.Fatalf("headState = (%q, %t)", sha, born)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	validDir := t.TempDir()
	validPin := Pin{Version: "1.2.3", ManifestSHA256: strings.Repeat("a", 64)}
	for _, tc := range []struct {
		name string
		opts Options
	}{
		{name: "blank target"},
		{name: "missing target", opts: Options{Target: missing, Pin: validPin}},
		{name: "target is file", opts: Options{Target: notDir, Pin: validPin}},
		{name: "missing pin", opts: Options{Target: validDir}},
		{name: "bad digest", opts: Options{Target: validDir, Pin: Pin{Version: "1.2.3", ManifestSHA256: "bad"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := tc.opts
			if err := validateOptions(&opts); err == nil {
				t.Fatal("validateOptions unexpectedly succeeded")
			}
		})
	}
	if _, err := parseRendered([]byte("schema: pyxis-config/v99\nunknown: true\n")); err == nil {
		t.Fatal("malformed rendered config was accepted")
	}
}

func TestCoverage90JournalRestoresBornAndUnbornRefs(t *testing.T) {
	isolateGitEnv(t)
	born := t.TempDir()
	if err := gitInitMain(t.Context(), born, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), born, "config", "user.name", "Coverage Test"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), born, "config", "user.email", "coverage@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), born, "commit", "--allow-empty", "-m", "initial"); err != nil {
		t.Fatal(err)
	}
	sha, ok := headState(t.Context(), born)
	if !ok {
		t.Fatal("expected born HEAD")
	}
	bornRun := &initRun{
		j:        newJournal(born),
		rep:      &Report{Commits: []Commit{{SHA: sha}}},
		branch:   "main",
		headBorn: true,
		headSHA:  sha,
	}
	if err := bornRun.restoreGitRef(t.Context()); err != nil {
		t.Fatal(err)
	}

	unborn := t.TempDir()
	if err := gitInitMain(t.Context(), unborn, "main"); err != nil {
		t.Fatal(err)
	}
	unbornRun := &initRun{
		j:      newJournal(unborn),
		rep:    &Report{Commits: []Commit{{SHA: "created"}}},
		branch: "main",
	}
	if err := unbornRun.restoreGitRef(t.Context()); err != nil {
		t.Fatal(err)
	}

	empty := filepath.Join(unborn, "empty")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := scanForeign(unborn); len(got) != 1 || got[0] != "empty/" {
		t.Fatalf("scanForeign = %v", got)
	}
}

func TestCoverage90AcquisitionCacheRemovalFailureIsReported(t *testing.T) {
	target := t.TempDir()
	dir := filepath.Join(target, ".pyxis", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := errors.New("remove failed")
	old := installerRemoveAll
	installerRemoveAll = func(string) error { return want }
	t.Cleanup(func() { installerRemoveAll = old })
	report := &RollbackReport{Restored: true}
	clearAcquisitionCache(target, report)
	if report.Restored || len(report.Errors) != 1 || !strings.Contains(report.Errors[0], want.Error()) {
		t.Fatalf("rollback report = %+v", report)
	}
}
