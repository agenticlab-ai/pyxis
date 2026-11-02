package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCompiledWireSchemaClosesTheObservedAliasAndNesting(t *testing.T) {
	data, err := JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	var schema WireSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Properties["schema"] != nil || schema.Properties["schema_version"].Const != SchemaVersion || !slices.Contains(schema.Required, "schema_version") || *schema.AdditionalProperties {
		t.Fatal("envelope schema permits the observed schema alias")
	}
	round := schema.Properties["rounds"].Items
	provenance := round.Properties["provenance"]
	if round.Properties["context_id"] == nil || !slices.Contains(round.Required, "context_id") || provenance.Properties["context_id"] != nil || *provenance.AdditionalProperties {
		t.Fatal("context identity can be misplaced into provenance")
	}
	if !slices.Contains(schema.Properties["rounds"].Type, "null") || !slices.Contains(schema.Properties["escalation_brief"].Type, "null") || slices.Contains(schema.Required, "escalation_brief") {
		t.Fatal("schema diverges from optional/null canonical serialization")
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "golden-valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := Unmarshal(golden)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := rep.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var value json.RawMessage = canonical
	assertSerializedKeys(t, &schema, value)
	for _, bad := range []string{
		strings.Replace(string(canonical), `"schema_version"`, `"schema"`, 1),
		strings.Replace(string(canonical), `"model":`, `"context_id":"misplaced","model":`, 1),
	} {
		if _, err := Unmarshal([]byte(bad)); err == nil {
			t.Fatal("closed parser was weakened by schema exposure")
		}
	}
}

func assertSerializedKeys(t *testing.T, schema *WireSchema, raw json.RawMessage) {
	t.Helper()
	if string(raw) == "null" {
		return
	}
	if slices.Contains(schema.Type, "object") {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		for name, value := range object {
			child, found := schema.Properties[name]
			if !found {
				t.Fatalf("canonical serializer emits a key missing from its schema: %s", name)
			}
			assertSerializedKeys(t, child, value)
		}
		for _, required := range schema.Required {
			if _, found := object[required]; !found {
				t.Fatalf("schema requires a key omitted by canonical serialization: %s", required)
			}
		}
	} else if slices.Contains(schema.Type, "array") {
		var array []json.RawMessage
		if err := json.Unmarshal(raw, &array); err != nil {
			t.Fatal(err)
		}
		for _, item := range array {
			assertSerializedKeys(t, schema.Items, item)
		}
	}
}

func TestSchemaCannotGuessUnsupportedOrImplicitWireFields(t *testing.T) {
	for _, value := range []any{
		struct{ Field string }{},
		struct {
			Bad chan int `json:"bad"`
		}{},
		[]chan int{},
		new(chan int),
	} {
		if _, err := schemaFor(reflect.TypeOf(value)); err == nil {
			t.Fatalf("unsupported wire shape was guessed: %T", value)
		}
	}
	if schema, err := schemaFor(reflect.TypeOf(struct {
		Ignored string `json:"-"`
	}{})); err != nil || len(schema.Properties) != 0 {
		t.Fatalf("JSON-ignored field leaked into schema: %v", err)
	}
}
