package resolve

// The converge-facing acquisition surface (plan installer/001 T5). Converge
// consumes the SAME config-rooted chain of trust this package already owns,
// so the manifest-ensure and the payload-archive acquisition live here — one
// acquisition owner, one predicate per artifact (AP-10/AP-11) — alongside
// read-only cache probes and would-do planners. The probes/planners are what
// keep `pyxis converge --preview` honest (FR-19: report every write and
// external operation, perform none) and doctor mutation-free (T6): they
// never fetch, never write, never remove.

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// payloadName is the payload archive's artifact name — the solution shape's
// artifact-set naming; like every artifact name it doubles as the cache-entry
// name (and entryVersion's janitor grammar recognizes it).
func payloadName(ver string) string { return "payload-" + ver + ".tar.gz" }

// EnsureManifest returns the pinned release manifest, verified against the
// config pin ON THIS CALL (DINV-2): cached bytes are re-hashed every time; a
// miss or a failed re-hash fetch-verifies and caches atomically. It is the
// exported face of the same ensure Tool and PreWarm run — one manifest
// predicate, one place (AP-11) — for converge (T5) and doctor (T6). Mode is
// deliberately not consulted here: callers own the source-mode gate
// (converge refuses by name, PreWarm skips).
func EnsureManifest(ctx context.Context, cfg Config) (relmanifest.Manifest, error) {
	if err := guard(ctx, cfg); err != nil {
		return relmanifest.Manifest{}, err
	}
	cfg = cfg.WithFetcher()
	ver, pin, err := pins(cfg.Repo)
	if err != nil {
		return relmanifest.Manifest{}, err
	}
	return manifest(ctx, cfg, ver, pin)
}

// CachedManifest is EnsureManifest's read-only probe: the manifest when its
// cached bytes re-hash to the config pin on THIS call — no trust downgrade —
// with ok false (nil error) when the cache misses or the bytes fail the pin
// (legs a real run answers by fetching; this probe never does). The error
// legs are real defects: missing/malformed pins, an unreadable cache file,
// or pin-true bytes that do not parse (defective pinned release metadata).
func CachedManifest(cfg Config) (relmanifest.Manifest, bool, error) {
	if cfg.Root == "" {
		return relmanifest.Manifest{}, false, errors.New("resolve: Config.Root is empty (the cache needs a repo root)")
	}
	ver, pin, err := pins(cfg.Repo)
	if err != nil {
		return relmanifest.Manifest{}, false, err
	}
	name := manifestName(ver)
	b, err := os.ReadFile(filepath.Join(cfg.cacheDir(), name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return relmanifest.Manifest{}, false, nil
	case err != nil:
		return relmanifest.Manifest{}, false, fmt.Errorf("resolve: read cached %s: %w", name, err)
	}
	if sha256Hex(b) != pin {
		return relmanifest.Manifest{}, false, nil // a real run refetches; the probe just reports "not verifiably cached"
	}
	m, err := parsePinned(b, ver, name, "cached")
	if err != nil {
		return relmanifest.Manifest{}, false, err
	}
	return m, true, nil
}

// EnsurePayload returns the cached payload archive's path after re-hashing
// it against the manifest's archive pin ON THIS CALL (the same per-use floor
// binaries get, DINV-1): a miss or a failed re-hash fetch-verifies — stream
// to a tmp file, verify against payload.archive_sha256, atomic rename, mode
// 0644 (an archive is data, never executed). The archive is part of the
// offline pre-fetch floor (installer-v1 §3.3), so converge ensures it on
// every real run; once cached, re-runs re-hash locally and fetch nothing.
func EnsurePayload(ctx context.Context, cfg Config, m relmanifest.Manifest) (string, error) {
	if err := guard(ctx, cfg); err != nil {
		return "", err
	}
	cfg = cfg.WithFetcher()
	want := strings.ToLower(m.Payload.ArchiveSHA256)
	ver := tag(m.SystemVersion)
	name := payloadName(ver)
	path := filepath.Join(cfg.cacheDir(), name)
	sum, err := sha256File(path)
	switch {
	case err == nil && sum == want:
		return path, nil // warm hit, verified on this very call
	case err == nil:
		cfg.logger().Warn("resolve: cached payload archive failed the re-hash; refetching",
			"artifact", name, "expected", want, "actual", sum)
	case !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("resolve: re-hash cached %s: %w", name, err)
	}
	if err := fetchVerifyTo(ctx, cfg, ver, name, want, path, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// CachedPayload is EnsurePayload's read-only probe: the cached archive path
// when its bytes re-hash to the manifest pin on this call; ok false (nil
// error) on a miss or a hash mismatch — the legs a real run answers by
// fetching. It never fetches and never writes.
func CachedPayload(cfg Config, m relmanifest.Manifest) (string, bool, error) {
	if cfg.Root == "" {
		return "", false, errors.New("resolve: Config.Root is empty (the cache needs a repo root)")
	}
	name := payloadName(tag(m.SystemVersion))
	path := filepath.Join(cfg.cacheDir(), name)
	sum, err := sha256File(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("resolve: re-hash cached %s: %w", name, err)
	case sum != strings.ToLower(m.Payload.ArchiveSHA256):
		return "", false, nil
	}
	return path, true, nil
}

// PreWarmPlan is PreWarm's read-only planner: the tools (sorted) whose
// current-platform binaries a PreWarm would fetch — those absent from the
// cache. Tools the manifest does not ship for this platform are skipped,
// exactly as PreWarm skips them. Presence, not per-byte integrity: a binary
// already in the cache is trusted (the exec path no longer re-hashes it — see
// ensureBinary), so pre-warm fetches only what is missing. Zero fetches, zero
// writes: the preview/doctor face of the pre-warm step.
func PreWarmPlan(cfg Config, m relmanifest.Manifest) ([]string, error) {
	if cfg.Root == "" {
		return nil, errors.New("resolve: Config.Root is empty (the cache needs a repo root)")
	}
	ver := tag(m.SystemVersion)
	plat := cfg.platform()
	var absent []string
	for _, tool := range slices.Sorted(maps.Keys(m.Binaries)) {
		if _, ok := m.Binaries[tool][plat]; !ok {
			continue
		}
		name := binaryName(tool, ver, plat)
		switch _, err := os.Stat(filepath.Join(cfg.cacheDir(), name)); {
		case errors.Is(err, fs.ErrNotExist):
			absent = append(absent, tool)
		case err != nil:
			return nil, fmt.Errorf("resolve: stat cached %s: %w", name, err)
		}
	}
	return absent, nil
}

// ReapPlan is Reap's read-only planner: the cache entries (sorted) the
// janitor would remove — off the pinned version under the cache-entry name
// grammars — without removing anything.
func ReapPlan(cfg Config) ([]string, error) {
	victims, err := reapScan(cfg)
	if err != nil {
		return nil, err
	}
	return victims, nil
}

// ManifestArtifact names the pinned release manifest artifact and the URL a
// fetch would use — preview/doctor reporting material (FR-19; the
// artifact-source override is loudly surfaced through the URL, installer-v1
// §3.9). It requires the full pin pair: reporting an acquisition for an
// unpinned config would be fiction.
func ManifestArtifact(cfg Config) (name, url string, err error) {
	ver, _, err := pins(cfg.Repo)
	if err != nil {
		return "", "", err
	}
	name = manifestName(ver)
	return name, artifactURL(cfg, ver, name), nil
}

// PayloadArtifact names the payload archive artifact and its fetch URL, from
// the same config pins (the manifest's system_version is pin-coherent by
// parsePinned, so the two spellings can never disagree).
func PayloadArtifact(cfg Config) (name, url string, err error) {
	ver, _, err := pins(cfg.Repo)
	if err != nil {
		return "", "", err
	}
	name = payloadName(ver)
	return name, artifactURL(cfg, ver, name), nil
}
