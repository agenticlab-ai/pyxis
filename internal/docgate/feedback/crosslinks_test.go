package feedback_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

// entryWith renders a well-formed entry whose `## Cross-links` section is the
// given body — the fixture the fold's cross-link collector reads (§3.5).
func entryWith(crosslinks string) []byte {
	return []byte(`# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @reviewer-one |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | review-feedback-capture#3.1 |
| **Template version** | review-feedback-entry/v1 |

## Comment

The fold should emit a content fingerprint.

## Cross-links

` + crosslinks + "\n")
}

// The fold collects both accepted grammars from a `## Cross-links` section, in
// document order — a `review:` link to a reviewer M2 residual and a `delivery:`
// link to an Argus E-n disposition (§3.5, FR-8).
func TestParseEntryCollectsCrossLinks(t *testing.T) {
	c, defs := feedback.ParseEntry("001-conversation.md", entryWith(
		"- review:auth-flow-001#FND-2 — the reviewer's residual\n- delivery:some-drive#E-3 — the Argus disposition"))
	if len(defs) != 0 {
		t.Fatalf("a well-formed entry must parse cleanly, got %v", defs)
	}
	want := []string{"review:auth-flow-001#FND-2", "delivery:some-drive#E-3"}
	if !reflect.DeepEqual(c.CrossLinks, want) {
		t.Fatalf("cross-links = %v, want %v", c.CrossLinks, want)
	}
	// The captured comment and attribution are untouched by the new section.
	if c.Comment != "The fold should emit a content fingerprint." {
		t.Errorf("comment corrupted: %q", c.Comment)
	}
}

// An entry with no `## Cross-links` section carries none — so the projection
// omits the field entirely and an old projection stays byte-identical (AP-6).
func TestParseEntryNoCrossLinksSection(t *testing.T) {
	entry := `# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @r |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | s#a |
| **Template version** | review-feedback-entry/v1 |

## Comment

body.
`
	c, defs := feedback.ParseEntry("001-conversation.md", []byte(entry))
	if len(defs) != 0 {
		t.Fatalf("clean parse expected, got %v", defs)
	}
	if c.CrossLinks != nil {
		t.Fatalf("absent section must yield nil cross-links, got %v", c.CrossLinks)
	}
}

// A cross-link shown inside an inline-code span or a fenced block, or written
// with the angle-bracketed grammar metavariable, is a displayed example — never
// collected as a live cross-link. Duplicates collapse to one.
func TestParseEntryCrossLinksRecognitionBoundary(t *testing.T) {
	body := strings.Join([]string{
		"- review:real-report#FND-1 — live",
		"- example `delivery:x#E-9` in a span",
		"- metavariable review:<report-stem>#<anchor>",
		"- duplicate review:real-report#FND-1",
		"```",
		"- fenced delivery:y#E-8",
		"```",
	}, "\n")
	c, defs := feedback.ParseEntry("001-conversation.md", entryWith(body))
	if len(defs) != 0 {
		t.Fatalf("clean parse expected, got %v", defs)
	}
	want := []string{"review:real-report#FND-1"}
	if !reflect.DeepEqual(c.CrossLinks, want) {
		t.Fatalf("only the live, de-duplicated citation should be collected, got %v", c.CrossLinks)
	}
}

// The projection carries cross_links only when present — the additive contract:
// an entry with links serializes the field; one without omits it.
func TestFoldProjectionCrossLinksAdditive(t *testing.T) {
	with := feedback.Fold("subject-alpha", []feedback.Entry{
		{Name: "001-conversation.md", Bytes: entryWith("- delivery:some-drive#E-3")},
	})
	if len(with.Defects) != 0 {
		t.Fatalf("fold defects: %v", with.Defects)
	}
	if !strings.Contains(string(with.SourceJSON), `"cross_links"`) {
		t.Errorf("projection should carry cross_links when present:\n%s", with.SourceJSON)
	}
	without := feedback.Fold("subject-alpha", []feedback.Entry{
		{Name: "001-conversation.md", Bytes: entryWith("(no live links here)")},
	})
	if strings.Contains(string(without.SourceJSON), `"cross_links"`) {
		t.Errorf("projection must omit cross_links when absent:\n%s", without.SourceJSON)
	}
}
