package budget

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	helperModeEnv     = "PYXIS_BUDGET_TEST_HELPER_MODE"
	helperIdentityEnv = "PYXIS_BUDGET_TEST_IDENTITY"
	helperStartEnv    = "PYXIS_BUDGET_TEST_START"
	helperSignalEnv   = "PYXIS_BUDGET_TEST_SIGNAL"
	helperReleaseEnv  = "PYXIS_BUDGET_TEST_RELEASE"
	helperDoneEnv     = "PYXIS_BUDGET_TEST_DONE"
)

// TestBudgetSubprocessHelper is entered only by the subprocess fixtures below.
// It uses the production lock/write path; its only seam pauses at a named
// durability boundary so the parent can force overlap or kill the process.
func TestBudgetSubprocessHelper(t *testing.T) {
	mode := os.Getenv(helperModeEnv)
	if mode == "" {
		return
	}
	if start := os.Getenv(helperStartEnv); start != "" {
		waitForTestFile(t, start)
	}
	identity := os.Getenv(helperIdentityEnv)
	b, err := NewForIdentity("github.com", identity, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	switch mode {
	case "acquire":
		if signal := os.Getenv(helperSignalEnv); signal != "" {
			writeTestSignal(t, signal)
		}
	case "pause-after-rename":
		baseWrite := b.writeRecord
		paused := false
		b.writeRecord = func(path string, v any) error {
			if path != b.umbrellaPath || paused {
				return baseWrite(path, v)
			}
			paused = true
			ops := osDurableOps
			ops.rename = func(oldpath, newpath string) error {
				if err := os.Rename(oldpath, newpath); err != nil {
					return err
				}
				writeTestSignal(t, os.Getenv(helperSignalEnv))
				waitForTestFile(t, os.Getenv(helperReleaseEnv))
				return nil
			}
			return writeDurableWith(ops, path, v)
		}
	case "pause-after-umbrella-durable":
		baseWrite := b.writeRecord
		paused := false
		b.writeRecord = func(path string, v any) error {
			if err := baseWrite(path, v); err != nil {
				return err
			}
			if path == b.umbrellaPath && !paused {
				paused = true
				writeTestSignal(t, os.Getenv(helperSignalEnv))
				waitForTestFile(t, os.Getenv(helperReleaseEnv))
			}
			return nil
		}
	case "pause-before-marker":
		baseWrite := b.writeRecord
		b.writeRecord = func(path string, v any) error {
			if path == b.markerPath {
				writeTestSignal(t, os.Getenv(helperSignalEnv))
				waitForTestFile(t, os.Getenv(helperReleaseEnv))
			}
			return baseWrite(path, v)
		}
	case "pause-after-old-temp":
		baseWrite := b.writeRecord
		paused := false
		b.writeRecord = func(path string, v any) error {
			if path != b.umbrellaPath || paused {
				return baseWrite(path, v)
			}
			paused = true
			ops := osDurableOps
			ops.createTemp = func(dir, pattern string) (durableFile, error) {
				f, err := os.CreateTemp(dir, pattern)
				if err != nil {
					return nil, err
				}
				old := time.Now().Add(-2 * longestWindow)
				if err := os.Chtimes(f.Name(), old, old); err != nil {
					f.Close()
					return nil, err
				}
				writeTestSignal(t, os.Getenv(helperSignalEnv))
				waitForTestFile(t, os.Getenv(helperReleaseEnv))
				return f, nil
			}
			return writeDurableWith(ops, path, v)
		}
	case "pause-after-unlink":
		path, err := MarkerPath("github.com")
		if err != nil {
			t.Fatal(err)
		}
		ops := osDurableOps
		ops.remove = func(target string) error {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			if target == path {
				writeTestSignal(t, os.Getenv(helperSignalEnv))
				waitForTestFile(t, os.Getenv(helperReleaseEnv))
			}
			return nil
		}
		if err := removeDurableWith(ops, path); err != nil {
			t.Fatal(err)
		}
		return
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}

	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if done := os.Getenv(helperDoneEnv); done != "" {
		writeTestSignal(t, done)
	}
}

func TestMixedResolvedAndUnresolvedWritersShareUmbrellaAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	cutOver(t, "github.com")
	start := filepath.Join(root, "start")

	resolved, resolvedOut := budgetHelper(t, root, "acquire", "octocat", helperStartEnv+"="+start)
	unresolved, unresolvedOut := budgetHelper(t, root, "acquire", "", helperStartEnv+"="+start)
	if err := resolved.Start(); err != nil {
		t.Fatal(err)
	}
	if err := unresolved.Start(); err != nil {
		resolved.Process.Kill()
		t.Fatal(err)
	}
	writeTestSignal(t, start)
	waitBudgetHelper(t, resolved, resolvedOut)
	waitBudgetHelper(t, unresolved, unresolvedOut)

	umbrella, _ := HostPath("github.com")
	identity, _ := Path("github.com", "octocat")
	if got := len(readLedger(t, umbrella).Minute); got != 2 {
		t.Fatalf("umbrella holds %d writes, want both processes' writes", got)
	}
	if got := len(readLedger(t, identity).Minute); got != 1 {
		t.Fatalf("identity holds %d writes, want only the resolved process's write", got)
	}
}

func TestSidecarLockSerializesAcrossRenameInTwoProcesses(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	cutOver(t, "github.com")
	renamed := filepath.Join(root, "renamed")
	release := filepath.Join(root, "release")
	secondStarted := filepath.Join(root, "second-started")
	secondDone := filepath.Join(root, "second-done")

	first, firstOut := budgetHelper(t, root, "pause-after-rename", "octocat",
		helperSignalEnv+"="+renamed, helperReleaseEnv+"="+release)
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, renamed)

	second, secondOut := budgetHelper(t, root, "acquire", "octocat",
		helperSignalEnv+"="+secondStarted, helperDoneEnv+"="+secondDone)
	if err := second.Start(); err != nil {
		first.Process.Kill()
		t.Fatal(err)
	}
	waitForTestFile(t, secondStarted)
	if waitForTestFileWithin(secondDone, 200*time.Millisecond) {
		first.Process.Kill()
		second.Process.Kill()
		t.Fatal("second process admitted while the first still held the sidecar across rename")
	}
	writeTestSignal(t, release)
	waitBudgetHelper(t, first, firstOut)
	waitBudgetHelper(t, second, secondOut)

	umbrella, _ := HostPath("github.com")
	if got := len(readLedger(t, umbrella).Minute); got != 2 {
		t.Fatalf("umbrella holds %d writes, want both serialized admissions", got)
	}
}

func TestCrashAfterUmbrellaCommitNeverLeavesIdentityOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	cutOver(t, "github.com")
	signal := filepath.Join(root, "umbrella-durable")
	release := filepath.Join(root, "never-release")

	cmd, out := budgetHelper(t, root, "pause-after-umbrella-durable", "octocat",
		helperSignalEnv+"="+signal, helperReleaseEnv+"="+release)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, signal)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	umbrella, _ := HostPath("github.com")
	identity, _ := Path("github.com", "octocat")
	if got := len(readLedger(t, umbrella).Minute); got != 1 {
		t.Fatalf("umbrella after crash holds %d entries, want 1; helper output: %s", got, out.String())
	}
	if _, err := os.Stat(identity); !os.IsNotExist(err) {
		t.Fatalf("identity ledger exists after crash between commits: %v", err)
	}
}

func TestCrashDuringInitializationNeverLeavesMarkerOnly(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	signal := filepath.Join(root, "before-marker")
	release := filepath.Join(root, "never-release")

	cmd, out := budgetHelper(t, root, "pause-before-marker", "octocat",
		helperSignalEnv+"="+signal, helperReleaseEnv+"="+release)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, signal)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	umbrella, _ := HostPath("github.com")
	identity, _ := Path("github.com", "octocat")
	marker, _ := MarkerPath("github.com")
	if len(readLedger(t, umbrella).Minute) != GitHubContentWritesPerMinute || len(readLedger(t, identity).Minute) != GitHubContentWritesPerMinute {
		t.Fatalf("conservative ledgers were not durable before marker; helper output: %s", out.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("marker exists after crash before marker commit: %v", err)
	}
}

func TestInterruptedMarkerInvalidationMustBeRetriedToCompletion(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	cutOver(t, "github.com")
	signal := filepath.Join(root, "unlinked")
	release := filepath.Join(root, "never-release")

	cmd, _ := budgetHelper(t, root, "pause-after-unlink", "",
		helperSignalEnv+"="+signal, helperReleaseEnv+"="+release)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, signal)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	// The interrupted call never completed, even if the unlink is visible.
	// Retrying performs the directory fsync and is the only completed downgrade.
	if err := InvalidateMarker("github.com"); err != nil {
		t.Fatalf("retry invalidation: %v", err)
	}
	marker, _ := MarkerPath("github.com")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("marker survived completed retry: %v", err)
	}
}

func TestCrashRestartReapsOldStrandedTemp(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", root)
	cutOver(t, "github.com")
	signal := filepath.Join(root, "temp-created")
	release := filepath.Join(root, "never-release")

	cmd, _ := budgetHelper(t, root, "pause-after-old-temp", "octocat",
		helperSignalEnv+"="+signal, helperReleaseEnv+"="+release)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, signal)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	umbrella, _ := HostPath("github.com")
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(umbrella), tempPrefix+"*"))
	if err != nil || len(temps) != 1 {
		t.Fatalf("crash left temps %v (err %v), want one stranded temp", temps, err)
	}
	b, err := NewForIdentity("github.com", "octocat", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Acquire(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	temps, err = filepath.Glob(filepath.Join(filepath.Dir(umbrella), tempPrefix+"*"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("restart left old temps %v (err %v)", temps, err)
	}
}

func budgetHelper(t *testing.T, root, mode, identity string, extra ...string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBudgetSubprocessHelper$")
	cmd.Env = append(os.Environ(),
		"XDG_STATE_HOME="+root,
		helperModeEnv+"="+mode,
		helperIdentityEnv+"="+identity,
	)
	cmd.Env = append(cmd.Env, extra...)
	out := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = out, out
	return cmd, out
}

func waitBudgetHelper(t *testing.T, cmd *exec.Cmd, out *bytes.Buffer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("budget helper: %v\n%s", err, out.String())
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("budget helper timed out\n%s", out.String())
	}
}

func writeTestSignal(t *testing.T, path string) {
	t.Helper()
	if path == "" {
		t.Fatal("empty test signal path")
	}
	if err := os.WriteFile(path, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitForTestFile(t *testing.T, path string) {
	t.Helper()
	if !waitForTestFileWithin(path, 10*time.Second) {
		t.Fatal(fmt.Sprintf("timed out waiting for %s", path))
	}
}

func waitForTestFileWithin(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
