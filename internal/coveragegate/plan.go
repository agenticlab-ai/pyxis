package coveragegate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Split divides one package's top-level tests across Parts concurrent
// `go test` invocations. Go runs a package's sequential tests in a single
// process, so one slow package otherwise bounds the whole suite while other
// cores sit idle.
type Split struct {
	Package string // import path
	Parts   int
}

// Plan lists the packages Measure splits, ordered by the expected duration of
// one part so the longest invocations start first. Splitting only schedules
// work: every test still runs exactly once in its own package's test binary
// with the same instrumentation, and the per-invocation profiles concatenate
// to the profile a single invocation writes. A package outside the measured
// pattern, or with too few tests to divide, runs unsplit.
type Plan []Split

// invocation is one `go test` selection: test-name flags and packages.
type invocation struct {
	label string
	args  []string
}

type listedPackage struct {
	ImportPath   string   `json:"ImportPath"`
	Dir          string   `json:"Dir"`
	TestGoFiles  []string `json:"TestGoFiles"`
	XTestGoFiles []string `json:"XTestGoFiles"`
}

// invocations returns the selections that together run every test in pattern
// exactly once. Splitting is a scheduling choice, never part of the verdict,
// so any failure to plan it falls back to the single invocation, which then
// reports the underlying problem itself.
func (p Plan) invocations(ctx context.Context, dir, pattern string) []invocation {
	whole := []invocation{{args: []string{pattern}}}
	if len(p) == 0 {
		return whole
	}
	packages, err := listPackages(ctx, dir, pattern)
	if err != nil {
		return whole
	}
	byPath := make(map[string]listedPackage, len(packages))
	for _, pkg := range packages {
		byPath[pkg.ImportPath] = pkg
	}
	split := make(map[string]bool)
	var perPackage [][]invocation
	for _, s := range p {
		pkg, listed := byPath[s.Package]
		if !listed || split[s.Package] {
			continue
		}
		names, err := testNames(pkg)
		if err != nil {
			return whole
		}
		selections := partition(names, s.Parts)
		if len(selections) == 0 {
			continue
		}
		split[s.Package] = true
		pkgParts := make([]invocation, len(selections))
		for i, selection := range selections {
			pkgParts[i] = invocation{
				label: fmt.Sprintf("%s part %d/%d", s.Package, i+1, len(selections)),
				args:  append(selection, s.Package),
			}
		}
		perPackage = append(perPackage, pkgParts)
	}
	if len(perPackage) == 0 {
		return whole
	}
	parts := interleave(perPackage)
	var rest []string
	for _, pkg := range packages {
		if !split[pkg.ImportPath] {
			rest = append(rest, pkg.ImportPath)
		}
	}
	if len(rest) == 0 {
		return parts
	}
	return append([]invocation{{label: "unsplit packages", args: rest}}, parts...)
}

// interleave deals one part of each package per round, in plan order. A
// package's slowest test may sit in any of its parts, so every listed package
// starts early instead of waiting behind another package's parts.
func interleave(perPackage [][]invocation) []invocation {
	var parts []invocation
	for round := 0; ; round++ {
		dealt := false
		for _, pkgParts := range perPackage {
			if round < len(pkgParts) {
				parts = append(parts, pkgParts[round])
				dealt = true
			}
		}
		if !dealt {
			return parts
		}
	}
}

func listPackages(ctx context.Context, dir, pattern string) ([]listedPackage, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json=ImportPath,Dir,TestGoFiles,XTestGoFiles", pattern)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list coverage packages: %w", err)
	}
	var packages []listedPackage
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			return packages, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode coverage packages: %w", err)
		}
		packages = append(packages, pkg)
	}
}

// testNames returns the package's top-level test functions in source order.
// The names only balance the split; they never decide whether a test runs.
func testNames(pkg listedPackage) ([]string, error) {
	fset := token.NewFileSet()
	seen := make(map[string]bool)
	var names []string
	for _, name := range slices.Concat(pkg.TestGoFiles, pkg.XTestGoFiles) {
		file, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("scan tests: %w", err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && isTest(fn.Name.Name) && !seen[fn.Name.Name] {
				seen[fn.Name.Name] = true
				names = append(names, fn.Name.Name)
			}
		}
	}
	return names, nil
}

// isTest applies go test's naming rule: Test, or Test followed by a
// non-lowercase rune. TestMain is the package's entry point, not a test.
func isTest(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok || name == "TestMain" {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// partition deals names round-robin into at most n selections. The last
// selection skips every name dealt to the others rather than naming its own,
// so examples, fuzz targets, and any test the source scan missed still run
// exactly once.
func partition(names []string, n int) [][]string {
	n = min(n, len(names))
	if n < 2 {
		return nil
	}
	groups := make([][]string, n)
	for i, name := range names {
		groups[i%n] = append(groups[i%n], name)
	}
	selections := make([][]string, n)
	var dealt []string
	for i, group := range groups[:n-1] {
		selections[i] = []string{"-run", anchored(group)}
		dealt = append(dealt, group...)
	}
	selections[n-1] = []string{"-skip", anchored(dealt)}
	return selections
}

// anchored matches exactly the named top-level tests. Without a slash the
// pattern applies only to the first name level, so their subtests all run.
func anchored(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = regexp.QuoteMeta(name)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}
