// Package cursor is the pipeline-measurement spend-source adapter for
// Cursor CLI (docs/tech-design/pipeline-measurement-v2.md §3.7): it reads
// cursor-agent's own headless event stream — the NDJSON `--print
// --output-format stream-json` produces
// (cursor.com/docs/cli/reference/output-format) — and implements
// internal/phasemeter/spend.Source over it.
//
// Unlike Claude Code, cursor-agent does not persist a discoverable local
// session-transcript store: there is no on-disk equivalent of
// ~/.claude/projects/<project>/<session-id>.jsonl to read after the fact
// (confirmed empirically: a real machine's ~/.cursor/projects/<project>
// directories hold only MCP-server cache metadata, never a chat/session
// record). The stream-json event stream IS cursor-agent's own accounting
// record for a run; a caller captures it by redirecting a headless
// invocation's stdout to a file (`cursor-agent --print --output-format
// stream-json ... > session.jsonl`), and Load reads that captured file —
// the same *usage* pattern as the Claude Code adapter's Load(path), even
// though the source of the file differs (Claude Code writes it
// unprompted; Cursor's caller must capture it).
//
// A phase brackets open on the first "user" event whose text begins with
// a recognized "/<command>" form — cursor-agent's own slash-command
// convention, the alternative open-marker form design §3.8 names
// alongside a dedicated tool call ("a Skill tool-call OR a
// slash-command"). Cursor's stream-json tool_call taxonomy (readToolCall /
// writeToolCall / editToolCall / shellToolCall / listToolCall /
// searchToolCall — no "invoke this skill" tool) carries no equivalent to
// Claude Code's own Skill tool call, so the slash-command form is the one
// this adapter actually has to work with; this is a heuristic, not a
// protocol cursor-agent guarantees — an interactive session's own meta
// command (e.g. "/help") would open a same-shaped, spend-empty phase this
// adapter cannot distinguish from a real pipeline skill invocation without
// hard-coding this repo's skill catalog into a harness adapter, which
// would overreach this package's job. A phase closes on the LAST
// completed, successful writeToolCall/editToolCall whose target path has
// a "docs" path segment before the next open marker (or end of stream) —
// the same "the frozen doc is the close" / docs/wiki/-is-not-an-artifact
// convention the Claude Code adapter documents and applies, carried over
// unchanged because it is this repo's own pipeline convention, not a
// Claude-Code-specific one. A write/edit that never reaches "completed"
// with result.success never closes a phase — an attempted-but-failed
// write produced no frozen artifact.
//
// Spend is unconditionally declared unavailable (DINV-3): as of this
// writing, cursor-agent's `json`/`stream-json` result envelope carries no
// token, cost, or per-model usage field at all — confirmed against
// Cursor's own published schema (cursor.com/docs/cli/reference/output-format:
// the terminal `result` event is exactly `type, subtype, is_error,
// duration_ms, duration_api_ms, result, session_id, request_id` — no usage
// field) and against three independent, still-unresolved Cursor community
// feature requests asking for exactly this (Nov 2025, Feb 2026, and one
// closed unanswered Jul 2026 — forum.cursor.com topics 146980/152.../156872).
// This is this design's own predicted degradation case (§3.7 R1: "Cursor's
// proxy billing the prime suspect"; PRD FR-5's interface row: "no
// per-model usage on the user's key → FR-6") — Cursor proxies the
// underlying model call itself rather than the user's own key hitting the
// provider directly, so there is no per-model figure on the user's own
// key to read, on any local channel. The one place Cursor usage data does
// exist is the account's web billing dashboard, reachable only by
// scraping an authenticated browser session cookie (the third-party
// "tokscale" usage tracker documents resorting to exactly this for
// Cursor, explicitly because — its own words — it "does not parse local
// Cursor Agent CLI state") — not a stable, local, "the user's own key"
// channel, and not one this adapter adopts: it would require network
// egress and a manually-extracted browser credential, cutting against
// this whole tool's local/offline, zero-new-egress posture (design §3.2,
// §3.5). A declared unavailable here is this design's own anticipated,
// first-class outcome (§3.7 R1), not a gap in this adapter.
//
// Never captured: prompt/response text beyond the narrow command-name
// prefix needed to detect the open marker, or any tool-call argument
// beyond its target path — no secret or credential value a captured
// stream might carry is ever retained past this package's parse, let
// alone written to a record (DINV-8).
package cursor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// HarnessID is the harness identity this adapter stamps on every record it
// feeds (design DINV-7, UB-4) — the same canonical name T2's phase package
// doc comments use for this harness.
const HarnessID = "cursor"

// rawEvent is the envelope every stream-json line shares
// (cursor.com/docs/cli/reference/output-format): a "type" discriminator
// ("system", "user", "assistant", "tool_call", "result"), a "subtype" for
// that type's own sub-states (e.g. tool_call: "started"/"completed"), and
// a type-specific payload this package decodes only once type/subtype
// narrow which shape applies.
type rawEvent struct {
	Type     string          `json:"type"`
	Subtype  string          `json:"subtype"`
	Message  json.RawMessage `json:"message"`
	ToolCall json.RawMessage `json:"tool_call"`
}

// rawMessage is a "user"/"assistant" event's message: an array of typed
// content blocks.
type rawMessage struct {
	Content []rawContent `json:"content"`
}

// rawContent is one content block. Only its text is read, and only far
// enough to extract a leading slash-command name (see commandName) — the
// full text is never retained (DINV-8).
type rawContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// rawToolCall is a "tool_call" event's payload: the *ToolCall family
// cursor-agent's stream-json emits, keyed by tool. Only the two
// file-mutating members this package's close heuristic needs are
// declared; the other members (readToolCall, shellToolCall, listToolCall,
// searchToolCall) are intentionally undeclared — encoding/json drops
// them, so their arguments/results (which can carry file content or shell
// output) never reach this package (DINV-8).
type rawToolCall struct {
	WriteToolCall *rawFileToolCall `json:"writeToolCall"`
	EditToolCall  *rawFileToolCall `json:"editToolCall"`
}

// rawFileToolCall is a write/edit tool call's own envelope: the target
// path (never its content — DINV-8) and, once the call completes, whether
// it succeeded.
type rawFileToolCall struct {
	Args   rawFileArgs    `json:"args"`
	Result *rawToolResult `json:"result"`
}

type rawFileArgs struct {
	Path string `json:"path"`
}

type rawToolResult struct {
	Success bool `json:"success"`
}

// Adapter is a parsed cursor-agent stream-json event stream, implementing
// spend.Source. The zero value is not usable — build one with Load or
// Parse.
type Adapter struct {
	turns []phase.Turn
}

// Load reads and parses a captured cursor-agent stream-json event stream
// at path (see the package doc: cursor-agent itself does not persist this
// anywhere for Load to find unprompted — path is wherever the caller
// captured a headless run's stdout). Reading never mutates path (NR-1,
// NR-3): the file is opened read-only.
func Load(path string) (*Adapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/cursor: load %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle

	a, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/cursor: load %s: %w", path, err)
	}
	return a, nil
}

// Parse parses a raw cursor-agent stream-json event stream — one JSON
// object per line — into an Adapter. A blank line is skipped; a
// non-blank line that fails to parse is a fail-closed error (this is the
// harness's own accounting, not user input — a malformed line means
// something is wrong, never something to guess past).
func Parse(r io.Reader) (*Adapter, error) {
	a := &Adapter{}
	lastCandidate := -1 // index into a.turns of the pending, not-yet-superseded close

	reader := bufio.NewReader(r)
	lineNo := 0
	for {
		line, rerr := reader.ReadString('\n')
		lineNo++
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if perr := a.parseLine(trimmed, &lastCandidate); perr != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, perr)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return nil, fmt.Errorf("line %d: %w", lineNo, rerr)
		}
	}
	return a, nil
}

// parseLine turns one non-blank stream-json line into this Adapter's next
// turn, if it contributes one. "system" (session init) and "result" (the
// run's terminal summary) are envelope/meta events, not turns in the
// harness's own conversation — they carry no boundary signal and are
// never attributed to a phase.
func (a *Adapter) parseLine(line string, lastCandidate *int) error {
	var raw rawEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return fmt.Errorf("parse stream-json line: %w", err)
	}
	switch raw.Type {
	case "user":
		return a.pushUser(raw, lastCandidate)
	case "assistant":
		a.turns = append(a.turns, phase.Turn{Harness: HarnessID, Ordinal: len(a.turns)})
		return nil
	case "tool_call":
		return a.pushToolCall(raw, lastCandidate)
	default:
		return nil
	}
}

// pushUser turns a "user" event into this Adapter's next turn, setting
// SkillInvocation when the message's text begins with a recognized
// "/<command>" form — the only phase-open signal available to this
// adapter (see the package doc). Only the leading command token is ever
// read out of the message text; the remainder — which may carry a secret
// a user typed into their own prompt — is discarded unread past this line
// (DINV-8).
func (a *Adapter) pushUser(raw rawEvent, lastCandidate *int) error {
	var msg rawMessage
	if len(raw.Message) > 0 {
		if err := json.Unmarshal(raw.Message, &msg); err != nil {
			return fmt.Errorf("parse user message: %w", err)
		}
	}

	turn := phase.Turn{Harness: HarnessID, Ordinal: len(a.turns)}
	turn.SkillInvocation = commandName(msg)
	if turn.SkillInvocation != "" {
		*lastCandidate = -1 // a new run begins; any pending close already committed
	}
	a.turns = append(a.turns, turn)
	return nil
}

// commandName returns the slash-command name msg's first text block
// opens, if any — e.g. "/prd-new draft the PRD" yields "prd-new" — by
// reading only up to the first whitespace after the leading '/'. Nothing
// past that point in the text (the command's own arguments, any further
// content block) is ever inspected (DINV-8).
func commandName(msg rawMessage) string {
	for _, c := range msg.Content {
		if c.Type != "text" {
			continue
		}
		text := strings.TrimSpace(c.Text)
		if !strings.HasPrefix(text, "/") {
			return ""
		}
		rest := text[1:]
		if i := strings.IndexFunc(rest, unicode.IsSpace); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	return ""
}

// pushToolCall turns a "tool_call" event into this Adapter's next turn.
// Only the "completed" subtype is turned into a turn — "started" reports
// the same call with no path and no result yet, and would otherwise
// double-count one tool call as two turns. A write/edit that completed
// successfully against a docs/ path is this phase's close-candidate,
// coalesced last-write-wins exactly as the Claude Code adapter's own
// multi-draft handling does (see the package doc): several drafts of the
// same artifact before the true close are common, so only the LAST
// qualifying write before the next open marker (or end of stream) is
// treated as the close.
func (a *Adapter) pushToolCall(raw rawEvent, lastCandidate *int) error {
	if raw.Subtype != "completed" {
		return nil
	}
	var tc rawToolCall
	if len(raw.ToolCall) > 0 {
		if err := json.Unmarshal(raw.ToolCall, &tc); err != nil {
			return fmt.Errorf("parse tool_call event: %w", err)
		}
	}

	ordinal := len(a.turns)
	turn := phase.Turn{Harness: HarnessID, Ordinal: ordinal}
	if call := writeOrEdit(tc); call != nil && call.Result != nil && call.Result.Success && isDocArtifact(call.Args.Path) {
		turn.ArtifactWrite = true
	}

	if turn.ArtifactWrite {
		if *lastCandidate >= 0 {
			a.turns[*lastCandidate].ArtifactWrite = false // superseded by this later write
		}
		*lastCandidate = ordinal
	}

	a.turns = append(a.turns, turn)
	return nil
}

// writeOrEdit returns tc's file-mutating call, if it is one — a
// writeToolCall or an editToolCall — and nil for any other tool (read,
// shell, list, search: none of these can close a phase).
func writeOrEdit(tc rawToolCall) *rawFileToolCall {
	if tc.WriteToolCall != nil {
		return tc.WriteToolCall
	}
	return tc.EditToolCall
}

// nonArtifactDocDirs are docs/ subtrees that are reference material, not a
// phase's own frozen artifact — AGENTS.md documents docs/wiki/ as "freely
// editable via PR" and "never authoritative", the kind of doc a skill may
// touch in passing (a glossary term) without that touch being its close.
// This is this repo's own pipeline convention, not specific to Cursor —
// the Claude Code adapter (internal/adapters/spend/claudecode)
// documents and applies the identical rule over its own harness's
// tool-call shape.
var nonArtifactDocDirs = map[string]bool{"wiki": true}

// isDocArtifact reports whether path has a path segment exactly "docs" —
// matching both this repo's relative form (docs/prds/…) and an absolute
// workspace path (/repo/docs/prds/…), without false-matching an unrelated
// directory that merely contains "docs" as a substring (e.g. "adocs",
// "docs-old") — and whose immediate subtree is not a nonArtifactDocDirs
// entry.
func isDocArtifact(path string) bool {
	if path == "" {
		return false
	}
	segs := strings.Split(filepath.ToSlash(path), "/")
	for i, seg := range segs {
		if seg != "docs" {
			continue
		}
		return i+1 >= len(segs) || !nonArtifactDocDirs[segs[i+1]]
	}
	return false
}

// Harness implements spend.Source.
func (a *Adapter) Harness() string { return HarnessID }

// Turns implements spend.Source.
func (a *Adapter) Turns() []phase.Turn { return a.turns }

// Spend implements spend.Source. It always declares p's per-model spend
// unavailable: cursor-agent's local accounting — the only channel this
// adapter (or anyone, short of scraping an authenticated web session; see
// the package doc) can read on the user's own key — carries no token,
// cost, or per-model usage field at all, for any phase. This is this
// design's own anticipated degradation case (design §3.7 R1; PRD FR-5's
// interface row), not a gap in this adapter.
func (a *Adapter) Spend(phase.Phase) (map[string]record.ModelSpend, []record.Unavailable) {
	return nil, []record.Unavailable{{Harness: HarnessID, MissingFigure: "per-model spend"}}
}
