// Package releaseenv reads the machine-level release record the installer
// writes at $XDG_CONFIG_HOME/pyxis/release.env (else ~/.config/pyxis/release.env)
// when it installs.
//
// The record carries what the install knew and the repository cannot: the
// release pin (prd:installer/FR-2) and, since prd:installer/FR-30, the
// CHANNEL it was installed from, when that was a mirror rather than the public
// default — so `pyxis init` and `pyxis upgrade` need no channel flag afterwards.
//
// The channel is recorded as a ROOT, never a versioned directory: artifacts
// resolve at <root>/<version>/<name>, composed per version at resolve time.
// A record that baked the installed tag into the source would strand the
// next upgrade on the tag before it.
//
// A credential for a private source is deliberately not here: only the NAME of
// its environment variable is recorded — auth governs access, the pinned
// manifest hash governs acceptance.
package releaseenv

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// retiredChannelHost is the distribution host Pyxis no longer publishes to.
// Machines and repositories that installed through it still hold its address,
// and nothing answers there any more.
const retiredChannelHost = "dist.pyxis.agenticlab.ai"

// IsRetiredChannel reports whether source, a channel root or artifact base,
// points at the retired distribution host. Every reader treats such a value as
// absent, so the public default answers instead and the next upgrade rewrites
// the record without it. The check is the one place the old address survives.
func IsRetiredChannel(source string) bool {
	u, err := url.Parse(strings.TrimSpace(source))
	return err == nil && strings.EqualFold(u.Hostname(), retiredChannelHost)
}

// Record is what the file carries. Absent keys are empty strings; every
// consumer treats empty as "not recorded" and falls through to its own next
// rank rather than failing.
type Record struct {
	ReleaseProvider   string
	ReleaseBaseURL    string
	ReleaseRepository string
	ReleaseTokenEnv   string
	Version           string // PYXIS_SYSTEM_VERSION
	ManifestSHA       string // PYXIS_MANIFEST_SHA256
	Channel           string // PYXIS_CHANNEL — the channel ROOT, no version segment
	// ArtifactSource is the FLAT base a machine installed from when the
	// source was not laid out by tag — an air-gapped copy or a one-release
	// mirror, where there is no root to record. It names ONE release's
	// directory, so it is honoured only while Version is the release being
	// resolved; a channel, which composes, is always preferred.
	ArtifactSource string // PYXIS_ARTIFACT_SOURCE
}

// Dir returns the directory holding the record, honouring XDG_CONFIG_HOME
// exactly as the installer does. Empty when no home directory resolves.
func Dir() string {
	if d := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); d != "" {
		return filepath.Join(d, "pyxis")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "pyxis")
}

// Path is the record's full path, or "" when no home directory resolves.
func Path() string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "release.env")
}

// Read parses the record. An unreadable or absent file is not an error — it
// yields the zero Record, and callers fall through to their next rank.
func Read() Record {
	path := Path()
	if path == "" {
		return Record{}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Record{}
	}
	var r Record
	for line := range strings.SplitSeq(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "PYXIS_RELEASE_PROVIDER":
			r.ReleaseProvider = v
		case "PYXIS_RELEASE_BASE_URL":
			r.ReleaseBaseURL = v
		case "PYXIS_RELEASE_REPOSITORY":
			r.ReleaseRepository = v
		case "PYXIS_RELEASE_TOKEN_ENV":
			r.ReleaseTokenEnv = v
		case "PYXIS_SYSTEM_VERSION":
			r.Version = v
		case "PYXIS_MANIFEST_SHA256":
			r.ManifestSHA = v
		case "PYXIS_CHANNEL":
			if !IsRetiredChannel(v) {
				r.Channel = strings.TrimRight(v, "/")
			}
		case "PYXIS_ARTIFACT_SOURCE":
			if !IsRetiredChannel(v) {
				r.ArtifactSource = strings.TrimRight(v, "/")
			}
		}
	}
	return r
}

// Channel is the mirror channel this machine installed from, or "" when it did
// not install from one — the public default records none. A ROOT, so callers compose <root>/<version>.
//
// It comes from the record and nowhere else. A configuration value that lived
// in the environment would not survive a restart, a new shell, or a cron run,
// so the file is the store — an operator changes it with a flag, which writes
// back. Only credentials stay env-only, because they must not reach a file.
func Channel() string {
	return Read().Channel
}

// Body is the record's canonical serialization — the exact bytes install.sh
// writes, so a pin recorded by either the installer or an upgrade reads the
// same. The channel line is omitted when there is no channel: a machine
// installed from public GitHub Releases records no channel, and an empty value
// would read back as one.
func Body(r Record) string {
	b := "PYXIS_SYSTEM_VERSION=" + r.Version + "\n" +
		"PYXIS_MANIFEST_SHA256=" + r.ManifestSHA + "\n"
	if r.ReleaseProvider != "" {
		b += "PYXIS_RELEASE_PROVIDER=" + r.ReleaseProvider + "\n" + "PYXIS_RELEASE_BASE_URL=" + r.ReleaseBaseURL + "\n" + "PYXIS_RELEASE_REPOSITORY=" + r.ReleaseRepository + "\n" + "PYXIS_RELEASE_TOKEN_ENV=" + r.ReleaseTokenEnv + "\n"
	}
	if c := strings.TrimRight(strings.TrimSpace(r.Channel), "/"); c != "" {
		b += "PYXIS_CHANNEL=" + c + "\n"
	}
	if s := strings.TrimRight(strings.TrimSpace(r.ArtifactSource), "/"); s != "" {
		b += "PYXIS_ARTIFACT_SOURCE=" + s + "\n"
	}
	return b
}

// ArtifactRepository identifies the Pyxis binary source, independently of the
// project's release publishing targets. Only a credential reference is stored.
func (r Record) ArtifactRepository() (integrations.ResolvedRepository, bool, error) {
	if r.ReleaseProvider == "" {
		return integrations.ResolvedRepository{}, false, nil
	}
	c := integrations.Config{Connections: map[string]integrations.Connection{"machine": {Provider: r.ReleaseProvider, BaseURL: r.ReleaseBaseURL, TokenEnv: r.ReleaseTokenEnv}}}
	target, err := c.Resolve(integrations.Binding{Connection: "machine", Repository: r.ReleaseRepository})
	return target, true, err
}
