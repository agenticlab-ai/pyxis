package harnessconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
)

const economyProfile = "" +
	"        model: anthropic/claude-haiku-4-5\n" +
	"        effort: none\n" +
	"        capability: economy\n" +
	"        deliberation: light\n" +
	"        context: standard\n" +
	"        assurance: ordinary\n" +
	"        cost_rank: 1\n" +
	"        strongest: true\n"

const reviewProfile = "" +
	"        model: anthropic/claude-sonnet-5\n" +
	"        effort: none\n" +
	"        capability: standard\n" +
	"        deliberation: standard\n" +
	"        context: large\n" +
	"        assurance: elevated\n" +
	"        cost_rank: 1\n" +
	"        strongest: true\n"

func opencodeCatalog(modelBlock string) string {
	return "harness-config:\n  opencode:\n    implementation:\n      implementation-economy:\n" + modelBlock +
		"    review:\n      review-standard:\n" + reviewProfile
}

func TestParse_UnknownHarness(t *testing.T) {
	_, err := Parse([]byte("harness-config:\n  mood:\n    implementation:\n      economy:\n" + economyProfile))
	if !errors.Is(err, ErrUnknownHarness) {
		t.Fatalf("got %v, want ErrUnknownHarness", err)
	}
}

func TestParse_UnknownKey(t *testing.T) {
	_, err := Parse([]byte("harness-config:\n  cursor:\n    temperature: 1\n"))
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("got %v, want ErrUnknownKey", err)
	}
}

func TestParse_AbsentBlock(t *testing.T) {
	cfg, err := Parse([]byte("executor:\n  review:\n    backend: claude\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles(entity.OpenCode) != nil {
		t.Fatal("empty config should not install a model catalog")
	}
}

func TestLoad_LocalModelOverlayKeepsTheRestOfTheProfile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"config.yaml":       opencodeCatalog(economyProfile),
		"config.local.yaml": "harness-config:\n  opencode:\n    implementation:\n      implementation-economy:\n        model: opencode-go/gpt-5.6-luna\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Profiles(entity.OpenCode)["implementation"]
	if len(got) != 1 || got[0].Model != "opencode-go/gpt-5.6-luna" || got[0].Provides.Capability != executiondemand.CapabilityEconomy {
		t.Fatalf("merged profile = %+v", got)
	}
	got[0].Model = "mutated/model"
	if cfg.Profiles(entity.OpenCode)["implementation"][0].Model != "opencode-go/gpt-5.6-luna" {
		t.Fatal("caller mutated the stored model catalog")
	}
	if cfg.Profiles(entity.Cursor) != nil || (Config{}).Profiles(entity.OpenCode) != nil {
		t.Fatal("absent harness must not invent a catalog")
	}
}

func TestParse_ProfilesClosedGrammar(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"empty harness", "harness-config:\n  opencode:\n", ErrMalformed},
		{"empty role", "harness-config:\n  opencode:\n    implementation:\n", ErrMalformed},
		{"missing model", "harness-config:\n  opencode:\n    implementation:\n      economy:\n        effort: none\n", ErrMalformed},
		{"unknown level", "harness-config:\n  opencode:\n    implementation:\n      economy:\n" + "        model: p/m\n        effort: none\n        capability: beyond\n        deliberation: light\n        context: standard\n        assurance: ordinary\n        cost_rank: 1\n        strongest: true\n", ErrMalformed},
		{"two strongest", opencodeCatalog(economyProfile + "      implementation-frontier:\n        model: p/m\n        effort: none\n        capability: frontier\n        deliberation: maximum\n        context: extended\n        assurance: critical\n        cost_rank: 2\n        strongest: true\n"), ErrMalformed},
		{"unknown field", "harness-config:\n  opencode:\n    implementation:\n      economy:\n" + economyProfile + "        temperature: 1\n", ErrUnknownKey},
		{"unknown role", "harness-config:\n  opencode:\n    models:\n", ErrUnknownKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.body)); !errors.Is(err, tc.want) {
				t.Fatalf("Parse = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles(entity.OpenCode) != nil {
		t.Fatal("missing file must not install a model catalog")
	}
}

func TestLoad_FromDisk(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".pyxis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(opencodeCatalog(economyProfile)), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profiles(entity.OpenCode)["implementation"][0].Model != "anthropic/claude-haiku-4-5" {
		t.Fatalf("got %+v", cfg.Profiles(entity.OpenCode))
	}
}
