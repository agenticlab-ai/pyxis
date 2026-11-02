package upgrade

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/releaseenv"
)

func TestForgejoUpgradePinsAndSourceSurviveRestart(t *testing.T) {
	blob := []byte("fixture verified pyxis binary")
	manifest := strings.Repeat("b", 64)
	pins := installSH("v9.9.9", manifest, map[string]string{"darwin-arm64": sum(blob)})
	t.Setenv("UPGRADE_FORGEJO_TEST_TOKEN", "upgrade-secret")
	var s *httptest.Server
	s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token upgrade-secret" {
			t.Error("wrong source credential")
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/contents/install.sh"):
			if _, err := fmt.Fprintf(w, `{"encoding":"base64","content":%q}`, base64.StdEncoding.EncodeToString([]byte(pins))); err != nil {
				t.Error(err)
			}
		case strings.HasSuffix(r.URL.Path, "/releases/tags/v9.9.9"):
			if _, err := fmt.Fprint(w, `{"id":2}`); err != nil {
				t.Error(err)
			}
		case strings.HasSuffix(r.URL.Path, "/releases/2/assets"):
			assets := []forgerelease.Asset{}
			if r.URL.Query().Get("page") == "" {
				assets = append(assets, forgerelease.Asset{Name: "pyxis-v9.9.9-darwin-arm64", URL: s.URL + "/attachments/binary"})
			}
			if err := json.NewEncoder(w).Encode(assets); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/attachments/binary":
			_, _ = w.Write(blob)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer s.Close()
	selected := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: s.URL, Repository: "o/artifacts", TokenEnv: "UPGRADE_FORGEJO_TEST_TOKEN"}
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	target := installedBinary(t, "old")
	rep, err := Run(t.Context(), Options{Current: "v9.9.8", Platform: "darwin-arm64", TargetPath: target, ConfigDir: filepath.Join(configHome, "pyxis"), Release: &selected})
	if err != nil || !rep.BinaryReplaced {
		t.Fatalf("upgrade: %+v %v", rep, err)
	}
	actual, err := os.ReadFile(target)
	if err != nil || string(actual) != string(blob) {
		t.Fatalf("binary: %q %v", actual, err)
	}
	recorded, found, err := releaseenv.Read().ArtifactRepository()
	if err != nil || !found || recorded.Key() != selected.Key() || recorded.TokenEnv != selected.TokenEnv {
		t.Fatalf("record: %+v %v %v", recorded, found, err)
	}
	record, err := os.ReadFile(releaseenv.Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "upgrade-secret") {
		t.Fatal("secret persisted")
	}
	blob = []byte("tampered")
	target = installedBinary(t, "old")
	_, err = Run(t.Context(), Options{Current: "v9.9.8", Platform: "darwin-arm64", TargetPath: target, ConfigDir: t.TempDir(), Release: &selected})
	if err == nil {
		t.Fatal("accepted tampered Forgejo binary")
	}
	actual, err = os.ReadFile(target)
	if err != nil || string(actual) != "old" {
		t.Fatal("replaced original binary despite failed checksum")
	}
}
