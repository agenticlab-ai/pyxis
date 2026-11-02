package lifecycle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
)

func TestDraftScaffoldStartsAuthoredRequirementHistory(t *testing.T) {
	root, _ := gitFixture(t)
	template, err := os.ReadFile("../../../.pyxis/templates/prd-template.md")
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		body, err := draftshell.Run(t.Context(), root, nil, "git", args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(body))
	}
	write := func(name string, body []byte) {
		t.Helper()
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(".pyxis/templates/prd-template.md", template)
	git("add", ".pyxis/templates/prd-template.md")
	git("commit", "-qm", "use shipped template")
	git("push", "-q", "origin", "main")
	base := git("rev-parse", "origin/main")
	files, err := scaffold(t.Context(), root, config.Defaults(), "widget", draftshell.Run)
	if err != nil {
		t.Fatal(err)
	}
	git("switch", "-c", "draft/widget", base)
	paths := make([]string, 0, len(files))
	var initial []byte
	for _, file := range files {
		write(file.Path, file.Body)
		paths = append(paths, file.Path)
		if filepath.Base(file.Path) == "prd.md" {
			initial = file.Body
		}
	}
	doc, defects := ledger.ParseProjection("prd.md", schema.Normalize(initial))
	if len(defects) != 0 || len(doc.UnitOrder) != 0 {
		t.Fatalf("scaffold introduced requirement examples: units=%v defects=%v", doc.UnitOrder, defects)
	}
	for line := range strings.SplitSeq(string(template), "\n") {
		if (strings.HasPrefix(line, "#") || strings.HasPrefix(line, "| ID |") || strings.HasPrefix(line, "| **Template version**")) && !bytes.Contains(initial, []byte(strings.ReplaceAll(line, "<Feature or capability name>", "widget"))) {
			t.Fatalf("scaffold lost template structure: %s", line)
		}
	}
	git(append([]string{"add", "--"}, paths...)...)
	git("commit", "-qm", "open draft scaffold")
	authored, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	write("drafts/widget/prd.md", authored)
	git("add", "drafts/widget/prd.md")
	git("commit", "-qm", "author first contract")
	first := git("rev-parse", "HEAD")
	repo, err := gitsrc.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := compile.CompileBranch(t.Context(), repo, base, first, "drafts", "widget")
	if err != nil || len(result.Changes) != 0 {
		t.Fatalf("first authored contract contains template changes: %v %v", result.Changes, err)
	}
	revised := bytes.Replace(authored, []byte("The widget shall open."), []byte("The widget shall reopen."), 1)
	if bytes.Equal(authored, revised) {
		t.Fatal("fixture does not contain the requirement under test")
	}
	write("drafts/widget/prd.md", revised)
	git("add", "drafts/widget/prd.md")
	git("commit", "-qm", "revise authored requirement")
	second := git("rev-parse", "HEAD")
	result, err = compile.CompileBranch(t.Context(), repo, base, second, "drafts", "widget")
	if err != nil || len(result.Changes) != 1 || result.Changes[0].ID != "FR-1" || !result.Changes[0].Unruled || result.Changes[0].FromCommit != first || result.Changes[0].ToCommit != second {
		t.Fatalf("real requirement edit lost its history: %v %v", result.Changes, err)
	}
}

func TestEmptyDraftPRDRowsKeepsNonRowContent(t *testing.T) {
	input := "# Draft\r\n\nFR-1 is an example in prose.\n| incomplete\n| Field | Value |\n| **FR-1** | example |\n| G1 *(qualitative)* | example |\n| Custom | keep |\n"
	want := "# Draft\r\n\nFR-1 is an example in prose.\n| incomplete\n| Field | Value |\n| Custom | keep |\n"
	if got := string(emptyDraftPRDRows([]byte(input))); got != want {
		t.Fatalf("non-row content changed: %q", got)
	}
}
