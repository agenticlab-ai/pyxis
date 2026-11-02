// nonregression_test.go is the T8 slice of the review-feedback ledger
// (docs/plans/review-feedback-capture/001-plan.md T8; PRD NR-1..NR-4): it
// proves the whole capture feature is *provably additive* — conversation's edit
// behavior, the reviewers' no-human-dependency posture, the merge path, and the
// existing captures (`review-report/v1` + the Argus ledgers) are untouched.
//
// These are guards, not behavior: each test encodes one non-regression row as a
// durable invariant so a later change that would break it reds this suite. They
// complement — not replace — the acceptance-criterion that the existing
// conversation, reviewer, and merge-path suites pass unchanged, which the full
// `go test ./...` run enforces on every PR (docs-gate.yml). This file
// reuses the package's own AST helpers (importsUnderPrefix, repoRoot from
// writer_test.go) so a feature file's imports are read the same way the
// DINV-2 boundary lint reads them.
package feedback_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

const modulePrefix = "github.com/agenticlab-ai/pyxis"

// featureGoFiles returns every non-test Go source file the review-feedback
// capture feature owns: the whole ledger package tree (core + adapters), the
// three self-registered docgate check/verb files, and the capture CLI. The
// import guards below read exactly this set — the feature's own code, never a
// neighbouring package's — so a forbidden edge is attributed to this feature.
func featureGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	tree := filepath.Join(root, "internal", "docgate", "feedback")
	if err := filepath.WalkDir(tree, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		files = append(files, p)
		return nil
	}); err != nil {
		t.Fatalf("walk feedback tree: %v", err)
	}
	for _, rel := range []string{
		filepath.Join("internal", "docgate", "gate", "feedback.go"),
		filepath.Join("internal", "docgate", "gate", "reg_check_review_feedback_resolution.go"),
		filepath.Join("internal", "docgate", "gate", "reg_check_review_feedback_crosslinks.go"),
		filepath.Join("cmd", "feedback-capture", "main.go"),
	} {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("feature file %s missing — update featureGoFiles: %v", rel, err)
		}
		files = append(files, p)
	}
	return files
}

func parseImports(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

// NR-4 (Inspection): `review-report/v1` and the Argus ledgers are cross-link
// targets only — the feature reads them to resolve a citation, never writes
// them (FR-8, NG-5, #D-6). The producing surface for the Argus ledgers is the
// Argus code tree (internal/argus/**, cmd/argus-*); a review-report has no Go
// writer at all. This guard proves the feature imports no Argus *producer*:
// its one edge to Argus data is the read-only docgate-side parser
// internal/docgate/argusledger (Parse/ParseAs only — no writer), which the
// forbidden set deliberately excludes. So the federation is structurally
// one-directional: the feature cannot mutate what it only parses.
func TestNR4_FeatureImportsNoArgusProducer(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{
		modulePrefix + "/internal/argus", // the Argus ledger / attribution producers
		modulePrefix + "/cmd/argus",      // the Argus CLIs (argus-stamp, argus-writehook)
	}
	const readOnlyArgusReader = modulePrefix + "/internal/docgate/argusledger"

	files := featureGoFiles(t, root)
	if len(files) == 0 {
		t.Fatal("scanned no feature files — the guard would pass vacuously")
	}
	sawArgusReader := false
	for _, p := range files {
		f := parseImports(t, p)
		for _, pref := range forbidden {
			if hits := importsUnderPrefix(f, pref); len(hits) != 0 {
				t.Errorf("feature file %s imports Argus producer package(s) %v — NR-4: the Argus ledgers are read-only cross-link targets; resolve them through the read-only %s, never a producer", rel(root, p), hits, readOnlyArgusReader)
			}
		}
		if len(importsUnderPrefix(f, readOnlyArgusReader)) != 0 {
			sawArgusReader = true
		}
	}
	// Non-vacuity: the feature genuinely reaches Argus data (the crosslinks
	// check resolves `delivery:` refs), so at least one file must import the
	// read-only reader. If this fails, either the resolution path was rewritten
	// (revisit the forbidden set) or the guard is testing nothing.
	if !sawArgusReader {
		t.Fatalf("no feature file imports the read-only %s — the read-only-federation guard is not exercising a real Argus edge", readOnlyArgusReader)
	}
}

// The forbidden-import predicate is not vacuous: given a file that imports an
// Argus producer package, the same predicate flags it. Guards the guard above.
func TestNR4_ArgusImportPredicateDetectsViolation(t *testing.T) {
	src := "package feedback\nimport _ \"" + modulePrefix + "/internal/argus/attribution\"\n"
	f, err := parser.ParseFile(token.NewFileSet(), "violation.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if hits := importsUnderPrefix(f, modulePrefix+"/internal/argus"); len(hits) == 0 {
		t.Fatal("the Argus-producer import predicate failed to flag an internal/argus import")
	}
}

// NR-4 (Inspection): the coded writer is the feature's only file-mutating path,
// and it only ever *adds* under the feedback ledger root (§3.1, DINV-1). This
// guard proves it structurally cannot escape that root onto a `review-report/v1`
// or Argus-ledger path: every subject anchor either is rejected (a stem with a
// path separator or traversal segment fails the kebab-case gate) or resolves to
// a path strictly under Root. There is no anchor that writes elsewhere.
func TestNR4_WriterWritesOnlyUnderTheFeedbackRoot(t *testing.T) {
	root := t.TempDir()
	w := feedback.NewWriter(root)
	cases := []struct {
		anchor       string
		wantRejected bool
	}{
		{"review-feedback-capture#3.1", false},           // ordinary subject → contained
		{"argus-artifacts#E-3", false},                   // a stem naming argus is still just a feedback subject dir, contained
		{"../../docs/reviews/argus-attribution#x", true}, // traversal → rejected before any write
		{"docs/reviews/review-report#x", true},           // path separators in the stem → rejected
		{"/etc/passwd#x", true},                          // absolute escape → rejected
	}
	for _, tc := range cases {
		path, err := w.WriteEntry(feedback.CapturedComment{
			SchemaVersion: feedback.CapturedCommentVersion,
			Comment:       "a captured review comment",
			Author:        "@reviewer",
			Timestamp:     "2026-07-14T00:00:00Z",
			SubjectAnchor: tc.anchor,
			SourceID:      "conversation",
		})
		if tc.wantRejected {
			if err == nil {
				t.Errorf("anchor %q was accepted (wrote %q) — a stem that escapes the feedback root must be rejected (NR-4)", tc.anchor, path)
			}
			continue
		}
		if err != nil {
			t.Errorf("anchor %q was rejected (%v) — an ordinary subject stem must write into the ledger", tc.anchor, err)
			continue
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(relPath, "..") {
			t.Errorf("anchor %q wrote to %q — outside the feedback root %q (NR-4: the writer only adds under the ledger)", tc.anchor, path, root)
		}
	}
}

// NR-2 (Test): the reviewers' no-human-dependency posture holds — human review
// stays an optional overlay, so no pipeline path acquires a *required* human
// review step (NG-4). This guard proves that a subject with no captured
// feedback is a fully valid state: an absent ledger root is not a defect
// (FeedbackDirs returns none, no error), and the review-feedback checks red on
// nothing when there is no feedback. Nothing the feature adds demands a human
// review be present for the pipeline to complete.
func TestNR2_AbsentFeedbackIsNotARequiredStep(t *testing.T) {
	cfg := config.Defaults()

	// A tree with no docs/feedback at all, and a tree with an empty feedback
	// root: both report zero ledger dirs and no error — an absent capture is not
	// a defect, so nothing forces a human review to be present.
	empty := t.TempDir()
	present := t.TempDir()
	if err := os.MkdirAll(filepath.Join(present, filepath.FromSlash(cfg.Feedback)), 0o755); err != nil {
		t.Fatalf("seed empty feedback root: %v", err)
	}
	for _, root := range []string{empty, present} {
		dirs, err := gate.FeedbackDirs(root, cfg)
		if err != nil {
			t.Fatalf("an absent/empty feedback ledger must not error: %v", err)
		}
		if len(dirs) != 0 {
			t.Fatalf("a tree with no captured feedback reported %d ledger dirs, want 0", len(dirs))
		}
	}

	// And over the live corpus (a real git tree, currently carrying no captured
	// human feedback), the enforced-resolution check reds nothing — the feature
	// imposes no required-human-review step on the pipeline.
	if defs := gate.CheckReviewFeedbackCrosslinks(repoRoot(t), cfg); len(defs) != 0 {
		t.Fatalf("the crosslinks check red on the current corpus (%d defects) — zero human review must be a valid state (NR-2)", len(defs))
	}
}

// NR-3 (Test): merge authorization, the required checks, and the risk lanes are
// unchanged — the feature touches no merge-path config. Its two docgate checks
// self-register into the registry (distributed-check-registration) and ride the
// single, pre-existing `docgate gate` context the merge-path config already
// invokes; the config was never edited to name or wire them. This guard proves
// (a) the merge-path CI config names no review-feedback verb, and (b) both
// feature checks register base-free (NeedsMergeBase: false) — they hook no
// merge-base authorization step.
func TestNR3_MergePathConfigUntouched(t *testing.T) {
	root := repoRoot(t)
	wfDir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		t.Fatalf("read workflows dir: %v", err)
	}
	scanned := 0
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(wfDir, e.Name()))
		if err != nil {
			t.Fatalf("read workflow %s: %v", e.Name(), err)
		}
		scanned++
		if strings.Contains(string(b), "review-feedback") || strings.Contains(string(b), "feedback-capture") {
			t.Errorf("merge-path config %s names the review-feedback feature — its checks must ride the generic registry-driven gate, leaving the merge-path config untouched (NR-3)", e.Name())
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no workflow files — the merge-path-config guard would pass vacuously")
	}

	for _, verb := range []string{"check-review-feedback-resolution", "check-review-feedback-crosslinks"} {
		c, ok := gate.Lookup(verb)
		if !ok {
			t.Fatalf("feature check %q is not registered — the guard cannot assert its merge-path posture", verb)
		}
		if c.NeedsMergeBase {
			t.Errorf("feature check %q registers NeedsMergeBase=true — it must be base-free, hooking no merge-base authorization step (NR-3)", verb)
		}
	}
}

// rel renders a repo-relative path for a readable failure message.
func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return r
	}
	return p
}
