// Package prdescription is the reviewer-facing pull request description every
// proposal carries (.pyxis/templates/pr-description-template.md): the
// template's structure, the check that a description follows it, and the
// renderer the executor uses. The Human Reviewer data comes first so a
// reviewer can size a review before reading the change.
package prdescription

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/changesize"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

// The template's sections, in the order a description gives them.
const (
	SectionReviewerData = "Human Reviewer data"
	SectionWhat         = "What"
	SectionWhy          = "Why"
	SectionHow          = "How"
	SectionTesting      = "Testing"
)

// The Human Reviewer data fields, in order.
const (
	FieldLines       = "Lines of code"
	FieldComplexity  = "Complexity"
	FieldBlastRadius = "Blast radius"
	FieldDecision    = "Decision type"
	FieldReviewNotes = "Review notes"
)

var (
	sections = []string{SectionReviewerData, SectionWhat, SectionWhy, SectionHow, SectionTesting}
	fields   = []string{FieldLines, FieldComplexity, FieldBlastRadius, FieldDecision, FieldReviewNotes}
)

// The markers delimit a description inside a larger text: an implementing
// agent's final message, or a proposal body that also carries sections other
// writers own.
const (
	BeginMarker = "<!-- pyxis:pr-description -->"
	EndMarker   = "<!-- /pyxis:pr-description -->"
)

// The ratings for Complexity and Blast radius.
const (
	Low    = "low"
	Medium = "medium"
	High   = "high"
)

// The Decision type values: whether the change can be reversed later without
// rewriting a large part of the system.
const (
	OneWay = "one-way"
	TwoWay = "two-way"
)

// Description is a parsed or composed description. Reviewer data values keep
// the author's text after the label, e.g. "medium (shared executor path)"; a
// value that continues on indented lines keeps them after a newline.
type Description struct {
	LinesOfCode string
	Complexity  string
	BlastRadius string
	Decision    string
	ReviewNotes string
	What        string
	Why         string
	How         string
	Testing     string
	// Rest is whatever follows the Testing section, kept verbatim.
	Rest string
}

// Problem is one way a description misses the template.
type Problem struct {
	// Field is the section or reviewer data field the problem concerns, empty
	// for the description's overall structure.
	Field   string
	Message string
}

func (p Problem) String() string {
	if p.Field == "" {
		return p.Message
	}
	return p.Field + ": " + p.Message
}

// block is one template section's lines, in document order.
type block struct {
	name  string
	lines []string
}

// Parse reads the description in body: the text between the markers when the
// body has them, otherwise the whole body. HTML comments are dropped first,
// since a reader never sees them. Parse reports every way the text misses the
// template's structure and values; comparing the line count with a change is
// Check's job.
func Parse(body string) (Description, []Problem) {
	text := strings.ReplaceAll(body, "\r\n", "\n")
	if inner, ok := Extract(text); ok {
		text = inner
	}
	text = stripComments(text)

	var problems []Problem
	report := func(field, format string, args ...any) {
		problems = append(problems, Problem{Field: field, Message: fmt.Sprintf(format, args...)})
	}
	const preamble, discard = -1, -2
	var blocks []block
	var before, rest []string
	current, inRest, testing := preamble, false, false
	seen := map[string]bool{}
	fence := ""
	for line := range strings.SplitSeq(text, "\n") {
		if inRest {
			rest = append(rest, line)
			continue
		}
		title, heading := headingTitle(line, &fence)
		if heading {
			name, known := sectionNamed(title)
			switch {
			case testing && (!known || seen[name]):
				inRest = true // a section of the author's own after the template's
				rest = append(rest, line)
			case !known:
				report("", "unexpected section %q before ## %s; the template's sections come first, in order: %s", "## "+title, SectionTesting, strings.Join(sections, ", "))
				current = discard
			case seen[name]:
				report(name, "the section appears more than once")
				current = discard
			default:
				seen[name] = true
				blocks = append(blocks, block{name: name})
				current = len(blocks) - 1
				testing = testing || name == SectionTesting
			}
			continue
		}
		switch current {
		case preamble:
			before = append(before, line)
		case discard:
		default:
			blocks[current].lines = append(blocks[current].lines, line)
		}
	}

	if strings.TrimSpace(strings.Join(before, "\n")) != "" {
		report("", "text before ## %s; the description starts with that section", SectionReviewerData)
	}
	for i := 1; i < len(blocks); i++ {
		if indexOf(sections, blocks[i].name) < indexOf(sections, blocks[i-1].name) {
			report("", "sections out of order; give them in this order: %s", strings.Join(sections, ", "))
			break
		}
	}
	for _, name := range sections {
		if !seen[name] {
			report(name, "missing ## %s section", name)
		}
	}

	var d Description
	for _, b := range blocks {
		content := strings.TrimSpace(strings.Join(b.lines, "\n"))
		switch b.name {
		case SectionReviewerData:
			parseReviewerData(b.lines, &d, report)
		case SectionWhat:
			d.What = content
		case SectionWhy:
			d.Why = content
		case SectionHow:
			d.How = content
		case SectionTesting:
			d.Testing = content
			if !checkbox.MatchString(content) {
				report(b.name, "list each check as - [x] <check and result>, or - [ ] <check not run, and why>")
			}
		}
		if b.name != SectionReviewerData && content == "" {
			report(b.name, "the section is empty")
		}
	}
	d.Rest = strings.TrimSpace(strings.Join(rest, "\n"))
	return d, problems
}

var (
	checkbox    = regexp.MustCompile(`(?m)^\s*[-*+]\s+\[[ xX]\]\s+\S`)
	fieldLine   = regexp.MustCompile(`^[-*+]\s+(?:\*\*)?([A-Za-z][A-Za-z ]*?)\s*(?::\*\*|\*\*:|:)(.*)$`)
	linesFormat = regexp.MustCompile(`(?i)^(?:n/a\b.*|\d+(?: \(\+\d+/-\d+\))?)$`)
	linesCount  = regexp.MustCompile(`^(\d+)(?: \(\+(\d+)/-(\d+)\))?$`)
	rating      = regexp.MustCompile(`(?i)^(low|medium|high)\b\.?(.*)$`)
	decision    = regexp.MustCompile(`(?i)^(one|two)[- ]way\b\.?(.*)$`)
)

// parseReviewerData reads the field bullets. A field's value may continue on
// indented lines below it.
func parseReviewerData(lines []string, d *Description, report func(string, string, ...any)) {
	values := map[string]string{}
	var order []string
	field := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indented := line[0] == ' ' || line[0] == '\t'
		if m := fieldLine.FindStringSubmatch(line); m != nil && !indented {
			name, ok := fieldNamed(m[1])
			switch {
			case !ok:
				report(SectionReviewerData, "unexpected field %q; the fields are: %s", strings.TrimSpace(m[1]), strings.Join(fields, ", "))
				field = ""
			case indexOf(order, name) >= 0:
				report(name, "the field appears more than once")
				field = ""
			default:
				field = name
				order = append(order, name)
				values[name] = strings.TrimSpace(m[2])
			}
			continue
		}
		if field != "" && indented {
			values[field] += "\n" + strings.TrimRight(line, " \t")
			continue
		}
		report(SectionReviewerData, "unexpected line %q; give each field as a bullet: - <Field>: <value>", trimmed)
		field = ""
	}
	for i := 1; i < len(order); i++ {
		if indexOf(fields, order[i]) < indexOf(fields, order[i-1]) {
			report(SectionReviewerData, "fields out of order; give them in this order: %s", strings.Join(fields, ", "))
			break
		}
	}
	d.LinesOfCode, d.Complexity, d.BlastRadius = values[FieldLines], values[FieldComplexity], values[FieldBlastRadius]
	d.Decision, d.ReviewNotes = values[FieldDecision], values[FieldReviewNotes]
	for _, name := range fields {
		value := strings.TrimSpace(values[name])
		first, _, _ := strings.Cut(value, "\n")
		switch {
		case indexOf(order, name) < 0:
			report(name, "missing; add - %s: <value>", name)
		case value == "":
			report(name, "the value is empty")
		case name == FieldLines && !linesFormat.MatchString(strings.TrimSpace(first)):
			report(name, "write the count as <total> (+<added>/-<removed>), or n/a when the change touches no code or prompt file")
		case (name == FieldComplexity || name == FieldBlastRadius) && !valid(rating, first):
			report(name, "start the value with low, medium, or high")
		case name == FieldDecision && !valid(decision, first):
			report(name, "start the value with one-way or two-way")
		}
	}
}

// valid reports a value that starts with one of the pattern's words and,
// optionally, continues with a note: "medium (shared executor path)" or
// "two-way — the template can change freely", never "low | medium | high"
// or "medium-high".
func valid(pattern *regexp.Regexp, value string) bool {
	m := pattern.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return false
	}
	raw := m[len(m)-1]
	note := strings.TrimSpace(raw)
	if note == "" {
		return true
	}
	for _, prefix := range []string{"(", ":", ";", ","} {
		if strings.HasPrefix(note, prefix) {
			return true
		}
	}
	for _, dash := range []string{"—", "–", "-"} {
		if strings.HasPrefix(raw, " ") && strings.HasPrefix(note, dash) {
			return true
		}
	}
	return false
}

// Check parses body and compares its line count with the measured change.
func Check(body string, size changesize.Size) []Problem {
	d, problems := Parse(body)
	for _, p := range problems {
		if p.Field == FieldLines {
			return problems // missing or malformed; Parse has said so
		}
	}
	value, _, _ := strings.Cut(strings.TrimSpace(d.LinesOfCode), "\n")
	value = strings.TrimSpace(value)
	want := size.String()
	hint := fmt.Sprintf("says %q, but the change measures %s; write %q (`pyxis pr-description lines` prints it)", value, want, want)
	if !size.Applies() {
		if !strings.HasPrefix(strings.ToLower(value), "n/a") {
			problems = append(problems, Problem{Field: FieldLines, Message: hint})
		}
		return problems
	}
	m := linesCount.FindStringSubmatch(value)
	if m == nil || atoi(m[1]) != size.Total() || (m[2] != "" && (atoi(m[2]) != size.Added() || atoi(m[3]) != size.Removed())) {
		problems = append(problems, Problem{Field: FieldLines, Message: hint})
	}
	return problems
}

// atoi converts a run of digits the caller's pattern already matched.
func atoi(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil {
		return -1 // out of range; never equal to a measured count
	}
	return n
}

// Render writes the description in the template's form, between the markers.
// Empty values are written empty, so a rendered description of an incomplete
// Description fails Parse the same way the incomplete one would.
func (d Description) Render() string {
	var b strings.Builder
	b.WriteString(BeginMarker + "\n## " + SectionReviewerData + "\n")
	for _, f := range []struct{ name, value string }{
		{FieldLines, d.LinesOfCode}, {FieldComplexity, d.Complexity}, {FieldBlastRadius, d.BlastRadius},
		{FieldDecision, d.Decision}, {FieldReviewNotes, d.ReviewNotes},
	} {
		first, more, continued := strings.Cut(strings.TrimRight(f.value, " \t\n"), "\n")
		b.WriteString("- " + f.name + ":")
		if first = strings.TrimSpace(first); first != "" {
			b.WriteString(" " + first)
		}
		b.WriteString("\n")
		if continued {
			b.WriteString(more + "\n")
		}
	}
	for _, s := range []struct{ name, body string }{
		{SectionWhat, d.What}, {SectionWhy, d.Why}, {SectionHow, d.How}, {SectionTesting, d.Testing},
	} {
		b.WriteString("\n## " + s.name + "\n" + strings.TrimSpace(s.body) + "\n")
	}
	if rest := strings.TrimSpace(d.Rest); rest != "" {
		b.WriteString("\n" + rest + "\n")
	}
	b.WriteString(EndMarker)
	return b.String()
}

// Skeleton is an unfilled description between the markers: the shape an
// implementing agent ends its final message with.
func Skeleton() string {
	return Description{
		LinesOfCode: "Pyxis computes this",
		Complexity:  "low | medium | high",
		BlastRadius: "low | medium | high",
		Decision:    "one-way | two-way",
		ReviewNotes: "…",
		What:        "- …",
		Why:         "…",
		How:         "- …",
		Testing:     "- [x] …",
	}.Render()
}

// Replace puts rendered, a Render result, in place of the marked description
// in body, keeping everything around it. A body without one gets the
// description at the top.
func Replace(body, rendered string) string {
	if start := strings.Index(body, BeginMarker); start >= 0 {
		if end := strings.Index(body[start:], EndMarker); end >= 0 {
			return body[:start] + rendered + body[start+end+len(EndMarker):]
		}
	}
	if strings.TrimSpace(body) == "" {
		return rendered + "\n"
	}
	return rendered + "\n\n" + strings.TrimLeft(body, "\n")
}

// Extract returns the text between the last pair of markers in s.
func Extract(s string) (string, bool) {
	start := strings.LastIndex(s, BeginMarker)
	if start < 0 {
		return "", false
	}
	inner, _, found := strings.Cut(s[start+len(BeginMarker):], EndMarker)
	return inner, found
}

// FromAgent reads the description an implementing agent ends its final
// message with. Pyxis computes the line count, so problems with that field
// are dropped. The text is the agent's and is published by Pyxis, so it gets
// no say over machine effects: HTML comments are already gone (no forged
// markers), and issue-closing references are defused (only Pyxis links the
// task's issue).
func FromAgent(message string) (Description, []Problem) {
	inner, ok := Extract(strings.ReplaceAll(message, "\r\n", "\n"))
	if !ok {
		return Description{}, []Problem{{Message: "the final message has no description between " + BeginMarker + " and " + EndMarker}}
	}
	d, problems := Parse(inner)
	var kept []Problem
	for _, p := range problems {
		if p.Field != FieldLines {
			kept = append(kept, p)
		}
	}
	for _, field := range []*string{&d.Complexity, &d.BlastRadius, &d.Decision, &d.ReviewNotes, &d.What, &d.Why, &d.How, &d.Testing, &d.Rest} {
		*field = defuseClosing(*field)
	}
	return d, kept
}

// ComplexityFor maps a task's derived implementation capability to the
// template's complexity: economy and standard are low, strong is medium, and
// frontier is high, the easy-and-hard reading the planning guide already
// applies to these profiles.
func ComplexityFor(c executiondemand.Capability) (string, bool) {
	switch c {
	case executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard:
		return Low, true
	case executiondemand.CapabilityStrong:
		return Medium, true
	case executiondemand.CapabilityFrontier:
		return High, true
	}
	return "", false
}

var (
	issueRef    = `(?:#\d+|[\w.-]+/[\w.-]+#\d+|https?://[^\s,)]+)`
	closingRefs = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\b\s*:?\s+` + issueRef + `(?:\s*,\s*(?:and\s+)?` + issueRef + `)*`)
	refInList   = regexp.MustCompile(issueRef)
)

// defuseClosing puts each issue reference that follows a closing keyword
// ("Fixes #12", "closes owner/repo#3, #4") in a code span, which code hosts
// do not link. The keyword and the number stay readable.
func defuseClosing(s string) string {
	return closingRefs.ReplaceAllStringFunc(s, func(match string) string {
		return refInList.ReplaceAllStringFunc(match, func(ref string) string { return "`" + ref + "`" })
	})
}

// headingTitle reports whether line is a level-2 heading outside a code
// fence, and its title. fence carries an open fence between calls.
func headingTitle(line string, fence *string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if run := fenceRun(trimmed); run != "" && len(line)-len(trimmed) <= 3 {
		if *fence == "" {
			*fence = run
		} else if run[0] == (*fence)[0] && len(run) >= len(*fence) && strings.TrimSpace(trimmed[len(run):]) == "" {
			*fence = ""
		}
		return "", false
	}
	if *fence != "" || !strings.HasPrefix(line, "## ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[3:]), "#")), true
}

// fenceRun is the backtick or tilde run that opens or closes a code fence.
func fenceRun(s string) string {
	if len(s) < 3 || (s[0] != '`' && s[0] != '~') {
		return ""
	}
	n := 1
	for n < len(s) && s[n] == s[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return s[:n]
}

func sectionNamed(title string) (string, bool) { return named(sections, title) }

func fieldNamed(label string) (string, bool) { return named(fields, label) }

// named matches a heading or label case-insensitively, ignoring a trailing
// colon, against the template's names.
func named(names []string, s string) (string, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), ":"))
	for _, name := range names {
		if strings.EqualFold(s, name) {
			return name, true
		}
	}
	return "", false
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// stripComments drops HTML comments. An unclosed comment hides the rest of
// the text, as it does when rendered.
func stripComments(s string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		j := strings.Index(s[i+4:], "-->")
		if j < 0 {
			return b.String()
		}
		s = s[i+4+j+3:]
	}
}

// Publish replaces the description in a proposal body through a code host's
// read and write. It uses the execution summary's optimistic protocol: re-read
// before writing so a concurrent edit is never overwritten, read back after
// writing, and give up after three attempts.
func Publish(ctx context.Context, rendered string, read func(context.Context) (string, error), write func(context.Context, string) error) error {
	for range 3 {
		body, err := read(ctx)
		if err != nil {
			return err
		}
		updated := Replace(body, rendered)
		if updated == body {
			return nil
		}
		fresh, err := read(ctx)
		if err != nil {
			return err
		}
		if fresh != body {
			continue
		}
		if err := write(ctx, updated); err != nil {
			return err
		}
		after, err := read(ctx)
		if err != nil {
			return err
		}
		if Replace(after, rendered) == after {
			return nil
		}
	}
	return errors.New("prdescription: the proposal description did not stabilize within three attempts")
}
