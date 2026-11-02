package forgejo_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func newClient(t *testing.T, f roundTripFunc) *forgejo.Client {
	t.Helper()
	c, err := forgejo.NewWithHTTP("http://127.0.0.1:3000/forge", "secret", &http.Client{Transport: f})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAuthenticatedRequestAndCurrentUser(t *testing.T) {
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/forge/api/v1/user" || r.Header.Get("Authorization") != "token secret" || r.Header.Get("Cache-Control") != "no-cache" {
			t.Fatalf("wrong request: %s", r.URL)
		}
		return response(r, 200, `{"id":7,"login":"assistant","new_server_field":true}`), nil
	})
	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 7 || u.Login != "assistant" || c.BaseURL() != "http://127.0.0.1:3000/forge" {
		t.Fatalf("%#v", u)
	}
	if forgejo.RepoPath("owner/my repo", "pulls") != "repos/owner/my%20repo/pulls" {
		t.Fatal("repository path not escaped")
	}
}

func TestListHandlesServerPageClampAndKeepsQuery(t *testing.T) {
	calls := 0
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if q.Get("state") != "open" || q.Get("limit") != "50" {
			t.Fatalf("lost query: %s", r.URL)
		}
		switch q.Get("page") {
		case "1":
			return response(r, 200, `[{"id":1}]`), nil
		case "2":
			return response(r, 200, `[{"id":2}]`), nil
		case "3":
			return response(r, 200, `[]`), nil
		default:
			t.Fatal("unexpected page")
			return nil, errors.New("unexpected page")
		}
	})
	var rows []struct {
		ID int `json:"id"`
	}
	if err := c.List(context.Background(), "repos/o/r/issues?state=open", &rows); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(rows) != 2 || rows[1].ID != 2 {
		t.Fatalf("incomplete result: %d %#v", calls, rows)
	}
}

func TestListContinuesPastFilteredEmptyPage(t *testing.T) {
	for _, withLink := range []bool{true, false} {
		calls := 0
		c := newClient(t, func(r *http.Request) (*http.Response, error) {
			calls++
			resp := response(r, 200, `[]`)
			resp.Header.Set("X-Total-Count", "2")
			if r.URL.Query().Get("page") == "1" && withLink {
				resp.Header.Set("Link", `<http://127.0.0.1:3000/forge/api/v1/reviews?page=2>; rel="next"`)
			}
			if r.URL.Query().Get("page") == "2" {
				resp = response(r, 200, `[{"id":7}]`)
				resp.Header.Set("X-Total-Count", "2")
				if withLink {
					resp.Header.Set("Link", `<http://127.0.0.1:3000/forge/api/v1/reviews?page=1>; rel="prev"`)
				}
			}
			return resp, nil
		})
		var rows []struct {
			ID int `json:"id"`
		}
		if err := c.List(context.Background(), "repos/o/r/pulls/1/reviews", &rows); err != nil || len(rows) != 1 || rows[0].ID != 7 {
			t.Fatalf("filtered reviews truncated: rows %#v err %v", rows, err)
		}
		if calls > 3 {
			t.Fatalf("unexpected pagination calls: %d", calls)
		}
	}
}

func TestIncompleteListDoesNotPublishPartialResult(t *testing.T) {
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "1" {
			return response(r, 200, `[{"id":1}]`), nil
		}
		return response(r, 503, `{"message":"secret"}`), nil
	})
	rows := []forgejo.User{{ID: 99, Login: "previous"}}
	err := c.List(context.Background(), "repos/o/r/issues", &rows)
	var status *forgejo.HTTPError
	if !errors.As(err, &status) || status.Status != 503 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("status/secret failure: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != 99 {
		t.Fatal("partial list escaped")
	}
}

func TestExactListRefusesNullWithoutPublishingRows(t *testing.T) {
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("page") == "1" {
			return response(r, 200, `[{"id":1}]`), nil
		}
		return response(r, 200, `null`), nil
	})
	rows := []forgejo.User{{ID: 99, Login: "previous"}}
	if err := c.ListExact(context.Background(), "repos/o/r/issues/42/dependencies", &rows); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("null dependency page established an empty set: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != 99 {
		t.Fatal("partial dependency set escaped")
	}
	// Existing plan consumers retain their transport compatibility. Strict
	// dependency admission belongs to the exact issue reader.
	if err := c.List(context.Background(), "repos/o/r/issues", &rows); err != nil || len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("ordinary list changed: %+v %v", rows, err)
	}
}

func TestNativeResponseRejectsOversizeAndIncompleteIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"oversized metadata", strings.Repeat(" ", 16<<20) + `{}`, "size bound"},
		{"incomplete identity", `{}`, "incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newClient(t, func(r *http.Request) (*http.Response, error) {
				return response(r, 200, tc.body), nil
			})
			if _, err := c.CurrentUser(context.Background()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid native fact accepted: %v", err)
			}
		})
	}
}

func TestRedirectContainmentAndNoWriteReplay(t *testing.T) {
	for _, tc := range []struct {
		name, method, location string
		calls                  int
		success                bool
	}{
		{"same origin read", "GET", "http://127.0.0.1:3000/forge/attachments/123", 2, true},
		{"different origin", "GET", "http://127.0.0.1:4000/forge/attachments/123", 1, false},
		{"base path escape", "GET", "http://127.0.0.1:3000/other/attachments/123", 1, false},
		{"write replay", "POST", "http://127.0.0.1:3000/forge/api/v1/other", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := newClient(t, func(r *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 {
					return response(r, 200, `{}`), nil
				}
				resp := response(r, 307, "")
				resp.Header.Set("Location", tc.location)
				return resp, nil
			})
			err := c.Request(context.Background(), tc.method, "repos/o/r/releases", nil, nil)
			if (err == nil) != tc.success || calls != tc.calls {
				t.Fatalf("calls %d err %v", calls, err)
			}
		})
	}
}

func TestFullAssetURLAndForeignEndpointRefusal(t *testing.T) {
	calls := 0
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/forge/attachments/123" {
			t.Fatalf("%s", r.URL)
		}
		return response(r, 200, "asset"), nil
	})
	resp, err := c.Stream(context.Background(), "GET", "http://127.0.0.1:3000/forge/attachments/123", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"https://example.test/asset", "//example.test/asset", "http://user:secret@127.0.0.1:3000/forge/a", "../user", "repos/o/../../user", "repos/o/%2e%2e%2fuser", "http://127.0.0.1:3000/forge/../other"} {
		if err := c.Request(context.Background(), "GET", p, nil, nil); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", p)
		}
	}
	if calls != 1 {
		t.Fatalf("unsafe endpoint reached transport: %d calls", calls)
	}
}

func TestBranchAndCompareRetainEscapedRefSeparators(t *testing.T) {
	for _, endpoint := range []string{"repos/o/r/branches/feat%2Fscope%2Ftask", "repos/o/r/compare/main...feat%2Fscope"} {
		t.Run(endpoint, func(t *testing.T) {
			c := newClient(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.EscapedPath() != "/forge/api/v1/"+endpoint {
					t.Fatalf("escaped ref changed: %s", r.URL.EscapedPath())
				}
				return response(r, 200, `{}`), nil
			})
			if err := c.Request(context.Background(), "GET", endpoint, nil, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHTTPFailureNeverRetriesWriteOrReflectsBody(t *testing.T) {
	calls := 0
	c := newClient(t, func(r *http.Request) (*http.Response, error) {
		calls++
		return response(r, 429, "secret request contents"), nil
	})
	err := c.Request(context.Background(), "POST", "repos/o/r/issues", struct {
		Title string `json:"title"`
	}{"example"}, nil)
	var e *forgejo.HTTPError
	if !errors.As(err, &e) || e.Status != 429 || calls != 1 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("calls %d err %v", calls, err)
	}
}

func TestCancellationAndValidation(t *testing.T) {
	c := newClient(t, func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.Request(ctx, "GET", "user", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	for _, base := range []string{"https://user:secret@example.test", "http://remote.example", "https://example.test/?token=x", "https://example.test/a/../b", "file:///tmp/forgejo"} {
		if _, err := forgejo.New(base, "secret"); err == nil {
			t.Fatalf("unsafe base accepted %s", base)
		}
	}
	if err := c.List(context.Background(), "user", nil); err == nil {
		t.Fatal("invalid list output accepted")
	}
}
