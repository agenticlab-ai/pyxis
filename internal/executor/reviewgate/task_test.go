package reviewgate

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func completeFixture(t *testing.T) (Binding, Snapshot, Policy) {
	t.Helper()
	b, err := Bind("/local/origin.git", codehost.ProposalGeneration{
		OID:      strings.Repeat("a", 40),
		Proposal: codehost.Proposal{SchemaVersion: codehost.ProposalSchema, Kind: "implementation", State: "prepared", ID: "task-widget-t1", Feature: "widget", Task: "T1", Sequence: 2, Branch: "refs/heads/task/widget-t1", Head: strings.Repeat("b", 40), TargetRef: "refs/heads/feature/widget", TargetOID: strings.Repeat("c", 40), TaskPreparation: &codehost.TaskPreparation{SchemaVersion: codehost.TaskPreparationSchema, Operation: codehost.TaskMerge, Nonce: strings.Repeat("0", 32)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cal, err := os.ReadFile("../../../.pyxis/skills/impl-review/calibration.md")
	if err != nil {
		t.Fatal(err)
	}
	p := Policy{SchemaVersion: PolicySchema, ReviewBand: report.BandStandard, NativeModel: "native-model", Prompts: "prompts-v1", Calibration: string(cal), Required: []string{"task-tests", "task-policy"},
		CodeGate: FreezeCodeGatePolicy(codegatemirror.Config{Checks: []codegatemirror.Check{
			{Name: "build", Command: "go build ./...", Class: codegatemirror.ClassRequired},
			{Name: "test", Command: "go test ./..."},
			{Name: "advisory", Command: "style-check", Class: codegatemirror.ClassAdvisory},
		}}),
	}
	r := &report.Report{SchemaVersion: report.SchemaVersion, ProposalRef: b.Subject(), ReviewID: "process-1", Band: report.BandStandard, Verdict: report.VerdictApprove,
		Rounds: []report.Round{{ReviewedRevision: b.Head, ContextID: "context-1", Provenance: report.Provenance{Model: p.NativeModel, PromptsVersion: p.Prompts, CalVersion: calibration.Version}}},
	}
	s := Snapshot{SchemaVersion: SnapshotSchema, Binding: b, Policy: p,
		Review: implreview.AggregateHandoff{Status: implreview.HandoffCompleted, Aggregate: &implreview.CompletedAggregate{ProcessID: r.ReviewID, ProposalID: b.Subject(), ReviewedRevision: b.Head, Report: r}},
		CodeGate: FreezeCodeGateReport(codegatemirror.Report{Passed: true, Outcomes: []codegatemirror.Outcome{
			{Name: "build", Class: codegatemirror.ClassRequired, Passed: true},
			{Name: "test", Class: codegatemirror.ClassRequired, Passed: true},
			{Name: "advisory", Class: codegatemirror.ClassAdvisory, Passed: false, Detail: "advisory only"},
		}}),
		Checks: []Check{{"task-tests", port.CheckGreen}, {"task-policy", port.CheckGreen}},
	}
	refreshDigest(t, &s)
	return b, s, p
}

func refreshDigest(t *testing.T, s *Snapshot) {
	t.Helper()
	payload, err := s.Review.Aggregate.Report.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	s.Review.DiagnosticDigest = fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
}

func TestCompleteCurrentTaskGateAcceptsFullEvidenceWithAdvisoryFailure(t *testing.T) {
	b, s, p := completeFixture(t)
	if err := Validate(b, s, p); err != nil {
		t.Fatal(err)
	}
	// All data required for validation survives a plain serialized round trip.
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var recovered Snapshot
	if err := json.Unmarshal(data, &recovered); err != nil {
		t.Fatal(err)
	}
	if err := Validate(b, recovered, p); err != nil {
		t.Fatalf("retained gate cannot be reconstructed: %v", err)
	}
}

func TestTaskGateRejectsIncompleteStaleOrUnboundEvidence(t *testing.T) {
	for _, failure := range []string{
		"wrong snapshot schema", "different generation", "ABA review replay", "different policy", "missing model policy", "invalid calibration",
		"missing process", "missing aggregate", "wrong process", "foreign report", "missing rounds", "stale review", "wrong native model", "wrong digest", "unsupported verdict",
		"summary gate only", "subset gate", "duplicate gate", "failed required gate", "wrong check class", "empty gate command", "duplicate gate policy", "only advisory gate",
		"missing required check", "pending required check", "duplicate check", "unnamed check", "duplicate required set", "empty required name",
	} {
		t.Run(failure, func(t *testing.T) {
			b, s, p := completeFixture(t)
			switch failure {
			case "wrong snapshot schema":
				s.SchemaVersion = "unknown"
			case "different generation":
				s.Binding.Sequence++
			case "ABA review replay":
				b.ProposalOID, b.Sequence = strings.Repeat("d", 40), b.Sequence+2
				s.Binding = b // Head still matches, but the old process is not current.
			case "different policy":
				p.Prompts = "new-prompt-policy"
			case "missing model policy":
				p.NativeModel = ""
				s.Policy = p
			case "invalid calibration":
				p.Calibration = "not a calibration table"
				s.Policy = p
			case "missing process":
				s.Review.Status = implreview.HandoffMissing
			case "missing aggregate":
				s.Review.Aggregate = nil
			case "wrong process":
				s.Review.Aggregate.ProcessID = "another-process"
			case "foreign report":
				s.Review.Aggregate.Report.ProposalRef = "another-proposal"
			case "missing rounds":
				s.Review.Aggregate.Report.Rounds = nil
			case "stale review":
				s.Review.Aggregate.Report.Rounds[0].ReviewedRevision = strings.Repeat("e", 40)
			case "wrong native model":
				s.Review.Aggregate.Report.Rounds[0].Provenance.Model = "wrong-model"
			case "wrong digest":
				s.Review.DiagnosticDigest = "sha256:" + strings.Repeat("0", 64)
			case "unsupported verdict":
				s.Review.Aggregate.Report.Verdict = report.VerdictApproveWithAdvisories
				refreshDigest(t, &s)
			case "summary gate only":
				s.CodeGate = GateReport{SchemaVersion: GateReportSchema, Passed: true}
			case "subset gate":
				s.CodeGate.Outcomes = s.CodeGate.Outcomes[:1]
			case "duplicate gate":
				s.CodeGate.Outcomes[1].Name = "build"
			case "failed required gate":
				s.CodeGate.Outcomes[1].Passed = false
			case "wrong check class":
				s.CodeGate.Outcomes[1].Class = "advisory"
			case "empty gate command":
				p.CodeGate.Checks[0].Command = ""
				s.Policy = p
			case "duplicate gate policy":
				p.CodeGate.Checks[1].Name = "build"
				s.Policy = p
			case "only advisory gate":
				for i := range p.CodeGate.Checks {
					p.CodeGate.Checks[i].Class = "advisory"
					s.CodeGate.Outcomes[i].Class = "advisory"
				}
				s.Policy = p
			case "missing required check":
				s.Checks = s.Checks[:1]
			case "pending required check":
				s.Checks[1].State = port.CheckPending
			case "duplicate check":
				s.Checks[1].Name = s.Checks[0].Name
			case "unnamed check":
				s.Checks[1].Name = ""
			case "duplicate required set":
				p.Required = append(p.Required, p.Required[0])
				s.Policy = p
			case "empty required name":
				p.Required = append(p.Required, "")
				s.Policy = p
			}
			if err := Validate(b, s, p); !errors.Is(err, ErrIncomplete) {
				t.Fatalf("invalid current task proof accepted: %v", err)
			}
		})
	}
}

func TestTaskGateBindingRequiresTaskAndFullPreparedIdentity(t *testing.T) {
	for _, failure := range []string{"open", "draft", "final", "zero sequence", "short object", "uppercase object", "mixed object format", "target alias", "unknown schema"} {
		t.Run(failure, func(t *testing.T) {
			b, _, _ := completeFixture(t)
			g := codehost.ProposalGeneration{OID: b.ProposalOID, Proposal: codehost.Proposal{SchemaVersion: codehost.ProposalSchema, ID: b.ProposalID, Kind: "implementation", State: "prepared", Feature: b.Feature, Task: b.Task, Sequence: b.Sequence, Branch: b.Branch, Head: b.Head, TargetRef: b.TargetRef, TargetOID: b.TargetOID}}
			g.Proposal.TaskPreparation = &codehost.TaskPreparation{SchemaVersion: codehost.TaskPreparationSchema, Operation: codehost.TaskMerge, Nonce: strings.Repeat("0", 32)}
			switch failure {
			case "open":
				g.Proposal.State = "open"
			case "draft":
				g.Proposal.Kind = "draft"
			case "final":
				g.Proposal.Task = ""
			case "zero sequence":
				g.Proposal.Sequence = 0
			case "short object":
				g.OID = "abc"
			case "uppercase object":
				g.OID = strings.ToUpper(g.OID)
			case "mixed object format":
				g.OID = strings.Repeat("a", 64)
			case "target alias":
				g.Proposal.Branch = g.Proposal.TargetRef
			case "unknown schema":
				b.SchemaVersion = "unknown"
				if !errors.Is(b.Validate(), ErrIncomplete) {
					t.Fatal("unknown task binding accepted")
				}
				return
			}
			if _, err := Bind(b.Origin, g); !errors.Is(err, ErrIncomplete) {
				t.Fatalf("invalid proposal bound for task review: %v", err)
			}
		})
	}
}
