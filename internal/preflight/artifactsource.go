// ArtifactSource is the registry's next check after Toolchain (design
// init-hardening-v1 §3.1 check 5; DINV-1; prd:init-hardening FR-5; plan
// init-hardening/001 T2): applicable only in effect — it is registered
// unconditionally, like every check that runs before the first question,
// but its OWN probe skips (StatusPass) the instant the release manifest is
// already verifiably cached, so a warm re-run never touches the network.
// On a cold cache it confirms the manifest URL answers through the transport
// resolve.Config.WithFetcher already selects — anonymous for the public default,
// a token for a private fork or a bound source — reused here rather than
// reimplemented (AP-11: one acquisition owner) —
// and discards the bytes: this is a reachability read, never a cache
// write, so a later check's refusal still finds the target byte-identical
// (DINV-1). The real fetch-verify-cache a converge run performs later is
// unaffected; this check only answers "can we reach it" ahead of time.
package preflight

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// ArtifactSourceID is the artifact-source check's registry id.
const ArtifactSourceID = "artifact-source"

var artifactSourceCheck = Check{
	ID:            ArtifactSourceID,
	Applicability: nil, // cache warmth, not knobs, decides whether the probe does anything — see probeArtifactSource
	Probe:         probeArtifactSource,
}

// probeArtifactSource skips (StatusPass, no network call) when in carries
// no pin yet — a caller that has not resolved Options.Pin has nothing for
// this check to resolve a URL against, so there is nothing to probe (init
// itself always has a pin here, DINV-2; this only guards a pin-less
// caller, e.g. a registry-shape test). Otherwise it skips when the
// manifest is already cached and re-hashes to the pin on this very call
// (resolve.CachedManifest — resolve's own "verifiably cached" contract),
// and probes reachability only on a genuine cold or tamper-mismatched
// cache.
func probeArtifactSource(ctx context.Context, in Input) (Outcome, error) {
	if strings.TrimSpace(in.System.Version) == "" || strings.TrimSpace(in.System.ManifestSHA256) == "" {
		return Outcome{Status: StatusPass}, nil
	}

	cfg := resolve.Config{Root: in.Target, Repo: config.Config{System: in.System}}
	if in.Integrations.SchemaVersion != "" {
		cfg.Integrations = &in.Integrations
	}
	if _, ok, err := resolve.CachedManifest(cfg); err != nil {
		return Outcome{}, fmt.Errorf("preflight: artifact-source: probe cache: %w", err)
	} else if ok {
		return Outcome{Status: StatusPass}, nil
	}

	cfg = cfg.WithFetcher()
	name, artifactURL, err := resolve.ManifestArtifact(cfg)
	if err != nil {
		return Outcome{}, fmt.Errorf("preflight: artifact-source: %w", err)
	}
	rc, ferr := cfg.Fetcher.Fetch(ctx, artifactURL)
	if ferr == nil {
		_, ferr = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
	if ferr != nil {
		return refuseArtifactSource(name, artifactURL, ferr), nil
	}
	return Outcome{Status: StatusPass}, nil
}

// refuseArtifactSource names the host that did not answer and the remedy that
// applies to it. The public default needs no credential, so a github.com miss
// names a private fork's token only as the exception.
func refuseArtifactSource(name, artifactURL string, cause error) Outcome {
	host := "the configured artifact source"
	if u, err := url.Parse(artifactURL); err == nil && u.Host != "" {
		host = u.Host
	}
	remedy := fmt.Sprintf("check network access to %s", host)
	if host == "github.com" {
		remedy += "; a public release needs no credential, and a private fork needs GH_TOKEN or GITHUB_TOKEN"
	} else {
		remedy += fmt.Sprintf("; if %s requires a credential, bind it as a release connection with a token_env", host)
	}
	return Outcome{
		Status:    StatusRefuse,
		Reason:    "the release manifest is not verifiably cached and the artifact source did not answer",
		Conflicts: []string{fmt.Sprintf("%s: %s: %v", name, artifactURL, cause)},
		Remedy:    remedy,
	}
}
