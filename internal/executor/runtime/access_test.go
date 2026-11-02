package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func accessGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func accessFixture(t *testing.T, linked bool) (CommandAccess, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "parent")
	state := filepath.Join(root, "runtime")
	for _, dir := range []string{repo, state, filepath.Join(state, "tmp")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	accessGit(t, repo, "init", "-q", "-b", "main")
	accessGit(t, repo, "-c", "user.name=Boundary Test", "-c", "user.email=boundary@example.invalid", "commit", "--allow-empty", "-qm", "base")
	work := repo
	if linked {
		work = filepath.Join(root, "task")
		accessGit(t, repo, "worktree", "add", "-q", "-b", "task", work)
	}
	return CommandAccess{Purpose: port.LaunchPurposeImplementation, Workdir: work, RuntimeStateDir: state}, repo
}

func TestLaunchAccessRoles(t *testing.T) {
	for _, purpose := range []port.LaunchPurpose{"", port.LaunchPurposeImplementation, port.LaunchPurposeReview} {
		got := LaunchAccess(port.LaunchSpec{Purpose: purpose, Workdir: "/task", RuntimeStateDir: "/state"})
		if got == nil {
			t.Fatalf("purpose %q: access = %+v", purpose, got)
		}
	}
}

func TestResolveAccessRejectsUnsafeRoots(t *testing.T) {
	base, parent := accessFixture(t, true)
	for _, tc := range []struct {
		name string
		edit func(*CommandAccess)
	}{
		{"missing state", func(p *CommandAccess) { p.RuntimeStateDir = "" }},
		{"shared root", func(p *CommandAccess) { p.RuntimeStateDir = p.Workdir }},
		{"ancestor state", func(p *CommandAccess) { p.RuntimeStateDir = filepath.Dir(p.Workdir) }},
		{"not a repo", func(p *CommandAccess) { p.Workdir = t.TempDir() }},
		{"unknown purpose", func(p *CommandAccess) { p.Purpose = "unrestricted" }},
		{"relative root", func(p *CommandAccess) { p.Workdir = "." }},
		{"filesystem root", func(p *CommandAccess) { p.Workdir = "/" }},
		{"metadata as scratch", func(p *CommandAccess) { p.RuntimeStateDir = filepath.Join(parent, ".git") }},
		{"missing root", func(p *CommandAccess) { p.Workdir = filepath.Join(p.Workdir, "missing") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.edit(&p)
			if _, err := resolveAccessPaths(t.Context(), p); err == nil {
				t.Fatal("unsafe policy accepted")
			}
		})
	}
}

func TestGitfileSymlinkCannotRedirectPolicy(t *testing.T) {
	p, parent := accessFixture(t, true)
	gitfile := filepath.Join(p.Workdir, ".git")
	if err := os.Remove(gitfile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, ".git"), gitfile); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAccessPaths(t.Context(), p); err == nil {
		t.Fatal("symlink Git entry accepted")
	}
}

func TestConfinementRejectsInvalidGitMetadataAndTemporaryStorage(t *testing.T) {
	for _, kind := range []string{"invalid gitfile", "different worktree root", "blocked temporary directory"} {
		t.Run(kind, func(t *testing.T) {
			policy, parent := accessFixture(t, kind != "different worktree root")
			switch kind {
			case "invalid gitfile":
				if err := os.WriteFile(filepath.Join(policy.Workdir, ".git"), []byte("not a gitfile\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "different worktree root":
				accessGit(t, parent, "config", "core.worktree", t.TempDir())
			case "blocked temporary directory":
				tmp := filepath.Join(policy.RuntimeStateDir, "tmp")
				if err := os.Remove(tmp); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(tmp, []byte("occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			payload := exec.Command("never-launch-this-model")
			if err := ConfineCommand(t.Context(), payload, &policy); err == nil || payload.Process != nil {
				t.Fatalf("invalid storage must fail before payload launch: %v", err)
			}
		})
	}
}

func TestConfinementSetupFailuresNeverStartPayload(t *testing.T) {
	policy, _ := accessFixture(t, true)
	for _, setupError := range []bool{false, true} {
		payload := exec.Command("never-launch-this-model")
		wrappedPayload := false
		wrap := func(cmd *exec.Cmd, _ accessPaths) error {
			if cmd == payload {
				wrappedPayload = true
				return nil
			}
			if setupError {
				return ErrAccessUnsupported
			}
			// Model the kernel refusing the sandbox. This is a Go subprocess,
			// not a model call, and its diagnostic also exercises output clamping.
			cmd.Path = os.Args[0]
			cmd.Args = []string{os.Args[0], "-test.run=^TestAccessProbe$"}
			cmd.Env = append(os.Environ(), "PYXIS_ACCESS_PROBE="+strings.Repeat("invalid", 300))
			return nil
		}
		err := confineCommand(t.Context(), payload, &policy, wrap)
		if !errors.Is(err, ErrAccessUnsupported) || wrappedPayload || payload.Process != nil {
			t.Fatalf("setup error=%v: err=%v wrapped=%v", setupError, err, wrappedPayload)
		}
	}
	cmd := exec.Command("authoring")
	if err := ConfineCommand(t.Context(), cmd, nil); err != nil || cmd.Path != "authoring" {
		t.Fatalf("authoring command was altered: %v", err)
	}
	if err := ConfineCommand(t.Context(), cmd, &CommandAccess{}); err == nil {
		t.Fatal("empty access policy accepted")
	}
}

// This runs the actual OS wrapper without a model or fake commander. Native
// harness argument tests separately establish that every adapter uses it.
func TestProcessAccessEnforcesGitOwnership(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprintf("linked=%v", linked), func(t *testing.T) {
			policy, parent := accessFixture(t, linked)
			paths, err := resolveAccessPaths(t.Context(), policy)
			if err != nil {
				t.Fatal(err)
			}
			head := accessGit(t, parent, "rev-parse", "HEAD")
			config, err := os.ReadFile(filepath.Join(parent, ".git", "config"))
			if err != nil {
				t.Fatal(err)
			}
			run := func(purpose port.LaunchPurpose, op, target string) ([]byte, error) {
				p := policy
				p.Purpose = purpose
				cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestAccessProbe$")
				cmd.Dir = p.Workdir
				cmd.Env = append(LaunchEnvironment(port.LaunchSpec{Purpose: purpose, Workdir: p.Workdir, RuntimeStateDir: p.RuntimeStateDir}),
					"PYXIS_ACCESS_PROBE="+op, "PYXIS_ACCESS_TARGET="+target)
				if err := ConfineCommand(t.Context(), cmd, &p); err != nil {
					return nil, err
				}
				return cmd.CombinedOutput()
			}
			if out, err := run(policy.Purpose, "read", ""); err != nil {
				unavailable := errors.Is(err, ErrAccessUnsupported) || strings.Contains(string(out), "sandbox_apply: Operation not permitted") || strings.Contains(string(out), "bwrap: Creating new namespace failed")
				if unavailable && os.Getenv("PYXIS_REQUIRE_PROCESS_CONFINEMENT") != "1" {
					t.Skipf("OS boundary cannot start on this runner (%s): %v: %s", goruntime.GOOS, err, out)
				}
				t.Fatalf("required confinement unavailable: %v: %s", err, out)
			}
			for _, tc := range []struct {
				name, op, target string
				allowed          bool
			}{
				{"source edit", "write", filepath.Join(paths.work, "change.txt"), true},
				{"scratch edit", "write", filepath.Join(paths.state, "tmp", "scratch"), true},
				{"stage", "git-add", "", false},
				{"commit", "git-commit", "", false},
				{"reference", "git-ref", "", false},
				{"configuration", "git-config", "", false},
				{"raw metadata", "write", filepath.Join(parent, ".git", "config"), false},
				{"absolute index", "write", filepath.Join(paths.protected[1], "index"), false},
				{"parent write", "write", filepath.Join(filepath.Dir(paths.work), "escape"), false},
				{"symlink metadata", "symlink-write", filepath.Join(parent, ".git", "config"), false},
				{"hardlink metadata", "hardlink-write", filepath.Join(parent, ".git", "config"), false},
				{"descendant metadata", "child-write", filepath.Join(parent, ".git", "config"), false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out, err := run(policy.Purpose, tc.op, tc.target)
					if (err == nil) != tc.allowed {
						t.Fatalf("allowed=%v, error=%v, output=%s", tc.allowed, err, out)
					}
				})
			}
			if linked {
				if out, err := run(policy.Purpose, "write", filepath.Join(paths.work, ".git")); err == nil {
					t.Fatalf("gitfile replacement succeeded: %s", out)
				}
			}
			source := filepath.Join(paths.work, "review-source.txt")
			original := []byte("exact reviewed source\n")
			if err := os.WriteFile(source, original, 0o600); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(paths.state, "tmp", "source-alias")
			if err := os.Symlink(source, alias); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name, op, target string
				allowed          bool
			}{
				{"review read", "file-read", source, true},
				{"review Git read", "read", "", true},
				{"review overwrite", "write", source, false},
				{"review create", "write", filepath.Join(paths.work, "review-created"), false},
				{"review delete", "remove", source, false},
				{"review rename", "rename", source, false},
				{"review chmod", "chmod", source, false},
				{"review descendant write", "child-write", source, false},
				{"review shell write", "shell-write", source, false},
				{"review scratch symlink escape", "write", alias, false},
				{"review scratch hardlink escape", "hardlink-scratch-write", source, false},
				{"review scratch", "write", filepath.Join(paths.state, "tmp", "review-scratch"), true},
				{"review Git ref", "git-ref", "", false},
				{"review Git config", "git-config", "", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					out, err := run(port.LaunchPurposeReview, tc.op, tc.target)
					if (err == nil) != tc.allowed {
						t.Fatalf("allowed=%v, error=%v, output=%s", tc.allowed, err, out)
					}
					if got, err := os.ReadFile(source); err != nil || string(got) != string(original) {
						t.Fatalf("review changed source: %v: %q", err, got)
					}
				})
			}
			if got := accessGit(t, parent, "rev-parse", "HEAD"); got != head {
				t.Fatal("child moved HEAD")
			}
			if got, err := os.ReadFile(filepath.Join(parent, ".git", "config")); err != nil || string(got) != string(config) {
				t.Fatalf("child changed protected config: %v", err)
			}
			// The supervisor retains write authority after the confined child exits.
			accessGit(t, policy.Workdir, "add", "--", "change.txt")
			accessGit(t, policy.Workdir, "-c", "user.name=Boundary Test", "-c", "user.email=boundary@example.invalid", "commit", "-qm", "supervisor checkpoint")
		})
	}
}

// The write boundary must still permit both roles' actual compiler,
// test subprocesses, and disposable build cache, without a network dependency.
func TestProcessAccessAllowsGoChecks(t *testing.T) {
	policy, _ := accessFixture(t, false)
	for name, content := range map[string]string{
		"go.mod":         "module boundary.example/task\n\ngo 1.24\n",
		"answer_test.go": "package task\nimport \"testing\"\nfunc TestAnswer(t *testing.T) { if 2 + 3 != 5 { t.Fatal(\"wrong answer\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(policy.Workdir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, purpose := range []port.LaunchPurpose{port.LaunchPurposeImplementation, port.LaunchPurposeReview} {
		t.Run(string(purpose), func(t *testing.T) {
			policy.Purpose = purpose
			cmd := exec.CommandContext(t.Context(), "go", "test", "-count=1", "./...")
			cmd.Dir = policy.Workdir
			cmd.Env = append(LaunchEnvironment(port.LaunchSpec{Purpose: purpose, Workdir: policy.Workdir, RuntimeStateDir: policy.RuntimeStateDir}),
				"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOTELEMETRY=off")
			if err := ConfineCommand(t.Context(), cmd, &policy); err != nil {
				if errors.Is(err, ErrAccessUnsupported) && os.Getenv("PYXIS_REQUIRE_PROCESS_CONFINEMENT") != "1" {
					t.Skipf("OS boundary unavailable: %v", err)
				}
				t.Fatal(err)
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("confined Go checks failed: %v: %s", err, out)
			}
		})
	}
}

// TestAccessProbe is an inert Go subprocess except when its parent explicitly
// supplies the test operation. It exercises actual filesystem/Git syscalls.
func TestAccessProbe(t *testing.T) {
	op := os.Getenv("PYXIS_ACCESS_PROBE")
	if op == "" {
		return
	}
	target := os.Getenv("PYXIS_ACCESS_TARGET")
	var err error
	switch op {
	case "read", "git-add", "git-commit", "git-ref", "git-config":
		args := map[string][]string{
			"read":       {"--no-optional-locks", "status", "--porcelain"},
			"git-add":    {"add", "--", "change.txt"},
			"git-commit": {"-c", "user.name=Child", "-c", "user.email=child@example.invalid", "commit", "--allow-empty", "-qm", "forbidden"},
			"git-ref":    {"update-ref", "refs/heads/forbidden", "HEAD"},
			"git-config": {"config", "--local", "remote.forbidden.url", "file:///forbidden"},
		}[op]
		var out []byte
		out, err = exec.CommandContext(context.Background(), "git", args...).CombinedOutput()
		if err != nil {
			err = fmt.Errorf("%w: %s", err, out)
		}
	case "write":
		err = os.WriteFile(target, []byte("child change\n"), 0o600)
	case "file-read":
		_, err = os.ReadFile(target)
	case "remove":
		err = os.Remove(target)
	case "rename":
		err = os.Rename(target, target+"-renamed")
	case "chmod":
		err = os.Chmod(target, 0o777)
	case "shell-write":
		out, runErr := exec.Command("/bin/sh", "-c", `printf forbidden > "$1"`, "probe", target).CombinedOutput()
		if runErr != nil {
			err = fmt.Errorf("%w: %s", runErr, out)
		}
	case "symlink-write", "hardlink-write", "hardlink-scratch-write":
		alias := "alias-" + op
		if op == "hardlink-scratch-write" {
			alias = filepath.Join(os.Getenv("TMPDIR"), alias)
		}
		if op == "symlink-write" {
			err = os.Symlink(target, alias)
		} else {
			err = os.Link(target, alias)
		}
		if err == nil {
			err = os.WriteFile(alias, []byte("forbidden\n"), 0o600)
		}
	case "child-write":
		cmd := exec.Command(os.Args[0], "-test.run=^TestAccessProbe$")
		cmd.Env = append(os.Environ(), "PYXIS_ACCESS_PROBE=write")
		err = cmd.Run()
	default:
		err = fmt.Errorf("unknown operation %q", op)
	}
	if err != nil {
		t.Fatal(err)
	}
}
