package forgejorelease

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func TestNativeReleaseFailureBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		call       func(*Client) error
	}{
		{"lookup forbidden", "", 403, func(c *Client) error { _, _, err := c.Lookup(t.Context(), "v1"); return err }},
		{"lookup missing id", `{}`, 200, func(c *Client) error { _, _, err := c.Lookup(t.Context(), "v1"); return err }},
		{"listing malformed", `{"id":1}`, 200, func(c *Client) error { _, _, err := c.Lookup(t.Context(), "v1"); return err }},
		{"create denied", "", 403, func(c *Client) error { _, err := c.Create(t.Context(), "v1", false); return err }},
		{"create missing id", `{}`, 200, func(c *Client) error { _, err := c.Create(t.Context(), "v1", false); return err }},
		{"create requires tag", "", 404, func(c *Client) error { _, err := c.Create(t.Context(), "v1", true); return err }},
		{"tag no commit", `{}`, 200, func(c *Client) error { _, err := c.TagCommit(t.Context(), "v1"); return err }},
		{"open forbidden", "", 403, func(c *Client) error { _, err := c.Open(t.Context(), "v1", "binary"); return err }},
		{"open missing release", "", 404, func(c *Client) error { _, err := c.Open(t.Context(), "v1", "binary"); return err }},
		{"install read denied", "", 403, func(c *Client) error { _, err := c.ReadInstall(t.Context()); return err }},
		{"install write denied", "", 403, func(c *Client) error { _, err := c.WriteInstall(t.Context(), []byte("pins"), "v1"); return err }},
		{"install malformed content", `{"encoding":"other"}`, 200, func(c *Client) error { _, err := c.WriteInstall(t.Context(), []byte("pins"), "v1"); return err }},
		{"install write failed", `{"encoding":"base64","content":"b2xk"}`, 200, func(c *Client) error { _, err := c.WriteInstall(t.Context(), []byte("pins"), "v1"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" {
					w.WriteHeader(403)
					return
				}
				w.WriteHeader(tc.status)
				if _, err := fmt.Fprint(w, tc.body); err != nil {
					t.Error(err)
				}
			}))
			defer s.Close()
			c, err := New(integrations.ResolvedRepository{BaseURL: s.URL, Repository: "o/r"})
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.call(c); err == nil {
				t.Fatal("failure accepted")
			}
		})
	}
	for _, target := range []integrations.ResolvedRepository{{BaseURL: "https://example.com", Repository: "invalid"}, {BaseURL: "not a URL", Repository: "o/r"}} {
		if _, err := New(target); err == nil {
			t.Fatal("invalid target accepted")
		}
	}
}

func TestUploadAndDownloadFailuresDoNotEscape(t *testing.T) {
	var reply string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, reply); err != nil {
			t.Error(err)
		}
	}))
	defer s.Close()
	c, err := New(integrations.ResolvedRepository{BaseURL: s.URL, Repository: "o/r"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Download(t.Context(), forgerelease.Asset{Name: "../unsafe"}); err == nil {
		t.Fatal("unsafe asset accepted")
	}
	if err := c.DownloadTo(t.Context(), forgerelease.Asset{Name: "binary", URL: "https://foreign.invalid/asset"}, t.TempDir()); err == nil {
		t.Fatal("foreign target accepted")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "binary")
	if err := os.WriteFile(file, []byte("bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"not-json", `{"name":"different"}`} {
		reply = r
		if err := c.Upload(t.Context(), 1, file); err == nil {
			t.Fatal("invalid upload response accepted")
		}
	}
	if err := c.Upload(t.Context(), 1, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("absent file accepted")
	}
	if err := c.Upload(t.Context(), 1, dir); err == nil {
		t.Fatal("directory uploaded")
	}
	if err := c.Upload(t.Context(), 1, "."); err == nil {
		t.Fatal("invalid filename accepted")
	}
	t.Setenv("TMPDIR", filepath.Join(dir, "absent"))
	if err := c.Upload(t.Context(), 1, file); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatal("invalid staging directory accepted")
	}
}
