package reviewgate

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

var errTaskInput = errors.New("task input unavailable")

func TestTaskReviewRejectsChangedLiveEvidenceWithoutChangingSavedIntent(t *testing.T) {
	for _, change := range []string{"required check red", "code gate failed", "different completed aggregate", "different passing gate", "oversized current gate", "current aggregate unavailable"} {
		t.Run(change, func(t *testing.T) {
			f := newRequestFixture(t)
			reader := &leasedPolicyReader{requestFixture: f}
			requests, err := NewTaskRequests(t.Context(), f, f, reader)
			if err != nil {
				t.Fatal(err)
			}
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
			if err != nil {
				t.Fatal(err)
			}
			generation, writes := f.g, f.writes
			switch change {
			case "required check red":
				f.snapshot.Checks[0].State = port.CheckRed
			case "code gate failed":
				f.snapshot.CodeGate.Passed = false
			case "different completed aggregate":
				f.snapshot.Review.Aggregate.Report.Rounds[0].ContextID += "-new"
			case "different passing gate":
				f.snapshot.CodeGate.Outcomes[0].Output += "new run"
			case "oversized current gate":
				f.snapshot.CodeGate.Outcomes[0].Output = strings.Repeat("x", MaxSnapshotBytes)
			case "current aggregate unavailable":
				f.fault = "native unavailable"
			}
			if _, err := requests.Observe(t.Context(), f.g.Proposal.ID); err == nil {
				t.Fatal("changed current evidence was presented as the retained green gate")
			}
			if err := requests.ValidateReview(t.Context(), packet.Binding); err == nil {
				t.Fatal("changed current evidence remained eligible for approval")
			}
			called := false
			if err := requests.WithReview(t.Context(), packet.Binding, func() error { called = true; return nil }); err == nil || called {
				t.Fatal("changed current evidence reached a mutation callback")
			}
			if !reflect.DeepEqual(f.g, generation) || f.writes != writes || string(f.rows[packet.Aggregate.Identity.Ref].Body) != string(packet.Aggregate.Body) || string(f.rows[packet.Request.Identity.Ref].Body) != string(packet.Request.Body) {
				t.Fatal("current-evidence refusal changed retained intent")
			}
		})
	}
}

type requestFixture struct {
	t        *testing.T
	g        codehost.ProposalGeneration
	scope    codehost.ProposalScope
	snapshot Snapshot
	policy   Policy
	rows     map[string]codehost.ReviewArtifact
	fault    string
	started  bool
	retained bool
	writes   int
}

func newRequestFixture(t *testing.T) *requestFixture {
	t.Helper()
	b, snapshot, policy := completeFixture(t)
	p := codehost.Proposal{SchemaVersion: codehost.ProposalSchema, ID: b.ProposalID, Kind: "implementation", Feature: b.Feature, Task: b.Task, Branch: b.Branch, Head: b.Head, TargetRef: b.TargetRef, TargetOID: b.TargetOID, State: "open", Sequence: 1}
	return &requestFixture{t: t, g: codehost.ProposalGeneration{OID: b.ProposalOID, Proposal: p}, scope: codehost.ProposalScope{SchemaVersion: codehost.ProposalScopeSchema, Origin: b.Origin, Kind: "implementation", MainlineRef: "refs/heads/trunk", TargetRef: b.TargetRef}, snapshot: snapshot, policy: policy, rows: make(map[string]codehost.ReviewArtifact)}
}

func (f *requestFixture) Scope(context.Context) (codehost.ProposalScope, error) {
	if f.fault == "scope unavailable" {
		return codehost.ProposalScope{}, errTaskInput
	}
	return f.scope, nil
}

func (f *requestFixture) List(context.Context) ([]codehost.ProposalGeneration, error) {
	return []codehost.ProposalGeneration{f.g}, nil
}

func (f *requestFixture) Read(context.Context, string) (codehost.ProposalGeneration, error) {
	if f.fault == "proposal unavailable" || (f.retained && f.fault == "final proposal unavailable") {
		return codehost.ProposalGeneration{}, errTaskInput
	}
	return f.g, nil
}

func (f *requestFixture) Publish(_ context.Context, change codehost.ProposalChange) (codehost.ProposalGeneration, error) {
	if f.fault == "prepare unavailable" {
		return codehost.ProposalGeneration{}, errTaskInput
	}
	f.writes++
	f.g.Proposal.Sequence++
	f.g.Proposal.State, f.g.Proposal.TaskPreparation = change.State, change.TaskPreparation
	f.g.OID = fmt.Sprintf("%040x", f.g.Proposal.Sequence)
	return f.g, nil
}

func (f *requestFixture) Policy(context.Context, Binding) (Policy, error) {
	if f.fault == "policy unavailable" || (f.started && f.fault == "presentation policy unavailable") {
		return Policy{}, errTaskInput
	}
	return f.policy, nil
}

func (f *requestFixture) Snapshot(_ context.Context, b Binding) (Snapshot, error) {
	f.started = true
	if f.fault == "native unavailable" {
		return Snapshot{}, errTaskInput
	}
	s := f.snapshot
	s.Binding = b
	s.Review.Aggregate.ProposalID, s.Review.Aggregate.ReviewedRevision = b.Subject(), b.Head
	s.Review.Aggregate.Report.ProposalRef = b.Subject()
	refreshDigest(f.t, &s)
	if f.fault == "partial gate" {
		s.CodeGate.Outcomes = nil
	}
	if f.fault == "oversized gate" {
		s.CodeGate.Outcomes[0].Output = strings.Repeat("x", MaxSnapshotBytes)
	}
	return s, nil
}

func (f *requestFixture) Retain(_ context.Context, g codehost.ProposalGeneration, kind string, body []byte) (codehost.ReviewArtifact, error) {
	if (kind == codehost.TaskAggregate && f.fault == "aggregate publication unavailable") || (kind == codehost.TaskRequest && f.fault == "request publication unavailable") {
		return codehost.ReviewArtifact{}, errTaskInput
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	ref := "refs/pyxis/review-aggregates/" + g.Proposal.ID + "/" + digest
	if kind == codehost.TaskRequest {
		ref = "refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID
	}
	r := codehost.ReviewArtifact{Identity: codehost.ArtifactRef{Ref: ref, OID: digest[:40], Digest: "sha256:" + digest}, Kind: kind, Body: body, Trust: codehost.Trust{SharedAccount: true}}
	f.rows[ref] = r
	if kind == codehost.TaskAggregate && f.fault == "wrong evidence namespace" {
		r.Identity.Ref = "refs/pyxis/review-aggregates/foreign/" + digest
	}
	if kind == codehost.TaskRequest {
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
			request, err := codehost.ParseTaskRequest(body)
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

func (f *requestFixture) ReadRequest(_ context.Context, g codehost.ProposalGeneration) (codehost.ReviewArtifact, error) {
	if f.fault == "request read unavailable" || (f.retained && f.fault == "final request unavailable") {
		return codehost.ReviewArtifact{}, errTaskInput
	}
	r, ok := f.rows["refs/pyxis/review-requests/"+g.Proposal.ID+"/"+g.OID]
	if !ok {
		return r, codehost.ErrReviewArtifactNotFound
	}
	return r, nil
}

func (f *requestFixture) ReadRetained(_ context.Context, _ codehost.ProposalGeneration, kind string, identity codehost.ArtifactRef) (codehost.ReviewArtifact, error) {
	if (kind == codehost.TaskAggregate && f.fault == "aggregate read unavailable") || (kind == codehost.TaskRequest && f.fault == "final request unavailable") {
		return codehost.ReviewArtifact{}, errTaskInput
	}
	r := f.rows[identity.Ref]
	if kind == codehost.TaskAggregate && f.fault == "malformed aggregate" {
		r.Body = []byte("{")
	}
	return r, nil
}

func TestTaskRequestsRefuseUnavailableMovedAndIncompleteInputs(t *testing.T) {
	for _, fault := range []string{
		"scope unavailable", "proposal unavailable", "prepare unavailable", "policy unavailable", "native unavailable", "partial gate", "oversized gate",
		"aggregate publication unavailable", "request publication unavailable", "wrong evidence namespace", "malformed request", "different request generation", "aggregate read unavailable", "malformed aggregate",
		"presentation policy unavailable", "origin changed", "final proposal unavailable", "generation moved", "final request unavailable", "wrong request namespace",
	} {
		t.Run(fault, func(t *testing.T) {
			f := newRequestFixture(t)
			requests, err := NewTaskRequests(t.Context(), f, f, f)
			if err != nil {
				t.Fatal(err)
			}
			f.fault = fault
			packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
			if err == nil || packet.SchemaVersion != "" {
				t.Fatalf("failed input presented a review packet: %v", err)
			}
			if f.g.Proposal.TargetOID != f.snapshot.Binding.TargetOID {
				t.Fatal("review preparation changed its target")
			}
		})
	}
}

func TestTaskRequestObservationCannotRepairMissingOrUnavailableRequests(t *testing.T) {
	f := newRequestFixture(t)
	requests, err := NewTaskRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"", "scope unavailable", "proposal unavailable", "request read unavailable"} {
		f.fault = fault
		if _, err := requests.Observe(t.Context(), f.g.Proposal.ID); err == nil || f.writes != 0 {
			t.Fatalf("read-only observation prepared a request: %v", err)
		}
	}
	f.fault = ""
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.fault = "request read unavailable"
	if _, err := requests.Prepare(t.Context(), f.g.Proposal.ID); !errors.Is(err, errTaskInput) || f.writes != 1 {
		t.Fatal("unreadable existing request was replaced")
	}
	f.fault = ""
	resumed, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil || resumed.Binding != packet.Binding || f.writes != 1 {
		t.Fatalf("existing request was not resumed: %v", err)
	}
}

func TestTaskRequestPortsAndProposalPurposeAreBoundBeforeWrites(t *testing.T) {
	f := newRequestFixture(t)
	if _, err := NewTaskRequests(t.Context(), nil, f, f); !errors.Is(err, ErrIncomplete) {
		t.Fatal("missing proposal port accepted")
	}
	for _, fault := range []string{"scope unavailable", "final scope", "mainline task", "unknown scope"} {
		f := newRequestFixture(t)
		switch fault {
		case "scope unavailable":
			f.fault = fault
		case "final scope":
			f.scope.HeadRef = "refs/heads/feature/widget"
		case "mainline task":
			f.scope.TargetRef = f.scope.MainlineRef
		case "unknown scope":
			f.scope.SchemaVersion = "unknown"
		}
		if _, err := NewTaskRequests(t.Context(), f, f, f); err == nil || f.writes != 0 {
			t.Fatalf("unbound ports accepted: %s", fault)
		}
	}
	for _, purpose := range []string{"final", "terminal", "foreign target", "missing feature"} {
		f := newRequestFixture(t)
		requests, err := NewTaskRequests(t.Context(), f, f, f)
		if err != nil {
			t.Fatal(err)
		}
		switch purpose {
		case "final":
			f.g.Proposal.Task = ""
		case "terminal":
			f.g.Proposal.State = "abandoned"
		case "foreign target":
			f.g.Proposal.TargetRef = "refs/heads/foreign"
		case "missing feature":
			f.g.Proposal.Feature = ""
		}
		if _, err := requests.Prepare(t.Context(), f.g.Proposal.ID); err == nil {
			t.Fatalf("wrong task purpose accepted: %s", purpose)
		}
	}
}

func TestTaskReceiptValidationRefreshesCurrentPacketAndExactSavedIntent(t *testing.T) {
	f := newRequestFixture(t)
	requests, err := NewTaskRequests(t.Context(), f, f, f)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := requests.Prepare(t.Context(), f.g.Proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := requests.ValidateReview(t.Context(), packet.Binding); err != nil {
		t.Fatal(err)
	}
	wrong := packet.Binding
	wrong.Nonce = strings.Repeat("f", 32)
	if requests.ValidateReview(t.Context(), wrong) == nil {
		t.Fatal("unpresented nonce accepted by current policy verifier")
	}
	wrong.Operation = "unknown"
	if requests.ValidateReview(t.Context(), wrong) == nil {
		t.Fatal("unsupported approval operation accepted")
	}
	f.policy.Prompts = "changed-prompts"
	if requests.ValidateReview(t.Context(), packet.Binding) == nil {
		t.Fatal("obsolete review policy remained current")
	}
}
