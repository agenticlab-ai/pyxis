package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// The fresh-operand vacuous pass (installer-v1 §3.5 step 4 / plan installer/001
// T8, carried finding (a)): a freshly `pyxis init`-ed operand has NO living
// rollups (docs/prd.md + docs/techspec.md) — rule 3 keeps them absent until the
// first capability ships — and lint-rollups must NOT red on that absence, so the
// install's green proof passes on the seeded tree. The previous behavior reded
// with "rollup missing or unreadable"; the leg here proves the honest fix.
func TestLintRollupsFreshOperandVacuousPass(t *testing.T) {
	cfg := config.Defaults()

	writeRollup := func(t *testing.T, root, rel string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "# rollup\n\n" +
			"| Field | Value |\n|---|---|\n" +
			"| **In one line** | x |\n" +
			"| **Relevant-when** | x |\n" +
			"| **Authority** | x |\n" +
			"| **Update rule** | x |\n" +
			"| **Seeded** | x |\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("both rollups absent passes vacuously", func(t *testing.T) {
		root := t.TempDir() // a bare seeded operand: no rollups at all
		if d := gate.LintRollups(root, cfg); len(d) != 0 {
			t.Fatalf("fresh operand with no rollups reded lint-rollups: %v", d)
		}
	})

	t.Run("both rollups present and valid passes", func(t *testing.T) {
		root := t.TempDir()
		writeRollup(t, root, cfg.RollupPRD)
		writeRollup(t, root, cfg.RollupTechspec)
		if d := gate.LintRollups(root, cfg); len(d) != 0 {
			t.Fatalf("valid rollups reded: %v", d)
		}
	})

	t.Run("one rollup present the other absent is a named half-shipped defect", func(t *testing.T) {
		root := t.TempDir()
		writeRollup(t, root, cfg.RollupPRD) // techspec deliberately absent
		d := gate.LintRollups(root, cfg)
		if len(d) == 0 {
			t.Fatal("a half-present rollup pair passed — the absence leg swallowed the inconsistency")
		}
		named := false
		for _, def := range d {
			if def.File == cfg.RollupTechspec {
				named = true
			}
		}
		if !named {
			t.Errorf("the half-shipped defect does not name the absent techspec rollup: %v", d)
		}
	})

	t.Run("present but missing header rows still red", func(t *testing.T) {
		root := t.TempDir()
		writeRollup(t, root, cfg.RollupPRD)
		// A techspec present but headerless: rollups exist, so the contract runs.
		p := filepath.Join(root, filepath.FromSlash(cfg.RollupTechspec))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# just a heading, no metadata\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		d := gate.LintRollups(root, cfg)
		if len(d) == 0 {
			t.Fatal("a headerless present rollup passed — the header contract was skipped")
		}
		if !strings.Contains(d[0].Reason, "header row") && !strings.Contains(d[0].Reason, "missing") {
			t.Errorf("defect is not the header-contract failure: %v", d)
		}
	})
}
