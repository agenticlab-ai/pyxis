package forgejorelease

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestNativeReleaseLifecycle(t *testing.T) {
	t.Setenv("FORGE_RELEASE_TEST_TOKEN", "test-secret")
	var server *httptest.Server
	published := false
	install := ""
	var assetBytes []byte
	var writes []string
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token test-secret" {
			t.Errorf("missing scoped credential")
			w.WriteHeader(401)
			return
		}
		key := r.Method + " " + r.URL.Path
		if r.Method != "GET" {
			writes = append(writes, key)
		}
		switch key {
		case "GET /api/v1/repos/o/r/releases/tags/v1":
			if !published {
				w.WriteHeader(404)
				return
			}
			if _, err := fmt.Fprint(w, `{"id":12,"tag_name":"v1"}`); err != nil {
				t.Error(err)
			}
		case "GET /api/v1/repos/o/r/tags/v1":
			if _, err := fmt.Fprint(w, `{"commit":{"sha":"commit-sha"}}`); err != nil {
				t.Error(err)
			}
		case "POST /api/v1/repos/o/r/tags":
			w.WriteHeader(201)
		case "POST /api/v1/repos/o/r/releases":
			published = true
			if _, err := fmt.Fprint(w, `{"id":12,"tag_name":"v1"}`); err != nil {
				t.Error(err)
			}
		case "GET /api/v1/repos/o/r/releases/12/assets":
			if assetBytes == nil {
				if _, err := fmt.Fprint(w, `[]`); err != nil {
					t.Error(err)
				}
				return
			}
			if err := json.NewEncoder(w).Encode([]forgerelease.Asset{{ID: 3, Name: "binary", URL: server.URL + "/attachments/binary"}}); err != nil {
				t.Error(err)
			}
		case "POST /api/v1/repos/o/r/releases/12/assets":
			file, _, err := r.FormFile("attachment")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			assetBytes, err = io.ReadAll(file)
			if err != nil {
				t.Error(err)
			}
			if r.URL.Query().Get("name") != "binary" {
				t.Error("missing asset name")
			}
			if _, err := fmt.Fprint(w, `{"id":3,"name":"binary"}`); err != nil {
				t.Error(err)
			}
		case "GET /attachments/binary":
			_, _ = w.Write(assetBytes)
		case "GET /api/v1/repos/o/r/contents/install.sh":
			if install == "" {
				w.WriteHeader(404)
				return
			}
			if _, err := fmt.Fprintf(w, `{"sha":"old","encoding":"base64","content":%q}`, base64.StdEncoding.EncodeToString([]byte(install))); err != nil {
				t.Error(err)
			}
		case "POST /api/v1/repos/o/r/contents/install.sh", "PUT /api/v1/repos/o/r/contents/install.sh":
			var in contents
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Error(err)
			}
			data, err := base64.StdEncoding.DecodeString(in.Content)
			if err != nil {
				t.Error(err)
			}
			install = string(data)
			w.WriteHeader(201)
		default:
			t.Errorf("unexpected %s", key)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	c, err := New(integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: server.URL, Repository: "o/r", TokenEnv: "FORGE_RELEASE_TEST_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, found, err := c.Lookup(ctx, "v1"); err != nil || found {
		t.Fatalf("lookup missing: %v %v", found, err)
	}
	if err := c.CreateTag(ctx, "v1", "commit-sha"); err != nil {
		t.Fatal(err)
	}
	rel, err := c.Create(ctx, "v1", true)
	if err != nil || rel.ID != 12 {
		t.Fatalf("create: %+v %v", rel, err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "binary")
	if err := os.WriteFile(file, []byte("verified artifact bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Upload(ctx, rel.ID, file); err != nil {
		t.Fatal(err)
	}
	body, err := c.Open(ctx, "v1", "binary")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err != nil || string(data) != "verified artifact bytes" {
		t.Fatalf("download: %q %v", data, err)
	}
	rel, _, err = c.Lookup(ctx, "v1")
	if err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := c.DownloadTo(ctx, rel.Assets[0], dest); err != nil {
		t.Fatal(err)
	}
	if err := c.DownloadTo(ctx, rel.Assets[0], dest); err == nil {
		t.Fatal("clobbered existing destination")
	}
	if _, err := c.Open(ctx, "v1", "absent"); err == nil {
		t.Fatal("accepted absent asset")
	}
	for _, want := range []string{"pins-v1", "pins-v1", "pins-v2"} {
		if _, err := c.WriteInstall(ctx, []byte(want), "v1"); err != nil {
			t.Fatal(err)
		}
	}
	data, err = c.ReadInstall(ctx)
	if err != nil || string(data) != "pins-v2" {
		t.Fatalf("install: %q %v", data, err)
	}
	if len(writes) != 5 {
		t.Fatalf("writes=%v, want tag/release/upload/create-install/update-install", writes)
	}
}

func TestReleaseRejectsUnsafeAssetsAndURLs(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escaped", `a\b`, "a\x00b"} {
		if safeAssetName(name) {
			t.Errorf("accepted %q", name)
		}
	}
	for _, assets := range []string{`[{"name":"../escape"}]`, `[{"name":"binary"},{"name":"binary"}]`} {
		t.Run(assets, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/assets") {
					if _, err := fmt.Fprint(w, assets); err != nil {
						t.Error(err)
					}
				} else {
					if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
						t.Error(err)
					}
				}
			}))
			defer s.Close()
			c, err := New(integrations.ResolvedRepository{BaseURL: s.URL, Repository: "o/r"})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := c.Lookup(t.Context(), "v1"); err == nil {
				t.Fatal("accepted unsafe listing")
			}
			if _, err := c.Download(t.Context(), forgerelease.Asset{Name: "binary", URL: "https://foreign.invalid/asset"}); err == nil {
				t.Fatal("accepted foreign credential destination")
			}
		})
	}
	if _, err := decodeContents(contents{Encoding: "other"}); err == nil {
		t.Fatal("accepted unknown encoding")
	}
	if _, err := decodeContents(contents{Encoding: "base64", Content: "!"}); err == nil {
		t.Fatal("accepted malformed base64")
	}
}

func TestReleaseAttachmentsAreAnUnpaginatedCompleteSet(t *testing.T) {
	var assets []forgerelease.Asset
	for i := int64(1); i <= 92; i++ {
		assets = append(assets, forgerelease.Asset{ID: i, Name: fmt.Sprintf("artifact-%03d", i)})
	}
	attachmentCalls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/assets") {
			attachmentCalls++
			if r.URL.RawQuery != "" {
				t.Errorf("unpaginated endpoint received query %q", r.URL.RawQuery)
			}
			// Forgejo 16 returns all attachments and ignores page/limit, so
			// repeating this endpoint would repeat the same 92 rows forever.
			if err := json.NewEncoder(w).Encode(assets); err != nil {
				t.Error(err)
			}
			return
		}
		if _, err := fmt.Fprint(w, `{"id":1}`); err != nil {
			t.Error(err)
		}
	}))
	defer s.Close()
	c, err := New(integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: s.URL, Repository: "o/r"})
	if err != nil {
		t.Fatal(err)
	}
	rel, found, err := c.Lookup(t.Context(), "v1.2.3")
	if err != nil || !found || len(rel.Assets) != 92 || attachmentCalls != 1 {
		t.Fatalf("complete attachment listing: found=%t assets=%d calls=%d err=%v", found, len(rel.Assets), attachmentCalls, err)
	}
}
