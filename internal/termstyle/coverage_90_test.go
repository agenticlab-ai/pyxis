package termstyle

import (
	"os"
	"testing"
)

func TestCoverage90DetectRegularFileIsPlain(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "plain")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if Detect(f).Enabled() {
		t.Fatal("regular file detected as a terminal")
	}
}

func TestCoverage90DetectClosedFileIsPlain(t *testing.T) {
	prior, hadPrior := os.LookupEnv("NO_COLOR")
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadPrior {
			_ = os.Setenv("NO_COLOR", prior)
		} else {
			_ = os.Unsetenv("NO_COLOR")
		}
	})
	f, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if Detect(f).Enabled() {
		t.Fatal("closed file detected as a terminal")
	}
}
