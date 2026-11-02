// Package shapes parses solution-shape documents — the non-frozen, concrete
// sibling of a technical design (docs/tech-design/solution-shape-v1.md §3.1,
// DINV-3). A shape carries a small CLOSED header — the paired design slug and
// a status drawn from a closed set — and a concrete body expressing the
// schema and interfaces in native form. Every parse failure is a typed
// Defect, never a lenient guess (the docgate defects-as-data pattern). Pure:
// all IO is the caller's; pairing resolution takes an existence predicate.
// Stdlib only, per the repo Go rules.
package shapes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The closed status enum — a shape's lifecycle position (§3.6). It is
// authoritative until its implementing code exists, then superseded-by-code.
const (
	StatusAuthoritative    = "authoritative"
	StatusSupersededByCode = "superseded-by-code"
)

// statuses is the closed set; any other value is the named unknown-status
// defect (DINV-3).
var statuses = map[string]bool{
	StatusAuthoritative:    true,
	StatusSupersededByCode: true,
}

// HeaderFields is the CLOSED solution-shape header schema (§3.1): exactly the
// paired design slug and the status. A missing or unknown field is a defect.
var HeaderFields = []string{"Design", "Status"}

// Shape is one parsed solution-shape — the typed value: a paired-design slug,
// a status, and a concrete body.
type Shape struct {
	Slug   string // the shape's own slug (its filename), the pairing key
	Design string // the paired design's slug (header "Design")
	Status string // authoritative | superseded-by-code
	Body   string // the concrete body — schema + interfaces, verbatim
}

var sepRowRe = regexp.MustCompile(`^\|[\s:|-]+\|?\s*$`)

func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "solution-shape", File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Parse reads one solution-shape document (normalizing internally) into its
// typed value. On any defect it returns a nil Shape and the named defects —
// an unknown status yields the defect and NO value (§3.6). A clean parse
// returns the populated Shape and no defects.
func Parse(slug string, b []byte) (*Shape, []schema.Defect) {
	name := slug + ".md"
	lines := strings.Split(strings.TrimSuffix(string(schema.Normalize(b)), "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return nil, []schema.Defect{def(name, "parse", "document does not start with an H1 title",
			"a solution-shape opens with a '# <title>' heading")}
	}

	// The header is the first markdown table: a pipe row followed by a
	// separator row, its rows read as Field | Value.
	header, j, found := scanHeader(lines)
	if !found {
		return nil, []schema.Defect{def(name, "header", "no header table",
			"a solution-shape carries a '| Field | Value |' header with Design and Status")}
	}
	body := strings.TrimSpace(strings.Join(lines[j:], "\n"))

	var defects []schema.Defect
	// Closed header: exactly Design and Status.
	for _, f := range HeaderFields {
		if _, ok := header[f]; !ok {
			defects = append(defects, def(name, "header", fmt.Sprintf("missing header field %q", f),
				"the solution-shape header is closed: Design and Status"))
		}
	}
	for f := range header {
		if !contains(HeaderFields, f) {
			defects = append(defects, def(name, "header", fmt.Sprintf("unknown header field %q", f),
				"the solution-shape header is closed: remove the field"))
		}
	}
	// Concrete body (§3.1): the schema/interfaces in native form — a fenced
	// block — not prose. An empty or prose-only body is malformed.
	switch {
	case body == "":
		defects = append(defects, def(name, "body", "empty body",
			"a solution-shape body carries the concrete schema and interfaces"))
	case !strings.Contains(body, "```"):
		defects = append(defects, def(name, "body", "body carries no native-form block",
			"express the schema/interfaces in native form (a fenced code block), not prose (§3.1)"))
	}
	// Closed status enum (§3.6): an unknown status is the named defect.
	status := header["Status"]
	if status != "" && !statuses[status] {
		defects = append(defects, def(name, "status", fmt.Sprintf("unknown status %q", status),
			"status is one of: authoritative, superseded-by-code"))
	}

	if len(defects) > 0 {
		return nil, defects
	}
	return &Shape{Slug: slug, Design: header["Design"], Status: status, Body: body}, nil
}

// LooksLikeShape reports whether a document is a solution-shape by its header
// signature — a header table carrying the paired-design pointer field `Design`,
// which no technical-design header contains (its closed schema has Status,
// Owner, … Template version, never Design). This is the detector for a shape
// misfiled into the technical-design corpus, where it would be admitted to the
// generated design index (DINV-3, shape-indexed-as-design). Deliberately
// lenient on the body and status: a shape is a shape by its home and header
// even when malformed — malformedness is the parse defect's job, misfiling is
// this one's. Pure: the caller supplies the bytes.
func LooksLikeShape(b []byte) bool {
	lines := strings.Split(strings.TrimSuffix(string(schema.Normalize(b)), "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		return false
	}
	header, _, found := scanHeader(lines)
	if !found {
		return false
	}
	_, hasDesign := header["Design"]
	return hasDesign
}

// scanHeader finds the first markdown table — a pipe row followed by a
// separator row — and reads its rows as Field | Value, returning the header
// map and the index of the first body line after the table. found reports
// whether a header table was present at all. One header scan, two callers
// (Parse and LooksLikeShape).
func scanHeader(lines []string) (header map[string]string, bodyStart int, found bool) {
	hdrStart := -1
	for i := 1; i+1 < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && sepRowRe.MatchString(lines[i+1]) {
			hdrStart = i
			break
		}
	}
	if hdrStart < 0 {
		return nil, 0, false
	}
	header = map[string]string{}
	j := hdrStart + 2
	for j < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[j]), "|") {
		c := cells(lines[j])
		if len(c) >= 2 {
			header[fieldName(c[0])] = strings.TrimSpace(c[1])
		}
		j++
	}
	return header, j, true
}

// ResolvePairing resolves a shape to its paired design by slug — no registry
// (§3.1). An absent paired design yields the named unpaired-shape defect; a
// resolved pairing yields none. designExists reports whether a design slug is
// present in the technical-design corpus.
func ResolvePairing(s *Shape, designExists func(slug string) bool) []schema.Defect {
	if s == nil {
		return nil
	}
	if s.Design == "" || designExists == nil || !designExists(s.Design) {
		return []schema.Defect{def(s.Slug+".md", "pairing",
			fmt.Sprintf("paired design %q is absent", s.Design),
			"pair the shape (by slug) with an existing design in the technical-design corpus")}
	}
	return nil
}

// cells splits a markdown table row into trimmed cell values.
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

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
