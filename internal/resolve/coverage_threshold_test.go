package resolve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCachedManifestRequiresRoot(t *testing.T) {
	if _, _, err := CachedManifest(Config{}); err == nil {
		t.Fatal("CachedManifest accepted an empty cache root")
	}
}

func TestAuthenticatedFetcherDefaultAndJSONFailureBoundaries(t *testing.T) {
	if _, ok := (githubAuthFetcher{}).plain().(HTTPFetcher); !ok {
		t.Fatal("github auth default transport is not HTTPFetcher")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad-status":
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		case "/bad-json":
			_, _ = w.Write([]byte("{"))
		default:
			_, _ = w.Write([]byte(`{"id":0}`))
		}
	}))
	defer server.Close()
	g := githubAuthFetcher{apiBase: server.URL, client: server.Client()}
	var dst map[string]any
	if err := g.getJSON(t.Context(), "status fixture", server.URL+"/bad-status", &dst); err == nil {
		t.Fatal("getJSON accepted a non-200 response")
	}
	if err := g.getJSON(t.Context(), "JSON fixture", server.URL+"/bad-json", &dst); err == nil {
		t.Fatal("getJSON accepted malformed JSON")
	}
	if err := g.getJSON(t.Context(), "URL fixture", "https://host/%zz", &dst); err == nil {
		t.Fatal("getJSON accepted a malformed URL")
	}
	if _, err := g.releaseID(t.Context(), "owner", "repo", "v1"); err == nil {
		t.Fatal("releaseID accepted a zero release identity")
	}
}

func TestFetchersRejectMalformedURLsBeforeTransport(t *testing.T) {
	ctx := context.Background()
	malformed := "https://host/%zz"
	tests := []struct {
		name  string
		fetch Fetcher
	}{
		{name: "http", fetch: HTTPFetcher{}},
		{name: "file", fetch: FileFetcher{}},
		{name: "scheme dispatcher", fetch: ByScheme{}},
		{name: "github auth", fetch: githubAuthFetcher{downloadHost: "github.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.fetch.Fetch(ctx, malformed); err == nil {
				t.Fatal("Fetch accepted a malformed URL")
			}
		})
	}
}
