package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func publishForgejo(ctx context.Context, opts PublishOptions) (PublishResult, error) {
	if err := verifyPublish(opts); err != nil {
		return PublishResult{}, fmt.Errorf("release: publish refused: %w", err)
	}
	c, err := forgerelease.New(*opts.Target)
	if err != nil {
		return PublishResult{}, err
	}
	r, found, err := c.Lookup(ctx, opts.Tag)
	if err != nil {
		return PublishResult{}, err
	}
	names, err := listArtifacts(opts.OutDir)
	if err != nil {
		return PublishResult{}, err
	}
	local := map[string]bool{}
	for _, name := range names {
		local[name] = true
	}
	assets := map[string]forgerelease.Asset{}
	for _, a := range r.Assets {
		if !local[a.Name] {
			return PublishResult{}, fmt.Errorf("release: publish refused: %s carries asset %q outside the local set", opts.Tag, a.Name)
		}
		assets[a.Name] = a
	}
	res := PublishResult{Created: !found, Actions: map[string]string{}}
	if !found {
		r, err = c.Create(ctx, opts.Tag, !opts.Mirror)
		if err != nil {
			return PublishResult{}, err
		}
	}
	for _, name := range names {
		if a, ok := assets[name]; ok {
			body, err := c.Download(ctx, a)
			if err != nil {
				return res, err
			}
			h := sha256.New()
			_, copyErr := io.Copy(h, body)
			closeErr := body.Close()
			if copyErr != nil {
				return res, copyErr
			}
			if closeErr != nil {
				return res, closeErr
			}
			want, err := hashFile(filepath.Join(opts.OutDir, name))
			if err != nil {
				return res, err
			}
			if got := hex.EncodeToString(h.Sum(nil)); got != want {
				return res, fmt.Errorf("release: publish refused: asset %s already exists with sha256 %s, local pack has %s; never clobbered", name, got, want)
			}
			res.Actions[name] = "verified"
		} else {
			if err := c.Upload(ctx, r.ID, filepath.Join(opts.OutDir, name)); err != nil {
				return res, err
			}
			res.Actions[name] = "uploaded"
		}
	}
	return res, nil
}

func downloadForgejoSet(ctx context.Context, target integrations.ResolvedRepository, tag, dir string) error {
	c, err := forgerelease.New(target)
	if err != nil {
		return err
	}
	r, found, err := c.Lookup(ctx, tag)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("release: %s has no release %s", target.Key(), tag)
	}
	for _, a := range r.Assets {
		if err := c.DownloadTo(ctx, a, dir); err != nil {
			return err
		}
	}
	return nil
}

func pinsFromForgejo(ctx context.Context, target integrations.ResolvedRepository, version string) (PinSet, error) {
	dir, err := os.MkdirTemp("", "pyxis-forgejo-pins-*")
	if err != nil {
		return PinSet{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }() // Best-effort cleanup of the verified private staging directory.
	if err := downloadForgejoSet(ctx, target, version, dir); err != nil {
		return PinSet{}, err
	}
	return pinsFromPacked(dir, version)
}

func sameMirrorTarget(opts MirrorOptions) bool {
	if opts.SourceTarget != nil && opts.DistTarget != nil {
		return opts.SourceTarget.Key() == opts.DistTarget.Key()
	}
	if opts.SourceTarget != nil && opts.SourceTarget.Provider == integrations.Forgejo {
		return false
	}
	if opts.DistTarget != nil && opts.DistTarget.Provider == integrations.Forgejo {
		return false
	}
	return opts.SourceRepo == opts.DistRepo
}

func downloadMirrorSource(ctx context.Context, opts MirrorOptions, dir string) error {
	if opts.SourceTarget != nil && opts.SourceTarget.Provider == integrations.Forgejo {
		return downloadForgejoSet(ctx, *opts.SourceTarget, opts.Tag, dir)
	}
	return ghRun(ctx, githubRunner(opts.Runner, opts.SourceTarget), []string{"release", "download", opts.Tag, "--repo", opts.SourceRepo, "--dir", dir, "--clobber"})
}

func githubRunner(r wire.Runner, target *integrations.ResolvedRepository) wire.Runner {
	if target == nil || target.Provider != integrations.GitHub {
		return r
	}
	if prod, ok := r.(wire.GHRunner); ok {
		prod.TokenEnv = target.TokenEnv
		prod.Host = "github.com"
		return prod
	}
	return r
}

func installerForTarget(src string, opts MirrorOptions) (string, error) {
	out := src
	if opts.SourceRepo != "" {
		out = transformInstallSH(src, opts.SourceRepo, opts.DistRepo)
	}
	if opts.DistTarget == nil {
		return out, nil
	}
	t := opts.DistTarget
	for _, v := range []string{t.Provider, t.BaseURL, t.Repository, t.TokenEnv} {
		if strings.ContainsAny(v, "\r\n\x00\"`$\\") {
			return "", fmt.Errorf("release: unsafe distribution installer setting")
		}
	}
	repoLine := regexp.MustCompile(`(?m)^REPO="[A-Za-z0-9_./-]+"$`)
	if len(repoLine.FindAllStringIndex(out, -1)) != 1 {
		return "", fmt.Errorf("release: installer must carry exactly one repository default")
	}
	out = repoLine.ReplaceAllStringFunc(out, func(string) string { return `REPO="` + t.Repository + `"` })
	defaults := []struct{ key, value string }{{"RELEASE_PROVIDER", t.Provider}, {"RELEASE_BASE_URL", t.BaseURL}, {"RELEASE_TOKEN_ENV", t.TokenEnv}}
	for _, d := range defaults {
		re := regexp.MustCompile(`(?m)^` + d.key + `="\$\{PYXIS_` + d.key + `:-[^\n]*\}"$`)
		if !re.MatchString(out) {
			return "", fmt.Errorf("release: installer lacks %s provider default; refresh install.sh before mirroring", d.key)
		}
		out = re.ReplaceAllStringFunc(out, func(string) string { return d.key + `="${PYXIS_` + d.key + `:-` + d.value + `}"` })
	}
	return out, nil
}

func refreshForgejoInstall(ctx context.Context, target integrations.ResolvedRepository, want, tag string) (string, error) {
	c, err := forgerelease.New(target)
	if err != nil {
		return "", err
	}
	return c.WriteInstall(ctx, []byte(want), tag)
}
