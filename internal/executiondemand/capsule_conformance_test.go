// This file supplies the local production-medium and presentation boundaries
// to the one shared capsule conformance harness. The GitHub adapter invokes
// the same harness from internal/plangate/tracker/capsule_conformance_test.go.
package executiondemand_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executiondemand/conformancetest"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func localWorktrackerRoundTrip(t *testing.T, body string) string {
	t.Helper()
	w := model.WorkItem{
		ID:      1,
		Type:    model.TypeTask,
		State:   model.StateOpen,
		Title:   "conformance fixture",
		Created: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Updated: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		Body:    body,
	}
	raw := localfs.Render(w)
	parsed, defects := localfs.Parse("show", "items/1.md", raw)
	if len(defects) != 0 {
		t.Fatalf("local worktracker round trip produced defects: %+v", defects)
	}
	return parsed.Body
}

func presentationSoftWrapRoundTrip(t *testing.T, body string) string {
	t.Helper()
	const markerPrefix = "<!-- pyxis:execution-demand "
	start := strings.Index(body, markerPrefix)
	if start < 0 {
		t.Fatal("no capsule marker to soft-wrap")
	}
	payloadStart := start + len(markerPrefix)
	closeOffset := strings.Index(body[payloadStart:], "-->")
	if closeOffset < 0 {
		t.Fatal("capsule marker has no close delimiter")
	}
	close := payloadStart + closeOffset
	payload := strings.TrimSpace(body[payloadStart:close])
	wrapped := softWrap(payload, 60)
	return body[:start] + fmt.Sprintf("<!-- pyxis:execution-demand\n%s\n-->", wrapped) + body[close+3:]
}

func softWrap(s string, width int) string {
	var lines []string
	for len(s) > width {
		lines = append(lines, s[:width])
		s = s[width:]
	}
	lines = append(lines, s)
	return strings.Join(lines, "\n")
}

func TestLocalWorktrackerRunsSharedCapsuleConformance(t *testing.T) {
	conformancetest.Run(t, "local_worktracker", localWorktrackerRoundTrip)
}

func TestPresentationNormalizationRunsSharedCapsuleConformance(t *testing.T) {
	conformancetest.Run(t, "presentation_soft_wrap", presentationSoftWrapRoundTrip)
}
