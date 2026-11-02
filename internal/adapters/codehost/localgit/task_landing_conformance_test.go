package localgit

import (
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewgate/conformancetest"
)

func TestTaskLandingPortConformance(t *testing.T) {
	t.Parallel()
	conformancetest.RunLandings(t, func(t *testing.T) conformancetest.LandingFixture {
		// Each case owns its repository, reader and operator ports.
		t.Parallel()
		o, store, reader, gate, packet := readyTaskLanding(t)
		main := fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk")
		return conformancetest.LandingFixture{
			Packet: packet, Landings: gate,
			State:  func(t *testing.T) string { return fixtureGit(t, o.Origin, "show-ref") },
			Forget: func(t *testing.T, ref codehost.ArtifactRef) { fixtureGit(t, o.Origin, "update-ref", "-d", ref.Ref) },
			RemoveCheckout: func(t *testing.T) {
				if err := os.RemoveAll(o.Root); err != nil {
					t.Fatal(err)
				}
				reader.failSnapshot = true
				fixtureGit(t, o.Origin, "gc", "--prune=now")
			},
			AssertSquash: func(t *testing.T, r codehost.TaskLandingResult) {
				b, l := packet.Binding, r.Landing
				if fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", l.MergeCommit) != l.MergeCommit+" "+b.TargetOID ||
					fixtureGit(t, o.Origin, "rev-parse", l.MergeCommit+"^{tree}") != fixtureGit(t, o.Origin, "rev-parse", b.Head+"^{tree}") ||
					fixtureGit(t, o.Origin, "rev-parse", "refs/heads/trunk") != main {
					t.Fatal("task squash changed its reviewed tree, parent, or mainline")
				}
				final, err := NewProposals(store.options).Read(t.Context(), b.ProposalID)
				if err != nil || final.Proposal.PreviousOID != b.ProposalOID || final.Proposal.Sequence != b.Sequence+1 {
					t.Fatalf("final generation did not retain its predecessor: %v", err)
				}
				refs, err := proposalRefs(t.Context(), o.Origin, b.Branch, final.Proposal.ArchiveRef)
				if err != nil || refs[b.Branch] != "" || refs[final.Proposal.ArchiveRef] != final.OID {
					t.Fatal("task landing did not archive and delete atomically")
				}
			},
			Clone: func(t *testing.T) codehost.TaskLandings {
				options := store.options
				options.Git = proposalClone(t, o)
				proposals, artifacts := NewProposals(options), NewReviewArtifacts(options)
				requests, err := reviewgate.NewTaskRequests(t.Context(), proposals, artifacts, reader)
				if err != nil {
					t.Fatal(err)
				}
				receipts := NewTaskReceipts(options, requests)
				clone, err := reviewgate.NewTaskLandingGate(t.Context(), proposals, artifacts, reader, receipts, NewTaskLandings(options, receipts))
				if err != nil {
					t.Fatal(err)
				}
				return clone
			},
		}
	})
}
