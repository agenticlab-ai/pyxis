package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
)

func TestRegistryParsingHelpersCoverFencesReferencesAndOrder(t *testing.T) {
	anchors := mapAnchors([]byte("| INV-1 | live |\n```md\n| D2 | fenced |\n```\n| D3 | live |\n"))
	if !anchors["INV-1"] || !anchors["D3"] || anchors["D2"] {
		t.Fatalf("anchors = %v", anchors)
	}
	if got := linkTargets("[inline](docs/a.md)"); !reflect.DeepEqual(got, []string{"docs/a.md"}) {
		t.Fatalf("targets = %v", got)
	}
	if got := linkTargets("[ref]: docs/b.md"); !reflect.DeepEqual(got, []string{"docs/b.md"}) {
		t.Fatalf("reference targets = %v", got)
	}
	if equalList([]string{"a"}, []string{"b"}) || equalList([]string{"a"}, nil) {
		t.Fatal("equalList accepted unequal inputs")
	}
}

func TestGateCoreHelpersReportMalformedAndMissingArtifacts(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if entries, defects := ReadEntries(missing); entries != nil || len(defects) == 0 {
		t.Fatalf("ReadEntries(missing) = %v, %v", entries, defects)
	}
	entries := []ledger.Entry{
		{Name: "000-base.md"},
		{Name: "bad.md"},
		{Name: "001-first.md"},
		{Name: "001-duplicate.md"},
		{Name: "003-gap.md"},
	}
	if defects := LintStructure("feature", entries); len(defects) < 3 {
		t.Fatalf("LintStructure defects = %v", defects)
	}
	if got := statusWord('R'); got != "renamed" {
		t.Fatalf("statusWord(R) = %q", got)
	}
	if got, defects := Diff(nil); got == "" || len(defects) == 0 {
		t.Fatalf("Diff(nil) = %q, %v", got, defects)
	}
	if got := short("short"); got != "short" {
		t.Fatalf("short() = %q", got)
	}
	if !inCorpus("AGENTS.md", config.Defaults(), true) || inCorpus("README.txt", config.Defaults(), false) {
		t.Fatal("inCorpus mishandled convention or non-markdown file")
	}

	root := t.TempDir()
	cfg := config.Defaults()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(cfg.TechDesigns)), 0o755); err != nil {
		t.Fatal(err)
	}
	defects := VerifyIndex(root, cfg)
	if len(defects) != 2 || !strings.Contains(defects[0].Reason+defects[1].Reason, "missing") {
		t.Fatalf("VerifyIndex missing artifacts = %v", defects)
	}
}
