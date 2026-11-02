// Package forgejo provides the shared authenticated Forgejo API transport.
// Consumer-owned adapters define the capability-specific request/response types.
package forgejo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	maxJSONBytes = 16 << 20
	maxListBytes = 64 << 20
	maxPages     = 1000
	pageSize     = 50
)

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

// BaseURL is the normalized server origin and optional installation base path.
func (c *Client) BaseURL() string { return c.base.String() }

// HTTPError exposes only the HTTP status, not untrusted server response text
// that may echo a credential or request content.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("forgejo HTTP %d: %s", e.Status, e.Message) }

type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

func New(baseURL, token string) (*Client, error) { return NewWithHTTP(baseURL, token, nil) }

// NewWithHTTP clones the supplied client so credential containment and the
// timeout apply without mutating a client's other users.
func NewWithHTTP(baseURL, token string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("forgejo base URL must identify an HTTP(S) server without credentials, query, or fragment")
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme == "http" && host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback()) {
		return nil, errors.New("forgejo HTTP is restricted to loopback hosts")
	}
	if strings.ContainsAny(u.Path, "\\\x00\r\n") || (u.Path != "" && u.Path != "/" && path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/")) {
		return nil, errors.New("forgejo base URL contains an unsafe path")
	}
	if strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("forgejo token contains invalid header characters")
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	client := http.Client{Timeout: 2 * time.Minute}
	if httpClient != nil {
		client = *httpClient
	}
	if client.Timeout <= 0 {
		client.Timeout = 2 * time.Minute
	}
	previousRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("forgejo redirect limit exceeded")
		}
		if len(via) > 0 && via[0].Method != http.MethodGet && via[0].Method != http.MethodHead {
			return errors.New("forgejo refuses to redirect or replay a write")
		}
		if req.URL.Scheme != u.Scheme || !strings.EqualFold(req.URL.Host, u.Host) || req.URL.User != nil || !withinBase(req.URL.Path, u.Path) {
			req.Header.Del("Authorization")
			return errors.New("forgejo refused redirect outside configured server")
		}
		if previousRedirect != nil {
			return previousRedirect(req, via)
		}
		return nil
	}
	return &Client{base: u, token: token, http: &client}, nil
}

func withinBase(requestPath, basePath string) bool {
	return basePath == "" || requestPath == basePath || strings.HasPrefix(requestPath, basePath+"/")
}

// RepoPath constructs a repository-relative API path. Request performs final
// path validation; callers cannot escape the configured API root through it.
func RepoPath(repo, suffix string) string {
	parts := strings.Split(repo, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	result := "repos/" + strings.Join(parts, "/")
	if suffix != "" {
		result += "/" + strings.TrimLeft(suffix, "/")
	}
	return result
}

func (c *Client) endpoint(p string) (*url.URL, error) {
	rel, err := url.Parse(p)
	if err != nil || rel.User != nil || rel.Fragment != "" {
		return nil, errors.New("forgejo endpoint must be an API-relative path")
	}
	if rel.IsAbs() || rel.Host != "" {
		if rel.Scheme != c.base.Scheme || !strings.EqualFold(rel.Host, c.base.Host) || !withinBase(rel.Path, c.base.Path) {
			return nil, errors.New("forgejo endpoint is outside configured server")
		}
	}
	if strings.ContainsAny(rel.Path, "\\\x00\r\n") {
		return nil, errors.New("forgejo endpoint contains an unsafe path")
	}
	for segment := range strings.SplitSeq(rel.Path, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("forgejo endpoint contains a traversal segment")
		}
	}
	if rel.IsAbs() {
		return rel, nil
	}
	relPath := strings.TrimLeft(rel.EscapedPath(), "/")
	relPath = strings.TrimPrefix(relPath, "api/v1/")
	u := *c.base
	u.RawPath = c.base.EscapedPath() + "/api/v1/" + relPath
	u.Path, err = url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, errors.New("forgejo endpoint encoding is invalid")
	}
	u.RawQuery = rel.RawQuery
	return &u, nil
}

// Stream performs one API request without replaying writes. The caller owns a
// successful response body and must close it. Non-success bodies are closed.
func (c *Client) Stream(ctx context.Context, method, p, contentType string, body io.Reader) (*http.Response, error) {
	u, err := c.endpoint(p)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create forgejo request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "pyxis-forgejo")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("forgejo request: %w", ctx.Err())
		}
		// Transport errors can include a URL. Do not expose its query or token.
		return nil, errors.New("forgejo request failed (transport, timeout, or refused redirect)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// No server body is included in errors, avoiding reflection of secrets.
		if err := resp.Body.Close(); err != nil {
			return nil, fmt.Errorf("close failed forgejo response: %w", err)
		}
		return nil, &HTTPError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	return resp, nil
}

func (c *Client) Request(ctx context.Context, method, p string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode forgejo request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	resp, err := c.Stream(ctx, method, p, "application/json", reader)
	if err != nil {
		return err
	}
	return decodeResponse(resp, out)
}

func decodeResponse(resp *http.Response, out any) (resultErr error) {
	defer func() {
		if err := resp.Body.Close(); resultErr == nil && err != nil {
			resultErr = fmt.Errorf("close forgejo response: %w", err)
		}
	}()
	if out == nil {
		_, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxJSONBytes))
		return err
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes+1))
	if err != nil {
		return fmt.Errorf("read forgejo response: %w", err)
	}
	if len(b) > maxJSONBytes {
		return errors.New("forgejo JSON response exceeds size bound")
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode forgejo response: %w", err)
	}
	return nil
}

// List completes page-number pagination before returning the combined array.
// It deliberately does not assume that the server honors the requested page
// size: an installation may clamp it below the request's limit.
func (c *Client) List(ctx context.Context, p string, out any) error {
	return c.list(ctx, p, out, false)
}

// ListExact requires every native fact page to be an array. A null response
// cannot establish that an authorization or dependency set is empty.
func (c *Client) ListExact(ctx context.Context, p string, out any) error {
	return c.list(ctx, p, out, true)
}

func (c *Client) list(ctx context.Context, p string, out any, exact bool) error {
	t := reflect.TypeOf(out)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Slice || reflect.ValueOf(out).IsNil() {
		return errors.New("forgejo list output must be a non-nil pointer to a slice")
	}
	u, err := url.Parse(p)
	if err != nil {
		return errors.New("forgejo list endpoint is invalid")
	}
	query := u.Query()
	query.Set("limit", strconv.Itoa(pageSize))
	var all []json.RawMessage
	totalBytes := 0
	for pageNumber := 1; pageNumber <= maxPages; pageNumber++ {
		query.Set("page", strconv.Itoa(pageNumber))
		u.RawQuery = query.Encode()
		resp, err := c.Stream(ctx, http.MethodGet, u.String(), "", nil)
		if err != nil {
			return err
		}
		var rows []json.RawMessage
		if err := decodeResponse(resp, &rows); err != nil {
			return err
		}
		if exact && rows == nil {
			return errors.New("forgejo list returned null instead of an array; result is incomplete")
		}
		all = append(all, rows...)
		for _, row := range rows {
			totalBytes += len(row)
		}
		if totalBytes > maxListBytes {
			return errors.New("forgejo list exceeds size bound; result is incomplete")
		}
		// Some Forgejo endpoints paginate before filtering private rows.
		// Such a page may be empty even while a later page is public. The
		// server's explicit continuation therefore governs before emptiness.
		if link := resp.Header.Get("Link"); link != "" {
			if strings.Contains(link, `rel="next"`) || strings.Contains(link, "rel=next") {
				continue
			}
			return assignList(all, out)
		}
		if count, err := strconv.Atoi(resp.Header.Get("X-Total-Count")); err == nil && count >= 0 {
			// With no Link, count may still describe unfiltered rows. The
			// effective page size is unknown (servers can clamp it), so the
			// conservative final empty page is bounded by one raw row/page.
			if len(all) >= count || len(rows) == 0 && pageNumber >= count {
				return assignList(all, out)
			}
			continue
		}
		if len(rows) == 0 {
			return assignList(all, out)
		}
	}
	return errors.New("forgejo pagination exceeded page bound; result is incomplete")
}

func assignList(rows []json.RawMessage, out any) error {
	if rows == nil {
		rows = []json.RawMessage{}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("assemble forgejo list: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode forgejo list: %w", err)
	}
	return nil
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var user User
	if err := c.Request(ctx, http.MethodGet, "user", nil, &user); err != nil {
		return user, err
	}
	if user.ID <= 0 || user.Login == "" {
		return User{}, errors.New("forgejo authenticated user response is incomplete")
	}
	return user, nil
}
