package harnessconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

func TestParseRejectsAmbiguousCatalogStructure(t *testing.T) {
	const role = "harness-config:\n  opencode:\n    implementation:\n"
	const profile = role + "      economy:\n"
	for _, tc := range []struct {
		name, body, detail string
	}{
		{"tab indentation", "harness-config:\n\topencode:\n", "tab in indentation"},
		{"nonmapping root", "harness-config\n", "not a mapping key"},
		{"scalar catalog", "harness-config: scalar\n", "not a scalar"},
		{"odd indentation", "harness-config:\n   opencode:\n", "unexpected indentation"},
		{"nested field", profile + economyProfile + "          model: p/m\n", "unexpected indentation"},
		{"scalar harness", "harness-config:\n  opencode: scalar\n", "expected harness name block"},
		{"duplicate harness", opencodeCatalog(economyProfile) + "  opencode:\n", "duplicate harness"},
		{"role without harness", "harness-config:\n    implementation:\n", "dangling key"},
		{"nonmapping role", "harness-config:\n  opencode:\n    implementation\n", "expected implementation or review block"},
		{"scalar role", "harness-config:\n  opencode:\n    implementation: scalar\n", "expected implementation or review block"},
		{"duplicate role", profile + economyProfile + "    implementation:\n", "duplicate role"},
		{"profile without role", "harness-config:\n  opencode:\n      economy:\n", "profile outside a role"},
		{"scalar profile", role + "      economy: scalar\n", "expected a profile block"},
		{"duplicate profile", profile + economyProfile + "      economy:\n" + economyProfile, "duplicate profile"},
		{"field without profile", role + "        model: p/m\n", "profile field outside a profile"},
		{"empty field", profile + "        model:\n", "needs a value"},
		{"nonmapping field", profile + "        model\n", "needs a value"},
		{"field name with spaces", profile + "        model name: p/m\n", "needs a value"},
		{"duplicate field", profile + economyProfile + "        model: p/m\n", "duplicate field"},
		{"duplicate cost rank", profile + economyProfile + "      second:\n" + strings.ReplaceAll(economyProfile, "strongest: true", "strongest: false"), "cost_rank 1 is used"},
		{"missing strongest", profile + strings.ReplaceAll(economyProfile, "        strongest: true\n", ""), "exactly one strongest"},
		{"invalid harness before sibling", "harness-config:\n  opencode:\nexecutor:\n", "has no roles"},
		{"invalid role before next harness", role + "  codex:\n", "has no profiles"},
		{"incomplete profile before next role", profile + "        model: p/m\n    review:\n", "requires effort"},
		{"incomplete profile before next profile", profile + "        model: p/m\n      second:\n", "requires effort"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.body))
			if !errors.Is(err, ErrMalformed) || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("Parse = %v, want ErrMalformed containing %q", err, tc.detail)
			}
		})
	}
}

func TestParseRequiresEveryProfileField(t *testing.T) {
	for _, field := range []string{"model", "effort", "capability", "deliberation", "context", "assurance", "cost_rank"} {
		t.Run(field, func(t *testing.T) {
			var body strings.Builder
			for line := range strings.SplitSeq(economyProfile, "\n") {
				if !strings.HasPrefix(line, "        "+field+":") {
					body.WriteString(line + "\n")
				}
			}
			_, err := Parse([]byte(opencodeCatalog(body.String())))
			if !errors.Is(err, ErrMalformed) || !strings.Contains(err.Error(), "requires "+field) {
				t.Fatalf("Parse = %v, want missing %s", err, field)
			}
		})
	}
}

func TestParseRejectsInvalidProfileValues(t *testing.T) {
	for _, tc := range []struct{ field, original, value, detail string }{
		{"model", "anthropic/claude-haiku-4-5", "p/m with-space", "single tokens"},
		{"effort", "none", "deep thinking", "single tokens"},
		{"deliberation", "light", "unknown", "unknown requirement level"},
		{"context", "standard", "unknown", "unknown requirement level"},
		{"assurance", "ordinary", "unknown", "unknown requirement level"},
		{"cost_rank", "1", "0", "positive integer"},
		{"cost_rank", "1", "invalid", "positive integer"},
		{"strongest", "true", "invalid", "true or false"},
	} {
		t.Run(tc.field+"/"+tc.value, func(t *testing.T) {
			body := strings.ReplaceAll(economyProfile, tc.field+": "+tc.original, tc.field+": "+tc.value)
			_, err := Parse([]byte(opencodeCatalog(body)))
			if !errors.Is(err, ErrMalformed) || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("Parse = %v, want ErrMalformed containing %q", err, tc.detail)
			}
		})
	}
}

func TestLoadRejectsMalformedOverlay(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(opencodeCatalog(economyProfile)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.local.yaml"), []byte("harness-config:\n\topencode:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "load configuration") {
		t.Fatalf("Load = %v, want configuration load error", err)
	}
}

func TestLoadDefaultErrors(t *testing.T) {
	for _, tc := range []struct {
		name, body, detail string
		missing, directory bool
	}{
		{name: "outside operand", missing: true, detail: "no repo root found"},
		{name: "config is directory", directory: true, detail: "read .pyxis/config.yaml"},
		{name: "malformed catalog", body: "harness-config: scalar\n", detail: "not a scalar"},
		{name: "missing harness", body: "executor:\n  review:\n    backend: claude\n", detail: "opencode has no model profiles"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ".pyxis", "config.yaml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.directory {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if !tc.missing {
				if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(root)
			t.Setenv("PATH", t.TempDir())
			if _, err := LoadDefault(entity.OpenCode); err == nil || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("LoadDefault = %v, want error containing %q", err, tc.detail)
			}
		})
	}
}

func TestDefaultAndExplicitCatalogOwnership(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"config.yaml":       opencodeCatalog(economyProfile) + "executor:\n  review:\n    backend: opencode\n",
		"config.local.yaml": "harness-config:\n  opencode:\n    implementation:\n      implementation-economy:\n        model: local/override\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	defaults, err := ResolveProfiles(entity.OpenCode, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := defaults[nativerouting.RoleImplementation][0].Model; got != "anthropic/claude-haiku-4-5" {
		t.Fatalf("default model = %q, want tracked model", got)
	}
	merged, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	profiles := merged.Profiles(entity.OpenCode)
	resolved, err := ResolveProfiles(entity.OpenCode, true, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved[nativerouting.RoleImplementation][0].Model; got != "local/override" {
		t.Fatalf("explicit model = %q, want merged local override", got)
	}
	resolved[nativerouting.RoleImplementation][0].Model = "mutated/model"
	if profiles[nativerouting.RoleImplementation][0].Model != "local/override" {
		t.Fatal("resolved catalog aliases caller's profiles")
	}
	if _, err := ResolveProfiles(entity.OpenCode, true, nil); err == nil || !strings.Contains(err.Error(), "has no model profiles") {
		t.Fatalf("empty explicit catalog = %v, want missing catalog error", err)
	}
}
