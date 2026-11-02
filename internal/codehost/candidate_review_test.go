package codehost

import (
	"bytes"
	"strings"
	"testing"
)

func candidateRequestFixture() (CandidateReviewRequest, ProposalGeneration) {
	c := FinalCandidate{"widget", strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40), strings.Repeat("d", 40), "merge-v1", "builder-v1"}
	p := Proposal{SchemaVersion: ProposalSchema, ID: "final-widget", Kind: "implementation", Feature: c.FeatureID, State: "prepared", Sequence: 2,
		Branch: "refs/heads/final/widget", Head: c.FeatureHead, TargetRef: "refs/heads/trunk", TargetOID: c.MainHead, SourceRef: "refs/heads/feature/widget",
		CandidatePreparation: &CandidatePreparation{CandidatePreparationSchema, FeatureFinalMerge, strings.Repeat("1", 32), c, "required-v1"}}
	g := ProposalGeneration{OID: strings.Repeat("e", 40), Proposal: p}
	digest := "sha256:" + strings.Repeat("2", 64)
	r := CandidateReviewRequest{CandidateRequestSchema, ReviewBinding{SchemaVersion: ReviewBindingSchema, ProposalID: p.ID, Kind: p.Kind, ProposalOID: g.OID, Sequence: p.Sequence,
		Operation: FeatureFinalMerge, Branch: p.Branch, Head: p.Head, TargetRef: p.TargetRef, TargetOID: p.TargetOID, Digest: digest, Nonce: p.CandidatePreparation.Nonce,
		Evidence: ArtifactRef{"refs/pyxis/candidate-evidence/" + p.ID + "/" + strings.TrimPrefix(digest, "sha256:"), strings.Repeat("f", 40), digest}, Candidate: c, ScenarioSetFingerprint: "required-v1"}}
	return r, g
}

func TestCandidateRequestPinsWholeIdentityAndPreparedGeneration(t *testing.T) {
	r, g := candidateRequestFixture()
	body, err := r.Canonical()
	if err != nil || r.Matches(g) != nil {
		t.Fatalf("valid candidate request: %v", err)
	}
	parsed, err := ParseCandidateRequest(body)
	if err != nil || parsed != r {
		t.Fatalf("canonical candidate round trip: %v", err)
	}
	if r.Binding.Validate() == nil {
		t.Fatal("receipt accepted a request without its retained identity")
	}
	r.Binding.Request = ArtifactRef{"refs/pyxis/review-requests/" + g.Proposal.ID + "/" + g.OID, strings.Repeat("9", 40), "sha256:" + strings.Repeat("8", 64)}
	if r.Binding.Validate() != nil {
		t.Fatal("complete final receipt binding rejected")
	}
	if _, err := r.Canonical(); err == nil {
		t.Fatal("request included its own later identity")
	}
}

func TestCandidateRequestRejectsChangedIdentityAndOpenProtocol(t *testing.T) {
	for _, fault := range []string{"schema", "binding schema", "purpose", "kind", "proposal", "sequence", "head", "main", "candidate", "tree", "scenario set", "nonce", "namespace", "short evidence", "digest", "alias", "control", "open", "task", "missing preparation", "missing source"} {
		t.Run(fault, func(t *testing.T) {
			r, g := candidateRequestFixture()
			switch fault {
			case "schema":
				r.SchemaVersion = "unknown"
			case "binding schema":
				r.Binding.SchemaVersion = "unknown"
			case "purpose":
				r.Binding.Operation = TaskMerge
			case "kind":
				r.Binding.Kind = "draft"
			case "proposal":
				r.Binding.ProposalOID = strings.Repeat("9", 40)
			case "sequence":
				r.Binding.Sequence++
			case "head":
				r.Binding.Head = strings.Repeat("9", 40)
			case "main":
				r.Binding.TargetOID = strings.Repeat("9", 40)
			case "candidate":
				r.Binding.Candidate.Commit = strings.Repeat("9", 40)
			case "tree":
				r.Binding.Candidate.Tree = strings.Repeat("9", 40)
			case "scenario set":
				r.Binding.ScenarioSetFingerprint = "different"
			case "nonce":
				r.Binding.Nonce = "invalid"
			case "namespace":
				r.Binding.Evidence.Ref = "refs/pyxis/candidate-evidence/foreign/" + strings.Repeat("2", 64)
			case "short evidence":
				r.Binding.Evidence.OID = "short"
			case "digest":
				r.Binding.Digest = "sha256:" + strings.Repeat("9", 64)
			case "alias":
				r.Binding.Branch = r.Binding.TargetRef
			case "control":
				r.Binding.ProposalID += "\n"
			case "open":
				g.Proposal.State = "open"
			case "task":
				g.Proposal.Task = "T1"
			case "missing preparation":
				g.Proposal.CandidatePreparation = nil
			case "missing source":
				g.Proposal.SourceRef = ""
			}
			if r.Matches(g) == nil {
				t.Fatal("changed candidate request accepted")
			}
		})
	}
	r, _ := candidateRequestFixture()
	body, err := r.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte("{"), append(append([]byte(nil), body...), body...), append([]byte(" "), body...), bytes.Replace(body, []byte(`"schema_version":`), []byte(`"unknown":1,"schema_version":`), 1), bytes.Replace(body, []byte(`"schema_version":`), []byte(`"schema_version":"duplicate","schema_version":`), 1), bytes.Repeat([]byte("x"), MaxReviewArtifactBytes+1)} {
		if _, err := ParseCandidateRequest(bad); err == nil {
			t.Fatal("open or noncanonical candidate request accepted")
		}
	}
	task, _ := taskRequestFixture()
	task.Binding.Candidate = r.Binding.Candidate
	if task.Validate() == nil {
		t.Fatal("task intent accepted final candidate metadata")
	}
	if _, err := ParseTaskRequest(body); err == nil {
		t.Fatal("final request accepted by task decoder")
	}
}
