package invoke

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// TestParseInvocation_ValidInvocations tests successful parsing.
func TestParseInvocation_ValidInvocations(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectWS    string
		expectDB    string
		expectIncr  bool
		expectError bool
	}{
		{
			name:        "basic run",
			args:        []string{"--workspace", "/tmp/ws", "--diff-base", "main"},
			expectWS:    "/tmp/ws",
			expectDB:    "main",
			expectIncr:  false,
			expectError: false,
		},
		{
			name:        "with incremental",
			args:        []string{"--workspace", "/tmp/ws", "--diff-base", "main", "--incremental"},
			expectWS:    "/tmp/ws",
			expectDB:    "main",
			expectIncr:  true,
			expectError: false,
		},
		{
			name:        "flags in any order",
			args:        []string{"--diff-base", "main", "--workspace", "/tmp/ws"},
			expectWS:    "/tmp/ws",
			expectDB:    "main",
			expectIncr:  false,
			expectError: false,
		},
		{
			name:        "with equals syntax",
			args:        []string{"--workspace=/tmp/ws", "--diff-base=main"},
			expectWS:    "/tmp/ws",
			expectDB:    "main",
			expectIncr:  false,
			expectError: false,
		},
		{
			name:        "mixed flag syntax",
			args:        []string{"--workspace=/tmp/ws", "--diff-base", "main"},
			expectWS:    "/tmp/ws",
			expectDB:    "main",
			expectIncr:  false,
			expectError: false,
		},
		{
			name:        "complex paths",
			args:        []string{"--workspace", "/home/user/.local/ws", "--diff-base", "refs/heads/feature"},
			expectWS:    "/home/user/.local/ws",
			expectDB:    "refs/heads/feature",
			expectIncr:  false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, cerr := ParseInvocation(tt.args)
			if tt.expectError {
				if cerr == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if cerr != nil {
				t.Errorf("unexpected error: %v", cerr.Message)
			}

			if inv.Workspace != tt.expectWS {
				t.Errorf("workspace mismatch: got %q, want %q", inv.Workspace, tt.expectWS)
			}

			if inv.DiffBase != tt.expectDB {
				t.Errorf("diff-base mismatch: got %q, want %q", inv.DiffBase, tt.expectDB)
			}

			if inv.Incremental != tt.expectIncr {
				t.Errorf("incremental mismatch: got %v, want %v", inv.Incremental, tt.expectIncr)
			}
		})
	}
}

// TestParseInvocation_Refusals tests all refusal classes (exit 2).
func TestParseInvocation_Refusals(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		expectStdoutOnly bool
	}{
		{
			name:             "missing --workspace",
			args:             []string{"--diff-base", "main"},
			expectStdoutOnly: true,
		},
		{
			name:             "missing --diff-base",
			args:             []string{"--workspace", "/tmp/ws"},
			expectStdoutOnly: true,
		},
		{
			name:             "unknown flag",
			args:             []string{"--workspace", "/tmp/ws", "--diff-base", "main", "--unknown"},
			expectStdoutOnly: true,
		},
		{
			name:             "flag-like positional",
			args:             []string{"--workspace", "/tmp/ws", "--diff-base", "main", "extra"},
			expectStdoutOnly: true,
		},
		{
			name:             "flag without value",
			args:             []string{"--workspace"},
			expectStdoutOnly: true,
		},
		{
			name:             "flag with flag-like value",
			args:             []string{"--workspace", "--diff-base", "main"},
			expectStdoutOnly: true,
		},
		{
			name:             "duplicate --workspace",
			args:             []string{"--workspace", "/tmp/ws", "--workspace", "/tmp/ws2", "--diff-base", "main"},
			expectStdoutOnly: true,
		},
		{
			name:             "duplicate --diff-base",
			args:             []string{"--workspace", "/tmp/ws", "--diff-base", "main", "--diff-base", "other"},
			expectStdoutOnly: true,
		},
		{
			name:             "--incremental with value",
			args:             []string{"--workspace", "/tmp/ws", "--diff-base", "main", "--incremental=true"},
			expectStdoutOnly: true,
		},
		{
			name:             "empty workspace value",
			args:             []string{"--workspace", "", "--diff-base", "main"},
			expectStdoutOnly: true,
		},
		{
			name:             "empty diff-base value",
			args:             []string{"--workspace", "/tmp/ws", "--diff-base", ""},
			expectStdoutOnly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv, cerr := ParseInvocation(tt.args)
			if inv != nil {
				t.Errorf("expected error, got invocation: %+v", inv)
				return
			}

			if cerr == nil {
				t.Errorf("expected ConfigError, got nil")
				return
			}

			if cerr.StdoutOnly != tt.expectStdoutOnly {
				t.Errorf("StdoutOnly mismatch: got %v, want %v\n  message: %s", cerr.StdoutOnly, tt.expectStdoutOnly, cerr.Message)
			}
		})
	}
}

// TestResolveBindings tests binding resolution and failure modes.
func TestResolveBindings_GitValidation(t *testing.T) {
	// Create a temporary git repository for testing.
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Test successful resolution.
	t.Run("valid clean repo", func(t *testing.T) {
		// Create .pyxis/codegate.json with minimal valid manifest.
		pyxisDir := filepath.Join(tmpDir, ".pyxis")
		os.Mkdir(pyxisDir, 0755)

		manifest := `{"version":"1","stack":"go","verbs":{}}`
		manifestPath := filepath.Join(pyxisDir, "codegate.json")
		os.WriteFile(manifestPath, []byte(manifest), 0644)

		// Add .pyxis to .git/info/exclude to keep tree clean.
		gitinfoDir := filepath.Join(tmpDir, ".git", "info")
		os.MkdirAll(gitinfoDir, 0755)
		excludePath := filepath.Join(gitinfoDir, "exclude")
		os.WriteFile(excludePath, []byte(".pyxis/\n"), 0644)

		bindings, cerr := ResolveBindings(context.Background(), tmpDir, "HEAD")
		if cerr != nil {
			t.Errorf("unexpected error: %v", cerr.Message)
			return
		}

		if bindings.HeadSHA == "" {
			t.Error("HeadSHA is empty")
		}

		if bindings.ManifestHash == "" {
			t.Error("ManifestHash is empty")
		}

		if bindings.CodegateVersion != CodegateVersion {
			t.Errorf("CodegateVersion mismatch: got %q, want %q", bindings.CodegateVersion, CodegateVersion)
		}

		if bindings.GitVersion == "" {
			t.Error("GitVersion is empty")
		}
	})

	// Test missing manifest.
	t.Run("missing manifest", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		initGitRepo(t, tmpDir2)

		bindings, cerr := ResolveBindings(context.Background(), tmpDir2, "HEAD")
		if bindings != nil {
			t.Errorf("expected error, got bindings: %+v", bindings)
			return
		}

		if cerr == nil {
			t.Error("expected ConfigError, got nil")
			return
		}

		if cerr.StdoutOnly {
			t.Error("manifest-not-found should not be stdout-only")
		}
	})

	// Test malformed manifest.
	t.Run("malformed manifest", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		initGitRepo(t, tmpDir2)

		pyxisDir := filepath.Join(tmpDir2, ".pyxis")
		os.Mkdir(pyxisDir, 0755)

		// Write invalid JSON
		manifestPath := filepath.Join(pyxisDir, "codegate.json")
		os.WriteFile(manifestPath, []byte(`{invalid json}`), 0644)

		bindings, cerr := ResolveBindings(context.Background(), tmpDir2, "HEAD")
		if bindings != nil {
			t.Errorf("expected error, got bindings: %+v", bindings)
			return
		}

		if cerr == nil {
			t.Error("expected ConfigError, got nil")
			return
		}

		if cerr.StdoutOnly {
			t.Error("manifest-malformed should not be stdout-only")
		}
	})
}

// TestResolveBindings_DirtyTree tests the clean-tree assertion.
func TestResolveBindings_DirtyTree(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	// Create a valid manifest.
	pyxisDir := filepath.Join(tmpDir, ".pyxis")
	os.Mkdir(pyxisDir, 0755)
	manifest := `{"version":"1","stack":"go","verbs":{}}`
	manifestPath := filepath.Join(pyxisDir, "codegate.json")
	os.WriteFile(manifestPath, []byte(manifest), 0644)

	// Add an untracked file to make the tree dirty.
	dummyFile := filepath.Join(tmpDir, "untracked.txt")
	os.WriteFile(dummyFile, []byte("untracked"), 0644)

	bindings, cerr := ResolveBindings(context.Background(), tmpDir, "HEAD")
	if bindings != nil {
		t.Errorf("expected error for dirty tree, got bindings: %+v", bindings)
		return
	}

	if cerr == nil {
		t.Error("expected ConfigError for dirty tree, got nil")
		return
	}

	if cerr.StdoutOnly {
		t.Error("dirty-tree error should not be stdout-only")
	}
}

// TestWorkspaceLock tests flock-based locking.
func TestWorkspaceLock_SingleLock(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	lock1, err := AcquireWorkspaceLock(tmpDir)
	if err != nil {
		t.Errorf("failed to acquire first lock: %v", err.Message)
		return
	}
	defer lock1.Release()

	// The lock should be held; verify we can't acquire a second one.
	lock2, err := AcquireWorkspaceLock(tmpDir)
	if lock2 != nil {
		lock2.Release()
		t.Error("expected second lock to fail while first is held")
		return
	}

	if err == nil {
		t.Error("expected ConfigError for busy workspace, got nil")
		return
	}

	// Check that it's a lock-busy error.
	if err.Message != "workspace is already locked by another invocation" {
		t.Logf("got error message: %s (may indicate lock is busy)", err.Message)
		// Still pass if it looks like a busy error
		if !isLockBusyMsg(err.Message) {
			t.Errorf("unexpected error message: %s", err.Message)
		}
	}
}

func isLockBusyMsg(msg string) bool {
	return strings.Contains(msg, "locked") || strings.Contains(msg, "temporarily unavailable")
}

// TestWorkspaceLock_Release tests lock release.
func TestWorkspaceLock_Release(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	lock1, err := AcquireWorkspaceLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err.Message)
	}

	// Release the lock.
	if releaseErr := lock1.Release(); releaseErr != nil {
		t.Fatalf("failed to release lock: %v", releaseErr)
	}

	// Now acquiring a second lock should succeed.
	lock2, err := AcquireWorkspaceLock(tmpDir)
	if err != nil {
		t.Errorf("failed to acquire second lock after release: %v", err.Message)
		return
	}
	defer lock2.Release()
}

// Test helpers

// initGitRepo builds a hermetic git repo at path with one commit — using
// `git -C` (never os.Chdir, which races the shared process cwd) and an isolated
// git identity/config so the host environment cannot leak in.
func initGitRepo(t *testing.T, path string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	gitOK(t, path, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(path, "test.txt"), []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitOK(t, path, "add", "test.txt")
	gitOK(t, path, "commit", "-q", "-m", "initial commit")
}

func gitOK(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// --- #404 review regression tests ---------------------------------------------

// The git-version floor must compare NUMERICALLY: git 2.10–2.49 (every common
// distro/CI git) must pass the 2.5.0 floor. The old lexical `>=` rejected them.
func TestGitVersionCompare(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"2.5.0", true}, {"2.5.1", true}, {"2.6.0", true},
		{"2.10.0", true}, {"2.34.1", true}, {"2.39.3", true}, {"2.43.0", true}, {"2.50.1", true},
		{"2.39.3 (Apple Git-146)", true}, // git reports suffixes on some platforms
		{"2.4.9", false}, {"2.0.0", false}, {"1.9.9", false},
	}
	for _, c := range cases {
		if got := isGitVersionAtLeast(c.v, gitFloorVersion); got != c.want {
			t.Errorf("isGitVersionAtLeast(%q, %q) = %v, want %v", c.v, gitFloorVersion, got, c.want)
		}
	}
}

// A flag-like --diff-base value must be refused in the `=` form too, so it can't
// inject git options (e.g. `git rev-parse --all`).
func TestParseInvocation_RejectsEqualsFormFlagValue(t *testing.T) {
	for _, args := range [][]string{
		{"--workspace", "/tmp/ws", "--diff-base=--all"},
		{"--workspace=--evil", "--diff-base", "main"},
	} {
		if inv, cerr := ParseInvocation(args); cerr == nil {
			t.Errorf("ParseInvocation(%v) accepted a flag-like =value: %+v", args, inv)
		}
	}
}

// Emit's artifact path was previously untested. With an injected clock it writes
// a deterministic, invariant-valid config-error verdict.
func TestEmit_WritesArtifact(t *testing.T) {
	orig := nowFunc
	nowFunc = func() time.Time { return time.Unix(1000, 0).UTC() }
	defer func() { nowFunc = orig }()

	dir := t.TempDir()
	ce := &ConfigError{Message: "manifest is malformed", Workspace: dir, Code: ExitConfigErr}
	if err := ce.Emit(); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".codegate", "verdict.json"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var v schema.Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}
	if v.State != "error" {
		t.Errorf("state = %q, want error", v.State)
	}
	if len(v.Findings) != 1 || v.Findings[0].Message != "manifest is malformed" {
		t.Errorf("findings = %+v, want the config-error message", v.Findings)
	}
	// No temp residue left behind.
	entries, _ := os.ReadDir(filepath.Join(dir, ".codegate"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".codegate-verdict-") {
			t.Errorf("temp residue left behind: %s", e.Name())
		}
	}
}
