package release

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// The install.sh pin block — the ONE trust anchor a consumer has before any
// pyxis byte runs (installer-v1 §3.4: an anchor deliberately independent of
// the release manifest, since parsing the manifest already requires the
// binary the anchor gates). install.sh is served from the source repo's default
// branch over TLS, so its committed bytes ARE the root: it fetches `pyxis-<version>-<platform>`, hashes it, and refuses
// on any mismatch.
//
// Those five hashes were hand-transcribed until now — the step the mirror's
// stale-pins precondition refuses on, with no verb behind its own "regenerate
// the pins first" instruction. Pins is that verb: it DERIVES the block from
// an artifact set (a locally packed one, or the published release itself)
// and splices it into install.sh, so the anchor is generated, never typed.
//
// The pinned tool is the front door alone: install.sh installs `pyxis` and
// nothing else, and every further artifact is gated by the config-pinned
// manifest hash from there on (DINV-1/DINV-2).
const frontDoorTool = "pyxis"

// installSHName is the front-door script's fixed repo-root path. Fixed here
// like the artifact names: a bootstrap-era binding, one place.
const installSHName = "install.sh"

// PinSet is the derived anchor for one release: the release manifest's hash
// (handed to `pyxis init` as the config pin) plus the front-door binary's
// hash per platform.
type PinSet struct {
	Version     string
	ManifestSHA string
	Binaries    map[string]string // platform → sha256 of pyxis-<version>-<platform>
}

// PinsOptions is Pins' closed operand.
type PinsOptions struct {
	Target *integrations.ResolvedRepository
	// Version is the release the pins point at (vX.Y.Z).
	Version string
	// RepoRoot is the checkout whose install.sh is rewritten ("" = ".").
	RepoRoot string
	// OutDir is a locally packed artifact set to derive from. When empty,
	// Pins fetches the pinned artifacts from the PUBLISHED release instead
	// (SourceRepo + Runner) — the operator leg, which needs no local build
	// and derives the anchor from exactly the bytes consumers will fetch.
	OutDir string
	// SourceRepo is the canonical release host for the remote leg,
	// "owner/name". Unused when OutDir is set.
	SourceRepo string
	// Runner is the gh transport for the remote leg.
	Runner wire.Runner
	// Check derives and compares without writing: a divergent install.sh is
	// a named refusal instead of a rewrite (the CI/verification posture).
	Check bool
}

// PinsResult reports the derived set, what happened to install.sh
// ("written" | "current" | "checked"), and where the pins came from.
type PinsResult struct {
	Pins   PinSet
	Action string
	Source string
}

// pinsBlockRe matches the whole generated block, BEGIN line through END line
// inclusive. Non-greedy so a malformed file with two blocks is caught by the
// marker count below rather than silently swallowed. The trailing newline is
// optional: a file whose END PINS is the last line without one is perfectly
// legal, and requiring it would refuse that file as "unterminated".
var pinsBlockRe = regexp.MustCompile(`(?ms)^# BEGIN PINS[^\n]*\n.*?^# END PINS[^\n]*(\n|$)`)

var (
	pinsBeginRe = regexp.MustCompile(`(?m)^# BEGIN PINS[^\n]*$`)
	pinsEndRe   = regexp.MustCompile(`(?m)^# END PINS[^\n]*$`)
)

// Pins regenerates install.sh's pin block for Version from a verified
// artifact set and reports what it did (installer-v1 §3.4).
//
// Fail-closed throughout, because this writes the trust anchor:
//
//   - the local leg runs Verify first — an anchor is never derived from a
//     set that does not match its own checksums and manifest;
//   - the remote leg downloads the five pinned artifacts and hashes THE
//     BYTES, then cross-checks them against the release's SHA256SUMS — a
//     divergence between the two is a named refusal, not a preference;
//   - the spliced result is re-parsed exactly the way mirror's precondition
//     and install.sh itself read it, so a write that would strand consumers
//     cannot land.
//
// Re-runnable (AP-22): an install.sh already carrying the derived block is
// reported "current" and left untouched, byte for byte.
func Pins(ctx context.Context, opts PinsOptions) (PinsResult, error) {
	if err := checkVersion(opts.Version); err != nil {
		return PinsResult{}, err
	}
	root := opts.RepoRoot
	if root == "" {
		root = "."
	}
	path := filepath.Join(root, installSHName)
	current, err := os.ReadFile(path)
	if err != nil {
		return PinsResult{}, fmt.Errorf("release: pins: read %s (the file whose anchor is being regenerated): %w", path, err)
	}

	var pins PinSet
	var source string
	if opts.OutDir != "" {
		if pins, err = pinsFromPacked(opts.OutDir, opts.Version); err != nil {
			return PinsResult{}, err
		}
		source = "the packed set in " + opts.OutDir
	} else if opts.Target != nil && opts.Target.Provider == integrations.Forgejo {
		if pins, err = pinsFromForgejo(ctx, *opts.Target, opts.Version); err != nil {
			return PinsResult{}, err
		}
		source = "the published release " + opts.Version + " on " + opts.Target.Key()
	} else {
		if opts.Target != nil {
			opts.SourceRepo = opts.Target.Repository
			opts.Runner = githubRunner(opts.Runner, opts.Target)
		}
		if opts.Runner == nil {
			return PinsResult{}, fmt.Errorf("release: pins: no gh runner wired for the published-release leg (programmer error)")
		}
		if opts.SourceRepo == "" {
			return PinsResult{}, fmt.Errorf("release: pins requires --out (a packed set) or a source repo to derive from")
		}
		if pins, err = pinsFromRelease(ctx, opts.Runner, opts.SourceRepo, opts.Version); err != nil {
			return PinsResult{}, err
		}
		source = "the published release " + opts.Version + " on " + opts.SourceRepo
	}

	next, err := splicePins(string(current), pins)
	if err != nil {
		return PinsResult{}, fmt.Errorf("release: pins: %s: %w", path, err)
	}
	if opts.Target != nil {
		next, err = installerForTarget(next, MirrorOptions{SourceRepo: opts.SourceRepo, DistRepo: opts.Target.Repository, DistTarget: opts.Target})
		if err != nil {
			return PinsResult{}, fmt.Errorf("release: pins provider: %w", err)
		}
	}
	// The self-check: read the produced bytes back exactly as mirror's
	// precondition and install.sh's own parser do. A block that renders but
	// does not READ as this version would sail past this verb and strand
	// consumers at the mirror instead.
	if err := checkPinnedVersion([]byte(next), opts.Version); err != nil {
		return PinsResult{}, fmt.Errorf("release: pins: %w", err)
	}

	res := PinsResult{Pins: pins, Source: source}
	switch {
	case next == string(current):
		res.Action = "current"
		if opts.Check {
			res.Action = "checked"
		}
	case opts.Check:
		return PinsResult{}, fmt.Errorf("release: pins --check failed — %s does not carry the pins %s derives from %s; run pins without --check to regenerate", path, opts.Version, source)
	default:
		if err := writeFileAtomic(path, []byte(next)); err != nil {
			return PinsResult{}, fmt.Errorf("release: pins: write %s: %w", path, err)
		}
		res.Action = "written"
	}
	return res, nil
}

// writeFileAtomic replaces path via a same-directory temp file and a rename,
// preserving the original's mode. install.sh is BOTH the consumer trust
// anchor and an executable script, so a truncating write that dies midway
// would leave a half-file that still parses as shell — the one failure mode
// worth spending a rename on.
func writeFileAtomic(path string, content []byte) error {
	mode := os.FileMode(0o755)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer func() {
		// Best-effort scratch cleanup; the atomic write determines the result.
		_ = os.Remove(tmp.Name())
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close() // Preserve the write failure over cleanup errors.
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close() // Preserve the permission failure over cleanup errors.
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// pinsFromPacked derives the anchor from a locally packed set. Verify runs
// first and covers both directions (SHA256SUMS ↔ bytes ↔ manifest), so the
// sums file is then a faithful index of the artifacts' bytes.
func pinsFromPacked(outDir, version string) (PinSet, error) {
	if err := Verify(outDir, version); err != nil {
		return PinSet{}, fmt.Errorf("release: pins refused — the set in %s failed verification, and a trust anchor is never derived from an unverified set: %w", outDir, err)
	}
	sums, err := readSums(filepath.Join(outDir, SumsName))
	if err != nil {
		return PinSet{}, err
	}
	return pinSetFrom(version, func(name string) (string, error) {
		sum, ok := sums[name]
		if !ok {
			return "", fmt.Errorf("%s is absent from %s in %s", name, SumsName, outDir)
		}
		return sum, nil
	})
}

// pinsFromRelease derives the anchor from the PUBLISHED release: it
// downloads the five pinned artifacts plus SHA256SUMS, hashes the bytes it
// actually received, and cross-checks them against the sums file. Hashing
// the bytes (rather than trusting the sums file alone) is the point — the
// anchor then describes exactly what a consumer's own download will carry.
func pinsFromRelease(ctx context.Context, r wire.Runner, repo, version string) (PinSet, error) {
	tmp, err := os.MkdirTemp("", "pyxis-pins-*")
	if err != nil {
		return PinSet{}, fmt.Errorf("release: pins: %w", err)
	}
	defer func() {
		// Best-effort scratch cleanup; preserve the release verification result.
		_ = os.RemoveAll(tmp)
	}()

	fetch := func(name string) (string, error) {
		dst := filepath.Join(tmp, name)
		if err := ghRunOp(ctx, r, "pins", []string{"release", "download", version, "--repo", repo, "--pattern", name, "--output", dst}); err != nil {
			return "", fmt.Errorf("release: pins: fetch %s from %s release %s: %w", name, repo, version, err)
		}
		sum, err := hashFile(dst)
		if err != nil {
			return "", fmt.Errorf("release: pins: hash the downloaded %s: %w", name, err)
		}
		return sum, nil
	}

	pins, err := pinSetFrom(version, fetch)
	if err != nil {
		return PinSet{}, err
	}
	// The cross-check: the release's own SHA256SUMS must agree with the
	// bytes we received for every artifact we pinned.
	if _, err := fetch(SumsName); err != nil {
		return PinSet{}, fmt.Errorf("release: pins: %s is the published set's checksum floor and must be present: %w", SumsName, err)
	}
	sums, err := readSums(filepath.Join(tmp, SumsName))
	if err != nil {
		return PinSet{}, err
	}
	var defects []string
	check := func(name, got string) {
		switch want, ok := sums[name]; {
		case !ok:
			defects = append(defects, fmt.Sprintf("%s: downloaded but absent from the release's %s", name, SumsName))
		case want != got:
			defects = append(defects, fmt.Sprintf("%s: the release's %s says %s but the downloaded bytes hash to %s", name, SumsName, want, got))
		}
	}
	check(manifestName(version), pins.ManifestSHA)
	for _, plat := range Platforms {
		check(frontDoorName(version, plat), pins.Binaries[plat])
	}
	if len(defects) > 0 {
		return PinSet{}, fmt.Errorf("release: pins refused — the published set for %s on %s is not self-consistent (fail closed; nothing was written):\n  %s", version, repo, strings.Join(defects, "\n  "))
	}
	return pins, nil
}

// pinSetFrom assembles the anchor by asking sumOf for each pinned artifact's
// hash, accumulating every miss so one run names the whole gap.
func pinSetFrom(version string, sumOf func(name string) (string, error)) (PinSet, error) {
	pins := PinSet{Version: version, Binaries: map[string]string{}}
	var defects []string
	sum, err := sumOf(manifestName(version))
	if err != nil {
		defects = append(defects, err.Error())
	}
	pins.ManifestSHA = sum
	for _, plat := range Platforms {
		sum, err := sumOf(frontDoorName(version, plat))
		if err != nil {
			defects = append(defects, err.Error())
			continue
		}
		pins.Binaries[plat] = sum
	}
	if len(defects) > 0 {
		return PinSet{}, fmt.Errorf("release: pins refused — the artifact set does not carry the pinned front-door set for %s (%s × %v, plus the release manifest):\n  %s",
			version, frontDoorTool, Platforms, strings.Join(defects, "\n  "))
	}
	return pins, nil
}

func frontDoorName(version, platform string) string {
	return frontDoorTool + "-" + version + "-" + platform
}

// Block renders the generated pin block install.sh carries — the exact text
// between (and including) its markers. Platform order is Platforms' order,
// so the block is a deterministic function of the release: two operators
// deriving the same tag produce byte-identical output.
func (p PinSet) Block() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# BEGIN PINS %s\n", p.Version)
	fmt.Fprintf(&b, "PINNED_VERSION=%q\n", p.Version)
	fmt.Fprintf(&b, "MANIFEST_SHA256=%q\n", p.ManifestSHA)
	for _, plat := range Platforms {
		fmt.Fprintf(&b, "sha_%s=%q\n", strings.ReplaceAll(plat, "-", "_"), p.Binaries[plat])
	}
	b.WriteString("# END PINS\n")
	return b.String()
}

// splicePins replaces the one BEGIN…END block in src. Exactly one marker
// pair may exist: zero means this is not a pins-bearing copy, and more than
// one means an ambiguous file where a rewrite could leave a stale block
// behind for install.sh to read instead.
func splicePins(src string, pins PinSet) (string, error) {
	begins, ends := len(pinsBeginRe.FindAllString(src, -1)), len(pinsEndRe.FindAllString(src, -1))
	switch {
	case begins == 0 || ends == 0:
		return "", fmt.Errorf("no `# BEGIN PINS` / `# END PINS` block — not a pins-bearing install.sh")
	case begins != 1 || ends != 1:
		return "", fmt.Errorf("carries %d `# BEGIN PINS` and %d `# END PINS` markers — exactly one block may exist, else a rewrite leaves a stale one behind", begins, ends)
	}
	block := pinsBlockRe.FindStringIndex(src)
	if block == nil {
		return "", fmt.Errorf("the `# END PINS` marker precedes `# BEGIN PINS`, or the block is unterminated")
	}
	return src[:block[0]] + pins.Block() + src[block[1]:], nil
}

// checkPinnedVersion reads install.sh bytes the way every consumer of the
// anchor does — mirror's precondition, and install.sh's own shell parse —
// and asserts they land on version.
func checkPinnedVersion(installSH []byte, version string) error {
	m := pinnedVersionRe.FindSubmatch(installSH)
	if m == nil {
		return fmt.Errorf("the regenerated install.sh carries no PINNED_VERSION line (refusing to write an unreadable anchor)")
	}
	if got := string(m[1]); got != version {
		return fmt.Errorf("the regenerated install.sh reads as %s, not %s (refusing to write a stranding anchor)", got, version)
	}
	return nil
}
