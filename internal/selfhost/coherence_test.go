package selfhost

// A-2 — the coherence inspection (design config-driven-portability-v1 §3.8, the
// basis's DOD-4; #A-2; #DINV-1). A first-pass scan of engine code (internal/,
// cmd/) that asserts ZERO target-facing literals remain outside the schema's
// defaults. It formalizes DINV-1's property — "the config is the single source
// for every target-facing value" — as an independent grep, complementary to
// A-1: A-1's self-host on a non-default config is the FULL proof (a missed or
// constant-folded derivation fails THERE, on the real corpus); this inspection
// catches a re-introduced literal early and names its file+line, cheaply, even
// under -short.
//
// The scan is deliberately coarse and legible (A-2 is a "first-pass scan"): it
// reads each non-test .go file, strips comments (a literal named in a docstring
// that EXPLAINS the single-source rule is not a violation), and reports any
// remaining high-value target-facing literal:
//
//   - the three governing-doc paths (product-vision.md, architecture-principles.md,
//     design-principles.md) — the paths T1 moved behind config;
//   - the composed mainline literal "origin/main", and the bare "main"/"origin"
//     tokens where they name the repository mainline — the branch/remote T2 moved
//     behind config.
//
// A small, commented allow-list names the SANCTIONED homes — the files that
// legitimately hold these literals and must never trip the inspection. Keeping
// the allow-list explicit (rather than widening the scan's blind spots) is what
// keeps it a maintainable audit surface: a new sanctioned home is a one-line,
// reasoned addition here.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// targetFacingLiterals are the high-value target-facing string literals the
// inspection hunts (each as it appears in Go source, double-quoted). The
// governing-doc paths and the composed mainline literal are unambiguous; the
// bare mainline tokens are caught by mainlineTokenLiterals below (which must not
// flag an unrelated "main" like a package name — see there).
var targetFacingLiterals = []string{
	`"docs/product-vision.md"`,
	`"docs/architecture-principles.md"`,
	`"docs/design-principles.md"`,
	`"origin/main"`,
}

// mainlineTokenLiterals are the bare branch/remote tokens — flagged only where
// they name the repository mainline. They are separated because a bare "main"
// or "origin" can legitimately appear meaning something else entirely (a Go
// package named "main", say). The allow-list carries the one such sanctioned
// non-mainline use (internal/sweep, which compares a Go package name), so a bare
// occurrence anywhere NOT allow-listed is treated as a suspected mainline
// literal — the conservative, fail-toward-catching posture A-2 wants.
var mainlineTokenLiterals = []string{
	`"main"`,
	`"origin"`,
}

// sanctionedHomes maps a repo-relative Go file to the reason it legitimately
// holds a target-facing literal. These files are the config's single source or
// a documented cross-feature exclusion; the inspection never flags them. Every
// entry carries its justification so the allow-list stays legible (a reviewer
// can see WHY each is exempt without archaeology).
var sanctionedHomes = map[string]string{
	// THE single source of the defaults (config.Defaults + the pathSchema
	// table): the one place every target-facing literal is spelled, which every
	// consumer resolves FROM (AP-11, DINV-1). This is precisely "the schema's
	// defaults" A-2 scopes the literals OUT of.
	"internal/docgate/config/config.go": "the Defaults() single source — the schema defaults the inspection scopes literals out of (AP-11, DINV-1)",

	// The merge-lane security floor (a documented cross-feature exclusion): it
	// hardcodes the governing-doc paths as the always-HUMAN-reviewed set — a
	// deliberate defense-in-depth constant that must NOT drift to a config value
	// (a config author must not be able to demote the trust root out of the
	// floor). Named in the T6 brief as a sanctioned home.
	"internal/lane/policy/policy.go": "the merge-lane security floor's governing-doc set — a deliberate constant, never config-driven (defense-in-depth; T6 exclusion)",

	// A bare "main" that names a GO PACKAGE, never the repository mainline: it
	// filters `go list` output (a package named main is a shipped binary, not an
	// unwired-package candidate). Semantically unrelated to the branch surface.
	"internal/sweep/unwired.go": `a bare "main" naming a Go package (go list filter), not the repository mainline`,

	// CapsuleMismatch.Field names the semantic capsule member that failed.
	// The bare "origin" values in this file are execution-demand schema field
	// names, never the repository remote moved behind config by DINV-1.
	"internal/executiondemand/capsule.go": "the bare origin token names an execution-demand capsule field, not the repository remote",
}

// TestCoherenceNoTargetFacingLiterals is A-2: the inspection over internal/ and
// cmd/. It walks every non-test .go file, strips comments, and asserts no
// sanctioned-home-external file carries a target-facing literal. A failure names
// the exact file+line so a re-introduced literal is a legible catch, not a
// mystery gate red.
func TestCoherenceNoTargetFacingLiterals(t *testing.T) {
	root := repoRoot(t)
	var violations []string

	for _, dir := range []string{"internal", "cmd"} {
		base := filepath.Join(root, dir)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
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
			// testdata/ and fixtures/ hold intentionally-literal corpus content,
			// never engine code (the T6 brief's allow-listed trees).
			if strings.Contains(rel, "/testdata/") || strings.Contains(rel, "/fixtures/") {
				return nil
			}
			if _, sanctioned := sanctionedHomes[rel]; sanctioned {
				return nil
			}
			violations = append(violations, scanFileForLiterals(t, path, rel)...)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("the coherence inspection found %d target-facing literal(s) outside the schema defaults (DINV-1) — resolve them from config, or add a reasoned allow-list entry if genuinely sanctioned:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
	t.Logf("A-2: coherence inspection clean — zero target-facing literals in engine code outside %d sanctioned home(s)", len(sanctionedHomes))
}

// scanFileForLiterals reads one Go file, strips comments line by line, and
// returns a violation string per code line carrying a target-facing literal.
// Comment stripping is what lets a docstring explain "the ONE place the
// origin/main literals live, DINV-1" without tripping the inspection — only
// executable code text is scanned.
func scanFileForLiterals(t *testing.T, path, rel string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// Struct tags name serialized fields, not configured branch or remote
	// values. Remove only tags identified by Go's parser; runtime literals in
	// the same file or on the same line still participate in the inspection.
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, b, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		if ok && field.Tag != nil {
			for i := fset.Position(field.Tag.Pos()).Offset; i < fset.Position(field.Tag.End()).Offset; i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
		}
		return true
	})
	var out []string
	inBlockComment := false
	for i, raw := range strings.Split(string(b), "\n") {
		code, blockNow := stripComments(raw, inBlockComment)
		inBlockComment = blockNow
		if strings.TrimSpace(code) == "" {
			continue
		}
		for _, lit := range targetFacingLiterals {
			if strings.Contains(code, lit) {
				out = append(out, formatViolation(rel, i+1, lit, raw))
			}
		}
		for _, lit := range mainlineTokenLiterals {
			if strings.Contains(code, lit) {
				out = append(out, formatViolation(rel, i+1, lit, raw))
			}
		}
	}
	return out
}

func TestCoherenceDistinguishesSchemaTagsFromRuntimeDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         int
	}{
		{"schema fields", "package fixture\ntype Data struct { Origin string `json:\"origin\"`; Main string `json:\"main\"` }\n", 0},
		{"runtime remote", "package fixture\ntype Data struct { Origin string `json:\"origin\"` }; var remote = \"origin\"\n", 1},
		{"runtime mainline", "package fixture\ntype Data struct { Main string `json:\"main\"` }; var branch = \"main\"\n", 1},
		{"runtime path", "package fixture\nvar branch = \"origin/main\"\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.go")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if got := len(scanFileForLiterals(t, path, "fixture/source.go")); got != tc.want {
				t.Fatalf("violations = %d, want %d", got, tc.want)
			}
		})
	}
}

// formatViolation renders one finding: file:line, the offending literal, and
// the trimmed source line for context.
func formatViolation(rel string, line int, lit, raw string) string {
	return "  - " + rel + ":" + itoa(line) + ": " + lit + "  |  " + strings.TrimSpace(raw)
}

// stripComments removes Go comments from one source line, tracking multi-line
// /* */ block state across lines. It is a pragmatic lexer sufficient for the
// first-pass scan: it does not model string literals that CONTAIN "//" or "/*"
// (a "//" inside a string would be treated as a comment start). No engine string
// literal in this repo embeds a comment marker alongside a target-facing token,
// so the approximation is safe here and keeps the inspection legible; a future
// case that breaks it is caught by A-1's real-corpus proof regardless.
func stripComments(line string, inBlock bool) (code string, stillInBlock bool) {
	var b strings.Builder
	i := 0
	for i < len(line) {
		if inBlock {
			if i+1 < len(line) && line[i] == '*' && line[i+1] == '/' {
				inBlock = false
				i += 2
				continue
			}
			i++
			continue
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
			break // line comment: the rest is not code
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '*' {
			inBlock = true
			i += 2
			continue
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String(), inBlock
}

// itoa is a tiny int->string for the violation formatter (avoids pulling
// strconv into a test file that needs nothing else from it).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
