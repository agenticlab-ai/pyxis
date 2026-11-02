package codex_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/spend/codex"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/spend"
)

// -- fixture builders, matching Codex CLI's own transcript shape as
// captured from a real ~/.codex/sessions/<yyyy>/<mm>/<dd>/rollout-*.jsonl
// session: a top-level {type, payload} envelope, where the active model
// (turn_context) and a completion's own usage (an event_msg/token_count)
// arrive as two separate record kinds rather than one assistant message. --

type usage struct{ input, output, cached int64 }

func line(t *testing.T, v map[string]any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func turnContext(t *testing.T, model string) string {
	t.Helper()
	return line(t, map[string]any{
		"type":    "turn_context",
		"payload": map[string]any{"turn_id": "019f0000-0000-0000-0000-000000000000", "model": model},
	})
}

func userMessage(t *testing.T, text string) string {
	t.Helper()
	return line(t, map[string]any{
		"type":    "event_msg",
		"payload": map[string]any{"type": "user_message", "message": text},
	})
}

func tokenCount(t *testing.T, u usage) string {
	t.Helper()
	return line(t, map[string]any{
		"type": "event_msg",
		"payload": map[string]any{
			"type": "token_count",
			"info": map[string]any{
				"last_token_usage": map[string]any{
					"input_tokens":            u.input,
					"cached_input_tokens":     u.cached,
					"output_tokens":           u.output,
					"reasoning_output_tokens": 0,
					"total_tokens":            u.input + u.output,
				},
			},
		},
	})
}

func patchApplyEnd(t *testing.T, success bool, paths ...string) string {
	t.Helper()
	changes := map[string]any{}
	for _, p := range paths {
		changes[p] = map[string]any{"type": "update", "unified_diff": "@@ -1 +1 @@\n-a\n+b\n"}
	}
	return line(t, map[string]any{
		"type":    "event_msg",
		"payload": map[string]any{"type": "patch_apply_end", "success": success, "changes": changes},
	})
}

func transcript(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

func mustParse(t *testing.T, raw string) *codex.Adapter {
	t.Helper()
	a, err := codex.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return a
}

// Acceptance (T6): "A Codex reference-run fixture yields per-model spend …
// attributed to the correct phase." The three completions' usage is summed
// by hand here from the SAME numbers fed into the fixture — this is exactly
// Codex's own accounting for the bracket — and the adapter's output must
// equal it, with the cached-token normalization applied (§ package doc).
func TestSpend_ReconcilesToCodexsOwnAccounting(t *testing.T) {
	open := usage{input: 100, output: 20, cached: 50}
	mid := usage{input: 200, output: 40}
	closeU := usage{input: 10, output: 5}
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "let's kick off the $prd-new to start drafting this"),
		tokenCount(t, open),
		tokenCount(t, mid),
		patchApplyEnd(t, true, "/repo/README.md", "/repo/docs/prds/x/000-base.md"),
		tokenCount(t, closeU),
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
		Input:     (open.input - open.cached) + mid.input + closeU.input,
		Output:    open.output + mid.output + closeU.output,
		CacheRead: open.cached,
	}
	ms, ok := gotSpend["gpt-5"]
	if !ok {
		t.Fatalf("want a gpt-5 entry, got %+v", gotSpend)
	}
	if ms.TokenQuartet != want {
		t.Errorf("token quartet mismatch:\nwant %+v\ngot  %+v", want, ms.TokenQuartet)
	}
	if len(gotSpend) != 1 {
		t.Errorf("want exactly 1 model, got %d: %+v", len(gotSpend), gotSpend)
	}
}

// Codex's cached_input_tokens is a SUBSET of input_tokens (unlike Claude
// Code's disjoint cache_read/input split) — pinned on its own so a
// regression that sums the raw fields instead of the normalized ones is
// caught directly, independent of the broader reconciliation test.
func TestParse_CachedInputTokens_NotDoubleCounted(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$prd-new please"),
		tokenCount(t, usage{input: 1000, cached: 400, output: 50}),
		patchApplyEnd(t, true, "/repo/docs/reviews/x-000.md"),
		tokenCount(t, usage{input: 10, output: 2}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	gotSpend, _ := a.Spend(phases[0])
	ms := gotSpend["gpt-5"]
	if ms.Input != 1000-400+10 {
		t.Errorf("Input: want %d (fresh, cached excluded), got %d", 1000-400+10, ms.Input)
	}
	if ms.CacheRead != 400 {
		t.Errorf("CacheRead: want 400, got %d", ms.CacheRead)
	}
	if ms.Input+ms.CacheRead != 1000+10 {
		t.Errorf("Input+CacheRead must reproduce the true total input (%d), got %d", 1000+10, ms.Input+ms.CacheRead)
	}
}

// A phase whose only turn is a trailing marker flushed at end-of-transcript
// (no billed completion ever arrived for it) has no capturable usage and
// must declare Unavailable, never a silent zero (DINV-3) — while still
// being recorded as its own (incomplete) phase, never dropped (UB-3).
func TestSpend_TranscriptEndsBeforeAnyBilledCompletion_DeclaresUnavailable(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$tech-design-new let's go"),
	)
	a := mustParse(t, tr)
	if len(a.Turns()) != 1 {
		t.Fatalf("want the flushed marker-only turn, got %d turns", len(a.Turns()))
	}
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("want an incomplete phase (no close arrived), got complete")
	}

	gotSpend, unavail := a.Spend(phases[0])
	if gotSpend != nil {
		t.Fatalf("want a nil spend map, got %+v", gotSpend)
	}
	want := record.Unavailable{Harness: codex.HarnessID, MissingFigure: "per-model spend"}
	if len(unavail) != 1 || unavail[0] != want {
		t.Errorf("want Unavailable %+v, got %+v", want, unavail)
	}
}

// A phase that switches model mid-flight (turn_context changes between
// token_count events) must carry each model as its own entry, never merged.
func TestSpend_MultiModel_KeptSeparate(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$plan-new begin"),
		tokenCount(t, usage{input: 100, output: 10}),
		turnContext(t, "gpt-5-mini"),
		tokenCount(t, usage{input: 5, output: 1}),
		turnContext(t, "gpt-5"),
		patchApplyEnd(t, true, "/repo/docs/plans/x/001-plan.md"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	gotSpend, _ := a.Spend(phases[0])
	if len(gotSpend) != 2 {
		t.Fatalf("want 2 distinct models, got %d: %+v", len(gotSpend), gotSpend)
	}
	if gotSpend["gpt-5"].Input != 100+1 {
		t.Errorf("gpt-5 input: want %d, got %d", 100+1, gotSpend["gpt-5"].Input)
	}
	if gotSpend["gpt-5-mini"].Input != 5 {
		t.Errorf("gpt-5-mini input: want 5, got %d", gotSpend["gpt-5-mini"].Input)
	}
}

// A real captured false-positive class (§ package doc): Codex sessions
// often carry pasted shell text full of "$VAR"-style references. None of
// these are hyphenated, so none must ever be mistaken for a skill mention.
func TestParse_ShellVariableMentions_NeverMistakenForSkill(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "run: for f in $file; do echo $BASE/$PID >> $LOG; done"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	if len(a.Turns()) != 1 {
		t.Fatalf("want 1 turn, got %d", len(a.Turns()))
	}
	if got := a.Turns()[0].SkillInvocation; got != "" {
		t.Errorf("a shell-variable mention must never be read as a skill invocation, got %q", got)
	}
}

// A real captured example of this repo's own convention (README.md: "In
// Codex, mention `$prd-new`"): a hyphenated skill mention embedded mid-
// sentence, not just at message start, must still be detected.
func TestParse_HyphenatedSkillMention_DetectedMidSentence(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "let's kick off the $feature-brainstorm to start discussing this feature"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	if got := a.Turns()[0].SkillInvocation; got != "feature-brainstorm" {
		t.Errorf("SkillInvocation = %q, want feature-brainstorm", got)
	}
}

// A patch that failed to apply changed nothing, so it must never close the
// phase — only a success: true patch counts.
func TestParse_UnsuccessfulPatch_NeverCloses(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$prd-new go"),
		tokenCount(t, usage{input: 1, output: 1}),
		patchApplyEnd(t, false, "/repo/docs/prds/x/000-base.md"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a failed patch must never close the phase, got complete")
	}
}

// docs/wiki/ is freely-editable reference material, never a phase's own
// frozen artifact (AGENTS.md) — a patch touching only a wiki path must
// never close the phase, mirroring the claudecode adapter's own rule.
func TestParse_WikiOnlyPatch_NeverCloses(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$prd-new go"),
		tokenCount(t, usage{input: 1, output: 1}),
		patchApplyEnd(t, true, "/repo/docs/wiki/glossary.md"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a wiki-only patch must never close a phase, got complete")
	}
}

// A patch entirely outside docs/ (e.g. this feature's own Go source) must
// never be mistaken for the artifact-write close.
func TestParse_NonDocsPatch_NeverCloses(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$implement-now go"),
		tokenCount(t, usage{input: 1, output: 1}),
		patchApplyEnd(t, true, "/repo/internal/phasemeter/spend/spend.go"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if phases[0].Complete {
		t.Errorf("a non-docs patch must never close the phase, got complete")
	}
}

// A multi-file patch — Codex's apply_patch can touch several files in one
// shot — closes as soon as ANY changed path is a doc artifact, even mixed
// with unrelated files (already exercised by the main reconciliation test
// above; pinned again here in isolation for the boundary itself).
func TestParse_MultiFilePatch_AnyDocsPathCloses(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$prd-review this"),
		tokenCount(t, usage{input: 1, output: 1}),
		patchApplyEnd(t, true, "/repo/AGENTS.md", "/repo/docs/reviews/x-000.md", "/repo/README.md"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	a := mustParse(t, tr)
	phases := phase.Detect(a.Turns())
	if len(phases) != 1 {
		t.Fatalf("want 1 phase, got %d", len(phases))
	}
	if !phases[0].Complete {
		t.Errorf("want the mixed patch (one docs/ path among several) to close the phase")
	}
}

func TestHarness(t *testing.T) {
	a := mustParse(t, "")
	if got := a.Harness(); got != "codex" {
		t.Errorf("Harness() = %q, want codex", got)
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

// Blank lines between JSONL records must be tolerated.
func TestParse_BlankLines_Tolerated(t *testing.T) {
	tr := "\n" + turnContext(t, "gpt-5") + "\n\n" + userMessage(t, "$prd-new go") + "\n" +
		tokenCount(t, usage{input: 1, output: 1}) + "\n\n"
	a := mustParse(t, tr)
	if len(a.Turns()) != 1 {
		t.Fatalf("want 1 turn, got %d", len(a.Turns()))
	}
}

// A malformed line is a fail-closed error — this is the harness's own
// accounting, not user input to tolerate past.
func TestParse_MalformedLine_FailsClosed(t *testing.T) {
	tr := transcript(
		turnContext(t, "gpt-5"),
		`{"type":"event_msg","payload":{`, // truncated JSON
	)
	if _, err := codex.Parse(strings.NewReader(tr)); err == nil {
		t.Fatal("want an error for a malformed transcript line, got nil")
	}
}

// Acceptance (DINV-8): a seeded secret anywhere in the transcript —
// including inside a patch's own unified-diff body, which this package
// deliberately never decodes — must never reach a written record.
func TestSpend_SeededSecret_NeverInAnyRecord(t *testing.T) {
	const secret = "sk-SEEDED-SECRET-DO-NOT-LEAK-4f8a"
	tr := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "here is my key: "+secret+" — kick off $prd-new"),
		tokenCount(t, usage{input: 1, output: 1}),
		line(t, map[string]any{
			"type": "event_msg",
			"payload": map[string]any{
				"type":    "patch_apply_end",
				"success": true,
				"changes": map[string]any{
					"/repo/docs/prds/x/000-base.md": map[string]any{
						"type":         "update",
						"unified_diff": "@@ -1 +1 @@\n-a\n+contains " + secret + "\n",
					},
				},
			},
		}),
		tokenCount(t, usage{input: 1, output: 1}),
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

// Acceptance (T6): "Reading Codex's accounting leaves Codex's own output
// byte-unchanged."
func TestLoad_TranscriptFileLeftByteUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-test.jsonl")
	content := transcript(
		turnContext(t, "gpt-5"),
		userMessage(t, "$prd-new go"),
		tokenCount(t, usage{input: 1, output: 1}),
		patchApplyEnd(t, true, "/repo/docs/prds/x/000-base.md"),
		tokenCount(t, usage{input: 1, output: 1}),
	)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	if _, err := codex.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("transcript mutated by Load: NR-1/NR-3 violated")
	}
}
