package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// PackOptions is Pack's closed operand.
type PackOptions struct {
	// Version is the system tag being packed (vX.Y.Z).
	Version string
	// RepoRoot is the source checkout to stage from ("" = ".").
	RepoRoot string
	// OutDir receives the complete artifact set; created if absent. Pack is
	// idempotent over it (AP-22): a re-run rewrites the same bytes.
	OutDir string
	// BinariesDir holds the prebuilt per-platform binaries, named
	// <tool>-<version>-<platform>. Pack copies them into OutDir so the out
	// dir IS the publishable set. The set must be exactly the scan-derived
	// tool list × Platforms — a partial build cannot pack (A-1).
	BinariesDir string
}

// PackResult reports what Pack wrote.
type PackResult struct {
	Manifest  relmanifest.Manifest
	Artifacts []string // out-dir file names, sorted (SHA256SUMS included)
}

// Pack builds one complete release artifact set (installer-v1 §3.1; the T11
// charter): stage the operand-side payload, assert the release
// preconditions, write the deterministic payload archive,
// copy the binaries in, derive the manifest via the class-policy walk
// (DINV-6), and finish with SHA256SUMS over everything in OutDir. Fail
// closed: any refusal aborts before SHA256SUMS exists, so verify/publish
// can never treat a partial pack as a release.
func Pack(ctx context.Context, opts PackOptions) (PackResult, error) {
	if err := checkVersion(opts.Version); err != nil {
		return PackResult{}, err
	}
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	if opts.OutDir == "" {
		return PackResult{}, fmt.Errorf("release: pack requires --out")
	}
	if opts.BinariesDir == "" {
		return PackResult{}, fmt.Errorf("release: pack requires --binaries — a system release ships every scan-derived binary (A-1); build them first (the system-release workflow's matrix step)")
	}

	// The authored class policy (AP-24) — parsed before any work.
	policyBytes, err := os.ReadFile(filepath.Join(repoRoot, ".pyxis", "class-policy.yaml"))
	if err != nil {
		return PackResult{}, fmt.Errorf("release: read authored class policy: %w", err)
	}
	policy, err := relmanifest.ParseClassPolicy(policyBytes)
	if err != nil {
		return PackResult{}, fmt.Errorf("release: %w", err)
	}

	// Stage the operand-side payload tree.
	staged, err := os.MkdirTemp("", "pyxis-payload-*")
	if err != nil {
		return PackResult{}, fmt.Errorf("release: staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(staged) }() // Best-effort scratch cleanup; the pack result governs.
	if _, err := StagePayload(ctx, repoRoot, staged); err != nil {
		return PackResult{}, err
	}

	// Preconditions (a) and (b) — before any artifact is written.
	if err := CheckRetiredResolvers(staged); err != nil {
		return PackResult{}, err
	}
	if err := CheckWorkflowToolchainFree(staged); err != nil {
		return PackResult{}, err
	}

	// The scan-derived tool list and the prebuilt binaries against it.
	tools, err := ShippedTools(repoRoot)
	if err != nil {
		return PackResult{}, err
	}
	binaries, binFiles, err := collectBinaries(opts.BinariesDir, opts.Version, tools)
	if err != nil {
		return PackResult{}, err
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return PackResult{}, fmt.Errorf("release: out dir: %w", err)
	}
	if err := checkOutDirForVersion(opts.OutDir, opts.Version); err != nil {
		return PackResult{}, err
	}

	// The payload archive.
	payloadEntries, err := treeEntries(staged)
	if err != nil {
		return PackResult{}, fmt.Errorf("release: collect staged payload: %w", err)
	}
	payloadPath := filepath.Join(opts.OutDir, payloadName(opts.Version))
	if err := writeDeterministicTarGz(payloadPath, payloadEntries); err != nil {
		return PackResult{}, err
	}
	archiveSum, err := hashFile(payloadPath)
	if err != nil {
		return PackResult{}, fmt.Errorf("release: %w", err)
	}

	// The binaries, copied so OutDir is the complete publishable set.
	for _, name := range binFiles {
		if err := copyArtifact(filepath.Join(opts.BinariesDir, name), filepath.Join(opts.OutDir, name)); err != nil {
			return PackResult{}, err
		}
	}

	// The derived manifest (relmanifest.Walk fails on any
	// unclassed staged file — DINV-6, A-11).
	commit, err := sourceCommit(ctx, repoRoot)
	if err != nil {
		return PackResult{}, err
	}
	manifest, err := relmanifest.Walk(staged, policy, relmanifest.Meta{
		SystemVersion: opts.Version,
		SourceCommit:  commit,
		Binaries:      binaries,
		ArchiveSHA256: archiveSum,
	})
	if err != nil {
		return PackResult{}, fmt.Errorf("release: %w", err)
	}
	encoded, err := manifest.Encode()
	if err != nil {
		return PackResult{}, fmt.Errorf("release: %w", err)
	}
	if err := os.WriteFile(filepath.Join(opts.OutDir, manifestName(opts.Version)), encoded, 0o644); err != nil {
		return PackResult{}, fmt.Errorf("release: write manifest: %w", err)
	}

	// SHA256SUMS over everything in OutDir — written LAST, so its presence
	// certifies a completed pack.
	artifacts, err := writeSums(opts.OutDir)
	if err != nil {
		return PackResult{}, err
	}
	return PackResult{Manifest: manifest, Artifacts: artifacts}, nil
}

// checkOutDirForVersion refuses to pack into a dir already holding another
// release's artifacts. SHA256SUMS is written by sweeping the whole out dir
// (writeSums), so packing v2 over a leftover v1 set yields ONE set covering
// both — internally consistent, verifiable, and publishable, which would
// upload v1's binaries as assets of v2. Refusing early also leaves the
// operator's existing set untouched, rather than half-overwritten.
//
// An out dir carrying THIS version's artifacts is fine: pack is idempotent
// over it (AP-22), and --binaries may legitimately equal --out.
func checkOutDirForVersion(outDir, version string) error {
	names, err := listArtifacts(outDir)
	if err != nil {
		return err
	}
	var foreign []string
	for _, name := range names {
		if name == depsName(version) || (name != SumsName && !belongsToRelease(name, version)) {
			foreign = append(foreign, name)
		}
	}
	if len(foreign) == 0 {
		return nil
	}
	sort.Strings(foreign)
	return fmt.Errorf("release: pack refused — out dir %s already holds %d artifact(s) from another release or a retired bundle: %s.\n  %s covers everything in the dir, so packing over them would publish them under %s. Remove the dir or pack into a fresh one",
		outDir, len(foreign), strings.Join(foreign, ", "), SumsName, version)
}

// collectBinaries validates the prebuilt binary dir against the scan-derived
// tool list × Platforms — exactly, both ways — and returns the manifest's
// binaries section plus the sorted file names. Fail closed: an unparseable
// name, a missing tool×platform, or an extra file (a stray, or a tool
// outside the shipped set — BuildOnlyTool included) is a named refusal.
func collectBinaries(binDir, version string, tools []string) (map[string]map[string]string, []string, error) {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return nil, nil, fmt.Errorf("release: read binaries dir: %w", err)
	}
	shipped := make(map[string]bool, len(tools))
	for _, t := range tools {
		shipped[t] = true
	}
	binaries := map[string]map[string]string{}
	var files []string
	var defects []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			defects = append(defects, fmt.Sprintf("%s: not a regular file", name))
			continue
		}
		tool, plat, ok := parseBinaryName(name, version)
		if !ok {
			defects = append(defects, fmt.Sprintf("%s: not a <tool>-%s-<platform> binary of this release", name, version))
			continue
		}
		if !shipped[tool] {
			defects = append(defects, fmt.Sprintf("%s: tool %q is outside the scan-derived shipped set (see ShippedTools; %s never ships)", name, tool, BuildOnlyTool))
			continue
		}
		sum, err := hashFile(filepath.Join(binDir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("release: %w", err)
		}
		if binaries[tool] == nil {
			binaries[tool] = map[string]string{}
		}
		binaries[tool][plat] = sum
		files = append(files, name)
	}
	for _, tool := range tools {
		for _, plat := range Platforms {
			if binaries[tool][plat] == "" {
				defects = append(defects, fmt.Sprintf("missing binary %s-%s-%s — the release ships every scan-derived tool on every platform (A-1)", tool, version, plat))
			}
		}
	}
	if len(defects) > 0 {
		sort.Strings(defects)
		return nil, nil, fmt.Errorf("release: binaries dir %s does not carry exactly the shipped set × platforms:\n  %s", binDir, strings.Join(defects, "\n  "))
	}
	sort.Strings(files)
	return binaries, files, nil
}

// parseBinaryName splits <tool>-<version>-<platform> for a known platform.
func parseBinaryName(name, version string) (tool, plat string, ok bool) {
	for _, p := range Platforms {
		suffix := "-" + version + "-" + p
		if t, found := strings.CutSuffix(name, suffix); found && t != "" {
			return t, p, true
		}
	}
	return "", "", false
}

// copyArtifact copies one binary into the out dir (0755 — a published
// binary is executable), skipping the degenerate same-file case so
// --binaries may equal --out.
func copyArtifact(src, dst string) error {
	if sa, err := os.Stat(src); err == nil {
		if da, err := os.Stat(dst); err == nil && os.SameFile(sa, da) {
			return nil
		}
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("release: copy binary: %w", err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		return fmt.Errorf("release: copy binary: %w", err)
	}
	return nil
}

// writeSums writes SHA256SUMS over every artifact in dir (itself excluded),
// `<sha256>  <name>` lines sorted by name — the shasum -a 256 format the
// docgate-release workflow established. Returns the full artifact list,
// SHA256SUMS included.
func writeSums(dir string) ([]string, error) {
	names, err := listArtifacts(dir)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	var all []string
	for _, name := range names {
		if name == SumsName {
			continue
		}
		sum, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("release: %w", err)
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, name)
		all = append(all, name)
	}
	if err := os.WriteFile(filepath.Join(dir, SumsName), []byte(b.String()), 0o644); err != nil {
		return nil, fmt.Errorf("release: write %s: %w", SumsName, err)
	}
	all = append(all, SumsName)
	sort.Strings(all)
	return all, nil
}
