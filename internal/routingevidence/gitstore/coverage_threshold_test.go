package gitstore

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/routingevidence"
)

func TestAdapterAndPreparationBoundaryRefusals(t *testing.T) {
	if _, err := New("", "", routingevidence.ProducerPolicy{}); err == nil {
		t.Fatal("New accepted an invalid producer policy")
	}
	fx := newFixture(t, false)
	if _, err := New("", "origin", fx.policy); err == nil {
		t.Fatal("New accepted an empty repository")
	}
	adapter := fx.adapter(t, fx.first)
	if _, err := adapter.Append(t.Context(), routingevidence.AppendRequest{}); err == nil {
		t.Fatal("Append accepted an incomplete request")
	}

	observed := fx.genesisHead()
	if _, _, _, err := adapter.prepare(routingevidence.AppendRequest{ObservedHead: observed}); err == nil {
		t.Fatal("prepare accepted neither event nor composite")
	}
	empty := routingevidence.Event{}
	if _, _, _, err := adapter.prepare(routingevidence.AppendRequest{ObservedHead: observed, Event: &empty}); err == nil {
		t.Fatal("prepare accepted a non-canonical event")
	}
	event := fx.demand("demand-coverage", "task-coverage")
	if _, _, _, err := adapter.prepare(routingevidence.AppendRequest{
		ObservedHead: observed,
		Composite:    &routingevidence.AtomicComposite{Events: []routingevidence.Event{event}},
	}); err == nil {
		t.Fatal("prepare accepted a one-event composite")
	}
	if _, _, _, err := adapter.prepare(routingevidence.AppendRequest{
		ObservedHead: observed,
		Composite:    &routingevidence.AtomicComposite{Events: []routingevidence.Event{event, event}},
	}); err == nil {
		t.Fatal("prepare accepted duplicate immutable paths")
	}
}

func TestAdapterReadAndSnapshotBoundaryRefusals(t *testing.T) {
	fx := newFixture(t, false)
	raw, err := gitrefstore.New(t.TempDir(), "origin", fx.policy.EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	bad := &Adapter{git: raw, policy: fx.policy, reducer: routingevidence.NewReducer(fx.policy)}
	if _, err := bad.ReadSubject(t.Context(), routingevidence.SubjectID{Kind: routingevidence.SubjectDemand, ID: "demand"}); err == nil {
		t.Fatal("ReadSubject hid a git fetch failure")
	}
	if _, err := bad.ReadHead(t.Context()); err == nil {
		t.Fatal("ReadHead hid a git fetch failure")
	}
	if _, err := bad.ReadProof(t.Context(), routingevidence.EvidenceProof{}); err == nil {
		t.Fatal("ReadProof accepted an incomplete proof")
	}

	adapter := fx.adapter(t, fx.first)
	eventForProof := canonicalCoverageEvent(t, fx.demand("demand-read-proof", "task-read-proof"))
	eventPath, err := routingevidence.EventPath(eventForProof.EventID)
	if err != nil {
		t.Fatal(err)
	}
	shapedProof := routingevidence.EvidenceProof{EvidenceRef: fx.policy.EvidenceRef, Commit: "absent", Entries: []routingevidence.EvidenceEntry{{Path: eventPath, EventID: eventForProof.EventID, ContentDigest: eventForProof.ContentDigest}}}
	if _, err := adapter.ReadProof(t.Context(), shapedProof); err == nil {
		t.Fatal("ReadProof accepted a commit outside current history")
	}
	if _, err := adapter.ReadSubject(t.Context(), routingevidence.SubjectID{}); err == nil {
		t.Fatal("ReadSubject accepted an invalid subject")
	}
	staleEvent := fx.demand("demand-stale", "task-stale")
	if _, err := adapter.Append(t.Context(), routingevidence.AppendRequest{ObservedHead: routingevidence.ObservedHead{EvidenceRef: fx.policy.EvidenceRef, Commit: "absent"}, Event: &staleEvent}); err == nil {
		t.Fatal("Append accepted a stale observed head")
	}
	if _, _, err := adapter.validateEventFiles(gitrefstore.Snapshot{Files: map[string][]byte{"foreign": []byte("x")}}); err == nil {
		t.Fatal("validateEventFiles accepted a foreign path")
	}
	if _, _, err := adapter.validateEventFiles(gitrefstore.Snapshot{Files: map[string][]byte{"events/bad": []byte("not json")}}); err == nil {
		t.Fatal("validateEventFiles accepted malformed event bytes")
	}
	event := canonicalCoverageEvent(t, fx.demand("demand-path", "task-path"))
	wire, err := routingevidence.EncodeCanonical(event)
	if err != nil {
		t.Fatal(err)
	}
	path, err := routingevidence.EventPath(event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	last := "0"
	if strings.HasSuffix(path, last) {
		last = "1"
	}
	wrongPath := path[:len(path)-1] + last
	if _, _, err := adapter.validateEventFiles(gitrefstore.Snapshot{Files: map[string][]byte{wrongPath: wire}}); err == nil {
		t.Fatal("validateEventFiles accepted an event under another identity")
	}
}

func TestProofHelperBoundaryRefusals(t *testing.T) {
	fx := newFixture(t, false)
	event := canonicalCoverageEvent(t, fx.demand("demand-proof", "task-proof"))
	path, err := routingevidence.EventPath(event.EventID)
	if err != nil {
		t.Fatal(err)
	}
	proof := routingevidence.EvidenceProof{
		EvidenceRef: fx.policy.EvidenceRef,
		Commit:      fx.genesis.Commit,
		Entries: []routingevidence.EvidenceEntry{{
			Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest,
		}},
	}
	if err := validateLease(routingevidence.ObservedHead{EvidenceRef: fx.policy.EvidenceRef, Commit: "absent"}, gitrefstore.Snapshot{Ref: fx.policy.EvidenceRef}); err == nil {
		t.Fatal("validateLease accepted an object outside history")
	}
	if _, err := proofForEvents("head", fx.policy.EvidenceRef, []routingevidence.Event{{}}); err == nil {
		t.Fatal("proofForEvents accepted an invalid event identity")
	}
	if err := validateProofShape(routingevidence.EvidenceProof{}, fx.policy.EvidenceRef); err == nil {
		t.Fatal("validateProofShape accepted an incomplete proof")
	}
	if err := verifyProofBytes(routingevidence.EvidenceProof{}, nil); err == nil {
		t.Fatal("verifyProofBytes accepted an incomplete proof")
	}
	badEntry := proof
	badEntry.Entries = append([]routingevidence.EvidenceEntry(nil), proof.Entries...)
	badEntry.Entries[0].ContentDigest = ""
	if err := validateProofShape(badEntry, fx.policy.EvidenceRef); err == nil {
		t.Fatal("validateProofShape accepted an incomplete entry")
	}
	if err := verifyProofBytes(proof, map[string][]byte{}); err == nil {
		t.Fatal("verifyProofBytes accepted the wrong path count")
	}
	if err := verifyProofBytes(proof, map[string][]byte{"events/other": []byte("x")}); err == nil {
		t.Fatal("verifyProofBytes accepted an absent proof path")
	}
	if err := verifyProofBytes(proof, map[string][]byte{path: []byte("not json")}); err == nil {
		t.Fatal("verifyProofBytes accepted changed event bytes")
	}
	if !containsCommit([]gitrefstore.Commit{{Object: fx.genesis.Commit}}, fx.genesis.Commit) {
		t.Fatal("containsCommit missed the matching object")
	}
}

func canonicalCoverageEvent(t *testing.T, event routingevidence.Event) routingevidence.Event {
	t.Helper()
	wire, err := routingevidence.EncodeCanonical(event)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := routingevidence.DecodeCanonical(wire)
	if err != nil {
		t.Fatal(err)
	}
	return stored
}
