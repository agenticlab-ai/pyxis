package localgit

import (
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate/conformancetest"
)

func TestTaskReviewPortConformance(t *testing.T) {
	t.Parallel()
	conformancetest.RunTasks(t, func(t *testing.T) conformancetest.TaskFixture {
		// The conformance runner creates a private fixture for each subtest.
		t.Parallel()
		o, proposals, artifacts, reader, requests, opened := taskReviewFixture(t)
		receipts := NewTaskReceipts(proposals.options, requests)
		return conformancetest.TaskFixture{
			ID: opened.Proposal.ID, Proposals: proposals, Artifacts: artifacts, Requests: requests, Receipts: receipts,
			State:        func(t *testing.T) string { return fixtureGit(t, o.Origin, "show-ref") },
			ChangePolicy: func(*testing.T) { reader.policy.Prompts = "changed-prompts" },
			Forget: func(t *testing.T, ref codehost.ArtifactRef) {
				fixtureGit(t, o.Origin, "update-ref", "-d", ref.Ref)
			},
			RemoveCheckout: func(t *testing.T) {
				if err := os.RemoveAll(o.Root); err != nil {
					t.Fatal(err)
				}
			},
			Clone: func(t *testing.T) conformancetest.TaskFixture {
				options := proposals.options
				options.Git = proposalClone(t, o)
				proposals, artifacts := NewProposals(options), NewReviewArtifacts(options)
				requests, err := reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, reader)
				if err != nil {
					t.Fatal(err)
				}
				return conformancetest.TaskFixture{Proposals: proposals, Artifacts: artifacts, Requests: requests, Receipts: NewTaskReceipts(options, requests)}
			},
		}
	})
}
