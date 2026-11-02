package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/entity"
)

func TestPrototypeConfiguration(t *testing.T) {
	for _, generation := range []string{"", "schema_version: pyxis-config/v2\n"} {
		for _, harness := range entity.Harnesses() {
			source := generation + "prototype:\n  harness: " + harness.String() + "\n  profile: local-build\n"
			cfg, defects := config.Parse([]byte(source))
			if len(defects) != 0 || cfg.Prototype == nil || cfg.Prototype.Harness != harness || cfg.Prototype.Profile != "local-build" {
				t.Fatalf("%q: prototype = %+v, defects = %v", source, cfg.Prototype, defects)
			}
		}
	}
	if cfg, defects := config.Parse(nil); len(defects) != 0 || !reflect.DeepEqual(cfg, config.Defaults()) || cfg.Prototype != nil {
		t.Fatalf("absent block changed defaults: %+v, %v", cfg, defects)
	}
}

func TestPrototypeConfigurationDefects(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"harness alone", "prototype:\n  harness: codex\n", "prototype requires both harness and profile"},
		{"profile alone", "prototype:\n  profile: local-build\n", "prototype requires both harness and profile"},
		{"empty block", "prototype:\n", "prototype requires both harness and profile"},
		{"unknown harness", "prototype:\n  harness: future\n  profile: local-build\n", "unknown prototype.harness"},
		{"unknown key", "prototype:\n  harness: codex\n  profile: local-build\n  model: wrong\n", `unknown key "model" in prototype`},
		{"empty profile", "prototype:\n  harness: codex\n  profile:\n", "prototype.profile requires a non-empty scalar"},
		{"empty harness", "prototype:\n  harness:\n  profile: local-build\n", "prototype.harness requires a non-empty scalar"},
		{"scalar block", "prototype: codex\n", "prototype must be a block"},
		{"nested profile", "prototype:\n  harness: codex\n  profile:\n    model: wrong\n", "structure in prototype"},
		{"flow profile", "prototype:\n  harness: codex\n  profile: [local-build]\n", "prototype.profile requires a non-empty scalar"},
		{"duplicate field", "prototype:\n  harness: codex\n  profile: local-build\n  profile: other\n", "duplicate prototype.profile"},
		{"duplicate block", "prototype:\n  harness: codex\n  profile: local-build\nprototype:\n  harness: claude\n  profile: other\n", "duplicate prototype block"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, defects := config.Parse([]byte(tc.source))
			for _, defect := range defects {
				if defect.Verb == "config" && strings.Contains(defect.Reason, tc.want) {
					return
				}
			}
			t.Fatalf("want named config defect %q, got %v", tc.want, defects)
		})
	}
}

func TestPrototypeConfigurationOverlay(t *testing.T) {
	for _, global := range []string{"", "prototype:\n  harness: claude\n  profile: tracked-build\n"} {
		root := t.TempDir()
		dir := filepath.Join(root, ".pyxis")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		for name, source := range map[string]string{
			"config.yaml":       global,
			"config.local.yaml": "prototype:\n  harness: codex\n  profile: local-build\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		cfg, defects := config.Load(root)
		if len(defects) != 0 || cfg.Prototype == nil || cfg.Prototype.Harness != entity.Codex || cfg.Prototype.Profile != "local-build" {
			t.Fatalf("overlay: %+v, %v", cfg.Prototype, defects)
		}
	}
}
