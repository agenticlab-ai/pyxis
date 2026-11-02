package preflight_test

// The tree-wide one-home sweep (plan init-hardening/001 T10's acceptance;
// design init-hardening-v1 #DINV-2, A-9): the prerequisite presence
// predicates — exec.LookPath over git/gh — are implemented in this
// package alone. Every other hit across cmd/ and internal/ is a defect
// unless it is the named, reasoned exemption below, with a
// DIFFERENT contract from operand-prerequisite checking:
//
//   - internal/wire/runner.go — the gh transport's own exec resolution: it
//     locates gh to RUN it and returns the raw error for callers (this
//     suite included) to interpret; it renders no presence verdict.
import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var prereqLookPathRe = regexp.MustCompile(`exec\.LookPath\("(?:git|gh)"\)`)

var oneHomeExemptions = map[string]bool{
	"internal/wire/runner.go": true, // the transport's exec resolution, not a presence verdict
}

func TestPrerequisiteProbesHaveOneHomeTreeWide(t *testing.T) {
	root := moduleRoot(t)
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if strings.HasPrefix(rel, "internal/preflight/") || oneHomeExemptions[rel] {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if m := prereqLookPathRe.Find(src); m != nil {
				t.Errorf("%s probes a prerequisite executable (%s) — the presence predicates live in internal/preflight alone (one home, design init-hardening-v1 #DINV-2); consume preflight.Prerequisites or the suite's probes instead", rel, m)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", top, err)
		}
	}
}

// moduleRoot walks up from the package directory to the go.mod holder.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package directory")
		}
		dir = parent
	}
}
