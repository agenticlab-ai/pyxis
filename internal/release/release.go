// Package release owns the build side of the system release (installer-v1
// §3.1, §3.10; plan installer/001 T11): one tag's complete, verified,
// re-runnable artifact set. cmd/pyxis-release is its thin main; the
// system-release workflow (.github/workflows/system-release.yml) is its thin
// caller. Four operations:
//
//   - Pack: stage the payload tree exactly as an operand receives it (the
//     tracked .pyxis/** tree, the design indexes, PLUS the per-file overlay
//     mapping .pyxis/dist/github-workflows/* → .github/workflows/* from T9's
//     README), assert the release preconditions (§3.10, fail-closed, each
//     named), then write the deterministic artifact set into --out: the
//     payload archive, every prebuilt binary, the
//     derived release manifest (relmanifest.Walk over the authored class
//     policy — DINV-6 is enforced there), and SHA256SUMS over all of it.
//   - Verify: the upload-or-verify re-run leg's local half — re-hash
//     everything in --out against SHA256SUMS and the manifest.
//   - Publish: upload-or-verify against GitHub Releases (AP-22): a missing
//     release is created, a missing asset is uploaded, an existing asset is
//     DOWNLOADED and byte-compared — never clobbered. A re-run completes or
//     verifies; it can never fork the artifact set under a tag.
//   - ShippedTools: the scan-derived binary list (AP-27) — tools.go is the
//     ONE place the shipped-set rule lives.
//
// Determinism: the archives are byte-stable across runs on the same tree and
// toolchain — entries sorted by path, timestamps zeroed, uid/gid zeroed,
// modes normalized (writeDeterministicTarGz).
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
)

// Artifact names — the solution shape's release artifact set. The manifest,
// not the naming, is the contract (DINV-2), but the names double as cache
// keys in internal/resolve, so they are fixed here in one place.
func payloadName(version string) string { return "payload-" + version + ".tar.gz" }
func depsName(version string) string    { return "deps-" + version + ".tar.gz" }
func manifestName(version string) string {
	return "release-manifest-" + version + ".json"
}

// SumsName is the checksum file covering every artifact in the out dir
// (itself excluded) — the docgate-release.yml convention, generalized.
const SumsName = "SHA256SUMS"

// Platforms is the supported platform tuple set the release builds and the
// manifest pins — {darwin,linux} × {amd64,arm64}, sorted (the
// docgate-release.yml matrix, unchanged).
var Platforms = []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64"}

// versionRe admits the system tag grammar: vMAJOR.MINOR.PATCH with an
// optional pre-release suffix (v1.2.3, v1.2.3-rc.1). Fail-closed: pack and
// verify refuse anything else by name before touching the filesystem.
var versionRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

func checkVersion(v string) error {
	if !versionRe.MatchString(v) {
		return fmt.Errorf("release: version %q is not a system tag (vMAJOR.MINOR.PATCH, optional -suffix)", v)
	}
	return nil
}

// hashFile streams one file through sha256, lowercase hex.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }() // Read-only handle; a successful hash governs.
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// listArtifacts returns the sorted regular-file names directly under dir —
// the flat artifact set SHA256SUMS covers and publish uploads. A
// subdirectory is a named defect: release assets are flat, and a nested file
// would silently escape the checksum floor.
func listArtifacts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("release: read out dir %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			return nil, fmt.Errorf("release: out dir %s contains a subdirectory %q — the artifact set is flat, and a nested file would escape SHA256SUMS (fail closed)", dir, e.Name())
		}
		if !e.Type().IsRegular() {
			return nil, fmt.Errorf("release: out dir %s contains a non-regular entry %q", dir, e.Name())
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
