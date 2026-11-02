package contextpack

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCoverage90ResolutionReadAndStageBoundaries(t *testing.T) {
	if pack, defects := Build(t.TempDir(), "missing-stage", "subject", Declaration{}); pack != nil || len(defects) == 0 {
		t.Fatalf("Build unknown stage = (%q, %+v)", pack, defects)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "subject.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := Declaration{"stage": {{Kind: KindSubject, Selector: "subject.md"}}}
	if pack, defects := Build(root, "stage", "subject", d); pack != nil || len(defects) == 0 {
		t.Fatalf("Build directory source = (%q, %+v)", pack, defects)
	}

	if got := Stages(Declaration{"z": nil, "a": nil}); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("Stages = %v", got)
	}
}
