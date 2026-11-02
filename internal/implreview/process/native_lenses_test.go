package process

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func nativeLensFixtureProof() implreview.NativeLensEvidence {
	return implreview.NativeLensEvidence{SchemaVersion: implreview.NativeLensEvidenceSchema,
		PackDigest: "sha256:" + strings.Repeat("a", 64), EvidenceDigest: "sha256:" + strings.Repeat("b", 64), AggregateDigest: "sha256:" + strings.Repeat("c", 64)}
}

func TestNativeLensEvidenceSurvivesRestartAndRejectsLegacyCompletion(t *testing.T) {
	repo := processGitFixture(t)
	s, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := testCoordinator(t)
	c.store = s
	subject := implreview.ReviewSubject{ReviewObligationID: "native-obligation", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
	admitted, err := c.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	proof := nativeLensFixtureProof()
	for range 2 {
		if err := c.RecordNativeLensEvidence(t.Context(), admitted, proof); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.VerifyNativeLensEvidence(t.Context(), admitted); err == nil {
		t.Fatal("uncompleted process accepted")
	}
	other := proof
	other.EvidenceDigest = "sha256:" + strings.Repeat("d", 64)
	if err := c.RecordNativeLensEvidence(t.Context(), admitted, other); err == nil {
		t.Fatal("conflicting proof replaced the original")
	}
	if err := s.Finish(t.Context(), admitted, implreview.HandoffCompleted, proof.AggregateDigest); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.VerifyNativeLensEvidence(t.Context(), admitted); err != nil {
		t.Fatal(err)
	}
	if err := restarted.RecordNativeLensEvidence(t.Context(), admitted, proof); err == nil {
		t.Fatal("completed history acquired native evidence retrospectively")
	}
	wrong := admitted
	wrong.Subject.ReviewedRevision = "different-head"
	if err := restarted.VerifyNativeLensEvidence(t.Context(), wrong); err == nil {
		t.Fatal("proof applied to another revision")
	}
	legacy, err := s.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(t.Context(), legacy, implreview.HandoffCompleted, proof.AggregateDigest); err != nil {
		t.Fatal(err)
	}
	if err := restarted.VerifyNativeLensEvidence(t.Context(), legacy); err == nil {
		t.Fatal("legacy parent approval became a completed native panel")
	}
	unknown := legacy
	unknown.ReviewProcessID = "unknown-process"
	if err := restarted.RecordNativeLensEvidence(t.Context(), unknown, proof); err == nil {
		t.Fatal("unknown process acquired a proof")
	}
	if err := restarted.VerifyNativeLensEvidence(t.Context(), implreview.AdmittedProcess{}); err == nil {
		t.Fatal("empty subject accepted")
	}
}

func TestNativeLensEvidenceBindsExactAggregateAndFailsClosedOnCorruption(t *testing.T) {
	repo := processGitFixture(t)
	s, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: "native-obligation", ProposalID: "github:o/r#8", ReviewedRevision: "abc123"}
	a, err := s.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	proof := nativeLensFixtureProof()
	if err := s.RecordNativeLensEvidence(t.Context(), a, proof); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(t.Context(), a, implreview.HandoffCompleted, "sha256:"+strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyNativeLensEvidence(t.Context(), a); err == nil {
		t.Fatal("native proof accepted a different aggregate")
	}
	if _, err := s.log.Update(t.Context(), nativeLensPath, "fixture: corrupt proof", func([]byte) ([]byte, bool, error) {
		return []byte("not JSON"), true, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyNativeLensEvidence(t.Context(), a); err == nil {
		t.Fatal("corrupt native proof accepted")
	}
	b, err := s.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	defer s.release(b.ReviewProcessID)
	if err := s.RecordNativeLensEvidence(t.Context(), b, proof); err == nil {
		t.Fatal("recording ignored corrupt history")
	}
}

func TestNativeLensProofClosedSchemaAndUnavailableStore(t *testing.T) {
	a := implreview.AdmittedProcess{ReviewProcessID: "process", Subject: implreview.ReviewSubject{ReviewObligationID: "obligation", ProposalID: "github:o/r#1", ReviewedRevision: "head"}}
	proof := nativeLensFixtureProof()
	data, err := json.Marshal(nativeLensRow{a, proof})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"not JSON", strings.Replace(string(data), `"process":`, `"unknown":true,"process":`, 1),
		strings.Replace(string(data), implreview.NativeLensEvidenceSchema, "unknown-version", 1),
		strings.Replace(string(data), proof.PackDigest, "sha256:not-hex", 1),
		strings.Replace(string(data), `"review_process_id":"process"`, `"review_process_id":""`, 1),
	} {
		if _, err := parseNativeLensRows([]byte(bad)); err == nil {
			t.Fatalf("invalid proof accepted: %s", bad)
		}
	}
	c, _ := testCoordinator(t)
	if err := c.RecordNativeLensEvidence(t.Context(), a, proof); err == nil {
		t.Fatal("missing protected writer accepted")
	}
	if err := c.VerifyNativeLensEvidence(t.Context(), a); err == nil {
		t.Fatal("missing protected reader accepted")
	}
}
