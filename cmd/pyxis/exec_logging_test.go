package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

func TestExecJournalRecordsHandoffBeforeExecutionWithoutArguments(t *testing.T) {
	for _, mode := range []string{"binary", "source"} {
		t.Run(mode, func(t *testing.T) {
			root := tempRepo(t)
			checkHandoff := func() {
				t.Helper()
				paths, err := filepath.Glob(filepath.Join(root, ".pyxis", "logs", "cli", "exec-docgate-*.jsonl"))
				if err != nil || len(paths) != 1 {
					t.Fatalf("journals=%v err=%v", paths, err)
				}
				data, err := os.ReadFile(paths[0])
				if err != nil {
					t.Fatal(err)
				}
				if bytes.Contains(data, []byte("secret-argument")) {
					t.Fatal("raw tool arguments leaked into the journal")
				}
				found := false
				for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
					var record struct {
						Message string `json:"msg"`
						Tool    string `json:"tool"`
						Mode    string `json:"mode"`
					}
					if err := json.Unmarshal([]byte(line), &record); err != nil {
						t.Fatalf("journal is empty or invalid: %v", err)
					}
					found = found || record.Message == "tool_delegated" && record.Tool == "docgate" && record.Mode == mode
				}
				if !found {
					t.Fatalf("handoff was not recorded before execution: %s", data)
				}
			}
			if mode == "binary" {
				swap(t, &resolveTool, (&resolveRec{path: "/fake/docgate"}).call)
				swap(t, &execFn, func(string, []string, []string) (int, error) { checkHandoff(); return 7, nil })
			} else {
				swap(t, &resolveTool, (&resolveRec{err: &resolve.SourceMode{Tool: "docgate", RepoRoot: root}}).call)
				swap(t, &goRunFn, func(*resolve.SourceMode, []string) (int, error) { checkHandoff(); return 7, nil })
			}
			var out, diagnostic bytes.Buffer
			if code := run([]string{"exec", "docgate", "--", "secret-argument"}, &out, &diagnostic); code != 7 {
				t.Fatalf("exit=%d: %s", code, &diagnostic)
			}
		})
	}
}

func TestExecJournalRecordsResolutionExecutionAndReturnOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		resolveErr    error
		executionErr  error
		code          int
	}{
		{"resolution refused", "tool_resolution_failed", errors.New("resolution denied"), nil, 1},
		{"binary launch failed", "tool_execution_failed", nil, errors.New("launch denied"), 1},
		{"source launch failed", "tool_execution_failed", &resolve.SourceMode{Tool: "docgate"}, errors.New("build denied"), 1},
		{"binary returned", "tool_finished", nil, nil, 7},
		{"source returned", "tool_finished", &resolve.SourceMode{Tool: "docgate"}, nil, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tempRepo(t)
			swap(t, &resolveTool, (&resolveRec{path: "/fake/tool", err: tc.resolveErr}).call)
			swap(t, &execFn, (&execRec{code: tc.code, err: tc.executionErr}).call)
			swap(t, &goRunFn, (&goRunRec{code: tc.code, err: tc.executionErr}).call)
			var out, diagnostic bytes.Buffer
			if code := run([]string{"exec", "docgate", "--"}, &out, &diagnostic); code != tc.code {
				t.Fatalf("exit=%d, want %d: %s", code, tc.code, &diagnostic)
			}
			paths, err := filepath.Glob(filepath.Join(root, ".pyxis", "logs", "cli", "*.jsonl"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("journals=%v err=%v", paths, err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(data, []byte(`"msg":"`+tc.message+`"`)) || !bytes.Contains(data, []byte(`"tool":"docgate"`)) {
				t.Fatalf("missing tool outcome: %s", data)
			}
		})
	}
}
