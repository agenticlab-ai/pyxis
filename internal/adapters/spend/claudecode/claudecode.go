// Package claudecode is the pipeline-measurement spend-source adapter for
// Claude Code (docs/tech-design/pipeline-measurement-v2.md §3.7): it reads
// Claude Code's own session transcript — the JSONL file it appends one
// line per turn to under ~/.claude/projects/<project>/<session-id>.jsonl —
// and implements internal/phasemeter/spend.Source over it.
//
// A phase brackets open on a Skill tool-call (design §3.8, D-3) and close
// on a Write/Edit tool-call whose target path has a "docs" path segment —
// this repo's convention for "the frozen doc" a pipeline stage produces
// (AGENTS.md: "the documents under docs/ *are* the deliverable") — EXCEPT
// docs/wiki/, which AGENTS.md itself documents as "freely editable" and
// "never authoritative" reference material a skill may touch in passing,
// never a phase's own frozen artifact. Several drafts of the same
// artifact before the true close are common — a skill iterating on its
// own record over many turns — so only the LAST qualifying write before
// the next Skill invocation (or end of record) is treated as the close;
// earlier drafts, and writes to any OTHER doc along the way, stay
// attributed to the still-open phase instead of prematurely popping it.
// This is a heuristic, not a protocol Claude Code guarantees: a true
// nested sub-skill invocation (one skill invoking another before
// finishing its own remaining work) can still close the parent early
// under this rule. That drift is exactly what a later task's
// reconciliation against Claude Code's own totals is designed to catch
// (design R2, DINV-4) — this adapter does not need to eliminate it
// outright to measure a real phase correctly end to end.
//
// Only the top-level conversation (isSidechain == false) is read: a
// sub-agent's own turns run as a separate thread interleaved in FILE
// order, not conversation order, and folding them into one linear turn
// sequence would scramble phase brackets rather than clarify them.
//
// Never captured: prompt/response text, thinking content, or tool
// input/output beyond the narrow fields needed to detect a boundary (the
// invoked skill name, a written path) — no secret or credential value a
// transcript might carry is ever retained past this package's parse, let
// alone written to a record (DINV-8).
package claudecode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/phasemeter/phase"
	"github.com/agenticlab-ai/pyxis/internal/phasemeter/record"
)

// HarnessID is the harness identity this adapter stamps on every record it
// feeds (design DINV-7, UB-4) — the same canonical name T2's phase package
// doc comments use for this harness.
const HarnessID = "claude-code"

// writeTools names this repo's file-mutating tools whose target path can
// be a docs/ artifact close marker. Bash-driven writes (e.g. a `gh pr
// create`) are deliberately not covered — the pipeline stages available to
// measure today all close on a doc write, not a PR (see the package doc).
var writeTools = map[string]bool{"Write": true, "Edit": true}

// turnUsage is one turn's own per-model token accounting, keyed by its
// phase.Turn.Ordinal so Spend can sum exactly the turns a phase.Phase
// collected.
type turnUsage struct {
	model  string
	tokens record.TokenQuartet
}

// Adapter is a parsed Claude Code session transcript, implementing
// spend.Source. The zero value is not usable — build one with Load or
// Parse.
type Adapter struct {
	turns []phase.Turn
	usage map[int]turnUsage
}

// Load reads and parses the Claude Code session transcript at path — the
// harness's own JSONL accounting file. Reading never mutates path (NR-1,
// NR-3): the file is opened read-only.
func Load(path string) (*Adapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/claudecode: load %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only handle

	a, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("phasemeter/adapter/claudecode: load %s: %w", path, err)
	}
	return a, nil
}

// Parse parses raw Claude Code session-transcript JSONL — one JSON object
// per line, the shape Claude Code itself appends — into an Adapter. A
// blank line is skipped; a non-blank line that fails to parse is a fail-
// closed error (this is the harness's own accounting, not user input —
// a malformed line means something is wrong, never something to guess
// past).
func Parse(r io.Reader) (*Adapter, error) {
	a := &Adapter{usage: map[int]turnUsage{}}
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

// rawLine is the envelope every transcript line shares. message stays raw
// here: a "user" line's message.content is a bare string, while an
// "assistant" line's is an array of blocks — decoding it only once the
// type is known avoids that shape mismatch entirely, and this package
// only ever looks past the envelope for "assistant" lines.
type rawLine struct {
	Type        string          `json:"type"`
	IsSidechain bool            `json:"isSidechain"`
	Message     json.RawMessage `json:"message"`
}

type rawMessage struct {
	Model   string       `json:"model"`
	Content []rawContent `json:"content"`
	Usage   *rawUsage    `json:"usage"`
}

// rawContent is one content block of an assistant message. Only the two
// narrow input fields this package needs to detect a boundary are
// declared — encoding/json silently drops every other key (prompt text,
// thinking, a tool's other arguments, …) rather than retaining it, which
// is what keeps a secret embedded elsewhere in a turn out of this
// package entirely (DINV-8).
type rawContent struct {
	Type  string       `json:"type"`
	Name  string       `json:"name"`
	Input rawToolInput `json:"input"`
}

type rawToolInput struct {
	Skill    string `json:"skill"`
	FilePath string `json:"file_path"`
}

type rawUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// parseLine turns one non-blank transcript line into this Adapter's next
// turn (if it contributes one) and updates lastCandidate — the
// last-write-wins coalescing state the package doc describes — in place.
func (a *Adapter) parseLine(line string, lastCandidate *int) error {
	var raw rawLine
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return fmt.Errorf("parse transcript line: %w", err)
	}
	if raw.Type != "assistant" || raw.IsSidechain {
		return nil
	}
	var msg rawMessage
	if err := json.Unmarshal(raw.Message, &msg); err != nil {
		return fmt.Errorf("parse assistant message: %w", err)
	}

	ordinal := len(a.turns)
	turn := phase.Turn{Harness: HarnessID, Ordinal: ordinal}
	for _, c := range msg.Content {
		if c.Type != "tool_use" {
			continue
		}
		if c.Name == "Skill" && c.Input.Skill != "" && turn.SkillInvocation == "" {
			turn.SkillInvocation = c.Input.Skill
		}
		if writeTools[c.Name] && isDocArtifact(c.Input.FilePath) {
			turn.ArtifactWrite = true
		}
	}

	if turn.SkillInvocation != "" {
		*lastCandidate = -1 // a new run begins; any pending close already committed
	}
	if turn.ArtifactWrite {
		if *lastCandidate >= 0 {
			a.turns[*lastCandidate].ArtifactWrite = false // superseded by this later write
		}
		*lastCandidate = ordinal
	}

	if msg.Usage != nil {
		a.usage[ordinal] = turnUsage{
			model: msg.Model,
			tokens: record.TokenQuartet{
				Input:         msg.Usage.InputTokens,
				Output:        msg.Usage.OutputTokens,
				CacheCreation: msg.Usage.CacheCreationInputTokens,
				CacheRead:     msg.Usage.CacheReadInputTokens,
			},
		}
	}

	a.turns = append(a.turns, turn)
	return nil
}

// nonArtifactDocDirs are docs/ subtrees that are reference material, not a
// phase's own frozen artifact — AGENTS.md documents docs/wiki/ as "freely
// editable via PR" and "never authoritative", the kind of doc a skill may
// touch in passing (a glossary term) without that touch being its close.
var nonArtifactDocDirs = map[string]bool{"wiki": true}

// isDocArtifact reports whether path has a path segment exactly "docs" —
// matching both this repo's relative form (docs/prds/…) and the absolute
// paths Claude Code's own tool calls carry (/Users/…/pyxis/docs/prds/…),
// without false-matching an unrelated directory that merely contains
// "docs" as a substring (e.g. "adocs", "docs-old") — and whose immediate
// subtree is not a nonArtifactDocDirs entry.
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
