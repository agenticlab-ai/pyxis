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
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
)

func TestPrototypeBuildRefusesBeforeLaunch(t *testing.T) {
	for _, failure := range []string{"runtime", "process adapter", "bound", "idea", "state"} {
		t.Run(failure, func(t *testing.T) {
			_, idea, p := prototypeBuildFixture(t)
			p.Route.Runtime = &prototypeBuildRuntime{launch: func(context.Context, port.LaunchSpec) (port.Handle, error) {
				t.Fatal("launched despite failed preparation")
				return nil, nil
			}}
			run := RunCommand(func(context.Context, string, []string, string, ...string) ([]byte, error) {
				t.Fatal("Git ran despite failed preparation")
				return nil, nil
			})
			bound := time.Minute
			want := "needs a runtime, process adapter, and positive time bound"
			switch failure {
			case "runtime":
				p.Route.Runtime = nil
			case "process adapter":
				run = nil
			case "bound":
				bound = 0
			case "idea":
				idea += ".missing"
				want = "read prototype idea"
			case "state":
				prototypeWrite(t, executorruntime.TaskStateRoot(p.Worktree), "occupied")
				want = "prepare prototype runtime state"
			}
			result, err := BuildPrototype(t.Context(), p, idea, bound, run)
			if err == nil || !strings.Contains(err.Error(), want) || result.Head != "" {
				t.Fatalf("preparation result=%+v error=%v", result, err)
			}
			if prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != p.Base || prototypeGit(t, p.Worktree, "status", "--porcelain") != "" {
				t.Fatal("failed preparation changed prototype head or worktree")
			}
		})
	}
}

func TestPrototypeBuildCheckpointFailures(t *testing.T) {
	for _, failure := range []string{"branch read", "branch changed", "stage", "commit", "head", "findings"} {
		t.Run(failure, func(t *testing.T) {
			_, idea, p := prototypeBuildFixture(t)
			buildErr, injected := errors.New("build stopped"), errors.New("injected persistence failure")
			p.Route.Runtime = &prototypeBuildRuntime{
				launch: func(context.Context, port.LaunchSpec) (port.Handle, error) {
					prototypeWrite(t, filepath.Join(p.Worktree, "partial.txt"), "partial build")
					if failure == "findings" {
						dir := filepath.Join(p.Worktree, ".pyxis/scratch")
						if err := os.MkdirAll(dir, 0o700); err != nil {
							t.Fatal(err)
						}
						// A symlink loop produces a deterministic stat error without
						// relying on permission checks that differ when tests run as root.
						if err := os.Symlink("prototype-findings.md", filepath.Join(dir, "prototype-findings.md")); err != nil {
							t.Fatal(err)
						}
					}
					return prototypeHandle{}, nil
				},
				await: func(context.Context, port.Handle) (port.RunResult, error) { return port.RunResult{}, buildErr },
			}
			stages, commits := 0, 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				switch args[0] {
				case "symbolic-ref":
					if failure == "branch read" {
						return nil, injected
					}
					if failure == "branch changed" {
						return []byte("refs/heads/another-branch\n"), nil
					}
				case "add":
					stages++
					if failure == "stage" {
						return nil, injected
					}
				case "commit":
					commits++
					if failure == "commit" {
						return nil, injected
					}
				case "rev-parse":
					if failure == "head" {
						return nil, injected
					}
				}
				return draftshell.Run(ctx, dir, env, name, args...)
			}
			result, err := BuildPrototype(t.Context(), p, idea, time.Minute, run)
			if !errors.Is(err, buildErr) {
				t.Fatalf("checkpoint failure hid the build error: %v", err)
			}
			if failure == "branch changed" {
				if !strings.Contains(err.Error(), "refuse prototype commit") {
					t.Fatalf("wrong branch was not refused: %v", err)
				}
			} else if failure == "findings" {
				if !strings.Contains(err.Error(), "inspect prototype findings") || result.Head == "" || result.Findings != "" {
					t.Fatalf("findings error was treated as missing: %+v %v", result, err)
				}
			} else if !errors.Is(err, injected) {
				t.Fatalf("lost persistence error: %v", err)
			}
			switch failure {
			case "branch read", "branch changed":
				if stages != 0 || commits != 0 {
					t.Fatalf("mutated an unverified branch: stages=%d commits=%d", stages, commits)
				}
			case "stage":
				if stages != 1 || commits != 0 {
					t.Fatalf("continued after stage failure: stages=%d commits=%d", stages, commits)
				}
			default:
				if stages != 1 || commits != 1 {
					t.Fatalf("retried persistence: stages=%d commits=%d", stages, commits)
				}
			}
			head := prototypeGit(t, p.Worktree, "rev-parse", "HEAD")
			if (head != p.Base) != (failure == "head" || failure == "findings") {
				t.Fatalf("unexpected commit after %s: %s", failure, head)
			}
			if failure != "findings" && result.Head != "" {
				t.Fatalf("reported a head before verifying it: %+v", result)
			}
			if body, err := os.ReadFile(filepath.Join(p.Worktree, "partial.txt")); err != nil || string(body) != "partial build" {
				t.Fatalf("lost partial work: %q %v", body, err)
			}
		})
	}
}

func TestPrototypeBuildMissingHandlePreservesWork(t *testing.T) {
	_, idea, p := prototypeBuildFixture(t)
	p.Route.Runtime = &prototypeBuildRuntime{launch: func(context.Context, port.LaunchSpec) (port.Handle, error) {
		prototypeWrite(t, filepath.Join(p.Worktree, "partial.txt"), "possible writer")
		return nil, nil
	}}
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		t.Fatal("Git ran without a handle or stop acknowledgement")
		return nil, nil
	}
	result, err := BuildPrototype(t.Context(), p, idea, time.Minute, run)
	if !errors.Is(err, port.ErrNotQuiescent) || result.Head != "" || prototypeGit(t, p.Worktree, "rev-parse", "HEAD") != p.Base || prototypeGit(t, p.Worktree, "diff", "--cached", "--name-only") != "" {
		t.Fatalf("missing-handle result=%+v error=%v", result, err)
	}
}

func TestPrototypeAwaitCancellationRetainsUsageAfterStop(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	killed := make(chan struct{})
	kills := 0
	rt := &prototypeBuildRuntime{
		await: func(context.Context, port.Handle) (port.RunResult, error) {
			cancel()
			<-killed
			return port.RunResult{Tokens: 31, TokensObserved: true}, errors.New("killed")
		},
		kill: func(ctx context.Context, _ port.Handle) error {
			if ctx.Err() != nil {
				t.Error("kill inherited the cancelled context")
			}
			kills++
			close(killed)
			return nil
		},
	}
	usage, err := awaitPrototype(ctx, rt, prototypeHandle{}, time.Minute, time.Second)
	if !errors.Is(err, context.Canceled) || errors.Is(err, port.ErrNotQuiescent) || usage.Tokens != 31 || kills != 1 {
		t.Fatalf("cancellation lost acknowledgement or usage: %+v %v kills=%d", usage, err, kills)
	}
}
