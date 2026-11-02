package issueentry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
)

func TestResumeReconstructsOnlyTheExactAdmittedTask(t *testing.T) {
	input, snapshot := intake(t)
	key := "github:https://github.com:o/r"
	c, original, err := Prepare(input, "github", key, 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, same := range []Input{input, c.Input} {
		retained, task, err := Resume(same, c, key)
		if err != nil || retained.ID != c.ID || task.ID != original.ID || task.Brief != original.Brief || task.DemandID != original.DemandID {
			t.Fatalf("same contract changed on recovery: %+v %v", task, err)
		}
	}
	for _, change := range []string{"corrupt contract", "wrong binding", "wrong schema", "changed intake", "invalid charter"} {
		t.Run(change, func(t *testing.T) {
			i, admitted, binding := input, c, key
			switch change {
			case "corrupt contract":
				admitted.ID = "sha256:other"
			case "wrong binding":
				binding = "github:https://github.com:other/repo"
			case "wrong schema":
				i.Schema = "future/v9"
			case "changed intake":
				i.Goal = "Another goal"
			case "invalid charter":
				i.Goal = ""
			}
			if _, _, err := Resume(i, admitted, binding); err == nil {
				t.Fatal("recovery widened or lost admitted authority")
			}
		})
	}
}

func TestVerifiedAssociationRetainsAtomicProofAcrossFreshStore(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Admit(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	p := port.Proposal{Number: 77, Branch: "feat/issue"}
	sha := strings.Repeat("a", 40)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, err := range []error{
		store.AssociateVerified(ctx, c, p, sha),
		store.AssociateVerified(t.Context(), c, p, "invalid"),
		store.AssociateVerified(t.Context(), c, port.Proposal{Number: 77}, sha),
	} {
		if err == nil {
			t.Fatal("failed publication persisted an association without proof")
		}
	}
	if _, ok, err := fresh.Proposal(t.Context(), c); err != nil || ok {
		t.Fatal("failed CAS left a partial proposal association")
	}
	for range 2 {
		if err := store.AssociateVerified(t.Context(), c, p, sha); err != nil {
			t.Fatal(err)
		}
	}
	if known, ok, err := fresh.Proposal(t.Context(), c); err != nil || !ok || known != p {
		t.Fatalf("fresh proposal recovery: %+v %v", known, err)
	}
	if yes, err := fresh.LocalVerified(t.Context(), c, p, sha); err != nil || !yes {
		t.Fatalf("fresh local proof recovery: %v", err)
	}
	for _, mismatch := range []struct {
		proposal port.Proposal
		revision string
	}{{p, strings.Repeat("b", 40)}, {port.Proposal{Number: 77, Branch: "other"}, sha}} {
		if yes, err := fresh.LocalVerified(t.Context(), c, mismatch.proposal, mismatch.revision); err != nil || yes {
			t.Fatal("another head or branch borrowed the retained proof")
		}
	}
}

func TestIntakeRefusesMalformedSourceAndCharter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Input, *port.IssueSnapshot)
	}{
		{"unsupported intake", func(i *Input, _ *port.IssueSnapshot) { i.Schema = "future/v9" }},
		{"invalid blocker", func(_ *Input, s *port.IssueSnapshot) { s.Dependencies[0].Number = 0 }},
		{"duplicate blocker", func(_ *Input, s *port.IssueSnapshot) { s.Dependencies = append(s.Dependencies, s.Dependencies[0]) }},
		{"unidentified comment", func(_ *Input, s *port.IssueSnapshot) { s.Comments[0].ID = "" }},
		{"duplicate comment", func(_ *Input, s *port.IssueSnapshot) { s.Comments = append(s.Comments, s.Comments[0]) }},
		{"duplicate citation", func(i *Input, _ *port.IssueSnapshot) {
			i.Source.Citations = append(i.Source.Citations, i.Source.Citations[0])
		}},
		{"no goal", func(i *Input, _ *port.IssueSnapshot) { i.Goal = "" }},
		{"no acceptance", func(i *Input, _ *port.IssueSnapshot) { i.Acceptance = nil }},
		{"invalid task requirements", func(i *Input, _ *port.IssueSnapshot) { i.Requirements.Schema = "future/v9" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, snapshot := intake(t)
			tc.change(&input, &snapshot)
			if _, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot); err == nil {
				t.Fatal("admitted malformed authority/charter")
			}
		})
	}
	if _, err := Inspect("", "", port.IssueSnapshot{}); err == nil {
		t.Fatal("inspected unknown source")
	}
	input, snapshot := intake(t)
	c, task, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := taskspec.ParseTask(task.Brief, nil)
	if err != nil {
		t.Fatal(err)
	}
	marker, err := taskspec.RenderNative(spec)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Comments = append(snapshot.Comments, port.IssueComment{ID: "5", Body: marker})
	if matched, _, err := Prepare(c.Input, "github", task.SourceKey, 17, snapshot); err != nil || matched.ID != c.ID {
		t.Fatalf("same capsule lost identity: %v", err)
	}
	spec.PlanTaskID = "adhoc/other-task"
	marker, err = taskspec.RenderNative(spec)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Comments[len(snapshot.Comments)-1].Body = marker
	if _, _, err := Prepare(c.Input, "github", task.SourceKey, 17, snapshot); err == nil {
		t.Fatal("accepted valid but conflicting native capsule")
	}
	scoped := &ScopedTracker{Reader: &exactTracker{}, Contract: c, Task: task}
	if _, err := scoped.Frontier(t.Context()); err == nil {
		t.Fatal("lost exact-reader failure")
	}
	_, snapshot = intake(t)
	scoped.Reader = &exactTracker{snapshot: snapshot}
	scoped.Provider, scoped.Key = "github", task.SourceKey
	scoped.Contract.Input.Goal = "Changed charter"
	if _, err := scoped.Frontier(t.Context()); err == nil {
		t.Fatal("accepted changed contract bytes before dispatch")
	}
}

func TestGitIssueEvidenceRefusesCorruptionAndConflictingIdentity(t *testing.T) {
	store, fresh := stores(t)
	input, snapshot := intake(t)
	c, _, err := Prepare(input, "github", "github:https://github.com:o/r", 17, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore("", "origin"); err == nil {
		t.Fatal("accepted no evidence repository")
	}
	if _, ok, err := store.Load(t.Context(), "github", "other", 17); err != nil || ok {
		t.Fatal("invented an admission for another binding")
	}
	if _, _, err := store.Load(t.Context(), "", "", 0); err == nil {
		t.Fatal("accepted missing source identity")
	}
	if err := store.Admit(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	corrupt := c
	corrupt.ID = "sha256:changed"
	for _, err := range []error{
		store.Admit(t.Context(), corrupt),
		store.Associate(t.Context(), corrupt, port.Proposal{Number: 77, Branch: "feat/issue"}),
		store.RecordLocal(t.Context(), corrupt, port.Proposal{}, "bad"),
	} {
		if err == nil {
			t.Fatal("corrupt contract authorized durable evidence")
		}
	}
	if _, _, err := store.Proposal(t.Context(), corrupt); err == nil {
		t.Fatal("corrupt contract read proposal evidence")
	}
	if _, err := store.LocalVerified(t.Context(), c, port.Proposal{}, "bad"); err == nil {
		t.Fatal("malformed revision read local evidence")
	}
	p := port.Proposal{Number: 77, Branch: "feat/issue"}
	sha := strings.Repeat("a", 40)
	if err := store.RecordLocal(t.Context(), c, p, sha); err == nil {
		t.Fatal("unassociated proposal gained local evidence")
	}
	for _, bad := range []port.Proposal{{Number: 77}, {Branch: "feat/issue"}} {
		if err := store.Associate(t.Context(), c, bad); err == nil {
			t.Fatal("incomplete proposal association")
		}
	}
	if err := store.Associate(t.Context(), c, p); err != nil {
		t.Fatal(err)
	}
	if err := store.Associate(t.Context(), c, port.Proposal{Number: 78, Branch: p.Branch}); err == nil {
		t.Fatal("replaced proposal association")
	}
	if err := store.RecordLocal(t.Context(), c, port.Proposal{Number: 78, Branch: p.Branch}, sha); err == nil {
		t.Fatal("local evidence aliased another proposal")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := fresh.Load(ctx, "github", "github:https://github.com:o/r", 17); err == nil {
		t.Fatal("ignored evidence read cancellation")
	}
	// Simulate retained-wire corruption on the real shared CAS ref. Unknown
	// fields and wrong identities must be refused by a freshly fetching reader.
	contractWire, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte("broken"), append([]byte("{\"unknown\":0,"), contractWire[1:]...), []byte("{}")} {
		replaceEvidence(t, store, "issue-state/contracts/"+c.SourceID+".json", bad)
		if _, _, err := fresh.Load(t.Context(), "github", "github:https://github.com:o/r", 17); err == nil {
			t.Fatal("loaded corrupt contract")
		}
	}
	replaceEvidence(t, store, "issue-state/contracts/"+c.SourceID+".json", contractWire)
	path, err := contractPath(c, "proposals")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"broken", "{}", "{\"schema_version\":\"issue-proposal/v1\",\"contract_id\":\"wrong\",\"proposal_number\":77,\"branch\":\"feat/issue\"}"} {
		replaceEvidence(t, store, path+".json", []byte(bad))
		if _, _, err := fresh.Proposal(t.Context(), c); err == nil {
			t.Fatal("loaded corrupt proposal")
		}
	}
	wire, err := json.Marshal(Association{Schema: "issue-proposal/v1", ContractID: c.ID, ProposalNumber: p.Number, Branch: p.Branch})
	if err != nil {
		t.Fatal(err)
	}
	replaceEvidence(t, store, path+".json", wire)
	proof, err := proofPath(c, sha)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"broken", "{}"} {
		replaceEvidence(t, store, proof, []byte(bad))
		if _, err := fresh.LocalVerified(t.Context(), c, p, sha); err == nil {
			t.Fatal("loaded corrupt local proof")
		}
	}
}

func replaceEvidence(t *testing.T, s *Store, path string, bytes []byte) {
	t.Helper()
	if _, err := s.git.Update(t.Context(), path, "test: corrupt selected wire", func([]byte) ([]byte, bool, error) { return bytes, true, nil }); err != nil {
		t.Fatal(err)
	}
}
