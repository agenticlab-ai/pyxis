package reviewinvoke

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

func TestReviewOutputSurvivesIsolatedCheckoutCleanup(t *testing.T) {
	for _, hosted := range []bool{false, true} {
		t.Run(fmt.Sprintf("hosted=%t", hosted), func(t *testing.T) {
			world := newLocalWorld(t)
			var reviewed string
			rt := &fakeRuntime{launchContextCheck: func(ctx context.Context, spec port.LaunchSpec) error {
				reviewed = spec.Workdir
				info, err := os.Stat(reviewed)
				if err != nil {
					return err
				}
				if info.Mode().Perm()&0o222 != 0 {
					return fmt.Errorf("review checkout is writable: %s", reviewed)
				}
				var output bytes.Buffer
				stdout, stderr, closeLogs := executorruntime.CaptureOutput(ctx, reviewed, "codex", &output, &output)
				defer closeLogs()
				if _, err := stdout.Write([]byte("review response")); err != nil {
					return err
				}
				if _, err := stderr.Write([]byte("review diagnostic")); err != nil {
					return err
				}
				if output.String() != "review responsereview diagnostic" {
					return fmt.Errorf("capture changed runtime output: %q", output.String())
				}
				if _, err := os.Stat(filepath.Join(reviewed, runlogs.Root)); !os.IsNotExist(err) {
					return fmt.Errorf("logs were written inside the review checkout: %v", err)
				}
				return nil
			}}
			// The invoker may itself be constructed from a linked task worktree.
			// Logs still belong to the persistent repository, not either worktree.
			i := localInvoker(t, rt, world.worktree)
			task := port.Task{ID: "T5", Brief: "Review the widget.",
				DemandID: executiondemand.DemandID(strings.Repeat("a", 64)),
				DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier,
					Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended,
					Assurance: executiondemand.AssuranceCritical}}
			if hosted {
				i.WithProposalInspector(e2eProposalInspector{revision: world.tip}).
					WithWorkspacePreparer(func(ctx context.Context, _ string) (string, func(), error) {
						return i.prepareLocalWorkspace(ctx, world.worktree, world.branch, world.tip)
					})
				proposal := port.Proposal{Number: 548, Branch: world.branch}
				capture := i.Capture(t.Context(), task, proposal)
				if err := i.CompleteHandoff(t.Context(), capture); err != nil {
					t.Fatal(err)
				}
				if result := i.Invoke(t.Context(), task, proposal); result.Err != nil {
					t.Fatal(result.Err)
				}
			} else if result := i.ReviewLocal(t.Context(), task, localWorkdir{dir: world.worktree, branch: world.branch}, 1); result.Err != nil {
				t.Fatal(result.Err)
			}
			if reviewed == "" {
				t.Fatal("reviewer was not launched")
			}
			if _, err := os.Stat(reviewed); !os.IsNotExist(err) {
				t.Fatalf("review checkout was not removed: %v", err)
			}
			for name, want := range map[string]string{"stdout.log": "review response", "stderr.log": "review diagnostic"} {
				files, err := filepath.Glob(filepath.Join(world.repo, runlogs.Agents, "codex", "*", name))
				if err != nil || len(files) != 1 {
					t.Fatalf("persistent %s: files=%v err=%v", name, files, err)
				}
				data, err := os.ReadFile(files[0])
				if err != nil || string(data) != want {
					t.Fatalf("persistent %s: %q %v", name, data, err)
				}
				if info, err := os.Stat(files[0]); err != nil || info.Mode().Perm() != 0o600 {
					t.Fatalf("log permissions: %v %v", info, err)
				}
			}
		})
	}
}
