package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedClock is a deterministic clock for hermetic time stamping.
func fixedClock() func() time.Time {
	t := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

// captureOperator builds an OperatorSink over a text handler writing to buf, with
// the record's wall-clock time dropped so assertions are deterministic.
func captureOperator(buf *bytes.Buffer) *OperatorSink {
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	return NewOperatorSink(slog.New(h))
}

func TestOperatorSink_TaskDispatchedLogsModelRoute(t *testing.T) {
	var buf bytes.Buffer
	event := TaskDispatched("T1", 11, "codex")
	event.Model = "gpt-5.6-sol"
	event.Effort = "xhigh"

	if err := captureOperator(&buf).Deliver(event); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	for _, want := range []string{"task dispatched", "model=gpt-5.6-sol", "effort=xhigh"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("operator view missing %q\n---\n%s", want, buf.String())
		}
	}
}

// journalEvents parses the JSONL journal buffer back into events.
func journalEvents(t *testing.T, buf *bytes.Buffer) []Event {
	t.Helper()
	var out []Event
	sc := bufio.NewScanner(buf)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("journal line is not valid JSON: %q: %v", line, err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan journal: %v", err)
	}
	return out
}

// A full per-task lifecycle emitted through one Reporter lands identically on the
// attended operator view AND the unattended argus journal — the same run reports
// under both drivers over one stream, two sinks (§3.9, AP-2). This is T13's first
// acceptance criterion at the reporting layer.
func TestReporter_OneStreamReachesBothSinks(t *testing.T) {
	var opBuf, journal bytes.Buffer
	op := captureOperator(&opBuf)
	argus := NewArgusSink(&journal)
	r := NewReporterWith("run-xyz", []Sink{op, argus}, WithClock(fixedClock()))

	// One run's worth of lifecycle events (EVT-1..6).
	r.Emit(RunStarted("", ""))
	r.Emit(TaskDispatched("T1", 11, "claude"))
	r.Emit(CIFixAttempt("T1", 1, 3))
	r.Emit(TaskVerified("T1", 11, VerdictGreen))
	r.Emit(TaskOutcome("T1", 11, "succeeded", false, ""))
	r.Emit(RunFinished(Counts{Succeeded: 1}))

	// Attended: every lifecycle line is in the operator view, correlated by run.
	opText := opBuf.String()
	for _, want := range []string{
		"run started", "task dispatched", "ci fix attempt",
		"task verified", "task outcome", "run finished",
		"run=run-xyz", "task=T1", "harness=claude", "verdict=green", "class=succeeded",
	} {
		if !strings.Contains(opText, want) {
			t.Errorf("operator view missing %q\n---\n%s", want, opText)
		}
	}

	// Unattended: the identical stream is the JSONL journal, one line per event.
	evs := journalEvents(t, &journal)
	if len(evs) != 6 {
		t.Fatalf("journal has %d events, want 6 (one per lifecycle event)", len(evs))
	}
	wantKinds := []Kind{KindRunStarted, KindTaskDispatched, KindCIFixAttempt, KindTaskVerified, KindTaskOutcome, KindRunFinished}
	for i, k := range wantKinds {
		if evs[i].Kind != k {
			t.Errorf("journal[%d].Kind = %q, want %q", i, evs[i].Kind, k)
		}
		if evs[i].RunID != "run-xyz" {
			t.Errorf("journal[%d].RunID = %q, want the stamped run id", i, evs[i].RunID)
		}
		if evs[i].Time.IsZero() {
			t.Errorf("journal[%d].Time is zero; the Reporter must stamp it", i)
		}
	}
	// run_finished carries the final counts (present even when some are zero).
	last := evs[5]
	if last.Counts == nil || last.Counts.Succeeded != 1 {
		t.Errorf("run_finished counts = %+v, want succeeded 1", last.Counts)
	}
}

// A stuck task (respawn/self-heal cap exhausted) fail-closed-escalates to the
// argus sink: it is journaled with escalated=true AND surfaced through the
// escalation handler, and the sink's escalation count reflects it — T13's
// fail-closed acceptance criterion (§3.9).
func TestArgusSink_StuckTaskFailClosedEscalates(t *testing.T) {
	var journal bytes.Buffer
	var surfaced []Event
	argus := NewArgusSink(&journal, WithEscalationHandler(func(e Event) {
		surfaced = append(surfaced, e)
	}))
	r := NewReporterWith("run-esc", []Sink{argus}, WithClock(fixedClock()))

	r.Emit(TaskDispatched("T7", 70, "codex"))
	// The cap was exhausted: a stuck, escalated failed terminal.
	r.Emit(TaskOutcome("T7", 70, "failed", true, "worker respawn cap exhausted without convergence"))
	r.Emit(RunFinished(Counts{Failed: 1}))

	if got := argus.Escalations(); got != 1 {
		t.Fatalf("escalations surfaced = %d, want 1", got)
	}
	if len(surfaced) != 1 || surfaced[0].TaskID != "T7" || !surfaced[0].Escalated {
		t.Fatalf("escalation handler saw %+v, want one escalated T7", surfaced)
	}
	// files-as-truth: the escalation is in the journal (escalated=true) before it
	// was surfaced — nothing escalates that was not first recorded.
	evs := journalEvents(t, &journal)
	var found bool
	for _, e := range evs {
		if e.Kind == KindTaskOutcome && e.TaskID == "T7" {
			found = true
			if !e.Escalated || e.Class != "failed" {
				t.Errorf("journaled outcome = %+v, want escalated failed", e)
			}
		}
	}
	if !found {
		t.Error("the escalated task_outcome is absent from the journal")
	}
}

// A plain failed task (not cap-exhausted) still fail-closed-escalates — the
// design escalates a "stuck/failed" task, so class=failed alone is an escalation
// even without the escalated flag.
func TestArgusSink_PlainFailedAlsoEscalates(t *testing.T) {
	var journal bytes.Buffer
	argus := NewArgusSink(&journal)
	r := NewReporter("run-f", argus)

	r.Emit(TaskOutcome("T2", 20, "failed", false, "checks red"))
	r.Emit(TaskOutcome("T3", 30, "succeeded", false, ""))
	r.Emit(TaskOutcome("T4", 40, "skipped", false, ""))
	r.Emit(TaskOutcome("T5", 50, "blocked", false, ""))

	if got := argus.Escalations(); got != 1 {
		t.Errorf("escalations = %d, want 1 (only the failed task escalates)", got)
	}
}

func TestIsEscalation(t *testing.T) {
	tests := []struct {
		name string
		e    Event
		want bool
	}{
		{"failed escalates", TaskOutcome("T", 1, "failed", false, ""), true},
		{"escalated flag escalates", TaskOutcome("T", 1, "succeeded", true, ""), true},
		{"succeeded does not", TaskOutcome("T", 1, "succeeded", false, ""), false},
		{"failed review escalates", ImplReviewTerminal(ImplReviewTerminalFields{Harness: "claude", Phase: "artifact", Status: ReviewStatusFailed, ObligationStatus: ReviewObligationPending, FailureClass: "artifact_unverified", Escalated: true}), true},
		{"blocked does not", TaskOutcome("T", 1, "blocked", false, ""), false},
		{"skipped does not", TaskOutcome("T", 1, "skipped", false, ""), false},
		{"dispatched is progress", TaskDispatched("T", 1, "claude"), false},
		{"verified red is progress not escalation", TaskVerified("T", 1, VerdictRed), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEscalation(tt.e); got != tt.want {
				t.Errorf("IsEscalation(%+v) = %v, want %v", tt.e, got, tt.want)
			}
		})
	}
}

func TestFailedReviewTerminalReachesBothSinksWithRoutingIdentity(t *testing.T) {
	var operator, journal bytes.Buffer
	argus := NewArgusSink(&journal)
	r := NewReporterWith("run-review", []Sink{captureOperator(&operator), argus}, WithClock(fixedClock()))

	e := ImplReviewTerminal(ImplReviewTerminalFields{TaskID: "T9", Harness: "claude",
		ProposalRef: "github:o/r#90", Revision: "abc123", NativeProfile: "opus-max-review",
		Catalog: "claude-review-v1", Policy: "sha256:policy", Model: "claude-opus-5", Effort: "max",
		Phase: "artifact", Status: ReviewStatusFailed, ObligationStatus: ReviewObligationPending,
		FailureClass: "artifact_unverified", Reason: "artifact absent", Escalated: true})
	r.Emit(e)

	for _, want := range []string{
		"impl review terminal", "escalated=true", "harness=claude", "proposal=github:o/r#90",
		"revision=abc123", "native_profile=opus-max-review", "catalog=claude-review-v1",
		"policy=sha256:policy", "model=claude-opus-5", "effort=max", "status=failed",
		"failure_class=artifact_unverified", "reason=\"artifact absent\"",
	} {
		if !strings.Contains(operator.String(), want) {
			t.Errorf("operator review terminal missing %q\n---\n%s", want, operator.String())
		}
	}
	events := journalEvents(t, &journal)
	if len(events) != 1 {
		t.Fatalf("journal events = %d, want 1", len(events))
	}
	got := events[0]
	if got.Kind != KindImplReviewTerminal || !got.Escalated || got.Status != ReviewStatusFailed ||
		got.Catalog != "claude-review-v1" || got.Policy != "sha256:policy" || got.Reason != "artifact absent" {
		t.Fatalf("journaled review terminal = %+v", got)
	}
	if argus.Escalations() != 1 {
		t.Fatalf("review escalations = %d, want 1", argus.Escalations())
	}
}

func TestImplReviewTerminalWireUsesExactEVT15Fields(t *testing.T) {
	e := ImplReviewTerminal(ImplReviewTerminalFields{DriveRoundID: "round-1", AttemptID: "attempt-1", ObligationID: "obligation-1", TaskID: "T1", ProposalRef: "github:o/r#1", Revision: "abc", Harness: "claude", ReviewerPrincipal: "review-bot", NativeProfile: "profile", Catalog: "catalog", Policy: "policy", Model: "model", Effort: "high", Phase: "attribution", Status: ReviewStatusFailed, ObligationStatus: ReviewObligationPending, FailureClass: "artifact_untrusted", ArtifactRef: "github:7", ArtifactProofRef: "github:broker-receipt/7", Reason: "attempt proof mismatch", Escalated: true})
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"kind", "drive_round_id", "review_attempt_id", "review_obligation_id", "task_id", "proposal_ref", "revision", "backend", "reviewer_principal", "native_profile", "catalog", "policy", "model", "effort", "phase", "status", "obligation_status", "failure_class", "artifact_ref", "artifact_proof_ref", "reason", "escalated"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("EVT-15 missing %q: %s", key, b)
		}
	}
	for _, forbidden := range []string{"class", "review_status", "review_backend", "review_proposal", "review_revision"} {
		if _, ok := wire[forbidden]; ok {
			t.Errorf("EVT-15 carried undeclared %q: %s", forbidden, b)
		}
	}
}

func TestVerifiedImplReviewTerminalCarriesExplicitNonEscalation(t *testing.T) {
	b, err := json.Marshal(ImplReviewTerminal(ImplReviewTerminalFields{
		AttemptID: "attempt-1", ObligationID: "obligation-1", TaskID: "T1",
		ProposalRef: "github:o/r#1", Revision: "abc", Harness: "claude",
		ReviewerPrincipal: "review-bot", Phase: "artifact", Status: ReviewStatusCompleted,
		ObligationStatus: ReviewObligationVerified, ArtifactRef: "github:7",
		ArtifactProofRef: "github:broker-receipt/7",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	if got, ok := wire["escalated"]; !ok || string(got) != "false" {
		t.Fatalf("verified EVT-15 escalated = %s, present=%v; wire=%s", got, ok, b)
	}
	legacy, err := json.Marshal(TaskOutcome("T1", 1, "succeeded", false, ""))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacy), `"escalated"`) {
		t.Fatalf("legacy task_outcome wire changed: %s", legacy)
	}
}

func TestHostedReviewPublicationTerminalWireAndSinksUseExactEVT17Fields(t *testing.T) {
	var operator, journal bytes.Buffer
	reporter := NewReporterWith("run-publication", []Sink{captureOperator(&operator), NewArgusSink(&journal)}, WithClock(fixedClock()))
	event := HostedReviewPublicationTerminal(HostedReviewPublicationTerminalFields{
		AttemptID: "process-1", ObligationID: "obligation-1", ProposalRef: "github:o/r#7", Revision: "abc123",
		Status: ReviewStatusCompleted, ArtifactRef: "sha256:artifact", NativeReceipt: "github-comment:R_test:7",
		AttemptProofRef: "proof-1", Escalated: false,
	})
	reporter.Emit(event)
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"kind", "review_attempt_id", "review_obligation_id", "proposal_ref", "revision", "status", "artifact_ref", "native_receipt", "attempt_proof_ref", "escalated"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("EVT-17 missing %q: %s", key, b)
		}
	}
	for _, forbidden := range []string{"publication_attempt_id", "artifact_proof_ref", "reviewer_principal", "class"} {
		if _, ok := wire[forbidden]; ok {
			t.Errorf("EVT-17 carried undeclared %q: %s", forbidden, b)
		}
	}
	for _, want := range []string{"hosted review publication terminal", "review_attempt_id=process-1", "review_obligation_id=obligation-1", "native_receipt=github-comment:R_test:7", "attempt_proof=proof-1"} {
		if !strings.Contains(operator.String(), want) {
			t.Errorf("operator EVT-17 missing %q:\n%s", want, operator.String())
		}
	}
	events := journalEvents(t, &journal)
	if len(events) != 1 || events[0].Kind != KindHostedReviewPublicationTerminal || events[0].NativeReceipt != "github-comment:R_test:7" || events[0].AttemptProofRef != "proof-1" {
		t.Fatalf("journal EVT-17 = %+v", events)
	}
}

func TestReviewArtifactPublicationTerminalUsesExactEVT13Wire(t *testing.T) {
	event := ReviewArtifactPublicationTerminal(ReviewArtifactPublicationTerminalFields{
		ProcessID: "process-1", ObligationID: "obligation-1", PublicationAttemptID: "publication-1",
		ProposalID: "github:o/r#7", ReviewedRevision: "abc123", Medium: "github",
		IdempotencyID: "idempotency-1", Status: "verified", ArtifactRef: "sha256:artifact",
		NativeReceipt: "github-comment:R_test:7", AttemptProofRef: "proof-1",
	})
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"kind", "review_process_id", "review_obligation_id", "publication_attempt_id", "proposal_id", "reviewed_revision", "medium", "idempotency_id", "status", "artifact_ref", "native_receipt", "attempt_proof_ref"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("EVT-13 missing %q: %s", key, b)
		}
	}
	for _, forbidden := range []string{"review_attempt_id", "proposal_ref", "revision", "artifact_proof_ref", "escalated"} {
		if _, ok := wire[forbidden]; ok {
			t.Errorf("EVT-13 carried host-side field %q: %s", forbidden, b)
		}
	}
}

func TestRunFinishedUsesExactEVT16CountSchema(t *testing.T) {
	event := RunFinished(Counts{Succeeded: 1, Tokens: 99, Skipped: 7, AwaitingProposal: 2, ProbeUnavailable: 1})
	b, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		t.Fatal(err)
	}
	want := []string{"succeeded", "failed", "blocked", "tokens", "routing_exact", "routing_promoted", "routing_unsupported", "routing_invalid", "review_completed", "review_failed", "review_pending", "review_artifact_unverified", "review_artifact_untrusted", "review_infrastructure_failed"}
	if len(wire.Counts) != len(want) {
		t.Fatalf("EVT-16 count keys = %v", wire.Counts)
	}
	for _, key := range want {
		if _, ok := wire.Counts[key]; !ok {
			t.Errorf("EVT-16 missing %q: %s", key, b)
		}
	}
	if wire.Counts["tokens"] != 99 {
		t.Fatalf("EVT-16 lost declared token count: %v", wire.Counts)
	}
}

func TestWithheldFrontierRemainsDistinctFromIdleThroughTaskTerminal(t *testing.T) {
	var withheld, idle bytes.Buffer
	withheldReporter := NewReporterWith("run", []Sink{NewArgusSink(&withheld)}, WithClock(fixedClock()))
	withheldReporter.Emit(TaskOutcome("T7", 7, "awaiting-proposal", false, "proposal already open"))
	withheldReporter.Emit(RunFinished(Counts{AwaitingProposal: 1}))
	NewReporterWith("run", []Sink{NewArgusSink(&idle)}, WithClock(fixedClock())).Emit(RunFinished(Counts{}))
	if withheld.String() == idle.String() {
		t.Fatal("withheld frontier became byte-identical to an idle run")
	}
	events := journalEvents(t, &withheld)
	if len(events) != 2 || events[0].Kind != KindTaskOutcome || events[0].Class != "awaiting-proposal" || events[1].Kind != KindRunFinished {
		t.Fatalf("withheld journal = %+v", events)
	}
}

// The Reporter stamps a producer's empty RunID/Time but never overwrites values a
// producer already set (a producer may correlate to a foreign run).
func TestReporter_StampsButNeverOverwrites(t *testing.T) {
	var journal bytes.Buffer
	argus := NewArgusSink(&journal)
	r := NewReporterWith("stamp", []Sink{argus}, WithClock(fixedClock()))

	pre := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	r.Emit(Event{Kind: KindRunStarted})                            // empty → stamped
	r.Emit(Event{Kind: KindRunStarted, RunID: "other", Time: pre}) // preset → kept

	evs := journalEvents(t, &journal)
	if evs[0].RunID != "stamp" || !evs[0].Time.Equal(fixedClock()()) {
		t.Errorf("event 0 = %+v, want stamped run id and clock time", evs[0])
	}
	if evs[1].RunID != "other" || !evs[1].Time.Equal(pre) {
		t.Errorf("event 1 = %+v, want preserved run id and time", evs[1])
	}
}

// A failing sink is surfaced through the delivery-error handler and never stops a
// sibling sink from receiving the event — delivery is best-effort (§3.9: one core
// + two sinks, never one silencing the other).
func TestReporter_BestEffortDelivery(t *testing.T) {
	var journal bytes.Buffer
	good := NewArgusSink(&journal)
	bad := sinkFunc(func(Event) error { return errors.New("boom") })

	var gotErr error
	r := NewReporterWith("be", []Sink{bad, good}, WithDeliveryErrorHandler(func(_ Sink, _ Event, err error) {
		gotErr = err
	}))
	r.Emit(RunStarted("", ""))

	if gotErr == nil {
		t.Error("delivery-error handler was not called for the failing sink")
	}
	if len(journalEvents(t, &journal)) != 1 {
		t.Error("the good sink did not receive the event despite the bad sink failing")
	}
}

// A nil journal writer fails closed at Deliver rather than silently dropping.
func TestArgusSink_NilWriterFailsClosed(t *testing.T) {
	s := NewArgusSink(nil)
	if err := s.Deliver(RunStarted("", "")); err == nil {
		t.Fatal("Deliver to a nil journal writer must error (fail closed), not drop silently")
	}
}

// Concurrent producers (the parallel Workers of a sweep, each with its self-heal
// loop) emit through one Reporter without interleaving or racing: every event
// lands as its own well-formed journal line. Run under -race, this proves the
// serialized fan-out.
func TestReporter_ConcurrentEmitsAreSerialized(t *testing.T) {
	const workers = 16
	const perWorker = 8
	var journal bytes.Buffer
	argus := NewArgusSink(&journal)
	op := captureOperator(&bytes.Buffer{})
	r := NewReporter("conc", argus, op)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range perWorker {
				r.Emit(CIFixAttempt("T", i+1, perWorker))
			}
		})
	}
	wg.Wait()

	evs := journalEvents(t, &journal)
	if len(evs) != workers*perWorker {
		t.Fatalf("journal has %d events, want %d (no lost or interleaved lines)", len(evs), workers*perWorker)
	}
}

// sinkFunc adapts a function to Sink, for the best-effort-delivery test.
type sinkFunc func(Event) error

func (f sinkFunc) Deliver(e Event) error { return f(e) }

// Guard: the operator sink tolerates a nil logger by falling back to the default.
func TestOperatorSink_NilLoggerFallsBack(t *testing.T) {
	s := NewOperatorSink(nil)
	if s.log == nil {
		t.Fatal("nil logger must fall back to slog.Default, never a nil logger")
	}
	if err := s.Deliver(RunStarted("", "")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
}
