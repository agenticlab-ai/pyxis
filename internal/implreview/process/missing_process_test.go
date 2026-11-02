package process

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
)

func TestMissingProcessesRequiresExactCanonicalMissingTerminal(t *testing.T) {
	repo := processGitFixture(t)
	store, err := NewGitStore(repo, "origin", DefaultEvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
	first, err := store.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.MissingProcesses(t.Context(), subject); err != nil || len(got) != 0 {
		t.Fatalf("live process=%+v err=%v", got, err)
	}
	if err := store.Finish(t.Context(), first, implreview.HandoffMissing, diagnosticDigest(implreview.HandoffMissing, "host invocation ended before process handoff")); err != nil {
		t.Fatal(err)
	}
	completed, err := store.Prepare(t.Context(), subject)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(t.Context(), completed, implreview.HandoffCompleted, diagnosticDigest(implreview.HandoffCompleted, "fixture")); err != nil {
		t.Fatal(err)
	}
	other := subject
	other.ReviewedRevision = "different"
	unrelated, err := store.Prepare(t.Context(), other)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(t.Context(), unrelated, implreview.HandoffMissing, diagnosticDigest(implreview.HandoffMissing, "fixture")); err != nil {
		t.Fatal(err)
	}
	coordinator := &Coordinator{store: store}
	got, err := coordinator.MissingProcesses(t.Context(), subject)
	if err != nil || len(got) != 1 || got[0] != first {
		t.Fatalf("missing=%+v err=%v", got, err)
	}
	if _, err := store.MissingProcesses(t.Context(), implreview.ReviewSubject{}); err == nil {
		t.Fatal("invalid subject accepted")
	}
	if _, err := coordinator.MissingProcesses(t.Context(), implreview.ReviewSubject{}); err == nil {
		t.Fatal("coordinator accepted invalid subject")
	}
	unsupported := &Coordinator{store: &memoryProcessStore{}}
	if got, err := unsupported.MissingProcesses(t.Context(), subject); err != nil || len(got) != 0 {
		t.Fatalf("unsupported evidence=%+v err=%v", got, err)
	}
	processGit(t, repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "unavailable.git"))
	if _, err := coordinator.MissingProcesses(t.Context(), subject); err == nil {
		t.Fatal("unavailable canonical evidence accepted")
	}
}

func TestMissingProcessesRejectsConflictingOrMalformedEvidence(t *testing.T) {
	tests := []string{"conflicting admission", "unadmitted terminal", "mismatched terminal", "conflicting terminal", "malformed row", "duplicate exact rows"}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			store, err := NewGitStore(processGitFixture(t), "origin", DefaultEvidenceRef)
			if err != nil {
				t.Fatal(err)
			}
			subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
			admitted, err := store.Prepare(t.Context(), subject)
			if err != nil {
				t.Fatal(err)
			}
			defer store.release(admitted.ReviewProcessID)
			row := processRow{SchemaVersion: processRowSchema, Kind: processTerminal, ReviewProcessID: admitted.ReviewProcessID, Subject: subject,
				Status: implreview.HandoffMissing, DiagnosticDigest: diagnosticDigest(implreview.HandoffMissing, "fixture")}
			switch name {
			case "conflicting admission":
				row.Kind, row.Status, row.DiagnosticDigest = processAdmission, "", ""
				row.Subject.ReviewedRevision = "other"
			case "unadmitted terminal":
				row.ReviewProcessID = "unknown"
			case "mismatched terminal":
				row.Subject.ReviewedRevision = "other"
			case "conflicting terminal":
				if err := store.Finish(t.Context(), admitted, implreview.HandoffMissing, row.DiagnosticDigest); err != nil {
					t.Fatal(err)
				}
				row.Status = implreview.HandoffMalformed
			case "duplicate exact rows":
				if err := store.Finish(t.Context(), admitted, implreview.HandoffMissing, row.DiagnosticDigest); err != nil {
					t.Fatal(err)
				}
			}
			_, err = store.log.Update(t.Context(), processStatePath, "test: inconsistent process evidence", func(data []byte) ([]byte, bool, error) {
				if name == "malformed row" {
					return []byte("{}\n"), true, nil
				}
				if name == "duplicate exact rows" {
					admission := row
					admission.Kind, admission.Status, admission.DiagnosticDigest = processAdmission, "", ""
					var err error
					data, _, err = appendRow(data, admission)
					if err != nil {
						return nil, false, err
					}
				}
				return appendRow(data, row)
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := store.MissingProcesses(context.Background(), subject)
			if name == "duplicate exact rows" {
				if err != nil || len(got) != 1 || got[0] != admitted {
					t.Fatalf("deduplicated=%+v err=%v", got, err)
				}
			} else if err == nil {
				t.Fatalf("unsafe evidence accepted: %+v", got)
			}
		})
	}
}
