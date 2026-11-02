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

// crosslinkEntry renders a review-feedback entry whose `## Cross-links` section
// is the given body — the entry whose one-directional cross-links the arm
// resolves against the seeded reviewer report and Argus delivery ledger.
func crosslinkEntry(crosslinks string) string {
	return `# Review-feedback entry 001

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

## Cross-links

` + crosslinks + "\n"
}

// seededReviewReport is a review-report/v1 carrying one finding heading — the
// `review:reviewer-alpha#FND-1` cross-link target (a reviewer M2 residual).
const seededReviewReport = `# Review: reviewer-alpha — review report

| Field | Value |
|---|---|
| **Verdict** | approve-with-advisories |
| **Template version** | review-report/v1 |

## Round 1

### FND-1 — the reviewer's residual

- **Severity**: advisory
`

// crosslinkCorpus builds a temp git repo seeded with one review report and one
// Argus delivery ledger (the cross-link targets), plus a review-feedback entry
// carrying the given `## Cross-links` body, then runs the cross-link arm over it
// via the real `git ls-files` corpus read.
func crosslinkCorpus(t *testing.T, crosslinks string) []schema.Defect {
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
	write("docs/reviews/reviewer-alpha.md", seededReviewReport)
	write("docs/problems/seeded-demo/delivery.md", seededDelivery)
	write("docs/feedback/subject-alpha/001-conversation.md", crosslinkEntry(crosslinks))
	run("add", ".")
	run("commit", "-qm", "corpus")
	return gate.CheckReviewFeedbackCrosslinks(root, config.Defaults())
}

// Acceptance: an entry's cross-links resolve to their targets — a reviewer M2
// residual (a review-report finding) and an Argus E-n journal disposition — and
// the arm is silent.
func TestCrosslinksResolveSilently(t *testing.T) {
	defects := crosslinkCorpus(t,
		"- review:reviewer-alpha#FND-1 — the residual\n- delivery:seeded-demo#E-1 — the disposition")
	if len(defects) != 0 {
		t.Fatalf("resolving cross-links must be silent, got %v", defects)
	}
}

// A `review:` cross-link whose report file does not exist reds the arm.
func TestCrosslinksDeadReviewReportReds(t *testing.T) {
	defects := crosslinkCorpus(t, "- review:no-such-report#FND-1")
	if !hasOp(defects, "unresolved-crosslink") {
		t.Fatalf("a missing review report must red the arm, got %v", defects)
	}
	for _, d := range defects {
		if d.Op == "unresolved-crosslink" && d.File != "docs/feedback/subject-alpha/001-conversation.md" {
			t.Errorf("defect should name the citing entry, got %q", d.File)
		}
	}
}

// A `review:` cross-link to a real report but an absent finding anchor reds.
func TestCrosslinksDeadReviewFindingReds(t *testing.T) {
	defects := crosslinkCorpus(t, "- review:reviewer-alpha#FND-9")
	if !hasOp(defects, "unresolved-crosslink") {
		t.Fatalf("an absent finding anchor must red the arm, got %v", defects)
	}
}

// A `delivery:` cross-link whose slug names no ledger reds — resolution is
// enforced (the inverse of check-ledger-citations, which would silently drop it).
func TestCrosslinksUnknownDeliverySlugReds(t *testing.T) {
	defects := crosslinkCorpus(t, "- delivery:no-such-slug#E-1")
	if !hasOp(defects, "unresolved-crosslink") {
		t.Fatalf("an unknown delivery slug must red the arm, got %v", defects)
	}
}

// A `delivery:` cross-link to a real ledger but an absent journal anchor reds.
func TestCrosslinksDeadDeliveryAnchorReds(t *testing.T) {
	defects := crosslinkCorpus(t, "- delivery:seeded-demo#E-9")
	if !hasOp(defects, "unresolved-crosslink") {
		t.Fatalf("an absent journal anchor must red the arm, got %v", defects)
	}
}

// A `delivery:` cross-link landing in the never-citable derived-state block reds
// — the same posture as check-ledger-citations (a journal E-n is the target).
func TestCrosslinksDerivedStateAnchorReds(t *testing.T) {
	defects := crosslinkCorpus(t, "- delivery:seeded-demo#Status")
	if !hasOp(defects, "unresolved-crosslink") {
		t.Fatalf("a derived-state anchor must red the arm, got %v", defects)
	}
}

// A citation shown in a code span or fenced block, or written with the grammar
// metavariable, is not a live cross-link (the fold never collects it) — silent.
func TestCrosslinksCodeAndMetavariableSilent(t *testing.T) {
	defects := crosslinkCorpus(t, strings.Join([]string{
		"- example `review:no-such-report#FND-1` in a span",
		"- metavariable delivery:<slug>#<anchor>",
		"```",
		"- fenced review:no-such-report#FND-9",
		"```",
	}, "\n"))
	if len(defects) != 0 {
		t.Fatalf("code-span, fenced, and metavariable forms must be silent, got %v", defects)
	}
}

// A citation-shaped string in the verbatim `## Comment` (not the `## Cross-links`
// section) is not a cross-link — the arm enforces only an entry's declared links.
func TestCrosslinksCommentCitationNotEnforced(t *testing.T) {
	entry := `# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @r |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | s#a |
| **Template version** | review-feedback-entry/v1 |

## Comment

The reviewer mentioned review:no-such-report#FND-1 in passing.
`
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	p := filepath.Join(root, "docs/feedback/subject-alpha/001-conversation.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q", "-b", "main")
	run("add", ".")
	run("commit", "-qm", "corpus")
	defects := gate.CheckReviewFeedbackCrosslinks(root, config.Defaults())
	if len(defects) != 0 {
		t.Fatalf("a citation in the verbatim comment is not a cross-link, got %v", defects)
	}
}

// One-directional + report-only: a run mutates no file in the corpus, so the
// cited targets (review-report/v1, the Argus ledger) are provably unmodified.
func TestCrosslinksReportOnlyTargetsUnmodified(t *testing.T) {
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
	write("docs/reviews/reviewer-alpha.md", seededReviewReport)
	write("docs/problems/seeded-demo/delivery.md", seededDelivery)
	write("docs/feedback/subject-alpha/001-conversation.md",
		crosslinkEntry("- review:reviewer-alpha#FND-1\n- delivery:seeded-demo#E-1"))
	run("add", ".")
	run("commit", "-qm", "corpus")
	_ = gate.CheckReviewFeedbackCrosslinks(root, config.Defaults())
	status, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(status)) != "" {
		t.Fatalf("the arm must not mutate the working tree, got: %s", status)
	}
}

// The arm registers as a base-free repo-wide check discoverable in the registry.
func TestCrosslinksRegistered(t *testing.T) {
	c, ok := gate.Lookup("check-review-feedback-crosslinks")
	if !ok {
		t.Fatal("check-review-feedback-crosslinks is not registered")
	}
	if c.NeedsMergeBase {
		t.Error("the cross-link arm must be base-free (NeedsMergeBase=false)")
	}
}
