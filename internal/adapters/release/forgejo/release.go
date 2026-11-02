// Package forgejorelease implements the Forgejo release and attachment API.
package forgejorelease

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/forgerelease"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func init() {
	forgerelease.Bind(func(repo integrations.ResolvedRepository) (forgerelease.Host, error) {
		return New(repo)
	})
}

type Client struct {
	Repo integrations.ResolvedRepository
	api  *forgejo.Client
}

var _ forgerelease.Host = (*Client)(nil)

func New(repo integrations.ResolvedRepository) (*Client, error) {
	if err := integrations.ValidateRepository(repo.Repository); err != nil {
		return nil, err
	}
	c, err := forgejo.New(repo.BaseURL, repo.Token())
	if err != nil {
		return nil, err
	}
	return &Client{Repo: repo, api: c}, nil
}

func (c *Client) path(suffix string) string { return forgejo.RepoPath(c.Repo.Repository, suffix) }

func (c *Client) Lookup(ctx context.Context, tag string) (forgerelease.Release, bool, error) {
	var r forgerelease.Release
	err := c.api.Request(ctx, http.MethodGet, c.path("/releases/tags/"+url.PathEscape(tag)), nil, &r)
	var he *forgejo.HTTPError
	if errors.As(err, &he) && he.Status == http.StatusNotFound {
		return forgerelease.Release{}, false, nil
	}
	if err != nil {
		return forgerelease.Release{}, false, err
	}
	if r.ID <= 0 {
		return forgerelease.Release{}, false, fmt.Errorf("forgejo release: tag %q has no release id", tag)
	}
	// Forgejo's attachment-list endpoint returns the complete release set.
	// It accepts no page/limit parameters (and ignores them if supplied), so
	// generic pagination repeats the same assets until its safety bound.
	if err := c.api.Request(ctx, http.MethodGet, c.path(fmt.Sprintf("/releases/%d/assets", r.ID)), nil, &r.Assets); err != nil {
		return forgerelease.Release{}, false, err
	}
	seen := map[string]bool{}
	for _, a := range r.Assets {
		if !safeAssetName(a.Name) || seen[a.Name] {
			return forgerelease.Release{}, false, fmt.Errorf("forgejo release: invalid or duplicate asset name %q", a.Name)
		}
		seen[a.Name] = true
	}
	return r, true, nil
}

func safeAssetName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name && !strings.ContainsAny(name, "\\\x00\r\n")
}

func (c *Client) Create(ctx context.Context, tag string, requireTag bool) (forgerelease.Release, error) {
	if requireTag {
		if _, err := c.TagCommit(ctx, tag); err != nil {
			return forgerelease.Release{}, err
		}
	}
	var r forgerelease.Release
	err := c.api.Request(ctx, http.MethodPost, c.path("/releases"), struct {
		Tag  string `json:"tag_name"`
		Name string `json:"name"`
		Body string `json:"body"`
	}{tag, tag, "Pyxis system release; artifact checksums and release manifest define the immutable published set."}, &r)
	if err == nil && r.ID <= 0 {
		return forgerelease.Release{}, fmt.Errorf("forgejo release: created release has no id")
	}
	return r, err
}

func (c *Client) TagCommit(ctx context.Context, tag string) (string, error) {
	var r struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	err := c.api.Request(ctx, http.MethodGet, c.path("/tags/"+url.PathEscape(tag)), nil, &r)
	if err != nil {
		return "", err
	}
	if r.Commit.SHA == "" {
		return "", fmt.Errorf("forgejo release: tag %q has no commit", tag)
	}
	return r.Commit.SHA, nil
}

func (c *Client) CreateTag(ctx context.Context, tag, commit string) error {
	return c.api.Request(ctx, http.MethodPost, c.path("/tags"), struct {
		Name   string `json:"tag_name"`
		Target string `json:"target"`
	}{tag, commit}, nil)
}

func (c *Client) Download(ctx context.Context, a forgerelease.Asset) (io.ReadCloser, error) {
	if !safeAssetName(a.Name) {
		return nil, fmt.Errorf("forgejo release: invalid asset name %q", a.Name)
	}
	r, err := c.api.Stream(ctx, http.MethodGet, a.URL, "", nil)
	if err != nil {
		return nil, err
	}
	return r.Body, nil
}

func (c *Client) Open(ctx context.Context, tag, name string) (io.ReadCloser, error) {
	r, found, err := c.Lookup(ctx, tag)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("forgejo release: tag %q is not published on %s", tag, c.Repo.Key())
	}
	for _, a := range r.Assets {
		if a.Name == name {
			return c.Download(ctx, a)
		}
	}
	return nil, fmt.Errorf("forgejo release: %q is missing from %s release %s", name, c.Repo.Key(), tag)
}

func (c *Client) DownloadTo(ctx context.Context, a forgerelease.Asset, dir string) (err error) {
	r, err := c.Download(ctx, a)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Close()) }()
	f, err := os.OpenFile(filepath.Join(dir, a.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	return errors.Join(copyErr, closeErr)
}

func (c *Client) Upload(ctx context.Context, releaseID int64, file string) (err error) {
	name := filepath.Base(file)
	if !safeAssetName(name) {
		return fmt.Errorf("forgejo release: invalid asset name %q", name)
	}
	// A temporary multipart file bounds memory while the HTTP request streams.
	body, err := os.CreateTemp("", "pyxis-release-upload-*")
	if err != nil {
		return err
	}
	defer func() {
		// The HTTP transport closes a request body after handoff, including on
		// request failure. Closing our staging file again is harmless cleanup.
		if closeErr := body.Close(); !errors.Is(closeErr, os.ErrClosed) {
			err = errors.Join(err, closeErr)
		}
		_ = os.Remove(body.Name()) // Best-effort removal of private staging bytes; publication outcome is already known.
	}()
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("attachment", name)
	if err != nil {
		return err
	}
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(part, src)
	if err := errors.Join(copyErr, src.Close()); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	if _, err := body.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r, err := c.api.Stream(ctx, http.MethodPost, c.path(fmt.Sprintf("/releases/%d/assets?name=%s", releaseID, url.QueryEscape(name))), mw.FormDataContentType(), body)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.Body.Close()) }()
	var a forgerelease.Asset
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		return err
	}
	if a.Name != name {
		return fmt.Errorf("forgejo release: uploaded asset name %q differs from %q", a.Name, name)
	}
	return nil
}

type contents struct {
	SHA      string `json:"sha"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (c *Client) ReadInstall(ctx context.Context) ([]byte, error) {
	var cur contents
	if err := c.api.Request(ctx, http.MethodGet, c.path("/contents/install.sh"), nil, &cur); err != nil {
		return nil, err
	}
	return decodeContents(cur)
}

func decodeContents(cur contents) ([]byte, error) {
	if cur.Encoding != "base64" {
		return nil, fmt.Errorf("forgejo release: unexpected contents encoding %q", cur.Encoding)
	}
	return base64.StdEncoding.DecodeString(strings.Join(strings.Fields(cur.Content), ""))
}

func (c *Client) WriteInstall(ctx context.Context, want []byte, tag string) (string, error) {
	var cur contents
	err := c.api.Request(ctx, http.MethodGet, c.path("/contents/install.sh"), nil, &cur)
	method := http.MethodPut
	var he *forgejo.HTTPError
	switch {
	case err == nil:
		b, err := decodeContents(cur)
		if err != nil {
			return "", err
		}
		if bytes.Equal(b, want) {
			return "verified", nil
		}
	case errors.As(err, &he) && he.Status == http.StatusNotFound:
		method = http.MethodPost
	default:
		return "", err
	}
	err = c.api.Request(ctx, method, c.path("/contents/install.sh"), struct {
		Content string `json:"content"`
		SHA     string `json:"sha,omitempty"`
		Message string `json:"message"`
	}{base64.StdEncoding.EncodeToString(want), cur.SHA, "chore: refresh install.sh pins for " + tag}, nil)
	if err != nil {
		return "", err
	}
	return "written", nil
}
