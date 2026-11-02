// Package pack compiles the context an agent gets for a task: the claims in
// scope, the decisions that serve them, the policies that bind, the questions
// already answered and still open, the designs the scope has reopened, the
// work other branches have in flight on the same families, and the learnings
// behind the scope — under a token budget, with every omission named so the
// reader knows what it did not see.
package pack

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/capture"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// Scope selects what the task is about.
type Scope struct {
	Subjects []string // subjects whose active claims are the task (prd/x, design/x …)
	Families []string // optional: only these families of those subjects
	Code     []string `json:"code,omitempty"` // explicit repository-relative task paths
}

// Item is one record as the pack carries it.
type Item struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Subject      string     `json:"subject"`
	Family       string     `json:"family,omitempty"`
	Statement    string     `json:"statement"`
	Intent       string     `json:"intent,omitempty"`
	Rationale    string     `json:"rationale,omitempty"`
	Fit          string     `json:"fit_criterion,omitempty"`
	Serves       []string   `json:"serves,omitempty"`
	Note         string     `json:"note,omitempty"` // branch for in-flight, answer for questions, reopen detail
	Tokens       int        `json:"tokens"`
	Alternatives []string   `json:"alternatives_rejected,omitempty"`
	LearnedFrom  []string   `json:"learned_from,omitempty"`
	Expires      string     `json:"expires,omitempty"`
	Issuer       string     `json:"issuer,omitempty"`
	CreatedAt    string     `json:"created_at,omitempty"`
	Provenance   Provenance `json:"provenance,omitzero"`
	Sources      []string   `json:"sources,omitempty"`
	Code         []string   `json:"code,omitempty"`
}

// Provenance carries attribution without copying a record's potentially large
// evidence inventory. The immutable record identified by ID retains that proof.
type Provenance struct {
	Source string `json:"source,omitempty"`
	PR     int    `json:"pr,omitempty"`
	Commit string `json:"commit,omitempty"`
	Note   string `json:"note,omitempty"`
}

// Pack is the compiled context.
type Pack struct {
	Scope             Scope    `json:"scope"`
	Budget            int      `json:"budget_tokens"`
	Used              int      `json:"used_tokens"`
	Requirements      []Item   `json:"requirements"`
	Decisions         []Item   `json:"decisions"`
	Policies          []Item   `json:"policies"`
	Open              []Item   `json:"open_questions"`
	Answered          []Item   `json:"answered_questions"`
	Reopened          []Item   `json:"reopened"`
	Concurrent        []Item   `json:"concurrent"`
	Learnings         []Item   `json:"learnings"`
	Narrative         []Item   `json:"narrative"` // the document sections that embed the claims in scope
	AdjacentNotLoaded []string `json:"adjacent_not_loaded"`
	Omissions         []string `json:"omissions"`
}

type section struct {
	name  string
	items []Item
	dest  *[]Item
}

// Compile builds the pack under a token budget. The budget bounds what is
// delivered: measure sizes the serialization the caller will hand over
// (markdown when nil), headings and metadata included. Policies are retained;
// requirements, decisions, lessons and source narrative share remaining space
// after auxiliary sections are trimmed. A budget that cannot hold policies
// and the scope envelope is an error, as is a
// non-positive one. inflight maps a branch to the records it adds.
func Compile(p *resolve.Projection, inflight map[string]*store.Set, scope Scope, budget int, measure func(*Pack) int) (*Pack, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	if budget <= 0 {
		return nil, fmt.Errorf("budget %d: a pack needs a positive token budget", budget)
	}
	if measure == nil {
		measure = func(pk *Pack) int { return record.Tokens(pk.Render()) }
	}
	pk := &Pack{Scope: scope, Budget: budget, AdjacentNotLoaded: []string{}, Omissions: []string{}}
	want := map[string]bool{}
	for _, f := range scope.Families {
		want[f] = true
	}
	inScope := map[string]bool{}
	var reqs []Item
	var scopedDecisions []*record.Record
	for _, sub := range scope.Subjects {
		for _, r := range p.Active(sub) {
			if !record.IsClaim(r.Kind) || r.Kind == record.Policy || r.Kind == record.Document || capture.Derived(r) {
				continue
			}
			if len(want) > 0 && !want[r.Family] && !want[sub+"#"+r.Family] {
				pk.AdjacentNotLoaded = append(pk.AdjacentNotLoaded, sub+"#"+r.Family)
				continue
			}
			inScope[r.ID] = true
			if r.Kind == record.Decision {
				scopedDecisions = append(scopedDecisions, r)
				continue
			}
			reqs = append(reqs, item(p, r))
		}
	}
	var decisions []Item
	emitted := map[string]bool{}
	expanded := map[string]bool{}
	addDecision := func(r *record.Record) {
		if capture.Derived(r) {
			return
		}
		if !emitted[r.ID] {
			emitted[r.ID] = true
			decisions = append(decisions, item(p, r))
		}
		if expanded[r.ID] {
			return
		}
		expanded[r.ID] = true
		for _, t := range r.Rel(record.DependsOn) { // closure depth one: what the decision rests on
			if dep := p.Set.Records[t]; dep != nil && !emitted[dep.ID] && p.Status(dep.ID) == resolve.Active && !capture.Derived(dep) {
				emitted[dep.ID] = true
				decisions = append(decisions, item(p, dep))
			}
		}
	}
	for _, d := range scopedDecisions {
		addDecision(d)
	}
	// Explicit code associations join a task to knowledge without guessing from
	// prose or manufacturing code-identity records.
	for _, id := range p.Set.IDs() {
		r := p.Set.Get(id)
		if p.Status(id) == resolve.Active && r.Kind == record.Decision && matchesCode(r, p, scope.Code) {
			addDecision(r)
		}
	}
	for id := range inScope {
		for _, d := range p.Dependents(id) {
			if d.Type != record.Serves || d.Record.Kind != record.Decision {
				continue
			}
			addDecision(d.Record)
		}
	}
	sort.Slice(decisions, func(i, j int) bool {
		// Explicit choices precede generated coverage bindings and dependency
		// documents, so trimming cannot keep only an index of what they serve.
		if a, b := decisionPriority(decisions[i]), decisionPriority(decisions[j]); a != b {
			return a < b
		}
		return decisions[i].ID < decisions[j].ID
	})
	var policies []Item
	for _, sub := range p.Set.Subjects() {
		if record.Namespace(sub) != "governance" {
			continue
		}
		for _, r := range p.Active(sub) {
			if r.Kind == record.Policy {
				policies = append(policies, item(p, r))
			}
		}
	}
	var open, answered []Item
	for _, q := range p.OpenQuestions(scope.Subjects) {
		open = append(open, item(p, q))
	}
	for _, sub := range scope.Subjects {
		for _, r := range p.Active(sub) {
			if r.Kind != record.Question {
				continue
			}
			if a := p.Answer(r.ID); a != nil {
				it := item(p, r)
				it.Note = "answer (" + a.Issuer + ", " + a.CreatedAt[:10] + "): " + a.Statement
				it.Tokens += record.Tokens(it.Note)
				answered = append(answered, it)
			}
		}
	}
	var reopened []Item
	for _, re := range p.Reopened(nil) {
		target := p.Set.Records[re.Serves]
		if target == nil || !subjectIn(target.Subject, scope.Subjects) {
			continue
		}
		it := item(p, re.Record)
		it.Note = fmt.Sprintf("serves %s#%s, which is %s", target.Subject, target.Family, re.Status)
		if re.Successor != nil {
			it.Note += "; the head is now " + re.Successor.ID
		}
		it.Tokens += record.Tokens(it.Note)
		reopened = append(reopened, it)
	}
	var concurrent []Item
	branches := make([]string, 0, len(inflight))
	for b := range inflight {
		branches = append(branches, b)
	}
	sort.Strings(branches)
	for _, b := range branches {
		branch := resolve.New(inflight[b])
		for _, id := range inflight[b].IDs() {
			if p.Set.Get(id) != nil {
				continue // current history governs a record already known here
			}
			r := inflight[b].Records[id]
			if branch.Status(id) != resolve.Active || capture.Derived(r) {
				continue
			}
			if !subjectIn(r.Subject, scope.Subjects) {
				continue
			}
			if len(want) > 0 && !want[r.Family] && !want[r.Subject+"#"+r.Family] {
				continue
			}
			it := item(p, r)
			it.Note = "in flight on " + b
			it.Tokens += record.Tokens(it.Note)
			concurrent = append(concurrent, it)
		}
	}
	var learnings []Item
	code := append([]string{}, scope.Code...)
	for id := range inScope {
		code = append(code, association.Scopes(p.Set.Get(id), p.Set)...)
	}
	for _, it := range decisions {
		code = append(code, association.Scopes(p.Set.Get(it.ID), p.Set)...)
	}
	for _, id := range p.Set.IDs() {
		r := p.Set.Get(id)
		if p.Status(id) != resolve.Active || r.Kind != record.Learning || capture.Derived(r) {
			continue
		}
		linked := subjectIn(r.Subject, scope.Subjects) || matchesCode(r, p, code)
		for _, edge := range r.Relations {
			linked = linked || inScope[edge.Target] || emitted[edge.Target]
		}
		if linked {
			learnings = append(learnings, item(p, r))
		}
	}
	var narrative []Item
	inScopeRefs := map[string]bool{}
	for id := range inScope {
		r := p.Set.Records[id]
		inScopeRefs[r.Subject+"#"+r.Family] = true
	}
	for _, it := range decisions {
		if it.Kind == record.Decision {
			inScopeRefs[it.Subject+"#"+it.Family] = true
		}
	}
	// every active document anywhere may narrate a claim in scope: a design's
	// document embedding {{prd/x#FR-1}} belongs in the pack for prd/x#FR-1;
	// the scope's own documents come first
	docSubjects := append([]string{}, scope.Subjects...)
	for _, sub := range p.Set.Subjects() {
		if !subjectIn(sub, scope.Subjects) {
			docSubjects = append(docSubjects, sub)
		}
	}
	for _, sub := range docSubjects {
		for _, d := range p.Active(sub) {
			if d.Kind != record.Document || sessionlog.Document(d) {
				continue
			}
			for _, sec := range docSections(string(p.Set.Bodies[d.ID])) {
				hit := d.LiteralBody && literalSectionSelected(d, sec, scope, inScopeRefs)
				if !d.LiteralBody {
					for _, ref := range record.ParseEmbeds([]byte(sec.title+"\n"+sec.text), sub) { // a marker in the heading selects the section too
						if inScopeRefs[ref] {
							hit = true
						}
					}
				}
				if !hit {
					continue
				}
				it := Item{ID: d.ID, Kind: d.Kind, Subject: d.Subject, Family: d.Family, Statement: d.Statement + " › " + sec.title, Note: strings.TrimSpace(sec.text)}
				it.Tokens = record.Tokens(it.Statement + it.Note)
				narrative = append(narrative, it)
			}
		}
	}
	sort.Strings(pk.AdjacentNotLoaded)
	// Policies are mandatory context. Requirements, decisions and lessons share
	// the remaining budget, so a large requirements corpus cannot erase all why.
	sections := []section{
		{"requirements", reqs, &pk.Requirements}, {"decisions", decisions, &pk.Decisions}, {"policies", policies, &pk.Policies},
		{"open questions", open, &pk.Open}, {"answered questions", answered, &pk.Answered}, {"reopened", reopened, &pk.Reopened},
		{"concurrent", concurrent, &pk.Concurrent}, {"learnings", learnings, &pk.Learnings}, {"narrative", narrative, &pk.Narrative},
	}
	for _, s := range sections {
		*s.dest = append([]Item{}, s.items...)
	}
	for {
		pk.Omissions = []string{}
		for _, s := range sections {
			if dropped := len(s.items) - len(*s.dest); dropped > 0 {
				pk.Omissions = append(pk.Omissions, fmt.Sprintf("%s: %d of %d omitted for budget", s.name, dropped, len(s.items)))
			}
		}
		// the serialization prints its own used figure, so the measure is a
		// fixed point: measure with the figure in place until it stops moving
		pk.Used = 0
		for range 6 {
			m := measure(pk)
			if m == pk.Used {
				break
			}
			pk.Used = m
		}
		if pk.Used <= budget {
			return pk, nil
		}
		dropped := false
		for i := len(sections) - 1; i >= 3 && !dropped; i-- {
			if sections[i].name == "learnings" || sections[i].name == "narrative" {
				continue
			}
			if n := len(*sections[i].dest); n > 0 {
				*sections[i].dest = (*sections[i].dest)[:n-1]
				dropped = true
			}
		}
		if !dropped {
			dropped = trimKnowledge(pk)
		}
		if !dropped {
			return nil, fmt.Errorf("budget %d too small: policies and scope metadata need %d tokens", budget, pk.Used)
		}
	}
}

type docSection struct {
	title, text string
	ancestors   []string
}

// docSections splits a markdown body at its headings; text before the first
// heading is the preamble.
func docSections(body string) []docSection {
	var out []docSection
	title := "preamble"
	var headings []string
	humanLevel := 0
	var text strings.Builder
	flush := func() {
		if humanLevel == 0 && strings.TrimSpace(text.String()) != "" {
			out = append(out, docSection{title: title, text: text.String(), ancestors: append([]string{}, headings...)})
		}
		text.Reset()
	}
	lines := strings.Split(body, "\n")
	structure := strings.Split(string(record.MaskMarkdownInert([]byte(body))), "\n")
	for i, line := range lines {
		if next, ok := atxHeading(line, structure[i]); ok {
			flush()
			level := len(strings.Fields(strings.TrimSpace(structure[i]))[0])
			if humanLevel != 0 && level <= humanLevel {
				humanLevel = 0
			}
			if humanLevel == 0 && humanRoutingHeading(next) {
				humanLevel = level
			}
			for len(headings) < level {
				headings = append(headings, "")
			}
			headings = headings[:level]
			headings[level-1] = next
			title = next
			continue
		}
		text.WriteString(line)
		text.WriteByte('\n')
	}
	flush()
	return out
}

// atxHeading recognizes a CommonMark ATX heading from the same-length inert-
// masked line, then returns the original heading text. Up to three leading
// spaces and one to six # characters are structural; #text is ordinary text.
func atxHeading(original, structure string) (string, bool) {
	i := 0
	for i < len(structure) && i < 4 && structure[i] == ' ' {
		i++
	}
	if i > 3 {
		return "", false
	}
	start := i
	for i < len(structure) && structure[i] == '#' {
		i++
	}
	if i == start || i-start > 6 || i < len(structure) && structure[i] != ' ' && structure[i] != '\t' && structure[i] != '\r' {
		return "", false
	}
	content := i
	for content < len(original) && (original[content] == ' ' || original[content] == '\t') {
		content++
	}
	end := len(original)
	visibleEnd := len(structure)
	for visibleEnd > content && (structure[visibleEnd-1] == ' ' || structure[visibleEnd-1] == '\t' || structure[visibleEnd-1] == '\r') {
		visibleEnd--
	}
	hashes := visibleEnd
	for hashes > content && structure[hashes-1] == '#' {
		hashes--
	}
	if hashes < visibleEnd && hashes > content && (structure[hashes-1] == ' ' || structure[hashes-1] == '\t') {
		end = hashes
	}
	if end < content {
		end = content
	}
	return strings.TrimSpace(original[content:end]), true
}

func subjectIn(sub string, subjects []string) bool {
	return slices.Contains(subjects, sub)
}

func item(p *resolve.Projection, r *record.Record) Item {
	it := Item{ID: r.ID, Kind: r.Kind, Subject: r.Subject, Family: r.Family, Statement: r.Statement, Intent: r.Intent, Rationale: r.Rationale, Fit: r.FitCriterion}
	it.Alternatives, it.LearnedFrom = r.Alternatives, r.LearnedFrom
	it.Expires, it.Issuer, it.CreatedAt = r.Expires, r.Issuer, r.CreatedAt
	it.Provenance = Provenance{Source: r.Provenance.Source, PR: r.Provenance.PR, Commit: r.Provenance.Commit, Note: r.Provenance.Note}
	it.Code = association.Scopes(r, p.Set)
	if r.Kind == record.Decision {
		for _, d := range p.Active(r.Subject) {
			if d.Kind == record.Document && !sessionlog.Document(d) {
				it.Sources = append(it.Sources, store.Dir+"/"+d.BodyPath())
			}
		}
	}
	for _, t := range r.Rel(record.Serves) {
		if s := p.Set.Records[t]; s != nil {
			it.Serves = append(it.Serves, s.Subject+"#"+s.Family)
		}
	}
	it.Tokens = record.Tokens(it.Statement + it.Intent + it.Rationale + it.Fit + strings.Join(it.Serves, " "))
	it.Tokens += record.Tokens(strings.Join(it.Alternatives, " ") + strings.Join(it.LearnedFrom, " ") + strings.Join(it.Sources, " ") + strings.Join(it.Code, " ") + it.Expires + it.Issuer + it.CreatedAt + it.Provenance.Source + it.Provenance.Commit + it.Provenance.Note)
	return it
}

// Render writes the pack as markdown for an agent's context window.
func (pk *Pack) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Task pack — %s\n\n", strings.Join(pk.Scope.Subjects, ", "))
	fmt.Fprintf(&b, "budget %d tokens, used %d · families: %s\n\n", pk.Budget, pk.Used, orAll(pk.Scope.Families))
	if len(pk.Scope.Code) > 0 {
		fmt.Fprintf(&b, "code scope: %s\n\n", strings.Join(pk.Scope.Code, ", "))
	}
	write := func(title string, items []Item, empty string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(items) == 0 {
			fmt.Fprintf(&b, "- %s\n\n", empty)
			return
		}
		for _, it := range items {
			head := it.Kind
			if it.Family != "" {
				head += " " + it.Family
			}
			fmt.Fprintf(&b, "- **%s** (%s, %s): %s\n", head, it.Subject, it.ID, it.Statement)
			if it.Intent != "" {
				fmt.Fprintf(&b, "  - intent: %s\n", it.Intent)
			}
			if it.Rationale != "" {
				fmt.Fprintf(&b, "  - why: %s\n", it.Rationale)
			}
			if it.Fit != "" {
				fmt.Fprintf(&b, "  - fit criterion: %s\n", it.Fit)
			}
			if len(it.Serves) > 0 {
				fmt.Fprintf(&b, "  - serves: %s\n", strings.Join(it.Serves, ", "))
			}
			if it.Note != "" {
				fmt.Fprintf(&b, "  - %s\n", it.Note)
			}
			for _, alternative := range it.Alternatives {
				fmt.Fprintf(&b, "  - rejected: %s\n", alternative)
			}
			for _, source := range it.LearnedFrom {
				fmt.Fprintf(&b, "  - learned from: %s\n", source)
			}
			for _, source := range it.Sources {
				fmt.Fprintf(&b, "  - source explanation: %s\n", source)
			}
			if len(it.Code) > 0 {
				fmt.Fprintf(&b, "  - code scope: %s\n", strings.Join(it.Code, ", "))
			}
			if it.Expires != "" {
				fmt.Fprintf(&b, "  - expires: %s\n", it.Expires)
			}
			if it.Issuer != "" {
				fmt.Fprintf(&b, "  - recorded by: %s at %s\n", it.Issuer, it.CreatedAt)
			}
			if it.Provenance.Source != "" {
				fmt.Fprintf(&b, "  - source: %s\n", it.Provenance.Source)
			}
			if it.Provenance.PR != 0 {
				fmt.Fprintf(&b, "  - source PR: %d\n", it.Provenance.PR)
			}
			if it.Provenance.Commit != "" {
				fmt.Fprintf(&b, "  - source revision: %s\n", it.Provenance.Commit)
			}
			if it.Provenance.Note != "" {
				fmt.Fprintf(&b, "  - provenance: %s\n", it.Provenance.Note)
			}
		}
		b.WriteString("\n")
	}
	write("Requirements (the task)", pk.Requirements, "no active claims in scope")
	write("Decisions that serve them", pk.Decisions, "no decision serves these claims: they are undesigned")
	write("Policies that bind", pk.Policies, "none recorded")
	write("Open questions — ask before assuming", pk.Open, "none")
	write("Already answered — do not ask again", pk.Answered, "none")
	write("Reopened — decisions whose claim moved", pk.Reopened, "none")
	write("Concurrent — other branches on these families", pk.Concurrent, "none")
	write("Learnings", pk.Learnings, "none")
	write("Narrative around the task", pk.Narrative, "no document section embeds these claims")
	b.WriteString("## Not loaded\n\n")
	if len(pk.AdjacentNotLoaded) == 0 && len(pk.Omissions) == 0 {
		b.WriteString("- nothing: the pack is complete for its scope\n")
	}
	for _, a := range pk.AdjacentNotLoaded {
		fmt.Fprintf(&b, "- adjacent, not requested: %s\n", a)
	}
	for _, o := range pk.Omissions {
		fmt.Fprintf(&b, "- omitted: %s\n", o)
	}
	return b.String()
}

func orAll(xs []string) string {
	if len(xs) == 0 {
		return "all"
	}
	return strings.Join(xs, ", ")
}
