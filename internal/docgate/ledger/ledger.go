// Package ledger parses PRD ledger entries — the base document and typed
// amendments — into anchored units and operations, against the closed
// registries in internal/docgate/schema. Every parse failure is a Defect,
// never a panic and never a lenient guess (DINV-2). This is the only
// package that understands the pinned markdown grammar.
package ledger

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Entry is one ledger file, already normalized by the caller.
type Entry struct {
	Name  string
	Bytes []byte
}

// Table is one pinned-schema markdown table found in a document.
type Table struct {
	Kind       string // ID prefix whose schema the header matches exactly, or ""
	Cols       []string
	HeaderLine int   // index into Doc.Lines of the header row
	RowLines   []int // indexes of data-row lines, in order
}

// Unit is one ID-bearing table row.
type Unit struct {
	ID      string
	Prefix  string
	Line    int // index into Doc.Lines
	Cells   []string
	Table   int    // index into Doc.Tables
	Section string // enclosing numbered-section anchor, e.g. "5.1"
}

// Section is a numbered heading and its body range [Start+1, End).
type Section struct {
	Anchor string
	Level  int
	Start  int // heading line index
	End    int // exclusive
}

// Doc is a parsed base (or rebase) document.
type Doc struct {
	Title     string
	Lines     []string // body lines, after the H1 line
	Header    map[string]string
	Sections  []Section
	Tables    []Table
	Units     map[string]*Unit
	UnitOrder []string
}

var (
	headingRe = regexp.MustCompile(`^(#{2,3})\s+(?:(\d+(?:\.\d+)*)\.?\s+)?(.*)$`)
	sepRowRe  = regexp.MustCompile(`^\|[\s:|-]+\|?\s*$`)
	opRe      = regexp.MustCompile(`^###\s+op:\s+([a-z]+)\s*(.*)$`)
	seqRe     = regexp.MustCompile(`^(\d{3})-[a-z0-9-]+\.md$`)
)

// Seq returns an entry's dense sequence number ("000") and ok.
func Seq(name string) (string, bool) {
	m := seqRe.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// IsRebase reports whether the entry is a rebase checkpoint.
func IsRebase(name string) bool {
	return strings.HasSuffix(name, "-rebase.md")
}

func cells(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func fieldName(cell string) string {
	return strings.TrimSpace(strings.Trim(cell, "* "))
}

func defect(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// ParseBase parses a base or rebase document. Merged bases predate strict
// projection parsing, so a table whose header has drifted from the registered
// schema continues to infer its kind from the first recognized unit row.
func ParseBase(name string, normalized []byte) (*Doc, []schema.Defect) {
	return parseDocument(name, normalized, false)
}

// ParseProjection parses an effective PRD projection for compiler consumers.
// It retains ParseBase's merged-document compatibility rule (row kind may be
// inferred when a frozen header predates the current schema), but only for
// prefixes in the addressable TableSchemas registry. Section 0 and all of its
// descendants are human-only and never participate in that inference.
func ParseProjection(name string, normalized []byte) (*Doc, []schema.Defect) {
	return parseDocument(name, normalized, true)
}

func parseDocument(name string, normalized []byte, strictProjection bool) (*Doc, []schema.Defect) {
	var defects []schema.Defect
	all := strings.Split(strings.TrimSuffix(string(normalized), "\n"), "\n")
	if len(all) == 0 || !strings.HasPrefix(all[0], "# ") {
		return nil, []schema.Defect{defect("fold", name, "parse-base", "document does not start with an H1 title", "instantiate from docs/templates/prd-template.md")}
	}
	d := &Doc{Title: strings.TrimPrefix(all[0], "# "), Lines: all[1:], Header: map[string]string{}, Units: map[string]*Unit{}}

	// Sections.
	type open struct{ idx int }
	var stack []int
	for i, l := range d.Lines {
		m := headingRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		level := len(m[1])
		for len(stack) > 0 {
			last := &d.Sections[stack[len(stack)-1]]
			if last.Level >= level {
				last.End = i
				stack = stack[:len(stack)-1]
			} else {
				break
			}
		}
		anchor := m[2]
		if anchor == "" {
			anchor = strings.ToLower(strings.Fields(m[3])[0])
		}
		d.Sections = append(d.Sections, Section{Anchor: anchor, Level: level, Start: i, End: len(d.Lines)})
		stack = append(stack, len(d.Sections)-1)
	}

	// Tables and units.
	headerSeen := false
	for i := 0; i < len(d.Lines); i++ {
		l := d.Lines[i]
		if !strings.HasPrefix(strings.TrimSpace(l), "|") || i+1 >= len(d.Lines) || !sepRowRe.MatchString(d.Lines[i+1]) {
			continue
		}
		t := Table{Cols: cells(l), HeaderLine: i}
		j := i + 2
		for j < len(d.Lines) && strings.HasPrefix(strings.TrimSpace(d.Lines[j]), "|") {
			t.RowLines = append(t.RowLines, j)
			j++
		}
		// The first table is the document header (Field | Value).
		if !headerSeen {
			headerSeen = true
			for _, r := range t.RowLines {
				c := cells(d.Lines[r])
				if len(c) >= 2 {
					d.Header[fieldName(c[0])] = c[1]
				}
			}
			i = j - 1
			continue
		}
		t.Kind = exactTableKind(t.Cols)
		ti := len(d.Tables)
		for _, r := range t.RowLines {
			c := cells(d.Lines[r])
			id, prefix, ok := schema.ParseID(c[0])
			if !ok {
				continue
			}
			if _, known := schema.TableSchemas[prefix]; !known {
				continue
			}
			if strictProjection && inSectionZeroAt(d, r) {
				continue
			}
			section := sectionAt(d, r)
			// Base tables parse deterministically: kind is established by the
			// exact template header when it matches, else by the table's own
			// unit rows. Strict column matching is a PAYLOAD contract (UB-2);
			// the base's authority is its merged, immutable self.
			if t.Kind == "" {
				t.Kind = prefix
			}
			if t.Kind != prefix {
				defects = append(defects, defect("fold", name, "parse-base",
					fmt.Sprintf("row %s sits in a %s table — one table, one kind", id, t.Kind),
					"move the row to its own kind's table"))
				continue
			}
			if _, dup := d.Units[id]; dup {
				defects = append(defects, defect("fold", name, "parse-base",
					fmt.Sprintf("duplicate ID %s", id), "IDs are never reused; pick the next free number"))
				continue
			}
			u := &Unit{ID: id, Prefix: prefix, Line: r, Cells: c, Table: ti, Section: section}
			d.Units[id] = u
			d.UnitOrder = append(d.UnitOrder, id)
		}
		d.Tables = append(d.Tables, t)
		i = j - 1
	}
	if v := d.Header["Template version"]; !strings.Contains(v, schema.PRDTemplateVersion) && !strings.Contains(v, schema.PRDTemplateV2) {
		defects = append(defects, defect("fold", name, "parse-base",
			fmt.Sprintf("Template version %q is not in the supported set (%s, %s)", v, schema.PRDTemplateVersion, schema.PRDTemplateV2),
			"record the template version the document instantiates"))
	}
	return d, defects
}

func exactTableKind(cols []string) string {
	// Prefixes is ordered, unlike map iteration, so even a future accidental
	// duplicate schema cannot make parsing depend on process map order.
	for _, prefix := range schema.Prefixes {
		if want, ok := schema.TableSchemas[prefix]; ok && equalCols(cols, want) {
			return prefix
		}
	}
	return ""
}

func inSectionZeroAt(d *Doc, line int) bool {
	for _, section := range d.Sections {
		if section.Start <= line && line < section.End && (section.Anchor == "0" || strings.HasPrefix(section.Anchor, "0.")) {
			return true
		}
	}
	return false
}

func sectionAt(d *Doc, line int) string {
	best := ""
	for _, s := range d.Sections {
		if s.Start <= line && line < s.End {
			best = s.Anchor
		}
	}
	return best
}

func equalCols(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if fieldName(a[i]) != fieldName(b[i]) {
			return false
		}
	}
	return true
}

// Op is one typed operation inside an amendment.
type Op struct {
	Verb        string
	Arg         string // target ID, target KIND (add), or §anchor (supersede-section)
	Reason      string
	PayloadCols []string
	PayloadRows [][]string
	Fields      map[string]string // answer payload
	Body        []string          // supersede-section replacement body
}

// Amendment is one parsed ledger amendment.
type Amendment struct {
	Name   string
	Seq    string
	Header map[string]string
	Ops    []Op
}

// ParseAmendment parses an amendment entry against the envelope contract.
func ParseAmendment(name string, normalized []byte) (*Amendment, []schema.Defect) {
	var defects []schema.Defect
	lines := strings.Split(strings.TrimSuffix(string(normalized), "\n"), "\n")
	seq, _ := Seq(name)
	a := &Amendment{Name: name, Seq: seq, Header: map[string]string{}}

	// Envelope: the first Field|Value table.
	i := 0
	for ; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && i+1 < len(lines) && sepRowRe.MatchString(lines[i+1]) {
			break
		}
	}
	if i == len(lines) {
		return nil, []schema.Defect{defect("validate-amendments", name, "envelope", "no envelope header table found", "instantiate from docs/templates/prd-amendment-template.md")}
	}
	j := i + 2
	for ; j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|"); j++ {
		c := cells(lines[j])
		if len(c) >= 2 {
			a.Header[fieldName(c[0])] = c[1]
		}
	}
	for _, f := range schema.EnvelopeFields {
		if _, ok := a.Header[f]; !ok {
			defects = append(defects, defect("validate-amendments", name, "envelope",
				fmt.Sprintf("missing envelope field %q", f), "the envelope is closed: all seven fields, exact names"))
		}
	}
	// Fields beyond the required seven are allowed per the DECLARED
	// template version (FR-24): v1 permits none, v2 permits Tracking.
	optional, versionKnown := envelopeOptional(a.Header["Template version"])
	for f := range a.Header {
		if contains(schema.EnvelopeFields, f) || (versionKnown && contains(optional, f)) {
			continue
		}
		defects = append(defects, defect("validate-amendments", name, "envelope",
			fmt.Sprintf("field %q unknown to the declared template version", f),
			"the envelope is closed per its declared version: remove the field or declare a version that permits it"))
	}

	// Operations.
	for k := j; k < len(lines); k++ {
		m := opRe.FindStringSubmatch(lines[k])
		if m == nil {
			continue
		}
		op := Op{Verb: m[1], Arg: strings.TrimSpace(m[2])}
		end := len(lines)
		for n := k + 1; n < len(lines); n++ {
			if strings.HasPrefix(lines[n], "### ") || strings.HasPrefix(lines[n], "## ") {
				end = n
				break
			}
		}
		block := lines[k+1 : end]
		for bi := 0; bi < len(block); bi++ {
			b := block[bi]
			if strings.HasPrefix(b, "Reason:") {
				op.Reason = strings.TrimSpace(strings.TrimPrefix(b, "Reason:"))
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(b), "|") && bi+1 < len(block) && sepRowRe.MatchString(block[bi+1]) {
				op.PayloadCols = cells(b)
				for ri := bi + 2; ri < len(block) && strings.HasPrefix(strings.TrimSpace(block[ri]), "|"); ri++ {
					op.PayloadRows = append(op.PayloadRows, cells(block[ri]))
					bi = ri
				}
			}
		}
		if op.Verb == schema.OpAnswer {
			op.Fields = map[string]string{}
			if len(op.PayloadCols) == 2 && fieldName(op.PayloadCols[0]) == "Field" {
				for _, r := range op.PayloadRows {
					if len(r) >= 2 {
						op.Fields[fieldName(r[0])] = r[1]
					}
				}
			}
		}
		if op.Verb == schema.OpSupersede && strings.HasPrefix(op.Arg, "§") {
			var body []string
			for _, b := range block {
				if strings.HasPrefix(b, "Reason:") {
					continue
				}
				body = append(body, b)
			}
			op.Body = trimBlank(body)
		}
		switch op.Verb {
		case schema.OpAdd, schema.OpSupersede, schema.OpRetire, schema.OpAnswer, schema.OpRescope:
		default:
			defects = append(defects, defect("validate-amendments", name, "op: "+op.Verb,
				"unknown operation verb", "the vocabulary is closed: add, supersede, retire, answer, rescope"))
			continue
		}
		a.Ops = append(a.Ops, op)
	}
	if len(a.Ops) == 0 {
		defects = append(defects, defect("validate-amendments", name, "operations",
			"no operations found", "an amendment carries at least one '### op:' block"))
	}
	return a, defects
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func trimBlank(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// envelopeOptional returns the optional-field set for a declared envelope
// template version and whether the version is in the supported set.
func envelopeOptional(declared string) ([]string, bool) {
	switch {
	case strings.Contains(declared, schema.AmendmentTemplateV2):
		return schema.EnvelopeOptionalFields[schema.AmendmentTemplateV2], true
	case strings.Contains(declared, schema.AmendmentTemplateVersion):
		return schema.EnvelopeOptionalFields[schema.AmendmentTemplateVersion], true
	}
	return nil, false
}
