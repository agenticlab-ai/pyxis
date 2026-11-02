package releaseenv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
)

func write(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyxis", "release.env"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestRead(t *testing.T) {
	tests := []struct {
		name, body   string
		ver, sha, ch string
	}{
		{
			name: "the full record an installer writes",
			body: "PYXIS_SYSTEM_VERSION=v0.1.47\nPYXIS_MANIFEST_SHA256=deadbeef\nPYXIS_CHANNEL=https://mirror.example.com/pyxis\n",
			ver:  "v0.1.47", sha: "deadbeef", ch: "https://mirror.example.com/pyxis",
		},
		{
			name: "a pre-FR-30 record still reads — the channel is simply absent",
			body: "PYXIS_SYSTEM_VERSION=v0.1.46\nPYXIS_MANIFEST_SHA256=cafe\n",
			ver:  "v0.1.46", sha: "cafe",
		},
		{
			name: "a trailing slash on the channel is normalised away",
			body: "PYXIS_CHANNEL=https://mirror.example.com/pyxis/\n",
			ch:   "https://mirror.example.com/pyxis",
		},
		{
			name: "surrounding whitespace and unknown keys are tolerated",
			body: "# written by install.sh\nPYXIS_SYSTEM_VERSION = v0.2.0 \nPYXIS_FUTURE_KEY=x\n\n",
			ver:  "v0.2.0",
		},
		{
			// An older install stored the retired host. It reads as not recorded,
			// so the public default answers and the machine is not stranded.
			name: "the retired channel reads as absent",
			body: "PYXIS_SYSTEM_VERSION=v0.1.47\nPYXIS_MANIFEST_SHA256=deadbeef\nPYXIS_CHANNEL=https://dist.pyxis.agenticlab.ai\n",
			ver:  "v0.1.47", sha: "deadbeef",
		},
		{name: "an empty file yields the zero record"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			write(t, tc.body)
			got := releaseenv.Read()
			if got.Version != tc.ver || got.ManifestSHA != tc.sha || got.Channel != tc.ch {
				t.Errorf("Read() = %+v, want {%s %s %s}", got, tc.ver, tc.sha, tc.ch)
			}
		})
	}
}

// An absent file is the ordinary case on a machine installed some other way:
// the zero record, never an error, so callers fall through to their next rank.
func TestReadWithoutAFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := (releaseenv.Read()); got != (releaseenv.Record{}) {
		t.Errorf("Read() = %+v, want the zero record", got)
	}
}

func TestPathHonoursXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if want := filepath.Join(dir, "pyxis", "release.env"); releaseenv.Path() != want {
		t.Errorf("Path() = %s, want %s", releaseenv.Path(), want)
	}
}

func TestPathFallsBackToHomeAndReadWithoutHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if want := filepath.Join(home, ".config", "pyxis", "release.env"); releaseenv.Path() != want {
		t.Fatalf("home fallback = %q, want %q", releaseenv.Path(), want)
	}
	t.Setenv("HOME", "")
	if releaseenv.Dir() != "" || releaseenv.Path() != "" || releaseenv.Read() != (releaseenv.Record{}) {
		t.Fatal("missing home did not produce an absent release record")
	}
}

// Channel is the one owner of "which channel did this machine install from":
// the environment redirect first, then the record, then nothing.
func TestChannel(t *testing.T) {
	tests := []struct {
		name, body, env, want string
	}{
		{name: "the recorded channel", body: "PYXIS_CHANNEL=https://dist.example\n", want: "https://dist.example"},
		// The environment is not where a setting lives: it does not survive a
		// restart. An operator changes the channel with a flag, which writes
		// back to the record.
		{name: "the environment does not override the record", body: "PYXIS_CHANNEL=https://dist.example\n", env: "https://env.example", want: "https://dist.example"},
		{name: "the environment alone decides nothing", env: "https://env.example/", want: ""},
		{name: "no record yields nothing, which selects the GitHub leg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			write(t, tc.body)
			t.Setenv("PYXIS_CHANNEL", tc.env)
			if got := releaseenv.Channel(); got != tc.want {
				t.Errorf("Channel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRetiredFlatSourceReadsAsAbsent(t *testing.T) {
	write(t, "PYXIS_SYSTEM_VERSION=v0.1.47\nPYXIS_ARTIFACT_SOURCE=https://dist.pyxis.agenticlab.ai/v0.1.47\n")
	if got := releaseenv.Read().ArtifactSource; got != "" {
		t.Errorf("ArtifactSource = %q, want the retired host dropped", got)
	}
}

func TestIsRetiredChannel(t *testing.T) {
	for source, want := range map[string]bool{
		"https://dist.pyxis.agenticlab.ai":         true,
		"https://dist.pyxis.agenticlab.ai/":        true,
		"https://DIST.pyxis.agenticlab.ai/v0.1.47": true,
		" https://dist.pyxis.agenticlab.ai ":       true,
		"https://mirror.example.com/pyxis":         false,
		"https://github.com/agenticlab-ai/pyxis":   false,
		"file:///srv/pyxis-artifacts":              false,
		"":                                         false,
		"https://[bad":                             false,
	} {
		if got := releaseenv.IsRetiredChannel(source); got != want {
			t.Errorf("IsRetiredChannel(%q) = %v, want %v", source, got, want)
		}
	}
}
