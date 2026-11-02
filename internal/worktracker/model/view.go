package model

import "time"

// ItemView is the schema-versioned machine (JSON) projection of a WorkItem
// (prd:local-work-tracker/FR-15). It is a stable, additive contract: the
// `schema` tag pins the shape, timestamps are RFC 3339 UTC, and every header
// field plus the body round-trips. The log is projected as its typed entries
// (empty in the first task). This view — not the Go struct — is what `carina
// show --json` emits and what the golden schema validates.
type ItemView struct {
	Schema      string         `json:"schema"`
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	State       string         `json:"state"`
	Title       string         `json:"title"`
	Signals     []string       `json:"signals"`
	Assignees   []string       `json:"assignees"`
	Parent      string         `json:"parent"`
	BlockedBy   []string       `json:"blocked_by"`
	Created     string         `json:"created"`
	Updated     string         `json:"updated"`
	CloseReason string         `json:"close_reason"`
	Priority    string         `json:"priority"`
	Body        string         `json:"body"`
	Log         []LogEntryView `json:"log"`
}

// LogEntryView is the machine projection of one append-log entry.
type LogEntryView struct {
	Kind      string `json:"kind"`
	Author    string `json:"author"`
	At        string `json:"at"`
	Namespace string `json:"namespace"`
	Payload   string `json:"payload"`
}

// View projects a WorkItem onto its schema-versioned machine form. Slices are
// always present (never null) so a consumer reads a stable shape.
func (w WorkItem) View() ItemView {
	v := ItemView{
		Schema:      SchemaVersion,
		ID:          w.ID,
		Type:        string(w.Type),
		State:       string(w.State),
		Title:       w.Title,
		Signals:     nonNil(w.Signals),
		Assignees:   nonNil(w.Assignees),
		Parent:      w.Parent,
		BlockedBy:   nonNil(w.BlockedBy),
		Created:     stamp(w.Created),
		Updated:     stamp(w.Updated),
		CloseReason: string(w.CloseReason),
		Priority:    string(w.Priority),
		Body:        w.Body,
		Log:         []LogEntryView{},
	}
	for _, e := range w.Log {
		v.Log = append(v.Log, LogEntryView{
			Kind:      string(e.Kind),
			Author:    e.Author,
			At:        stamp(e.At),
			Namespace: e.Namespace,
			Payload:   e.Payload,
		})
	}
	return v
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
