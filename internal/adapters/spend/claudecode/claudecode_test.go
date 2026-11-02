package claudecode_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/claudecode"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
)

// -- fixture builders, matching Claude Code's own transcript shape as
// captured from a real ~/.claude/projects/*/*.jsonl session: a top-level
// envelope (type, isSidechain, message) wrapping a message whose content
// is an array of typed blocks, with per-turn usage on assistant messages. --

type usage struct{ input, output, cacheCreation, cacheRead int64 }

func line(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func assistantLine(t *testing.T, model string, u *usage, sidechain bool, blocks ...map[string]any) string {
	t.Helper()
	if len(blocks) == 0 {
		blocks = []map[string]any{{"type": "text", "text": "ok"}}
	}
	msg := map[string]any{"model": model, "id": "msg_x", "type": "message", "role": "assistant", "content": blocks}
	if u != nil {
		msg["usage"] = map[string]any{
			"input_tokens":                u.input,
			"output_tokens":               u.output,
			"cache_creation_input_tokens": u.cacheCreation,
			"cache_read_input_tokens":     u.cacheRead,
		}
	}
	return line(t, map[string]any{
		"type": "assistant", "isSidechain": sidechain, "message": msg,
		"uuid": "u", "timestamp": "2026-07-23T00:00:00Z",
	})
}

func userLine(t *testing.T, text string) string {
	t.Helper()
	return line(t, map[string]any{
		"type": "user", "isSidechain": false,
		"message": map[string]any{"role": "user", "content": text},
		"uuid":    "u", "timestamp": "2026-07-23T00:00:00Z",
	})
}

func skillBlock(skill, args string) map[string]any {
	return map[string]any{"type": "tool_use", "id": "t1", "name": "Skill", "input": map[string]any{"skill": skill, "args": args}}
}

func writeBlock(path, content string) map[string]any {
	return map[string]any{"type": "tool_use", "id": "t2", "name": "Write", "input": map[string]any{"file_path": path, "content": content}}
}

func editBlock(path, oldStr, newStr string) map[string]any {
	return map[string]any{"type": "tool_use", "id": "t3", "name": "Edit", "input": map[string]any{"file_path": path, "old_string": oldStr, "new_string": newStr}}
}

func bashBlock(command string) map[string]any {
	return map[string]any{"type": "tool_use", "id": "t4", "name": "Bash", "input": map[string]any{"command": command}}
}

func transcript(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func mustParse(t *testing.T, raw string) *claudecode.Adapter {
	t.Helper()
	a, err := claudecode.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return a
}

// Acceptance (T3): "Running a real Claude Code phase yields a record whose
// per-model tokens equal Claude Code's own accounting for that bracket."
// The three turns' usage is summed by hand here from the SAME numbers fed
// into the fixture — this is exactly Claude Code's own accounting for the
// bracket, and the adapter's output must equal it.
func TestSpend_ReconcilesToClaudeCodesOwnAccounting(t *testing.T) {
	open := usage{input: 100, output: 20, cacheCreation: 5, cacheRead: 50}
	mid := usage{input: 200, output: 40}
	closeU := usage{input: 10, output: 5}
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &open, false, skillBlock("prd-new", "synthesize the PRD")),
		assistantLine(t, "claude-sonnet-5", &mid, false),
		assistantLine(t, "claude-sonnet-5", &closeU, false, writeBlock("/repo/docs/prds/x/000-base.md", "# PRD")),
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

	gotSpend, unavail := a.Spend(p)
	if len(unavail) != 0 {
		t.Fatalf("want no Unavailable, got %+v", unavail)
	}
	want := record.TokenQuartet{
		Input:         open.input + mid.input + closeU.input,
		Output:        open.output + mid.output + closeU.output,
		CacheCreation: open.cacheCreation,
		CacheRead:     open.cacheRead,
	}
	ms, ok := gotSpend["claude-sonnet-5"]
	if !ok {
		t.Fatalf("want a claude-sonnet-5 entry, got %+v", gotSpend)
	}
	if ms.TokenQuartet != want {
		t.Errorf("token quartet mismatch:\nwant %+v\ngot  %+v", want, ms.TokenQuartet)
	}
	if len(gotSpend) != 1 {
		t.Errorf("want exactly 1 model, got %d: %+v", len(gotSpend), gotSpend)
	}
}

// Real feature-brainstorm/prd-review runs edit their own record many times
// before finishing — a captured real transcript showed 13 consecutive
// edits to one brainstorm doc. Every one of those turns' usage must be
// captured, not just the first (which would prematurely close the phase
// and silently lose the rest).
func TestSpend_MultiEditCoalescing_NoUsageLostToPrematureClose(t *testing.T) {
	u1 := usage{input: 1, output: 1}
	u2 := usage{input: 10, output: 2}
	u3 := usage{input: 20, output: 3}
	u4 := usage{input: 30, output: 4}
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &u1, false, skillBlock("feature-brainstorm", "…")),
		assistantLine(t, "claude-sonnet-5", &u2, false, editBlock("/repo/docs/brainstorms/x.md", "a", "b")),
		assistantLine(t, "claude-sonnet-5", &u3, false, editBlock("/repo/docs/brainstorms/x.md", "b", "c")),
		assistantLine(t, "claude-sonnet-5", &u4, false, editBlock("/repo/docs/brainstorms/x.md", "c", "d")),
	)
	a := mustParse(t, tr)

	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase (all 4 turns belong to one brainstorm phase), got %d: %+v", len(phases), phases)
	}
	p := phases[0]
	if !p.Complete {
		t.Fatalf("want the last edit to close the phase, got incomplete: %+v", p)
	}
	if len(p.Turns) != 4 {
		t.Fatalf("want all 4 turns attributed to the one phase, got %d", len(p.Turns))
	}

	gotSpend, unavail := a.Spend(p)
	if len(unavail) != 0 {
		t.Fatalf("want no Unavailable, got %+v", unavail)
	}
	want := record.TokenQuartet{
		Input:  u1.input + u2.input + u3.input + u4.input,
		Output: u1.output + u2.output + u3.output + u4.output,
	}
	if got := gotSpend["claude-sonnet-5"].TokenQuartet; got != want {
		t.Errorf("coalescing lost usage: want %+v, got %+v", want, got)
	}
}

// A docs/wiki/ edit touched in passing (e.g. a glossary term fixed
// mid-review) must never be mistaken for the close — AGENTS.md documents
// docs/wiki/ as freely-editable reference material, never a phase's own
// frozen artifact — while the phase's real, later artifact write still
// correctly closes it and every turn's usage is still captured.
func TestSpend_WikiSideEdit_IgnoredForBoundary(t *testing.T) {
	uOpen := usage{input: 5, output: 1}
	uDraft := usage{input: 6, output: 1}
	uSide := usage{input: 999, output: 999} // would wrongly close the phase early if mistaken for the artifact
	uFinal := usage{input: 7, output: 2}
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &uOpen, false, skillBlock("prd-review", "…")),
		assistantLine(t, "claude-sonnet-5", &uDraft, false, editBlock("/repo/docs/reviews/x-000.md", "a", "b")),
		assistantLine(t, "claude-sonnet-5", &uSide, false, editBlock("/repo/docs/wiki/glossary.md", "a", "b")),
		assistantLine(t, "claude-sonnet-5", &uFinal, false, writeBlock("/repo/docs/reviews/x-000.md", "verdict")),
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

	gotSpend, unavail := a.Spend(p)
	if len(unavail) != 0 {
		t.Fatalf("want no Unavailable — every turn still carried usage, got %+v", unavail)
	}
	want := record.TokenQuartet{
		Input:  uOpen.input + uDraft.input + uSide.input + uFinal.input,
		Output: uOpen.output + uDraft.output + uSide.output + uFinal.output,
	}
	if got := gotSpend["claude-sonnet-5"].TokenQuartet; got != want {
		t.Errorf("want every turn's usage summed regardless of the wiki side-edit: want %+v, got %+v", want, got)
	}
}

// The wiki exclusion is narrow — it never suppresses a REAL close: a
// phase whose entire artifact genuinely lives under docs/wiki/ (there is
// no such skill today, but the rule must not silently swallow one that
// existed) still closes on nothing, i.e. this pins that a non-wiki write
// is required, not that wiki writes are invisible to parsing entirely.
func TestParse_WikiOnlyWrite_NeverCloses(t *testing.T) {
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("prd-new", "…")),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, editBlock("/repo/docs/wiki/glossary.md", "a", "b")),
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

// Acceptance (DINV-3): a phase with no capturable usage at all — every
// turn's usage was absent — must declare Unavailable, never a silent
// zero.
func TestSpend_NoUsage_DeclaresUnavailable(t *testing.T) {
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", nil, false, skillBlock("tech-design-new", "…")),
		assistantLine(t, "claude-sonnet-5", nil, false),
		assistantLine(t, "claude-sonnet-5", nil, false, writeBlock("/repo/docs/tech-design/x.md", "…")),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}

	gotSpend, unavail := a.Spend(phases[0])
	if gotSpend != nil {
		t.Fatalf("want a nil spend map, got %+v", gotSpend)
	}
	want := []record.Unavailable{{Harness: claudecode.HarnessID, MissingFigure: "per-model spend"}}
	if len(unavail) != 1 || unavail[0] != want[0] {
		t.Errorf("want Unavailable %+v, got %+v", want, unavail)
	}
}

// A phase that uses more than one model must carry each as its own entry,
// never merged into one.
func TestSpend_MultiModel_KeptSeparate(t *testing.T) {
	sonnet := usage{input: 100, output: 10}
	haiku := usage{input: 5, output: 1}
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &sonnet, false, skillBlock("plan-new", "…")),
		assistantLine(t, "claude-haiku-4-5", &haiku, false),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, writeBlock("/repo/docs/plans/x/001-plan.md", "…")),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	gotSpend, _ := a.Spend(phases[0])

	if len(gotSpend) != 2 {
		t.Fatalf("want 2 distinct models, got %d: %+v", len(gotSpend), gotSpend)
	}
	if gotSpend["claude-sonnet-5"].Input != sonnet.input+1 {
		t.Errorf("claude-sonnet-5 input: want %d, got %d", sonnet.input+1, gotSpend["claude-sonnet-5"].Input)
	}
	if gotSpend["claude-haiku-4-5"].Input != haiku.input {
		t.Errorf("claude-haiku-4-5 input: want %d, got %d", haiku.input, gotSpend["claude-haiku-4-5"].Input)
	}
}

// A write outside any docs/ directory (e.g. this feature's own Go source)
// must never be mistaken for the artifact-write close.
func TestParse_NonDocsWrite_NeverCloses(t *testing.T) {
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("implement-now", "…")),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, writeBlock("/repo/internal/phasemeter/spend/spend.go", "package spend")),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, bashBlock("gh pr create --title x")),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a non-docs write (or a Bash gh pr create) must not close the phase — want incomplete, got complete")
	}
}

// A sub-agent's own turns (isSidechain == true) run as a separate thread
// interleaved in file order, not conversation order; they must not be
// folded into the main bracket/usage stream.
func TestParse_SidechainTurns_Excluded(t *testing.T) {
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("prd-new", "…")),
		assistantLine(t, "claude-opus-4-8", &usage{input: 12345}, true), // sub-agent turn — must be excluded
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, writeBlock("/repo/docs/prds/x/000-base.md", "…")),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if len(phases[0].Turns) != 2 {
		t.Fatalf("want 2 top-level turns (the sidechain turn excluded), got %d", len(phases[0].Turns))
	}
	gotSpend, _ := a.Spend(phases[0])
	if _, ok := gotSpend["claude-opus-4-8"]; ok {
		t.Errorf("sidechain model must not appear in top-level spend, got %+v", gotSpend)
	}
	if gotSpend["claude-sonnet-5"].Input != 2 {
		t.Errorf("want claude-sonnet-5 input 2 (sidechain excluded), got %d", gotSpend["claude-sonnet-5"].Input)
	}
}

// A bare "user" line (message.content is a plain string, not an array of
// blocks — a different shape than an assistant message) must parse
// without error and contribute no turn.
func TestParse_UserLines_SkippedWithoutError(t *testing.T) {
	tr := transcript(
		userLine(t, "please start"),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("prd-new", "…")),
		userLine(t, "<local-command-caveat>…</local-command-caveat>"),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, writeBlock("/repo/docs/prds/x/000-base.md", "…")),
	)
	a := mustParse(t, tr)
	if len(a.Turns()) != 2 {
		t.Fatalf("want 2 turns (user lines contribute none), got %d", len(a.Turns()))
	}
}

// Blank lines between JSONL records must be tolerated.
func TestParse_BlankLines_Tolerated(t *testing.T) {
	tr := "\n" + assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("prd-new", "…")) + "\n\n" +
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, writeBlock("/repo/docs/prds/x/000-base.md", "…")) + "\n\n"
	a := mustParse(t, tr)
	if len(a.Turns()) != 2 {
		t.Fatalf("want 2 turns, got %d", len(a.Turns()))
	}
}

// A malformed line is a fail-closed error — this is the harness's own
// accounting, not user input to tolerate past.
func TestParse_MalformedLine_FailsClosed(t *testing.T) {
	tr := transcript(
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false, skillBlock("prd-new", "…")),
		`{"type":"assistant","message":{`, // truncated JSON
	)
	if _, err := claudecode.Parse(strings.NewReader(tr)); err == nil {
		t.Fatal("want an error for a malformed transcript line, got nil")
	}
}

// Acceptance (T3, DINV-8): a seeded secret anywhere in the transcript —
// prompt text, thinking content, or a tool's own input/output beyond the
// narrow fields this adapter reads — must never reach a written record.
func TestSpend_SeededSecret_NeverInAnyRecord(t *testing.T) {
	const secret = "sk-SEEDED-SECRET-DO-NOT-LEAK-4f8a"
	tr := transcript(
		userLine(t, "here is my key: "+secret),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false,
			map[string]any{"type": "thinking", "thinking": "the user's secret is " + secret},
			skillBlock("prd-new", "use "+secret+" if needed")),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false,
			editBlock("/repo/docs/prds/x/000-base.md", secret, "redacted"),
		),
		assistantLine(t, "claude-sonnet-5", &usage{input: 1}, false,
			map[string]any{"type": "text", "text": "done, the secret was " + secret},
			writeBlock("/repo/docs/prds/x/000-base.md", "contains "+secret),
		),
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

func TestHarness(t *testing.T) {
	a := mustParse(t, "")
	if got := a.Harness(); got != "claude-code" {
		t.Errorf("Harness() = %q, want claude-code", got)
	}
}

// An empty transcript parses to zero turns and zero phases — never an
// error.
func TestParse_Empty(t *testing.T) {
	a := mustParse(t, "")
	if len(a.Turns()) != 0 {
		t.Fatalf("want 0 turns, got %d", len(a.Turns()))
	}
}
