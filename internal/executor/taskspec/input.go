package taskspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"strings"
)

// NativeInput is the portable charter shared by Carina creation and issue intake.
type NativeInput struct {
	Schema       string                `json:"schema_version"`
	Title        string                `json:"title"`
	Goal         string                `json:"goal"`
	Acceptance   []string              `json:"acceptance"`
	Scope        string                `json:"scope"`
	Requirements taskrequirements.Spec `json:"requirements"`
}

func CompileInput(input NativeInput, taskID string) (Spec, string, error) {
	if input.Schema != "native-task-input/v1" || strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Goal) == "" || strings.TrimSpace(input.Scope) == "" || len(input.Acceptance) == 0 || strings.TrimSpace(taskID) == "" {
		return Spec{}, "", fmt.Errorf("native task requires native-task-input/v1, title, goal, scope, acceptance, and task identity")
	}
	for _, criterion := range input.Acceptance {
		if strings.TrimSpace(criterion) == "" {
			return Spec{}, "", fmt.Errorf("native task acceptance criteria cannot be empty")
		}
	}
	compiled, err := input.Requirements.Compile(taskrequirements.Sources(nil))
	if err != nil {
		return Spec{}, "", err
	}
	input.Requirements = compiled
	canonical, err := json.Marshal(input)
	if err != nil {
		return Spec{}, "", fmt.Errorf("encode native task: %w", err)
	}
	digest := sha256.Sum256(canonical)
	spec := Spec{Goal: input.Goal, Acceptance: input.Acceptance, Scope: input.Scope, Type: "feat", DoD: "every acceptance criterion verified", PlanTaskID: taskID, PlanFingerprint: "sha256:" + hex.EncodeToString(digest[:]), Requirements: &input.Requirements}
	body, err := RenderInput(input, spec)
	return spec, body, err
}

func RenderInput(input NativeInput, spec Spec) (string, error) {
	marker, err := RenderNative(spec)
	if err != nil {
		return "", err
	}
	var body strings.Builder
	fmt.Fprintf(&body, "## Goal\n\n%s\n\n## Scope\n\n%s\n\n## Acceptance criteria\n\n", input.Goal, input.Scope)
	for _, criterion := range input.Acceptance {
		fmt.Fprintf(&body, "- [ ] %s\n", criterion)
	}
	fmt.Fprintf(&body, "\n## Task requirements\n\n- Implementation: %s\n- Review: %s\n\n%s\n", spec.Requirements.Implementation.Summary(), spec.Requirements.Review.Summary(), marker)
	if _, err := ParseTask(body.String(), nil); err != nil {
		return "", fmt.Errorf("native task body: %w", err)
	}
	return strings.TrimSpace(body.String()), nil
}
