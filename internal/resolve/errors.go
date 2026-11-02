package resolve

import (
	"errors"
	"fmt"
	"strings"
)

// The refusal taxonomy is part of this package's contract (installer plan
// T3): every way resolution can end without a verified executable is a
// typed, named result the caller can branch on with errors.As / errors.Is.
// None of them ever accompanies a usable exec path — refusals are total
// (DINV-1: zero execution on any unverified or ambiguous path).

// ErrSourceMode is the sentinel under every *SourceMode result, so a caller
// can ask errors.Is(err, ErrSourceMode) without holding the struct. It marks
// the one non-defect "error": resolution deliberately falling through to
// source execution (installer-v1 §3.4 source mode).
var ErrSourceMode = errors.New("resolve: source mode")

// SourceMode is the typed fall-through result for source execution
// (installer-v1 §3.4; DINV-8). Tool returns execPath "" plus *SourceMode when
// the config signals source and the tool's source tree exists; the caller
// (pyxis exec, T4) runs `go run ./cmd/<tool>` from RepoRoot instead of an
// acquired binary. It unwraps to ErrSourceMode.
type SourceMode struct {
	Tool     string // the tool to run from source
	RepoRoot string // the repo root the source tree lives under
	CmdDir   string // RepoRoot/cmd/<tool> — the probed source directory
}

func (s *SourceMode) Error() string {
	return fmt.Sprintf("resolve: %s: source mode — execute from source (go run ./cmd/%s, root %s)", s.Tool, s.Tool, s.RepoRoot)
}

func (s *SourceMode) Unwrap() error { return ErrSourceMode }

// TamperError is the fail-closed verification refusal (DINV-1): an artifact's
// bytes — fetched or cached — hash to something other than the pinned
// expectation. It always names the artifact and both hashes, and it is never
// returned alongside a usable path: nothing tampered is executed or cached.
type TamperError struct {
	Artifact string // the artifact name, e.g. "docgate-v0.2.0-darwin-arm64"
	Expected string // the pinned sha256 (config pin or manifest entry)
	Actual   string // what the bytes hashed to
}

func (e *TamperError) Error() string {
	return fmt.Sprintf("resolve: %s: sha256 mismatch — expected %s, actual %s: refusing with zero execution (DINV-1, fail closed)", e.Artifact, e.Expected, e.Actual)
}

// OfflineError is the cache-miss-with-no-source refusal: the artifact is not
// verifiably cached and the artifact source cannot supply it (offline, a
// missing fixture path, a non-200 response). The remedy is named: pre-warm
// while online (`pyxis converge` runs resolve.PreWarm) or re-run the
// bootstrap (prd:installer/FR-17, UB-2).
type OfflineError struct {
	Artifact string // the artifact that could not be fetched
	URL      string // where the fetch was attempted
	Err      error  // the underlying transport failure
}

func (e *OfflineError) Error() string {
	return fmt.Sprintf("resolve: %s: cannot fetch %s: %v — no verified cache entry and the artifact source is unreachable; pre-warm while online (`pyxis converge`) or re-run the bootstrap (FR-17)", e.Artifact, e.URL, e.Err)
}

func (e *OfflineError) Unwrap() error { return e.Err }

// MissingPinError names the absent half of the system pin pair. Release-mode
// resolution is impossible without both system.version and
// system.manifest_sha256 (DINV-2: the pin pair is the trust root).
type MissingPinError struct {
	Key string // "system.version" or "system.manifest_sha256"
}

func (e *MissingPinError) Error() string {
	return fmt.Sprintf("resolve: config pins no %s — release resolution requires the system pin pair (system.version + system.manifest_sha256, DINV-2); pin a release, or use source mode where the source lives", e.Key)
}

// WrongPlatformError names a tool/platform pair the pinned release manifest
// does not ship. Known lists the platforms the manifest does carry for the
// tool; empty Known means the manifest lists no binary of that name at all.
type WrongPlatformError struct {
	Tool     string
	Platform string
	Version  string   // the pinned system version (tag form)
	Known    []string // platforms the manifest ships for Tool; sorted, nil if the tool is unknown
}

func (e *WrongPlatformError) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("resolve: %s: release-manifest %s lists no binary named %q", e.Tool, e.Version, e.Tool)
	}
	return fmt.Sprintf("resolve: %s: no binary for platform %s in release-manifest %s (shipped: %s)", e.Tool, e.Platform, e.Version, strings.Join(e.Known, ", "))
}

// SourceTreeAbsentError is the DINV-8 refusal: the config signals source mode
// but the tool's source tree does not exist under the repo root. Source
// execution is legal only where the source lives — an installed repo must
// pin a release instead.
type SourceTreeAbsentError struct {
	Tool   string
	CmdDir string // the probed directory that does not exist
}

func (e *SourceTreeAbsentError) Error() string {
	return fmt.Sprintf("resolve: %s: source mode with no source tree at %s — source execution is legal only where the source lives (DINV-8); pin a release (system.version + system.manifest_sha256) instead", e.Tool, e.CmdDir)
}
