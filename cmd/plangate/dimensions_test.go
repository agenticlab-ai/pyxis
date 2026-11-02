package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

func TestDimensionsCLIAndOutputFailure(t *testing.T) {
	code, out, _ := captureProcessOutput(t, func() int { return run([]string{"dimensions"}) })
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("dimensions: %d %s", code, out)
	}
	code, _, _ = captureProcessOutput(t, func() int { return run([]string{"dimensions", "unexpected"}) })
	if code != 2 {
		t.Fatal("accepted extra arguments")
	}
	code, _, _ = captureProcessOutput(t, func() int {
		if err := os.Stdout.Close(); err != nil {
			t.Fatal(err)
		}
		return run([]string{"dimensions"})
	})
	if code != 1 {
		t.Fatal("reported success after output failed")
	}
}

type dimensionOutputFailure struct{}

func (dimensionOutputFailure) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func TestDimensionsExposeTypedValuesAndPolicyWithoutHarnesses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	if err := printDimensions(&out); err != nil {
		t.Fatal(err)
	}
	var got taskrequirements.DimensionValues
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, taskrequirements.Values()) {
		t.Fatalf("lost dimension values: %+v", got)
	}
	if err := printDimensions(dimensionOutputFailure{}); err == nil {
		t.Fatal("lost output error")
	}
}
