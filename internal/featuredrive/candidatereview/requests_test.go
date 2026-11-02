package candidatereview

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/featuredrive"
)

var errRequestInput = errors.New("candidate request input unavailable")

func TestPrepareRotatesOnlyAcceptedAuthorizedRerunEvidence(t *testing.T) {
	for _, change := range []string{"accepted rerun", "failed rerun", "conflicting rerun", "partial results", "results unavailable", "attribution unavailable", "policy changed"} {
		t.Run(change, func(t *testing.T) {
			f := newRequestSource(t)
			requests, err := NewRequests(t.Context(), f, f, f)
			if err != nil {
				t.Fatal(err)
			}
			candidate, scenarios := codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint
			old, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, scenarios)
			if err != nil {
				t.Fatal(err)
			}
			generation, writes := f.g, f.writes
			result := f.evidence.Results[0]
			result.EvidenceRef += "-rerun"
			result.CompletedAt = result.CompletedAt.Add(time.Second)
			switch change {
			case "accepted rerun", "attribution unavailable":
				f.evidence.Results[0] = result
				if change == "attribution unavailable" {
					f.source.fault = "attribution"
				}
			case "failed rerun":
				result.Outcome = featuredrive.OutcomeFailed
				f.evidence.Results = append(f.evidence.Results, result)
			case "conflicting rerun":
				result.CompletedAt = f.evidence.Results[0].CompletedAt
				result.Outcome = featuredrive.OutcomeFailed
				f.evidence.Results = append(f.evidence.Results, result)
			case "partial results":
				f.evidence.Results = f.evidence.Results[:1]
			case "results unavailable":
				f.source.fault = "evidence"
			case "policy changed":
				f.plan.ScenarioSetFingerprint += "-new"
			}
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, scenarios)
			if change != "accepted rerun" {
				if err == nil || errors.Is(err, ErrEvidenceChanged) || !reflect.DeepEqual(f.g, generation) || f.writes != writes {
					t.Fatalf("invalid rerun replaced retained intent: %v", err)
				}
				return
			}
			if err != nil || packet.Binding.ProposalOID == old.Binding.ProposalOID || packet.Binding.Nonce == old.Binding.Nonce || packet.Binding.Evidence == old.Binding.Evidence || packet.Binding.Sequence != old.Binding.Sequence+1 || f.writes != writes+1 {
				t.Fatalf("accepted rerun did not create fresh review intent: %+v %v", packet.Binding, err)
			}
			if requests.ValidateReview(t.Context(), old.Binding) == nil {
				t.Fatal("previous approval remained eligible after accepted rerun")
			}
			resumed, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, scenarios)
			if err != nil || !reflect.DeepEqual(resumed, packet) || f.writes != writes+1 {
				t.Fatalf("unchanged rerun rotated again: %v", err)
			}
			if string(f.rows[old.Request.Identity.Ref].Body) != string(old.Request.Body) || string(f.rows[old.Evidence.Identity.Ref].Body) != string(old.Evidence.Body) {
				t.Fatal("rotation changed immutable historical proof")
			}
		})
	}
}

func TestCandidateReviewRejectsNewFailedOrConflictingResults(t *testing.T) {
	for _, change := range []string{"new failed result", "conflicting terminal result", "different passing result", "current results unavailable"} {
		t.Run(change, func(t *testing.T) {
			f := newRequestSource(t)
			reader := &leasedEvidenceReader{requestSource: f}
			requests, err := NewRequests(t.Context(), f, f, reader)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint)
			if err != nil {
				t.Fatal(err)
			}
			generation, writes := f.g, f.writes
			result := f.evidence.Results[0]
			result.EvidenceRef += "-new"
			switch change {
			case "new failed result":
				result.Outcome = featuredrive.OutcomeFailed
				result.CompletedAt = result.CompletedAt.Add(time.Second)
				f.evidence.Results = append(f.evidence.Results, result)
			case "conflicting terminal result":
				result.Outcome = featuredrive.OutcomeFailed
				f.evidence.Results = append(f.evidence.Results, result)
			case "different passing result":
				f.evidence.Results[0] = result
			case "current results unavailable":
				f.source.fault = "evidence"
			}
			if _, err := requests.Observe(t.Context(), f.g.Proposal.ID); err == nil {
				t.Fatal("changed result set was presented as accepted retained proof")
			}
			if err := requests.ValidateReview(t.Context(), packet.Binding); err == nil {
				t.Fatal("changed result set remained eligible for approval")
			}
			called := false
			if err := requests.WithReview(t.Context(), packet.Binding, func() error { called = true; return nil }); err == nil || called {
				t.Fatal("changed result set reached a mutation callback")
			}
			if !reflect.DeepEqual(f.g, generation) || f.writes != writes || string(f.rows[packet.Evidence.Identity.Ref].Body) != string(packet.Evidence.Body) || string(f.rows[packet.Request.Identity.Ref].Body) != string(packet.Request.Body) {
				t.Fatal("result-set refusal changed retained intent")
			}
		})
	}
}

type requestSource struct {
	*source
	t        *testing.T
	g        codehost.ProposalGeneration
	scope    codehost.ProposalScope
	rows     map[string]codehost.ReviewArtifact
	fault    string
	writes   int
	retained bool
}

func newRequestSource(t *testing.T) *requestSource {
	t.Helper()
	b, _, reader := fixture(t)
	g := b.Generation
	g.Proposal.State = "open"
	g.Proposal.Sequence = 1
	g.Proposal.CandidatePreparation = nil
	return &requestSource{source: reader, t: t, g: g, scope: codehost.ProposalScope{SchemaVersion: codehost.ProposalScopeSchema, Origin: b.Origin, Kind: "implementation", MainlineRef: g.Proposal.TargetRef, TargetRef: g.Proposal.TargetRef, HeadRef: g.Proposal.SourceRef}, rows: make(map[string]codehost.ReviewArtifact)}
}
func (f *requestSource) Scope(context.Context) (codehost.ProposalScope, error) {
	if f.fault == "scope unavailable" || (f.retained && f.fault == "final scope unavailable") {
		return codehost.ProposalScope{}, errRequestInput
	}
	return f.scope, nil
}
func (f *requestSource) List(context.Context) ([]codehost.ProposalGeneration, error) {
	return []codehost.ProposalGeneration{f.g}, nil
}
func (f *requestSource) Read(context.Context, string) (codehost.ProposalGeneration, error) {
	if f.fault == "proposal unavailable" || (f.retained && f.fault == "final proposal unavailable") {
		return codehost.ProposalGeneration{}, errRequestInput
	}
	return f.g, nil
}
func (f *requestSource) Publish(_ context.Context, change codehost.ProposalChange) (codehost.ProposalGeneration, error) {
	if f.fault == "prepare unavailable" {
		return codehost.ProposalGeneration{}, errRequestInput
	}
	f.writes++
	f.g.Proposal.Sequence++
	f.g.Proposal.State = change.State
	f.g.Proposal.CandidatePreparation = change.CandidatePreparation
	f.g.OID = fmt.Sprintf("%040x", f.g.Proposal.Sequence)
	return f.g, nil
}
func (f *requestSource) Evidence(ctx context.Context, b Binding) (featuredrive.CandidateEvidence, error) {
	e, err := f.source.Evidence(ctx, b)
	if f.fault == "oversized evidence" {
		e.Results[0].EvidenceRef = strings.Repeat("x", codehost.MaxReviewArtifactBytes)
	}
	if f.fault == "partial evidence" {
		e.Results = e.Results[:1]
	}
	return e, err
}
func (f *requestSource) Retain(_ context.Context, g codehost.ProposalGeneration, kind string, body []byte) (codehost.ReviewArtifact, error) {
	if (kind == codehost.CandidateEvidence && f.fault == "evidence publication unavailable") || (kind == codehost.CandidateRequest && f.fault == "request publication unavailable") {
		return codehost.ReviewArtifact{}, errRequestInput
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	ref := "refs/pyxis/candidate-evidence/" + g.Proposal.ID + "/" + digest
	if kind == codehost.CandidateRequest {
		ref = "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID
	}
	r := codehost.ReviewArtifact{Identity: codehost.ArtifactRef{Ref: ref, OID: digest[:40], Digest: "sha256:" + digest}, Kind: kind, Body: body, Trust: codehost.Trust{SharedAccount: true}}
	f.rows[ref] = r
	if kind == codehost.CandidateEvidence && f.fault == "wrong evidence namespace" {
		r.Identity.Ref = "refs/pyxis/candidate-evidence/foreign/" + digest
	}
	if kind == codehost.CandidateRequest {
		f.retained = true
		switch f.fault {
		case "generation moved":
			f.g.OID = strings.Repeat("9", 40)
			f.g.Proposal.Sequence++
		case "origin changed":
			f.scope.Origin += "/changed"
		case "malformed request":
			r.Body = []byte("{")
		case "different request generation":
			request, err := codehost.ParseCandidateRequest(body)
			if err != nil {
				f.t.Fatal(err)
			}
			request.Binding.Sequence++
			r.Body, err = request.Canonical()
			if err != nil {
				f.t.Fatal(err)
			}
		case "wrong request namespace":
			r.Identity.Ref = "refs/pyxis/review-requests/foreign/" + g.OID
		}
	}
	return r, nil
}
func (f *requestSource) ReadRetained(_ context.Context, _ codehost.ProposalGeneration, kind string, identity codehost.ArtifactRef) (codehost.ReviewArtifact, error) {
	if f.fault == "evidence read unavailable" {
		return codehost.ReviewArtifact{}, errRequestInput
	}
	r := f.rows[identity.Ref]
	if kind == codehost.CandidateEvidence && f.fault == "malformed evidence" {
		r.Body = []byte("{")
	}
	return r, nil
}
func (f *requestSource) ReadRequest(_ context.Context, g codehost.ProposalGeneration) (codehost.ReviewArtifact, error) {
	if f.fault == "request read unavailable" || (f.retained && f.fault == "final request unavailable") {
		return codehost.ReviewArtifact{}, errRequestInput
	}
	r, ok := f.rows["refs/pyxis/review-requests/"+g.Proposal.ID+"/"+g.OID]
	if !ok {
		return r, codehost.ErrReviewArtifactNotFound
	}
	return r, nil
}

func TestRequestsRefuseUnavailablePartialOrMovedProof(t *testing.T) {
	for _, fault := range []string{"scope unavailable", "proposal unavailable", "prepare unavailable", "plan", "evidence", "attribution", "partial evidence", "oversized evidence", "evidence publication unavailable", "request publication unavailable", "wrong evidence namespace", "malformed request", "different request generation", "evidence read unavailable", "malformed evidence", "final scope unavailable", "origin changed", "final proposal unavailable", "generation moved", "final request unavailable", "wrong request namespace"} {
		t.Run(fault, func(t *testing.T) {
			f := newRequestSource(t)
			requests, err := NewRequests(t.Context(), f, f, f)
			if err != nil {
				t.Fatal(err)
			}
			f.fault = fault
			f.source.fault = fault
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint)
			if err == nil || packet.SchemaVersion != "" {
				t.Fatalf("failed input presented final review: %v", err)
			}
		})
	}
}

func TestRequestsObserveAndResumeWithoutRerunningOrRepairing(t *testing.T) {
	f := newRequestSource(t)
	requests, err := NewRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"", "scope unavailable", "proposal unavailable", "request read unavailable"} {
		f.fault = fault
		if _, err := requests.Observe(t.Context(), f.g.Proposal.ID); err == nil || f.writes != 0 {
			t.Fatal("observation prepared or repaired a request")
		}
	}
	f.fault = ""
	candidate := codehost.FinalCandidate(f.plan.Candidate)
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, f.plan.ScenarioSetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	f.fault = "request read unavailable"
	if _, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, f.plan.ScenarioSetFingerprint); err == nil || f.writes != 1 {
		t.Fatal("unreadable existing request was replaced")
	}
	f.fault = ""
	f.source.fault = "evidence"
	if _, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, f.plan.ScenarioSetFingerprint); err == nil || f.writes != 1 {
		t.Fatal("unavailable current results reused or replaced a prepared request")
	}
	f.source.fault = ""
	resumed, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, f.plan.ScenarioSetFingerprint)
	if err != nil || !reflect.DeepEqual(resumed, packet) || f.writes != 1 || f.evidenceReads != 5 {
		t.Fatalf("complete proof was not resumed: %v", err)
	}
	delete(f.rows, packet.Request.Identity.Ref)
	f.source.fault = ""
	newPacket, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, f.plan.ScenarioSetFingerprint)
	if err != nil || newPacket.Binding.Nonce == packet.Binding.Nonce || newPacket.Binding.ProposalOID == packet.Binding.ProposalOID {
		t.Fatalf("interrupted preparation reused request intent: %v", err)
	}
}

func TestRequestsRequireBoundFinalWorkflowAndCandidateInputs(t *testing.T) {
	f := newRequestSource(t)
	for _, ports := range [][3]any{{nil, f, f}, {f, nil, f}, {f, f, nil}} {
		p, _ := ports[0].(codehost.ScopedProposals)
		a, _ := ports[1].(codehost.ReviewArtifacts)
		r, _ := ports[2].(Reader)
		if _, err := NewRequests(t.Context(), p, a, r); err == nil {
			t.Fatal("missing port accepted")
		}
	}
	for _, fault := range []string{"schema", "kind", "target", "source", "origin"} {
		f := newRequestSource(t)
		switch fault {
		case "schema":
			f.scope.SchemaVersion = "unknown"
		case "kind":
			f.scope.Kind = "draft"
		case "target":
			f.scope.TargetRef = f.scope.HeadRef
		case "source":
			f.scope.HeadRef = ""
		case "origin":
			f.scope.Origin = ""
		}
		if _, err := NewRequests(t.Context(), f, f, f); err == nil {
			t.Fatal("wrong workflow accepted")
		}
	}
	for _, fault := range []string{"task", "state", "source", "target", "feature", "head", "main", "scenarios"} {
		f := newRequestSource(t)
		requests, err := NewRequests(t.Context(), f, f, f)
		if err != nil {
			t.Fatal(err)
		}
		candidate, scenarios := codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint
		switch fault {
		case "task":
			f.g.Proposal.Task = "T1"
		case "state":
			f.g.Proposal.State = "abandoned"
		case "source":
			f.g.Proposal.SourceRef = "refs/heads/foreign"
		case "target":
			f.g.Proposal.TargetRef = "refs/heads/foreign"
		case "feature":
			candidate.FeatureID = "other"
		case "head":
			candidate.FeatureHead = strings.Repeat("9", 40)
		case "main":
			candidate.MainHead = strings.Repeat("9", 40)
		case "scenarios":
			scenarios = ""
		}
		if _, err := requests.Prepare(t.Context(), f.g.Proposal.ID, candidate, scenarios); err == nil || f.writes != 0 {
			t.Fatal("wrong final inputs advanced shared state")
		}
	}
}

func TestFinalReceiptValidationRefreshesPolicyAttributionAndSavedIntent(t *testing.T) {
	f := newRequestSource(t)
	requests, err := NewRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if err := requests.ValidateReview(t.Context(), packet.Binding); err != nil {
		t.Fatal(err)
	}
	wrong := packet.Binding
	wrong.Nonce = strings.Repeat("f", 32)
	if requests.ValidateReview(t.Context(), wrong) == nil {
		t.Fatal("unpresented final nonce accepted")
	}
	wrong.Operation = codehost.TaskMerge
	if requests.ValidateReview(t.Context(), wrong) == nil {
		t.Fatal("task operation accepted by final policy verifier")
	}
	f.source.fault = "attribution"
	if requests.ValidateReview(t.Context(), packet.Binding) == nil {
		t.Fatal("unavailable source attribution retained final approval")
	}
}
