package record

import (
	"bytes"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

func fixed(kind string, n byte) string {
	var e [10]byte
	e[9] = n
	return NewID(kind, t0, e)
}

func valid() Record {
	return Record{Schema: Schema, ID: fixed(Promise, 1), Kind: Promise, Subject: "prd/implement-now", Family: "FR-1",
		Statement: "When invoked, the executor shall build the ready frontier.", Intent: "one frontier", Issuer: "m@example.com",
		CreatedAt: t0.Format(time.RFC3339)}
}

func TestValidateAcceptsAWellFormedClaim(t *testing.T) {
	r := valid()
	if err := r.Validate(); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
}

func TestValidateAcceptsQuestionSupersession(t *testing.T) {
	r := valid()
	r.Kind, r.ID, r.Family = Question, fixed(Question, 2), "Q-1"
	r.Relations = []Relation{{Supersedes, fixed(Question, 1)}}
	if err := r.Validate(); err != nil {
		t.Fatalf("question supersession rejected: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Record)
		want string
	}{
		{"unknown schema", func(r *Record) { r.Schema = 2 }, "schema 2 is not 1"},
		{"unknown kind", func(r *Record) { r.Kind = "Thing" }, `kind "Thing"`},
		{"id kind mismatch", func(r *Record) { r.ID = fixed(Decision, 1) }, "does not encode kind"},
		{"bad subject", func(r *Record) { r.Subject = "PRD" }, "namespace/name"},
		{"claim without family", func(r *Record) { r.Family = "" }, "carry a family"},
		{"bad family", func(r *Record) { r.Family = "FR 1" }, "plain identifier"},
		{"empty statement", func(r *Record) { r.Statement = "  " }, "statement is empty"},
		{"bad consequence", func(r *Record) { r.Consequence = "huge" }, "consequence"},
		{"no issuer", func(r *Record) { r.Issuer = "" }, "issuer is empty"},
		{"bad time", func(r *Record) { r.CreatedAt = "yesterday" }, "created_at"},
		{"bad expires", func(r *Record) { r.Expires = "soon" }, "expires"},
		{"bad tag", func(r *Record) { r.Tags = []string{"Code:X"} }, "tag"},
		{"unknown relation", func(r *Record) { r.Relations = []Relation{{"likes", fixed(Promise, 2)}} }, "relation type"},
		{"bad target", func(r *Record) { r.Relations = []Relation{{Supersedes, "FR-1"}} }, "not a record id"},
		{"self target", func(r *Record) { r.Relations = []Relation{{Supersedes, r.ID}} }, "targets the record itself"},
		{"repeated relation", func(r *Record) {
			r.Relations = []Relation{{Supersedes, fixed(Promise, 2)}, {Supersedes, fixed(Promise, 2)}}
		}, "repeated"},
		{"supersedes across kinds", func(r *Record) { r.Relations = []Relation{{Supersedes, fixed(Decision, 2)}} }, "not a Promise"},
		{"promise serves", func(r *Record) { r.Relations = []Relation{{Serves, fixed(Promise, 2)}} }, "only a Decision or a Plan serves"},
		{"decision serves a decision", func(r *Record) {
			r.Kind, r.ID = Decision, fixed(Decision, 1)
			r.Relations = []Relation{{Serves, fixed(Decision, 2)}}
		}, "only a Promise, an Expectation or a Need is served"},
		{"answer without question", func(r *Record) { r.Kind, r.ID, r.Family = Answer, fixed(Answer, 1), "" }, "exactly one answers"},
		{"answers from a promise", func(r *Record) { r.Relations = []Relation{{Answers, fixed(Question, 2)}} }, "only Answer → Question"},
		{"revocation without target", func(r *Record) { r.Kind, r.ID, r.Family = Revocation, fixed(Revocation, 1), "" }, "exactly one revokes"},
		{"promise revokes", func(r *Record) { r.Relations = []Relation{{Revokes, fixed(Promise, 2)}} }, "only a Revocation revokes"},
		{"assumption without expiry", func(r *Record) { r.Kind, r.ID = Assumption, fixed(Assumption, 1) }, "carries expires"},
		{"revocation does not supersede", func(r *Record) {
			r.Kind, r.ID, r.Family = Revocation, fixed(Revocation, 1), ""
			r.Relations = []Relation{{Supersedes, fixed(Revocation, 2)}}
		}, "does not supersede"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := valid()
			c.edit(&r)
			err := r.Validate()
			if err == nil {
				t.Fatalf("expected a validation error mentioning %q", c.want)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("error does not wrap ErrInvalid: %v", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not mention %q", err, c.want)
			}
		})
	}
}

func TestValidateAcceptsEveryKindInItsShape(t *testing.T) {
	q := fixed(Question, 3)
	cases := []Record{
		{Kind: Need, Family: "anchor"},
		{Kind: Expectation, Family: "DOD-1"},
		{Kind: Decision, Family: "D-1", Relations: []Relation{{Serves, fixed(Promise, 2)}, {About, fixed(Need, 2)}, {DependsOn, fixed(Need, 3)}}},
		{Kind: Assumption, Family: "A-1", Expires: t0.Add(time.Hour).Format(time.RFC3339)},
		{Kind: Policy, Family: "AP-1"},
		{Kind: Learning, LearnedFrom: []string{"incident:I2"}},
		{Kind: Plan, Relations: []Relation{{Serves, fixed(Need, 2)}}},
		{Kind: Question},
		{Kind: Answer, Relations: []Relation{{Answers, q}}},
		{Kind: Answer, Relations: []Relation{{Answers, q}, {Supersedes, fixed(Answer, 2)}}},
		{Kind: Revocation, Relations: []Relation{{Revokes, fixed(Decision, 9)}}},
		{Kind: Document, Family: "prd", BodySHA256: HashBody([]byte("# x\n{{FR-1}}")), Embeds: []string{"prd/implement-now#FR-1"}},
	}
	for _, c := range cases {
		t.Run(c.Kind, func(t *testing.T) {
			r := valid()
			r.Kind, r.ID, r.Family, r.Relations, r.Expires, r.LearnedFrom = c.Kind, fixed(c.Kind, 1), c.Family, c.Relations, c.Expires, c.LearnedFrom
			r.BodySHA256, r.Embeds = c.BodySHA256, c.Embeds
			if err := r.Validate(); err != nil {
				t.Fatalf("%s rejected: %v", c.Kind, err)
			}
		})
	}
}

func TestDocuments(t *testing.T) {
	body := []byte("# Stacked PRs\n\nThe executor opens {{FR-1}} and {{ prd/other#DOD-2 }}; see {{FR-1}} again and {{design/x#D-1}}.\n")
	if got := strings.Join(ParseEmbeds(body, "prd/x"), ","); got != "design/x#D-1,prd/other#DOD-2,prd/x#FR-1" {
		t.Fatalf("embeds %q", got)
	}
	if got := ParseEmbeds([]byte("{{\nFR-1\n}} and {{ FR-2\t}}"), "prd/x"); strings.Join(got, ",") != "prd/x#FR-2" {
		t.Fatalf("a marker spanning lines is not an embed: %v", got)
	}
	inert := []byte("visible {{FR-1}} `{{FR-2}}` <!-- {{FR-3}} -->\n<!--\n{{FR-4}}\n-->\n````markdown\n```\n{{FR-5}}\n```\n````\n~~~sh\n{{FR-6}}\n~~~\n`<!--` then {{FR-7}}\n<!-- ` --> then {{FR-8}}\nunmatched ` then {{FR-9}}\n")
	if got := strings.Join(ParseEmbeds(inert, "prd/x"), ","); got != "prd/x#FR-1,prd/x#FR-7,prd/x#FR-8,prd/x#FR-9" {
		t.Fatalf("only markers in active Markdown are embeds: %v", got)
	}
	masked := MaskMarkdownInert(inert)
	if len(masked) != len(inert) || bytes.Count(masked, []byte("\n")) != bytes.Count(inert, []byte("\n")) {
		t.Fatal("mask must preserve offsets and lines")
	}
	markers := EmbedMarkers(body, "prd/x")
	if len(markers) != 4 || markers[0][2] != "prd/x#FR-1" || markers[1][2] != "prd/other#DOD-2" || markers[3][2] != "design/x#D-1" {
		t.Fatalf("markers %v", markers)
	}
	if len(HashBody(body)) != 64 || HashBody(body) == HashBody([]byte("other")) {
		t.Fatal("HashBody")
	}
	lf := []byte("# Title\n\nOne\nTwo\n")
	crlf := []byte("# Title\r\n\r\nOne\r\nTwo\r\n")
	if got := string(NormalizeBody(crlf)); got != string(lf) {
		t.Fatalf("normalized body %q", got)
	}
	if HashBody(lf) != HashBody(crlf) {
		t.Fatal("body hashes must not depend on Git's checkout line endings")
	}
	legacyCRLFHash := hashBodyBytes(crlf)
	if !BodyHashMatches(lf, legacyCRLFHash) || !BodyHashMatches(crlf, legacyCRLFHash) {
		t.Fatal("a pre-normalization CRLF hash must remain readable from either checkout form")
	}
	if BodyHashMatches([]byte("changed\n"), legacyCRLFHash) {
		t.Fatal("compatibility must not admit changed content")
	}
	if !ValidRef("prd/x#FR-1") || ValidRef("prd/x") || ValidRef("FR-1") {
		t.Fatal("ValidRef")
	}
	r := valid()
	r.Kind, r.ID, r.Family = Document, fixed(Document, 1), "prd"
	if r.BodyPath() != "prd/implement-now/"+r.ID+".md" {
		t.Fatalf("body path %q", r.BodyPath())
	}
	cases := []struct {
		name string
		edit func(*Record)
		want string
	}{
		{"no hash", func(r *Record) {}, "body_sha256"},
		{"bad embed", func(r *Record) { r.BodySHA256 = HashBody(nil); r.Embeds = []string{"FR-1"} }, "not subject#family"},
		{"unsorted embeds", func(r *Record) { r.BodySHA256 = HashBody(nil); r.Embeds = []string{"prd/x#FR-2", "prd/x#FR-1"} }, "sorted"},
		{"literal document with embeds", func(r *Record) {
			r.BodySHA256 = HashBody(nil)
			r.LiteralBody = true
			r.Embeds = []string{"prd/x#FR-1"}
		}, "literal Document carries no embeds"},
		{"hash on a promise", func(r *Record) {
			r.Kind, r.ID, r.Family = Promise, fixed(Promise, 2), "FR-1"
			r.BodySHA256 = HashBody(nil)
		}, "only a Document"},
		{"literal mode on a promise", func(r *Record) {
			r.Kind, r.ID, r.Family = Promise, fixed(Promise, 2), "FR-1"
			r.LiteralBody = true
		}, "only a Document"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := r
			c.edit(&d)
			if err := d.Validate(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
	r.BodySHA256 = HashBody(body)
	r.Embeds = ParseEmbeds(body, "prd/x")
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Embeds = nil
	r.LiteralBody = true
	if err := r.Validate(); err != nil {
		t.Fatalf("literal document rejected: %v", err)
	}
}

func TestMarshalIsCanonicalAndClosed(t *testing.T) {
	r := valid()
	r.Provenance = Provenance{Source: "docs/prds/implement-now/000-base.md", PR: 336}
	b, err := Marshal(&r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "}\n") || !strings.HasPrefix(string(b), "{\n  \"schema\": 1,\n  \"id\": ") {
		t.Fatalf("not the canonical layout:\n%s", b)
	}
	back, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(b) {
		t.Fatalf("round trip changed the bytes:\n%s\n---\n%s", b, again)
	}
	if _, err := Unmarshal([]byte(`{"schema":1,"id":"x","colour":"red"}`)); err == nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown field accepted: %v", err)
	}
	if _, err := Unmarshal(append(b, []byte("{}")...)); err == nil {
		t.Fatal("trailing data accepted")
	}
	empty := Record{}
	if b, _ := Marshal(&empty); strings.Contains(string(b), "provenance") {
		t.Fatalf("empty provenance should be omitted:\n%s", b)
	}
}

func TestULIDShapeAndOrder(t *testing.T) {
	var e [10]byte
	a := ULID(t0, e)
	b := ULID(t0.Add(time.Millisecond), e)
	if len(a) != 26 || len(b) != 26 {
		t.Fatalf("length: %q %q", a, b)
	}
	if a >= b {
		t.Fatalf("later id must sort after: %q vs %q", a, b)
	}
	for _, c := range a + b {
		if !strings.ContainsRune(crockford, c) {
			t.Fatalf("non-crockford character %q in %q", c, a)
		}
	}
	e2 := e
	e2[9] = 1
	if ULID(t0, e) == ULID(t0, e2) {
		t.Fatal("entropy must change the id")
	}
	ids := []string{ULID(t0.Add(time.Hour), e), ULID(t0, e), ULID(t0.Add(time.Minute), e)}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	if sorted[0] != ids[1] || sorted[2] != ids[0] {
		t.Fatalf("lexicographic order is not time order: %v", sorted)
	}
	if _, err := Entropy(); err != nil {
		t.Fatal(err)
	}
	if got := KindOfID(NewID(Decision, t0, e)); got != Decision {
		t.Fatalf("KindOfID = %q", got)
	}
	if !IDBefore(NewID(Promise, t0, e), NewID(Decision, t0.Add(time.Millisecond), e)) {
		t.Fatal("IDBefore must compare ULID chronology across kind prefixes")
	}
	if IDBefore(NewID(Promise, t0, e), NewID(Decision, t0, e)) || IDBefore("bad", NewID(Decision, t0, e)) {
		t.Fatal("equal chronology and malformed ids must not compare before")
	}
	if KindOfID("Promise-short") != "" || KindOfID("promise-"+a) != "" {
		t.Fatal("malformed ids must not decode")
	}
}

func TestHelpers(t *testing.T) {
	r := valid()
	r.Tags = []string{"code:internal/executor"}
	r.Relations = []Relation{{Supersedes, fixed(Promise, 2)}, {Refines, fixed(Promise, 3)}}
	if r.Path() != "prd/implement-now/"+r.ID+".json" {
		t.Fatalf("path %q", r.Path())
	}
	if got := r.Rel(Supersedes); len(got) != 1 || got[0] != fixed(Promise, 2) {
		t.Fatalf("Rel = %v", got)
	}
	if !r.HasTag("code:internal/executor") || r.HasTag("x") {
		t.Fatal("HasTag")
	}
	if Namespace("prd/implement-now") != "prd" || Namespace("odd") != "odd" {
		t.Fatal("Namespace")
	}
	if !IsClaim(Promise) || IsClaim(Learning) || !IsRelation(Serves) || IsRelation("likes") || !IsKind(Plan) || IsKind("Plane") {
		t.Fatal("predicates")
	}
	if len(Kinds()) != 12 {
		t.Fatalf("kinds: %v", Kinds())
	}
	if Tokens("") != 0 || Tokens("abcd") != 1 || Tokens("abcde") != 2 {
		t.Fatal("Tokens")
	}
	if !ValidSubject("code/internal.executor") || ValidSubject("a/b/c") || ValidSubject("/x") {
		t.Fatal("ValidSubject")
	}
}
