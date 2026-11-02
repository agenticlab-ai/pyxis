package taskspec

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const NativeMarkerPrefix = "<!-- fs-spec:v2 "

// ParseTask accepts a native spec in a task body (atomic direct creation) or
// its comments (plan projection). Legacy body text remains non-authoritative.
func ParseTask(body string, comments []string) (Spec, error) {
	inputs := append([]string(nil), comments...)
	if strings.Contains(body, "<!-- fs-spec:") {
		inputs = append(inputs, body)
	}
	return Parse(inputs)
}

// TaskIdentity preserves a title's explicit plan task ID and supplies the
// self-contained ID for native ad hoc tasks without a title prefix.
func (s Spec) TaskIdentity(titleID string) string {
	if s.Requirements == nil || (titleID != "" && !strings.HasPrefix(s.PlanTaskID, "adhoc/")) {
		return titleID
	}
	_, id, found := strings.Cut(s.PlanTaskID, "/")
	if !found {
		return s.PlanTaskID
	}
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}

// MatchNative reports absence separately from corrupt, stale, conflicting, or
// future-version carriers. Absence may be healed; a mismatch must never route.
func MatchNative(comments []string, expected string) (bool, error) {
	found := false
	for _, body := range comments {
		if strings.Contains(body, "<!-- fs-spec:") {
			found = true
		}
	}
	if !found {
		return false, nil
	}
	want, err := Parse([]string{expected})
	if err != nil {
		return false, fmt.Errorf("invalid expected native spec: %w", err)
	}
	got, err := Parse(comments)
	if err != nil {
		return false, err
	}
	if want.Requirements == nil || !reflect.DeepEqual(got, want) {
		return false, fmt.Errorf("task spec differs from the frozen native plan")
	}
	return true, nil
}

type nativeWire struct {
	Schema          string                `json:"schema_version"`
	Goal            string                `json:"goal"`
	Acceptance      []string              `json:"acceptance"`
	Scope           string                `json:"scope"`
	Type            string                `json:"type"`
	Deps            []int                 `json:"deps,omitempty"`
	DoD             string                `json:"dod"`
	Design          string                `json:"design,omitempty"`
	Plan            string                `json:"plan"`
	PlanTaskID      string                `json:"plan_task_id"`
	PlanFingerprint string                `json:"plan_fingerprint"`
	Requirements    taskrequirements.Spec `json:"requirements"`
	Traces          []string              `json:"traces,omitempty"`
}

// RenderNative emits the closed native task carrier. JSON escapes HTML so no
// charter or rationale can terminate the comment or trigger a mention.
func RenderNative(s Spec) (string, error) {
	if s.Requirements == nil {
		return "", fmt.Errorf("task needs complete requirements")
	}
	w := nativeWire{Schema: "task-spec/v2", Goal: s.Goal, Acceptance: s.Acceptance, Scope: s.Scope, Type: s.Type, Deps: s.Deps, DoD: s.DoD, Design: s.Design, Plan: s.Plan, PlanTaskID: s.PlanTaskID, PlanFingerprint: s.PlanFingerprint, Requirements: *s.Requirements, Traces: s.Traces}
	b, err := json.Marshal(w)
	if err != nil {
		return "", err
	}
	if _, err := decodeNative(b); err != nil {
		return "", err
	}
	// Preserve exact decoded text while preventing tracker mention triggers.
	b = []byte(strings.ReplaceAll(string(b), "@", `\u0040`))
	return NativeMarkerPrefix + string(b) + " -->", nil
}

func decodeNative(b []byte) (Spec, error) {
	var w nativeWire
	if err := taskrouting.Decode(b, &w); err != nil {
		return Spec{}, err
	}
	if w.Schema != "task-spec/v2" {
		return Spec{}, fmt.Errorf("unsupported native task schema %q", w.Schema)
	}
	if strings.TrimSpace(w.Goal) == "" || len(w.Acceptance) == 0 || w.PlanTaskID == "" || w.PlanFingerprint == "" {
		return Spec{}, fmt.Errorf("native task requires goal, acceptance, task identity, and source fingerprint")
	}
	if err := w.Requirements.Validate(taskrequirements.Sources(w.Traces)); err != nil {
		return Spec{}, err
	}
	return Spec{Goal: w.Goal, Acceptance: w.Acceptance, Scope: w.Scope, Type: w.Type, Deps: w.Deps, DoD: w.DoD, Design: w.Design, Plan: w.Plan, PlanTaskID: w.PlanTaskID, PlanFingerprint: w.PlanFingerprint, Requirements: &w.Requirements, Traces: w.Traces}, nil
}
