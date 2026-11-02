package main

import (
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/plangate/project"
)

func TestMachineOutputFailureRefusesSuccess(t *testing.T) {
	closed, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		run        func() int
	}{
		{"projection report", "write report", func() int { return printReport(&project.Report{Schema: project.ReportSchemaVersion}) }},
		{"JSON defects", "file already closed", func() int { return emit([]schema.Defect{{Reason: "test defect"}}, "-") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := captureProcessOutput(t, func() int {
				previous := os.Stdout
				os.Stdout = closed
				defer func() { os.Stdout = previous }()
				return tc.run()
			})
			if code != 2 || !strings.Contains(stderr, tc.want) {
				t.Fatalf("lost machine output reported success: code=%d, stderr=%s", code, stderr)
			}
		})
	}
}
