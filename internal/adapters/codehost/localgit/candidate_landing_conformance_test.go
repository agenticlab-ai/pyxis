package localgit

import (
	"os"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview/conformancetest"
)

func TestCandidateLandingPortConformance(t *testing.T) {
	t.Parallel()
	conformancetest.RunLandings(t, func(t *testing.T) conformancetest.LandingFixture {
		o, store, reader, gate, packet := readyCandidateLanding(t)
		return conformancetest.LandingFixture{
			Packet: packet, Landings: gate,
			State: func(t *testing.T) string { return fixtureGit(t, o.Origin, "show-ref") },
			SetProof: func(t *testing.T, ref codehost.ArtifactRef, present bool) {
				if present {
					fixtureGit(t, o.Origin, "update-ref", ref.Ref, ref.OID)
				} else {
					fixtureGit(t, o.Origin, "update-ref", "-d", ref.Ref)
				}
			},
			ChangePolicy: func(*testing.T) { reader.plan.ScenarioSetFingerprint += "-changed" },
			RemoveCheckout: func(t *testing.T) {
				if err := os.RemoveAll(o.Root); err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Origin, "gc", "--prune=now")
			},
			AssertDelivery: func(t *testing.T, result codehost.CandidateLandingResult) {
				l, c := result.Landing, result.Landing.Candidate
				for _, shape := range [][2]string{{l.MergeCommit, c.MainHead}, {l.FeatureMarker, c.FeatureHead}} {
					if fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", shape[0]) != shape[0]+" "+shape[1] || fixtureGit(t, o.Origin, "rev-parse", shape[0]+"^{tree}") != c.Tree {
						t.Fatal("delivery changed candidate tree or squash/marker parents")
					}
				}
				final, err := NewProposals(store.options).Read(t.Context(), packet.Binding.ProposalID)
				if err != nil || final.Proposal.PreviousOID != packet.Binding.ProposalOID || final.Proposal.Sequence != packet.Binding.Sequence+1 {
					t.Fatalf("final generation lost reviewed predecessor: %v", err)
				}
				refs, err := proposalRefs(t.Context(), o.Origin, packet.Binding.Branch, final.Proposal.ArchiveRef)
				if err != nil || refs[packet.Binding.Branch] != "" || refs[final.Proposal.ArchiveRef] != final.OID {
					t.Fatal("delivery did not archive and delete atomically")
				}
			},
			Clone: func(t *testing.T) codehost.CandidateLandings {
				options := store.options
				options.Git = proposalClone(t, o)
				p, a := NewProposals(options), NewReviewArtifacts(options)
				r, err := candidatereview.NewRequests(t.Context(), p, a, reader)
				if err != nil {
					t.Fatal(err)
				}
				receipts := NewCandidateReceipts(options, r)
				clone, err := candidatereview.NewLandingGate(t.Context(), p, a, reader, receipts, NewCandidateLandings(options, receipts))
				if err != nil {
					t.Fatal(err)
				}
				return clone
			},
		}
	})
}
