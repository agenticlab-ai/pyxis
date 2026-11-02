// Package record defines the one unit the memory stores: an immutable, typed,
// schema-versioned record kept as a single JSON file under memory/<subject>/.
//
// A record is never edited once it has reached the default branch. Change is
// expressed by a new record that names what it supersedes; status (active,
// superseded, revoked) is a projection computed by package resolve, never a
// stored field. The file is the only copy: every view is rendered from it.
package record

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Schema is the record schema version written into every file. Evolution is
// additive only; a reader refuses a schema it does not know.
const Schema = 1

// Kinds of record. Claims (the first six) carry a family, the exclusivity key
// inside a subject: at most one active claim per subject and family.
const (
	Need        = "Need"        // a goal, a metric, an anchor (what a design or a code package is for)
	Promise     = "Promise"     // a requirement the product commits to (FR, NFR, UB, NR, EVT, ABUSE rows)
	Expectation = "Expectation" // a done or evaluation criterion (DOD, EVAL, KPI rows)
	Decision    = "Decision"    // a design or product decision, with what it serves
	Assumption  = "Assumption"  // something taken as true until it expires or is answered
	Policy      = "Policy"      // a governance rule (principles)
	Learning    = "Learning"    // why a change happened; the lesson, with what it was learned from
	Plan        = "Plan"        // scaffolding: a task or a plan delta; never truth
	Question    = "Question"    // one question wording; a correction supersedes it
	Answer      = "Answer"      // answered once; relation answers → Question
	Revocation  = "Revocation"  // withdraws a record; relation revokes → target
	Document    = "Document"    // the narrative of a subject (a PRD, a design): markdown that embeds claims by reference, never restates them
)

// Relation types. Targets are record ids.
const (
	Supersedes = "supersedes" // history: this record replaces the target in its family
	Serves     = "serves"     // a Decision serves a Promise / Expectation / Need
	DependsOn  = "depends_on" // this record rests on the target
	About      = "about"      // this record is about the target (a code anchor, a subject anchor)
	Refines    = "refines"    // narrows or details the target without replacing it
	Conflicts  = "conflicts"  // declared incompatibility, kept for the record
	Answers    = "answers"    // Answer → Question
	Revokes    = "revokes"    // Revocation → any record
)

// Consequence tiers, as declared by an author or computed by the packet.
const (
	Low    = "low"
	Medium = "medium"
	High   = "high"
)

var (
	kinds         = []string{Need, Promise, Expectation, Decision, Assumption, Policy, Document, Learning, Plan, Question, Answer, Revocation}
	claimKinds    = map[string]bool{Need: true, Promise: true, Expectation: true, Decision: true, Assumption: true, Policy: true, Document: true}
	relationTypes = map[string]bool{Supersedes: true, Serves: true, DependsOn: true, About: true, Refines: true, Conflicts: true, Answers: true, Revokes: true}
	consequences  = map[string]bool{"": true, Low: true, Medium: true, High: true}

	subjectRE = regexp.MustCompile(`^[a-z][a-z0-9-]*/[a-z0-9][a-z0-9._-]*$`)
	familyRE  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]*$`)
	idRE      = regexp.MustCompile(`^([A-Z][a-z]+)-([0-9A-HJKMNP-TV-Z]{26})$`)
	tagRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9:/._-]*$`)
	// a marker sits on one line: the renderer expands line by line, so a marker
	// spanning lines would be recorded as an embed and never expanded
	embedRE  = regexp.MustCompile(`\{\{[ \t]*(?:([a-z][a-z0-9-]*/[a-z0-9][a-z0-9._-]*)#)?([A-Za-z][A-Za-z0-9._-]*)[ \t]*\}\}`)
	refRE    = regexp.MustCompile(`^[a-z][a-z0-9-]*/[a-z0-9][a-z0-9._-]*#[A-Za-z][A-Za-z0-9._-]*$`)
	sha256RE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ErrInvalid is wrapped by every validation failure.
var ErrInvalid = errors.New("invalid record")

// Relation is one typed edge to another record.
type Relation struct {
	Type   string `json:"type"`
	Target string `json:"target"`
}

// Provenance says where a record came from. Every field is optional.
type Provenance struct {
	Source   string    `json:"source,omitempty"` // the document, log, review or incident it was distilled from
	PR       int       `json:"pr,omitempty"`     // the pull request that carried it, once known
	Commit   string    `json:"commit,omitempty"`
	Note     string    `json:"note,omitempty"`
	Evidence *Evidence `json:"evidence,omitempty"`
}

// Record is the stored unit. Field order is the canonical file order.
type Record struct {
	Schema       int        `json:"schema"`
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Subject      string     `json:"subject"`
	Family       string     `json:"family,omitempty"`
	Statement    string     `json:"statement"`
	Intent       string     `json:"intent,omitempty"`
	Rationale    string     `json:"rationale,omitempty"`
	FitCriterion string     `json:"fit_criterion,omitempty"`
	LearnedFrom  []string   `json:"learned_from,omitempty"`
	Alternatives []string   `json:"alternatives_rejected,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
	Consequence  string     `json:"consequence,omitempty"`
	Expires      string     `json:"expires,omitempty"`
	Relations    []Relation `json:"relations,omitempty"`
	BodySHA256   string     `json:"body_sha256,omitempty"`  // Document: the hash of the sibling <id>.md body
	Embeds       []string   `json:"embeds,omitempty"`       // Document: the subject#family references its body embeds, sorted
	LiteralBody  bool       `json:"literal_body,omitempty"` // Document: preserve/import the body without interpreting {{…}} markers
	Issuer       string     `json:"issuer"`
	CreatedAt    string     `json:"created_at"`
	Provenance   Provenance `json:"provenance,omitzero"`
}

// Kinds lists every record kind in display order.
func Kinds() []string { return append([]string(nil), kinds...) }

// IsKind reports whether k names a record kind.
func IsKind(k string) bool {
	return slices.Contains(kinds, k)
}

// IsClaim reports whether records of kind k carry a family and are exclusive
// inside it.
func IsClaim(k string) bool { return claimKinds[k] }

// IsRelation reports whether t names a relation type.
func IsRelation(t string) bool { return relationTypes[t] }

// ValidSubject reports whether s has the form namespace/name.
func ValidSubject(s string) bool { return subjectRE.MatchString(s) }

// Namespace returns the part of a subject before the slash.
func Namespace(subject string) string {
	if before, _, ok := strings.Cut(subject, "/"); ok {
		return before
	}
	return subject
}

// KindOfID returns the kind encoded in a record id, or "" if the id is malformed.
func KindOfID(id string) string {
	m := idRE.FindStringSubmatch(id)
	if m == nil {
		return ""
	}
	return m[1]
}

// NewID mints an id for a kind at a moment: <Kind>-<ULID>. Ids sort by time.
func NewID(kind string, at time.Time, entropy [10]byte) string {
	return kind + "-" + ULID(at, entropy)
}

// IDBefore reports whether older's ULID chronology is strictly before
// newer's. Record kind prefixes are deliberately ignored: resolver order is
// carried by the ULID suffix across every kind. Malformed IDs are never
// ordered; callers validate records separately and therefore fail closed.
func IDBefore(older, newer string) bool {
	olderKind := KindOfID(older)
	newerKind := KindOfID(newer)
	if olderKind == "" || newerKind == "" {
		return false
	}
	return older[len(olderKind)+1:] < newer[len(newerKind)+1:]
}

// Path is the file a record lives in, relative to the memory root:
// <subject>/<id>.json.
func (r *Record) Path() string { return path.Join(r.Subject, r.ID+".json") }

// BodyPath is the sibling markdown file a Document's narrative lives in:
// <subject>/<id>.md.
func (r *Record) BodyPath() string { return path.Join(r.Subject, r.ID+".md") }

// NormalizeBody returns the canonical bytes stored and hashed for a Document
// body. Git may check text out as CRLF on one machine and LF on another; line
// endings therefore cannot be part of a body record's identity.
func NormalizeBody(body []byte) []byte {
	if !bytes.Contains(body, []byte("\r\n")) {
		return body
	}
	return bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
}

// HashBody is the digest a Document envelope carries for its canonical body.
func HashBody(body []byte) string {
	return hashBodyBytes(NormalizeBody(body))
}

// BodyHashMatches reports whether body has the hash carried by a Document
// envelope. Besides the canonical hash it accepts the raw hash used before
// bodies were normalized. The synthesized CRLF form keeps an old body that
// was originally hashed with CRLF readable after Git has normalized its blob
// to LF; every new write uses only the canonical hash.
func BodyHashMatches(body []byte, want string) bool {
	canonical := NormalizeBody(body)
	if hashBodyBytes(canonical) == want {
		return true
	}
	if !bytes.Equal(canonical, body) {
		return hashBodyBytes(body) == want
	}
	legacyCRLF := bytes.ReplaceAll(canonical, []byte("\n"), []byte("\r\n"))
	return hashBodyBytes(legacyCRLF) == want
}

func hashBodyBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// MaskMarkdownInert returns a same-length copy of body with fenced code,
// inline code spans and HTML comments replaced by spaces. Newlines and byte
// offsets are preserved. Markers and headings inside those regions are
// examples or hidden source, not document structure a Markdown reader sees.
func MaskMarkdownInert(body []byte) []byte {
	masked := append([]byte(nil), body...)
	fenceMark, fenceLen := byte(0), 0
	for at := 0; at < len(body); {
		if at == 0 || body[at-1] == '\n' {
			end := bytes.IndexByte(body[at:], '\n')
			if end < 0 {
				end = len(body)
			} else {
				end += at
			}
			line := body[at:end]
			if fenceMark != 0 {
				blankMarkdown(masked[at:end])
				if closesFence(line, fenceMark, fenceLen) {
					fenceMark, fenceLen = 0, 0
				}
				at = end
				if at < len(body) { // keep the newline, then begin at the next line
					at++
				}
				continue
			}
			if mark, n := opensFence(line); mark != 0 {
				fenceMark, fenceLen = mark, n
				blankMarkdown(masked[at:end])
				at = end
				if at < len(body) {
					at++
				}
				continue
			}
		}
		switch {
		case bytes.HasPrefix(body[at:], []byte("<!--")):
			end := bytes.Index(body[at+len("<!--"):], []byte("-->"))
			if end < 0 {
				blankMarkdown(masked[at:])
				return masked
			}
			end += at + len("<!--") + len("-->")
			blankMarkdown(masked[at:end])
			at = end
		case body[at] == '`' && !escapedMarkdown(body, at):
			end := at
			for end < len(body) && body[end] == '`' {
				end++
			}
			run := end - at
			close := closingCodeRun(body, end, run)
			if close < 0 { // an unmatched backtick run is literal text
				at = end
				continue
			}
			blankMarkdown(masked[at : close+run])
			at = close + run
		default:
			at++
		}
	}
	return masked
}

func blankMarkdown(b []byte) {
	for i := range b {
		if b[i] != '\n' {
			b[i] = ' '
		}
	}
}

func opensFence(line []byte) (byte, int) {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0
	}
	mark := line[i]
	j := i
	for j < len(line) && line[j] == mark {
		j++
	}
	if j-i < 3 || mark == '`' && bytes.IndexByte(line[j:], '`') >= 0 {
		return 0, 0
	}
	return mark, j - i
}

func closesFence(line []byte, mark byte, need int) bool {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i > 3 {
		return false
	}
	j := i
	for j < len(line) && line[j] == mark {
		j++
	}
	if j-i < need {
		return false
	}
	for ; j < len(line); j++ {
		if line[j] != ' ' && line[j] != '\t' && line[j] != '\r' {
			return false
		}
	}
	return true
}

func closingCodeRun(body []byte, from, want int) int {
	for i := from; i < len(body); {
		if body[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(body) && body[j] == '`' {
			j++
		}
		if j-i == want {
			return i
		}
		i = j
	}
	return -1
}

func escapedMarkdown(body []byte, at int) bool {
	n := 0
	for at > 0 && body[at-1] == '\\' {
		n++
		at--
	}
	return n%2 == 1
}

func embedLocations(body []byte) [][]int {
	return embedRE.FindAllSubmatchIndex(MaskMarkdownInert(body), -1)
}

// ParseEmbeds lists the claims a body embeds, as sorted unique
// subject#family references. A marker is {{subject#family}}, or {{family}}
// for a claim on the document's own subject.
func ParseEmbeds(body []byte, subject string) []string {
	seen := map[string]bool{}
	for _, loc := range embedLocations(body) {
		sub := subject
		if loc[2] >= 0 {
			sub = string(body[loc[2]:loc[3]])
		}
		seen[sub+"#"+string(body[loc[4]:loc[5]])] = true
	}
	out := make([]string, 0, len(seen))
	for ref := range seen {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// EmbedMarkers finds every marker in a body with its position, so a renderer
// can replace it in place. Each entry is (start, end, subject#family).
func EmbedMarkers(body []byte, subject string) [][3]string {
	var out [][3]string
	for _, loc := range embedLocations(body) {
		sub := subject
		if loc[2] >= 0 {
			sub = string(body[loc[2]:loc[3]])
		}
		out = append(out, [3]string{fmt.Sprint(loc[0]), fmt.Sprint(loc[1]), sub + "#" + string(body[loc[4]:loc[5]])})
	}
	return out
}

// ValidRef reports whether s has the form subject#family.
func ValidRef(s string) bool { return refRE.MatchString(s) }

// Rel returns the targets of every relation of type t, in file order.
func (r *Record) Rel(t string) []string {
	var out []string
	for _, rel := range r.Relations {
		if rel.Type == t {
			out = append(out, rel.Target)
		}
	}
	return out
}

// HasTag reports whether the record carries tag t.
func (r *Record) HasTag(t string) bool {
	return slices.Contains(r.Tags, t)
}

// Validate checks everything that can be checked on the record alone. Cross-
// record checks (targets exist, families are exclusive) belong to the store
// and the resolver.
func (r *Record) Validate() error {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	if r.Schema != Schema {
		add("schema %d is not %d", r.Schema, Schema)
	}
	if !IsKind(r.Kind) {
		add("kind %q is not one of %s", r.Kind, strings.Join(kinds, ", "))
	}
	if KindOfID(r.ID) != r.Kind {
		add("id %q does not encode kind %q as <Kind>-<ULID>", r.ID, r.Kind)
	}
	if !ValidSubject(r.Subject) {
		add("subject %q is not namespace/name (lowercase, digits, dots, dashes)", r.Subject)
	}
	if IsClaim(r.Kind) {
		if r.Family == "" {
			add("%s records carry a family (the exclusivity key inside the subject)", r.Kind)
		}
	}
	if r.Family != "" && !familyRE.MatchString(r.Family) {
		add("family %q is not a plain identifier", r.Family)
	}
	if strings.TrimSpace(r.Statement) == "" {
		add("statement is empty")
	}
	if !consequences[r.Consequence] {
		add("consequence %q is not low, medium or high", r.Consequence)
	}
	if r.Issuer == "" {
		add("issuer is empty")
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
		add("created_at %q is not RFC 3339", r.CreatedAt)
	}
	if r.Expires != "" {
		if _, err := time.Parse(time.RFC3339, r.Expires); err != nil {
			add("expires %q is not RFC 3339", r.Expires)
		}
	}
	for _, t := range r.Tags {
		if !tagRE.MatchString(t) {
			add("tag %q is not lowercase [a-z0-9:/._-]", t)
		}
	}
	seen := map[Relation]bool{}
	for _, rel := range r.Relations {
		if !IsRelation(rel.Type) {
			add("relation type %q is unknown", rel.Type)
		}
		if KindOfID(rel.Target) == "" {
			add("relation %s target %q is not a record id", rel.Type, rel.Target)
		}
		if rel.Target == r.ID {
			add("relation %s targets the record itself", rel.Type)
		}
		if seen[rel] {
			add("relation %s → %s is repeated", rel.Type, rel.Target)
		}
		seen[rel] = true
		if err := relationAllowed(r.Kind, rel); err != nil {
			add("%v", err)
		}
	}
	if r.Kind == Document {
		if !sha256RE.MatchString(r.BodySHA256) {
			add("a Document carries body_sha256, the hex digest of its sibling .md body")
		}
		if r.LiteralBody && len(r.Embeds) != 0 {
			add("a literal Document carries no embeds")
		}
		for _, e := range r.Embeds {
			if !ValidRef(e) {
				add("embed %q is not subject#family", e)
			}
		}
		if !sort.StringsAreSorted(r.Embeds) {
			add("embeds are sorted")
		}
	} else if r.BodySHA256 != "" || len(r.Embeds) != 0 || r.LiteralBody {
		add("only a Document carries body_sha256, embeds and literal_body")
	}
	switch r.Kind {
	case Answer:
		if len(r.Rel(Answers)) != 1 {
			add("an Answer carries exactly one answers relation")
		}
	case Revocation:
		if len(r.Rel(Revokes)) != 1 {
			add("a Revocation carries exactly one revokes relation")
		}
	case Assumption:
		if r.Expires == "" {
			add("an Assumption carries expires")
		}
	}
	if len(problems) == 0 {
		if err := r.ValidateEvidence(); err != nil {
			return err
		}
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("%w %s: %s", ErrInvalid, r.ID, strings.Join(problems, "; "))
}

func relationAllowed(kind string, rel Relation) error {
	tk := KindOfID(rel.Target)
	switch rel.Type {
	case Supersedes:
		if !IsClaim(kind) && kind != Plan && kind != Learning && kind != Answer && kind != Question {
			return fmt.Errorf("a %s does not supersede", kind)
		}
		if tk != kind {
			return fmt.Errorf("supersedes target %s is a %s, not a %s", rel.Target, tk, kind)
		}
	case Serves:
		if kind != Decision && kind != Plan {
			return fmt.Errorf("only a Decision or a Plan serves; a %s does not", kind)
		}
		if tk != Promise && tk != Expectation && tk != Need {
			return fmt.Errorf("serves target %s is a %s; only a Promise, an Expectation or a Need is served", rel.Target, tk)
		}
	case Answers:
		if kind != Answer || tk != Question {
			return fmt.Errorf("answers is only Answer → Question")
		}
	case Revokes:
		if kind != Revocation {
			return fmt.Errorf("only a Revocation revokes")
		}
	}
	return nil
}

// Marshal renders the canonical file bytes: two-space indented JSON with the
// struct field order and a trailing newline. Two records with the same content
// produce the same bytes, which is what immutability checks compare.
func Marshal(r *Record) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", r.ID, err)
	}
	return append(b, '\n'), nil
}

// Unmarshal parses one record file. Unknown fields are errors: the schema is
// closed so that a typo never silently becomes a lost field.
func Unmarshal(b []byte) (*Record, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var r Record
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: trailing data after the record", ErrInvalid)
	}
	return &r, nil
}

// Tokens estimates the token count of text the way the packet and the pack
// budget count it: about four characters per token.
func Tokens(text string) int { return (len(text) + 3) / 4 }
