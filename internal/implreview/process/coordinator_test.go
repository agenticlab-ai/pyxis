package process

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

type memoryProcessStore struct {
	admitted  implreview.AdmittedProcess
	status    implreview.HandoffStatus
	digest    string
	finishErr error
}

func (*memoryProcessStore) Preflight(context.Context) error { return nil }

func (s *memoryProcessStore) Prepare(_ context.Context, subject implreview.ReviewSubject) (implreview.AdmittedProcess, error) {
	if s.admitted.ReviewProcessID == "" {
		s.admitted = implreview.AdmittedProcess{ReviewProcessID: "process-1", Subject: subject}
	}
	return s.admitted, nil
}

func (s *memoryProcessStore) Finish(_ context.Context, admitted implreview.AdmittedProcess, status implreview.HandoffStatus, digest string) error {
	if s.finishErr != nil {
		return s.finishErr
	}
	s.admitted, s.status, s.digest = admitted, status, digest
	return nil
}

func testCoordinator(t *testing.T) (*Coordinator, *memoryProcessStore) {
	t.Helper()
	table, err := calibration.Parse(strings.NewReader("| Row | Lens | Default severity | Escalation condition |\n|---|---|---|---|\n| CAL-1 | spec-fidelity | blocking | test |\n"))
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryProcessStore{}
	coordinator, err := New(store, table)
	if err != nil {
		t.Fatal(err)
	}
	return coordinator, store
}

func testArtifact(t *testing.T, processID, proposal, revision string) string {
	t.Helper()
	body, err := persist.ComposeArtifact(&implreport.Report{SchemaVersion: implreport.SchemaVersion, ProposalRef: proposal,
		ReviewID: processID, Band: implreport.BandStandard, Verdict: implreport.VerdictApprove,
		Rounds: []implreport.Round{{ReviewedRevision: revision, ContextID: "context-1", Findings: []implreport.Finding{}}}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCoordinatorAcceptsOnlyExactAdmittedAggregate(t *testing.T) {
	coordinator, store := testCoordinator(t)
	subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
	admitted, err := coordinator.Prepare(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := coordinator.Complete(context.Background(), admitted, implreview.ProcessHandoff{
		Status: implreview.HandoffCompleted, ArtifactBody: testArtifact(t, admitted.ReviewProcessID, subject.ProposalID, subject.ReviewedRevision),
	})
	if err != nil {
		t.Fatal(err)
	}
	if handoff.Status != implreview.HandoffCompleted || handoff.Aggregate == nil || handoff.Aggregate.ProcessID != admitted.ReviewProcessID || handoff.DiagnosticDigest == "" {
		t.Fatalf("aggregate handoff = %+v", handoff)
	}
	if store.status != implreview.HandoffCompleted || store.digest != handoff.DiagnosticDigest {
		t.Fatalf("durable terminal = %q %q", store.status, store.digest)
	}
}

func TestCoordinatorChecksOnlyCurrentRoundAgainstNativeLaunch(t *testing.T) {
	for _, tc := range []struct {
		name, expected, reported string
		want                     implreview.HandoffStatus
	}{
		{"exact", "gpt-5.6-sol", "gpt-5.6-sol", implreview.HandoffCompleted},
		{"family alias", "gpt-5.6-sol", "gpt-5", implreview.HandoffMismatched},
		{"missing provenance", "gpt-5.6-sol", "", implreview.HandoffMismatched},
		{"empty host expectation", "", "gpt-5.6-sol", implreview.HandoffMismatched},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coordinator, store := testCoordinator(t)
			subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
			ctx := implreview.WithNativeModel(context.Background(), tc.expected)
			admitted, err := coordinator.Prepare(ctx, subject)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := persist.ParseHandoff(testArtifact(t, admitted.ReviewProcessID, subject.ProposalID, subject.ReviewedRevision))
			if err != nil {
				t.Fatal(err)
			}
			prior := parsed.Rounds[0]
			prior.ContextID = "earlier-context"
			prior.Provenance.Model = "earlier-model"
			parsed.Rounds[0].Provenance.Model = tc.reported
			parsed.Rounds = append([]implreport.Round{prior}, parsed.Rounds...)
			body, err := persist.ComposeArtifact(parsed)
			if err != nil {
				t.Fatal(err)
			}
			got, err := coordinator.Complete(ctx, admitted, implreview.ProcessHandoff{Status: implreview.HandoffCompleted, ArtifactBody: body})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.want || store.status != tc.want || store.digest != got.DiagnosticDigest {
				t.Fatalf("handoff=%+v durable=%q, want %q", got, store.status, tc.want)
			}
			if tc.want != implreview.HandoffCompleted {
				if got.Aggregate != nil {
					t.Fatal("mismatched model produced a canonical aggregate")
				}
				return
			}
			if got.Aggregate.Report.Rounds[0].Provenance.Model != "earlier-model" || got.Aggregate.Report.Rounds[1].Provenance.Model != tc.reported {
				t.Fatal("coordinator rewrote historical or current model provenance")
			}
		})
	}
}

func TestCoordinatorClassifiesMissingSurplusMalformedAndMismatched(t *testing.T) {
	cases := []struct {
		name string
		body func(*testing.T, implreview.AdmittedProcess, implreview.ReviewSubject) string
		want implreview.HandoffStatus
	}{
		{name: "missing", body: func(*testing.T, implreview.AdmittedProcess, implreview.ReviewSubject) string { return "" }, want: implreview.HandoffMissing},
		{name: "malformed", body: func(*testing.T, implreview.AdmittedProcess, implreview.ReviewSubject) string {
			return "not an artifact"
		}, want: implreview.HandoffMalformed},
		{name: "surplus", body: func(t *testing.T, p implreview.AdmittedProcess, s implreview.ReviewSubject) string {
			body := testArtifact(t, p.ReviewProcessID, s.ProposalID, s.ReviewedRevision)
			return body + "\n" + body
		}, want: implreview.HandoffSurplus},
		{name: "process mismatch", body: func(t *testing.T, _ implreview.AdmittedProcess, s implreview.ReviewSubject) string {
			return testArtifact(t, "worker-forged-process", s.ProposalID, s.ReviewedRevision)
		}, want: implreview.HandoffMismatched},
		{name: "revision mismatch", body: func(t *testing.T, p implreview.AdmittedProcess, s implreview.ReviewSubject) string {
			return testArtifact(t, p.ReviewProcessID, s.ProposalID, "moving-head")
		}, want: implreview.HandoffMismatched},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			coordinator, store := testCoordinator(t)
			subject := implreview.ReviewSubject{ReviewObligationID: "obligation-1", ProposalID: "github:o/r#7", ReviewedRevision: "abc123"}
			admitted, err := coordinator.Prepare(context.Background(), subject)
			if err != nil {
				t.Fatal(err)
			}
			status := implreview.HandoffCompleted
			body := tc.body(t, admitted, subject)
			if body == "" {
				status = implreview.HandoffMissing
			}
			got, err := coordinator.Complete(context.Background(), admitted, implreview.ProcessHandoff{Status: status, ArtifactBody: body})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.want || got.Aggregate != nil || got.DiagnosticDigest == "" || store.status != tc.want {
				t.Fatalf("handoff = %+v durable=%q, want %q", got, store.status, tc.want)
			}
		})
	}
}

func TestCoordinatorRejectsInvalidConstructionAndHandoffs(t *testing.T) {
	coordinator, store := testCoordinator(t)
	if _, err := New(nil, coordinator.calibration); err == nil {
		t.Fatal("New accepted a nil store")
	}
	if _, err := New(store, nil); err == nil {
		t.Fatal("New accepted a nil calibration table")
	}
	if err := coordinator.Preflight(context.Background()); err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if _, err := coordinator.Prepare(context.Background(), implreview.ReviewSubject{}); err == nil {
		t.Fatal("Prepare accepted an empty subject")
	}
	if _, err := coordinator.Complete(context.Background(), implreview.AdmittedProcess{}, implreview.ProcessHandoff{}); err == nil {
		t.Fatal("Complete accepted an empty admission")
	}

	subject := implreview.ReviewSubject{
		ReviewObligationID: "obligation-invalid-status",
		ProposalID:         "github:o/r#8",
		ReviewedRevision:   "def456",
	}
	admitted, err := coordinator.Prepare(context.Background(), subject)
	if err != nil {
		t.Fatal(err)
	}
	got, err := coordinator.Complete(context.Background(), admitted, implreview.ProcessHandoff{
		Status:           implreview.HandoffStatus("unknown"),
		DiagnosticDigest: "untrusted diagnostic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != implreview.HandoffMalformed || store.status != implreview.HandoffMalformed {
		t.Fatalf("unknown handoff status = %+v, durable=%q", got, store.status)
	}

	got, err = coordinator.Complete(context.Background(), admitted, implreview.ProcessHandoff{})
	if err != nil || got.Status != implreview.HandoffMissing {
		t.Fatalf("empty handoff = %+v, %v", got, err)
	}

	want := errors.New("finish failed")
	store.finishErr = want
	if _, err := coordinator.Complete(context.Background(), admitted, implreview.ProcessHandoff{Status: implreview.HandoffMissing}); !errors.Is(err, want) {
		t.Fatalf("finish error = %v, want %v", err, want)
	}
}
