// Package feedback is the review-feedback ledger's storage class and its
// deterministic fold (docs/tech-design/review-feedback-capture.md §3.1). A
// review-feedback ledger is a per-subject directory of append-only markdown
// entry files — one file per captured human review comment — plus a
// docgate-generated machine projection (effective.source.json) the fold
// produces for the ledger-native measures (FR-5/FR-7).
//
// The fold is PURE (DINV-1): ordered entry bytes in, projection bytes out —
// no filesystem, network, clock, or environment. Identical inputs yield
// identical output on every platform, so verify-generated is recompute-and
// -compare by construction. Every parse failure is a named Defect, never a
// panic and never a lenient guess.
//
// This package is the T1 slice: the entry schema, the per-subject layout, and
// the fold. The coded writer, the CaptureSource port and its adapters, and
// disposition determination (§3.3–§3.4) are later tasks; the schema here
// merely *carries* an appended disposition block so an entry file stays
// append-only across a later disposition transition (§3.1, DINV-1).
package feedback

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// EntryTemplateVersion is the pinned schema version an entry file declares.
// The parse is closed against it: an entry declaring another version is a
// defect, not a leniently-parsed guess (AP-6).
const EntryTemplateVersion = "review-feedback-entry/v1"

// ProjectionSchemaVersion is the schema_version stamped into the projection.
const ProjectionSchemaVersion = "1"

// ProjectionFile is the sole generated artifact — the machine face; the
// human face is the entry files themselves (§3.1, so no effective.md).
const ProjectionFile = "effective.source.json"

// The four typed dispositions (§3.4). The set is closed; an entry naming
// another disposition is a defect.
var dispositions = map[string]bool{
	"applied":              true,
	"declined-with-reason": true,
	"became-incident":      true,
	"no-action":            true,
}

// entryNameRe matches an entry filename: NNN-<source>.md (dense-numbered,
// kebab-case source stem), the per-subject directory's only content besides
// the generated projection.
var entryNameRe = regexp.MustCompile(`^(\d{3})-([a-z0-9][a-z0-9-]*)\.md$`)

// Entry is one ledger file on disk — its filename and raw bytes. The caller
// (the reader) supplies these already; Fold normalizes internally.
type Entry struct {
	Name  string
	Bytes []byte
}

// Disposition is the transition appended to an entry when its resolving event
// is observed (§3.4). Absent (nil) until then. This T1 slice parses it so the
// schema carries it append-only; determination is a later task.
type Disposition struct {
	Disposition  string `json:"disposition"`             // applied | declined-with-reason | became-incident | no-action
	Outcome      string `json:"outcome"`                 // the stable outcome-ref (§3.5)
	ResolvedBy   string `json:"resolved_by"`             // the observed resolving event (DINV-3)
	EditDistance *int   `json:"edit_distance,omitempty"` // frozen at write for an applied outcome (§3.6)
}

// Captured is one folded entry — the projection's per-entry record. Field
// order is the JSON contract (AP-6); it is additive-evolution only.
type Captured struct {
	Entry         string `json:"entry"`  // the dense sequence, e.g. "001"
	Source        string `json:"source"` // conversation | code-host | …
	Author        string `json:"author"`
	Timestamp     string `json:"timestamp"`
	SubjectAnchor string `json:"subject_anchor"`
	Comment       string `json:"comment"` // the review comment, verbatim
	// EditCommit is the commit associated with the resulting edit, recorded at capture so the
	// disposition observer can ask whether it landed (§3.4, T4). Empty when the
	// comment produced no edit. Additive (omitempty): an entry without it projects
	// exactly as before (AP-6).
	EditCommit  string       `json:"edit_commit,omitempty"`
	Disposition *Disposition `json:"disposition"` // nil until a transition appends
	SourceFile  string       `json:"source_file"` // the entry's filename
	// CrossLinks are the entry's one-directional cross-link citations (§3.5):
	// `review:<report-stem>#<anchor>` to a reviewer M2 residual (a review-report
	// finding) and `delivery:<slug>#<anchor>` to an Argus E-n disposition. We
	// only ever cite them; the targets are never edited to cite back (NG-5).
	// Appended (additive, omitempty) — an entry without cross-links projects
	// exactly as before, so an old projection stays byte-identical (AP-6).
	CrossLinks []string `json:"cross_links,omitempty"`
}

// Projection is the generated machine face — effective.source.json.
type Projection struct {
	SchemaVersion string      `json:"schema_version"`
	GeneratedBy   string      `json:"generated_by"`
	ToolVersion   string      `json:"tool_version"`
	Subject       string      `json:"subject"`
	AsOf          string      `json:"as_of"`
	Fingerprint   string      `json:"content_fingerprint"`
	Entries       []*Captured `json:"entries"`
	// Measures is the ledger-native measure block (§3.6, FR-5) — the derived
	// signals the reviewer-calibration consumer reads, computed from the folded
	// entries alone.
	Measures Measures `json:"measures"`
}

// Measures is the fold's ledger-native measure block (§3.6, FR-5): the derived
// signals the reviewer-calibration consumer (M2) reads, each computed from the
// folded entries alone — no external state, no re-diff on read. Every field is
// always present (a stable shape) so a consumer never guesses a missing key.
type Measures struct {
	// Total is every folded entry; Disposed carry a terminal disposition, Open
	// do not. Total == Disposed + Open (§3.4).
	Total    int `json:"total"`
	Disposed int `json:"disposed"`
	Open     int `json:"open"`
	// DispositionCounts is the count per typed disposition over disposed entries.
	// All four closed dispositions are always present (0 when none), so the shape
	// is stable across ledgers (§3.4).
	DispositionCounts map[string]int `json:"disposition_counts"`
	// AppliedDecline is the applied-vs-decline ratio over disposed entries — the
	// feedback-level rejection signal (§3.6). Its Value is applied ÷ (applied +
	// declined-with-reason): the fraction of decided feedback that was applied;
	// null when that denominator is zero, never a fabricated 0.
	AppliedDecline Ratio `json:"applied_decline_ratio"`
	// EditDistance rolls up the per-entry edit_distance frozen at applied-write
	// (§3.6, AP-21 judge-once) — summed from the projection's own stored values,
	// never recomputed by re-diffing on read.
	EditDistance EditDistanceMeasure `json:"edit_distance"`
}

// Ratio is a numerator/denominator pair plus its computed value; Value is null
// when the denominator is zero — an undefined ratio is never a fabricated 0.
type Ratio struct {
	Applied  int      `json:"applied"`
	Declined int      `json:"declined"`
	Value    *float64 `json:"value"`
}

// EditDistanceMeasure aggregates the frozen per-entry edit_distance values: how
// many applied entries carry one and their sum (a consumer derives mean =
// sum ÷ count). Both are read from the entries' stored values, never re-diffed.
type EditDistanceMeasure struct {
	Count int `json:"count"`
	Sum   int `json:"sum"`
}

// computeMeasures derives the ledger-native measures from the folded entries
// alone (§3.6). It is pure and total: every closed disposition key is seeded so
// the shape is stable, and the applied/decline ratio is left null on a zero
// denominator rather than fabricating a 0.
func computeMeasures(entries []*Captured) Measures {
	m := Measures{
		Total: len(entries),
		DispositionCounts: map[string]int{
			"applied":              0,
			"declined-with-reason": 0,
			"became-incident":      0,
			"no-action":            0,
		},
	}
	for _, c := range entries {
		if c == nil || c.Disposition == nil {
			m.Open++
			continue
		}
		m.Disposed++
		d := c.Disposition
		if _, ok := m.DispositionCounts[d.Disposition]; ok {
			m.DispositionCounts[d.Disposition]++
		}
		// edit_distance is read from the entry's own frozen value (§3.6) — the
		// fold never re-diffs to recompute it.
		if d.EditDistance != nil {
			m.EditDistance.Count++
			m.EditDistance.Sum += *d.EditDistance
		}
	}
	applied := m.DispositionCounts["applied"]
	declined := m.DispositionCounts["declined-with-reason"]
	m.AppliedDecline = Ratio{Applied: applied, Declined: declined}
	if denom := applied + declined; denom > 0 {
		v := float64(applied) / float64(denom)
		m.AppliedDecline.Value = &v
	}
	return m
}

// Result is a fold's full output: the projection bytes plus the derived
// facts the caller reports, and any parse defects.
type Result struct {
	SourceJSON  []byte
	Fingerprint string
	AsOf        string
	Entries     []*Captured
	Measures    Measures
	Defects     []schema.Defect
}

func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "fold", File: file, Op: op, Reason: reason, Remedy: remedy}
}

// requiredFields are the header fields every capture writes (§3.1). All must
// be present and non-empty; the fold rejects a partial header rather than
// projecting a hole.
var requiredFields = []string{"Entry", "Source", "Author", "Timestamp", "Subject-anchor"}

// ParseEntry parses one entry file against the closed schema, emitting named
// defects (never a panic) for any missing header field, version mismatch,
// filename/header disagreement, or unknown disposition.
func ParseEntry(name string, b []byte) (*Captured, []schema.Defect) {
	var defects []schema.Defect
	fseq, fsrc, ok := parseName(name)
	if !ok {
		return nil, []schema.Defect{def(name, "entry-name",
			"filename is not NNN-<source>.md", "dense three-digit numbering, kebab-case source stem")}
	}

	lines := strings.Split(strings.TrimSuffix(string(schema.Normalize(b)), "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return nil, []schema.Defect{def(name, "parse-entry",
			"entry does not start with an H1 title", "an entry file opens with a '# ' title line")}
	}

	header, comment, disp, xlinks, pdef := parseSections(name, lines)
	defects = append(defects, pdef...)

	for _, f := range requiredFields {
		if strings.TrimSpace(header[f]) == "" {
			defects = append(defects, def(name, "header",
				fmt.Sprintf("required header field %q is empty or absent", f),
				"capture writes id · source · author · timestamp · subject-anchor"))
		}
	}
	if v := header["Template version"]; !strings.Contains(v, EntryTemplateVersion) {
		got := v
		if strings.TrimSpace(got) == "" {
			got = "(absent)"
		}
		defects = append(defects, def(name, "header",
			fmt.Sprintf("declared template version %s is not the pinned %s", got, EntryTemplateVersion),
			"instantiate against review-feedback-entry/v1; the closed parse reads only the pinned version"))
	}
	// The filename and header must agree — the filename is the ledger's index,
	// the header the entry's own claim; a mismatch is an authoring error, not a
	// thing to silently prefer one side of.
	if h := strings.TrimSpace(header["Entry"]); h != "" && h != fseq {
		defects = append(defects, def(name, "header",
			fmt.Sprintf("Entry field %q disagrees with the filename sequence %q", h, fseq),
			"the Entry field equals the filename's NNN"))
	}
	if h := strings.TrimSpace(header["Source"]); h != "" && h != fsrc {
		defects = append(defects, def(name, "header",
			fmt.Sprintf("Source field %q disagrees with the filename source %q", h, fsrc),
			"the Source field equals the filename's source stem"))
	}
	if disp != nil && !dispositions[disp.Disposition] {
		defects = append(defects, def(name, "disposition",
			fmt.Sprintf("unknown disposition %q", disp.Disposition),
			"the set is closed: applied, declined-with-reason, became-incident, no-action"))
	}

	if len(defects) > 0 {
		return nil, defects
	}
	return &Captured{
		Entry:         fseq,
		Source:        fsrc,
		Author:        strings.TrimSpace(header["Author"]),
		Timestamp:     strings.TrimSpace(header["Timestamp"]),
		SubjectAnchor: strings.TrimSpace(header["Subject-anchor"]),
		Comment:       comment,
		EditCommit:    strings.TrimSpace(header["Edit-commit"]),
		Disposition:   disp,
		SourceFile:    name,
		CrossLinks:    xlinks,
	}, nil
}

// parseName splits an entry filename into its dense sequence and source stem.
func parseName(name string) (seq, source string, ok bool) {
	m := entryNameRe.FindStringSubmatch(name)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// parseSections walks an entry's lines once: the leading Field|Value header
// table, the `## Comment` body (verbatim, trimmed of surrounding blanks), and
// an optional appended `## Disposition` Field|Value table.
func parseSections(name string, lines []string) (map[string]string, string, *Disposition, []string, []schema.Defect) {
	header := map[string]string{}
	var defects []schema.Defect

	// The leading header table is the first Field|Value table in the file.
	i := firstTable(lines, 1)
	if i < 0 {
		defects = append(defects, def(name, "header",
			"no Field | Value header table found", "an entry carries a header table of id · source · author · timestamp · subject-anchor"))
	} else {
		readKV(lines, i, header)
	}

	var comment string
	var disp *Disposition
	var xlinks []string
	for k := range len(lines) {
		title, ok := heading(lines[k])
		if !ok {
			continue
		}
		body := lines[k+1 : sectionEnd(lines, k+1)]
		switch title {
		case "Comment":
			comment = joinTrim(body)
		case "Disposition":
			disp = parseDisposition(body)
		case "Cross-links":
			xlinks = parseCrossLinks(body)
		}
	}
	if strings.TrimSpace(comment) == "" {
		defects = append(defects, def(name, "comment",
			"the '## Comment' section is empty or absent", "capture stores the review comment verbatim under '## Comment'"))
	}
	return header, comment, disp, xlinks, defects
}

// crossLinkRe matches one one-directional cross-link citation an entry may carry
// (§3.5): a `review:<report-stem>#<anchor>` to a reviewer M2 residual (a review
// report finding) or a `delivery:<slug>#<anchor>` to an Argus E-n disposition.
// The angle-bracketed grammar metavariables carry a `<`, which this concrete
// grammar never matches, so an illustration written with a placeholder is not
// even an occurrence — the same recognition discipline as the citation checks.
var crossLinkRe = regexp.MustCompile(`(?:review|delivery):[a-z0-9]+(?:-[a-z0-9]+)*#[A-Za-z0-9][A-Za-z0-9_-]*`)

// inlineCodeRe strips inline `code` spans so a cross-link displayed as an
// example inside the section is not collected as a live citation.
var inlineCodeRe = regexp.MustCompile("`[^`]*`")

// parseCrossLinks collects the recognized one-directional cross-link citations
// from a `## Cross-links` section body, in document order and de-duplicated. A
// fenced block or an inline-code span is a displayed example, never a live
// cross-link (the corpus citation convention); a prose line carrying neither
// grammar is ignored (the section may annotate its links). Only the two accepted
// grammars are recognized — a cross-link is to a reviewer M2 residual or an
// Argus E-n disposition, nothing else.
func parseCrossLinks(body []string) []string {
	var out []string
	seen := map[string]bool{}
	fenced := false
	for _, l := range body {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, m := range crossLinkRe.FindAllString(inlineCodeRe.ReplaceAllString(l, ""), -1) {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// parseDisposition reads the transition's Field|Value table into a typed
// Disposition; an absent table yields nil.
func parseDisposition(body []string) *Disposition {
	i := firstTable(body, 0)
	if i < 0 {
		return &Disposition{}
	}
	kv := map[string]string{}
	readKV(body, i, kv)
	d := &Disposition{
		Disposition: strings.TrimSpace(kv["Disposition"]),
		Outcome:     strings.TrimSpace(kv["Outcome"]),
		ResolvedBy:  strings.TrimSpace(kv["Resolved-by"]),
	}
	if v := strings.TrimSpace(kv["Edit-distance"]); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			d.EditDistance = &n
		}
	}
	return d
}

var sepRowRe = regexp.MustCompile(`^\|[\s:|-]+\|?\s*$`)

// firstTable returns the index of the first Field|Value table's header row at
// or after `from`, or -1. A table is a `|`-led line followed by a separator.
func firstTable(lines []string, from int) int {
	for i := from; i+1 < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && sepRowRe.MatchString(lines[i+1]) {
			c := cells(lines[i])
			if len(c) >= 2 && fieldName(c[0]) == "Field" {
				return i
			}
		}
	}
	return -1
}

// readKV loads the rows of the table whose header is at `hdr` into m, keyed by
// the (de-emphasized) first cell.
func readKV(lines []string, hdr int, m map[string]string) {
	for r := hdr + 2; r < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[r]), "|"); r++ {
		c := cells(lines[r])
		if len(c) >= 2 {
			m[fieldName(c[0])] = c[1]
		}
	}
}

var headingRe = regexp.MustCompile(`^##\s+(.*)$`)

func heading(l string) (string, bool) {
	m := headingRe.FindStringSubmatch(l)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// sectionEnd returns the exclusive end of a section body beginning at `start`:
// the next `## ` heading, or end of file.
func sectionEnd(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if _, ok := heading(lines[i]); ok {
			return i
		}
	}
	return len(lines)
}

func cells(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func fieldName(cell string) string {
	return strings.TrimSpace(strings.Trim(cell, "* "))
}

// joinTrim joins a body range and trims surrounding blank lines, preserving
// the comment's internal shape verbatim.
func joinTrim(body []string) string {
	start, end := 0, len(body)
	for start < end && strings.TrimSpace(body[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(body[end-1]) == "" {
		end--
	}
	return strings.Join(body[start:end], "\n")
}

// Fold replays a subject's entry files, in filename order, into the projection
// (§3.1). It is pure: same bytes in, same bytes out. `subject` is the
// per-subject directory stem stamped into the projection.
func Fold(subject string, entries []Entry) Result {
	var res Result
	if len(entries) == 0 {
		res.Defects = append(res.Defects, def(subject, "replay",
			"the subject directory holds no entries", "a review-feedback ledger holds one file per captured comment"))
		return res
	}

	norm := make([][]byte, len(entries))
	seen := map[string]string{}
	for i, e := range entries {
		norm[i] = schema.Normalize(e.Bytes)
		if seq, _, ok := parseName(e.Name); ok {
			if prev, dup := seen[seq]; dup {
				res.Defects = append(res.Defects, def(e.Name, "entry-name",
					fmt.Sprintf("duplicate sequence %s (also %s)", seq, prev),
					"entry sequences are dense and unique; never reuse a number"))
			}
			seen[seq] = e.Name
		}
	}
	res.Fingerprint = schema.Fingerprint(norm)

	var captured []*Captured
	for i, e := range entries {
		c, pdef := ParseEntry(e.Name, norm[i])
		res.Defects = append(res.Defects, pdef...)
		if c != nil {
			captured = append(captured, c)
		}
	}
	if len(res.Defects) > 0 {
		return res
	}
	res.AsOf = captured[len(captured)-1].Entry
	res.Entries = captured
	res.Measures = computeMeasures(captured)
	res.SourceJSON = emitProjection(subject, res.AsOf, res.Fingerprint, captured, res.Measures)
	return res
}

func emitProjection(subject, asOf, fingerprint string, entries []*Captured, measures Measures) []byte {
	out, err := json.MarshalIndent(Projection{
		SchemaVersion: ProjectionSchemaVersion,
		GeneratedBy:   "docgate",
		ToolVersion:   schema.ToolVersion,
		Subject:       subject,
		AsOf:          asOf,
		Fingerprint:   fingerprint,
		Entries:       entries,
		Measures:      measures,
	}, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(out, '\n')
}
