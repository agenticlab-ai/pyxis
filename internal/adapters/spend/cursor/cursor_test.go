package cursor_test

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/cursor"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
)

// -- fixture builders, matching cursor-agent's own stream-json shape as
// documented (cursor.com/docs/cli/reference/output-format) and as
// captured in third-party write-ups of real output: one JSON object per
// line, a "type"/"subtype" discriminator, "user"/"assistant" messages
// carrying an array of typed content blocks, and "tool_call" events
// carrying a *ToolCall-keyed payload (writeToolCall/editToolCall/…, each
// with "args" and, once "completed", a "result"). --

func line(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func userLine(t *testing.T, text string) string {
	t.Helper()
	return line(t, map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]any{{"type": "text", "text": text}},
		},
		"session_id": "sid",
	})
}

func assistantLine(t *testing.T, text string) string {
	t.Helper()
	return line(t, map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": text}},
		},
		"session_id": "sid",
	})
}

// toolCallLine builds one tool_call event. success is only rendered when
// subtype is "completed" (a "started" event carries no result yet, the
// same as cursor-agent's own real shape). extraArgs seeds additional
// fields onto "args" — e.g. a write's own file content — that this
// package's adapter must never retain (DINV-8).
func toolCallLine(t *testing.T, subtype, tool, path string, success bool, extraArgs map[string]any) string {
	t.Helper()
	args := map[string]any{"path": path, "toolCallId": "tc1"}
	maps.Copy(args, extraArgs)
	call := map[string]any{"args": args}
	if subtype == "completed" {
		call["result"] = map[string]any{"success": success}
	}
	return line(t, map[string]any{
		"type":      "tool_call",
		"subtype":   subtype,
		"tool_call": map[string]any{tool: call},
	})
}

func resultLine(t *testing.T) string {
	t.Helper()
	return line(t, map[string]any{
		"type": "result", "subtype": "success", "is_error": false,
		"duration_ms": 1234, "duration_api_ms": 1000, "result": "done", "session_id": "sid",
	})
}

func transcript(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func mustParse(t *testing.T, raw string) *cursor.Adapter {
	t.Helper()
	a, err := cursor.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return a
}

// A slash-command user message opens a phase; a completed, successful
// write to a docs/ path closes it.
func TestPhase_OpensOnSlashCommand_ClosesOnDocsWrite(t *testing.T) {
	tr := transcript(
		userLine(t, "/prd-new synthesize the PRD"),
		assistantLine(t, "working on it"),
		toolCallLine(t, "started", "writeToolCall", "/repo/docs/prds/x/000-base.md", false, nil),
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/prds/x/000-base.md", true, nil),
		resultLine(t),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d: %+v", len(phases), phases)
	}
	p := phases[0]
	if p.ID != "prd-new" || !p.Complete {
		t.Fatalf("want a complete prd-new phase, got %+v", p)
	}
	if p.Harness != cursor.HarnessID {
		t.Errorf("want harness %q, got %q", cursor.HarnessID, p.Harness)
	}
}

// Acceptance (T7): "Where Cursor exposes no per-model spend on the user's
// key, the adapter declares unavailable, never a fabricated figure" — this
// must hold even over a phase with real, varied activity (a read, several
// assistant turns, a successful write), not just an empty one.
func TestSpend_AlwaysUnavailable_EvenWithActivity(t *testing.T) {
	tr := transcript(
		userLine(t, "/tech-design-new draft the design"),
		assistantLine(t, "let me look at the repo first"),
		toolCallLine(t, "started", "readToolCall", "/repo/docs/architecture-principles.md", false, nil),
		toolCallLine(t, "completed", "readToolCall", "/repo/docs/architecture-principles.md", true, nil),
		assistantLine(t, "drafting now"),
		toolCallLine(t, "started", "writeToolCall", "/repo/docs/tech-design/x.md", false, nil),
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/tech-design/x.md", true, nil),
		resultLine(t),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d: %+v", len(phases), phases)
	}

	gotSpend, unavail := a.Spend(phases[0])
	if gotSpend != nil {
		t.Fatalf("want a nil spend map — Cursor exposes no per-model figure — got %+v", gotSpend)
	}
	want := []record.Unavailable{{Harness: cursor.HarnessID, MissingFigure: "per-model spend"}}
	if len(unavail) != 1 || unavail[0] != want[0] {
		t.Errorf("want Unavailable %+v, got %+v", want, unavail)
	}
}

// Several drafts of the same artifact before the true close are common —
// only the LAST qualifying write closes the phase, mirroring the Claude
// Code adapter's own multi-draft coalescing.
func TestPhase_MultiEditCoalescing_LastWriteWins(t *testing.T) {
	tr := transcript(
		userLine(t, "/feature-brainstorm let's think this through"),
		toolCallLine(t, "started", "editToolCall", "/repo/docs/brainstorms/x.md", false, nil),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/brainstorms/x.md", true, nil),
		toolCallLine(t, "started", "editToolCall", "/repo/docs/brainstorms/x.md", false, nil),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/brainstorms/x.md", true, nil),
		toolCallLine(t, "started", "editToolCall", "/repo/docs/brainstorms/x.md", false, nil),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/brainstorms/x.md", true, nil),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase (all edits belong to one brainstorm phase), got %d: %+v", len(phases), phases)
	}
	p := phases[0]
	if !p.Complete {
		t.Fatalf("want the last edit to close the phase, got incomplete: %+v", p)
	}
	// 1 open (user) + 3 "completed" edit events; each paired "started"
	// event contributes no turn of its own (see
	// TestParse_StartedSubtypeAlone_NeverContributesATurn).
	if len(p.Turns) != 4 {
		t.Fatalf("want all 4 turns attributed to the one phase, got %d", len(p.Turns))
	}
}

// A docs/wiki/ edit touched in passing must never be mistaken for the
// close — AGENTS.md documents docs/wiki/ as freely-editable reference
// material, never a phase's own frozen artifact — while the phase's real,
// later artifact write still correctly closes it and every turn is still
// attributed.
func TestPhase_WikiSideEdit_IgnoredForBoundary(t *testing.T) {
	tr := transcript(
		userLine(t, "/prd-review review this PRD"),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/reviews/x-000.md", true, nil),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/wiki/glossary.md", true, nil),  // side edit — must not close
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/reviews/x-000.md", true, nil), // the real close
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d: %+v", len(phases), phases)
	}
	p := phases[0]
	if len(p.Turns) != 4 {
		t.Fatalf("want all 4 turns attributed (nothing dropped), got %d", len(p.Turns))
	}
	if !p.Complete {
		t.Fatalf("want the final reviews/x-000.md write to close the phase, got incomplete")
	}
}

// The wiki exclusion is narrow — it never suppresses a REAL close: a
// phase whose only write lives entirely under docs/wiki/ still closes on
// nothing.
func TestParse_WikiOnlyWrite_NeverCloses(t *testing.T) {
	tr := transcript(
		userLine(t, "/prd-new synthesize the PRD"),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/wiki/glossary.md", true, nil),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a wiki-only write must never close a phase, got complete")
	}
}

// A write outside any docs/ directory (e.g. this feature's own Go source)
// — or a shell call, e.g. `gh pr create` — must never be mistaken for the
// artifact-write close.
func TestParse_NonDocsWrite_NeverCloses(t *testing.T) {
	tr := transcript(
		userLine(t, "/implement-now build it"),
		toolCallLine(t, "completed", "writeToolCall", "/repo/internal/phasemeter/spend/spend.go", true, nil),
		toolCallLine(t, "completed", "shellToolCall", "n/a", true, map[string]any{"command": "gh pr create --title x"}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a non-docs write (or a shell gh pr create) must not close the phase — want incomplete, got complete")
	}
}

// A write that completes unsuccessfully produced no frozen artifact and
// must never close the phase.
func TestParse_FailedWrite_NeverCloses(t *testing.T) {
	tr := transcript(
		userLine(t, "/plan-new plan it"),
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/plans/x/001-plan.md", false, nil),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a failed write must never close a phase, got complete")
	}
}

// A bare "started" tool_call event, never followed by "completed" in the
// captured stream, contributes no turn of its own (it carries no path and
// no result — nothing this adapter can act on).
func TestParse_StartedSubtypeAlone_NeverContributesATurn(t *testing.T) {
	tr := transcript(
		userLine(t, "/plan-new plan it"),
		toolCallLine(t, "started", "writeToolCall", "/repo/docs/plans/x/001-plan.md", false, nil),
	)
	a := mustParse(t, tr)
	if len(a.Turns()) != 1 {
		t.Fatalf("want 1 turn (the open marker only), got %d", len(a.Turns()))
	}
}

// Ordinary conversation — no leading slash-command — never opens a phase.
func TestParse_NoSlashCommand_NoPhaseOpened(t *testing.T) {
	tr := transcript(
		userLine(t, "what does this function do?"),
		assistantLine(t, "let me explain..."),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 0 {
		t.Fatalf("want 0 phases (no slash command ever appeared), got %d: %+v", len(phases), phases)
	}
}

// Blank lines between JSONL records must be tolerated.
func TestParse_BlankLines_Tolerated(t *testing.T) {
	tr := "\n" + userLine(t, "/prd-new go") + "\n\n" +
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/prds/x/000-base.md", true, nil) + "\n\n"
	a := mustParse(t, tr)
	if len(a.Turns()) != 2 {
		t.Fatalf("want 2 turns, got %d", len(a.Turns()))
	}
}

// A malformed line is a fail-closed error — this is the harness's own
// accounting, not user input to tolerate past.
func TestParse_MalformedLine_FailsClosed(t *testing.T) {
	tr := transcript(
		userLine(t, "/prd-new synthesize the PRD"),
		`{"type":"user","message":{`, // truncated JSON
	)
	if _, err := cursor.Parse(strings.NewReader(tr)); err == nil {
		t.Fatal("want an error for a malformed stream-json line, got nil")
	}
}

// An empty stream parses to zero turns — never an error.
func TestParse_Empty(t *testing.T) {
	a := mustParse(t, "")
	if len(a.Turns()) != 0 {
		t.Fatalf("want 0 turns, got %d", len(a.Turns()))
	}
}

func TestHarness(t *testing.T) {
	a := mustParse(t, "")
	if got := a.Harness(); got != "cursor" {
		t.Errorf("Harness() = %q, want cursor", got)
	}
}

// Acceptance (T7, DINV-8): a seeded secret anywhere in the stream — the
// user's own prompt text past the slash-command, an assistant's text, or
// a tool call's own file content — must never reach a written record.
func TestSpend_SeededSecret_NeverInAnyRecord(t *testing.T) {
	const secret = "sk-SEEDED-SECRET-DO-NOT-LEAK-4f8a"
	tr := transcript(
		userLine(t, "/prd-new here is my key: "+secret),
		assistantLine(t, "got it, using "+secret+" now"),
		toolCallLine(t, "completed", "editToolCall", "/repo/docs/prds/x/000-base.md", true,
			map[string]any{"fileText": "contains " + secret}),
		toolCallLine(t, "completed", "writeToolCall", "/repo/docs/prds/x/000-base.md", true,
			map[string]any{"fileText": "final, has " + secret}),
	)
	a := mustParse(t, tr)
	recs := spend.Records(a, "run-1", "pipeline-measurement")
	if len(recs) == 0 {
		t.Fatal("want at least 1 record to check")
	}
	b, err := json.Marshal(recs)
	if err != nil {
		t.Fatalf("marshal records: %v", err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatalf("seeded secret leaked into a written record:\n%s", b)
	}
}
