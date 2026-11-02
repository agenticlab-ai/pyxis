package contractgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCoverage90ResolveRejectsNilUnitsAndExercisesReadEnvironment(t *testing.T) {
	cfg := config.Defaults()
	root := t.TempDir()
	ledger := filepath.Join(root, filepath.FromSlash(cfg.PRDs), "demo")
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "effective.source.json"), []byte(`{"schema_version":"source/v1","units":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	contract, verdict := resolve(root, "demo", cfg)
	if contract != nil || verdict == nil || !strings.Contains(verdict.Reason, "no units map") {
		t.Fatalf("resolve = contract %+v verdict %+v", contract, verdict)
	}

	if got := designFingerprint(root, cfg, "missing"); got != "" {
		t.Fatalf("missing design fingerprint = %q", got)
	}
	env := designEnv(root, cfg)
	if env.DocExists("missing.md") {
		t.Fatal("missing document reported present")
	}
}

func TestCoverage90ReportFallbacksAndAtomicWriteFailure(t *testing.T) {
	report := DefectReport{Feature: "demo", Evaluated: []InputIdentity{{Kind: "source-map", Ref: "docs/prds/demo/effective.source.json"}}}
	if got := report.Markdown(); !strings.Contains(got, "(no fingerprint)") {
		t.Fatalf("Markdown missing fingerprint fallback:\n%s", got)
	}
	if got := reportBaseName(""); got != "contract-defect-report" {
		t.Fatalf("empty report base = %q", got)
	}
	if err := atomicWrite(filepath.Join(t.TempDir(), "missing", "report.json"), []byte("data")); err == nil {
		t.Fatal("atomicWrite accepted a missing sink directory")
	}
	root := t.TempDir()
	target := filepath.Join(root, "occupied")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(target, []byte("replacement")); err == nil {
		t.Fatal("atomicWrite replaced a non-empty directory")
	}
}

func TestCoverage90AffectedFeaturesIgnoresBlankPaths(t *testing.T) {
	if got := affectedFeatures(t.TempDir(), config.Defaults(), []string{"", "   "}); len(got) != 0 {
		t.Fatalf("affectedFeatures = %v", got)
	}
}

func TestCoverage90PublicGateFacesRejectMalformedConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("unknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if result := Continuous(root, []string{"docs/prds/demo/000-base.md"}); !result.Failed() || result.ConfigError == "" {
		t.Fatalf("Continuous = %+v", result)
	}
	if verdict := Run(root, "demo"); verdict.Class != ClassConfigurationError {
		t.Fatalf("Run = %+v", verdict)
	}
	if result := Sweep(root); !result.Failed() || result.ConfigError == "" {
		t.Fatalf("Sweep = %+v", result)
	}
}
