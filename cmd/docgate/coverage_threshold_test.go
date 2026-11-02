package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

func initializeDocgateGit(t *testing.T, root string) string {
	t.Helper()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@pyxis.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@pyxis.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	run("add", ".pyxis/config.yaml")
	run("commit", "-qm", "seed")
	return run("rev-parse", "HEAD")
}

func withDocgateStdin(t *testing.T, input string, fn func() int) int {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, input); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = old
		_ = r.Close()
	}()
	return fn()
}

func TestRunRepositoryDiscoveryAndConfigurationFailures(t *testing.T) {
	t.Run("flag shaped positional", func(t *testing.T) {
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold", "-"}) })
		if code != 2 || !strings.Contains(stderr, "flag") {
			t.Fatalf("run = %d stderr=%q", code, stderr)
		}
	})

	t.Run("repository root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold"}) })
		if code != 2 || stderr == "" {
			t.Fatalf("run = %d stderr=%q", code, stderr)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		root := docgateRoot(t)
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("unknown_key: true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold"}) })
		if code != 1 || stderr == "" {
			t.Fatalf("run = %d stderr=%q", code, stderr)
		}
	})

	t.Run("ledger root is a file", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "prds"), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold"}) })
		if code != 2 || stderr == "" {
			t.Fatalf("run = %d stderr=%q", code, stderr)
		}
	})
}

func TestRunMergeBaseGateAndPrePushPaths(t *testing.T) {
	t.Run("merge base failure", func(t *testing.T) {
		root := docgateRoot(t)
		t.Chdir(root)
		code, _, stderr := captureDocgateOutput(t, func() int {
			return run([]string{"gate", "--merge-base", "missing-ref"})
		})
		if code != 1 || stderr == "" {
			t.Fatalf("run = %d stderr=%q", code, stderr)
		}
	})

	root := docgateRoot(t)
	head := initializeDocgateGit(t, root)
	t.Chdir(root)
	code, _, _ := captureDocgateOutput(t, func() int { return run([]string{"gate", "--merge-base", "HEAD"}) })
	if code == 2 {
		t.Fatal("gate failed at environment/usage boundary")
	}

	code, _, _ = captureDocgateOutput(t, func() int {
		return withDocgateStdin(t, "", func() int { return run([]string{"pre-push", "--merge-base", "HEAD"}) })
	})
	if code == 2 {
		t.Fatal("empty-stdin pre-push failed at environment/usage boundary")
	}

	pushLine := "refs/heads/main " + head + " refs/heads/main " + head + "\n"
	code, _, _ = captureDocgateOutput(t, func() int {
		return withDocgateStdin(t, pushLine, func() int { return run([]string{"pre-push", "--merge-base", "HEAD"}) })
	})
	if code == 2 {
		t.Fatal("pushed-head pre-push failed at environment/usage boundary")
	}
}

func TestPrePushUsesSuppliedProposalBase(t *testing.T) {
	for _, tc := range []struct {
		name, env, wantRefError string
		flags                   []string
	}{
		{name: "supplied base", env: "HEAD"},
		{name: "unavailable supplied base", env: "missing-proposal-base", wantRefError: "missing-proposal-base"},
		{name: "explicit flag wins", env: "missing-proposal-base", flags: []string{"--merge-base", "HEAD"}},
		{name: "configured default", wantRefError: "origin/main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := docgateRoot(t)
			head := initializeDocgateGit(t, root)
			t.Chdir(root)
			t.Setenv("PYXIS_CI_BASE", tc.env)
			for _, input := range []string{"", "refs/heads/topic " + head + " refs/heads/topic " + head + "\n"} {
				code, _, stderr := captureDocgateOutput(t, func() int {
					return withDocgateStdin(t, input, func() int {
						return run(append([]string{"pre-push"}, tc.flags...))
					})
				})
				if tc.wantRefError != "" {
					if code != 1 || !strings.Contains(stderr, tc.wantRefError) {
						t.Fatalf("pre-push = %d, want unavailable %q: %s", code, tc.wantRefError, stderr)
					}
				} else if code == 2 || strings.Contains(stderr, "origin/main") || strings.Contains(stderr, "missing-proposal-base") {
					t.Fatalf("pre-push ignored selected base: code %d, %s", code, stderr)
				}
			}
		})
	}
}

func TestRunFoldAndIndexWriteFailures(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("..", "..", "internal", "docgate", "fold", "testdata", "fixture-zero", "000-base.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"effective.md", "effective.source.json"} {
		t.Run(target, func(t *testing.T) {
			root := docgateRoot(t)
			ledger := filepath.Join(root, "docs", "prds", "demo")
			if err := os.MkdirAll(ledger, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "000-base.md"), base, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(ledger, target), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"fold", ledger}) })
			if code != 2 || stderr == "" {
				t.Fatalf("run = %d stderr=%q", code, stderr)
			}
		})
	}

	for _, target := range []string{"index.md", "index.json"} {
		t.Run(target, func(t *testing.T) {
			root := docgateRoot(t)
			designDir := filepath.Join(root, "docs", "tech-design")
			if err := os.MkdirAll(designDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(designDir, target), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			code, _, stderr := captureDocgateOutput(t, func() int { return run([]string{"gen-index"}) })
			if code != 2 || stderr == "" {
				t.Fatalf("run = %d stderr=%q", code, stderr)
			}
		})
	}
}

func TestRunFeedbackRegisteredCheckAndContractReportFailures(t *testing.T) {
	root := docgateRoot(t)
	feedbackDir := filepath.Join(root, "docs", "feedback", "empty")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs", "tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	code, _, _ := captureDocgateOutput(t, func() int { return run([]string{"fold", feedbackDir}) })
	if code != 0 {
		t.Fatalf("empty feedback fold = %d", code)
	}

	registered := ""
	for _, check := range gate.Checks() {
		if !check.NeedsMergeBase {
			registered = check.Verb
			break
		}
	}
	if registered == "" {
		t.Fatal("no tree-local registered check")
	}
	code, _, _ = captureDocgateOutput(t, func() int { return run([]string{registered}) })
	if code == 2 {
		t.Fatalf("registered check %q failed at usage boundary", registered)
	}

	badSink := filepath.Join(root, "report-file")
	if err := os.WriteFile(badSink, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := captureDocgateOutput(t, func() int {
		return run([]string{"contract-gate", "demo", "--report-out", badSink})
	})
	if code != 2 || !strings.Contains(stderr, "emitting defect report") {
		t.Fatalf("contract report = %d stderr=%q", code, stderr)
	}
}

func TestContractSurfacesFailClosedOnInvalidConfiguration(t *testing.T) {
	root := docgateRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("unknown_key: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := captureDocgateOutput(t, func() int {
		return contractGateContinuous(root, "docs/prds/demo/000-base.md", "-")
	})
	if code != 1 || !strings.Contains(stderr, "configuration-error") {
		t.Fatalf("continuous = %d stderr=%q", code, stderr)
	}
	code, _, stderr = captureDocgateOutput(t, func() int { return contractGateSweep(root) })
	if code != 1 || !strings.Contains(stderr, "configuration-error") {
		t.Fatalf("sweep = %d stderr=%q", code, stderr)
	}
}
