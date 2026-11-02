package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func prototypeTestRoute(context.Context) (PrototypeRoute, error) {
	return PrototypeRoute{}, nil
}

func prototypeGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := draftshell.Run(context.Background(), root, nil, "git", args...)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func prototypeWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func prototypeFixture(t *testing.T) (string, string, config.Config) {
	t.Helper()
	root, _ := gitFixture(t)
	// Git reports physical paths on macOS, where TempDir may use /var.
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	spacedRoot := filepath.Join(filepath.Dir(root), "main checkout ")
	if err := os.Rename(root, spacedRoot); err != nil {
		t.Fatal(err)
	}
	root = spacedRoot
	cfg := config.Defaults()
	cfg.Draft.BranchPrefix = "contracts/"
	prototypeGit(t, root, "branch", cfg.Draft.BranchPrefix+"widget")
	idea := filepath.Join(t.TempDir(), "idea.txt")
	prototypeWrite(t, idea, "Exercise the idea.")
	return root, idea, cfg
}

func assertNoPrototype(t *testing.T, root string) {
	t.Helper()
	p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(p.Worktree); !os.IsNotExist(err) {
		t.Fatalf("prototype directory remains: %v", err)
	}
	if refs := prototypeGit(t, root, "for-each-ref", "--format=%(refname)", "refs/heads/prototype/"); refs != "" {
		t.Fatalf("prototype branch remains: %s", refs)
	}
	if worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain", "-z"); strings.Contains(worktrees, "worktree "+p.Worktree+"\x00") {
		t.Fatalf("prototype registration remains: %s", worktrees)
	}
}

func TestPrototypeCreateFromMainAndLinkedCheckout(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "main checkout", true: "linked checkout"}[linked], func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			mainBefore := prototypeGit(t, root, "rev-parse", "main")
			draftBefore := prototypeGit(t, root, "rev-parse", "contracts/widget")
			// Advance remote mainline without updating the caller's local main.
			publisher := filepath.Join(t.TempDir(), "publisher")
			prototypeGit(t, root, "clone", "--branch", "main", prototypeGit(t, root, "remote", "get-url", "origin"), publisher)
			prototypeGit(t, publisher, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "new mainline")
			prototypeGit(t, publisher, "push", "origin", "main")
			wantBase := prototypeGit(t, publisher, "rev-parse", "HEAD")
			caller := root
			if linked {
				caller = filepath.Join(filepath.Dir(root), "linked")
				prototypeGit(t, root, "worktree", "add", "-b", "caller", caller, "HEAD")
			}
			prototypeWrite(t, filepath.Join(caller, "keep.txt"), "uncommitted caller data")
			before := prototypeGit(t, caller, "status", "--porcelain=v1")
			callerHead := prototypeGit(t, caller, "rev-parse", "HEAD")
			callerBranch := prototypeGit(t, caller, "symbolic-ref", "HEAD")
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name != "git" || args[0] == "push" || args[0] == "switch" {
					t.Fatalf("unexpected publishing or checkout command: %s %v", name, args)
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			p, err := CreatePrototype(context.Background(), caller, cfg, "widget", idea, prototypeTestRoute, run)
			if err != nil {
				t.Fatal(err)
			}
			wantPath := filepath.Join(filepath.Dir(root), ".pyxis-worktrees", filepath.Base(root), "prototype-widget")
			if p.Worktree != wantPath || p.Branch != "prototype/widget" || p.Base != wantBase {
				t.Fatalf("unexpected prototype: %+v; path=%s base=%s", p, wantPath, wantBase)
			}
			if prototypeGit(t, p.Worktree, "branch", "--show-current") != p.Branch || prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != wantBase {
				t.Fatal("prototype not on fetched mainline")
			}
			if upstream := prototypeGit(t, root, "for-each-ref", "--format=%(upstream)", "refs/heads/"+p.Branch); upstream != "" {
				t.Fatalf("prototype has upstream %s", upstream)
			}
			if prototypeGit(t, root, "rev-parse", "contracts/widget") != draftBefore || prototypeGit(t, root, "rev-parse", "main") != mainBefore || prototypeGit(t, caller, "rev-parse", "HEAD") != callerHead || prototypeGit(t, caller, "symbolic-ref", "HEAD") != callerBranch || prototypeGit(t, caller, "status", "--porcelain=v1") != before {
				t.Fatal("caller or draft changed")
			}
			// Removal must accept committed prototype work ahead of mainline.
			prototypeWrite(t, filepath.Join(p.Worktree, "built.txt"), "prototype")
			prototypeGit(t, p.Worktree, "add", "built.txt")
			prototypeGit(t, p.Worktree, "commit", "-m", "prototype")
			state := p.Worktree + "-runtime"
			if err := os.MkdirAll(filepath.Join(state, "codex"), 0o700); err != nil {
				t.Fatal(err)
			}
			prototypeWrite(t, filepath.Join(state, "codex", "state"), "runtime")
			removed, err := RemovePrototype(context.Background(), caller, "widget", run)
			if err != nil || !removed {
				t.Fatalf("remove: %t %v", removed, err)
			}
			assertNoPrototype(t, root)
			if _, err := os.Stat(state); !os.IsNotExist(err) {
				t.Fatalf("state remains: %v", err)
			}
			removed, err = RemovePrototype(context.Background(), caller, "widget", run)
			if err != nil || removed {
				t.Fatalf("nothing to remove: %t %v", removed, err)
			}
		})
	}
}

func TestPrototypeFetchMappingPreservesDraftAndCaller(t *testing.T) {
	for _, linked := range []bool{false, true} {
		for _, outcome := range []string{"success", "existing prototype", "creation failure"} {
			name := "main/" + outcome
			if linked {
				name = "linked/" + outcome
			}
			t.Run(name, func(t *testing.T) {
				root, idea, cfg := prototypeFixture(t)
				base := prototypeGit(t, root, "rev-parse", "origin/main")
				// Keep the draft ahead of remote mainline so overwriting it is visible.
				prototypeGit(t, root, "commit", "--allow-empty", "-qm", "local draft work")
				draft := cfg.Draft.BranchPrefix + "widget"
				prototypeGit(t, root, "branch", "-f", draft, "HEAD")
				mapping := "+refs/heads/main:refs/heads/" + draft
				prototypeGit(t, root, "config", "remote.origin.fetch", mapping)
				caller := root
				if linked {
					caller = filepath.Join(filepath.Dir(root), "caller")
					prototypeGit(t, root, "worktree", "add", "-b", "caller", caller, "HEAD")
				}
				if outcome == "existing prototype" {
					prototypeGit(t, root, "branch", "prototype/widget")
				}
				prototypeWrite(t, filepath.Join(caller, "keep.txt"), "uncommitted caller data")
				draftHead := prototypeGit(t, root, "rev-parse", draft)
				mainHead := prototypeGit(t, root, "rev-parse", "main")
				callerHead := prototypeGit(t, caller, "rev-parse", "HEAD")
				callerBranch := prototypeGit(t, caller, "symbolic-ref", "HEAD")
				callerStatus := prototypeGit(t, caller, "status", "--porcelain")
				refs := prototypeGit(t, root, "show-ref", "--heads")
				cause := errors.New("injected worktree creation failure")
				attempts := 0
				run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					if strings.HasPrefix(strings.Join(args, " "), "worktree add ") {
						attempts++
						if outcome == "creation failure" {
							// Exercise cleanup after Git has registered the worktree.
							if _, err := draftshell.Run(ctx, dir, env, name, args...); err != nil {
								t.Fatal(err)
							}
							return nil, cause
						}
					}
					return draftshell.Run(ctx, dir, env, name, args...)
				}
				p, err := CreatePrototype(context.Background(), caller, cfg, "widget", idea, prototypeTestRoute, run)
				if prototypeGit(t, root, "rev-parse", draft) != draftHead || prototypeGit(t, root, "rev-parse", "main") != mainHead || prototypeGit(t, caller, "rev-parse", "HEAD") != callerHead || prototypeGit(t, caller, "symbolic-ref", "HEAD") != callerBranch || prototypeGit(t, caller, "status", "--porcelain") != callerStatus {
					t.Fatal("fetch changed the draft branch, mainline, or invoking checkout")
				}
				switch outcome {
				case "success":
					if err != nil || p.Base != base || prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != base || attempts != 1 {
						t.Fatalf("prototype did not use fetched mainline: %+v %v attempts=%d", p, err, attempts)
					}
					if removed, err := RemovePrototype(context.Background(), caller, "widget", draftshell.Run); err != nil || !removed {
						t.Fatalf("remove prototype: %t %v", removed, err)
					}
				case "existing prototype":
					if err == nil || !strings.Contains(err.Error(), "already exists") || attempts != 0 {
						t.Fatalf("existing prototype not refused: %v attempts=%d", err, attempts)
					}
				case "creation failure":
					if !errors.Is(err, cause) || attempts != 1 {
						t.Fatalf("creation failure lost or retried: %v attempts=%d", err, attempts)
					}
					assertNoPrototype(t, root)
				}
				if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "config", "--get", "remote.origin.fetch") != mapping {
					t.Fatal("prototype operation changed existing branch refs or fetch configuration")
				}
			})
		}
	}
}

func TestPrototypeCreateOrderedRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		fetches    int
	}{
		{"invalid slug", "lowercase hyphenated slug", 0},
		{"missing idea", "read idea file", 0},
		{"empty idea", "is empty", 0},
		{"missing draft", "open the draft first", 0},
		{"existing branch", "prototype branch prototype/widget already exists", 1},
		{"existing directory", "prototype directory", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			feature := "widget"
			p, err := prototypeLocation(context.Background(), root, feature, draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "invalid slug":
				feature, idea = "../Bad", "missing"
			case "missing idea":
				idea += ".missing"
			case "empty idea":
				prototypeWrite(t, idea, " \n\t")
			case "missing draft":
				prototypeGit(t, root, "branch", "-D", "contracts/widget")
			case "existing branch":
				prototypeGit(t, root, "branch", p.Branch)
			case "existing directory":
				if err := os.MkdirAll(p.Worktree, 0o755); err != nil {
					t.Fatal(err)
				}
				prototypeWrite(t, filepath.Join(p.Worktree, "keep"), "existing data")
			}
			refsBefore := prototypeGit(t, root, "show-ref", "--heads")
			worktreesBefore := prototypeGit(t, root, "worktree", "list", "--porcelain")
			fetches := 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if args[0] == "fetch" {
					fetches++
				} else if args[0] != "rev-parse" && args[0] != "for-each-ref" {
					t.Fatalf("unexpected command before refusal: %v", args)
				}
				if tc.name == "invalid slug" || tc.name == "missing idea" || tc.name == "empty idea" {
					t.Fatalf("Git ran before input refusal: %v", args)
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			_, err = CreatePrototype(context.Background(), root, cfg, feature, idea, prototypeTestRoute, run)
			if err == nil || !strings.Contains(err.Error(), tc.want) || fetches != tc.fetches {
				t.Fatalf("refusal: %v, fetches=%d; want %q, %d", err, fetches, tc.want, tc.fetches)
			}
			if prototypeGit(t, root, "show-ref", "--heads") != refsBefore || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktreesBefore {
				t.Fatal("refusal changed branches or worktree registrations")
			}
			if tc.name == "existing directory" {
				if data, err := os.ReadFile(filepath.Join(p.Worktree, "keep")); err != nil || string(data) != "existing data" {
					t.Fatalf("existing directory changed: %q %v", data, err)
				}
			} else if _, err := os.Lstat(p.Worktree); !os.IsNotExist(err) {
				t.Fatalf("created prototype directory: %v", err)
			}
		})
	}
}

func TestPrototypeSeparateGitDirectoryRefusedBeforeFetchOrRemoval(t *testing.T) {
	for _, name := range []string{"metadata.git", "metadata/gitdir"} {
		t.Run(name, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			separate := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(filepath.Dir(separate), 0o755); err != nil {
				t.Fatal(err)
			}
			prototypeGit(t, root, "init", "--separate-git-dir", separate)
			linked := filepath.Join(t.TempDir(), "linked")
			prototypeGit(t, root, "worktree", "add", "-b", "linked-caller", linked)
			before := prototypeGit(t, root, "show-ref")
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if args[0] != "rev-parse" {
					t.Fatalf("command before layout refusal: %v", args)
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			_, createErr := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, run)
			_, removeErr := RemovePrototype(context.Background(), root, "widget", run)
			for _, err := range []error{createErr, removeErr} {
				if err == nil || !strings.Contains(err.Error(), "unsupported repository layout") || !strings.Contains(err.Error(), separate) {
					t.Fatalf("layout refusal: %v", err)
				}
			}
			if prototypeGit(t, root, "show-ref") != before {
				t.Fatal("layout refusal changed refs")
			}
			_, createErr = CreatePrototype(context.Background(), linked, cfg, "widget", idea, prototypeTestRoute, run)
			_, removeErr = RemovePrototype(context.Background(), linked, "widget", run)
			for _, err := range []error{createErr, removeErr} {
				if err == nil || !strings.Contains(err.Error(), "unsupported repository layout") {
					t.Fatalf("linked layout refusal: %v", err)
				}
			}
		})
	}
}

func TestPrototypeSeparateGitDirectoryNamedDotGitUsesReportedMainCheckout(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(parent, "reported-main")
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	prototypeGit(t, root, "init", "--separate-git-dir", filepath.Join(main, ".git"))
	linked := filepath.Join(t.TempDir(), "linked")
	prototypeGit(t, root, "worktree", "add", "-b", "linked-caller", linked)
	if top := prototypeGit(t, main, "rev-parse", "--show-toplevel"); top != main {
		t.Fatalf("Git reports main checkout %q, want %q", top, main)
	}
	callers := []string{root, main, linked}
	wantPath := filepath.Join(filepath.Dir(main), ".pyxis-worktrees", filepath.Base(main), "prototype-widget")
	for i, caller := range callers {
		refs := prototypeGit(t, root, "show-ref", "--heads")
		before := prototypeGit(t, caller, "status", "--porcelain")
		p, err := CreatePrototype(context.Background(), caller, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
		if err != nil {
			t.Fatalf("create from %s: %v", caller, err)
		}
		if p.Worktree != wantPath || p.Branch != "prototype/widget" || p.Base != prototypeGit(t, root, "rev-parse", "FETCH_HEAD") {
			t.Fatalf("prototype location or base: %+v; want path %s", p, wantPath)
		}
		if prototypeGit(t, p.Worktree, "branch", "--show-current") != p.Branch || prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != p.Base {
			t.Fatal("prototype does not use its own branch at fetched mainline")
		}
		if upstream := prototypeGit(t, root, "for-each-ref", "--format=%(upstream)", "refs/heads/"+p.Branch); upstream != "" {
			t.Fatalf("prototype has upstream %s", upstream)
		}
		state := p.Worktree + "-runtime"
		if err := os.MkdirAll(state, 0o700); err != nil {
			t.Fatal(err)
		}
		prototypeWrite(t, filepath.Join(state, "keep"), "prototype runtime state")
		removed, err := RemovePrototype(context.Background(), callers[(i+1)%len(callers)], "widget", draftshell.Run)
		if err != nil || !removed {
			t.Fatalf("remove from another checkout: %t %v", removed, err)
		}
		assertNoPrototype(t, caller)
		if _, err := os.Lstat(state); !os.IsNotExist(err) {
			t.Fatalf("runtime state remains: %v", err)
		}
		if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, caller, "status", "--porcelain") != before {
			t.Fatal("prototype operation changed existing branches or invoking checkout")
		}
	}
}

func TestPrototypeCreationAndConfirmationFailureCleanup(t *testing.T) {
	for _, failure := range []string{"before creation", "partial creation", "after creation", "confirmation error", "wrong confirmation", "upstream inspection"} {
		t.Run(failure, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			draftBefore := prototypeGit(t, root, "rev-parse", "contracts/widget")
			cause := errors.New("injected worktree failure")
			attempts, confirmations := 0, 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if failure == "upstream inspection" && strings.Join(args, " ") == "for-each-ref --format=%(upstream) refs/heads/prototype/widget" {
					return nil, cause
				}
				if strings.HasPrefix(strings.Join(args, " "), "worktree add ") {
					attempts++
					switch failure {
					case "before creation":
						return nil, cause
					case "partial creation":
						prototypeGit(t, root, "branch", "prototype/widget")
						if err := os.MkdirAll(args[len(args)-2], 0o755); err != nil {
							t.Fatal(err)
						}
						return nil, cause
					case "after creation":
						if _, err := draftshell.Run(ctx, dir, env, name, args...); err != nil {
							t.Fatal(err)
						}
						return nil, cause
					}
				}
				if attempts > 0 && strings.HasPrefix(strings.Join(args, " "), "worktree list ") {
					confirmations++
					if confirmations == 1 && failure == "confirmation error" {
						return nil, cause
					}
					if confirmations == 1 && failure == "wrong confirmation" {
						out, err := draftshell.Run(ctx, dir, env, name, args...)
						return []byte(strings.ReplaceAll(string(out), "refs/heads/prototype/widget", "refs/heads/wrong")), err
					}
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			_, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, run)
			if err == nil || attempts != 1 || failure != "wrong confirmation" && !errors.Is(err, cause) {
				t.Fatalf("failure not reported without retry: %v, attempts=%d", err, attempts)
			}
			assertNoPrototype(t, root)
			if prototypeGit(t, root, "rev-parse", "contracts/widget") != draftBefore {
				t.Fatal("failure changed draft branch")
			}
		})
	}
}

func TestPrototypeRemoveRefusalsPreserveData(t *testing.T) {
	for _, state := range []string{"dirty", "other branch", "detached", "unregistered directory", "branch only"} {
		t.Run(state, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			p, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			switch state {
			case "dirty":
				prototypeWrite(t, filepath.Join(p.Worktree, "untracked.txt"), "keep")
				prototypeWrite(t, filepath.Join(p.Worktree, ".pyxis/templates/prd-template.md"), "dirty tracked file")
			case "other branch":
				prototypeGit(t, p.Worktree, "switch", "-c", "other")
			case "detached":
				prototypeGit(t, p.Worktree, "checkout", "--detach")
			case "unregistered directory", "branch only":
				prototypeGit(t, root, "worktree", "remove", p.Worktree)
				if state == "unregistered directory" {
					if err := os.MkdirAll(p.Worktree, 0o755); err != nil {
						t.Fatal(err)
					}
					prototypeWrite(t, filepath.Join(p.Worktree, "keep"), "keep")
				}
			}
			if err := os.MkdirAll(p.Worktree+"-runtime", 0o700); err != nil {
				t.Fatal(err)
			}
			prototypeWrite(t, p.Worktree+"-runtime/keep", "keep")
			refs := prototypeGit(t, root, "show-ref")
			worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain")
			removed, err := RemovePrototype(context.Background(), root, "widget", draftshell.Run)
			if err == nil || removed {
				t.Fatalf("unsafe removal accepted: %t %v", removed, err)
			}
			if state == "dirty" && (!strings.Contains(err.Error(), "untracked.txt") || !strings.Contains(err.Error(), "prd-template.md")) {
				t.Fatalf("dirty refusal missing filenames: %v", err)
			}
			if prototypeGit(t, root, "show-ref") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktrees {
				t.Fatal("refusal changed branches or registrations")
			}
			if _, err := os.Stat(p.Worktree + "-runtime/keep"); err != nil {
				t.Fatalf("runtime state changed: %v", err)
			}
			if state != "branch only" {
				if _, err := os.Stat(p.Worktree); err != nil {
					t.Fatalf("worktree removed: %v", err)
				}
			}
		})
	}
}

func TestPrototypeRemoveDoesNotDeleteAnotherPrototypeAsRuntimeState(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	p, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	prototypeGit(t, root, "branch", "contracts/widget-runtime")
	other, err := CreatePrototype(context.Background(), root, cfg, "widget-runtime", idea, prototypeTestRoute, draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := RemovePrototype(context.Background(), root, "widget", draftshell.Run)
	if removed || err == nil || !strings.Contains(err.Error(), "another registered worktree") {
		t.Fatalf("runtime path collision not refused: %t %v", removed, err)
	}
	for _, worktree := range []PrototypeResult{p, other} {
		registered, branch, _, err := prototypeRegistration(context.Background(), root, worktree.Worktree, draftshell.Run)
		if err != nil || !registered || branch != "refs/heads/"+worktree.Branch {
			t.Fatalf("prototype changed: %+v %t %s %v", worktree, registered, branch, err)
		}
	}
}

func TestPrototypeCreatePreservesStaleRegistration(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	prototypeGit(t, root, "worktree", "add", "--detach", p.Worktree, "HEAD")
	if err := os.RemoveAll(p.Worktree); err != nil {
		t.Fatal(err)
	}
	before := prototypeGit(t, root, "worktree", "list", "--porcelain")
	_, err = CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("stale registration not refused: %v", err)
	}
	if prototypeGit(t, root, "worktree", "list", "--porcelain") != before {
		t.Fatal("creation cleaned up a registration it did not create")
	}
}

func TestPrototypeCreateGitFailuresStopBeforeCreation(t *testing.T) {
	for _, command := range []string{
		"rev-parse --path-format=absolute --git-common-dir",
		"for-each-ref --format=%(refname) refs/heads/contracts/widget",
		"fetch --refmap= origin main",
		"rev-parse --verify FETCH_HEAD^{commit}",
		"for-each-ref --format=%(refname) refs/heads/prototype/widget",
		"worktree list --porcelain -z",
	} {
		t.Run(command, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			refs := prototypeGit(t, root, "show-ref")
			cause := errors.New("injected Git failure")
			failed := false
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if failed || len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
					t.Fatalf("command after failure or unexpected creation: %v", args)
				}
				if strings.Join(args, " ") == command {
					failed = true
					return nil, cause
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			_, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, run)
			if !failed || !errors.Is(err, cause) {
				t.Fatalf("Git failure lost: %v", err)
			}
			assertNoPrototype(t, root)
			if prototypeGit(t, root, "show-ref") != refs {
				t.Fatal("Git failure changed refs")
			}
		})
	}
}

func TestPrototypeLocationRejectsUnresolvableLayout(t *testing.T) {
	for _, failure := range []string{"missing adapter", "missing common directory", "main inspection", "missing main checkout", "different main checkout"} {
		t.Run(failure, func(t *testing.T) {
			root, _, _ := prototypeFixture(t)
			var run RunCommand = func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				mainProbe := strings.Join(args, " ") == "rev-parse --show-toplevel"
				switch {
				case !mainProbe && failure == "missing common directory":
					return []byte(filepath.Join(root, "missing", ".git") + "\n"), nil
				case mainProbe && failure == "main inspection":
					return nil, errors.New("cannot inspect layout")
				case mainProbe && failure == "missing main checkout":
					return []byte(filepath.Join(root, "missing") + "\n"), nil
				case mainProbe && failure == "different main checkout":
					return []byte(filepath.Dir(root) + "\n"), nil
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			if failure == "missing adapter" {
				run = nil
			}
			if _, err := prototypeLocation(context.Background(), root, "widget", run); err == nil {
				t.Fatal("unresolvable repository layout accepted")
			}
			assertNoPrototype(t, root)
		})
	}
}

func TestPrototypeRemoveGitFailuresPreserveRemainingResources(t *testing.T) {
	for _, failure := range []string{"registration", "branch inspection", "state registration", "branch deletion"} {
		t.Run(failure, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			if failure != "branch inspection" {
				p, err = CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
			}
			cause := errors.New("injected removal failure")
			registrations, failures := 0, 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				command := strings.Join(args, " ")
				if command == "worktree list --porcelain -z" {
					registrations++
				}
				if failure == "registration" && registrations == 1 || failure == "state registration" && registrations == 2 || failure == "branch inspection" && args[0] == "for-each-ref" || failure == "branch deletion" && args[0] == "branch" {
					failures++
					return nil, cause
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			removed, err := RemovePrototype(context.Background(), root, "widget", run)
			if removed || !errors.Is(err, cause) || failures != 1 {
				t.Fatalf("removal failure lost or retried: removed=%t err=%v failures=%d", removed, err, failures)
			}
			if failure != "branch inspection" {
				if exists, err := prototypeBranchExists(context.Background(), root, p.Branch, draftshell.Run); err != nil || !exists {
					t.Fatalf("remaining branch lost: %t %v", exists, err)
				}
				_, statErr := os.Stat(p.Worktree)
				if failure == "branch deletion" && !os.IsNotExist(statErr) || failure != "branch deletion" && statErr != nil {
					t.Fatalf("unexpected worktree state after failure: %v", statErr)
				}
			}
		})
	}
}

func TestPrototypeSymlinkedWorktreeParents(t *testing.T) {
	for _, linkAt := range []string{"worktree root", "repository directory"} {
		for _, failure := range []string{"none", "creation", "confirmation", "cancelled confirmation"} {
			t.Run(linkAt+"/"+failure, func(t *testing.T) {
				root, idea, cfg := prototypeFixture(t)
				p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
				physical, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				link := filepath.Dir(filepath.Dir(p.Worktree))
				if linkAt == "repository directory" {
					if err := os.MkdirAll(link, 0o755); err != nil {
						t.Fatal(err)
					}
					link = filepath.Dir(p.Worktree)
				}
				if err := os.Symlink(physical, link); err != nil {
					t.Fatal(err)
				}
				prototypeWrite(t, filepath.Join(physical, "keep"), "unrelated parent data")
				before := prototypeGit(t, root, "show-ref", "--heads")
				cause := errors.New("injected prototype failure")
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				if failure == "cancelled confirmation" {
					cause = context.Canceled
				}
				attempts := 0
				failed := false
				run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					command := strings.Join(args, " ")
					if strings.HasPrefix(command, "worktree add ") {
						attempts++
						if failure == "creation" {
							if _, err := draftshell.Run(ctx, dir, env, name, args...); err != nil {
								t.Fatal(err)
							}
							return nil, cause
						}
					}
					if failure == "confirmation" && attempts > 0 && !failed && command == "worktree list --porcelain -z" {
						failed = true
						return nil, cause
					}
					if failure == "cancelled confirmation" && attempts > 0 && !failed && command == "worktree list --porcelain -z" {
						failed = true
						out, err := draftshell.Run(ctx, dir, env, name, args...)
						cancel()
						return out, err
					}
					return draftshell.Run(ctx, dir, env, name, args...)
				}
				result, err := CreatePrototype(ctx, root, cfg, "widget", idea, prototypeTestRoute, run)
				if attempts != 1 {
					t.Fatalf("creation attempted %d times", attempts)
				}
				if failure == "none" {
					if err != nil || result.Worktree != p.Worktree {
						t.Fatalf("create through symlink: %+v %v", result, err)
					}
					physicalWorktree, err := filepath.EvalSymlinks(p.Worktree)
					if err != nil {
						t.Fatal(err)
					}
					if physicalWorktree == p.Worktree || !strings.Contains(prototypeGit(t, root, "worktree", "list", "--porcelain", "-z"), "worktree "+physicalWorktree+"\x00") {
						t.Fatal("fixture did not register a different physical path")
					}
					if err := os.MkdirAll(p.Worktree+"-runtime", 0o700); err != nil {
						t.Fatal(err)
					}
					prototypeWrite(t, p.Worktree+"-runtime/state", "runtime data")
					if removed, err := RemovePrototype(context.Background(), root, "widget", draftshell.Run); err != nil || !removed {
						t.Fatalf("remove through symlink: %t %v", removed, err)
					}
					if _, err := os.Stat(p.Worktree + "-runtime"); !os.IsNotExist(err) {
						t.Fatalf("runtime state remains: %v", err)
					}
				} else if !errors.Is(err, cause) {
					t.Fatalf("creation failure lost: %v", err)
				}
				assertNoPrototype(t, root)
				if strings.Contains(prototypeGit(t, root, "worktree", "list", "--porcelain"), "prototype-widget") {
					t.Fatal("physical prototype registration remains")
				}
				if prototypeGit(t, root, "show-ref", "--heads") != before {
					t.Fatal("existing branches changed")
				}
				if target, err := os.Readlink(link); err != nil || target != physical {
					t.Fatalf("parent symlink changed: %q %v", target, err)
				}
				if body, err := os.ReadFile(filepath.Join(physical, "keep")); err != nil || string(body) != "unrelated parent data" {
					t.Fatalf("parent data changed: %q %v", body, err)
				}
			})
		}
	}
}

func TestPrototypeRemoveRefusesInvokingCheckout(t *testing.T) {
	for _, invokeFrom := range []string{"checkout", "subdirectory", "alias", "inspection error", "unresolved checkout"} {
		t.Run(invokeFrom, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			p, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			caller := p.Worktree
			switch invokeFrom {
			case "subdirectory":
				caller = filepath.Join(p.Worktree, ".pyxis", "templates")
			case "alias":
				caller = filepath.Join(t.TempDir(), "prototype-alias")
				if err := os.Symlink(p.Worktree, caller); err != nil {
					t.Fatal(err)
				}
			}
			state := p.Worktree + "-runtime"
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}
			prototypeWrite(t, filepath.Join(state, "keep"), "runtime data")
			refs := prototypeGit(t, root, "show-ref", "--heads")
			worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain")
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if dir == caller && strings.Join(args, " ") == "rev-parse --show-toplevel" {
					switch invokeFrom {
					case "inspection error":
						return nil, errors.New("injected checkout inspection error")
					case "unresolved checkout":
						return []byte(filepath.Join(root, "missing") + "\n"), nil
					}
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			removed, err := RemovePrototype(context.Background(), caller, "widget", run)
			if removed || err == nil || !strings.Contains(err.Error(), "invoking checkout") {
				t.Fatalf("self-removal not refused: %t %v", removed, err)
			}
			if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktrees || prototypeGit(t, p.Worktree, "status", "--porcelain") != "" {
				t.Fatal("self-removal changed the clean prototype or its registration")
			}
			if body, err := os.ReadFile(filepath.Join(state, "keep")); err != nil || string(body) != "runtime data" {
				t.Fatalf("runtime state changed: %q %v", body, err)
			}
			if removed, err := RemovePrototype(context.Background(), root, "widget", draftshell.Run); err != nil || !removed {
				t.Fatalf("removal from another checkout: %t %v", removed, err)
			}
			assertNoPrototype(t, root)
		})
	}
}

func TestPrototypeSymlinkedMissingParentPreservesStaleRegistration(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	physical := t.TempDir()
	if err := os.Symlink(physical, filepath.Dir(filepath.Dir(p.Worktree))); err != nil {
		t.Fatal(err)
	}
	prototypeGit(t, root, "worktree", "add", "--detach", p.Worktree, "HEAD")
	physicalWorktree, err := filepath.EvalSymlinks(p.Worktree)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(physicalWorktree)); err != nil {
		t.Fatal(err)
	}
	before := prototypeGit(t, root, "worktree", "list", "--porcelain")
	_, err = CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("stale physical registration not refused: %v", err)
	}
	if prototypeGit(t, root, "worktree", "list", "--porcelain") != before {
		t.Fatal("creation changed a registration it did not create")
	}
	if _, err := os.Lstat(filepath.Dir(p.Worktree)); !os.IsNotExist(err) {
		t.Fatalf("preflight created a worktree parent: %v", err)
	}
}

func TestPrototypeRemoveProtectsNestedInvokingCheckout(t *testing.T) {
	for _, location := range []string{"runtime child", "runtime descendant", "prototype child", "runtime prefix sibling"} {
		t.Run(location, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			p, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			state := p.Worktree + "-runtime"
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}
			prototypeWrite(t, filepath.Join(state, "keep"), "runtime data")
			caller := filepath.Join(state, "caller")
			switch location {
			case "runtime descendant":
				caller = filepath.Join(state, "nested", "caller")
			case "prototype child":
				caller = filepath.Join(p.Worktree, "caller")
			case "runtime prefix sibling":
				caller = filepath.Join(state+"-other", "caller")
			}
			prototypeGit(t, root, "worktree", "add", "-b", "caller", caller, "HEAD")
			refs := prototypeGit(t, root, "show-ref", "--heads")
			worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain")
			status := prototypeGit(t, caller, "status", "--porcelain")
			head := prototypeGit(t, caller, "rev-parse", "HEAD")
			removed, err := RemovePrototype(context.Background(), caller, "widget", draftshell.Run)
			if location == "runtime prefix sibling" {
				if !removed || err != nil {
					t.Fatalf("unrelated sibling blocked removal: %t %v", removed, err)
				}
				assertNoPrototype(t, root)
			} else {
				if removed || err == nil || !strings.Contains(err.Error(), "invoking checkout") {
					t.Fatalf("nested invoking checkout not protected: %t %v", removed, err)
				}
				if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktrees {
					t.Fatal("refusal changed branches or registrations")
				}
				if body, err := os.ReadFile(filepath.Join(state, "keep")); err != nil || string(body) != "runtime data" {
					t.Fatalf("runtime data changed: %q %v", body, err)
				}
			}
			if prototypeGit(t, caller, "rev-parse", "HEAD") != head || prototypeGit(t, caller, "status", "--porcelain") != status {
				t.Fatal("invoking checkout changed")
			}
		})
	}
}

func TestPrototypeCreateRefusesDestinationInsideInvokingCheckout(t *testing.T) {
	for _, callerKind := range []string{"main", "linked"} {
		for _, linkAt := range []string{"worktree root", "repository directory"} {
			t.Run(callerKind+"/"+linkAt, func(t *testing.T) {
				root, idea, cfg := prototypeFixture(t)
				caller := root
				if callerKind == "linked" {
					caller = filepath.Join(filepath.Dir(root), "caller")
					prototypeGit(t, root, "worktree", "add", "-b", "caller", caller, "HEAD")
				}
				p, err := prototypeLocation(context.Background(), caller, "widget", draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
				link := filepath.Dir(filepath.Dir(p.Worktree))
				if linkAt == "repository directory" {
					if err := os.MkdirAll(link, 0o755); err != nil {
						t.Fatal(err)
					}
					link = filepath.Dir(p.Worktree)
				}
				if err := os.Symlink(caller, link); err != nil {
					t.Fatal(err)
				}
				refs := prototypeGit(t, root, "show-ref", "--heads")
				worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain")
				status := prototypeGit(t, caller, "status", "--porcelain")
				_, err = CreatePrototype(context.Background(), caller, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
				if err == nil || !strings.Contains(err.Error(), "invoking checkout") {
					t.Fatalf("overlapping destination not refused: %v", err)
				}
				assertNoPrototype(t, root)
				if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktrees || prototypeGit(t, caller, "status", "--porcelain") != status {
					t.Fatal("refusal changed invoking checkout, branches, or registrations")
				}
				if _, err := os.Lstat(filepath.Dir(p.Worktree)); linkAt == "worktree root" && !os.IsNotExist(err) {
					t.Fatalf("creation wrote a directory in the invoking checkout: %v", err)
				}
			})
		}
	}
}

func TestPrototypeCreateRefusesRegisteredParent(t *testing.T) {
	for _, linked := range []bool{false, true} {
		for _, layout := range []string{"direct", "symlinked worktree root", "symlinked repository directory"} {
			name := "main/" + layout
			if linked {
				name = "linked/" + layout
			}
			t.Run(name, func(t *testing.T) {
				root, idea, cfg := prototypeFixture(t)
				p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
				other := filepath.Dir(p.Worktree)
				if layout != "direct" {
					other = filepath.Join(t.TempDir(), "other")
				}
				prototypeGit(t, root, "worktree", "add", "-b", "other", other, "HEAD")
				if layout != "direct" {
					link := filepath.Dir(filepath.Dir(p.Worktree))
					if layout == "symlinked repository directory" {
						if err := os.MkdirAll(link, 0o755); err != nil {
							t.Fatal(err)
						}
						link = filepath.Dir(p.Worktree)
					}
					if err := os.Symlink(other, link); err != nil {
						t.Fatal(err)
					}
				}
				prototypeWrite(t, filepath.Join(other, "keep.txt"), "committed work")
				prototypeGit(t, other, "add", "keep.txt")
				prototypeGit(t, other, "commit", "-qm", "existing checkout work")
				caller := root
				if linked {
					caller = filepath.Join(filepath.Dir(root), "caller")
					prototypeGit(t, root, "worktree", "add", "-b", "caller", caller, "HEAD")
				}
				prototypeWrite(t, filepath.Join(caller, "uncommitted.txt"), "caller work")
				refs := prototypeGit(t, root, "show-ref", "--heads")
				registrations := prototypeGit(t, root, "worktree", "list", "--porcelain", "-z")
				callerStatus := prototypeGit(t, caller, "status", "--porcelain")
				otherStatus := prototypeGit(t, other, "status", "--porcelain")
				_, parentErr := os.Lstat(filepath.Dir(p.Worktree))
				_, err = CreatePrototype(context.Background(), caller, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
				if err == nil || !strings.Contains(err.Error(), "registered worktree") {
					t.Fatalf("registered parent not refused before creation: %v", err)
				}
				assertNoPrototype(t, root)
				if _, err := os.Lstat(filepath.Dir(p.Worktree)); os.IsNotExist(parentErr) && !os.IsNotExist(err) {
					t.Fatalf("creation refusal created the worktree parent: %v", err)
				}
				if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain", "-z") != registrations || prototypeGit(t, caller, "status", "--porcelain") != callerStatus || prototypeGit(t, other, "status", "--porcelain") != otherStatus {
					t.Fatal("creation refusal changed branches, registrations, or checkout files")
				}
				if body, err := os.ReadFile(filepath.Join(other, "keep.txt")); err != nil || string(body) != "committed work" {
					t.Fatalf("enclosing checkout data changed: %q %v", body, err)
				}
			})
		}
	}
}

func TestPrototypeRemovePreservesRegisteredRuntimeDescendants(t *testing.T) {
	for _, location := range []string{"child", "descendant", "symlinked parent"} {
		t.Run(location, func(t *testing.T) {
			root, idea, cfg := prototypeFixture(t)
			if location == "symlinked parent" {
				p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Dir(filepath.Dir(p.Worktree))); err != nil {
					t.Fatal(err)
				}
			}
			p, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
			if err != nil {
				t.Fatal(err)
			}
			state := p.Worktree + "-runtime"
			if err := os.MkdirAll(state, 0o700); err != nil {
				t.Fatal(err)
			}
			prototypeWrite(t, filepath.Join(state, "keep"), "runtime data")
			other := filepath.Join(state, "other")
			if location == "descendant" {
				other = filepath.Join(state, "nested", "other")
			}
			prototypeGit(t, root, "worktree", "add", "-b", "other", other, "HEAD")
			prototypeWrite(t, filepath.Join(other, "uncommitted.txt"), "uncommitted work")
			refs := prototypeGit(t, root, "show-ref", "--heads")
			worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain")
			status := prototypeGit(t, other, "status", "--porcelain")
			removed, err := RemovePrototype(context.Background(), root, "widget", draftshell.Run)
			if removed || err == nil || !strings.Contains(err.Error(), "registered worktree") {
				t.Fatalf("registered runtime descendant not protected: %t %v", removed, err)
			}
			if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain") != worktrees || prototypeGit(t, other, "status", "--porcelain") != status {
				t.Fatal("refusal changed a branch, registration, or unrelated worktree")
			}
			for path, want := range map[string]string{filepath.Join(state, "keep"): "runtime data", filepath.Join(other, "uncommitted.txt"): "uncommitted work"} {
				if body, err := os.ReadFile(path); err != nil || string(body) != want {
					t.Fatalf("data changed at %s: %q %v", path, body, err)
				}
			}
		})
	}
}

func TestPrototypeRemovePreservesOverlappingWorktrees(t *testing.T) {
	for _, location := range []string{"ignored prototype child", "runtime ancestor"} {
		for _, symlinked := range []bool{false, true} {
			name := location
			if symlinked {
				name += "/symlinked parent"
			}
			t.Run(name, func(t *testing.T) {
				root, idea, cfg := prototypeFixture(t)
				p, err := prototypeLocation(context.Background(), root, "widget", draftshell.Run)
				if err != nil {
					t.Fatal(err)
				}
				if symlinked {
					if err := os.Symlink(t.TempDir(), filepath.Dir(filepath.Dir(p.Worktree))); err != nil {
						t.Fatal(err)
					}
				}
				state := p.Worktree + "-runtime"
				other := filepath.Dir(p.Worktree)
				if location == "runtime ancestor" {
					prototypeGit(t, root, "worktree", "add", "-b", "other", other, "HEAD")
					prototypeWrite(t, filepath.Join(other, ".gitignore"), "prototype-widget/\n")
					// Model an unsafe layout made by hand; creation now refuses it.
					prototypeGit(t, root, "worktree", "add", "--no-track", "-b", p.Branch, p.Worktree, "HEAD")
				} else {
					p, err = CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
					if err != nil {
						t.Fatal(err)
					}
				}
				if err := os.MkdirAll(state, 0o700); err != nil {
					t.Fatal(err)
				}
				sentinel := filepath.Join(state, "keep")
				prototypeWrite(t, sentinel, "runtime data")
				if location == "ignored prototype child" {
					prototypeWrite(t, filepath.Join(p.Worktree, ".gitignore"), "dependency/\n")
					prototypeGit(t, p.Worktree, "add", ".gitignore")
					prototypeGit(t, p.Worktree, "commit", "-qm", "ignore dependency checkout")
					other = filepath.Join(p.Worktree, "dependency")
					prototypeGit(t, root, "worktree", "add", "-b", "other", other, "HEAD")
					prototypeWrite(t, filepath.Join(other, "uncommitted.txt"), "uncommitted work")
				} else {
					prototypeGit(t, other, "add", ".gitignore", "prototype-widget-runtime/keep")
					prototypeGit(t, other, "commit", "-qm", "tracked file in runtime directory")
				}
				if status := prototypeGit(t, p.Worktree, "status", "--porcelain"); status != "" {
					t.Fatalf("prototype must appear clean to Git: %s", status)
				}
				refs := prototypeGit(t, root, "show-ref", "--heads")
				worktrees := prototypeGit(t, root, "worktree", "list", "--porcelain", "-z")
				callerStatus := prototypeGit(t, root, "status", "--porcelain")
				otherStatus := prototypeGit(t, other, "status", "--porcelain")
				run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					if strings.HasPrefix(strings.Join(args, " "), "worktree remove ") {
						t.Fatal("removal reached Git before refusing overlapping worktrees")
					}
					return draftshell.Run(ctx, dir, env, name, args...)
				}
				removed, err := RemovePrototype(context.Background(), root, "widget", run)
				if removed || err == nil || !strings.Contains(err.Error(), "registered worktree") {
					t.Fatalf("overlapping worktree not protected: %t %v", removed, err)
				}
				if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "worktree", "list", "--porcelain", "-z") != worktrees || prototypeGit(t, root, "status", "--porcelain") != callerStatus || prototypeGit(t, other, "status", "--porcelain") != otherStatus {
					t.Fatal("refusal changed branches, registrations, or checkout files")
				}
				if body, err := os.ReadFile(sentinel); err != nil || string(body) != "runtime data" {
					t.Fatalf("runtime data changed: %q %v", body, err)
				}
				if location == "ignored prototype child" {
					if body, err := os.ReadFile(filepath.Join(other, "uncommitted.txt")); err != nil || string(body) != "uncommitted work" {
						t.Fatalf("nested checkout data changed: %q %v", body, err)
					}
				}
			})
		}
	}
}

func TestPrototypeCreateRefusesInheritedUpstream(t *testing.T) {
	root, idea, cfg := prototypeFixture(t)
	// Branch metadata can outlive the branch ref; --no-track does not clear it.
	prototypeGit(t, root, "config", "branch.prototype/widget.remote", "origin")
	prototypeGit(t, root, "config", "branch.prototype/widget.merge", "refs/heads/main")
	refs := prototypeGit(t, root, "show-ref", "--heads")
	status := prototypeGit(t, root, "status", "--porcelain")
	attempts := 0
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if strings.HasPrefix(strings.Join(args, " "), "worktree add ") {
			attempts++
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	_, err := CreatePrototype(context.Background(), root, cfg, "widget", idea, prototypeTestRoute, run)
	if err == nil || !strings.Contains(err.Error(), "upstream") || !strings.Contains(err.Error(), "origin/main") || attempts != 1 {
		t.Fatalf("inherited upstream not reported without retry: %v attempts=%d", err, attempts)
	}
	assertNoPrototype(t, root)
	if prototypeGit(t, root, "show-ref", "--heads") != refs || prototypeGit(t, root, "status", "--porcelain") != status {
		t.Fatal("failed creation changed draft branch or invoking checkout")
	}
}
