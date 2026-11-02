package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverage90UsageAndValidationBranches(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		args []string
		code int
	}{
		{name: "unknown flag", args: []string{"create", "--root", root, "--unknown"}, code: 2},
		{name: "comment missing item", args: []string{"comment", "--root", root, "999", "--body", "x"}, code: 1},
		{name: "record missing namespace", args: []string{"record", "--root", root, "1"}, code: 1},
		{name: "list invalid parent", args: []string{"list", "--root", root, "--parent", "bad"}, code: 1},
		{name: "dep missing operands", args: []string{"dep", "--root", root, "add"}, code: 2},
		{name: "dep invalid id", args: []string{"dep", "--root", root, "add", "bad", "1"}, code: 1},
		{name: "dep unknown operation", args: []string{"dep", "--root", root, "bogus"}, code: 2},
		{name: "parent set missing operands", args: []string{"parent", "--root", root, "set"}, code: 2},
		{name: "parent set invalid child", args: []string{"parent", "--root", root, "set", "bad", "1"}, code: 1},
		{name: "parent set invalid parent", args: []string{"parent", "--root", root, "set", "1", "bad"}, code: 1},
		{name: "parent clear missing child", args: []string{"parent", "--root", root, "clear"}, code: 2},
		{name: "parent clear invalid child", args: []string{"parent", "--root", root, "clear", "bad"}, code: 1},
		{name: "parent unknown operation", args: []string{"parent", "--root", root, "bogus"}, code: 2},
		{name: "signal invalid id", args: []string{"signal", "--root", root, "add", "bad", "ready"}, code: 1},
		{name: "signal missing name", args: []string{"signal", "--root", root, "add", "1"}, code: 1},
		{name: "signal unknown operation", args: []string{"signal", "--root", root, "bogus"}, code: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, got := exec(tt.args...)
			if got != tt.code {
				t.Fatalf("exit code = %d, want %d", got, tt.code)
			}
		})
	}
}

func TestCoverage90InvalidConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("not: [valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := exec("create", "--root", root, "--title", "x"); code != 1 {
		t.Fatalf("create exit = %d, want 1", code)
	}
}

func TestCoverage90SignalVocabularyAndMultilineLog(t *testing.T) {
	root := t.TempDir()
	out, stderr, code := exec("signal", "--root", root, "def", "kind:custom", "classification", "--json")
	if code != 0 || !strings.Contains(out, `"name":"kind:custom"`) {
		t.Fatalf("signal def: code=%d stdout=%q stderr=%q", code, out, stderr)
	}

	out, stderr, code = exec("signal", "--root", root, "ls")
	if code != 0 || !strings.Contains(out, "NAME") || !strings.Contains(out, "kind:custom") {
		t.Fatalf("signal ls: code=%d stdout=%q stderr=%q", code, out, stderr)
	}

	if _, stderr, code = exec("create", "--root", root, "--title", "logged"); code != 0 {
		t.Fatalf("create: code=%d stderr=%q", code, stderr)
	}
	if _, stderr, code = exec("comment", "--root", root, "1", "--body", "first line\nsecond line"); code != 0 {
		t.Fatalf("comment: code=%d stderr=%q", code, stderr)
	}
	out, stderr, code = exec("show", "--root", root, "1")
	if code != 0 || !strings.Contains(out, "    second line") {
		t.Fatalf("show: code=%d stdout=%q stderr=%q", code, out, stderr)
	}
}
