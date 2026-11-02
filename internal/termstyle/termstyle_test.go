package termstyle

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A non-file writer (every test buffer, every pipe consumer) gets plain
// bytes — the words identical, zero escapes.
func TestDetectPlainForNonTerminalWriters(t *testing.T) {
	s := Detect(&bytes.Buffer{})
	if s.Enabled() {
		t.Fatal("a bytes.Buffer detected as a terminal")
	}
	if got := s.Bold("hello"); got != "hello" {
		t.Fatalf("plain Bold = %q, want the identity", got)
	}
}

// A regular file is not a character device: plain.
func TestDetectPlainForRegularFiles(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if Detect(f).Enabled() {
		t.Fatal("a regular file detected as a terminal")
	}
}

// NO_COLOR presence disables styling regardless of the writer — even the
// empty value counts (the no-color.org contract).
func TestNoColorDisablesEvenWhenEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if Detect(os.Stdout).Enabled() {
		t.Fatal("NO_COLOR set (empty) but styling enabled")
	}
}

// Forced styling wraps without changing the words; the plain text is a
// strict substring of the styled text.
func TestForcedWrapsWithoutChangingTheWords(t *testing.T) {
	s := Forced(true)
	styled := s.Green("green words")
	if !strings.Contains(styled, "green words") {
		t.Fatalf("styled text %q lost the words", styled)
	}
	if !strings.HasPrefix(styled, "\x1b[32m") || !strings.HasSuffix(styled, "\x1b[0m") {
		t.Fatalf("styled text %q is not wrapped in the green escape pair", styled)
	}
	if s.Bold("") != "" {
		t.Fatal("styling an empty string must stay empty")
	}
}

func TestDetectCharacterDeviceAndEveryStyle(t *testing.T) {
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
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	if !Detect(device).Enabled() {
		t.Fatal("the platform null character device was not detected as a terminal-capable writer")
	}

	styled := Forced(true)
	for name, got := range map[string]string{
		"dim": styled.Dim("text"), "cyan": styled.Cyan("text"), "yellow": styled.Yellow("text"),
	} {
		if !strings.Contains(got, "text") || !strings.HasPrefix(got, "\x1b[") || !strings.HasSuffix(got, "\x1b[0m") {
			t.Errorf("%s style = %q", name, got)
		}
	}
	plain := Forced(false)
	if plain.Dim("text") != "text" || plain.Cyan("text") != "text" || plain.Yellow("text") != "text" {
		t.Fatal("disabled styles changed plain text")
	}
}
