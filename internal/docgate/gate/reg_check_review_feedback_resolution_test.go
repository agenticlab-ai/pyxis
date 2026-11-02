package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// seededFeedbackEntry is a well-formed review-feedback ledger entry — the file a
// `review-feedback:subject-alpha#001` citation resolves to. The resolution check
// keys off the filename's NNN under docs/feedback/<stem>/, so the entry's body
// is immaterial to resolution; it is realistic to keep the fixture honest.
const seededFeedbackEntry = `# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @reviewer-one |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | review-feedback-capture#3.1 |
| **Template version** | review-feedback-entry/v1 |

## Comment

The fold should emit a content fingerprint.
`

// feedbackCorpus builds a temp git repo seeded with one real ledger entry under
// docs/feedback/subject-alpha/ and the given citing documents, then runs the
// resolution arm over it via the real `git ls-files` corpus read. The subject
// stem (`subject-alpha`) is fixed so the fixtures cite a known-resolvable entry.
func feedbackCorpus(t *testing.T, docs map[string]string) []schema.Defect {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("docs/feedback/subject-alpha/001-conversation.md", seededFeedbackEntry)
	for rel, content := range docs {
		write(rel, content)
	}
	run("add", ".")
	run("commit", "-qm", "corpus")
	return gate.CheckReviewFeedbackResolution(root, config.Config{Feedback: "docs/feedback"})
}

// Acceptance: a `review-feedback:` citation resolving by grep to its entry is
// silent.
func TestReviewFeedbackResolutionResolvesSilently(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/wiki/summary.md": "The calibration reads review-feedback:subject-alpha#001 as evidence.\n",
	})
	if len(defects) != 0 {
		t.Fatalf("a resolving citation must be silent, got %v", defects)
	}
}

// Acceptance: a dead `review-feedback:` ref — the subject exists but the anchor
// names no entry — reds the resolution check, naming the citing document.
func TestReviewFeedbackResolutionDeadAnchorReds(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/wiki/summary.md": "See review-feedback:subject-alpha#999 for the origin.\n",
	})
	if !hasOp(defects, "unresolved-review-feedback") {
		t.Fatalf("a dead anchor must red the check, got %v", defects)
	}
	var d schema.Defect
	for _, x := range defects {
		if x.Op == "unresolved-review-feedback" {
			d = x
		}
	}
	if d.File != "docs/wiki/summary.md" {
		t.Errorf("defect should name the citing document, got file %q", d.File)
	}
	if d.Verb != "check-review-feedback-resolution" {
		t.Errorf("wrong verb %q", d.Verb)
	}
}

// A dead ref whose *subject* names no ledger directory also reds — resolution is
// enforced, not scope-decided by resolvability (the inverse of the delivery arm).
func TestReviewFeedbackResolutionUnknownSubjectReds(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/wiki/summary.md": "A stray review-feedback:no-such-subject#001 citation.\n",
	})
	if !hasOp(defects, "unresolved-review-feedback") {
		t.Fatalf("an unknown subject must red the check, got %v", defects)
	}
}

// Acceptance: a `Learned-from: review-thread` ref pointing at a real entry (the
// review-feedback anchor riding the amendment model's free ref slot) passes.
func TestReviewFeedbackResolutionLearnedFromResolves(t *testing.T) {
	amendment := "| **Learned-from** | review-thread:review-feedback:subject-alpha#001 · operator-decision:x |\n"
	defects := feedbackCorpus(t, map[string]string{
		"docs/prds/demo/001-change.md": amendment,
	})
	if len(defects) != 0 {
		t.Fatalf("a Learned-from ref at a real entry must pass, got %v", defects)
	}
}

// Acceptance: a dangling `Learned-from: review-thread` review-feedback ref reds
// the check — the same uniform recognition as a bare citation.
func TestReviewFeedbackResolutionLearnedFromDanglingReds(t *testing.T) {
	amendment := "| **Learned-from** | review-thread:review-feedback:subject-alpha#404 |\n"
	defects := feedbackCorpus(t, map[string]string{
		"docs/prds/demo/001-change.md": amendment,
	})
	if !hasOp(defects, "unresolved-review-feedback") {
		t.Fatalf("a dangling Learned-from review-feedback ref must red, got %v", defects)
	}
}

// A legacy `Learned-from: review-thread` free-text ref (not a review-feedback
// citation) is untouched — proving the amendment model needs no change: existing
// amendments stay green and only review-feedback anchors are enforced.
func TestReviewFeedbackResolutionLegacyReviewThreadSilent(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/prds/demo/001-change.md": "| **Learned-from** | review-thread:pr-56-bundled-machinery |\n",
	})
	if len(defects) != 0 {
		t.Fatalf("a non-feedback review-thread ref must be untouched, got %v", defects)
	}
}

// A citation shown inside a code span or fenced block, or written with the
// angle-bracketed grammar metavariables, is a displayed example — never a
// citation, so the design/plan grammar illustrations stay green.
func TestReviewFeedbackResolutionCodeAndMetavariableSilent(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/wiki/span.md":  "An example is `review-feedback:subject-alpha#999` inline.\n",
		"docs/wiki/fence.md": "```\nreview-feedback:subject-alpha#999\n```\n",
		"docs/wiki/meta.md":  "Cite as review-feedback:<subject-stem>#<entry-anchor> in prose.\n",
	})
	if len(defects) != 0 {
		t.Fatalf("code-span, fenced, and metavariable forms must be silent, got %v", defects)
	}
}

// The recognition combined fixture: only the concrete, non-code, non-resolving
// citation classifies; every other form is silent (the §3.5 recognition shape).
func TestReviewFeedbackResolutionRecognitionBoundary(t *testing.T) {
	defects := feedbackCorpus(t, map[string]string{
		"docs/wiki/combined.md": strings.Join([]string{
			"i.   resolving review-feedback:subject-alpha#001",                // silent (resolves)
			"ii.  dead review-feedback:subject-alpha#777",                     // classified
			"iii. metavariable review-feedback:<subject-stem>#<entry-anchor>", // drop
			"iv.  code span `review-feedback:subject-alpha#888`",              // drop
			"```",
			"v.   fenced review-feedback:subject-alpha#999", // drop
			"```",
			"",
		}, "\n"),
	})
	ops := opsFor(defects, "docs/wiki/combined.md")
	if len(ops) != 1 || ops[0] != "unresolved-review-feedback" {
		t.Fatalf("only the concrete dead citation should classify, got ops %v (defects %v)", ops, defects)
	}
}

// The arm is report-only: a run mutates no file in the corpus.
func TestReviewFeedbackResolutionReportOnly(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("docs/feedback/subject-alpha/001-conversation.md", seededFeedbackEntry)
	write("docs/wiki/cite.md", "review-feedback:subject-alpha#999\n")
	run("add", ".")
	run("commit", "-qm", "corpus")
	_ = gate.CheckReviewFeedbackResolution(root, config.Config{Feedback: "docs/feedback"})
	status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(status)) != "" {
		t.Fatalf("the arm must not mutate the working tree, got: %s", status)
	}
}

// The arm registers as a base-free repo-wide check discoverable in the registry.
func TestReviewFeedbackResolutionRegistered(t *testing.T) {
	c, ok := gate.Lookup("check-review-feedback-resolution")
	if !ok {
		t.Fatal("check-review-feedback-resolution is not registered")
	}
	if c.NeedsMergeBase {
		t.Error("the resolution arm must be base-free (NeedsMergeBase=false)")
	}
}
