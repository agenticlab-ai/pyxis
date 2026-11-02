package changesize

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// initRepo creates a hermetic Git repository with an isolated identity.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_AUTHOR_NAME", "changesize-test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "changesize-test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	gitOK(t, dir, "init", "-q", "-b", "main")
	return dir
}

func gitOK(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "--allow-empty", "-m", msg)
	return gitOK(t, dir, "rev-parse", "HEAD")
}

func measure(t *testing.T, dir, base, head string) Size {
	t.Helper()
	size, err := Measure(context.Background(), dir, base, head)
	if err != nil {
		t.Fatal(err)
	}
	return size
}

func files(s Size) map[string]File {
	out := map[string]File{}
	for _, f := range s.Files {
		out[f.Path] = f
	}
	return out
}

const goBase = `package demo

// Old explains the old function.
func Old() int {
	return 1
}
`

const goHead = `package demo

// Old explains the old function.
func Old() int {
	return 2
}

/*
New is documented in a block comment
that spans lines.
*/
func New() string {
	// a whole-line comment

	return ` + "`" + `first

third` + "`" + ` // trailing comment keeps the line
}
`

func TestMeasureCountsCodeAndPromptsOnly(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, "internal/demo/demo.go", goBase)
	write(t, dir, "internal/demo/demo_test.go", "package demo\n")
	write(t, dir, ".pyxis/authoring/guide.md", "# Guide\n\nStep one.\n")
	base := commit(t, dir, "base")

	write(t, dir, "internal/demo/demo.go", goHead)
	write(t, dir, "internal/demo/demo_test.go", "package demo\n\nfunc helper() {}\n")
	write(t, dir, "internal/demo/testdata/case.go", "package x\n")
	write(t, dir, ".pyxis/authoring/guide.md", "# Guide\n\nStep one.\n<!-- a note for maintainers -->\n\nStep two.\n")
	write(t, dir, ".pyxis/skills/demo/fixtures/case.md", "fixture prose\n")
	write(t, dir, "AGENTS.md", "Rule one.\n\nRule two.\n")
	write(t, dir, "README.md", "Documentation is not counted.\n")
	write(t, dir, "docs/tech-design/x.md", "Design prose.\n")
	write(t, dir, "memory/record.json", "{}\n")
	write(t, dir, "config.yaml", "# comment\nkey: value # trailing\n\nother: 1\n")
	write(t, dir, "data.json", "{\"a\": 1}\n")
	head := commit(t, dir, "head")

	size := measure(t, dir, base, head)
	got := files(size)
	want := map[string]File{
		"internal/demo/demo.go":     {Path: "internal/demo/demo.go", Class: Code, Added: 5, Removed: 1},
		".pyxis/authoring/guide.md": {Path: ".pyxis/authoring/guide.md", Class: Prompt, Added: 1},
		"AGENTS.md":                 {Path: "AGENTS.md", Class: Prompt, Added: 2},
		"config.yaml":               {Path: "config.yaml", Class: Code, Added: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files:\n got %+v\nwant %+v", got, want)
	}
	if size.Total() != 11 || size.Added() != 10 || size.Removed() != 1 || size.String() != "11 (+10/-1)" {
		t.Fatalf("size = %s (added %d removed %d)", size, size.Added(), size.Removed())
	}
}

func TestMeasureDocumentsOnlyIsNotApplicable(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, "docs/prds/x/000-base.md", "# PRD\n")
	base := commit(t, dir, "base")
	write(t, dir, "docs/prds/x/000-base.md", "# PRD\n\nMore requirements.\n")
	write(t, dir, "drafts/x/prd.md", "Draft prose.\n")
	head := commit(t, dir, "head")

	size := measure(t, dir, base, head)
	if size.Applies() || size.String() != NotApplicable || size.Total() != 0 {
		t.Fatalf("size = %+v, %q", size, size.String())
	}
}

func TestMeasureCommentOnlyCodeChangeCountsZero(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	base := commit(t, dir, "base")
	write(t, dir, "main.go", "package main\n\n// main does nothing.\nfunc main() {}\n")
	head := commit(t, dir, "head")

	size := measure(t, dir, base, head)
	if !size.Applies() || size.String() != "0 (+0/-0)" {
		t.Fatalf("size = %q", size.String())
	}
}

func TestMeasureAgainstMergeBase(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, "a.go", "package a\n")
	gitOK(t, dir, "add", "-A")
	gitOK(t, dir, "commit", "-q", "-m", "root")
	gitOK(t, dir, "switch", "-q", "-c", "feature")
	write(t, dir, "b.go", "package a\n\nvar B = 1\n")
	head := commit(t, dir, "feature")
	gitOK(t, dir, "switch", "-q", "main")
	write(t, dir, "c.go", "package a\n\nvar C = 1\nvar D = 2\n")
	commit(t, dir, "mainline moves on")

	size := measure(t, dir, "main", head)
	if got := files(size); len(got) != 1 || got["b.go"].Added != 2 {
		t.Fatalf("files = %+v; mainline-only changes must not count", got)
	}
}

func TestMeasureRenamesDeletesAndSpecialFiles(t *testing.T) {
	dir := initRepo(t)
	write(t, dir, "old/name.go", "package old\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\nfunc D() {}\n")
	write(t, dir, "gone.sh", "#!/bin/sh\n# remove me\necho gone\n")
	write(t, dir, "image.bin", "\x00\x01\x02")
	base := commit(t, dir, "base")

	if err := os.Remove(filepath.Join(dir, "old/name.go")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "new/name.go", "package old\n\nfunc A() {}\nfunc B() {}\nfunc C() {}\nfunc E() {}\n")
	gitOK(t, dir, "rm", "-q", "gone.sh")
	write(t, dir, "image.bin", "\x00\x03\x04")
	write(t, dir, "gen.go", "// Code generated by tool. DO NOT EDIT.\n\npackage gen\n\nvar X = 1\n")
	write(t, dir, ".pyxis/hooks/pre-push", "#!/bin/sh\n# delegate\nexec true\n")
	write(t, dir, "notes", "plain text without a shebang\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	head := commit(t, dir, "head")

	got := files(measure(t, dir, base, head))
	want := map[string]File{
		"new/name.go":           {Path: "new/name.go", Class: Code, Added: 1, Removed: 1},
		"gone.sh":               {Path: "gone.sh", Class: Code, Removed: 1},
		".pyxis/hooks/pre-push": {Path: ".pyxis/hooks/pre-push", Class: Code, Added: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("files:\n got %+v\nwant %+v", got, want)
	}
}

func TestMeasureRejectsUnsafeRevisions(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "root")
	for _, rev := range []string{"", "-p", "a b", "main\n"} {
		if _, err := Measure(context.Background(), dir, rev, "HEAD"); err == nil {
			t.Errorf("Measure accepted base %q", rev)
		}
	}
	if _, err := Measure(context.Background(), dir, "HEAD", "no-such-ref"); err == nil || !strings.Contains(err.Error(), "list changed files") {
		t.Fatalf("unknown revision: %v", err)
	}
}

func TestCodeLinesBySyntax(t *testing.T) {
	cases := []struct {
		name    string
		content string
		syntax  syntax
		want    []bool
	}{
		{"hash", "#!/bin/sh\n# note\necho hi # trailing\n\n", hashSyntax, []bool{false, false, true, false}},
		{"slash block", "/* a\n b */ x = 1;\ny = 2; /* open\nstill */\n// c\nz // see /* not a block\nw\n", slashSyntax, []bool{false, true, true, false, false, true, true}},
		{"markup", "<!-- a -->\ntext <!-- b\nc -->\n<!-- d --> e\n", markupSyntax, []bool{false, true, false, true}},
		{"sql", "-- c\nselect 1; -- trailing\n", sqlSyntax, []bool{false, true}},
		{"css", "/* c */\na { b: c }\n", cssSyntax, []bool{false, true}},
		{"php", "# c\n// d\n$a = 1;\n", phpSyntax, []bool{false, false, true}},
		{"no final newline", "x\ny", hashSyntax, []bool{true, true}},
		{"empty", "", hashSyntax, []bool{}},
		{"go", "package p\n\n// c\nvar s = `a\n\nb`\n/* x\n*/\n", goSyntax, []bool{true, false, false, true, false, true, false, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codeLines([]byte(c.content), c.syntax); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("codeLines = %v, want %v", got, c.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		path    string
		content string
		class   Class
		counted bool
	}{
		{"cmd/x/main.go", "", Code, true},
		{"internal/x/x_test.go", "", "", false},
		{"web/app.test.ts", "", "", false},
		{"tools/test_tool.py", "", "", false},
		{"pkg/spec/run.rb", "", "", false},
		{"third/vendor/lib.go", "", "", false},
		{".pyxis/vendor/mermaid/mermaid.js", "", "", false},
		{".pyxis/scripts/tests/run.sh", "", "", false},
		{".pyxis/skills/argus/SKILL.md", "", Prompt, true},
		{".pyxis/templates/prd-template.md", "", Prompt, true},
		{"nested/AGENTS.md", "", Prompt, true},
		{".pyxis/pipeline.md", "", "", false},
		{"Makefile", "", Code, true},
		{"go.mod", "", Code, true},
		{".github/workflows/ci.yml", "", Code, true},
		{"go.sum", "", "", false},
		{"bin/tool", "#!/usr/bin/env bash\n", Code, true},
		{"LICENSE", "MIT\n", "", false},
	}
	for _, c := range cases {
		var class Class
		counted := candidate(c.path)
		if counted {
			class, _, counted = classify(c.path, []byte(c.content))
		}
		if counted != c.counted || (counted && class != c.class) {
			t.Errorf("%s: counted=%v class=%q, want counted=%v class=%q", c.path, counted, class, c.counted, c.class)
		}
	}
}

func TestParseRawAndHunksRejectMalformedInput(t *testing.T) {
	for _, raw := range []string{"M\x00path\x00", ":100644 100644 a b\x00p\x00", ":100644 100644 a b R100\x00only-old\x00"} {
		if _, err := parseRaw([]byte(raw)); err == nil {
			t.Errorf("parseRaw(%q) accepted malformed input", raw)
		}
	}
	if _, _, err := parseHunks([]byte("@@ broken @@\n")); err == nil {
		t.Fatal("parseHunks accepted a malformed header")
	}
	removed, added, err := parseHunks([]byte("@@ -3 +4,0 @@\n@@ -9,0 +10,2 @@\n"))
	if err != nil || !reflect.DeepEqual(removed, [][2]int{{3, 1}, {9, 0}}) || !reflect.DeepEqual(added, [][2]int{{4, 0}, {10, 2}}) {
		t.Fatalf("parseHunks = %v %v %v", removed, added, err)
	}
}

func TestGeneratedAndBinaryDetection(t *testing.T) {
	long := strings.Repeat("x", 9000) + "\x00"
	if binary([]byte(long)) {
		t.Fatal("a NUL after the first 8000 bytes must not mark a file binary")
	}
	if !generated([]byte("# Code generated by gen. DO NOT EDIT.\nkey: 1\n")) {
		t.Fatal("hash-form generated marker missed")
	}
	if generated([]byte("package p\n\n// Code generated by gen. DO NOT EDIT.\n")) {
		t.Fatal("a marker after the package clause is not a file header")
	}
	if generated([]byte(strings.Repeat("y", 3000) + "\n// Code generated by gen. DO NOT EDIT.\n")) {
		t.Fatal("a marker past the header window is not a file header")
	}
}

func TestGitReportsStderr(t *testing.T) {
	dir := t.TempDir()
	if _, err := git(context.Background(), dir, "rev-parse", "HEAD"); err == nil || !strings.Contains(err.Error(), "git rev-parse") {
		t.Fatalf("git outside a repository: %v", err)
	}
	write(t, dir, "a", "one\n")
	write(t, dir, "b", "two\n")
	if _, err := git(context.Background(), dir, "diff", "--no-index", "--quiet", "a", "b"); err == nil || err.Error() != "git diff: exit status 1" {
		t.Fatalf("a silent git failure: %v", err)
	}
}

func TestLineCount(t *testing.T) {
	for content, want := range map[string]int{"": 0, "a": 1, "a\n": 1, "a\nb": 2, "a\nb\n": 2, "\n": 1} {
		if got := lineCount([]byte(content)); got != want {
			t.Errorf("lineCount(%q) = %d, want %d", content, got, want)
		}
	}
}
