package feedback_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
)

var update = flag.Bool("update", false, "rewrite golden effective.source.json fixtures")

// readSubject loads a subject fixture directory's entry files, sorted by name
// — the same discipline the reader in package gate applies.
func readSubject(t *testing.T, dir string) []feedback.Entry {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var entries []feedback.Entry
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, feedback.Entry{Name: name, Bytes: b})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// A-1 (T1 half): N entry files fold to a projection carrying N entries, each
// with every header field populated.
func TestFoldNEntries(t *testing.T) {
	entries := readSubject(t, "testdata/subject-alpha")
	res := feedback.Fold("subject-alpha", entries)
	if len(res.Defects) != 0 {
		t.Fatalf("clean fixture has defects: %v", res.Defects)
	}
	if len(res.Entries) != len(entries) {
		t.Fatalf("projected %d entries, want %d", len(res.Entries), len(entries))
	}
	for _, e := range res.Entries {
		if e.Entry == "" || e.Source == "" || e.Author == "" || e.Timestamp == "" || e.SubjectAnchor == "" || e.Comment == "" {
			t.Fatalf("entry %s has an unpopulated field: %+v", e.SourceFile, e)
		}
	}
	if res.AsOf != "002" {
		t.Fatalf("as-of = %q, want 002", res.AsOf)
	}
}

// A-2 (T1 half): the fold regenerates the projection deterministically.
func TestFoldDeterministic(t *testing.T) {
	entries := readSubject(t, "testdata/subject-alpha")
	a := feedback.Fold("subject-alpha", entries)
	b := feedback.Fold("subject-alpha", entries)
	if !bytes.Equal(a.SourceJSON, b.SourceJSON) {
		t.Fatal("fold is not deterministic across runs")
	}
	if a.Fingerprint != b.Fingerprint || a.Fingerprint == "" {
		t.Fatalf("unstable or empty fingerprint: %q vs %q", a.Fingerprint, b.Fingerprint)
	}
}

// A-2 (byte-verify): the golden projection is regenerated exactly; a run with
// -update rewrites it. A drift here is precisely what reds verify-generated.
func TestGoldenProjection(t *testing.T) {
	dir := "testdata/subject-alpha"
	entries := readSubject(t, dir)
	res := feedback.Fold("subject-alpha", entries)
	if len(res.Defects) != 0 {
		t.Fatalf("defects: %v", res.Defects)
	}
	golden := filepath.Join(dir, feedback.ProjectionFile)
	if *update {
		if err := os.WriteFile(golden, res.SourceJSON, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	if !bytes.Equal(want, res.SourceJSON) {
		t.Fatalf("projection drift:\n got %s\nwant %s", res.SourceJSON, want)
	}
}

// A-3 (T1 half): appending a disposition transition to an entry leaves its
// captured text and attribution unchanged.
func TestAppendDispositionPreservesCapture(t *testing.T) {
	base := []byte(`# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @reviewer-one |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | some-doc#3.1 |
| **Template version** | review-feedback-entry/v1 |

## Comment

Keep the verbatim comment intact across a later disposition.
`)
	before, defs := feedback.ParseEntry("001-conversation.md", base)
	if len(defs) != 0 {
		t.Fatalf("clean entry has defects: %v", defs)
	}
	if before.Disposition != nil {
		t.Fatal("captured entry should have no disposition yet")
	}

	appended := append(append([]byte{}, base...), []byte(`
## Disposition

| Field | Value |
|---|---|
| **Disposition** | applied |
| **Outcome** | pr:216 |
| **Resolved-by** | delivery:x#E-1 |
`)...)
	after, defs := feedback.ParseEntry("001-conversation.md", appended)
	if len(defs) != 0 {
		t.Fatalf("appended entry has defects: %v", defs)
	}
	if after.Comment != before.Comment {
		t.Fatalf("comment changed:\n before %q\n after %q", before.Comment, after.Comment)
	}
	if after.Author != before.Author || after.Timestamp != before.Timestamp || after.SubjectAnchor != before.SubjectAnchor {
		t.Fatalf("attribution changed: before %+v after %+v", before, after)
	}
	if after.Disposition == nil || after.Disposition.Disposition != "applied" {
		t.Fatalf("disposition not captured after append: %+v", after.Disposition)
	}
}

func TestParseDefects(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		body   string
		wantOp string
	}{
		{
			name:   "missing required field",
			file:   "001-conversation.md",
			body:   "# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\nhi\n",
			wantOp: "header",
		},
		{
			name:   "wrong template version",
			file:   "001-conversation.md",
			body:   "# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @a |\n| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v2 |\n\n## Comment\n\nhi\n",
			wantOp: "header",
		},
		{
			name:   "filename disagrees with header",
			file:   "002-conversation.md",
			body:   "# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @a |\n| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\nhi\n",
			wantOp: "header",
		},
		{
			name:   "empty comment",
			file:   "001-conversation.md",
			body:   "# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @a |\n| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\n",
			wantOp: "comment",
		},
		{
			name:   "bad filename",
			file:   "not-an-entry.md",
			body:   "# t\n",
			wantOp: "entry-name",
		},
		{
			name:   "unknown disposition",
			file:   "001-conversation.md",
			body:   "# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @a |\n| **Timestamp** | 2026-07-13T09:30:00Z |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\nhi\n\n## Disposition\n\n| Field | Value |\n|---|---|\n| **Disposition** | maybe |\n",
			wantOp: "disposition",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, defs := feedback.ParseEntry(tc.file, []byte(tc.body))
			if len(defs) == 0 {
				t.Fatalf("expected a defect, got none")
			}
			found := false
			for _, d := range defs {
				if d.Op == tc.wantOp {
					found = true
				}
			}
			if !found {
				t.Fatalf("no defect with op %q; got %+v", tc.wantOp, defs)
			}
		})
	}
}

func TestFoldEmptyIsDefect(t *testing.T) {
	res := feedback.Fold("empty", nil)
	if len(res.Defects) == 0 {
		t.Fatal("an empty ledger should be a defect")
	}
}

func TestFoldDuplicateSequence(t *testing.T) {
	e := feedback.Entry{Name: "001-conversation.md", Bytes: []byte("# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | conversation |\n| **Author** | @a |\n| **Timestamp** | t |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\nhi\n")}
	f := feedback.Entry{Name: "001-code-host.md", Bytes: []byte("# t\n\n| Field | Value |\n|---|---|\n| **Entry** | 001 |\n| **Source** | code-host |\n| **Author** | @b |\n| **Timestamp** | t |\n| **Subject-anchor** | x#1 |\n| **Template version** | review-feedback-entry/v1 |\n\n## Comment\n\nyo\n")}
	res := feedback.Fold("dup", []feedback.Entry{e, f})
	if len(res.Defects) == 0 {
		t.Fatal("duplicate sequence should be a defect")
	}
}

// entry builds one entry file's bytes — the capture block plus an optional
// appended disposition (with an optional frozen edit-distance) — so a measures
// test can compose a ledger of every disposition kind without on-disk fixtures.
func entry(seq, source, disp, editDistance string) feedback.Entry {
	var b strings.Builder
	b.WriteString("# Review-feedback entry " + seq + "\n\n")
	b.WriteString("| Field | Value |\n|---|---|\n")
	b.WriteString("| **Entry** | " + seq + " |\n")
	b.WriteString("| **Source** | " + source + " |\n")
	b.WriteString("| **Author** | @a |\n")
	b.WriteString("| **Timestamp** | 2026-07-13T09:30:00Z |\n")
	b.WriteString("| **Subject-anchor** | x#1 |\n")
	b.WriteString("| **Template version** | review-feedback-entry/v1 |\n\n")
	b.WriteString("## Comment\n\nhi\n")
	if disp != "" {
		b.WriteString("\n## Disposition\n\n| Field | Value |\n|---|---|\n")
		b.WriteString("| **Disposition** | " + disp + " |\n")
		b.WriteString("| **Outcome** | pr:216 |\n")
		b.WriteString("| **Resolved-by** | delivery:x#E-1 |\n")
		if editDistance != "" {
			b.WriteString("| **Edit-distance** | " + editDistance + " |\n")
		}
	}
	return feedback.Entry{Name: seq + "-" + source + ".md", Bytes: []byte(b.String())}
}

// f64 dereferences a *float64 for a test assertion, failing on nil.
func f64(t *testing.T, v *float64) float64 {
	t.Helper()
	if v == nil {
		t.Fatal("expected a ratio value, got null")
	}
	return *v
}

// The measures block (§3.6, FR-5) — disposition counts, the applied/decline
// ratio, and the frozen edit_distance rollup — computes from the ledger alone.
func TestMeasures(t *testing.T) {
	entries := []feedback.Entry{
		entry("001", "conversation", "applied", "12"),
		entry("002", "code-host", "applied", "8"),
		entry("003", "conversation", "declined-with-reason", ""),
		entry("004", "code-host", "became-incident", ""),
		entry("005", "conversation", "no-action", ""),
		entry("006", "code-host", "", ""), // still open
	}
	res := feedback.Fold("mixed", entries)
	if len(res.Defects) != 0 {
		t.Fatalf("clean ledger has defects: %v", res.Defects)
	}
	m := res.Measures
	if m.Total != 6 || m.Disposed != 5 || m.Open != 1 {
		t.Fatalf("totals = {total %d, disposed %d, open %d}, want {6,5,1}", m.Total, m.Disposed, m.Open)
	}
	wantCounts := map[string]int{"applied": 2, "declined-with-reason": 1, "became-incident": 1, "no-action": 1}
	for k, want := range wantCounts {
		if m.DispositionCounts[k] != want {
			t.Fatalf("disposition_counts[%q] = %d, want %d", k, m.DispositionCounts[k], want)
		}
	}
	// applied ÷ (applied + declined) = 2 / 3.
	if m.AppliedDecline.Applied != 2 || m.AppliedDecline.Declined != 1 {
		t.Fatalf("ratio poles = {applied %d, declined %d}, want {2,1}", m.AppliedDecline.Applied, m.AppliedDecline.Declined)
	}
	if got := f64(t, m.AppliedDecline.Value); got != 2.0/3.0 {
		t.Fatalf("applied/decline value = %v, want %v", got, 2.0/3.0)
	}
	// edit_distance rolls up only the applied entries' frozen values: 12 + 8.
	if m.EditDistance.Count != 2 || m.EditDistance.Sum != 20 {
		t.Fatalf("edit_distance = {count %d, sum %d}, want {2,20}", m.EditDistance.Count, m.EditDistance.Sum)
	}

	// The acceptance is "from the projection alone": the same measures must
	// unmarshal out of the emitted JSON with no access to the source entries.
	var p feedback.Projection
	if err := json.Unmarshal(res.SourceJSON, &p); err != nil {
		t.Fatalf("projection is not valid JSON: %v", err)
	}
	if p.Measures.Disposed != 5 || p.Measures.DispositionCounts["applied"] != 2 ||
		p.Measures.EditDistance.Sum != 20 || f64(t, p.Measures.AppliedDecline.Value) != 2.0/3.0 {
		t.Fatalf("measures did not round-trip through the projection: %+v", p.Measures)
	}
}

// A ledger with no applied and no declined entry leaves the ratio null — an
// undefined ratio is never a fabricated 0 (§3.6).
func TestMeasuresRatioNullOnZeroDenominator(t *testing.T) {
	res := feedback.Fold("none", []feedback.Entry{
		entry("001", "conversation", "became-incident", ""),
		entry("002", "code-host", "", ""),
	})
	if len(res.Defects) != 0 {
		t.Fatalf("clean ledger has defects: %v", res.Defects)
	}
	if res.Measures.AppliedDecline.Value != nil {
		t.Fatalf("ratio value = %v, want null on a zero denominator", *res.Measures.AppliedDecline.Value)
	}
	// The four closed dispositions are always present, so a consumer never
	// guesses a missing key.
	for _, k := range []string{"applied", "declined-with-reason", "became-incident", "no-action"} {
		if _, ok := res.Measures.DispositionCounts[k]; !ok {
			t.Fatalf("disposition_counts is missing the closed key %q", k)
		}
	}
	if res.Measures.EditDistance.Count != 0 || res.Measures.EditDistance.Sum != 0 {
		t.Fatalf("edit_distance rollup should be zero with no applied entry: %+v", res.Measures.EditDistance)
	}
}

// The projection is valid JSON with the schema-versioned envelope (AP-6).
func TestProjectionEnvelope(t *testing.T) {
	entries := readSubject(t, "testdata/subject-alpha")
	res := feedback.Fold("subject-alpha", entries)
	var p feedback.Projection
	if err := json.Unmarshal(res.SourceJSON, &p); err != nil {
		t.Fatalf("projection is not valid JSON: %v", err)
	}
	if p.SchemaVersion != feedback.ProjectionSchemaVersion || p.GeneratedBy != "docgate" || p.Subject != "subject-alpha" {
		t.Fatalf("bad envelope: %+v", p)
	}
}
