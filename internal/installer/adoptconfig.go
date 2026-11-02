package installer

// The config-establishment seam adoption's `apply` consumes (existing-repo-
// onboarding-v2 §3.4; plan existing-repo-onboarding/001 T8). A non-empty repo
// being adopted carries no `.pyxis/config.yaml` yet — the config surface is
// repo-owned-from-birth, written directly at init, never manifest-listed
// (helpers.go's operand-owned set). Adoption must establish that same surface,
// with the release pin and the install epoch (`epoch.commit` = the repo's
// current commit) baked in and NO commit of its own — the operator commits the
// adoption (DINV-4). This function is the single reuse point for the config
// renderer (renderConfig, AP-11): adopt never re-implements the config spelling
// or the knob defaults, it drives the installer's own renderer with the
// promptless defaults an `AcceptDefaults` init would resolve.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

var parseAdoptionConfig = parseRendered

// WriteAdoptionConfig establishes `.pyxis/config.yaml` for an adopted, non-empty
// repository: it renders the closed pyxis-config/v2 surface at the compiled
// defaults — the promptless knob defaults an `AcceptDefaults` init resolves, so
// adoption seeds exactly what the adopter then tunes — with the release pin and
// the install epoch baked in (epoch.commit = commitSHA), and writes it
// atomically WITHOUT creating a commit. It is the config-establishment half of
// adoption's ClassAdd for the config surface (the existing-config epoch update
// is config.SetEpochCommit's targeted edit); it returns the written bytes'
// sha256 — the added-path baseline apply stamps for undo's modification test.
// The rendered bytes are round-tripped through the closed parser before they
// land, so a renderer/parser drift fails closed here rather than writing a
// config the engine cannot read (the same guard init's own epoch step uses).
func WriteAdoptionConfig(target string, pin Pin, artifactSource, commitSHA string) (string, error) {
	knobs, err := resolveKnobs(context.Background(), &Options{}, nil) // promptless: the recommended default for every knob (the conditional exchange never fires promptless, so the context is never consumed)
	if err != nil {
		return "", fmt.Errorf("adopt: resolve default knobs: %w", err)
	}
	ep := &config.Epoch{
		SystemVersion:  pin.Version,
		Commit:         commitSHA,
		ManifestSHA256: pin.ManifestSHA256,
	}
	b := renderConfig(pin, artifactSource, knobs, defaultBranch, ep)
	if _, err := parseAdoptionConfig(b); err != nil {
		return "", err
	}
	if err := writeFileAtomic(filepath.Join(target, filepath.FromSlash(configRel)), b, 0o644); err != nil {
		return "", fmt.Errorf("adopt: write %s: %w", configRel, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
