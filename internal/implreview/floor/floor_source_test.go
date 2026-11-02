package floor

// This test keeps the mirrored findingClasses set HONEST against codegate's actual
// source without importing or editing internal/verify. The finer finding classes
// codegate emits are string literals inside internal/verify/checks (e.g. `Class:
// "secret"`), not an exported symbol; this package mirrors them (findingClasses)
// and this test scans the check source files read-only, extracts every `Class:`
// literal, and fails if any literal is NOT covered by the enforced set (check ids
// ∪ findingClasses). Drift — a NEW codegate finding class the floor would silently
// let through — becomes a test failure here, not a production floor gap (FR-4).
//
// It is an in-package (white-box) test so it can read findingClasses directly. It
// is hermetic (reads repo files, no network/clock) and treats internal/verify as
// READ-ONLY — it opens the files, it never writes them.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// classLiteral matches a `Class:   "some-class"` assignment in the check source.
// It matches ANY double-quoted string literal (not just lowercase-kebab) so a
// future codegate class in a different form — an underscore, an uppercase letter, a
// leading digit — cannot SILENTLY escape the coverage guard (which would re-open the
// exact floor gap this test exists to close). A parametric `Class: string(res.Class)`
// or `Class: checkID` carries no quoted literal and is correctly not matched — those
// checks' concrete class literals live in the same files and ARE matched.
var classLiteral = regexp.MustCompile(`Class:\s*"([^"]+)"`)

// checksSourceDir is codegate's check package, relative to this package.
func checksSourceDir() string {
	return filepath.Join("..", "..", "..", "internal", "verify", "checks")
}

// TestFindingClassesCoverCodegateLiterals — every `Class:` string literal in
// internal/verify/checks is covered by the floor's enforced set. A literal that is
// (a) a check ID resolves via the registry, and (b) a finer finding class must be
// in findingClasses. A `Class: string(res.Class)` (a variable, not a literal) is
// correctly not matched by the literal regex — those are the parametric classes the
// diff checks emit, whose concrete literals live in the same files and ARE matched.
func TestFindingClassesCoverCodegateLiterals(t *testing.T) {
	dir := checksSourceDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	enforced := EnforcedClasses() // check ids (registry) ∪ findingClasses

	found := map[string]string{} // class literal -> file it appeared in
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" {
			continue
		}
		// Skip test files — the mirror pins PRODUCTION classes, and a test file may
		// carry example literals that are not real emitted classes.
		if len(name) > len("_test.go") && name[len(name)-len("_test.go"):] == "_test.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range classLiteral.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = name
		}
	}

	if len(found) == 0 {
		t.Fatal("no Class: literals found in internal/verify/checks — the scan is broken (or the checks moved)")
	}

	for class, file := range found {
		if !enforced[class] {
			t.Errorf("codegate emits Class:%q (in %s) but the floor's enforced set does not cover it — "+
				"add %q to floor.findingClasses (a new codegate class the floor would silently let through)",
				class, file, class)
		}
	}
}

// TestNoStaleFindingClasses — the mirror carries no class that is NOT actually a
// codegate literal or check id (a stale entry would be dead and misleading). Every
// findingClasses entry must appear as a real `Class:` literal in the check source.
func TestNoStaleFindingClasses(t *testing.T) {
	dir := checksSourceDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	literals := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" {
			continue
		}
		if len(name) > len("_test.go") && name[len(name)-len("_test.go"):] == "_test.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, m := range classLiteral.FindAllStringSubmatch(string(data), -1) {
			literals[m[1]] = true
		}
	}

	for _, cl := range findingClasses {
		if !literals[cl] {
			t.Errorf("floor.findingClasses carries %q but no such Class: literal exists in internal/verify/checks (stale mirror entry)", cl)
		}
	}
}
