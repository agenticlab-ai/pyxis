// Package codex is the pipeline-measurement spend-source adapter for Codex
// CLI (docs/tech-design/pipeline-measurement-v2.md §3.7): it reads Codex's
// own session transcript — the JSONL file it appends one line per event to
// under ~/.codex/sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<id>.jsonl —
// and implements internal/phasemeter/spend.Source over it.
//
// Codex's own transcript shape (the empirical capability-matrix spike for
// this harness, T6) differs from Claude Code's in three ways this adapter's
// parse reflects:
//
//   - Model and usage are reported on two SEPARATE top-level record kinds,
//     not one assistant message: a "turn_context" record names the model
//     active for everything that follows it, and a "token_count" event_msg —
//     one per underlying model completion, several of which can occur within
//     a single user-visible Codex turn (a task_started…task_complete span) —
//     carries that one completion's own usage in info.last_token_usage. This
//     is the same per-completion granularity Claude Code's per-assistant-
//     message usage gives; Codex just splits "which model" and "how much
//     usage" across two record kinds instead of one. (info.total_token_usage
//     is a separately-accumulated running session-wide sum — successive
//     last_token_usage.total_tokens deltas sum to it exactly, confirming
//     last_token_usage is genuinely that one completion's own figure, not
//     already-aggregated — but this adapter reads only last_token_usage;
//     the running total is not needed here.) Each token_count is this
//     adapter's own phase.Turn unit.
//
//   - Codex has no distinct "invoke a skill" tool call: this repo's own
//     convention (for example, mentioning `$prd-new`) is a
//     plain-text `$skill-name` mention in the user's own message, not a
//     structural field. The open marker is therefore a regex match against
//     event_msg/user_message text, restricted to a hyphenated, lowercase
//     identifier — a POSIX shell variable name cannot contain a hyphen, so
//     requiring one is what keeps an incidental `$VAR`-style shell
//     reference (common in pasted command text) from being mistaken for a
//     skill mention. A single-word skill mention (e.g. "$argus") is
//     indistinguishable from an ordinary shell variable through text alone
//     and is a known gap — the same kind of heuristic-not-protocol caveat
//     the claudecode adapter documents for its own artifact-close
//     detection.
//
//   - The artifact-write close marker is Codex's own "patch_apply_end"
//     event_msg — fired when Codex's patch-apply tool actually writes
//     files, naming every changed path in `changes` — rather than a
//     per-file tool call's input, and counts only when `success` is true
//     (an unsuccessful patch changed nothing). A patch can touch several
//     files in one shot; as with Claude Code, any changed path with a
//     "docs" path segment (excluding docs/wiki/, per AGENTS.md) closes the
//     phase.
//
// A Codex sub-agent (the spawn_agent/wait_agent/send_message multi-agent
// tools) runs as an entirely separate rollout file with its own session
// id — never inlined in the parent's file the way Claude Code's
// isSidechain turns are — so, unlike claudecode.go, this adapter needs no
// sidechain filter: reading one rollout file is already exactly one
// top-level thread.
//
// Only the narrow fields each record kind needs are ever decoded — this
// adapter never unmarshals a "response_item" (the prompt/reasoning/tool-call
// content, and a patch's own unified-diff body) at all, so no secret or
// credential value a transcript might carry is ever retained past this
// package's parse, let alone written to a record (DINV-8).
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// HarnessID is the harness identity this adapter stamps on every record it
// feeds (design DINV-7, UB-4) — the same bare name phase.Turn's own doc
// comment uses as this harness's example id.
const HarnessID = "codex"

// skillMention matches this repo's Codex skill-invocation convention: a
// literal "$skill-name" token, restricted to a lowercase, hyphenated
// identifier (see the package doc for why the hyphen requirement matters).
var skillMention = regexp.MustCompile(`\$([a-z][a-z0-9]*(?:-[a-z0-9]+)+)`)

// nonArtifactDocDirs are docs/ subtrees that are reference material, not a
// phase's own frozen artifact — mirrors the claudecode adapter's own rule:
// AGENTS.md documents docs/wiki/ as "freely editable via PR" and "never
// authoritative", the kind of doc a skill may touch in passing without
// that touch being its close.
var nonArtifactDocDirs = map[string]bool{"wiki": true}

// isDocArtifact reports whether path has a path segment exactly "docs" —
// matching both this repo's relative form and the absolute paths Codex's
// own patch_apply_end events carry — whose immediate subtree is not a
// nonArtifactDocDirs entry. Same predicate as claudecode.go's, applied
// here to Codex's own changed-path shape.
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

// anyDocArtifact reports whether any key of changes (Codex's own
// patch_apply_end.changes map, keyed by changed path) is a doc artifact.
func anyDocArtifact(changes map[string]json.RawMessage) bool {
	for path := range changes {
		if isDocArtifact(path) {
			return true
		}
	}
	return false
}

// turnUsage is one token_count event's own per-model usage, keyed by its
// phase.Turn.Ordinal so Spend can sum exactly the turns a phase.Phase
// collected.
type turnUsage struct {
	model  string
	tokens record.TokenQuartet
}

// Adapter is a parsed Codex session transcript, implementing spend.Source.
// The zero value is not usable — build one with Load or Parse.
type Adapter struct {
	turns []phase.Turn
	usage map[int]turnUsage
}

// Load reads and parses the Codex session transcript at path — the
// harness's own JSONL rollout file. Reading never mutates path (NR-1,
// NR-3): the file is opened read-only.
func Load(path string) (*Adapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/codex: load %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle

	a, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/codex: load %s: %w", path, err)
	}
	return a, nil
}

// Parse parses raw Codex session-transcript JSONL — one JSON object per
// line, the shape Codex CLI itself appends to a rollout file — into an
// Adapter. A blank line is skipped; a non-blank line that fails to parse is
// a fail-closed error (this is the harness's own accounting, not user
// input — a malformed line means something is wrong, never something to
// guess past).
func Parse(r io.Reader) (*Adapter, error) {
	a := &Adapter{usage: map[int]turnUsage{}}
	var st parseState

	reader := bufio.NewReader(r)
	lineNo := 0
	for {
		line, rerr := reader.ReadString('\n')
		lineNo++
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if perr := a.parseLine(trimmed, &st); perr != nil {
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
	// A marker with no token_count left to land on — the transcript ended
	// right after a skill mention or a patch apply, before Codex reported
	// any billed completion for it — still becomes its own turn, carrying
	// no usage, so phase.Detect still brackets it instead of silently
	// losing the boundary (UB-3: never dropped). With no usage entry for
	// it, Spend declares Unavailable for a phase that ends up resting on
	// only this turn, rather than fabricating a figure (DINV-3).
	if st.pendingSkill != "" || st.pendingArtifactWrite {
		a.turns = append(a.turns, phase.Turn{
			Harness:         HarnessID,
			Ordinal:         len(a.turns),
			SkillInvocation: st.pendingSkill,
			ArtifactWrite:   st.pendingArtifactWrite,
		})
	}
	return a, nil
}

// parseState carries the pending, not-yet-attributed markers across lines:
// the model named by the most recently seen turn_context, and a skill
// mention / artifact-write waiting to land on the next token_count's turn
// (design §3.8: the same "rides the accounting record" join claudecode.go
// uses, expressed over Codex's two-record-kind shape instead of one).
type parseState struct {
	model                string
	pendingSkill         string
	pendingArtifactWrite bool
}

// rawLine is the envelope every rollout line shares.
type rawLine struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// rawTurnContext is the only turn_context field this package needs: the
// model active for every token_count until the next turn_context.
type rawTurnContext struct {
	Model string `json:"model"`
}

// rawEventType discriminates an event_msg payload before it is decoded
// further — mirroring the two-stage decode claudecode.go uses for its own
// envelope, since event_msg's own shape varies by its type field.
type rawEventType struct {
	Type string `json:"type"`
}

// rawUserMessage is the only user_message field this package needs: the
// message text, scanned (never retained) for a skill mention.
type rawUserMessage struct {
	Message string `json:"message"`
}

// rawTokenCount is the only token_count field this package needs: the
// reporting completion's own usage.
type rawTokenCount struct {
	Info struct {
		LastTokenUsage struct {
			InputTokens       int64 `json:"input_tokens"`
			CachedInputTokens int64 `json:"cached_input_tokens"`
			OutputTokens      int64 `json:"output_tokens"`
		} `json:"last_token_usage"`
	} `json:"info"`
}

// rawPatchApplyEnd is the only patch_apply_end fields this package needs:
// whether the patch actually applied, and which paths it touched. The
// changes map's values (each a unified diff of the written content) are
// left undecoded (json.RawMessage, discarded) — DINV-8: a patch to a docs/
// artifact can easily carry user content, and this package never looks
// past the path itself.
type rawPatchApplyEnd struct {
	Success bool                       `json:"success"`
	Changes map[string]json.RawMessage `json:"changes"`
}

// parseLine turns one non-blank transcript line into this Adapter's next
// turn (if it is a token_count) and updates st — the pending-marker state
// the package doc describes — in place.
func (a *Adapter) parseLine(line string, st *parseState) error {
	var raw rawLine
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return fmt.Errorf("parse session line: %w", err)
	}

	switch raw.Type {
	case "turn_context":
		var tc rawTurnContext
		if err := json.Unmarshal(raw.Payload, &tc); err != nil {
			return fmt.Errorf("parse turn_context: %w", err)
		}
		st.model = tc.Model

	case "event_msg":
		var et rawEventType
		if err := json.Unmarshal(raw.Payload, &et); err != nil {
			return fmt.Errorf("parse event_msg: %w", err)
		}
		switch et.Type {
		case "user_message":
			var um rawUserMessage
			if err := json.Unmarshal(raw.Payload, &um); err != nil {
				return fmt.Errorf("parse user_message: %w", err)
			}
			if m := skillMention.FindStringSubmatch(um.Message); m != nil {
				st.pendingSkill = m[1]
			}

		case "patch_apply_end":
			var pa rawPatchApplyEnd
			if err := json.Unmarshal(raw.Payload, &pa); err != nil {
				return fmt.Errorf("parse patch_apply_end: %w", err)
			}
			if pa.Success && anyDocArtifact(pa.Changes) {
				st.pendingArtifactWrite = true
			}

		case "token_count":
			var tk rawTokenCount
			if err := json.Unmarshal(raw.Payload, &tk); err != nil {
				return fmt.Errorf("parse token_count: %w", err)
			}
			ordinal := len(a.turns)
			a.turns = append(a.turns, phase.Turn{
				Harness:         HarnessID,
				Ordinal:         ordinal,
				SkillInvocation: st.pendingSkill,
				ArtifactWrite:   st.pendingArtifactWrite,
			})
			st.pendingSkill = ""
			st.pendingArtifactWrite = false

			lu := tk.Info.LastTokenUsage
			a.usage[ordinal] = turnUsage{
				model: st.model,
				// OpenAI-style accounting reports cached_input_tokens as a
				// SUBSET of input_tokens (unlike Claude Code, where
				// input/cache_creation/cache_read are disjoint and all sum
				// to the prompt total). Subtracting here restores that
				// same disjoint-parts invariant, so a later sum over the
				// quartet's fields reproduces the true total instead of
				// double-counting the cached portion.
				tokens: record.TokenQuartet{
					Input:     lu.InputTokens - lu.CachedInputTokens,
					Output:    lu.OutputTokens,
					CacheRead: lu.CachedInputTokens,
				},
			}
		}
	}
	return nil
}

// Harness implements spend.Source.
func (a *Adapter) Harness() string { return HarnessID }

// Turns implements spend.Source.
func (a *Adapter) Turns() []phase.Turn { return a.turns }

// Spend implements spend.Source: it sums p's own turns' usage — looked up
// by each turn's Ordinal in the usage this Adapter captured while
// parsing — into a per-model token quartet. A phase with no capturable
// usage (every turn's usage was absent, or p has no turns at all) returns
// a nil map and one Unavailable naming this harness, never a silent zero
// (DINV-3).
func (a *Adapter) Spend(p phase.Phase) (map[string]record.ModelSpend, []record.Unavailable) {
	spend := map[string]record.ModelSpend{}
	for _, t := range p.Turns {
		u, ok := a.usage[t.Ordinal]
		if !ok {
			continue
		}
		ms := spend[u.model]
		ms.Input += u.tokens.Input
		ms.Output += u.tokens.Output
		ms.CacheCreation += u.tokens.CacheCreation
		ms.CacheRead += u.tokens.CacheRead
		spend[u.model] = ms
	}
	if len(spend) == 0 {
		return nil, []record.Unavailable{{Harness: HarnessID, MissingFigure: "per-model spend"}}
	}
	return spend, nil
}
