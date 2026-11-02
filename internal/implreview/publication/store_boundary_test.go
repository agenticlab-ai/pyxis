package publication

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func boundaryRecord(stage Stage) Record {
	r := Record{
		Admission:            testAdmission(),
		PublicationAttemptID: "publication-boundary",
		IdempotencyID:        "idempotency-boundary",
		Stage:                stage,
	}
	switch stage {
	case StageArmed:
		r.CreateArmed = true
	case StageReceiptRecorded:
		r.CreateArmed = true
		r.NativeReceipt = "receipt-boundary"
	}
	return r
}

func boundaryFailedTerminal(r Record) implreview.PublicationTerminal {
	return implreview.PublicationTerminal{
		ReviewProcessID: r.Binding.ReviewProcessID, ReviewObligationID: r.Binding.ReviewObligationID,
		PublicationAttemptID: r.PublicationAttemptID, ProposalID: r.Binding.ProposalID,
		ReviewedRevision: r.Binding.ReviewedRevision, Medium: r.Medium, IdempotencyID: r.IdempotencyID,
		Status: implreview.PublicationFailed, FailureClass: implreview.FailureInfrastructure,
		Reason: "stopped", EVT13Ref: "evidence:evt13:boundary",
	}
}

func boundaryVerifiedTerminal(r Record) implreview.PublicationTerminal {
	t := boundaryFailedTerminal(r)
	t.Status = implreview.PublicationVerified
	t.FailureClass, t.Reason = "", ""
	t.ArtifactRef = "artifact-boundary"
	t.NativeReceipt = "receipt-boundary"
	t.AttemptProofRef = r.Binding.AttemptProofRef
	return t
}

func TestPublicationStateMethodsRejectInvalidLocalTransitions(t *testing.T) {
	store := &GitStore{}
	accepted := boundaryRecord(StageAccepted)
	armed := boundaryRecord(StageArmed)
	receipted := boundaryRecord(StageReceiptRecorded)

	if got, owner, err := store.Arm(t.Context(), armed); err != nil || owner || got.Stage != StageArmed {
		t.Fatalf("armed replay = %+v, %v, %v", got, owner, err)
	}
	invalid := accepted
	invalid.Stage = Stage("unknown")
	if _, _, err := store.Arm(t.Context(), invalid); err == nil {
		t.Fatal("invalid publication armed")
	}

	if _, err := store.RecordReceipt(t.Context(), armed, " "); err == nil {
		t.Fatal("empty receipt accepted")
	}
	if got, err := store.RecordReceipt(t.Context(), receipted, receipted.NativeReceipt); err != nil || got.NativeReceipt != receipted.NativeReceipt {
		t.Fatalf("receipt replay = %+v, %v", got, err)
	}
	if _, err := store.RecordReceipt(t.Context(), receipted, "different"); err == nil {
		t.Fatal("conflicting receipt replay accepted")
	}
	if _, err := store.RecordReceipt(t.Context(), accepted, "receipt"); err == nil {
		t.Fatal("receipt recorded before arming")
	}

	terminal := boundaryFailedTerminal(accepted)
	finished := accepted
	finished.Stage, finished.Terminal = StageTerminal, &terminal
	if got, err := store.Finish(t.Context(), finished, terminal); err != nil || got.Stage != StageTerminal {
		t.Fatalf("terminal replay = %+v, %v", got, err)
	}
	conflict := terminal
	conflict.Reason = "different"
	if _, err := store.Finish(t.Context(), finished, conflict); err == nil {
		t.Fatal("conflicting terminal replay accepted")
	}
	if _, err := store.Finish(t.Context(), invalid, terminal); err == nil {
		t.Fatal("terminal accepted from an unknown stage")
	}
	terminal.ProposalID = "wrong"
	if _, err := store.Finish(t.Context(), accepted, terminal); err == nil {
		t.Fatal("terminal identity mismatch accepted")
	}
}

func TestPublicationValidationCoversEveryClosedVariant(t *testing.T) {
	admission := testAdmission()
	admission.Binding.ProofEvidenceRef = admission.Binding.AttemptProofRef
	if err := validateAdmission(admission); err == nil {
		t.Fatal("aliased proof references accepted")
	}
	if err := validateAdmission(Admission{}); err == nil {
		t.Fatal("empty admission accepted")
	}

	invalidRecords := []Record{
		func() Record { r := boundaryRecord(StageAccepted); r.CreateArmed = true; return r }(),
		func() Record { r := boundaryRecord(StageArmed); r.CreateArmed = false; return r }(),
		func() Record { r := boundaryRecord(StageReceiptRecorded); r.NativeReceipt = ""; return r }(),
		func() Record { r := boundaryRecord(StageTerminal); return r }(),
		func() Record { r := boundaryRecord(Stage("unknown")); return r }(),
	}
	for i, record := range invalidRecords {
		if err := validateRecord(record); err == nil {
			t.Fatalf("invalid record %d accepted: %+v", i, record)
		}
	}

	record := boundaryRecord(StageReceiptRecorded)
	verified := boundaryVerifiedTerminal(record)
	verified.ArtifactRef = ""
	if err := validateTerminal(record, verified); err == nil {
		t.Fatal("incomplete verified terminal accepted")
	}
	failed := boundaryFailedTerminal(record)
	failed.Reason = ""
	if err := validateTerminal(record, failed); err == nil {
		t.Fatal("incomplete failed terminal accepted")
	}
	unknown := boundaryFailedTerminal(record)
	unknown.Status = implreview.PublicationStatus("unknown")
	if err := validateTerminal(record, unknown); err == nil {
		t.Fatal("unknown terminal status accepted")
	}

	for stage, want := range map[Stage]int{StageTerminal: 3, StageReceiptRecorded: 2, StageArmed: 1, StageAccepted: 0} {
		if got := recoveryPriority(stage); got != want {
			t.Fatalf("recoveryPriority(%q) = %d, want %d", stage, got, want)
		}
	}
	if !validSuccessor(record, record) {
		t.Fatal("identical state was not a valid replay")
	}
}

func TestReduceRejectsMalformedAndConflictingRows(t *testing.T) {
	valid := boundaryRecord(StageAccepted)
	row := func(schema string, record Record) []byte {
		b, err := json.Marshal(stateRow{SchemaVersion: schema, Record: record})
		if err != nil {
			t.Fatal(err)
		}
		return append(b, '\n')
	}

	unknownSchema := row("future", valid)
	invalidRecord := valid
	invalidRecord.IdempotencyID = ""
	identityConflict := valid
	identityConflict.Admission.HandoffDigest = "different"
	invalidSuccessor := boundaryRecord(StageReceiptRecorded)

	cases := [][]byte{
		[]byte("{"),
		unknownSchema,
		row(rowSchema, invalidRecord),
		append(row(rowSchema, valid), row(rowSchema, identityConflict)...),
		append(row(rowSchema, valid), row(rowSchema, invalidSuccessor)...),
	}
	for i, data := range cases {
		if _, err := reduce(data); err == nil {
			t.Fatalf("malformed history %d reduced clean: %s", i, strings.TrimSpace(string(data)))
		}
	}
}

func TestPublicationReadBoundariesRejectEmptySubjects(t *testing.T) {
	store := &GitStore{}
	if _, _, err := store.Record(t.Context(), " "); err == nil {
		t.Fatal("empty publication attempt read accepted")
	}
	if _, _, err := store.Recover(t.Context(), implreview.ReviewSubject{}); err == nil {
		t.Fatal("empty recovery subject accepted")
	}
	if _, err := NewGitStore("", "", ""); err == nil {
		t.Fatal("empty git store configuration accepted")
	}
}
