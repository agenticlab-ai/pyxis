package installer_test

// Hermetic tests for the write-journal rollback (design init-hardening-v1
// §3.6, DINV-3/DINV-4; plan init-hardening/001 T4): per-stage failure
// injection proving the target restores byte-identical to pre-run (the
// created-repo case returning the folder to empty; the adopted-repo case
// restoring the branch ref + index rather than deleting .git), a foreign
// entry planted mid-apply surviving and being named, a remotely created
// repository never being deleted (only its cleanup command reported), a
// context cancellation ("signal") mid-apply unwinding exactly like any other
// stage failure, a re-run proceeding past admission afterward, and
// init_rolled_back firing with its typed properties.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/installer"
)

// ---- FR-10/DINV-3: per-stage failure injection restores a bare dir to empty

func TestRollbackPerStageFailureRestoresBareDirToEmpty(t *testing.T) {
	for _, stage := range []string{"seed", "hooks", "wiring", "green-proof", "welcome"} {
		t.Run(stage, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			r := newRelease(t)
			target := t.TempDir()

			restore := installer.SetFailingStageForTest(stage)
			rep, err := installer.Init(context.Background(), r.baseOptions(target))
			restore()
			if err == nil {
				t.Fatalf("Init = nil error, want the injected %s-stage failure", stage)
			}
			if !strings.Contains(err.Error(), stage) {
				t.Errorf("error does not name the failed stage %q: %v", stage, err)
			}

			rb := rep.Rollback
			if stage == "welcome" {
				if rb != nil || !rep.Publication.Published {
					t.Fatalf("post-publication failure must retain install: %+v", rep)
				}
				if !strings.Contains(err.Error(), "re-run pyxis init") {
					t.Fatalf("missing publication recovery: %v", err)
				}
				return
			}
			if rb == nil {
				t.Fatalf("Report.Rollback is nil for a %s-stage failure", stage)
			}
			if rb.FailedStage != stage {
				t.Errorf("Rollback.FailedStage = %q, want %q", rb.FailedStage, stage)
			}
			if !rb.Restored {
				t.Errorf("Rollback.Restored = false, want true (errors: %v)", rb.Errors)
			}
			if !rb.RepoRemoved {
				t.Error("Rollback.RepoRemoved = false, want true — init created the repository this run")
			}
			if len(rb.ForeignPreserved) != 0 {
				t.Errorf("ForeignPreserved = %v, want empty (nothing foreign was planted)", rb.ForeignPreserved)
			}

			entries, rerr := os.ReadDir(target)
			if rerr != nil {
				t.Fatalf("readdir target: %v", rerr)
			}
			if len(entries) != 0 {
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name()
				}
				t.Errorf("target not empty after %s-stage rollback: %v", stage, names)
			}

			// A subsequent init proceeds past admission (the restored seam
			// runs the real stage this time).
			if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
				t.Fatalf("re-run after %s-stage rollback: %v", stage, err)
			}
		})
	}
}

// ---- FR-10/DINV-3: an adopted pre-existing empty repo restores its ref +
// index rather than losing .git, so a re-run still proceeds past admission —
// both when the failure lands before any commit (the index was never
// populated at all — restoreGitRef's `git read-tree --empty` must tolerate a
// virgin repo with no index file yet) and after both commits (the branch ref
// this run created its first-ever commit on must be cleared back to unborn).

func TestRollbackAdoptedEmptyRepoRestoresRefAndIndexNotDeleted(t *testing.T) {
	for _, stage := range []string{"seed", "hooks", "green-proof"} {
		t.Run(stage, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Ada", "ada@example.com")
			r := newRelease(t)
			target := t.TempDir()
			git(t, target, "init", "-b", "main") // pre-existing empty repo, HEAD unborn

			before := snap(t, target) // .git-excluded — should be empty

			restore := installer.SetFailingStageForTest(stage)
			rep, err := installer.Init(context.Background(), r.baseOptions(target))
			restore()
			if err == nil {
				t.Fatalf("Init = nil error, want the injected %s failure", stage)
			}
			if rep.Rollback == nil {
				t.Fatal("Report.Rollback is nil")
			}
			if !rep.Rollback.Restored {
				t.Errorf("Rollback.Restored = false, want true (errors: %v)", rep.Rollback.Errors)
			}
			if rep.Rollback.RepoRemoved {
				t.Error("Rollback.RepoRemoved = true, want false — init did not create this repository")
			}
			if _, statErr := os.Stat(filepath.Join(target, ".git")); statErr != nil {
				t.Errorf(".git no longer exists after rollback of an adopted repo: %v", statErr)
			}

			sameSnap(t, before, snap(t, target), "adopted-empty-repo rollback ("+stage+")")
			if n := gitRevListCount(t, target); n != 0 {
				t.Errorf("HEAD has %d commit(s) after rollback, want unborn (0)", n)
			}
			if tracked := strings.TrimSpace(git(t, target, "ls-files")); tracked != "" {
				t.Errorf("tracked files survive rollback: %q", tracked)
			}
			if dirty := strings.TrimSpace(git(t, target, "status", "--porcelain")); dirty != "" {
				t.Errorf("worktree dirty after rollback: %q", dirty)
			}

			// A subsequent init proceeds past admission.
			if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
				t.Fatalf("re-run after %s rollback: %v", stage, err)
			}
		})
	}
}

// ---- FR-10/DINV-3: a foreign file planted mid-apply survives and is named

func TestRollbackForeignFileSurvivesAndIsNamed(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	const foreignRel = "sub/planted-foreign.txt"
	restore := installer.SetStageBodyForTest("hooks", func(_ context.Context, root string) error {
		p := filepath.Join(root, filepath.FromSlash(foreignRel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte("not init's\n"), 0o644); err != nil {
			return err
		}
		return errors.New("injected hooks-stage failure after planting foreign content (test)")
	})
	defer restore()

	rep, err := installer.Init(context.Background(), r.baseOptions(target))
	if err == nil {
		t.Fatal("Init = nil error, want the injected hooks failure")
	}
	if rep.Rollback == nil {
		t.Fatal("Report.Rollback is nil")
	}
	if !slices.Contains(rep.Rollback.ForeignPreserved, foreignRel) {
		t.Errorf("ForeignPreserved = %v, want it to contain %q", rep.Rollback.ForeignPreserved, foreignRel)
	}
	if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(foreignRel))); statErr != nil {
		t.Errorf("the foreign file did not survive the unwind: %v", statErr)
	}

	// Nothing else survives: the foreign file's directory is the only
	// top-level entry left once init's own repository + writes are gone.
	entries, rerr := os.ReadDir(target)
	if rerr != nil {
		t.Fatalf("readdir target: %v", rerr)
	}
	if len(entries) != 1 || entries[0].Name() != "sub" {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("unexpected top-level survivors: %v, want only [sub]", names)
	}
}

// ---- DINV-4: a remotely created repository is never deleted; its cleanup
// command is named in the rollback report.

func TestRollbackRemoteCreatedRepoNeverDeletedNamesCleanupCommand(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	git(t, target, "init", "-b", "main")
	git(t, target, "remote", "add", "origin", "https://github.com/acme/widgets.git")

	restore := installer.SetFailingStageForTest("green-proof") // after wiring
	opts := r.baseOptions(target)
	opts.Answers.WiringRepo = "create" // declares create-repo; TestMain's default fake marks it Applied
	rep, err := installer.Init(context.Background(), opts)
	restore()
	if err == nil {
		t.Fatal("Init = nil error, want the injected green-proof failure")
	}
	if rep.Rollback == nil {
		t.Fatal("Report.Rollback is nil")
	}
	want := "gh repo delete acme/widgets --yes"
	if !slices.Contains(rep.Rollback.RemoteCleanup, want) {
		t.Errorf("RemoteCleanup = %v, want it to contain %q", rep.Rollback.RemoteCleanup, want)
	}
}

// ---- UB-3: a context cancellation ("signal") mid-apply unwinds exactly like
// any other stage failure — including the git-command cleanup on an adopted
// repository, which must survive the ALREADY-cancelled context (unwind uses
// context.WithoutCancel for its own git calls).

func TestRollbackContextCancellationMidApplyUnwinds(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	git(t, target, "init", "-b", "main")

	before := snap(t, target)

	ctx, cancel := context.WithCancel(context.Background())
	restore := installer.SetStageBodyForTest("green-proof", func(ctx context.Context, _ string) error {
		cancel() // simulate a signal delivered mid-apply
		<-ctx.Done()
		return ctx.Err()
	})
	defer restore()

	_, err := installer.Init(ctx, r.baseOptions(target))
	if err == nil {
		t.Fatal("Init = nil error, want the cancellation to fail the run")
	}

	sameSnap(t, before, snap(t, target), "cancellation-mid-apply rollback")
	if n := gitRevListCount(t, target); n != 0 {
		t.Errorf("HEAD has %d commit(s) after rollback, want unborn (0)", n)
	}
	if tracked := strings.TrimSpace(git(t, target, "ls-files")); tracked != "" {
		t.Errorf("tracked files survive rollback: %q", tracked)
	}

	restore()
	if _, err := installer.Init(context.Background(), r.baseOptions(target)); err != nil {
		t.Fatalf("re-run after cancellation rollback: %v", err)
	}
}

// ---- EVT-3: init_rolled_back fires with its typed properties

func TestInitRolledBackEventFiresWithTypedProperties(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	restore := installer.SetFailingStageForTest("hooks")
	defer restore()

	var buf bytes.Buffer
	opts := r.baseOptions(target)
	opts.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	_, err := installer.Init(context.Background(), opts)
	if err == nil {
		t.Fatal("Init = nil error, want the injected hooks failure")
	}

	evt := findEvent(t, buf.String(), "init_rolled_back")
	if evt == nil {
		t.Fatalf("no init_rolled_back event in:\n%s", buf.String())
	}
	if evt["failed_stage"] != "hooks" {
		t.Errorf("failed_stage = %v, want %q", evt["failed_stage"], "hooks")
	}
	if evt["restored"] != true {
		t.Errorf("restored = %v, want true", evt["restored"])
	}
	if evt["run_id"] == nil || evt["run_id"] == "" {
		t.Error("event missing run_id (the correlation id)")
	}
	// install_completed must NOT ALSO fire — the pipeline failed.
	if strings.Contains(buf.String(), "install_completed") {
		t.Errorf("install_completed fired despite a rolled-back pipeline:\n%s", buf.String())
	}
}
