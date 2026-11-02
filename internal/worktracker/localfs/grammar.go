package localfs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The on-disk grammar: one work item is a closed front-matter header, a
// free-markdown body, and an append-only log region — parsed by a hand-written
// closed grammar in the manner of docgate's config parser (stdlib only, no YAML
// dependency; docs/tech-design/local-work-tracker.md §3.1). An unknown key, a
// missing required field, or an out-of-set enum value is a named defect (AP-6/
// DINV-11), so a hand-corrupted file fails closed with file, operation, reason,
// and remedy — never a lenient or partial parse.

const (
	fence      = "---"    // front-matter delimiter
	logHeader  = "## Log" // the append-log region marker (empty in the first task)
	parentNone = "—"      // em dash: an unparented item
)

// headerKeys is the closed header field set, in canonical render order. A key
// outside this set is a named defect (closed schema, AP-6/INV-32).
var headerKeys = []string{
	"id", "type", "state", "title", "signals", "assignees",
	"parent", "blocked_by", "created", "updated", "close_reason", "priority",
}

var knownKey = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range headerKeys {
		m[k] = true
	}
	return m
}()

// requiredKeys must each be present; a missing one is a named defect.
var requiredKeys = []string{"id", "type", "state", "title", "created", "updated"}

func gdef(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Render serializes a WorkItem to its normalized on-disk bytes. The output is
// the canonical form Parse round-trips exactly (§3.1); timestamps are RFC 3339
// UTC and lists render inline. The log region is emitted as its header with the
// item's entries beneath (none in the first task).
func Render(w model.WorkItem) []byte {
	var b strings.Builder
	b.WriteString(fence + "\n")
	put := func(k, v string) { fmt.Fprintf(&b, "%s: %s\n", k, v) }
	put("id", strconv.Itoa(w.ID))
	put("type", string(w.Type))
	put("state", string(w.State))
	put("title", w.Title)
	put("signals", renderList(w.Signals))
	put("assignees", renderList(w.Assignees))
	if w.Parent == "" {
		put("parent", parentNone)
	} else {
		put("parent", w.Parent)
	}
	put("blocked_by", renderList(w.BlockedBy))
	put("created", stamp(w.Created))
	put("updated", stamp(w.Updated))
	put("close_reason", string(w.CloseReason))
	put("priority", string(w.Priority))
	b.WriteString(fence + "\n\n")
	if body := strings.TrimRight(w.Body, "\n"); body != "" {
		b.WriteString(body + "\n\n")
	}
	b.WriteString(logHeader + "\n")
	for _, e := range w.Log {
		b.WriteString(renderLogEntry(e))
	}
	return schema.Normalize([]byte(b.String()))
}

// Parse reads on-disk bytes back to a WorkItem under verb, naming file in every
// defect. It fails closed: any structural or closed-set violation returns a
// defect and no usable item, quarantining exactly this file (§3.13, R5).
func Parse(verb, file string, raw []byte) (model.WorkItem, []schema.Defect) {
	var defects []schema.Defect
	bad := func(op, reason, remedy string) {
		defects = append(defects, gdef(file, op, reason, remedy))
	}
	lines := strings.Split(string(schema.Normalize(raw)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != fence {
		bad("parse-header", "missing opening front-matter fence", "an item begins with a "+fence+" line (§3.1)")
		return model.WorkItem{}, stampVerb(verb, defects)
	}
	// Header runs until the closing fence.
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fence {
			end = i
			break
		}
	}
	if end < 0 {
		bad("parse-header", "missing closing front-matter fence", "close the header with a "+fence+" line (§3.1)")
		return model.WorkItem{}, stampVerb(verb, defects)
	}

	w := model.WorkItem{}
	seen := map[string]bool{}
	for i := 1; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			bad("parse-header", fmt.Sprintf("header line %d is not key: value", i+1), "each header line is a single key: value pair (§3.1)")
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if !knownKey[k] {
			bad("parse-header", fmt.Sprintf("unknown header key %q", k), "the header schema is closed: see local-work-tracker §3.1")
			continue
		}
		if seen[k] {
			bad("parse-header", fmt.Sprintf("duplicate header key %q", k), "each header key appears once (§3.1)")
			continue
		}
		seen[k] = true
		parseField(&w, k, v, bad)
	}
	for _, k := range requiredKeys {
		if !seen[k] {
			bad("parse-header", fmt.Sprintf("missing required header key %q", k), "id, type, state, title, created, updated are required (§3.1)")
		}
	}

	// Body is everything between the closing fence and the log header.
	rest := lines[end+1:]
	logAt := -1
	for i, l := range rest {
		if strings.TrimSpace(l) == logHeader {
			logAt = i
			break
		}
	}
	if logAt < 0 {
		bad("parse-body", "missing "+logHeader+" region marker", "an item ends with the "+logHeader+" append-log region (§3.1)")
		return model.WorkItem{}, stampVerb(verb, defects)
	}
	w.Body = strings.Trim(strings.Join(rest[:logAt], "\n"), "\n")

	// The log region is every entry line beneath the header. Transitions are the
	// first kind written (T2); the parse round-trips whatever Render emitted so a
	// mutated item reloads with its full audit trail.
	w.Log = parseLog(rest[logAt+1:], logAt+1+end+1, bad)

	return w, stampVerb(verb, defects)
}

// parseLog reads the append-log region back into typed entries, mirroring
// renderLogEntry's `- kind · author · stamp · namespace · payload` form. A line
// that is not a well-formed entry, or whose kind is out of the closed set
// (DINV-11), is a named defect — the log parses closed like the header. base is
// the 0-based line offset of the region's first line, so a defect names the
// item-file line number.
func parseLog(lines []string, base int, bad func(op, reason, remedy string)) []model.LogEntry {
	var log []model.LogEntry
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineNo := base + i + 1
		body, ok := strings.CutPrefix(line, "- ")
		if !ok {
			bad("parse-log", fmt.Sprintf("log line %d is not a `- ` entry", lineNo), "each log entry is a `- kind · author · timestamp · namespace · payload` line (§3.1)")
			continue
		}
		parts := strings.SplitN(body, " · ", 5)
		if len(parts) != 5 {
			bad("parse-log", fmt.Sprintf("log line %d has %d fields, want 5", lineNo, len(parts)), "a log entry is `kind · author · timestamp · namespace · payload` (§3.1)")
			continue
		}
		kind := model.LogKind(strings.TrimSpace(parts[0]))
		if !model.ValidLogKind(kind) {
			bad("parse-log", fmt.Sprintf("log line %d kind %q is out of set", lineNo, kind), "kind is one of "+join(model.LogKinds)+" (§3.14)")
			continue
		}
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[2]))
		if err != nil {
			bad("parse-log", fmt.Sprintf("log line %d timestamp %q is not RFC 3339", lineNo, parts[2]), "timestamps are RFC 3339 UTC (§3.1)")
			continue
		}
		ns := strings.TrimSpace(parts[3])
		if ns == parentNone {
			ns = ""
		}
		log = append(log, model.LogEntry{
			Kind:      kind,
			Author:    parts[1],
			At:        at.UTC(),
			Namespace: ns,
			Payload:   decodePayload(parts[4]),
		})
	}
	return log
}

// parseField assigns one closed header field, validating enum-valued fields
// against their closed sets (DINV-11). An out-of-set value names the value and
// the allowed set.
func parseField(w *model.WorkItem, k, v string, bad func(op, reason, remedy string)) {
	switch k {
	case "id":
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			bad("parse-header", fmt.Sprintf("id %q is not a positive integer", v), "ids are positive integers (§3.3)")
			return
		}
		w.ID = n
	case "type":
		if t := model.Type(v); model.ValidType(t) {
			w.Type = t
		} else {
			bad("parse-header", fmt.Sprintf("type %q is out of set", v), "type is one of "+join(model.Types)+" (§3.14)")
		}
	case "state":
		if s := model.State(v); model.ValidState(s) {
			w.State = s
		} else {
			bad("parse-header", fmt.Sprintf("state %q is out of set", v), "state is one of "+join(model.States)+" (§3.14)")
		}
	case "title":
		w.Title = v
	case "signals":
		w.Signals = parseList(v)
	case "assignees":
		w.Assignees = parseList(v)
	case "parent":
		if v == parentNone || v == "" {
			w.Parent = ""
		} else {
			w.Parent = v
		}
	case "blocked_by":
		w.BlockedBy = parseList(v)
	case "created":
		w.Created = parseStamp(v, "created", bad)
	case "updated":
		w.Updated = parseStamp(v, "updated", bad)
	case "close_reason":
		if r := model.CloseReason(v); model.ValidCloseReason(r) {
			w.CloseReason = r
		} else {
			bad("parse-header", fmt.Sprintf("close_reason %q is out of set", v), "close_reason is empty or one of "+join(model.CloseReasons)+" (§3.14)")
		}
	case "priority":
		if p := model.Priority(v); model.ValidPriority(p) {
			w.Priority = p
		} else {
			bad("parse-header", fmt.Sprintf("priority %q is out of set", v), "priority is empty or one of "+join(model.Priorities)+" (§3.14)")
		}
	}
}

func parseStamp(v, field string, bad func(op, reason, remedy string)) time.Time {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		bad("parse-header", fmt.Sprintf("%s %q is not RFC 3339", field, v), "timestamps are RFC 3339 UTC (§3.1)")
		return time.Time{}
	}
	return t.UTC()
}

// parseList reads an inline [a, b] list; [] or empty yields no elements.
func parseList(v string) []string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func renderList(xs []string) string { return "[" + strings.Join(xs, ", ") + "]" }

func renderLogEntry(e model.LogEntry) string {
	ns := e.Namespace
	if ns == "" {
		ns = "—"
	}
	return fmt.Sprintf("- %s · %s · %s · %s · %s\n", e.Kind, e.Author, stamp(e.At), ns, encodePayload(e.Payload))
}

// encodePayload keeps a log entry on a single physical line: a comment or
// decision-record payload may carry newlines (multi-line prose), but each entry
// is one line in the on-disk grammar (§3.1), so a newline is encoded as the
// two-character sequence `\n` and a literal backslash is doubled. Transition
// payloads carry neither, so their on-disk form is unchanged (round-trip-stable).
func encodePayload(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// decodePayload reverses encodePayload, restoring newlines and literal
// backslashes when a log entry is parsed back.
func decodePayload(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func join[T ~string](xs []T) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = string(x)
	}
	return strings.Join(parts, "|")
}

// stampVerb fills the CLI verb on every defect the grammar raised, so the
// rendered failure names the verb alongside file, operation, reason, remedy.
func stampVerb(verb string, defects []schema.Defect) []schema.Defect {
	for i := range defects {
		defects[i].Verb = verb
	}
	return defects
}
