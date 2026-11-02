// Package store reads and writes the memory's files: one JSON record per file
// under memory/<subject>/<id>.json, plus memory/policy.json.
//
// A Set is everything one tree holds — the working directory, or a git ref
// through a Source adapter — parsed and cross-checked once, so every other
// package works on records rather than files. The store never edits a file:
// records are written once and read forever.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

const (
	// Dir is the memory root, relative to the repository root.
	Dir = "memory"
	// PolicyFile is the policy document inside the memory root.
	PolicyFile = "policy.json"
)

// ErrExists is returned when a write would overwrite a record file.
var ErrExists = errors.New("record file already exists; records are immutable")

// Source provides the files of one memory tree, keyed by path relative to the
// memory root ("prd/implement-now/Promise-….json", "policy.json"). Only
// regular files are content; anything else under the root (a symlink, a
// submodule) is returned as irregular with the reason, so the bytes a check
// reads are the bytes git stores and never what a link points at.
type Source interface {
	Files(ctx context.Context) (files map[string][]byte, irregular map[string]string, err error)
}

// DirSource reads the memory root from the working directory.
type DirSource struct{ Root string }

// Files walks the directory without following links. A missing root is an
// empty memory, not an error.
func (d DirSource) Files(ctx context.Context) (map[string][]byte, map[string]string, error) {
	out, irregular := map[string][]byte{}, map[string]string{}
	if info, err := os.Lstat(d.Root); err == nil && !info.IsDir() {
		return nil, nil, fmt.Errorf("memory root %s is not a directory", d.Root)
	}
	err := filepath.WalkDir(d.Root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(d.Root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch {
		case p == d.Root:
			return nil
		case e.Type()&fs.ModeSymlink != 0:
			irregular[rel] = "a symlink; the memory holds regular files only"
			return nil
		case e.IsDir():
			return nil
		case !e.Type().IsRegular():
			irregular[rel] = "not a regular file"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return out, irregular, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read memory dir %s: %w", d.Root, err)
	}
	return out, irregular, nil
}

// MapSource serves files from memory; tests and the packet use it to compose
// trees.
type MapSource map[string][]byte

// Files returns the map itself.
func (m MapSource) Files(context.Context) (map[string][]byte, map[string]string, error) {
	return m, nil, nil
}

// Edge is one relation seen from its target.
type Edge struct {
	Type string
	From string
}

// Problem is a file the set could not admit, with why.
type Problem struct {
	Path string
	Err  error
}

// Dangling is a relation whose target is not in the set.
type Dangling struct {
	From   string
	Type   string
	Target string
}

// DocumentIssue is a refused Document file associated with the subject and
// slot whose projection it could change. An empty Slot means the broken file
// could belong to any document slot on the subject.
type DocumentIssue struct {
	Subject string
	Slot    string
	ID      string
	Problem Problem
}

// Set is one parsed memory tree.
type Set struct {
	Records        map[string]*record.Record   // by id
	Bodies         map[string][]byte           // Document id → its markdown body
	Bytes          map[string][]byte           // by path: canonical LF bytes used for Git-tree comparisons
	RawBytes       map[string][]byte           // exact input bytes; evidence never normalizes record envelopes
	Paths          map[string]string           // id → path
	BySubject      map[string][]*record.Record // subject → records in id (time) order
	To             map[string][]Edge           // target id → relations pointing at it
	Policy         []byte                      // policy.json, or nil
	Problems       []Problem                   // files refused, in path order
	DocumentIssues []DocumentIssue             // refused Document files, for fail-closed document views
	Dangling       []Dangling                  // relations whose target is absent, in from/type/target order
}

// Load parses every file of a source. Malformed files and dangling targets do
// not fail the load: they are reported on the set so a check can list them all.
func Load(ctx context.Context, src Source) (*Set, error) {
	files, irregular, err := src.Files(ctx)
	if err != nil {
		return nil, err
	}
	loadedBytes := make(map[string][]byte, len(files))
	for p, b := range files {
		loadedBytes[p] = record.NormalizeBody(b)
	}
	s := &Set{Records: map[string]*record.Record{}, Bodies: map[string][]byte{}, Bytes: loadedBytes, Paths: map[string]string{}, BySubject: map[string][]*record.Record{}, To: map[string][]Edge{}}
	s.RawBytes = files
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	bodies := map[string]bool{} // .md paths claimed by a Document envelope
	for _, p := range paths {
		if p == PolicyFile {
			s.Policy = s.Bytes[p]
			continue
		}
		if strings.HasSuffix(p, ".md") {
			continue // paired with its envelope below, or reported as an orphan after
		}
		r, err := parseAt(p, files[p])
		if err != nil {
			pr := Problem{Path: p, Err: err}
			s.Problems = append(s.Problems, pr)
			s.noteDocumentIssue(p, files[p], nil, pr)
			continue
		}
		if _, dup := s.Records[r.ID]; dup {
			pr := Problem{Path: p, Err: fmt.Errorf("%w: id %s is already used by %s", record.ErrInvalid, r.ID, s.Paths[r.ID])}
			s.Problems = append(s.Problems, pr)
			s.noteDocumentIssue(p, files[p], r, pr)
			continue
		}
		if r.Kind == record.Document {
			body, ok := files[r.BodyPath()]
			if !ok {
				pr := Problem{Path: p, Err: fmt.Errorf("%w: Document %s has no body at %s", record.ErrInvalid, r.ID, r.BodyPath())}
				s.Problems = append(s.Problems, pr)
				s.noteDocumentIssue(p, files[p], r, pr)
				continue
			}
			bodies[r.BodyPath()] = true
			if !documentBodyHashMatches(body, r.BodySHA256, r.LiteralBody) {
				pr := Problem{Path: r.BodyPath(), Err: fmt.Errorf("%w: the body of %s does not match body_sha256; bodies are immutable, revise the document instead", record.ErrInvalid, r.ID)}
				s.Problems = append(s.Problems, pr)
				s.noteDocumentIssue(p, files[p], r, pr)
				continue
			}
			if !r.LiteralBody {
				body = record.NormalizeBody(body)
			}
			var want []string
			if !r.LiteralBody {
				want = record.ParseEmbeds(body, r.Subject)
			}
			if strings.Join(want, ",") != strings.Join(r.Embeds, ",") {
				pr := Problem{Path: p, Err: fmt.Errorf("%w: the embeds of %s do not match its body (body: %s; envelope: %s)", record.ErrInvalid, r.ID,
					strings.Join(want, ", "), strings.Join(r.Embeds, ", "))}
				s.Problems = append(s.Problems, pr)
				s.noteDocumentIssue(p, files[p], r, pr)
				continue
			}
			s.Bytes[r.BodyPath()] = body
			s.Bodies[r.ID] = body
		}
		s.Records[r.ID] = r
		s.Paths[r.ID] = p
		s.BySubject[r.Subject] = append(s.BySubject[r.Subject], r)
	}
	for p, why := range irregular {
		pr := Problem{Path: p, Err: fmt.Errorf("%w: %s", record.ErrInvalid, why)}
		s.Problems = append(s.Problems, pr)
		s.noteDocumentIssue(p, nil, nil, pr)
	}
	for _, p := range paths {
		if strings.HasSuffix(p, ".md") && !bodies[p] {
			pr := Problem{Path: p, Err: fmt.Errorf("%w: a body with no Document envelope", record.ErrInvalid)}
			s.Problems = append(s.Problems, pr)
			s.noteDocumentIssue(p, nil, nil, pr)
		}
	}
	sort.Slice(s.Problems, func(i, j int) bool { return s.Problems[i].Path < s.Problems[j].Path })
	for _, sub := range s.Subjects() {
		rs := s.BySubject[sub]
		sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
	}
	ids := s.IDs()
	for _, id := range ids {
		r := s.Records[id]
		for _, rel := range r.Relations {
			if _, ok := s.Records[rel.Target]; !ok {
				s.Dangling = append(s.Dangling, Dangling{From: r.ID, Type: rel.Type, Target: rel.Target})
				continue
			}
			s.To[rel.Target] = append(s.To[rel.Target], Edge{Type: rel.Type, From: r.ID})
		}
	}
	return s, nil
}

// DocumentProblems returns the invalid files that make one document slot's
// current head or history unknowable. An empty slot selects every Document
// issue on the subject for overview views. Problems on other subjects do not
// prevent the requested document from rendering.
func (s *Set) DocumentProblems(subject, slot string) []Problem {
	var out []Problem
	for _, issue := range s.DocumentIssues {
		if issue.Subject == subject && (slot == "" || issue.Slot == "" || issue.Slot == slot) {
			out = append(out, issue.Problem)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Err.Error() < out[j].Err.Error()
	})
	return out
}

func (s *Set) noteDocumentIssue(p string, b []byte, r *record.Record, problem Problem) {
	if r != nil && r.Kind == record.Document {
		s.DocumentIssues = append(s.DocumentIssues, DocumentIssue{Subject: r.Subject, Slot: r.Family, ID: r.ID, Problem: problem})
		return
	}
	fileID := strings.TrimSuffix(path.Base(p), path.Ext(p))
	if strings.HasSuffix(p, ".md") && record.KindOfID(fileID) == record.Document {
		// A rejected body often follows an already diagnosed envelope (missing
		// because the body is irregular, or invalid so its body becomes an
		// orphan). Carry that envelope's exact slot instead of broadening one
		// broken slot into a failure for every document on the subject.
		matched := false
		bodySubject := path.Dir(p)
		for _, issue := range s.DocumentIssues {
			if issue.ID == fileID && issue.Subject == bodySubject {
				s.DocumentIssues = append(s.DocumentIssues, DocumentIssue{Subject: issue.Subject, Slot: issue.Slot, ID: fileID, Problem: problem})
				matched = true
			}
		}
		if matched {
			return
		}
	}
	var identity struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Subject string `json:"subject"`
		Family  string `json:"family"`
	}
	_ = json.Unmarshal(b, &identity) // malformed input falls back to its canonical path and id
	if identity.Kind != record.Document && record.KindOfID(identity.ID) != record.Document && record.KindOfID(fileID) != record.Document {
		return
	}
	// The containing path is authoritative for fail-closed projection. If a
	// damaged envelope's subject field was changed, trusting that field would
	// let the subject it used to occupy silently fall back to an older version.
	pathSubject := path.Dir(p)
	subject := pathSubject
	if !record.ValidSubject(subject) {
		subject = identity.Subject
	}
	if !record.ValidSubject(subject) {
		return
	}
	slot := identity.Family
	if !record.ValidRef(subject + "#" + slot) {
		slot = ""
	}
	id := identity.ID
	if record.KindOfID(id) != record.Document {
		id = fileID
	}
	s.DocumentIssues = append(s.DocumentIssues, DocumentIssue{Subject: subject, Slot: slot, ID: id, Problem: problem})
}

func parseAt(p string, b []byte) (*record.Record, error) {
	if !strings.HasSuffix(p, ".json") {
		return nil, fmt.Errorf("%w: not a record file (records are .json envelopes, document bodies .md)", record.ErrInvalid)
	}
	r, err := record.Unmarshal(b)
	if err != nil {
		return nil, err
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if want := r.Path(); want != p {
		return nil, fmt.Errorf("%w: %s is stored at %s, its path is %s", record.ErrInvalid, r.ID, p, want)
	}
	return r, nil
}

// Get returns a record by id, or nil.
func (s *Set) Get(id string) *record.Record { return s.Records[id] }

// IDs lists every record id in time order.
func (s *Set) IDs() []string {
	ids := make([]string, 0, len(s.Records))
	for id := range s.Records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Subjects lists every subject with at least one record, sorted.
func (s *Set) Subjects() []string {
	out := make([]string, 0, len(s.BySubject))
	for sub := range s.BySubject {
		out = append(out, sub)
	}
	sort.Strings(out)
	return out
}

// From returns the records that carry a relation of type t to target, in id order.
func (s *Set) From(target, t string) []*record.Record {
	var out []*record.Record
	for _, e := range s.To[target] {
		if e.Type == t {
			out = append(out, s.Records[e.From])
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ErrEscapes is returned when a write would leave the memory root or pass
// through a symlink.
var ErrEscapes = errors.New("path leaves the memory root or passes through a link")

// confined returns the absolute path for rel under root after checking that it
// stays under root and that every existing directory on the way is a real
// directory, not a link. The final component is created with O_EXCL by the
// caller, which never follows a link either.
func confined(root, rel string) (string, error) {
	root = filepath.Clean(root)
	full := filepath.Join(root, filepath.FromSlash(rel))
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrEscapes, rel)
	}
	dir := filepath.Dir(full)
	for d := dir; len(d) >= len(root); d = filepath.Dir(d) {
		info, err := os.Lstat(d)
		if errors.Is(err, fs.ErrNotExist) {
			if d == root {
				break
			}
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect %s: %w", d, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("%w: %s is not a real directory", ErrEscapes, d)
		}
		if d == root {
			break
		}
	}
	return full, nil
}

// Write stores a new record under root (the memory directory). It refuses to
// overwrite: a record that exists is never changed. A Document is written with
// WriteDocument, which stores its body beside it.
func Write(root string, r *record.Record) (string, error) {
	if r.Kind == record.Document {
		return "", fmt.Errorf("%w: a Document is written with its body", record.ErrInvalid)
	}
	return writeFile(root, r)
}

// WriteDocument stores a Document envelope and its markdown body as two
// sibling files. The envelope's hash and, unless the body is literal, embeds
// are set from the body.
func WriteDocument(root string, r *record.Record, body []byte) (string, error) {
	if r.Kind != record.Document {
		return "", fmt.Errorf("%w: %s is not a Document", record.ErrInvalid, r.Kind)
	}
	if !r.LiteralBody {
		body = record.NormalizeBody(body)
	}
	r.BodySHA256 = documentBodyHash(body, r.LiteralBody)
	r.Embeds = nil
	if !r.LiteralBody {
		r.Embeds = record.ParseEmbeds(body, r.Subject)
	}
	if err := r.Validate(); err != nil {
		return "", err
	}
	bodyFull, err := confined(root, r.BodyPath())
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(bodyFull); err == nil {
		return "", fmt.Errorf("%w: %s", ErrExists, r.BodyPath()) // anything at the path, a dangling link included
	}
	if err := os.MkdirAll(filepath.Dir(bodyFull), 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", filepath.Dir(bodyFull), err)
	}
	// the body lands first, then the envelope that names it: a reader never
	// sees an envelope whose body is missing, and a failed envelope leaves
	// no body behind. publish links the finished bytes into place and never
	// replaces anything, a dangling link included.
	if err := publish(bodyFull, body); err != nil {
		return "", err
	}
	p, err := writeFile(root, r)
	if err != nil {
		_ = os.Remove(bodyFull) // no envelope, no body: the pair is written or not at all
		return "", err
	}
	return p, nil
}

// documentBodyHash preserves the identity of literal source bytes. Native
// Documents use the canonical, checkout-independent hash defined by record.
func documentBodyHash(body []byte, literal bool) string {
	if !literal {
		return record.HashBody(body)
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum)
}

func documentBodyHashMatches(body []byte, want string, literal bool) bool {
	if literal {
		return documentBodyHash(body, true) == want
	}
	return record.BodyHashMatches(body, want)
}

func writeFile(root string, r *record.Record) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	b, err := record.Marshal(r)
	if err != nil {
		return "", err
	}
	full, err := confined(root, r.Path())
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(full); err == nil {
		return "", fmt.Errorf("%w: %s", ErrExists, r.Path())
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", filepath.Dir(full), err)
	}
	if err := publish(full, b); err != nil {
		return "", err
	}
	return path.Join(Dir, r.Path()), nil
}

// publish writes content to a temporary file in the target's directory,
// syncs it, and links it into place under its final name. The final name
// appears only once the bytes are complete, and never replaces an existing
// file: an interrupted write leaves a temporary file to clean up, not a
// truncated record at its canonical path.
func publish(full string, content []byte) error {
	dir := filepath.Dir(full)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(full)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create %s: %w", full, err)
	}
	name := tmp.Name()
	fail := func(step string, err error) error {
		_ = tmp.Close()     // best effort on the failure path
		_ = os.Remove(name) // the temporary never becomes a record
		return fmt.Errorf("%s %s: %w", step, full, err)
	}
	if _, err := tmp.Write(content); err != nil {
		return fail("write", err)
	}
	if err := tmp.Sync(); err != nil {
		return fail("sync", err)
	}
	if err := tmp.Close(); err != nil {
		return fail("close", err)
	}
	if err := os.Link(name, full); err != nil {
		_ = os.Remove(name)
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrExists, filepath.Base(full))
		}
		return fmt.Errorf("publish %s: %w", full, err)
	}
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("publish %s: remove temporary: %w", full, err)
	}
	return nil
}

// WritePolicy stores policy.json under root, replacing any previous policy
// (policy is the one file that is edited in place; git keeps its history).
func WritePolicy(root string, b []byte) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", root, err)
	}
	full, err := confined(root, PolicyFile)
	if err != nil {
		return err
	}
	if info, err := os.Lstat(full); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrEscapes, PolicyFile)
	}
	if err := os.WriteFile(full, b, 0o644); err != nil {
		return fmt.Errorf("write policy: %w", err)
	}
	return nil
}
