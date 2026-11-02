package forgejopersist

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	api "github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNativeClientValidatesIdentityAndPermission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		id     int64
		push   bool
	}{
		{"valid", 200, 42, true}, {"read only", 200, 42, false}, {"missing identity", 200, 0, false}, {"unavailable", 503, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := api.NewWithHTTP("https://forgejo.test/base", "secret", &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				var body any
				switch {
				case strings.HasSuffix(r.URL.Path, "/user"):
					body = map[string]any{"id": tc.id, "login": "actor"}
				case strings.Contains(r.URL.Path, "/issues/"):
					body = map[string]any{"id": tc.id, "body": "artifact"}
				default:
					body = map[string]any{"id": tc.id, "permissions": map[string]bool{"push": tc.push}}
				}
				data, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(data))), Request: r}, nil
			})})
			if err != nil {
				t.Fatal(err)
			}
			c := &Client{API: client, Repository: integrations.ResolvedRepository{Provider: "forgejo", BaseURL: "https://forgejo.test/base", Repository: "owner/repo"}}
			id, err := c.PostComment(t.Context(), 7, "artifact")
			if (err == nil) != (tc.status == 200 && tc.id > 0) {
				t.Fatalf("post=%s %v", id, err)
			}
			_, err = c.Read(t.Context(), "42")
			if (err == nil) != (tc.status == 200 && tc.id == 42) {
				t.Fatalf("read=%v", err)
			}
			_, err = c.ReadComment(t.Context(), "42")
			if (err == nil) != (tc.status == 200 && tc.id == 42) {
				t.Fatalf("read body=%v", err)
			}
			_, err = c.CurrentLogin(t.Context())
			if (err == nil) != (tc.status == 200 && tc.id > 0) {
				t.Fatalf("login=%v", err)
			}
			node, err := c.RepositoryIdentity(t.Context())
			if (err == nil) != (tc.status == 200 && tc.id > 0) {
				t.Fatalf("repository=%s %v", node, err)
			}
			if err == nil {
				raw, err := base64.RawURLEncoding.DecodeString(node)
				if err != nil || string(raw) != "https://forgejo.test/base/repositories/42" {
					t.Fatalf("identity=%s %v", raw, err)
				}
			}
			writable, err := c.CanWriteComments(t.Context())
			if writable != tc.push || (err != nil) != (tc.status != 200) {
				t.Fatalf("permission=%v %v", writable, err)
			}
			if _, err := c.Read(t.Context(), "bad"); err == nil {
				t.Fatal("bad receipt accepted")
			}
		})
	}
	for _, binding := range []integrations.ResolvedRepository{{Provider: "github"}, {Provider: "forgejo", BaseURL: "bad"}, {Provider: "forgejo", BaseURL: "https://forgejo.test", Repository: "owner/repo"}} {
		if _, err := NewHostAdapter(binding, nil); err == nil {
			t.Fatal("missing store or bad provider accepted")
		}
	}
}
