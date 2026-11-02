package report

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// WireSchema describes canonical JSON serialization, not review validity or
// provenance. Its field names come from the same compiled types Unmarshal
// reads. Semantic validation, exact bindings and native admission still apply.
type WireSchema struct {
	Dialect              string                 `json:"$schema,omitempty"`
	Comment              string                 `json:"$comment,omitempty"`
	Type                 []string               `json:"type"`
	Properties           map[string]*WireSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *WireSchema            `json:"items,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Const                string                 `json:"const,omitempty"`
	Enum                 []string               `json:"enum,omitempty"`
}

// JSONSchema supplies the installed reviewer with the wire contract without
// requiring a Source checkout or an independently maintained payload example.
func JSONSchema() ([]byte, error) {
	schema, err := schemaFor(reflect.TypeFor[Report]())
	if err != nil {
		return nil, fmt.Errorf("review report wire schema: %w", err)
	}
	schema.Dialect = "https://json-schema.org/draft/2020-12/schema"
	schema.Comment = "Compiled impl-review-report/v1 serialization shape. This is not a report or provenance proof; report.Validate and native host admission remain mandatory."
	schema.Properties["schema_version"].Const = SchemaVersion
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode review wire schema: %w", err)
	}
	return append(data, '\n'), nil
}

func schemaFor(t reflect.Type) (*WireSchema, error) {
	if t.Kind() == reflect.Pointer {
		node, err := schemaFor(t.Elem())
		if err != nil {
			return nil, err
		}
		node.Type = append(node.Type, "null")
		return node, nil
	}
	node := &WireSchema{}
	switch t.Kind() {
	case reflect.Struct:
		closed := false
		node.Type, node.AdditionalProperties = []string{"object"}, &closed
		node.Properties = map[string]*WireSchema{}
		for field := range t.Fields() {
			name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if field.PkgPath != "" || name == "" {
				return nil, fmt.Errorf("%s.%s needs an exported explicit JSON field", t.Name(), field.Name)
			}
			child, err := schemaFor(field.Type)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", t.Name(), field.Name, err)
			}
			child.Enum = fieldEnum(t, name)
			node.Properties[name] = child
			if !strings.Contains(","+options+",", ",omitempty,") {
				node.Required = append(node.Required, name)
			}
		}
	case reflect.Slice:
		child, err := schemaFor(t.Elem())
		if err != nil {
			return nil, err
		}
		node.Type, node.Items = []string{"array", "null"}, child
	case reflect.String:
		node.Type = []string{"string"}
	case reflect.Int:
		node.Type = []string{"integer"}
	default:
		return nil, fmt.Errorf("unsupported review wire type %s", t)
	}
	return node, nil
}

func fieldEnum(t reflect.Type, name string) []string {
	switch t {
	case reflect.TypeFor[Report]():
		switch name {
		case "band":
			return []string{BandSkip, BandStandard, BandDeep}
		case "verdict":
			return []string{VerdictApprove, VerdictApproveWithAdvisories, VerdictRequestChanges, VerdictEscalate}
		}
	case reflect.TypeFor[Finding]():
		switch name {
		case "lens":
			return []string{LensSpecFidelity, LensDesignConformance, LensSecurityAbuse, LensCorrectnessCritic}
		case "severity":
			return []string{SeverityBlocking, SeverityAdvisory}
		}
	case reflect.TypeFor[Disposition]():
		switch name {
		case "actor":
			return []string{ActorWorker, ActorOperator}
		case "verdict":
			return []string{VerdictApplied, VerdictDeclined, VerdictAccepted, VerdictRejected}
		}
	}
	return nil
}
