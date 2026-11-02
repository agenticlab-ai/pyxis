package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The T10 defect-quadruple sweep at the CLI boundary — every file-implicating
// failure the operator can hit surfaces a Defect naming the full quadruple:
// file, operation, reason, and remedy (docs/tech-design/local-work-tracker.md
// §3.9, §3.13; prd:local-work-tracker/UB-1). The conformance kit proves this
// over the port (proof.go); this proves the same contract survives the CLI's
// --json rendering — what a scripting caller actually parses off stderr.

func TestDefectQuadrupleAtCLI(t *testing.T) {
	root := t.TempDir()
	// Seed a couple of real items so cycle/corruption cases have subjects.
	mustCarina(t, "create", "--root", root, "--title", "first")  // #1
	mustCarina(t, "create", "--root", root, "--title", "second") // #2
	mustCarina(t, "dep", "add", "--root", root, "2", "1")        // #2 blocked_by #1

	cases := []struct {
		name string
		args []string
	}{
		{"MissingItem", []string{"show", "--root", root, "999999"}},
		{"UndefinedSignal", []string{"signal", "add", "--root", root, "1", "no-such-signal"}},
		{"DependencyCycle", []string{"dep", "add", "--root", root, "1", "2"}}, // closes 1→2→1
		{"DanglingDependency", []string{"dep", "add", "--root", root, "1", "888888"}},
		{"MalformedItem", nil}, // handled specially below (corrupts a file first)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.name == "MalformedItem" {
				// Corrupt #1's file on disk, then read it through the CLI.
				bad := filepath.Join(root, ".workitems", "items", "1.md")
				if err := os.WriteFile(bad, []byte("not a work item\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				args = []string{"show", "--root", root, "1"}
			}
			// --json so a scripting caller gets machine-parseable defects on stderr.
			code, _, stderr := carina(append([]string{args[0], "--json"}, args[1:]...)...)
			if code == 0 {
				t.Fatalf("%s did not fail (exit 0); stderr: %q", tc.name, stderr)
			}
			defects := parseJSONDefects(t, stderr)
			if len(defects) == 0 {
				t.Fatalf("%s emitted no JSON defect on stderr: %q", tc.name, stderr)
			}
			for _, d := range defects {
				if d.File == "" || d.Op == "" || d.Reason == "" || d.Remedy == "" {
					t.Fatalf("%s CLI defect omits part of the quadruple (file/op/reason/remedy): %+v", tc.name, d)
				}
			}
		})
	}
}

// parseJSONDefects reads the CLI's --json defect stream — one JSON object per
// line on stderr — into typed defects.
func parseJSONDefects(t *testing.T, stderr string) []schema.Defect {
	t.Helper()
	var out []schema.Defect
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var d schema.Defect
		if err := json.Unmarshal([]byte(line), &d); err != nil {
			t.Fatalf("stderr line is not a JSON defect (%v): %q", err, line)
		}
		out = append(out, d)
	}
	return out
}
