package adapters_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const adapterPrefix = "github.com/agenticlab-ai/pyxis/internal/adapters"
const commonPrefix = adapterPrefix + "/common"

// core packages may not import port adapters (AP-1). cmd/ is the composition
// root. Tests may import adapters. adapters/common is shared medium support,
// not a port implementation, so core may import it. lane/githost is a
// remaining merge-lane adapter that still constructs the Forgejo CodeHost
// directly.
var allowlistedCore = map[string]bool{
	"lane/githost": true,
}

func TestCorePackagesImportNoAdapter(t *testing.T) {
	root := repoRoot(t)
	internal := filepath.Join(root, "internal")
	fset := token.NewFileSet()
	scanned := 0
	err := filepath.WalkDir(internal, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(internal, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(rel, "adapters") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if allowlisted(rel) {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		scanned++
		if hits := portAdapterImports(file); len(hits) != 0 {
			t.Errorf("%s imports adapter package(s) %v — AP-1: engine packages import ports only; cmd/ wires adapters", rel, hits)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}
	if scanned < 50 {
		t.Fatalf("scanned %d core files — the lint would pass vacuously", scanned)
	}
}

func TestImportBoundaryLintDetectsViolation(t *testing.T) {
	src := `package core
import _ "` + adapterPrefix + `/harness/claude"
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "violation.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if hits := portAdapterImports(f); len(hits) == 0 {
		t.Fatal("the import-boundary predicate failed to flag an adapter import")
	}
}

func TestImportBoundaryAllowsCommon(t *testing.T) {
	src := `package core
import _ "` + commonPrefix + `/github/githubauth"
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "common.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if hits := portAdapterImports(f); len(hits) != 0 {
		t.Fatalf("adapters/common is shared medium support; flagged %v", hits)
	}
}

func allowlisted(rel string) bool {
	for dir := range allowlistedCore {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

func portAdapterImports(f *ast.File) []string {
	var hits []string
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if p == commonPrefix || strings.HasPrefix(p, commonPrefix+"/") {
			continue
		}
		if p == adapterPrefix || strings.HasPrefix(p, adapterPrefix+"/") {
			hits = append(hits, p)
		}
	}
	return hits
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
