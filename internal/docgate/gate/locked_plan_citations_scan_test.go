package gate

import (
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCitationScanIncludesConfiguredPlansOutsideDocs(t *testing.T) {
	root, _ := epochRepo(t)
	cfg := config.Defaults()
	cfg.Plans = "contracts/plans"
	first := "contracts/plans/widget/001-plan.md"
	second := "contracts/plans/widget/002-plan.md"
	writeAt(t, root, first, "# first\n")
	writeAt(t, root, second, "# second\n")
	if files := citationScanFiles(root, cfg, ScanScope{}); len(files) != 2 {
		t.Fatalf("full citation scan missed configured plans: %v", files)
	}
	scope := ScanScope{epochActive: true, changed: map[string]bool{first: true}}
	if files := citationScanFiles(root, cfg, scope); len(files) != 1 || files[0] != filepath.Join(root, first) {
		t.Fatalf("epoch citation scan did not select changed plan: %v", files)
	}
}
