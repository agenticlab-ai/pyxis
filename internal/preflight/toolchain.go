// Toolchain checks the Git executable before any install question or write.
// Runtime-specific tools are supplied by the selected integrations, not by
// the document review workflow.
//
// # The floor set's home (design Q2, resolved here)
//
// DINV-6 forecloses a bare literal: the floor set must resolve from config
// or release data, never a Go constant compiled straight into comparison
// logic — the shape brainstorm:init-hardening#P-2 pointed at
// (internal/verify/invoke's gitFloorVersion), reused below only for its
// COMPARISON pattern (versionAtLeast), not its storage. This package
// instead embeds toolchain_floors.json: a small, versioned DATA asset
// authored in the source repo and compiled into the pyxis binary via
// the go:embed directive — the same "authored in the source repo, shipped with the
// release" shape relmanifest.go's class-policy.yaml already documents.
// Two alternatives considered: the release manifest's metadata (fetched
// and verified over the network, resolve.EnsureManifest) would make a
// LOCAL, pre-elicitation presence/version check depend on the artifact
// source being reachable — entangling this check's remedy with the
// Artifact-source check's (artifactsource.go) and forcing a network call
// before every init, cold cache or warm; free-form release documentation
// (prose) is not machine-checkable. An embedded data file needs neither:
// it travels with the binary, resolves with zero network calls, and is
// edited independently of the comparison logic that reads it.
package preflight

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ToolchainID is the toolchain check's registry id.
const ToolchainID = "toolchain"

//go:embed toolchain_floors.json
var toolchainFloorsJSON []byte

// toolchainFloors is the parsed embedded floor set: tool name → minimum
// version "x.y.z". Parsed once at package init; a parse failure is a build
// defect of this package's own embedded asset (never a runtime
// possibility in a working build), so it panics rather than threading an
// error through every caller (programmer error, not an operational one).
var toolchainFloors = mustParseFloors(toolchainFloorsJSON)

func mustParseFloors(b []byte) map[string]string {
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		panic(fmt.Sprintf("preflight: toolchain_floors.json does not parse (a build defect): %v", err))
	}
	return m
}

// toolchainTool is one probed executable: its floor-table key and how to
// extract a bare "x.y.z" version from its `--version` output.
type toolchainTool struct {
	name    string
	extract func(output string) (version string, ok bool)
}

var toolchainTools = []toolchainTool{
	{name: "git", extract: extractGitVersion},
}

var toolchainCheck = Check{
	ID:            ToolchainID,
	Applicability: nil, // always applicable — runs before the first question, like Admission
	Probe:         probeToolchain,
}

// probeToolchain checks every tool in toolchainTools for presence and floor
// version, enumerating every miss into one Outcome (AP-16: report the
// whole problem, not just the first tool hit) rather than stopping at the
// first. It shells out directly (exec.LookPath + "<tool> --version"), the
// same unseamed style admission.go's git probes already use — no
// executable is ever run beyond a version query.
func probeToolchain(ctx context.Context, in Input) (Outcome, error) {
	var conflicts, remedies []string
	for _, tool := range toolchainTools {
		floor, known := toolchainFloors[tool.name]
		if !known || floor == "" {
			return Outcome{}, fmt.Errorf("preflight: toolchain: no floor recorded for %q (a build defect in toolchain_floors.json)", tool.name)
		}
		path, err := exec.LookPath(tool.name)
		if err != nil {
			conflicts = append(conflicts, fmt.Sprintf("%s: not found on PATH", tool.name))
			remedies = append(remedies, fmt.Sprintf("install %s >= %s and ensure it is on PATH", tool.name, floor))
			continue
		}
		out, err := exec.CommandContext(ctx, path, "--version").Output()
		if err != nil {
			return Outcome{}, fmt.Errorf("preflight: toolchain: run %s --version: %w", tool.name, err)
		}
		ver, ok := tool.extract(string(out))
		if !ok {
			return Outcome{}, fmt.Errorf("preflight: toolchain: %s --version printed an unrecognized format: %q", tool.name, strings.TrimSpace(string(out)))
		}
		if !versionAtLeast(ver, floor) {
			conflicts = append(conflicts, fmt.Sprintf("%s: %s is below the required %s", tool.name, ver, floor))
			remedies = append(remedies, fmt.Sprintf("upgrade %s to >= %s (found %s)", tool.name, floor, ver))
		}
	}
	if len(conflicts) > 0 {
		return Outcome{
			Status:    StatusRefuse,
			Reason:    "a toolchain executable is missing or below the release's documented floor version",
			Conflicts: conflicts,
			Remedy:    strings.Join(remedies, "; "),
		}, nil
	}
	return Outcome{Status: StatusPass}, nil
}

// extractGitVersion parses "git version 2.39.3 (Apple Git-146)\n" into
// "2.39.3" — the same field-position parse internal/verify/invoke's
// getGitVersion uses for the identical output shape.
func extractGitVersion(output string) (string, bool) {
	fields := strings.Fields(output)
	if len(fields) < 3 {
		return "", false
	}
	return fields[2], true
}

// versionAtLeast reports whether version is >= floor, comparing up to
// three numeric components (major.minor.patch) — the same
// tolerant-of-suffixes numeric parse internal/verify/invoke's
// gitFloorVersion precedent uses (brainstorm:init-hardening#P-2),
// reimplemented locally: that package's parse is unexported and answers a
// different question (codegate's own, separate git floor), so this is the
// same small, deliberate duplication admission.go's defaultBranch
// precedent already documents for this package.
func versionAtLeast(version, floor string) bool {
	v, f := parseVersionTriple(version), parseVersionTriple(floor)
	for i := range 3 {
		if v[i] != f[i] {
			return v[i] > f[i]
		}
	}
	return true
}

// parseVersionTriple extracts up to three leading numeric components,
// tolerating suffixes ("2.39.3 (Apple Git-146)", "22.1.0-nightly").
func parseVersionTriple(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(v, ".", 4)
	for i := 0; i < 3 && i < len(parts); i++ {
		digits := parts[i]
		for j, c := range digits {
			if c < '0' || c > '9' {
				digits = digits[:j]
				break
			}
		}
		if digits == "" {
			continue
		}
		if n, err := strconv.Atoi(digits); err == nil {
			out[i] = n
		}
	}
	return out
}
