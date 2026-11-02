package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The edit is targeted because the config is repo-owned: an operator's paths,
// comments and spacing must survive a change to one knob. A re-render would
// take all of it.
func TestWithCLISurfacesPreservesEverythingElse(t *testing.T) {
	const before = `# my repo's pyxis config — hand-tuned, do not clobber
schema_version: pyxis-config/v2
system:
  version: "v0.1.471"
  mode: release
knobs:
  tracker: carina      # we run offline
  cli_surfaces: [claude]
  ci_mode: local
prds: docs/requirements
epoch:
  system_version: "v0.1.471"
`
	got := string(withCLISurfaces([]byte(before), []string{"codex", "cursor"}))

	if !strings.Contains(got, "  cli_surfaces: [codex, cursor]") {
		t.Errorf("the knob was not rewritten:\n%s", got)
	}
	if strings.Contains(got, "[claude]") {
		t.Errorf("the old value survived:\n%s", got)
	}
	for _, keep := range []string{
		"# my repo's pyxis config — hand-tuned, do not clobber",
		"  tracker: carina      # we run offline",
		"  ci_mode: local",
		"prds: docs/requirements",
		`  system_version: "v0.1.471"`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("lost %q:\n%s", keep, got)
		}
	}
	if a, b := strings.Count(before, "\n"), strings.Count(got, "\n"); a != b {
		t.Errorf("line count changed %d -> %d — the edit is not targeted", a, b)
	}
}

func TestWithCLISurfacesInsertsWhenMissing(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{
			name: "the knobs block exists without the key",
			in:   "schema_version: pyxis-config/v2\nknobs:\n  tracker: github\n",
			want: "knobs:\n  cli_surfaces: [codex]\n  tracker: github\n",
		},
		{
			name: "there is no knobs block at all",
			in:   "schema_version: pyxis-config/v2\nsystem:\n  mode: release\n",
			want: "knobs:\n  cli_surfaces: [codex]\n",
		},
		{
			name: "a later top-level key ends the block, so the key lands inside it",
			in:   "knobs:\n  tracker: github\nprds: docs/prds\n",
			want: "knobs:\n  cli_surfaces: [codex]\n  tracker: github\nprds: docs/prds\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(withCLISurfaces([]byte(tc.in), []string{"codex"}))
			if !strings.Contains(got, tc.want) {
				t.Errorf("got:\n%s\nwant it to contain:\n%s", got, tc.want)
			}
		})
	}
}

// An empty selection is a real answer — "wire nothing" — and must not leave the
// key missing, which would read as never having been asked.
func TestWithCLISurfacesWritesAnEmptyList(t *testing.T) {
	got := string(withCLISurfaces([]byte("knobs:\n  cli_surfaces: [claude, codex]\n"), nil))
	if !strings.Contains(got, "cli_surfaces: []") {
		t.Errorf("an empty selection did not write an empty list:\n%s", got)
	}
}

// The bytes are re-parsed closed before they land, so an edit that would break
// the schema fails here rather than leaving a config the engine cannot read.
func TestSetCLISurfacesRoundTripsThroughTheParser(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, relPath)
	if err := os.WriteFile(path,
		[]byte("schema_version: pyxis-config/v2\nsystem:\n  mode: release\nknobs:\n  tracker: carina\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetCLISurfaces(root, []string{"claude", "opencode"}); err != nil {
		t.Fatalf("SetCLISurfaces: %v", err)
	}
	cfg, defects := Load(root)
	if len(defects) > 0 {
		t.Fatalf("the written config does not parse: %s", defects[0].Reason)
	}
	if got := strings.Join(cfg.Knobs.CLISurfaces, ","); got != "claude,opencode" {
		t.Errorf("read back %q, want claude,opencode", got)
	}
	if cfg.Knobs.Tracker != "carina" {
		t.Errorf("the tracker knob was disturbed: %q", cfg.Knobs.Tracker)
	}
}

// A repository that was never initialized is a named refusal, not a config
// conjured out of nothing.
func TestSetCLISurfacesRefusesWithoutAConfig(t *testing.T) {
	err := SetCLISurfaces(t.TempDir(), []string{"claude"})
	if err == nil {
		t.Fatal("no error for a repo with no config")
	}
	if !strings.Contains(err.Error(), "pyxis init") {
		t.Errorf("the refusal does not route to init: %v", err)
	}
}
