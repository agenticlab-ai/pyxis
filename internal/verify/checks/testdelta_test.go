package checks

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// --- registration -------------------------------------------------------------

func TestRegistration(t *testing.T) {
	c := registry.GetCheck("test-delta")
	if c == nil {
		t.Fatal("test-delta was not registered")
	}
	if c.Rung != "3" {
		t.Errorf("rung = %q, want 3", c.Rung)
	}
	if !c.AuthorIndependent {
		t.Error("test-delta must be author-independent")
	}
	if len(c.InputSet) == 0 {
		t.Error("test-delta must declare an input set")
	}
}

// --- TestDelta: pure floor logic ----------------------------------------------

func goTables() Tables {
	// "go" is always a registered stack, so this resolution cannot fail; a
	// panic here would mean a test-setup bug, never a condition under test.
	tables, err := TablesForStack(ecosystem.New(), "go", nil)
	if err != nil {
		panic(err)
	}
	return tables
}

func TestTablesForStack(t *testing.T) {
	tables := goTables()
	if len(tables.TestGlobs) == 0 {
		t.Fatal("expected test globs from ecosystem defaults")
	}
	if len(tables.SkipMarkers) == 0 {
		t.Fatal("expected skip markers from ecosystem defaults")
	}
	if len(tables.AssertionPatterns) == 0 {
		t.Fatal("expected assertion patterns from ecosystem defaults")
	}

	// A manifest override wins over the ecosystem default (mirrors
	// ecosystem.ResolveManifestDefaults, which this delegates to).
	custom, err := TablesForStack(ecosystem.New(), "go", []string{"custom/**/*_test.go"})
	if err != nil {
		t.Fatalf("TablesForStack(go): %v", err)
	}
	if len(custom.TestGlobs) != 1 || custom.TestGlobs[0] != "custom/**/*_test.go" {
		t.Errorf("manifest test-glob override not applied: %v", custom.TestGlobs)
	}
}

// TestTablesForStack_UnsupportedStackFailsClosed proves an unregistered stack is
// a hard error, not silently-empty tables that would disable the skip and
// assertion floors (a hollowed non-go test would otherwise pass clean).
func TestTablesForStack_UnsupportedStackFailsClosed(t *testing.T) {
	if _, err := TablesForStack(ecosystem.New(), "rust", nil); err == nil {
		t.Fatal("an unsupported stack must fail closed, got nil error")
	}
	// Even with manifest test-globs supplied, an unknown stack still errors —
	// globs alone cannot run the content floors. (ruby is unregistered; go,
	// python, and node are the supported set as of T10.)
	if _, err := TablesForStack(ecosystem.New(), "ruby", []string{"**/*_spec.rb"}); err == nil {
		t.Fatal("an unsupported stack with manifest globs must still fail closed")
	}
}

func TestTestDelta_DeletedTestFile(t *testing.T) {
	findings := TestDelta([]FileDelta{
		{Status: DeltaDeleted, Path: "pkg/foo_test.go"},
	}, goTables())

	mustFinding(t, findings, "test-deleted", "pkg/foo_test.go")
}

func TestTestDelta_DeletedNonTestFile(t *testing.T) {
	findings := TestDelta([]FileDelta{
		{Status: DeltaDeleted, Path: "pkg/foo.go"},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("deleting a non-test file must never finding: %+v", findings)
	}
}

func TestTestDelta_SkipMarkerAdded(t *testing.T) {
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tt.Skip(\"flaky\")\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	f := mustFinding(t, findings, "skip-marker-added", "pkg/foo_test.go")
	if f.Line != 4 {
		t.Errorf("skip-marker-added line = %d, want 4", f.Line)
	}
}

func TestTestDelta_SkipGuardInNewFileNotFlagged(t *testing.T) {
	// A brand-new test file has no prior version, so a legitimate skip guard in
	// it (the ubiquitous `if testing.Short() { t.Skip() }`) is original, not a
	// suppression added to an existing test. The skip floor must not fire on an
	// addition — it compares against a real pre-image or nothing at all.
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tif testing.Short() {\n\t\tt.Skip(\"slow\")\n\t}\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaAdded, Path: "pkg/foo_test.go", NewContent: next},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("a skip guard in a newly-added test must never finding: %+v", findings)
	}
}

func TestTestDelta_RenamedOutOfTestSet(t *testing.T) {
	// Renaming a test file to a non-test path removes it from `go test` just as
	// a deletion does; the audit must catch it (on the OLD path), never let the
	// rename disguise the loss.
	content := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaRenamed, Path: "pkg/foo.go", OldPath: "pkg/foo_test.go", OldContent: content},
	}, goTables())

	f := mustFinding(t, findings, "test-renamed-out", "pkg/foo_test.go")
	if f.Path != "pkg/foo_test.go" {
		t.Errorf("test-renamed-out must name the pre-rename test path, got %q", f.Path)
	}
}

func TestTestDelta_RenamedWithinTestSetUsesContentFloors(t *testing.T) {
	// A rename that stays within the test set is NOT a rename-out: it must run
	// the content floors on the new path, not fire test-renamed-out.
	content := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaRenamed, Path: "pkg/b/foo_test.go", OldPath: "pkg/a/foo_test.go", OldContent: content, NewContent: content},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("a pure rename within the test set must never finding: %+v", findings)
	}
}

func TestTestDelta_BuildIgnoreTagIsSkipMarker(t *testing.T) {
	// Adding a `//go:build ignore` constraint drops the whole file from `go
	// test` — a file-level exclusion the skip floor treats like an added skip
	// marker (table data, no check-logic change).
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	next := []byte("//go:build ignore\n\npackage pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	mustFinding(t, findings, "skip-marker-added", "pkg/foo_test.go")
}

func TestTestDelta_LegitimateBuildConstraintNotFlagged(t *testing.T) {
	// A normal platform build constraint (`//go:build linux`) is not an
	// exclusion and must not trip the skip floor.
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	next := []byte("//go:build linux\n\npackage pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("a legitimate build constraint must never finding: %+v", findings)
	}
}

func TestTestDelta_PreexistingSkipMarkerUntouched(t *testing.T) {
	// A skip marker that already existed (brownfield reality) and survives
	// an unrelated edit must NOT finding — only a net INCREASE counts.
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tt.Skip(\"platform\")\n\tassert.Equal(t, 1, 1)\n}\n")
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tt.Skip(\"platform\")\n\tassert.Equal(t, 2, 2)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("an unchanged pre-existing skip marker must never finding: %+v", findings)
	}
}

func TestTestDelta_AssertionCountDropped(t *testing.T) {
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n}\n")
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	mustFinding(t, findings, "assertion-count-drop", "pkg/foo_test.go")
}

func TestTestDelta_AssertionCountIncreased(t *testing.T) {
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("adding assertions must never finding: %+v", findings)
	}
}

func TestTestDelta_PureRenameNeverFindings(t *testing.T) {
	content := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{
			Status:     DeltaRenamed,
			Path:       "pkg/b/foo_test.go",
			OldPath:    "pkg/a/foo_test.go",
			OldContent: content,
			NewContent: content,
		},
	}, goTables())

	if len(findings) != 0 {
		t.Fatalf("a pure rename/move must never finding: %+v", findings)
	}
}

func TestTestDelta_RenameDoesNotExemptContentFloors(t *testing.T) {
	// A rename is not a blank check: hollowing a test out WHILE moving it
	// must still finding, on the new path, never as a deletion.
	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n}\n")
	next := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")

	findings := TestDelta([]FileDelta{
		{
			Status:     DeltaRenamed,
			Path:       "pkg/b/foo_test.go",
			OldPath:    "pkg/a/foo_test.go",
			OldContent: old,
			NewContent: next,
		},
	}, goTables())

	f := mustFinding(t, findings, "assertion-count-drop", "pkg/b/foo_test.go")
	if f.Path == "pkg/a/foo_test.go" {
		t.Error("finding must name the NEW path, never the pre-rename path")
	}
}

func TestTestDelta_MultipleFindingsSortedDeterministically(t *testing.T) {
	changes := []FileDelta{
		{Status: DeltaDeleted, Path: "pkg/z_test.go"},
		{Status: DeltaDeleted, Path: "pkg/a_test.go"},
	}
	a := TestDelta(changes, goTables())
	// Reverse input order; output must be identical (sorted by path).
	changes[0], changes[1] = changes[1], changes[0]
	b := TestDelta(changes, goTables())

	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("expected 2 findings each, got %d and %d", len(a), len(b))
	}
	if a[0].Path != "pkg/a_test.go" || a[1].Path != "pkg/z_test.go" {
		t.Errorf("findings not sorted by path: %+v", a)
	}
	if a[0].Path != b[0].Path || a[1].Path != b[1].Path {
		t.Errorf("finding order depends on input order: %+v vs %+v", a, b)
	}
}

// TestTestDelta_PatternTableAdditionIsDataOnly proves the plan's acceptance
// criterion directly: a new pattern reaches the floor purely by widening the
// Tables value passed in — zero changes to TestDelta or any other line of
// testdelta.go.
func TestTestDelta_PatternTableAdditionIsDataOnly(t *testing.T) {
	tables := goTables()
	tables.SkipMarkers = append(append([]*regexp.Regexp{}, tables.SkipMarkers...),
		regexp.MustCompile(`\bpytest\.mark\.skip\b`)) // a pattern the shipped go table does not carry

	old := []byte("package pkg\n\nfunc TestFoo(t *testing.T) {}\n")
	next := []byte("package pkg\n\n// pytest.mark.skip\nfunc TestFoo(t *testing.T) {}\n")

	findings := TestDelta([]FileDelta{
		{Status: DeltaModified, Path: "pkg/foo_test.go", OldContent: old, NewContent: next},
	}, tables)

	mustFinding(t, findings, "skip-marker-added", "pkg/foo_test.go")
}

func mustFinding(t *testing.T, findings []schema.Finding, class, path string) schema.Finding {
	t.Helper()
	for _, f := range findings {
		if f.Class == class && f.Path == path {
			return f
		}
	}
	t.Fatalf("expected a %q finding for %q, got: %+v", class, path, findings)
	return schema.Finding{}
}

// --- glob matching --------------------------------------------------------

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/*_test.go", "pkg/foo_test.go", true},
		{"**/*_test.go", "foo_test.go", true}, // root-level file, no leading directory
		{"**/*_test.go", "pkg/foo.go", false},
		{"**/*_test.go", "pkg/deep/nested/foo_test.go", true},
		{"**/testdata/**", "internal/verify/testdata/fixture/foo.go", true},
		{"**/testdata/**", "internal/verify/other/foo.go", false},
		// Non-ASCII path components must match literally (byte-preserving glob
		// translation), not be widened into mojibake that matches nothing.
		{"**/café/**", "src/café/foo_test.go", true},
		{"**/*_test.go", "pkg/café_test.go", true},
	}
	for _, c := range cases {
		res := compileGlobs([]string{c.pattern})
		if got := anyGlobMatch(res, c.path); got != c.want {
			t.Errorf("glob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

// --- ExtractFileDeltas: real git integration -----------------------------

var goGlobs = []string{"**/*_test.go"}

func TestExtractFileDeltas_DeletedTest(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	base := commitAll(t, dir, "base")

	mustRemove(t, dir, "pkg/foo_test.go")
	head := commitAll(t, dir, "delete test")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 1 || deltas[0].Status != DeltaDeleted || deltas[0].Path != "pkg/foo_test.go" {
		t.Fatalf("unexpected deltas: %+v", deltas)
	}

	mustFinding(t, TestDelta(deltas, goTables()), "test-deleted", "pkg/foo_test.go")
}

func TestExtractFileDeltas_SkipMarkerAdded(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	base := commitAll(t, dir, "base")

	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tt.Skip(\"flaky\")\n\tassert.Equal(t, 1, 1)\n}\n")
	head := commitAll(t, dir, "silence via skip")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 1 || deltas[0].Status != DeltaModified {
		t.Fatalf("unexpected deltas: %+v", deltas)
	}

	mustFinding(t, TestDelta(deltas, goTables()), "skip-marker-added", "pkg/foo_test.go")
}

func TestExtractFileDeltas_AssertionRemoved(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n}\n")
	base := commitAll(t, dir, "base")

	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	head := commitAll(t, dir, "hollow out the test")

	deltas := extract(t, dir, base, head, goGlobs)
	mustFinding(t, TestDelta(deltas, goTables()), "assertion-count-drop", "pkg/foo_test.go")
}

func TestExtractFileDeltas_RenameMoveRefactorPasses(t *testing.T) {
	dir := newGitRepo(t)
	content := "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n\tassert.Equal(t, 3, 3)\n}\n"
	writeFile(t, dir, "pkg/a/foo_test.go", content)
	writeFile(t, dir, "pkg/a/foo.go", "package pkg\n\nfunc Foo() int { return 1 }\n")
	base := commitAll(t, dir, "base")

	mustMkdirAll(t, filepath.Join(dir, "pkg/b"))
	gitOK(t, dir, "mv", "pkg/a/foo_test.go", "pkg/b/foo_test.go")
	head := commitAll(t, dir, "move package a -> b (pure refactor)")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 1 || deltas[0].Status != DeltaRenamed {
		t.Fatalf("expected a single rename delta, got: %+v", deltas)
	}

	findings := TestDelta(deltas, goTables())
	if len(findings) != 0 {
		t.Fatalf("a pure move/rename fixture must produce zero test-delta findings: %+v", findings)
	}
}

func TestExtractFileDeltas_RenameWithHollowingStillFindings(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/a/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n\tassert.Equal(t, 2, 2)\n\tassert.Equal(t, 3, 3)\n}\n")
	base := commitAll(t, dir, "base")

	mustMkdirAll(t, filepath.Join(dir, "pkg/b"))
	gitOK(t, dir, "mv", "pkg/a/foo_test.go", "pkg/b/foo_test.go")
	writeFile(t, dir, "pkg/b/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	head := commitAll(t, dir, "move AND hollow out in one commit")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 1 || deltas[0].Status != DeltaRenamed {
		t.Fatalf("expected a single rename delta (git must still detect the rename above its similarity floor), got: %+v", deltas)
	}

	mustFinding(t, TestDelta(deltas, goTables()), "assertion-count-drop", "pkg/b/foo_test.go")
}

func TestExtractFileDeltas_RenameOutOfGlobIsCaught(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	writeFile(t, dir, "pkg/keep.go", "package pkg\n")
	base := commitAll(t, dir, "base")

	// git mv to a NON-test path: git records a 100% rename, whose new path no
	// longer matches the test glob — the evasion the deletion floor alone misses.
	gitOK(t, dir, "mv", "pkg/foo_test.go", "pkg/foo.go")
	head := commitAll(t, dir, "disguise a test deletion as a rename")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 1 || deltas[0].Status != DeltaRenamed {
		t.Fatalf("expected a single rename delta, got: %+v", deltas)
	}
	mustFinding(t, TestDelta(deltas, goTables()), "test-renamed-out", "pkg/foo_test.go")
}

func TestExtractFileDeltas_NonTestChangeNeverFindings(t *testing.T) {
	dir := newGitRepo(t)
	writeFile(t, dir, "pkg/foo.go", "package pkg\n\nfunc Foo() int { return 1 }\n")
	writeFile(t, dir, "pkg/foo_test.go", "package pkg\n\nfunc TestFoo(t *testing.T) {\n\tassert.Equal(t, 1, 1)\n}\n")
	base := commitAll(t, dir, "base")

	mustRemove(t, dir, "pkg/foo.go")
	writeFile(t, dir, "pkg/foo.go", "package pkg\n\nfunc Foo() int { return 2 }\n")
	head := commitAll(t, dir, "unrelated source change")

	deltas := extract(t, dir, base, head, goGlobs)
	if len(deltas) != 0 {
		t.Fatalf("a diff touching no test-glob path must extract zero deltas: %+v", deltas)
	}
	findings := TestDelta(deltas, goTables())
	if len(findings) != 0 {
		t.Fatalf("a non-test change must never finding: %+v", findings)
	}
}

// --- test helpers ----------------------------------------------------------

func extract(t *testing.T, dir, base, head string, globs []string) []FileDelta {
	t.Helper()
	deltas, err := ExtractFileDeltas(context.Background(), dir, base, head, globs)
	if err != nil {
		t.Fatalf("ExtractFileDeltas: %v", err)
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].Path < deltas[j].Path })
	return deltas
}

// newGitRepo builds a hermetic git repo at a fresh t.TempDir() — using `git
// -C` (never os.Chdir, which races the shared process cwd) and an isolated
// git identity/config so the host environment cannot leak in. Mirrors
// invoke_test.go's initGitRepo.
func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "codegate-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "codegate-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitOK(t, dir, "init", "-q", "-b", "main")
	return dir
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustRemove(t *testing.T, dir, relPath string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, relPath)); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, msg string) string {
	t.Helper()
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "-m", msg)
	return gitOK(t, dir, "rev-parse", "HEAD")
}
