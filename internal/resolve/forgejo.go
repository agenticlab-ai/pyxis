package resolve

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
)

func artifactRepository(cfg Config) (integrations.ResolvedRepository, bool, error) {
	if configuredSource(cfg) != "" {
		return integrations.ResolvedRepository{}, false, nil
	}
	if r, found, err := boundArtifactRepository(cfg); found || err != nil {
		return r, found, err
	}
	return releaseenv.Read().ArtifactRepository()
}

// boundArtifactRepository is the artifact binding the repository's own profile
// declares.
func boundArtifactRepository(cfg Config) (integrations.ResolvedRepository, bool, error) {
	if cfg.Root == "" && cfg.Integrations == nil {
		return integrations.ResolvedRepository{}, false, nil
	}
	var c integrations.Config
	var err error
	if cfg.Integrations != nil {
		c = *cfg.Integrations
	} else {
		c, err = integrations.Load(cfg.Root)
	}
	if err != nil {
		return integrations.ResolvedRepository{}, false, err
	}
	return c.Artifacts()
}

// releaseDownloadBase is a bound repository's release-download base.
func releaseDownloadBase(target integrations.ResolvedRepository, ver string) string {
	base := strings.TrimRight(target.BaseURL, "/") + "/" + target.Repository + "/releases/download"
	if ver != "" {
		base += "/" + url.PathEscape(ver)
	}
	return base
}

type failedFetcher struct{ err error }

func (f failedFetcher) Fetch(context.Context, string) (io.ReadCloser, error) { return nil, f.err }

type forgejoFetcher struct {
	target integrations.ResolvedRepository
}

func configuredGitHubFetcher(target integrations.ResolvedRepository) Fetcher {
	u, err := url.Parse(target.BaseURL)
	if err != nil {
		return failedFetcher{err}
	}
	token := target.Token()
	if target.TokenEnv == "" {
		token = tokenFor()
	}
	if token == "" {
		return ByScheme{}
	}
	api := strings.TrimRight(target.BaseURL, "/") + "/api/v3"
	if u.Host == "github.com" {
		api = "https://api.github.com"
	}
	return ByScheme{HTTP: githubAuthFetcher{token: token, downloadHost: u.Host, apiBase: api, assets: newAssetMap()}}
}

func (f forgejoFetcher) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	prefix := strings.TrimRight(f.target.BaseURL, "/") + "/" + f.target.Repository + "/releases/download/"
	if !strings.HasPrefix(rawURL, prefix) {
		return nil, fmt.Errorf("resolve: artifact URL does not belong to the configured Forgejo source")
	}
	parts := strings.Split(strings.TrimPrefix(rawURL, prefix), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("resolve: invalid Forgejo release asset URL")
	}
	tag, err := url.PathUnescape(parts[0])
	if err != nil {
		return nil, err
	}
	name, err := url.PathUnescape(parts[1])
	if err != nil {
		return nil, err
	}
	c, err := forgerelease.New(f.target)
	if err != nil {
		return nil, err
	}
	return c.Open(ctx, tag, name)
}
