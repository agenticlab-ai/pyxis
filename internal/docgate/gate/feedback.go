package gate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// FeedbackDirs finds every per-subject review-feedback ledger directory under
// the configured feedback root (directories only). An absent root is not an
// error — a repo with no captured feedback simply has none.
func FeedbackDirs(root string, cfg config.Config) ([]string, error) {
	fb := filepath.Join(root, filepath.FromSlash(cfg.Feedback))
	items, err := os.ReadDir(fb)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", fb, err)
	}
	var dirs []string
	for _, it := range items {
		if it.IsDir() {
			dirs = append(dirs, filepath.Join(fb, it.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// IsFeedbackDir reports whether dir resolves under the configured feedback
// root — the class discriminator the fold and verify-generated verbs dispatch
// on (a feedback ledger folds to a projection only, never an effective.md).
func IsFeedbackDir(root string, cfg config.Config, dir string) bool {
	fb := filepath.Clean(filepath.Join(root, filepath.FromSlash(cfg.Feedback)))
	abs := dir
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, dir)
	}
	abs = filepath.Clean(abs)
	rel, err := filepath.Rel(fb, abs)
	if err != nil {
		return false
	}
	// A subject ledger lives under the feedback root; the root itself
	// (rel == ".") is not a ledger, and an escaping path (".." prefix) is out.
	return rel != "." && !strings.HasPrefix(rel, "..")
}

// ReadFeedbackEntries loads a subject directory's entry files, sorted by name.
// The generated projection is skipped — it is output, never input.
func ReadFeedbackEntries(dir string) ([]feedback.Entry, []schema.Defect) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, []schema.Defect{def("fold", dir, "read", err.Error(), "the ledger directory must be readable")}
	}
	var entries []feedback.Entry
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !entryRe.MatchString(name) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, []schema.Defect{def("fold", name, "read", err.Error(), "the entry must be readable")}
		}
		entries = append(entries, feedback.Entry{Name: name, Bytes: b})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// VerifyFeedbackGenerated recomputes the fold and byte-compares the committed
// projection artifact (§3.1 byte-verify contract; a stale projection reds).
func VerifyFeedbackGenerated(dir string, res feedback.Result) []schema.Defect {
	var defects []schema.Defect
	got, err := os.ReadFile(filepath.Join(dir, feedback.ProjectionFile))
	if err != nil {
		return append(defects, def("verify-generated", filepath.Base(dir)+"/"+feedback.ProjectionFile, "compare",
			"projection artifact missing", "run `docgate fold` and commit the regenerated projection"))
	}
	if !bytes.Equal(got, res.SourceJSON) {
		return append(defects, def("verify-generated", filepath.Base(dir)+"/"+feedback.ProjectionFile, "compare",
			"committed projection differs from the recomputed fold",
			"never hand-edit the projection: fix the entry or the fold, then `docgate fold` and recommit"))
	}
	return defects
}
