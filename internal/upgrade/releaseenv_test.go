package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The record install.sh writes is the only place a machine remembers which
// channel it installed from, and upgrade rewrites that file on every run. It
// used to rewrite it with the pin pair alone, which meant a mirror install
// recorded a channel, the first upgrade erased it, and the next `pyxis init`
// fell back to a release host the machine may not reach — the failure
// prd:installer/FR-30 exists to end, reintroduced one upgrade later.
func TestReleaseEnvCarriesTheChannelThroughAnUpgrade(t *testing.T) {
	const (
		ver     = "v0.1.471"
		sha     = "b18e4fdcca0f00ccd8726083054045f3278cbf5c37091b896d868b4c0781cf26"
		channel = "https://mirror.example.com/pyxis"
	)

	t.Run("a channelled upgrade records the channel", func(t *testing.T) {
		dir := t.TempDir()
		path, err := writeReleaseEnv(dir, ver, sha, channel)
		if err != nil {
			t.Fatal(err)
		}
		body := read(t, path)
		for _, want := range []string{
			"PYXIS_SYSTEM_VERSION=" + ver,
			"PYXIS_MANIFEST_SHA256=" + sha,
			"PYXIS_CHANNEL=" + channel,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("record is missing %q:\n%s", want, body)
			}
		}
	})

	t.Run("re-recording the same thing is not a repair", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := writeReleaseEnv(dir, ver, sha, channel); err != nil {
			t.Fatal(err)
		}
		_, repaired, err := reconcileReleaseEnv(dir, ver, sha, channel)
		if err != nil {
			t.Fatal(err)
		}
		if repaired {
			t.Error("an unchanged record was rewritten — the idempotence check no longer matches what is written")
		}
	})

	t.Run("a pre-FR-30 record gains the channel", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "release.env")
		if err := os.WriteFile(path,
			[]byte("PYXIS_SYSTEM_VERSION=v0.1.47\nPYXIS_MANIFEST_SHA256="+sha+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, repaired, err := reconcileReleaseEnv(dir, ver, sha, channel)
		if err != nil {
			t.Fatal(err)
		}
		if !repaired {
			t.Error("a record naming an older version was left alone")
		}
		if body := read(t, path); !strings.Contains(body, "PYXIS_CHANNEL="+channel) {
			t.Errorf("the repaired record carries no channel:\n%s", body)
		}
	})

	t.Run("an upgrade off the public leg records no channel", func(t *testing.T) {
		dir := t.TempDir()
		path, err := writeReleaseEnv(dir, ver, sha, "")
		if err != nil {
			t.Fatal(err)
		}
		if body := read(t, path); strings.Contains(body, "PYXIS_CHANNEL") {
			t.Errorf("a channel-less upgrade recorded one:\n%s", body)
		}
	})
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
