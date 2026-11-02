package coveragegate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMeasureRunsAnInstrumentedGoSuite(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.com/coveragefixture\n\ngo 1.25\n",
		"value.go":      "package value\n\nfunc Value(ok bool) int { if ok { return 1 }; return 0 }\n",
		"value_test.go": "package value\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value(true) != 1 { t.Fatal() } }\n",
		"race_test.go":  "//go:build race\n\npackage value\n\nimport \"testing\"\n\nfunc TestRaceDisabled(t *testing.T) { t.Fatal(\"coverage suite must run without race detection\") }\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var output strings.Builder
	report, err := Measure(t.Context(), dir, "./...", &output)
	if err != nil {
		t.Fatalf("Measure: %v\n%s", err, output.String())
	}
	if report.Total == 0 || report.Covered == 0 || len(report.Files) != 1 {
		t.Fatalf("report = %+v", report)
	}
	var second strings.Builder
	cached, err := Measure(t.Context(), dir, "./...", &second)
	if err != nil {
		t.Fatalf("cached Measure: %v\n%s", err, second.String())
	}
	if cached.Total != report.Total || cached.Covered != report.Covered || len(cached.Files) != len(report.Files) {
		t.Fatalf("cached report = %+v want %+v", cached, report)
	}
}

func TestMeasureRejectsEmptyPatternAndTestFailure(t *testing.T) {
	if _, err := Measure(t.Context(), t.TempDir(), "", io.Discard); err == nil {
		t.Fatal("empty package pattern succeeded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Measure(ctx, t.TempDir(), "./...", io.Discard); err == nil || !strings.Contains(err.Error(), "go test with coverage") {
		t.Fatalf("cancelled measurement error = %v", err)
	}
}
