package claim

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

type transitionStore struct{ record Record }

func (s transitionStore) Claim(context.Context, Record) (ClaimResult, error) {
	return ClaimResult{}, errors.New("unused")
}

func (s transitionStore) Transition(context.Context, Record, Terminal) (Record, error) {
	return s.record, nil
}

func fixtureRecord() Record {
	return Record{SchemaVersion: SchemaVersion, JobID: "daily", JobDate: "2026-10-20", Timezone: "America/Los_Angeles",
		OwnerNonce: strings.Repeat("a", 64), ConfigCommit: strings.Repeat("b", 40), ClaimedAt: time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC), Outcome: OutcomeClaimed,
		Route: nativerouting.SelectedProfile{CatalogID: "catalog", PolicyIdentity: "policy", Harness: entity.Codex, Role: nativerouting.RoleImplementation,
			NativeProfileID: "frontier", NativeModel: "gpt", NativeEffort: "high", Provides: nativerouting.Provides{Capability: executiondemand.CapabilityFrontier},
			QualificationID: "qualification", QualificationDigest: "digest", AdapterGeneration: "adapter"}}
}

func TestRecordCodecIsClosedAndTerminalMetadataIsBounded(t *testing.T) {
	record := fixtureRecord()
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil || !SameOwner(record, decoded) || decoded.Outcome != OutcomeClaimed {
		t.Fatalf("Decode() = (%+v, %v)", decoded, err)
	}
	unknown := bytes.Replace(data, []byte(`"job_id"`), []byte(`"unknown":true,"job_id"`), 1)
	if _, err := Decode(unknown); err == nil {
		t.Fatal("unknown record field was accepted")
	}
	duplicate := bytes.Replace(data, []byte(`"job_id":"daily"`), []byte(`"job_id":"daily","job_id":"daily"`), 1)
	if _, err := Decode(duplicate); err == nil {
		t.Fatal("duplicate record field was accepted")
	}
	if _, err := Decode([]byte(`{"schema_version":"future"}`)); err == nil {
		t.Fatal("incompatible record was accepted")
	}
	finished := record.ClaimedAt.Add(time.Minute)
	duration := int64(time.Minute / time.Millisecond)
	exit := 1
	terminal := Terminal{Outcome: OutcomeFailed, FinishedAt: finished, DurationMS: &duration, ExitCode: &exit, Summary: "bounded", Cause: "agent failed", DraftPRURL: "https://example.test/pull/1", CodeHead: strings.Repeat("c", 40), Evidence: []string{"artifact:report-1"}}
	updated, err := WithTerminal(record, terminal)
	if err != nil || updated.Outcome != OutcomeFailed || !SameOwner(record, updated) {
		t.Fatalf("WithTerminal() = (%+v, %v)", updated, err)
	}
	if _, err := WithTerminal(updated, terminal); err == nil {
		t.Fatal("second terminal transition was accepted")
	}
	terminal.Evidence = []string{"/private/host/path"}
	if _, err := WithTerminal(record, terminal); err == nil {
		t.Fatal("private absolute evidence path was accepted")
	}
	terminal.Evidence, terminal.Cause = nil, "failed at /Users/operator/private"
	if _, err := WithTerminal(record, terminal); err == nil {
		t.Fatal("private path in terminal cause was accepted")
	}
}

func TestRecordRejectsMalformedIdentityRouteAndOutcome(t *testing.T) {
	for name, edit := range map[string]func(*Record){
		"date":      func(r *Record) { r.JobDate = "2026-2-3" },
		"nonce":     func(r *Record) { r.OwnerNonce = "predictable" },
		"config":    func(r *Record) { r.ConfigCommit = "branch" },
		"timezone":  func(r *Record) { r.Timezone = "Local" },
		"route":     func(r *Record) { r.Route.PolicyIdentity = "" },
		"timestamp": func(r *Record) { r.ClaimedAt = time.Time{} },
		"outcome":   func(r *Record) { r.Outcome = "future" },
		"premature": func(r *Record) { value := 0; r.ExitCode = &value },
		"unsafe_url": func(r *Record) {
			r.Outcome = OutcomeFailed
			at := r.ClaimedAt
			r.FinishedAt = &at
			r.DraftPRURL = "file:///private/report"
		},
	} {
		t.Run(name, func(t *testing.T) {
			record := fixtureRecord()
			edit(&record)
			if err := record.Validate(); err == nil {
				t.Fatalf("invalid record accepted: %+v", record)
			}
		})
	}
}

func TestCodecAndServiceRefusalBranches(t *testing.T) {
	record := fixtureRecord()
	oversize := record
	oversize.Route.NativeModel = strings.Repeat("x", maxRecordBytes)
	if _, err := Encode(oversize); err == nil {
		t.Fatal("oversized record encoded")
	}
	if _, err := Decode(append(mustEncode(t, record), []byte(`{}`)...)); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
	if safeReference("private/path") || safeReference("artifact id") || !safeReference("https://example.test/evidence") || !safeReference("artifact:report-1") {
		t.Fatal("safe evidence reference grammar is incorrect")
	}
	finished := record.ClaimedAt.Add(time.Second)
	for _, cause := range []string{strings.Repeat("x", 2049), "bad\rtext", "C:\\private\\file"} {
		if _, err := WithTerminal(record, Terminal{Outcome: OutcomeFailed, FinishedAt: finished, Cause: cause}); err == nil {
			t.Fatalf("unsafe cause accepted: %q", cause)
		}
	}
	if _, err := (Service{}).Transition(t.Context(), record, Terminal{}); err == nil {
		t.Fatal("transition without store succeeded")
	}
	want := record
	want.Outcome, want.FinishedAt = OutcomeTimedOut, &finished
	got, err := (Service{Store: transitionStore{record: want}}).Transition(t.Context(), record, Terminal{Outcome: OutcomeTimedOut, FinishedAt: finished})
	if err != nil || got.Outcome != OutcomeTimedOut {
		t.Fatalf("service transition = (%+v, %v)", got, err)
	}
}

func mustEncode(t *testing.T, record Record) []byte {
	t.Helper()
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
