// Package upgrade brings the INSTALLED pyxis CLI forward (#1028): discover
// the release the channel currently pins, fetch that platform binary, verify
// it, and replace the running executable in place.
//
// # The trust model is install.sh's, unchanged
//
// `install.sh`'s BEGIN PINS block — the release version, the manifest sha,
// and a sha256 per platform binary — is the documented trust anchor for a
// fresh install (README, "How it stays trustworthy"). Upgrade reads the same
// block from the same places, so it introduces no new trust surface: the
// host chooses WHICH version is current, never which bytes are acceptable.
// Nothing is written until the fetched binary hashes to the pinned value
// (DINV-1, the checksum floor).
//
// # Legs
//
//   - Public GitHub Releases (default): install.sh on the DEFAULT BRANCH names
//     the release and carries its per-platform digests, and the release asset
//     serves the bytes — both over anonymous HTTPS, the same path the install
//     one-liner uses, with no credential and no gh. Reading it at a TAG would
//     be wrong here and dangerously so: a tag can never carry the pins for its
//     own artifacts (the pins name the manifest hash, which names the source
//     commit, so the pin commit necessarily lands after the tag —
//     internal/release/cut.go), so a tag's copy names the PREVIOUS release and
//     would silently downgrade a current machine.
//   - Channel (--channel): a mirror's <root>/install.sh carries the pins and
//     <root>/<tag>/<binary> serves the bytes, also anonymously.
//   - Bound repository: a Forgejo source reads through its own API, and a
//     private GitHub fork that names a token environment reads through gh,
//     because a private asset's plain download URL 404s whatever the token.
//
// # Scope: the binary, deliberately
//
// An installed REPOSITORY carries its own pin in .pyxis/config.yaml and moves
// forward by converge; this package never touches a repository. The caller
// prints a pointer at that path when the repo it runs in is behind.
package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/resolve"
)

// DefaultRepo is the release repository the GitHub leg reads.
const DefaultRepo = "agenticlab-ai/pyxis"

// InstallLine is the public one-liner every "install one first" refusal names.
// It reads the default branch's install.sh, so the line never goes stale between
// releases and names no version.
const InstallLine = "curl -fsSL https://raw.githubusercontent.com/agenticlab-ai/pyxis/main/install.sh | sh"

// publicPinsURL is where repo's public GitHub leg reads its pins. It reads the
// default branch and never a tag; see the package comment.
func publicPinsURL(repo string) string {
	return "https://raw.githubusercontent.com/" + repo + "/main/install.sh"
}

// publicAssetURL is repo's anonymous release download, the same layout as
// resolve.DefaultChannelRoot. GitHub answers it with a redirect to its object
// host, which redirectPolicy admits.
func publicAssetURL(repo, version, name string) string {
	return "https://github.com/" + repo + "/releases/download/" + version + "/" + name
}

// maxArtifactBytes bounds a pre-verification read. Release binaries run
// ~9 MB; this leaves generous headroom while refusing to buffer a hostile
// stream without limit.
const maxArtifactBytes = 512 << 20

// Runner executes a command and returns its stdout — the gh seam a private
// GitHub fork reads through. Tests inject a transcript.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

// Doer performs an HTTP request — the public and channel legs' seam.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Options is Run's closed operand.
type Options struct {
	Release *integrations.ResolvedRepository // explicit Pyxis artifact source; independent of project releases
	Root    string
	Repo    string // owner/name; "" uses DefaultRepo
	// Channel is a mirror's root URL. "" selects public GitHub Releases, and
	// Run treats the default root or the retired channel the same way.
	Channel string
	Current string // the running build's version; "source" refuses (see Run)
	// To / SHA256 install an EXPLICIT release instead of the channel's pin,
	// mirroring install.sh's own escape hatch (PYXIS_VERSION + PYXIS_SHA256):
	// the channel's pins describe one release, so any other target's digest
	// must come from the operator. Both or neither.
	To     string
	SHA256 string
	// ManifestSHA256 is the target release's manifest digest, required when
	// To names a release the channel does not pin: the pin hand-off is
	// unconditional, so an install with no manifest digest to record would
	// leave `pyxis init` resolving the previous release.
	ManifestSHA256 string
	// AllowDowngrade admits a pinned release OLDER than the installed one —
	// a channel rollback. Never implicit: moving backwards is a decision.
	// An explicit To needs no such flag: naming the release IS the intent.
	AllowDowngrade bool
	Platform       string // "os-arch"; "" uses this machine's
	TargetPath     string // the executable to replace; "" uses os.Executable
	ConfigDir      string // release.env's directory; "" uses the XDG default
	Preview        bool   // report the would-do set, perform none of it
	Runner         Runner // nil uses execRunner; only a private GitHub fork reads through it
	HTTP           Doer   // nil uses a default client
	// ShellSetup reinstalls the zsh worktree launcher from the installed
	// binary and returns its one-line account ("" when that release carries
	// no launcher). The launcher is embedded per release, so the installed
	// binary supplies it, never this process. The command wires
	// InstalledShellSetup; nil performs no shell setup.
	ShellSetup func(ctx context.Context, binary string) (string, error)
}

// Report is Run's account of what happened (or, in preview, would).
type Report struct {
	Current    string // the running version
	Target     string // the version the channel pins (or --to)
	UpToDate   bool   // the running version already matches the target
	BinaryPath string // the executable replaced
	Artifact   string // the release artifact name
	SHA256     string // the verified digest
	Downgrade  bool   // the target is older than the installed version
	// BinaryReplaced records that the executable was ALREADY swapped when a
	// later step failed. The caller must disclose it: the alternative is an
	// operator told only "upgrade failed" while holding a new binary.
	BinaryReplaced bool
	PinRecorded    string // the release.env written ("" when none)
	// PinRepaired marks a pin the run reconciled without installing
	// anything — the recovery path after a swap that failed to record it.
	PinRepaired bool
	Shell       string // the installed binary's account of its zsh setup ("" when none ran)
	Preview     bool
}

// pins is install.sh's BEGIN PINS block: the trust anchor.
type pins struct {
	Version      string
	ManifestSHA  string
	PlatformSHAs map[string]string // "darwin-arm64" → sha256
}

var (
	// assignRE captures every `KEY="value"` line the pins block can carry,
	// valid or not. Validation happens AFTER the effective final assignment
	// is known: matching only well-formed values would let a later block
	// that blanks a key silently inherit an earlier release's value.
	assignRE = regexp.MustCompile(`(?m)^(PINNED_VERSION|MANIFEST_SHA256|sha_[a-z0-9]+_[a-z0-9]+)="([^"]*)"`)
	sha256RE = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// parsePins reads install.sh's pins block: last assignment wins per key, as
// `sh` itself would resolve the file, and each effective value is validated
// only once it is known to be the one in force. A copy without a usable
// PINNED_VERSION is not a pins-bearing copy and is refused rather than
// guessed at — the same refusal the release mirror makes.
func parsePins(installSH string) (pins, error) {
	effective := map[string]string{}
	for _, m := range assignRE.FindAllStringSubmatch(installSH, -1) {
		effective[m[1]] = m[2]
	}
	version := strings.TrimSpace(effective["PINNED_VERSION"])
	if version == "" {
		return pins{}, errors.New("upgrade: the channel's install.sh carries no PINNED_VERSION line — not a pins-bearing copy")
	}
	p := pins{Version: version, PlatformSHAs: map[string]string{}}
	for key, value := range effective {
		if !strings.HasPrefix(key, "sha_") {
			continue
		}
		if !sha256RE.MatchString(value) {
			continue // in force but unusable ⇒ that platform is unpinned
		}
		os, arch, found := strings.Cut(strings.TrimPrefix(key, "sha_"), "_")
		if !found {
			continue
		}
		p.PlatformSHAs[os+"-"+arch] = value
	}
	if len(p.PlatformSHAs) == 0 {
		return pins{}, fmt.Errorf("upgrade: install.sh pins %s but carries no usable per-platform sha lines", p.Version)
	}
	// Required, not optional: the manifest digest is what `pyxis init` and
	// converge resolve every later artifact through. Installing a binary
	// while leaving an older pin behind is a half-upgrade, so a block whose
	// effective value is missing or malformed is refused before any fetch —
	// never quietly satisfied by an earlier release's digest.
	p.ManifestSHA = strings.TrimSpace(effective["MANIFEST_SHA256"])
	if !sha256RE.MatchString(p.ManifestSHA) {
		return pins{}, fmt.Errorf("upgrade: install.sh pins %s but carries no usable MANIFEST_SHA256 — refusing a half-upgrade that would leave an older pin for `pyxis init`", p.Version)
	}
	return p, nil
}

// Platform renders this machine's platform in the release's naming.
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// artifactName is the release's binary naming: pyxis-<version>-<platform>.
func artifactName(version, platform string) string {
	return "pyxis-" + version + "-" + platform
}

// Run performs the upgrade (or, with Preview, reports it and writes nothing).
func Run(ctx context.Context, opts Options) (Report, error) {
	opts.Channel = effectiveChannel(opts.Channel)
	if opts.Release != nil && opts.Channel != "" {
		return Report{}, fmt.Errorf("upgrade: choose either an artifact repository or a channel, not both")
	}
	if opts.Repo == "" {
		opts.Repo = DefaultRepo
	}
	if opts.Release != nil {
		opts.Repo = opts.Release.Repository
	}
	if opts.Platform == "" {
		opts.Platform = Platform()
	}
	if opts.Runner == nil {
		runner := execRunner{}
		if opts.Release != nil && opts.Release.Provider == integrations.GitHub {
			runner.tokenEnv = opts.Release.TokenEnv
			runner.host = "github.com"
		}
		opts.Runner = runner
	}
	if opts.HTTP == nil {
		// No client timeout: ctx governs cancellation end to end, the same
		// posture internal/resolve's HTTPFetcher documents. A blanket
		// deadline here would fail a slow but healthy 9 MB download.
		opts.HTTP = &http.Client{CheckRedirect: redirectPolicy}
	}

	// A build that does not know its own version was not installed by
	// install.sh: under `go run` the executable is a throwaway in a build
	// cache, so replacing it would report success and change nothing an
	// operator can run tomorrow.
	if opts.Current == "" || opts.Current == "source" {
		return Report{}, fmt.Errorf("upgrade: this pyxis reports version %q — it is a source build, not an installed release, and has no installed CLI to replace; install one first:\n  %s", opts.Current, InstallLine)
	}
	if opts.Channel != "" {
		if err := assertTLSChannel(opts.Channel); err != nil {
			return Report{}, err
		}
	}
	if err := checkExplicitFlags(opts); err != nil {
		return Report{}, err
	}

	target, err := resolveTargetPath(opts.TargetPath)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Current: opts.Current, BinaryPath: target, Preview: opts.Preview}

	explicit := opts.To != ""
	// A fully specified target carries everything verification and the pin
	// hand-off need, so the channel is never consulted: an unreachable or
	// malformed install.sh must not block installing a release the operator
	// has already pinned by hand.
	selfContained := explicit && opts.ManifestSHA256 != ""

	var p pins
	if !selfContained {
		p, err = discover(ctx, opts)
		if err != nil {
			return rep, err
		}
	}

	want, sum, manifest := p.Version, "", p.ManifestSHA
	if explicit {
		want, sum = opts.To, opts.SHA256
		manifest = opts.ManifestSHA256
		if manifest == "" {
			// Only the pinned release's manifest digest is discoverable; for
			// any other target it must come from the operator, or the pin
			// hand-off below could not honestly happen.
			if p.Version != want {
				return rep, fmt.Errorf("upgrade: --to %s also needs --manifest-sha256 — the channel pins %s, so only THAT release's manifest digest is discoverable, and installing without one would leave `pyxis init` resolving %s", want, p.Version, p.Version)
			}
			// Matching the version is not enough to BORROW that release's
			// manifest: the manifest describes the release whose binary the
			// channel pins, so reusing it for different bytes would record a
			// pin that does not describe what was installed. The supplied
			// digest must therefore BE the channel's digest for this
			// platform — otherwise the operator is installing something else
			// and owes its manifest too.
			pinned, ok := p.PlatformSHAs[opts.Platform]
			if !ok {
				return rep, fmt.Errorf("upgrade: --to %s also needs --manifest-sha256 — the channel pins %s but no sha256 for platform %s, so there is no release whose manifest could describe these bytes", want, p.Version, opts.Platform)
			}
			if !strings.EqualFold(sum, pinned) {
				return rep, fmt.Errorf("upgrade: --sha256 %s is not the digest the channel pins for %s on %s (%s) — these are different bytes than that release's, so its manifest cannot describe them; pass --manifest-sha256 for what you are installing, or drop --to/--sha256 to install the pinned release", strings.ToLower(sum), want, opts.Platform, pinned)
			}
			manifest = p.ManifestSHA
		}
	} else {
		pinned, ok := p.PlatformSHAs[opts.Platform]
		if !ok {
			return rep, fmt.Errorf("upgrade: the release pins no sha256 for platform %s (pinned platforms: %s)", opts.Platform, strings.Join(sortedKeys(p.PlatformSHAs), ", "))
		}
		sum = pinned
	}
	rep.Target = want
	rep.Artifact = artifactName(want, opts.Platform)
	rep.SHA256 = sum

	// Already installed: the binary needs nothing, but the PIN might — a
	// previous run can have swapped the binary and then failed to record it
	// (rep.BinaryReplaced), and telling the operator to re-run only helps if
	// the re-run actually repairs that. So this path reconciles the pin
	// rather than returning early.
	if opts.Current == want {
		rep.UpToDate = true
		if opts.Preview {
			return rep, nil
		}
		path, repaired, perr := reconcileReleaseEnv(opts.ConfigDir, want, manifest, opts.Channel, opts.Release)
		if perr != nil {
			return rep, perr
		}
		if repaired {
			rep.PinRecorded = path
			rep.PinRepaired = true
		}
		// A current binary can still sit beside an older launcher: an earlier
		// run can have swapped the binary and then failed this step, so a
		// re-run repairs it the same way it repairs the pin.
		err := refreshShell(ctx, opts, target, &rep)
		return rep, err
	}

	// Moving BACKWARDS is a decision, never a side effect of running
	// upgrade: a channel that rolls its pin back would otherwise replace a
	// newer installed binary with an older artifact, verifying cleanly the
	// whole way (the digests genuinely describe the older release).
	direction, comparable := compareVersions(want, opts.Current)
	rep.Downgrade = comparable && direction < 0
	switch {
	case explicit: // naming the release IS the intent (#1028)
	case !comparable && !opts.AllowDowngrade:
		return rep, fmt.Errorf("upgrade: cannot tell whether %s is newer or older than the installed %s — pass --allow-downgrade to install it anyway", want, opts.Current)
	case comparable && direction < 0 && !opts.AllowDowngrade:
		return rep, fmt.Errorf("upgrade: the channel pins %s, which is OLDER than the installed %s — refusing to move backwards; pass --allow-downgrade to install it anyway", want, opts.Current)
	}

	if opts.Preview {
		return rep, nil
	}

	// Both destinations are checked BEFORE any fetch, so an unwritable one
	// costs no download and — more importantly — cannot strand the install
	// half-done, with the new binary in place and the pin still naming the
	// old release.
	if err := assertReplaceable(target); err != nil {
		return rep, err
	}
	pinDir, err := releaseEnvDir(opts.ConfigDir)
	if err != nil {
		return rep, err
	}
	if err := os.MkdirAll(pinDir, 0o755); err != nil {
		return rep, fmt.Errorf("upgrade: create %s (the release pin's home): %w", pinDir, err)
	}
	if err := assertReplaceable(filepath.Join(pinDir, "release.env")); err != nil {
		return rep, err
	}

	blob, err := fetchArtifact(ctx, opts, want, rep.Artifact)
	if err != nil {
		return rep, err
	}
	got := sha256.Sum256(blob)
	if !strings.EqualFold(hex.EncodeToString(got[:]), sum) {
		return rep, fmt.Errorf("upgrade: checksum mismatch for %s — refusing to install\n  want %s\n  got  %s", rep.Artifact, strings.ToLower(sum), hex.EncodeToString(got[:]))
	}
	if err := replaceExecutable(target, blob); err != nil {
		return rep, err
	}
	rep.BinaryReplaced = true

	// The pin hand-off is unconditional (prd:installer's contract): every
	// install this verb performs leaves `pyxis init` resolving the release
	// that was just installed, explicit target or not.
	path, werr := writeReleaseEnv(opts.ConfigDir, want, manifest, opts.Channel, opts.Release)
	if werr != nil {
		return rep, werr
	}
	rep.PinRecorded = path
	err = refreshShell(ctx, opts, target, &rep)
	return rep, err
}

// refreshShell brings the zsh worktree launcher onto the installed release, so
// a new harness or a changed worktree lifecycle arrives with the binary that
// ships it.
func refreshShell(ctx context.Context, opts Options, binary string, rep *Report) error {
	if opts.ShellSetup == nil {
		return nil
	}
	account, err := opts.ShellSetup(ctx, binary)
	if err != nil {
		return fmt.Errorf("upgrade: pyxis %s is installed, but its zsh worktree setup failed: %w\n  retry: %s shell install", rep.Target, err, binary)
	}
	rep.Shell = account
	return nil
}

// InstalledShellSetup runs the installed binary's own `shell install`, the
// same step install.sh ends with. A release from before that verb refuses its
// help and carries no launcher, so there is nothing to install from it.
func InstalledShellSetup(ctx context.Context, binary string) (string, error) {
	if exec.CommandContext(ctx, binary, "shell", "install", "--help").Run() != nil {
		return "", nil
	}
	var out, errOut strings.Builder
	cmd := exec.CommandContext(ctx, binary, "shell", "install")
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("%w: %s", err, firstLine(msg))
		}
		return "", err
	}
	return strings.TrimPrefix(firstLine(strings.TrimSpace(out.String())), "pyxis: "), nil
}

// checkExplicitFlags validates the --to / --sha256 / --manifest-sha256
// grammar before anything is fetched or written. --to and --sha256 travel
// together — the channel's pins describe one release, so any other target's
// binary digest comes from the operator, exactly as install.sh requires of
// PYXIS_VERSION + PYXIS_SHA256 — and a digest with no target is a typo, not
// an instruction.
func checkExplicitFlags(opts Options) error {
	switch {
	case opts.To != "" && opts.SHA256 == "":
		return fmt.Errorf("upgrade: --to %s needs its sha256 — the channel's pins describe only the release it pins, so an explicit target's digest must come from you (as install.sh requires of PYXIS_VERSION + PYXIS_SHA256): pyxis upgrade --to %s --sha256 <hex>", opts.To, opts.To)
	case opts.To == "" && opts.SHA256 != "":
		return errors.New("upgrade: --sha256 names a digest for no target — pass --to <version> with it, or drop it to install the release the channel pins")
	case opts.To == "" && opts.ManifestSHA256 != "":
		return errors.New("upgrade: --manifest-sha256 names a manifest for no target — pass --to <version> with it, or drop it to install the release the channel pins")
	case opts.To != "" && !sha256RE.MatchString(opts.SHA256):
		return fmt.Errorf("upgrade: --sha256 %q is not a 64-character sha256 digest", opts.SHA256)
	case opts.ManifestSHA256 != "" && !sha256RE.MatchString(opts.ManifestSHA256):
		return fmt.Errorf("upgrade: --manifest-sha256 %q is not a 64-character sha256 digest", opts.ManifestSHA256)
	}
	return nil
}

// reconcileReleaseEnv brings the release pin into agreement with the
// installed version, reporting whether it had to. It is what makes "re-run
// upgrade" a real remedy after a run that swapped the binary and then failed
// to record the pin: without it the re-run would see a current binary and
// return, leaving the stale pin in place forever.
func reconcileReleaseEnv(configDir, version, manifestSHA, channel string, source ...*integrations.ResolvedRepository) (path string, repaired bool, err error) {
	dir, err := releaseEnvDir(configDir)
	if err != nil {
		return "", false, err
	}
	path = filepath.Join(dir, "release.env")
	if current, rerr := os.ReadFile(path); rerr == nil && string(current) == releaseEnvBody(version, manifestSHA, channel, source...) {
		return path, false, nil // already agrees; nothing to do
	}
	written, werr := writeReleaseEnv(configDir, version, manifestSHA, channel, source...)
	if werr != nil {
		return "", false, werr
	}
	return written, true, nil
}

// resolveTargetPath resolves the executable to replace, following symlinks so
// the real file is swapped rather than the link.
func resolveTargetPath(override string) (string, error) {
	path := override
	if path == "" {
		p, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("upgrade: locate the running pyxis binary: %w", err)
		}
		path = p
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

// effectiveChannel maps a channel naming the public default, or the retired
// distribution host an older install recorded, to "" — the public GitHub leg.
// The default root has no <root>/install.sh to read, and the retired host no
// longer answers, so honouring either literally would strand the machine.
func effectiveChannel(channel string) string {
	root := strings.TrimRight(strings.TrimSpace(channel), "/")
	if root == resolve.DefaultChannelRoot || releaseenv.IsRetiredChannel(root) {
		return ""
	}
	return channel
}

// publicGitHub reports whether the run reads public GitHub Releases
// anonymously: no mirror, and no repository binding that names a token.
func publicGitHub(opts Options) bool {
	return opts.Channel == "" && (opts.Release == nil || (opts.Release.Provider == integrations.GitHub && opts.Release.TokenEnv == ""))
}

// discover reads the pins block from whichever leg the options select.
func discover(ctx context.Context, opts Options) (pins, error) {
	if opts.Release != nil && opts.Release.Provider == integrations.Forgejo {
		c, err := forgerelease.New(*opts.Release)
		if err != nil {
			return pins{}, err
		}
		body, err := c.ReadInstall(ctx)
		if err != nil {
			return pins{}, err
		}
		return parsePins(string(body))
	}
	if opts.Channel != "" {
		body, err := channelGet(ctx, opts, strings.TrimRight(opts.Channel, "/")+"/install.sh")
		if err != nil {
			return pins{}, err
		}
		return parsePins(string(body))
	}
	// The default branch's install.sh — the one the documented install
	// one-liner runs — is the statement of "the release you should be on", and
	// the only copy whose digests describe artifacts that exist. Upgrade
	// therefore installs exactly what a fresh install would, by construction,
	// and never consults "latest": during a release window the tag exists
	// before its pins do, and the pinned release is the one whose bytes can be
	// verified.
	if publicGitHub(opts) {
		body, err := channelGet(ctx, opts, publicPinsURL(opts.Repo))
		if err != nil {
			return pins{}, fmt.Errorf("upgrade: read install.sh from %s (its pins are the trust anchor): %w", opts.Repo, err)
		}
		return parsePins(string(body))
	}
	out, err := opts.Runner.Run(ctx, "gh", "api",
		"repos/"+opts.Repo+"/contents/install.sh",
		"-H", "Accept: application/vnd.github.raw")
	if err != nil {
		return pins{}, fmt.Errorf("upgrade: read install.sh from %s (its pins are the trust anchor): %w", opts.Repo, err)
	}
	return parsePins(out)
}

// fetchArtifact serves the platform binary's bytes from the selected leg.
func fetchArtifact(ctx context.Context, opts Options, version, name string) ([]byte, error) {
	if opts.Release != nil && opts.Release.Provider == integrations.Forgejo {
		c, err := forgerelease.New(*opts.Release)
		if err != nil {
			return nil, err
		}
		body, err := c.Open(ctx, version, name)
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(body, maxArtifactBytes+1))
		err = errors.Join(err, body.Close())
		if err != nil {
			return nil, err
		}
		if len(b) > maxArtifactBytes {
			return nil, fmt.Errorf("upgrade: Forgejo artifact exceeds size limit")
		}
		return b, nil
	}
	if opts.Channel != "" {
		return channelGet(ctx, opts, strings.TrimRight(opts.Channel, "/")+"/"+version+"/"+name)
	}
	if publicGitHub(opts) {
		return channelGet(ctx, opts, publicAssetURL(opts.Repo, version, name))
	}
	dir, err := os.MkdirTemp("", "pyxis-upgrade-*")
	if err != nil {
		return nil, fmt.Errorf("upgrade: staging dir: %w", err)
	}
	defer func() {
		// Best-effort download scratch cleanup; preserve the verification result.
		_ = os.RemoveAll(dir)
	}()
	if _, err := opts.Runner.Run(ctx, "gh", "release", "download", version,
		"--repo", opts.Repo, "--pattern", name, "--dir", dir, "--clobber"); err != nil {
		return nil, fmt.Errorf("upgrade: download %s from release %s: %w", name, version, err)
	}
	blob, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("upgrade: read the downloaded %s: %w", name, err)
	}
	return blob, nil
}

// channelGet fetches one anonymous HTTPS URL for the public or mirror leg.
func channelGet(ctx context.Context, opts Options, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade: build request for %s: %w", rawURL, err)
	}
	resp, err := opts.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upgrade: fetch %s: %w", rawURL, err)
	}
	defer func() {
		// Response status and body reads determine the result; closing is cleanup.
		_ = resp.Body.Close()
	}()
	switch {
	case resp.StatusCode == http.StatusNotFound && opts.Channel != "" && strings.HasSuffix(rawURL, "/install.sh"):
		return nil, fmt.Errorf("upgrade: %s answered 404 — --channel wants the channel ROOT (which serves install.sh), not a versioned artifact directory like PYXIS_ARTIFACT_SOURCE's <root>/vX.Y.Z", rawURL)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("upgrade: fetch %s: %s", rawURL, resp.Status)
	}
	// Capped: these bytes are read BEFORE any digest exists to check them
	// against, so a hostile channel must not be able to exhaust memory.
	blob, err := io.ReadAll(io.LimitReader(resp.Body, maxArtifactBytes))
	if err != nil {
		return nil, fmt.Errorf("upgrade: read %s: %w", rawURL, err)
	}
	return blob, nil
}

// assertTLSChannel refuses a channel that is not TLS. The channel supplies this
// verb's trust anchor: over plain http an intermediary can serve a forged
// install.sh together with a binary matching its forged digest, and
// verification would pass while installing the attacker's bytes.
func assertTLSChannel(channel string) error {
	u, err := url.Parse(strings.TrimRight(channel, "/"))
	if err != nil {
		return fmt.Errorf("upgrade: parse --channel %q: %w", channel, err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("upgrade: --channel must be https (got %q) — the channel supplies both the pins this verb trusts and the binary it installs, so plain %s would let an intermediary choose the bytes AND the digest they are checked against", channel, u.Scheme)
	}
	return nil
}

// redirectPolicy follows redirects across hosts, which GitHub's release
// downloads require (github.com answers with its object host), but never off
// TLS: a downgrade would let an intermediary serve the pins or the binary (see
// assertTLSChannel).
func redirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("upgrade: stopped after 10 redirects")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("upgrade: refusing to follow a redirect to plain %s at %s — the release channel must stay on TLS", req.URL.Scheme, req.URL.Host)
	}
	return nil
}

// release is a parsed version: the numeric core plus any prerelease tail.
type release struct {
	core [3]int
	pre  string // "" for a final release; "rc1" for v1.2.4-rc1
}

// compareVersions orders two release versions, tolerating a leading "v" and
// a prerelease tail. ok is false when the pair cannot be ordered honestly —
// a non-numeric core, or two DIFFERENT prereleases of the same core, whose
// precedence this package will not invent. The caller treats "cannot order"
// as needing explicit intent, never as "safe to proceed".
func compareVersions(a, b string) (direction int, ok bool) {
	ra, oka := parseRelease(a)
	rb, okb := parseRelease(b)
	if !oka || !okb {
		return 0, false
	}
	for i := range 3 {
		if ra.core[i] != rb.core[i] {
			if ra.core[i] > rb.core[i] {
				return 1, true
			}
			return -1, true
		}
	}
	switch {
	case ra.pre == rb.pre:
		return 0, true
	case ra.pre == "": // a final release outranks its own prereleases
		return 1, true
	case rb.pre == "":
		return -1, true
	default:
		return 0, false // rc1 vs rc2: not this package's call
	}
}

// parseRelease reads "vX.Y.Z" with an optional "-<prerelease>" tail. Missing
// core components are zero. ok is false when any PRESENT core component is
// non-numeric: treating "1.2.4-rc1" as 1.2.0 (the defect this replaces) let
// a genuinely older release read as an upgrade.
func parseRelease(v string) (release, bool) {
	body := strings.TrimPrefix(strings.TrimSpace(v), "v")
	if body == "" {
		return release{}, false
	}
	var r release
	if core, pre, found := strings.Cut(body, "-"); found {
		body, r.pre = core, pre
		if r.pre == "" {
			return release{}, false // "1.2.3-" is malformed, not final
		}
	}
	parts := strings.Split(body, ".")
	if len(parts) > 3 {
		return release{}, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return release{}, false
		}
		r.core[i] = n
	}
	return r, true
}

// assertReplaceable refuses, before any download, when the installed binary
// cannot be swapped — naming the path and the one-liner that can.
func assertReplaceable(target string) error {
	dir := filepath.Dir(target)
	probe, err := os.CreateTemp(dir, ".pyxis-upgrade-probe-*")
	if err != nil {
		return fmt.Errorf("upgrade: %s is not writable, so the installed pyxis cannot be replaced there (%v) — re-run the install line instead:\n  %s", dir, err, InstallLine)
	}
	name := probe.Name()
	_ = probe.Close() // the probe's contents are irrelevant; only creatability was
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("upgrade: remove the writability probe %s: %w", name, err)
	}
	return nil
}

// replaceExecutable installs blob over target atomically: a temp file in the
// SAME directory (so the rename cannot cross filesystems), then rename. On
// unix, renaming over a running binary is safe — the running process keeps
// its open inode and the next invocation gets the new bytes.
func replaceExecutable(target string, blob []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".pyxis-upgrade-*")
	if err != nil {
		return fmt.Errorf("upgrade: stage the new binary beside %s: %w", target, err)
	}
	staged := tmp.Name()
	// ENOENT once the rename below succeeds (the path is gone); this only
	// bites on the failure paths, where the staged file must not survive.
	defer func() { _ = os.Remove(staged) }()
	if _, err := tmp.Write(blob); err != nil {
		_ = tmp.Close() // Preserve the write failure over cleanup errors.
		return fmt.Errorf("upgrade: write the new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("upgrade: close the staged binary: %w", err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return fmt.Errorf("upgrade: make the staged binary executable: %w", err)
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("upgrade: install the new binary over %s: %w", target, err)
	}
	return nil
}

// writeReleaseEnv records the pin exactly as install.sh does, so `pyxis init`
// keeps needing no pin flags after an upgrade.
func writeReleaseEnv(configDir, version, manifestSHA, channel string, source ...*integrations.ResolvedRepository) (string, error) {
	dir, err := releaseEnvDir(configDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("upgrade: create %s: %w", dir, err)
	}
	path := filepath.Join(dir, "release.env")
	if err := os.WriteFile(path, []byte(releaseEnvBody(version, manifestSHA, channel, source...)), 0o644); err != nil {
		return "", fmt.Errorf("upgrade: record the release pin in %s: %w", path, err)
	}
	return path, nil
}

// releaseEnvBody is the pin file's exact content. releaseenv.Body is the one
// serializer — install.sh writes the same shape — so an upgrade cannot quietly
// drop a key the installer recorded (prd:installer/FR-30).
func releaseEnvBody(version, manifestSHA, channel string, source ...*integrations.ResolvedRepository) string {
	r := releaseenv.Record{Version: version, ManifestSHA: manifestSHA, Channel: channel}
	if len(source) > 0 && source[0] != nil {
		s := source[0]
		r.ReleaseProvider = s.Provider
		r.ReleaseBaseURL = s.BaseURL
		r.ReleaseRepository = s.Repository
		r.ReleaseTokenEnv = s.TokenEnv
	}
	return releaseenv.Body(r)
}

// releaseEnvDir resolves where install.sh keeps the release pin.
func releaseEnvDir(configDir string) (string, error) {
	if configDir != "" {
		return configDir, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("upgrade: resolve the config dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "pyxis"), nil
}

// RepoPin reads an installed repository's pinned version through the repo
// config's own closed parser — never a second, looser reading of the same
// file. It exists so the caller can point a behind repo at its own upgrade
// path; "" means "not an installed repo, or no pin to report".
func RepoPin(root string) string {
	blob, err := os.ReadFile(filepath.Join(root, ".pyxis", "config.yaml"))
	if err != nil {
		return ""
	}
	cfg, defects := config.Parse(blob)
	if len(defects) > 0 {
		return "" // a malformed config is doctor's to report, not upgrade's
	}
	return cfg.System.Version
}

// execRunner is the production Runner: gh, under a private fork's named token.
type execRunner struct{ tokenEnv, host string }

func (r execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var err error
	cmd.Env, err = githubauth.EnvironmentForHost(r.tokenEnv, r.host)
	if err != nil {
		return "", err
	}
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(errOut.String()); msg != "" {
			return "", fmt.Errorf("%s: %w: %s", name, err, firstLine(msg))
		}
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return out.String(), nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(keys []string) {
	// Small, fixed-size set; a simple insertion sort keeps this dependency-free.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
}
