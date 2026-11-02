package opencode_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/opencode"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
)

// -- fixture builders, matching `opencode export`'s own JSON shape as
// verified against a live OpenCode installation (packages/opencode/src/
// session/message-v2.ts's zod schema, cross-checked against real exported
// session data): {"messages":[{"info":{...}, "parts":[...]}]}. --

type tokens struct {
	input, output, reasoning, cacheRead, cacheWrite int64
}

func doc(t *testing.T, messages ...map[string]any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"info":     map[string]any{"id": "ses_fixture"},
		"messages": messages,
	})
	if err != nil {
		t.Fatalf("marshal fixture doc: %v", err)
	}
	return string(b)
}

func assistantMsg(t *testing.T, modelID string, tk *tokens, parts ...map[string]any) map[string]any {
	t.Helper()
	info := map[string]any{
		"id": "msg_x", "role": "assistant", "providerID": "anthropic", "modelID": modelID,
	}
	if tk != nil {
		info["tokens"] = map[string]any{
			"input": tk.input, "output": tk.output, "reasoning": tk.reasoning,
			"cache": map[string]any{"read": tk.cacheRead, "write": tk.cacheWrite},
		}
		info["cost"] = 0.01
	}
	if len(parts) == 0 {
		parts = []map[string]any{textPart("ok")}
	}
	return map[string]any{"info": info, "parts": parts}
}

func userMsg(t *testing.T, parts ...map[string]any) map[string]any {
	t.Helper()
	return map[string]any{
		"info":  map[string]any{"id": "msg_u", "role": "user"},
		"parts": parts,
	}
}

func textPart(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func skillPart(name string) map[string]any {
	return map[string]any{
		"type": "tool", "tool": "skill", "callID": "c1",
		"state": map[string]any{"status": "completed", "input": map[string]any{"name": name}},
	}
}

func toolPart(tool, filePath string) map[string]any {
	return map[string]any{
		"type": "tool", "tool": tool, "callID": "c2",
		"state": map[string]any{"status": "completed", "input": map[string]any{"filePath": filePath}},
	}
}

func subtaskPart(command, agent string) map[string]any {
	return map[string]any{
		"type": "subtask", "agent": agent, "description": "d", "command": command, "prompt": "p",
	}
}

func mustParse(t *testing.T, raw string) *opencode.Adapter {
	t.Helper()
	a, err := opencode.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return a
}

// Acceptance (T8): "Running a real OpenCode phase yields a record whose
// per-model tokens equal OpenCode's own accounting for that bracket." The
// three turns' usage is summed by hand from the SAME numbers fed into the
// fixture, folding reasoning into output per OpenCode's own stats
// convention.
func TestSpend_ReconcilesToOpenCodesOwnAccounting(t *testing.T) {
	open := tokens{input: 100, output: 20, reasoning: 5, cacheWrite: 8, cacheRead: 50}
	mid := tokens{input: 200, output: 40}
	closeT := tokens{input: 10, output: 5}
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &open, skillPart("prd-new")),
		assistantMsg(t, "claude-sonnet-5", &mid),
		assistantMsg(t, "claude-sonnet-5", &closeT, toolPart("write", "/repo/docs/prds/x/000-base.md")),
	)
	a := mustParse(t, raw)

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
		Input:         open.input + mid.input + closeT.input,
		Output:        open.output + open.reasoning + mid.output + closeT.output,
		CacheCreation: open.cacheWrite,
		CacheRead:     open.cacheRead,
	}
	ms, ok := gotSpend["anthropic/claude-sonnet-5"]
	if !ok {
		t.Fatalf("want an anthropic/claude-sonnet-5 entry, got %+v", gotSpend)
	}
	if ms.TokenQuartet != want {
		t.Errorf("token quartet mismatch:\nwant %+v\ngot  %+v", want, ms.TokenQuartet)
	}
	if len(gotSpend) != 1 {
		t.Errorf("want exactly 1 model, got %d: %+v", len(gotSpend), gotSpend)
	}
}

// Reasoning tokens have no slot of their own in the shared record schema
// (frozen, out of this task's scope); this pins that they are folded into
// Output, matching OpenCode's own `stats` rollup rather than being silently
// dropped.
func TestSpend_ReasoningTokens_FoldedIntoOutput(t *testing.T) {
	tk := tokens{input: 1, output: 10, reasoning: 7}
	raw := doc(t, assistantMsg(t, "claude-opus-4-8", &tk, skillPart("plan-new")))
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	gotSpend, _ := a.Spend(phases[0])
	if got := gotSpend["anthropic/claude-opus-4-8"].Output; got != 17 {
		t.Errorf("Output = %d, want 17 (10 output + 7 reasoning)", got)
	}
}

// A skill iterating on its own doc over many edits must have every turn's
// usage captured, not just the first — which would prematurely close the
// phase and silently lose the rest.
func TestSpend_MultiEditCoalescing_NoUsageLostToPrematureClose(t *testing.T) {
	u1 := tokens{input: 1, output: 1}
	u2 := tokens{input: 10, output: 2}
	u3 := tokens{input: 20, output: 3}
	u4 := tokens{input: 30, output: 4}
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &u1, skillPart("feature-brainstorm")),
		assistantMsg(t, "claude-sonnet-5", &u2, toolPart("edit", "/repo/docs/brainstorms/x.md")),
		assistantMsg(t, "claude-sonnet-5", &u3, toolPart("edit", "/repo/docs/brainstorms/x.md")),
		assistantMsg(t, "claude-sonnet-5", &u4, toolPart("multiedit", "/repo/docs/brainstorms/x.md")),
	)
	a := mustParse(t, raw)

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
	if got := gotSpend["anthropic/claude-sonnet-5"].TokenQuartet; got != want {
		t.Errorf("coalescing lost usage: want %+v, got %+v", want, got)
	}
}

// A docs/wiki/ edit touched in passing must never be mistaken for the
// close, while the phase's real, later artifact write still correctly
// closes it and every turn's usage is still captured.
func TestSpend_WikiSideEdit_IgnoredForBoundary(t *testing.T) {
	uOpen := tokens{input: 5, output: 1}
	uDraft := tokens{input: 6, output: 1}
	uSide := tokens{input: 999, output: 999} // would wrongly close the phase early if mistaken for the artifact
	uFinal := tokens{input: 7, output: 2}
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &uOpen, skillPart("prd-review")),
		assistantMsg(t, "claude-sonnet-5", &uDraft, toolPart("edit", "/repo/docs/reviews/x-000.md")),
		assistantMsg(t, "claude-sonnet-5", &uSide, toolPart("edit", "/repo/docs/wiki/glossary.md")),
		assistantMsg(t, "claude-sonnet-5", &uFinal, toolPart("write", "/repo/docs/reviews/x-000.md")),
	)
	a := mustParse(t, raw)
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
	if got := gotSpend["anthropic/claude-sonnet-5"].TokenQuartet; got != want {
		t.Errorf("want every turn's usage summed regardless of the wiki side-edit: want %+v, got %+v", want, got)
	}
}

// The wiki exclusion never suppresses a REAL close silently: this pins that
// a non-wiki write is required, not that wiki writes are invisible to
// parsing entirely.
func TestParse_WikiOnlyWrite_NeverCloses(t *testing.T) {
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, skillPart("prd-new")),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, toolPart("edit", "/repo/docs/wiki/glossary.md")),
	)
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a wiki-only write must never close a phase, got complete")
	}
}

// Acceptance (DINV-3): a phase opened via a subtask-marked command whose
// session ends before the assistant ever replies (no usage-bearing turn
// exists at all within the bracket) must declare Unavailable, never a
// silent zero. This is the realistic path to "no capturable usage" for
// OpenCode: every assistant message that DOES exist always carries a
// tokens object (see message-v2.ts's zod schema), so the only way to
// capture nothing is for no assistant turn to exist in the bracket at all.
func TestSpend_AbortedAfterSubtaskOpen_DeclaresUnavailable(t *testing.T) {
	raw := doc(t, userMsg(t, subtaskPart("tech-design-new", "build")))
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	p := phases[0]
	if p.ID != "tech-design-new" {
		t.Fatalf("want phase id tech-design-new, got %q", p.ID)
	}
	if p.Complete {
		t.Fatalf("want an incomplete (aborted) phase, got complete")
	}

	gotSpend, unavail := a.Spend(p)
	if gotSpend != nil {
		t.Fatalf("want a nil spend map, got %+v", gotSpend)
	}
	want := []record.Unavailable{{Harness: opencode.HarnessID, MissingFigure: "per-model spend"}}
	if len(unavail) != 1 || unavail[0] != want[0] {
		t.Errorf("want Unavailable %+v, got %+v", want, unavail)
	}
}

// A "subtask" part's command is the open marker for a command routed
// through OpenCode's dedicated command API against a subagent-mode agent —
// an alternative path to the "skill" tool call, attached to a USER message
// rather than an assistant one (see the package doc's phase-boundary
// discussion).
func TestParse_SubtaskCommand_OpensPhase(t *testing.T) {
	tk := tokens{input: 3, output: 1}
	raw := doc(t,
		userMsg(t, subtaskPart("tech-design-review", "reviewer")),
		assistantMsg(t, "claude-opus-4-8", &tk, toolPart("write", "/repo/docs/reviews/x.md")),
	)
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d: %+v", len(phases), phases)
	}
	p := phases[0]
	if p.ID != "tech-design-review" || !p.Complete {
		t.Fatalf("want a complete tech-design-review phase, got %+v", p)
	}
	gotSpend, unavail := a.Spend(p)
	if len(unavail) != 0 {
		t.Fatalf("want no Unavailable, got %+v", unavail)
	}
	if gotSpend["anthropic/claude-opus-4-8"].Input != 3 {
		t.Errorf("want input 3, got %+v", gotSpend)
	}
}

// A phase that uses more than one model must carry each as its own entry,
// never merged into one.
func TestSpend_MultiModel_KeptSeparate(t *testing.T) {
	sonnet := tokens{input: 100, output: 10}
	haiku := tokens{input: 5, output: 1}
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &sonnet, skillPart("plan-new")),
		assistantMsg(t, "claude-haiku-4-5", &haiku),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, toolPart("write", "/repo/docs/plans/x/001-plan.md")),
	)
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	gotSpend, _ := a.Spend(phases[0])

	if len(gotSpend) != 2 {
		t.Fatalf("want 2 distinct models, got %d: %+v", len(gotSpend), gotSpend)
	}
	if gotSpend["anthropic/claude-sonnet-5"].Input != sonnet.input+1 {
		t.Errorf("anthropic/claude-sonnet-5 input: want %d, got %d", sonnet.input+1, gotSpend["anthropic/claude-sonnet-5"].Input)
	}
	if gotSpend["anthropic/claude-haiku-4-5"].Input != haiku.input {
		t.Errorf("anthropic/claude-haiku-4-5 input: want %d, got %d", haiku.input, gotSpend["anthropic/claude-haiku-4-5"].Input)
	}
}

// A write outside any docs/ directory (e.g. this feature's own Go source,
// or a bash-driven mutation) must never be mistaken for the artifact-write
// close.
func TestParse_NonDocsWrite_NeverCloses(t *testing.T) {
	raw := doc(t,
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, skillPart("implement-now")),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, toolPart("write", "/repo/internal/phasemeter/spend/spend.go")),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, map[string]any{
			"type": "tool", "tool": "bash", "callID": "c9",
			"state": map[string]any{"status": "completed", "input": map[string]any{"command": "gh pr create --title x"}},
		}),
	)
	a := mustParse(t, raw)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a non-docs write (or a bash gh pr create) must not close the phase — want incomplete, got complete")
	}
}

// A plain user message (no subtask part) must contribute no turn at all —
// mirroring the claude-code adapter's "user lines contribute none".
func TestParse_PlainUserMessages_ContributeNoTurn(t *testing.T) {
	raw := doc(t,
		userMsg(t, textPart("please start")),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, skillPart("prd-new")),
		userMsg(t, textPart("looks good")),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, toolPart("write", "/repo/docs/prds/x/000-base.md")),
	)
	a := mustParse(t, raw)
	if len(a.Turns()) != 2 {
		t.Fatalf("want 2 turns (plain user messages contribute none), got %d", len(a.Turns()))
	}
}

// A well-formed document with zero messages parses to zero turns and zero
// phases — never an error. This differs from the claude-code adapter's
// "empty file" case only in shape: `opencode export` always emits a
// well-formed JSON object, so the empty case here is an empty messages
// array, not zero bytes (see TestParse_MalformedJSON_FailsClosed for the
// latter).
func TestParse_NoMessages(t *testing.T) {
	a := mustParse(t, doc(t))
	if len(a.Turns()) != 0 {
		t.Fatalf("want 0 turns, got %d", len(a.Turns()))
	}
	phases := phase.Detect(a.Turns())
	if len(phases) != 0 {
		t.Fatalf("want 0 phases, got %d", len(phases))
	}
}

// Truly malformed/empty input is a fail-closed error — this is the
// harness's own accounting, not user input to tolerate past.
func TestParse_MalformedJSON_FailsClosed(t *testing.T) {
	for _, raw := range []string{"", "not json", `{"messages":[{"info":`} {
		if _, err := opencode.Parse(strings.NewReader(raw)); err == nil {
			t.Errorf("want an error for malformed input %q, got nil", raw)
		}
	}
}

// Acceptance (T8, DINV-8): a seeded secret anywhere in the export — message
// text, a tool's own free-text input/output beyond the narrow fields this
// adapter reads — must never reach a written record.
func TestSpend_SeededSecret_NeverInAnyRecord(t *testing.T) {
	const secret = "sk-SEEDED-SECRET-DO-NOT-LEAK-4f8a"
	raw := doc(t,
		userMsg(t, textPart("here is my key: "+secret)),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1},
			map[string]any{"type": "reasoning", "text": "the user's secret is " + secret},
			skillPart("prd-new"),
		),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1}, map[string]any{
			"type": "tool", "tool": "edit", "callID": "c1",
			"state": map[string]any{
				"status": "completed",
				"input": map[string]any{
					"filePath":  "/repo/docs/prds/x/000-base.md",
					"oldString": secret, "newString": "redacted",
				},
			},
		}),
		assistantMsg(t, "claude-sonnet-5", &tokens{input: 1},
			textPart("done, the secret was "+secret),
			toolPart("write", "/repo/docs/prds/x/000-base.md"),
		),
	)
	a := mustParse(t, raw)
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
	a := mustParse(t, doc(t))
	if got := a.Harness(); got != "opencode" {
		t.Errorf("Harness() = %q, want opencode", got)
	}
}

// -- Load: the IO boundary --------------------------------------------------

func TestLoad_EmptySessionID_FailsClosed(t *testing.T) {
	if _, err := opencode.Load(context.Background(), t.TempDir(), ""); err == nil {
		t.Fatal("Load with an empty sessionID must fail closed, never fall back to an interactive picker")
	}
}

func TestLoad_EmptyWorkdir_FailsClosed(t *testing.T) {
	if _, err := opencode.Load(context.Background(), "", "ses_whatever"); err == nil {
		t.Fatal("Load with an empty workdir must fail closed")
	}
}

// TestLive_OpenCodeExport_MissingSession is the only test that touches a
// real `opencode` binary. It gates on availability and skips cleanly where
// the CLI is absent — the hermetic tests above are the in-session coverage
// (mirroring internal/adapters/harness/opencode's own TestLive_* pattern).
// It deliberately never references a real session id (one machine's real
// session ids are not a fact about the opencode CLI in general, and
// wouldn't exist on any other machine or in CI) and spends no model
// tokens: a clearly-fake id must fail cleanly and promptly, proving the
// subprocess plumbing (args, cwd, stdout/stderr capture) works end to end
// against the live binary without depending on local data or state.
func TestLive_OpenCodeExport_MissingSession(t *testing.T) {
	if _, err := exec.LookPath(binaryForLiveTest); err != nil {
		t.Skipf("opencode not installed (%v) — live check skipped; hermetic tests above cover the adapter", err)
	}
	_, err := opencode.Load(context.Background(), t.TempDir(), "ses_definitely_not_a_real_session_id")
	if err == nil {
		t.Fatal("Load with a nonexistent session id must return an error, not hang or succeed")
	}
}

// binaryForLiveTest names the executable the live test gates on — kept as
// its own constant (rather than reaching into the adapter package) since
// opencode_test is an external test package (opencode_test, not opencode).
const binaryForLiveTest = "opencode"
