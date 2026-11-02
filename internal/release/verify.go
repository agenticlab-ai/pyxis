package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// Verify is the upload-or-verify re-run leg's LOCAL half (installer-v1
// §3.1; NFR-2): re-hash everything in outDir against SHA256SUMS and the
// release manifest. It proves the packed set is internally coherent —
// publish runs it before touching the remote, and a re-run operator can run
// it standalone. Checks, all accumulated so one run names every defect:
//
//   - every artifact in the set BELONGS to this release — SHA256SUMS is
//     written by sweeping the whole out dir, so packing a second version
//     into a dir that still holds the first produces a set covering both,
//     internally consistent and publishable. Name-checking against the
//     version is what makes that impossible;
//   - SHA256SUMS covers exactly the artifact set (both directions) and
//     every hash matches the bytes on disk;
//   - the manifest for version parses closed and its system_version is the
//     version being verified;
//   - the payload archive, any legacy auxiliary bundle, and every binaries[tool][platform]
//     entry hash to exactly what the manifest pins, with no pinned binary
//     missing from the set.
func Verify(outDir, version string) error {
	if err := checkVersion(version); err != nil {
		return err
	}
	names, err := listArtifacts(outDir)
	if err != nil {
		return err
	}
	sums, err := readSums(filepath.Join(outDir, SumsName))
	if err != nil {
		return err
	}

	var defects []string
	present := map[string]bool{}
	actual := map[string]string{}
	for _, name := range names {
		if name == SumsName {
			continue
		}
		if !belongsToRelease(name, version) {
			defects = append(defects, fmt.Sprintf("%s: not an artifact of release %s — the set under a version is closed, and a foreign artifact would be published under this tag", name, version))
		}
		present[name] = true
		sum, err := hashFile(filepath.Join(outDir, name))
		if err != nil {
			return fmt.Errorf("release: verify: %w", err)
		}
		actual[name] = sum
		want, ok := sums[name]
		switch {
		case !ok:
			defects = append(defects, fmt.Sprintf("%s: present in %s but absent from %s", name, outDir, SumsName))
		case want != sum:
			defects = append(defects, fmt.Sprintf("%s: %s pins %s but the bytes hash to %s", name, SumsName, want, sum))
		}
	}
	for name := range sums {
		if !present[name] {
			defects = append(defects, fmt.Sprintf("%s: listed in %s but missing from %s", name, SumsName, outDir))
		}
	}

	// The manifest and its pins.
	mName := manifestName(version)
	mBytes, err := os.ReadFile(filepath.Join(outDir, mName))
	if err != nil {
		defects = append(defects, fmt.Sprintf("%s: %v — no release manifest, no release", mName, err))
		return verifyResult(defects)
	}
	m, err := relmanifest.ParseManifest(mBytes)
	if err != nil {
		defects = append(defects, err.Error())
		return verifyResult(defects)
	}
	if m.SystemVersion != version {
		defects = append(defects, fmt.Sprintf("%s: system_version %q does not match the verified version %q", mName, m.SystemVersion, version))
	}
	checkPin := func(name, pinned, what string) {
		got, ok := actual[name]
		if !ok {
			defects = append(defects, fmt.Sprintf("%s: pinned by the manifest (%s) but missing from %s", name, what, outDir))
			return
		}
		if got != pinned {
			defects = append(defects, fmt.Sprintf("%s: manifest %s pins %s but the bytes hash to %s", name, what, pinned, got))
		}
	}
	checkPin(payloadName(version), m.Payload.ArchiveSHA256, "payload.archive_sha256")
	// Older manifests may pin an auxiliary bundle. New releases omit it.
	if m.DepsBundleSHA256 != "" {
		checkPin(depsName(version), m.DepsBundleSHA256, "deps_bundle_sha256")
	} else if present[depsName(version)] {
		defects = append(defects, "unexpected dependency bundle: this manifest does not pin one")
	}
	var tools []string
	for tool := range m.Binaries {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	for _, tool := range tools {
		var plats []string
		for plat := range m.Binaries[tool] {
			plats = append(plats, plat)
		}
		sort.Strings(plats)
		for _, plat := range plats {
			name := tool + "-" + version + "-" + plat
			checkPin(name, m.Binaries[tool][plat], fmt.Sprintf("binaries[%q][%q]", tool, plat))
		}
	}
	return verifyResult(defects)
}

// belongsToRelease reports whether one artifact name is part of version's
// closed set: the three fixed artifacts, or a <tool>-<version>-<platform>
// binary. Deliberately name-only — Verify runs wherever a set is handed
// around, with no source tree and so no shipped-tool list to consult; the
// manifest cross-check above covers the other direction (every pinned
// binary present).
func belongsToRelease(name, version string) bool {
	switch name {
	case payloadName(version), depsName(version), manifestName(version):
		return true
	}
	_, _, ok := parseBinaryName(name, version)
	return ok
}

func verifyResult(defects []string) error {
	if len(defects) == 0 {
		return nil
	}
	sort.Strings(defects)
	return fmt.Errorf("release: verify failed — the artifact set does not match its checksums/manifest (fail closed):\n  %s", strings.Join(defects, "\n  "))
}

// readSums parses SHA256SUMS: `<64-hex>  <name>` lines (the shasum format).
func readSums(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("release: verify: %s: %w — a completed pack always writes it last", SumsName, err)
	}
	sums := map[string]string{}
	for i, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		hex, name, ok := strings.Cut(line, "  ")
		if !ok || len(hex) != 64 || name == "" || strings.ContainsAny(name, "/\\") {
			return nil, fmt.Errorf("release: verify: %s line %d: %q is not a `<sha256>  <name>` entry", SumsName, i+1, line)
		}
		if _, dup := sums[name]; dup {
			return nil, fmt.Errorf("release: verify: %s line %d: duplicate entry for %q", SumsName, i+1, name)
		}
		sums[name] = hex
	}
	if len(sums) == 0 {
		return nil, fmt.Errorf("release: verify: %s is empty", SumsName)
	}
	return sums, nil
}
