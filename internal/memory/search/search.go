// Package search finds the records a statement is about, in two stages.
//
// Stage one is cheap and local: a BM25 index over statements, intents and
// rationales, narrowed by structure — the subject, the family, the tags, the
// code a record is about, and whether it is still in flight on another
// branch. It exists to hand a short list to stage two. Stage two is judgment:
// an external command (an agent CLI of the operator's choosing) reads the
// draft and the candidates and says which it supersedes, narrows, refines,
// conflicts with, or leaves alone. Without a judge, a heuristic stands in and
// says so.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"sort"
	"strings"
	"unicode"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
)

// Relations a judge may return, plus review, which means "a human should look".
const (
	RelSupersedes = "supersedes"
	RelNarrows    = "narrows"
	RelRefines    = "refines"
	RelCompatible = "compatible"
	RelConflicts  = "conflicts"
	RelUnrelated  = "unrelated"
	RelReview     = "review"
)

// Relations lists what a judge may answer.
var Relations = []string{RelSupersedes, RelNarrows, RelRefines, RelCompatible, RelConflicts, RelUnrelated, RelReview}

// ErrJudge wraps every failure of the judge command.
var ErrJudge = errors.New("judge")

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "of": true, "to": true, "in": true, "on": true, "for": true, "is": true,
	"are": true, "be": true, "shall": true, "when": true, "that": true, "this": true, "it": true, "its": true, "as": true, "by": true,
	"with": true, "at": true, "from": true, "not": true, "no": true, "any": true, "all": true, "one": true, "if": true, "then": true,
	"than": true, "into": true, "which": true, "while": true, "must": true, "may": true, "can": true, "will": true, "every": true,
	"each": true, "so": true, "we": true, "our": true, "their": true, "they": true, "was": true, "were": true, "has": true, "have": true,
}

// Tokenize lowercases, splits on anything that is not a letter or digit,
// drops stopwords and one-letter tokens, and trims plural and verb endings so
// "obligations" and "obligation" meet.
func Tokenize(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) < 2 || stopwords[w] {
			continue
		}
		out = append(out, stem(w))
	}
	return out
}

func stem(w string) string {
	switch {
	case len(w) > 4 && strings.HasSuffix(w, "ies"):
		return w[:len(w)-3] + "y"
	case len(w) > 5 && strings.HasSuffix(w, "ing"):
		return w[:len(w)-3]
	case len(w) > 4 && strings.HasSuffix(w, "ed"):
		return w[:len(w)-2]
	case len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss"):
		return w[:len(w)-1]
	}
	return w
}

// Doc is one searchable record.
type Doc struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Subject    string   `json:"subject"`
	Family     string   `json:"family,omitempty"`
	Status     string   `json:"status"`
	Statement  string   `json:"statement"`
	Intent     string   `json:"intent,omitempty"`
	Rationale  string   `json:"rationale,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Code       []string `json:"code,omitempty"`       // code paths the record is about
	InFlight   string   `json:"in_flight,omitempty"`  // the branch carrying it, when not yet on the base
	Body       string   `json:"-"`                    // a Document's narrative; Candidate exposes a bounded excerpt when judgment needs it
	Historical bool     `json:"historical,omitempty"` // session history or derived bookkeeping; explicit --all access only
}

// Index is a BM25 index over docs, built in memory on every run.
type Index struct {
	Docs  []*Doc
	terms []map[string]int
	df    map[string]int
	lens  []int
	avg   float64
}

// Build indexes the docs.
func Build(docs []*Doc) *Index {
	ix := &Index{Docs: docs, df: map[string]int{}}
	total := 0
	for _, d := range docs {
		tf := map[string]int{}
		toks := Tokenize(d.Statement + " " + d.Intent + " " + d.Rationale + " " + strings.Join(d.Tags, " ") + " " + d.Family + " " + d.Body)
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			ix.df[t]++
		}
		ix.terms = append(ix.terms, tf)
		ix.lens = append(ix.lens, len(toks))
		total += len(toks)
	}
	if len(docs) > 0 {
		ix.avg = float64(total) / float64(len(docs))
	}
	return ix
}

const (
	k1 = 1.2
	b  = 0.75
)

func (ix *Index) bm25(q []string, i int) float64 {
	n := float64(len(ix.Docs))
	var score float64
	for _, t := range q {
		tf := ix.terms[i][t]
		if tf == 0 {
			continue
		}
		df := float64(ix.df[t])
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))
		norm := 1 - b + b*float64(ix.lens[i])/math.Max(ix.avg, 1)
		score += idf * float64(tf) * (k1 + 1) / (float64(tf) + k1*norm)
	}
	return score
}

// Query narrows and ranks.
type Query struct {
	Text            string
	Subject         string // exact subject, or a namespace prefix ending in "/"
	Family          string
	Kind            string
	Tags            []string
	Code            []string
	K               int
	IncludeInactive bool
}

// Hit is one ranked doc with what put it there.
type Hit struct {
	Doc     *Doc
	Score   float64
	Lexical float64
	Boosts  []string
	Rank    int
}

// Search ranks docs by BM25 plus structural boosts: the same subject, the
// same family, shared tags, shared code, and in-flight records. Docs with
// nothing in common are dropped.
func (ix *Index) Search(q Query) []Hit {
	toks := Tokenize(q.Text)
	var hits []Hit
	for i, d := range ix.Docs {
		if !q.IncludeInactive && (d.Status != "active" || d.Historical) {
			continue
		}
		if q.Kind != "" && d.Kind != q.Kind {
			continue
		}
		h := Hit{Doc: d, Lexical: ix.bm25(toks, i)}
		h.Score = h.Lexical
		boost := func(w float64, why string) {
			h.Score += w
			h.Boosts = append(h.Boosts, why)
		}
		switch {
		case q.Subject != "" && d.Subject == q.Subject:
			boost(3, "subject")
		case q.Subject != "" && strings.HasSuffix(q.Subject, "/") && strings.HasPrefix(d.Subject, q.Subject):
			boost(1, "namespace")
		case q.Subject != "":
			continue
		}
		if q.Family != "" && d.Family == q.Family && d.Subject == q.Subject {
			boost(4, "family")
		}
		for _, t := range q.Tags {
			for _, dt := range d.Tags {
				if t == dt {
					boost(1.5, "tag:"+t)
				}
			}
		}
		for _, c := range q.Code {
			for _, dc := range d.Code {
				if association.Matches(dc, c) {
					boost(2, "code:"+dc)
					break
				}
			}
		}
		if h.Score <= 0 {
			continue // nothing in common; in-flight alone is not relevance
		}
		if d.InFlight != "" {
			boost(0.5, "in-flight:"+d.InFlight)
		}
		hits = append(hits, h)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Doc.ID < hits[j].Doc.ID
	})
	k := q.K
	if k <= 0 {
		k = 10
	}
	if len(hits) > k {
		hits = hits[:k]
	}
	for i := range hits {
		hits[i].Rank = i + 1
	}
	return hits
}

// Draft is the statement being written, before it is a record.
type Draft struct {
	Kind      string   `json:"kind,omitempty"`
	Subject   string   `json:"subject,omitempty"`
	Family    string   `json:"family,omitempty"`
	Statement string   `json:"statement"`
	Intent    string   `json:"intent,omitempty"`
	Rationale string   `json:"rationale,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Code      []string `json:"code,omitempty"`
}

// Candidate is a stage-one hit handed to the judge.
type Candidate struct {
	Doc
	Rank    int      `json:"rank"`
	Score   float64  `json:"score"`
	Boosts  []string `json:"boosts,omitempty"`
	Excerpt string   `json:"excerpt,omitempty"` // bounded narrative context for a Document hit
}

// Candidates runs stage one for a draft.
func Candidates(ix *Index, d Draft, k int) []Candidate {
	query := d.Statement + " " + d.Intent + " " + d.Rationale
	hits := ix.Search(Query{Text: query, Subject: d.Subject, Family: d.Family, Tags: d.Tags, Code: d.Code, K: k})
	out := make([]Candidate, 0, len(hits))
	for _, h := range hits {
		out = append(out, Candidate{
			Doc: *h.Doc, Rank: h.Rank, Score: math.Round(h.Score*100) / 100, Boosts: h.Boosts,
			Excerpt: candidateExcerpt(h.Doc.Body, query),
		})
	}
	return out
}

const candidateExcerptLimit = 800

// candidateExcerpt gives a judge enough of a Document's narrative to assess
// a body-only match without copying the entire body into the judge request.
// It collapses Markdown whitespace and centers the window near the first
// query term when possible.
func candidateExcerpt(body, query string) string {
	compact := strings.Join(strings.Fields(body), " ")
	runes := []rune(compact)
	if len(runes) <= candidateExcerptLimit {
		return compact
	}

	lower := []rune(strings.ToLower(compact))
	match := -1
	for _, term := range Tokenize(query) {
		if at := runeIndex(lower, []rune(term)); at >= 0 && (match < 0 || at < match) {
			match = at
		}
	}
	if match < 0 {
		return string(runes[:candidateExcerptLimit-1]) + "…"
	}

	content := candidateExcerptLimit - 2 // reserve ellipses on both sides
	start := match - content/3
	if start <= 0 {
		return string(runes[:candidateExcerptLimit-1]) + "…"
	}
	if start+content >= len(runes) {
		return "…" + string(runes[len(runes)-(candidateExcerptLimit-1):])
	}
	return "…" + string(runes[start:start+content]) + "…"
}

func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// Verdict is the judge's answer for one candidate.
type Verdict struct {
	ID       string `json:"id"`
	Relation string `json:"relation"`
	Why      string `json:"why,omitempty"`
}

// JudgeInput is what the judge command reads on stdin.
type JudgeInput struct {
	Draft        Draft       `json:"draft"`
	Candidates   []Candidate `json:"candidates"`
	Relations    []string    `json:"relations"`
	Instructions string      `json:"instructions"`
}

// JudgeOutput is what the judge command writes on stdout.
type JudgeOutput struct {
	Verdicts []Verdict `json:"verdicts"`
}

// Instructions is the standing brief for a judge.
const Instructions = "For each candidate, decide how the draft relates to it: supersedes (the draft replaces it in the same role), " +
	"narrows (the draft restricts it), refines (the draft details it without replacing it), compatible (both can stand), " +
	"conflicts (both cannot stand and the draft does not replace it), unrelated, or review (a human must look). " +
	"Answer with JSON {\"verdicts\":[{\"id\":…,\"relation\":…,\"why\":…}]} and nothing else."

// Heuristic stands in for a judge: the active head of the draft's own family
// is what it supersedes, an in-flight record in that family is concurrent
// work, the top lexical hits deserve a look, the rest are unrelated.
func Heuristic(d Draft, cands []Candidate) []Verdict {
	var out []Verdict
	for _, c := range cands {
		v := Verdict{ID: c.ID}
		switch {
		case d.Subject != "" && c.Subject == d.Subject && d.Family != "" && c.Family == d.Family && c.InFlight != "":
			v.Relation, v.Why = RelConflicts, "another branch ("+c.InFlight+") is changing the same family"
		case d.Subject != "" && c.Subject == d.Subject && d.Family != "" && c.Family == d.Family && c.Status == "active":
			v.Relation, v.Why = RelSupersedes, "the active head of the draft's family"
		case c.Rank <= 3:
			v.Relation, v.Why = RelReview, "a top lexical match; no judge ran"
		default:
			v.Relation = RelUnrelated
		}
		out = append(out, v)
	}
	return out
}

// RunJudge runs the judge command (split on whitespace; no shell) with the
// input on stdin and parses its verdicts. Every verdict must name a candidate
// and a known relation.
func RunJudge(ctx context.Context, command string, in JudgeInput) (JudgeOutput, error) {
	args := strings.Fields(command)
	if len(args) == 0 {
		return JudgeOutput{}, fmt.Errorf("%w: empty command", ErrJudge)
	}
	in.Relations = Relations
	in.Instructions = Instructions
	payload, err := json.Marshal(in)
	if err != nil {
		return JudgeOutput{}, fmt.Errorf("%w: encode input: %v", ErrJudge, err)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return JudgeOutput{}, fmt.Errorf("%w %q: %v: %s", ErrJudge, command, err, strings.TrimSpace(stderr.String()))
	}
	return ParseVerdicts(out, in.Candidates)
}

// ParseVerdicts reads a judge's output. The JSON object may be surrounded by
// prose; the first balanced object is taken. A judge answers for every
// candidate exactly once: a missing, repeated or unknown verdict is an error,
// so a judge that says nothing cannot pass for a judge that found nothing.
func ParseVerdicts(out []byte, cands []Candidate) (JudgeOutput, error) {
	start := bytes.IndexByte(out, '{')
	end := bytes.LastIndexByte(out, '}')
	if start < 0 || end < start {
		return JudgeOutput{}, fmt.Errorf("%w: no JSON object in output: %.200s", ErrJudge, out)
	}
	var res JudgeOutput
	if err := json.Unmarshal(out[start:end+1], &res); err != nil {
		return JudgeOutput{}, fmt.Errorf("%w: parse output: %v", ErrJudge, err)
	}
	known := map[string]bool{}
	for _, c := range cands {
		known[c.ID] = true
	}
	rels := map[string]bool{}
	for _, r := range Relations {
		rels[r] = true
	}
	seen := map[string]bool{}
	for _, v := range res.Verdicts {
		if !known[v.ID] {
			return JudgeOutput{}, fmt.Errorf("%w: verdict names %s, which is not a candidate", ErrJudge, v.ID)
		}
		if seen[v.ID] {
			return JudgeOutput{}, fmt.Errorf("%w: %s is judged twice", ErrJudge, v.ID)
		}
		seen[v.ID] = true
		if !rels[v.Relation] {
			return JudgeOutput{}, fmt.Errorf("%w: verdict for %s uses relation %q; known: %s", ErrJudge, v.ID, v.Relation, strings.Join(Relations, ", "))
		}
	}
	var missing []string
	for _, c := range cands {
		if !seen[c.ID] {
			missing = append(missing, c.ID)
		}
	}
	if len(missing) > 0 {
		return JudgeOutput{}, fmt.Errorf("%w: no verdict for %s; a judge answers for every candidate", ErrJudge, strings.Join(missing, ", "))
	}
	return res, nil
}
