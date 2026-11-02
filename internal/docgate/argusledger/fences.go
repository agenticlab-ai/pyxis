package argusledger

import (
	"iter"
	"strings"
)

// ProseLines yields original line indexes and content outside Markdown code
// fences. Backtick and tilde fences close only on the same marker with at least
// the opening length and no trailing text. An unclosed fence extends to EOF.
// Callers retain the original bytes; this only selects live structural lines.
func ProseLines(b []byte) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		var marker byte
		var width int
		i := 0
		for line := range strings.SplitSeq(strings.TrimSuffix(string(b), "\n"), "\n") {
			m, n, tail := fencePrefix(line)
			if marker != 0 {
				if m == marker && n >= width && strings.Trim(tail, " \t") == "" {
					marker = 0
				}
			} else if n >= 3 && (m == '~' || !strings.ContainsRune(tail, '`')) {
				marker, width = m, n
			} else if !yield(i, line) {
				return
			}
			i++
		}
	}
}

func fencePrefix(line string) (byte, int, string) {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 || i == len(line) || (line[i] != '`' && line[i] != '~') {
		return 0, 0, ""
	}
	start, marker := i, line[i]
	for i < len(line) && line[i] == marker {
		i++
	}
	return marker, i - start, line[i:]
}
