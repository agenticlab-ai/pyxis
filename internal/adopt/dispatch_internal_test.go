package adopt

// Internal (white-box) tests for the empty-tree route to init (UB-1): these
// swap installerInitFn — a package-private seam — so they can prove Inspect
// hands installer.Init exactly the right Options without paying for a full
// init-pipeline fixture (seeding, hooks, wiring), which the empty-repo
// installer's own tests already cover. Every other dispatch/classify
// behavior is tested black-box in inspect_test.go (package adopt_test),
// matching this package's established convention (plan_test.go,
// render_test.go).

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
)

// gitInit creates an empty repository at dir on branch main for these
// dispatch-only tests (no fixture release/manifest is needed: Inspect never
// reaches converge on the empty-tree leg).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "init", "-b", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// swapInstallerInit replaces installerInitFn for one test, restoring it on
// cleanup.
func swapInstallerInit(t *testing.T, fn func(context.Context, installer.Options) (installer.Report, error)) {
	t.Helper()
	orig := installerInitFn
	installerInitFn = fn
	t.Cleanup(func() { installerInitFn = orig })
}

func TestInspectRoutesEmptyTreeToInstallerInit(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)

	var calls int
	var got installer.Options
	swapInstallerInit(t, func(_ context.Context, opts installer.Options) (installer.Report, error) {
		calls++
		got = opts
		return installer.Report{Branch: "main", EpochWritten: true}, nil
	})

	pin := installer.Pin{Version: "v1.2.3", ManifestSHA256: strings.Repeat("ab", 32)}
	rep, err := Inspect(context.Background(), Options{
		Target:         root,
		Pin:            pin,
		ArtifactSource: "file:///nowhere",
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if calls != 1 {
		t.Fatalf("installerInitFn calls = %d, want 1", calls)
	}
	if !rep.RoutedToInit {
		t.Error("Report.RoutedToInit = false, want true for an empty git repository")
	}
	if rep.Init.Branch != "main" || !rep.Init.EpochWritten {
		t.Errorf("Report.Init = %+v, want the canned installer.Report passed through", rep.Init)
	}
	if got.Target != root {
		t.Errorf("installer.Options.Target = %q, want %q", got.Target, root)
	}
	if got.Pin != pin {
		t.Errorf("installer.Options.Pin = %+v, want %+v", got.Pin, pin)
	}
	if !got.AcceptDefaults {
		t.Error("installer.Options.AcceptDefaults = false, want true — adopt never prompts")
	}
	if got.ArtifactSource != "file:///nowhere" {
		t.Errorf("installer.Options.ArtifactSource = %q, want passthrough", got.ArtifactSource)
	}
}

func TestInspectEmptyTreeRefusesMissingPinBeforeCallingInit(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)

	var calls int
	swapInstallerInit(t, func(context.Context, installer.Options) (installer.Report, error) {
		calls++
		return installer.Report{}, nil
	})

	_, err := Inspect(context.Background(), Options{Target: root})
	if !errors.Is(err, ErrPinRequired) {
		t.Fatalf("err = %v, want ErrPinRequired", err)
	}
	if calls != 0 {
		t.Fatalf("installerInitFn calls = %d, want 0 (refuse before action)", calls)
	}
}

// TestInspectUntrackedFileMeansNotEmpty proves UB-1's emptiness predicate is
// tracked-files-AND-clean-worktree, not merely zero commits: a single
// untracked file already makes the tree non-empty, so Inspect must not route
// it to init — it should instead attempt to classify (which fails here for
// want of a manifest fixture; this test only asserts installerInitFn was
// never called).
func TestInspectUntrackedFileMeansNotEmpty(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls int
	swapInstallerInit(t, func(context.Context, installer.Options) (installer.Report, error) {
		calls++
		return installer.Report{}, nil
	})

	_, _ = Inspect(context.Background(), Options{
		Target:         root,
		Pin:            installer.Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("ab", 32)},
		ArtifactSource: "file://" + t.TempDir(), // empty — classify fails, which is fine here
	})
	if calls != 0 {
		t.Fatalf("installerInitFn calls = %d, want 0 — an untracked file means the tree is not empty (UB-1 routes only a clean, zero-tracked tree)", calls)
	}
}
