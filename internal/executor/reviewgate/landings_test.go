package reviewgate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type landingPorts struct {
	packet       Packet
	result       codehost.TaskLandingResult
	fault        string
	writes       int
	afterLanding func()
}

func (p *landingPorts) Binding(context.Context, string) (codehost.ReviewBinding, error) {
	return p.packet.Binding, nil
}

func (p *landingPorts) Verify(context.Context, codehost.ReviewBinding) (codehost.ReceiptRecord, error) {
	if p.fault == "receipt unavailable" {
		return codehost.ReceiptRecord{}, errTaskInput
	}
	return p.result.Landing.Review, nil
}

func (p *landingPorts) LandTask(context.Context, codehost.ReviewBinding) (codehost.TaskLandingResult, error) {
	if p.fault == "landing unavailable" {
		return codehost.TaskLandingResult{}, errTaskInput
	}
	p.writes++
	if p.afterLanding != nil {
		p.afterLanding()
	}
	return p.result, nil
}

func TestTaskLandingGateValidatesCompletedMutationAgainstItsLeasedPolicy(t *testing.T) {
	f, p, gate := landingGateFixture(t)
	p.afterLanding = func() { f.policy.Prompts += "-next-policy" }
	result, err := gate.LandTask(t.Context(), p.packet.Binding)
	if err != nil || !result.Created || p.writes != 1 {
		t.Fatalf("later policy change invalidated completed landing: %+v %v", result, err)
	}
	if _, err := gate.VerifyTaskLanding(t.Context(), p.packet.Binding.ProposalID); err == nil || p.writes != 1 {
		t.Fatal("independent recovery failed to resolve current policy")
	}
}

func (p *landingPorts) VerifyTaskLanding(context.Context, string) (codehost.TaskLandingResult, error) {
	if p.fault == "verification unavailable" {
		return codehost.TaskLandingResult{}, errTaskInput
	}
	result := p.result
	result.Created = false
	return result, nil
}

func landingGateFixture(t *testing.T) (*requestFixture, *landingPorts, *TaskLandingGate) {
	t.Helper()
	f := newRequestFixture(t)
	requests, err := NewTaskRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt := codehost.ReviewReceipt{SchemaVersion: codehost.ReceiptSchema, Binding: packet.Binding, Reviewer: "declared operator"}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	merge := strings.Repeat("5", 40)
	p := &landingPorts{packet: packet, result: codehost.TaskLandingResult{Prepared: f.g, FeatureHead: merge, Created: true,
		Landing: codehost.TaskLanding{SchemaVersion: codehost.TaskLandingSchema, Feature: f.g.Proposal.Feature, Task: f.g.Proposal.Task, MergeCommit: merge,
			Aggregate: packet.Aggregate, Request: packet.Request,
			Review: codehost.ReceiptRecord{Receipt: receipt, OID: strings.Repeat("6", 40), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(append(body, '\n')))}}}}
	gate, err := NewTaskLandingGate(t.Context(), f, f, f, p, p)
	if err != nil {
		t.Fatal(err)
	}
	return f, p, gate
}

func TestTaskLandingGateRequiresCompletePortsAndMatchingApproval(t *testing.T) {
	f, p, gate := landingGateFixture(t)
	for _, ports := range []struct {
		proposals codehost.ScopedProposals
		receipts  codehost.ReceiptReader
		landings  codehost.TaskLandings
	}{{f, nil, p}, {f, p, nil}, {nil, p, p}} {
		if _, err := NewTaskLandingGate(t.Context(), ports.proposals, f, f, ports.receipts, ports.landings); !errors.Is(err, ErrIncomplete) {
			t.Fatalf("missing port accepted: %v", err)
		}
	}
	if _, err := gate.LandTask(t.Context(), codehost.ReviewBinding{}); err == nil || p.writes != 0 {
		t.Fatal("invalid approval reached mutation")
	}
	different := p.packet.Binding
	different.Sequence++
	if _, err := gate.LandTask(t.Context(), different); err == nil || p.writes != 0 {
		t.Fatal("different generation reached mutation")
	}
	result, err := gate.LandTask(t.Context(), p.packet.Binding)
	if err != nil || !result.Created || p.writes != 1 {
		t.Fatalf("valid whole-proof landing failed: %v", err)
	}
	f.g.Proposal.State = "landed"
	result, err = gate.LandTask(t.Context(), p.packet.Binding)
	if err != nil || result.Created || p.writes != 1 {
		t.Fatalf("landed recovery mutated again: %v", err)
	}
	if _, err := gate.LandTask(t.Context(), different); err == nil || p.writes != 1 {
		t.Fatal("landed recovery accepted another approval")
	}
}

func TestTaskLandingGateRefusesMissingCurrentInputsBeforeMutation(t *testing.T) {
	for _, fault := range []string{"proposal unavailable", "request read unavailable", "scope unavailable", "receipt unavailable", "landing unavailable"} {
		t.Run(fault, func(t *testing.T) {
			f, p, gate := landingGateFixture(t)
			f.fault, p.fault = fault, fault
			if _, err := gate.LandTask(t.Context(), p.packet.Binding); err == nil || p.writes != 0 {
				t.Fatalf("unavailable input allowed landing: %v", err)
			}
		})
	}
}

func TestTaskLandingGateRefusesPartialForeignAndStaleRecoveredProof(t *testing.T) {
	for _, fault := range []string{"verification unavailable", "scope unavailable", "policy unavailable", "changed policy", "foreign id", "invalid landing", "different feature", "different task", "foreign target", "invalid feature head", "wrong predecessor", "unprepared predecessor", "malformed aggregate"} {
		t.Run(fault, func(t *testing.T) {
			f, p, gate := landingGateFixture(t)
			f.fault, p.fault = fault, fault
			id := p.packet.Binding.ProposalID
			switch fault {
			case "changed policy":
				f.policy.Prompts += "-changed"
			case "foreign id":
				id = "different-task"
			case "invalid landing":
				p.result.Landing.SchemaVersion = "unknown"
			case "different feature":
				p.result.Landing.Feature += "-other"
			case "different task":
				p.result.Landing.Task += "-other"
			case "foreign target":
				f.scope.TargetRef += "-other"
			case "invalid feature head":
				p.result.FeatureHead = "abbreviated"
			case "wrong predecessor":
				p.result.Prepared.OID = strings.Repeat("9", 40)
			case "unprepared predecessor":
				p.result.Prepared.Proposal.TaskPreparation = nil
			case "malformed aggregate":
				p.result.Landing.Aggregate.Body = []byte("{\n")
			}
			if _, err := gate.VerifyTaskLanding(t.Context(), id); err == nil || p.writes != 0 {
				t.Fatalf("partial or stale landed proof accepted: %v", err)
			}
		})
	}
}
