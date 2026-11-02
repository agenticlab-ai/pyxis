// Package changesize measures how much of a change a reviewer has to read: the
// lines added and removed in production code and agent prompts, leaving out
// tests, comments, blank lines, generated and vendored files, and documents.
// It is the "Lines of code" figure of the pull request description template
// (.pyxis/templates/pr-description-template.md). The executor, the
// `pyxis pr-description` command and the pr-description check all compute it
// here, so the stated and checked figures cannot drift apart.
package changesize

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Class names what a counted file holds.
type Class string

const (
	// Code is production source, scripts and configuration.
	Code Class = "code"
	// Prompt is an instruction file an agent loads: AGENTS.md and the
	// authoring guides, skills and templates under .pyxis/.
	Prompt Class = "prompt"
)

// NotApplicable is the figure for a change that touches no code or prompt
// file, such as one that only edits a PRD or a technical design.
const NotApplicable = "n/a (no code or prompt changes)"

// File is one counted file's share of a change.
type File struct {
	Path    string
	Class   Class
	Added   int
	Removed int
}

// Size is the reviewable size of a change. A change with no counted files has
// no line count.
type Size struct {
	Files []File
}

// Added is the number of counted lines the change adds.
func (s Size) Added() int {
	n := 0
	for _, f := range s.Files {
		n += f.Added
	}
	return n
}

// Removed is the number of counted lines the change removes.
func (s Size) Removed() int {
	n := 0
	for _, f := range s.Files {
		n += f.Removed
	}
	return n
}

// Total is the line count: counted lines added plus counted lines removed.
func (s Size) Total() int { return s.Added() + s.Removed() }

// Applies reports whether the change touches a code or prompt file at all.
// A change whose counted files changed only comments or blank lines still
// applies, with a count of 0.
func (s Size) Applies() bool { return len(s.Files) > 0 }

// String renders the figure as the template states it: "139 (+101/-38)", or
// NotApplicable.
func (s Size) String() string {
	if !s.Applies() {
		return NotApplicable
	}
	return fmt.Sprintf("%d (+%d/-%d)", s.Total(), s.Added(), s.Removed())
}

// Measure sizes the change from base to head as a pull request shows it:
// against their merge base, with renames detected. dir is any directory in
// the repository; base and head are revisions.
func Measure(ctx context.Context, dir, base, head string) (Size, error) {
	for _, rev := range []string{base, head} {
		if rev == "" || strings.HasPrefix(rev, "-") || strings.ContainsAny(rev, " \t\r\n\x00") {
			return Size{}, fmt.Errorf("changesize: revision %q is not a plain revision", rev)
		}
	}
	raw, err := git(ctx, dir, "diff", "--raw", "-z", "--no-abbrev", "--find-renames", "--no-relative", base+"..."+head, "--")
	if err != nil {
		return Size{}, fmt.Errorf("changesize: list changed files %s...%s: %w", base, head, err)
	}
	entries, err := parseRaw(raw)
	if err != nil {
		return Size{}, err
	}
	var size Size
	for _, e := range entries {
		f, counted, err := measureEntry(ctx, dir, e)
		if err != nil {
			return Size{}, err
		}
		if counted {
			size.Files = append(size.Files, f)
		}
	}
	sort.Slice(size.Files, func(i, j int) bool { return size.Files[i].Path < size.Files[j].Path })
	return size, nil
}

// rawEntry is one record of `git diff --raw -z`.
type rawEntry struct {
	oldMode, newMode string
	oldSHA, newSHA   string
	status           byte
	oldPath, newPath string
}

func parseRaw(out []byte) ([]rawEntry, error) {
	out = bytes.TrimSuffix(out, []byte{0})
	if len(out) == 0 {
		return nil, nil
	}
	fields := bytes.Split(out, []byte{0})
	var entries []rawEntry
	for i := 0; i < len(fields); {
		meta := string(fields[i])
		i++
		parts := strings.Fields(strings.TrimPrefix(meta, ":"))
		if !strings.HasPrefix(meta, ":") || len(parts) != 5 || parts[4] == "" {
			return nil, fmt.Errorf("changesize: unexpected diff record %q", meta)
		}
		e := rawEntry{oldMode: parts[0], newMode: parts[1], oldSHA: parts[2], newSHA: parts[3], status: parts[4][0]}
		paths := 1
		if e.status == 'R' || e.status == 'C' {
			paths = 2
		}
		if i+paths > len(fields) {
			return nil, fmt.Errorf("changesize: diff record %q lacks its path", meta)
		}
		e.oldPath, e.newPath = string(fields[i]), string(fields[i+paths-1])
		i += paths
		entries = append(entries, e)
	}
	return entries, nil
}

// side is one version of a changed file.
type side struct {
	sha     string
	path    string
	content []byte
	class   Class
	syntax  syntax
	counted bool
}

func measureEntry(ctx context.Context, dir string, e rawEntry) (File, bool, error) {
	before, err := loadSide(ctx, dir, e.oldPath, e.oldMode, e.oldSHA)
	if err != nil {
		return File{}, false, err
	}
	after, err := loadSide(ctx, dir, e.newPath, e.newMode, e.newSHA)
	if err != nil {
		return File{}, false, err
	}
	if !before.counted && !after.counted {
		return File{}, false, nil
	}
	f := File{Path: after.path, Class: after.class}
	if !after.counted {
		f = File{Path: before.path, Class: before.class}
	}
	var removed, added [][2]int
	switch {
	case before.sha != "" && after.sha != "":
		patch, err := git(ctx, dir, "diff", "--unified=0", "--no-color", "--no-ext-diff", "--no-textconv",
			"--diff-algorithm=myers", "--indent-heuristic", before.sha, after.sha)
		if err != nil {
			return File{}, false, fmt.Errorf("changesize: diff %s: %w", f.Path, err)
		}
		if removed, added, err = parseHunks(patch); err != nil {
			return File{}, false, fmt.Errorf("changesize: diff %s: %w", f.Path, err)
		}
	case after.sha != "":
		added = [][2]int{{1, lineCount(after.content)}}
	default:
		removed = [][2]int{{1, lineCount(before.content)}}
	}
	if before.counted {
		f.Removed = countLines(codeLines(before.content, before.syntax), removed)
	}
	if after.counted {
		f.Added = countLines(codeLines(after.content, after.syntax), added)
	}
	return f, true, nil
}

// loadSide reads one version of a file. Absent versions, symlinks and
// submodules have no sha; a present text version is read so its lines can be
// told apart from its comments.
func loadSide(ctx context.Context, dir, p, mode, sha string) (side, error) {
	s := side{path: p}
	if !strings.HasPrefix(mode, "100") || strings.Trim(sha, "0") == "" {
		return s, nil // absent, a symlink, or a submodule
	}
	s.sha = sha
	if !candidate(p) {
		return s, nil
	}
	content, err := git(ctx, dir, "cat-file", "blob", sha)
	if err != nil {
		return side{}, fmt.Errorf("changesize: read %s: %w", p, err)
	}
	s.content = content
	if binary(content) || generated(content) {
		return s, nil
	}
	s.class, s.syntax, s.counted = classify(p, content)
	return s, nil
}

// excludedSegments are directory names whose files are tests, fixtures or
// third-party code wherever they appear.
var excludedSegments = map[string]bool{
	"test": true, "tests": true, "testdata": true, "fixtures": true, "__tests__": true,
	"__mocks__": true, "spec": true, "vendor": true, "node_modules": true,
}

// excludedRoots are repository trees that hold documents, records and data
// rather than behavior: requirements, designs, plans and reviews (docs/),
// memory records, historical logs, work items, and generated surfaces.
var excludedRoots = []string{"docs/", "memory/", "logs/", ".workitems/", ".pyxis/generated/", ".pyxis/logs/", ".pyxis/scratch/"}

// promptRoots hold the instruction files agents load.
var promptRoots = []string{".pyxis/authoring/", ".pyxis/skills/", ".pyxis/templates/"}

var testFile = regexp.MustCompile(`(_test\.(go|sh|py)|^test_.*\.py|\.(test|spec)\.(js|mjs|cjs|jsx|ts|tsx))$`)

// candidate is the path-only half of classification: tests, documents,
// records and third-party trees never count, whatever their content.
func candidate(p string) bool {
	for _, root := range excludedRoots {
		if strings.HasPrefix(p, root) {
			return false
		}
	}
	for d := range strings.SplitSeq(path.Dir(p), "/") {
		if excludedSegments[d] {
			return false
		}
	}
	return !testFile.MatchString(path.Base(p))
}

// classify decides how a candidate file counts. Agent prompts are the
// Markdown files under the prompt roots plus AGENTS.md and CLAUDE.md; code is
// recognized by extension, by a well-known file name, or, for a file with no
// extension, by a shebang. Everything else (Markdown documents, JSON data,
// lock files, text) does not count.
func classify(p string, content []byte) (Class, syntax, bool) {
	base := path.Base(p)
	ext := strings.ToLower(path.Ext(base))
	if base == "AGENTS.md" || base == "CLAUDE.md" {
		return Prompt, markupSyntax, true
	}
	if ext == ".md" {
		for _, root := range promptRoots {
			if strings.HasPrefix(p, root) {
				return Prompt, markupSyntax, true
			}
		}
		return "", syntax{}, false
	}
	if s, ok := codeNames[base]; ok {
		return Code, s, true
	}
	if s, ok := codeExtensions[ext]; ok {
		return Code, s, true
	}
	if ext == "" && bytes.HasPrefix(content, []byte("#!")) {
		return Code, hashSyntax, true
	}
	return "", syntax{}, false
}

var codeNames = map[string]syntax{
	"Makefile": hashSyntax, "GNUmakefile": hashSyntax, "Dockerfile": hashSyntax,
	"go.mod": modSyntax, "go.work": modSyntax,
}

var codeExtensions = map[string]syntax{
	".go": goSyntax,
	".sh": hashSyntax, ".bash": hashSyntax, ".zsh": hashSyntax, ".py": hashSyntax, ".rb": hashSyntax,
	".yml": hashSyntax, ".yaml": hashSyntax, ".toml": hashSyntax, ".mk": hashSyntax,
	".js": slashSyntax, ".mjs": slashSyntax, ".cjs": slashSyntax, ".jsx": slashSyntax,
	".ts": slashSyntax, ".tsx": slashSyntax, ".rs": slashSyntax, ".java": slashSyntax,
	".kt": slashSyntax, ".kts": slashSyntax, ".swift": slashSyntax, ".scss": slashSyntax,
	".c": slashSyntax, ".h": slashSyntax, ".cc": slashSyntax, ".cpp": slashSyntax,
	".cxx": slashSyntax, ".hpp": slashSyntax, ".hh": slashSyntax, ".cs": slashSyntax,
	".php": phpSyntax, ".css": cssSyntax, ".sql": sqlSyntax,
	".html": markupSyntax, ".htm": markupSyntax,
}

// binary mirrors Git's own heuristic: a NUL byte in the first 8000 bytes.
func binary(content []byte) bool {
	head := content
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// generatedMarker is the standard "Code generated ... DO NOT EDIT." header
// (https://go.dev/s/generatedcode), in Go or hash comment form.
var (
	generatedMarker = regexp.MustCompile(`(?m)^(//|#) Code generated .* DO NOT EDIT\.$`)
	packageClause   = regexp.MustCompile(`(?m)^package `)
)

// generated reports a generated-file header: before a Go package clause, or
// within the first 2 KiB of any other file.
func generated(content []byte) bool {
	head := content
	if loc := packageClause.FindIndex(head); loc != nil {
		head = head[:loc[0]]
	} else if len(head) > 2048 {
		head = head[:2048]
	}
	return generatedMarker.Match(head)
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// parseHunks reads the line ranges of a zero-context patch. Each range is
// {first line, count} in 1-based line numbers of its version.
func parseHunks(patch []byte) (removed, added [][2]int, err error) {
	for line := range strings.SplitSeq(string(patch), "\n") {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			return nil, nil, fmt.Errorf("unexpected hunk header %q", line)
		}
		removed = append(removed, hunkRange(m[1], m[2]))
		added = append(added, hunkRange(m[3], m[4]))
	}
	return removed, added, nil
}

func hunkRange(start, count string) [2]int {
	first, _ := strconv.Atoi(start) // the pattern admits digits only
	n := 1
	if count != "" {
		n, _ = strconv.Atoi(count) // the pattern admits digits only
	}
	return [2]int{first, n}
}

// countLines counts the code lines that fall inside the ranges.
func countLines(code []bool, ranges [][2]int) int {
	n := 0
	for _, r := range ranges {
		for line := r[0]; line < r[0]+r[1]; line++ {
			if line >= 1 && line <= len(code) && code[line-1] {
				n++
			}
		}
	}
	return n
}

func lineCount(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %w: %s", args[0], err, msg)
		}
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}
