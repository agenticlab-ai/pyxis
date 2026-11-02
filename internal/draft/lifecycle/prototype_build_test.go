package lifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type prototypeHandle struct{}

func (prototypeHandle) RuntimeHandle() {}

type prototypeBuildRuntime struct {
	launch func(context.Context, port.LaunchSpec) (port.Handle, error)
	await  func(context.Context, port.Handle) (port.RunResult, error)
	kill   func(context.Context, port.Handle) error
}

func (*prototypeBuildRuntime) AuthCheck(context.Context) error { return nil }
func (r *prototypeBuildRuntime) Launch(ctx context.Context, spec port.LaunchSpec) (port.Handle, error) {
	return r.launch(ctx, spec)
}
func (r *prototypeBuildRuntime) Await(ctx context.Context, h port.Handle) (port.RunResult, error) {
	return r.await(ctx, h)
}
func (r *prototypeBuildRuntime) Kill(ctx context.Context, h port.Handle) error {
	return r.kill(ctx, h)
}

func prototypeBuildFixture(t *testing.T) (string, string, PrototypeResult) {
	t.Helper()
	root, idea, cfg := prototypeFixture(t)
	p, err := CreatePrototype(t.Context(), root, cfg, "widget", idea, prototypeTestRoute, draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	// Keep fixture findings out of the checkpoint just as the installed repo's
	// ordinary scratch ignore does. Existing files exercise deletions as well.
	prototypeWrite(t, filepath.Join(p.Worktree, ".gitignore"), ".pyxis/scratch/\n")
	prototypeWrite(t, filepath.Join(p.Worktree, "modify.txt"), "before")
	prototypeWrite(t, filepath.Join(p.Worktree, "delete.txt"), "before")
	prototypeGit(t, p.Worktree, "add", "--all")
	prototypeGit(t, p.Worktree, "commit", "-qm", "fixture build inputs")
	p.Base = prototypeGit(t, p.Worktree, "rev-parse", "HEAD")
	p.Route = PrototypeRoute{Harness: "fixture", Profile: nativerouting.ProfileSpec{ID: "build", Model: "fixture-model", Effort: "high"}}
	return root, idea, p
}

func TestPrototypeBuildTerminalOutcomes(t *testing.T) {
	for _, outcome := range []string{"finished", "launch failed", "abnormal exit", "incomplete", "timed out"} {
		t.Run(outcome, func(t *testing.T) {
			root, idea, p := prototypeBuildFixture(t)
			callerHead := prototypeGit(t, root, "rev-parse", "HEAD")
			draftHead := prototypeGit(t, root, "rev-parse", "contracts/widget")
			launches, kills := 0, 0
			stopped := false
			killed := make(chan struct{})
			usage := port.RunResult{Tokens: 17, Turns: 2, TokensObserved: true, TurnsObserved: true}
			rt := &prototypeBuildRuntime{
				launch: func(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
					launches++
					if spec.Purpose != port.LaunchPurposePrototype || spec.Workdir != p.Worktree || spec.HeadBranch != p.Branch || spec.Model != "fixture-model" || spec.Effort != "high" || spec.RuntimeStateDir != filepath.Join(executorruntime.TaskStateRoot(p.Worktree), "fixture") || spec.Session != nil || spec.RequestDescription {
						t.Fatalf("launch spec: %+v", spec)
					}
					brief, err := executorruntime.AssembleBriefContext(t.Context(), spec)
					if err != nil || !strings.Contains(brief, "> Exercise the idea.") || strings.Contains(brief, "delivery protocol") {
						t.Fatalf("brief: %s, %v", brief, err)
					}
					prototypeWrite(t, filepath.Join(p.Worktree, "modify.txt"), "after")
					prototypeWrite(t, filepath.Join(p.Worktree, "new.txt"), "built")
					if err := os.Remove(filepath.Join(p.Worktree, "delete.txt")); err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(filepath.Join(p.Worktree, ".pyxis/scratch"), 0o700); err != nil {
						t.Fatal(err)
					}
					prototypeWrite(t, filepath.Join(p.Worktree, ".pyxis/scratch/prototype-findings.md"), "Kind: technical challenge\nTried: build\nObserved: working path\n")
					if outcome == "launch failed" {
						stopped = true
						return nil, errors.New(outcome)
					}
					return prototypeHandle{}, nil
				},
				await: func(ctx context.Context, _ port.Handle) (port.RunResult, error) {
					if outcome == "timed out" {
						select {
						case <-killed:
						case <-ctx.Done():
							return port.RunResult{}, port.ErrNotQuiescent
						}
					}
					// Channel send/receive in awaitPrototype orders this write before
					// the host's subsequent checkpoint (also on the kill path).
					stopped = true
					if outcome == "timed out" {
						return usage, errors.New("process killed by bound")
					}
					if outcome == "abnormal exit" {
						return usage, errors.New(outcome)
					}
					if outcome == "incomplete" {
						usage.Incomplete, usage.Reason = true, outcome
					}
					return usage, nil
				},
				kill: func(context.Context, port.Handle) error { kills++; close(killed); return nil },
			}
			p.Route.Runtime = rt
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if !stopped {
					t.Fatalf("Git before stop acknowledgement: %v", args)
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			bound := time.Minute
			if outcome == "timed out" {
				bound = time.Millisecond
			}
			result, err := BuildPrototype(t.Context(), p, idea, bound, run)
			if (err == nil) != (outcome == "finished") || err != nil && !strings.Contains(err.Error(), outcome) {
				t.Fatalf("outcome: %+v, %v", result, err)
			}
			if outcome == "timed out" && err.Error() != "timed out" {
				t.Fatalf("kill exit replaced timeout cause: %v", err)
			}
			if launches != 1 || (kills == 1) != (outcome == "timed out") || result.Head == "" || result.Head == p.Base {
				t.Fatalf("launches=%d kills=%d result=%+v", launches, kills, result)
			}
			if outcome != "launch failed" && result.Usage.Tokens != 17 {
				t.Fatalf("lost usage: %+v", result.Usage)
			}
			if result.Findings != filepath.Join(p.Worktree, ".pyxis/scratch/prototype-findings.md") || prototypeGit(t, p.Worktree, "status", "--porcelain") != "" {
				t.Fatalf("findings or dirty checkpoint: %+v", result)
			}
			if prototypeGit(t, p.Worktree, "show", "HEAD:new.txt") != "built" || prototypeGit(t, p.Worktree, "show", "HEAD:modify.txt") != "after" || strings.Contains(prototypeGit(t, p.Worktree, "ls-tree", "--name-only", "HEAD"), "delete.txt") {
				t.Fatal("checkpoint lost build changes")
			}
			if prototypeGit(t, root, "rev-parse", "HEAD") != callerHead || prototypeGit(t, root, "rev-parse", "contracts/widget") != draftHead {
				t.Fatal("build changed invoking checkout or draft branch")
			}
			if _, err := os.Stat(executorruntime.TaskStateRoot(p.Worktree)); err != nil {
				t.Fatalf("runtime state was removed: %v", err)
			}
		})
	}
}

func TestPrototypeBuildNotQuiescentPreservesEverything(t *testing.T) {
	for _, origin := range []string{"launch", "await", "kill", "after kill", "no acknowledgement"} {
		t.Run(origin, func(t *testing.T) {
			_, idea, p := prototypeBuildFixture(t)
			launches, kills := 0, 0
			killed := make(chan struct{})
			p.Route.Runtime = &prototypeBuildRuntime{
				launch: func(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
					launches++
					prototypeWrite(t, filepath.Join(spec.Workdir, "uncommitted.txt"), "possible live writer")
					prototypeWrite(t, filepath.Join(spec.RuntimeStateDir, "keep"), "state")
					if origin == "launch" {
						return nil, errors.Join(errors.New("launch uncertain"), port.ErrNotQuiescent)
					}
					return prototypeHandle{}, nil
				},
				await: func(ctx context.Context, _ port.Handle) (port.RunResult, error) {
					if origin != "await" {
						select {
						case <-killed:
						case <-ctx.Done():
						}
					}
					if origin == "no acknowledgement" {
						<-ctx.Done()
					}
					return port.RunResult{}, port.ErrNotQuiescent
				},
				kill: func(context.Context, port.Handle) error {
					kills++
					close(killed)
					if origin == "kill" {
						return errors.New("kill failed")
					}
					return nil
				},
			}
			run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Error("Git ran without a stop acknowledgement")
				return nil, errors.New("forbidden")
			}
			bound := time.Minute
			if origin != "launch" && origin != "await" {
				bound = time.Millisecond
			}
			result, err := BuildPrototype(t.Context(), p, idea, bound, run)
			if !errors.Is(err, port.ErrNotQuiescent) || result.Head != "" || launches != 1 {
				t.Fatalf("result=%+v error=%v launches=%d kills=%d", result, err, launches, kills)
			}
			if prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != p.Base || prototypeGit(t, p.Worktree, "diff", "--cached", "--name-only") != "" || prototypeGit(t, p.Worktree, "symbolic-ref", "--short", "HEAD") != p.Branch {
				t.Fatal("uncertain process changed head, index or branch")
			}
			for path, want := range map[string]string{filepath.Join(p.Worktree, "uncommitted.txt"): "possible live writer", filepath.Join(executorruntime.TaskStateRoot(p.Worktree), "fixture", "keep"): "state"} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != want {
					t.Fatalf("preserved file %s: %q %v", path, got, err)
				}
			}
		})
	}
}

func TestPrototypeBuildFileLoginReachesLaunch(t *testing.T) {
	_, idea, p := prototypeBuildFixture(t)
	p.Route.Harness = entity.Codex
	login := t.TempDir()
	t.Setenv("CODEX_HOME", login)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CODEX_API_KEY", "")
	prototypeWrite(t, filepath.Join(login, "auth.json"), `{"fixture_login":"file-only"}`)
	p.Route.Runtime = &prototypeBuildRuntime{
		launch: func(_ context.Context, spec port.LaunchSpec) (port.Handle, error) {
			access := executorruntime.LaunchAccess(spec)
			if access == nil || access.RuntimeStateDir != spec.RuntimeStateDir || access.Purpose != port.LaunchPurposePrototype || access.Workdir != p.Worktree {
				t.Fatalf("launch access: %+v", access)
			}
			env := executorruntime.LaunchEnvironment(spec, "OPENAI_API_KEY", "CODEX_API_KEY", "CODEX_HOME")
			for _, item := range env {
				if dir, ok := strings.CutPrefix(item, "CODEX_HOME="); ok {
					got, err := os.ReadFile(filepath.Join(dir, "auth.json"))
					if err != nil || string(got) != `{"fixture_login":"file-only"}` || dir != filepath.Join(spec.RuntimeStateDir, "model-auth/codex") {
						t.Fatalf("login snapshot: %q %v in %s", got, err, dir)
					}
					return prototypeHandle{}, nil
				}
			}
			t.Fatal("launch environment lost CODEX_HOME")
			return nil, nil
		},
		await: func(context.Context, port.Handle) (port.RunResult, error) { return port.RunResult{}, nil },
	}
	if result, err := BuildPrototype(t.Context(), p, idea, time.Minute, draftshell.Run); err != nil || result.Head == "" || result.Findings != "" {
		t.Fatalf("file-login build: %+v %v", result, err)
	}
}
