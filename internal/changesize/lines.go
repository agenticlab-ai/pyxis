package changesize

import (
	"go/scanner"
	"go/token"
	"strings"
)

// syntax is how a language marks comments. Go is scanned with its own
// tokenizer; the other languages use a line scanner that recognizes comment
// markers but not string literals, so a marker inside a string can misjudge a
// line. Only whole-line comments are excluded: a line with code and a
// trailing comment counts.
type syntax struct {
	goSource   bool
	line       []string
	blockStart string
	blockEnd   string
}

var (
	goSyntax     = syntax{goSource: true}
	modSyntax    = syntax{line: []string{"//"}}
	hashSyntax   = syntax{line: []string{"#"}}
	slashSyntax  = syntax{line: []string{"//"}, blockStart: "/*", blockEnd: "*/"}
	phpSyntax    = syntax{line: []string{"//", "#"}, blockStart: "/*", blockEnd: "*/"}
	cssSyntax    = syntax{blockStart: "/*", blockEnd: "*/"}
	sqlSyntax    = syntax{line: []string{"--"}, blockStart: "/*", blockEnd: "*/"}
	markupSyntax = syntax{blockStart: "<!--", blockEnd: "-->"}
)

// codeLines reports, for each line of content (index 0 is line 1), whether the
// line holds anything besides whitespace and comments.
func codeLines(content []byte, s syntax) []bool {
	lines := strings.Split(string(content), "\n")
	if len(content) > 0 && content[len(content)-1] == '\n' {
		lines = lines[:len(lines)-1]
	}
	if len(content) == 0 {
		lines = nil
	}
	if s.goSource {
		return goCodeLines(content, lines)
	}
	code := make([]bool, len(lines))
	inBlock := false
	for i, line := range lines {
		for j := 0; j < len(line); {
			rest := line[j:]
			if inBlock {
				end := strings.Index(rest, s.blockEnd)
				if end < 0 {
					break
				}
				inBlock = false
				j += end + len(s.blockEnd)
				continue
			}
			if c := rest[0]; c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' {
				j++
				continue
			}
			if s.lineCommentAt(rest) == 0 {
				break
			}
			if s.blockStart != "" && strings.HasPrefix(rest, s.blockStart) {
				inBlock = true
				j += len(s.blockStart)
				continue
			}
			code[i] = true
			// Keep scanning only for a block comment that opens later on this
			// line and is not itself inside a trailing line comment.
			if s.blockStart == "" {
				break
			}
			open := strings.Index(rest, s.blockStart)
			if open < 0 {
				break
			}
			if lc := s.lineCommentAt(rest); lc >= 0 && lc < open {
				break
			}
			j += open
		}
	}
	return code
}

// lineCommentAt is the earliest index of a line-comment marker in s, or -1.
func (s syntax) lineCommentAt(text string) int {
	first := -1
	for _, marker := range s.line {
		if i := strings.Index(text, marker); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	return first
}

// goCodeLines marks the lines a Go token other than a comment touches. A token
// spanning lines (a raw string) marks each of its non-blank lines.
func goCodeLines(content []byte, lines []string) []bool {
	code := make([]bool, len(lines))
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(content))
	var sc scanner.Scanner
	sc.Init(file, content, nil, scanner.ScanComments) // errors are tolerated: a broken file is still counted
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT || (tok == token.SEMICOLON && lit == "\n") {
			continue // comments, and the semicolons Go inserts at line ends
		}
		first := file.Line(pos)
		last := first + strings.Count(lit, "\n")
		for line := first; line <= last && line <= len(lines); line++ {
			if strings.TrimSpace(lines[line-1]) != "" {
				code[line-1] = true
			}
		}
	}
	return code
}
