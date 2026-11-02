package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

var t0 = time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

func rec(kind, subject, family, statement string, n byte, rels ...record.Relation) *record.Record {
	var e [10]byte
	e[9] = n
	return &record.Record{Schema: record.Schema, ID: record.NewID(kind, t0.Add(time.Duration(n)*time.Second), e), Kind: kind, Subject: subject,
		Family: family, Statement: statement, Issuer: "m@example.com", CreatedAt: t0.Format(time.RFC3339), Relations: rels}
}

func bytesOf(t *testing.T, r *record.Record) []byte {
	t.Helper()
	b, err := record.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoadParsesIndexesAndReportsProblems(t *testing.T) {
	fr1 := rec(record.Promise, "prd/x", "FR-1", "the executor shall build the frontier", 1)
	fr1b := rec(record.Promise, "prd/x", "FR-1", "the executor shall build the ready frontier", 2, record.Relation{Type: record.Supersedes, Target: fr1.ID})
	d1 := rec(record.Decision, "design/x", "D-1", "a dispatcher over a worker", 3, record.Relation{Type: record.Serves, Target: fr1b.ID},
		record.Relation{Type: record.About, Target: "Need-01ARZ3NDEKTSV4RRFFQ69G5FAV"})
	src := MapSource{
		fr1.Path():                  bytesOf(t, fr1),
		fr1b.Path():                 bytesOf(t, fr1b),
		d1.Path():                   bytesOf(t, d1),
		"policy.json":               []byte(`{"schema":1}`),
		"prd/x/notes.txt":           []byte("hello"),
		"prd/x/" + fr1.ID + ".json": bytesOf(t, fr1), // duplicate id at a second path → refused
		"prd/y/" + d1.ID + ".json":  bytesOf(t, d1),  // wrong path → refused
		"prd/z/broken.json":         []byte(`{"schema":1,`),
	}
	// a second record with the same id on another subject sits at its own valid path and sorts after the original
	dup := *fr1
	dup.Subject = "prd/xx"
	delete(src, "prd/x/"+fr1.ID+".json")
	src[fr1.Path()] = bytesOf(t, fr1)
	src[dup.Path()] = bytesOf(t, &dup)
	s, err := Load(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Records) != 3 {
		t.Fatalf("records: %v", s.IDs())
	}
	if string(s.Policy) != `{"schema":1}` {
		t.Fatalf("policy: %q", s.Policy)
	}
	if got := s.Subjects(); strings.Join(got, ",") != "design/x,prd/x" {
		t.Fatalf("subjects: %v", got)
	}
	if got := s.BySubject["prd/x"]; len(got) != 2 || got[0].ID != fr1.ID {
		t.Fatalf("by subject order: %v", got)
	}
	if got := s.From(fr1.ID, record.Supersedes); len(got) != 1 || got[0].ID != fr1b.ID {
		t.Fatalf("From: %v", got)
	}
	if s.Get(d1.ID) == nil || s.Get("nope") != nil {
		t.Fatal("Get")
	}
	if len(s.Dangling) != 1 || s.Dangling[0].From != d1.ID || s.Dangling[0].Type != record.About {
		t.Fatalf("dangling: %+v", s.Dangling)
	}
	var paths []string
	for _, p := range s.Problems {
		paths = append(paths, p.Path)
		if !errors.Is(p.Err, record.ErrInvalid) {
			t.Fatalf("problem %s does not wrap ErrInvalid: %v", p.Path, p.Err)
		}
	}
	if strings.Join(paths, ",") != "prd/x/notes.txt,prd/xx/"+fr1.ID+".json,prd/y/"+d1.ID+".json,prd/z/broken.json" {
		t.Fatalf("problems: %v", paths)
	}
	for _, p := range s.Problems {
		switch {
		case strings.HasPrefix(p.Path, "prd/xx/"):
			if !strings.Contains(p.Err.Error(), "already used") {
				t.Fatalf("duplicate id message: %v", p.Err)
			}
		case strings.HasPrefix(p.Path, "prd/y/"):
			if !strings.Contains(p.Err.Error(), "its path is") {
				t.Fatalf("wrong path message: %v", p.Err)
			}
		}
	}
}

func TestDirSourceAndWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	if files, _, err := (DirSource{Root: root}).Files(context.Background()); err != nil || len(files) != 0 {
		t.Fatalf("missing root must read as empty: %v %v", files, err)
	}
	r := rec(record.Promise, "prd/x", "FR-1", "shall", 1)
	p, err := Write(root, r)
	if err != nil {
		t.Fatal(err)
	}
	if p != "memory/prd/x/"+r.ID+".json" {
		t.Fatalf("path %q", p)
	}
	if _, err := Write(root, r); !errors.Is(err, ErrExists) {
		t.Fatalf("overwrite must be refused: %v", err)
	}
	bad := *r
	bad.Statement = ""
	if _, err := Write(root, &bad); !errors.Is(err, record.ErrInvalid) {
		t.Fatalf("invalid record must be refused before writing: %v", err)
	}
	if err := WritePolicy(root, []byte(`{"schema":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := WritePolicy(root, []byte(`{"schema":1,"x":1}`)); err != nil {
		t.Fatal("policy is replaceable")
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if s.Get(r.ID) == nil || string(s.Policy) != `{"schema":1,"x":1}` || len(s.Problems) != 0 {
		t.Fatalf("loaded: %v %q %v", s.IDs(), s.Policy, s.Problems)
	}
	if err := os.WriteFile(filepath.Join(root, "prd", "x", "stray.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Load(context.Background(), DirSource{Root: root})
	if err != nil || len(s.Problems) != 1 {
		t.Fatalf("stray file must be a problem: %v %v", s.Problems, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := (DirSource{Root: root}).Files(ctx); err == nil {
		t.Fatal("cancelled context must stop the walk")
	}
	if _, _, err := (DirSource{Root: filepath.Join(root, "prd", "x", "stray.json")}).Files(context.Background()); err == nil {
		t.Fatal("a file as root is an error")
	}
	if _, err := Write(filepath.Join(root, "prd", "x", "stray.json"), rec(record.Promise, "prd/q", "FR-1", "s", 2)); err == nil {
		t.Fatal("writing under a file must fail")
	}
	if err := WritePolicy(filepath.Join(root, "prd", "x", "stray.json"), nil); err == nil {
		t.Fatal("policy under a file must fail")
	}
}

func TestPublicationIsAtomicAndNeverOverwrites(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	r := rec(record.Promise, "prd/x", "FR-1", "shall", 1)
	dir := filepath.Join(root, "prd", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// a directory that refuses new files: nothing appears at the final path and no temporary is left behind
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, r); err == nil {
		t.Fatal("an unwritable directory must fail")
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a failed publication leaves nothing: %v", entries)
	}
	// a record that appears at the final path between the check and the link is never overwritten
	if _, err := Write(root, r); err != nil {
		t.Fatal(err)
	}
	if err := publish(filepath.Join(dir, r.ID+".json"), []byte("other")); !errors.Is(err, ErrExists) {
		t.Fatalf("link into an existing name: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("no temporary survives a refused publication: %v", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, r.ID+".json")); !strings.Contains(string(b), "shall") {
		t.Fatal("the stored record is untouched")
	}
	if err := publish(filepath.Join(t.TempDir(), "missing", "x.json"), []byte("x")); err == nil {
		t.Fatal("a missing directory fails before any file exists")
	}
}

func TestLinksAreRefusedOnReadAndWrite(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, Dir)
	r := rec(record.Promise, "prd/x", "FR-1", "shall", 1)
	if _, err := Write(root, r); err != nil {
		t.Fatal(err)
	}
	// a record that is a symlink to a file outside the memory, and a linked directory
	outside := filepath.Join(base, "payload.json")
	if err := os.WriteFile(outside, bytesOf(t, rec(record.Promise, "prd/x", "FR-2", "smuggled", 2)), 0o644); err != nil {
		t.Fatal(err)
	}
	linked := rec(record.Promise, "prd/x", "FR-2", "smuggled", 2)
	if err := os.Symlink(outside, filepath.Join(root, "prd", "x", linked.ID+".json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "prd", "x"), filepath.Join(root, "prd", "y")); err != nil {
		t.Fatal(err)
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if s.Get(linked.ID) != nil || len(s.Problems) != 2 {
		t.Fatalf("links must not read as records: %v %v", s.IDs(), s.Problems)
	}
	for _, p := range s.Problems {
		if !errors.Is(p.Err, record.ErrInvalid) || !strings.Contains(p.Err.Error(), "symlink") {
			t.Fatalf("problem %s: %v", p.Path, p.Err)
		}
	}
	// writes never pass through a link or leave the root
	if _, err := Write(root, rec(record.Promise, "prd/y", "FR-3", "via the linked dir", 3)); !errors.Is(err, ErrEscapes) {
		t.Fatalf("write through a linked directory: %v", err)
	}
	escaping := rec(record.Promise, "prd/x", "FR-4", "s", 4)
	escaping.Subject = "prd/../../x" // an invalid subject is refused by validation before the path is even built
	if _, err := Write(root, escaping); !errors.Is(err, record.ErrInvalid) {
		t.Fatalf("escaping subject: %v", err)
	}
	if _, err := confined(root, "../outside.json"); !errors.Is(err, ErrEscapes) {
		t.Fatalf("confined must refuse a path leaving the root: %v", err)
	}
	if err := os.Remove(filepath.Join(root, PolicyFile)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, PolicyFile)); err != nil {
		t.Fatal(err)
	}
	if err := WritePolicy(root, []byte(`{"schema":1}`)); !errors.Is(err, ErrEscapes) {
		t.Fatalf("policy through a link: %v", err)
	}
	if got, _ := os.ReadFile(outside); !strings.Contains(string(got), "smuggled") {
		t.Fatal("the link target must be untouched")
	}
	// a record file that is a symlink at the write path is never overwritten
	if _, err := Write(root, linked); !errors.Is(err, ErrExists) {
		t.Fatalf("existing link at the record path: %v", err)
	}
}

type failingSource struct{}

func (failingSource) Files(context.Context) (map[string][]byte, map[string]string, error) {
	return nil, nil, errors.New("source down")
}

func TestReadAndWriteFailuresAreErrorsNotSilence(t *testing.T) {
	if _, err := Load(context.Background(), failingSource{}); err == nil || !strings.Contains(err.Error(), "source down") {
		t.Fatalf("a failing source fails the load: %v", err)
	}
	base := t.TempDir()
	root := filepath.Join(base, Dir)
	r := rec(record.Promise, "prd/x", "FR-1", "shall", 1)
	if _, err := Write(root, r); err != nil {
		t.Fatal(err)
	}
	// something that is neither a file nor a directory under the root is irregular, never content
	if err := syscall.Mkfifo(filepath.Join(root, "prd", "x", "pipe.json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, irregular, err := (DirSource{Root: root}).Files(context.Background())
	if err != nil || irregular["prd/x/pipe.json"] != "not a regular file" {
		t.Fatalf("a fifo is irregular: %v %v", irregular, err)
	}
	if err := os.Remove(filepath.Join(root, "prd", "x", "pipe.json")); err != nil {
		t.Fatal(err)
	}
	// a record that cannot be read fails the walk rather than reading as absent
	unreadable := rec(record.Promise, "prd/x", "FR-2", "hidden", 2)
	if _, err := Write(root, unreadable); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, "prd", "x", unreadable.ID+".json")
	if err := os.Chmod(full, 0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (DirSource{Root: root}).Files(context.Background()); err == nil {
		t.Fatal("an unreadable record must fail the read")
	}
	if err := os.Chmod(full, 0o644); err != nil {
		t.Fatal(err)
	}
	// a directory that cannot be entered fails the walk
	closed := filepath.Join(root, "prd", "closed")
	if err := os.MkdirAll(closed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o755) })
	if _, _, err := (DirSource{Root: root}).Files(context.Background()); err == nil {
		t.Fatal("an unenterable directory must fail the read")
	}
	if err := os.Chmod(closed, 0o755); err != nil {
		t.Fatal(err)
	}
	// a root whose directories cannot be created refuses the write with the reason
	sealed := filepath.Join(t.TempDir(), Dir)
	if err := os.MkdirAll(sealed, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o755) })
	if _, err := Write(sealed, rec(record.Promise, "prd/y", "FR-1", "s", 3)); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("unwritable root: %v", err)
	}
	if err := WritePolicy(filepath.Join(sealed, "sub"), []byte(`{"schema":1}`)); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("policy under an unwritable root: %v", err)
	}
	// a policy root that is a link to a directory is refused like any other link
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WritePolicy(link, []byte(`{"schema":1}`)); !errors.Is(err, ErrEscapes) {
		t.Fatalf("policy through a linked root: %v", err)
	}
}
func TestDocumentsPairEnvelopeAndBody(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	doc := rec(record.Document, "prd/x", "prd", "Stacked PRs", 7)
	body := []byte("# Stacked PRs\n\nThe executor shall honour {{FR-1}} and {{design/x#D-1}}.\n")
	if _, err := Write(root, doc); err == nil {
		t.Fatal("a Document needs its body")
	}
	promise := rec(record.Promise, "prd/x", "FR-1", "one", 8)
	if _, err := WriteDocument(root, promise, body); err == nil {
		t.Fatal("only a Document takes a body")
	}
	p, err := WriteDocument(root, doc, body)
	if err != nil {
		t.Fatal(err)
	}
	if p != "memory/prd/x/"+doc.ID+".json" || doc.BodySHA256 != record.HashBody(body) || strings.Join(doc.Embeds, ",") != "design/x#D-1,prd/x#FR-1" {
		t.Fatalf("written %s %+v", p, doc)
	}
	if _, err := WriteDocument(root, doc, body); !errors.Is(err, ErrExists) {
		t.Fatalf("bodies are immutable: %v", err)
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Problems) != 0 || string(s.Bodies[doc.ID]) != string(body) || s.Get(doc.ID) == nil {
		t.Fatalf("loaded: %v %q", s.Problems, s.Bodies[doc.ID])
	}
	// tampering with the body, dropping it, and a body with no envelope are all problems
	if err := os.WriteFile(filepath.Join(root, "prd", "x", doc.ID+".md"), []byte("edited in place"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "prd", "x", "Document-01ARZ3NDEKTSV4RRFFQ69G5FAV.md"), []byte("orphan"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ = Load(context.Background(), DirSource{Root: root})
	var msgs []string
	for _, pr := range s.Problems {
		msgs = append(msgs, pr.Path+": "+pr.Err.Error())
	}
	joined := strings.Join(msgs, "\n")
	if len(s.Problems) != 2 || !strings.Contains(joined, "does not match body_sha256") || !strings.Contains(joined, "a body with no Document envelope") {
		t.Fatalf("problems:\n%s", joined)
	}
	if err := os.Remove(filepath.Join(root, "prd", "x", doc.ID+".md")); err != nil {
		t.Fatal(err)
	}
	s, _ = Load(context.Background(), DirSource{Root: root})
	if len(s.Problems) != 2 || !strings.Contains(s.Problems[0].Path+s.Problems[1].Path, doc.ID+".json") {
		t.Fatalf("missing body must be a problem on the envelope: %v", s.Problems)
	}
	if _, err := WriteDocument(filepath.Join(root, "prd", "x", "Document-01ARZ3NDEKTSV4RRFFQ69G5FAV.md"), rec(record.Document, "prd/q", "prd", "t", 9), body); err == nil {
		t.Fatal("writing under a file must fail")
	}
}

func TestDocumentBodiesUseCanonicalLineEndings(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	lf := []byte("# Title\n\nNarrative {{FR-1}}.\n")
	crlf := []byte("# Title\r\n\r\nNarrative {{FR-1}}.\r\n")
	doc := rec(record.Document, "prd/x", "prd", "Title", 15)
	if _, err := WriteDocument(root, doc, crlf); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc.BodyPath())))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(lf) || doc.BodySHA256 != record.HashBody(lf) {
		t.Fatalf("write must publish canonical LF bytes: body %q hash %s", written, doc.BodySHA256)
	}

	// Simulate the same blob checked out as CRLF despite local Git settings.
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(doc.BodyPath())), crlf, 0o644); err != nil {
		t.Fatal(err)
	}
	envelopePath := filepath.Join(root, filepath.FromSlash(doc.Path()))
	envelope, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envelopePath, []byte(strings.ReplaceAll(string(envelope), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	policyBody := []byte("{\r\n  \"default\": \"low\"\r\n}\r\n")
	if err := os.WriteFile(filepath.Join(root, PolicyFile), policyBody, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil || len(s.Problems) != 0 || string(s.Bodies[doc.ID]) != string(lf) || string(s.Bytes[doc.BodyPath()]) != string(lf) ||
		strings.Contains(string(s.Bytes[doc.Path()]), "\r") || strings.Contains(string(s.Policy), "\r") {
		t.Fatalf("a CRLF checkout must load as canonical LF: problems %v body %q envelope %q policy %q err %v", s.Problems, s.Bodies[doc.ID], s.Bytes[doc.Path()], s.Policy, err)
	}

	// Before canonical hashing, a CRLF-authored body carried the raw CRLF
	// digest. It remains readable whether Git presents its blob as LF or CRLF.
	legacy := rec(record.Document, "prd/legacy", "prd", "Legacy", 16)
	sum := sha256.Sum256(crlf)
	legacy.BodySHA256 = hex.EncodeToString(sum[:])
	legacy.Embeds = record.ParseEmbeds(lf, legacy.Subject)
	for name, checkout := range map[string][]byte{"lf checkout": lf, "crlf checkout": crlf} {
		t.Run(name, func(t *testing.T) {
			s, err := Load(context.Background(), MapSource{legacy.Path(): bytesOf(t, legacy), legacy.BodyPath(): checkout})
			if err != nil || len(s.Problems) != 0 || string(s.Bodies[legacy.ID]) != string(lf) {
				t.Fatalf("legacy CRLF body must load canonically: problems %v body %q err %v", s.Problems, s.Bodies[legacy.ID], err)
			}
		})
	}
}

func TestLiteralDocumentBodiesPreserveExactBytes(t *testing.T) {
	literalRoot := filepath.Join(t.TempDir(), Dir)
	body := []byte("# Imported\r\n\r\nTemplate {{NAME}}.\r\nNo trailing newline")
	literal := rec(record.Document, "source/imported", "source", "Imported source", 21)
	literal.LiteralBody = true
	if _, err := WriteDocument(literalRoot, literal, body); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(literalRoot, filepath.FromSlash(literal.BodyPath())))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if string(written) != string(body) || literal.BodySHA256 != hex.EncodeToString(sum[:]) || len(literal.Embeds) != 0 {
		t.Fatalf("literal write changed source bytes: body %q hash %s embeds %v", written, literal.BodySHA256, literal.Embeds)
	}
	s, err := Load(context.Background(), DirSource{Root: literalRoot})
	if err != nil || len(s.Problems) != 0 || string(s.Bodies[literal.ID]) != string(body) || string(s.Bytes[literal.BodyPath()]) != string(body) {
		t.Fatalf("literal load changed source bytes: problems %v body %q bytes %q err %v", s.Problems, s.Bodies[literal.ID], s.Bytes[literal.BodyPath()], err)
	}

	// Literal identity is byte-exact: normalizing the stored body after the
	// write is a modification, even though native Documents accept that
	// checkout transformation.
	lf := record.NormalizeBody(body)
	if err := os.WriteFile(filepath.Join(literalRoot, filepath.FromSlash(literal.BodyPath())), lf, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Load(context.Background(), DirSource{Root: literalRoot})
	if err != nil || s.Get(literal.ID) != nil || len(s.Problems) != 1 || !strings.Contains(s.Problems[0].Err.Error(), "does not match body_sha256") {
		t.Fatalf("a normalized literal body must be rejected: records %v problems %v err %v", s.IDs(), s.Problems, err)
	}

	nativeRoot := filepath.Join(t.TempDir(), Dir)
	native := rec(record.Document, "prd/native", "prd", "Native", 22)
	if _, err := WriteDocument(nativeRoot, native, body); err != nil {
		t.Fatal(err)
	}
	written, err = os.ReadFile(filepath.Join(nativeRoot, filepath.FromSlash(native.BodyPath())))
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(lf) || native.BodySHA256 != record.HashBody(lf) {
		t.Fatalf("native write must retain canonical LF behavior: body %q hash %s", written, native.BodySHA256)
	}
}

func TestMalformedDocumentEnvelopePoisonsOnlyItsSubject(t *testing.T) {
	doc := rec(record.Document, "prd/x", "prd", "Title", 17)
	s, err := Load(context.Background(), MapSource{doc.Path(): []byte(`{"schema":1`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DocumentProblems("prd/x", "prd")) != 1 || len(s.DocumentProblems("prd/x", "design")) != 1 {
		t.Fatalf("an unreadable Document envelope has an unknown slot and must poison this subject: %+v", s.DocumentIssues)
	}
	if got := s.DocumentProblems("prd/y", "prd"); len(got) != 0 {
		t.Fatalf("an unrelated subject must remain readable: %v", got)
	}

	// A corrupted subject inside the envelope cannot redirect the poison away
	// from the path whose document history has become unknowable.
	b := bytesOf(t, doc)
	b = []byte(strings.Replace(string(b), `"subject":"prd/x"`, `"subject":"prd/y"`, 1))
	s, err = Load(context.Background(), MapSource{doc.Path(): b, doc.BodyPath(): []byte("# Title\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DocumentProblems("prd/x", "prd")) == 0 {
		t.Fatalf("the canonical path's subject must fail closed: %+v", s.DocumentIssues)
	}
	if got := s.DocumentProblems("prd/x", "design"); len(got) != 0 {
		t.Fatalf("a valid family still scopes a subject-mismatched envelope to its slot: %v", got)
	}
	if got := s.DocumentProblems("prd/y", "prd"); len(got) != 0 {
		t.Fatalf("an invalid cross-subject path cannot poison the claimed subject: %v", got)
	}
}

func TestRejectedDocumentBodyPoisonsOnlyItsEnvelopeSlot(t *testing.T) {
	design := rec(record.Document, "prd/x", "design", "Design", 18)
	prd := rec(record.Document, "prd/x", "prd", "PRD", 19)
	prdBody := []byte("# PRD\n")
	prd.BodySHA256 = record.HashBody(prdBody)

	// A parseable but invalid design envelope leaves its sibling body orphaned.
	// Both diagnostics must remain scoped to design, not poison the healthy PRD.
	invalidDesign := bytesOf(t, design)
	s, err := Load(context.Background(), MapSource{
		design.Path():     invalidDesign,
		design.BodyPath(): []byte("# Design\n"),
		prd.Path():        bytesOf(t, prd),
		prd.BodyPath():    prdBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DocumentProblems("prd/x", "design")) != 2 {
		t.Fatalf("the invalid envelope and orphan body must diagnose the design slot: %+v", s.DocumentIssues)
	}
	if got := s.DocumentProblems("prd/x", "prd"); len(got) != 0 {
		t.Fatalf("a healthy sibling slot must remain readable: %v", got)
	}

	// DirSource excludes a symlink from files and reports it as irregular. The
	// valid envelope still supplies the slot identity for that body diagnostic.
	root := filepath.Join(t.TempDir(), Dir)
	if err := os.MkdirAll(filepath.Join(root, "prd", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	designBody := []byte("# Design\n")
	design.BodySHA256 = record.HashBody(designBody)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(design.Path())), bytesOf(t, design), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "outside.md"), filepath.Join(root, filepath.FromSlash(design.BodyPath()))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(prd.Path())), bytesOf(t, prd), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(prd.BodyPath())), prdBody, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err = Load(context.Background(), DirSource{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DocumentProblems("prd/x", "design")) != 2 {
		t.Fatalf("the missing and irregular body must diagnose the design slot: %+v", s.DocumentIssues)
	}
	if got := s.DocumentProblems("prd/x", "prd"); len(got) != 0 {
		t.Fatalf("an irregular design body must not poison prd: %v", got)
	}

	orphan := rec(record.Document, "prd/orphan", "prd", "Orphan", 20)
	s, err = Load(context.Background(), MapSource{orphan.BodyPath(): []byte("# Orphan\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.DocumentProblems("prd/orphan", "prd")) != 1 || len(s.DocumentProblems("prd/orphan", "design")) != 1 {
		t.Fatalf("a pure orphan has no recoverable slot and must poison all slots on its path subject: %+v", s.DocumentIssues)
	}
}

func TestDocumentBodyIsConfinedAndExclusive(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, Dir)
	outside := filepath.Join(base, "escaped.md")
	doc := rec(record.Document, "prd/x", "prd", "t", 12)
	if err := os.MkdirAll(filepath.Join(root, "prd", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a dangling link at the body path: neither followed nor overwritten
	if err := os.Symlink(outside, filepath.Join(root, filepath.FromSlash(doc.BodyPath()))); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDocument(root, doc, []byte("# t\n")); !errors.Is(err, ErrExists) {
		t.Fatalf("a link at the body path must refuse the write: %v", err)
	}
	if _, err := os.Lstat(outside); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("nothing may be created outside the memory root")
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(doc.Path()))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("no envelope without its body")
	}
	// a linked parent directory is refused before anything is written
	if err := os.Symlink(filepath.Join(root, "prd", "x"), filepath.Join(root, "prd", "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDocument(root, rec(record.Document, "prd/linked", "prd", "t", 13), []byte("# t\n")); !errors.Is(err, ErrEscapes) {
		t.Fatalf("a linked directory must refuse the write: %v", err)
	}
	// when the envelope cannot be written the body does not remain
	clean := rec(record.Document, "prd/y", "prd", "t", 14)
	if _, err := WriteDocument(root, clean, []byte("# t\n")); err != nil {
		t.Fatal(err)
	}
	dup := rec(record.Document, "prd/y", "prd", "t", 14)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(dup.BodyPath()))); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDocument(root, dup, []byte("# t2\n")); !errors.Is(err, ErrExists) {
		t.Fatalf("an existing envelope refuses the pair: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dup.BodyPath()))); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("the body written for a refused pair must be removed")
	}
}

func TestEnvelopeEmbedsMustMatchTheBody(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	body := []byte("# x\n\n{{FR-DOES-NOT-EXIST}}\n")
	forged := rec(record.Document, "prd/x", "prd", "hand-written", 11)
	forged.BodySHA256 = record.HashBody(body)
	forged.Embeds = nil // the envelope claims the body embeds nothing
	if err := os.MkdirAll(filepath.Join(root, "prd", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(forged.Path())), bytesOf(t, forged), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(forged.BodyPath())), body, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if s.Get(forged.ID) != nil || len(s.Problems) != 1 || !strings.Contains(s.Problems[0].Err.Error(), "embeds of "+forged.ID+" do not match its body (body: prd/x#FR-DOES-NOT-EXIST; envelope: )") {
		t.Fatalf("a forged embeds list must be refused: %v %v", s.IDs(), s.Problems)
	}
}

func TestStoreErrorPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), Dir)
	r := rec(record.Promise, "prd/x", "FR-1", "shall", 1)
	if _, err := Write(root, r); err != nil {
		t.Fatal(err)
	}
	// a special file under the memory is irregular, not content
	if err := syscall.Mkfifo(filepath.Join(root, "prd", "x", "pipe.json"), 0o600); err != nil {
		t.Skipf("no fifos here: %v", err)
	}
	s, err := Load(context.Background(), DirSource{Root: root})
	if err != nil || len(s.Problems) != 1 || !strings.Contains(s.Problems[0].Err.Error(), "not a regular file") {
		t.Fatalf("fifo: %v %v", s, err)
	}
	if err := os.Remove(filepath.Join(root, "prd", "x", "pipe.json")); err != nil {
		t.Fatal(err)
	}
	// a file as the memory root fails the load itself
	if _, err := Load(context.Background(), DirSource{Root: filepath.Join(root, filepath.FromSlash(r.Path()))}); err == nil {
		t.Fatal("a file as root must fail the load")
	}
	// an invalid document is refused before anything is written
	bad := rec(record.Document, "prd/x", "prd", "", 2)
	if _, err := WriteDocument(root, bad, []byte("# t\n")); !errors.Is(err, record.ErrInvalid) {
		t.Fatalf("invalid document: %v", err)
	}
	// a directory that cannot be written refuses both a record and a document body
	sealed := filepath.Join(root, "prd", "sealed")
	if err := os.MkdirAll(sealed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sealed, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o755) })
	if _, err := Write(root, rec(record.Promise, "prd/sealed", "FR-1", "s", 3)); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("sealed record dir: %v", err)
	}
	if _, err := WriteDocument(root, rec(record.Document, "prd/sealed", "prd", "t", 4), []byte("# t\n")); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("sealed document dir: %v", err)
	}
	// an unreadable record file fails the walk with the reason
	unreadable := rec(record.Promise, "prd/x", "FR-2", "hidden", 5)
	if _, err := Write(root, unreadable); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(unreadable.Path())), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, filepath.FromSlash(unreadable.Path())), 0o644) })
	if _, err := Load(context.Background(), DirSource{Root: root}); err == nil || !strings.Contains(err.Error(), "read memory dir") {
		t.Fatalf("unreadable record: %v", err)
	}
}
