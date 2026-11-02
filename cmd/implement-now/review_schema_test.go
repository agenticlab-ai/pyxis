package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type schemaBrokenOutput struct{}

func (schemaBrokenOutput) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestSchemaHelperNeedsNoSourceRepositoryOrCredential(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, diagnostic bytes.Buffer
	if code := run([]string{"--internal-review-schema"}, &out, &diagnostic); code != 0 || diagnostic.Len() != 0 {
		t.Fatalf("installed helper required repo/credential configuration: %d %s", code, diagnostic.String())
	}
	if !strings.Contains(out.String(), `"schema_version"`) || !strings.Contains(out.String(), `"additionalProperties": false`) || !strings.Contains(out.String(), `"impl-review-report/v1"`) {
		t.Fatal("helper omitted the compiled closed report shape")
	}
	if code := run([]string{"--internal-review-schema", "--repo", "operand"}, io.Discard, &diagnostic); code != 2 {
		t.Fatal("unexpected schema operands were silently accepted")
	}
	if code := run([]string{"--internal-review-schema"}, schemaBrokenOutput{}, &diagnostic); code != 1 {
		t.Fatal("failed schema output was reported as complete")
	}
	previous := compileReviewSchema
	t.Cleanup(func() { compileReviewSchema = previous })
	compileReviewSchema = func() ([]byte, error) { return nil, errors.New("unsupported compiled payload") }
	if code := run([]string{"--internal-review-schema"}, io.Discard, &diagnostic); code != 1 {
		t.Fatal("schema compilation error silently passed")
	}
}

func TestSchemaHelperHandlesFailedDiagnostics(t *testing.T) {
	previous := compileReviewSchema
	t.Cleanup(func() { compileReviewSchema = previous })
	for _, tc := range []struct {
		name       string
		args       []string
		stdout     io.Writer
		compileErr bool
	}{
		{name: "invalid operands", args: []string{"operand"}, stdout: io.Discard},
		{name: "compiler failure", stdout: io.Discard, compileErr: true},
		{name: "output failure", stdout: schemaBrokenOutput{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compileReviewSchema = previous
			if tc.compileErr {
				compileReviewSchema = func() ([]byte, error) { return nil, errors.New("unsupported payload") }
			}
			if code := reviewSchemaTool(tc.args, tc.stdout, schemaBrokenOutput{}); code != 1 {
				t.Fatalf("failed diagnostic output returned %d", code)
			}
		})
	}
}
