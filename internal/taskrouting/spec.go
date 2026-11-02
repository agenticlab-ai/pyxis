// Package taskrouting carries explicit harness-native choices from a task's
// author to its implementation and review launches. It contains no model ladder.
package taskrouting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/entity"
)

const Schema = "task-native-routing/v1"

// Route is an exact native choice, not a hint to a downstream model picker.
type Route struct {
	Harness entity.Harness `json:"harness"`
	Model   string         `json:"model"`
	Effort  string         `json:"effort"`
}

// Spec freezes independent implementation and review choices. Rationale is
// reviewed with the task; it is never interpreted as launch arguments.
type Spec struct {
	Schema         string `json:"schema_version"`
	Implementation Route  `json:"implementation"`
	Review         Route  `json:"review"`
	Rationale      string `json:"rationale"`
}

// Parse accepts a single closed JSON object, including rejecting duplicate
// keys rather than letting the JSON decoder silently take the last value.
func Parse(b []byte) (Spec, error) {
	var s Spec
	if err := Decode(b, &s); err != nil {
		return Spec{}, err
	}
	if err := s.Validate(); err != nil {
		return Spec{}, err
	}
	return s, nil
}

// Decode decodes one closed JSON value and rejects duplicate fields, unknown
// fields, and trailing values. Callers still validate their schema and values.
func Decode(b []byte, dst any) error {
	if err := uniqueKeys(json.NewDecoder(bytes.NewReader(b))); err != nil {
		return fmt.Errorf("native routing: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return fmt.Errorf("native routing: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("native routing: expected exactly one object")
	}
	return nil
}

func uniqueKeys(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			t, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := t.(string)
			if !ok {
				return fmt.Errorf("invalid object key")
			}
			if seen[key] {
				return fmt.Errorf("duplicate key %q", key)
			}
			seen[key] = true
			if err := uniqueKeys(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueKeys(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
	_, err = d.Token()
	return err
}

func (s Spec) Validate() error {
	if s.Schema != Schema {
		return fmt.Errorf("native routing: unsupported schema %q", s.Schema)
	}
	for _, role := range []struct {
		name  string
		route Route
	}{{"implementation", s.Implementation}, {"review", s.Review}} {
		if !role.route.Harness.Known() {
			return fmt.Errorf("native routing: %s has unknown harness %q", role.name, role.route.Harness)
		}
		for _, field := range []struct{ name, value string }{{"model", role.route.Model}, {"effort", role.route.Effort}} {
			if field.value == "" || strings.ContainsAny(field.value, " \t\r\n\x00") || strings.HasPrefix(field.value, "-") {
				return fmt.Errorf("native routing: %s.%s must be one nonempty native identifier", role.name, field.name)
			}
		}
	}
	if strings.TrimSpace(s.Rationale) == "" {
		return fmt.Errorf("native routing: rationale is required")
	}
	return nil
}
