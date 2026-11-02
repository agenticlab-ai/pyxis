package incidents_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/incidents"
)

const registry = `# Incidents registry

## Entries

**I1 (2026-07-07) [docgate, tech-design]**

- **Decision:** d
- **Broke:** b
- **Lesson:** l
- **Links:** PR #38.
`

func fixture(t *testing.T, content string) (string, config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := config.Defaults()
	p := filepath.Join(root, filepath.FromSlash(cfg.Incidents))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if content != "" {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, cfg
}

func read(t *testing.T, root string, cfg config.Config) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg.Incidents)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func entry() incidents.Entry {
	return incidents.Entry{
		Date: "2026-07-08", Tags: []string{"gate", "review-wave"},
		Decision: "d2", Broke: "b2", Lesson: "l2", Links: "issue #54.",
	}
}

func TestAddAllocatesDense(t *testing.T) {
	root, cfg := fixture(t, registry)
	id, d := incidents.Add(root, cfg, entry())
	if len(d) != 0 || id != 2 {
		t.Fatalf("want I2 clean, got id=%d defects=%v", id, d)
	}
	id, d = incidents.Add(root, cfg, entry())
	if len(d) != 0 || id != 3 {
		t.Fatalf("second add: want I3, got id=%d defects=%v", id, d)
	}
	got := read(t, root, cfg)
	if !strings.Contains(got, "**I2 (2026-07-08) [gate, review-wave]**") || !strings.Contains(got, "**I3 ") {
		t.Fatalf("entries not written:\n%s", got)
	}
	// Prior entry byte-preserved (NR-1).
	if !strings.Contains(got, "**I1 (2026-07-07) [docgate, tech-design]**") || !strings.Contains(got, "- **Links:** PR #38.") {
		t.Fatalf("I1 not preserved:\n%s", got)
	}
}

func TestAddRefusalsLeaveRegistryUntouched(t *testing.T) {
	// Empty tags → the would-be entry fails lint; nothing written (UB-1/DINV-4).
	root, cfg := fixture(t, registry)
	e := entry()
	e.Tags = nil
	if _, d := incidents.Add(root, cfg, e); len(d) == 0 {
		t.Fatal("tagless add accepted")
	}
	if read(t, root, cfg) != registry {
		t.Fatal("registry changed on refusal")
	}
	// Missing registry → refusal (UB-1).
	root2, cfg2 := fixture(t, "")
	if _, d := incidents.Add(root2, cfg2, entry()); len(d) == 0 {
		t.Fatal("missing-registry add accepted")
	}
	// Pre-corrupted registry (bad tag) → refusal naming the precondition.
	root3, cfg3 := fixture(t, strings.Replace(registry, "[docgate, tech-design]", "[Bad Tag!]", 1))
	if _, d := incidents.Add(root3, cfg3, entry()); len(d) == 0 {
		t.Fatal("write onto red registry accepted")
	}
}

func TestGraduate(t *testing.T) {
	root, cfg := fixture(t, registry)
	if d := incidents.Graduate(root, cfg, 1, "AP-9"); len(d) != 0 {
		t.Fatalf("graduate red: %v", d)
	}
	got := read(t, root, cfg)
	if !strings.Contains(got, "- **Links:** PR #38. Graduated: AP-9.") {
		t.Fatalf("Links did not grow:\n%s", got)
	}
	// Only the Links line changed: everything else byte-identical.
	if strings.Count(got, "**I1 ") != 1 || !strings.Contains(got, "- **Lesson:** l") {
		t.Fatalf("entry body changed:\n%s", got)
	}
	// Double graduation → red.
	if d := incidents.Graduate(root, cfg, 1, "AP-10"); len(d) == 0 {
		t.Fatal("double graduation accepted")
	}
	// Unknown ID → red.
	if d := incidents.Graduate(root, cfg, 7, "AP-9"); len(d) == 0 {
		t.Fatal("unknown ID accepted")
	}
}
