// Package taskspec encodes and validates task carriers at the tracker readback
// boundary. fs-spec/v2 carries complete planning requirements.
package taskspec

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

// Spec is the typed view of a projected task capsule.
type Spec struct {
	Goal            string
	Acceptance      []string
	Scope           string
	Type            string
	Deps            []int
	DoD             string
	Design          string
	Plan            string
	Requirements    *taskrequirements.Spec
	Traces          []string
	PlanTaskID      string
	PlanFingerprint string
}

// DemandIdentity supplies the repository/task identity that belongs to the
// tracker carrier rather than the fs-spec payload itself.
type DemandIdentity struct {
	Repository string
	Task       string
}

// Parse reads all comment bodies for exactly one unambiguous, structurally
// valid fs-spec/v2 capsule. Repeated equivalent projections are harmless;
// conflicting capsules fail closed before a dispatcher can claim the task.
func Parse(comments []string) (Spec, error) {
	var (
		found bool
		want  Spec
	)
	for commentIndex, body := range comments {
		rest := body
		for {
			start := strings.Index(rest, "<!-- fs-spec:")
			if start < 0 {
				break
			}
			prefix := NativeMarkerPrefix
			if !strings.HasPrefix(rest[start:], prefix) {
				return Spec{}, fmt.Errorf("taskspec: comment %d has an unsupported spec version", commentIndex+1)
			}
			rest = rest[start+len(prefix):]
			end := strings.Index(rest, "-->")
			if end < 0 {
				return Spec{}, fmt.Errorf("taskspec: comment %d has an unterminated fs-spec:v2 capsule", commentIndex+1)
			}
			got, err := decodeNative([]byte(strings.TrimSpace(rest[:end])))
			if err != nil {
				return Spec{}, fmt.Errorf("taskspec: comment %d: %w", commentIndex+1, err)
			}
			if found && !reflect.DeepEqual(got, want) {
				return Spec{}, errors.New("taskspec: conflicting fs-spec:v2 capsules")
			}
			want = got
			found = true
			rest = rest[end+len("-->"):]
		}
	}
	if !found {
		return Spec{}, errors.New("taskspec: no fs-spec:v2 capsule found")
	}
	return want, nil
}
