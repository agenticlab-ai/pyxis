package candidatereview

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type landingPorts struct {
	packet       Packet
	result       codehost.CandidateLandingResult
	fault        string
	writes       int
	afterLanding func()
}

func (p *landingPorts) Binding(context.Context, string) (codehost.ReviewBinding, error) {
	return p.packet.Binding, nil
}
func (p *landingPorts) Verify(context.Context, codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	if p.fault == "receipt unavailable" {
		return codehost.ReceiptRecord{}, errRequestInput
	}
	return p.result.Landing.Review, nil
}
func (p *landingPorts) LandCandidate(context.Context, codehost.ReviewBinding) (codehost.CandidateLandingResult, error) {
	if p.fault == "landing unavailable" {
		return codehost.CandidateLandingResult{}, errRequestInput
	}
	p.writes++
	if p.afterLanding != nil {
		p.afterLanding()
	}
	result := p.result
	if p.fault == "different returned approval" {
		result.Landing.Review.Receipt.Binding.Sequence++
	}
	return result, nil
}

func TestCandidateLandingGateValidatesCompletedMutationAgainstLeasedEvidence(t *testing.T) {
	f, p, gate := landingFixture(t)
	p.afterLanding = func() { f.plan.ScenarioSetFingerprint += "-next-policy" }
	result, err := gate.LandCandidate(t.Context(), p.packet.Binding)
	if err != nil || !result.Created || p.writes != 1 {
		t.Fatalf("later policy change invalidated completed candidate: %+v %v", result, err)
	}
	if _, err := gate.VerifyCandidateLanding(t.Context(), p.packet.Binding.ProposalID); err == nil || p.writes != 1 {
		t.Fatal("independent recovery failed to resolve current evidence")
	}
}
func (p *landingPorts) VerifyCandidateLanding(context.Context, string) (codehost.CandidateLandingResult, error) {
	if p.fault == "verification unavailable" {
		return codehost.CandidateLandingResult{}, errRequestInput
	}
	result := p.result
	result.Created = false
	return result, nil
}

func landingFixture(t *testing.T) (*requestSource, *landingPorts, *LandingGate) {
	t.Helper()
	f := newRequestSource(t)
	requests, err := NewRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID, codehost.FinalCandidate(f.plan.Candidate), f.plan.ScenarioSetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	receipt := codehost.ReviewReceipt{SchemaVersion: codehost.ReceiptSchema, Binding: packet.Binding, Reviewer: "declared operator"}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	merge, marker := strings.Repeat("5", 40), strings.Repeat("7", 40)
	p := &landingPorts{packet: packet, result: codehost.CandidateLandingResult{Prepared: f.g, MainHead: merge, FeatureHead: marker, Created: true,
		Landing: codehost.CandidateLanding{SchemaVersion: codehost.CandidateLandingSchema, Candidate: packet.Binding.Candidate, MergeCommit: merge, FeatureMarker: marker, Evidence: packet.Evidence, Request: packet.Request,
			Review: codehost.ReceiptRecord{Receipt: receipt, OID: strings.Repeat("6", 40), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n')))}}}}
	gate, err := NewLandingGate(t.Context(), f, f, f, p, p)
	if err != nil {
		t.Fatal(err)
	}
	return f, p, gate
}

func TestCandidateLandingGateRequiresMatchingApprovalAndRecoversReadOnly(t *testing.T) {
	f, p, gate := landingFixture(t)
	for _, ports := range []struct {
		proposals codehost.ScopedProposals
		receipts  codehost.ReceiptReader
		landings  codehost.CandidateLandings
	}{{f, nil, p}, {f, p, nil}, {nil, p, p}} {
		if _, err := NewLandingGate(t.Context(), ports.proposals, f, f, ports.receipts, ports.landings); err == nil {
			t.Fatal("missing port accepted")
		}
	}
	if _, err := gate.LandCandidate(t.Context(), codehost.ReviewBinding{}); err == nil || p.writes != 0 {
		t.Fatal("invalid approval reached mutation")
	}
	different := p.packet.Binding
	different.Sequence++
	if _, err := gate.LandCandidate(t.Context(), different); err == nil || p.writes != 0 {
		t.Fatal("different generation reached mutation")
	}
	result, err := gate.LandCandidate(t.Context(), p.packet.Binding)
	if err != nil || !result.Created || p.writes != 1 {
		t.Fatalf("complete landing refused: %v", err)
	}
	f.g.Proposal.State = "landed"
	result, err = gate.LandCandidate(t.Context(), p.packet.Binding)
	if err != nil || result.Created || p.writes != 1 {
		t.Fatalf("recovery repeated landing: %v", err)
	}
	if _, err := gate.LandCandidate(t.Context(), different); err == nil || p.writes != 1 {
		t.Fatal("recovery accepted another approval")
	}
}

func TestCandidateLandingGateRefusesUnavailableInputsAndForeignResults(t *testing.T) {
	for _, fault := range []string{"proposal unavailable", "request read unavailable", "scope unavailable", "receipt unavailable", "landing unavailable", "different returned approval"} {
		t.Run(fault, func(t *testing.T) {
			f, p, gate := landingFixture(t)
			f.fault, p.fault = fault, fault
			if _, err := gate.LandCandidate(t.Context(), p.packet.Binding); err == nil {
				t.Fatal("unavailable or foreign proof accepted")
			}
			if fault != "different returned approval" && p.writes != 0 {
				t.Fatal("unavailable input reached mutation")
			}
		})
	}
}

func TestCandidateLandingGateRefreshesRetainedPolicyAndAttribution(t *testing.T) {
	for _, fault := range []string{"verification unavailable", "scope unavailable", "plan", "attribution", "changed policy", "foreign id", "invalid landing", "foreign target", "foreign source", "invalid main", "invalid feature", "wrong predecessor", "unprepared predecessor", "malformed evidence"} {
		t.Run(fault, func(t *testing.T) {
			f, p, gate := landingFixture(t)
			f.fault, p.fault, f.source.fault = fault, fault, fault
			id := p.packet.Binding.ProposalID
			switch fault {
			case "changed policy":
				f.plan.ScenarioSetFingerprint += "-changed"
			case "foreign id":
				id += "-other"
			case "invalid landing":
				p.result.Landing.SchemaVersion = "unknown"
			case "foreign target":
				f.scope.TargetRef += "-other"
			case "foreign source":
				f.scope.HeadRef += "-other"
			case "invalid main":
				p.result.MainHead = "abbreviated"
			case "invalid feature":
				p.result.FeatureHead = "abbreviated"
			case "wrong predecessor":
				p.result.Prepared.OID = strings.Repeat("9", 40)
			case "unprepared predecessor":
				p.result.Prepared.Proposal.CandidatePreparation = nil
			case "malformed evidence":
				p.result.Landing.Evidence.Body = []byte("{\n")
			}
			if _, err := gate.VerifyCandidateLanding(t.Context(), id); err == nil || p.writes != 0 {
				t.Fatalf("stale recovered proof accepted: %v", err)
			}
		})
	}
}
