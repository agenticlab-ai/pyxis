package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig/harnessconfigtest"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type prototypeCLIRuntime struct {
	authErr error
}

func (r prototypeCLIRuntime) AuthCheck(context.Context) error { return r.authErr }
func (prototypeCLIRuntime) Launch(context.Context, port.LaunchSpec) (port.Handle, error) {
	return prototypeCLIHandle{}, nil
}
func (prototypeCLIRuntime) Await(context.Context, port.Handle) (port.RunResult, error) {
	return port.RunResult{}, nil
}
func (prototypeCLIRuntime) Kill(context.Context, port.Handle) error { return nil }

type prototypeCLIHandle struct{}

func (prototypeCLIHandle) RuntimeHandle() {}

func prototypeCLIAuthentication(t *testing.T) {
	t.Helper()
	login := t.TempDir()
	t.Setenv("CODEX_HOME", login)
	t.Setenv("CLAUDE_CONFIG_DIR", login)
	writeFile(t, filepath.Join(login, ".credentials.json"), []byte(`{}`))
}

func prototypeCLIRouteFixture(t *testing.T, root string) {
	t.Helper()
	prototypeCLIAuthentication(t)
	writeFile(t, filepath.Join(root, ".pyxis/config.local.yaml"), []byte(harnessconfigtest.YAML+"prototype:\n  harness: codex\n  profile: codex-implementation-economy\n"))
	swap(t, &draftPrototypeRuntime, func(entity.Harness, executorconfig.Config, harnessconfig.Config) (port.AgentRuntime, error) {
		return prototypeCLIRuntime{}, nil
	})
}

func TestDraftPrototypeCLI(t *testing.T) {
	root, _ := localGitCLIFixture(t, true)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	localGitCLI(t, root, "branch", "draft/widget")
	prototypeCLIRouteFixture(t, root)
	idea := filepath.Join(t.TempDir(), "idea.txt")
	if err := os.WriteFile(idea, []byte("Try the idea."), 0o600); err != nil {
		t.Fatal(err)
	}
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	swap(t, &draftCommand, draftshell.Run)
	var out, diagnostic bytes.Buffer
	if code := draftVerb([]string{"prototype", "widget", "--idea", idea}, &out, &diagnostic); code != 0 || diagnostic.Len() != 0 {
		t.Fatalf("create: %d %s %s", code, &out, &diagnostic)
	}
	path := filepath.Join(filepath.Dir(root), ".pyxis-worktrees", filepath.Base(root), "prototype-widget")
	want := "worktree: " + path + "\nbranch: prototype/widget\nbase: " + localGitCLI(t, root, "rev-parse", "upstream/trunk") + "\nroute: configured codex codex-implementation-economy\n"
	want += "model: gpt-5.6-luna\neffort: low\noutcome: finished\nhead: " + localGitCLI(t, path, "rev-parse", "HEAD") + "\nfindings: missing\nusage: tokens=unavailable turns=unavailable tool_calls=unavailable\n"
	if out.String() != want {
		t.Fatalf("output %q, want %q", out.String(), want)
	}
	for _, want := range []string{"prototype removed\n", "nothing to remove\n"} {
		out.Reset()
		if code := draftVerb([]string{"prototype", "widget", "--remove"}, &out, &diagnostic); code != 0 || out.String() != want || diagnostic.Len() != 0 {
			t.Fatalf("remove: %d %s %s", code, &out, &diagnostic)
		}
	}
	for _, args := range [][]string{
		{"prototype"}, {"prototype", "widget"}, {"prototype", "--bad", "--remove"},
		{"prototype", "widget", "--idea"}, {"prototype", "widget", "--idea", ""},
		{"prototype", "widget", "--remove", "--idea", idea},
	} {
		out.Reset()
		diagnostic.Reset()
		if code := draftVerb(args, &out, &diagnostic); code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "usage:") {
			t.Fatalf("invalid args %v: %d %s %s", args, code, &out, &diagnostic)
		}
	}
	for _, args := range [][]string{{"prototype", "Bad", "--idea", idea}, {"prototype", "Bad", "--remove"}, {"prototype", "missing", "--idea", idea}} {
		out.Reset()
		diagnostic.Reset()
		if code := draftVerb(args, &out, &diagnostic); code != 1 || out.Len() != 0 || diagnostic.Len() == 0 {
			t.Fatalf("refusal %v: %d %s %s", args, code, &out, &diagnostic)
		}
	}
	swap(t, &draftCommand, func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if args[0] == "worktree" && args[1] == "add" {
			return nil, errors.New("injected creation error")
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"prototype", "widget", "--idea", idea}, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "injected creation error") {
		t.Fatalf("creation failure: %d %s %s", code, &out, &diagnostic)
	}
}

func TestDraftPrototypeCLIContextFailures(t *testing.T) {
	for _, failure := range []string{"repository", "configuration"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			swap(t, &draftRepoRoot, func() (string, error) {
				if failure == "repository" {
					return "", errors.New("repository unavailable")
				}
				return root, nil
			})
			if failure == "configuration" {
				if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("unknown_prototype_key: true\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			swap(t, &draftCommand, func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Fatal("Git ran despite invalid invocation context")
				return nil, nil
			})
			for _, flags := range [][]string{{"--idea", "idea.txt"}, {"--remove"}} {
				var out, diagnostic bytes.Buffer
				args := append([]string{"prototype", "widget"}, flags...)
				if code := draftVerb(args, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), failure) {
					t.Fatalf("context failure: exit=%d out=%s err=%s", code, &out, &diagnostic)
				}
			}
		})
	}
}

func TestDraftPrototypeCLIRemoveRefusesInitializedSubmodule(t *testing.T) {
	root, _ := localGitCLIFixture(t, true)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	localGitCLI(t, root, "branch", "draft/widget")
	prototypeCLIRouteFixture(t, root)
	idea := filepath.Join(t.TempDir(), "idea.txt")
	if err := os.WriteFile(idea, []byte("Try the idea."), 0o600); err != nil {
		t.Fatal(err)
	}
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	swap(t, &draftCommand, draftshell.Run)
	var out, diagnostic bytes.Buffer
	if code := draftVerb([]string{"prototype", "widget", "--idea", idea}, &out, &diagnostic); code != 0 {
		t.Fatalf("create: %d %s", code, &diagnostic)
	}
	path := filepath.Join(filepath.Dir(root), ".pyxis-worktrees", filepath.Base(root), "prototype-widget")
	// Clone the fixture's committed mainline as the prototype's dependency.
	localGitCLI(t, path, "-c", "protocol.file.allow=always", "submodule", "add", root, "dependency")
	localGitCLI(t, path, "commit", "-qm", "prototype with initialized submodule")
	if status := localGitCLI(t, path, "status", "--porcelain", "--ignore-submodules=none"); status != "" {
		t.Fatalf("prototype is not clean: %s", status)
	}
	state := path + "-runtime"
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(state, "keep")
	if err := os.WriteFile(sentinel, []byte("runtime data"), 0o600); err != nil {
		t.Fatal(err)
	}
	refs := localGitCLI(t, root, "show-ref", "--heads")
	worktrees := localGitCLI(t, root, "worktree", "list", "--porcelain", "-z")
	status := localGitCLI(t, root, "status", "--porcelain")
	branch := localGitCLI(t, root, "symbolic-ref", "HEAD")
	dependency := filepath.Join(path, "dependency")
	dependencyHead := localGitCLI(t, dependency, "rev-parse", "HEAD")
	removals := 0
	swap(t, &draftCommand, func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
			removals++
			if len(args) != 3 || args[2] != path {
				t.Fatalf("removal must remain unforced: %v", args)
			}
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	})
	out.Reset()
	diagnostic.Reset()
	if code := draftVerb([]string{"prototype", "widget", "--remove"}, &out, &diagnostic); code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "submodules") {
		t.Fatalf("submodule refusal: %d out=%s err=%s", code, &out, &diagnostic)
	}
	if removals != 1 {
		t.Fatalf("removal attempts: %d, want 1", removals)
	}
	if localGitCLI(t, root, "show-ref", "--heads") != refs || localGitCLI(t, root, "worktree", "list", "--porcelain", "-z") != worktrees {
		t.Fatal("refusal changed branches or registrations")
	}
	if localGitCLI(t, root, "status", "--porcelain") != status || localGitCLI(t, root, "symbolic-ref", "HEAD") != branch {
		t.Fatal("refusal changed the invoking checkout")
	}
	if localGitCLI(t, dependency, "rev-parse", "HEAD") != dependencyHead || localGitCLI(t, path, "status", "--porcelain", "--ignore-submodules=none") != "" {
		t.Fatal("refusal changed the prototype or its initialized submodule")
	}
	if body, err := os.ReadFile(sentinel); err != nil || string(body) != "runtime data" {
		t.Fatalf("runtime data changed: %q %v", body, err)
	}
}

const prototypeLocalProfile = `harness-config:
  codex:
    implementation:
      machine-only:
        model: local-model
        effort: high
        capability: economy
        deliberation: light
        context: standard
        assurance: ordinary
        cost_rank: 99
`

func TestDraftPrototypeCLIRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, tracked, overlay, backend, model string
		failure, want                          string
		buildErr, authErr                      error
	}{
		{name: "configured", tracked: "prototype:\n  harness: codex\n  profile: codex-implementation-economy\n", backend: "claude", model: "claude-haiku-4-5", want: "configured codex codex-implementation-economy"},
		{name: "overlay only profile and route override", tracked: "prototype:\n  harness: claude\n  profile: haiku-low\n", overlay: prototypeLocalProfile + "prototype:\n  harness: codex\n  profile: machine-only\n", want: "configured codex machine-only"},
		{name: "ambient", backend: "claude", model: "claude-haiku-4-5", want: "ambient claude haiku-low"},
		{name: "ambient lowest cost", backend: "claude", model: "claude-sonnet-5", want: "ambient claude sonnet-medium"},
		{name: "ambient overlay only profile", overlay: prototypeLocalProfile, backend: "codex", model: "local-model", want: "ambient codex machine-only"},
		{name: "missing ambient flags", failure: "prototype config block"},
		{name: "missing ambient model", backend: "codex", failure: "prototype config block"},
		{name: "missing ambient backend", model: "gpt-5.6-luna", failure: "prototype config block"},
		{name: "ambient unknown harness", backend: "future", model: "gpt-5.6-luna", failure: "ambient backend"},
		{name: "ambient unknown model", backend: "codex", model: "missing-model", failure: "prototype config block"},
		{name: "configured missing profile ignores valid ambient", tracked: "prototype:\n  harness: codex\n  profile: missing-profile\n", backend: "claude", model: "claude-haiku-4-5", failure: `does not hold profile "missing-profile"`},
		{name: "configured runtime unavailable ignores valid ambient", tracked: "prototype:\n  harness: codex\n  profile: codex-implementation-economy\n", backend: "claude", model: "claude-haiku-4-5", buildErr: errors.New("runtime unavailable"), failure: "runtime unavailable"},
		{name: "configured authentication failure ignores valid ambient", tracked: "prototype:\n  harness: codex\n  profile: codex-implementation-economy\n", backend: "claude", model: "claude-haiku-4-5", authErr: errors.New("login missing"), failure: "login missing"},
		{name: "ambient authentication failure", backend: "codex", model: "gpt-5.6-luna", authErr: errors.New("login missing"), failure: "login missing"},
		{name: "partial prototype block", tracked: "prototype:\n  harness: codex\n", failure: "prototype requires both harness and profile"},
		{name: "unknown configured harness", tracked: "prototype:\n  harness: future\n  profile: build\n", failure: "unknown prototype.harness"},
		{name: "unknown prototype key", tracked: "prototype:\n  harness: codex\n  profile: build\n  extra: true\n", failure: `unknown key "extra" in prototype`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prototypeCLIAuthentication(t)
			root, _ := localGitCLIFixture(t, true)
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			localGitCLI(t, root, "branch", "draft/widget")
			tracked := filepath.Join(root, ".pyxis/config.yaml")
			body, err := os.ReadFile(tracked)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, tracked, append(body, []byte(harnessconfigtest.YAML+tc.tracked)...))
			writeFile(t, filepath.Join(root, ".pyxis/config.local.yaml"), []byte(tc.overlay))
			idea := filepath.Join(t.TempDir(), "idea.txt")
			writeFile(t, idea, []byte("Exercise the idea."))
			swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
			builds, auths, launches := 0, 0, 0
			var launched port.LaunchSpec
			swap(t, &draftPrototypeRuntime, func(backend entity.Harness, cfg executorconfig.Config, hc harnessconfig.Config) (port.AgentRuntime, error) {
				builds++
				if builds > 1 || tc.tracked != "" && tc.failure != "" && backend != entity.Codex {
					t.Fatalf("fallback runtime constructed: %s", backend)
				}
				if tc.buildErr != nil {
					return nil, tc.buildErr
				}
				// Exercise the real constructor and its native catalog validation,
				// replacing the install/login probe and process with hermetic doubles.
				rt, err := harness.New(backend, cfg, hc)
				if err != nil {
					return nil, err
				}
				var selected nativerouting.CatalogEntry
				if tc.want != "" {
					id := strings.Fields(tc.want)[2]
					catalog, _, err := rt.(port.CatalogProvider).Catalog(nativerouting.RoleImplementation)
					if err != nil {
						t.Fatal(err)
					}
					found := false
					for _, entry := range catalog.Entries {
						if entry.NativeProfileID == id {
							selected = entry
							found = true
							if err := rt.(nativerouting.NativeCapability).ValidateProfile(entry); err != nil {
								t.Fatal(err)
							}
							if id == "machine-only" && (entry.NativeModel != "local-model" || entry.NativeEffort != "high") {
								t.Fatalf("adapter lost overlay model/effort: %+v", entry)
							}
						}
					}
					if !found {
						t.Fatalf("adapter lost selected profile %s", id)
					}
				}
				return &prototypeAuthRuntime{AgentRuntime: rt, launch: func(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
					launches++
					launched = spec
					if spec.Purpose != port.LaunchPurposePrototype || spec.HeadBranch != "prototype/widget" || filepath.Base(spec.Workdir) != "prototype-widget" || spec.RuntimeStateDir == "" {
						t.Fatalf("prototype launch: %+v", spec)
					}
					// Validate the actual launch's pair against the real adapter,
					// including a profile present only in the original local overlay.
					if err := rt.(nativerouting.NativeCapability).ValidateProfile(selected); err != nil {
						t.Fatal(err)
					}
					if spec.Model != selected.NativeModel || spec.Effort != selected.NativeEffort {
						t.Fatalf("launch lost resolved profile: %+v", spec)
					}
					writeFile(t, filepath.Join(spec.Workdir, "prototype.txt"), []byte("built"))
					return prototypeCLIHandle{}, nil
				}, auth: func(context.Context) error {
					auths++
					return tc.authErr
				}}, nil
			})
			mutated := false
			swap(t, &draftCommand, func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if !mutated {
					// Change the disk after the initial read. Selection and runtime
					// construction must keep using the invocation's original snapshot.
					writeFile(t, filepath.Join(root, ".pyxis/config.local.yaml"), []byte("broken snapshot replacement: [\n"))
					mutated = true
				}
				if args[0] == "fetch" || args[0] == "worktree" && args[1] == "add" {
					if tc.failure != "" || auths != 1 {
						t.Fatalf("Git mutation before usable route: %v", args)
					}
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			})
			before := localGitCLI(t, root, "show-ref", "--heads")
			args := []string{"prototype", "widget", "--idea", idea}
			if tc.backend != "" {
				args = append(args, "--ambient-backend", tc.backend)
			}
			if tc.model != "" {
				args = append(args, "--ambient-model", tc.model)
			}
			var out, diagnostic bytes.Buffer
			exit := draftVerb(args, &out, &diagnostic)
			if tc.failure == "" {
				if exit != 0 || diagnostic.Len() != 0 || !strings.Contains(out.String(), "\nroute: "+tc.want+"\n") || builds != 1 || auths != 1 || launches != 1 {
					t.Fatalf("route: exit=%d out=%s err=%s builds=%d auths=%d", exit, &out, &diagnostic, builds, auths)
				}
				path := filepath.Join(filepath.Dir(root), ".pyxis-worktrees", filepath.Base(root), "prototype-widget")
				if localGitCLI(t, path, "show", "HEAD:prototype.txt") != "built" || localGitCLI(t, path, "status", "--porcelain") != "" {
					t.Fatal("resolved launch's changes were not committed")
				}
				for _, line := range []string{"model: " + launched.Model, "effort: " + launched.Effort, "outcome: finished", "head: " + localGitCLI(t, path, "rev-parse", "HEAD"), "findings: missing", "usage: tokens=19 turns=unavailable tool_calls=0"} {
					if !strings.Contains(out.String(), line+"\n") {
						t.Fatalf("missing report %q: %s", line, &out)
					}
				}
				return
			}
			if exit != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), tc.failure) {
				t.Fatalf("refusal: exit=%d out=%s err=%s, want %q", exit, &out, &diagnostic, tc.failure)
			}
			path := filepath.Join(filepath.Dir(root), ".pyxis-worktrees", filepath.Base(root), "prototype-widget")
			if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Fatalf("refusal created a parent directory: %v", err)
			}
			if localGitCLI(t, root, "show-ref", "--heads") != before || strings.Contains(localGitCLI(t, root, "worktree", "list", "--porcelain"), path) {
				t.Fatal("refusal changed branches or worktree registrations")
			}
		})
	}
}

type prototypeAuthRuntime struct {
	port.AgentRuntime
	auth   func(context.Context) error
	launch func(context.Context, port.LaunchSpec) (port.Handle, error)
	await  func(context.Context, port.Handle) (port.RunResult, error)
	kill   func(context.Context, port.Handle) error
}

func (r *prototypeAuthRuntime) AuthCheck(ctx context.Context) error { return r.auth(ctx) }
func (r *prototypeAuthRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	return r.launch(ctx, spec)
}
func (r *prototypeAuthRuntime) Await(ctx context.Context, h port.Handle) (port.RunResult, error) {
	if r.await != nil {
		return r.await(ctx, h)
	}
	return port.RunResult{Tokens: 19, TokensObserved: true, ToolCallsObserved: true}, nil
}
func (r *prototypeAuthRuntime) Kill(ctx context.Context, h port.Handle) error {
	return r.kill(ctx, h)
}

func TestDraftPrototypeCLIBuildOutcomes(t *testing.T) {
	for _, outcome := range []string{"finished", "launch failed", "abnormal exit", "timed out", "launch not quiescent", "await not quiescent"} {
		t.Run(outcome, func(t *testing.T) {
			root, _ := localGitCLIFixture(t, true)
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			localGitCLI(t, root, "branch", "draft/widget")
			prototypeCLIRouteFixture(t, root)
			idea := filepath.Join(t.TempDir(), "idea.txt")
			writeFile(t, idea, []byte("Exercise the idea."))
			swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
			swap(t, &draftCommand, draftshell.Run)
			bound := time.Minute
			if outcome == "timed out" {
				bound = time.Millisecond
			}
			swap(t, &draftPrototypeWorkerBound, bound)
			launches, kills := 0, 0
			killed := make(chan struct{})
			path, state := "", ""
			swap(t, &draftPrototypeRuntime, func(entity.Harness, executorconfig.Config, harnessconfig.Config) (port.AgentRuntime, error) {
				return &prototypeAuthRuntime{
					auth: func(context.Context) error { return nil },
					launch: func(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
						launches++
						path, state = spec.Workdir, spec.RuntimeStateDir
						writeFile(t, filepath.Join(path, "prototype.txt"), []byte("partial work"))
						writeFile(t, filepath.Join(path, ".gitignore"), []byte(".pyxis/scratch/\n"))
						writeFile(t, filepath.Join(state, "keep"), []byte("state"))
						if outcome == "finished" {
							writeFile(t, filepath.Join(path, ".pyxis/scratch/prototype-findings.md"), []byte("Kind: technical challenge\nTried: build\nObserved: working path\n"))
						}
						if outcome == "launch failed" {
							return nil, errors.New(outcome)
						}
						if outcome == "launch not quiescent" {
							return nil, errors.Join(errors.New(outcome), port.ErrNotQuiescent)
						}
						return prototypeCLIHandle{}, nil
					},
					await: func(ctx context.Context, _ port.Handle) (port.RunResult, error) {
						if outcome == "timed out" {
							select {
							case <-killed:
							case <-ctx.Done():
								return port.RunResult{}, port.ErrNotQuiescent
							}
						}
						if outcome == "await not quiescent" {
							return port.RunResult{}, errors.Join(errors.New(outcome), port.ErrNotQuiescent)
						}
						if outcome == "abnormal exit" {
							return port.RunResult{}, errors.New(outcome)
						}
						return port.RunResult{Tokens: 29, TokensObserved: true}, nil
					},
					kill: func(context.Context, port.Handle) error { kills++; close(killed); return nil },
				}, nil
			})
			var out, diagnostic bytes.Buffer
			exit := draftVerb([]string{"prototype", "widget", "--idea", idea}, &out, &diagnostic)
			if launches != 1 || (kills == 1) != (outcome == "timed out") {
				t.Fatalf("launches=%d kills=%d", launches, kills)
			}
			if outcome == "finished" {
				if exit != 0 || diagnostic.Len() != 0 || !strings.Contains(out.String(), "findings: "+filepath.Join(path, ".pyxis/scratch/prototype-findings.md")+"\n") {
					t.Fatalf("finished: exit=%d out=%s err=%s", exit, &out, &diagnostic)
				}
			} else if exit != 1 || !strings.Contains(out.String(), "outcome: failed: ") || !strings.Contains(diagnostic.String(), outcome) {
				t.Fatalf("failure: exit=%d out=%s err=%s", exit, &out, &diagnostic)
			}
			if strings.Contains(outcome, "not quiescent") {
				if localGitCLI(t, path, "diff", "--cached", "--name-only") != "" || localGitCLI(t, path, "rev-parse", "HEAD") != localGitCLI(t, root, "rev-parse", "upstream/trunk") || strings.Contains(out.String(), "\nhead:") {
					t.Fatalf("uncertain writer was checkpointed: %s", &out)
				}
			} else if localGitCLI(t, path, "status", "--porcelain") != "" || localGitCLI(t, path, "show", "HEAD:prototype.txt") != "partial work" || !strings.Contains(out.String(), "\nhead: ") {
				t.Fatalf("terminal work was not committed: %s", &out)
			}
			if body, err := os.ReadFile(filepath.Join(state, "keep")); err != nil || string(body) != "state" {
				t.Fatalf("runtime state was not retained: %q %v", body, err)
			}
		})
	}
}
