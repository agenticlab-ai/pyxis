// Package termstyle is the terminal styling seam (#948): ANSI escapes when
// the writer is a live terminal and NO_COLOR is unset, byte-identical plain
// text otherwise. Styling only ever WRAPS the text it is given — the words
// are the same on a TTY and in a pipe — so tests, journals, and machine
// consumers keep matching the exact strings the human sees.
package termstyle

import (
	"io"
	"os"
)

// Style renders text for one writer. The zero value is plain (no escapes).
type Style struct{ enabled bool }

// Detect probes a writer: styling is on only for a character-device *os.File
// with NO_COLOR unset (https://no-color.org — presence disables, any value).
func Detect(w io.Writer) Style {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return Style{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return Style{}
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return Style{}
	}
	return Style{enabled: true}
}

// Forced returns a Style with an explicit enablement — the test seam, and
// the escape hatch for callers that already know their terminal.
func Forced(enabled bool) Style { return Style{enabled: enabled} }

// Enabled reports whether this style emits escapes.
func (s Style) Enabled() bool { return s.enabled }

func (s Style) wrap(code, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// Bold — headings and the words that carry the sentence.
func (s Style) Bold(text string) string { return s.wrap("1", text) }

// Dim — structure and pointers (rules, detail lines) that should recede.
func (s Style) Dim(text string) string { return s.wrap("2", text) }

// Green — the color of a converged, gated, healthy state.
func (s Style) Green(text string) string { return s.wrap("32", text) }

// Cyan — keys and commands the eye should be able to jump to.
func (s Style) Cyan(text string) string { return s.wrap("36", text) }

// Yellow — the pending and the waiting-on-you.
func (s Style) Yellow(text string) string { return s.wrap("33", text) }
