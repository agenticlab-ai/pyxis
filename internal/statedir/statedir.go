// Package statedir resolves the machine's per-user pyxis state directory —
// the single authority for where machine-local, machine-shared pyxis state
// lives. State, not config: run journals and rate-limit ledgers are local
// evidence about this machine, never settings.
//
// One authority matters here rather than being tidy (AP-11): the write-budget
// ledgers must be shared by every pyxis process on the machine, and two
// functions computing "the state dir" independently is how one process ends
// up metering against a ledger another never reads.
package statedir

import (
	"fmt"
	"os"
	"path/filepath"
)

// Root is ${XDG_STATE_HOME:-$HOME/.local/state}/pyxis.
func Root() (string, error) {
	if s := os.Getenv("XDG_STATE_HOME"); s != "" {
		if !filepath.IsAbs(s) {
			return "", fmt.Errorf("resolve the pyxis state dir: XDG_STATE_HOME must be absolute, got %q", s)
		}
		return filepath.Join(s, "pyxis"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve the pyxis state dir: %w", err)
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("resolve the pyxis state dir: home directory must be absolute, got %q", home)
	}
	return filepath.Join(home, ".local", "state", "pyxis"), nil
}

// Sub is Root joined with elem — the form callers that own a named area use.
func Sub(elem ...string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{root}, elem...)...), nil
}
