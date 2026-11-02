package planbasis

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func header(feature, basis string) string {
	return "# Plan\n\n| Field | Value |\n|---|---|\n| **Feature** | " + feature + " |\n| **PRD** | prd/" + feature + " |\n| **Design** | design/" + feature + " |\n| **Basis** | " + basis + " |\n\n## Tasks\n\n| **Basis** | lock/ignored/v9 |\n"
}

func TestOfReadsOnlyAPlansOwnLock(t *testing.T) {
	cfg := config.Defaults()
	for _, tc := range []struct{ name, rel, body, want string }{
		{"draft-born plan", "docs/plans/widget/001-plan.md", header("widget", "lock/widget/v2"), "lock/widget/v2"},
		{"document-pipeline basis", "docs/plans/widget/001-plan.md", header("widget", "prd/widget/003"), ""},
		{"another feature's lock", "docs/plans/widget/001-plan.md", header("widget", "lock/gadget/v1"), ""},
		{"wrong feature row", "docs/plans/widget/001-plan.md", header("gadget", "lock/gadget/v1"), ""},
		{"zero version", "docs/plans/widget/001-plan.md", header("widget", "lock/widget/v0"), ""},
		{"not a plan file", "docs/plans/widget/notes.md", header("widget", "lock/widget/v1"), ""},
		{"outside plans", "docs/widget/001-plan.md", header("widget", "lock/widget/v1"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, basis := Of(tc.rel, []byte(tc.body), cfg); basis != tc.want {
				t.Fatalf("basis %q, want %q", basis, tc.want)
			}
		})
	}
	cfg.Plans = "."
	if feature := Feature("widget/001-plan.md", cfg); feature != "widget" {
		t.Fatalf("root plans directory: %q", feature)
	}
}

func TestRequiredListsEachPlanLockOnce(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	if bases, err := Required(root, cfg); err != nil || len(bases) != 0 {
		t.Fatalf("absent plans directory: %v %v", bases, err)
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/plans/widget/001-plan.md", header("widget", "lock/widget/v1"))
	write("docs/plans/widget/002-plan.md", header("widget", "lock/widget/v2"))
	write("docs/plans/widget/002-plan.json", header("widget", "lock/widget/v7"))
	write("docs/plans/gadget/001-plan.md", header("gadget", "lock/gadget/v1"))
	write("docs/plans/gadget/003-plan.md", header("gadget", "lock/gadget/v1"))
	write("docs/plans/legacy/001-plan.md", header("legacy", "prd/legacy/001"))
	write("docs/plans/README.md", "plans")
	bases, err := Required(root, cfg)
	if err != nil || !slices.Equal(bases, []string{"lock/gadget/v1", "lock/widget/v1", "lock/widget/v2"}) {
		t.Fatalf("required locks %v %v", bases, err)
	}
	if err := os.Chmod(filepath.Join(root, "docs/plans/widget/001-plan.md"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Required(root, cfg); err == nil && os.Getuid() != 0 {
		t.Fatal("an unreadable plan was skipped")
	}
}
