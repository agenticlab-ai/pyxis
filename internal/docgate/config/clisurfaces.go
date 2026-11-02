package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetCLISurfaces rewrites knobs.cli_surfaces in the repo config surface — the
// targeted edit `pyxis surfaces` performs when an operator wires an agent CLI
// installed after init (the auto sentinel is resolved once, at init, and a
// machine that gains a harness later has no other way to say so).
//
// Targeted, not re-rendered: the config is repo-owned, so an operator's paths,
// comments and spacing must survive a change to one knob. Every other byte is
// preserved, exactly as the epoch edits do it.
//
// The result is re-parsed closed before it lands, so an edit that would break
// the schema fails here rather than leaving a config the engine cannot read.
// An empty set writes an empty flow list, which is how a repo says "wire
// nothing" without the key going missing.
func SetCLISurfaces(root string, surfaces []string) error {
	path := filepath.Join(root, relPath)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config: %s does not exist — this is not an initialized repository; run `pyxis init` first", relPath)
		}
		return fmt.Errorf("config: read %s: %w", relPath, err)
	}
	updated := withCLISurfaces(b, surfaces)
	if _, defects := Parse(updated); len(defects) > 0 {
		return fmt.Errorf("config: writing knobs.cli_surfaces would break the closed schema (%s: %s) — refusing", relPath, defects[0].Reason)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", relPath, err)
	}
	return nil
}

// withCLISurfaces returns b with knobs.cli_surfaces set to surfaces,
// preserving every other byte. It replaces the key inside the knobs block when
// present, inserts it there when the block exists without it, and appends a
// knobs block when there is none. The block ends at the first non-indented,
// non-blank line — the same rule the epoch edits use.
func withCLISurfaces(b []byte, surfaces []string) []byte {
	line := "  cli_surfaces: [" + strings.Join(surfaces, ", ") + "]"
	lines := strings.Split(string(b), "\n")

	knobsIdx := -1
	for i, l := range lines {
		if strings.TrimRight(l, " \t") == "knobs:" {
			knobsIdx = i
			break
		}
	}
	if knobsIdx < 0 {
		return []byte(strings.TrimRight(string(b), "\n") + "\nknobs:\n" + line + "\n")
	}
	for i := knobsIdx + 1; i < len(lines); i++ {
		l := lines[i]
		if l != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			break // left the knobs block without finding the key
		}
		if strings.HasPrefix(strings.TrimSpace(l), "cli_surfaces:") {
			lines[i] = line
			return []byte(strings.Join(lines, "\n"))
		}
	}
	out := append([]string{}, lines[:knobsIdx+1]...)
	out = append(out, line)
	out = append(out, lines[knobsIdx+1:]...)
	return []byte(strings.Join(out, "\n"))
}
