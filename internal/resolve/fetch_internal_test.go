package resolve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestParseReleaseDownloadPath(t *testing.T) {
	cases := []struct {
		path                   string
		owner, repo, tag, name string
		ok                     bool
	}{
		{"/agenticlab-ai/pyxis/releases/download/v0.1.0/pyxis-v0.1.0-darwin-arm64", "agenticlab-ai", "pyxis", "v0.1.0", "pyxis-v0.1.0-darwin-arm64", true},
		{"/o/r/releases/download/v1/name", "o", "r", "v1", "name", true},
		{"/o/r/blob/main/file", "", "", "", "", false},                  // not a release path
		{"/o/r/releases/download/v1", "", "", "", "", false},            // missing name
		{"/o/r/releases/download/v1/name/extra", "", "", "", "", false}, // slash tag/name unsupported
		{"", "", "", "", "", false},
	}
	for _, c := range cases {
		o, r, tag, n, ok := parseReleaseDownloadPath(c.path)
		if ok != c.ok || o != c.owner || r != c.repo || tag != c.tag || n != c.name {
			t.Errorf("parse(%q) = (%q,%q,%q,%q,%v), want (%q,%q,%q,%q,%v)",
				c.path, o, r, tag, n, ok, c.owner, c.repo, c.tag, c.name, c.ok)
		}
	}
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "  gh-tok  ")
	t.Setenv("GITHUB_TOKEN", "actions-tok")
	if got := tokenFromEnv(); got != "gh-tok" {
		t.Errorf("GH_TOKEN precedence: got %q, want gh-tok", got)
	}
	t.Setenv("GH_TOKEN", "")
	if got := tokenFromEnv(); got != "actions-tok" {
		t.Errorf("GITHUB_TOKEN fallback: got %q, want actions-tok", got)
	}
	t.Setenv("GITHUB_TOKEN", "")
	if got := tokenFromEnv(); got != "" {
		t.Errorf("no env token: got %q, want empty", got)
	}
}

type fetcherFunc func(context.Context, string) (io.ReadCloser, error)

func (f fetcherFunc) Fetch(ctx context.Context, u string) (io.ReadCloser, error) { return f(ctx, u) }

func writeFetchResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write fixture response: %v", err)
	}
}

func TestGitHubAuthFetcher(t *testing.T) {
	const token = "secret-token"
	want := []byte("the verified pyxis binary bytes")
	var gotListAuth, gotAssetsAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/tags/v0.1.0":
			gotListAuth = r.Header.Get("Authorization")
			writeFetchResponse(t, w, `{"id":99}`)
		case "/repos/o/r/releases/99/assets":
			gotAssetsAuth = r.Header.Get("Authorization")
			writeFetchResponse(t, w, `[{"name":"other","id":7},{"name":"pyxis-v0.1.0-darwin-arm64","id":42}]`)
		case "/repos/o/r/releases/assets/42":
			gotAccept = r.Header.Get("Accept")
			_, _ = w.Write(want)
		default:
			http.Error(w, "nope", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	g := githubAuthFetcher{token: token, downloadHost: host, apiBase: srv.URL}

	// A private release-download URL is served through the API asset endpoint,
	// with the bearer token and octet-stream Accept.
	rc, err := g.Fetch(context.Background(), srv.URL+"/o/r/releases/download/v0.1.0/pyxis-v0.1.0-darwin-arm64")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, want) {
		t.Errorf("bytes = %q, want %q", got, want)
	}
	if gotListAuth != "Bearer "+token || gotAssetsAuth != "Bearer "+token {
		t.Errorf("list Authorization = (%q, %q), want both Bearer %s", gotListAuth, gotAssetsAuth, token)
	}
	if gotAccept != "application/octet-stream" {
		t.Errorf("asset Accept = %q, want application/octet-stream", gotAccept)
	}

	// An asset the release does not carry is a named error.
	if _, err := g.Fetch(context.Background(), srv.URL+"/o/r/releases/download/v0.1.0/nope"); err == nil || !strings.Contains(err.Error(), "no asset") {
		t.Errorf("missing asset err = %v, want a 'no asset' error", err)
	}

	// A non-release / non-host URL falls through to the plain fetcher — the
	// token is never attached there.
	var plainURL string
	g.http = fetcherFunc(func(_ context.Context, u string) (io.ReadCloser, error) {
		plainURL = u
		return io.NopCloser(strings.NewReader("plain")), nil
	})
	if _, err := g.Fetch(context.Background(), "https://example.com/some/mirror/name"); err != nil {
		t.Fatalf("fall-through Fetch: %v", err)
	}
	if plainURL != "https://example.com/some/mirror/name" {
		t.Errorf("plain fall-through url = %q, want the verbatim mirror URL", plainURL)
	}
}

func TestFetcherAttachesTokenOnlyToAPrivateGitHubSource(t *testing.T) {
	restoreTok, restoreRec := tokenFor, recordedBase
	defer func() { tokenFor, recordedBase = restoreTok, restoreRec }()
	tokenFor = func() string { return "a-token" }
	recordedBase = func(string) string { return "" }

	// The built-in default is public, so a token attaches nothing: a gh login
	// on the machine is not a licence to send that token, and an expired one
	// must not break an anonymous read.
	def := Config{}
	bs, ok := def.fetcher().(ByScheme)
	if !ok {
		t.Fatalf("default fetcher type = %T, want ByScheme", def.fetcher())
	}
	if bs.HTTP != nil {
		t.Fatalf("default HTTP leg = %#v, want plain — the public channel takes no token", bs.HTTP)
	}

	// Another github.com artifact_source may be a private fork, which is where
	// the token belongs.
	ghCfg := Config{Repo: config.Config{System: config.System{ArtifactSource: "https://github.com/o/private/releases/download"}}}
	gbs := ghCfg.fetcher().(ByScheme)
	gh, ok := gbs.HTTP.(githubAuthFetcher)
	if !ok || gh.token != "a-token" || gh.downloadHost != "github.com" {
		t.Fatalf("github-source HTTP leg = %#v, want githubAuthFetcher{a-token, github.com}", gbs.HTTP)
	}

	// A third-party mirror: no auth leg.
	mir := Config{Repo: config.Config{System: config.System{ArtifactSource: "https://mirror.example/rel"}}}
	mbs, ok := mir.fetcher().(ByScheme)
	if !ok || mbs.HTTP != nil {
		t.Fatalf("mirror fetcher = %#v, want ByScheme with a nil (unauthenticated) HTTP leg", mir.fetcher())
	}
}

// roundTripFunc adapts a func to http.RoundTripper so a test can route requests
// by host in-process — no DNS — and put the API host and the signed-object host
// on distinct hostnames (httptest servers all share 127.0.0.1, which net/http
// treats as one host, so they cannot exercise the cross-host header strip).
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stringResp(r *http.Request, code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r, ContentLength: int64(len(body))}
}

func redirectResp(r *http.Request, loc string) *http.Response {
	return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {loc}}, Body: io.NopCloser(strings.NewReader("")), Request: r}
}

// TestGitHubAuthFetcherDropsAuthCrossHost exercises the containment the design's
// DINV-2 leans on (FND-3 / A-6): the bearer token reaches the GitHub API host,
// but the asset endpoint's 302 to a DIFFERENT signed-object host must NOT carry
// it. A custom transport puts the two on distinct hostnames so net/http's real
// redirect-header logic — not a mock of it — does the stripping.
func TestGitHubAuthFetcherDropsAuthCrossHost(t *testing.T) {
	const token = "secret-token"
	want := []byte("signed object bytes")
	var apiAssetAuth, cdnAuth, cdnAccept string
	var apiAssetSeen, cdnSeen bool

	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "api.test" && r.URL.Path == "/repos/o/r/releases/tags/v0.1.0":
			return stringResp(r, 200, `{"id":99}`), nil
		case r.URL.Host == "api.test" && r.URL.Path == "/repos/o/r/releases/99/assets":
			return stringResp(r, 200, `[{"name":"pyxis-v0.1.0-darwin-arm64","id":42}]`), nil
		case r.URL.Host == "api.test" && r.URL.Path == "/repos/o/r/releases/assets/42":
			apiAssetSeen = true
			apiAssetAuth = r.Header.Get("Authorization")
			return redirectResp(r, "https://cdn.test/signed/pyxis?sig=abc"), nil
		case r.URL.Host == "cdn.test":
			cdnSeen = true
			cdnAuth = r.Header.Get("Authorization")
			cdnAccept = r.Header.Get("Accept")
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(want)), Request: r, ContentLength: int64(len(want))}, nil
		default:
			return stringResp(r, 404, "no"), nil
		}
	})
	g := githubAuthFetcher{token: token, downloadHost: "github.test", apiBase: "https://api.test", client: &http.Client{Transport: rt}, assets: newAssetMap()}

	rc, err := g.Fetch(context.Background(), "https://github.test/o/r/releases/download/v0.1.0/pyxis-v0.1.0-darwin-arm64")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()

	if !bytes.Equal(got, want) {
		t.Errorf("bytes = %q, want %q", got, want)
	}
	if !apiAssetSeen || apiAssetAuth != "Bearer "+token {
		t.Errorf("asset request to the API host = seen:%v auth:%q, want the bearer token attached", apiAssetSeen, apiAssetAuth)
	}
	if !cdnSeen {
		t.Fatal("the signed-object host was never reached — the 302 was not followed")
	}
	if cdnAuth != "" {
		t.Errorf("the cross-host redirect LEAKED Authorization to the object store: %q (net/http must strip it)", cdnAuth)
	}
	// The non-sensitive Accept header DOES survive the redirect — so the empty
	// Authorization above is a targeted strip, not net/http dropping all headers.
	if cdnAccept != "application/octet-stream" {
		t.Errorf("cross-host Accept = %q, want it carried through (proves only Authorization is stripped)", cdnAccept)
	}
}

// TestGitHubAuthFetcherMemoizesReleaseListing proves FND-6: with the asset-map
// memo, resolving several assets from one release lists it exactly once — a
// cold multi-binary pre-warm no longer re-lists per artifact.
func TestGitHubAuthFetcherMemoizesReleaseListing(t *testing.T) {
	const token = "secret-token"
	var tagHits, assetsHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/tags/v0.1.0":
			atomic.AddInt32(&tagHits, 1)
			writeFetchResponse(t, w, `{"id":99}`)
		case "/repos/o/r/releases/99/assets":
			atomic.AddInt32(&assetsHits, 1)
			writeFetchResponse(t, w, `[{"name":"a","id":1},{"name":"b","id":2}]`)
		case "/repos/o/r/releases/assets/1", "/repos/o/r/releases/assets/2":
			writeFetchResponse(t, w, "bytes")
		default:
			http.Error(w, "nope", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	g := githubAuthFetcher{token: token, downloadHost: host, apiBase: srv.URL, assets: newAssetMap()}

	for _, name := range []string{"a", "b"} {
		rc, err := g.Fetch(context.Background(), srv.URL+"/o/r/releases/download/v0.1.0/"+name)
		if err != nil {
			t.Fatalf("Fetch %s: %v", name, err)
		}
		_, _ = io.ReadAll(rc)
		_ = rc.Close()
	}
	if got := atomic.LoadInt32(&tagHits); got != 1 {
		t.Errorf("release-tag listings = %d, want 1 (the memo must serve the second asset)", got)
	}
	if got := atomic.LoadInt32(&assetsHits); got != 1 {
		t.Errorf("assets-page listings = %d, want 1 (the memo must serve the second asset)", got)
	}
}

// TestGitHubAuthFetcherPaginatesAssets proves FND-5: a release with more assets
// than one page holds still resolves an asset that falls on a later page,
// instead of surfacing it as a bad-pin "no asset" error.
func TestGitHubAuthFetcherPaginatesAssets(t *testing.T) {
	const token = "secret-token"
	want := []byte("late asset bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/tags/v0.1.0":
			writeFetchResponse(t, w, `{"id":99}`)
		case "/repos/o/r/releases/99/assets":
			switch r.URL.Query().Get("page") {
			case "1": // a full page → the fetcher must ask for another
				var b strings.Builder
				b.WriteByte('[')
				for i := range assetsPerPage {
					if i > 0 {
						b.WriteByte(',')
					}
					fmt.Fprintf(&b, `{"name":"filler-%d","id":%d}`, i, 1000+i)
				}
				b.WriteByte(']')
				writeFetchResponse(t, w, b.String())
			case "2": // a short page carrying the target → the walk stops here
				writeFetchResponse(t, w, `[{"name":"pyxis-late","id":42}]`)
			default:
				writeFetchResponse(t, w, `[]`)
			}
		case "/repos/o/r/releases/assets/42":
			_, _ = w.Write(want)
		default:
			http.Error(w, "nope", http.StatusNotFound)
		}
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	g := githubAuthFetcher{token: token, downloadHost: host, apiBase: srv.URL, assets: newAssetMap()}

	rc, err := g.Fetch(context.Background(), srv.URL+"/o/r/releases/download/v0.1.0/pyxis-late")
	if err != nil {
		t.Fatalf("Fetch of a second-page asset failed (pagination gap): %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, want) {
		t.Errorf("bytes = %q, want %q", got, want)
	}
}

// The fetcher-attach rule (cfg.fetcher()): a token travels only to a github.com
// source other than the public default, and no token at all is an anonymous
// fetch rather than a failure. Where the token then actually GOES per request is
// what TestGitHubAuthFetcher exercises; this rule only decides whether the
// authenticated path exists at all.
func TestFetcherAttachRule(t *testing.T) {
	restoreTok, restoreRec := tokenFor, recordedBase
	defer func() {
		tokenFor = restoreTok
		recordedBase = restoreRec
	}()

	cases := []struct {
		name, src, ch, tok string
		wantGithub         bool
	}{
		{"default base + token", "", "", "tok", false},
		{"default base, no token", "", "", "", false},
		{"public source named explicitly + token", DefaultChannelRoot, "", "tok", false},
		{"public versioned source + token", DefaultChannelRoot + "/v0.1.1", "", "tok", false},
		{"public root recorded + token", "", DefaultChannelRoot, "tok", false},
		{"private fork + token", "https://github.com/o/private/releases/download", "", "tok", true},
		{"private fork, no token stays anonymous", "https://github.com/o/private/releases/download", "", "", false},
		{"third-party mirror + token", "https://mirror.example.com/pyxis", "", "tok", false},
		{"recorded mirror + token", "", "https://mirror.example.com/pyxis", "tok", false},
		{"config outranks the channel", "https://github.com/o/private/releases/download", "https://mirror.example.com/pyxis", "tok", true},
		{"file mirror + token stays plain", "file:///srv/pyxis-artifacts", "", "tok", false},
		{"retired config source is the public default", "https://dist.pyxis.agenticlab.ai/v0.1.1", "", "tok", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tokenFor = func() string { return c.tok }
			recordedBase = func(string) string { return c.ch }
			cfg := Config{Repo: config.Config{System: config.System{ArtifactSource: c.src}}}
			bs, ok := cfg.fetcher().(ByScheme)
			if !ok {
				t.Fatalf("fetcher() = %T, want ByScheme", cfg.fetcher())
			}
			if _, isGithub := bs.HTTP.(githubAuthFetcher); isGithub != c.wantGithub {
				t.Errorf("artifact_source %q: github fetcher attached = %v, want %v", c.src, isGithub, c.wantGithub)
			}
			if !c.wantGithub && bs.HTTP != nil {
				t.Errorf("artifact_source %q: attached %T, want plain", c.src, bs.HTTP)
			}
		})
	}
}
