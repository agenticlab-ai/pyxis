package search

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTokenize(t *testing.T) {
	got := Tokenize("The executor shall build the ready frontier of tasks; obligations are recorded, reviewing entries, retried.")
	want := "executor build ready frontier task obligation record review entry retri"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q want %q", strings.Join(got, " "), want)
	}
	if len(Tokenize("a an the")) != 0 {
		t.Fatal("stopwords only")
	}
	if stem("class") != "class" || stem("policies") != "policy" || stem("is") != "is" {
		t.Fatal("stem")
	}
}

func docs() []*Doc {
	return []*Doc{
		{ID: "Promise-1", Kind: "Promise", Subject: "prd/implement-now", Family: "FR-1", Status: "active",
			Statement: "When invoked on a frozen plan, the executor shall build the ready frontier of open, unblocked, unclaimed tasks.", Code: []string{"internal/executor"}},
		{ID: "Promise-2", Kind: "Promise", Subject: "prd/implement-now", Family: "FR-33", Status: "active",
			Statement: "The executor shall establish one durable pending review obligation keyed to the task and proposal.", Tags: []string{"review"}},
		{ID: "Promise-3", Kind: "Promise", Subject: "prd/impl-review", Family: "FR-2", Status: "active",
			Statement: "A review verdict shall be published as an artifact bound to the proposal revision.", Tags: []string{"review"}, Code: []string{"internal/implreview"}},
		{ID: "Promise-4", Kind: "Promise", Subject: "prd/implement-now", Family: "FR-33", Status: "superseded",
			Statement: "The executor shall record a review obligation."},
		{ID: "Decision-5", Kind: "Decision", Subject: "design/implement-now", Family: "D-1", Status: "active",
			Statement: "A dispatcher over a portable worker; the review obligation is written at exact green.", Intent: "keep review durable", Code: []string{"internal/executor"}},
		{ID: "Promise-6", Kind: "Promise", Subject: "prd/implement-now", Family: "FR-33", Status: "active",
			Statement: "Review obligations capture the exact verified head revision atomically.", InFlight: "feature/auth-review"},
		{ID: "Need-7", Kind: "Need", Subject: "prd/other", Family: "G-1", Status: "active", Statement: "Ship the thing."},
	}
}

func TestSearchRanksLexicallyAndByStructure(t *testing.T) {
	ix := Build(docs())
	hits := ix.Search(Query{Text: "durable review obligation for the task", K: 3})
	if len(hits) != 3 || hits[0].Doc.ID != "Promise-2" || hits[0].Rank != 1 {
		t.Fatalf("lexical top: %+v", ids(hits))
	}
	for _, h := range hits {
		if h.Doc.ID == "Promise-4" {
			t.Fatal("superseded records are hidden by default")
		}
	}
	all := ix.Search(Query{Text: "review obligation", IncludeInactive: true, K: 10})
	if !contains(ids(all), "Promise-4") {
		t.Fatal("IncludeInactive shows superseded records")
	}
	// same subject + family outranks a better lexical match elsewhere
	hits = ix.Search(Query{Text: "review verdict artifact", Subject: "prd/implement-now", Family: "FR-33", K: 5})
	if hits[0].Doc.Family != "FR-33" || hits[1].Doc.Family != "FR-33" || !contains(hits[0].Boosts, "family") || !contains(hits[0].Boosts, "subject") {
		t.Fatalf("structure boost: %+v %+v", hits[0], hits[1])
	}
	if contains(ids(hits), "Promise-3") {
		t.Fatal("an exact subject filter excludes other subjects")
	}
	// a namespace prefix keeps every prd but boosts nothing beyond the namespace
	hits = ix.Search(Query{Text: "review verdict artifact", Subject: "prd/", K: 5})
	if hits[0].Doc.ID != "Promise-3" || !contains(hits[0].Boosts, "namespace") || contains(ids(hits), "Decision-5") {
		t.Fatalf("namespace: %+v", ids(hits))
	}
	// tags, code and in-flight boosts
	hits = ix.Search(Query{Text: "frontier revision", Tags: []string{"review"}, Code: []string{"internal/executor/worker.go"}, K: 10})
	if hits[0].Doc.ID != "Promise-1" || !contains(hits[0].Boosts, "code:internal/executor") {
		t.Fatalf("code boost: %+v", hits[0])
	}
	var tagged, inflight bool
	for _, h := range hits {
		if h.Doc.ID == "Promise-2" && contains(h.Boosts, "tag:review") {
			tagged = true
		}
		if h.Doc.ID == "Promise-6" && contains(h.Boosts, "in-flight:feature/auth-review") {
			inflight = true
		}
	}
	if !tagged || !inflight {
		t.Fatalf("tag/in-flight boosts missing: %+v", hits)
	}
	if got := ix.Search(Query{Text: "frontier", Kind: "Decision"}); len(got) != 0 {
		t.Fatalf("kind filter: %v", ids(got))
	}
	if got := ix.Search(Query{Text: "zzz nothing"}); len(got) != 0 {
		t.Fatalf("nothing in common → nothing: %v", ids(got))
	}
	if got := Build(nil).Search(Query{Text: "x"}); len(got) != 0 {
		t.Fatal("empty index")
	}
	withDoc := append(docs(), &Doc{ID: "Document-8", Kind: "Document", Subject: "prd/implement-now", Family: "prd", Status: "active", Statement: "The PRD",
		Body: "Narrative about the hermeticity admission before a proposal is opened."})
	if hits := Build(withDoc).Search(Query{Text: "hermeticity admission"}); len(hits) != 1 || hits[0].Doc.ID != "Document-8" {
		t.Fatalf("bodies are indexed: %v", ids(hits))
	}
}

func TestSearchCodeMatchesOnlyAtPathBoundaries(t *testing.T) {
	ix := Build([]*Doc{
		{ID: "Promise-foo", Kind: "Promise", Status: "active", Statement: "foo", Code: []string{"internal/foo"}},
		{ID: "Promise-foobar", Kind: "Promise", Status: "active", Statement: "foobar", Code: []string{"internal/foobar"}},
		{ID: "Promise-old", Kind: "Promise", Status: "superseded", Statement: "old", Code: []string{"internal/old"}},
	})
	hits := ix.Search(Query{Code: []string{"internal/foo/file.go"}, K: 10})
	if got := ids(hits); len(got) != 1 || got[0] != "Promise-foo" {
		t.Fatalf("path-boundary matches = %v, want [Promise-foo]", got)
	}
	hits = ix.Search(Query{Code: []string{"internal"}, K: 10})
	if got := ids(hits); !contains(got, "Promise-foo") || !contains(got, "Promise-foobar") {
		t.Fatalf("parent path matches = %v, want both children", got)
	}
	if got := ix.Search(Query{Code: []string{"internal/old"}, K: 10}); len(got) != 0 {
		t.Fatalf("inactive code association matched by default: %v", ids(got))
	}
	if got := ix.Search(Query{Code: []string{"internal/old"}, K: 10, IncludeInactive: true}); len(got) != 1 || got[0].Doc.ID != "Promise-old" {
		t.Fatalf("IncludeInactive code matches = %v, want [Promise-old]", ids(got))
	}
}

func TestCandidatesAndHeuristic(t *testing.T) {
	ix := Build(docs())
	d := Draft{Kind: "Promise", Subject: "prd/implement-now", Family: "FR-33", Statement: "Capture the review obligation at exact green with the verified revision."}
	cands := Candidates(ix, d, 5)
	if len(cands) == 0 || cands[0].Rank != 1 {
		t.Fatalf("candidates: %+v", cands)
	}
	verdicts := Heuristic(d, cands)
	byID := map[string]Verdict{}
	for _, v := range verdicts {
		byID[v.ID] = v
	}
	if byID["Promise-2"].Relation != RelSupersedes {
		t.Fatalf("head of the family: %+v", byID["Promise-2"])
	}
	if byID["Promise-6"].Relation != RelConflicts || !strings.Contains(byID["Promise-6"].Why, "feature/auth-review") {
		t.Fatalf("in-flight same family: %+v", byID["Promise-6"])
	}
	var reviews, unrelated int
	for _, v := range verdicts {
		switch v.Relation {
		case RelReview:
			reviews++
		case RelUnrelated:
			unrelated++
		}
	}
	if reviews == 0 {
		t.Fatalf("top hits get review: %+v", verdicts)
	}
	if len(cands) > 3 && unrelated == 0 {
		t.Fatalf("tail is unrelated: %+v", verdicts)
	}
}

func TestDocumentCandidateCarriesBoundedRelevantExcerpt(t *testing.T) {
	body := "PRIVATE-PREFIX " + strings.Repeat("background context ", 120) +
		"Hermeticity admission happens before a proposal is opened. " +
		strings.Repeat("later detail ", 120) + " PRIVATE-SUFFIX"
	doc := &Doc{ID: "Document-1", Kind: "Document", Subject: "prd/x", Family: "prd", Status: "active", Statement: "The PRD", Body: body}
	draft := Draft{Statement: "hermeticity admission"}
	cands := Candidates(Build([]*Doc{doc}), draft, 1)
	if len(cands) != 1 || !strings.Contains(cands[0].Excerpt, "Hermeticity admission") {
		t.Fatalf("body-only hit lacks relevant narrative context: %+v", cands)
	}
	if utf8.RuneCountInString(cands[0].Excerpt) > candidateExcerptLimit {
		t.Fatalf("excerpt is longer than %d runes: %d", candidateExcerptLimit, utf8.RuneCountInString(cands[0].Excerpt))
	}
	payload, err := json.Marshal(JudgeInput{Draft: draft, Candidates: cands})
	if err != nil {
		t.Fatal(err)
	}
	got := string(payload)
	if !strings.Contains(got, `"excerpt":"`) || !strings.Contains(got, "Hermeticity admission") {
		t.Fatalf("judge input omits the body excerpt: %s", got)
	}
	if strings.Contains(got, "PRIVATE-PREFIX") || strings.Contains(got, "PRIVATE-SUFFIX") || strings.Contains(got, `"body"`) {
		t.Fatalf("judge input leaked the full Document body: %s", got)
	}
}

func TestInactiveInFlightDocumentsStayOutOfDefaultSearch(t *testing.T) {
	doc := &Doc{ID: "Document-9", Kind: "Document", Subject: "prd/export", Family: "prd", Status: "superseded", InFlight: "feature/export", Statement: "Old export", Body: "quokka-only obsolete narrative"}
	ix := Build([]*Doc{doc})
	if got := ix.Search(Query{Text: "quokka obsolete"}); len(got) != 0 {
		t.Fatalf("superseded in-flight document leaked into search: %+v", got)
	}
	if got := ix.Search(Query{Text: "quokka obsolete", IncludeInactive: true}); len(got) != 1 || got[0].Doc.ID != doc.ID {
		t.Fatalf("explicit inactive search should retain history: %+v", got)
	}
}

func TestParseVerdicts(t *testing.T) {
	cands := []Candidate{{Doc: Doc{ID: "Promise-2"}}}
	out, err := ParseVerdicts([]byte("Sure, here you go:\n{\"verdicts\":[{\"id\":\"Promise-2\",\"relation\":\"supersedes\",\"why\":\"same role\"}]}\nDone."), cands)
	if err != nil || len(out.Verdicts) != 1 || out.Verdicts[0].Relation != RelSupersedes {
		t.Fatalf("%+v %v", out, err)
	}
	cases := map[string]string{
		"no json here": "no JSON object",
		"{not json}":   "parse output",
		`{"verdicts":[{"id":"Promise-9","relation":"supersedes"}]}`: "not a candidate",
		`{"verdicts":[{"id":"Promise-2","relation":"likes"}]}`:      "relation",
		`{"verdicts":[]}`: "no verdict for Promise-2; a judge answers for every candidate",
		`{"verdicts":[{"id":"Promise-2","relation":"supersedes"},{"id":"Promise-2","relation":"unrelated"}]}`: "judged twice",
	}
	for in, want := range cases {
		if _, err := ParseVerdicts([]byte(in), cands); err == nil || !errors.Is(err, ErrJudge) || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: %v", in, err)
		}
	}
}

// TestMain lets this test binary double as a judge command.
func TestMain(m *testing.M) {
	switch os.Getenv("MEMORY_TEST_JUDGE") {
	case "":
		os.Exit(m.Run())
	case "ok":
		var in JudgeInput
		if err := readJSON(os.Stdin, &in); err != nil {
			os.Exit(3)
		}
		if in.Instructions == "" || len(in.Relations) == 0 {
			os.Exit(4)
		}
		out := JudgeOutput{}
		for _, c := range in.Candidates {
			out.Verdicts = append(out.Verdicts, Verdict{ID: c.ID, Relation: RelCompatible, Why: "judge says so"})
		}
		writeJSON(os.Stdout, out)
		os.Exit(0)
	case "fail":
		os.Stderr.WriteString("judge exploded")
		os.Exit(1)
	}
}

func TestRunJudge(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	in := JudgeInput{Draft: Draft{Statement: "x"}, Candidates: []Candidate{{Doc: Doc{ID: "Promise-2"}}}}
	t.Setenv("MEMORY_TEST_JUDGE", "ok")
	out, err := RunJudge(context.Background(), exe+" -test.run=XXX", in)
	if err != nil || len(out.Verdicts) != 1 || out.Verdicts[0].Relation != RelCompatible || out.Verdicts[0].Why != "judge says so" {
		t.Fatalf("%+v %v", out, err)
	}
	t.Setenv("MEMORY_TEST_JUDGE", "fail")
	if _, err := RunJudge(context.Background(), exe+" -test.run=XXX", in); err == nil || !strings.Contains(err.Error(), "judge exploded") {
		t.Fatalf("failure must carry stderr: %v", err)
	}
	if _, err := RunJudge(context.Background(), "   ", in); err == nil {
		t.Fatal("empty command")
	}
	if _, err := RunJudge(context.Background(), "/definitely/not/a/command", in); err == nil {
		t.Fatal("missing command")
	}
}

func readJSON(r io.Reader, v any) error { return json.NewDecoder(r).Decode(v) }

func writeJSON(w io.Writer, v any) {
	_ = json.NewEncoder(w).Encode(v) // a broken pipe here fails the caller's parse, which is the signal
}

func ids(hits []Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Doc.ID)
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
