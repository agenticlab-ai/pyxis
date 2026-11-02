package featuredrive

import (
	"context"
	"errors"
	"testing"
)

type verifierStub struct {
	proof LandingProof
	err   error
}

func (v verifierStub) VerifyLanding(context.Context, ProposalObservation, ReviewObservation, string) (LandingProof, error) {
	return v.proof, v.err
}

type recorderStub struct {
	calls int
	task  string
}

func (r *recorderStub) RecordLanding(context.Context, string, string, LandingProof) error {
	r.calls++
	return nil
}

func proposal() ProposalObservation {
	return ProposalObservation{TaskID: "T1", ProposalID: "P1", Head: "head", Base: "refs/heads/feature/f-1", ChecksGreen: true, Open: true}
}

func TestRecordLandingRequiresReviewedCurrentHead(t *testing.T) {
	r := &recorderStub{}
	err := RecordLanding(context.Background(), verifierStub{proof: LandingProof{ReviewedHead: "old", LandedRevision: "landed", FeatureHead: "feature"}}, r, proposal(), ReviewObservation{ProposalID: "P1", Head: "head", State: "approved"}, "feature")
	if !errors.Is(err, ErrLandingNotProven) || r.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
}

func TestRecordLandingRecordsOnlyVerifiedProof(t *testing.T) {
	r := &recorderStub{}
	err := RecordLanding(context.Background(), verifierStub{proof: LandingProof{ReviewedHead: "head", ReviewedBase: "refs/heads/feature/f-1", LandedRevision: "landed", FeatureHead: "feature", FeatureBase: "refs/heads/feature/f-1"}}, r, proposal(), ReviewObservation{ProposalID: "P1", Head: "head", State: "approved"}, "feature")
	if err != nil || r.calls != 1 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
}

func TestRecordLandingRejectsWrongBase(t *testing.T) {
	r := &recorderStub{}
	err := RecordLanding(context.Background(), verifierStub{proof: LandingProof{
		ReviewedHead: "head", ReviewedBase: "refs/heads/main", LandedRevision: "landed",
		FeatureHead: "feature", FeatureBase: "refs/heads/main",
	}}, r, proposal(), ReviewObservation{ProposalID: "P1", Head: "head", State: "approved"}, "feature")
	if !errors.Is(err, ErrLandingNotProven) || r.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
}

func TestRecordLandingRejectsUnmergedProposal(t *testing.T) {
	r := &recorderStub{}
	p := proposal()
	p.Open = false
	err := RecordLanding(context.Background(), verifierStub{proof: LandingProof{
		ReviewedHead: "head", ReviewedBase: p.Base, LandedRevision: "landed",
		FeatureHead: "feature", FeatureBase: p.Base,
	}}, r, p, ReviewObservation{ProposalID: "P1", Head: "head", State: "approved"}, "feature")
	if !errors.Is(err, ErrLandingNotProven) || r.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
}

func TestRecordLandingRejectsDifferentObservedLandedRevision(t *testing.T) {
	r := &recorderStub{}
	p := proposal()
	p.Landed = "landed-observed"
	err := RecordLanding(context.Background(), verifierStub{proof: LandingProof{
		ReviewedHead: "head", ReviewedBase: p.Base, LandedRevision: "landed-other",
		FeatureHead: "feature", FeatureBase: p.Base,
	}}, r, p, ReviewObservation{ProposalID: "P1", Head: "head", State: "approved"}, "feature")
	if !errors.Is(err, ErrLandingNotProven) || r.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, r.calls)
	}
}

func TestReadyFrontierRequiresLandedBlockers(t *testing.T) {
	tasks := []FrontierTask{{ID: "T1", Open: true, Landed: false}, {ID: "T2", Open: true, BlockedBy: []string{"T1"}}, {ID: "T3", Open: true, BlockedBy: []string{"T1"}, Claimed: true}}
	if got := ReadyFrontier(tasks); len(got) != 1 || got[0] != "T1" {
		t.Fatalf("frontier=%v", got)
	}
	tasks[0].Landed = true
	if got := ReadyFrontier(tasks); len(got) != 1 || got[0] != "T2" {
		t.Fatalf("frontier=%v", got)
	}
}
