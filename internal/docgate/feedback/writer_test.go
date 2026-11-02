package feedback_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback/adapters/stub"
)

func cc(comment, author, ts, anchor, source string) feedback.CapturedComment {
	return feedback.CapturedComment{
		SchemaVersion: feedback.CapturedCommentVersion,
		Comment:       comment,
		Author:        author,
		Timestamp:     ts,
		SubjectAnchor: anchor,
		SourceID:      source,
	}
}

// A-2 (T2 conformance): the Writer produces one entry per captured comment,
// driven through the CaptureSource port by a stub adapter, and every rendered
// entry folds back cleanly with all header fields populated.
func TestWriterProducesOneEntryPerComment(t *testing.T) {
	root := t.TempDir()
	src := stub.Source{Captured: []feedback.CapturedComment{
		cc("The fold should emit a fingerprint.", "@reviewer-one", "2026-07-13T09:30:00Z", "some-doc#3.1", "conversation"),
		cc("Keep the disposition set closed.", "@reviewer-two", "2026-07-13T11:05:00Z", "some-doc#3.4", "code-host"),
		cc("A third, verbatim comment.\n\nWith a blank line inside.", "@reviewer-one", "2026-07-13T12:00:00Z", "some-doc#5", "conversation"),
	}}

	w := feedback.NewWriter(root)
	paths, err := w.Capture(context.Background(), src)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if len(paths) != len(src.Captured) {
		t.Fatalf("wrote %d entries, want %d", len(paths), len(src.Captured))
	}

	// One subject dir, dense-numbered files, no clobber.
	dir := filepath.Join(root, "some-doc")
	names := entryNames(t, dir)
	want := []string{"001-conversation.md", "002-code-host.md", "003-conversation.md"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("entry files = %v, want %v", names, want)
	}

	// Every produced entry folds cleanly, carries no disposition (capture writes
	// the initial block only), and preserves its verbatim comment.
	entries := readSubject(t, dir)
	res := feedback.Fold("some-doc", entries)
	if len(res.Defects) != 0 {
		t.Fatalf("produced entries have fold defects: %v", res.Defects)
	}
	if len(res.Entries) != len(src.Captured) {
		t.Fatalf("folded %d entries, want %d", len(res.Entries), len(src.Captured))
	}
	for i, e := range res.Entries {
		in := src.Captured[i]
		if e.Author != in.Author || e.Timestamp != in.Timestamp || e.SubjectAnchor != in.SubjectAnchor || e.Source != in.SourceID {
			t.Fatalf("entry %s attribution drifted: %+v vs %+v", e.SourceFile, e, in)
		}
		if e.Comment != strings.TrimRight(in.Comment, "\n") {
			t.Fatalf("entry %s comment not verbatim:\n got %q\nwant %q", e.SourceFile, e.Comment, in.Comment)
		}
		if e.Disposition != nil {
			t.Fatalf("entry %s should have no disposition at capture: %+v", e.SourceFile, e.Disposition)
		}
	}
}

// The Writer routes each comment to its subject's directory and continues the
// dense sequence across separate Capture calls — it only ever adds (DINV-1).
func TestWriterRoutesBySubjectAndAppends(t *testing.T) {
	root := t.TempDir()
	w := feedback.NewWriter(root)

	if _, err := w.WriteEntry(cc("alpha one", "@a", "t1", "alpha#1", "conversation")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteEntry(cc("beta one", "@b", "t2", "beta#1", "code-host")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteEntry(cc("alpha two", "@a", "t3", "alpha#2", "conversation")); err != nil {
		t.Fatal(err)
	}

	if got := entryNames(t, filepath.Join(root, "alpha")); strings.Join(got, ",") != "001-conversation.md,002-conversation.md" {
		t.Fatalf("alpha entries = %v", got)
	}
	if got := entryNames(t, filepath.Join(root, "beta")); strings.Join(got, ",") != "001-code-host.md" {
		t.Fatalf("beta entries = %v", got)
	}
}

// The Writer refuses to render an ill-formed comment rather than emit a
// corrupt entry (fail-closed, AP-6): a wrong schema version and a
// table-breaking field are both rejected.
func TestWriterFailsClosedOnBadComment(t *testing.T) {
	root := t.TempDir()
	w := feedback.NewWriter(root)

	bad := cc("hi", "@a", "t", "doc#1", "conversation")
	bad.SchemaVersion = "captured-comment/v2"
	if _, err := w.WriteEntry(bad); err == nil {
		t.Fatal("expected rejection of an unpinned schema version")
	}

	pipe := cc("hi", "@a | @b", "t", "doc#1", "conversation")
	if _, err := w.WriteEntry(pipe); err == nil {
		t.Fatal("expected rejection of a table-breaking author")
	}
}

// A source error surfaces from Capture and produces no entries.
func TestWriterSurfacesSourceError(t *testing.T) {
	root := t.TempDir()
	w := feedback.NewWriter(root)
	_, err := w.Capture(context.Background(), stub.Source{Err: errors.New("boom")})
	if err == nil {
		t.Fatal("expected the source error to surface")
	}
	if items, _ := os.ReadDir(root); len(items) != 0 {
		t.Fatalf("a failed drain wrote %d dirs", len(items))
	}
}

func TestWriterBoundaryFailures(t *testing.T) {
	t.Run("nil source", func(t *testing.T) {
		if _, err := feedback.NewWriter(t.TempDir()).Capture(context.Background(), nil); err == nil {
			t.Fatal("expected a nil capture source to be rejected")
		}
	})

	t.Run("capture reports write failure", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(root, []byte("file"), 0o644); err != nil {
			t.Fatal(err)
		}
		src := stub.Source{Captured: []feedback.CapturedComment{
			cc("comment", "@author", "2026-07-13T09:30:00Z", "subject#1", "conversation"),
		}}
		paths, err := feedback.NewWriter(root).Capture(context.Background(), src)
		if err == nil || len(paths) != 0 {
			t.Fatalf("write failure: paths=%v err=%v", paths, err)
		}
	})

	t.Run("invalid subject anchor", func(t *testing.T) {
		bad := cc("comment", "@author", "timestamp", "Bad Subject#1", "conversation")
		if _, err := feedback.NewWriter(t.TempDir()).WriteEntry(bad); err == nil {
			t.Fatal("expected invalid subject anchor to be rejected")
		}
	})

	t.Run("invalid sequence", func(t *testing.T) {
		if _, err := feedback.RenderEntry("1", cc("comment", "@author", "timestamp", "subject#1", "conversation")); err == nil {
			t.Fatal("expected invalid entry sequence to be rejected")
		}
	})

	t.Run("invalid source id", func(t *testing.T) {
		if _, err := feedback.RenderEntry("001", cc("comment", "@author", "timestamp", "subject#1", "Bad Source")); err == nil {
			t.Fatal("expected invalid source id to be rejected")
		}
	})

	t.Run("invalid edit commit", func(t *testing.T) {
		bad := cc("comment", "@author", "timestamp", "subject#1", "conversation")
		bad.EditCommit = "bad|commit"
		if _, err := feedback.RenderEntry("001", bad); err == nil {
			t.Fatal("expected table-breaking edit commit to be rejected")
		}
	})
}

// A rendered entry round-trips through ParseEntry unchanged (the fold's parser
// accepts exactly what the writer emits).
func TestRenderEntryRoundTrips(t *testing.T) {
	in := cc("A verbatim comment.", "@reviewer", "2026-07-13T09:30:00Z", "doc#3.1", "conversation")
	b, err := feedback.RenderEntry("001", in)
	if err != nil {
		t.Fatal(err)
	}
	got, defs := feedback.ParseEntry("001-conversation.md", b)
	if len(defs) != 0 {
		t.Fatalf("rendered entry has parse defects: %v", defs)
	}
	if got.Entry != "001" || got.Source != "conversation" || got.Author != "@reviewer" ||
		got.Timestamp != "2026-07-13T09:30:00Z" || got.SubjectAnchor != "doc#3.1" ||
		got.Comment != "A verbatim comment." || got.Disposition != nil {
		t.Fatalf("round-trip drifted: %+v", got)
	}
}

// coreImportPrefix is the writer core's own module path; adapterPrefix is the
// package tree adapters live under. DINV-2: no non-test file in the core may
// import anything beneath adapterPrefix.
const (
	coreImportPrefix = "github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	adapterPrefix    = coreImportPrefix + "/adapters"
)

// A-2 (import-boundary lint): the writer core imports no adapter package
// (DINV-2). This is the enforcement CLAUDE.md/AP-1 demand — a lint rule, not
// convention — and it runs on every `go test ./...`.
func TestWriterCoreImportsNoAdapter(t *testing.T) {
	fset := token.NewFileSet()
	//nolint:staticcheck // Import-only AST inspection intentionally does not need package loading or build-tag association.
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse core package: %v", err)
	}
	scanned := 0
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			scanned++
			if hits := importsUnderPrefix(file, adapterPrefix); len(hits) != 0 {
				t.Errorf("writer core file %s imports adapter package(s) %v — DINV-2: a new source is a new adapter, never new core code", name, hits)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no core files — the lint would pass vacuously")
	}
}

// The lint is not vacuous: given a file that does import an adapter package,
// the same predicate flags it. This guards the guard.
func TestImportBoundaryLintDetectsViolation(t *testing.T) {
	src := `package feedback
import _ "` + adapterPrefix + `/stub"
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "violation.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if hits := importsUnderPrefix(f, adapterPrefix); len(hits) == 0 {
		t.Fatal("the import-boundary predicate failed to flag an adapter import")
	}
}

func importsUnderPrefix(f *ast.File, prefix string) []string {
	var hits []string
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			hits = append(hits, p)
		}
	}
	return hits
}

// DINV-6: the Writer is the sole entry producer — no guide hand-authors an
// entry. Unlike PRDs (prd-template) and designs (tech-design-template), the
// ledger ships no instantiable entry template under .pyxis/templates; the
// coded Writer is the only way an entry comes to exist. This guards that no
// such hand-authoring guide is added later.
func TestNoHandAuthoringGuide(t *testing.T) {
	tpl := filepath.Join(repoRoot(t), ".pyxis", "templates")
	items, err := os.ReadDir(tpl)
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	for _, it := range items {
		n := strings.ToLower(it.Name())
		if strings.Contains(n, "review-feedback-entry") || strings.Contains(n, "feedback-entry") {
			t.Fatalf("found a hand-authoring guide %q under .pyxis/templates — entries are produced only by the coded Writer (DINV-6)", it.Name())
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the test working directory")
		}
		dir = parent
	}
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var names []string
	for _, it := range items {
		if !it.IsDir() && strings.HasSuffix(it.Name(), ".md") {
			names = append(names, it.Name())
		}
	}
	sort.Strings(names)
	return names
}
