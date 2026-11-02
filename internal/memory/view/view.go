// Package view renders the documents people read from the records: the PRD
// of a subject, a design, the why-chain of a claim, delivery (what is
// designed and what is not), and the status sweep. Nothing here is stored;
// every view is computed from the files on each run.
package view

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
)

// PRD renders the active claims of a subject grouped by kind and ordered by
// family. since marks the records that are new relative to an earlier state
// (ids absent there), so a reader can see what changed.
func PRD(p *resolve.Projection, subject string, since map[string]bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", subject)
	active := p.Active(subject)
	superseded, revoked := 0, 0
	for _, r := range p.Set.BySubject[subject] {
		switch p.Status(r.ID) {
		case resolve.Superseded:
			superseded++
		case resolve.Revoked:
			revoked++
		}
	}
	fmt.Fprintf(&b, "%d active records · history: %d superseded, %d revoked · rendered from memory/%s/\n\n", len(active), superseded, revoked, subject)
	for _, r := range active {
		if r.Kind == record.Document {
			fmt.Fprintf(&b, "Narrative: `%s` document \"%s\" (version %d) — render it with `memory render %s --slot %s`\n\n", r.Family, r.Statement, len(p.Chain(r.ID, 1000)), subject, r.Family)
		}
	}
	if len(since) > 0 {
		var changed []string
		for _, r := range active {
			if since[r.ID] {
				changed = append(changed, r.ID)
			}
		}
		fmt.Fprintf(&b, "**Changed since the reference**: %d records new here", len(changed))
		if len(changed) == 0 {
			b.WriteString(" (nothing)")
		}
		b.WriteString("\n\n")
	}
	for _, kind := range []string{record.Need, record.Promise, record.Expectation, record.Decision, record.Assumption, record.Policy} {
		var rs []*record.Record
		for _, r := range active {
			if r.Kind == kind {
				rs = append(rs, r)
			}
		}
		if len(rs) == 0 {
			continue
		}
		sortByFamily(rs)
		fmt.Fprintf(&b, "## %s\n\n", plural(kind))
		for _, r := range rs {
			mark := ""
			if since[r.ID] {
				if len(r.Rel(record.Supersedes)) > 0 {
					mark = " **(replaced)**"
				} else {
					mark = " **(new)**"
				}
			}
			versions := len(p.Chain(r.ID, 1000)) - 1
			fmt.Fprintf(&b, "### %s%s\n\n%s\n\n", r.Family, mark, r.Statement)
			details(&b, r)
			fmt.Fprintf(&b, "- since %s by %s", r.CreatedAt[:10], r.Issuer)
			if versions > 0 {
				fmt.Fprintf(&b, " · %d earlier version(s)", versions)
			}
			serving := servingDecisions(p, r.ID)
			if len(serving) > 0 {
				fmt.Fprintf(&b, " · served by %s", strings.Join(serving, ", "))
			} else if kind == record.Promise || kind == record.Expectation {
				b.WriteString(" · **undesigned**")
			}
			b.WriteString("\n\n")
		}
	}
	questions(&b, p, subject)
	learnings(&b, p, subject)
	return b.String()
}

func details(b *strings.Builder, r *record.Record) {
	if r.Intent != "" {
		fmt.Fprintf(b, "- intent: %s\n", r.Intent)
	}
	if r.Rationale != "" {
		fmt.Fprintf(b, "- why: %s\n", r.Rationale)
	}
	if r.FitCriterion != "" {
		fmt.Fprintf(b, "- fit criterion: %s\n", r.FitCriterion)
	}
	if len(r.LearnedFrom) > 0 {
		fmt.Fprintf(b, "- learned from: %s\n", strings.Join(r.LearnedFrom, ", "))
	}
	if len(r.Alternatives) > 0 {
		fmt.Fprintf(b, "- alternatives rejected: %s\n", strings.Join(r.Alternatives, "; "))
	}
	if len(r.Tags) > 0 {
		fmt.Fprintf(b, "- tags: %s\n", strings.Join(r.Tags, ", "))
	}
	if r.Expires != "" {
		fmt.Fprintf(b, "- expires: %s\n", r.Expires[:10])
	}
}

func questions(b *strings.Builder, p *resolve.Projection, subject string) {
	var open, answered []*record.Record
	for _, r := range p.Active(subject) {
		if r.Kind != record.Question {
			continue
		}
		if p.Answer(r.ID) == nil {
			open = append(open, r)
		} else {
			answered = append(answered, r)
		}
	}
	if len(open)+len(answered) == 0 {
		return
	}
	b.WriteString("## Questions\n\n")
	for _, q := range open {
		fmt.Fprintf(b, "- **open** (%s, asked %s): %s\n", q.ID, q.CreatedAt[:10], q.Statement)
	}
	for _, q := range answered {
		a := p.Answer(q.ID)
		fmt.Fprintf(b, "- %s → **%s** (%s, %s)\n", q.Statement, a.Statement, a.Issuer, a.CreatedAt[:10])
	}
	b.WriteString("\n")
}

func learnings(b *strings.Builder, p *resolve.Projection, subject string) {
	var ls []*record.Record
	for _, r := range p.Active(subject) {
		if r.Kind == record.Learning {
			ls = append(ls, r)
		}
	}
	if len(ls) == 0 {
		return
	}
	b.WriteString("## Learnings\n\n")
	for _, l := range ls {
		fmt.Fprintf(b, "- %s: %s", l.CreatedAt[:10], l.Statement)
		if len(l.LearnedFrom) > 0 {
			fmt.Fprintf(b, " _(from %s)_", strings.Join(l.LearnedFrom, ", "))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// Render produces the human document of a subject's slot (prd, design …):
// the head Document's narrative with every {{subject#family}} marker replaced
// by the current head of that claim, then the claims the narrative does not
// cover, the questions and the learnings, and the document's own history.
// source names what the records were read from (a ref, or the working tree).
// A broken Document file on this subject and slot is an error: silently
// falling back to an older revision would present stale narrative as current.
func Render(p *resolve.Projection, subject, slot, source string) (string, error) {
	if err := ValidateDocumentFiles(p, subject, slot); err != nil {
		return "", fmt.Errorf("cannot render %s#%s: %w", subject, slot, err)
	}
	head := p.Head(subject, slot)
	if head == nil || head.Kind != record.Document {
		return "", fmt.Errorf("%s has no active %s document; write one with `memory add --kind Document --subject %s --family %s --body-file …`", subject, slot, subject, slot)
	}
	chain := p.Chain(head.ID, 1000)
	var b strings.Builder
	mode := "embedded claims resolve to their current heads"
	if head.LiteralBody {
		mode = "literal source body; brace-shaped text is not interpreted"
	}
	fmt.Fprintf(&b, "> **%s · %s** · version %d · since %s by %s · rendered from %s · %s\n\n",
		subject, slot, len(chain), head.CreatedAt[:10], head.Issuer, source, mode)
	body := p.Set.Bodies[head.ID]
	if head.LiteralBody {
		b.Write(body)
	} else {
		b.WriteString(expandEmbeds(p, body, subject))
	}
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteString("\n")
	}
	covered := map[string]bool{}
	for _, e := range head.Embeds {
		covered[e] = true
	}
	var uncovered []*record.Record
	for _, r := range p.Active(subject) {
		if record.IsClaim(r.Kind) && r.Kind != record.Document && !covered[subject+"#"+r.Family] {
			uncovered = append(uncovered, r) // every claim the narrative leaves out is appended: nothing active is invisible
		}
	}
	if len(uncovered) > 0 {
		sortByFamily(uncovered)
		b.WriteString("\n## Claims this document does not cover\n\n")
		for _, r := range uncovered {
			fmt.Fprintf(&b, "- **%s** (%s): %s\n", r.Family, r.Kind, r.Statement)
		}
	}
	b.WriteString("\n")
	questions(&b, p, subject)
	learnings(&b, p, subject)
	b.WriteString("## Document history\n\n")
	for i, d := range chain {
		fmt.Fprintf(&b, "- v%d · %s · %s · %s", len(chain)-i, d.CreatedAt[:10], d.Issuer, d.ID)
		if d.Intent != "" {
			fmt.Fprintf(&b, " · %s", d.Intent)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

// ValidateDocumentFiles checks whether a subject and slot can be projected
// without discarding a broken Document revision. An empty slot validates all
// slots advertised by a subject overview. A subject with no Document and no
// associated problem is valid: ordinary claim-only views still work.
func ValidateDocumentFiles(p *resolve.Projection, subject, slot string) error {
	if problems := p.Set.DocumentProblems(subject, slot); len(problems) > 0 {
		details := make([]string, 0, len(problems))
		for _, problem := range problems {
			details = append(details, fmt.Sprintf("%s: %v", problem.Path, problem.Err))
		}
		target := subject
		if slot != "" {
			target += "#" + slot
		}
		return fmt.Errorf("Document files for %s are invalid: %s", target, strings.Join(details, "; "))
	}
	return nil
}

// expandEmbeds replaces every marker in a body: a marker alone on its line
// becomes a block, a marker inside a sentence becomes an inline mention.
func expandEmbeds(p *resolve.Projection, body []byte, subject string) string {
	var out strings.Builder
	text := string(body)
	markers := record.EmbedMarkers(body, subject)
	marker := 0
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		end := offset + len(line)
		first := marker
		for marker < len(markers) && atoi(markers[marker][0]) < end {
			marker++
		}
		lineMarkers := markers[first:marker]
		if len(lineMarkers) == 0 {
			out.WriteString(line)
			offset = end
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(lineMarkers) == 1 && trimmed == strings.TrimSpace(text[atoi(lineMarkers[0][0]):atoi(lineMarkers[0][1])]) {
			out.WriteString(embedBlock(p, lineMarkers[0][2]))
			offset = end
			continue
		}
		last := offset
		for _, m := range lineMarkers {
			start, end := atoi(m[0]), atoi(m[1])
			out.WriteString(text[last:start])
			out.WriteString(embedInline(p, m[2]))
			last = end
		}
		out.WriteString(text[last:end])
		offset = end
	}
	return out.String()
}

func embedHead(p *resolve.Projection, ref string) *record.Record {
	i := strings.IndexByte(ref, '#')
	if i < 0 {
		return nil
	}
	return p.Head(ref[:i], ref[i+1:])
}

func embedInline(p *resolve.Projection, ref string) string {
	r := embedHead(p, ref)
	if r == nil {
		return fmt.Sprintf("**⚠ %s has no active head**", ref)
	}
	return fmt.Sprintf("**%s** (%s)", r.Family, strings.Join(strings.Fields(r.Statement), " "))
}

func embedBlock(p *resolve.Projection, ref string) string {
	r := embedHead(p, ref)
	if r == nil {
		return fmt.Sprintf("> **⚠ %s has no active head**\n", ref)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "> **%s** · %s\n", r.Family, strings.Join(strings.Fields(r.Statement), " "))
	var details []string
	if r.Intent != "" {
		details = append(details, "intent: "+r.Intent)
	}
	if r.Rationale != "" {
		details = append(details, "why: "+r.Rationale)
	}
	if r.FitCriterion != "" {
		details = append(details, "fit criterion: "+r.FitCriterion)
	}
	details = append(details, "since "+r.CreatedAt[:10])
	serving := servingDecisions(p, r.ID)
	if len(serving) > 0 {
		details = append(details, "served by "+strings.Join(serving, ", "))
	} else if r.Kind == record.Promise || r.Kind == record.Expectation {
		details = append(details, "**undesigned**")
	}
	fmt.Fprintf(&b, ">\n> _%s_\n", strings.Join(details, " · "))
	return b.String()
}

func servingDecisions(p *resolve.Projection, id string) []string {
	var serving []string
	for _, d := range p.Dependents(id) {
		if d.Type == record.Serves && d.Record.Kind == record.Decision {
			serving = append(serving, d.Record.Subject+"#"+d.Record.Family)
		}
	}
	return serving
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// Design renders a design subject: its anchors, its decisions with what they
// serve, and whether any served claim has moved from under them.
func Design(p *resolve.Projection, subject string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", subject)
	active := p.Active(subject)
	sortByFamily(active)
	reopened := map[string]resolve.Reopen{}
	for _, re := range p.Reopened([]string{subject}) {
		reopened[re.Record.ID+"/"+re.Serves] = re
	}
	n := 0
	for _, r := range active {
		if r.Kind != record.Need {
			continue
		}
		if n == 0 {
			b.WriteString("## Anchors\n\n")
		}
		n++
		fmt.Fprintf(&b, "- %s: %s\n", r.Family, r.Statement)
	}
	if n > 0 {
		b.WriteString("\n")
	}
	b.WriteString("## Decisions\n\n")
	n = 0
	for _, r := range active {
		if r.Kind != record.Decision {
			continue
		}
		n++
		fmt.Fprintf(&b, "### %s\n\n%s\n\n", r.Family, r.Statement)
		details(&b, r)
		for _, t := range r.Rel(record.Serves) {
			target := p.Set.Records[t]
			if target == nil {
				fmt.Fprintf(&b, "- serves %s **(missing)**\n", t)
				continue
			}
			line := fmt.Sprintf("- serves %s#%s: %s", target.Subject, target.Family, clip(target.Statement, 100))
			if re, ok := reopened[r.ID+"/"+t]; ok {
				line += fmt.Sprintf(" **(reopened: the claim is %s", re.Status)
				if re.Successor != nil {
					line += "; the head is now " + re.Successor.ID
				}
				line += ")**"
			}
			b.WriteString(line + "\n")
		}
		for _, t := range r.Rel(record.About) {
			if target := p.Set.Records[t]; target != nil {
				fmt.Fprintf(&b, "- about %s\n", target.Subject)
			}
		}
		for _, t := range r.Rel(record.DependsOn) {
			if target := p.Set.Records[t]; target != nil {
				fmt.Fprintf(&b, "- depends on %s#%s\n", target.Subject, target.Family)
			}
		}
		versions := len(p.Chain(r.ID, 1000)) - 1
		fmt.Fprintf(&b, "- since %s by %s", r.CreatedAt[:10], r.Issuer)
		if versions > 0 {
			fmt.Fprintf(&b, " · %d earlier version(s)", versions)
		}
		b.WriteString("\n\n")
	}
	if n == 0 {
		b.WriteString("- no active decisions\n\n")
	}
	questions(&b, p, subject)
	learnings(&b, p, subject)
	return b.String()
}

// Why renders the chain behind a claim: the head, then each version it
// replaced, with the intent and rationale that moved it.
func Why(p *resolve.Projection, id string) string {
	chain := p.Chain(id, 100)
	if len(chain) == 0 {
		return fmt.Sprintf("%s: no such record\n", id)
	}
	var b strings.Builder
	head := chain[0]
	fmt.Fprintf(&b, "# why %s#%s says what it says\n\n", head.Subject, head.Family)
	for i, r := range chain {
		label := "now"
		if i > 0 {
			label = fmt.Sprintf("v-%d", i)
		}
		fmt.Fprintf(&b, "## %s · %s · %s · %s\n\n%s\n\n", label, p.Status(r.ID), r.CreatedAt[:10], r.ID, r.Statement)
		details(&b, r)
		if r.Provenance.Source != "" || r.Provenance.PR != 0 || r.Provenance.Commit != "" || r.Provenance.Note != "" {
			fmt.Fprintf(&b, "- from: %s", r.Provenance.Source)
			if r.Provenance.PR != 0 {
				fmt.Fprintf(&b, " (PR #%d)", r.Provenance.PR)
			}
			if r.Provenance.Commit != "" {
				fmt.Fprintf(&b, " (commit %s)", r.Provenance.Commit)
			}
			if r.Provenance.Note != "" {
				fmt.Fprintf(&b, " — %s", r.Provenance.Note)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if deps := p.Dependents(head.ID); len(deps) > 0 {
		b.WriteString("## Rests on it now\n\n")
		for _, d := range deps {
			fmt.Fprintf(&b, "- %s %s#%s (%s)\n", d.Type, d.Record.Subject, d.Record.Family, d.Record.Kind)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Delivery renders what is designed and what is not, per promise and
// expectation of a subject. Evidence is out of scope for now, so this is the
// designed / undesigned split, not realized or verified.
func Delivery(p *resolve.Projection, subject string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# delivery — %s\n\n", subject)
	var claims []*record.Record
	for _, r := range p.Active(subject) {
		if r.Kind == record.Promise || r.Kind == record.Expectation {
			claims = append(claims, r)
		}
	}
	sortByFamily(claims)
	undesigned := map[string]bool{}
	for _, r := range p.Undesigned(subject) {
		undesigned[r.ID] = true
	}
	designed := 0
	b.WriteString("| claim | designed by | state |\n|---|---|---|\n")
	for _, r := range claims {
		var by []string
		for _, d := range p.Dependents(r.ID) {
			if d.Type == record.Serves && d.Record.Kind == record.Decision {
				by = append(by, d.Record.Subject+"#"+d.Record.Family)
			}
		}
		state := "designed"
		if undesigned[r.ID] {
			state = "**undesigned**"
		} else {
			designed++
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.Family, strings.Join(by, ", "), state)
	}
	fmt.Fprintf(&b, "\n%d of %d claims designed; %d undesigned. Realization and verification need evidence records, which are not in scope yet.\n", designed, len(claims), len(claims)-designed)
	return b.String()
}

// Status renders the sweep: every subject with its counts, then everything
// that needs a human — conflicts, invalid files, dangling relations, reopened
// designs, orphaned decisions, open questions, expired assumptions.
func Status(p *resolve.Projection, now time.Time) string {
	var b strings.Builder
	b.WriteString("# memory status\n\n| subject | active | superseded | revoked |\n|---|---|---|---|\n")
	total := 0
	for _, sub := range p.Set.Subjects() {
		a, s, r := 0, 0, 0
		for _, rec := range p.Set.BySubject[sub] {
			switch p.Status(rec.ID) {
			case resolve.Active:
				a++
			case resolve.Superseded:
				s++
			case resolve.Revoked:
				r++
			}
		}
		total += len(p.Set.BySubject[sub])
		fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", sub, a, s, r)
	}
	fmt.Fprintf(&b, "\n%d records in %d subjects\n\n## Needs a human\n\n", total, len(p.Set.Subjects()))
	n := 0
	for _, pr := range p.Set.Problems {
		n++
		fmt.Fprintf(&b, "- invalid file %s: %v\n", pr.Path, pr.Err)
	}
	for _, d := range p.Set.Dangling {
		n++
		fmt.Fprintf(&b, "- dangling: %s %s %s\n", d.From, d.Type, d.Target)
	}
	for _, c := range p.Conflicts() {
		n++
		fmt.Fprintf(&b, "- conflict: %s#%s has %d active claims (%s)\n", c.Subject, c.Family, len(c.IDs), strings.Join(c.IDs, ", "))
	}
	for _, v := range p.Violations {
		n++
		fmt.Fprintf(&b, "- %s: %s\n", v.Code, v.What)
	}
	for _, c := range p.AnswerConflicts() {
		n++
		fmt.Fprintf(&b, "- answer-conflict: %s has %d active answers (%s)\n", c.Question, len(c.IDs), strings.Join(c.IDs, ", "))
	}
	for _, c := range p.RevocationConflicts() {
		n++
		fmt.Fprintf(&b, "- revocation-conflict: %s has %d active revocations (%s)\n", c.Target, len(c.IDs), strings.Join(c.IDs, ", "))
	}
	for _, re := range p.Reopened(nil) {
		n++
		fmt.Fprintf(&b, "- reopened: %s#%s serves %s, which is %s\n", re.Record.Subject, re.Record.Family, re.Serves, re.Status)
	}
	for _, o := range p.Orphaned() {
		n++
		fmt.Fprintf(&b, "- orphaned: %s#%s serves nothing that is still wanted (%s)\n", o.Subject, o.Family, clip(o.Statement, 80))
	}
	for _, q := range p.OpenQuestions(nil) {
		n++
		fmt.Fprintf(&b, "- open question on %s since %s: %s\n", q.Subject, q.CreatedAt[:10], q.Statement)
	}
	for _, a := range p.Expired(now) {
		n++
		fmt.Fprintf(&b, "- expired assumption %s#%s (%s): %s\n", a.Subject, a.Family, a.Expires[:10], clip(a.Statement, 80))
	}
	if n == 0 {
		b.WriteString("- nothing\n")
	}
	return b.String()
}

func plural(kind string) string {
	switch kind {
	case record.Policy:
		return "Policies"
	default:
		return kind + "s"
	}
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// sortByFamily orders FR-2 before FR-10: prefix first, then the numeric part.
func sortByFamily(rs []*record.Record) {
	sort.SliceStable(rs, func(i, j int) bool {
		pi, ni := splitFamily(rs[i].Family)
		pj, nj := splitFamily(rs[j].Family)
		if pi != pj {
			return pi < pj
		}
		if ni != nj {
			return ni < nj
		}
		return rs[i].ID < rs[j].ID
	})
}

func splitFamily(f string) (string, int) {
	i := strings.LastIndexAny(f, "-.")
	if i < 0 {
		return f, 0
	}
	n, err := strconv.Atoi(f[i+1:])
	if err != nil {
		return f, 0
	}
	return f[:i], n
}
