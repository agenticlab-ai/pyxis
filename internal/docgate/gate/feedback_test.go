package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const feedbackEntry = `# Review-feedback entry 001

| Field | Value |
|---|---|
| **Entry** | 001 |
| **Source** | conversation |
| **Author** | @a |
| **Timestamp** | 2026-07-13T09:30:00Z |
| **Subject-anchor** | x#3.1 |
| **Template version** | review-feedback-entry/v1 |

## Comment

A captured review comment.
`

// A feedback root holding one subject ledger: fold it, write the projection,
// and prove verify-generated is green on a fresh fold and reds on a tamper.
func setupFeedbackRepo(t *testing.T) (root string, cfg config.Config, subject string) {
	t.Helper()
	root = t.TempDir()
	cfg = config.Defaults()
	subject = filepath.Join(root, "docs/feedback", "some-subject")
	if err := os.MkdirAll(subject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "001-conversation.md"), []byte(feedbackEntry), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, defs := gate.ReadFeedbackEntries(subject)
	if len(defs) != 0 {
		t.Fatalf("read defects: %v", defs)
	}
	res := feedback.Fold("some-subject", entries)
	if len(res.Defects) != 0 {
		t.Fatalf("fold defects: %v", res.Defects)
	}
	if err := os.WriteFile(filepath.Join(subject, feedback.ProjectionFile), res.SourceJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, cfg, subject
}

func TestFeedbackDirsAndClassifier(t *testing.T) {
	root, cfg, subject := setupFeedbackRepo(t)

	dirs, err := gate.FeedbackDirs(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 1 || dirs[0] != subject {
		t.Fatalf("FeedbackDirs = %v, want [%s]", dirs, subject)
	}
	if !gate.IsFeedbackDir(root, cfg, subject) {
		t.Fatal("subject dir should classify as feedback")
	}
	if gate.IsFeedbackDir(root, cfg, filepath.Join(root, "docs/prds/x")) {
		t.Fatal("a PRD dir must not classify as feedback")
	}
}

func TestFeedbackDirsAbsentRootIsClean(t *testing.T) {
	root := t.TempDir()
	dirs, err := gate.FeedbackDirs(root, config.Defaults())
	if err != nil {
		t.Fatalf("absent feedback root should not error: %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("want no dirs, got %v", dirs)
	}
}

func TestVerifyFeedbackGenerated(t *testing.T) {
	_, _, subject := setupFeedbackRepo(t)

	// Green: a fresh fold matches the committed projection.
	entries, _ := gate.ReadFeedbackEntries(subject)
	res := feedback.Fold("some-subject", entries)
	if defs := gate.VerifyFeedbackGenerated(subject, res); len(defs) != 0 {
		t.Fatalf("fresh projection should verify clean, got %v", defs)
	}

	// Red: a hand-edited projection reds verify-generated.
	if err := os.WriteFile(filepath.Join(subject, feedback.ProjectionFile), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defs := gate.VerifyFeedbackGenerated(subject, res)
	if len(defs) == 0 {
		t.Fatal("a byte-mismatched projection must red verify-generated")
	}
	wantDefect(t, defs, "differs from the recomputed fold")

	// Red: a missing projection also reds.
	if err := os.Remove(filepath.Join(subject, feedback.ProjectionFile)); err != nil {
		t.Fatal(err)
	}
	wantDefect(t, gate.VerifyFeedbackGenerated(subject, res), "projection artifact missing")
}

// The umbrella Gate() folds and byte-verifies feedback ledgers alongside PRDs.
// A bare temp repo may raise unrelated repo-wide defects, so we assert on the
// delta: tampering the projection introduces the byte-mismatch defect.
func TestGateVerifiesFeedback(t *testing.T) {
	root, cfg, subject := setupFeedbackRepo(t)

	hasMismatch := func(defs []schema.Defect) bool {
		for _, d := range defs {
			if strings.Contains(d.Reason, "differs from the recomputed fold") {
				return true
			}
		}
		return false
	}

	if hasMismatch(gate.Gate(root, "", nil, cfg)) {
		t.Fatal("a fresh projection must not red the umbrella")
	}
	if err := os.WriteFile(filepath.Join(subject, feedback.ProjectionFile), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasMismatch(gate.Gate(root, "", nil, cfg)) {
		t.Fatal("a tampered feedback projection must red the umbrella")
	}
}
