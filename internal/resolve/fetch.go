package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Fetcher is the artifact transport seam: it turns an artifact URL into a
// byte stream. Verification NEVER lives behind it — the resolver hashes and
// compares before trusting a single byte (DINV-1), so a fetcher (like the
// artifact-source override it serves) chooses where bytes come from, never
// whether they are accepted (installer-v1 §3.9 leg 1). Tests inject counting
// and scripted fetchers through Config.Fetcher.
type Fetcher interface {
	// Fetch opens the artifact at rawURL for reading. It returns an error —
	// never a partial reader — when the artifact cannot be served (offline,
	// absent path, non-200 status). The caller owns closing the reader and
	// honors ctx end to end.
	Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error)
}

// HTTPFetcher serves http and https URLs — the release-host and mirror leg.
// The zero value is ready; ctx governs cancellation (no client timeout is
// imposed here).
type HTTPFetcher struct {
	Client *http.Client // nil uses a private default client
}

func (f HTTPFetcher) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve: build request for %s: %w", rawURL, err)
	}
	client := f.Client
	if client == nil {
		client = defaultHTTPClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve: GET %s: %w", rawURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() // the status IS the failure; the body adds nothing to the contract
		return nil, fmt.Errorf("resolve: GET %s: %s", rawURL, resp.Status)
	}
	return resp.Body, nil
}

// defaultHTTPClient is HTTPFetcher's nil-Client default — package-private so
// no global (http.DefaultClient) state is touched.
var defaultHTTPClient = &http.Client{}

// FileFetcher serves file:// URLs — the fixture leg the hermetic tests and
// air-gapped mirrors use (the T3 charter requires file sources to work under
// the exact same verification as http ones).
type FileFetcher struct{}

func (FileFetcher) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("resolve: parse artifact URL %s: %w", rawURL, err)
	}
	if u.Host != "" && u.Host != "localhost" {
		return nil, fmt.Errorf("resolve: file URL %s: remote hosts are not supported", rawURL)
	}
	f, err := os.Open(u.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve: open %s: %w", u.Path, err)
	}
	return f, nil
}

// ByScheme dispatches to the fetcher owning the URL's scheme — http(s) or
// file, the two schemes the artifact source admits. The zero value is the
// package default transport.
type ByScheme struct {
	HTTP Fetcher // nil uses HTTPFetcher{}
	File Fetcher // nil uses FileFetcher{}
}

func (b ByScheme) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("resolve: parse artifact URL %s: %w", rawURL, err)
	}
	switch u.Scheme {
	case "http", "https":
		f := b.HTTP
		if f == nil {
			f = HTTPFetcher{}
		}
		return f.Fetch(ctx, rawURL)
	case "file":
		f := b.File
		if f == nil {
			f = FileFetcher{}
		}
		return f.Fetch(ctx, rawURL)
	default:
		return nil, fmt.Errorf("resolve: artifact URL %s: unsupported scheme %q (http, https, or file)", rawURL, u.Scheme)
	}
}

// githubAuthFetcher fetches release assets from a PRIVATE GitHub repo — a fork
// or mirror that sets a token. The public default channel never uses it: its
// assets are served anonymously at the plain download URL. A private repo's
// plain download URL cannot serve them: a token on
// github.com/<o>/<r>/releases/download/<tag>/<name> still 404s, so private
// assets must go through the API asset endpoint. For a release-download URL on
// downloadHost it (1) lists the release to map the asset name to its id, then
// (2) GETs repos/<o>/<r>/releases/assets/<id> with Accept:
// application/octet-stream and the bearer token — GitHub 302-redirects to a
// signed object URL, which the client follows. Go drops the Authorization
// header across that cross-host hop (net/http strips sensitive headers when the
// redirect target is a different host, and the signed URL needs none) — a
// containment TestGitHubAuthFetcherDropsAuthCrossHost exercises directly, not
// merely asserts. Any other URL (a public asset, a mirror, a non-release path)
// falls through to plain HTTP unauthenticated. The token is attached ONLY to
// downloadHost/apiBase, so a PYXIS_ARTIFACT_SOURCE mirror never receives it.
// Verification is unchanged: the resolver still hashes every returned byte
// against the manifest pin — auth governs access, never acceptance (§3.9 leg 1).
//
// The listing is paginated (a release with more assets than the tag endpoint
// inlines still resolves every asset) and memoized through assets, so a
// multi-artifact resolve — PreWarm fetches the manifest plus every binary —
// lists each release at most once, not once per cache-miss artifact.
type githubAuthFetcher struct {
	token        string
	downloadHost string       // release-download URLs on this host get the API treatment ("github.com")
	apiBase      string       // GitHub REST base ("https://api.github.com")
	http         Fetcher      // fall-through for non-release / non-host URLs (nil → HTTPFetcher{})
	client       *http.Client // nil → defaultHTTPClient (follows the asset 302)
	assets       *assetMap    // per-release name→id memo; nil disables it (list every call)
}

func (g githubAuthFetcher) plain() Fetcher {
	if g.http != nil {
		return g.http
	}
	return HTTPFetcher{}
}

func (g githubAuthFetcher) do() *http.Client {
	if g.client != nil {
		return g.client
	}
	return defaultHTTPClient
}

func (g githubAuthFetcher) Fetch(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("resolve: parse artifact URL %s: %w", rawURL, err)
	}
	owner, repo, tag, name, ok := parseReleaseDownloadPath(u.Path)
	if !ok || u.Host != g.downloadHost {
		return g.plain().Fetch(ctx, rawURL) // public asset, mirror, or non-release URL
	}
	id, err := g.assetID(ctx, owner, repo, tag, name)
	if err != nil {
		return nil, err
	}
	assetURL := fmt.Sprintf("%s/repos/%s/%s/releases/assets/%d", g.apiBase, owner, repo, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve: build asset request for %s: %w", name, err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := g.do().Do(req)
	if err != nil {
		return nil, fmt.Errorf("resolve: GET release asset %s: %w", name, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("resolve: GET release asset %s: %s", name, resp.Status)
	}
	return resp.Body, nil
}

// assetID resolves an asset's numeric id from its name via the release's
// (memoized) asset map. A missing release or asset is a named error — the same
// "cannot be served" contract as a 404 download.
func (g githubAuthFetcher) assetID(ctx context.Context, owner, repo, tag, name string) (int64, error) {
	assets, err := g.releaseAssets(ctx, owner, repo, tag)
	if err != nil {
		return 0, err
	}
	id, ok := assets[name]
	if !ok {
		return 0, fmt.Errorf("resolve: release %s carries no asset %q", tag, name)
	}
	return id, nil
}

// releaseAssets returns the release's complete asset name→id map, memoized
// (g.assets) so later lookups in the same resolve reuse the one listing rather
// than re-listing per cache-miss artifact. A cache hit makes no API call; a
// miss lists the release once (paginated) and records it.
func (g githubAuthFetcher) releaseAssets(ctx context.Context, owner, repo, tag string) (map[string]int64, error) {
	key := owner + "/" + repo + "/" + tag
	if g.assets != nil {
		if m, ok := g.assets.get(key); ok {
			return m, nil
		}
	}
	m, err := g.listReleaseAssets(ctx, owner, repo, tag)
	if err != nil {
		return nil, err
	}
	if g.assets != nil {
		g.assets.put(key, m)
	}
	return m, nil
}

// assetsPerPage is the page size for the paginated release-assets endpoint
// (GitHub's max); maxAssetPages bounds the walk so a misbehaving server that
// never returns a short page cannot hang the resolver (10k assets is already
// far past any real release).
const (
	assetsPerPage = 100
	maxAssetPages = 100
)

// listReleaseAssets returns the release's complete asset name→id map. It reads
// the tag endpoint for the release id, then pages
// repos/<o>/<r>/releases/<id>/assets?per_page=100 until a short page. The tag
// endpoint inlines only its first ~100 assets, so a larger release would
// otherwise surface a present asset as a bad-pin "no asset" error; paging the
// dedicated endpoint resolves every asset regardless of count.
func (g githubAuthFetcher) listReleaseAssets(ctx context.Context, owner, repo, tag string) (map[string]int64, error) {
	id, err := g.releaseID(ctx, owner, repo, tag)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for page := 1; page <= maxAssetPages; page++ {
		assetsURL := fmt.Sprintf("%s/repos/%s/%s/releases/%d/assets?per_page=%d&page=%d", g.apiBase, owner, repo, id, assetsPerPage, page)
		var assets []releaseAsset
		if err := g.getJSON(ctx, fmt.Sprintf("list release %s assets", tag), assetsURL, &assets); err != nil {
			return nil, err
		}
		for _, a := range assets {
			out[a.Name] = a.ID
		}
		if len(assets) < assetsPerPage {
			return out, nil
		}
	}
	return nil, fmt.Errorf("resolve: release %s has more than %d assets — refusing to page further (fail closed)", tag, assetsPerPage*maxAssetPages)
}

// releaseAsset is one entry of the release-assets endpoint's JSON array.
type releaseAsset struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
}

// releaseID resolves a release's numeric id from its tag — the handle the
// paginated assets endpoint needs. A missing release is a named error.
func (g githubAuthFetcher) releaseID(ctx context.Context, owner, repo, tag string) (int64, error) {
	var rel struct {
		ID int64 `json:"id"`
	}
	relURL := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", g.apiBase, owner, repo, tag)
	if err := g.getJSON(ctx, fmt.Sprintf("list release %s", tag), relURL, &rel); err != nil {
		return 0, err
	}
	if rel.ID == 0 {
		return 0, fmt.Errorf("resolve: release %s carries no id", tag)
	}
	return rel.ID, nil
}

// getJSON performs an authenticated GitHub REST GET and decodes the JSON body
// into v. what names the operation for errors; a non-200 is a named failure —
// the same "cannot be served" contract as a download 404. The body is
// size-capped so a hostile endpoint cannot exhaust memory.
func (g githubAuthFetcher) getJSON(ctx context.Context, what, apiURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("resolve: build %s request: %w", what, err)
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := g.do().Do(req)
	if err != nil {
		return fmt.Errorf("resolve: %s: %w", what, err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only response; nothing to recover from a close error
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resolve: %s: %s", what, resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v); err != nil {
		return fmt.Errorf("resolve: decode %s: %w", what, err)
	}
	return nil
}

// assetMap memoizes releases' asset name→id maps so a multi-artifact resolve
// (PreWarm fetches the manifest plus every current-platform binary) lists each
// release at most once, not once per cache-miss artifact — cutting call volume
// and rate-limit exposure. Keyed "owner/repo/tag"; the mutex guards the shared
// map (fetchVerifyTo's atomic-rename race admits concurrent resolvers). A nil
// *assetMap disables the memo: assetID then lists on every call.
type assetMap struct {
	mu    sync.Mutex
	byTag map[string]map[string]int64
}

func newAssetMap() *assetMap { return &assetMap{byTag: map[string]map[string]int64{}} }

func (a *assetMap) get(key string) (map[string]int64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m, ok := a.byTag[key]
	return m, ok
}

func (a *assetMap) put(key string, m map[string]int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.byTag[key] = m
}

// parseReleaseDownloadPath matches a GitHub release-download URL path,
// /<owner>/<repo>/releases/download/<tag>/<name>, and pulls out its parts.
// Single-segment tag and name (the system release's vX.Y.Z tags qualify).
func parseReleaseDownloadPath(p string) (owner, repo, tag, name string, ok bool) {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	if len(parts) != 6 || parts[2] != "releases" || parts[3] != "download" {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[4], parts[5], true
}

// tokenFromEnv returns the first non-empty GH_TOKEN/GITHUB_TOKEN — the explicit,
// CI-friendly token source. Pure.
func tokenFromEnv() string {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if t := strings.TrimSpace(os.Getenv(k)); t != "" {
			return t
		}
	}
	return ""
}

// tokenFor is the token source cfg.fetcher() consults — a package seam so
// tests can drive the auth-attachment branch without the process-wide memo.
var tokenFor = resolvedToken

var (
	tokenOnce   sync.Once
	tokenCached string
)

// resolvedToken is the process-wide GitHub token for a private GitHub source:
// GH_TOKEN/GITHUB_TOKEN, else `gh auth token` when gh happens to be logged in.
// Resolved once (the `gh` shell-out runs at most once per process); "" when none
// is available, in which case the resolver fetches over the anonymous download
// URL — gh is never required, and a missing binary reads as no token.
func resolvedToken() string {
	tokenOnce.Do(func() {
		if t := tokenFromEnv(); t != "" {
			tokenCached = t
			return
		}
		out, err := exec.Command("gh", "auth", "token").Output()
		if err == nil {
			tokenCached = strings.TrimSpace(string(out))
		}
	})
	return tokenCached
}
