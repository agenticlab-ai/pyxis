package installer_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func realPublicationGit(ctx context.Context, root string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return string(out), nil
}

func publicationBare(t *testing.T) string {
	t.Helper()
	// Resolve the restricted-PATH symlink before asking relocatable Git for
	// its helper directory, as the fresh-clone fixture does. A hook can put
	// Git's libexec alias ahead of the system launcher.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err = filepath.EvalSymlinks(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	helperPath, err := exec.CommandContext(t.Context(), gitPath, "--exec-path").Output()
	if err != nil || strings.TrimSpace(string(helperPath)) == "" {
		t.Fatalf("discover Git helper path: %q %v", helperPath, err)
	}
	execPath := strings.TrimSpace(string(helperPath))
	t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+execPath)
	bare := filepath.Join(t.TempDir(), "destination.git")
	git(t, t.TempDir(), "init", "--bare", "-b", "main", bare)
	return bare
}

func localPublicationOptions(t *testing.T, r release, target, bare string) installer.Options {
	t.Helper()
	opts := r.carinaOptions(target)
	opts.Answers.RemoteURL = bare
	opts.PublicationGit = realPublicationGit
	return opts
}

func TestInitPublishesExactHeadToConfiguredLocalGit(t *testing.T) {
	for _, existingGit := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-git=%v", existingGit), func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Test", "test@example.invalid")
			r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
			opts := localPublicationOptions(t, r, target, bare)
			if existingGit {
				git(t, target, "init", "-b", "main")
				git(t, target, "remote", "add", "origin", bare)
				opts.Answers.RemoteURL = ""
			}
			rep, err := installer.Init(t.Context(), opts)
			if err != nil {
				t.Fatal(err)
			}
			head := strings.TrimSpace(git(t, target, "rev-parse", "HEAD"))
			remoteHead := strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/main"))
			if !rep.Publication.Published || head != remoteHead || head != rep.Publication.Head || rep.GitInited == existingGit {
				t.Fatalf("publication=%+v local=%s remote=%s GitInited=%v", rep.Publication, head, remoteHead, rep.GitInited)
			}
			if got := strings.TrimSpace(git(t, target, "rev-parse", "@{upstream}")); got != head {
				t.Fatalf("upstream=%s want %s", got, head)
			}
			if got := strings.TrimSpace(git(t, target, "config", "core.hooksPath")); got != ".githooks" {
				t.Fatalf("installed hooks were disabled: %s", got)
			}
			if _, err := gitIn(t, target, "config", "--local", "--get", "pyxis.init-publication"); err == nil {
				t.Fatal("successful installation retained pending publication")
			}
			if len(strings.Fields(git(t, target, "ls-tree", "-r", "--name-only", "HEAD", ".workitems/items"))) != 4 {
				t.Fatal("initial push omitted the welcome/onboarding records")
			}
		})
	}
}

func TestLocalFirstPushUsesTrunkAndLaterGovernanceStillNeedsHistoryGate(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	git(t, target, "init", "-b", "trunk")
	git(t, target, "remote", "add", "origin", bare)
	// Init captures an existing repo's branch and configured remote. The
	// explicit local-git lifecycle binding is selected after this bootstrap.
	opts := localPublicationOptions(t, r, target, bare)
	opts.Answers.RemoteURL = ""
	git(t, target, "config", "branch.trunk.remote", "origin")
	git(t, target, "config", "branch.trunk.merge", "refs/heads/trunk")
	pushed := false
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			pushed = true
			if !slices.Contains(args, "--force-with-lease=refs/heads/trunk:") {
				t.Fatalf("first push lacks zero-old lease: %v", args)
			}
		}
		return realPublicationGit(ctx, root, args...)
	}
	rep, err := installer.Init(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.T8.GreenProof.Green {
		t.Fatal("first push lacked its verified init proof")
	}
	if !pushed || !rep.Publication.Published || rep.Publication.Branch != "trunk" || strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/trunk")) != rep.Publication.Head {
		t.Fatalf("publication=%+v", rep.Publication)
	}
	cfg, defects := config.Load(target)
	if len(defects) > 0 || cfg.Branch != "trunk" {
		t.Fatalf("config=%+v %v", cfg, defects)
	}
	vision := filepath.Join(target, cfg.ProductVision)
	if err := os.WriteFile(vision, []byte("Rewritten without operator authorization.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, target, "-c", "core.hooksPath=", "add", "--", cfg.ProductVision)
	git(t, target, "-c", "core.hooksPath=", "commit", "-m", "invalid governing rewrite")
	head := strings.TrimSpace(git(t, target, "rev-parse", "HEAD"))
	if defects := gate.GateAt(target, rep.Publication.Head, head, nil, cfg); len(defects) == 0 {
		t.Fatal("later history violation bypassed the gate")
	}
	cfg.Epoch.Commit = strings.Repeat("0", 40)
	if defects := gate.GateAt(target, rep.Publication.Head, head, nil, cfg); len(defects) == 0 {
		t.Fatal("wrong epoch bypassed the gate")
	}
}

func TestInitRequiresDestinationBeforeWrites(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target := newRelease(t), t.TempDir()
	opts := r.carinaOptions(target)
	opts.Answers.RemoteURL = ""
	if _, err := installer.Init(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "no Git publication destination") {
		t.Fatalf("missing destination: %v", err)
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing destination wrote target: %v, %v", entries, err)
	}
}

func TestInitPreviewDescribesBoundPublicationWithoutTransport(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	t.Setenv("TEST_INIT_NATIVE_TOKEN", "preview-token")
	target := t.TempDir()
	opts, writes := forgejoInitOptions(t, target)
	opts.Answers.RemoteURL = ""
	opts.Preview = true
	opts.PublicationGit = func(context.Context, string, ...string) (string, error) {
		t.Fatal("preview reached Git publication transport")
		return "", nil
	}
	rep, err := installer.Init(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	publication := rep.Publication
	if !publication.WouldPush || publication.Published || publication.Remote != "origin" || publication.Branch != "main" || publication.Destination != "http://127.0.0.1:3000/owner/repository.git" {
		t.Fatalf("bound preview destination: %+v", publication)
	}
	if *writes != 0 || len(rep.Commits) != 0 || rep.GitInited || !rep.WouldGitInit {
		t.Fatalf("preview performed installation work: writes=%d report=%+v", *writes, rep)
	}
	if !slices.Contains(rep.Stages, "first-push") || !slices.Contains(rep.Stages, "finish-wiring") {
		t.Fatalf("preview omitted publication or post-push wiring: %v", rep.Stages)
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preview wrote target: %v %v", entries, err)
	}
}

func TestInitRefusesUnusableLocalStoreBeforePublication(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	opts.PublicationGit = func(context.Context, string, ...string) (string, error) {
		t.Fatal("unusable local store reached publication transport")
		return "", nil
	}
	const foreign = ".workitems/items"
	restore := installer.SetStageBodyForTest("wiring", func(_ context.Context, root string) error {
		item := filepath.Join(root, filepath.FromSlash(foreign))
		if err := os.MkdirAll(filepath.Dir(item), 0o755); err != nil {
			return err
		}
		return os.WriteFile(item, []byte("concurrent non-directory store entry\n"), 0o644)
	})
	defer restore()
	rep, err := installer.Init(t.Context(), opts)
	if err == nil || !strings.Contains(err.Error(), "welcome") {
		t.Fatalf("unusable local store did not refuse local welcome: %v", err)
	}
	if rep.Publication.Published || rep.Rollback == nil || !slices.Contains(rep.Rollback.ForeignPreserved, foreign) {
		t.Fatalf("refusal lost the publication or foreign-file outcome: %+v", rep)
	}
	if body, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(foreign))); err != nil || string(body) != "concurrent non-directory store entry\n" {
		t.Fatalf("rollback changed concurrent store entry: %q %v", body, err)
	}
	if refs := strings.TrimSpace(git(t, bare, "for-each-ref", "--format=%(refname)")); refs != "" {
		t.Fatalf("unusable local store published refs: %s", refs)
	}
}

func publishOtherHead(t *testing.T, bare string) string {
	t.Helper()
	other := t.TempDir()
	git(t, other, "init", "-b", "main")
	git(t, other, "commit", "--allow-empty", "-m", "independent mainline")
	git(t, other, "push", "--", bare, "HEAD:refs/heads/main")
	return strings.TrimSpace(git(t, other, "rev-parse", "HEAD"))
}

func TestInitNeverOverwritesExistingDestination(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	prior := publishOtherHead(t, bare)
	rep, err := installer.Init(t.Context(), localPublicationOptions(t, r, target, bare))
	if err == nil || !strings.Contains(err.Error(), "already has a different") {
		t.Fatalf("existing destination: %v", err)
	}
	if rep.Rollback == nil || !rep.Rollback.Restored {
		t.Fatalf("unpublished install did not roll back: %+v", rep)
	}
	if got := strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/main")); got != prior {
		t.Fatalf("remote rewritten: %s != %s", got, prior)
	}
}

func TestInitialPushLeaseRejectsConcurrentDestinationCreation(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	var prior string
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			prior = publishOtherHead(t, bare)
		}
		return realPublicationGit(ctx, root, args...)
	}
	rep, err := installer.Init(t.Context(), opts)
	if err == nil || rep.Publication.Published {
		t.Fatalf("racing remote accepted: %+v %v", rep, err)
	}
	if got := strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/main")); got != prior {
		t.Fatalf("race overwrote destination: %s != %s", got, prior)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("potentially published installation was deleted: %v", err)
	}
}

func TestInitResumesUncertainPublicationWithoutReinstalling(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	attempted := false
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			attempted = true
			return "", errors.New("connection dropped")
		}
		if attempted {
			return "", errors.New("cannot confirm remote state")
		}
		return realPublicationGit(ctx, root, args...)
	}
	first, err := installer.Init(t.Context(), opts)
	if err == nil || first.Rollback != nil || !strings.Contains(err.Error(), "re-run pyxis init") {
		t.Fatalf("uncertain publication: %+v %v", first, err)
	}
	head := strings.TrimSpace(git(t, target, "rev-parse", "HEAD"))
	// Preview describes resumption but cannot publish or clear pending state.
	opts.Preview = true
	opts.PublicationGit = func(context.Context, string, ...string) (string, error) {
		t.Fatal("preview reached publication transport")
		return "", nil
	}
	preview, err := installer.Init(t.Context(), opts)
	if err != nil || !preview.Resumed || !preview.Publication.WouldPush {
		t.Fatalf("resume preview: %+v %v", preview, err)
	}
	opts.Preview = false
	opts.PublicationGit = realPublicationGit
	resumed, err := installer.Init(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if !resumed.Resumed || !resumed.Publication.Published || resumed.RunID != first.RunID {
		t.Fatalf("resume=%+v first=%+v", resumed, first)
	}
	if got := strings.TrimSpace(git(t, target, "rev-parse", "HEAD")); got != head {
		t.Fatalf("resumption changed install commits: %s != %s", got, head)
	}
	if got := strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/main")); got != head {
		t.Fatalf("resumption published %s want %s", got, head)
	}
	if dirty := strings.TrimSpace(git(t, target, "status", "--porcelain", "--untracked-files=no")); dirty != "" {
		t.Fatalf("resumption changed committed installation: %s", dirty)
	}
}

func TestInitConfirmsPushAfterLostAcknowledgment(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		out, err := realPublicationGit(ctx, root, args...)
		if err == nil && slices.Contains(args, "push") {
			return "", errors.New("acknowledgment lost after receive")
		}
		return out, err
	}
	rep, err := installer.Init(t.Context(), opts)
	if err != nil || !rep.Publication.Published {
		t.Fatalf("confirmed receive reported failure: %+v %v", rep, err)
	}
}

func TestInitRetainsRejectedPushForRetry(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			return "", errors.New("receive denied")
		}
		return realPublicationGit(ctx, root, args...)
	}
	rep, err := installer.Init(t.Context(), opts)
	if err == nil || rep.Rollback != nil || !strings.Contains(err.Error(), "re-run pyxis init") {
		t.Fatalf("push failure lost retry state: %+v %v", rep, err)
	}
	opts.PublicationGit = realPublicationGit
	resumed, err := installer.Init(t.Context(), opts)
	if err != nil || !resumed.Resumed || !resumed.Publication.Published || resumed.Publication.Head != rep.Commits[1].SHA {
		t.Fatalf("rejected push did not resume the same installation: %+v %v", resumed, err)
	}
}

func TestInitRetainsStateWhenReceiveFinishesAfterAbsentReadback(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			return "", errors.New("connection interrupted before receive completed")
		}
		return realPublicationGit(ctx, root, args...)
	}
	first, err := installer.Init(t.Context(), opts)
	if err == nil || first.Rollback != nil || first.Publication.Published {
		t.Fatalf("absent readback was mistaken for definitive rollback authority: %+v %v", first, err)
	}
	// The remote receive completes after the failed invocation's empty readback.
	head := first.Commits[1].SHA
	if _, err := realPublicationGit(t.Context(), target, "-c", "core.hooksPath=", "push", "--", bare, head+":refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if slices.Contains(args, "push") {
			t.Fatal("resumption pushed a head that the delayed receive already published")
		}
		return realPublicationGit(ctx, root, args...)
	}
	resumed, err := installer.Init(t.Context(), opts)
	if err != nil || !resumed.Resumed || !resumed.Publication.Published || resumed.Publication.Head != head {
		t.Fatalf("late receive stranded its installation: %+v %v", resumed, err)
	}
}

func TestInitResumptionRefusesChangedInstallation(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Test", "test@example.invalid")
	r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
	opts := localPublicationOptions(t, r, target, bare)
	attempted := false
	opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
		if attempted || slices.Contains(args, "push") {
			attempted = true
			return "", errors.New("transport unavailable after attempt")
		}
		return realPublicationGit(ctx, root, args...)
	}
	if _, err := installer.Init(t.Context(), opts); err == nil {
		t.Fatal("fixture should retain an uncertain installation")
	}
	opts.PublicationGit = func(context.Context, string, ...string) (string, error) {
		t.Fatal("changed installation reached the publication transport")
		return "", nil
	}
	refuse := func(want string) {
		t.Helper()
		if _, err := installer.Init(t.Context(), opts); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("resume should refuse %q: %v", want, err)
		}
	}
	vision := filepath.Join(target, "docs/product-vision.md")
	originalVision, err := os.ReadFile(vision)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vision, append(originalVision, []byte("\nChanged governance.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	refuse("installed tracked files changed")
	if err := os.WriteFile(vision, originalVision, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, target, "remote", "set-url", "origin", bare+"-redirect")
	refuse("destination changed")
	git(t, target, "remote", "set-url", "origin", bare)
	opts.Answers.RemoteURL = bare + "-redirect"
	refuse("differs from the pending publication destination")
	opts.Answers.RemoteURL = bare
	configPath := filepath.Join(target, ".pyxis/config.yaml")
	configBody, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(git(t, target, "rev-parse", "HEAD^"))
	if err := os.WriteFile(configPath, []byte(strings.ReplaceAll(string(configBody), tree, "HEAD")), 0o644); err != nil {
		t.Fatal(err)
	}
	refuse("epoch changed")
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, target, "-c", "core.hooksPath=", "commit", "--allow-empty", "-m", "post-install commit")
	refuse("installation head")
}

func TestHostedWelcomeRunsAfterPublicationAndResumesWithoutAnotherPush(t *testing.T) {
	for _, protection := range []string{"none", "required-checks"} {
		t.Run(protection, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Test", "test@example.invalid")
			r, target, bare := newRelease(t), t.TempDir(), publicationBare(t)
			opts := r.baseOptions(target)
			opts.Answers.WiringProtection = protection
			pushes := 0
			opts.PublicationGit = func(ctx context.Context, root string, args ...string) (string, error) {
				argv := append([]string(nil), args...)
				argv[len(argv)-2] = bare
				if slices.Contains(args, "push") {
					pushes++
				}
				return realPublicationGit(ctx, root, argv...)
			}
			fake := &fakeWire{}
			restoreWire := installer.SetWireExecuteForTest(func(ctx context.Context, ops []wire.Op) (wire.Audit, error) {
				for _, op := range ops {
					if op.Name == wire.OpSetProtection || op.Name == wire.OpWelcomeItem || op.Name == wire.OpOnboardingItems {
						if pushes == 0 || strings.TrimSpace(git(t, bare, "rev-parse", "refs/heads/main")) != strings.TrimSpace(git(t, target, "rev-parse", "HEAD")) {
							t.Fatal("protection or hosted welcome ran before the first push")
						}
					}
				}
				return fake.execute(ctx, ops)
			})
			t.Cleanup(restoreWire)
			restoreWelcome := installer.SetFailingStageForTest("welcome")
			first, err := installer.Init(t.Context(), opts)
			restoreWelcome()
			if err == nil || first.Rollback != nil || !first.Publication.Published {
				t.Fatalf("late failure lost published state: %+v %v", first, err)
			}
			resumed, err := installer.Init(t.Context(), opts)
			if err != nil || !resumed.Resumed || !resumed.Publication.Published || !resumed.T8.Welcome.Emitted || pushes != 1 {
				t.Fatalf("published recovery: pushes=%d report=%+v err=%v", pushes, resumed, err)
			}
			if strings.TrimSpace(git(t, target, "rev-parse", "HEAD")) != first.Publication.Head {
				t.Fatal("recovery recreated installation commits")
			}
		})
	}
}

func TestInitialPublicationRequiresValidRefAdvertisementAndRetainedHead(t *testing.T) {
	for _, scenario := range []string{"read failure", "invalid advertisement", "receive without retained ref"} {
		t.Run(scenario, func(t *testing.T) {
			isolateGit(t)
			ambientIdentityConfig(t, "Test", "test@example.invalid")
			r, target := newRelease(t), t.TempDir()
			opts := r.carinaOptions(target)
			pushes := 0
			opts.PublicationGit = func(_ context.Context, _ string, args ...string) (string, error) {
				if slices.Contains(args, "push") {
					pushes++
					return "", nil
				}
				switch scenario {
				case "read failure":
					return "", errors.New("destination cannot be read")
				case "invalid advertisement":
					return "HEAD refs/heads/main", nil
				default:
					return "", nil
				}
			}
			rep, err := installer.Init(t.Context(), opts)
			if err == nil || rep.Publication.Published {
				t.Fatalf("unverified publication accepted: %+v %v", rep, err)
			}
			if scenario == "receive without retained ref" {
				if rep.Rollback != nil {
					t.Fatal("an attempted publication was destructively rolled back")
				}
			} else if rep.Rollback == nil || !rep.Rollback.Restored {
				t.Fatal("failure before any push did not restore the installation target")
			}
			if (scenario == "receive without retained ref") != (pushes == 1) {
				t.Fatalf("unsafe ref advertisement led to a push: %d", pushes)
			}
		})
	}
}
