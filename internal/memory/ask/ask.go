// Package ask keeps a question from being asked twice. Before a new Question
// record is written, the memory is searched for one that already covers it;
// if that question has an answer, the answer is the result and nothing is
// written; if it is still open, the caller is told to wait for it.
package ask

import (
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/search"
)

// Match is an existing question that covers the one being asked.
type Match struct {
	Question   *record.Record
	Answer     *record.Record // nil while the question is still open
	Similarity float64
}

// Find looks for a question on the subject (any subject when empty) whose
// words overlap the text at or above the threshold (Jaccard over stemmed
// tokens). The best match wins; an answered question beats an open one at
// equal similarity.
func Find(p *resolve.Projection, subject, text string, threshold float64) *Match {
	want := set(search.Tokenize(text))
	if len(want) == 0 {
		return nil
	}
	var best *Match
	subjects := []string{subject}
	if subject == "" {
		subjects = p.Set.Subjects()
	}
	for _, sub := range subjects {
		for _, q := range p.Active(sub) {
			if q.Kind != record.Question {
				continue
			}
			sim := jaccard(want, set(search.Tokenize(q.Statement)))
			if sim < threshold {
				continue
			}
			m := &Match{Question: q, Answer: p.Answer(q.ID), Similarity: sim}
			if best == nil || sim > best.Similarity || (sim == best.Similarity && best.Answer == nil && m.Answer != nil) {
				best = m
			}
		}
	}
	return best
}

func set(tokens []string) map[string]bool {
	out := map[string]bool{}
	for _, t := range tokens {
		out[t] = true
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if b[t] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}
