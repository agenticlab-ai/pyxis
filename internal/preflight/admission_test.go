package preflight_test

// The admission fixture matrix (design init-hardening-v1 §3.1 check 1;
// DINV-5; plan init-hardening/001 T1's acceptance criteria): across the
// canonical shapes — empty dir, empty repo, repo+content, non-git+content,
// dirty repo — exactly the two empty shapes pass; every refusal names the
// adoption route. installer_test.go separately proves the full Init
// pipeline leaves a refused target byte-identical; this file exercises the
// probe directly, in-process, per the design's §3.10 testing strategy.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
)

// isolateGit neutralizes ambient git config so status/ls-files output — and
// hence the admission verdict — never depends on the machine running the
// test: no global excludes, no global identity, no system config.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", t.TempDir())
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errOut.String())
	}
	return out.String()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// admission runs the registry and returns just the admission outcome (the
// only one registered in T1) alongside Run's error.
func admission(t *testing.T, target string) (preflight.Outcome, error) {
	t.Helper()
	rep, err := preflight.Run(context.Background(), preflight.Input{Target: target})
	for _, out := range rep {
		if out.Check == preflight.AdmissionID {
			return out, err
		}
	}
	t.Fatalf("Report carries no admission outcome: %+v (err=%v)", rep, err)
	return preflight.Outcome{}, err
}

func TestAdmission(t *testing.T) {
	branch := config.Defaults().Branch // "main" — the one place this literal lives (DINV-6)

	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)

		wantPass    bool
		wantGitInit bool
		wantBranch  string // checked only when wantPass and non-empty

		wantConflicts      []string
		wantRouteAdopt     bool
		wantRemedyContains []string
		wantRemedyExcludes []string
	}{
		{
			name:        "empty directory",
			setup:       func(t *testing.T, dir string) {},
			wantPass:    true,
			wantGitInit: true,
			wantBranch:  branch,
		},
		{
			name:        "metadata-only repository",
			setup:       func(t *testing.T, dir string) { git(t, dir, "init", "-b", "trunk") },
			wantPass:    true,
			wantGitInit: false,
			wantBranch:  "trunk", // the repo's ACTUAL branch, not the config default
		},
		{
			name: "directory containing only init diagnostics",
			setup: func(t *testing.T, dir string) {
				logs := filepath.Join(dir, ".pyxis", "logs", "cli")
				if err := os.MkdirAll(logs, 0o700); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(logs, "init.jsonl"), "diagnostic\n")
			},
			wantPass: true, wantGitInit: true, wantBranch: branch,
		},
		{
			name: "repository containing only init diagnostics",
			setup: func(t *testing.T, dir string) {
				git(t, dir, "init", "-b", branch)
				logs := filepath.Join(dir, ".pyxis", "logs", "cli")
				if err := os.MkdirAll(logs, 0o700); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(logs, "init.jsonl"), "diagnostic\n")
			},
			wantPass: true, wantGitInit: false, wantBranch: branch,
		},
		{
			name: "content-holding repository",
			setup: func(t *testing.T, dir string) {
				git(t, dir, "init", "-b", branch)
				writeFile(t, filepath.Join(dir, "README.md"), "existing project\n")
				git(t, dir, "add", "--", "README.md")
				git(t, dir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "seed")
			},
			wantConflicts:      []string{"README.md"},
			wantRouteAdopt:     true,
			wantRemedyContains: []string{"pyxis adopt"},
			wantRemedyExcludes: []string{"git init"}, // a repo already exists — no two-step here
		},
		{
			name: "dirty worktree",
			setup: func(t *testing.T, dir string) {
				git(t, dir, "init", "-b", branch)
				writeFile(t, filepath.Join(dir, "scratch.txt"), "wip\n") // untracked → dirty porcelain
			},
			wantConflicts:      []string{"scratch.txt"},
			wantRouteAdopt:     true,
			wantRemedyContains: []string{"pyxis adopt"},
		},
		{
			name: "non-git target with content",
			setup: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, "README.md"), "existing project\n")
			},
			wantConflicts:      []string{"README.md"},
			wantRouteAdopt:     true,
			wantRemedyContains: []string{"git init -b " + branch, "pyxis adopt"},
		},
		{
			name: "non-git target holding only an empty subdirectory",
			// Strict emptiness (plan T1): any entry at all is content,
			// including a directory that itself holds nothing.
			setup: func(t *testing.T, dir string) {
				if err := os.Mkdir(filepath.Join(dir, "empty-sub"), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			},
			wantConflicts:  []string{"empty-sub/"}, // directories are suffixed
			wantRouteAdopt: true,
		},
		{
			name: "OS droppings alone",
			// design §3.1: "OS droppings are content and appear in the
			// conflict list" — a bare directory holding only a stray
			// .DS_Store still refuses.
			setup: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, ".DS_Store"), "\x00\x01")
			},
			wantConflicts:  []string{".DS_Store"},
			wantRouteAdopt: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateGit(t)
			target := t.TempDir()
			tc.setup(t, target)

			out, err := admission(t, target)

			if tc.wantPass {
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
				if out.Status != preflight.StatusPass {
					t.Fatalf("outcome = %+v, want pass", out)
				}
				if out.GitInit != tc.wantGitInit {
					t.Errorf("GitInit = %v, want %v", out.GitInit, tc.wantGitInit)
				}
				if out.Branch != tc.wantBranch {
					t.Errorf("Branch = %q, want %q", out.Branch, tc.wantBranch)
				}
				return
			}

			if err == nil {
				t.Fatal("Run = nil error, want a refusal")
			}
			if out.Status != preflight.StatusRefuse {
				t.Fatalf("outcome = %+v, want refuse", out)
			}
			if !slices.Equal(out.Conflicts, tc.wantConflicts) {
				t.Errorf("Conflicts = %v, want %v", out.Conflicts, tc.wantConflicts)
			}
			if tc.wantRouteAdopt && out.Route != "pyxis adopt" {
				t.Errorf("Route = %q, want %q", out.Route, "pyxis adopt")
			}
			for _, want := range tc.wantRemedyContains {
				if !strings.Contains(out.Remedy, want) {
					t.Errorf("Remedy = %q, want it to contain %q", out.Remedy, want)
				}
			}
			for _, exclude := range tc.wantRemedyExcludes {
				if strings.Contains(out.Remedy, exclude) {
					t.Errorf("Remedy = %q, must not contain %q", out.Remedy, exclude)
				}
			}
			var refusal *preflight.RefusalError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v (%T), want *preflight.RefusalError", err, err)
			}
			if !errors.Is(err, preflight.ErrRefused) {
				t.Error("errors.Is(err, preflight.ErrRefused) = false, want true")
			}
		})
	}
}

// TestGitRepoConflictsDedupedAcrossTrackedAndDirty proves conflictPaths'
// merge: a file that is BOTH tracked (ls-files) and modified (dirty
// porcelain) must be named exactly once in the refusal.
func TestGitRepoConflictsDedupedAcrossTrackedAndDirty(t *testing.T) {
	isolateGit(t)
	target := t.TempDir()
	git(t, target, "init", "-b", "main")
	writeFile(t, filepath.Join(target, "a.txt"), "v1\n")
	git(t, target, "add", "--", "a.txt")
	git(t, target, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "seed")
	// a.txt is now tracked AND modified (dirty); b.txt is untracked only.
	writeFile(t, filepath.Join(target, "a.txt"), "v2\n")
	writeFile(t, filepath.Join(target, "b.txt"), "new\n")

	out, err := admission(t, target)
	if err == nil {
		t.Fatal("Run = nil error, want a refusal")
	}
	if want := []string{"a.txt", "b.txt"}; !slices.Equal(out.Conflicts, want) {
		t.Errorf("Conflicts = %v, want %v (a.txt deduped despite appearing in both tracked and dirty)", out.Conflicts, want)
	}
}
