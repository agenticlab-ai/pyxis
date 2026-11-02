package runner

import (
	"os"
	"path/filepath"
	"testing"
)

// WriteAllowedRoots always lists the workspace first, includes the toolchain's
// EXISTING scratch/cache dirs, and — critically — drops a non-existent one. A
// missing root would fail the whole Landlock ruleset closed (restrictSelfToPaths
// errors on a directory it cannot open), reding every verb rather than the one
// stray write; the filter is what keeps a relocated-but-absent cache override
// from breaking confinement.
func TestWriteAllowedRoots(t *testing.T) {
	ws := t.TempDir()
	realCache := t.TempDir()
	t.Setenv("GOCACHE", realCache)                             // an existing override → included
	t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "gone")) // a non-existent override → dropped

	roots := WriteAllowedRoots(ws)

	if len(roots) == 0 || roots[0] != ws {
		t.Fatalf("roots = %v, want the workspace %q first", roots, ws)
	}
	found := map[string]bool{}
	for _, r := range roots {
		if found[r] {
			t.Errorf("duplicate root %q", r)
		}
		found[r] = true
		if info, err := os.Stat(r); err != nil || !info.IsDir() {
			t.Errorf("root %q is not an existing directory — a missing root fails the ruleset closed: %v", r, err)
		}
	}
	if !found[realCache] {
		t.Errorf("roots %v missing the existing GOCACHE override %q", roots, realCache)
	}
}
