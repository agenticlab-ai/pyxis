package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/upgrade"
)

// Upgrading through a mirror must not require a --channel flag carrying what
// the install already recorded (prd:installer/FR-30). A machine that recorded
// nothing, or only the retired distribution host, takes the public GitHub leg,
// which an empty channel selects.
//
// The seam is upgradeRun, so this asserts what the verb decided rather than
// reaching the network.
func TestUpgradeChannelPrecedence(t *testing.T) {
	const (
		recorded = "https://mirror.example/pyxis"
		flagged  = "https://flag.example"
		fromEnv  = "https://env.example"
	)
	tests := []struct {
		name   string
		record string // PYXIS_CHANNEL line in release.env ("" = none)
		env    string // PYXIS_CHANNEL
		args   []string
		want   string
	}{
		{
			name:   "the recorded channel needs no flag",
			record: recorded, want: recorded,
		},
		{
			// A machine that recorded nothing — a public install, or a binary
			// copied here — upgrades anonymously from public GitHub Releases.
			name: "no record selects the public leg",
			want: "",
		},
		{
			// An older install stored the host that no longer answers. Honouring
			// it would strand the machine.
			name:   "the retired channel selects the public leg",
			record: "https://dist.pyxis.agenticlab.ai",
			want:   "",
		},
		{
			name:   "the flag outranks the record",
			record: recorded, env: fromEnv,
			args: []string{"--channel", flagged}, want: flagged,
		},
		{
			// The environment is not where a channel lives — it would not
			// survive a restart. A flag is how an operator changes it, and the
			// flag writes back.
			name:   "the environment does not override the record",
			record: recorded, env: fromEnv, want: recorded,
		},
		{
			name: "the environment alone selects the public leg, never itself",
			env:  fromEnv, want: "",
		},
		{
			name:   "a trailing slash is normalised off the flag",
			record: recorded,
			args:   []string{"--channel", flagged + "/"}, want: flagged,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "pyxis"), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "PYXIS_SYSTEM_VERSION=v0.1.47\n"
			if tc.record != "" {
				body += "PYXIS_CHANNEL=" + tc.record + "\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "pyxis", "release.env"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("PYXIS_CHANNEL", tc.env)

			var got string
			var seen bool
			swap(t, &upgradeRun, func(_ context.Context, opts upgrade.Options) (upgrade.Report, error) {
				got, seen = opts.Channel, true
				return upgrade.Report{
					Current: "v0.1.47", Target: "v0.1.47",
					BinaryPath: "/tmp/pyxis", UpToDate: true,
				}, nil
			})

			var stdout, stderr bytes.Buffer
			args := append([]string{"--preview"}, tc.args...)
			if code := upgradeVerb(args, &stdout, &stderr); code != 0 {
				t.Fatalf("upgrade exited %d: %s", code, stderr.String())
			}
			if !seen {
				t.Fatal("the verb never reached upgradeRun")
			}
			if got != tc.want {
				t.Errorf("channel = %q, want %q", got, tc.want)
			}
		})
	}
}
