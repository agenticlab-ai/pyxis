// writer.go is the T2 slice of the review-feedback ledger
// (docs/tech-design/review-feedback-capture.md §3.3): the consumer-side
// CaptureSource port, the schema-versioned CapturedComment it yields, and the
// coded Writer — the sole producer of ledger entries.
//
// The Writer is fed through the CaptureSource port and imports no adapter
// package (DINV-2): a new capture source is a new adapter implementing the
// port, never new core code. The Writer is the only entry-producing path;
// no other code and no guide hand-authors an entry (DINV-6). Medium specifics
// (a confirm hook, a webhook, a poll) live inside the adapters and die at the
// port boundary (RW-3); this file knows only the medium-neutral CapturedComment.
package feedback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CapturedCommentVersion is the pinned schema version a CaptureSource stamps
// into every CapturedComment it yields. The Writer reads only this version:
// a comment declaring another is rejected, never rendered against a guessed
// shape (AP-6, fail-closed).
const CapturedCommentVersion = "captured-comment/v1"

// CapturedComment is the schema-versioned unit a CaptureSource yields (§3.3):
// the five medium-neutral facts the Writer needs to render one ledger entry.
// The Writer assigns the entry's dense sequence itself — it is not carried
// here, so a source cannot pick an entry number.
//
// SourceID is the capturing source's identity (conversation | code-host | …) —
// it becomes the entry's Source field and the filename's source stem, so it
// must be a kebab-case stem. SubjectAnchor names the reviewed location
// (`<subject-stem>#<section>`); its subject-stem selects the per-subject
// directory the entry lands in.
type CapturedComment struct {
	SchemaVersion string `json:"schema_version"`
	Comment       string `json:"comment"` // the review comment, verbatim (§3.8)
	Author        string `json:"author"`
	Timestamp     string `json:"timestamp"`
	SubjectAnchor string `json:"subject_anchor"`
	SourceID      string `json:"source_id"`
	// EditCommit is the commit associated with the resulting edit,
	// recorded at capture so the disposition observer can later ask whether it
	// landed on the subject (§3.4, T4). Empty when the confirmed comment produced
	// no edit — a comment that yields no change (a candidate no-action, UB-3).
	// Additive (omitempty): an entry without it renders and folds exactly as
	// before, so an old projection stays byte-identical (AP-6).
	EditCommit string `json:"edit_commit,omitempty"`
}

// CaptureSource is the consumer-side port (§3.3, DINV-2): the Writer declares
// what it needs — a batch of captured comments — and each medium implements it
// in its own adapter package. The Writer core imports no implementation of
// this interface; the import-boundary lint (writer_test.go) is the enforcement.
type CaptureSource interface {
	// Comments yields the captured comments to persist. The medium's ingestion
	// mechanism (explicit capture / webhook / poll) is the adapter's concern and
	// stays behind this boundary (RW-3).
	Comments(ctx context.Context) ([]CapturedComment, error)
}

// Writer is the coded, sole producer of ledger entries (DINV-6). Root is the
// feedback ledger root (docs/feedback); each entry lands under Root/<subject-
// stem>/ as the next dense-numbered append-only file.
type Writer struct {
	Root string
}

// NewWriter returns a Writer rooted at the feedback ledger directory.
func NewWriter(root string) *Writer { return &Writer{Root: root} }

// Capture drains a source and writes one entry per captured comment, returning
// the paths written in order. It is the sole entry-producing path — there is no
// hand-authoring alternative (DINV-6). A write failure stops the drain and is
// returned with the paths written so far; fail-open retry/queueing is the
// conversation adapter's concern (T3, off the review critical path).
func (w *Writer) Capture(ctx context.Context, src CaptureSource) ([]string, error) {
	if src == nil {
		return nil, fmt.Errorf("capture: nil source")
	}
	comments, err := src.Comments(ctx)
	if err != nil {
		return nil, fmt.Errorf("capture source: %w", err)
	}
	var paths []string
	for i, cc := range comments {
		p, err := w.WriteEntry(cc)
		if err != nil {
			return paths, fmt.Errorf("capture comment %d: %w", i, err)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// WriteEntry renders one CapturedComment into the next dense-numbered entry
// file under its subject directory and writes it. It never overwrites an
// existing file — the Writer only ever adds (AP-23/AP-27, DINV-1).
func (w *Writer) WriteEntry(cc CapturedComment) (string, error) {
	stem, err := subjectStem(cc.SubjectAnchor)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(w.Root, stem)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create subject dir %s: %w", dir, err)
	}
	seq, err := nextSeq(dir)
	if err != nil {
		return "", err
	}
	b, err := RenderEntry(seq, cc)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.md", seq, cc.SourceID))
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("entry %s already exists — the writer only adds new entries, never rewrites (DINV-1)", path)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("write entry %s: %w", path, err)
	}
	return path, nil
}

// stemRe matches a single kebab-case slug — the shape of both a subject-stem
// and a source stem. It mirrors the source group of entryNameRe so a rendered
// entry's filename is always well-formed.
var stemRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// RenderEntry renders one entry file's initial block (§3.1) from a captured
// comment: the H1 title, the Field|Value header, and the verbatim `## Comment`.
// It is pure — no filesystem, clock, or environment — so the output is a
// deterministic function of (seq, cc) and round-trips through ParseEntry. The
// parse is closed: a wrong schema version, a missing field, or a value that
// would break the header table is a rejection, never a leniently-rendered
// entry (AP-6, fail-closed). No disposition block is written — capture writes
// the initial block only; the disposition transition appends later (§3.4).
func RenderEntry(seq string, cc CapturedComment) ([]byte, error) {
	if !regexp.MustCompile(`^\d{3}$`).MatchString(seq) {
		return nil, fmt.Errorf("entry sequence %q is not three digits", seq)
	}
	if cc.SchemaVersion != CapturedCommentVersion {
		return nil, fmt.Errorf("captured comment declares schema %q, want %q", cc.SchemaVersion, CapturedCommentVersion)
	}
	if !stemRe.MatchString(cc.SourceID) {
		return nil, fmt.Errorf("source id %q is not a kebab-case stem", cc.SourceID)
	}
	if strings.TrimSpace(cc.Comment) == "" {
		return nil, fmt.Errorf("captured comment is empty; the ledger stores the review comment verbatim")
	}
	// The header fields are embedded in a Markdown table row: a `|` or a newline
	// in any of them would silently corrupt the table, so reject them here
	// rather than emit a malformed entry.
	for _, f := range []struct{ name, val string }{
		{"author", cc.Author},
		{"timestamp", cc.Timestamp},
		{"subject-anchor", cc.SubjectAnchor},
	} {
		if strings.TrimSpace(f.val) == "" {
			return nil, fmt.Errorf("captured comment %s is empty", f.name)
		}
		if strings.ContainsAny(f.val, "|\n") {
			return nil, fmt.Errorf("captured comment %s %q contains a table-breaking character ('|' or newline)", f.name, f.val)
		}
	}
	// Edit-commit is optional (a comment may produce no edit); when present it is
	// a table-cell value, so it is held to the same no-`|`/no-newline discipline.
	if cc.EditCommit != "" && strings.ContainsAny(cc.EditCommit, "|\n") {
		return nil, fmt.Errorf("captured comment edit-commit %q contains a table-breaking character ('|' or newline)", cc.EditCommit)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Review-feedback entry %s\n\n", seq)
	b.WriteString("| Field | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| **Entry** | %s |\n", seq)
	fmt.Fprintf(&b, "| **Source** | %s |\n", cc.SourceID)
	fmt.Fprintf(&b, "| **Author** | %s |\n", cc.Author)
	fmt.Fprintf(&b, "| **Timestamp** | %s |\n", cc.Timestamp)
	fmt.Fprintf(&b, "| **Subject-anchor** | %s |\n", cc.SubjectAnchor)
	if cc.EditCommit != "" {
		fmt.Fprintf(&b, "| **Edit-commit** | %s |\n", cc.EditCommit)
	}
	fmt.Fprintf(&b, "| **Template version** | %s |\n", EntryTemplateVersion)
	b.WriteString("\n## Comment\n\n")
	b.WriteString(strings.TrimRight(cc.Comment, "\n"))
	b.WriteString("\n")
	return []byte(b.String()), nil
}

// subjectStem extracts the per-subject directory stem from a subject anchor
// (`<subject-stem>#<section>`), rejecting an anchor with no valid slug.
func subjectStem(anchor string) (string, error) {
	stem, _, _ := strings.Cut(anchor, "#")
	stem = strings.TrimSpace(stem)
	if !stemRe.MatchString(stem) {
		return "", fmt.Errorf("subject anchor %q has no valid subject-stem before '#'", anchor)
	}
	return stem, nil
}

// nextSeq returns the next dense three-digit sequence for a subject directory:
// one past the highest existing NNN-<source>.md, or 001 for an empty/new dir.
func nextSeq(dir string) (string, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "001", nil
		}
		return "", fmt.Errorf("read subject dir %s: %w", dir, err)
	}
	max := 0
	for _, it := range items {
		if it.IsDir() {
			continue
		}
		seq, _, ok := parseName(it.Name())
		if !ok {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(seq, "%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%03d", max+1), nil
}
