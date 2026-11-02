package matrix

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
)

// harnessPackages are the stack-blind verification packages: the ladder, the
// checks, the runner, and their support. Stack-specific behavior must NOT live
// in any of them — only in ecosystem (data) and each fixture's manifest.
// ecosystem is deliberately absent: it IS the data home for per-stack tables.
var harnessPackages = []string{
	"../checks", "../ladder", "../runner", "../diffset",
	"../invoke", "../registry", "../schema", "../worktree", "../baseline",
}

// TestHarnessCarriesNoPerStackCode mechanizes A-1's inspection note: no harness
// source file may contain a per-stack string literal. python and node are the
// probe — they are the stacks T10 added, so a harness file that special-cased
// either would have to spell its quoted name. The "go" literal also appears in
// toolchain paths, so probe it specifically in comparisons and switch cases.
// A pass is the machine-checkable evidence that the three stacks are
// adjudicated by identical code, differing only in ecosystem data and manifests
// (FR-16).
func TestHarnessCarriesNoPerStackCode(t *testing.T) {
	needles := []string{`"python"`, `"node"`}
	for _, pkg := range harnessPackages {
		entries, err := os.ReadDir(pkg)
		if err != nil {
			t.Fatalf("read %s: %v", pkg, err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(pkg, name))
			if err != nil {
				t.Fatalf("read %s/%s: %v", pkg, name, err)
			}
			for _, n := range needles {
				if strings.Contains(string(data), n) {
					t.Errorf("%s/%s contains the per-stack literal %s — stack-specific behavior must live only in ecosystem data and fixture manifests (FR-16 / A-1)", pkg, name, n)
				}
			}
			tree, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
			if err != nil {
				t.Fatalf("parse %s/%s: %v", pkg, name, err)
			}
			ast.Inspect(tree, func(n ast.Node) bool {
				var values []ast.Expr
				switch expr := n.(type) {
				case *ast.BinaryExpr:
					if expr.Op == token.EQL || expr.Op == token.NEQ {
						values = []ast.Expr{expr.X, expr.Y}
					}
				case *ast.CaseClause:
					values = expr.List
				}
				for _, value := range values {
					if literal, ok := value.(*ast.BasicLit); ok && literal.Value == `"go"` {
						t.Errorf("%s/%s branches on the per-stack literal %s — stack-specific behavior must live only in ecosystem data and fixture manifests (FR-16 / A-1)", pkg, name, literal.Value)
					}
				}
				return true
			})
		}
	}
}

// TestStacksDeclaredOnlyInEcosystem asserts the supported-stack set is exactly
// what ecosystem.New() declares — one place. Adding a stack is an ecosystem
// data edit; no harness code enumerates stacks (the check above proves none
// names one).
func TestStacksDeclaredOnlyInEcosystem(t *testing.T) {
	stacks := ecosystem.New().Stacks
	for _, want := range []string{"go", "python", "node"} {
		if stacks[want] == nil {
			t.Errorf("ecosystem.New().Stacks is missing %q (the matrix binds it)", want)
		}
	}
	// Every declared stack must carry the tables TablesForStack needs, or a
	// fixture of that stack would fail closed at rung 3 rather than bind — the
	// property that lets the matrix driver stay stack-blind.
	eco := ecosystem.New()
	for name := range stacks {
		if eco.AssertionPatterns[name] == nil {
			t.Errorf("stack %q has no assertion patterns — test-delta/falsification would fail closed", name)
		}
		if eco.SkipMarkers[name] == nil {
			t.Errorf("stack %q has no skip markers — test-delta/falsification would fail closed", name)
		}
	}
}
