// Package entity holds closed domain values shared across the factory.
package entity

import (
	"errors"
	"fmt"
	"strings"
)

// Harness is one agent harness the factory can run.
// The closed set is claude, codex, cursor, and opencode.
type Harness string

const (
	Claude   Harness = "claude"
	Codex    Harness = "codex"
	Cursor   Harness = "cursor"
	OpenCode Harness = "opencode"
)

// ErrUnknownHarness is returned when a name is outside the closed set.
var ErrUnknownHarness = errors.New("unknown harness")

// String returns the harness name.
func (h Harness) String() string { return string(h) }

// Known reports whether h is one of the four harnesses.
func (h Harness) Known() bool {
	switch h {
	case Claude, Codex, Cursor, OpenCode:
		return true
	default:
		return false
	}
}

// ParseHarness accepts one closed harness name.
// Surrounding space is ignored. Any other value is ErrUnknownHarness.
func ParseHarness(name string) (Harness, error) {
	h := Harness(strings.TrimSpace(name))
	if !h.Known() {
		return "", fmt.Errorf("%w: %q", ErrUnknownHarness, name)
	}
	return h, nil
}

// Harnesses returns the closed set in canonical order.
func Harnesses() []Harness {
	return []Harness{Claude, Codex, Cursor, OpenCode}
}
