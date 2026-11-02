package release

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// errNoDistributionBinding is the refusal for a mirror with nowhere to publish.
// Public GitHub Releases needs no mirror: a plain cut is the complete release.
var errNoDistributionBinding = errors.New("release: mirror needs an explicit releases.distribution binding — public GitHub Releases is the default channel, and plain `pyxis-release cut` is the complete release; nothing was published")

// MirrorOptions parameterizes distribution to an explicitly bound repository.
// There is no implicit distribution target.
type MirrorOptions struct {
	SourceTarget *integrations.ResolvedRepository
	DistTarget   *integrations.ResolvedRepository
	Tag          string
	OutDir       string // an already-fetched artifact set; "" downloads into a temp dir
	RepoRoot     string // the source checkout carrying install.sh ("" = ".")
	SourceRepo   string // the canonical release host the set is fetched from
	DistRepo     string // the mirror target the set is published to
	Runner       wire.Runner
}

// MirrorResult reports the run: whether the set was downloaded here, the
// publish outcome, and what happened to the dist install.sh ("written" |
// "verified").
type MirrorResult struct {
	Downloaded bool
	Publish    PublishResult
	InstallSH  string
}

// pinnedVersionRe pulls the PINNED_VERSION line out of install.sh's BEGIN
// PINS block — the anchor the tag precondition checks against.
var pinnedVersionRe = regexp.MustCompile(`(?m)^PINNED_VERSION="([^"]*)"`)

// Mirror fetches the canonical artifact set (unless OutDir already carries
// it), verifies it, and publishes it through the upload-or-verify grammar to the
// bound repository, with an install.sh retargeted to that provider.
//
// The stranding hazard is a precondition, not a footnote: the checkout's
// install.sh must already pin Tag (its PINNED_VERSION line), else Mirror
// refuses before any remote operation — mirroring artifacts while
// republishing stale pins would strand every dist consumer on the previous
// release.
//
// Re-runnable end to end (AP-22): Publish completes-or-verifies, and the
// install.sh write is skipped as "verified" when the dist copy already
// matches byte-for-byte.
func Mirror(ctx context.Context, opts MirrorOptions) (MirrorResult, error) {
	if err := checkVersion(opts.Tag); err != nil {
		return MirrorResult{}, err
	}
	if opts.DistTarget == nil {
		return MirrorResult{}, errNoDistributionBinding
	}
	if opts.SourceTarget != nil {
		opts.SourceRepo = opts.SourceTarget.Repository
	}
	if opts.OutDir == "" && (opts.SourceRepo == "" || (opts.Runner == nil && (opts.SourceTarget == nil || opts.SourceTarget.Provider != integrations.Forgejo))) {
		return MirrorResult{}, fmt.Errorf("release: mirror: downloading the canonical set requires a source repo and gh runner or Forgejo binding; supply --out to publish a local set")
	}
	opts.DistRepo = opts.DistTarget.Repository
	if sameMirrorTarget(opts) {
		return MirrorResult{}, fmt.Errorf("release: mirror refused — source and distribution target are both %s; a mirror publishes elsewhere", opts.SourceRepo)
	}
	if opts.Runner == nil && opts.DistTarget.Provider != integrations.Forgejo {
		return MirrorResult{}, fmt.Errorf("release: mirror: repository distribution requires a gh runner or Forgejo binding")
	}
	root := opts.RepoRoot
	if root == "" {
		root = "."
	}
	installSH, err := os.ReadFile(filepath.Join(root, "install.sh"))
	if err != nil {
		return MirrorResult{}, fmt.Errorf("release: mirror: read the checkout's install.sh (its pins gate the run): %w", err)
	}
	m := pinnedVersionRe.FindSubmatch(installSH)
	if m == nil {
		return MirrorResult{}, fmt.Errorf("release: mirror: install.sh carries no PINNED_VERSION line — not a pins-bearing copy")
	}
	if pinned := string(m[1]); pinned != opts.Tag {
		return MirrorResult{}, fmt.Errorf("release: mirror refused — install.sh pins %s but the mirror targets %s; regenerate the pins first, or every dist consumer is stranded on %s (the runbook's step-3 hazard, made mechanical)", pinned, opts.Tag, pinned)
	}
	mirrorInstaller, err := installerForTarget(string(installSH), opts)
	if err != nil {
		return MirrorResult{}, err
	}

	res := MirrorResult{}
	outDir := opts.OutDir
	if outDir == "" {
		tmp, err := os.MkdirTemp("", "pyxis-mirror-*")
		if err != nil {
			return MirrorResult{}, fmt.Errorf("release: mirror: %w", err)
		}
		defer func() {
			// Best-effort scratch cleanup; preserve the release operation result.
			_ = os.RemoveAll(tmp)
		}()
		if err := downloadMirrorSource(ctx, opts, tmp); err != nil {
			return MirrorResult{}, fmt.Errorf("release: mirror: fetch the canonical set from %s: %w", opts.SourceRepo, err)
		}
		res.Downloaded = true
		outDir = tmp
	}

	if err := Verify(outDir, opts.Tag); err != nil {
		return MirrorResult{}, fmt.Errorf("release: mirror refused — the artifact set failed verification before distribution: %w", err)
	}
	res.Publish, err = Publish(ctx, PublishOptions{Tag: opts.Tag, OutDir: outDir, Runner: opts.Runner, Target: opts.DistTarget, Mirror: true})
	if err != nil {
		return MirrorResult{}, err
	}
	res.InstallSH, err = refreshDistInstallSH(ctx, opts, mirrorInstaller)
	if err != nil {
		return MirrorResult{}, err
	}
	return res, nil
}

// transformInstallSH retargets every sourceRepo reference to distRepo. A
// reference already naming distRepo is guarded first, so a source copy that
// mentions the mirror (distRepo contains sourceRepo as a prefix) never
// degrades to "<dist>-dist" — the latent corruption a bare global
// replacement carries.
func transformInstallSH(src, sourceRepo, distRepo string) string {
	const guard = "\x00pyxis-mirror-guard\x00"
	out := strings.ReplaceAll(src, distRepo, guard)
	out = strings.ReplaceAll(out, sourceRepo, distRepo)
	return strings.ReplaceAll(out, guard, distRepo)
}

// refreshDistInstallSH writes the transformed install.sh into the dist
// repo via the gh contents API — created when absent, updated when its
// bytes diverge, skipped as "verified" when already current.
func refreshDistInstallSH(ctx context.Context, opts MirrorOptions, want string) (string, error) {
	if opts.DistTarget != nil && opts.DistTarget.Provider == integrations.Forgejo {
		return refreshForgejoInstall(ctx, *opts.DistTarget, want, opts.Tag)
	}
	opts.Runner = githubRunner(opts.Runner, opts.DistTarget)
	path := fmt.Sprintf("repos/%s/contents/install.sh", opts.DistRepo)
	stdout, stderr, code, err := opts.Runner.RunGH(ctx, "api", path)
	if err != nil {
		return "", fmt.Errorf("release: mirror: %w", err)
	}
	sha := ""
	switch {
	case code == 0:
		var cur struct {
			SHA      string `json:"sha"`
			Content  string `json:"content"`
			Encoding string `json:"encoding"`
		}
		if err := json.Unmarshal([]byte(stdout), &cur); err != nil {
			return "", fmt.Errorf("release: mirror: parse the dist install.sh listing: %w", err)
		}
		if cur.Encoding != "base64" {
			return "", fmt.Errorf("release: mirror: dist install.sh listing has encoding %q, want base64", cur.Encoding)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(cur.Content), ""))
		if err != nil {
			return "", fmt.Errorf("release: mirror: decode the dist install.sh content: %w", err)
		}
		if string(raw) == want {
			return "verified", nil
		}
		sha = cur.SHA
	case strings.Contains(strings.ToLower(stderr), "not found"):
		// Absent → the create leg (PUT without sha).
	default:
		return "", fmt.Errorf("release: mirror: gh api %s exited %d: %s", path, code, strings.TrimSpace(stderr))
	}

	args := []string{"api", "-X", "PUT", path,
		"-f", fmt.Sprintf("message=install.sh: refresh to the %s pins (generated from the source copy — never hand-edit)", opts.Tag),
		"-f", "content=" + base64.StdEncoding.EncodeToString([]byte(want)),
	}
	if sha != "" {
		args = append(args, "-f", "sha="+sha)
	}
	if err := ghRun(ctx, opts.Runner, args); err != nil {
		return "", err
	}
	return "written", nil
}
