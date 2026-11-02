package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/config"
)

func TestReviewEvidenceRefsPreserveDefaultsAndIndependentOverrides(t *testing.T) {
	t.Parallel()
	const (
		obligations = "refs/pyxis/implement-now-review-state"
		process     = "refs/pyxis/impl-review-process-state"
		publication = "refs/pyxis/impl-review-publication-state"
	)
	for _, tc := range []struct {
		name, body                   string
		wantObligations, wantProcess string
		wantPublication              string
	}{
		{"absent", "", obligations, process, publication},
		{"legacy review", "    backend: codex\n", obligations, process, publication},
		{"obligations only", "    evidence_ref: refs/pyxis/run/obligations\n", "refs/pyxis/run/obligations", process, publication},
		{"process only", "    process_evidence_ref: refs/pyxis/run/process\n", obligations, "refs/pyxis/run/process", publication},
		{"publication only", "    publication_evidence_ref: refs/pyxis/run/publication\n", obligations, process, "refs/pyxis/run/publication"},
		{"all selected", "    evidence_ref: refs/pyxis/run/obligations\n    process_evidence_ref: refs/pyxis/run/process\n    publication_evidence_ref: refs/pyxis/run/publication\n", "refs/pyxis/run/obligations", "refs/pyxis/run/process", "refs/pyxis/run/publication"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.body
			if source != "" {
				source = "executor:\n  review:\n" + source
			}
			cfg, err := config.Parse([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ReviewEvidenceRef() != tc.wantObligations || cfg.ReviewProcessEvidenceRef() != tc.wantProcess || cfg.ReviewPublicationEvidenceRef() != tc.wantPublication {
				t.Fatalf("review refs = %q / %q / %q; want %q / %q / %q", cfg.ReviewEvidenceRef(), cfg.ReviewProcessEvidenceRef(), cfg.ReviewPublicationEvidenceRef(), tc.wantObligations, tc.wantProcess, tc.wantPublication)
			}
			if cfg.ReviewConfigured() != (tc.body != "") {
				t.Fatal("evidence defaults changed whether review is configured")
			}
		})
	}
}

func TestReviewEvidenceRefsRejectUnsafeRefsAndUnknownKeys(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"process_evidence_ref", "publication_evidence_ref"} {
		t.Run(key, func(t *testing.T) {
			for _, ref := range []string{"", "refs/heads/main", "refs/pyxis/../main", "refs/pyxis/has space", "refs/pyxis/name.lock", "refs/pyxis/.hidden", "refs/pyxis/name/", "refs/pyxis/name@{1}", "refs/pyxis//name"} {
				_, err := config.Parse([]byte("executor:\n  review:\n    " + key + ": " + ref + "\n"))
				if !errors.Is(err, config.ErrMalformed) {
					t.Errorf("unsafe ref %q: got %v, want ErrMalformed", ref, err)
				}
			}
		})
	}
	for _, key := range []string{"process_ref", "publication_ref", "process_evidence_refs", "publication_evidence_refs"} {
		_, err := config.Parse([]byte("executor:\n  review:\n    " + key + ": refs/pyxis/run/review\n"))
		if !errors.Is(err, config.ErrUnknownKey) {
			t.Errorf("unknown review key %q: got %v, want ErrUnknownKey", key, err)
		}
	}
}

func TestReviewEvidenceRefsMergeLocalProfileWithoutReplacingOtherStores(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"config.yaml":       "executor:\n  review:\n    backend: codex\n    evidence_ref: refs/pyxis/shared/obligations\n    process_evidence_ref: refs/pyxis/shared/process\n    publication_evidence_ref: refs/pyxis/shared/publication\n",
		"config.local.yaml": "executor:\n  review:\n    process_evidence_ref: refs/pyxis/local/process\n    publication_evidence_ref: refs/pyxis/local/publication\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReviewHarness() != "codex" || cfg.ReviewEvidenceRef() != "refs/pyxis/shared/obligations" || cfg.ReviewProcessEvidenceRef() != "refs/pyxis/local/process" || cfg.ReviewPublicationEvidenceRef() != "refs/pyxis/local/publication" {
		t.Fatalf("local review refs did not merge independently: backend %q; refs %q / %q / %q", cfg.ReviewHarness(), cfg.ReviewEvidenceRef(), cfg.ReviewProcessEvidenceRef(), cfg.ReviewPublicationEvidenceRef())
	}
}
