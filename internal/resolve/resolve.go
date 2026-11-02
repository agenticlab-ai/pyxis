// Package resolve owns the acquisition core of the installer design
// (installer-v1 §3.4; plan installer/001 T3): every step from the operand
// config's pin pair to a verified executable — manifest fetch-verify-cache,
// binary fetch-verify-cache, the artifact-source override, the source-mode
// fall-through, and the cache janitor.
//
// The chain of trust (DINV-2 — every byte is verified against its pin BEFORE
// it enters the cache). Note the per-exec re-hash of DINV-1 is deliberately
// relaxed (see ensureBinary): binaries are verified at fetch, then trusted
// from the version-named cache — re-hashing a multi-megabyte binary on every
// hook and gate run is cost without a matching threat (an attacker who can
// rewrite .pyxis/bin/ can already rewrite config or PATH).
//
//	config system.manifest_sha256 ──verifies──▶ release-manifest-<ver>.json
//	        cached at <root>/.pyxis/bin/release-manifest-<ver>.json (the
//	        solution shape's named location, gitignored); the bytes' sha256
//	        must equal the config pin BEFORE the manifest is parsed or
//	        trusted — on every call, cached or fetched (the anchor stays cheap)
//	manifest.binaries[tool][platform] ──verifies──▶ .pyxis/bin/<tool>-<ver>-<plat>
//	        verified WHEN FETCHED: stream to a tmp file in the cache dir,
//	        verify, chmod 0755, atomic rename — concurrent resolvers race
//	        safely, the last verified writer wins; a version-named binary
//	        already in the cache is then trusted and returned as-is
//
// Refusals are typed and named (errors.go): tamper carries expected vs
// actual, an offline miss names the pre-warm/bootstrap remedy, a missing pin
// names its key, a wrong platform names what the release ships, and source
// mode without a source tree names DINV-8. This package never executes
// anything — it only returns a path whose bytes it verified before caching
// (a cached binary is then trusted as-is); running it is the caller's (pyxis
// exec, T4).
//
// Source mode (DINV-8): when the config signals source — system.mode
// "source", or, during the dual-read window, a legacy per-tool pin section
// still present in a v2 config saying "source" (installer-v1 §3.10) — Tool
// falls through to source execution, returning the typed *SourceMode result
// instead of a path, but only where the tool's source (cmd/<tool>/ under the
// repo root) actually lives. Legacy per-tool RELEASE pins are deliberately
// not resolved here: those remain the bootstrap shell resolvers' path until
// the source-repo migration (T12) retires them.
//
// The cache ships with its janitor (AP-12): Reap removes cached binaries and
// manifests whose version is off the pinned system version, and PreWarm
// fetch-verifies the manifest plus every current-platform binary so the
// offline floor holds (prd:installer/FR-13/FR-17).
package resolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

// Config is the resolver's operand: the repo root the cache hangs off, the
// parsed repo config (T2's surface), and the injectable seams.
type Config struct {
	Integrations        *integrations.Config // explicit pre-install profile; nil reads persisted configuration
	CodeHostDiagnostics codehost.Diagnostics // read-only medium selected by the CLI composition root
	// Root is the repo root; the cache lives at Root/.pyxis/bin (gitignored)
	// and the source-mode probe looks for Root/cmd/<tool>.
	Root string
	// Repo is the parsed operand config: the system pin pair, mode, the
	// artifact-source override, and — during the dual-read window — the
	// legacy per-tool pin sections (internal/docgate/config, schema v2).
	Repo config.Config
	// Platform overrides PreWarm's platform tuple; "" derives Platform().
	// Tool takes its platform as an explicit parameter and ignores this.
	Platform string
	// Fetcher overrides the artifact transport; nil uses ByScheme's scheme
	// dispatch (http(s) + file). Tests inject counting/scripted fetchers.
	Fetcher Fetcher
	// Logger receives the structured events (fetch, verify-fail, reap);
	// nil uses slog.Default().
	Logger *slog.Logger
}

func (c Config) cacheDir() string { return filepath.Join(c.Root, ".pyxis", "bin") }

func (c Config) fetcher() Fetcher {
	if c.Fetcher != nil {
		return c.Fetcher
	}
	if target, found, err := artifactRepository(c); err != nil {
		return failedFetcher{err}
	} else if found && target.Provider == "forgejo" {
		return forgejoFetcher{target}
	} else if found && target.Provider == "github" {
		return configuredGitHubFetcher(target)
	}
	// The public default channel is anonymous: no token is consulted, so an
	// expired gh login on the machine cannot break a read that needs no
	// credential, and no token is sent where none is wanted.
	src, _ := EffectiveSource(c, "")
	if src == DefaultChannelRoot || strings.HasPrefix(src, DefaultChannelRoot+"/") {
		return ByScheme{}
	}
	// Any other github.com source may be a private fork, whose assets only the
	// API route serves. The fetcher scopes its token per-request to that host and
	// passes every other URL through plain, so no mirror ever sees it (§3.9
	// leg 1). No token is an anonymous fetch, not a failure.
	if u, err := url.Parse(src); err == nil && u.Host == "github.com" {
		if tok := tokenFor(); tok != "" {
			return ByScheme{HTTP: githubAuthFetcher{token: tok, downloadHost: "github.com", apiBase: "https://api.github.com", assets: newAssetMap()}}
		}
	}
	return ByScheme{}
}

// WithFetcher returns a copy of the config with the artifact transport frozen
// onto it (Config.Fetcher), so a multi-artifact operation — PreWarm fetches the
// manifest plus every binary; converge chains manifest/payload/pre-warm —
// constructs the fetcher, and its per-release asset-map memo, exactly once
// instead of once per artifact. A config that already carries a Fetcher (a test
// seam, or a caller sharing one across several resolves) keeps it. The public
// entry points freeze it internally; a multi-call caller (converge) freezes it
// up front so the whole run shares one memo.
func (c Config) WithFetcher() Config {
	if c.Fetcher == nil {
		c.Fetcher = c.fetcher()
	}
	return c
}

func (c Config) logger() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

func (c Config) platform() string {
	if c.Platform != "" {
		return c.Platform
	}
	return Platform()
}

// Platform derives the runtime's platform tuple, "darwin-arm64" style — the
// key the release manifest's binaries map and the artifact names use. Tests
// pass their own tuple to Tool (or set Config.Platform for PreWarm) instead.
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// DefaultChannelRoot is the built-in artifact channel: this project's public
// GitHub Releases. It is EffectiveSource's last rank — the answer for a machine
// that recorded nothing, whose repository configures nothing: a fresh install,
// an operand CI runner, a binary copied between machines.
//
// Composition is <root>/<version>/<name>, the public asset URL. It needs no
// credential and no gh: the download answers anonymously and redirects to
// GitHub's object host, which the default HTTP client follows. Verification is
// the manifest pin's job, wherever the redirect lands.
const DefaultChannelRoot = "https://github.com/agenticlab-ai/pyxis/releases/download"

// Tool resolves tool for platform to an executable path whose bytes were
// verified against the pinned release manifest ON THIS CALL (the per-exec
// floor, DINV-1): config pin → manifest (cached or fetch-verified) → the
// tool's platform sha256 → the cached binary returned as-is, fetch-verified on
// a cache miss. platform "" derives Platform().
//
// In source mode it returns ("", *SourceMode) — the typed fall-through the
// caller turns into `go run ./cmd/<tool>` (DINV-8). Every other non-nil
// error is a refusal with zero execution; see the taxonomy in errors.go.
func Tool(ctx context.Context, cfg Config, tool, platform string) (string, error) {
	if err := guard(ctx, cfg); err != nil {
		return "", err
	}
	if tool == "" {
		return "", errors.New("resolve: empty tool name")
	}
	if platform == "" {
		platform = cfg.platform()
	}
	if sourceSignaled(cfg.Repo, tool) {
		return "", sourceFallThrough(cfg, tool)
	}
	cfg = cfg.WithFetcher() // one transport (+ asset-map memo) for the manifest and binary fetches
	ver, pin, err := pins(cfg.Repo)
	if err != nil {
		return "", err
	}
	m, err := manifest(ctx, cfg, ver, pin)
	if err != nil {
		return "", err
	}
	want, err := binaryHash(m, ver, tool, platform)
	if err != nil {
		return "", err
	}
	return ensureBinary(ctx, cfg, ver, tool, platform, want)
}

// PreWarm fetch-verifies the release manifest plus every binary the manifest
// lists for the current platform (Config.Platform or Platform()), so the
// offline floor holds after one online pass (installer-v1 §3.4; converge
// orchestrates this, T5). Tools the manifest does not ship for this platform
// are skipped — exec would refuse them by name and doctor reports them. In
// source mode there is nothing to warm. Per-tool failures are joined so one
// unreachable artifact does not hide the rest.
func PreWarm(ctx context.Context, cfg Config) error {
	if err := guard(ctx, cfg); err != nil {
		return err
	}
	if cfg.Repo.System.Mode == "source" {
		cfg.logger().Info("resolve: pre-warm skipped — source mode has nothing to fetch (DINV-8)")
		return nil
	}
	cfg = cfg.WithFetcher() // one transport (+ asset-map memo) across the manifest and every binary
	ver, pin, err := pins(cfg.Repo)
	if err != nil {
		return err
	}
	m, err := manifest(ctx, cfg, ver, pin)
	if err != nil {
		return err
	}
	plat := cfg.platform()
	var errs []error
	for _, tool := range slices.Sorted(maps.Keys(m.Binaries)) {
		if err := ctx.Err(); err != nil {
			errs = append(errs, fmt.Errorf("resolve: pre-warm: %w", err))
			break
		}
		want, ok := m.Binaries[tool][plat]
		if !ok {
			continue
		}
		if _, err := ensureBinary(ctx, cfg, ver, tool, plat, want); err != nil {
			errs = append(errs, fmt.Errorf("pre-warm %s: %w", tool, err))
		}
	}
	return errors.Join(errs...)
}

// Reap is the cache's janitor (AP-12): it removes cache entries — binaries,
// manifests, and payload archives — whose version differs from the pinned
// system version and returns the removed names, sorted. It never touches
// anything outside the cache-entry name grammars (entryVersion), never
// descends into subdirectories, and leaves in-flight fetch tmp files
// (".tmp-" infix) to their owners; doctor names any straggler it cannot
// judge. Removal failures are joined, and every entry is still attempted.
func Reap(cfg Config) ([]string, error) {
	victims, err := reapScan(cfg)
	if err != nil {
		return nil, err
	}
	var removed []string
	var errs []error
	for _, name := range victims {
		if err := os.Remove(filepath.Join(cfg.cacheDir(), name)); err != nil {
			errs = append(errs, fmt.Errorf("resolve: reap %s: %w", name, err))
			continue
		}
		removed = append(removed, name)
	}
	if len(removed) > 0 {
		cfg.logger().Info("resolve: reaped off-version cache entries", "pinned", tag(cfg.Repo.System.Version), "removed", removed)
	}
	return removed, errors.Join(errs...)
}

// reapScan is the janitor's one victim predicate (AP-11), shared by Reap
// (which removes) and ReapPlan (which only reports): the sorted cache-entry
// names whose version is off the pin. Read-only; a missing cache dir is a
// clean empty scan.
func reapScan(cfg Config) ([]string, error) {
	if cfg.Root == "" {
		return nil, errors.New("resolve: Config.Root is empty (the cache needs a repo root)")
	}
	if strings.TrimSpace(cfg.Repo.System.Version) == "" {
		return nil, &MissingPinError{Key: "system.version"}
	}
	keep := tag(cfg.Repo.System.Version)
	dir := cfg.cacheDir()
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve: reap %s: %w", dir, err)
	}
	var victims []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || strings.Contains(name, ".tmp-") {
			continue
		}
		v, ok := entryVersion(name)
		if !ok || v == keep {
			continue
		}
		victims = append(victims, name)
	}
	slices.Sort(victims)
	return victims, nil
}

// guard is the shared precondition check: a live context and a repo root.
func guard(ctx context.Context, cfg Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if cfg.Root == "" {
		return errors.New("resolve: Config.Root is empty (the cache needs a repo root)")
	}
	return nil
}

// sourceSignaled reports whether resolution for tool falls through to source
// execution. Two signals, either suffices (the T3 charter's either-signal
// rule for the dual-read window, installer-v1 §3.10):
//
//   - system.mode "source" — the v2 mode bit; also every legacy v1 config,
//     where the key cannot be set and defaults to "source" (the source-repo
//     era posture).
//   - a legacy per-tool pin section pinning "source", while the dual-read
//     window is open. Presence is read from Config.Deprecations() — the one
//     surface T2 exposes for "a legacy section was actually in the file" —
//     because the per-tool version fields default to "source" even in pure
//     v2 configs, and a default must never signal (an installed repo would
//     otherwise dead-end on SourceTreeAbsent).
func sourceSignaled(repo config.Config, tool string) bool {
	if repo.System.Mode == "source" {
		return true
	}
	if !config.LegacyPinWindowOpen {
		return false
	}
	var legacyVer string
	switch tool {
	case "docgate":
		legacyVer = repo.DocgateVersion
	case "codegate":
		legacyVer = repo.CodegateVersion
	default:
		return false
	}
	return legacyVer == "source" && legacySectionPresent(repo, tool)
}

// legacySectionPresent reports whether the tool's legacy pin section was
// actually present in a v2 config, via the dual-read deprecation notices
// (config.noteDeprecation's one-per-section contract).
func legacySectionPresent(repo config.Config, tool string) bool {
	needle := fmt.Sprintf("legacy per-tool pin section %q", tool)
	for _, d := range repo.Deprecations() {
		if strings.Contains(d, needle) {
			return true
		}
	}
	return false
}

// sourceFallThrough probes the tool's source tree and returns either the
// typed *SourceMode result or the DINV-8 refusal.
func sourceFallThrough(cfg Config, tool string) error {
	cmdDir := filepath.Join(cfg.Root, "cmd", tool)
	st, err := os.Stat(cmdDir)
	switch {
	case err == nil && st.IsDir():
		return &SourceMode{Tool: tool, RepoRoot: cfg.Root, CmdDir: cmdDir}
	case err == nil || errors.Is(err, fs.ErrNotExist):
		// Absent — or present but not a directory, which is no source tree.
		return &SourceTreeAbsentError{Tool: tool, CmdDir: cmdDir}
	default:
		return fmt.Errorf("resolve: %s: probe source tree %s: %w", tool, cmdDir, err)
	}
}

// pins extracts and validates the system pin pair (DINV-2): the version in
// tag form and the manifest sha256, lowercased. Missing halves are the named
// MissingPinError; a malformed hash refuses by name.
func pins(repo config.Config) (ver, pin string, err error) {
	if strings.TrimSpace(repo.System.Version) == "" {
		return "", "", &MissingPinError{Key: "system.version"}
	}
	if strings.TrimSpace(repo.System.ManifestSHA256) == "" {
		return "", "", &MissingPinError{Key: "system.manifest_sha256"}
	}
	pin = strings.ToLower(strings.TrimSpace(repo.System.ManifestSHA256))
	if !isHex64(pin) {
		return "", "", fmt.Errorf("resolve: system.manifest_sha256 %q is not a 64-char sha256 hex (DINV-2)", repo.System.ManifestSHA256)
	}
	return tag(repo.System.Version), pin, nil
}

// tag normalizes a configured version to the release-tag form — exactly one
// leading "v" ("0.2.0" and "v0.2.0" both yield "v0.2.0") — the system tag
// the default base URL carries and every cache/artifact name embeds.
func tag(version string) string {
	v := strings.TrimSpace(version)
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// Artifact names double as cache-entry names (the solution shape's
// artifact-set naming): the cache holds artifacts under exactly the names
// the release publishes them as.
func manifestName(ver string) string { return "release-manifest-" + ver + ".json" }

func binaryName(tool, ver, plat string) string { return tool + "-" + ver + "-" + plat }

// artifactURL joins the artifact source base and an artifact name. The
// default base is the GitHub Releases download URL for the system tag;
// system.artifact_source (FR-22) replaces the WHOLE base — artifacts then
// resolve flat at <override>/<name>, so a mirror serving one release needs
// no tag layout. The override changes where bytes come from, never whether
// they are accepted (§3.9 leg 1).
func artifactURL(cfg Config, ver, name string) string {
	base, _ := EffectiveSource(cfg, ver)
	return base + "/" + name
}

// recordedBase is what the machine's own config says about where releases
// come from — the durable half of the precedence. An environment variable is
// a redirect for ONE invocation and does not survive a restart, so it is never
// the store; the record install.sh writes is (prd:installer/FR-30).
//
// A recorded CHANNEL wins because it composes: <root>/<version>, so one record
// keeps serving after an upgrade. A recorded flat SOURCE is the fallback for a
// source not laid out by tag — an air-gapped copy or a one-release mirror,
// where there is no root to record. That base names a single release's
// directory, so it is honoured only while the release it was recorded for is
// the one being resolved; otherwise it would answer for bytes it cannot have.
//
// ver may be empty when only the host matters (the credential-attachment
// gate), which no recorded value is version-checked for. A value pointing at
// the retired channel never reaches here: releaseenv reads it as not recorded.
var recordedBase = func(ver string) string {
	if c := releaseenv.Channel(); c != "" {
		if ver == "" {
			return c
		}
		return c + "/" + ver
	}
	r := releaseenv.Read()
	s := strings.TrimRight(strings.TrimSpace(r.ArtifactSource), "/")
	if s == "" || (ver != "" && r.Version != ver) {
		return ""
	}
	return s
}

// EffectiveSource is the ONE artifact-source precedence (prd:installer/FR-30),
// most specific to this invocation first:
//
//  1. system.artifact_source — the repository's configured override, the rank
//     an --artifact-source flag lands in (captured verbatim at init)
//  2. the machine's own record — its channel, composed as <root>/<version>,
//     or the flat source it installed from when that release is this one
//  3. the built-in channel — public GitHub Releases
//
// Every rank is a FILE. No configuration value is read from the environment:
// a variable does not survive a restart, a new shell, or a cron run, so it
// cannot be where a setting lives. An operator changes the answer with a flag,
// and the flag is written down. Credentials are the sole exception, env-only
// in the opposite direction — they must never reach a file.
//
// A value naming the retired channel, in any rank, counts as absent: a machine
// or repository that recorded it falls through to the built-in channel rather
// than being stranded on a host that no longer answers.
//
// Rank 1 is a FLAT base — artifacts resolve at <base>/<name>, so a mirror
// serving a single release needs no tag layout (FR-27). Rank 2 composes per
// version when it holds a channel root, which is the whole point of recording
// a root rather than a source.
//
// isDefault reports rank 3, so preview and doctor can say whether the source
// answering is the built-in one or something the operator chose. ver may be
// empty when only the host matters (the credential-attachment gate).
func EffectiveSource(cfg Config, ver string) (base string, isDefault bool) {
	if s := configuredSource(cfg); s != "" {
		return s, false
	}
	if target, found, err := artifactRepository(cfg); found && err == nil {
		return releaseDownloadBase(target, ver), false
	}
	if base := recordedBase(ver); base != "" {
		return base, false
	}
	if ver == "" {
		return DefaultChannelRoot, true
	}
	return DefaultChannelRoot + "/" + ver, true
}

// RepositorySource is the source a hosted CI runner resolves for release ver:
// EffectiveSource over only what the repository commits. A runner has no machine
// record, so rank 2 never answers there, and baking one into a committed workflow
// would make its bytes differ by the machine that rendered them.
func RepositorySource(cfg Config, ver string) (string, error) {
	if s := configuredSource(cfg); s != "" {
		return s, nil
	}
	target, found, err := boundArtifactRepository(cfg)
	if err != nil {
		return "", err
	}
	if found {
		return releaseDownloadBase(target, ver), nil
	}
	return DefaultChannelRoot + "/" + ver, nil
}

// configuredSource is the repository's system.artifact_source override, or ""
// when it is unset or names the retired channel.
func configuredSource(cfg Config) string {
	s := strings.TrimRight(strings.TrimSpace(cfg.Repo.System.ArtifactSource), "/")
	if releaseenv.IsRetiredChannel(s) {
		return ""
	}
	return s
}

// manifest returns the pinned release manifest, verified against the config
// pin ON THIS CALL: the cached bytes are re-hashed every time; a miss or a
// failed re-hash fetch-verifies (the manifest bytes' sha256 must equal the
// pin BEFORE parse or trust), caches atomically, then parses. A hash-true
// manifest that fails to parse — or whose system_version contradicts the
// pinned version — is a named defect of the pinned release metadata itself,
// not a tamper.
func manifest(ctx context.Context, cfg Config, ver, pin string) (relmanifest.Manifest, error) {
	name := manifestName(ver)
	path := filepath.Join(cfg.cacheDir(), name)
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		sum := sha256Hex(b)
		if sum == pin {
			return parsePinned(b, ver, name, "cached")
		}
		cfg.logger().Warn("resolve: cached manifest failed the pin re-hash; refetching",
			"artifact", name, "expected", pin, "actual", sum)
	case !errors.Is(err, fs.ErrNotExist):
		return relmanifest.Manifest{}, fmt.Errorf("resolve: read cached %s: %w", name, err)
	}
	url := artifactURL(cfg, ver, name)
	b, err = fetchAll(ctx, cfg, name, url)
	if err != nil {
		return relmanifest.Manifest{}, err
	}
	if sum := sha256Hex(b); sum != pin {
		cfg.logger().Error("resolve: fetched manifest failed verification",
			"artifact", name, "url", url, "expected", pin, "actual", sum)
		return relmanifest.Manifest{}, &TamperError{Artifact: name, Expected: pin, Actual: sum}
	}
	// Cache only after the pin verified; parse defects below are then
	// deterministic on every subsequent call (no refetch loop).
	if err := writeAtomic(cfg.cacheDir(), name, b, 0o644); err != nil {
		return relmanifest.Manifest{}, err
	}
	return parsePinned(b, ver, name, "fetched")
}

// parsePinned parses manifest bytes that already passed the pin, then holds
// the pin pair coherent: the manifest's own system_version must match the
// pinned version (DINV-2 — one pin pair, one release).
func parsePinned(b []byte, ver, name, origin string) (relmanifest.Manifest, error) {
	m, err := relmanifest.ParseManifest(b)
	if err != nil {
		return relmanifest.Manifest{}, fmt.Errorf("resolve: %s %s matches the config pin but does not parse — the pinned release metadata itself is defective: %w", origin, name, err)
	}
	if tag(m.SystemVersion) != ver {
		return relmanifest.Manifest{}, fmt.Errorf("resolve: %s %s carries system_version %q but the config pins %q — the pin pair does not cohere (DINV-2)", origin, name, m.SystemVersion, ver)
	}
	return m, nil
}

// binaryHash resolves the tool's expected sha256 for platform from the
// manifest, or refuses by name.
func binaryHash(m relmanifest.Manifest, ver, tool, platform string) (string, error) {
	platforms, ok := m.Binaries[tool]
	if !ok {
		return "", &WrongPlatformError{Tool: tool, Platform: platform, Version: ver}
	}
	want, ok := platforms[platform]
	if !ok {
		return "", &WrongPlatformError{Tool: tool, Platform: platform, Version: ver, Known: slices.Sorted(maps.Keys(platforms))}
	}
	return want, nil
}

// ensureBinary returns the cached binary's path, fetch-verifying it into the
// cache on a miss. The bytes are verified against want WHEN FETCHED (streamed
// to a tmp file, hashed in flight, chmod 0755, atomic rename); a version-named
// binary already present in the cache is trusted and returned as-is. The cache
// is verified-once-at-fetch, not re-hashed on every exec — guarding the
// operand's own `.pyxis/bin/` against on-disk edits is the filesystem's
// concern, not a cost worth paying on every hook and gate run. want feeds the
// fetch leg's verification.
func ensureBinary(ctx context.Context, cfg Config, ver, tool, platform, want string) (string, error) {
	name := binaryName(tool, ver, platform)
	path := filepath.Join(cfg.cacheDir(), name)
	switch info, err := os.Stat(path); {
	case err == nil && info.Mode().IsRegular():
		return path, nil // cached hit — verified when it was fetched
	case err == nil:
		return "", fmt.Errorf("resolve: cached %s is not a regular file (%s)", name, info.Mode())
	case !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("resolve: stat cached %s: %w", name, err)
	}
	if err := fetchVerifyTo(ctx, cfg, ver, name, want, path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

// fetchVerifyTo streams one artifact to a tmp file in the cache dir, hashes
// it in flight, and only a byte-exact artifact is chmodded to mode (0755
// binaries, 0644 archives) and renamed into place (atomic; concurrent
// fetchers race safely — every writer renames only bytes it verified). A
// tampered or truncated artifact is removed and refused with zero execution.
func fetchVerifyTo(ctx context.Context, cfg Config, ver, name, want, dst string, mode fs.FileMode) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("resolve: create cache dir %s: %w", dir, err)
	}
	url := artifactURL(cfg, ver, name)
	cfg.logger().Info("resolve: fetch", "artifact", name, "url", url)
	rc, err := cfg.fetcher().Fetch(ctx, url)
	if err != nil {
		return classifyFetchErr(ctx, name, url, err)
	}
	defer func() { _ = rc.Close() }() // read side: a close failure cannot corrupt verified bytes
	tmp, err := os.CreateTemp(dir, name+".tmp-*")
	if err != nil {
		return fmt.Errorf("resolve: create tmp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName) // best-effort: an abandoned tmp is cache garbage, never trusted
		}
	}()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), rc); err != nil {
		_ = tmp.Close() // the copy error governs; the tmp is removed by the deferred cleanup
		return classifyFetchErr(ctx, name, url, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("resolve: write %s: %w", tmpName, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		cfg.logger().Error("resolve: fetched artifact failed verification",
			"artifact", name, "url", url, "expected", want, "actual", got)
		return &TamperError{Artifact: name, Expected: want, Actual: got}
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("resolve: chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("resolve: install %s: %w", name, err)
	}
	tmpName = "" // renamed into place; nothing to clean
	return nil
}

// fetchAll fetches one small artifact (the manifest) fully into memory.
func fetchAll(ctx context.Context, cfg Config, name, url string) ([]byte, error) {
	cfg.logger().Info("resolve: fetch", "artifact", name, "url", url)
	rc, err := cfg.fetcher().Fetch(ctx, url)
	if err != nil {
		return nil, classifyFetchErr(ctx, name, url, err)
	}
	defer func() { _ = rc.Close() }() // read side: a close failure cannot corrupt verified bytes
	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, classifyFetchErr(ctx, name, url, err)
	}
	return b, nil
}

// classifyFetchErr separates caller cancellation from a genuinely
// unreachable artifact source: a dead context surfaces as itself (wrapped),
// anything else is the named OfflineError with its remedy.
func classifyFetchErr(ctx context.Context, name, url string, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("resolve: fetch %s: %w", name, cerr)
	}
	return &OfflineError{Artifact: name, URL: url, Err: err}
}

// writeAtomic writes name under dir via tmp + rename, so readers only ever
// see absent or complete bytes.
func writeAtomic(dir, name string, b []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("resolve: create cache dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, name+".tmp-*")
	if err != nil {
		return fmt.Errorf("resolve: create tmp for %s: %w", name, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName) // best-effort: an abandoned tmp is cache garbage, never trusted
		}
	}()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close() // the write error governs; the tmp is removed by the deferred cleanup
		return fmt.Errorf("resolve: write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("resolve: write %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("resolve: chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("resolve: install %s: %w", name, err)
	}
	tmpName = ""
	return nil
}

// entryVersion parses a cache-entry name into the version it belongs to.
// Exactly three grammars are cache entries (the shape's artifact-set naming
// — artifact names double as cache names):
//
//	release-manifest-<ver>.json
//	payload-<ver>.tar.gz       — cached by converge's payload ensure (T5);
//	                             the janitor arrived with the entry (AP-12)
//	<tool>-<ver>-<os>-<arch>   — <ver> starts "v<digit>"; the platform tuple
//	                             is the final two segments
//
// Anything else is not a cache entry and is never touched (the janitor's
// never-touch-non-cache-files contract).
func entryVersion(name string) (string, bool) {
	if rest, ok := strings.CutPrefix(name, "release-manifest-"); ok {
		v, ok := strings.CutSuffix(rest, ".json")
		if !ok || v == "" {
			return "", false
		}
		return v, true
	}
	if rest, ok := strings.CutPrefix(name, "payload-"); ok {
		if v, ok := strings.CutSuffix(rest, ".tar.gz"); ok && v != "" {
			return v, true
		}
		// Not the archive grammar — fall through: a tool named "payload-…"
		// still parses under the binary grammar below.
	}
	segs := strings.Split(name, "-")
	if len(segs) < 4 {
		return "", false
	}
	for i := 1; i <= len(segs)-3; i++ {
		s := segs[i]
		if len(s) >= 2 && s[0] == 'v' && s[1] >= '0' && s[1] <= '9' {
			return strings.Join(segs[i:len(segs)-2], "-"), true
		}
	}
	return "", false
}

// isHex64 reports whether v is a 64-char lowercase sha256 hex string.
func isHex64(v string) bool {
	if len(v) != 64 {
		return false
	}
	for _, c := range v {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sha256File hashes a file's current bytes; fs.ErrNotExist passes through
// for the miss branch.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }() // read-only handle
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
