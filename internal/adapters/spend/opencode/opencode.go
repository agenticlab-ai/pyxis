// Package opencode is the pipeline-measurement spend-source adapter for
// OpenCode (docs/tech-design/pipeline-measurement-v2.md §3.7): it shells out
// to OpenCode's own `opencode export <sessionID>` — the CLI's dedicated,
// documented "export session data as JSON" surface — and implements
// internal/phasemeter/spend.Source over the result. Unlike the claude-code
// adapter (which opens its harness's transcript file directly), this
// channel is a subprocess call: OpenCode's own accounting lives in a local
// SQLite database with no public schema, so `export` — not the database
// file — is the harness's best exposed channel (§3.7's "empirical spike",
// Q1). A pure-Go SQLite driver would be a new dependency coupled to an
// undocumented, private schema; the CLI subcommand is stable, documented,
// and already returns exactly the shape this adapter needs.
//
// # Read-only, with one documented, unavoidable caveat
//
// `export` itself never mutates a session/message/part row: two consecutive
// exports of the same session were verified byte-for-byte identical, and
// the store's row counts were unchanged before and after (verified against
// a live OpenCode installation while authoring this adapter). But invoking
// ANY `opencode` subcommand — export included — runs the CLI's own
// bootstrap, which unconditionally upserts a `time_updated` bookkeeping
// timestamp on the project row resolved from the invocation's working
// directory (falling back to a shared "global" project for a non-git
// directory). This is not specific to export and cannot be suppressed by a
// flag; it is bookkeeping metadata about "when was this project last
// touched by any opencode command", never the measured session's own
// content. Load takes the measured run's own working directory expressly so
// this incidental touch lands on the SAME project row OpenCode's own
// ordinary use of that repo already churns constantly, never a foreign one.
//
// # Phase boundaries
//
// A phase opens on either of two structurally distinct markers found while
// investigating OpenCode's real session shape:
//
//   - an assistant message's "skill" tool call (state.input.name) — the
//     model autonomously loading a skill mid-conversation, OpenCode's
//     closest analog to Claude Code's Skill tool; or
//   - a "subtask" part's command field, attached to the user message a
//     command produces when OpenCode routes it through its dedicated
//     command API AND the target agent runs in subagent mode (or the
//     command itself declares subtask: true).
//
// KNOWN GAP, deliberately not covered: a command run through that same
// dedicated API against OpenCode's *primary* agent — the common shape for
// this repo's own pipeline-stage skills run headlessly, e.g. `opencode run
// --command prd-new "..."` — produces a plain text part with no structural
// marker at all of which command produced it, and OpenCode does not persist
// its in-process "command executed" event anywhere on disk once the
// process exits (confirmed against a live installation: the durable event
// log carries only message/session sync events, never a command-execution
// record). This is a real, harness-side gap, not an oversight in this
// package: like the claude-code adapter's own documented heuristic limits,
// this adapter does not need to close it outright to measure a real,
// tool-invoked phase correctly end to end, and drift it causes is exactly
// what a later task's reconciliation against OpenCode's own totals
// (design R2, DINV-4) is designed to catch.
//
// A phase closes on the LAST "write", "edit", or "multiedit" tool call
// (OpenCode's file-mutating tools; all three take the target path as
// state.input.filePath) targeting a docs/ path before the next open marker
// or the end of the record — never the first — so a skill's own drafts
// stay attributed to the still-open phase, mirroring the claude-code
// adapter's coalescing rule (same repo convention, independent of which
// CLI drove it). docs/wiki/ is excluded for the same reason AGENTS.md gives
// there: freely-editable reference material, never a phase's own frozen
// artifact.
//
// Unlike Claude Code, a sub-agent invocation (OpenCode's "task" tool) never
// interleaves its own turns into this session's own message list at all —
// it runs as an entirely separate child session with its own id — so there
// is no sidechain filter to apply here: exporting one session already
// yields only that session's own top-level turns.
//
// Never captured: message text, reasoning content, or a tool's own
// input/output beyond the narrow fields this package reads (the invoked
// skill/command name, a written path) — no secret or credential value an
// exported session might carry is ever retained past this package's parse,
// let alone written to a record (DINV-8).
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// HarnessID is the harness identity this adapter stamps on every record it
// feeds (design DINV-7, UB-4) — the same canonical name T2's phase package
// doc comments and the executor's own OpenCode runtime adapter use for this
// harness.
const HarnessID = "opencode"

// binary is the OpenCode CLI executable.
const binary = "opencode"

// writeTools names OpenCode's file-mutating tools whose target path can be
// a docs/ artifact close marker: "write" creates a file, "edit" and
// "multiedit" (single- and batch-hunk edits) modify one. A "patch"/
// apply_patch call is deliberately not covered — it carries a unified diff
// touching arbitrary paths in free text, not a single filePath field, and
// is not how the pipeline stages available to measure today close (see the
// package doc's close-marker discussion).
var writeTools = map[string]bool{"write": true, "edit": true, "multiedit": true}

var openCodeCommandContext = exec.CommandContext

// turnUsage is one turn's own per-model token accounting, keyed by its
// phase.Turn.Ordinal so Spend can sum exactly the turns a phase.Phase
// collected.
type turnUsage struct {
	model  string
	tokens record.TokenQuartet
}

// Adapter is a parsed OpenCode session export, implementing spend.Source.
// The zero value is not usable — build one with Load or Parse.
type Adapter struct {
	turns []phase.Turn
	usage map[int]turnUsage
}

// Load runs `opencode export <sessionID>` for the session that recorded a
// measured phase and parses its output. workdir is the measured run's own
// working directory (see the package doc's read-only caveat): it becomes
// the subprocess's cwd, so the bookkeeping touch every opencode invocation
// makes lands on the project that repo already owns. sessionID is
// required — Load never falls back to `export`'s own interactive picker
// (which prompts on stdin and would hang a non-interactive recorder) — an
// empty id is a fail-closed error before any subprocess runs.
//
// The child's stdout is captured through a scratch file, never an
// in-memory pipe (verified against a live installation while authoring
// this adapter): opencode is a Node/Bun CLI, and Node's own documented I/O
// model makes a process's stdout writes ASYNCHRONOUS when the destination
// is a pipe — so a large export (any real multi-turn session comfortably
// exceeds the ~64KB pipe buffer) can have its tail silently dropped when
// the process exits before the pipe write drains, which `cmd.Output()`'s
// in-memory pipe capture hits reliably and reproducibly. A regular file's
// writes are SYNCHRONOUS on POSIX, so redirecting stdout straight to one
// sidesteps the hazard entirely rather than papering over it with a retry
// or a larger buffer. The scratch file is this package's own, created and
// removed here — never a file opencode itself manages — so this changes
// nothing about the read-only posture (NR-3).
func Load(ctx context.Context, workdir, sessionID string) (*Adapter, error) {
	if strings.TrimSpace(workdir) == "" {
		return nil, errors.New("phasemeter/adapter/opencode: load: workdir is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("phasemeter/adapter/opencode: load: sessionID is required (never falls back to opencode export's own interactive picker)")
	}

	scratch, err := os.CreateTemp("", "phasemeter-opencode-export-*.json")
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/opencode: load session %s: create scratch file: %w", sessionID, err)
	}
	defer func() {
		_ = scratch.Close()
		_ = os.Remove(scratch.Name())
	}()

	var stderr bytes.Buffer
	cmd := openCodeCommandContext(ctx, binary, "export", sessionID)
	cmd.Dir = workdir
	cmd.Stdout = scratch
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/opencode: load session %s: %w (stderr: %s)",
			sessionID, err, strings.TrimSpace(stderr.String()))
	}

	if _, err := scratch.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/opencode: load session %s: rewind scratch file: %w", sessionID, err)
	}
	a, err := Parse(scratch)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/opencode: load session %s: %w", sessionID, err)
	}
	return a, nil
}

// exportDoc is the subset of `opencode export`'s JSON this adapter reads.
// It is parsed OPEN (unknown fields, including the session-level "info"
// envelope, are ignored) on purpose: this is third-party CLI output that
// grows fields over time, not one of our own schema-versioned config files
// (AP-6's closed-parse rule does not apply to it).
type exportDoc struct {
	Messages []exportMessage `json:"messages"`
}

type exportMessage struct {
	Info  rawMessageInfo `json:"info"`
	Parts []rawPart      `json:"parts"`
}

// rawMessageInfo is the narrow slice of OpenCode's Message.Info this
// package needs. Tokens is a pointer so its absence (every "user" message)
// is distinguishable from a captured-but-zero figure (DINV-3).
type rawMessageInfo struct {
	Role       string     `json:"role"`
	ProviderID string     `json:"providerID"`
	ModelID    string     `json:"modelID"`
	Tokens     *rawTokens `json:"tokens"`
}

type rawTokens struct {
	Input     int64 `json:"input"`
	Output    int64 `json:"output"`
	Reasoning int64 `json:"reasoning"`
	Cache     struct {
		Read  int64 `json:"read"`
		Write int64 `json:"write"`
	} `json:"cache"`
}

// rawPart is one part of a message. Only the fields needed to detect a
// phase boundary are declared; a "text"/"reasoning"/"step-*" part simply
// leaves Tool/State/Command at their zero value, which never matches a
// boundary predicate below (DINV-8: message text itself is never decoded
// past this envelope).
type rawPart struct {
	Type    string       `json:"type"`
	Tool    string       `json:"tool"`    // set when Type == "tool"
	State   rawToolState `json:"state"`   // set when Type == "tool"
	Command string       `json:"command"` // set when Type == "subtask"
}

type rawToolState struct {
	Input rawToolInput `json:"input"`
}

// rawToolInput covers both boundary-relevant tools: Name is the "skill"
// tool's parameter, FilePath is "write"/"edit"/"multiedit"'s.
type rawToolInput struct {
	Name     string `json:"name"`
	FilePath string `json:"filePath"`
}

// Parse decodes raw `opencode export` JSON — a single JSON object, the
// shape the CLI itself prints — into an Adapter. A structurally malformed
// document is a fail-closed error (this is the harness's own accounting,
// not user input to tolerate past); a well-formed document with zero
// messages parses to zero turns.
func Parse(r io.Reader) (*Adapter, error) {
	var doc exportDoc
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse export JSON: %w", err)
	}

	a := &Adapter{usage: map[int]turnUsage{}}
	lastCandidate := -1 // index into a.turns of the pending, not-yet-superseded close

	for _, msg := range doc.Messages {
		isAssistant := msg.Info.Role == "assistant"

		var skillName string
		var artifactWrite bool
		for _, p := range msg.Parts {
			switch {
			case p.Type == "tool" && p.Tool == "skill" && skillName == "":
				if name := p.State.Input.Name; name != "" {
					skillName = name
				}
			case p.Type == "tool" && writeTools[p.Tool]:
				if isDocArtifact(p.State.Input.FilePath) {
					artifactWrite = true
				}
			case p.Type == "subtask" && skillName == "":
				if cmdName := p.Command; cmdName != "" {
					skillName = cmdName
				}
			}
		}

		// A plain user turn (no subtask marker) carries neither a boundary
		// nor any usage — skip it entirely, mirroring the claude-code
		// adapter's "user lines contribute none".
		if !isAssistant && skillName == "" {
			continue
		}

		ordinal := len(a.turns)
		turn := phase.Turn{
			Harness:         HarnessID,
			Ordinal:         ordinal,
			SkillInvocation: skillName,
			ArtifactWrite:   artifactWrite,
		}

		if turn.SkillInvocation != "" {
			lastCandidate = -1 // a new run begins; any pending close already committed
		}
		if turn.ArtifactWrite {
			if lastCandidate >= 0 {
				a.turns[lastCandidate].ArtifactWrite = false // superseded by this later write
			}
			lastCandidate = ordinal
		}

		if isAssistant && msg.Info.Tokens != nil {
			t := msg.Info.Tokens
			a.usage[ordinal] = turnUsage{
				model: msg.Info.ProviderID + "/" + msg.Info.ModelID,
				tokens: record.TokenQuartet{
					Input: t.Input,
					// OpenCode's own stats rollup folds reasoning tokens into
					// output (packages/opencode/src/cli/cmd/stats.ts); this
					// adapter matches that convention rather than inventing a
					// fifth token category the shared record schema (frozen,
					// out of this task's scope) has no slot for.
					Output:        t.Output + t.Reasoning,
					CacheCreation: t.Cache.Write,
					CacheRead:     t.Cache.Read,
				},
			}
		}

		a.turns = append(a.turns, turn)
	}
	return a, nil
}

// nonArtifactDocDirs are docs/ subtrees that are reference material, not a
// phase's own frozen artifact — AGENTS.md documents docs/wiki/ as "freely
// editable via PR" and "never authoritative", the kind of doc a skill may
// touch in passing (a glossary term) without that touch being its close.
var nonArtifactDocDirs = map[string]bool{"wiki": true}

// isDocArtifact reports whether path has a path segment exactly "docs" —
// matching both this repo's relative form (docs/prds/…) and an absolute
// path (/Users/…/pyxis/docs/prds/…), without false-matching an unrelated
// directory that merely contains "docs" as a substring (e.g. "adocs",
// "docs-old") — and whose immediate subtree is not a nonArtifactDocDirs
// entry. Same convention the claude-code adapter applies (design §3.8's
// close marker is a repo convention, independent of which CLI drove it).
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

// Spend implements spend.Source: it sums p's own turns' usage — looked up
// by each turn's Ordinal in the usage this Adapter captured while parsing
// — into a per-model token quartet. A phase with no capturable usage
// (every turn's usage was absent, or p has no turns at all) returns a nil
// map and one Unavailable naming this harness, never a silent zero
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
