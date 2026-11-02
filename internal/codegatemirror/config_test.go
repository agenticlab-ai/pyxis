package codegatemirror

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
)

func TestLoadAppliesLocalOverridesAndPreservesChecks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeF(t, root, configlayer.GlobalPath, `code-gate:
  build:
    command: go build ./...
  test-race:
    command: go test -race ./...
    timeout-s: 900
    env: CGO_ENABLED
`)
	writeF(t, root, configlayer.LocalPath, `code-gate:
  test-race:
    command: go test -race ./internal/codegatemirror
`)

	got, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []Check{
		{Name: "build", Command: "go build ./...", FailOn: FailOnExit, Class: ClassRequired},
		{Name: "test-race", Command: "go test -race ./internal/codegatemirror", FailOn: FailOnExit, Class: ClassRequired, TimeoutS: 900, Env: []string{"CGO_ENABLED"}},
	}
	if !reflect.DeepEqual(got.Checks, want) {
		t.Fatalf("Load checks =\n  %+v\nwant\n  %+v", got.Checks, want)
	}
}

func TestLoadRejectsInvalidConfigurationLayers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		path      string
		body      string
		directory bool
		want      string
		wantErr   error
	}{
		{
			name: "unreadable global file",
			path: configlayer.GlobalPath, directory: true,
			want: "code-gate: load configuration: read global config " + configlayer.GlobalPath,
		},
		{
			name: "unreadable local file",
			path: configlayer.LocalPath, directory: true,
			want: "code-gate: load configuration: read local config " + configlayer.LocalPath,
		},
		{
			name: "malformed local file",
			path: configlayer.LocalPath, body: "code-gate:\n   build:\n",
			want: "code-gate: load configuration: parse local config " + configlayer.LocalPath,
		},
		{
			name: "unknown local check field",
			path: configlayer.LocalPath, body: "code-gate:\n  build:\n    unknown: value\n",
			want: "unknown", wantErr: ErrUnknownKey,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tt.path != configlayer.GlobalPath {
				writeF(t, root, configlayer.GlobalPath, "code-gate:\n  build:\n    command: go build ./...\n")
			}
			if tt.directory {
				// A directory fails os.ReadFile even when tests run as root.
				if err := os.MkdirAll(filepath.Join(root, tt.path), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeF(t, root, tt.path, tt.body)
			}

			got, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want %q", err, tt.want)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Load error = %v, want %v", err, tt.wantErr)
			}
			if tt.directory {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) || pathErr.Path != filepath.Join(root, tt.path) {
					t.Errorf("Load error = %v, want wrapped file error for %s", err, tt.path)
				}
			}
			if len(got.Checks) != 0 {
				t.Errorf("Load returned checks after a configuration error: %+v", got.Checks)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    []Check
		wantErr error
	}{
		{
			name: "full block with every field and defaults",
			in: `code-gate:
  gofmt:
    command: gofmt -l .
    fail-on: output
  vet:
    command: go vet ./...
  test-race:
    command: go test -race ./...
    class: advisory
    timeout-s: 900
    env: CGO_ENABLED GOFLAGS
    phase: 2
`,
			want: []Check{
				{Name: "gofmt", Command: "gofmt -l .", FailOn: FailOnOutput, Class: ClassRequired},
				{Name: "vet", Command: "go vet ./...", FailOn: FailOnExit, Class: ClassRequired},
				{Name: "test-race", Command: "go test -race ./...", FailOn: FailOnExit, Class: ClassAdvisory, TimeoutS: 900, Env: []string{"CGO_ENABLED", "GOFLAGS"}, Phase: 2},
			},
		},
		{
			name: "sibling top-level blocks are skipped, not rejected",
			in: `schema_version: pyxis-config/v2
executor:
  gate:
    build: go build ./...
code-gate:
  build:
    command: go build ./...
paths:
  prds: docs/prds
`,
			want: []Check{{Name: "build", Command: "go build ./...", FailOn: FailOnExit, Class: ClassRequired}},
		},
		{
			name: "absent block is empty config",
			in: `executor:
  gate:
    build: go build ./...
`,
			want: nil,
		},
		{
			name:    "unknown field under a check",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n    bogus: x\n",
			wantErr: ErrUnknownKey,
		},
		{
			name:    "bad fail-on value",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n    fail-on: sometimes\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "bad class value",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n    class: maybe\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "negative timeout",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n    timeout-s: -3\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "phase below one",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n    phase: 0\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "check with no command",
			in:      "code-gate:\n  vet:\n    class: required\n",
			wantErr: ErrInvalidCheck,
		},
		{
			name:    "duplicate check name",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n  vet:\n    command: go vet ./...\n",
			wantErr: ErrInvalidCheck,
		},
		{
			name:    "check with a scalar value is rejected",
			in:      "code-gate:\n  vet: go vet ./...\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "code-gate as a scalar is rejected",
			in:      "code-gate: nope\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "duplicate code-gate block",
			in:      "code-gate:\n  a:\n    command: true\ncode-gate:\n  b:\n    command: true\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "tab indentation",
			in:      "code-gate:\n\tvet:\n",
			wantErr: ErrMalformed,
		},
		{
			name:    "over-deep indentation",
			in:      "code-gate:\n  vet:\n    command: go vet ./...\n      deep: x\n",
			wantErr: ErrMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse([]byte(tt.in))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Parse err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: unexpected err %v", err)
			}
			if !reflect.DeepEqual(got.Checks, tt.want) {
				t.Fatalf("Parse checks =\n  %+v\nwant\n  %+v", got.Checks, tt.want)
			}
		})
	}
}
