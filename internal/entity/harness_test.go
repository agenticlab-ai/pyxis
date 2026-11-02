package entity

import "testing"

func TestParseHarness(t *testing.T) {
	for _, name := range []string{"claude", "codex", "cursor", "opencode", " codex "} {
		h, err := ParseHarness(name)
		if err != nil {
			t.Fatalf("ParseHarness(%q) error = %v", name, err)
		}
		if !h.Known() || h.String() == "" {
			t.Fatalf("ParseHarness(%q) = %q, want a known harness", name, h)
		}
	}
	if _, err := ParseHarness("leaf"); err == nil {
		t.Fatal("ParseHarness(leaf) error = nil, want unknown harness")
	}
	got := Harnesses()
	if len(got) != 4 || got[0] != Claude || got[3] != OpenCode {
		t.Fatalf("Harnesses() = %v", got)
	}
}
