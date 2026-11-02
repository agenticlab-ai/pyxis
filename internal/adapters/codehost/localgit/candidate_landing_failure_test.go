package localgit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestCandidateLandingAdapterRefusesUnavailableInputsAndPreparationFailures(t *testing.T) {
	t.Parallel()
	o, store, reader, _, packet := readyCandidateLanding(t)
	b := packet.Binding
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, options := range []ProposalOptions{{}, {Git: o, Kind: "draft"}, {Git: o, Kind: "implementation", IntegrationRef: store.options.HeadRef}} {
		other := NewCandidateLandings(options, nil)
		if _, err := other.LandCandidate(t.Context(), b); err == nil {
			t.Fatal("unbound operator accepted final landing")
		}
		if _, err := other.VerifyCandidateLanding(t.Context(), b.ProposalID); err == nil {
			t.Fatal("unbound reader accepted final landing")
		}
	}
	if _, err := NewCandidateLandings(store.options, nil).LandCandidate(t.Context(), b); err == nil {
		t.Fatal("landing has no current receipt verifier")
	}
	if _, err := store.VerifyCandidateLanding(t.Context(), b.ProposalID); err == nil {
		t.Fatal("prepared final proposal was presented as landed")
	}
	if _, err := store.VerifyCandidateLanding(t.Context(), "missing-final"); err == nil {
		t.Fatal("missing proposal was presented as landed")
	}
	wrong := b
	wrong.ProposalID = "missing-final"
	if _, err := store.LandCandidate(t.Context(), wrong); err == nil {
		t.Fatal("missing proposal landed")
	}
	wrong = b
	wrong.Sequence++
	if _, err := store.LandCandidate(t.Context(), wrong); err == nil {
		t.Fatal("different generation used prior approval")
	}
	for _, fault := range []string{"fetch", "merge commit", "marker commit", "changed policy", "changed attribution"} {
		t.Run(fault, func(t *testing.T) {
			entered, commits := false, 0
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				if args[0] == "commit-tree" {
					commits++
				}
				if fault == "fetch" && args[0] == "fetch" || fault == "merge commit" && commits == 1 || fault == "marker commit" && commits == 2 {
					entered = true
					return nil, errors.New("Git input or object storage unavailable")
				}
				out, err := proposalGit(ctx, root, input, env, args...)
				if err == nil && args[0] == "commit-tree" {
					if fault == "changed policy" {
						reader.plan.ScenarioSetFingerprint = "required-v2"
						entered = true
					}
					if fault == "changed attribution" {
						reader.fault = "attribution unavailable"
						entered = true
					}
				}
				return out, err
			}
			if result, err := store.LandCandidate(t.Context(), b); err == nil || result.Created || !entered || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("preparation failure advanced refs: %v", err)
			}
			reader.plan.ScenarioSetFingerprint, reader.fault = b.ScenarioSetFingerprint, ""
		})
	}
}

func TestCandidateLandingAdapterRecoversExactRetryAndRefusesBrokenRetainedProof(t *testing.T) {
	t.Parallel()
	o, store, packet := readyCandidateAdapter(t)
	b := packet.Binding
	landed, err := store.LandCandidate(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	if result, err := store.LandCandidate(t.Context(), b); err != nil || result.Created || !reflect.DeepEqual(result.Landing, landed.Landing) {
		t.Fatalf("direct retry changed delivery: %v", err)
	}
	wrong := b
	wrong.Sequence++
	if _, err := store.LandCandidate(t.Context(), wrong); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("direct retry used another approval")
	}
	for _, fault := range []string{"merge parents", "marker parents", "merge tree", "marker tree", "main ancestry", "feature ancestry"} {
		t.Run(fault, func(t *testing.T) {
			entered := false
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				match := args[0] == "rev-list" && (fault == "merge parents" && args[len(args)-1] == landed.Landing.MergeCommit || fault == "marker parents" && args[len(args)-1] == landed.Landing.FeatureMarker) ||
					args[0] == "rev-parse" && (fault == "merge tree" && args[1] == landed.Landing.MergeCommit+"^{tree}" || fault == "marker tree" && args[1] == landed.Landing.FeatureMarker+"^{tree}") ||
					args[0] == "merge-base" && (fault == "main ancestry" && args[2] == landed.Landing.MergeCommit || fault == "feature ancestry" && args[2] == landed.Landing.FeatureMarker)
				if match {
					entered = true
					return nil, errors.New("retained Git proof unavailable")
				}
				return proposalGit(ctx, root, input, env, args...)
			}
			if _, err := store.VerifyCandidateLanding(t.Context(), b.ProposalID); err == nil || !entered || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("broken object proof accepted: %v", err)
			}
		})
	}
	store.run = proposalGit
	refs := []string{b.Evidence.Ref, b.Request.Ref, receiptRef(b), b.TargetRef, store.options.HeadRef}
	original, err := proposalRefs(t.Context(), o.Origin, refs...)
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		t.Run("missing "+ref, func(t *testing.T) {
			fixtureGit(t, o.Origin, "update-ref", "-d", ref)
			state := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.VerifyCandidateLanding(t.Context(), b.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != state {
				t.Fatal("missing proof repaired or accepted")
			}
			fixtureGit(t, o.Origin, "update-ref", ref, original[ref])
		})
	}
	final, err := NewProposals(store.options).Read(t.Context(), b.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"evidence trust", "receipt trust", "lost main ancestry", "lost feature ancestry"} {
		t.Run(fault, func(t *testing.T) {
			if strings.HasPrefix(fault, "lost") {
				ref, oid := b.TargetRef, b.TargetOID
				if fault == "lost feature ancestry" {
					ref, oid = store.options.HeadRef, b.Head
				}
				fixtureGit(t, o.Origin, "update-ref", ref, oid)
			} else {
				p := final.Proposal
				l := *p.CandidateLanding
				p.CandidateLanding = &l
				if fault == "evidence trust" {
					l.Evidence.Trust.ApproverAuthenticated = true
				} else {
					l.Review.Trust.ApproverAuthenticated = true
				}
				oid, err := prepareProposal(t.Context(), o.Root, p)
				if err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, o.Root, "push", "--atomic", "--force", o.Origin, oid+":"+proposalPrefix+p.ID, oid+":"+p.ArchiveRef)
			}
			state := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.VerifyCandidateLanding(t.Context(), b.ProposalID); err == nil || fixtureGit(t, o.Origin, "show-ref") != state {
				t.Fatal("altered retained proof accepted")
			}
			fixtureGit(t, o.Origin, "update-ref", b.TargetRef, original[b.TargetRef])
			fixtureGit(t, o.Origin, "update-ref", store.options.HeadRef, original[store.options.HeadRef])
			fixtureGit(t, o.Origin, "update-ref", proposalPrefix+b.ProposalID, final.OID)
			fixtureGit(t, o.Origin, "update-ref", final.Proposal.ArchiveRef, final.OID)
		})
	}
}

func TestCandidateLandingRechecksApprovalScopeAndRecoveryReads(t *testing.T) {
	t.Parallel()
	o, store, packet := readyCandidateAdapter(t)
	b := packet.Binding
	review, err := store.receipts.Verify(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	changed := review.Receipt
	changed.Reviewer = "another declared reviewer"
	body, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := prepareReceipt(t.Context(), o.Root, b.ProposalOID, append(body, '\n'))
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, replacement)
	for _, fault := range []string{"scope unavailable", "receipt deleted", "receipt replaced"} {
		t.Run(fault, func(t *testing.T) {
			mutated, want := false, ""
			store.options.Git.Run = func(ctx context.Context, root string, args ...string) ([]byte, error) {
				if mutated && fault == "scope unavailable" {
					return nil, errors.New("configured origin probe unavailable")
				}
				return runGit(ctx, root, args...)
			}
			store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
				out, err := proposalGit(ctx, root, input, env, args...)
				if err == nil && args[0] == "commit-tree" {
					mutated = true
					if fault == "receipt deleted" {
						fixtureGit(t, o.Origin, "update-ref", "-d", receiptRef(b))
					}
					if fault == "receipt replaced" {
						fixtureGit(t, o.Origin, "update-ref", receiptRef(b), replacement)
					}
					want = fixtureGit(t, o.Origin, "show-ref")
				}
				return out, err
			}
			if result, err := store.LandCandidate(t.Context(), b); err == nil || result.Created || !mutated || fixtureGit(t, o.Origin, "show-ref") != want {
				t.Fatalf("changed approval or unreadable scope advanced refs: %v", err)
			}
			fixtureGit(t, o.Origin, "update-ref", receiptRef(b), review.OID)
			store.options.Git.Run, store.run = nil, proposalGit
		})
	}
	landed, err := store.LandCandidate(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	next := fixtureGit(t, o.Origin, "-c", "user.name=Fixture Author", "-c", "user.email=fixture@example.test", "commit-tree", landed.Landing.Candidate.Tree, "-p", landed.Landing.MergeCommit, "-m", "later mainline commit")
	want := ""
	store.run = func(ctx context.Context, root string, input []byte, env []string, args ...string) ([]byte, error) {
		out, err := proposalGit(ctx, root, input, env, args...)
		if err == nil && args[0] == "merge-base" && args[2] == landed.Landing.FeatureMarker {
			fixtureGit(t, o.Origin, "update-ref", b.TargetRef, next)
			want = fixtureGit(t, o.Origin, "show-ref")
		}
		return out, err
	}
	if result, err := store.VerifyCandidateLanding(t.Context(), b.ProposalID); !errors.Is(err, codehost.ErrProposalConflict) || result.Created || want == "" || fixtureGit(t, o.Origin, "show-ref") != want {
		t.Fatalf("mixed recovery reads certified delivery: %v", err)
	}
}
