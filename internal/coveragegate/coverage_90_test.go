package coveragegate

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCoverage90MeasureReportsTemporaryDirectoryFailure(t *testing.T) {
	want := errors.New("temporary storage unavailable")
	old := measureMkdirTemp
	measureMkdirTemp = func(string, string) (string, error) { return "", want }
	t.Cleanup(func() { measureMkdirTemp = old })
	if _, err := Measure(t.Context(), t.TempDir(), "./...", io.Discard); !errors.Is(err, want) {
		t.Fatalf("Measure error = %v, want %v", err, want)
	}
}

func TestCoverage90MeasureRejectsUnknownPackage(t *testing.T) {
	_, err := Measure(t.Context(), t.TempDir(), "./package-that-does-not-exist", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "go test with coverage") {
		t.Fatalf("Measure unknown package error = %v", err)
	}
}
