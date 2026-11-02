package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCoverage90ParserCompactAndCommentForms(t *testing.T) {
	p, _ := Parse([]byte("auto_classes: []\n"))
	if p.AutoClasses == nil || len(p.AutoClasses) != 0 {
		t.Fatalf("empty auto_classes = %#v", p.AutoClasses)
	}
	if got := unquote(`value # trailing comment`); got != "value" {
		t.Fatalf("unquote comment = %q", got)
	}
}

func TestCoverage90LoadFileRejectsDirectory(t *testing.T) {
	root := t.TempDir()
	rel := "policy-dir"
	if err := os.Mkdir(filepath.Join(root, rel), 0o755); err != nil {
		t.Fatal(err)
	}
	if policy, defects := LoadFile(root, rel, nil); policy != nil || len(defects) == 0 {
		t.Fatalf("LoadFile(directory) = (%+v, %+v)", policy, defects)
	}
}
