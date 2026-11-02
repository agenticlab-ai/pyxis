// Package events keeps draft lifecycle observations in the draft's working
// file, which the compiler retains as a literal Document at lock.
package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const Schema = "draft-event/v1"
const heading = "# Draft events\n"

const (
	DraftOpened        = "draft_opened"
	DraftStepFailed    = "draft_step_failed"
	RequirementChanged = "requirement_changed"
	LockRefused        = "lock_refused"
	DraftReopened      = "draft_reopened"
)

// Event has a closed schema. Kind-specific fields are validated before an
// event enters either the working log or the cross-clone pending ref.
type Event struct {
	Schema          string   `json:"schema"`
	ID              string   `json:"id"`
	Key             string   `json:"key,omitempty"`
	Name            string   `json:"name"`
	Feature         string   `json:"feature"`
	VersionDigest   string   `json:"version_digest,omitempty"`
	OccurredAt      string   `json:"occurred_at"`
	Origin          string   `json:"origin,omitempty"`
	BasisVersion    *int     `json:"basis_version,omitempty"`
	Step            string   `json:"step,omitempty"`
	ErrorClass      string   `json:"error_class,omitempty"`
	Requirement     string   `json:"requirement,omitempty"`
	RulingAuthor    string   `json:"ruling_author,omitempty"`
	ReasonPresent   bool     `json:"reason_present,omitempty"`
	Problem         string   `json:"problem,omitempty"`
	UnmetConditions []string `json:"unmet_conditions,omitempty"`
	FromVersion     int      `json:"from_version,omitempty"`
	Cause           string   `json:"cause,omitempty"`
	CauseRef        string   `json:"cause_ref,omitempty"`
}

func New(name, feature, digest string, at time.Time) (Event, error) {
	var entropy [12]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Event{}, fmt.Errorf("mint draft event: %w", err)
	}
	return Event{Schema: Schema, ID: hex.EncodeToString(entropy[:]), Name: name, Feature: feature, VersionDigest: digest, OccurredAt: at.UTC().Format(time.RFC3339Nano)}, nil
}

func (e Event) Validate() error {
	if e.Schema != Schema || len(e.ID) != 24 || e.Feature == "" || strings.ContainsAny(e.Feature, " \t\r\n/") {
		return fmt.Errorf("invalid draft event identity or feature")
	}
	if _, err := hex.DecodeString(e.ID); err != nil {
		return fmt.Errorf("invalid draft event ID: %w", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, e.OccurredAt); err != nil {
		return fmt.Errorf("invalid draft event time: %w", err)
	}
	switch e.Name {
	case DraftOpened:
		if e.Origin != "new" && e.Origin != "reopen" || e.Origin == "new" && e.BasisVersion != nil || e.Origin == "reopen" && (e.BasisVersion == nil || *e.BasisVersion < 1) {
			return fmt.Errorf("draft_opened needs origin and matching basis version")
		}
	case DraftStepFailed:
		if e.Step == "" || e.ErrorClass == "" {
			return fmt.Errorf("draft_step_failed needs step and error class")
		}
	case RequirementChanged:
		if e.Key == "" || e.Requirement == "" || e.RulingAuthor == "" || !e.ReasonPresent || e.Problem == "" {
			return fmt.Errorf("requirement_changed needs ruling commit key, requirement, author, reason, and problem")
		}
	case LockRefused:
		if e.VersionDigest == "" || len(e.UnmetConditions) == 0 {
			return fmt.Errorf("lock_refused needs digest and unmet conditions")
		}
	case DraftReopened:
		if e.FromVersion < 1 || e.CauseRef == "" || e.Cause != "new-need" && e.Cause != "dependency-change" && e.Cause != "gap-lock-should-have-caught" && e.Cause != "other" {
			return fmt.Errorf("draft_reopened needs prior version, cause, and reference")
		}
	default:
		return fmt.Errorf("unknown draft event %q", e.Name)
	}
	return nil
}

// Parse reads the working-file format: one closed JSON event per Markdown
// list item. Empty scaffolds are valid. Other content is a named defect.
func Parse(body []byte) ([]Event, error) {
	if len(body) == 0 {
		return nil, nil
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if lines[0] != strings.TrimSuffix(heading, "\n") {
		return nil, fmt.Errorf("draft event log needs the %q heading", strings.TrimSpace(heading))
	}
	var out []Event
	seenID, seenKey := map[string]bool{}, map[string]bool{}
	for i, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			return nil, fmt.Errorf("draft event log line %d must be a JSON list item", i+2)
		}
		var event Event
		decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(line, "- ")))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			return nil, fmt.Errorf("draft event log line %d: %w", i+2, err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return nil, fmt.Errorf("draft event log line %d has trailing content", i+2)
		}
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("draft event log line %d: %w", i+2, err)
		}
		if seenID[event.ID] || event.Key != "" && seenKey[event.Key] {
			return nil, fmt.Errorf("draft event log line %d repeats event ID or key", i+2)
		}
		seenID[event.ID] = true
		if event.Key != "" {
			seenKey[event.Key] = true
		}
		out = append(out, event)
	}
	return out, nil
}

// Append preserves the existing log and adds each event once by its ID or
// stable key. Re-reading pending events in another clone cannot duplicate it.
func Append(body []byte, additions ...Event) ([]byte, bool, error) {
	current, err := Parse(body)
	if err != nil {
		return nil, false, err
	}
	ids, keys := map[string]bool{}, map[string]bool{}
	for _, event := range current {
		ids[event.ID] = true
		if event.Key != "" {
			keys[event.Key] = true
		}
	}
	changed := false
	for _, event := range additions {
		if err := event.Validate(); err != nil {
			return nil, false, err
		}
		if ids[event.ID] || event.Key != "" && keys[event.Key] {
			continue
		}
		ids[event.ID] = true
		if event.Key != "" {
			keys[event.Key] = true
		}
		current = append(current, event)
		changed = true
	}
	if !changed {
		return body, false, nil
	}
	var out strings.Builder
	out.WriteString(heading)
	for _, event := range current {
		encoded, err := json.Marshal(event)
		if err != nil {
			return nil, false, fmt.Errorf("encode draft event: %w", err)
		}
		out.WriteString("- ")
		out.Write(encoded)
		out.WriteByte('\n')
	}
	return []byte(out.String()), true, nil
}
