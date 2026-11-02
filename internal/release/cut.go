package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// Cut is the whole release runbook as one verb (installer-v1 §3.1): build
// the matrix, pack the set, verify it, publish it to the canonical host, and
// regenerate install.sh's pins from exactly what was published. That is the
// complete public GitHub release; only a repository with an explicit
// releases.distribution binding continues into a mirror.
//
// It exists because the runbook was a copy-paste sequence of five commands
// plus a hand-transcribed shell block, and every step of it is mechanical.
// The one step that is NOT mechanical stays a human gate: the pin block
// lands on main through a reviewed PR, so Cut writes install.sh and STOPS.
// With a distribution binding, a second run (with Mirror) after that PR merges
// publishes the mirror — re-running is free, because every stage is
// upload-or-verify (AP-22).
//
// The gate is not a style choice. install.sh's pins name the release
// manifest's hash, the manifest names the source commit, and install.sh is
// committed — so a tag can never carry the pins for its own artifacts, and
// the pin commit necessarily lands after the tag. Mirror's stale-pins
// precondition is what holds the two halves together; Cut refuses to mirror
// from an uncommitted install.sh for the same reason, since dist consumers
// would then be served pins the source repo's own front door does not have.
type CutOptions struct {
	SourceTarget *integrations.ResolvedRepository
	DistTarget   *integrations.ResolvedRepository
	// Version is the system tag being cut (vX.Y.Z).
	Version string
	// RepoRoot is the source checkout ("" = ".").
	RepoRoot string
	// OutDir holds the artifact set; "" is DefaultOutDir under RepoRoot.
	// When it already carries a set that verifies for Version, Cut REUSES
	// it rather than repacking, preserving the original published bytes
	// even if the source tree or build toolchain has since changed.
	OutDir string
	// BinariesDir supplies prebuilt binaries instead of building them; ""
	// builds the matrix into a temp dir.
	BinariesDir string
	// Repack forces a fresh build+pack over an existing verified set.
	Repack bool
	// PushTag creates the release tag on SourceRepo when it is absent,
	// anchored at the packed set's own source commit (tag == commit ==
	// artifacts). Default off: creating a tag is a real, public act.
	PushTag bool
	// Mirror publishes to the explicitly bound distribution target after the
	// pins are committed. Off for the first run of a release, and refused
	// without a DistTarget.
	Mirror bool
	// Jobs bounds concurrent compiler invocations (0 = GOMAXPROCS).
	Jobs int

	SourceRepo string // canonical release host, "owner/name"
	DistRepo   string // optional override of an explicitly bound distribution repo
	Runner     wire.Runner

	// Log receives one line per completed stage; nil discards.
	Log func(string)
}

// DefaultOutDir is the artifact directory the one-line runbook uses when
// none is given — the same `dist` the release workflow passes explicitly.
// Gitignored at the repo root: a release's artifact set is ~60 binaries
// plus archives, regenerable by re-running the verb, and never committed.
const DefaultOutDir = "dist"

// CutResult reports each stage's outcome.
type CutResult struct {
	Reused       bool          // the artifact set was reused, not rebuilt
	Build        *BuildResult  // nil when reused
	Pack         *PackResult   // nil when reused
	SourceCommit string        // the commit the artifacts were packed from
	Tag          string        // "present" | "created"
	Publish      PublishResult // the canonical publish
	Pins         PinsResult    // install.sh's regenerated anchor
	Mirror       *MirrorResult // nil unless Mirror ran
}

// Cut runs the release.
func Cut(ctx context.Context, opts CutOptions) (CutResult, error) {
	if err := checkVersion(opts.Version); err != nil {
		return CutResult{}, err
	}
	if opts.Runner == nil && (opts.SourceTarget == nil || opts.SourceTarget.Provider != integrations.Forgejo) {
		return CutResult{}, fmt.Errorf("release: cut: no gh runner wired (programmer error)")
	}
	if opts.SourceRepo == "" {
		return CutResult{}, fmt.Errorf("release: cut requires a source repo — the canonical release host")
	}
	if opts.Mirror && opts.DistTarget == nil {
		return CutResult{}, errNoDistributionBinding
	}
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	// Default the artifact dir UNDER the repo root, not the working
	// directory: --repo-root elsewhere would otherwise scatter the set
	// wherever the operator happened to stand, outside the .gitignore entry
	// that keeps 60-odd binaries out of `git status`.
	outDir := opts.OutDir
	if outDir == "" {
		outDir = filepath.Join(repoRoot, DefaultOutDir)
	}
	log := opts.Log
	if log == nil {
		log = func(string) {}
	}
	res := CutResult{}

	// Stage 1 — the artifact set: reuse a verified one, else build + pack.
	if !opts.Repack && Verify(outDir, opts.Version) == nil {
		res.Reused = true
		log(fmt.Sprintf("set: reusing the verified set already in %s (--repack forces a rebuild)", outDir))
	} else {
		binDir := opts.BinariesDir
		if binDir == "" {
			// A fresh dir per run: pack refuses a binaries dir carrying
			// anything but this release's exact matrix, so a reused one
			// would fail on the previous tag's leftovers.
			tmp, err := os.MkdirTemp("", "pyxis-build-*")
			if err != nil {
				return CutResult{}, fmt.Errorf("release: cut: %w", err)
			}
			defer func() {
				// Best-effort scratch cleanup; preserve the release operation result.
				_ = os.RemoveAll(tmp)
			}()
			binDir = tmp
			b, err := Build(ctx, BuildOptions{Version: opts.Version, RepoRoot: repoRoot, OutDir: binDir, Jobs: opts.Jobs})
			if err != nil {
				return CutResult{}, err
			}
			res.Build = &b
			log(fmt.Sprintf("build: %d binaries (%d scan-derived tools × %d platforms)", len(b.Binaries), len(b.Tools), len(Platforms)))
		}
		p, err := Pack(ctx, PackOptions{
			Version: opts.Version, RepoRoot: repoRoot, OutDir: outDir,
			BinariesDir: binDir,
		})
		if err != nil {
			return CutResult{}, err
		}
		res.Pack = &p
		log(fmt.Sprintf("pack: %d artifact(s) in %s, source commit %s", len(p.Artifacts), outDir, p.Manifest.SourceCommit))
		if err := Verify(outDir, opts.Version); err != nil {
			return CutResult{}, err
		}
		log("verify: the set matches " + SumsName + " and the release manifest")
	}

	commit, err := packedSourceCommit(outDir, opts.Version)
	if err != nil {
		return CutResult{}, err
	}
	res.SourceCommit = commit

	// Stage 2 — the tag. publish --verify-tag refuses a tag the remote does
	// not carry, so resolve it here where the refusal can name the fix.
	tagged, err := cutTagCommit(ctx, opts)
	switch {
	case err != nil:
		return CutResult{}, err
	case tagged == "":
		if !opts.PushTag {
			return CutResult{}, fmt.Errorf("release: cut refused — %s carries no tag %s, and publish anchors the release at a real tag. Create it (it must point at %s, the commit these artifacts were packed from):\n  git tag %s %s && git push origin %s\nor re-run with --push-tag to create it at that commit",
				opts.SourceRepo, opts.Version, commit, opts.Version, commit, opts.Version)
		}
		if err := cutCreateTag(ctx, opts, commit); err != nil {
			return CutResult{}, err
		}
		res.Tag = "created"
		log(fmt.Sprintf("tag: created %s on %s at %s", opts.Version, opts.SourceRepo, commit))
	case tagged != commit:
		return CutResult{}, fmt.Errorf("release: cut refused — tag %s on %s points at %s but this artifact set was packed from %s; tag, commit, and artifacts are one provenance hop (installer-v1 §3.1). Repack from the tagged commit, or move the tag deliberately",
			opts.Version, opts.SourceRepo, tagged, commit)
	default:
		res.Tag = "present"
		log(fmt.Sprintf("tag: %s on %s points at the packed commit", opts.Version, opts.SourceRepo))
	}

	// Stage 3 — the canonical publish (upload-or-verify).
	pub, err := Publish(ctx, PublishOptions{Tag: opts.Version, OutDir: outDir, Runner: opts.Runner, Target: opts.SourceTarget})
	if err != nil {
		// The one confusing way to reach a publish refusal: this run
		// REPACKED (the set in OutDir was gone or stale) for a tag already
		// published. Changed source or toolchain inputs can produce different
		// bytes, which upload-or-verify correctly refuses to overwrite.
		if !res.Reused {
			return res, fmt.Errorf("%w\n\nThis run repacked rather than reusing a verified set in %s, and the published tag refused the upload. Changed source or toolchain inputs can produce different bytes. Recover with the ORIGINAL packed set, or skip the rebuild entirely and re-pin from the published release:\n  pyxis-release pins --version %s", err, outDir, opts.Version)
		}
		return res, err
	}
	res.Publish = pub
	outcome := "completed"
	if pub.Created {
		outcome = "created"
	}
	log(fmt.Sprintf("publish: %s on %s, %d artifact(s) uploaded-or-verified", outcome, opts.SourceRepo, len(pub.Actions)))

	// Stage 4 — the trust anchor, derived from the set just published.
	pins, err := Pins(ctx, PinsOptions{Version: opts.Version, RepoRoot: repoRoot, OutDir: outDir, SourceRepo: opts.SourceRepo, Target: opts.SourceTarget})
	if err != nil {
		return res, err // the release IS published; the caller must see that
	}
	res.Pins = pins
	log(fmt.Sprintf("pins: %s %s from %s", installSHName, pins.Action, pins.Source))

	if !opts.Mirror {
		return res, nil
	}

	// Stage 5 — the mirror, gated on the pins actually being committed.
	if err := requireCommittedPins(ctx, repoRoot); err != nil {
		return res, err
	}
	m, err := Mirror(ctx, MirrorOptions{
		Tag: opts.Version, OutDir: outDir, RepoRoot: repoRoot,
		SourceRepo: opts.SourceRepo, DistRepo: opts.DistRepo, Runner: opts.Runner,
		SourceTarget: opts.SourceTarget, DistTarget: opts.DistTarget,
	})
	if err != nil {
		return res, err
	}
	res.Mirror = &m
	log(fmt.Sprintf("mirror repository: %d artifact(s) uploaded-or-verified, %s %s", len(m.Publish.Actions), installSHName, m.InstallSH))
	return res, nil
}

func cutTagCommit(ctx context.Context, opts CutOptions) (string, error) {
	if opts.SourceTarget == nil || opts.SourceTarget.Provider != integrations.Forgejo {
		return remoteTagCommit(ctx, githubRunner(opts.Runner, opts.SourceTarget), opts.SourceRepo, opts.Version)
	}
	c, err := forgerelease.New(*opts.SourceTarget)
	if err != nil {
		return "", err
	}
	commit, err := c.TagCommit(ctx, opts.Version)
	var he *forgejo.HTTPError
	if errors.As(err, &he) && he.Status == 404 {
		return "", nil
	}
	return commit, err
}

func cutCreateTag(ctx context.Context, opts CutOptions, commit string) error {
	if opts.SourceTarget == nil || opts.SourceTarget.Provider != integrations.Forgejo {
		return createRemoteTag(ctx, githubRunner(opts.Runner, opts.SourceTarget), opts.SourceRepo, opts.Version, commit)
	}
	c, err := forgerelease.New(*opts.SourceTarget)
	if err != nil {
		return err
	}
	return c.CreateTag(ctx, opts.Version, commit)
}

// packedSourceCommit reads the commit the set was packed from — the
// provenance the tag must agree with.
func packedSourceCommit(outDir, version string) (string, error) {
	b, err := os.ReadFile(filepath.Join(outDir, manifestName(version)))
	if err != nil {
		return "", fmt.Errorf("release: cut: read the packed release manifest: %w", err)
	}
	m, err := relmanifest.ParseManifest(b)
	if err != nil {
		return "", fmt.Errorf("release: cut: %w", err)
	}
	if m.SourceCommit == "" {
		return "", fmt.Errorf("release: cut: the packed manifest names no source commit")
	}
	return m.SourceCommit, nil
}

// remoteTagCommit resolves a tag to its COMMIT on the remote, "" when the
// tag is absent. An annotated tag's ref points at the tag object, so the
// answer needs a second hop — without it, every annotated tag would look
// like a provenance mismatch.
func remoteTagCommit(ctx context.Context, r wire.Runner, repo, tag string) (string, error) {
	var ref struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
	found, err := ghAPIJSON(ctx, r, "cut", fmt.Sprintf("repos/%s/git/ref/tags/%s", repo, tag), &ref)
	if err != nil || !found {
		return "", err
	}
	if ref.Object.Type != "tag" {
		return ref.Object.SHA, nil
	}
	var obj struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	found, err = ghAPIJSON(ctx, r, "cut", fmt.Sprintf("repos/%s/git/tags/%s", repo, ref.Object.SHA), &obj)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("release: cut: tag %s on %s references annotated object %s, which the remote does not resolve", tag, repo, ref.Object.SHA)
	}
	return obj.Object.SHA, nil
}

// createRemoteTag anchors the tag at the packed commit through the same gh
// seam — no local tag, no push, and no way for the tag to land on a
// different commit than the artifacts were packed from.
func createRemoteTag(ctx context.Context, r wire.Runner, repo, tag, commit string) error {
	return ghRunOp(ctx, r, "cut", []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/git/refs", repo),
		"-f", "ref=refs/tags/" + tag, "-f", "sha=" + commit})
}

// ghAPIJSON runs one gh api GET, decoding stdout into out. found=false is
// the remote's 404 — an absent object, not a failure.
func ghAPIJSON(ctx context.Context, r wire.Runner, op, path string, out any) (bool, error) {
	stdout, stderr, code, err := r.RunGH(ctx, "api", path)
	if err != nil {
		return false, fmt.Errorf("release: %s: %w", op, err)
	}
	if code != 0 {
		if strings.Contains(strings.ToLower(stderr), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("release: %s: gh api %s exited %d: %s", op, path, code, strings.TrimSpace(stderr))
	}
	if err := json.Unmarshal([]byte(stdout), out); err != nil {
		return false, fmt.Errorf("release: %s: parse gh api %s output: %w", op, path, err)
	}
	return true, nil
}

// requireCommittedPins refuses to mirror from an uncommitted install.sh.
// Mirror publishes the checkout's copy to every distribution channel, so an
// uncommitted anchor would serve dist consumers pins that the source repo's
// own front door — the same file, read from main — does not carry.
func requireCommittedPins(ctx context.Context, repoRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "status", "--porcelain", "--", installSHName)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("release: cut: git status for %s in %s: %w (%s)", installSHName, repoRoot, err, strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(stdout.String()) == "" {
		return nil
	}
	return fmt.Errorf("release: cut refused to mirror — %s is modified but not committed in %s. The pins are the front door for BOTH channels: mirroring now would serve dist consumers an anchor the source repo does not carry. Commit it, get the PR merged, pull, then re-run this same command (every stage re-verifies rather than redoing work)", installSHName, repoRoot)
}
