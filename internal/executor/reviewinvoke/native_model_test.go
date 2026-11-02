package reviewinvoke

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/process"
)

type nativeModelProcessStore struct {
	fakeProcessCoordinator
	status implreview.HandoffStatus
}

func (s *nativeModelProcessStore) Finish(_ context.Context, _ implreview.AdmittedProcess, status implreview.HandoffStatus, _ string) error {
	s.status = status
	return nil
}

func TestNativeModelMismatchCannotCompleteLocalOrHostedReview(t *testing.T) {
	for _, mode := range []string{"hosted", "local"} {
		for _, reported := range []string{"review-model", "gpt-5"} {
			t.Run(mode+"/"+reported, func(t *testing.T) {
				store := &nativeModelProcessStore{}
				table, err := calibration.Parse(strings.NewReader("| Row | Lens | Default severity | Escalation condition |\n|---|---|---|---|\n| CAL-1 | spec-fidelity | blocking | test |\n"))
				if err != nil {
					t.Fatal(err)
				}
				coordinator, err := process.New(store, table)
				if err != nil {
					t.Fatal(err)
				}
				rt := &fakeRuntime{}
				rt.launchCheck = func(spec port.LaunchSpec) error {
					parsed, err := persist.ParseHandoff(artifactFromBrokerBrief(spec.Brief))
					if err != nil {
						return err
					}
					parsed.Rounds[0].Provenance.Model = reported
					rt.result.Output, err = persist.ComposeArtifact(parsed)
					return err
				}
				exact := reported == "review-model"
				if mode == "local" {
					world := newLocalWorld(t)
					i := localInvoker(t, rt, world.repo)
					i.processes = coordinator
					result := i.ReviewLocal(context.Background(), port.Task{ID: "T5", Brief: "Review the widget against its frozen charter."}, localWorkdir{dir: world.worktree, branch: world.branch}, 1)
					if exact {
						if result.Err != nil || result.Artifact == "" {
							t.Fatalf("exact local result=%+v", result)
						}
					} else if result.Err == nil || result.Artifact != "" || result.Verdict != "" || result.FailureClass != worker.ReviewFailureArtifactUnverified {
						t.Fatalf("mismatched local result=%+v", result)
					}
				} else {
					verifier := &fakeVerifier{}
					i := lifecycleInvoker(t, rt, verifier, time.Minute)
					i.processes = coordinator
					result := i.Invoke(context.Background(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
					if exact {
						if result.Err != nil || result.Status != "verified" || result.ArtifactRef == "" {
							t.Fatalf("exact hosted result=%+v", result)
						}
					} else if result.Err == nil || result.Status != "failed" || result.ArtifactRef != "" || result.NativeReceipt != "" || result.FailureClass != worker.ReviewFailureArtifactUnverified || verifier.handoff.Aggregate != nil || verifier.handoff.Status != implreview.HandoffMismatched {
						t.Fatalf("mismatched hosted result=%+v handoff=%+v", result, verifier.handoff)
					}
				}
				want := implreview.HandoffMismatched
				if exact {
					want = implreview.HandoffCompleted
				}
				if store.status != want || rt.launches != 1 {
					t.Fatalf("durable terminal=%s launches=%d, want %s/1", store.status, rt.launches, want)
				}
			})
		}
	}
}
