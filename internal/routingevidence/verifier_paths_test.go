package routingevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSubjectStateCoversEverySubjectKind(t *testing.T) {
	tests := []struct {
		name      string
		view      SubjectView
		wantState string
		wantOwner int
	}{
		{name: "frozen demand", view: SubjectView{Demand: &DemandView{}}, wantState: "demand_frozen"},
		{name: "invalidated demand", view: SubjectView{Demand: &DemandView{InvalidationIDs: []string{"invalidate-1"}}}, wantState: string(StateInvalidated)},
		{name: "attempt", view: SubjectView{Attempt: &AttemptView{State: StateResolvedTerminal, OwnerGeneration: 4}}, wantState: string(StateResolvedTerminal), wantOwner: 4},
		{name: "process", view: SubjectView{Process: &ProcessView{State: StateProcessInFlight}}, wantState: string(StateProcessInFlight)},
		{name: "invalidation", view: SubjectView{Invalidation: &InvalidationView{State: StateInvalidated}}, wantState: string(StateInvalidated)},
		{name: "eligible activation", view: SubjectView{Activation: &ActivationView{ActivationEligible: true}}, wantState: "activation_eligible"},
		{name: "non-accepting activation", view: SubjectView{Activation: &ActivationView{}}, wantState: "activation_non_accepting"},
		{name: "invalidated activation", view: SubjectView{Activation: &ActivationView{ActivationEligible: true, InvalidationIDs: []string{"invalidate-1"}}}, wantState: string(StateInvalidated)},
		{name: "unknown", view: SubjectView{}, wantState: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, owner := subjectState(tt.view)
			if state != tt.wantState || owner != tt.wantOwner {
				t.Fatalf("subjectState() = (%q, %d), want (%q, %d)", state, owner, tt.wantState, tt.wantOwner)
			}
		})
	}
}

func TestEventMatchesSubjectCoversEveryJoin(t *testing.T) {
	event := EventView{
		EventID:   "event-1",
		DemandID:  "demand-1",
		AttemptID: "attempt-1",
		ProcessID: "process-1",
	}
	tests := []struct {
		name string
		view SubjectView
		want bool
	}{
		{name: "demand match", view: SubjectView{Demand: &DemandView{DemandID: "demand-1"}}, want: true},
		{name: "demand miss", view: SubjectView{Demand: &DemandView{DemandID: "other"}}},
		{name: "attempt identity match", view: SubjectView{Attempt: &AttemptView{AttemptID: "attempt-1"}}, want: true},
		{name: "attempt demand match", view: SubjectView{Attempt: &AttemptView{AttemptID: "other", DemandID: "demand-1"}}, want: true},
		{name: "attempt miss", view: SubjectView{Attempt: &AttemptView{AttemptID: "other", DemandID: "other"}}},
		{name: "process identity match", view: SubjectView{Process: &ProcessView{ReviewProcessID: "process-1"}}, want: true},
		{name: "process pass match", view: SubjectView{Process: &ProcessView{ReviewProcessID: "other", Passes: []PassView{{AttemptID: "attempt-1"}}}}, want: true},
		{name: "process miss", view: SubjectView{Process: &ProcessView{ReviewProcessID: "other", Passes: []PassView{{AttemptID: "other"}}}}},
		{name: "invalidation match", view: SubjectView{Invalidation: &InvalidationView{EventID: "event-1"}}, want: true},
		{name: "invalidation miss", view: SubjectView{Invalidation: &InvalidationView{EventID: "other"}}},
		{name: "activation match", view: SubjectView{Activation: &ActivationView{EventID: "event-1"}}, want: true},
		{name: "activation miss", view: SubjectView{Activation: &ActivationView{EventID: "other"}}},
		{name: "unknown", view: SubjectView{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventMatchesSubject(event, tt.view); got != tt.want {
				t.Fatalf("eventMatchesSubject() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestStructuralStateSubjectViewSelectsEveryCanonicalSubject(t *testing.T) {
	invalidation := InvalidationView{InvalidationID: "invalidate-1", EventID: "event-invalidate", State: StateInvalidated}
	brief := RemediationBrief{InvalidationID: "invalidate-1", DecisionRequired: "rerun"}
	activation := ActivationView{ActivationPolicyVersion: "policy/1", ExactMainlineObject: "commit-1", ProofManifestDigest: "manifest-1", InvalidationIDs: []string{"invalidate-1"}}
	state := StructuralState{
		Demands:           []DemandView{{DemandID: "demand-1", InvalidationIDs: []string{"invalidate-1"}}},
		Attempts:          []AttemptView{{AttemptID: "attempt-1", InvalidationIDs: []string{"invalidate-1"}}},
		Processes:         []ProcessView{{ReviewProcessID: "process-1", InvalidationIDs: []string{"invalidate-1"}}},
		Invalidations:     []InvalidationView{invalidation},
		RemediationBriefs: []RemediationBrief{brief},
		Activations:       []ActivationView{activation},
	}
	tests := []struct {
		name    string
		subject SubjectID
		check   func(SubjectView) bool
	}{
		{name: "demand", subject: SubjectID{Kind: SubjectDemand, ID: "demand-1"}, check: func(v SubjectView) bool { return v.Demand != nil }},
		{name: "attempt", subject: SubjectID{Kind: SubjectAttempt, ID: "attempt-1"}, check: func(v SubjectView) bool { return v.Attempt != nil }},
		{name: "process", subject: SubjectID{Kind: SubjectProcess, ID: "process-1"}, check: func(v SubjectView) bool { return v.Process != nil }},
		{name: "invalidation", subject: SubjectID{Kind: SubjectInvalidation, ID: "invalidate-1"}, check: func(v SubjectView) bool { return v.Invalidation != nil }},
		{name: "activation", subject: SubjectID{Kind: SubjectActivation, ID: activationSubject(activation)}, check: func(v SubjectView) bool { return v.Activation != nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view, err := state.SubjectView(tt.subject)
			if err != nil || !tt.check(view) {
				t.Fatalf("SubjectView(%+v) = %+v, %v", tt.subject, view, err)
			}
			if len(view.Invalidations) != 1 || len(view.RemediationBriefs) != 1 {
				t.Fatalf("invalidation details = %+v / %+v", view.Invalidations, view.RemediationBriefs)
			}
		})
	}
	if _, err := state.SubjectView(SubjectID{Kind: SubjectDemand, ID: "missing"}); !errors.Is(err, ErrReducerIntegrity) {
		t.Fatalf("missing subject error = %v", err)
	}
	if _, err := state.SubjectView(SubjectID{Kind: "future", ID: "one"}); !errors.Is(err, ErrReducerIntegrity) {
		t.Fatalf("unknown subject error = %v", err)
	}
}

func TestCanonicalJSONWritesTheClosedEscapingAndNumberGrammar(t *testing.T) {
	var dst bytes.Buffer
	writeJSONString(&dst, "\"\\\b\f\n\r\t\x01☃")
	if got, want := dst.String(), `"\"\\\b\f\n\r\t\u0001☃"`; got != want {
		t.Fatalf("escaped string = %q, want %q", got, want)
	}

	for _, value := range []string{"0", "1", "-1", "987654321"} {
		if !canonicalInteger(value) {
			t.Errorf("canonicalInteger(%q) = false", value)
		}
	}
	for _, value := range []string{"", "-", "-0", "01", "1.0", "+1", "1e2"} {
		if canonicalInteger(value) {
			t.Errorf("canonicalInteger(%q) = true", value)
		}
	}

	dst.Reset()
	value := map[string]any{"z": []any{true, false, "ok", json.Number("3")}, "a": "first"}
	if err := writeCanonical(&dst, value); err != nil {
		t.Fatal(err)
	}
	if got := dst.String(); got != `{"a":"first","z":[true,false,"ok",3]}` {
		t.Fatalf("canonical object = %s", got)
	}
	for _, invalid := range []any{nil, json.Number("01"), make(chan int)} {
		dst.Reset()
		if err := writeCanonical(&dst, invalid); err == nil {
			t.Errorf("writeCanonical(%T) succeeded", invalid)
		}
	}
	if _, err := canonicalizeInput([]byte("{} trailing")); err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("trailing-data error = %v", err)
	}
	if _, err := canonicalJSON(map[string]string{"x": "y"}); err == nil || !strings.Contains(err.Error(), "maps are not schema values") {
		t.Fatalf("map schema error = %v", err)
	}
}
