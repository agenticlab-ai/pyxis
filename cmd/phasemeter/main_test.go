package main

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/price"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/store"
)

// -- fixture builders: the same Claude Code transcript envelope shape the
// claudecode adapter parses (type/isSidechain/message, an array of typed
// content blocks, per-turn usage on assistant messages). --

type tokenUsage struct{ input, output, cacheCreation, cacheRead int64 }

func assistantLine(t *testing.T, model string, u *tokenUsage, blocks ...map[string]any) string {
	t.Helper()
	msg := map[string]any{"model": model, "id": "msg_x", "type": "message", "role": "assistant", "content": blocks}
	if u != nil {
		msg["usage"] = map[string]any{
			"input_tokens":                u.input,
			"output_tokens":               u.output,
			"cache_creation_input_tokens": u.cacheCreation,
			"cache_read_input_tokens":     u.cacheRead,
		}
	}
	line, err := json.Marshal(map[string]any{"type": "assistant", "isSidechain": false, "message": msg})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(line)
}

func skillBlock(skill string) map[string]any {
	return map[string]any{"type": "tool_use", "id": "t1", "name": "Skill", "input": map[string]any{"skill": skill}}
}

func writeBlock(path string, extra ...map[string]any) map[string]any {
	input := map[string]any{"file_path": path}
	for _, e := range extra {
		maps.Copy(input, e)
	}
	return map[string]any{"type": "tool_use", "id": "t2", "name": "Write", "input": input}
}

func textBlock(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func writeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture transcript: %v", err)
	}
	return path
}

func capture(args []string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// -- fixture builders for the --harness=cursor end-to-end test: cursor-agent's
// own stream-json shape (cursor.com/docs/cli/reference/output-format) — a
// "type"/"subtype" discriminator, "user" messages carrying the slash-command
// open marker, and "tool_call" events carrying a *ToolCall-keyed payload. See
// internal/adapters/spend/cursor's own tests for the full fixture-shape
// rationale; only what this end-to-end test needs is reproduced here.

func cursorUserLine(t *testing.T, text string) string {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": []map[string]any{{"type": "text", "text": text}},
		},
		"session_id": "sid",
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(line)
}

func cursorWriteToolCallLine(t *testing.T, subtype, path string, success bool) string {
	t.Helper()
	call := map[string]any{"args": map[string]any{"path": path, "toolCallId": "tc1"}}
	if subtype == "completed" {
		call["result"] = map[string]any{"success": success}
	}
	line, err := json.Marshal(map[string]any{
		"type": "tool_call", "subtype": subtype,
		"tool_call": map[string]any{"writeToolCall": call},
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(line)
}

// -- usage / DINV-9 misparse refusal --

func TestNoArgsPrintsUsage(t *testing.T) {
	if code, _, _ := capture(nil); code != 2 {
		t.Fatalf("no-args exit = %d, want 2", code)
	}
}

func TestUnknownVerbRefuses(t *testing.T) {
	if code, _, _ := capture([]string{"bogus"}); code != 2 {
		t.Fatalf("unknown verb exit = %d, want 2", code)
	}
}

// Acceptance (T3, DINV-9/I3): a flag-like positional — here a flag placed
// AFTER the transcript-path positional — must be refused as a usage error
// before any write. storeDir is a path that does not exist yet; the refusal
// must leave it that way.
func TestRecordFlagAfterPositionalRefusesBeforeAnyWrite(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")))
	storeDir := recordStoreDir(t)

	code, _, stderr := capture([]string{
		"record", transcript, "--run-id=run-1", "--feature-id=pipeline-measurement", "--store-dir=" + storeDir,
	})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage refusal); stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "nothing was written") {
		t.Errorf("stderr should name the I3 refusal, got %q", stderr)
	}
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatalf("store dir must not be created on a misparse refusal, stat err = %v", err)
	}
}

func TestRecordUnknownFlagRefuses(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")))
	code, _, _ := capture([]string{"record", "--bogus-flag", "--run-id=run-1", "--feature-id=f", transcript})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestRecordMissingPositionalRefuses(t *testing.T) {
	code, _, stderr := capture([]string{"record", "--run-id=run-1", "--feature-id=pipeline-measurement"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "got 0") {
		t.Errorf("stderr should name the missing positional, got %q", stderr)
	}
}

func TestRecordTooManyPositionalsRefuses(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")))
	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", transcript, "extra-arg",
	})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "got 2") {
		t.Errorf("stderr should name the too-many-positionals count, got %q", stderr)
	}
}

func TestRecordHelpFlagPrintsFullUsage(t *testing.T) {
	code, _, stderr := capture([]string{"record", "-h"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage: phasemeter") {
		t.Errorf("record -h should print the full usage, got %q", stderr)
	}
}

func TestRecordMissingRequiredFlagsRefuses(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")))
	tests := [][]string{
		{"record", "--feature-id=pipeline-measurement", transcript},
		{"record", "--run-id=run-1", transcript},
		{"record", transcript},
	}
	for _, args := range tests {
		if code, _, _ := capture(args); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}

func TestRecordUnsupportedHarnessRefuses(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")))
	storeDir := recordStoreDir(t)
	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--harness=no-such-harness", "--store-dir=" + storeDir, transcript,
	})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, `"no-such-harness"`) || !strings.Contains(stderr, "claude-code") {
		t.Errorf("stderr should name the bad value and the supported set, got %q", stderr)
	}
}

// -- domain-level refusals: parsing was attempted, exit 1 --

func TestRecordRefusesStoreOutsideLocalLogsBeforeReadingTranscript(t *testing.T) {
	localStore := recordStoreDir(t)
	outside := filepath.Join(t.TempDir(), "outside-store")
	missing := filepath.Join(t.TempDir(), "missing-transcript.jsonl")
	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + outside, missing,
	})
	if code != 2 || !strings.Contains(stderr, "execution log path must be inside") {
		t.Fatalf("record = %d stderr=%q", code, stderr)
	}
	for _, path := range []string{outside, localStore} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("refused record created a store at %s: %v", path, err)
		}
	}
}

func TestRecordUnreadableTranscriptExitsOne(t *testing.T) {
	storeDir := recordStoreDir(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist.jsonl")
	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + storeDir, missing,
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
}

func TestRecordMalformedTranscriptExitsOne(t *testing.T) {
	transcript := writeTranscript(t,
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")),
		`{"type":"assistant","message":{`, // truncated JSON — the harness's own accounting is malformed
	)
	storeDir := recordStoreDir(t)
	code, _, _ := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + storeDir, transcript,
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (fail closed on a malformed transcript)", code)
	}
}

// -- the end-to-end tracer bullet --

// Acceptance (T3): "Running a real Claude Code phase yields a record whose
// per-model tokens equal Claude Code's own accounting for that bracket."
func TestRecordEndToEnd_TwoPhases_ReconcileToTranscriptUsage(t *testing.T) {
	open1 := tokenUsage{input: 100, output: 20, cacheCreation: 5, cacheRead: 50}
	mid1 := tokenUsage{input: 200, output: 40}
	close1 := tokenUsage{input: 10, output: 5}
	open2 := tokenUsage{input: 7, output: 3}

	transcript := writeTranscript(t,
		assistantLine(t, "claude-sonnet-5", &open1, skillBlock("prd-new")),
		assistantLine(t, "claude-sonnet-5", &mid1, textBlock("working")),
		assistantLine(t, "claude-sonnet-5", &close1, writeBlock("/repo/docs/prds/x/000-base.md")),
		assistantLine(t, "claude-opus-4-8", &open2, skillBlock("prd-review")),
		assistantLine(t, "claude-opus-4-8", &tokenUsage{input: 1, output: 1}, writeBlock("/repo/docs/reviews/x-000.md")),
	)
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=pipeline-measurement", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "recorded 2 phase(s)") {
		t.Errorf("stdout summary missing phase count, got %q", stdout)
	}

	s := store.New(storeDir)
	got, err := s.LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 records written to the store, got %d: %+v", len(got), got)
	}

	byPhase := map[string]record.Record{}
	for _, r := range got {
		byPhase[r.PhaseID] = r
	}

	prdNew, ok := byPhase["prd-new"]
	if !ok {
		t.Fatalf("no prd-new record, got %+v", byPhase)
	}
	if prdNew.RunID != "run-1" || prdNew.FeatureID != "pipeline-measurement" || prdNew.Harness != "claude-code" {
		t.Errorf("prd-new identity mismatch: %+v", prdNew)
	}
	wantQuartet := record.TokenQuartet{
		Input:         open1.input + mid1.input + close1.input,
		Output:        open1.output + mid1.output + close1.output,
		CacheCreation: open1.cacheCreation,
		CacheRead:     open1.cacheRead,
	}
	if got := prdNew.Spend["claude-sonnet-5"].TokenQuartet; got != wantQuartet {
		t.Errorf("prd-new token quartet: want %+v, got %+v", wantQuartet, got)
	}

	prdReview, ok := byPhase["prd-review"]
	if !ok {
		t.Fatalf("no prd-review record, got %+v", byPhase)
	}
	wantQuartet2 := record.TokenQuartet{Input: open2.input + 1, Output: open2.output + 1}
	if got := prdReview.Spend["claude-opus-4-8"].TokenQuartet; got != wantQuartet2 {
		t.Errorf("prd-review token quartet: want %+v, got %+v", wantQuartet2, got)
	}
}

// -- Codex-shaped fixtures (T6): a top-level {type, payload} envelope,
// where the active model (turn_context) and a completion's own usage
// (event_msg/token_count) arrive as two separate record kinds rather than
// one assistant message — see internal/adapters/spend/codex's own doc
// comment for why. --

func codexTurnContext(t *testing.T, model string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type":    "turn_context",
		"payload": map[string]any{"turn_id": "t1", "model": model},
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func codexUserMessage(t *testing.T, text string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type":    "event_msg",
		"payload": map[string]any{"type": "user_message", "message": text},
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func codexTokenCount(t *testing.T, u tokenUsage) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": "event_msg",
		"payload": map[string]any{
			"type": "token_count",
			"info": map[string]any{
				"last_token_usage": map[string]any{
					"input_tokens":        u.input,
					"cached_input_tokens": u.cacheRead,
					"output_tokens":       u.output,
					"total_tokens":        u.input + u.output,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

func codexPatchApplyEnd(t *testing.T, success bool, paths ...string) string {
	t.Helper()
	changes := map[string]any{}
	for _, p := range paths {
		changes[p] = map[string]any{"type": "update", "unified_diff": "@@ -1 +1 @@\n"}
	}
	b, err := json.Marshal(map[string]any{
		"type":    "event_msg",
		"payload": map[string]any{"type": "patch_apply_end", "success": success, "changes": changes},
	})
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(b)
}

// Acceptance (T6): "A Codex reference-run fixture yields per-model spend …
// attributed to the correct phase" — exercised through the actual `record`
// verb wired to --harness codex, not just the adapter package directly.
func TestRecordEndToEnd_Codex_ReconcilesToTranscriptUsage(t *testing.T) {
	open := tokenUsage{input: 100, output: 20, cacheRead: 50}
	closeU := tokenUsage{input: 10, output: 5}

	transcript := writeTranscript(t,
		codexTurnContext(t, "gpt-5"),
		codexUserMessage(t, "kick off $prd-new please"),
		codexTokenCount(t, open),
		codexPatchApplyEnd(t, true, "/repo/docs/prds/x/000-base.md"),
		codexTokenCount(t, closeU),
	)
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=pipeline-measurement", "--harness=codex", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "recorded 1 phase(s)") {
		t.Errorf("stdout summary missing phase count, got %q", stdout)
	}

	got, err := store.New(storeDir).LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 record written to the store, got %d: %+v", len(got), got)
	}
	rec := got[0]
	if rec.PhaseID != "prd-new" || rec.Harness != "codex" {
		t.Fatalf("identity mismatch: %+v", rec)
	}
	want := record.TokenQuartet{
		Input:     (open.input - open.cacheRead) + closeU.input,
		Output:    open.output + closeU.output,
		CacheRead: open.cacheRead,
	}
	if got := rec.Spend["gpt-5"].TokenQuartet; got != want {
		t.Errorf("token quartet: want %+v, got %+v", want, got)
	}
}

// writeOpenCodeExport writes a fixture in `opencode export`'s JSON shape to a
// temp file and returns its path — OpenCode's session has no native transcript
// file, so --harness opencode reads this exported JSON (see main.go/openExport).
func writeOpenCodeExport(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "opencode-export.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write opencode export fixture: %v", err)
	}
	return p
}

// Acceptance (T8, completing the wiring): --harness opencode reads an
// `opencode export` JSON file end to end. The adapter package landed with #638
// but was never wired into the recorder's registry; this exercises the seam
// through the actual `record` verb, not opencode.Parse in isolation. The skill
// part opens a prd-new phase and the docs/ write closes it; the two turns' usage
// is summed (reasoning folded into output, per the adapter's convention).
func TestRecordEndToEnd_OpenCode_ReadsExportFile(t *testing.T) {
	export := writeOpenCodeExport(t, `{"info":{"id":"ses_test"},"messages":[
{"info":{"id":"m1","role":"assistant","providerID":"anthropic","modelID":"claude-sonnet-5","tokens":{"input":100,"output":20,"reasoning":0,"cache":{"read":0,"write":0}},"cost":0.01},"parts":[{"type":"tool","tool":"skill","callID":"c1","state":{"status":"completed","input":{"name":"prd-new"}}}]},
{"info":{"id":"m2","role":"assistant","providerID":"anthropic","modelID":"claude-sonnet-5","tokens":{"input":10,"output":5,"reasoning":0,"cache":{"read":0,"write":0}},"cost":0.01},"parts":[{"type":"tool","tool":"write","callID":"c2","state":{"status":"completed","input":{"filePath":"/repo/docs/prds/x/000-base.md"}}}]}
]}`)
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=pipeline-measurement", "--harness=opencode", "--store-dir=" + storeDir, export,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "recorded 1 phase(s)") {
		t.Errorf("stdout summary missing phase count, got %q", stdout)
	}

	got, err := store.New(storeDir).LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 record written, got %d: %+v", len(got), got)
	}
	rec := got[0]
	if rec.PhaseID != "prd-new" || rec.Harness != "opencode" {
		t.Fatalf("identity mismatch: %+v", rec)
	}
	want := record.TokenQuartet{Input: 110, Output: 25}
	if got := rec.Spend["anthropic/claude-sonnet-5"].TokenQuartet; got != want {
		t.Errorf("token quartet: want %+v, got %+v", want, got)
	}
}

// A transcript with no phase boundary at all (no skill invocation) yields no
// records and — since Append is never called — no store file, never an error.
func TestRecordNoPhaseBoundaries_NothingWritten(t *testing.T) {
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, textBlock("just chatting")))
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "nothing recorded") {
		t.Errorf("stdout should say nothing was recorded, got %q", stdout)
	}
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatalf("store dir must not be created when there is nothing to write, stat err = %v", err)
	}
}

// A phase with no capturable usage still yields its own record, carrying a
// declared Unavailable rather than a silent zero (DINV-3), and the summary
// names it.
func TestRecordDeclaresUnavailable_NeverASilentZero(t *testing.T) {
	transcript := writeTranscript(t,
		assistantLine(t, "claude-sonnet-5", nil, skillBlock("tech-design-new")),
		assistantLine(t, "claude-sonnet-5", nil, writeBlock("/repo/docs/tech-design/x.md")),
	)
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "unavailable: per-model spend") {
		t.Errorf("stdout should name the unavailable figure, got %q", stdout)
	}

	got, err := store.New(storeDir).LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(got) != 1 || got[0].Spend != nil {
		t.Fatalf("want 1 record with a nil Spend map, got %+v", got)
	}
	if len(got[0].Unavailable) != 1 || got[0].Unavailable[0].Harness != "claude-code" {
		t.Errorf("want a claude-code Unavailable marker, got %+v", got[0].Unavailable)
	}
}

// Acceptance (T7): "A Cursor CLI reference-run fixture yields per-model
// spend or a declared unavailable, attributed to the correct phase." Cursor
// exposes no per-model figure on any local channel (see
// internal/adapters/spend/cursor's package doc) — the reference outcome
// here is the FR-6 declaration, not a captured figure, and that is exactly
// what this asserts: an expected, first-class outcome, not a failure.
func TestRecordCursorHarness_EndToEnd_DeclaresUnavailable(t *testing.T) {
	transcript := writeTranscript(t,
		cursorUserLine(t, "/tech-design-new draft the design"),
		cursorWriteToolCallLine(t, "started", "/repo/docs/tech-design/x.md", false),
		cursorWriteToolCallLine(t, "completed", "/repo/docs/tech-design/x.md", true),
	)
	storeDir := recordStoreDir(t)

	code, stdout, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--harness=cursor", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "recorded 1 phase(s)") {
		t.Errorf("stdout should report 1 recorded phase, got %q", stdout)
	}
	if !strings.Contains(stdout, "unavailable: per-model spend") {
		t.Errorf("stdout should name the unavailable figure, got %q", stdout)
	}

	got, err := store.New(storeDir).LoadRun("run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 record, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.PhaseID != "tech-design-new" || r.Harness != "cursor" {
		t.Errorf("want phase tech-design-new on harness cursor, got %+v", r)
	}
	if r.Spend != nil {
		t.Errorf("want a nil Spend map — Cursor exposes no per-model figure — got %+v", r.Spend)
	}
	if len(r.Unavailable) != 1 || r.Unavailable[0] != (record.Unavailable{Harness: "cursor", MissingFigure: "per-model spend"}) {
		t.Errorf("want a cursor Unavailable marker, got %+v", r.Unavailable)
	}
}

// Acceptance (T3, DINV-2/NR-1): measurement never mutates the run it
// measures — the transcript's bytes must be identical before and after a
// full record pass.
func TestRecord_TranscriptLeftByteUnchanged(t *testing.T) {
	transcript := writeTranscript(t,
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")),
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, writeBlock("/repo/docs/prds/x/000-base.md")),
	)
	before, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatalf("read fixture before: %v", err)
	}

	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + recordStoreDir(t), transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}

	after, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatalf("read fixture after: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("transcript mutated by measurement: DINV-2 violated")
	}
}

// Acceptance (T3, DINV-8): a seeded secret anywhere in the transcript must
// never reach the store file actually written to disk — the strongest form
// of this check, at the level of the persisted artifact rather than the
// in-memory record.
func TestRecord_SeededSecret_NeverInTheWrittenStoreFile(t *testing.T) {
	const secret = "sk-SEEDED-SECRET-DO-NOT-LEAK-4f8a"
	transcript := writeTranscript(t,
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1},
			map[string]any{"type": "thinking", "thinking": "the secret is " + secret},
			skillBlock("prd-new")),
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1},
			textBlock("done, the secret was "+secret),
			writeBlock("/repo/docs/prds/x/000-base.md", map[string]any{"content": "contains " + secret})),
	)
	storeDir := recordStoreDir(t)

	code, _, stderr := capture([]string{
		"record", "--run-id=run-1", "--feature-id=f", "--store-dir=" + storeDir, transcript,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}

	storeBytes, err := os.ReadFile(filepath.Join(storeDir, "run-1.jsonl"))
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	if strings.Contains(string(storeBytes), secret) {
		t.Fatalf("seeded secret leaked into the written store file:\n%s", storeBytes)
	}
}

// The default --store-dir resolves against the repo root (t.Chdir is the
// harness-safe per-test cwd override this repo's other cmd tests use — see
// cmd/pyxis/main_test.go — never a raw os.Chdir, which would race a shared
// process cwd across parallel tests).
func TestRecordDefaultStoreDirResolvesAgainstRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	transcript := writeTranscript(t, assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, skillBlock("prd-new")),
		assistantLine(t, "claude-sonnet-5", &tokenUsage{input: 1}, writeBlock("/repo/docs/prds/x/000-base.md")))
	t.Chdir(root)

	code, _, stderr := capture([]string{"record", "--run-id=run-1", "--feature-id=f", transcript})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	want := filepath.Join(root, ".pyxis", "logs", "phasemeter", "run-1.jsonl")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("want a store file at the default repo-root-relative path %s: %v", want, err)
	}
}

// -- `read` verb fixtures: a throwaway git repo with a fixed identity and
// injected commit dates (never the wall clock — the hermetic-test
// discipline; mirrors internal/phasemeter/efficiency's own fixture), plus a
// small hand-authored price snapshot in the price.Parse shape. --

func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runGit(t, dir, "2026-07-01T00:00:00Z", "init", "-q", "-b", "main")
	return dir
}

func runGit(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitAt(t *testing.T, dir, path, content, date string) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, date, "add", path)
	runGit(t, dir, date, "commit", "-q", "-m", "commit "+path+" at "+date)
}

const readPriceFixture = `{
  "model-a": {"litellm_provider": "anthropic", "mode": "chat", "input_cost_per_token": 0.000003, "output_cost_per_token": 0.000015}
}`

func writePriceFixture(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "prices.json")
	if err := os.WriteFile(p, []byte(readPriceFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// -- usage / DINV-9 misparse refusal --

// Acceptance (T12, DINV-9/I3): a flag-like positional — here a flag placed
// AFTER the run-id positional — must be refused as a usage error before any
// read side effect. --repo-root names a directory that does not exist, so a
// real attempt to resolve it (store load, price load) would exit 1, not 2 —
// proving the refusal happens before any of that runs.
func TestReadFlagAfterPositionalRefusesBeforeAnyReadSideEffect(t *testing.T) {
	bogusRoot := filepath.Join(t.TempDir(), "does-not-exist")
	code, _, stderr := capture([]string{"read", "run-1", "--repo-root=" + bogusRoot})
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (usage refusal); stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "nothing was read") {
		t.Errorf("stderr should name the I3 refusal, got %q", stderr)
	}
}

func TestReadUnknownFlagRefuses(t *testing.T) {
	code, _, _ := capture([]string{"read", "--bogus-flag", "run-1"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}

func TestReadMissingPositionalRefuses(t *testing.T) {
	code, _, stderr := capture([]string{"read"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "got 0") {
		t.Errorf("stderr should name the missing positional, got %q", stderr)
	}
}

func TestReadTooManyPositionalsRefuses(t *testing.T) {
	code, _, stderr := capture([]string{"read", "run-1", "extra-arg"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "got 2") {
		t.Errorf("stderr should name the too-many-positionals count, got %q", stderr)
	}
}

func TestReadHelpFlagPrintsFullUsage(t *testing.T) {
	code, _, stderr := capture([]string{"read", "-h"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage: phasemeter") {
		t.Errorf("read -h should print the full usage, got %q", stderr)
	}
}

// -- domain-level refusals: parsing was attempted, exit 1 --

func TestReadMissingPriceFileExitsOne(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	code, _, stderr := capture([]string{
		"read", "--repo-root=" + t.TempDir(), "--store-dir=" + t.TempDir(), "--price-file=" + missing, "run-1",
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
	}
}

// -- the end-to-end reference-run read --

// Acceptance (T12): "For a reference run, the read lists every phase with
// its per-model tokens, cost, and efficiency signals, or the declared
// unavailable markers."
func TestReadEndToEnd_ReferenceRun_ListsPhasesTokensCostEfficiency(t *testing.T) {
	repoRoot := newGitRepo(t)
	commitAt(t, repoRoot, "docs/brainstorms/foo.md", "draft\n", "2026-07-01T00:00:00Z")
	commitAt(t, repoRoot, "docs/brainstorms/foo.md", "### D-1 — pick approach\n- **Status**: live\n", "2026-07-01T01:00:00Z")

	storeDir := filepath.Join(repoRoot, "store")
	s := store.New(storeDir)
	if err := s.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo",
		PhaseID: "feature-brainstorm", Harness: "claude-code",
		Spend: map[string]record.ModelSpend{
			"model-a": {TokenQuartet: record.TokenQuartet{Input: 100, Output: 20}, ReconciliationTier: record.TierExact},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo",
		PhaseID: "prd-review", Harness: "cursor",
		Unavailable: []record.Unavailable{{Harness: "cursor", MissingFigure: "per-model spend"}},
	}); err != nil {
		t.Fatal(err)
	}
	priceFile := writePriceFixture(t, t.TempDir())

	code, stdout, stderr := capture([]string{
		"read", "--repo-root=" + repoRoot, "--store-dir=" + storeDir, "--price-file=" + priceFile, "run-1",
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "run run-1 — 2 phase(s)") {
		t.Errorf("stdout should report the phase count, got %q", stdout)
	}
	if !strings.Contains(stdout, "feature-brainstorm") || !strings.Contains(stdout, "model-a") {
		t.Errorf("stdout missing feature-brainstorm/model-a, got %q", stdout)
	}
	if !strings.Contains(stdout, "tokens(input=100 output=20 cache_creation=0 cache_read=0)") {
		t.Errorf("stdout missing token quartet, got %q", stdout)
	}
	if !strings.Contains(stdout, "cost=$0.000600") {
		t.Errorf("stdout missing derived cost, got %q", stdout)
	}
	if !strings.Contains(stdout, "reconciliation=exact") {
		t.Errorf("stdout missing reconciliation tier, got %q", stdout)
	}
	if !strings.Contains(stdout, "questions=1") {
		t.Errorf("stdout missing the feature-brainstorm D-1 efficiency signal, got %q", stdout)
	}
	if !strings.Contains(stdout, "latency=1h0m0s") {
		t.Errorf("stdout missing the git-derived latency, got %q", stdout)
	}
	if !strings.Contains(stdout, "prd-review") {
		t.Errorf("stdout missing the prd-review phase, got %q", stdout)
	}
	if !strings.Contains(stdout, "unavailable: per-model spend (cursor)") {
		t.Errorf("stdout missing the declared unavailable marker, got %q", stdout)
	}
}

func TestReadUnknownRun_ReportsNothingRecordedNotAFault(t *testing.T) {
	priceFile := writePriceFixture(t, t.TempDir())
	code, stdout, stderr := capture([]string{
		"read", "--repo-root=" + t.TempDir(), "--store-dir=" + t.TempDir(), "--price-file=" + priceFile, "no-such-run",
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, `no phases recorded for run "no-such-run"`) {
		t.Errorf("stdout should say nothing was recorded, got %q", stdout)
	}
}

// A model with no price-table entry reads "unpriced" in the report, never a
// fabricated $0 (UB-2), and a figure the record never reconciled reads
// "not-reconciled" rather than silently blank (design §3.4: self-describing).
func TestReadReport_UnpricedModelAndUnreconciledFigure_NeverSilent(t *testing.T) {
	repoRoot := t.TempDir()
	storeDir := t.TempDir()
	s := store.New(storeDir)
	if err := s.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "f", PhaseID: "unknown-phase", Harness: "claude-code",
		Spend: map[string]record.ModelSpend{"never-priced-model": {TokenQuartet: record.TokenQuartet{Input: 10}}},
	}); err != nil {
		t.Fatal(err)
	}
	priceFile := writePriceFixture(t, t.TempDir())

	code, stdout, stderr := capture([]string{
		"read", "--repo-root=" + repoRoot, "--store-dir=" + storeDir, "--price-file=" + priceFile, "run-1",
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "cost=unpriced") {
		t.Errorf("stdout should flag the unpriced model, got %q", stdout)
	}
	if !strings.Contains(stdout, "reconciliation=not-reconciled") {
		t.Errorf("stdout should name the unset reconciliation tier explicitly, got %q", stdout)
	}
}

// The default --store-dir/--repo-root/--price-file all resolve against the
// discovered repo root (t.Chdir is the harness-safe per-test cwd override
// this repo's other cmd tests use — see cmd/pyxis/main_test.go — never a
// raw os.Chdir, which would race a shared process cwd across parallel tests).
func TestReadDefaultsResolveAgainstRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	priceDefault := filepath.Join(root, filepath.FromSlash(price.DefaultRelPath))
	if err := os.MkdirAll(filepath.Dir(priceDefault), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(priceDefault, []byte(readPriceFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	s := store.New(filepath.Join(root, filepath.FromSlash(store.DefaultRelDir)))
	if err := s.Append(record.Record{
		SchemaVersion: record.SchemaVersion, RunID: "run-1", FeatureID: "foo", PhaseID: "unknown-phase", Harness: "claude-code",
	}); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	code, stdout, stderr := capture([]string{"read", "run-1"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "unknown-phase") {
		t.Errorf("stdout should list the recorded phase, got %q", stdout)
	}
}

func TestHelpVerbPrintsUsage(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		code, _, stderr := capture(args)
		if code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
		if !strings.Contains(stderr, "usage: phasemeter") {
			t.Errorf("run(%v) stderr missing usage text, got %q", args, stderr)
		}
	}
}

func recordStoreDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	return filepath.Join(root, ".pyxis", "logs", "phasemeter")
}
