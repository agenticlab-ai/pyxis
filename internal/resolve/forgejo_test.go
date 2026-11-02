package resolve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestForgejoArtifactBindingAndFilePrecedence(t *testing.T) {
	t.Setenv("RESOLVE_FORGEJO_TOKEN", "resolve-secret")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token resolve-secret" {
			t.Error("missing selected artifact credential")
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/tags/v1.2.3"):
			if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
				t.Error(err)
			}
		case strings.HasSuffix(r.URL.Path, "/releases/1/assets"):
			assets := []forgerelease.Asset{}
			if r.URL.Query().Get("page") == "" {
				assets = append(assets, forgerelease.Asset{Name: "manifest", URL: server.URL + "/attachments/manifest"})
			}
			if err := json.NewEncoder(w).Encode(assets); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/attachments/manifest":
			if _, err := fmt.Fprint(w, "artifact bytes"); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected URL %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	profile := fmt.Sprintf(`schema_version: pyxis-config/v2
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    artifacts:
      provider: forgejo
      base_url: %s
      token_env: RESOLVE_FORGEJO_TOKEN
  bindings:
    artifacts:
      connection: artifacts
      repository: o/artifacts
`, server.URL)
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Root: root, Repo: config.Defaults()}
	url, _ := EffectiveSource(cfg, "v1.2.3")
	if url != server.URL+"/o/artifacts/releases/download/v1.2.3" {
		t.Fatal(url)
	}
	body, err := cfg.fetcher().Fetch(t.Context(), url+"/manifest")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err != nil || string(data) != "artifact bytes" {
		t.Fatalf("fetch %q %v", data, err)
	}
	if _, err := cfg.fetcher().Fetch(t.Context(), server.URL+"/o/other/releases/download/v1.2.3/manifest"); err == nil {
		t.Fatal("accepted unbound repository")
	}
	cfg.Repo.System.ArtifactSource = "file:///mirror"
	if got, _ := EffectiveSource(cfg, "v1.2.3"); got != "file:///mirror" {
		t.Fatal("flat file override lost precedence")
	}
	if _, found, err := artifactRepository(cfg); found || err != nil {
		t.Fatal("binding bypassed flat artifact override")
	}
	profileOverride, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte("integrations:\n  unknown: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Repo.System.ArtifactSource = ""
	if _, err := cfg.fetcher().Fetch(t.Context(), server.URL); err == nil {
		t.Fatal("invalid integration profile fell through")
	}
	cfg.Integrations = &profileOverride
	if got, _ := EffectiveSource(cfg, "v1.2.3"); got != url {
		t.Fatal("cold-init override ignored")
	}
	body, err = cfg.fetcher().Fetch(t.Context(), url+"/manifest")
	if err != nil {
		t.Fatal(err)
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConfiguredGitHubArtifactCredentialIsIndependent(t *testing.T) {
	t.Setenv("SCOPED_ARTIFACT_TOKEN", "selected")
	t.Setenv("GH_TOKEN", "ambient")
	selected := configuredGitHubFetcher(integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/artifacts", TokenEnv: "SCOPED_ARTIFACT_TOKEN"}).(ByScheme).HTTP.(githubAuthFetcher)
	if selected.token != "selected" || selected.apiBase != "https://api.github.com" {
		t.Fatal("wrong artifact credential")
	}
	t.Setenv("SCOPED_ARTIFACT_TOKEN", "")
	if got := configuredGitHubFetcher(integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/artifacts", TokenEnv: "SCOPED_ARTIFACT_TOKEN"}).(ByScheme); got.HTTP != nil {
		t.Fatal("empty selected credential fell through to ambient token")
	}
	legacy := configuredGitHubFetcher(integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/artifacts"}).(ByScheme).HTTP.(githubAuthFetcher)
	if legacy.token != "ambient" {
		t.Fatal("legacy GitHub token compatibility changed")
	}
	if _, err := configuredGitHubFetcher(integrations.ResolvedRepository{BaseURL: "https://%"}).Fetch(t.Context(), "unused"); err == nil {
		t.Fatal("invalid GitHub URL accepted")
	}
}

func TestForgejoArtifactRejectsMalformedReleaseCoordinates(t *testing.T) {
	f := forgejoFetcher{target: integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "http://127.0.0.1:3000", Repository: "o/r"}}
	base := f.target.BaseURL + "/o/r/releases/download/"
	for _, suffix := range []string{"tag/path/extra", "%zz/name", "tag/%zz"} {
		if _, err := f.Fetch(t.Context(), base+suffix); err == nil {
			t.Fatalf("malformed release coordinates accepted: %s", suffix)
		}
	}
	f.target.Repository = "invalid"
	if _, err := f.Fetch(t.Context(), f.target.BaseURL+"/invalid/releases/download/tag/name"); err == nil {
		t.Fatal("invalid repository accepted")
	}
}
