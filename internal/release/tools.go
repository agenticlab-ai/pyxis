package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The shipped-set rule — THE one place it lives (AP-27: the binary list is
// derived by scan, never hand-kept; the system-release workflow shells
// `pyxis-release tools` instead of keeping its own list):
//
//	Every directory under cmd/ is a shipped engine tool, EXCEPT
//	  - BuildOnlyTool (cmd/pyxis-release) — the release build tool itself:
//	    it exists to produce releases, an operand never executes it, and
//	    shipping it would put the packer inside its own payload's trust
//	    root for no consumer;
//	  - dot/underscore-prefixed directories and testdata (the Go toolchain
//	    ignores them, so they can never be buildable tools).
//
// Growing the shipped set is therefore exactly `mkdir cmd/<tool>` — the
// scan, the matrix, and the manifest pick it up with zero list edits.
const BuildOnlyTool = "pyxis-release"

// ShippedTools derives the shipped tool list from repoRoot's command tree,
// sorted. An empty result is a named defect: a system release with zero
// binaries is not a release.
func ShippedTools(repoRoot string) ([]string, error) {
	cmdDir := filepath.Join(repoRoot, "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return nil, fmt.Errorf("release: scan command tree %s: %w", cmdDir, err)
	}
	var tools []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == BuildOnlyTool || name == "testdata" ||
			strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		tools = append(tools, name)
	}
	if len(tools) == 0 {
		return nil, fmt.Errorf("release: command tree %s scans to zero shipped tools — a system release ships every cmd/* except %s (AP-27)", cmdDir, BuildOnlyTool)
	}
	sort.Strings(tools)
	return tools, nil
}
