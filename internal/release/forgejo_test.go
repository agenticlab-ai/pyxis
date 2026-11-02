package release_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
)

// An interrupted existing-asset download is not verified evidence. Publication
// must stop before uploading the remainder of the otherwise valid local set.
func TestForgejoPublishRejectsTruncatedExistingAsset(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	var writes atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes.Add(1)
			http.Error(w, "publication must stop after a truncated download", http.StatusInternalServerError)
			return
		}
		switch r.URL.Path {
		case "/api/v1/repos/o/r/releases/tags/" + fixtureVersion:
			if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
				t.Error(err)
			}
		case "/api/v1/repos/o/r/releases/1/assets":
			if err := json.NewEncoder(w).Encode([]forgerelease.Asset{{Name: release.SumsName, URL: "http://" + r.Host + "/attachments/" + release.SumsName}}); err != nil {
				t.Error(err)
			}
		case "/attachments/" + release.SumsName:
			w.Header().Set("Content-Length", "1024")
			if _, err := fmt.Fprint(w, "truncated"); err != nil {
				t.Error(err)
			}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	target := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: server.URL, Repository: "o/r"}
	result, err := release.Publish(t.Context(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Target: &target})
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated release asset error = %v, want unexpected EOF", err)
	}
	if result.Created || len(result.Actions) != 0 || writes.Load() != 0 {
		t.Fatalf("partial asset advanced publication: result=%+v writes=%d", result, writes.Load())
	}
}

func TestForgejoPublishVerifiesRerunAndNeverClobbers(t *testing.T) {
	out, _ := packFixture(t, fixtureRepo(t))
	blobs := map[string][]byte{}
	exists := false
	writes := 0
	var install []byte
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/releases/tags/"):
			if !exists {
				w.WriteHeader(404)
				return
			}
			if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
				t.Error(err)
			}
		case strings.Contains(r.URL.Path, "/tags/"):
			if _, err := fmt.Fprint(w, `{"commit":{"sha":"abc"}}`); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/api/v1/repos/o/r/releases" && r.Method == "POST":
			exists = true
			writes++
			if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/api/v1/repos/o/r/releases/1/assets" && r.Method == "GET":
			assets := []forgerelease.Asset{}
			if r.URL.Query().Get("page") == "" {
				for name := range blobs {
					assets = append(assets, forgerelease.Asset{Name: name, URL: "http://" + r.Host + "/attachments/" + name})
				}
			}
			if err := json.NewEncoder(w).Encode(assets); err != nil {
				t.Error(err)
			}
		case r.URL.Path == "/api/v1/repos/o/r/releases/1/assets" && r.Method == "POST":
			f, h, err := r.FormFile("attachment")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer func() {
				if err := f.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, found := blobs[h.Filename]; found {
				t.Error("attempted clobber")
			}
			blobs[h.Filename], err = io.ReadAll(f)
			if err != nil {
				t.Error(err)
			}
			writes++
			if _, err := fmt.Fprintf(w, `{"name":%q}`, h.Filename); err != nil {
				t.Error(err)
			}
		case strings.HasPrefix(r.URL.Path, "/attachments/"):
			name := strings.TrimPrefix(r.URL.Path, "/attachments/")
			_, _ = w.Write(blobs[name])
		case strings.HasSuffix(r.URL.Path, "/contents/install.sh"):
			if r.Method == "GET" {
				if install == nil {
					w.WriteHeader(404)
					return
				}
				if _, err := fmt.Fprintf(w, `{"encoding":"base64","content":%q}`, base64.StdEncoding.EncodeToString(install)); err != nil {
					t.Error(err)
				}
				return
			}
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			var err error
			install, err = base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Error(err)
			}
			w.WriteHeader(201)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer s.Close()
	target := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: s.URL, Repository: "o/r"}
	opts := release.PublishOptions{Tag: fixtureVersion, OutDir: out, Target: &target}
	first, err := release.Publish(t.Context(), opts)
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	wantWrites := writes
	second, err := release.Publish(t.Context(), opts)
	if err != nil || second.Created || writes != wantWrites {
		t.Fatalf("rerun: %+v %v writes=%d", second, err, writes)
	}
	for name, action := range second.Actions {
		if action != "verified" {
			t.Errorf("%s=%s", name, action)
		}
	}
	root := t.TempDir()
	sourceInstaller, err := os.ReadFile("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "install.sh"), sourceInstaller, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := release.Pins(t.Context(), release.PinsOptions{Version: fixtureVersion, RepoRoot: root, SourceRepo: "o/r", Target: &target}); err != nil {
		t.Fatal(err)
	}
	secondServer := httptest.NewServer(s.Config.Handler)
	defer secondServer.Close()
	dist := target
	dist.BaseURL = secondServer.URL
	if _, err := release.Mirror(t.Context(), release.MirrorOptions{Tag: fixtureVersion, RepoRoot: root, SourceRepo: "o/r", DistRepo: "o/r", SourceTarget: &target, DistTarget: &dist}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(install), secondServer.URL) {
		t.Fatal("distribution installer retained source server")
	}
	invalid := target
	invalid.BaseURL = "invalid"
	if _, err := release.Publish(t.Context(), release.PublishOptions{Tag: fixtureVersion, OutDir: out, Target: &invalid}); err == nil {
		t.Fatal("accepted invalid release connection")
	}
	for name := range blobs {
		blobs[name] = []byte("remote tampering")
		break
	}
	if _, err := release.Publish(t.Context(), opts); err == nil || !strings.Contains(err.Error(), "never clobbered") {
		t.Fatalf("accepted mismatch: %v", err)
	}
	if writes != wantWrites {
		t.Fatal("mutated mismatched release")
	}
	blobs["foreign"] = []byte("foreign")
	if _, err := release.Publish(t.Context(), opts); err == nil {
		t.Fatal("accepted foreign asset")
	}
	if err := os.WriteFile(filepath.Join(out, "foreign-local"), []byte("unexpected"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := release.Publish(t.Context(), opts); err == nil {
		t.Fatal("accepted defective local pack")
	}
}
