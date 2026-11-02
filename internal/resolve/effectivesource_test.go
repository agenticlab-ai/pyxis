package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// record writes an installer release.env carrying channel and points
// XDG_CONFIG_HOME at it, which is exactly how a mirror install leaves the
// machine. An empty channel writes the pin alone — the pre-FR-30 record.
func record(t *testing.T, channel string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "PYXIS_SYSTEM_VERSION=v1.2.3\nPYXIS_MANIFEST_SHA256=abc\n"
	if channel != "" {
		body += "PYXIS_CHANNEL=" + channel + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "pyxis", "release.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

// The one precedence (prd:installer/FR-30), driven through the real seams —
// environment, repository config, and the installer's own record.
func TestEffectiveSourcePrecedence(t *testing.T) {
	const (
		root    = "https://channel.example.com/pyxis"
		mirror  = "https://mirror.example.com/pyxis"
		envSrc  = "https://env.example.com/pyxis"
		pubBase = resolve.DefaultChannelRoot + "/v1.2.3" // public GitHub Releases
		retired = "https://dist.pyxis.agenticlab.ai"
	)
	tests := []struct {
		name        string
		env, chEnv  string
		cfgSrc, rec string
		want        string
		wantDefault bool
	}{
		{
			name: "nothing recorded falls back to public GitHub Releases",
			want: pubBase, wantDefault: true,
		},
		{
			name: "a recorded retired channel falls back to the built-in channel",
			rec:  retired, want: pubBase, wantDefault: true,
		},
		{
			name:   "a configured retired source falls through to the record",
			cfgSrc: retired + "/v1.2.3", rec: root, want: root + "/v1.2.3",
		},
		{
			name:   "a configured retired source with no record is the built-in channel",
			cfgSrc: retired + "/v1.2.3", want: pubBase, wantDefault: true,
		},
		{
			name: "the recorded channel composes with the version",
			rec:  root, want: root + "/v1.2.3",
		},
		{
			// A setting that lived in the environment would not survive a
			// restart, so the environment is not where settings live. Both
			// variables are set here and neither is consulted.
			name:  "the environment does not override the record",
			rec:   root,
			chEnv: mirror, env: envSrc, want: root + "/v1.2.3",
		},
		{
			name:   "repository config outranks the recorded channel",
			rec:    root,
			cfgSrc: mirror, want: mirror,
		},
		{
			name: "the environment does not override repository config",
			rec:  root, cfgSrc: mirror,
			env: envSrc, want: mirror,
		},
		{
			name: "trailing slashes never double up",
			rec:  root + "/", want: root + "/v1.2.3",
		},
		{
			name:   "a configured source stays flat — no version appended",
			cfgSrc: mirror + "/", want: mirror,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record(t, tc.rec)
			t.Setenv("PYXIS_ARTIFACT_SOURCE", tc.env)
			t.Setenv("PYXIS_CHANNEL", tc.chEnv)
			cfg := resolve.Config{Repo: config.Config{System: config.System{ArtifactSource: tc.cfgSrc}}}
			got, isDefault := resolve.EffectiveSource(cfg, "v1.2.3")
			if got != tc.want {
				t.Errorf("base = %s, want %s", got, tc.want)
			}
			if isDefault != tc.wantDefault {
				t.Errorf("isDefault = %v, want %v", isDefault, tc.wantDefault)
			}
		})
	}
}

// A hosted runner has no machine record, so what it resolves, and what a workflow
// committed for it may carry, comes from the repository alone.
func TestRepositorySourceIgnoresTheMachineRecord(t *testing.T) {
	const (
		mirror = "https://mirror.example.com/pyxis"
		bound  = "https://forge.example.com/o/binaries/releases/download/v1.2.3"
		public = resolve.DefaultChannelRoot + "/v1.2.3"
	)
	binding := `integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: https://forge.example.com/
  bindings:
    artifacts:
      connection: local
      repository: o/binaries
`
	tests := []struct {
		name, cfgSrc, profile, want string
	}{
		{name: "no repository source is the public channel", want: public},
		{name: "the configured source stays flat", cfgSrc: mirror + "/", want: mirror},
		{name: "the artifact binding", profile: binding, want: bound},
		{name: "the configured source outranks the binding", cfgSrc: mirror, profile: binding, want: mirror},
		{name: "a retired source counts as absent", cfgSrc: "https://dist.pyxis.agenticlab.ai", want: public},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record(t, "https://channel.example.com/pyxis")
			root := t.TempDir()
			if tc.profile != "" {
				if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(tc.profile), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cfg := resolve.Config{Root: root, Repo: config.Config{System: config.System{ArtifactSource: tc.cfgSrc}}}
			got, err := resolve.RepositorySource(cfg, "v1.2.3")
			if err != nil || got != tc.want {
				t.Fatalf("RepositorySource = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("integrations:\n  invalid: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve.RepositorySource(resolve.Config{Root: root}, "v1.2.3"); err == nil {
		t.Fatal("an invalid integrations profile resolved a source")
	}
}

// The reason the record holds a ROOT and not a source: one record has to keep
// serving after the version moves. A recorded <root>/<tag> would pin the next
// upgrade to the tag before it.
func TestRecordedChannelCarriesNoVersion(t *testing.T) {
	const root = "https://channel.example.com/pyxis"
	record(t, root)
	t.Setenv("PYXIS_ARTIFACT_SOURCE", "")
	t.Setenv("PYXIS_CHANNEL", "")

	cfg := resolve.Config{}
	for _, ver := range []string{"v0.1.47", "v0.2.0", "v9.9.9"} {
		got, isDefault := resolve.EffectiveSource(cfg, ver)
		if want := root + "/" + ver; got != want {
			t.Errorf("%s: base = %s, want %s", ver, got, want)
		}
		if isDefault {
			t.Errorf("%s: reported as the built-in default", ver)
		}
	}
}

// The host-only form the credential-attachment gate asks for: no version, and
// never a bare trailing slash for it to mis-parse.
func TestEffectiveSourceWithoutAVersion(t *testing.T) {
	const root = "https://channel.example.com/pyxis"
	record(t, root)
	t.Setenv("PYXIS_ARTIFACT_SOURCE", "")
	t.Setenv("PYXIS_CHANNEL", "")
	if got, _ := resolve.EffectiveSource(resolve.Config{}, ""); got != root {
		t.Errorf("base = %s, want %s", got, root)
	}

	record(t, "")
	got, isDefault := resolve.EffectiveSource(resolve.Config{}, "")
	if got != resolve.DefaultChannelRoot || !isDefault {
		t.Errorf("default base = %s (isDefault %v), want the public channel as the built-in default", got, isDefault)
	}
}

// recordFlat writes the record an air-gapped or one-release install leaves:
// no channel, because there was no root to derive, and the flat base it
// actually fetched from instead.
func recordFlat(t *testing.T, version, source string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "PYXIS_SYSTEM_VERSION=" + version + "\nPYXIS_MANIFEST_SHA256=abc\n" +
		"PYXIS_ARTIFACT_SOURCE=" + source + "\n"
	if err := os.WriteFile(filepath.Join(dir, "pyxis", "release.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

// The offline adopter: a locked-down machine fetches one release from a flat
// directory, and every later verb has to find it there. The release host is
// the one place that machine cannot reach, so falling back to it is the same
// failure the recorded channel exists to prevent.
func TestRecordedFlatSourceServesTheReleaseItNames(t *testing.T) {
	const mirror = "file:///srv/pyxis-artifacts"
	recordFlat(t, "v0.1.471", mirror)
	t.Setenv("PYXIS_ARTIFACT_SOURCE", "")
	t.Setenv("PYXIS_CHANNEL", "")

	got, isDefault := resolve.EffectiveSource(resolve.Config{}, "v0.1.471")
	if got != mirror || isDefault {
		t.Errorf("base = %s (isDefault %v), want the recorded source", got, isDefault)
	}
	// Host-only, for the credential gate: no version to check against.
	if got, _ := resolve.EffectiveSource(resolve.Config{}, ""); got != mirror {
		t.Errorf("host-only base = %s, want %s", got, mirror)
	}
}

// A flat base names ONE release's directory. Honouring it for another version
// would point at bytes that were never there, so the record steps aside and
// the ordinary default answers — a reachable wrong answer beats a confident
// one.
func TestRecordedFlatSourceStepsAsideForAnotherRelease(t *testing.T) {
	recordFlat(t, "v0.1.471", "file:///srv/pyxis-artifacts")
	t.Setenv("PYXIS_ARTIFACT_SOURCE", "")
	t.Setenv("PYXIS_CHANNEL", "")

	got, isDefault := resolve.EffectiveSource(resolve.Config{}, "v0.2.0")
	if !isDefault {
		t.Errorf("base = %s, want the built-in default for a release the record cannot describe", got)
	}
}

// Both recorded is not a state install.sh writes — it records a root or a flat
// base, never two answers. If one ever appeared, the channel wins: it is the
// only one that survives a version move.
func TestRecordedChannelOutranksARecordedFlatSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "PYXIS_SYSTEM_VERSION=v0.1.471\nPYXIS_MANIFEST_SHA256=abc\n" +
		"PYXIS_CHANNEL=https://channel.example.com/pyxis\n" +
		"PYXIS_ARTIFACT_SOURCE=file:///srv/pyxis-artifacts\n"
	if err := os.WriteFile(filepath.Join(dir, "pyxis", "release.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("PYXIS_ARTIFACT_SOURCE", "")
	t.Setenv("PYXIS_CHANNEL", "")

	if got, _ := resolve.EffectiveSource(resolve.Config{}, "v0.1.471"); got != "https://channel.example.com/pyxis/v0.1.471" {
		t.Errorf("base = %s, want the composed channel", got)
	}
}
