package reconcile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

const designDecisionMarker = "**Decision:**"

var (
	// ErrDesignDecisionExtraction means the source cannot be converted into
	// decision evidence without guessing. Callers must retain the source as
	// unresolved migration debt rather than emitting a partial record set.
	ErrDesignDecisionExtraction = errors.New("design decision extraction failed")

	designSectionNumberRE = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*)(?:\.|[ \t]|$)`)
)

// DesignDecisionBlock is one explicit binding **Decision:** paragraph from a
// live technical design. Statement contains the source bytes after the marker
// (including source line wrapping); Evidence contains the complete source
// paragraph byte-for-byte. Byte offsets are half-open and line numbers are
// one-based, so a caller can independently prove both values against the
// pinned source blob.
//
// Family is a deterministic memory family: decision.<section>.<ordinal>.
// Ordinal is local to Section. Frozen design documents make that structural
// address stable without inventing an identifier that the source never had.
// The extractor deliberately exposes no inferred rationale: adjacent prose is
// explanatory under the design convention and cannot be assigned to one
// decision mechanically.
type DesignDecisionBlock struct {
	Subject            string
	Source             string
	SourceSHA256       string
	Section            string
	SectionHeading     string
	SectionLine        int
	Family             string
	Ordinal            int
	Statement          string
	Evidence           string
	EvidenceSHA256     string
	StartLine          int
	EndLine            int
	MarkerLine         int
	StartByte          int
	EndByte            int
	MarkerByte         int
	StatementStartByte int
	StatementEndByte   int
}

// DesignDecisionExtractionError locates a source construct which made exact
// extraction impossible. It unwraps to ErrDesignDecisionExtraction.
type DesignDecisionExtractionError struct {
	Source string
	Line   int
	Reason string
}

func (e *DesignDecisionExtractionError) Error() string {
	where := e.Source
	if where == "" {
		where = "technical design"
	}
	if e.Line > 0 {
		where += ":" + strconv.Itoa(e.Line)
	}
	return where + ": " + e.Reason
}

func (e *DesignDecisionExtractionError) Unwrap() error {
	return ErrDesignDecisionExtraction
}

// ExtractLiveDesignDecisions returns every explicit binding Decision paragraph
// in source order. It accepts the design-index entry and pinned File together
// so liveness, path identity, and both advertised content digests are checked
// before any evidence is returned.
func ExtractLiveDesignDecisions(design Design, file File) ([]DesignDecisionBlock, error) {
	fail := func(line int, format string, args ...any) ([]DesignDecisionBlock, error) {
		return nil, &DesignDecisionExtractionError{Source: file.Path, Line: line, Reason: fmt.Sprintf(format, args...)}
	}
	if design.Status != "live" {
		return fail(0, "design %q has status %q, not live", design.Slug, design.Status)
	}
	if design.Source == "" || file.Path == "" || design.Source != file.Path {
		return fail(0, "design source %q does not match pinned file %q", design.Source, file.Path)
	}
	if path.Ext(file.Path) != ".md" || strings.TrimSuffix(path.Base(file.Path), ".md") != design.Slug {
		return fail(0, "pinned source path does not identify design %q", design.Slug)
	}
	if !record.ValidSubject("design/" + design.Slug) {
		return fail(0, "design slug %q cannot form a memory subject", design.Slug)
	}
	actual := decisionDigest(file.Body)
	if file.SHA256 == "" || file.SHA256 != actual {
		return fail(0, "pinned file digest %q does not match its bytes (%s)", file.SHA256, actual)
	}
	if design.SHA256 == "" || design.SHA256 != actual {
		return fail(0, "design-index digest %q does not match pinned source bytes (%s)", design.SHA256, actual)
	}

	blocks, err := extractDesignDecisionBlocks(design, file)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return fail(0, "live design has no explicit binding %s paragraph", designDecisionMarker)
	}
	return blocks, nil
}

type decisionSourceLine struct {
	number     int
	start      int
	contentEnd int
	next       int
	original   []byte
	masked     []byte
}

type decisionSection struct {
	level   int
	anchor  string
	title   string
	top     string
	line    int
	ordinal int
}

type pendingDecisionBlock struct {
	block   DesignDecisionBlock
	section *decisionSection
}

func extractDesignDecisionBlocks(design Design, file File) ([]DesignDecisionBlock, error) {
	fail := func(line int, format string, args ...any) error {
		return &DesignDecisionExtractionError{Source: file.Path, Line: line, Reason: fmt.Sprintf(format, args...)}
	}

	maskedBody := record.MaskMarkdownInert(file.Body)
	lines := decisionLines(file.Body, maskedBody)
	sections := make([]*decisionSection, 0, 6)
	sectionOccurrences := map[string]int{}
	var pending []pendingDecisionBlock

	for i := 0; i < len(lines); {
		line := lines[i]
		if decisionLineBlank(line.masked) {
			i++
			continue
		}
		if level, title, anchorPart, ok := parseDecisionHeading(line); ok {
			for len(sections) > 0 && sections[len(sections)-1].level >= level {
				sections = sections[:len(sections)-1]
			}
			parent := ""
			top := ""
			if len(sections) > 0 {
				parentSection := sections[len(sections)-1]
				if parentSection.level >= 2 {
					parent = parentSection.anchor
				}
				top = parentSection.top
			}
			anchor := anchorPart
			if !designSectionNumberRE.MatchString(anchorPart) && parent != "" {
				anchor = parent + "-" + anchorPart
			}
			if level == 1 {
				top = ""
			} else if level == 2 {
				top = anchor
			}
			section := &decisionSection{level: level, anchor: anchor, title: title, top: top, line: line.number}
			sections = append(sections, section)
			if level >= 2 {
				sectionOccurrences[anchor]++
			}
			i++
			continue
		}

		start := i
		for i < len(lines) && !decisionLineBlank(lines[i].masked) {
			if _, _, _, heading := parseDecisionHeading(lines[i]); heading {
				break
			}
			i++
		}
		if i == start { // a heading immediately followed the preceding heading
			continue
		}
		blockLines := lines[start:i]
		blockStart := blockLines[0].start
		blockEnd := blockLines[len(blockLines)-1].next
		maskedBlock := maskedBody[blockStart:blockEnd]
		markers := visibleDecisionMarkers(maskedBlock)
		if len(markers) == 0 {
			continue
		}
		var section *decisionSection
		for j := len(sections) - 1; j >= 0; j-- {
			if sections[j].level >= 2 {
				section = sections[j]
				break
			}
		}
		if section == nil {
			return nil, fail(blockLines[0].number, "%s paragraph has no section heading", designDecisionMarker)
		}
		if section.anchor == "" {
			return nil, fail(section.line, "decision section heading %q has no stable textual identity", section.title)
		}
		// Section 0 is explicitly human-only and non-normative. Its mentions
		// never become memory Decisions.
		if section.top == "0" || strings.HasPrefix(section.top, "0.") {
			continue
		}
		if len(markers) != 1 {
			return nil, fail(blockLines[0].number, "one Markdown paragraph contains %d visible %s markers", len(markers), designDecisionMarker)
		}
		markerStart := blockStart + markers[0]
		markerEnd := markerStart + len(designDecisionMarker)
		markerLine := decisionLineAt(lines, markerStart)
		if markerLine == nil {
			return nil, fail(blockLines[0].number, "internal source-span mismatch at %s", designDecisionMarker)
		}
		if invalidDecisionBlockContext(blockLines, markerLine) {
			return nil, fail(markerLine.number, "%s appears in a list, quote, table, or indented block instead of a decision paragraph", designDecisionMarker)
		}
		if markerEnd >= markerLine.contentEnd || (file.Body[markerEnd] != ' ' && file.Body[markerEnd] != '\t') {
			return nil, fail(markerLine.number, "%s must be followed on the same line by its statement", designDecisionMarker)
		}
		statementStart := markerEnd
		for statementStart < markerLine.contentEnd && (file.Body[statementStart] == ' ' || file.Body[statementStart] == '\t') {
			statementStart++
		}
		if statementStart == markerLine.contentEnd {
			return nil, fail(markerLine.number, "%s has no statement on its marker line", designDecisionMarker)
		}
		statementEnd := blockLines[len(blockLines)-1].contentEnd
		statement := string(file.Body[statementStart:statementEnd])
		if strings.TrimSpace(statement) == "" {
			return nil, fail(markerLine.number, "%s statement is empty", designDecisionMarker)
		}

		section.ordinal++
		evidence := file.Body[blockStart:blockEnd]
		pending = append(pending, pendingDecisionBlock{
			section: section,
			block: DesignDecisionBlock{
				Subject: "design/" + design.Slug, Source: file.Path, SourceSHA256: file.SHA256,
				Section: section.anchor, SectionHeading: section.title, SectionLine: section.line,
				Family: "decision." + section.anchor + "." + strconv.Itoa(section.ordinal), Ordinal: section.ordinal,
				Statement: statement, Evidence: string(evidence), EvidenceSHA256: decisionDigest(evidence),
				StartLine: blockLines[0].number, EndLine: blockLines[len(blockLines)-1].number, MarkerLine: markerLine.number,
				StartByte: blockStart, EndByte: blockEnd, MarkerByte: markerStart,
				StatementStartByte: statementStart, StatementEndByte: statementEnd,
			},
		})
	}

	out := make([]DesignDecisionBlock, 0, len(pending))
	for _, item := range pending {
		if sectionOccurrences[item.section.anchor] != 1 {
			return nil, fail(item.section.line, "section identity %q occurs %d times; decision families would be ambiguous", item.section.anchor, sectionOccurrences[item.section.anchor])
		}
		out = append(out, item.block)
	}
	return out, nil
}

func decisionLines(body, masked []byte) []decisionSourceLine {
	var lines []decisionSourceLine
	for start, number := 0, 1; start < len(body); number++ {
		end := bytes.IndexByte(body[start:], '\n')
		next := len(body)
		if end < 0 {
			end = len(body)
		} else {
			end += start
			next = end + 1
		}
		contentEnd := end
		if contentEnd > start && body[contentEnd-1] == '\r' {
			contentEnd--
		}
		lines = append(lines, decisionSourceLine{
			number: number, start: start, contentEnd: contentEnd, next: next,
			original: body[start:contentEnd], masked: masked[start:contentEnd],
		})
		start = next
	}
	return lines
}

func decisionLineBlank(line []byte) bool {
	return len(bytes.TrimSpace(line)) == 0
}

func parseDecisionHeading(line decisionSourceLine) (level int, title, anchor string, ok bool) {
	i := 0
	for i < len(line.masked) && i < 4 && line.masked[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line.masked) || line.masked[i] != '#' {
		return 0, "", "", false
	}
	start := i
	for i < len(line.masked) && line.masked[i] == '#' {
		i++
	}
	level = i - start
	if level > 6 || i < len(line.masked) && line.masked[i] != ' ' && line.masked[i] != '\t' {
		return 0, "", "", false
	}
	for i < len(line.masked) && (line.masked[i] == ' ' || line.masked[i] == '\t') {
		i++
	}
	end := len(line.masked)
	for end > i && (line.masked[end-1] == ' ' || line.masked[end-1] == '\t') {
		end--
	}
	closing := end
	for closing > i && line.masked[closing-1] == '#' {
		closing--
	}
	if closing < end && closing > i && (line.masked[closing-1] == ' ' || line.masked[closing-1] == '\t') {
		end = closing - 1
		for end > i && (line.masked[end-1] == ' ' || line.masked[end-1] == '\t') {
			end--
		}
	}
	title = strings.TrimSpace(string(line.original[i:end]))
	structuralTitle := strings.TrimSpace(string(line.masked[i:end]))
	if title == "" || structuralTitle == "" {
		return level, title, "", true
	}
	if match := designSectionNumberRE.FindStringSubmatch(structuralTitle); match != nil {
		return level, title, match[1], true
	}
	return level, title, decisionHeadingSlug(structuralTitle), true
}

func decisionHeadingSlug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == '.', r == '_':
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-._")
}

func visibleDecisionMarkers(block []byte) []int {
	marker := []byte(designDecisionMarker)
	var out []int
	for from := 0; from < len(block); {
		at := bytes.Index(block[from:], marker)
		if at < 0 {
			break
		}
		at += from
		backslashes := 0
		for i := at - 1; i >= 0 && block[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			out = append(out, at)
		}
		from = at + len(marker)
	}
	return out
}

func invalidDecisionBlockPrefix(line []byte) bool {
	trimmed := bytes.TrimLeft(line, " \t")
	if len(line) != len(trimmed) || len(trimmed) == 0 {
		return true
	}
	if trimmed[0] == '|' || trimmed[0] == '>' || bytes.HasPrefix(trimmed, []byte("- ")) || bytes.HasPrefix(trimmed, []byte("* ")) || bytes.HasPrefix(trimmed, []byte("+ ")) {
		return true
	}
	digits := 0
	for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits+1 < len(trimmed) && (trimmed[digits] == '.' || trimmed[digits] == ')') && (trimmed[digits+1] == ' ' || trimmed[digits+1] == '\t') {
		return true
	}
	return false
}

// invalidDecisionBlockContext rejects a marker that is textually unprefixed
// but remains inside a Markdown container established on an earlier line of
// the same nonblank block. Markdown permits a list item's continuation to be
// indented by fewer than four spaces, so checking the marker line alone would
// misclassify this as a top-level binding paragraph:
//
//   - explanatory list item
//     **Decision:** illustrative text
//
// Fail closed whenever any line through the marker establishes a structural
// container. Plain wrapped prose remains accepted.
func invalidDecisionBlockContext(block []decisionSourceLine, marker *decisionSourceLine) bool {
	for i := range block {
		if invalidDecisionBlockPrefix(block[i].original) {
			return true
		}
		if block[i].number == marker.number {
			return false
		}
	}
	return true
}

func decisionLineAt(lines []decisionSourceLine, offset int) *decisionSourceLine {
	for i := range lines {
		if offset >= lines[i].start && offset < lines[i].next {
			return &lines[i]
		}
	}
	return nil
}

func decisionDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
