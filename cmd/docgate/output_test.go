package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputFailureReturnsNonzero(t *testing.T) {
	base, err := os.ReadFile(filepath.Join("..", "..", "internal", "docgate", "fold", "testdata", "fixture-zero", "000-base.md"))
	if err != nil {
		t.Fatal(err)
	}
	root := docgateRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "docs", "tech-design"), 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(root, "docs", "prds", "demo")
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "000-base.md"), base, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "context-pack.yaml"), []byte("test-output:\n  - subject docs/prds/{subject}/000-base.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	commands := [][]string{
		{"coverage"},
		{"coverage", "--json"},
		{"fold", "--dry-run", ledger},
		{"context-pack", "--stage", "test-output", "--subject", "demo"},
	}
	// First prove that every fixture reaches a successful write. A setup or
	// parse error must not stand in for the output failure under test.
	for _, args := range commands {
		code, stdout, stderr := captureDocgateOutput(t, func() int { return run(args) })
		if code != 0 || stdout == "" {
			t.Fatalf("healthy %v = %d, stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}
	readOnly, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = readOnly
	t.Cleanup(func() {
		os.Stdout = previous
		if err := readOnly.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, args := range commands {
		if code := run(args); code != 2 {
			t.Fatalf("%v with failed output = %d, want 2", args, code)
		}
	}
}
