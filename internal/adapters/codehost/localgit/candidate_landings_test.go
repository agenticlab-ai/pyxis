package localgit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	featurelocal "github.com/agenticlab-ai/pyxis/internal/adapters/featuredrive/localgit"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive/candidatereview"
)

func candidateLandingInputs(t *testing.T) (Options, *ProposalStore, *ReviewArtifactStore, *candidateReviewSource, *candidatereview.Requests, codehost.ProposalGeneration) {
	t.Helper()
	o, proposals, artifacts, reader, requests, opened := candidateReviewFixture(t)
	base, feature := opened.Proposal.TargetOID, opened.Proposal.Head
	fixtureGit(t, o.Root, "switch", "--detach", base)
	if err := os.WriteFile(filepath.Join(o.Root, "main.txt"), []byte("mainline change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "add", "main.txt")
	fixtureGit(t, o.Root, "commit", "-m", "mainline change")
	main := fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", o.Remote, main+":"+opened.Proposal.TargetRef)
	fixtureGit(t, o.Root, "switch", "--detach", feature)
	if err := os.WriteFile(filepath.Join(o.Root, "feature.txt"), []byte("feature change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Root, "add", "feature.txt")
	fixtureGit(t, o.Root, "commit", "-m", "feature change")
	feature = fixtureGit(t, o.Root, "rev-parse", "HEAD")
	fixtureGit(t, o.Root, "push", o.Remote, feature+":"+proposals.options.HeadRef)
	change := revision(opened, feature)
	change.ExpectedTargetOID = main
	opened, err := proposals.Publish(t.Context(), change)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := featuredrive.NewGitCandidateBuilder(o.Root).Build(t.Context(), feature, main, featuredrive.MergePolicy{Version: "merge-v1"}, "builder-v1")
	if err != nil {
		t.Fatal(err)
	}
	candidate.FeatureID = opened.Proposal.Feature
	reader.plan.Candidate, reader.evidence.Candidate = candidate, candidate
	for i := range reader.evidence.Results {
		reader.evidence.Results[i].CandidateID = candidate.Digest()
	}
	return o, proposals, artifacts, reader, requests, opened
}

func readyCandidateLanding(t *testing.T) (Options, *CandidateLandingStore, *candidateReviewSource, *candidatereview.LandingGate, candidatereview.Packet) {
	t.Helper()
	o, proposals, artifacts, reader, requests, opened := candidateLandingInputs(t)
	packet := seedCandidateReview(t, o, reader, opened)
	seedReceipt(t, o, packet.Binding)
	receipts := NewCandidateReceipts(proposals.options, requests)
	store := NewCandidateLandings(proposals.options, receipts)
	gate, err := candidatereview.NewLandingGate(t.Context(), proposals, artifacts, reader, receipts, store)
	if err != nil {
		t.Fatal(err)
	}
	return o, store, reader, gate, packet
}

// Keep one complete lifecycle from public preparation through retained recovery.
func readyCandidateLandingWorkflow(t *testing.T) (Options, *CandidateLandingStore, *candidateReviewSource, *candidatereview.LandingGate, candidatereview.Packet) {
	t.Helper()
	o, proposals, artifacts, reader, requests, opened := candidateLandingInputs(t)
	packet, err := requests.Prepare(t.Context(), opened.Proposal.ID, codehost.FinalCandidate(reader.plan.Candidate), reader.plan.ScenarioSetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	receipts := NewCandidateReceipts(proposals.options, requests)
	if _, err := receipts.Record(t.Context(), packet.Binding, "declared operator", ""); err != nil {
		t.Fatal(err)
	}
	store := NewCandidateLandings(proposals.options, receipts)
	gate, err := candidatereview.NewLandingGate(t.Context(), proposals, artifacts, reader, receipts, store)
	if err != nil {
		t.Fatal(err)
	}
	return o, store, reader, gate, packet
}

func TestCandidateLandingRefusesStaleInputsAndMissingProofWithoutWrites(t *testing.T) {
	t.Parallel()
	o, store, reader, gate, packet := readyCandidateLanding(t)
	b := packet.Binding
	refs := []string{b.TargetRef, store.options.HeadRef, b.Branch, proposalPrefix + b.ProposalID, b.Evidence.Ref, b.Request.Ref, receiptRef(b)}
	original, err := proposalRefs(t.Context(), o.Origin, refs...)
	if err != nil {
		t.Fatal(err)
	}
	next := fixtureGit(t, o.Root, "commit-tree", b.Head+"^{tree}", "-p", b.Head, "-m", "another workflow input")
	fixtureGit(t, o.Origin, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, next)
	originalResults := append([]featuredrive.ScenarioResult(nil), reader.evidence.Results...)
	for _, fault := range []string{"main", "feature", "branch", "proposal", "evidence", "request", "receipt", "scenario policy", "new failed scenario result", "attribution"} {
		t.Run(fault, func(t *testing.T) {
			ref := ""
			switch fault {
			case "main":
				ref = b.TargetRef
			case "feature":
				ref = store.options.HeadRef
			case "branch":
				ref = b.Branch
			case "proposal":
				ref = proposalPrefix + b.ProposalID
			case "evidence":
				ref = b.Evidence.Ref
			case "request":
				ref = b.Request.Ref
			case "receipt":
				ref = receiptRef(b)
			case "scenario policy":
				reader.plan.ScenarioSetFingerprint = "required-v2"
			case "new failed scenario result":
				failed := originalResults[0]
				failed.Outcome, failed.EvidenceRef = featuredrive.OutcomeFailed, "new-failed-run"
				failed.CompletedAt = failed.CompletedAt.Add(time.Second)
				reader.evidence.Results = append(append([]featuredrive.ScenarioResult(nil), originalResults...), failed)
			case "attribution":
				reader.fault = "attribution unavailable"
			}
			if ref != "" {
				fixtureGit(t, o.Origin, "update-ref", ref, next)
			}
			before := fixtureGit(t, o.Origin, "show-ref")
			if result, err := gate.LandCandidate(t.Context(), b); err == nil || result.Created || fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatalf("stale input allowed a partial landing: %v", err)
			}
			if ref != "" {
				fixtureGit(t, o.Origin, "update-ref", ref, original[ref])
			}
			reader.plan.ScenarioSetFingerprint = b.ScenarioSetFingerprint
			reader.evidence.Results = append([]featuredrive.ScenarioResult(nil), originalResults...)
			reader.fault = ""
		})
	}
}

func TestCandidateLandingRequiresFreshApprovalAfterAcceptedScenarioRerun(t *testing.T) {
	t.Parallel()
	o, store, reader, gate, old := readyCandidateLanding(t)
	reader.evidence.Results[0].CompletedAt = reader.evidence.Results[0].CompletedAt.Add(time.Second)
	reader.evidence.Results[0].EvidenceRef += "-accepted-rerun"
	p, a := NewProposals(store.options), NewReviewArtifacts(store.options)
	requests, err := candidatereview.NewRequests(t.Context(), p, a, reader)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), old.Binding.ProposalID, old.Binding.Candidate, old.Binding.ScenarioSetFingerprint)
	if err != nil || packet.Binding.ProposalOID == old.Binding.ProposalOID || packet.Binding.Nonce == old.Binding.Nonce {
		t.Fatalf("accepted rerun did not produce fresh review intent: %v", err)
	}
	before := fixtureGit(t, o.Origin, "show-ref")
	for _, binding := range []codehost.ReviewBinding{old.Binding, packet.Binding} {
		if _, err := gate.LandCandidate(t.Context(), binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
			t.Fatal("old or missing approval landed the accepted rerun")
		}
	}
	if _, err := NewCandidateReceipts(store.options, requests).Record(t.Context(), packet.Binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	result, err := gate.LandCandidate(t.Context(), packet.Binding)
	if err != nil || !result.Created || result.Landing.Review.Receipt.Binding != packet.Binding || result.Landing.Evidence.Identity != packet.Evidence.Identity {
		t.Fatalf("fresh approved rerun did not land with its exact evidence: %+v %v", result, err)
	}
	accepted, landed, current := acceptedLanding(packet, result)
	if err := featuredrive.VerifyDelivery(t.Context(), accepted, landed, current, featurelocal.NewCandidateDeliveryVerifier(gate)); err != nil {
		t.Fatalf("approved rerun delivery verification failed: %v", err)
	}
}

func TestCandidateLandingAtomicLeasesAndLostAcknowledgement(t *testing.T) {
	t.Parallel()
	o, store, packet := readyCandidateAdapter(t)
	b := packet.Binding
	// The ready fixture has an additional open round before preparation.
	prepared, err := NewProposals(store.options).Read(t.Context(), b.ProposalID)
	if err != nil {
		t.Fatal(err)
	}
	p := prepared.Proposal
	p.Sequence++
	archive := archiveRef(p)
	for _, ref := range []string{b.TargetRef, store.options.HeadRef, b.Branch, proposalPrefix + b.ProposalID, archive} {
		t.Run(ref, func(t *testing.T) {
			original, err := proposalRefs(t.Context(), o.Origin, ref)
			if err != nil {
				t.Fatal(err)
			}
			next := fixtureGit(t, o.Root, "commit-tree", b.Head+"^{tree}", "-p", b.Head, "-m", "concurrent ref change")
			fixtureGit(t, o.Origin, "fetch", "--no-tags", "--no-write-fetch-head", o.Root, next)
			want := ""
			store.push = func(ctx context.Context, root string, args ...string) error {
				fixtureGit(t, o.Origin, "update-ref", ref, next)
				want = fixtureGit(t, o.Origin, "show-ref")
				_, err := proposalGit(ctx, root, nil, nil, args...)
				return err
			}
			if result, err := store.LandCandidate(t.Context(), b); err == nil || result.Created || want == "" || fixtureGit(t, o.Origin, "show-ref") != want {
				t.Fatalf("stale lease advanced some landing refs: %v", err)
			}
			if original[ref] == "" {
				fixtureGit(t, o.Origin, "update-ref", "-d", ref)
			} else {
				fixtureGit(t, o.Origin, "update-ref", ref, original[ref])
			}
		})
	}
	store.push = nil
	fixtureGit(t, o.Origin, "config", "receive.advertiseAtomic", "false")
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := store.LandCandidate(t.Context(), b); !errors.Is(err, codehost.ErrAtomicUnavailable) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("unsupported atomic push used a fallback: %v", err)
	}
	fixtureGit(t, o.Origin, "config", "receive.advertiseAtomic", "true")
	store.push = func(ctx context.Context, root string, args ...string) error {
		if _, err := proposalGit(ctx, root, nil, nil, args...); err != nil {
			return err
		}
		return errors.New("landing reply lost")
	}
	result, err := store.LandCandidate(t.Context(), b)
	if err != nil || result.Created {
		t.Fatalf("lost acknowledgement did not recover exact landing: %v", err)
	}
	before = fixtureGit(t, o.Origin, "show-ref")
	store.push = nil
	again, err := store.LandCandidate(t.Context(), b)
	if err != nil || again.Created || !reflect.DeepEqual(again.Landing, result.Landing) || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatalf("retry delivered twice: %v", err)
	}
}

func acceptedLanding(packet candidatereview.Packet, result codehost.CandidateLandingResult) (featuredrive.AcceptedCandidate, featuredrive.LandedRevision, featuredrive.CurrentMain) {
	c := featuredrive.CandidateIdentity(result.Landing.Candidate)
	b := packet.Binding
	return featuredrive.AcceptedCandidate{Candidate: c, Proposal: featuredrive.FinalProposal{FeatureID: c.FeatureID, ProposalID: b.ProposalID, Head: b.Head, FeatureHead: c.FeatureHead, MainHead: c.MainHead, Candidate: c, ScenarioSetFingerprint: b.ScenarioSetFingerprint, EvidenceID: b.Evidence.Digest}},
		featuredrive.LandedRevision{Revision: result.Landing.MergeCommit, Base: c.MainHead, Tree: c.Tree}, featuredrive.CurrentMain{Revision: result.MainHead}
}

func TestCandidateLandingPreservesMergedTreeAndRecoversWithoutAuthorCheckout(t *testing.T) {
	t.Parallel()
	o, store, reader, gate, packet := readyCandidateLandingWorkflow(t)
	result, err := gate.LandCandidate(t.Context(), packet.Binding)
	if err != nil || !result.Created || result.Landing.Validate() != nil {
		t.Fatalf("candidate landing: %+v %v", result, err)
	}
	l, c := result.Landing, result.Landing.Candidate
	for _, shape := range [][2]string{{l.MergeCommit, c.MainHead}, {l.FeatureMarker, c.FeatureHead}} {
		if fixtureGit(t, o.Origin, "rev-list", "--parents", "-n", "1", shape[0]) != shape[0]+" "+shape[1] || fixtureGit(t, o.Origin, "rev-parse", shape[0]+"^{tree}") != c.Tree {
			t.Fatal("wrong candidate squash or marker shape")
		}
		if fixtureGit(t, o.Origin, "show", shape[0]+":main.txt") != "mainline change" || fixtureGit(t, o.Origin, "show", shape[0]+":feature.txt") != "feature change" {
			t.Fatal("delivery dropped mainline or feature changes")
		}
	}
	if _, err := proposalGit(t.Context(), o.Origin, nil, nil, "merge-base", "--is-ancestor", l.FeatureMarker, l.MergeCommit); err == nil {
		t.Fatal("fixture did not distinguish delivered L from feature marker")
	}
	final, err := NewProposals(store.options).Read(t.Context(), packet.Binding.ProposalID)
	if err != nil || final.Proposal.PreviousOID != packet.Binding.ProposalOID || final.Proposal.Sequence != packet.Binding.Sequence+1 || final.Proposal.CandidatePreparation != nil {
		t.Fatalf("missing reviewed predecessor: %v", err)
	}
	refs, err := proposalRefs(t.Context(), o.Origin, packet.Binding.Branch, final.Proposal.ArchiveRef)
	if err != nil || refs[packet.Binding.Branch] != "" || refs[final.Proposal.ArchiveRef] != final.OID {
		t.Fatal("final landing did not archive and delete atomically")
	}
	options := store.options
	options.Git = proposalClone(t, o)
	if err := os.RemoveAll(o.Root); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "gc", "--prune=now")
	reader.fault = "evidence unavailable"
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
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := clone.LandCandidate(t.Context(), packet.Binding); err == nil || fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("unavailable current scenario results certified delivery or changed origin")
	}
	reader.fault = ""
	readsBeforeRecovery := reader.evidenceReads
	recovered, err := clone.LandCandidate(t.Context(), packet.Binding)
	if err != nil || recovered.Created || !reflect.DeepEqual(recovered.Landing, l) || fixtureGit(t, o.Origin, "show-ref") != before || reader.evidenceReads != readsBeforeRecovery+1 {
		t.Fatalf("retained delivery repeated work: %+v %v", recovered, err)
	}
	accepted, landed, current := acceptedLanding(packet, recovered)
	if err := featuredrive.VerifyDelivery(t.Context(), accepted, landed, current, featurelocal.NewCandidateDeliveryVerifier(clone)); err != nil {
		t.Fatalf("valid delivered L refused: %v", err)
	}
	landed.Revision = l.FeatureMarker
	if err := featuredrive.VerifyDelivery(t.Context(), accepted, landed, current, featurelocal.NewCandidateDeliveryVerifier(clone)); err == nil {
		t.Fatal("feature marker was certified as delivered mainline revision")
	}
}
